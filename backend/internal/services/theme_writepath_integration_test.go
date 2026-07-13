package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

// TestThemeWritePathDualWrite verifies the unified write-path: a staff theme save
// (mirrored here) updates BOTH the structured tenant_themes row and the legacy
// restaurants.settings_json.theme, and that the read path prefers the structured
// row while still bridging legacy-only restaurants.
func TestThemeWritePathDualWrite(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"tenant_themes",
		"organization_entitlement_overrides",
		"organization_plan_assignments",
	)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()

	repos := repository.New(pool, zerolog.Nop())
	ent := services.NewEntitlementService(repos, services.NewSubscriptionService(repos), observability.NewMetrics())
	theme := services.NewThemeService(repos, ent)

	// ── Mirror BranchHandler.UpdateBranch's theme block: structured first, legacy second.
	preset := "modern-minimal"
	if _, err := theme.SetTheme(ctx, f.OrganizationID, preset, nil, nil); err != nil {
		t.Fatalf("structured SetTheme: %v", err)
	}
	if err := repos.UpdateRestaurantThemeByBranchID(ctx, f.BranchID, preset); err != nil {
		t.Fatalf("legacy theme write: %v", err)
	}

	// Structured tenant_themes row exists with the chosen preset.
	tt, err := repos.GetTenantThemeByRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("GetTenantThemeByRestaurant: %v", err)
	}
	if tt.Preset != preset {
		t.Fatalf("tenant_themes preset = %q, want %q", tt.Preset, preset)
	}

	// Legacy settings_json.theme is also set (back-compat).
	rest, err := repos.GetRestaurantByBranchID(ctx, f.BranchID)
	if err != nil {
		t.Fatalf("GetRestaurantByBranchID: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(rest.SettingsJson, &settings); err != nil {
		t.Fatalf("unmarshal settings_json: %v", err)
	}
	if settings["theme"] != preset {
		t.Fatalf("settings_json.theme = %v, want %q", settings["theme"], preset)
	}

	// Read path prefers the structured row.
	cfg, err := theme.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("GetThemeForRestaurant: %v", err)
	}
	if cfg.Preset != preset {
		t.Fatalf("resolved preset = %q, want %q", cfg.Preset, preset)
	}

	// ── Legacy bridge intact: a restaurant with only settings_json.theme (no
	// tenant_themes row) still resolves to that preset.
	testutil.TruncateTables(t, pool, "tenant_themes")
	if err := repos.UpdateRestaurantThemeByBranchID(ctx, f.BranchID, "warm-cafe"); err != nil {
		t.Fatalf("legacy-only write: %v", err)
	}
	bridged, err := theme.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("bridge resolve: %v", err)
	}
	if bridged.Preset != "warm-cafe" {
		t.Fatalf("bridged preset = %q, want warm-cafe", bridged.Preset)
	}
}
