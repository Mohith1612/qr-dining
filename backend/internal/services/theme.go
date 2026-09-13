package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5"
)

// defaultThemePreset matches the frontend default (frontend/providers/ThemeProvider.tsx).
const defaultThemePreset = "serene"

// allowedThemeTokens is the curated allowlist of overridable design-token keys.
// These map 1:1 to CSS custom properties consumed by frontend/styles/themes.css.
// Arbitrary keys are rejected; values must be hex colors (no CSS strings).
var allowedThemeTokens = map[string]bool{
	"accent":        true,
	"accent-strong": true,
	"accent-soft":   true,
	"accent-ink":    true,
	"bg-base":       true,
	"bg-elev-1":     true,
	"bg-elev-2":     true,
	"ink-1":         true,
	"ink-2":         true,
	"ink-3":         true,
	"ok":            true,
	"warn":          true,
	"alert":         true,
	"info":          true,
}

// hexColorRe matches #RGB? no — only #RRGGBB or #RRGGBBAA (uppercase or lowercase).
var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)

// ThemeService manages structured tenant theme config. Custom tokens require the
// org's custom.theme entitlement; presets are always allowed. No arbitrary CSS.
type ThemeService struct {
	repos *repository.Repos
	ent   *EntitlementService
}

func NewThemeService(repos *repository.Repos, ent *EntitlementService) *ThemeService {
	return &ThemeService{repos: repos, ent: ent}
}

// ThemeConfig is the resolved structured theme for a tenant.
type ThemeConfig struct {
	Preset string            `json:"preset"`
	Tokens map[string]string `json:"tokens"`
}

// AllowedThemeTokenKeys returns the sorted allowlist (for API discovery responses).
func AllowedThemeTokenKeys() []string {
	keys := make([]string, 0, len(allowedThemeTokens))
	for k := range allowedThemeTokens {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validateThemeTokens rejects unknown keys and non-hex values. Pure (unit-testable).
func validateThemeTokens(tokens map[string]string) error {
	for k, v := range tokens {
		if !allowedThemeTokens[k] {
			return fmt.Errorf("%w: unknown token key %q", domain.ErrInvalidThemeToken, k)
		}
		if !hexColorRe.MatchString(v) {
			return fmt.Errorf("%w: %q must be a hex color (#RRGGBB or #RRGGBBAA)", domain.ErrInvalidThemeToken, k)
		}
	}
	return nil
}

// SetTheme validates the preset and (if any custom tokens are supplied) requires the
// org's custom.theme entitlement, then persists the structured theme for the org's
// restaurant. orgID is the platform-facing handle; org<->restaurant is 1:1 today.
func (s *ThemeService) SetTheme(ctx context.Context, orgID int64, preset string, tokens map[string]string, actorID *int64) (ThemeConfig, error) {
	if _, err := s.repos.GetThemePreset(ctx, preset); err != nil {
		return ThemeConfig{}, err // domain.ErrThemePresetNotFound
	}
	if len(tokens) > 0 {
		allowed, err := s.ent.HasCapability(ctx, orgID, EntitlementCustomTheme)
		if err != nil {
			return ThemeConfig{}, err
		}
		if !allowed {
			return ThemeConfig{}, domain.ErrCustomThemeNotEntitled
		}
		if err := validateThemeTokens(tokens); err != nil {
			return ThemeConfig{}, err
		}
	}
	restaurant, err := s.repos.GetRestaurantByOrganizationID(ctx, orgID)
	if err != nil {
		return ThemeConfig{}, err
	}
	raw, err := json.Marshal(tokens)
	if err != nil {
		return ThemeConfig{}, err
	}
	if tokens == nil {
		raw = json.RawMessage(`{}`)
	}
	row, err := s.repos.UpsertTenantTheme(ctx, restaurant.ID, preset, raw, actorID)
	if err != nil {
		return ThemeConfig{}, err
	}
	return decodeTheme(row.Preset, row.TokensJson), nil
}

// GetThemeForRestaurant returns the structured theme for a restaurant. When no
// tenant_themes row exists it bridges to the legacy restaurants.settings_json.theme
// preset (if a valid catalog preset), else the default — so restaurants configured
// only via the legacy path never lose their theming when the new system is adopted.
func (s *ThemeService) GetThemeForRestaurant(ctx context.Context, restaurantID int64) (ThemeConfig, error) {
	row, err := s.repos.GetTenantThemeByRestaurant(ctx, restaurantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ThemeConfig{Preset: s.legacyPresetFallback(ctx, restaurantID), Tokens: map[string]string{}}, nil
		}
		return ThemeConfig{}, err
	}
	return decodeTheme(row.Preset, row.TokensJson), nil
}

// legacyPresetFallback reads the legacy settings_json.theme for a restaurant and
// returns it when it is a known preset; otherwise the default. Best-effort: any
// lookup/decode failure degrades to the default preset.
func (s *ThemeService) legacyPresetFallback(ctx context.Context, restaurantID int64) string {
	restaurant, err := s.repos.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		return defaultThemePreset
	}
	var settings struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(restaurant.SettingsJson, &settings); err != nil || settings.Theme == "" {
		return defaultThemePreset
	}
	if _, err := s.repos.GetThemePreset(ctx, settings.Theme); err != nil {
		return defaultThemePreset // not a known preset
	}
	return settings.Theme
}

// GetThemeForBranch resolves the branch's restaurant then returns its theme.
func (s *ThemeService) GetThemeForBranch(ctx context.Context, branchID int64) (ThemeConfig, error) {
	restaurant, err := s.repos.GetRestaurantByBranchID(ctx, branchID)
	if err != nil {
		return ThemeConfig{}, err
	}
	return s.GetThemeForRestaurant(ctx, restaurant.ID)
}

func decodeTheme(preset string, raw json.RawMessage) ThemeConfig {
	tokens := map[string]string{}
	_ = json.Unmarshal(raw, &tokens)
	return ThemeConfig{Preset: preset, Tokens: tokens}
}
