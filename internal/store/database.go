package store

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func ConnectDB() (*sql.DB, error) {
	connStr := os.Getenv("DATABASE_URL")

	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}
