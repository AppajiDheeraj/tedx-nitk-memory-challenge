package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appauth "memory-match/internal/auth"

	"github.com/gin-gonic/gin"
)

type fakeAuthService struct {
	player appauth.Player
	err    error
}

func TestEnterSetsHttpOnlySessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&fakeAuthService{player: appauth.Player{ID: 3, Username: "signal_7"}}, false)
	router := gin.New()
	router.POST("/auth/enter", handler.Enter)
	req := httptest.NewRequest(http.MethodPost, "/auth/enter", strings.NewReader(`{"email":"a@b.com","username":"signal_7"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "a@b.com") {
		t.Fatalf("response leaked email: %s", response.Body.String())
	}
	foundSession := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == appauth.SessionCookieName && cookie.Value == "token" && cookie.HttpOnly {
			foundSession = true
		}
	}
	if !foundSession {
		t.Fatal("enter did not set an HttpOnly session cookie")
	}
}

func TestEnterMapsConflictTo409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&fakeAuthService{err: appauth.ErrUsernameTaken}, false)
	router := gin.New()
	router.POST("/auth/enter", handler.Enter)
	req := httptest.NewRequest(http.MethodPost, "/auth/enter", strings.NewReader(`{"email":"a@b.com","username":"taken"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
}

func (f *fakeAuthService) Enter(context.Context, string, string) (appauth.Player, string, error) {
	if f.err != nil {
		return appauth.Player{}, "", f.err
	}
	return f.player, "token", nil
}
func (f *fakeAuthService) CurrentPlayer(context.Context, string) (appauth.Player, error) {
	return f.player, nil
}
func (f *fakeAuthService) Logout(context.Context, string) error { return nil }

func TestAuthMeResponseDoesNotExposePrivateIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&fakeAuthService{player: appauth.Player{
		ID: 7, Username: "signal_7",
		Progress: appauth.Progress{Easy: true, Medium: true},
	}}, false)
	router := gin.New()
	router.GET("/me", handler.RequirePlayer(), handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: appauth.SessionCookieName, Value: "valid"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "person@example.com") || strings.Contains(response.Body.String(), "google_subject") || strings.Contains(response.Body.String(), `"email"`) {
		t.Fatalf("response leaked private identity: %s", response.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["username"] != "signal_7" {
		t.Fatalf("username = %v", got["username"])
	}
}

func TestAuthMiddlewareRejectsMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&fakeAuthService{}, false)
	router := gin.New()
	router.GET("/me", handler.RequirePlayer(), handler.Me)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
