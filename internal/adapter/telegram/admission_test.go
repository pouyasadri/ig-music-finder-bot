package telegram

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"telegram-audio-bot/internal/domain"
)

type stubRateLimiter struct {
	allow bool
	err   error
}

func (s stubRateLimiter) Allow(context.Context, int64, int, time.Duration) (bool, error) {
	return s.allow, s.err
}
func (s stubRateLimiter) Reset(context.Context, int64) error { return nil }

type stubLease struct {
	mu       sync.Mutex
	held     map[string]string
	acquire  bool
	acqErr   error
	releases int
}

func (s *stubLease) Acquire(_ context.Context, key, owner string, _ time.Duration) (bool, error) {
	if s.acqErr != nil {
		return false, s.acqErr
	}
	if !s.acquire {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held == nil {
		s.held = map[string]string{}
	}
	s.held[key] = owner
	return true, nil
}
func (s *stubLease) Renew(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (s *stubLease) Release(_ context.Context, key, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases++
	if s.held[key] == owner {
		delete(s.held, key)
	}
	return nil
}

func TestAdmissionRateLimited(t *testing.T) {
	a := NewAdmission(stubRateLimiter{allow: false}, nil, 1, 0, 1, time.Hour, time.Minute)
	_, err := a.TryAdmit(context.Background(), 42, "https://example.com/a")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("got %v, want ErrRateLimited", err)
	}
}

func TestAdmissionLeaseHeld(t *testing.T) {
	leases := &stubLease{acquire: false}
	a := NewAdmission(stubRateLimiter{allow: true}, leases, 1, 0, 1, time.Hour, time.Minute)
	_, err := a.TryAdmit(context.Background(), 42, "https://example.com/a")
	if !errors.Is(err, domain.ErrLeaseHeld) {
		t.Fatalf("got %v, want ErrLeaseHeld", err)
	}
}

func TestAdmissionQueueFullReleasesLease(t *testing.T) {
	leases := &stubLease{acquire: true}
	a := NewAdmission(stubRateLimiter{allow: true}, leases, 1, 0, 1, time.Hour, time.Minute)
	first, err := a.TryAdmit(context.Background(), 1, "https://example.com/one")
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.TryAdmit(context.Background(), 2, "https://example.com/two")
	if !errors.Is(err, domain.ErrQueueFull) {
		t.Fatalf("got %v, want ErrQueueFull", err)
	}
	if leases.releases == 0 {
		t.Fatal("expected lease release on queue full")
	}
	first.Release()
}

func TestWorkerPoolBusy(t *testing.T) {
	pool := NewWorkerPool(1, time.Minute)
	if pool.Busy() {
		t.Fatal("empty pool should not be busy")
	}
	release := pool.Acquire()
	if !pool.Busy() {
		t.Fatal("full pool should be busy")
	}
	release()
	if pool.Busy() {
		t.Fatal("released pool should not be busy")
	}
}
