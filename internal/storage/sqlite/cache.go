package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache entry not found")

type CacheEntry struct {
	Key       string
	Value     []byte
	ExpiresAt time.Time
}

type Cache interface {
	Get(ctx context.Context, key string) (CacheEntry, error)
	Set(ctx context.Context, entry CacheEntry) error
	Delete(ctx context.Context, key string) error
}

type CacheRepository interface {
	Cache
}

type SQLiteCacheRepository struct{ db *sql.DB }

func NewCacheRepository(db *sql.DB) *SQLiteCacheRepository { return &SQLiteCacheRepository{db: db} }

func (r *SQLiteCacheRepository) Get(ctx context.Context, key string) (CacheEntry, error) {
	var entry CacheEntry
	var expires int64
	err := r.db.QueryRowContext(ctx,
		`SELECT key, value, expires_at FROM cache_entries WHERE key = ? AND expires_at > ?`,
		key, time.Now().UnixNano()).Scan(&entry.Key, &entry.Value, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return CacheEntry{}, ErrCacheMiss
	}
	if err != nil {
		return CacheEntry{}, err
	}
	entry.ExpiresAt = time.Unix(0, expires)
	return entry, nil
}

func (r *SQLiteCacheRepository) Set(ctx context.Context, entry CacheEntry) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cache_entries(key, value, expires_at, created_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, expires_at=excluded.expires_at`,
		entry.Key, entry.Value, entry.ExpiresAt.UnixNano(), time.Now().UnixNano())
	return err
}

func (r *SQLiteCacheRepository) Delete(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM cache_entries WHERE key = ?`, key)
	return err
}
