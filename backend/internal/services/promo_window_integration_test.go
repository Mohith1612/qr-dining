package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Promo daily time windows are branch wall-clock times. The lookup must
// compare against the branch's timezone — comparing against the DB server's
// LOCALTIME (UTC) silently disabled windowed promos for non-UTC branches.
func TestPromoTimeWindowUsesBranchTimezone(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())

	// Branch clock: Asia/Kolkata (UTC+5:30) — guaranteed != UTC wall time.
	mustExecPromo(t, pool, `UPDATE branches SET timezone = 'Asia/Kolkata' WHERE id = $1`, f.BranchID)

	istNow := time.Now().UTC().Add(5*time.Hour + 30*time.Minute)
	utcNow := time.Now().UTC()

	// Narrow ±30min windows: IST and UTC are 5.5h apart so the two windows can
	// never overlap. A window that wraps midnight (start > end) can't be
	// expressed in the schema's start<=t<=end comparison — skip in that hour.
	window := func(center time.Time) (string, string) {
		start, end := center.Add(-30*time.Minute), center.Add(30*time.Minute)
		if start.Format("15:04") > end.Format("15:04") {
			t.Skip("window would wrap midnight; skipping this hour")
		}
		return start.Format("15:04"), end.Format("15:04")
	}

	insertPromo := func(code, start, end string) {
		mustExecPromo(t, pool, `
			INSERT INTO promos (branch_id, code, type, value, valid_from, valid_until, time_window_start, time_window_end, is_active)
			VALUES ($1, $2, 'percentage', 10, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 day', $3::time, $4::time, TRUE)`,
			f.BranchID, code, start, end)
	}

	// Window around the branch-local (IST) time: must match.
	istStart, istEnd := window(istNow)
	insertPromo("ISTWINDOW", istStart, istEnd)
	if _, err := repos.GetPromoByCode(ctx, f.BranchID, "ISTWINDOW"); err != nil {
		t.Fatalf("IST-window promo not found at IST time (window %s-%s): %v", istStart, istEnd, err)
	}

	// Window around UTC time (and clear of IST time, 5.5h apart): must NOT match.
	utcStart, utcEnd := window(utcNow)
	insertPromo("UTCWINDOW", utcStart, utcEnd)
	if _, err := repos.GetPromoByCode(ctx, f.BranchID, "UTCWINDOW"); err == nil {
		t.Fatalf("UTC-window promo matched at IST time (window %s-%s) — comparison is not branch-local", utcStart, utcEnd)
	}
}

func mustExecPromo(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
