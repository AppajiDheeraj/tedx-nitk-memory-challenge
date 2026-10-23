package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appauth "memory-match/internal/auth"
	"memory-match/internal/game"

	"github.com/gin-gonic/gin"
)

type fakeGameService struct {
	createdPlayerID int64
	game            *game.Game
	err             error
	leaders         []game.LeaderboardEntry
}

func (f *fakeGameService) CreateGame(playerID int64, _ string) (*game.Game, error) {
	f.createdPlayerID = playerID
	return f.game, f.err
}
func (f *fakeGameService) GetGame(_, _ int64) (*game.Game, error) { return f.game, f.err }
func (f *fakeGameService) FlipCard(_, _ int64, _ int) (*game.FlipResult, error) {
	return nil, f.err
}
func (f *fakeGameService) ResolveMismatch(_, _ int64) error { return f.err }
func (f *fakeGameService) GetLeaderboard(_ string, _ int) ([]game.LeaderboardEntry, error) {
	return f.leaders, f.err
}

type fixedPlayerReader struct{ player appauth.Player }

func (f fixedPlayerReader) CurrentPlayer(context.Context, string) (appauth.Player, error) {
	return f.player, nil
}

func TestCreateGameUsesAuthenticatedPlayerNotRequestName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeGameService{game: &game.Game{ID: "1", Difficulty: "easy", Rows: 4, Columns: 4, TotalPairs: 8, Status: "in_progress"}}
	handler := NewGameHandler(service)
	router := gin.New()
	router.POST("/games", appauth.RequirePlayer(fixedPlayerReader{appauth.Player{ID: 44, Username: "signal_7"}}), handler.CreateGame)
	req := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(`{"player_name":"spoofed","difficulty":"easy"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: appauth.SessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if service.createdPlayerID != 44 {
		t.Fatalf("CreateGame player = %d, want 44", service.createdPlayerID)
	}
}

func TestGetGameOmitsHiddenCardValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeGameService{game: &game.Game{ID: "1", Difficulty: "easy", Rows: 4, Columns: 4, TotalPairs: 8, Status: "in_progress", Cards: []game.Card{{Position: 0, Value: "secret"}}}}
	handler := NewGameHandler(service)
	router := gin.New()
	router.GET("/games/:id", appauth.RequirePlayer(fixedPlayerReader{appauth.Player{ID: 44, Username: "signal_7"}}), handler.GetGame)
	req := httptest.NewRequest(http.MethodGet, "/games/1", nil)
	req.AddCookie(&http.Cookie{Name: appauth.SessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("hidden card leaked: %s", response.Body.String())
	}
}

func TestLeaderboardResponseContainsOnlyPublicFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeGameService{leaders: []game.LeaderboardEntry{{Rank: 1, Username: "signal_7", Difficulty: "hard", Moves: 22, AttemptCount: 3}}}
	handler := NewGameHandler(service)
	router := gin.New()
	router.GET("/leaderboard", handler.GetLeaderboard)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/leaderboard?difficulty=hard", nil))
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	encoded := response.Body.String()
	if !strings.Contains(encoded, "signal_7") || strings.Contains(encoded, "email") || strings.Contains(encoded, "google") {
		t.Fatalf("leaderboard response = %s", encoded)
	}
}
