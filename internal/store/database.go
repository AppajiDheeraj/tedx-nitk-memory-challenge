package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// ConnectDB opens DATABASE_URL, retrying while a suspended Neon compute wakes up.
func ConnectDB() (*sql.DB, error) {
	connStr := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set (use the Neon connection string)")
	}
	// Neon's psql snippet includes channel_binding, which lib/pq does not support.
	connStr = strings.ReplaceAll(connStr, "&channel_binding=require", "")
	connStr = strings.ReplaceAll(connStr, "channel_binding=require&", "")
	connStr = strings.ReplaceAll(connStr, "?channel_binding=require", "")

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}

	for attempt := 1; ; attempt++ {
		err = db.Ping()
		if err == nil {
			break
		}
		if attempt == 6 {
			db.Close()
			return nil, fmt.Errorf("connecting to database: %w", err)
		}
		log.Printf("database not reachable yet (attempt %d): %v", attempt, err)
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}
