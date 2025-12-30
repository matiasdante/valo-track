package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"valo-track/internal/models"
)

// APIClient gestiona las llamadas a la API de Valorant
type APIClient struct {
	baseURL        string
	apiKey         string
	region         string
	httpClient     *http.Client
	maxRetries     int
	retryDelay     time.Duration
}

// NewAPIClient crea un nuevo cliente de API
func NewAPIClient(apiKey, region string, timeout time.Duration, maxRetries int) *APIClient {
	return &APIClient{
		baseURL: "https://api.henrikdev.xyz/valorant",
		apiKey:  apiKey,
		region:  region,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
		retryDelay: 500 * time.Millisecond,
	}
}

// GetLifetimeMatches obtiene las partidas competitivas de un jugador (v3)
func (ac *APIClient) GetLifetimeMatches(name, tag string, queueMode string) ([]string, error) {
	url := fmt.Sprintf("%s/v3/by-puuid/account/%s/%s", ac.baseURL, ac.region, queueMode)

	// Primero obtener el PUUID del jugador
	puuid, err := ac.GetPlayerPUUID(name, tag)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo PUUID de %s#%s: %w", name, tag, err)
	}

	url = fmt.Sprintf("%s/v3/by-puuid/account/%s/%s", ac.baseURL, puuid, queueMode)

	body, err := ac.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status int      `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error decodificando response de v3: %w", err)
	}

	if response.Status != 200 {
		return nil, fmt.Errorf("API error: status code %d", response.Status)
	}

	return response.Data, nil
}

// GetPlayerPUUID obtiene el PUUID de un jugador
func (ac *APIClient) GetPlayerPUUID(name, tag string) (string, error) {
	url := fmt.Sprintf("%s/v1/account/%s/%s/%s", ac.baseURL, ac.region, name, tag)

	body, err := ac.makeRequest(url)
	if err != nil {
		return "", err
	}

	var response struct {
		Status int `json:"status"`
		Data   struct {
			PUUID string `json:"puuid"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error decodificando PUUID: %w", err)
	}

	if response.Status != 200 {
		return "", fmt.Errorf("API error al obtener PUUID: status code %d", response.Status)
	}

	return response.Data.PUUID, nil
}

// GetMatchDetailsV4 obtiene los detalles completos de una partida (v4)
func (ac *APIClient) GetMatchDetailsV4(matchID string) (*V4MatchResponse, error) {
	url := fmt.Sprintf("%s/v4/match/%s/%s", ac.baseURL, ac.region, matchID)

	body, err := ac.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var response V4MatchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error decodificando match details v4: %w", err)
	}

	if response.Status != 200 {
		return nil, fmt.Errorf("API error al obtener match details: status code %d", response.Status)
	}

	return &response, nil
}

// makeRequest realiza una request HTTP con reintentos automáticos
func (ac *APIClient) makeRequest(url string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= ac.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(ac.retryDelay * time.Duration(attempt))
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creando request: %w", err)
		}

		req.Header.Add("Authorization", ac.apiKey)

		resp, err := ac.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		// Leer el body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("error leyendo response body: %w", err)
			continue
		}

		// Manejo de status codes
		if resp.StatusCode == http.StatusTooManyRequests {
			// 429 - Rate limited, esperar antes de reintentar
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				retrySeconds := 60 // Default a 60 segundos
				fmt.Sscanf(retryAfter, "%d", &retrySeconds)
				time.Sleep(time.Duration(retrySeconds) * time.Second)
				lastErr = fmt.Errorf("rate limited, esperando %d segundos", retrySeconds)
				continue
			}
			lastErr = fmt.Errorf("rate limited (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			// No reintentar en algunos errores
			if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
				return nil, lastErr
			}
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("se agotaron los reintentos: %w", lastErr)
}

// Structs para respuestas de API v4

type V4MatchPlayer struct {
	PUUID  string `json:"puuid"`
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	TeamID string `json:"team_id"`
	Agent  struct {
		Name string `json:"name"`
	} `json:"agent"`
	Stats struct {
		Score     int `json:"score"`
		Kills     int `json:"kills"`
		Deaths    int `json:"deaths"`
		Assists   int `json:"assists"`
		Headshots int `json:"headshots"`
		Bodyshots int `json:"bodyshots"`
		Legshots  int `json:"legshots"`
		Damage    struct {
			Dealt    int `json:"dealt"`
			Received int `json:"received"`
		} `json:"damage"`
	} `json:"stats"`
}

type V4RoundStatsEntry struct {
	Player struct {
		PUUID string `json:"puuid"`
		Name  string `json:"name"`
		Tag   string `json:"tag"`
		Team  string `json:"team"`
	} `json:"player"`
	Stats struct {
		Damage int `json:"damage"`
		Kills  int `json:"kills"`
	} `json:"stats"`
}

type V4Round struct {
	ID          int    `json:"id"`
	WinningTeam string `json:"winning_team"`
	Plant       *struct {
		Player struct {
			Team string `json:"team"`
		} `json:"player"`
	} `json:"plant"`
	Stats []V4RoundStatsEntry `json:"stats"`
}

type V4KillEventResponse struct {
	Round           int `json:"round"`
	TimeInRoundInMs int `json:"time_in_round_in_ms"`
	Killer          struct {
		PUUID string `json:"puuid"`
		Name  string `json:"name"`
		Tag   string `json:"tag"`
		Team  string `json:"team"`
	} `json:"killer"`
	Victim struct {
		PUUID string `json:"puuid"`
		Name  string `json:"name"`
		Tag   string `json:"tag"`
		Team  string `json:"team"`
	} `json:"victim"`
	Assistants []struct {
		PUUID string `json:"puuid"`
		Name  string `json:"name"`
		Tag   string `json:"tag"`
		Team  string `json:"team"`
	} `json:"assistants"`
}

type V4MatchResponse struct {
	Status int `json:"status"`
	Data   struct {
		ID       string `json:"id"`
		Metadata struct {
			Map struct {
				Name string `json:"name"`
			} `json:"map"`
			Queue struct {
				ID string `json:"id"`
			} `json:"queue"`
			Region    string `json:"region"`
			GameStart int64  `json:"game_start"`
		} `json:"metadata"`
		Players []V4MatchPlayer `json:"players"`
		Teams   []struct {
			TeamID string `json:"team_id"`
			Rounds struct {
				Won  int `json:"won"`
				Lost int `json:"lost"`
			} `json:"rounds"`
			Won bool `json:"won"`
		} `json:"teams"`
		Rounds []V4Round              `json:"rounds"`
		Kills  []V4KillEventResponse `json:"kills"`
	} `json:"data"`
}
