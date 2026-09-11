//go:build integration

package worker_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/Mohith1612/qr-dining/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type reconciliationTestQuerier struct {
	worker.Querier
	repos      *repository.Repos
	sessionID  uuid.UUID
	reconciled chan struct{}
}

func (q *reconciliationTestQuerier) ReconcileSessionTables(ctx context.Context) ([]repository.SessionTableReconciliation, error) {
	return q.repos.ReconcileSessionTables(ctx)
}

func (q *reconciliationTestQuerier) LogEvent(ctx context.Context, sessionID uuid.UUID, branchID int64, eventType, actorType string, actorID int64, payload any) {
	q.repos.LogEvent(ctx, sessionID, branchID, eventType, actorType, actorID, payload)
	if sessionID == q.sessionID && eventType == "SESSION_RECONCILED" {
		select {
		case q.reconciled <- struct{}{}:
		default:
		}
	}
}

func TestSessionTableReconcilerDoesNotCloseActiveSessionWhenRepairingAvailableTable(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openTestRedis(t)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "session_events", "sessions", "session_participants")
	})

	ctx := context.Background()
	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO sessions (branch_id, table_id, session_token, status, session_business_date, visit_number, session_number)
		 VALUES ($1, $2, $3, 'active', CURRENT_DATE, 1, $4)
		 RETURNING id`,
		f.BranchID, f.TableID, "reconcile-"+uuid.NewString(), "reconcile-"+uuid.NewString(),
	).Scan(&sessionID); err != nil {
		t.Fatalf("insert active session: %v", err)
	}

	presence := redisPkg.NewPresence(redisClient)
	if err := presence.HeartbeatScoped(ctx, f.OrganizationID, f.BranchID, sessionID, 42); err != nil {
		t.Fatalf("seed scoped presence: %v", err)
	}
	t.Cleanup(func() {
		presence.DeleteScoped(context.Background(), f.OrganizationID, f.BranchID, sessionID)
	})

	metrics := observability.NewMetrics()
	pubsub := redisPkg.NewPubSub(redisClient, zerolog.Nop(), metrics)
	publisher := events.NewPublisher(pubsub, zerolog.Nop())
	publisher.SetEventStore(repos)
	reconciled := make(chan struct{}, 1)
	querier := &reconciliationTestQuerier{
		repos:      repos,
		sessionID:  sessionID,
		reconciled: reconciled,
	}
	w := worker.New(pool, querier, redisClient, publisher, presence, metrics, nil, "reconcile-test-"+uuid.NewString(), zerolog.Nop())

	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.RunSessionTableReconciler(workerCtx, time.Millisecond)
	}()

	select {
	case <-reconciled:
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timed out waiting for reconciliation pass")
	}
	<-done

	var tableStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM tables WHERE id = $1`, f.TableID).Scan(&tableStatus); err != nil {
		t.Fatalf("query table status: %v", err)
	}
	if tableStatus != "occupied" {
		t.Fatalf("table status: got %q, want occupied", tableStatus)
	}

	var sessionStatus string
	var closedAt pgtype.Timestamptz
	if err := pool.QueryRow(ctx, `SELECT status, closed_at FROM sessions WHERE id = $1`, sessionID).Scan(&sessionStatus, &closedAt); err != nil {
		t.Fatalf("query session: %v", err)
	}
	if sessionStatus != "active" {
		t.Fatalf("session status: got %q, want active", sessionStatus)
	}
	if closedAt.Valid {
		t.Fatalf("session closed_at: got %s, want NULL", closedAt.Time)
	}

	var closedEventCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_events WHERE session_id = $1 AND event = 'SESSION_CLOSED'`,
		sessionID,
	).Scan(&closedEventCount); err != nil {
		t.Fatalf("count SESSION_CLOSED events: %v", err)
	}
	if closedEventCount != 0 {
		t.Fatalf("SESSION_CLOSED event count: got %d, want 0", closedEventCount)
	}

	present, err := presence.GetPresentScoped(ctx, f.OrganizationID, f.BranchID, sessionID)
	if err != nil {
		t.Fatalf("get scoped presence: %v", err)
	}
	if _, ok := present[42]; !ok {
		t.Fatal("scoped presence key was deleted")
	}
}

func openTestRedis(t *testing.T) *goredis.Client {
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
		_ = client.Close()
		t.Fatalf("ping test Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestStaleSessionCleaner verifies that sessions idle beyond the interval are
// detected by ListStaleSessions and can be marked abandoned via the repository.
func TestStaleSessionCleaner(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()

	// Insert an active session created 3 hours in the past (ListStaleSessions
	// keys staleness off created_at). session_business_date / visit_number /
	// session_number are NOT NULL with no default, so supply them.
	var rawID [16]byte
	if err := pool.QueryRow(ctx,
		`INSERT INTO sessions (branch_id, table_id, session_token, status, created_at, session_business_date, visit_number, session_number)
		 VALUES ($1, $2, $3, 'active', NOW() - INTERVAL '3 hours', CURRENT_DATE, 1, $4)
		 RETURNING id`,
		f.BranchID, f.TableID, "stale-test-"+uuid.NewString(), "stale-"+uuid.NewString(),
	).Scan(&rawID); err != nil {
		t.Fatalf("insert stale session: %v", err)
	}
	staleID := uuid.UUID(rawID)

	// 2-hour interval — the 3-hour-old session must be in the result set.
	interval := pgtype.Interval{Microseconds: 2 * 3_600_000_000, Valid: true}
	rows, err := repos.ListStaleSessions(ctx, interval)
	if err != nil {
		t.Fatalf("ListStaleSessions: %v", err)
	}

	found := false
	for _, r := range rows {
		if r.ID == staleID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("stale session %s not returned by ListStaleSessions", staleID)
	}

	// AbandonStaleSession marks it abandoned.
	if _, abandoned, err := repos.AbandonStaleSession(ctx, staleID, f.TableID); err != nil {
		t.Fatalf("AbandonStaleSession: %v", err)
	} else if !abandoned {
		t.Fatal("AbandonStaleSession did not abandon stale session")
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM sessions WHERE id = $1`, staleID).Scan(&status); err != nil {
		t.Fatalf("query session status: %v", err)
	}
	if status != "abandoned" {
		t.Errorf("session status: got %q, want abandoned", status)
	}
}
