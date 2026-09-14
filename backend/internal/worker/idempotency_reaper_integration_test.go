//go:build integration

package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/Mohith1612/qr-dining/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Nothing reaped idempotency_keys — none of the six workers registered in
// cmd/server/main.go touched it — so the table grew without bound while
// migration 000021 carried idx_idempotency_keys_expires_at for a janitor that
// was never written.
//
// The reaper must be strictly a janitor. Expiry enforcement lives on the
// lookup path (see services.GetIdempotencyKey); if deleting a row were what
// stopped it replaying, reaper scheduling would decide whether a guest got a
// stale payment.

type reaperTestQuerier struct {
	worker.Querier
	repos *repository.Repos
}

func (q *reaperTestQuerier) DeleteExpiredIdempotencyKeys(ctx context.Context, expiredBefore time.Time, limit int32) (int64, error) {
	return q.repos.DeleteExpiredIdempotencyKeys(ctx, expiredBefore, limit)
}

// seedIdempotencyKey inserts a key whose expiry sits expiresIn from now
// (negative for already-expired) and returns its scope key.
func seedIdempotencyKey(t *testing.T, pool *pgxpool.Pool, expiresIn time.Duration) string {
	t.Helper()
	key := uuid.NewString()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO idempotency_keys (scope_type, scope_id, actor_type, actor_id, key, request_hash, status, expires_at)
		VALUES ('session', $1, 'participant', '1', $2, 'hash', 'completed', NOW() + $3::interval)`,
		uuid.NewString(), key, expiresIn.String(),
	); err != nil {
		t.Fatalf("seed idempotency key: %v", err)
	}
	return key
}

func idempotencyKeyExists(t *testing.T, pool *pgxpool.Pool, key string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM idempotency_keys WHERE key = $1`, key).Scan(&n); err != nil {
		t.Fatalf("count idempotency key: %v", err)
	}
	return n > 0
}

func TestIdempotencyKeyReaper(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	repos := testutil.NewTestRepos(pool)
	redisClient := openTestRedis(t)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "idempotency_keys")
	})

	retention := worker.IdempotencyKeyRetention

	// Live: inside its 24h window, still replayable.
	live := seedIdempotencyKey(t, pool, 12*time.Hour)
	// Expired but inside the retention grace. No longer replayable (the lookup
	// filters it), but kept so support can still answer "what happened to this
	// payment tap on Saturday?".
	inGrace := seedIdempotencyKey(t, pool, -1*time.Hour)
	// Expired longer ago than the grace period. Reapable.
	stale := seedIdempotencyKey(t, pool, -(retention + 24*time.Hour))

	metrics := observability.NewMetrics()
	querier := &reaperTestQuerier{repos: repos}
	w := worker.New(pool, querier, redisClient, nil, nil, metrics, nil, "reaper-test-"+uuid.NewString(), zerolog.Nop())

	deleted, err := w.ReapIdempotencyKeys(context.Background(), retention)
	if err != nil {
		t.Fatalf("ReapIdempotencyKeys: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted count: got %d, want 1", deleted)
	}

	if idempotencyKeyExists(t, pool, stale) {
		t.Errorf("key expired beyond the retention window survived the reaper")
	}
	if !idempotencyKeyExists(t, pool, inGrace) {
		t.Errorf("key still inside the retention grace was reaped")
	}
	if !idempotencyKeyExists(t, pool, live) {
		t.Errorf("live key was reaped")
	}
}

// A second pass over an already-clean table must delete nothing and must not
// spin: the batch loop terminates on a short batch.
func TestIdempotencyKeyReaperIsIdempotent(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	repos := testutil.NewTestRepos(pool)
	redisClient := openTestRedis(t)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "idempotency_keys")
	})

	retention := worker.IdempotencyKeyRetention
	seedIdempotencyKey(t, pool, -(retention + time.Hour))

	w := worker.New(pool, &reaperTestQuerier{repos: repos}, redisClient, nil, nil,
		observability.NewMetrics(), nil, "reaper-test-"+uuid.NewString(), zerolog.Nop())

	ctx := context.Background()
	if deleted, err := w.ReapIdempotencyKeys(ctx, retention); err != nil || deleted != 1 {
		t.Fatalf("first pass: deleted=%d err=%v, want 1, nil", deleted, err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if deleted, err := w.ReapIdempotencyKeys(ctx, retention); err != nil || deleted != 0 {
			t.Errorf("second pass: deleted=%d err=%v, want 0, nil", deleted, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("reaper did not terminate on an empty table")
	}
}
