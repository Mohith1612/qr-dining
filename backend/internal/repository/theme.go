package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) ListThemePresets(ctx context.Context) ([]sqlc.ThemePreset, error) {
	return r.q.ListThemePresets(ctx)
}

// GetThemePreset returns a preset, mapping not-found to domain.ErrThemePresetNotFound.
func (r *Repos) GetThemePreset(ctx context.Context, key string) (sqlc.ThemePreset, error) {
	p, err := r.q.GetThemePreset(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ThemePreset{}, domain.ErrThemePresetNotFound
	}
	return p, err
}

// GetTenantThemeByRestaurant returns the structured theme for a restaurant, or
// pgx.ErrNoRows when none has been set (caller falls back to a default preset).
func (r *Repos) GetTenantThemeByRestaurant(ctx context.Context, restaurantID int64) (sqlc.TenantTheme, error) {
	return r.q.GetTenantThemeByRestaurant(ctx, restaurantID)
}

func (r *Repos) UpsertTenantTheme(ctx context.Context, restaurantID int64, preset string, tokens json.RawMessage, updatedBy *int64) (sqlc.TenantTheme, error) {
	return r.q.UpsertTenantTheme(ctx, sqlc.UpsertTenantThemeParams{
		RestaurantID:            restaurantID,
		Preset:                  preset,
		TokensJson:              tokens,
		UpdatedByPlatformUserID: int8FromPtr(updatedBy),
	})
}
