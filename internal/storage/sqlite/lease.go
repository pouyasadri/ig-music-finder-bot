package sqlite

import (
	"context"
	"database/sql"
	"time"
)

type Lease interface {
	Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	Renew(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key, owner string) error
}

type LeaseRepository interface {
	Lease
}

type SQLiteLeaseRepository struct{ db *sql.DB }

func NewLeaseRepository(db *sql.DB) *SQLiteLeaseRepository { return &SQLiteLeaseRepository{db: db} }

func (r *SQLiteLeaseRepository) Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	now := time.Now()
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO request_leases(key, owner, acquired_at, expires_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET owner=excluded.owner,
			acquired_at=excluded.acquired_at, expires_at=excluded.expires_at
			WHERE request_leases.expires_at <= ?`,
		key, owner, now.UnixNano(), now.Add(ttl).UnixNano(), now.UnixNano())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *SQLiteLeaseRepository) Renew(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`UPDATE request_leases SET expires_at = ? WHERE key = ? AND owner = ? AND expires_at > ?`,
		now.Add(ttl).UnixNano(), key, owner, now.UnixNano())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *SQLiteLeaseRepository) Release(ctx context.Context, key, owner string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM request_leases WHERE key = ? AND owner = ?`, key, owner)
	return err
}
