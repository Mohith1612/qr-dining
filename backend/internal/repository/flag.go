package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ── Catalog ──────────────────────────────────────────────────────────────────

func (r *Repos) ListFeatureFlags(ctx context.Context) ([]sqlc.PlatformFeatureFlag, error) {
	return r.q.ListFeatureFlags(ctx)
}

func (r *Repos) GetFeatureFlag(ctx context.Context, key string) (sqlc.PlatformFeatureFlag, error) {
	f, err := r.q.GetFeatureFlag(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlatformFeatureFlag{}, domain.ErrFlagNotFound
	}
	return f, err
}

func (r *Repos) CreateFeatureFlag(ctx context.Context, key, name, description string, defaultEnabled bool) (sqlc.PlatformFeatureFlag, error) {
	return r.q.CreateFeatureFlag(ctx, sqlc.CreateFeatureFlagParams{
		Key:            key,
		Name:           name,
		Description:    description,
		DefaultEnabled: defaultEnabled,
	})
}

func (r *Repos) UpdateFeatureFlag(ctx context.Context, key, name, description string, defaultEnabled bool) (sqlc.PlatformFeatureFlag, error) {
	f, err := r.q.UpdateFeatureFlag(ctx, sqlc.UpdateFeatureFlagParams{
		Key:            key,
		Name:           name,
		Description:    description,
		DefaultEnabled: defaultEnabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlatformFeatureFlag{}, domain.ErrFlagNotFound
	}
	return f, err
}

// ── Global overrides ─────────────────────────────────────────────────────────

func (r *Repos) ListGlobalFlagOverrides(ctx context.Context) ([]sqlc.PlatformFlagGlobalOverride, error) {
	return r.q.ListGlobalFlagOverrides(ctx)
}

func (r *Repos) UpsertGlobalFlagOverride(ctx context.Context, flagKey string, enabled bool, updatedBy *int64) (sqlc.PlatformFlagGlobalOverride, error) {
	return r.q.UpsertGlobalFlagOverride(ctx, sqlc.UpsertGlobalFlagOverrideParams{
		FlagKey:                 flagKey,
		Enabled:                 enabled,
		UpdatedByPlatformUserID: int8FromPtr(updatedBy),
	})
}

func (r *Repos) DeleteGlobalFlagOverride(ctx context.Context, flagKey string) error {
	return r.q.DeleteGlobalFlagOverride(ctx, flagKey)
}

// ── Organization overrides ───────────────────────────────────────────────────

func (r *Repos) ListOrganizationFlagOverrides(ctx context.Context, orgID int64) ([]sqlc.PlatformFlagOrganizationOverride, error) {
	return r.q.ListOrganizationFlagOverrides(ctx, orgID)
}

func (r *Repos) UpsertOrganizationFlagOverride(ctx context.Context, orgID int64, flagKey string, enabled bool, reason string, updatedBy *int64) (sqlc.PlatformFlagOrganizationOverride, error) {
	return r.q.UpsertOrganizationFlagOverride(ctx, sqlc.UpsertOrganizationFlagOverrideParams{
		OrganizationID:          orgID,
		FlagKey:                 flagKey,
		Enabled:                 enabled,
		Reason:                  reason,
		UpdatedByPlatformUserID: int8FromPtr(updatedBy),
	})
}

func (r *Repos) DeleteOrganizationFlagOverride(ctx context.Context, orgID int64, flagKey string) error {
	return r.q.DeleteOrganizationFlagOverride(ctx, sqlc.DeleteOrganizationFlagOverrideParams{
		OrganizationID: orgID,
		FlagKey:        flagKey,
	})
}

// ── Branch overrides ─────────────────────────────────────────────────────────

func (r *Repos) ListBranchFlagOverrides(ctx context.Context, branchID int64) ([]sqlc.PlatformFlagBranchOverride, error) {
	return r.q.ListBranchFlagOverrides(ctx, branchID)
}

func (r *Repos) UpsertBranchFlagOverride(ctx context.Context, branchID int64, flagKey string, enabled bool, reason string, updatedBy *int64) (sqlc.PlatformFlagBranchOverride, error) {
	return r.q.UpsertBranchFlagOverride(ctx, sqlc.UpsertBranchFlagOverrideParams{
		BranchID:                branchID,
		FlagKey:                 flagKey,
		Enabled:                 enabled,
		Reason:                  reason,
		UpdatedByPlatformUserID: int8FromPtr(updatedBy),
	})
}

func (r *Repos) DeleteBranchFlagOverride(ctx context.Context, branchID int64, flagKey string) error {
	return r.q.DeleteBranchFlagOverride(ctx, sqlc.DeleteBranchFlagOverrideParams{
		BranchID: branchID,
		FlagKey:  flagKey,
	})
}
