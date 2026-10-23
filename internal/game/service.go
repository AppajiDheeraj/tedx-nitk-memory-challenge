package game

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
)

const TimeLimitSeconds = 300

var gameTimeLimit = time.Duration(TimeLimitSeconds) * time.Second

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateGame(playerID int64, difficulty string) (*Game, error) {
	if playerID <= 0 {
		return nil, ErrGameNotFound
	}

	layout, err := GetLayout(difficulty)
	if err != nil {
		return nil, err
	}
	cards, totalPairs, err := GenerateBoard(layout.Rows, layout.Columns)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, playerID); err != nil {
		return nil, fmt.Errorf("locking player game creation: %w", err)
	}
	prerequisite := map[string]string{"medium": "easy", "hard": "medium"}[layout.Difficulty]
	if prerequisite != "" {
		var unlocked bool
		if err := tx.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM games WHERE player_id = $1 AND difficulty = $2 AND status = 'completed')`,
			playerID, prerequisite,
		).Scan(&unlocked); err != nil {
			return nil, fmt.Errorf("checking difficulty unlock: %w", err)
		}
		if !unlocked {
			return nil, ErrDifficultyLocked
		}
	}
	if _, err := tx.Exec(
		`UPDATE games SET status = 'expired' WHERE player_id = $1 AND status = 'in_progress'`, playerID,
	); err != nil {
		return nil, fmt.Errorf("expiring previous game: %w", err)
	}

	var gameID int64
	var startedAt time.Time
	err = tx.QueryRow(
		`INSERT INTO games (player_id, difficulty, row_count, col_count, total_pairs)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, started_at`,
		playerID, layout.Difficulty, layout.Rows, layout.Columns, totalPairs,
	).Scan(&gameID, &startedAt)
	if err != nil {
		return nil, fmt.Errorf("creating game: %w", err)
	}

	positions := make([]int64, len(cards))
	pairIDs := make([]int64, len(cards))
	values := make([]string, len(cards))
	for i, card := range cards {
		positions[i], pairIDs[i], values[i] = int64(card.Position), int64(card.PairID), card.Value
	}
	if _, err := tx.Exec(
		`INSERT INTO game_cards (game_id, position, pair_id, value)
		 SELECT $1, u.position, u.pair_id, u.value
		 FROM unnest($2::int[], $3::int[], $4::text[]) AS u(position, pair_id, value)`,
		gameID, pq.Array(positions), pq.Array(pairIDs), pq.Array(values),
	); err != nil {
		return nil, fmt.Errorf("saving cards: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing game: %w", err)
	}

	g := &Game{
		ID: strconv.FormatInt(gameID, 10), Difficulty: layout.Difficulty,
		Rows: layout.Rows, Columns: layout.Columns, Moves: 0, PairsFound: 0,
		TotalPairs: totalPairs, Status: "in_progress", Cards: cards,
		TimeLimitSeconds: TimeLimitSeconds, TimeLeftSeconds: TimeLimitSeconds,
		ElapsedSeconds: 0, StartedAt: startedAt,
	}
	return g, nil
}

var (
	ErrGameNotFound     = errors.New("game not found")
	ErrCardNotFound     = errors.New("card position not found")
	ErrGameCompleted    = errors.New("game is already completed")
	ErrGameExpired      = errors.New("game time has expired")
	ErrCardUnavailable  = errors.New("card is already revealed or matched")
	ErrDifficultyLocked = errors.New("complete the previous difficulty first")
)

func (s *Service) expireGameIfNeeded(playerID, gameID int64) error {
	_, err := s.db.Exec(
		`UPDATE games SET status = 'expired'
		 WHERE id = $1 AND player_id = $2 AND status = 'in_progress'
		   AND started_at <= NOW() - INTERVAL '5 minutes'`,
		gameID, playerID,
	)
	if err != nil {
		return fmt.Errorf("checking game deadline: %w", err)
	}
	return nil
}

func timerValues(startedAt time.Time, status string, completedAt *time.Time, now time.Time) (left, elapsed int) {
	end := now
	if status == "expired" {
		end = startedAt.Add(gameTimeLimit)
	} else if status == "completed" && completedAt != nil {
		end = *completedAt
	}
	elapsed = int(end.Sub(startedAt).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > TimeLimitSeconds {
		elapsed = TimeLimitSeconds
	}
	left = TimeLimitSeconds - elapsed
	if left < 0 {
		left = 0
	}
	return left, elapsed
}

func (s *Service) GetGame(playerID, gameID int64) (*Game, error) {
	if err := s.expireGameIfNeeded(playerID, gameID); err != nil {
		return nil, err
	}

	var g Game
	var id int64
	var completedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, difficulty, row_count, col_count, moves, pairs_found,
		        total_pairs, status, started_at, completed_at
		 FROM games WHERE id = $1 AND player_id = $2`, gameID, playerID,
	).Scan(
		&id, &g.Difficulty, &g.Rows, &g.Columns, &g.Moves, &g.PairsFound,
		&g.TotalPairs, &g.Status, &g.StartedAt, &completedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting game: %w", err)
	}
	g.ID = strconv.FormatInt(id, 10)
	if completedAt.Valid {
		t := completedAt.Time
		g.CompletedAt = &t
	}
	g.TimeLimitSeconds = TimeLimitSeconds
	g.TimeLeftSeconds, g.ElapsedSeconds = timerValues(g.StartedAt, g.Status, g.CompletedAt, time.Now())

	rows, err := s.db.Query(
		`SELECT position, pair_id, value, is_matched, is_revealed
		 FROM game_cards WHERE game_id = $1 ORDER BY position`, gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting game cards: %w", err)
	}
	defer rows.Close()

	g.Cards = make([]Card, 0, g.Rows*g.Columns)
	for rows.Next() {
		var card Card
		if err := rows.Scan(&card.Position, &card.PairID, &card.Value, &card.Matched, &card.Revealed); err != nil {
			return nil, fmt.Errorf("reading game card: %w", err)
		}
		g.Cards = append(g.Cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating game cards: %w", err)
	}
	return &g, nil
}

type FlipResult struct {
	GameID           string `json:"game_id"`
	Position         int    `json:"position"`
	Value            string `json:"value"`
	RelatedPosition  *int   `json:"related_position,omitempty"`
	RelatedValue     string `json:"related_value,omitempty"`
	Result           string `json:"result"`
	Matched          bool   `json:"matched"`
	Revealed         bool   `json:"revealed"`
	Moves            int    `json:"moves"`
	PairsFound       int    `json:"pairs_found"`
	TotalPairs       int    `json:"total_pairs"`
	Status           string `json:"status"`
	TimeLimitSeconds int    `json:"time_limit_seconds"`
	TimeLeftSeconds  int    `json:"time_left_seconds"`
	ElapsedSeconds   int    `json:"elapsed_seconds"`
}

// FlipCard reveals one card. It reads the game and its cards in one query and
// writes every change in one statement, so a flip costs four database round
// trips. A mismatched pair still face up is hidden automatically, which lets
// the client flip back locally instead of calling ResolveMismatch.
func (s *Service) FlipCard(playerID, gameID int64, position int) (*FlipResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT g.status, g.moves, g.pairs_found, g.total_pairs, g.started_at, NOW(),
		        c.position, c.pair_id, c.value, c.is_matched, c.is_revealed
		 FROM games g JOIN game_cards c ON c.game_id = g.id
		 WHERE g.id = $1 AND g.player_id = $2
		 ORDER BY c.position
		 FOR UPDATE OF g`, gameID, playerID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting game: %w", err)
	}
	var status string
	var moves, pairsFound, totalPairs int
	var startedAt, dbNow time.Time
	cards := make([]Card, 0, 30)
	for rows.Next() {
		var card Card
		if err := rows.Scan(&status, &moves, &pairsFound, &totalPairs, &startedAt, &dbNow,
			&card.Position, &card.PairID, &card.Value, &card.Matched, &card.Revealed); err != nil {
			rows.Close()
			return nil, fmt.Errorf("reading game: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterating game: %w", err)
	}
	rows.Close()
	if len(cards) == 0 {
		return nil, ErrGameNotFound
	}
	if status == "expired" {
		return nil, ErrGameExpired
	}
	if status != "in_progress" {
		return nil, ErrGameCompleted
	}
	if dbNow.Sub(startedAt) >= gameTimeLimit {
		if _, err := tx.Exec(`UPDATE games SET status = 'expired' WHERE id = $1`, gameID); err != nil {
			return nil, fmt.Errorf("expiring game: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("committing expired game: %w", err)
		}
		return nil, ErrGameExpired
	}
	if position < 0 || position >= len(cards) || cards[position].Position != position {
		return nil, ErrCardNotFound
	}
	card := &cards[position]
	changed := map[int]*Card{}
	var open []*Card
	for i := range cards {
		if cards[i].Revealed && !cards[i].Matched {
			open = append(open, &cards[i])
		}
	}
	// The client flips a mismatched pair back on its own; hide it here first.
	if len(open) >= 2 {
		for _, previous := range open {
			previous.Revealed = false
			changed[previous.Position] = previous
		}
		open = nil
	}
	if card.Matched || card.Revealed {
		return nil, ErrCardUnavailable
	}
	changed[card.Position] = card
	card.Revealed = true

	left, elapsed := timerValues(startedAt, status, nil, dbNow)
	result := &FlipResult{
		GameID: strconv.FormatInt(gameID, 10), Position: card.Position, Value: card.Value,
		Result: "first_card_revealed", Revealed: true,
		TotalPairs: totalPairs, TimeLimitSeconds: TimeLimitSeconds,
	}
	if len(open) == 1 {
		previous := open[0]
		result.RelatedPosition = &previous.Position
		result.RelatedValue = previous.Value
		moves++
		if previous.PairID == card.PairID {
			previous.Matched, card.Matched = true, true
			changed[previous.Position] = previous
			pairsFound++
			result.Result = "match"
			result.Matched = true
			if pairsFound == totalPairs {
				status = "completed"
				// Transaction time matches the completed_at value written below.
				left, elapsed = timerValues(startedAt, status, &dbNow, dbNow)
			}
		} else {
			result.Result = "mismatch"
		}
	}

	positions := make([]int64, 0, len(changed))
	revealed := make([]bool, 0, len(changed))
	matched := make([]bool, 0, len(changed))
	for _, c := range changed {
		positions = append(positions, int64(c.Position))
		revealed = append(revealed, c.Revealed)
		matched = append(matched, c.Matched)
	}
	if _, err := tx.Exec(
		`WITH changed AS (
		   UPDATE game_cards c SET is_revealed = u.revealed, is_matched = u.matched
		   FROM unnest($2::int[], $3::bool[], $4::bool[]) AS u(position, revealed, matched)
		   WHERE c.game_id = $1 AND c.position = u.position
		 )
		 UPDATE games SET moves = $5, pairs_found = $6, status = $7::text,
		   completed_at = CASE WHEN $7::text = 'completed' AND completed_at IS NULL THEN NOW() ELSE completed_at END
		 WHERE id = $1`,
		gameID, pq.Array(positions), pq.Array(revealed), pq.Array(matched), moves, pairsFound, status,
	); err != nil {
		return nil, fmt.Errorf("saving flip: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing flip: %w", err)
	}
	result.Moves, result.PairsFound, result.Status = moves, pairsFound, status
	result.TimeLeftSeconds, result.ElapsedSeconds = left, elapsed
	return result, nil
}

var ErrNoMismatchToResolve = errors.New("no unresolved mismatch to resolve")

func (s *Service) ResolveMismatch(playerID, gameID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var status string
	var startedAt, dbNow time.Time
	err = tx.QueryRow(
		`SELECT status, started_at, NOW() FROM games WHERE id = $1 AND player_id = $2 FOR UPDATE`, gameID, playerID,
	).Scan(&status, &startedAt, &dbNow)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGameNotFound
	}
	if err != nil {
		return fmt.Errorf("getting game: %w", err)
	}
	if status == "expired" {
		return ErrGameExpired
	}
	if status != "in_progress" {
		return ErrGameCompleted
	}
	if dbNow.Sub(startedAt) >= gameTimeLimit {
		if _, err := tx.Exec(`UPDATE games SET status = 'expired' WHERE id = $1`, gameID); err != nil {
			return fmt.Errorf("expiring game: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing expired game: %w", err)
		}
		return ErrGameExpired
	}

	rows, err := tx.Query(
		`SELECT position FROM game_cards
		 WHERE game_id = $1 AND is_revealed = TRUE AND is_matched = FALSE
		 ORDER BY position FOR UPDATE`, gameID,
	)
	if err != nil {
		return fmt.Errorf("getting unmatched cards: %w", err)
	}
	positions := make([]int, 0, 2)
	for rows.Next() {
		var position int
		if err := rows.Scan(&position); err != nil {
			rows.Close()
			return fmt.Errorf("reading card position: %w", err)
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterating cards: %w", err)
	}
	rows.Close()
	if len(positions) != 2 {
		return ErrNoMismatchToResolve
	}
	if _, err := tx.Exec(
		`UPDATE game_cards SET is_revealed = FALSE
		 WHERE game_id = $1 AND position IN ($2, $3) AND is_matched = FALSE`,
		gameID, positions[0], positions[1],
	); err != nil {
		return fmt.Errorf("hiding unmatched cards: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing mismatch resolution: %w", err)
	}
	return nil
}

func (s *Service) GetLeaderboard(difficulty string, limit int) ([]LeaderboardEntry, error) {
	layout, err := GetLayout(difficulty)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	rows, err := s.db.Query(
		`WITH ranked AS (
		 SELECT g.id, p.username, g.difficulty, g.moves,
		        GREATEST(0, CEIL(EXTRACT(EPOCH FROM (g.completed_at - g.started_at))))::INT AS elapsed_seconds,
		        g.completed_at,
		        COUNT(*) OVER (PARTITION BY p.id, g.difficulty) AS attempt_count,
		        ROW_NUMBER() OVER (PARTITION BY p.id, g.difficulty
		          ORDER BY g.moves, (g.completed_at - g.started_at), g.completed_at) AS player_rank
		 FROM games g
		 JOIN players p ON p.id = g.player_id
		 WHERE g.status = 'completed' AND g.completed_at IS NOT NULL
		   AND g.difficulty = $1 AND p.username IS NOT NULL
		)
		SELECT id, username, difficulty, moves, elapsed_seconds, completed_at, attempt_count
		FROM ranked WHERE player_rank = 1
		ORDER BY moves, elapsed_seconds, completed_at
		LIMIT $2`, layout.Difficulty, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying leaderboard: %w", err)
	}
	defer rows.Close()
	entries := make([]LeaderboardEntry, 0, limit)
	for rows.Next() {
		var entry LeaderboardEntry
		var id int64
		if err := rows.Scan(&id, &entry.Username, &entry.Difficulty, &entry.Moves, &entry.ElapsedSeconds, &entry.CompletedAt, &entry.AttemptCount); err != nil {
			return nil, fmt.Errorf("reading leaderboard row: %w", err)
		}
		entry.GameID = strconv.FormatInt(id, 10)
		entry.Rank = len(entries) + 1
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating leaderboard rows: %w", err)
	}
	return entries, nil
}
