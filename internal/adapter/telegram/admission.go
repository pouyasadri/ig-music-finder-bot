package telegram

import (
	"context"
	"fmt"
	"time"

	"telegram-audio-bot/internal/domain"
)

// Admission gates inbound media requests with rate limits, leases, and a bounded queue.
type Admission struct {
	rateLimiter domain.RateLimiter
	leases      domain.Lease
	queue       chan struct{}
	rateLimit   int
	rateWindow  time.Duration
	leaseTTL    time.Duration
}

// AdmissionPermit is released when request processing finishes (or admission rolls back).
type AdmissionPermit struct {
	LeaseKey   string
	LeaseOwner string
	heldQueue  bool
	admission  *Admission
}

func NewAdmission(rateLimiter domain.RateLimiter, leases domain.Lease, maxWorkers, queueLimit, rateLimit int, rateWindow, leaseTTL time.Duration) *Admission {
	if queueLimit < 0 {
		queueLimit = 0
	}
	var queue chan struct{}
	if maxWorkers+queueLimit > 0 {
		queue = make(chan struct{}, maxWorkers+queueLimit)
	}
	return &Admission{
		rateLimiter: rateLimiter,
		leases:      leases,
		queue:       queue,
		rateLimit:   rateLimit,
		rateWindow:  rateWindow,
		leaseTTL:    leaseTTL,
	}
}

// TryAdmit enforces rate limit → lease → admission queue. On failure it returns a domain sentinel error.
func (a *Admission) TryAdmit(ctx context.Context, telegramUserID int64, leaseKey string) (*AdmissionPermit, error) {
	if a == nil {
		return &AdmissionPermit{}, nil
	}

	if a.rateLimiter != nil {
		allowed, err := a.rateLimiter.Allow(ctx, telegramUserID, a.rateLimit, a.rateWindow)
		if err != nil {
			return nil, fmt.Errorf("rate limit check failed: %w", err)
		}
		if !allowed {
			return nil, domain.ErrRateLimited
		}
	}

	permit := &AdmissionPermit{
		LeaseKey:   leaseKey,
		LeaseOwner: fmt.Sprintf("%d-%d", telegramUserID, time.Now().UnixNano()),
		admission:  a,
	}

	if a.leases != nil {
		acquired, err := a.leases.Acquire(ctx, permit.LeaseKey, permit.LeaseOwner, a.leaseTTL)
		if err != nil {
			return nil, fmt.Errorf("request lease acquisition failed: %w", err)
		}
		if !acquired {
			return nil, domain.ErrLeaseHeld
		}
	}

	if a.queue != nil {
		select {
		case a.queue <- struct{}{}:
			permit.heldQueue = true
		default:
			permit.Release()
			return nil, domain.ErrQueueFull
		}
	}

	return permit, nil
}

// Release frees lease and queue slot. Safe to call multiple times / on nil.
func (p *AdmissionPermit) Release() {
	if p == nil || p.admission == nil {
		return
	}
	a := p.admission
	if p.heldQueue && a.queue != nil {
		<-a.queue
		p.heldQueue = false
	}
	if a.leases != nil && p.LeaseKey != "" && p.LeaseOwner != "" {
		_ = a.leases.Release(context.Background(), p.LeaseKey, p.LeaseOwner)
		p.LeaseOwner = ""
	}
}
