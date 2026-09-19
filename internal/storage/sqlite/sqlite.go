package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database and applies the storage schema migrations.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	// A single writer is sufficient for this small service and also makes
	// in-memory databases behave consistently across repository operations.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate creates the tables used by the persistence repositories.
func Migrate(ctx context.Context, db *sql.DB) error {
	const schema = `
PRAGMA busy_timeout = 5000;
CREATE TABLE IF NOT EXISTS cache_entries (
    key TEXT PRIMARY KEY,
    value BLOB NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS cache_entries_expires_at_idx ON cache_entries(expires_at);
CREATE TABLE IF NOT EXISTS request_leases (
    key TEXT PRIMARY KEY,
    owner TEXT NOT NULL,
    acquired_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS request_leases_expires_at_idx ON request_leases(expires_at);
CREATE TABLE IF NOT EXISTS rate_limits (
    user_id INTEGER PRIMARY KEY,
    window_started_at INTEGER NOT NULL,
    request_count INTEGER NOT NULL
);`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate sqlite database: %w", err)
	}
	return nil
}
