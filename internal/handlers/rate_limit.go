package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	appauth "memory-match/internal/auth"

	"github.com/gin-gonic/gin"
)

type windowCount struct {
	started time.Time
	count   int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]windowCount
	limit   int
	window  time.Duration
	now     func() time.Time
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{entries: make(map[string]windowCount), limit: limit, window: window, now: time.Now}
}

func (l *fixedWindowLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.started) >= l.window {
		l.entries[key] = windowCount{started: now, count: 1}
		l.cleanup(now)
		return true, 0
	}
	if entry.count >= l.limit {
		return false, entry.started.Add(l.window).Sub(now)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}

func (l *fixedWindowLimiter) cleanup(now time.Time) {
	if len(l.entries) < 1024 {
		return
	}
	for key, entry := range l.entries {
		if now.Sub(entry.started) >= l.window {
			delete(l.entries, key)
		}
	}
	if len(l.entries) >= 1024 {
		for key := range l.entries {
			delete(l.entries, key)
			break
		}
	}
}

func NewRateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	limiter := newFixedWindowLimiter(limit, window)
	return func(c *gin.Context) {
		key := c.ClientIP()
		if player, ok := appauth.PlayerFromContext(c); ok {
			key = fmt.Sprintf("player:%d", player.ID)
		}
		allowed, retry := limiter.Allow(key)
		if !allowed {
			seconds := int(retry.Round(time.Second) / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", seconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}
