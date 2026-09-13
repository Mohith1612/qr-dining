//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSessionTimeoutRecentActivityPreventsWarningAndClosure(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "recent-activity")
	setSessionCreatedAgo(t, pool, sessionID, "3 hours")
	insertParticipantLastSeenAgo(t, pool, sessionID, "10 minutes")
	setBranchSessionTimeout(t, pool, f.BranchID, 120)

	expiringSoon, err := repos.ListSessionsExpiringSoon(ctx)
	if err != nil {
		t.Fatalf("ListSessionsExpiringSoon: %v", err)
	}
	if containsExpiringSession(expiringSoon, sessionID) {
		t.Fatalf("recently active session %s was selected for an expiry warning", sessionID)
	}

	expired, err := repos.ListExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("ListExpiredSessions: %v", err)
	}
	if containsExpiredSession(expired, sessionID) {
		t.Fatalf("recently active session %s was selected for closure", sessionID)
	}

	session, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if session.Status != sqlc.SessionStatusActive {
		t.Fatalf("session status: got %q, want active", session.Status)
	}
	if session.WarnedAt.Valid {
		t.Fatalf("warned_at: got %s, want NULL", session.WarnedAt.Time)
	}
}

func TestSessionTimeoutWarnsThenClosesAfterLastActivityDeadline(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "idle-lifecycle")
	setSessionCreatedAgo(t, pool, sessionID, "3 hours")
	insertParticipantLastSeenAgo(t, pool, sessionID, "110 minutes")
	setBranchSessionTimeout(t, pool, f.BranchID, 120)

	expiringSoon, err := repos.ListSessionsExpiringSoon(ctx)
	if err != nil {
		t.Fatalf("ListSessionsExpiringSoon: %v", err)
	}
	if !containsExpiringSession(expiringSoon, sessionID) {
		t.Fatalf("session %s with ten minutes left was not selected for an expiry warning", sessionID)
	}
	if err := repos.MarkSessionWarned(ctx, sessionID); err != nil {
		t.Fatalf("MarkSessionWarned: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE session_participants SET last_seen_at = NOW() - INTERVAL '121 minutes' WHERE session_id = $1`,
		sessionID,
	); err != nil {
		t.Fatalf("advance session beyond inactivity timeout: %v", err)
	}

	expired, err := repos.ListExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("ListExpiredSessions: %v", err)
	}
	if !containsExpiredSession(expired, sessionID) {
		t.Fatalf("session %s idle beyond its timeout was not selected for closure", sessionID)
	}
	if _, abandoned, err := repos.AbandonStaleSession(ctx, sessionID, f.TableID); err != nil {
		t.Fatalf("AbandonStaleSession: %v", err)
	} else if !abandoned {
		t.Fatalf("session %s was selected but not abandoned", sessionID)
	}

	session, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !session.WarnedAt.Valid {
		t.Fatal("warned_at: got NULL, want warning timestamp")
	}
	if session.Status != sqlc.SessionStatusAbandoned {
		t.Fatalf("session status: got %q, want abandoned", session.Status)
	}
}

func TestSessionTimeoutWithoutParticipantsFallsBackToCreationTime(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "no-participants")
	setSessionCreatedAgo(t, pool, sessionID, "3 hours")
	setBranchSessionTimeout(t, pool, f.BranchID, 120)

	expired, err := repos.ListExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("ListExpiredSessions: %v", err)
	}
	if !containsExpiredSession(expired, sessionID) {
		t.Fatalf("session %s without participants was not selected for closure", sessionID)
	}
	if _, abandoned, err := repos.AbandonStaleSession(ctx, sessionID, f.TableID); err != nil {
		t.Fatalf("AbandonStaleSession: %v", err)
	} else if !abandoned {
		t.Fatalf("session %s was selected but not abandoned", sessionID)
	}

	session, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if session.Status != sqlc.SessionStatusAbandoned {
		t.Fatalf("session status: got %q, want abandoned", session.Status)
	}
}

func TestSessionTimeoutParticipantWithoutHeartbeatUsesJoinActivity(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "no-heartbeat")
	setSessionCreatedAgo(t, pool, sessionID, "3 hours")
	insertParticipantAtSessionCreation(t, pool, sessionID)
	setBranchSessionTimeout(t, pool, f.BranchID, 120)

	expired, err := repos.ListExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("ListExpiredSessions: %v", err)
	}
	if !containsExpiredSession(expired, sessionID) {
		t.Fatalf("session %s whose participant never heartbeated was not selected for closure", sessionID)
	}
	if _, abandoned, err := repos.AbandonStaleSession(ctx, sessionID, f.TableID); err != nil {
		t.Fatalf("AbandonStaleSession: %v", err)
	} else if !abandoned {
		t.Fatalf("session %s was selected but not abandoned", sessionID)
	}
}

func TestSessionTimeoutQueriesReturnOneRowPerSession(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "cardinality")
	setSessionCreatedAgo(t, pool, sessionID, "3 hours")
	insertParticipantLastSeenAgo(t, pool, sessionID, "110 minutes")
	insertParticipantLastSeenAgo(t, pool, sessionID, "115 minutes")
	setBranchSessionTimeout(t, pool, f.BranchID, 120)

	expiringSoon, err := repos.ListSessionsExpiringSoon(ctx)
	if err != nil {
		t.Fatalf("ListSessionsExpiringSoon: %v", err)
	}
	if got := countExpiringSession(expiringSoon, sessionID); got != 1 {
		t.Fatalf("expiring-soon row count for session %s: got %d, want 1", sessionID, got)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE session_participants SET last_seen_at = NOW() - INTERVAL '121 minutes' WHERE session_id = $1`,
		sessionID,
	); err != nil {
		t.Fatalf("advance session beyond inactivity timeout: %v", err)
	}
	expired, err := repos.ListExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("ListExpiredSessions: %v", err)
	}
	if got := countExpiredSession(expired, sessionID); got != 1 {
		t.Fatalf("expired row count for session %s: got %d, want 1", sessionID, got)
	}
}

func TestSessionParticipantLastSeenAtCannotBeNull(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	sessionID := insertTestActiveSession(t, pool, f.BranchID, f.TableID, "null-last-seen")
	_, err := pool.Exec(context.Background(),
		`INSERT INTO session_participants (session_id, display_name, device_fingerprint, last_seen_at)
		 VALUES ($1, 'Never Connected', $2, NULL)`,
		sessionID, uuid.NewString(),
	)
	if err == nil {
		t.Fatal("participant with NULL last_seen_at was inserted")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23502" {
		t.Fatalf("NULL last_seen_at error: got %v, want PostgreSQL not_null_violation (23502)", err)
	}
}

func setSessionCreatedAgo(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, interval string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE sessions SET created_at = NOW() - $2::interval WHERE id = $1`,
		sessionID, interval,
	); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
}

func setBranchSessionTimeout(t *testing.T, pool *pgxpool.Pool, branchID int64, minutes int16) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE branches SET session_timeout_minutes = $2 WHERE id = $1`,
		branchID, minutes,
	); err != nil {
		t.Fatalf("set branch session timeout: %v", err)
	}
}

func insertParticipantLastSeenAgo(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, interval string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO session_participants
			(session_id, display_name, device_fingerprint, joined_at, last_seen_at, is_host)
		 SELECT id, 'Guest', $2, created_at, NOW() - $3::interval, TRUE
		 FROM sessions WHERE id = $1`,
		sessionID, uuid.NewString(), interval,
	); err != nil {
		t.Fatalf("insert participant: %v", err)
	}
}

func insertParticipantAtSessionCreation(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO session_participants
			(session_id, display_name, device_fingerprint, joined_at, last_seen_at, is_host)
		 SELECT id, 'Never Heartbeated', $2, created_at, created_at, TRUE
		 FROM sessions WHERE id = $1`,
		sessionID, uuid.NewString(),
	); err != nil {
		t.Fatalf("insert never-heartbeated participant: %v", err)
	}
}

func containsExpiredSession(rows []sqlc.ListExpiredSessionsRow, sessionID uuid.UUID) bool {
	return countExpiredSession(rows, sessionID) > 0
}

func countExpiredSession(rows []sqlc.ListExpiredSessionsRow, sessionID uuid.UUID) int {
	count := 0
	for _, row := range rows {
		if row.ID == sessionID {
			count++
		}
	}
	return count
}

func containsExpiringSession(rows []sqlc.ListSessionsExpiringSoonRow, sessionID uuid.UUID) bool {
	return countExpiringSession(rows, sessionID) > 0
}

func countExpiringSession(rows []sqlc.ListSessionsExpiringSoonRow, sessionID uuid.UUID) int {
	count := 0
	for _, row := range rows {
		if row.ID == sessionID {
			count++
		}
	}
	return count
}
