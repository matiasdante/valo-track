package models

// PlayerStats contiene las estadísticas agregadas de un jugador
type PlayerStats struct {
	Name           string
	Kills          int
	Deaths         int
	Assists        int
	Headshots      int
	Bodyshots      int
	Legshots       int
	Score          int
	Wins           int
	Losses         int
	TotalGames     int
	TotalRounds    int
	Agents         map[string]int // Contador de agentes usados
	DamageMade     int
	DamageReceived int
	FirstKills     int
	FirstDeaths    int
	KASTRounds     int // Rondas con Kill, Assist, Survive o Trade

	// Stats por lado
	AttackKills   int
	AttackDeaths  int
	AttackDamage  int
	AttackRounds  int
	DefenseKills  int
	DefenseDeaths int
	DefenseDamage int
	DefenseRounds int
	MultiKills    map[int]int // 2K/3K/4K/5K (ace)
	Clutches      int
}

// MatchData contiene los datos de una partida y su análisis
type MatchData struct {
	MatchID      string
	Map          string
	Mode         string
	PlayerData   map[string]PlayerMatchStats
	Won          bool
	RoundsPlayed int
	FirstKills   map[string]int
	FirstDeaths  map[string]int
	KASTRounds   map[string]int
	PlayerTeams  map[string]string // TeamID (Red o Blue)

	// Stats por lado
	AttackKills   map[string]int
	AttackDeaths  map[string]int
	AttackDamage  map[string]int
	AttackRounds  map[string]int
	DefenseKills  map[string]int
	DefenseDeaths map[string]int
	DefenseDamage map[string]int
	DefenseRounds map[string]int
	MultiKills    map[string]map[int]int
	Clutches      map[string]int
	Timestamp     int64 // Timestamp de la partida
}

// PlayerMatchStats contiene los stats de un jugador en una partida específica
type PlayerMatchStats struct {
	Kills          int
	Deaths         int
	Assists        int
	Headshots      int
	Bodyshots      int
	Legshots       int
	Score          int
	Agent          string
	Team           string // TeamID
	DamageMade     int
	DamageReceived int
}

// KillEvent representa un evento de muerte en una ronda
type KillEvent struct {
	Round       int
	Time        int
	KillerName  string
	VictimName  string
	KillerTeam  string
	VictimTeam  string
	KillerPUUID string
	VictimPUUID string
	Assistants  []string
}

// AnalysisRequest representa una solicitud de análisis para un usuario
type AnalysisRequest struct {
	PlayerName string
	PlayerTag  string
	Region     string
	QueueMode  string
	MaxGames   int
	Priority   int // Para ordenamiento en cola
}

// AnalysisResult contiene el resultado del análisis de un usuario
type AnalysisResult struct {
	PlayerName string
	PlayerTag  string
	Stats      *PlayerStats
	Matches    []MatchData
	Error      error
	Timestamp  int64
}

// RateLimitStatus contiene información sobre el estado del rate limiter
type RateLimitStatus struct {
	RequestsMade      int
	RequestsRemaining int
	ResetTime         int64
	IsThrottled       bool
}
