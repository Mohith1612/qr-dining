package services_test

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// newTestFeatureGate builds the entitlement+flag gate against the test DB
// (nil Redis cache — every check resolves fresh).
func newTestFeatureGate(repos *repository.Repos) *services.FeatureGate {
	metrics := observability.NewMetrics()
	subSvc := services.NewSubscriptionService(repos)
	ent := services.NewEntitlementService(repos, subSvc, metrics)
	flags := services.NewFlagService(repos, metrics)
	return services.NewFeatureGate(ent, flags, repos, nil)
}

// enableFeatureGate grants the entitlement to the org (override) and enables
// the flag for the branch (branch override). Catalog rows are ensured with
// ON CONFLICT DO NOTHING because other tests truncate the catalogs.
func enableFeatureGate(t *testing.T, pool *pgxpool.Pool, repos *repository.Repos, orgID, branchID int64, entKey, flagKey string) {
	t.Helper()
	ctx := context.Background()
	mustExecGate(t, pool, `INSERT INTO entitlements (key, kind, description) VALUES ($1, 'capability', 'test') ON CONFLICT (key) DO NOTHING`, entKey)
	mustExecGate(t, pool, `INSERT INTO organization_entitlement_overrides (organization_id, entitlement_key, enabled, reason)
		VALUES ($1, $2, TRUE, 'test') ON CONFLICT (organization_id, entitlement_key) DO UPDATE SET enabled = TRUE`, orgID, entKey)
	mustExecGate(t, pool, `INSERT INTO platform_feature_flags (key, name, description, default_enabled) VALUES ($1, $1, '', FALSE) ON CONFLICT (key) DO NOTHING`, flagKey)
	if _, err := repos.UpsertBranchFlagOverride(ctx, branchID, flagKey, true, "test", nil); err != nil {
		t.Fatalf("branch flag override: %v", err)
	}
}

func mustExecGate(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func seedStaff(t *testing.T, pool *pgxpool.Pool, branchID int64, name, role string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO staff (branch_id, name, role, pin_hash, staff_code)
		 VALUES ($1, $2, $3, 'x', $4) RETURNING id`,
		branchID, name, role, name+"-"+uuid.NewString()[:8]).Scan(&id); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return id
}

func seedEventLog(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, branchID int64, eventType string, staffID int64, payload map[string]any, at time.Time) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO event_log (session_id, branch_id, event_type, actor_type, actor_id, payload, created_at)
		 VALUES ($1, $2, $3, 'staff', $4, $5, $6)`,
		sessionID, branchID, eventType, strconv.FormatInt(staffID, 10), raw, at); err != nil {
		t.Fatalf("seed event_log %s: %v", eventType, err)
	}
}

func TestStaffPerformanceAnalytics(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"organization_entitlement_overrides",
		"platform_flag_branch_overrides",
		"platform_flag_organization_overrides",
		"platform_flag_global_overrides",
		"event_log",
		"staff_sessions",
	)
	f := testutil.SeedFixtures(t, pool)
	t.Cleanup(func() {
		// Leave the shared test DB the way later tests expect it (e.g. the
		// platform-analytics test asserts zero GMV): drop the sessions (and
		// cascaded payments/event_log) plus loyalty rows created here.
		testutil.TruncateTables(t, pool,
			"customer_loyalty_transactions",
			"customer_loyalty_accounts",
			"organization_loyalty_programs",
			"sessions",
			"event_log",
			"staff_sessions",
		)
	})
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	gate := newTestFeatureGate(repos)
	svc := services.NewStaffAnalyticsService(repos, gate, nil)

	// ── Gate off (default state): every surface refuses ──
	if _, err := svc.GetWaiterPerformance(ctx, f.BranchID, "weekly"); !services.IsStaffAnalyticsDisabled(err) {
		t.Fatalf("gate off: err = %v, want ErrStaffAnalyticsDisabled", err)
	}
	if _, err := svc.GetKitchenPerformance(ctx, f.BranchID, "weekly"); !services.IsStaffAnalyticsDisabled(err) {
		t.Fatalf("gate off kitchen: err = %v, want ErrStaffAnalyticsDisabled", err)
	}

	// ── Enable: entitlement override + branch flag override ──
	enableFeatureGate(t, pool, repos, f.OrganizationID, f.BranchID, "analytics.staff_performance", "staff_performance_analytics")

	// ── Seed activity ──
	waiterID := seedStaff(t, pool, f.BranchID, "Wendy", "waiter")
	kitchenID := seedStaff(t, pool, f.BranchID, "Kabir", "kitchen")

	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO sessions (branch_id, table_id, session_token, session_business_date, session_number, status, visit_number)
		VALUES ($1, $2, $3, CURRENT_DATE, 'SA-S-001', 'active', 1) RETURNING id`,
		f.BranchID, f.TableID, "tok-"+uuid.NewString()).Scan(&sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	base := time.Now().UTC().Add(-2 * time.Hour)
	orderID := uuid.NewString()

	// Kitchen: preparing → ready 300s later; waiter serves afterwards.
	seedEventLog(t, pool, sessionID, f.BranchID, "ORDER_STATUS_CHANGED", kitchenID,
		map[string]any{"order_id": orderID, "old_status": "confirmed", "new_status": "preparing"}, base)
	seedEventLog(t, pool, sessionID, f.BranchID, "ORDER_STATUS_CHANGED", kitchenID,
		map[string]any{"order_id": orderID, "old_status": "preparing", "new_status": "ready"}, base.Add(300*time.Second))
	seedEventLog(t, pool, sessionID, f.BranchID, "ORDER_STATUS_CHANGED", waiterID,
		map[string]any{"order_id": orderID, "old_status": "ready", "new_status": "served"}, base.Add(400*time.Second))

	// Assistance: requested at base+10m (payload created_at), acked 120s later.
	ackAt := base.Add(10 * time.Minute)
	seedEventLog(t, pool, sessionID, f.BranchID, "ASSISTANCE_ACKNOWLEDGED", waiterID,
		map[string]any{"id": 1, "session_id": sessionID.String(), "created_at": ackAt.Add(-120 * time.Second).Format(time.RFC3339)}, ackAt)

	// Payment settled by the waiter 60s after initiation.
	mustExecGate(t, pool, `
		INSERT INTO payments (session_id, branch_id, amount, method, status, currency,
		                      payment_reference, payment_business_date, payment_sequence,
		                      initiated_at, settled_by_staff_id, settled_at, completed_at)
		VALUES ($1, $2, 250.00, 'cash', 'completed', 'INR', 'SA-PAY-001', CURRENT_DATE, 1, $3, $4, $5, $5)`,
		sessionID, f.BranchID, base.Add(20*time.Minute), waiterID, base.Add(20*time.Minute+60*time.Second))

	// One waiter login, active ~1h.
	mustExecGate(t, pool, `
		INSERT INTO staff_sessions (staff_id, branch_id, token_hash, token_version, pin_version, created_at, last_seen_at, expires_at)
		VALUES ($1, $2, $3, 1, 1, $4, $5, $6)`,
		waiterID, f.BranchID, "hash-"+uuid.NewString(), base, base.Add(time.Hour), base.Add(24*time.Hour))

	// ── Waiter metrics ──
	waiters, err := svc.GetWaiterPerformance(ctx, f.BranchID, "weekly")
	if err != nil {
		t.Fatalf("GetWaiterPerformance: %v", err)
	}
	var wendy *services.WaiterPerformanceRow
	for i := range waiters {
		if waiters[i].StaffID == waiterID {
			wendy = &waiters[i]
		}
	}
	if wendy == nil {
		t.Fatalf("waiter %d missing from rows: %+v", waiterID, waiters)
	}
	if wendy.OrdersServed != 1 {
		t.Errorf("OrdersServed = %d, want 1", wendy.OrdersServed)
	}
	if wendy.AssistanceAccepted != 1 {
		t.Errorf("AssistanceAccepted = %d, want 1", wendy.AssistanceAccepted)
	}
	if math.Abs(wendy.AvgResponseSeconds-120) > 1 {
		t.Errorf("AvgResponseSeconds = %f, want ~120", wendy.AvgResponseSeconds)
	}
	if wendy.PaymentsSettled != 1 {
		t.Errorf("PaymentsSettled = %d, want 1", wendy.PaymentsSettled)
	}
	if math.Abs(wendy.AvgSettlementSeconds-60) > 1 {
		t.Errorf("AvgSettlementSeconds = %f, want ~60", wendy.AvgSettlementSeconds)
	}
	if wendy.LoginCount != 1 {
		t.Errorf("LoginCount = %d, want 1", wendy.LoginCount)
	}
	if math.Abs(wendy.ActiveSeconds-3600) > 5 {
		t.Errorf("ActiveSeconds = %f, want ~3600", wendy.ActiveSeconds)
	}
	if wendy.SessionsHandled != 1 {
		t.Errorf("SessionsHandled = %d, want 1", wendy.SessionsHandled)
	}

	// ── Kitchen metrics ──
	kitchen, err := svc.GetKitchenPerformance(ctx, f.BranchID, "weekly")
	if err != nil {
		t.Fatalf("GetKitchenPerformance: %v", err)
	}
	var kabir *services.KitchenPerformanceRow
	for i := range kitchen {
		if kitchen[i].StaffID == kitchenID {
			kabir = &kitchen[i]
		}
	}
	if kabir == nil {
		t.Fatalf("kitchen staff %d missing from rows: %+v", kitchenID, kitchen)
	}
	if kabir.OrdersCompleted != 1 {
		t.Errorf("OrdersCompleted = %d, want 1", kabir.OrdersCompleted)
	}
	if math.Abs(kabir.AvgPrepSeconds-300) > 1 {
		t.Errorf("AvgPrepSeconds = %f, want ~300", kabir.AvgPrepSeconds)
	}
	if kabir.PeakOrdersPerHour != 1 {
		t.Errorf("PeakOrdersPerHour = %d, want 1", kabir.PeakOrdersPerHour)
	}

	// ── Manager summary: both staff appear with activity ──
	summary, err := svc.GetStaffSummary(ctx, f.BranchID, "weekly")
	if err != nil {
		t.Fatalf("GetStaffSummary: %v", err)
	}
	seen := map[int64]bool{}
	for _, r := range summary {
		seen[r.StaffID] = true
		if r.Day == "" {
			t.Errorf("summary row missing day: %+v", r)
		}
	}
	if !seen[waiterID] || !seen[kitchenID] {
		t.Errorf("summary missing staff: %+v", summary)
	}

	// ── Platform read bypasses the tenant gate ──
	report, err := svc.GetBranchPerformanceForPlatform(ctx, f.BranchID, "weekly")
	if err != nil {
		t.Fatalf("GetBranchPerformanceForPlatform: %v", err)
	}
	if len(report.Waiters) == 0 || len(report.Kitchen) == 0 {
		t.Errorf("platform report empty: %+v", report)
	}
}
