package sqlite

import (
	"context"
	"database/sql"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SQLiteFavoriteRepository struct{ db *sql.DB }

func NewFavoriteRepository(db *sql.DB) *SQLiteFavoriteRepository {
	return &SQLiteFavoriteRepository{db}
}

func (r *SQLiteFavoriteRepository) Add(ctx context.Context, v domain.Favorite) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	_, e := r.db.ExecContext(ctx, `INSERT INTO favorites(user_id,track_id,created_at) VALUES(?,?,?) ON CONFLICT(user_id,track_id) DO NOTHING`, v.UserID, v.TrackID, nanos(v.CreatedAt))
	return e
}

func (r *SQLiteFavoriteRepository) ListByUser(ctx context.Context, uid int64, p domain.Page) ([]domain.Favorite, error) {
	l, o := page(p)
	rows, e := r.db.QueryContext(ctx, `SELECT user_id,track_id,created_at FROM favorites WHERE user_id=? ORDER BY created_at DESC,track_id DESC LIMIT ? OFFSET ?`, uid, l, o)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Favorite{}
	for rows.Next() {
		var v domain.Favorite
		var c int64
		if e = rows.Scan(&v.UserID, &v.TrackID, &c); e != nil {
			return nil, e
		}
		v.CreatedAt = fromNanos(c)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *SQLiteFavoriteRepository) Exists(ctx context.Context, uid, tid int64) (bool, error) {
	var n int
	e := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM favorites WHERE user_id=? AND track_id=?)`, uid, tid).Scan(&n)
	return n != 0, e
}

func (r *SQLiteFavoriteRepository) Delete(ctx context.Context, uid, tid int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM favorites WHERE user_id=? AND track_id=?`, uid, tid)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}

func (r *SQLiteFavoriteRepository) ListEnrichedByUser(ctx context.Context, uid int64, p domain.Page) ([]domain.FavoriteTrack, error) {
	l, o := page(p)
	rows, e := r.db.QueryContext(ctx, `SELECT f.user_id,f.track_id,f.created_at,t.id,t.title,t.artist,t.duration,t.spotify_url,t.youtube_url,t.created_at FROM favorites f JOIN tracks t ON t.id=f.track_id WHERE f.user_id=? ORDER BY f.created_at DESC,f.track_id DESC LIMIT ? OFFSET ?`, uid, l, o)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.FavoriteTrack{}
	for rows.Next() {
		var v domain.FavoriteTrack
		var fc, tc int64
		if e = rows.Scan(&v.Favorite.UserID, &v.Favorite.TrackID, &fc, &v.Track.ID, &v.Track.Title, &v.Track.Artist, &v.Track.Duration, &v.Track.SpotifyURL, &v.Track.YouTubeURL, &tc); e != nil {
			return nil, e
		}
		v.Favorite.CreatedAt = fromNanos(fc)
		v.Track.CreatedAt = fromNanos(tc)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *SQLiteFavoriteRepository) DeleteForUser(ctx context.Context, uid int64) error {
	_, e := r.db.ExecContext(ctx, `DELETE FROM favorites WHERE user_id=?`, uid)
	return e
}

func (r *SQLiteFavoriteRepository) DeleteAll(ctx context.Context, uid int64) error {
	return r.DeleteForUser(ctx, uid)
}
