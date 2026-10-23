package game

import (
	"testing"
	"time"
)

func TestDifficultyLayouts(t *testing.T) {
	tests := []struct {
		name       string
		rows       int
		columns    int
		totalPairs int
	}{
		{name: "easy", rows: 4, columns: 4, totalPairs: 8},
		{name: "medium", rows: 4, columns: 5, totalPairs: 10},
		{name: "hard", rows: 5, columns: 6, totalPairs: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := GetLayout(tt.name)
			if err != nil {
				t.Fatalf("GetLayout returned error: %v", err)
			}
			if layout.Rows != tt.rows || layout.Columns != tt.columns {
				t.Fatalf("layout = %dx%d, want %dx%d", layout.Rows, layout.Columns, tt.rows, tt.columns)
			}
			cards, pairs, err := GenerateBoard(layout.Rows, layout.Columns)
			if err != nil {
				t.Fatalf("GenerateBoard returned error: %v", err)
			}
			if pairs != tt.totalPairs || len(cards) != tt.rows*tt.columns {
				t.Fatalf("got %d cards / %d pairs; want %d cards / %d pairs", len(cards), pairs, tt.rows*tt.columns, tt.totalPairs)
			}
		})
	}
}

func TestTimerValues(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(4*time.Second + 200*time.Millisecond)
	left, elapsed := timerValues(start, "in_progress", nil, now)
	if elapsed != 4 || left != 296 {
		t.Fatalf("timer after 4.2s = left %d, elapsed %d; want 296 and 4", left, elapsed)
	}

	left, elapsed = timerValues(start, "expired", nil, now)
	if elapsed != TimeLimitSeconds || left != 0 {
		t.Fatalf("expired timer = left %d, elapsed %d; want 0 and %d", left, elapsed, TimeLimitSeconds)
	}
}
