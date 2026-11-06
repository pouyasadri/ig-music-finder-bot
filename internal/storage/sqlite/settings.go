package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SQLiteSettingsRepository struct{ db *sql.DB }

func NewSettingsRepository(db *sql.DB) *SQLiteSettingsRepository {
	return &SQLiteSettingsRepository{db}
}

func (r *SQLiteSettingsRepository) Get(ctx context.Context, id int64) (*domain.UserSettings, error) {
	s := new(domain.UserSettings)
	var c, a int64
	var n, keep int
	err := r.db.QueryRowContext(ctx, `SELECT user_id,language,notifications_enabled,output_mode,keep_history,created_at,updated_at FROM user_settings WHERE user_id=?`, id).Scan(&s.UserID, &s.Language, &n, &s.OutputMode, &keep, &c, &a)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	s.NotificationsEnabled = n != 0
	s.KeepHistory = keep != 0
	s.CreatedAt = fromNanos(c)
	s.UpdatedAt = fromNanos(a)
	return s, err
}

func (r *SQLiteSettingsRepository) Upsert(ctx context.Context, s *domain.UserSettings) error {
	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	if s.OutputMode == "" {
		s.OutputMode = domain.OutputModeFull
	}
	_, e := r.db.ExecContext(ctx, `INSERT INTO user_settings(user_id,language,notifications_enabled,output_mode,keep_history,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET language=excluded.language,notifications_enabled=excluded.notifications_enabled,output_mode=excluded.output_mode,keep_history=excluded.keep_history,updated_at=excluded.updated_at`, s.UserID, s.Language, boolInt(s.NotificationsEnabled), s.OutputMode, boolInt(s.KeepHistory), nanos(s.CreatedAt), nanos(s.UpdatedAt))
	return e
}

func (r *SQLiteSettingsRepository) Delete(ctx context.Context, id int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM user_settings WHERE user_id=?`, id)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
