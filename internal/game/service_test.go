package game

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateGameRejectsLockedDifficulty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("pg_advisory_xact_lock").WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(8), "easy").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	_, err = NewService(db).CreateGame(8, "medium")
	if !errors.Is(err, ErrDifficultyLocked) {
		t.Fatalf("CreateGame() error = %v, want ErrDifficultyLocked", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateGameExpiresPriorAttemptBeforeInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	started := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec("pg_advisory_xact_lock").WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE games SET status = 'expired'").WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO games").WithArgs(int64(8), "easy", 4, 4, 8).
		WillReturnRows(sqlmock.NewRows([]string{"id", "started_at"}).AddRow(3, started))
	for i := 0; i < 16; i++ {
		mock.ExpectExec("INSERT INTO game_cards").WillReturnResult(sqlmock.NewResult(int64(i+1), 1))
	}
	mock.ExpectCommit()

	game, err := NewService(db).CreateGame(8, "easy")
	if err != nil {
		t.Fatal(err)
	}
	if game.ID != "3" || game.TotalPairs != 8 {
		t.Fatalf("game = %+v", game)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetGameHidesCrossPlayerExistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("UPDATE games SET status = 'expired'").WithArgs(int64(12), int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("FROM games WHERE id = \\$1 AND player_id = \\$2").WithArgs(int64(12), int64(7)).WillReturnError(sql.ErrNoRows)

	_, err = NewService(db).GetGame(7, 12)
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("GetGame() error = %v, want ErrGameNotFound", err)
	}
}

func TestFlipCardLocksOnlyOwnedGame(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("FROM games WHERE id = \\$1 AND player_id = \\$2 FOR UPDATE").
		WithArgs(int64(12), int64(7)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = NewService(db).FlipCard(7, 12, 0)
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("FlipCard() error = %v, want ErrGameNotFound", err)
	}
}

func TestLeaderboardReturnsBestPublicEntriesByDifficulty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	completed := time.Now()
	mock.ExpectQuery("WITH ranked AS").WithArgs("hard", 10).WillReturnRows(
		sqlmock.NewRows([]string{"id", "username", "difficulty", "moves", "elapsed_seconds", "completed_at", "attempt_count"}).
			AddRow(5, "signal_7", "hard", 22, 51, completed, 3),
	)
	entries, err := NewService(db).GetLeaderboard("hard", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Username != "signal_7" || entries[0].AttemptCount != 3 || entries[0].Rank != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
