package telegram

import (
	"context"
	"time"
)

// WorkerPool bounds concurrent media processing jobs.
type WorkerPool struct {
	slots      chan struct{}
	requestTTL time.Duration
}

func NewWorkerPool(maxWorkers int, requestTTL time.Duration) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	return &WorkerPool{
		slots:      make(chan struct{}, maxWorkers),
		requestTTL: requestTTL,
	}
}

func (p *WorkerPool) Busy() bool {
	if p == nil || p.slots == nil {
		return false
	}
	return len(p.slots) == cap(p.slots)
}

// Acquire blocks until a worker slot is available (matches prior bot behavior).
func (p *WorkerPool) Acquire() (release func()) {
	if p == nil || p.slots == nil {
		return func() {}
	}
	p.slots <- struct{}{}
	return func() { <-p.slots }
}

func (p *WorkerPool) WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if p == nil || p.requestTTL <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, p.requestTTL)
}
