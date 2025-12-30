package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"valo-track/internal/analytics"
	"valo-track/internal/api"
	"valo-track/internal/config"
	"valo-track/internal/models"
	"valo-track/internal/queue"
)

func main() {
	// Parsear argumentos de línea de comandos
	analyzeFlag := flag.Bool("analyze", false, "Realizar análisis de partidas")
	updateFlag := flag.Bool("update", false, "Actualizar datos desde API")
	flag.Parse()

	// Cargar configuración desde variables de entorno
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error cargando configuración: %v", err)
	}

	// Crear cliente de API
	apiClient := api.NewAPIClient(cfg.APIKey, cfg.APIRegion, cfg.RequestTimeout, cfg.MaxRetries)

	// Crear servicio de análisis
	analyticsService := analytics.NewAnalyticsService(cfg.PlayerAccountsMap, cfg.TradeWindowMs)

	// Crear almacenamiento
	storage := NewFileStorage(cfg.MatchDataFile, cfg.StatsOutputFile)

	// Crear cola de solicitudes con rate limiting
	reqQueue := queue.NewRequestQueue(cfg.MaxRequestsPerMinute, cfg.BatchSize, 100)

	// Definir el procesador que usa la cola
	processor := func(req *models.AnalysisRequest) *models.AnalysisResult {
		return ProcessAnalysisRequest(req, apiClient, analyticsService, cfg)
	}

	// Iniciar workers
	numWorkers := 3
	reqQueue.StartWorkers(numWorkers, processor)

	if *updateFlag {
		fmt.Println("=== ACTUALIZACIÓN DE DATOS ===")
		fmt.Printf("Consultando partidas de %s#%s...\n", cfg.MainPlayerName, cfg.MainPlayerTag)

		err := UpdateMatchData(apiClient, analyticsService, cfg, storage)
		if err != nil {
			log.Fatalf("Error actualizando datos: %v", err)
		}
		fmt.Println("✅ Datos actualizados exitosamente")
	}

	if *analyzeFlag {
		fmt.Println("=== ANÁLISIS DE PARTIDAS ===")

		// Cargar datos guardados
		matches, err := storage.LoadMatches()
		if err != nil {
			log.Fatalf("Error cargando datos de partidas: %v", err)
		}

		if len(matches) == 0 {
			fmt.Println("⚠️  No hay datos de partidas. Ejecuta con -update primero.")
			return
		}

		// Procesar análisis a través de la cola
		req := &models.AnalysisRequest{
			PlayerName: cfg.MainPlayerName,
			PlayerTag:  cfg.MainPlayerTag,
			Region:     cfg.APIRegion,
			QueueMode:  cfg.QueueMode,
			MaxGames:   cfg.MaxGamesToAnalyze,
		}

		resultChan := reqQueue.Enqueue(req)
		result := <-resultChan

		if result.Error != nil {
			log.Fatalf("Error en análisis: %v", result.Error)
		}

		// Mostrar resultados
		PrintAnalysis(result.Stats, matches)

		// Guardar output
		err = storage.SaveStats(result.Stats, matches)
		if err != nil {
			log.Printf("Advertencia: No se pudieron guardar estadísticas: %v", err)
		}
	}

	// Obtener estado del rate limiter
	status := reqQueue.GetStatus()
	fmt.Printf("\n📊 Estado del Rate Limiter:\n")
	fmt.Printf("   Requests en esta ventana: %d/%d\n", status.RequestsMade, cfg.MaxRequestsPerMinute)
	fmt.Printf("   Solicitudes pendientes: %d\n", reqQueue.QueueSize())

	// Detener la cola
	reqQueue.Stop()
}

// ProcessAnalysisRequest procesa una solicitud de análisis
func ProcessAnalysisRequest(req *models.AnalysisRequest, apiClient *api.APIClient, analyticsService *analytics.AnalyticsService, cfg *config.Config) *models.AnalysisResult {
	result := &models.AnalysisResult{
		PlayerName: req.PlayerName,
		PlayerTag:  req.PlayerTag,
		Timestamp:  0, // Será actualizado
	}

	// Obtener lista de partidas
	matchIDs, err := apiClient.GetLifetimeMatches(req.PlayerName, req.PlayerTag, req.QueueMode)
	if err != nil {
		result.Error = fmt.Errorf("error obteniendo partidas: %w", err)
		return result
	}

	if len(matchIDs) == 0 {
		result.Error = fmt.Errorf("no se encontraron partidas para %s#%s", req.PlayerName, req.PlayerTag)
		return result
	}

	// Limitar a MaxGames
	if len(matchIDs) > req.MaxGames {
		matchIDs = matchIDs[:req.MaxGames]
	}

	fmt.Printf("Procesando %d partidas para %s#%s...\n", len(matchIDs), req.PlayerName, req.PlayerTag)

	// Procesar cada partida
	matches := make([]models.MatchData, 0, len(matchIDs))
	stackPlayerNames := []string{req.PlayerName}

	for i, matchID := range matchIDs {
		if i%10 == 0 {
			fmt.Printf("  Progreso: %d/%d\n", i, len(matchIDs))
		}

		// Obtener detalles de la partida
		apiMatch, err := apiClient.GetMatchDetailsV4(matchID)
		if err != nil {
			fmt.Printf("  ⚠️  Error procesando partida %s: %v\n", matchID, err)
			continue
		}

		// Procesar y analizar
		match := analyticsService.ProcessMatchDetails(apiMatch, cfg.MinStackPlayers)
		if match != nil {
			matches = append(matches, *match)
		}
	}

	// Consolidar estadísticas
	stats := analyticsService.AnalyzeMatches(matches, stackPlayerNames)
	stats.Name = req.PlayerName

	result.Stats = stats
	result.Matches = matches

	return result
}

// UpdateMatchData actualiza los datos de partidas desde la API
func UpdateMatchData(apiClient *api.APIClient, analyticsService *analytics.AnalyticsService, cfg *config.Config, storage *FileStorage) error {
	matchIDs, err := apiClient.GetLifetimeMatches(cfg.MainPlayerName, cfg.MainPlayerTag, cfg.QueueMode)
	if err != nil {
		return err
	}

	if len(matchIDs) > cfg.MaxGamesToAnalyze {
		matchIDs = matchIDs[:cfg.MaxGamesToAnalyze]
	}

	fmt.Printf("Descargando %d partidas...\n", len(matchIDs))

	matches := make([]models.MatchData, 0)

	for i, matchID := range matchIDs {
		if i%10 == 0 {
			fmt.Printf("  Progreso: %d/%d\n", i, len(matchIDs))
		}

		apiMatch, err := apiClient.GetMatchDetailsV4(matchID)
		if err != nil {
			fmt.Printf("  ⚠️  Error en partida %s: %v\n", matchID, err)
			continue
		}

		match := analyticsService.ProcessMatchDetails(apiMatch, cfg.MinStackPlayers)
		if match != nil {
			matches = append(matches, *match)
		}
	}

	return storage.SaveMatches(matches)
}

// PrintAnalysis imprime un análisis bonito de los stats
func PrintAnalysis(stats *models.PlayerStats, matches []models.MatchData) {
	if stats == nil {
		fmt.Println("❌ No hay estadísticas para mostrar")
		return
	}

	fmt.Printf("\n=== ESTADÍSTICAS DE %s ===\n\n", stats.Name)
	fmt.Printf("📊 RESUMEN GENERAL\n")
	fmt.Printf("   Partidas jugadas: %d\n", stats.TotalGames)
	fmt.Printf("   Victorias/Derrotas: %d/%d\n", stats.Wins, stats.Losses)
	if stats.TotalGames > 0 {
		winRate := float64(stats.Wins) * 100 / float64(stats.TotalGames)
		fmt.Printf("   Win Rate: %.1f%%\n", winRate)
	}
	fmt.Printf("   Total de rondas: %d\n\n", stats.TotalRounds)

	fmt.Printf("💀 COMBATE\n")
	fmt.Printf("   Kills: %d\n", stats.Kills)
	fmt.Printf("   Deaths: %d\n", stats.Deaths)
	fmt.Printf("   Assists: %d\n", stats.Assists)
	if stats.TotalGames > 0 {
		fmt.Printf("   K/D Promedio: %.2f\n", float64(stats.Kills)/float64(stats.Deaths+1))
		fmt.Printf("   Kills por partida: %.1f\n", float64(stats.Kills)/float64(stats.TotalGames))
	}

	fmt.Printf("   Headshots/Bodyshots/Legshots: %d/%d/%d\n\n", stats.Headshots, stats.Bodyshots, stats.Legshots)

	fmt.Printf("🎯 ESTADÍSTICAS AVANZADAS\n")
	fmt.Printf("   First Kills: %d\n", stats.FirstKills)
	fmt.Printf("   First Deaths: %d\n", stats.FirstDeaths)
	fmt.Printf("   KAST Rounds: %d\n", stats.KASTRounds)
	fmt.Printf("   Clutches: %d\n", stats.Clutches)

	fmt.Printf("   Multi-Kills:\n")
	for count := 2; count <= 5; count++ {
		fmt.Printf("      %dK: %d\n", count, stats.MultiKills[count])
	}

	fmt.Printf("\n⚔️  ATAQUE vs DEFENSA\n")
	fmt.Printf("   Ataque  - Kills/Deaths/Damage: %d/%d/%d\n", stats.AttackKills, stats.AttackDeaths, stats.AttackDamage)
	fmt.Printf("   Defensa - Kills/Deaths/Damage: %d/%d/%d\n", stats.DefenseKills, stats.DefenseDeaths, stats.DefenseDamage)

	fmt.Printf("\n💰 ECONOMÍA\n")
	fmt.Printf("   Score total: %d\n", stats.Score)
	fmt.Printf("   Damage hecho/Recibido: %d/%d\n", stats.DamageMade, stats.DamageReceived)

	fmt.Printf("\n🎮 AGENTES MÁS JUGADOS\n")
	for agent, count := range stats.Agents {
		fmt.Printf("   %s: %d veces\n", agent, count)
	}

	fmt.Printf("\n📈 ÚLTIMAS PARTIDAS ANALIZADAS: %d\n", len(matches))
}

// FileStorage gestiona la persistencia de datos
type FileStorage struct {
	matchDataFile string
	statsFile     string
}

// NewFileStorage crea un nuevo gestor de almacenamiento
func NewFileStorage(matchDataFile, statsFile string) *FileStorage {
	return &FileStorage{
		matchDataFile: matchDataFile,
		statsFile:     statsFile,
	}
}

// LoadMatches carga los datos de partidas guardados
func (fs *FileStorage) LoadMatches() ([]models.MatchData, error) {
	data, err := ioutil.ReadFile(fs.matchDataFile)
	if err != nil {
		return nil, err
	}

	var matches []models.MatchData
	if err := json.Unmarshal(data, &matches); err != nil {
		return nil, err
	}

	return matches, nil
}

// SaveMatches guarda los datos de partidas
func (fs *FileStorage) SaveMatches(matches []models.MatchData) error {
	data, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fs.matchDataFile, data, 0644)
}

// SaveStats guarda el análisis de estadísticas
func (fs *FileStorage) SaveStats(stats *models.PlayerStats, matches []models.MatchData) error {
	f, err := os.Create(fs.statsFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "[%s]\n", stats.Name)
	fmt.Fprintf(f, "Partidas: %d | Victorias: %d | Derrotas: %d | WR: %.1f%%\n",
		stats.TotalGames, stats.Wins, stats.Losses,
		float64(stats.Wins)*100/float64(stats.TotalGames+1))
	fmt.Fprintf(f, "K/D/A: %d/%d/%d | KDA: %.2f | +/-: %d\n",
		stats.Kills, stats.Deaths, stats.Assists,
		float64(stats.Kills+stats.Assists)/float64(stats.Deaths+1),
		stats.Kills-stats.Deaths)

	fmt.Fprintf(f, "Promedios: %.1f/%.1f/%.1f por partida\n",
		float64(stats.Kills)/float64(stats.TotalGames+1),
		float64(stats.Deaths)/float64(stats.TotalGames+1),
		float64(stats.Assists)/float64(stats.TotalGames+1))

	fmt.Fprintf(f, "ACS: %.2f | ADR: %.2f\n",
		float64(stats.Score)/float64(stats.TotalGames+1),
		float64(stats.DamageMade)/float64(stats.TotalRounds+1))

	fmt.Fprintf(f, "FK/FD: %d/%d (%.1f%%)\n",
		stats.FirstKills, stats.FirstDeaths,
		float64(stats.FirstKills)*100/float64(stats.FirstKills+stats.FirstDeaths+1))

	kastPct := float64(stats.KASTRounds) * 100 / float64(stats.TotalRounds+1)
	if kastPct >= 60 {
		fmt.Fprintf(f, "KAST: %.1f%% [OK] (%d rondas)\n", kastPct, stats.KASTRounds)
	} else {
		fmt.Fprintf(f, "KAST: %.1f%% [LOW] (%d rondas)\n", kastPct, stats.KASTRounds)
	}

	fmt.Fprintf(f, "Multi-Kills: 2K: %d | 3K: %d | 4K: %d | 5K: %d\n",
		stats.MultiKills[2], stats.MultiKills[3], stats.MultiKills[4], stats.MultiKills[5])

	fmt.Fprintf(f, "Clutches: %d\n", stats.Clutches)

	return nil
}
