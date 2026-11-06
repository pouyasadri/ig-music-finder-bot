package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SQLiteRateLimitRepository struct{ db *sql.DB }

func NewRateLimitRepository(db *sql.DB) *SQLiteRateLimitRepository {
	return &SQLiteRateLimitRepository{db: db}
}

var _ domain.RateLimiter = (*SQLiteRateLimitRepository)(nil)

// Allow atomically records a request against the user's sliding window.
// A single UPSERT statement keeps concurrent callers from overshooting the limit;
// SQLITE_BUSY is retried briefly so multi-connection writers can serialize.
func (r *SQLiteRateLimitRepository) Allow(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return false, nil
	}

	var lastErr error
	for attempt := 0; attempt < 32; attempt++ {
		ok, err := r.allowOnce(ctx, userID, limit, window)
		if err == nil {
			return ok, nil
		}
		if !isSQLiteBusy(err) {
			return false, err
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Millisecond):
		}
	}
	return false, lastErr
}

func (r *SQLiteRateLimitRepository) allowOnce(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error) {
	now := time.Now().UnixNano()
	windowNs := window.Nanoseconds()

	res, err := r.db.ExecContext(ctx, `
		INSERT INTO rate_limits(user_id, window_started_at, request_count)
		VALUES(?, ?, 1)
		ON CONFLICT(user_id) DO UPDATE SET
			window_started_at = CASE
				WHEN excluded.window_started_at >= rate_limits.window_started_at + ? THEN excluded.window_started_at
				ELSE rate_limits.window_started_at
			END,
			request_count = CASE
				WHEN excluded.window_started_at >= rate_limits.window_started_at + ? THEN 1
				ELSE rate_limits.request_count + 1
			END
		WHERE excluded.window_started_at >= rate_limits.window_started_at + ?
		   OR rate_limits.request_count < ?`,
		userID, now,
		windowNs,
		windowNs,
		windowNs,
		limit,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *SQLiteRateLimitRepository) Reset(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rate_limits WHERE user_id = ?`, userID)
	return err
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}
