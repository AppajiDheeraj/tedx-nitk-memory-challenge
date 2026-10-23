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
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

const (
	SessionCookieName = "tedx_session"
	SessionDuration   = 30 * 24 * time.Hour
)

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrInvalidEmail    = errors.New("enter a valid email address")
	ErrInvalidUsername = errors.New("username must be 3-24 letters, numbers, or underscores")
	ErrUsernameTaken   = errors.New("that username is already taken — pick another")
	ErrEmailInUse      = errors.New("this email already plays under a different username — use that username to continue")
	usernamePattern    = regexp.MustCompile(`^[A-Za-z0-9_]{3,24}$`)
)

type Progress struct {
	Easy   bool `json:"easy"`
	Medium bool `json:"medium"`
	Hard   bool `json:"hard"`
}

type Player struct {
	ID       int64    `json:"id"`
	Username string   `json:"username"`
	Progress Progress `json:"progress"`
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now}
}

func ValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

// NormalizeEmail returns the lower-cased address, or ErrInvalidEmail.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 320 {
		return "", ErrInvalidEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// Enter signs a player in by email and public username. A new email creates a
// player; a returning email must use the username it first registered with.
func (s *Service) Enter(ctx context.Context, rawEmail, username string) (Player, string, error) {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return Player{}, "", err
	}
	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return Player{}, "", ErrInvalidUsername
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Player{}, "", fmt.Errorf("beginning entry: %w", err)
	}
	defer tx.Rollback()

	var player Player
	var existing sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, username FROM players WHERE LOWER(email) = $1 FOR UPDATE`, email,
	).Scan(&player.ID, &existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		err = tx.QueryRowContext(ctx,
			`INSERT INTO players (username, email, email_verified) VALUES ($1, $2, FALSE) RETURNING id, username`,
			username, email,
		).Scan(&player.ID, &player.Username)
	case err != nil:
		return Player{}, "", fmt.Errorf("finding player: %w", err)
	case !existing.Valid:
		err = tx.QueryRowContext(ctx,
			`UPDATE players SET username = $1, updated_at = NOW() WHERE id = $2 RETURNING username`,
			username, player.ID,
		).Scan(&player.Username)
	case !strings.EqualFold(existing.String, username):
		return Player{}, "", ErrEmailInUse
	default:
		player.Username = existing.String
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		if strings.Contains(pqErr.Constraint, "email") {
			return Player{}, "", ErrEmailInUse
		}
		return Player{}, "", ErrUsernameTaken
	}
	if err != nil {
		return Player{}, "", fmt.Errorf("saving player: %w", err)
	}

	sessionToken, err := randomToken(32)
	if err != nil {
		return Player{}, "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, player_id, expires_at) VALUES ($1, $2, $3)`,
		hashToken(sessionToken), player.ID, s.now().Add(SessionDuration),
	); err != nil {
		return Player{}, "", fmt.Errorf("creating session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Player{}, "", fmt.Errorf("committing entry: %w", err)
	}
	player.Progress.Easy = true
	return player, sessionToken, nil
}

func (s *Service) CurrentPlayer(ctx context.Context, token string) (Player, error) {
	if token == "" {
		return Player{}, ErrUnauthenticated
	}
	var player Player
	var username sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT p.id, p.username,
		 EXISTS (SELECT 1 FROM games g WHERE g.player_id = p.id AND g.difficulty = 'easy' AND g.status = 'completed'),
		 EXISTS (SELECT 1 FROM games g WHERE g.player_id = p.id AND g.difficulty = 'medium' AND g.status = 'completed')
		 FROM sessions s JOIN players p ON p.id = s.player_id
		 WHERE s.token_hash = $1 AND s.expires_at > NOW()`,
		hashToken(token),
	).Scan(&player.ID, &username, &player.Progress.Medium, &player.Progress.Hard)
	if errors.Is(err, sql.ErrNoRows) {
		return Player{}, ErrUnauthenticated
	}
	if err != nil {
		return Player{}, fmt.Errorf("loading session: %w", err)
	}
	if !username.Valid {
		return Player{}, ErrUnauthenticated
	}
	player.Username = username.String
	player.Progress.Easy = true
	return player, nil
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
