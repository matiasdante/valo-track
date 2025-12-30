package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contiene la configuración de la aplicación cargada desde variables de entorno
type Config struct {
	// API Configuration
	APIKey           string
	APIRegion        string
	RequestTimeout   time.Duration
	MaxRetries       int

	// Rate Limiting
	MaxRequestsPerMinute int
	BatchSize            int

	// Player Pool Configuration
	MainPlayerName     string
	MainPlayerTag      string
	PlayerAccountsMap  map[string]string // Mapeo de cuentas de Valorant a nombres reales
	MinStackPlayers    int

	// Game Analysis
	QueueMode            string
	MaxGamesToAnalyze    int
	TradeWindowMs        int
	RecentMatchesToShow  int
	TimeframeAnalysis    string

	// Storage
	StatsOutputFile string
	MatchDataFile   string
	ConfigDir       string
}

// LoadConfig carga la configuración desde variables de entorno con valores por defecto seguros
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// API Configuration (valores por defecto seguros)
		APIKey:         getEnv("VALO_API_KEY", ""),
		APIRegion:      getEnv("VALO_REGION", "na"),
		RequestTimeout: parseDuration(getEnv("VALO_REQUEST_TIMEOUT", "12s"), 12*time.Second),
		MaxRetries:     parseInt(getEnv("VALO_MAX_RETRIES", "3"), 3),

		// Rate Limiting (30 requests per minute es el límite estricto de la API)
		MaxRequestsPerMinute: parseInt(getEnv("VALO_MAX_REQUESTS_PER_MINUTE", "30"), 30),
		BatchSize:            parseInt(getEnv("VALO_BATCH_SIZE", "5"), 5),

		// Player Pool Configuration
		MainPlayerName:  getEnv("VALO_MAIN_PLAYER_NAME", "Rosarino"),
		MainPlayerTag:   getEnv("VALO_MAIN_PLAYER_TAG", "CARC"),
		MinStackPlayers: parseInt(getEnv("VALO_MIN_STACK_PLAYERS", "4"), 4),

		// Game Analysis
		QueueMode:           getEnv("VALO_QUEUE_MODE", "competitive"),
		MaxGamesToAnalyze:   parseInt(getEnv("VALO_MAX_GAMES", "35"), 35),
		TradeWindowMs:       parseInt(getEnv("VALO_TRADE_WINDOW_MS", "5000"), 5000),
		RecentMatchesToShow: parseInt(getEnv("VALO_RECENT_MATCHES_TO_SHOW", "35"), 35),
		TimeframeAnalysis:   getEnv("VALO_TIMEFRAME", "all"),

		// Storage
		StatsOutputFile: getEnv("VALO_STATS_OUTPUT_FILE", "stats.txt"),
		MatchDataFile:   getEnv("VALO_MATCH_DATA_FILE", "matches.json"),
		ConfigDir:       getEnv("VALO_CONFIG_DIR", "./configs"),
	}

	// Validaciones críticas
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("error crítico: VALO_API_KEY no está definida. Define la variable de entorno o usa .env")
	}

	if cfg.MainPlayerName == "" || cfg.MainPlayerTag == "" {
		return nil, fmt.Errorf("error crítico: VALO_MAIN_PLAYER_NAME y VALO_MAIN_PLAYER_TAG deben estar definidas")
	}

	// Cargar mapeo de cuentas de jugadores
	cfg.PlayerAccountsMap = getPlayerAccountsMap()

	return cfg, nil
}

// getPlayerAccountsMap retorna el mapeo de cuentas de Valorant a nombres reales
// Este mapeo puede ser cargado desde un archivo JSON en el futuro
func getPlayerAccountsMap() map[string]string {
	return map[string]string{
		// Rosarino
		"Rosarino#CARC": "Rosarino",

		// Santi
		"Lessツ#2222":      "Santi",
		"Cuuurlyta#cutie": "Santi",

		// matutEv
		"b1chito#LAS":       "matutEv",
		"Di Maria#CARC":     "matutEv",
		"matutEv#9439":      "matutEv",
		"matutEv#CABJ":      "matutEv",
		"matutEv#VAC":       "matutEv",

		// Dxy
		"Dxy9#9999":          "Dxy",
		"matutEv#2003":       "Dxy",
		"Pokemon Player#Ros": "Dxy",

		// Klowiz
		"klowiz#cenfe": "Klowiz",
		"Bnn#6979":     "Klowiz",

		// Pxn
		"Eliam#qqq": "Pxn",

		// Chapa
		"Chapa#7851": "Chapa",
	}
}

// getEnv obtiene una variable de entorno con un valor por defecto
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseInt convierte un string a int con valor por defecto
func parseInt(value string, defaultValue int) int {
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}
	return defaultValue
}

// parseDuration convierte un string a time.Duration con valor por defecto
func parseDuration(value string, defaultValue time.Duration) time.Duration {
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	return defaultValue
}
