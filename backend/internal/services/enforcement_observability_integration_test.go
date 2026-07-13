package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

func TestEnforcementObservability(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"organization_subscriptions",
		"organization_entitlement_overrides",
		"organization_plan_assignments",
		"plan_entitlements",
		"subscription_plans",
		"platform_flag_global_overrides",
		"platform_flag_organization_overrides",
		"platform_flag_branch_overrides",
		"platform_feature_flags",
	)
	f := testutil.SeedFixtures(t, pool) // 1 org, 1 branch, 1 table (T1)
	ctx := context.Background()

	repos := repository.New(pool, zerolog.Nop())
	ent := services.NewEntitlementService(repos, services.NewSubscriptionService(repos), nil)
	obs := services.NewEnforcementObservabilityService(repos, ent, nil) // nil cache → always fresh

	// ── Plan with tight caps, assigned to the fixture org ────────────────────
	var capPlan int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
		 VALUES ('Cap','standard',0,'{}'::jsonb) RETURNING id`).Scan(&capPlan); err != nil {
		t.Fatalf("insert plan: %v", err)
	}
	mustExec(t, pool, `INSERT INTO plan_entitlements (plan_id, entitlement_key, enabled, limit_value) VALUES
		($1,'limit.branches',true,1),
		($1,'limit.tables',true,0),
		($1,'limit.staff',true,0)`, capPlan)
	mustExec(t, pool, `INSERT INTO organization_plan_assignments (organization_id, plan_id, status) VALUES ($1,$2,'active')`, f.OrganizationID, capPlan)

	// ── Entitlement breaches: fixture org has 1 table per branch vs limit 0 ──
	entReport, err := obs.EntitlementObservability(ctx)
	if err != nil {
		t.Fatalf("EntitlementObservability: %v", err)
	}
	foundTableBreach := false
	for _, b := range entReport.Breaches {
		if b.OrganizationID == f.OrganizationID && b.Key == services.EntitlementLimitTables {
			foundTableBreach = true
			if b.Actual != 1 || b.Limit != 0 {
				t.Errorf("table breach actual=%d limit=%d, want 1/0", b.Actual, b.Limit)
			}
		}
	}
	if !foundTableBreach {
		t.Errorf("expected a limit.tables breach for the fixture org; got %+v", entReport.Breaches)
	}

	// ── Subscriptions: suspended + trial-ending + expired across 3 orgs ──────
	// Unique codes so the test is re-runnable against a reused DB.
	suffix := timeSuffix()
	codeB, codeC := "obs-b-"+suffix, "obs-c-"+suffix
	orgB := insertOrg(t, pool, codeB)
	orgC := insertOrg(t, pool, codeC)
	now := time.Now().UTC()
	mustExec(t, pool, `INSERT INTO organization_subscriptions (organization_id, plan_id, status) VALUES ($1,$2,'suspended')`, f.OrganizationID, capPlan)
	mustExec(t, pool, `INSERT INTO organization_subscriptions (organization_id, plan_id, status, trial_ends_at) VALUES ($1,$2,'trial',$3)`, orgB, capPlan, now.Add(3*24*time.Hour))
	mustExec(t, pool, `INSERT INTO organization_subscriptions (organization_id, plan_id, status) VALUES ($1,$2,'expired')`, orgC, capPlan)

	subReport, err := obs.SubscriptionObservability(ctx)
	if err != nil {
		t.Fatalf("SubscriptionObservability: %v", err)
	}
	if subReport.Total != 3 {
		t.Errorf("subscription total = %d, want 3", subReport.Total)
	}
	reasons := map[string]string{} // org -> reason
	for _, a := range subReport.Alerts {
		reasons[a.OrgCode] = a.Reason
	}
	if reasons[codeB] != "trial_ending" {
		t.Errorf("orgB reason = %q, want trial_ending", reasons[codeB])
	}
	if reasons[codeC] != "expired" {
		t.Errorf("orgC reason = %q, want expired", reasons[codeC])
	}

	// ── Flags: one with a global override (in use), one orphaned ─────────────
	mustExec(t, pool, `INSERT INTO platform_feature_flags (key, name, default_enabled) VALUES ('test.flag','Test',false),('orphan.flag','Orphan',false)`)
	mustExec(t, pool, `INSERT INTO platform_flag_global_overrides (flag_key, enabled) VALUES ('test.flag',true)`)

	flagReport, err := obs.FlagObservability(ctx)
	if err != nil {
		t.Fatalf("FlagObservability: %v", err)
	}
	inUse := false
	for _, s := range flagReport.InUse {
		if s.Key == "test.flag" && s.Global == 1 && s.Total == 1 {
			inUse = true
		}
	}
	if !inUse {
		t.Errorf("expected test.flag in use with 1 global override; got %+v", flagReport.InUse)
	}
	orphaned := false
	for _, k := range flagReport.Orphaned {
		if k == "orphan.flag" {
			orphaned = true
		}
	}
	if !orphaned {
		t.Errorf("expected orphan.flag orphaned; got %v", flagReport.Orphaned)
	}
}

func insertOrg(t *testing.T, pool *pgxpool.Pool, code string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO organizations (code, name, settings_json) VALUES ($1,$1,'{}') RETURNING id`, code).Scan(&id); err != nil {
		t.Fatalf("insert org %s: %v", code, err)
	}
	return id
}
