package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

func TestThemeLegacyBridge(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	subSvc := services.NewSubscriptionService(repos)
	entSvc := services.NewEntitlementService(repos, subSvc, observability.NewMetrics())
	svc := services.NewThemeService(repos, entSvc)

	// Restaurant configured ONLY via the legacy settings_json.theme (no tenant_themes row).
	if _, err := pool.Exec(ctx,
		`UPDATE restaurants SET settings_json = '{"theme":"modern-minimal"}'::jsonb WHERE id = $1`,
		f.RestaurantID); err != nil {
		t.Fatalf("set legacy theme: %v", err)
	}

	got, err := svc.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Preset != "modern-minimal" {
		t.Fatalf("bridge preset = %q, want modern-minimal (legacy settings_json.theme)", got.Preset)
	}
	if len(got.Tokens) != 0 {
		t.Fatalf("bridge tokens = %v, want empty", got.Tokens)
	}

	// Branch resolution bridges the same way.
	byBranch, err := svc.GetThemeForBranch(ctx, f.BranchID)
	if err != nil {
		t.Fatalf("resolve by branch: %v", err)
	}
	if byBranch.Preset != "modern-minimal" {
		t.Fatalf("branch bridge preset = %q, want modern-minimal", byBranch.Preset)
	}

	// An unknown/invalid legacy value degrades to the default preset.
	if _, err := pool.Exec(ctx,
		`UPDATE restaurants SET settings_json = '{"theme":"not-a-preset"}'::jsonb WHERE id = $1`,
		f.RestaurantID); err != nil {
		t.Fatalf("set invalid legacy theme: %v", err)
	}
	got, err = svc.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("resolve invalid: %v", err)
	}
	if got.Preset != "dark-luxury" {
		t.Fatalf("invalid legacy preset = %q, want dark-luxury default", got.Preset)
	}

	// A structured tenant_themes row takes precedence over legacy.
	if _, err := svc.SetTheme(ctx, f.OrganizationID, "warm-cafe", nil, nil); err != nil {
		t.Fatalf("set structured: %v", err)
	}
	got, err = svc.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("resolve structured: %v", err)
	}
	if got.Preset != "warm-cafe" {
		t.Fatalf("structured preset = %q, want warm-cafe (overrides legacy)", got.Preset)
	}
}

func TestThemeSetGetAndEntitlementGate(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	subSvc := services.NewSubscriptionService(repos)
	entSvc := services.NewEntitlementService(repos, subSvc, observability.NewMetrics())
	svc := services.NewThemeService(repos, entSvc)

	// Default (no row) -> default preset, empty tokens.
	def, err := svc.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if def.Preset != "dark-luxury" || len(def.Tokens) != 0 {
		t.Fatalf("default = %+v, want dark-luxury/empty", def)
	}

	// Preset-only set requires no entitlement.
	if _, err := svc.SetTheme(ctx, f.OrganizationID, "modern-minimal", nil, nil); err != nil {
		t.Fatalf("set preset-only: %v", err)
	}
	got, err := svc.GetThemeForRestaurant(ctx, f.RestaurantID)
	if err != nil {
		t.Fatalf("get after preset set: %v", err)
	}
	if got.Preset != "modern-minimal" {
		t.Fatalf("preset = %q, want modern-minimal", got.Preset)
	}

	// Unknown preset rejected.
	if _, err := svc.SetTheme(ctx, f.OrganizationID, "no-such-preset", nil, nil); !errors.Is(err, domain.ErrThemePresetNotFound) {
		t.Fatalf("unknown preset err = %v, want ErrThemePresetNotFound", err)
	}

	// Custom tokens WITHOUT entitlement -> rejected.
	tokens := map[string]string{"accent": "#C9A876"}
	if _, err := svc.SetTheme(ctx, f.OrganizationID, "dark-luxury", tokens, nil); !errors.Is(err, domain.ErrCustomThemeNotEntitled) {
		t.Fatalf("custom-without-entitlement err = %v, want ErrCustomThemeNotEntitled", err)
	}

	// Grant custom.theme via an org entitlement override, then custom tokens succeed.
	if _, err := pool.Exec(ctx,
		`INSERT INTO organization_entitlement_overrides (organization_id, entitlement_key, enabled, reason)
		 VALUES ($1, 'custom.theme', TRUE, 'test')`, f.OrganizationID); err != nil {
		t.Fatalf("grant custom.theme: %v", err)
	}
	saved, err := svc.SetTheme(ctx, f.OrganizationID, "dark-luxury", tokens, nil)
	if err != nil {
		t.Fatalf("set custom after grant: %v", err)
	}
	if saved.Tokens["accent"] != "#C9A876" {
		t.Fatalf("custom token not saved: %+v", saved.Tokens)
	}

	// Invalid token value rejected even when entitled.
	if _, err := svc.SetTheme(ctx, f.OrganizationID, "dark-luxury", map[string]string{"accent": "red"}, nil); !errors.Is(err, domain.ErrInvalidThemeToken) {
		t.Fatalf("invalid token err = %v, want ErrInvalidThemeToken", err)
	}

	// Branch resolution returns the restaurant's theme.
	byBranch, err := svc.GetThemeForBranch(ctx, f.BranchID)
	if err != nil {
		t.Fatalf("get by branch: %v", err)
	}
	if byBranch.Preset != "dark-luxury" || byBranch.Tokens["accent"] != "#C9A876" {
		t.Fatalf("branch theme = %+v, want dark-luxury + accent token", byBranch)
	}
}
