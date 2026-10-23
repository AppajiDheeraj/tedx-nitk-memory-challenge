package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appauth "memory-match/internal/auth"

	"github.com/gin-gonic/gin"
)

type fakeAuthService struct {
	player appauth.Player
}

func TestAuthCallbackKeepsNewSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&fakeAuthService{}, []byte("01234567890123456789012345678901"), false)
	loginValue, err := handler.signLoginCookie("expected-state", "pkce-verifier", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/auth/google/callback", handler.GoogleCallback)
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=expected-state&code=valid-code", nil)
	req.AddCookie(&http.Cookie{Name: loginCookieName, Value: loginValue})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", response.Code)
	}
	if response.Header().Get("Clear-Site-Data") != "" {
		t.Fatalf("callback clears the newly created session: %q", response.Header().Get("Clear-Site-Data"))
	}
	foundSession := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == appauth.SessionCookieName && cookie.Value == "token" && cookie.HttpOnly {
			foundSession = true
		}
	}
	if !foundSession {
		t.Fatal("callback did not set an HttpOnly session cookie")
	}
}

func (f *fakeAuthService) BeginLogin() (appauth.Login, error) { return appauth.Login{}, nil }
func (f *fakeAuthService) CompleteLogin(context.Context, string, string) (appauth.Player, string, error) {
	return f.player, "token", nil
}
func (f *fakeAuthService) CurrentPlayer(context.Context, string) (appauth.Player, error) {
	return f.player, nil
}
func (f *fakeAuthService) SetUsername(context.Context, int64, string) error { return nil }
func (f *fakeAuthService) Logout(context.Context, string) error             { return nil }

func TestAuthMeResponseDoesNotExposePrivateIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&fakeAuthService{player: appauth.Player{
		ID: 7, Username: "signal_7", PictureURL: "https://example.com/p.png",
		Progress: appauth.Progress{Easy: true, Medium: true},
	}}, []byte("01234567890123456789012345678901"), false)
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
	handler := NewAuthHandler(&fakeAuthService{}, []byte("01234567890123456789012345678901"), false)
	router := gin.New()
	router.GET("/me", handler.RequirePlayer(), handler.Me)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
