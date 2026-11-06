package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SQLiteTrackRepository struct{ db *sql.DB }

func NewTrackRepository(db *sql.DB) *SQLiteTrackRepository { return &SQLiteTrackRepository{db} }

func (r *SQLiteTrackRepository) Create(ctx context.Context, v *domain.Track) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	res, e := r.db.ExecContext(ctx, `INSERT INTO tracks(title,artist,duration,spotify_url,youtube_url,created_at) VALUES(?,?,?,?,?,?)`, v.Title, v.Artist, v.Duration, v.SpotifyURL, v.YouTubeURL, nanos(v.CreatedAt))
	if e == nil {
		v.ID, e = res.LastInsertId()
	}
	return e
}

func (r *SQLiteTrackRepository) GetByID(ctx context.Context, id int64) (*domain.Track, error) {
	v := new(domain.Track)
	var c int64
	e := r.db.QueryRowContext(ctx, `SELECT id,title,artist,duration,spotify_url,youtube_url,created_at FROM tracks WHERE id=?`, id).Scan(&v.ID, &v.Title, &v.Artist, &v.Duration, &v.SpotifyURL, &v.YouTubeURL, &c)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e == nil {
		v.CreatedAt = fromNanos(c)
	}
	return v, e
}

func (r *SQLiteTrackRepository) Delete(ctx context.Context, id int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM tracks WHERE id=?`, id)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
