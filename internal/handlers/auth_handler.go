package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"

	appauth "memory-match/internal/auth"

	"github.com/gin-gonic/gin"
)

type authService interface {
	Enter(context.Context, string, string) (appauth.Player, string, error)
	CurrentPlayer(context.Context, string) (appauth.Player, error)
	Logout(context.Context, string) error
}

type AuthHandler struct {
	service       authService
	secureCookies bool
}

func NewAuthHandler(service authService, secureCookies bool) *AuthHandler {
	return &AuthHandler{service: service, secureCookies: secureCookies}
}

func (h *AuthHandler) RequirePlayer() gin.HandlerFunc {
	return appauth.RequirePlayer(h.service)
}

func (h *AuthHandler) Enter(c *gin.Context) {
	var request struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide an email and a username"})
		return
	}
	player, token, err := h.service.Enter(c.Request.Context(), request.Email, request.Username)
	if err != nil {
		switch {
		case errors.Is(err, appauth.ErrInvalidEmail), errors.Is(err, appauth.ErrInvalidUsername):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, appauth.ErrUsernameTaken), errors.Is(err, appauth.ErrEmailInUse):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			log.Printf("entry failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start your session — please try again"})
		}
		return
	}
	h.setCookie(c, appauth.SessionCookieName, token, int(appauth.SessionDuration.Seconds()))
	c.JSON(http.StatusOK, player)
}

func (h *AuthHandler) Me(c *gin.Context) {
	player, ok := appauth.PlayerFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.JSON(http.StatusOK, player)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(appauth.SessionCookieName)
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign out"})
		return
	}
	h.setCookie(c, appauth.SessionCookieName, "", -1)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) setCookie(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true,
		Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}
