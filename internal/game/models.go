package game

import "time"

type Card struct {
	Position int    `json:"position"`
	PairID   int    `json:"-"`
	Value    string `json:"value,omitempty"`
	Matched  bool   `json:"matched"`
	Revealed bool   `json:"revealed"`
}

type Game struct {
	ID               string     `json:"id"`
	Difficulty       string     `json:"difficulty"`
	Rows             int        `json:"rows"`
	Columns          int        `json:"columns"`
	Moves            int        `json:"moves"`
	PairsFound       int        `json:"pairs_found"`
	TotalPairs       int        `json:"total_pairs"`
	Status           string     `json:"status"`
	TimeLimitSeconds int        `json:"time_limit_seconds"`
	TimeLeftSeconds  int        `json:"time_left_seconds"`
	ElapsedSeconds   int        `json:"elapsed_seconds"`
	Cards            []Card     `json:"cards"`
	StartedAt        time.Time  `json:"-"`
	CompletedAt      *time.Time `json:"-"`
}

type LeaderboardEntry struct {
	Rank           int       `json:"rank"`
	GameID         string    `json:"game_id"`
	Username       string    `json:"username"`
	Difficulty     string    `json:"difficulty"`
	Moves          int       `json:"moves"`
	ElapsedSeconds int       `json:"elapsed_seconds"`
	CompletedAt    time.Time `json:"completed_at"`
	AttemptCount   int       `json:"attempt_count"`
}
