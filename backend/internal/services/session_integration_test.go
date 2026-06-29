//go:build integration

package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/testutil"
)

func TestCreateSession_HappyPath(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	result, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if result.Session.ID.String() == "" {
		t.Fatal("expected non-empty session ID")
	}
	if !result.Participant.IsHost {
		t.Fatal("expected first participant to be host")
	}
	if result.Session.BranchID != f.BranchID {
		t.Errorf("branch_id: got %d, want %d", result.Session.BranchID, f.BranchID)
	}

	// Table must be occupied.
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM tables WHERE id = $1`, f.TableID).Scan(&status); err != nil {
		t.Fatalf("query table status: %v", err)
	}
	if status != "occupied" {
		t.Errorf("table status: got %q, want %q", status, "occupied")
	}

	// event_log must have an entry.
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM event_log WHERE session_id = $1 AND event_type = 'SESSION_CREATED'`, result.Session.ID).Scan(&count); err != nil {
		t.Fatalf("query event_log: %v", err)
	}
	if count != 1 {
		t.Errorf("event_log count: got %d, want 1", count)
	}
}

func TestCreateSession_AlreadyActive(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	if _, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", ""); err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}

	_, err := svc.CreateSession(ctx, f.TableID, "Bob", "fp-bob", "")
	if err == nil {
		t.Fatal("expected ErrSessionAlreadyActive, got nil")
	}
	if !isErr(err, domain.ErrSessionAlreadyActive) {
		t.Errorf("expected ErrSessionAlreadyActive, got %v", err)
	}
}

func TestCreateSession_ConcurrentSingleActiveSession(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	const attempts = 12
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateSession(ctx, f.TableID, "Guest", "fp", "")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if !isErr(err, domain.ErrSessionAlreadyActive) {
			t.Fatalf("unexpected CreateSession error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful sessions: got %d, want 1", successes)
	}

	var activeCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE table_id = $1 AND status = 'active'`, f.TableID).Scan(&activeCount); err != nil {
		t.Fatalf("query active sessions: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("active sessions: got %d, want 1", activeCount)
	}
	var tableStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM tables WHERE id = $1`, f.TableID).Scan(&tableStatus); err != nil {
		t.Fatalf("query table: %v", err)
	}
	if tableStatus != "occupied" {
		t.Fatalf("table status: got %q, want occupied", tableStatus)
	}
}

func TestCloseSession_OnlyHost(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	result, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Non-host participant ID (host ID + 9999 is definitely not the host).
	nonHostID := result.Participant.ID + 9999
	if err := svc.CloseSession(ctx, result.Session.ID, &nonHostID); err == nil {
		t.Fatal("expected ErrNotSessionHost, got nil")
	}
}

func TestJoinSession(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	result, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	joiner, err := svc.JoinSession(ctx, result.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if joiner.IsHost {
		t.Error("joining participant should not be host")
	}
	if joiner.SessionID != result.Session.ID {
		t.Error("joined participant's session_id does not match")
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_participants WHERE session_id = $1`,
		result.Session.ID,
	).Scan(&count); err != nil {
		t.Fatalf("query participants: %v", err)
	}
	if count != 2 {
		t.Errorf("participant count: got %d, want 2", count)
	}
}

func TestReactivateSession(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	result, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Already-active reactivation is a no-op success.
	if sess, err := svc.Reactivate(ctx, result.Session.ID); err != nil {
		t.Fatalf("Reactivate(active): %v", err)
	} else if sess.Status != sqlc.SessionStatusActive {
		t.Errorf("status after active reactivate: got %q, want active", sess.Status)
	}

	// Worker put the session into awaiting_reactivation; the guest comes back.
	if _, err := repos.TransitionSessionToAwaitingReactivation(ctx, result.Session.ID); err != nil {
		t.Fatalf("TransitionSessionToAwaitingReactivation: %v", err)
	}
	sess, err := svc.Reactivate(ctx, result.Session.ID)
	if err != nil {
		t.Fatalf("Reactivate(awaiting): %v", err)
	}
	if sess.Status != sqlc.SessionStatusActive {
		t.Errorf("status after reactivate: got %q, want active", sess.Status)
	}

	persisted, err := repos.GetSessionByID(ctx, result.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if persisted.Status != sqlc.SessionStatusActive {
		t.Errorf("persisted status: got %q, want active", persisted.Status)
	}

	// Closed sessions cannot be reactivated.
	if err := svc.CloseSession(ctx, result.Session.ID, &result.Participant.ID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if _, err := svc.Reactivate(ctx, result.Session.ID); !errors.Is(err, domain.ErrSessionClosed) {
		t.Errorf("Reactivate(closed): got %v, want ErrSessionClosed", err)
	}
}

func TestCloseSession_ReleasesTable(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	svc := newTestSessionService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	result, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := svc.CloseSession(ctx, result.Session.ID, &result.Participant.ID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	sess, err := repos.GetSessionByID(ctx, result.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if sess.Status != sqlc.SessionStatusClosed {
		t.Errorf("session status: got %q, want closed", sess.Status)
	}

	var tableStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM tables WHERE id = $1`, f.TableID).Scan(&tableStatus); err != nil {
		t.Fatalf("query table: %v", err)
	}
	if tableStatus != "available" {
		t.Errorf("table status after close: got %q, want available", tableStatus)
	}
}

func isErr(err error, target error) bool {
	return err != nil && err.Error() == target.Error()
}
