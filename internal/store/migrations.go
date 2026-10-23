package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	migrationfiles "memory-match/db/migrations"
)

const migrationLockID int64 = 723_194_221

func Migrate(ctx context.Context, db *sql.DB) error {
	return migrateFS(ctx, db, migrationfiles.Files)
}

func migrateFS(ctx context.Context, db *sql.DB, files fs.FS) (resultErr error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("creating migration table: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("locking migrations: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockID); resultErr == nil && err != nil {
			resultErr = fmt.Errorf("unlocking migrations: %w", err)
		}
	}()

	applied := make(map[string]bool)
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("reading applied migrations: %w", err)
	}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("reading migration version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterating migrations: %w", err)
	}
	rows.Close()

	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("listing migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") || applied[name] {
			continue
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("applying migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", name, err)
		}
	}
	return nil
}
