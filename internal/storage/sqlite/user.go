package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SQLiteUserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *SQLiteUserRepository { return &SQLiteUserRepository{db} }

func (r *SQLiteUserRepository) Create(ctx context.Context, u *domain.User) error {
	now := time.Now()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = u.CreatedAt
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO users(telegram_id,username,first_name,last_name,created_at,updated_at) VALUES(?,?,?,?,?,?)`, u.TelegramID, u.Username, u.FirstName, u.LastName, nanos(u.CreatedAt), nanos(u.UpdatedAt))
	if err == nil {
		u.ID, err = res.LastInsertId()
	}
	return err
}

func (r *SQLiteUserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u := new(domain.User)
	var c, a int64
	err := r.db.QueryRowContext(ctx, `SELECT id,telegram_id,username,first_name,last_name,created_at,updated_at FROM users WHERE id=?`, id).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName, &c, &a)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil {
		u.CreatedAt = fromNanos(c)
		u.UpdatedAt = fromNanos(a)
	}
	return u, err
}

func (r *SQLiteUserRepository) GetByTelegramID(ctx context.Context, tid int64) (*domain.User, error) {
	u := new(domain.User)
	var c, a int64
	err := r.db.QueryRowContext(ctx, `SELECT id,telegram_id,username,first_name,last_name,created_at,updated_at FROM users WHERE telegram_id=?`, tid).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName, &c, &a)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil {
		u.CreatedAt = fromNanos(c)
		u.UpdatedAt = fromNanos(a)
	}
	return u, err
}

func (r *SQLiteUserRepository) Update(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now()
	res, err := r.db.ExecContext(ctx, `UPDATE users SET telegram_id=?,username=?,first_name=?,last_name=?,updated_at=? WHERE id=?`, u.TelegramID, u.Username, u.FirstName, u.LastName, nanos(u.UpdatedAt), u.ID)
	if err == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return err
}

func (r *SQLiteUserRepository) Delete(ctx context.Context, id int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
