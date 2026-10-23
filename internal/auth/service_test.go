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

func TestSetUsernameMapsCaseInsensitiveConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("UPDATE players").WithArgs("Signal", int64(9)).WillReturnError(&pq.Error{Code: "23505"})
	service := &Service{db: db, now: time.Now}
	err = service.SetUsername(context.Background(), 9, "Signal")
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("SetUsername() error = %v, want ErrUsernameTaken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleClaimsPersistPrivateIdentityButReturnPublicPlayer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("INSERT INTO players").
		WithArgs("google-subject", "person@example.com", true, "https://example.com/p.png").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "picture_url"}).AddRow(4, nil, "https://example.com/p.png"))
	service := &Service{db: db, now: time.Now}
	player, err := service.upsertGooglePlayer(context.Background(), Claims{
		Subject: "google-subject", Email: "person@example.com", EmailVerified: true, Picture: "https://example.com/p.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if player.ID != 4 || player.Username != "" || player.PictureURL != "https://example.com/p.png" {
		t.Fatalf("public player = %+v", player)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleClaimsRequireVerifiedEmail(t *testing.T) {
	service := &Service{now: time.Now}
	_, err := service.upsertGooglePlayer(context.Background(), Claims{Subject: "sub", Email: "person@example.com"})
	if !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("upsertGooglePlayer() error = %v, want ErrUnverifiedEmail", err)
	}
}
