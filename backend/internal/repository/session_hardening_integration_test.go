//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/testutil"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAppendSessionEvent_MonotonicUnderConcurrency(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "session_events", "sessions", "session_participants")
	})

	ctx := context.Background()
	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "sequence")

	const count = 20
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := repos.AppendSessionEvent(ctx, sessionID, ws.EventCartUpdated, map[string]any{"i": i})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("AppendSessionEvent: %v", err)
		}
	}

	events, err := repos.ListSessionEventsAfter(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("ListSessionEventsAfter: %v", err)
	}
	if len(events) != count {
		t.Fatalf("events: got %d, want %d", len(events), count)
	}
	for i, event := range events {
		want := int64(i + 1)
		if event.Sequence != want {
			t.Fatalf("sequence[%d]: got %d, want %d", i, event.Sequence, want)
		}
	}
}

func TestReconcileSessionTablesRepairsMismatches(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	secondTableID := insertTestTable(t, pool, f.BranchID, "T2")

	if _, err := pool.Exec(ctx, `UPDATE tables SET status = 'occupied' WHERE id = $1`, f.TableID); err != nil {
		t.Fatalf("mark occupied: %v", err)
	}
	sessionID := insertTestActiveSession(t, pool, f.BranchID, secondTableID, "available-active")
	if _, err := pool.Exec(ctx, `UPDATE tables SET status = 'available' WHERE id = $1`, secondTableID); err != nil {
		t.Fatalf("mark available: %v", err)
	}

	actions, err := repos.ReconcileSessionTables(ctx)
	if err != nil {
		t.Fatalf("ReconcileSessionTables: %v", err)
	}
	if len(actions) < 2 {
		t.Fatalf("actions: got %d, want at least 2", len(actions))
	}

	assertTableStatus(t, pool, f.TableID, "available")
	assertTableStatus(t, pool, secondTableID, "occupied")

	sess, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if string(sess.Status) != "active" {
		t.Fatalf("session status: got %q, want active", sess.Status)
	}
}

func TestAbandonStaleSessionDoesNotFreeNewActiveSessionTable(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	oldSession := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "old-stale")
	if _, err := pool.Exec(ctx, `UPDATE sessions SET status = 'closed', closed_at = NOW() WHERE id = $1`, oldSession); err != nil {
		t.Fatalf("close old session: %v", err)
	}
	_ = insertTestActiveSession(t, pool, f.BranchID, f.TableID, "new-active")
	if _, err := pool.Exec(ctx, `UPDATE tables SET status = 'occupied' WHERE id = $1`, f.TableID); err != nil {
		t.Fatalf("mark table occupied: %v", err)
	}

	_, abandoned, err := repos.AbandonStaleSession(ctx, oldSession, f.TableID)
	if err != nil {
		t.Fatalf("AbandonStaleSession: %v", err)
	}
	if abandoned {
		t.Fatal("closed stale session reported abandoned")
	}
	assertTableStatus(t, pool, f.TableID, "occupied")
}

func TestReconcileSessionTablesAbandonsDuplicateActiveSessions(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_one_active_per_table ON sessions(table_id) WHERE status = 'active'`)
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DROP INDEX IF EXISTS idx_sessions_one_active_per_table`); err != nil {
		t.Fatalf("drop active-session index: %v", err)
	}
	older := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "dup-old")
	newer := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "dup-new")

	actions, err := repos.ReconcileSessionTables(ctx)
	if err != nil {
		t.Fatalf("ReconcileSessionTables: %v", err)
	}
	foundDuplicate := false
	for _, action := range actions {
		if action.Action == "duplicate_abandoned" {
			foundDuplicate = true
			break
		}
	}
	if !foundDuplicate {
		t.Fatalf("duplicate_abandoned action not found: %#v", actions)
	}

	var activeID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM sessions WHERE table_id = $1 AND status = 'active'`, f.TableID).Scan(&activeID); err != nil {
		t.Fatalf("query active session: %v", err)
	}
	if activeID != newer {
		t.Fatalf("active session: got %s, want newer %s; older was %s", activeID, newer, older)
	}
}

func insertTestActiveSession(t *testing.T, pool *pgxpool.Pool, branchID, tableID int64, label string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	token := fmt.Sprintf("%s-%s", label, uuid.NewString())
	// session_business_date / visit_number / session_number are NOT NULL with no
	// default (the product's CreateSession always supplies them); supply unique
	// values here so the raw test insert satisfies the schema.
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO sessions (branch_id, table_id, session_token, status, session_business_date, visit_number, session_number)
		 VALUES ($1, $2, $3, 'active', CURRENT_DATE, 1, $4)
		 RETURNING id`,
		branchID, tableID, token, fmt.Sprintf("%s-%s", label, uuid.NewString()),
	).Scan(&id); err != nil {
		t.Fatalf("insert active session: %v", err)
	}
	return id
}

func insertTestTable(t *testing.T, pool *pgxpool.Pool, branchID int64, identifier string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
		 VALUES ($1, $2, 4, $3, 'available')
		 RETURNING id`,
		branchID, identifier, uuid.NewString(),
	).Scan(&id); err != nil {
		t.Fatalf("insert table: %v", err)
	}
	return id
}

func assertTableStatus(t *testing.T, pool *pgxpool.Pool, tableID int64, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM tables WHERE id = $1`, tableID).Scan(&got); err != nil {
		t.Fatalf("query table status: %v", err)
	}
	if got != want {
		t.Fatalf("table %d status: got %q, want %q", tableID, got, want)
	}
}
