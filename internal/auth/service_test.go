package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func TestUsernameValidationRejectsUnsafePublicNames(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"signal_7", true},
		{"abc", true},
		{"ab", false},
		{"has space", false},
		{"name!", false},
		{"abcdefghijklmnopqrstuvwxy", false},
	}
	for _, tt := range tests {
		if got := ValidUsername(tt.name); got != tt.want {
			t.Errorf("ValidUsername(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSessionLookupRejectsUnknownOrExpiredToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("FROM sessions s").WithArgs(hashToken("expired-token")).WillReturnError(sql.ErrNoRows)
	service := &Service{db: db, now: time.Now}
	_, err = service.CurrentPlayer(context.Background(), "expired-token")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("CurrentPlayer() error = %v, want ErrUnauthenticated", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"  Person@Example.com ", "person@example.com", true},
		{"a@b.in", "a@b.in", true},
		{"not-an-email", "", false},
		{"Name <a@b.com>", "", false},
		{"a@localhost", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeEmail(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; want %q ok=%v", tt.in, got, err, tt.want, tt.ok)
		}
	}
}

func TestEnterCreatesNewPlayerAndSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, username FROM players").WithArgs("person@example.com").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("INSERT INTO players").WithArgs("signal_7", "person@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(4, "signal_7"))
	mock.ExpectExec("INSERT INTO sessions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	service := &Service{db: db, now: time.Now}
	player, token, err := service.Enter(context.Background(), "Person@Example.com", "signal_7")
	if err != nil {
		t.Fatal(err)
	}
	if player.ID != 4 || player.Username != "signal_7" || token == "" || !player.Progress.Easy {
		t.Fatalf("player = %+v token = %q", player, token)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnterReturningEmailRequiresSameUsername(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, username FROM players").WithArgs("person@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(4, "signal_7"))
	mock.ExpectRollback()
	service := &Service{db: db, now: time.Now}
	_, _, err = service.Enter(context.Background(), "person@example.com", "someone_else")
	if !errors.Is(err, ErrEmailInUse) {
		t.Fatalf("Enter() error = %v, want ErrEmailInUse", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnterReturningEmailIgnoresUsernameCase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, username FROM players").WithArgs("person@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(4, "Signal_7"))
	mock.ExpectExec("INSERT INTO sessions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	service := &Service{db: db, now: time.Now}
	player, _, err := service.Enter(context.Background(), "person@example.com", "signal_7")
	if err != nil || player.Username != "Signal_7" {
		t.Fatalf("Enter() = %+v, %v", player, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnterMapsUsernameConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, username FROM players").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("INSERT INTO players").WillReturnError(&pq.Error{Code: "23505", Constraint: "players_username_lower_key"})
	mock.ExpectRollback()
	service := &Service{db: db, now: time.Now}
	_, _, err = service.Enter(context.Background(), "new@example.com", "Signal")
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("Enter() error = %v, want ErrUsernameTaken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
