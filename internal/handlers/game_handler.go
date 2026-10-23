package handlers

import (
	"errors"
	"log"
	appauth "memory-match/internal/auth"
	"memory-match/internal/game"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type GameHandler struct {
	service gameService
}

type gameService interface {
	CreateGame(playerID int64, difficulty string) (*game.Game, error)
	GetGame(playerID, gameID int64) (*game.Game, error)
	FlipCard(playerID, gameID int64, position int) (*game.FlipResult, error)
	ResolveMismatch(playerID, gameID int64) error
	GetLeaderboard(difficulty string, limit int) ([]game.LeaderboardEntry, error)
}

func NewGameHandler(service gameService) *GameHandler {
	return &GameHandler{service: service}
}

type createGameRequest struct {
	Difficulty string `json:"difficulty"`
}

type cardResponse struct {
	Position int    `json:"position"`
	Value    string `json:"value,omitempty"`
	Matched  bool   `json:"matched"`
	Revealed bool   `json:"revealed"`
}

type gameResponse struct {
	ID               string         `json:"id"`
	Difficulty       string         `json:"difficulty"`
	Rows             int            `json:"rows"`
	Columns          int            `json:"columns"`
	Moves            int            `json:"moves"`
	PairsFound       int            `json:"pairs_found"`
	TotalPairs       int            `json:"total_pairs"`
	Status           string         `json:"status"`
	TimeLimitSeconds int            `json:"time_limit_seconds"`
	TimeLeftSeconds  int            `json:"time_left_seconds"`
	ElapsedSeconds   int            `json:"elapsed_seconds"`
	Cards            []cardResponse `json:"cards"`
}

func (h *GameHandler) CreateGame(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if player.Username == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "choose a username before playing"})
		return
	}
	var req createGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide valid JSON with difficulty"})
		return
	}

	layout, err := game.GetLayout(req.Difficulty)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	created, err := h.service.CreateGame(player.ID, layout.Difficulty)
	if err != nil {
		if errors.Is(err, game.ErrDifficultyLocked) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		log.Printf("creating game failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not create game",
		})
		return
	}
	response := gameResponse{
		ID:               created.ID,
		Difficulty:       created.Difficulty,
		Rows:             created.Rows,
		Columns:          created.Columns,
		Moves:            created.Moves,
		PairsFound:       created.PairsFound,
		TotalPairs:       created.TotalPairs,
		Status:           created.Status,
		TimeLimitSeconds: created.TimeLimitSeconds,
		TimeLeftSeconds:  created.TimeLeftSeconds,
		ElapsedSeconds:   created.ElapsedSeconds,
		Cards:            make([]cardResponse, 0, len(created.Cards)),
	}
	for _, card := range created.Cards {
		response.Cards = append(response.Cards, cardResponse{
			Position: card.Position,
			Matched:  card.Matched,
			Revealed: card.Revealed,
		})
	}

	c.JSON(http.StatusCreated, response)
}

func (h *GameHandler) GetGame(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || gameID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "game ID must be a positive integer",
		})
		return
	}

	created, err := h.service.GetGame(player.ID, gameID)
	if err != nil {
		if errors.Is(err, game.ErrGameNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "game not found",
			})
			return
		}

		log.Printf("getting game failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve game",
		})
		return
	}

	response := gameResponse{
		ID:               created.ID,
		Difficulty:       created.Difficulty,
		Rows:             created.Rows,
		Columns:          created.Columns,
		Moves:            created.Moves,
		PairsFound:       created.PairsFound,
		TotalPairs:       created.TotalPairs,
		Status:           created.Status,
		TimeLimitSeconds: created.TimeLimitSeconds,
		TimeLeftSeconds:  created.TimeLeftSeconds,
		ElapsedSeconds:   created.ElapsedSeconds,
		Cards:            make([]cardResponse, 0, len(created.Cards)),
	}

	for _, card := range created.Cards {
		item := cardResponse{
			Position: card.Position,
			Matched:  card.Matched,
			Revealed: card.Revealed,
		}

		if card.Revealed || card.Matched {
			item.Value = card.Value
		}

		response.Cards = append(response.Cards, item)
	}

	c.JSON(http.StatusOK, response)
}

type flipCardRequest struct {
	Position *int `json:"position"`
}

func (h *GameHandler) FlipCard(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || gameID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "game ID must be a positive integer",
		})
		return
	}

	var req flipCardRequest
	if err := c.ShouldBindJSON(&req); err != nil ||
		req.Position == nil ||
		*req.Position < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "provide a valid non-negative card position",
		})
		return
	}

	result, err := h.service.FlipCard(player.ID, gameID, *req.Position)
	if err != nil {
		switch {
		case errors.Is(err, game.ErrGameNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})

		case errors.Is(err, game.ErrCardNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})

		case errors.Is(err, game.ErrGameExpired):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "status": "expired"})

		case errors.Is(err, game.ErrGameCompleted),
			errors.Is(err, game.ErrCardUnavailable),
			errors.Is(err, game.ErrMismatchPending):
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})

		default:
			log.Printf("flipping card failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not flip card",
			})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *GameHandler) ResolveMismatch(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || gameID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "game ID must be a positive integer",
		})
		return
	}

	err = h.service.ResolveMismatch(player.ID, gameID)
	if err != nil {
		switch {
		case errors.Is(err, game.ErrGameNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "game not found",
			})

		case errors.Is(err, game.ErrGameExpired):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "status": "expired"})

		case errors.Is(err, game.ErrNoMismatchToResolve),
			errors.Is(err, game.ErrGameCompleted):
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})

		default:
			log.Printf("resolving mismatch failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not resolve mismatch",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"game_id":        strconv.FormatInt(gameID, 10),
		"result":         "mismatch_resolved",
		"cards_revealed": false,
	})
}

func (h *GameHandler) GetLeaderboard(c *gin.Context) {
	difficulty := strings.ToLower(strings.TrimSpace(c.DefaultQuery("difficulty", "hard")))
	if _, err := game.GetLayout(difficulty); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
		limit = parsed
	}

	entries, err := h.service.GetLeaderboard(difficulty, limit)
	if err != nil {
		log.Printf("getting leaderboard failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve leaderboard"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"leaderboard": entries})
}
