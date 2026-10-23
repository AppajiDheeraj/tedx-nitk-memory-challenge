package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appauth "memory-match/internal/auth"

	"github.com/gin-gonic/gin"
)

const loginCookieName = "tedx_login"

type authService interface {
	BeginLogin() (appauth.Login, error)
	CompleteLogin(context.Context, string, string) (appauth.Player, string, error)
	CurrentPlayer(context.Context, string) (appauth.Player, error)
	SetUsername(context.Context, int64, string) error
	Logout(context.Context, string) error
}

type AuthHandler struct {
	service       authService
	signingKey    []byte
	secureCookies bool
}

func NewAuthHandler(service authService, signingKey []byte, secureCookies bool) *AuthHandler {
	return &AuthHandler{service: service, signingKey: signingKey, secureCookies: secureCookies}
}

func (h *AuthHandler) RequirePlayer() gin.HandlerFunc {
	return appauth.RequirePlayer(h.service)
}

func (h *AuthHandler) BeginGoogle(c *gin.Context) {
	login, err := h.service.BeginLogin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start sign-in"})
		return
	}
	value, err := h.signLoginCookie(login.State, login.Verifier, time.Now().Add(10*time.Minute))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start sign-in"})
		return
	}
	h.setCookie(c, loginCookieName, value, 600)
	c.Redirect(http.StatusFound, login.URL)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	stored, err := c.Cookie(loginCookieName)
	if err != nil || state == "" || code == "" {
		c.Redirect(http.StatusFound, "/?auth=failed")
		return
	}
	login, err := h.readLoginCookie(stored)
	if err != nil || !hmac.Equal([]byte(login.State), []byte(state)) || time.Now().Unix() > login.Expires {
		c.Redirect(http.StatusFound, "/?auth=failed")
		return
	}
	_, token, err := h.service.CompleteLogin(c.Request.Context(), code, login.Verifier)
	if err != nil {
		c.Redirect(http.StatusFound, "/?auth=failed")
		return
	}
	h.setCookie(c, appauth.SessionCookieName, token, 7*24*60*60)
	h.clearCookie(c, loginCookieName)
	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) Me(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.JSON(http.StatusOK, player)
}

func (h *AuthHandler) UpdateUsername(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var request struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": appauth.ErrInvalidUsername.Error()})
		return
	}
	if err := h.service.SetUsername(c.Request.Context(), player.ID, request.Username); err != nil {
		switch {
		case errors.Is(err, appauth.ErrInvalidUsername):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, appauth.ErrUsernameTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save username"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": request.Username})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(appauth.SessionCookieName)
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign out"})
		return
	}
	h.clearCookie(c, appauth.SessionCookieName)
	c.Status(http.StatusNoContent)
}

type loginCookie struct {
	State    string `json:"state"`
	Verifier string `json:"verifier"`
	Expires  int64  `json:"expires"`
}

func (h *AuthHandler) signLoginCookie(state, verifier string, expires time.Time) (string, error) {
	payload, err := json.Marshal(loginCookie{State: state, Verifier: verifier, Expires: expires.Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, h.signingKey)
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (h *AuthHandler) readLoginCookie(value string) (loginCookie, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return loginCookie{}, errors.New("invalid login cookie")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return loginCookie{}, err
	}
	mac := hmac.New(sha256.New, h.signingKey)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return loginCookie{}, errors.New("invalid login cookie signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return loginCookie{}, err
	}
	var result loginCookie
	if err := json.Unmarshal(payload, &result); err != nil {
		return loginCookie{}, err
	}
	return result, nil
}

func (h *AuthHandler) setCookie(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true,
		Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearCookie(c *gin.Context, name string) {
	h.setCookie(c, name, "", -1)
}
