package store

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMigrateFSAppliesFilesInOrderOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	files := fstest.MapFS{
		"002_second.up.sql": {Data: []byte("SELECT 2")},
		"001_first.up.sql":  {Data: []byte("SELECT 1")},
	}
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("pg_advisory_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT version FROM schema_migrations").WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectBegin()
	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("001_first.up.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("SELECT 2").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("002_second.up.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec("pg_advisory_unlock").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := migrateFS(context.Background(), db, files); err != nil {
		t.Fatalf("migrateFS() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateFSSkipsAppliedFiles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	files := fstest.MapFS{"001_first.up.sql": {Data: []byte("SELECT 1")}}
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("pg_advisory_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT version FROM schema_migrations").WillReturnRows(
		sqlmock.NewRows([]string{"version"}).AddRow("001_first.up.sql"),
	)
	mock.ExpectExec("pg_advisory_unlock").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := migrateFS(context.Background(), db, files); err != nil {
		t.Fatalf("migrateFS() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateFSRollsBackFailedMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	files := fstest.MapFS{"001_broken.up.sql": {Data: []byte("BROKEN")}}
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("pg_advisory_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT version FROM schema_migrations").WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectBegin()
	mock.ExpectExec("BROKEN").WillReturnError(errors.New("syntax error"))
	mock.ExpectRollback()
	mock.ExpectExec("pg_advisory_unlock").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := migrateFS(context.Background(), db, files); err == nil {
		t.Fatal("migrateFS() error = nil, want migration failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
