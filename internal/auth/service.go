package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"golang.org/x/oauth2"
)

const SessionCookieName = "tedx_session"

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrUnverifiedEmail = errors.New("a verified Google email is required")
	ErrInvalidUsername = errors.New("username must be 3-24 letters, numbers, or underscores")
	ErrUsernameTaken   = errors.New("username is already taken")
	usernamePattern    = regexp.MustCompile(`^[A-Za-z0-9_]{3,24}$`)
)

type Progress struct {
	Easy   bool `json:"easy"`
	Medium bool `json:"medium"`
	Hard   bool `json:"hard"`
}

type Player struct {
	ID         int64    `json:"id"`
	Username   string   `json:"username"`
	PictureURL string   `json:"picture_url,omitempty"`
	Progress   Progress `json:"progress"`
}

type Claims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture"`
}

type Verifier interface {
	Verify(context.Context, string) (Claims, error)
}

type GoogleVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewGoogleVerifier(ctx context.Context, clientID string) *GoogleVerifier {
	keys := oidc.NewRemoteKeySet(ctx, "https://www.googleapis.com/oauth2/v3/certs")
	return &GoogleVerifier{verifier: oidc.NewVerifier("https://accounts.google.com", keys, &oidc.Config{ClientID: clientID})}
}

func (v *GoogleVerifier) Verify(ctx context.Context, raw string) (Claims, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return Claims{}, fmt.Errorf("verifying Google ID token: %w", err)
	}
	var claims Claims
	if err := token.Claims(&claims); err != nil {
		return Claims{}, fmt.Errorf("reading Google claims: %w", err)
	}
	return claims, nil
}

type Login struct {
	URL      string
	State    string
	Verifier string
}

type Service struct {
	db       *sql.DB
	oauth    oauth2.Config
	verifier Verifier
	now      func() time.Time
}

func NewService(db *sql.DB, config oauth2.Config, verifier Verifier, baseURL string, _ bool) *Service {
	config.RedirectURL = strings.TrimRight(baseURL, "/") + "/auth/google/callback"
	return &Service{db: db, oauth: config, verifier: verifier, now: time.Now}
}

func ValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func (s *Service) BeginLogin() (Login, error) {
	state, err := randomToken(24)
	if err != nil {
		return Login{}, err
	}
	verifier := oauth2.GenerateVerifier()
	return Login{
		URL:      s.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)),
		State:    state,
		Verifier: verifier,
	}, nil
}

func (s *Service) CompleteLogin(ctx context.Context, code, pkceVerifier string) (Player, string, error) {
	token, err := s.oauth.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return Player{}, "", fmt.Errorf("exchanging Google authorization code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Player{}, "", errors.New("Google response did not include an ID token")
	}
	claims, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Player{}, "", err
	}
	player, err := s.upsertGooglePlayer(ctx, claims)
	if err != nil {
		return Player{}, "", err
	}
	sessionToken, err := randomToken(32)
	if err != nil {
		return Player{}, "", err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, player_id, expires_at) VALUES ($1, $2, $3)`,
		hashToken(sessionToken), player.ID, s.now().Add(7*24*time.Hour),
	); err != nil {
		return Player{}, "", fmt.Errorf("creating session: %w", err)
	}
	return player, sessionToken, nil
}

func (s *Service) upsertGooglePlayer(ctx context.Context, claims Claims) (Player, error) {
	if claims.Subject == "" || claims.Email == "" || !claims.EmailVerified {
		return Player{}, ErrUnverifiedEmail
	}
	var player Player
	var username, picture sql.NullString
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO players (google_subject, email, email_verified, picture_url)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (google_subject) WHERE google_subject IS NOT NULL DO UPDATE SET
		   email = EXCLUDED.email,
		   email_verified = EXCLUDED.email_verified,
		   picture_url = EXCLUDED.picture_url,
		   updated_at = NOW()
		 RETURNING id, username, picture_url`,
		claims.Subject, claims.Email, claims.EmailVerified, claims.Picture,
	).Scan(&player.ID, &username, &picture)
	if err != nil {
		return Player{}, fmt.Errorf("saving Google player: %w", err)
	}
	player.Username = username.String
	player.PictureURL = picture.String
	player.Progress.Easy = true
	return player, nil
}

func (s *Service) CurrentPlayer(ctx context.Context, token string) (Player, error) {
	if token == "" {
		return Player{}, ErrUnauthenticated
	}
	var player Player
	var username, picture sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT p.id, p.username, p.picture_url,
		 EXISTS (SELECT 1 FROM games g WHERE g.player_id = p.id AND g.difficulty = 'easy' AND g.status = 'completed'),
		 EXISTS (SELECT 1 FROM games g WHERE g.player_id = p.id AND g.difficulty = 'medium' AND g.status = 'completed')
		 FROM sessions s JOIN players p ON p.id = s.player_id
		 WHERE s.token_hash = $1 AND s.expires_at > NOW()`,
		hashToken(token),
	).Scan(&player.ID, &username, &picture, &player.Progress.Medium, &player.Progress.Hard)
	if errors.Is(err, sql.ErrNoRows) {
		return Player{}, ErrUnauthenticated
	}
	if err != nil {
		return Player{}, fmt.Errorf("loading session: %w", err)
	}
	player.Username = username.String
	player.PictureURL = picture.String
	player.Progress.Easy = true
	return player, nil
}

func (s *Service) SetUsername(ctx context.Context, playerID int64, username string) error {
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}
	var saved string
	err := s.db.QueryRowContext(ctx,
		`UPDATE players SET username = $1, updated_at = NOW() WHERE id = $2 RETURNING username`,
		username, playerID,
	).Scan(&saved)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrUsernameTaken
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthenticated
	}
	if err != nil {
		return fmt.Errorf("updating username: %w", err)
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token)); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

type SessionReader interface {
	CurrentPlayer(context.Context, string) (Player, error)
}

const playerContextKey = "authenticated_player"

func RequirePlayer(service SessionReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "authentication required"})
			return
		}
		player, err := service.CurrentPlayer(c.Request.Context(), cookie)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "authentication required"})
			return
		}
		c.Set(playerContextKey, player)
		c.Next()
	}
}

func PlayerFromContext(c *gin.Context) (Player, bool) {
	value, ok := c.Get(playerContextKey)
	player, valid := value.(Player)
	return player, ok && valid
}

func randomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
