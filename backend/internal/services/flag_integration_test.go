package services_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

func TestFeatureFlagResolution(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"platform_flag_branch_overrides",
		"platform_flag_organization_overrides",
		"platform_flag_global_overrides",
		"platform_feature_flags",
	)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	svc := services.NewFlagService(repos, observability.NewMetrics())

	mk := func(key string, def bool) {
		if _, err := repos.CreateFeatureFlag(ctx, key, key, "", def); err != nil {
			t.Fatalf("create flag %s: %v", key, err)
		}
	}
	mk("f.default_on", true)
	mk("f.global", false)
	mk("f.org", false)
	mk("f.branch", false)

	// Overrides: global on f.global; org on f.org (and f.branch); branch on f.branch (false, beats org true).
	if _, err := repos.UpsertGlobalFlagOverride(ctx, "f.global", true, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.UpsertOrganizationFlagOverride(ctx, f.OrganizationID, "f.org", true, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.UpsertOrganizationFlagOverride(ctx, f.OrganizationID, "f.branch", true, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.UpsertBranchFlagOverride(ctx, f.BranchID, "f.branch", false, "", nil); err != nil {
		t.Fatal(err)
	}

	// ── Resolve for branch: precedence branch > org > global > default ──
	byKey := map[string]services.FlagState{}
	states, err := svc.ResolveForBranch(ctx, f.BranchID)
	if err != nil {
		t.Fatalf("resolve branch: %v", err)
	}
	for _, s := range states {
		byKey[s.Key] = s
	}
	assertFlag(t, byKey, "f.default_on", true, "default")
	assertFlag(t, byKey, "f.global", true, "global_override")
	assertFlag(t, byKey, "f.org", true, "org_override")
	assertFlag(t, byKey, "f.branch", false, "branch_override") // branch false beats org true

	// ── Resolve for org: branch level ignored; f.branch falls to org override (true) ──
	orgByKey := map[string]services.FlagState{}
	orgStates, err := svc.ResolveForOrganization(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("resolve org: %v", err)
	}
	for _, s := range orgStates {
		orgByKey[s.Key] = s
	}
	assertFlag(t, orgByKey, "f.branch", true, "org_override")
	assertFlag(t, orgByKey, "f.global", true, "global_override")
	assertFlag(t, orgByKey, "f.default_on", true, "default")
}

func assertFlag(t *testing.T, m map[string]services.FlagState, key string, wantEnabled bool, wantSource string) {
	t.Helper()
	s, ok := m[key]
	if !ok {
		t.Fatalf("flag %q missing from resolution", key)
	}
	if s.Enabled != wantEnabled || s.Source != wantSource {
		t.Errorf("flag %q = {enabled:%v source:%q}, want {enabled:%v source:%q}", key, s.Enabled, s.Source, wantEnabled, wantSource)
	}
}
