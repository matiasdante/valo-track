# VALO-TRACK

Una herramienta para analizar y rastrear estadísticas de partidas competitivas de Valorant jugadas en 5-stack con arquitectura modular y gestión de rate limiting.

## 📋 Contenidos

- [Características](#características)
- [Estructura del Proyecto](#estructura-del-proyecto)
- [Instalación](#instalación)
- [Configuración](#configuración)
- [Uso](#uso)
- [Arquitectura Detallada](#arquitectura-detallada)
- [Rate Limiting y Optimización](#rate-limiting-y-optimización)
- [Desarrollo](#desarrollo)

## ✨ Características

- **Arquitectura Modular**: Separación clara de responsabilidades (config, modelos, API, cola, analytics)
- **Pool de Usuarios Simultáneos**: Gestión de múltiples perfiles sin límites artificiales
- **Rate Limiting Inteligente**: Control estricto de 30 requests/minuto con batching automático
- **Sistema de Cola**: Procesa solicitudes con workers paralelos respetando límites
- **Hardcoding**: 100% configuración por variables de entorno
- **Análisis Profundo**: Estadísticas de combate, trades, clutches, multi-kills
- **Gestión por Lado**: Separación de stats de ataque vs defensa
- **Persistencia JSON**: Almacenamiento de datos de partidas para análisis offline
- **Reintentos Automáticos**: Manejo robusto de fallos de API

## 📁 Estructura del Proyecto

```
valo-track/
├── cmd/
│   └── valo-track/
│       └── main.go               
├── internal/
│   ├── config/
│   │   └── config.go               
│   ├── models/
│   │   └── models.go              
│   ├── api/
│   │   └── client.go              
│   ├── queue/
│   │   └── queue.go                
│   ├── analytics/
│   │   └── service.go              
│   └── storage/
│       └── (expansión futura)
├── configs/                        
├── .env.example                   
├── go.mod                         
├── go.sum                        
├── Makefile                        
└── README.md                      

```

## 🚀 Instalación

### Requisitos

- **Go 1.21+** ([Descargar](https://golang.org/dl/))
- **API Key** de HenrikDev Valorant API ([Aquí](https://api.henrikdev.xyz/))
- **Git** (opcional)

### Pasos de Instalación

1. **Clonar o descargar el repositorio**
   ```bash
   git clone <repo-url>
   cd valo-track
   ```

2. **Instalar dependencias Go**
   ```bash
   go mod download
   go mod verify
   ```

3. **Crear archivo `.env`**
   ```bash
   cp .env.example .env
   ```

4. **Editar `.env` con tus valores**
   ```bash
   nano .env  # O tu editor favorito
   ```
   
   **Variables obligatorias:**
   - `VALO_API_KEY`: Tu API key de HenrikDev
   - `VALO_MAIN_PLAYER_NAME`: Nombre del jugador principal
   - `VALO_MAIN_PLAYER_TAG`: Tag del jugador principal

5. **Compilar el proyecto**
   ```bash
   go build -o valo-track ./cmd/valo-track
   ```

## ⚙️ Configuración

### Variables de Entorno Principales

```bash
# AUTENTICACIÓN
VALO_API_KEY=tu_api_key_aqui                    

# REGIÓN
VALO_REGION=na                                  

# RATE LIMITING
VALO_MAX_REQUESTS_PER_MINUTE=30                 
VALO_BATCH_SIZE=5                          

# JUGADOR PRINCIPAL
VALO_MAIN_PLAYER_NAME=matutEv                  # Nombre
VALO_MAIN_PLAYER_TAG=VAC                       # Tag

# ANÁLISIS
VALO_QUEUE_MODE=competitive                     
VALO_MAX_GAMES=35                               
VALO_MIN_STACK_PLAYERS=4                        
VALO_TRADE_WINDOW_MS=5000                      

# TIMEOUTS Y REINTENTOS
VALO_REQUEST_TIMEOUT=12s                  
VALO_MAX_RETRIES=3                     
```

Ver `.env.example` para documentación completa.

## 📖 Uso

### Actualizar datos desde API

Descarga las últimas partidas del jugador:

```bash
./valo-track -update
```

**Salida esperada:**
```
=== ACTUALIZACIÓN DE DATOS ===
Consultando partidas de Rosarino#CARC...
Descargando 35 partidas...
  Progreso: 0/35
  Progreso: 10/35
  Progreso: 20/35
  Progreso: 30/35
✅ Datos actualizados exitosamente
```

### Analizar partidas

Calcula estadísticas consolidadas:

```bash
./valo-track -analyze
```

**Salida esperada:**
```
=== ANÁLISIS DE PARTIDAS ===
Procesando 35 partidas para Rosarino#CARC...
  Progreso: 0/35
  Progreso: 10/35
  ...

=== ESTADÍSTICAS DE Rosarino ===

📊 RESUMEN GENERAL
   Partidas jugadas: 35
   Victorias/Derrotas: 21/14
   Win Rate: 60.0%
   Total de rondas: 665

💀 COMBATE
   Kills: 724
   Deaths: 521
   Assists: 312
   ...
```

### Actualizar y analizar (combinado)

```bash
./valo-track -update -analyze
```

### Con Makefile

```bash
make install    # Instalar dependencias
make build      # Compilar
make run        # Compilar y analizar
make update     # Actualizar datos
```

## 🏗️ Arquitectura Detallada

### Módulo: Config

**Ubicación:** `internal/config/config.go`

**Responsabilidad:** Gestionar toda la configuración de la aplicación

**Características:**
- Carga variables de entorno con validación
- Valores por defecto seguros
- Mapeo de cuentas de Valorant a nombres reales
- Inicialización de configuración centralizada

**Ejemplo:**
```go
cfg, err := config.LoadConfig()
if err != nil {
    log.Fatal(err)
}
fmt.Println(cfg.MaxRequestsPerMinute) // 30
```

### Módulo: Models

**Ubicación:** `internal/models/models.go`

**Responsabilidad:** Definir estructuras de datos

**Tipos principales:**
- `PlayerStats`: Estadísticas consolidadas
- `MatchData`: Datos de una partida
- `AnalysisRequest`: Solicitud de análisis
- `AnalysisResult`: Resultado de análisis
- `RateLimitStatus`: Estado del rate limiter

### Módulo: API Client

**Ubicación:** `internal/api/client.go`

**Responsabilidad:** Comunicación con API de Valorant

**Métodos principales:**
- `GetLifetimeMatches()`: Obtiene IDs de partidas
- `GetMatchDetailsV4()`: Descarga detalles de partida
- `GetPlayerPUUID()`: Obtiene PUUID del jugador

**Características de confiabilidad:**
- Reintentos automáticos con backoff exponencial
- Manejo de rate limits (429)
- Timeout configurable
- Errores descriptivos

### Módulo: Queue (Rate Limiting)

**Ubicación:** `internal/queue/queue.go`

**Responsabilidad:** Gestionar cola de solicitudes con control de rate limit

**Algoritmo de Rate Limiting:**
```
Ventana deslizante de 60 segundos
- Mantener lista de timestamps de requests
- Limpiar requests fuera de ventana
- Si count >= maxRequests, esperar a que expire el más antiguo
- Procesar nuevo request
```

**Características:**
- Workers paralelos configurables
- Batching automático
- Control estricto de 30 req/min
- Estado en tiempo real

**Ejemplo:**
```go
rq := queue.NewRequestQueue(30, 5, 100)
rq.StartWorkers(3, processor)

req := &models.AnalysisRequest{
    PlayerName: "Rosarino",
    PlayerTag: "CARC",
}

result := <-rq.Enqueue(req)
rq.Stop()
```

### Módulo: Analytics

**Ubicación:** `internal/analytics/service.go`

**Responsabilidad:** Procesar y analizar datos de partidas

**Funcionalidades:**
- Identificar jugadores del stack
- Calcular estadísticas individuales
- Detectar trades (venganzas)
- Contar multi-kills (2K/3K/4K/5K)
- Identificar clutches
- Estadísticas de ataque vs defensa
- Cálculo de KAST (Kill/Assist/Survive/Trade)

**Ejemplo de procesamiento:**
```go
analytics := analytics.NewAnalyticsService(
    cfg.PlayerAccountsMap,
    cfg.TradeWindowMs,
)

match := analytics.ProcessMatchDetails(
    apiResponse,
    cfg.MinStackPlayers,
)
```

### Flujo Principal

```
┌─────────────────────────────────────────────────────────┐
│                    MAIN.GO (Orquestador)                │
└─────────────────────────────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
    ┌───▼────┐        ┌───▼──────┐      ┌───▼──────┐
    │ Config │        │ API      │      │ Analytics│
    │ Loader │        │ Client   │      │ Service  │
    └────────┘        └──────────┘      └──────────┘
                           │
                      ┌────▼─────┐
                      │ Queue    │
                      │ (Workers)│
                      └──────────┘
                           │
                      ┌────▼──────┐
                      │ Storage   │
                      │ (JSON)    │
                      └───────────┘
```

## ⚡ Rate Limiting y Optimización

### El Problema: Límite de 30 Requests/Minuto

La API de HenrikDev tiene un límite estricto de **30 requests por minuto**. Un enfoque ingenuo sería:

```go
// ❌ MALO: Sin optimización
for i := 0; i < 35; i++ {
    match := api.GetMatchDetails(matchID)  // Usa 1 request
    // Esperar 2 segundos entre requests
    time.Sleep(2 * time.Second)
}
// Total: 35 requests × 2 segundos = 70 segundos
```

### Nuestra Solución: Batching + Rate Limiting

#### 1. **Descubrimiento Inteligente**
- Solo 1 request para obtener lista de partidas
- Descarga en paralelo respetando límite

#### 2. **Ventana Deslizante de 60 Segundos**
```
Tiempo:  0s      10s      20s      30s      40s      50s      60s
Req:      X       X        X        X        X        X        X
         [Ventana deslizante de 60 segundos]
         
Si llega nuevo request en 61s, el primer request ya salió de ventana
```

#### 3. **Batching Paralelo**
```
Worker 1: Request A → Espera → Response A
Worker 2: Request B → Espera → Response B
Worker 3: Request C → Espera → Response C
Worker 1: Request D → Espera → Response D
...

3 workers × 35 partidas ÷ 30 requests/min = ~70 segundos
(En lugar de 70-100 segundos sin batching)
```

#### 4. **Configuración Recomendada**
```bash
# Máximo: 30 requests/minuto (límite API)
VALO_MAX_REQUESTS_PER_MINUTE=30

# Óptimo para equilibrio velocidad/confiabilidad
VALO_BATCH_SIZE=5

# Parallelismo en main.go
numWorkers = 3

# Resultado: ~70 segundos para 35 partidas
```

### Cálculo de Tiempo

Para **35 partidas** con **3 workers** y **BATCH_SIZE=5**:

```
30 requests/minuto = 0.5 requests/segundo = 2 segundos por request

Batches: ⌈35/5⌉ = 7 batches
Tiempo total: 7 batches × 3 workers (paralelo) × 2s ≈ 47 segundos
Margen de seguridad: +20% = ~60 segundos
```

### Monitoreo en Tiempo Real

```bash
$ ./valo-track -analyze

📊 Estado del Rate Limiter:
   Requests en esta ventana: 7/30
   Solicitudes pendientes: 2
```

## 🔧 Desarrollo

### Estructura de Paquetes

```
valo-track/
├── cmd/valo-track/main.go          # Punto de entrada
├── internal/
│   ├── config/                     # Configuración
│   ├── models/                     # Tipos de datos
│   ├── api/                        # Integración con API
│   ├── queue/                      # Sistema de cola
│   └── analytics/                  # Lógica de análisis
```

### Agregar Nuevo Endpoint

1. **Agregar struct en `models/models.go`**
2. **Agregar método en `api/client.go`**
3. **Usar en `main.go`**

### Extender Rate Limiting

La clase `queue.RequestQueue` es agnóstica al procesamiento:

```go
customProcessor := func(req *models.AnalysisRequest) *models.AnalysisResult {
    // Tu lógica personalizada
}

rq.StartWorkers(5, customProcessor)
```

### Testing

```bash
# (En desarrollo futuro)
go test ./...
go test -v ./internal/queue/
```

## 📊 Ejemplos de Salida

### Estadísticas Completas

```
=== ESTADÍSTICAS DE Rosarino ===

📊 RESUMEN GENERAL
   Partidas jugadas: 35
   Victorias/Derrotas: 21/14
   Win Rate: 60.0%
   Total de rondas: 665

💀 COMBATE
   Kills: 724
   Deaths: 521
   Assists: 312
   K/D Promedio: 1.39
   Kills por partida: 20.7
   Headshots/Bodyshots/Legshots: 89/156/34

🎯 ESTADÍSTICAS AVANZADAS
   First Kills: 34
   First Deaths: 28
   KAST Rounds: 412
   Clutches: 12
   Multi-Kills:
      2K: 67
      3K: 23
      4K: 5
      5K: 1

⚔️  ATAQUE vs DEFENSA
   Ataque  - Kills/Deaths/Damage: 412/245/2847
   Defensa - Kills/Deaths/Damage: 312/276/2103

💰 ECONOMÍA
   Score total: 15420
   Damage hecho/Recibido: 4950/3456

🎮 AGENTES MÁS JUGADOS
   Reyna: 12 veces
   Omen: 8 veces
   Jett: 7 veces
   Sage: 5 veces
   Astra: 3 veces

📈 ÚLTIMAS PARTIDAS ANALIZADAS: 35
```

## 🐛 Troubleshooting

### "API error: status code 401"
→ Verifica `VALO_API_KEY` en `.env`

### "Error obteniendo PUUID"
→ Verifica `VALO_MAIN_PLAYER_NAME` y `VALO_MAIN_PLAYER_TAG`

### "Rate limited (429)"
→ El sistema reintentará automáticamente. Si persiste, reduce `VALO_BATCH_SIZE`

### "No hay datos de partidas"
→ Ejecuta `./valo-track -update` primero

## 📝 Notas de Versión

### v2.0 (Actual - Refactorización)
- Arquitectura modular completa
- Sistema de cola con workers
- Rate limiting inteligente
- 100% configuración por env vars
- Pool de usuarios (arquitectura lista)
- Código legible y documentado
- README

### v1.0 (Original)
- Monolito en main.go
- Sin control de rate limit
- Hardcoding de valores

## 📄 Licencia

MIT License - Libre para uso personal y comercial

## ⚖️ Disclaimer

Esta herramienta no está afiliada con Riot Games ni HenrikDev. 
Úsala responsablemente respetando los terms of service de ambos servicios.

---

