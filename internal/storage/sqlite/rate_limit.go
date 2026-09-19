package sqlite

import (
	"context"
	"database/sql"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error)
	Reset(ctx context.Context, userID int64) error
}

type RateLimitRepository interface {
	RateLimiter
}

type SQLiteRateLimitRepository struct{ db *sql.DB }

func NewRateLimitRepository(db *sql.DB) *SQLiteRateLimitRepository {
	return &SQLiteRateLimitRepository{db: db}
}

func (r *SQLiteRateLimitRepository) Allow(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	now := time.Now()
	var started, count int64
	err := r.db.QueryRowContext(ctx, `SELECT window_started_at, request_count FROM rate_limits WHERE user_id = ?`, userID).Scan(&started, &count)
	if err == sql.ErrNoRows || (err == nil && now.UnixNano() >= started+window.Nanoseconds()) {
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO rate_limits(user_id, window_started_at, request_count) VALUES(?, ?, 1)
			ON CONFLICT(user_id) DO UPDATE SET window_started_at=excluded.window_started_at, request_count=1`,
			userID, now.UnixNano())
		return err == nil, err
	}
	if err != nil {
		return false, err
	}
	if count >= int64(limit) {
		return false, nil
	}
	_, err = r.db.ExecContext(ctx, `UPDATE rate_limits SET request_count = request_count + 1 WHERE user_id = ?`, userID)
	return err == nil, err
}

func (r *SQLiteRateLimitRepository) Reset(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rate_limits WHERE user_id = ?`, userID)
	return err
}
