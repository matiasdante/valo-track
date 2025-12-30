package analytics

import (
	"fmt"
	"sort"
	"valo-track/internal/models"
)

// AnalyticsService realiza análisis de partidas y cálculo de estadísticas
type AnalyticsService struct {
	playerAccountsMap map[string]string
	tradeWindowMs     int
}

// NewAnalyticsService crea un nuevo servicio de análisis
func NewAnalyticsService(playerAccountsMap map[string]string, tradeWindowMs int) *AnalyticsService {
	return &AnalyticsService{
		playerAccountsMap: playerAccountsMap,
		tradeWindowMs:     tradeWindowMs,
	}
}

// AnalyzeMatches procesa un conjunto de partidas y construye estadísticas consolidadas
func (as *AnalyticsService) AnalyzeMatches(matches []models.MatchData, playerNames []string) *models.PlayerStats {
	stats := &models.PlayerStats{
		Agents:     make(map[string]int),
		MultiKills: make(map[int]int),
	}

	// Inicializar contador de victorias/derrotas
	for _, match := range matches {
		for _, playerName := range playerNames {
			if _, ok := match.PlayerData[playerName]; ok {
				// El jugador participó en esta partida
				if match.Won {
					stats.Wins++
				} else {
					stats.Losses++
				}
				stats.TotalGames++
				stats.TotalRounds += match.RoundsPlayed

				// Acumular stats del jugador en esta partida
				playerMatchStats := match.PlayerData[playerName]
				stats.Kills += playerMatchStats.Kills
				stats.Deaths += playerMatchStats.Deaths
				stats.Assists += playerMatchStats.Assists
				stats.Headshots += playerMatchStats.Headshots
				stats.Bodyshots += playerMatchStats.Bodyshots
				stats.Legshots += playerMatchStats.Legshots
				stats.Score += playerMatchStats.Score
				stats.DamageMade += playerMatchStats.DamageMade
				stats.DamageReceived += playerMatchStats.DamageReceived

				// Contar agentes
				if playerMatchStats.Agent != "" {
					stats.Agents[playerMatchStats.Agent]++
				}

				// Acumular stats por lado
				if fk, ok := match.FirstKills[playerName]; ok {
					stats.FirstKills += fk
				}
				if fd, ok := match.FirstDeaths[playerName]; ok {
					stats.FirstDeaths += fd
				}
				if kast, ok := match.KASTRounds[playerName]; ok {
					stats.KASTRounds += kast
				}

				// Stats de ataque
				stats.AttackKills += match.AttackKills[playerName]
				stats.AttackDeaths += match.AttackDeaths[playerName]
				stats.AttackDamage += match.AttackDamage[playerName]
				stats.AttackRounds += match.AttackRounds[playerName]

				// Stats de defensa
				stats.DefenseKills += match.DefenseKills[playerName]
				stats.DefenseDeaths += match.DefenseDeaths[playerName]
				stats.DefenseDamage += match.DefenseDamage[playerName]
				stats.DefenseRounds += match.DefenseRounds[playerName]

				// Multi-kills
				if mk, ok := match.MultiKills[playerName]; ok {
					for count, occurrences := range mk {
						stats.MultiKills[count] += occurrences
					}
				}

				// Clutches
				if clutch, ok := match.Clutches[playerName]; ok {
					stats.Clutches += clutch
				}

				break
			}
		}
	}

	return stats
}

// ProcessMatchDetails procesa los detalles completos de una partida desde la API
func (as *AnalyticsService) ProcessMatchDetails(apiMatch interface{}, minStackPlayers int) *models.MatchData {
	// Type assert con seguridad
	fullMatch, ok := apiMatch.(*models.V4MatchResponse)
	if !ok {
		return nil
	}

	// Verificar si la partida tiene suficientes jugadores del stack
	stackPlayers := as.GetStackPlayers(fullMatch.Data.Players)
	if len(stackPlayers) < minStackPlayers {
		return nil // No incluir esta partida
	}

	// Construir datos de la partida
	match := &models.MatchData{
		MatchID:      fullMatch.Data.ID,
		Map:          fullMatch.Data.Metadata.Map.Name,
		Mode:         fullMatch.Data.Metadata.Queue.ID,
		PlayerData:   make(map[string]models.PlayerMatchStats),
		FirstKills:   make(map[string]int),
		FirstDeaths:  make(map[string]int),
		KASTRounds:   make(map[string]int),
		PlayerTeams:  make(map[string]string),
		AttackKills:   make(map[string]int),
		AttackDeaths:  make(map[string]int),
		AttackDamage:  make(map[string]int),
		AttackRounds:  make(map[string]int),
		DefenseKills:  make(map[string]int),
		DefenseDeaths: make(map[string]int),
		DefenseDamage: make(map[string]int),
		DefenseRounds: make(map[string]int),
		MultiKills:    make(map[string]map[int]int),
		Clutches:      make(map[string]int),
		RoundsPlayed:  as.CalculateRoundsPlayed(fullMatch.Data.Rounds),
		Timestamp:     fullMatch.Data.Metadata.GameStart,
	}

	// Procesar stats de jugadores
	for _, player := range fullMatch.Data.Players {
		playerName := as.GetPlayerName(player.Name, player.Tag)
		if playerName == "" {
			continue // No es un jugador del stack
		}

		match.PlayerData[playerName] = models.PlayerMatchStats{
			Kills:          player.Stats.Kills,
			Deaths:         player.Stats.Deaths,
			Assists:        player.Stats.Assists,
			Headshots:      player.Stats.Headshots,
			Bodyshots:      player.Stats.Bodyshots,
			Legshots:       player.Stats.Legshots,
			Score:          player.Stats.Score,
			Agent:          player.Agent.Name,
			Team:           player.TeamID,
			DamageMade:     player.Stats.Damage.Dealt,
			DamageReceived: player.Stats.Damage.Received,
		}
		match.PlayerTeams[playerName] = player.TeamID
	}

	// Verificar si el equipo ganó
	for _, team := range fullMatch.Data.Teams {
		hasStackPlayer := false
		for _, player := range fullMatch.Data.Players {
			if player.TeamID == team.TeamID {
				playerName := as.GetPlayerName(player.Name, player.Tag)
				if playerName != "" {
					hasStackPlayer = true
					break
				}
			}
		}
		if hasStackPlayer {
			match.Won = team.Won
			break
		}
	}

	// Construir mapeo de equipos por PUUID
	teamMembers := as.BuildTeamMembers(fullMatch.Data.Players)

	// Procesar eventos de muerte y calcular stats avanzados
	stackNamesByPUUID := as.BuildStackNamesByPUUID(fullMatch.Data.Players)
	eventsByRound := as.BuildKillEvents(fullMatch.Data.Kills, stackNamesByPUUID)

	// Calcular trades
	trades := as.ComputeTrades(eventsByRound)

	// Calcular multi-kills
	multiKills := as.ComputeMultiKills(eventsByRound)
	for name, mkData := range multiKills {
		match.MultiKills[name] = mkData
	}

	// Calcular clutches
	clutches := as.ComputeClutches(eventsByRound, teamMembers)
	for name, count := range clutches {
		match.Clutches[name] = count
	}

	// Calcular First Kills/Deaths y KAST
	as.CalculateFirstKillsAndDeaths(eventsByRound, match)
	as.CalculateKAST(eventsByRound, trades, match)

	// Calcular stats por lado (ataque/defensa)
	as.CalculateSideStats(fullMatch.Data.Rounds, fullMatch.Data.Kills, match, stackNamesByPUUID)

	return match
}

// GetStackPlayers retorna los jugadores del stack presentes en la partida
func (as *AnalyticsService) GetStackPlayers(players []models.V4MatchPlayer) map[string]models.PlayerMatchStats {
	stackPlayers := make(map[string]models.PlayerMatchStats)

	for _, player := range players {
		playerName := as.GetPlayerName(player.Name, player.Tag)
		if playerName != "" {
			stackPlayers[playerName] = models.PlayerMatchStats{
				Kills:          player.Stats.Kills,
				Deaths:         player.Stats.Deaths,
				Assists:        player.Stats.Assists,
				Headshots:      player.Stats.Headshots,
				Bodyshots:      player.Stats.Bodyshots,
				Legshots:       player.Stats.Legshots,
				Score:          player.Stats.Score,
				Agent:          player.Agent.Name,
				Team:           player.TeamID,
				DamageMade:     player.Stats.Damage.Dealt,
				DamageReceived: player.Stats.Damage.Received,
			}
		}
	}

	return stackPlayers
}

// GetPlayerName mapea una cuenta de Valorant a un nombre real
func (as *AnalyticsService) GetPlayerName(name, tag string) string {
	key := fmt.Sprintf("%s#%s", name, tag)
	if player, ok := as.playerAccountsMap[key]; ok {
		return player
	}
	return ""
}

// BuildTeamMembers construye un mapeo de equipos con sus miembros (PUUIDs)
func (as *AnalyticsService) BuildTeamMembers(players []models.V4MatchPlayer) map[string]map[string]struct{} {
	teams := make(map[string]map[string]struct{})
	for _, p := range players {
		if _, ok := teams[p.TeamID]; !ok {
			teams[p.TeamID] = make(map[string]struct{})
		}
		teams[p.TeamID][p.PUUID] = struct{}{}
	}
	return teams
}

// BuildStackNamesByPUUID crea un mapeo de PUUID a nombres de jugadores del stack
func (as *AnalyticsService) BuildStackNamesByPUUID(players []models.V4MatchPlayer) map[string]string {
	mapping := make(map[string]string)
	for _, player := range players {
		playerName := as.GetPlayerName(player.Name, player.Tag)
		if playerName != "" {
			mapping[player.PUUID] = playerName
		}
	}
	return mapping
}

// BuildKillEvents construye eventos de muerte agrupados por ronda
func (as *AnalyticsService) BuildKillEvents(kills []models.V4KillEventResponse, stackNamesByPUUID map[string]string) map[int][]models.KillEvent {
	eventsByRound := make(map[int][]models.KillEvent)

	for _, kill := range kills {
		ke := models.KillEvent{
			Round:       kill.Round,
			Time:        kill.TimeInRoundInMs,
			KillerName:  stackNamesByPUUID[kill.Killer.PUUID],
			VictimName:  stackNamesByPUUID[kill.Victim.PUUID],
			KillerTeam:  kill.Killer.Team,
			VictimTeam:  kill.Victim.Team,
			KillerPUUID: kill.Killer.PUUID,
			VictimPUUID: kill.Victim.PUUID,
		}

		for _, assistant := range kill.Assistants {
			if assistantName := stackNamesByPUUID[assistant.PUUID]; assistantName != "" {
				ke.Assistants = append(ke.Assistants, assistantName)
			}
		}

		eventsByRound[kill.Round] = append(eventsByRound[kill.Round], ke)
	}

	// Ordenar eventos por timestamp dentro de cada ronda
	for round, events := range eventsByRound {
		sort.Slice(events, func(i, j int) bool {
			return events[i].Time < events[j].Time
		})
		eventsByRound[round] = events
	}

	return eventsByRound
}

// ComputeTrades detecta trades (muertes dentro de una ventana de tiempo)
func (as *AnalyticsService) ComputeTrades(eventsByRound map[int][]models.KillEvent) map[int]map[string]bool {
	trades := make(map[int]map[string]bool)

	for round, events := range eventsByRound {
		for i, ev := range events {
			if ev.VictimName == "" {
				continue
			}

			window := ev.Time + as.tradeWindowMs
			for j := i + 1; j < len(events) && events[j].Time <= window; j++ {
				next := events[j]
				if next.VictimPUUID == ev.KillerPUUID && next.KillerTeam == ev.VictimTeam {
					if _, ok := trades[round]; !ok {
						trades[round] = make(map[string]bool)
					}
					trades[round][ev.VictimName] = true
					break
				}
			}
		}
	}

	return trades
}

// ComputeMultiKills cuenta multi-kills (2K, 3K, 4K, 5K)
func (as *AnalyticsService) ComputeMultiKills(eventsByRound map[int][]models.KillEvent) map[string]map[int]int {
	multi := make(map[string]map[int]int)

	for _, events := range eventsByRound {
		killsThisRound := make(map[string]int)
		for _, ev := range events {
			if ev.KillerName == "" {
				continue
			}
			killsThisRound[ev.KillerName]++
		}

		for name, count := range killsThisRound {
			if count < 2 {
				continue
			}
			if _, ok := multi[name]; !ok {
				multi[name] = make(map[int]int)
			}
			multi[name][count]++
		}
	}

	return multi
}

// ComputeClutches detecta clutches (el último jugador del equipo obtiene la ronda)
func (as *AnalyticsService) ComputeClutches(eventsByRound map[int][]models.KillEvent, teamMembers map[string]map[string]struct{}) map[string]int {
	clutches := make(map[string]int)

	for _, events := range eventsByRound {
		alive := as.CloneTeamMembers(teamMembers)
		for _, ev := range events {
			teamAliveBefore := len(alive[ev.KillerTeam])
			enemyAliveBefore := len(alive[ev.VictimTeam])

			delete(alive[ev.VictimTeam], ev.VictimPUUID)

			if ev.KillerName == "" {
				continue
			}

			if enemyAliveBefore > 0 && len(alive[ev.VictimTeam]) == 0 && teamAliveBefore == 1 {
				clutches[ev.KillerName]++
			}
		}
	}

	return clutches
}

// CloneTeamMembers crea una copia profunda del mapeo de equipos
func (as *AnalyticsService) CloneTeamMembers(src map[string]map[string]struct{}) map[string]map[string]struct{} {
	clone := make(map[string]map[string]struct{}, len(src))
	for team, members := range src {
		clone[team] = make(map[string]struct{}, len(members))
		for puuid := range members {
			clone[team][puuid] = struct{}{}
		}
	}
	return clone
}

// CalculateFirstKillsAndDeaths calcula First Kills y First Deaths
func (as *AnalyticsService) CalculateFirstKillsAndDeaths(eventsByRound map[int][]models.KillEvent, match *models.MatchData) {
	for _, events := range eventsByRound {
		if len(events) == 0 {
			continue
		}

		// First kill es el primer evento
		firstKill := events[0]
		if firstKill.KillerName != "" {
			match.FirstKills[firstKill.KillerName]++
		}
		if firstKill.VictimName != "" {
			match.FirstDeaths[firstKill.VictimName]++
		}
	}
}

// CalculateKAST calcula KAST (Kill, Assist, Survive, Trade)
func (as *AnalyticsService) CalculateKAST(eventsByRound map[int][]models.KillEvent, trades map[int]map[string]bool, match *models.MatchData) {
	for round, events := range eventsByRound {
		survivedPlayers := make(map[string]bool)
		for playerName := range match.PlayerData {
			survivedPlayers[playerName] = true
		}

		for _, ev := range events {
			if ev.KillerName != "" {
				survivedPlayers[ev.KillerName] = true
			}
			if ev.VictimName != "" {
				survivedPlayers[ev.VictimName] = false
			}
			for _, assistant := range ev.Assistants {
				if assistant != "" {
					survivedPlayers[assistant] = true
				}
			}
		}

		for playerName := range match.PlayerData {
			hasKill := false
			hasAssist := false
			hasTrade := false

			for _, ev := range events {
				if ev.KillerName == playerName {
					hasKill = true
				}
				for _, assistant := range ev.Assistants {
					if assistant == playerName {
						hasAssist = true
					}
				}
			}

			if trades[round] != nil && trades[round][playerName] {
				hasTrade = true
			}

			if hasKill || hasAssist || survivedPlayers[playerName] || hasTrade {
				match.KASTRounds[playerName]++
			}
		}
	}
}

// CalculateSideStats calcula estadísticas de ataque y defensa
func (as *AnalyticsService) CalculateSideStats(rounds []models.V4Round, kills []models.V4KillEventResponse, match *models.MatchData, stackNamesByPUUID map[string]string) {
	// Determinar el equipo que atacaba primero
	firstAttackTeam := as.InferInitialAttackingTeam(rounds)
	if firstAttackTeam == "" {
		return
	}

	secondTeam := as.PickSecondTeam(match.PlayerTeams)
	if secondTeam == "" {
		return
	}

	attackingByRound := as.BuildAttackingTeamByRound(rounds, firstAttackTeam, secondTeam)

	// Procesar cada muerte
	for _, kill := range kills {
		killerName := stackNamesByPUUID[kill.Killer.PUUID]
		victimName := stackNamesByPUUID[kill.Victim.PUUID]

		attackingTeam := attackingByRound[kill.Round]
		if killerName == "" || victimName == "" {
			continue
		}

		if kill.Killer.Team == attackingTeam {
			// Killer estaba atacando
			match.AttackKills[killerName]++
			match.DefenseDeaths[victimName]++
		} else {
			// Killer estaba defendiendo
			match.DefenseKills[killerName]++
			match.AttackDeaths[victimName]++
		}
	}

	// Procesar damage por ronda
	for _, round := range rounds {
		attackingTeam := attackingByRound[round.ID]

		for _, stat := range round.Stats {
			playerName := stackNamesByPUUID[stat.Player.PUUID]
			if playerName == "" {
				continue
			}

			if stat.Player.Team == attackingTeam {
				match.AttackDamage[playerName] += stat.Stats.Damage
				match.AttackRounds[playerName]++
			} else {
				match.DefenseDamage[playerName] += stat.Stats.Damage
				match.DefenseRounds[playerName]++
			}
		}
	}
}

// InferInitialAttackingTeam detecta qué equipo atacaba primero
func (as *AnalyticsService) InferInitialAttackingTeam(rounds []models.V4Round) string {
	for _, round := range rounds {
		if round.Plant != nil && round.Plant.Player.Team != "" {
			return round.Plant.Player.Team
		}
	}
	return ""
}

// PickSecondTeam selecciona el equipo que no es el primero
func (as *AnalyticsService) PickSecondTeam(playerTeams map[string]string) string {
	seenTeams := make(map[string]bool)
	var firstTeam string
	for _, team := range playerTeams {
		if firstTeam == "" {
			firstTeam = team
		}
		seenTeams[team] = true
	}

	for team := range seenTeams {
		if team != firstTeam {
			return team
		}
	}
	return ""
}

// BuildAttackingTeamByRound mapea qué equipo atacaba en cada ronda
func (as *AnalyticsService) BuildAttackingTeamByRound(rounds []models.V4Round, firstAttack, secondTeam string) map[int]string {
	attackingByRound := make(map[int]string, len(rounds))

	for idx, round := range rounds {
		attackingTeam := ""
		if round.Plant != nil && round.Plant.Player.Team != "" {
			attackingTeam = round.Plant.Player.Team
		} else {
			attackingTeam = as.SideByRoundIndex(idx, firstAttack, secondTeam)
		}
		attackingByRound[round.ID] = attackingTeam
	}

	return attackingByRound
}

// SideByRoundIndex determina el equipo atacante basado en el índice de ronda
func (as *AnalyticsService) SideByRoundIndex(idx int, firstAttack, secondTeam string) string {
	switch {
	case firstAttack == "" || secondTeam == "":
		return ""
	case idx < 12:
		return firstAttack
	case idx < 24:
		return secondTeam
	default:
		if (idx-24)%2 == 0 {
			return firstAttack
		}
		return secondTeam
	}
}

// CalculateRoundsPlayed calcula el número de rondas jugadas
func (as *AnalyticsService) CalculateRoundsPlayed(rounds []models.V4Round) int {
	return len(rounds)
}

// Alias para compatibilidad con código existente
type V4MatchResponse = models.V4MatchResponse
type V4MatchPlayer = models.V4MatchPlayer
type V4Round = models.V4Round
type V4KillEventResponse = models.V4KillEventResponse
