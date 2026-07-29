//go:build integration

package worker_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

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
