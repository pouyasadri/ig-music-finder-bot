package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"sync/atomic"
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

// testDBConcurrent opens a WAL-backed DB with a multi-connection pool so
// concurrent repository calls can race (Open uses MaxOpenConns=1).
func testDBConcurrent(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "concurrent.sqlite")
	db, err := sql.Open("sqlite", withSQLitePragmas(path)+"&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
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

func TestRateLimitRepositoryWindowReset(t *testing.T) {
	r := NewRateLimitRepository(testDB(t))
	ctx := context.Background()
	window := 30 * time.Millisecond
	if ok, err := r.Allow(ctx, 9, 1, window); err != nil || !ok {
		t.Fatalf("first Allow() = %v, %v", ok, err)
	}
	if ok, err := r.Allow(ctx, 9, 1, window); err != nil || ok {
		t.Fatalf("second Allow() = %v, %v", ok, err)
	}
	time.Sleep(window + 10*time.Millisecond)
	if ok, err := r.Allow(ctx, 9, 1, window); err != nil || !ok {
		t.Fatalf("Allow after window = %v, %v", ok, err)
	}
}

func TestRateLimitRepositoryConcurrent(t *testing.T) {
	r := NewRateLimitRepository(testDBConcurrent(t))
	ctx := context.Background()
	const (
		limit      = 40
		goroutines = 120
		userID     = int64(42)
	)
	var allowed atomic.Int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ok, err := r.Allow(ctx, userID, limit, time.Minute)
			if err != nil {
				t.Errorf("Allow() error: %v", err)
				return
			}
			if ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != int64(limit) {
		t.Fatalf("concurrent Allow() granted %d, want exactly %d", got, limit)
	}
}

func TestLeaseAndCacheDeleteExpired(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	leases := NewLeaseRepository(db)
	if ok, err := leases.Acquire(ctx, "stale", "owner", time.Millisecond); err != nil || !ok {
		t.Fatalf("Acquire() = %v, %v", ok, err)
	}
	time.Sleep(5 * time.Millisecond)
	n, err := leases.DeleteExpired(ctx)
	if err != nil || n != 1 {
		t.Fatalf("DeleteExpired leases = %d, %v", n, err)
	}

	cache := NewCacheRepository(db)
	if err := cache.Set(ctx, CacheEntry{Key: "old", Value: []byte("v"), ExpiresAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	n, err = cache.DeleteExpired(ctx)
	if err != nil || n != 1 {
		t.Fatalf("DeleteExpired cache = %d, %v", n, err)
	}
}
