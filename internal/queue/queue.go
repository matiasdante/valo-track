package queue

import (
	"context"
	"sync"
	"time"
	"valo-track/internal/models"
)

// RequestQueue gestiona una cola de solicitudes con soporte para batching y rate limiting
type RequestQueue struct {
	queue              chan *models.AnalysisRequest
	results            map[string]chan *models.AnalysisResult
	requestsMutex      sync.RWMutex
	maxRequests        int           // Máximo de requests por minuto (30)
	batchSize          int           // Tamaño del batch
	requestTimestamps  []time.Time   // Timestamps de requests recientes
	throttleUntil      time.Time
	throttleMutex      sync.RWMutex
	activeWorkers      int
	workersMutex       sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
}

// NewRequestQueue crea una nueva cola de solicitudes
// maxRequests: máximo de requests por minuto (ej: 30)
// batchSize: cantidad de requests a procesar simultáneamente (ej: 5)
// maxQueueSize: tamaño máximo de la cola (ej: 100)
func NewRequestQueue(maxRequests, batchSize, maxQueueSize int) *RequestQueue {
	ctx, cancel := context.WithCancel(context.Background())

	rq := &RequestQueue{
		queue:             make(chan *models.AnalysisRequest, maxQueueSize),
		results:           make(map[string]chan *models.AnalysisResult),
		maxRequests:       maxRequests,
		batchSize:         batchSize,
		requestTimestamps: make([]time.Time, 0, maxRequests),
		ctx:               ctx,
		cancel:            cancel,
	}

	return rq
}

// Enqueue añade una solicitud a la cola
// Retorna un canal donde se recibirá el resultado
func (rq *RequestQueue) Enqueue(req *models.AnalysisRequest) <-chan *models.AnalysisResult {
	// Crear canal de resultado único para esta solicitud
	resultChan := make(chan *models.AnalysisResult, 1)
	key := req.PlayerName + "#" + req.PlayerTag

	rq.requestsMutex.Lock()
	rq.results[key] = resultChan
	rq.requestsMutex.Unlock()

	select {
	case rq.queue <- req:
		return resultChan
	case <-rq.ctx.Done():
		// Queue está cerrada
		close(resultChan)
		return resultChan
	}
}

// StartWorkers inicia N workers para procesar la cola
func (rq *RequestQueue) StartWorkers(numWorkers int, processor func(*models.AnalysisRequest) *models.AnalysisResult) {
	rq.workersMutex.Lock()
	rq.activeWorkers = numWorkers
	rq.workersMutex.Unlock()

	for i := 0; i < numWorkers; i++ {
		rq.wg.Add(1)
		go rq.worker(processor)
	}
}

// worker procesa las solicitudes de la cola respetando el rate limit
func (rq *RequestQueue) worker(processor func(*models.AnalysisRequest) *models.AnalysisResult) {
	defer rq.wg.Done()

	batch := make([]*models.AnalysisRequest, 0, rq.batchSize)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-rq.ctx.Done():
			// Procesar batch pendiente antes de salir
			if len(batch) > 0 {
				rq.processBatch(batch, processor)
			}
			return

		case req := <-rq.queue:
			batch = append(batch, req)

			// Procesar batch cuando alcanza el tamaño máximo
			if len(batch) >= rq.batchSize {
				rq.processBatch(batch, processor)
				batch = make([]*models.AnalysisRequest, 0, rq.batchSize)
			}

		case <-ticker.C:
			// Procesar batch si hay solicitudes pendientes (timeout)
			if len(batch) > 0 {
				rq.processBatch(batch, processor)
				batch = make([]*models.AnalysisRequest, 0, rq.batchSize)
			}
		}
	}
}

// processBatch procesa un lote de solicitudes respetando el rate limit
func (rq *RequestQueue) processBatch(batch []*models.AnalysisRequest, processor func(*models.AnalysisRequest) *models.AnalysisResult) {
	for _, req := range batch {
		// Verificar si estamos throttled
		rq.throttleMutex.RLock()
		if time.Now().Before(rq.throttleUntil) {
			rq.throttleMutex.RUnlock()
			// Esperar hasta que se pueda hacer el siguiente request
			time.Sleep(time.Until(rq.throttleUntil))
			rq.throttleMutex.Lock()
			rq.throttleUntil = time.Now()
			rq.throttleMutex.Unlock()
		} else {
			rq.throttleMutex.RUnlock()
		}

		// Verificar rate limit: máximo maxRequests en 60 segundos
		rq.allowRequest()

		// Procesar la solicitud
		result := processor(req)

		// Enviar resultado al canal correspondiente
		key := req.PlayerName + "#" + req.PlayerTag
		rq.requestsMutex.RLock()
		if ch, ok := rq.results[key]; ok {
			select {
			case ch <- result:
			case <-rq.ctx.Done():
			}
			close(ch)
			delete(rq.results, key)
		}
		rq.requestsMutex.RUnlock()
	}
}

// allowRequest verifica si podemos hacer un request sin violar el rate limit
// Si es necesario, espera hasta que sea permitido
func (rq *RequestQueue) allowRequest() {
	rq.requestsMutex.Lock()
	defer rq.requestsMutex.Unlock()

	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	// Remover timestamps que están fuera de la ventana de 60 segundos
	validIdx := 0
	for i, ts := range rq.requestTimestamps {
		if ts.After(oneMinuteAgo) {
			validIdx = i
			break
		}
	}
	rq.requestTimestamps = rq.requestTimestamps[validIdx:]

	// Si hemos alcanzado el límite, esperar
	if len(rq.requestTimestamps) >= rq.maxRequests {
		// Calcular cuánto tiempo esperar hasta que el request más antiguo salga de la ventana
		oldestRequest := rq.requestTimestamps[0]
		waitDuration := time.Until(oldestRequest.Add(time.Minute))
		if waitDuration > 0 {
			time.Sleep(waitDuration)
			// Recursivamente llamar después de esperar (el timestamp ya no estará en la ventana)
			rq.allowRequest()
			return
		}
	}

	// Agregar el timestamp actual
	rq.requestTimestamps = append(rq.requestTimestamps, now)
}

// GetStatus retorna el estado actual de la cola y rate limit
func (rq *RequestQueue) GetStatus() *models.RateLimitStatus {
	rq.requestsMutex.RLock()
	defer rq.requestsMutex.RUnlock()

	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	// Contar requests válidos en la ventana
	validCount := 0
	for _, ts := range rq.requestTimestamps {
		if ts.After(oneMinuteAgo) {
			validCount++
		}
	}

	resetTime := now.Unix() + 60
	if len(rq.requestTimestamps) > 0 {
		resetTime = rq.requestTimestamps[0].Add(time.Minute).Unix()
	}

	return &models.RateLimitStatus{
		RequestsMade:      validCount,
		RequestsRemaining: rq.maxRequests - validCount,
		ResetTime:         resetTime,
		IsThrottled:       len(rq.queue) > rq.batchSize*2, // Consideramos throttled si hay mucha acumulación
	}
}

// Stop detiene la procesamiento de la cola y espera que terminen todos los workers
func (rq *RequestQueue) Stop() {
	rq.cancel()
	rq.wg.Wait()
	close(rq.queue)
}

// QueueSize retorna el número de solicitudes pendientes en la cola
func (rq *RequestQueue) QueueSize() int {
	return len(rq.queue)
}
