package sqlite

import (
	"database/sql"
	"time"

	"telegram-audio-bot/internal/domain"
)

// ErrNotFound is returned when a persistence lookup misses.
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

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
