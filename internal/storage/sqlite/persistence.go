package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"telegram-audio-bot/internal/domain"
)

var ErrNotFound = sql.ErrNoRows

func page(p domain.Page) (int, int) {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 1000 {
		p.Limit = 1000
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p.Limit, p.Offset
}
func nanos(t time.Time) int64     { return t.UnixNano() }
func fromNanos(v int64) time.Time { return time.Unix(0, v) }

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

type SQLiteSettingsRepository struct{ db *sql.DB }

func NewSettingsRepository(db *sql.DB) *SQLiteSettingsRepository {
	return &SQLiteSettingsRepository{db}
}

type SQLiteUserSettingsRepository = SQLiteSettingsRepository

func NewUserSettingsRepository(db *sql.DB) *SQLiteSettingsRepository {
	return NewSettingsRepository(db)
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
		s.OutputMode = "full"
	}
	_, e := r.db.ExecContext(ctx, `INSERT INTO user_settings(user_id,language,notifications_enabled,output_mode,keep_history,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET language=excluded.language,notifications_enabled=excluded.notifications_enabled,output_mode=excluded.output_mode,keep_history=excluded.keep_history,updated_at=excluded.updated_at`, s.UserID, s.Language, boolInt(s.NotificationsEnabled), s.OutputMode, boolInt(s.KeepHistory), nanos(s.CreatedAt), nanos(s.UpdatedAt))
	return e
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
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

type SQLiteRequestRepository struct{ db *sql.DB }

func NewRequestRepository(db *sql.DB) *SQLiteRequestRepository { return &SQLiteRequestRepository{db} }

type SQLiteHistoryRepository = SQLiteRequestRepository

func NewHistoryRepository(db *sql.DB) *SQLiteRequestRepository { return NewRequestRepository(db) }
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
func (r *SQLiteFavoriteRepository) DeleteForUser(ctx context.Context, uid int64) error {
	_, e := r.db.ExecContext(ctx, `DELETE FROM favorites WHERE user_id=?`, uid)
	return e
}

type SQLitePendingCallbackRepository struct{ db *sql.DB }

func NewPendingCallbackRepository(db *sql.DB) *SQLitePendingCallbackRepository {
	return &SQLitePendingCallbackRepository{db}
}

type SQLiteCallbackActionRepository = SQLitePendingCallbackRepository

func NewCallbackActionRepository(db *sql.DB) *SQLitePendingCallbackRepository {
	return NewPendingCallbackRepository(db)
}
func (r *SQLitePendingCallbackRepository) Create(ctx context.Context, v *domain.PendingCallbackAction) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	res, e := r.db.ExecContext(ctx, `INSERT INTO pending_callback_actions(user_id,action,payload,expires_at,created_at) VALUES(?,?,?,?,?)`, v.UserID, v.Action, v.Payload, nanos(v.ExpiresAt), nanos(v.CreatedAt))
	if e == nil {
		v.ID, e = res.LastInsertId()
	}
	return e
}
func (r *SQLitePendingCallbackRepository) GetByIDForUser(ctx context.Context, id, uid int64) (*domain.PendingCallbackAction, error) {
	v := new(domain.PendingCallbackAction)
	var x, c int64
	e := r.db.QueryRowContext(ctx, `SELECT id,user_id,action,payload,expires_at,created_at FROM pending_callback_actions WHERE id=? AND user_id=?`, id, uid).Scan(&v.ID, &v.UserID, &v.Action, &v.Payload, &x, &c)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e == nil {
		v.ExpiresAt = fromNanos(x)
		v.CreatedAt = fromNanos(c)
	}
	return v, e
}
func (r *SQLitePendingCallbackRepository) ListByUser(ctx context.Context, uid int64, p domain.Page) ([]domain.PendingCallbackAction, error) {
	l, o := page(p)
	rows, e := r.db.QueryContext(ctx, `SELECT id,user_id,action,payload,expires_at,created_at FROM pending_callback_actions WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, uid, l, o)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.PendingCallbackAction{}
	for rows.Next() {
		var v domain.PendingCallbackAction
		var x, c int64
		if e = rows.Scan(&v.ID, &v.UserID, &v.Action, &v.Payload, &x, &c); e != nil {
			return nil, e
		}
		v.ExpiresAt = fromNanos(x)
		v.CreatedAt = fromNanos(c)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *SQLitePendingCallbackRepository) Delete(ctx context.Context, id, uid int64) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM pending_callback_actions WHERE id=? AND user_id=?`, id, uid)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
func (r *SQLitePendingCallbackRepository) DeleteExpired(ctx context.Context) error {
	_, e := r.db.ExecContext(ctx, `DELETE FROM pending_callback_actions WHERE expires_at<=?`, nanos(time.Now()))
	return e
}
