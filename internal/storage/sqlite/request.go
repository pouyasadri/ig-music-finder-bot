package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"telegram-audio-bot/internal/domain"
)

type SQLiteRequestRepository struct{ db *sql.DB }

func NewRequestRepository(db *sql.DB) *SQLiteRequestRepository { return &SQLiteRequestRepository{db} }

func (r *SQLiteRequestRepository) Create(ctx context.Context, v *domain.Request) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	var done interface{}
	var track interface{}
	if v.TrackID != 0 {
		track = v.TrackID
	}
	if v.CompletedAt != nil {
		done = nanos(*v.CompletedAt)
	}
	res, e := r.db.ExecContext(ctx, `INSERT INTO requests(user_id,url,status,error,track_id,created_at,completed_at) VALUES(?,?,?,?,?,?,?)`, v.UserID, v.URL, v.Status, v.Error, track, nanos(v.CreatedAt), done)
	if e == nil {
		v.ID, e = res.LastInsertId()
	}
	return e
}

func (r *SQLiteRequestRepository) Update(ctx context.Context, v *domain.Request) error {
	var completed interface{}
	var track interface{}
	if v.CompletedAt != nil {
		completed = nanos(*v.CompletedAt)
	}
	if v.TrackID != 0 {
		track = v.TrackID
	}
	res, err := r.db.ExecContext(ctx, `UPDATE requests SET url=?,status=?,error=?,track_id=?,completed_at=? WHERE id=? AND user_id=?`, v.URL, v.Status, v.Error, track, completed, v.ID, v.UserID)
	if err == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return err
}

func scanRequest(row *sql.Row) (*domain.Request, error) {
	v := new(domain.Request)
	var c int64
	var track sql.NullInt64
	var done sql.NullInt64
	e := row.Scan(&v.ID, &v.UserID, &v.URL, &v.Status, &v.Error, &track, &c, &done)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e == nil {
		v.CreatedAt = fromNanos(c)
		if track.Valid {
			v.TrackID = track.Int64
		}
		if done.Valid {
			t := fromNanos(done.Int64)
			v.CompletedAt = &t
		}
	}
	return v, e
}

func (r *SQLiteRequestRepository) GetByID(ctx context.Context, id int64) (*domain.Request, error) {
	return scanRequest(r.db.QueryRowContext(ctx, `SELECT id,user_id,url,status,error,track_id,created_at,completed_at FROM requests WHERE id=?`, id))
}

func (r *SQLiteRequestRepository) GetByIDForUser(ctx context.Context, id, uid int64) (*domain.Request, error) {
	return scanRequest(r.db.QueryRowContext(ctx, `SELECT id,user_id,url,status,error,track_id,created_at,completed_at FROM requests WHERE id=? AND user_id=?`, id, uid))
}

func (r *SQLiteRequestRepository) ListByUser(ctx context.Context, uid int64, p domain.Page) ([]domain.Request, error) {
	l, o := page(p)
	rows, e := r.db.QueryContext(ctx, `SELECT id,user_id,url,status,error,track_id,created_at,completed_at FROM requests WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, uid, l, o)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Request{}
	for rows.Next() {
		v := new(domain.Request)
		var c int64
		var track, d sql.NullInt64
		if e = rows.Scan(&v.ID, &v.UserID, &v.URL, &v.Status, &v.Error, &track, &c, &d); e != nil {
			return nil, e
		}
		v.CreatedAt = fromNanos(c)
		if track.Valid {
			v.TrackID = track.Int64
		}
		if d.Valid {
			t := fromNanos(d.Int64)
			v.CompletedAt = &t
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *SQLiteRequestRepository) Delete(ctx context.Context, id int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM requests WHERE id=?`, id)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}

func (r *SQLiteRequestRepository) DeleteForUser(ctx context.Context, id, uid int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM requests WHERE id=? AND user_id=?`, id, uid)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
