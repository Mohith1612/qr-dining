//go:build integration

package worker_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/Mohith1612/qr-dining/internal/worker"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func TestReactivationWorker_LiveFallbackHeartbeatKeepsSessionActive(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openWorkerTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, observability.NewMetrics(), nil)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sessions SET created_at = NOW() - INTERVAL '10 minutes' WHERE id = $1`, created.Session.ID); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
	if err := presence.Heartbeat(ctx, created.Session.ID, created.Participant.ID); err != nil {
		t.Fatalf("fallback Heartbeat: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE session_participants SET last_seen_at = NOW() - INTERVAL '6 minutes' WHERE session_id = $1`, created.Session.ID); err != nil {
		t.Fatalf("backdate durable participant presence: %v", err)
	}

	queries := newReactivationTestQuerier(repos)
	w := worker.New(pool, queries, redisClient, pub, presence, observability.NewMetrics(), nil, "test", zerolog.Nop())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.RunReactivationPipeline(ctx, 5*time.Millisecond, time.Minute, 5*time.Minute)
	}()
	waitForReactivationPass(t, queries.completed)
	cancel()
	<-done

	persisted, err := repos.GetSessionByID(context.Background(), created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if persisted.Status != "active" {
		t.Fatalf("session status: got %q, want active", persisted.Status)
	}
}

func TestReactivationWorker_EstablishedSessionIdleForThreeMinutesStaysActive(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openWorkerTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, observability.NewMetrics(), nil)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sessions SET created_at = NOW() - INTERVAL '10 minutes' WHERE id = $1`, created.Session.ID); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
	// Three minutes since the final real heartbeat can look almost five minutes
	// old in PostgreSQL because last_seen_at writes are throttled for two minutes.
	// Include 1m45s of durable-write lag while leaving enough scheduling margin
	// below the default five-minute idle threshold for a stable integration test.
	if _, err := pool.Exec(ctx, `UPDATE session_participants SET last_seen_at = NOW() - INTERVAL '4 minutes 45 seconds' WHERE session_id = $1`, created.Session.ID); err != nil {
		t.Fatalf("backdate participant presence: %v", err)
	}

	queries := newReactivationTestQuerier(repos)
	w := worker.New(pool, queries, redisClient, pub, presence, observability.NewMetrics(), nil, "test", zerolog.Nop())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.RunReactivationPipeline(ctx, 5*time.Millisecond, time.Minute, 5*time.Minute)
	}()
	waitForReactivationPass(t, queries.completed)
	cancel()
	<-done

	persisted, err := repos.GetSessionByID(context.Background(), created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if persisted.Status != "active" {
		t.Fatalf("session status after three idle minutes: got %q, want active", persisted.Status)
	}
}

type reactivationTestQuerier struct {
	repos     *repository.Repos
	completed chan struct{}
	once      sync.Once
}

func newReactivationTestQuerier(repos *repository.Repos) *reactivationTestQuerier {
	return &reactivationTestQuerier{repos: repos, completed: make(chan struct{})}
}

func (q *reactivationTestQuerier) ListExpiredSessions(context.Context) ([]worker.ExpiredSession, error) {
	return nil, nil
}

func (q *reactivationTestQuerier) AbandonStaleSession(context.Context, uuid.UUID, int64) (repository.SessionMaintenanceResult, bool, error) {
	return repository.SessionMaintenanceResult{}, false, nil
}

func (q *reactivationTestQuerier) ListSessionsExpiringSoon(context.Context) ([]worker.ExpiringSoonSession, error) {
	return nil, nil
}

func (q *reactivationTestQuerier) MarkSessionWarned(context.Context, uuid.UUID) error {
	return nil
}

func (q *reactivationTestQuerier) ReconcileSessionTables(context.Context) ([]repository.SessionTableReconciliation, error) {
	return nil, nil
}

func (q *reactivationTestQuerier) LogEvent(context.Context, uuid.UUID, int64, string, string, int64, any) {
}

func (q *reactivationTestQuerier) ListReactivationCandidates(ctx context.Context, createdBefore, idleBefore time.Time) ([]worker.ReactivationCandidate, error) {
	rows, err := q.repos.ListReactivationCandidates(ctx, createdBefore, idleBefore)
	if err != nil {
		return nil, err
	}
	candidates := make([]worker.ReactivationCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, worker.ReactivationCandidate{
			ID:             row.ID,
			OrganizationID: row.OrganizationID,
			BranchID:       row.BranchID,
			TableID:        row.TableID,
		})
	}
	return candidates, nil
}

func (q *reactivationTestQuerier) TransitionToAwaitingReactivation(ctx context.Context, id uuid.UUID) error {
	_, err := q.repos.TransitionSessionToAwaitingReactivation(ctx, id)
	return err
}

func (q *reactivationTestQuerier) ListAwaitingReactivationExpired(context.Context, time.Time) ([]worker.AwaitingReactivationExpired, error) {
	q.once.Do(func() { close(q.completed) })
	return nil, nil
}

func (q *reactivationTestQuerier) HasNonTerminalPayment(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	return q.repos.HasNonTerminalPaymentForSession(ctx, sessionID)
}

func (q *reactivationTestQuerier) ListPaymentPendingStalled(context.Context, time.Time) ([]worker.StalledPaymentPending, error) {
	return nil, nil
}

func openWorkerTestRedis(t *testing.T) *goredis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse TEST_REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	return client
}

func waitForReactivationPass(t *testing.T, completed <-chan struct{}) {
	t.Helper()
	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("reactivation worker did not complete a pass")
	}
}
