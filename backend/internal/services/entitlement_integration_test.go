package services_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func TestEntitlementResolution(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"organization_entitlement_overrides",
		"organization_plan_assignments",
		"plan_entitlements",
		"restaurant_subscriptions",
		"subscription_plans",
	)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()

	repos := repository.New(pool, zerolog.Nop())
	subSvc := services.NewSubscriptionService(repos)
	metrics := observability.NewMetrics()
	svc := services.NewEntitlementService(repos, subSvc, metrics)

	// Standard plan + restaurant subscription, but NO org plan assignment yet.
	var standardID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
		 VALUES ('Standard','standard',29,'{"max_branches":3,"max_tables":-1,"analytics":true,"multi_branch":false}'::jsonb)
		 RETURNING id`).Scan(&standardID); err != nil {
		t.Fatalf("insert standard plan: %v", err)
	}
	mustExec(t, pool,
		`INSERT INTO restaurant_subscriptions (restaurant_id, plan_id, status) VALUES ($1,$2,'active')`,
		f.RestaurantID, standardID)

	// ── Case A: backward-compat bridge from the restaurant subscription ──────
	eff, err := svc.ResolveForOrganization(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("resolve (bridge): %v", err)
	}
	if eff.Source != "restaurant_bridge" {
		t.Fatalf("source = %q, want restaurant_bridge", eff.Source)
	}
	if eff.PlanTier != "standard" {
		t.Fatalf("tier = %q, want standard", eff.PlanTier)
	}
	if !eff.Capabilities["analytics.basic"] {
		t.Error("expected analytics.basic granted via bridge")
	}
	if eff.Capabilities["multi_branch"] {
		t.Error("multi_branch should be false on standard")
	}
	if eff.Limits["limit.branches"] != 3 {
		t.Errorf("limit.branches = %d, want 3", eff.Limits["limit.branches"])
	}
	if eff.Limits["limit.tables"] != -1 {
		t.Errorf("limit.tables = %d, want -1 (unlimited)", eff.Limits["limit.tables"])
	}
	// Bridge must agree with the legacy gating source.
	feat, _ := subSvc.GetPlanFeatures(ctx, f.RestaurantID)
	if feat.Analytics != eff.Capabilities["analytics.basic"] {
		t.Error("bridge analytics disagrees with GetPlanFeatures")
	}

	// ── Case B: org plan assignment with explicit plan_entitlements ──────────
	var premiumID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
		 VALUES ('Premium','premium',79,'{}'::jsonb) RETURNING id`).Scan(&premiumID); err != nil {
		t.Fatalf("insert premium plan: %v", err)
	}
	mustExec(t, pool, `INSERT INTO plan_entitlements (plan_id, entitlement_key, enabled, limit_value) VALUES
		($1,'analytics.basic',true,NULL),
		($1,'analytics.advanced',true,NULL),
		($1,'multi_branch',true,NULL),
		($1,'limit.branches',true,NULL)`, premiumID)
	mustExec(t, pool,
		`INSERT INTO organization_plan_assignments (organization_id, plan_id, status) VALUES ($1,$2,'active')`,
		f.OrganizationID, premiumID)

	eff, err = svc.ResolveForOrganization(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("resolve (assignment): %v", err)
	}
	if eff.Source != "org_assignment" {
		t.Fatalf("source = %q, want org_assignment", eff.Source)
	}
	if eff.PlanTier != "premium" {
		t.Fatalf("tier = %q, want premium", eff.PlanTier)
	}
	if !eff.Capabilities["analytics.advanced"] || !eff.Capabilities["multi_branch"] {
		t.Error("expected analytics.advanced and multi_branch via plan assignment")
	}
	if eff.Limits["limit.branches"] != -1 {
		t.Errorf("limit.branches = %d, want -1 (NULL plan limit = unlimited)", eff.Limits["limit.branches"])
	}
	if eff.Limits["limit.tables"] != -1 {
		t.Errorf("limit.tables = %d, want -1 (absent from plan = unlimited)", eff.Limits["limit.tables"])
	}

	// ── Case C: org overrides take precedence over the plan ──────────────────
	mustExec(t, pool, `INSERT INTO organization_entitlement_overrides
		(organization_id, entitlement_key, enabled, limit_value, reason) VALUES
		($1,'multi_branch',false,NULL,'test'),
		($1,'limit.branches',NULL,2,'test')`, f.OrganizationID)

	eff, err = svc.ResolveForOrganization(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("resolve (override): %v", err)
	}
	if eff.Capabilities["multi_branch"] {
		t.Error("override should disable multi_branch")
	}
	if eff.Limits["limit.branches"] != 2 {
		t.Errorf("override limit.branches = %d, want 2", eff.Limits["limit.branches"])
	}
	if !eff.Capabilities["analytics.advanced"] {
		t.Error("non-overridden capability should remain granted")
	}

	// Shadow capability check should resolve and not error.
	ok, err := svc.HasCapability(ctx, f.OrganizationID, "analytics.advanced")
	if err != nil || !ok {
		t.Fatalf("HasCapability(analytics.advanced) = %v, err = %v", ok, err)
	}
}
