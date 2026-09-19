package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCacheRepository(t *testing.T) {
	r := NewCacheRepository(testDB(t))
	ctx := context.Background()
	if err := r.Set(ctx, CacheEntry{Key: "k", Value: []byte("v"), ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "k")
	if err != nil || string(got.Value) != "v" {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	if err := r.Set(ctx, CacheEntry{Key: "k", Value: []byte("old"), ExpiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, "k"); err != ErrCacheMiss {
		t.Fatalf("expired Get() error = %v, want ErrCacheMiss", err)
	}
}

func TestLeaseRepository(t *testing.T) {
	r := NewLeaseRepository(testDB(t))
	ctx := context.Background()
	if ok, err := r.Acquire(ctx, "k", "a", time.Minute); err != nil || !ok {
		t.Fatalf("first Acquire() = %v, %v", ok, err)
	}
	if ok, err := r.Acquire(ctx, "k", "b", time.Minute); err != nil || ok {
		t.Fatalf("contending Acquire() = %v, %v", ok, err)
	}
	if err := r.Release(ctx, "k", "a"); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.Acquire(ctx, "k", "b", time.Minute); err != nil || !ok {
		t.Fatalf("Acquire after Release() = %v, %v", ok, err)
	}
}

func TestRateLimitRepository(t *testing.T) {
	r := NewRateLimitRepository(testDB(t))
	ctx := context.Background()
	if ok, err := r.Allow(ctx, 7, 2, time.Minute); err != nil || !ok {
		t.Fatalf("first Allow() = %v, %v", ok, err)
	}
	if ok, err := r.Allow(ctx, 7, 2, time.Minute); err != nil || !ok {
		t.Fatalf("second Allow() = %v, %v", ok, err)
	}
	if ok, err := r.Allow(ctx, 7, 2, time.Minute); err != nil || ok {
		t.Fatalf("third Allow() = %v, %v", ok, err)
	}
}
