package handlers

import (
	"testing"
	"time"
)

func TestFixedWindowLimiterRejectsOnlyUntilWindowResets(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := newFixedWindowLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }
	if ok, _ := limiter.Allow("player-1"); !ok {
		t.Fatal("first request rejected")
	}
	if ok, _ := limiter.Allow("player-1"); !ok {
		t.Fatal("second request rejected")
	}
	if ok, retry := limiter.Allow("player-1"); ok || retry <= 0 {
		t.Fatalf("third request = (%v, %v), want rejected with retry", ok, retry)
	}
	if ok, _ := limiter.Allow("player-2"); !ok {
		t.Fatal("independent key rejected")
	}
	now = now.Add(time.Minute)
	if ok, _ := limiter.Allow("player-1"); !ok {
		t.Fatal("request rejected after reset")
	}
}
