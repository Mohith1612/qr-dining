package repository

import (
	"context"
	"encoding/json"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

// GetBranchCollateral returns the stored collateral config for a branch, or
// pgx.ErrNoRows when none has been set (caller falls back to defaults).
func (r *Repos) GetBranchCollateral(ctx context.Context, branchID int64) (sqlc.BranchCollateral, error) {
	return r.q.GetBranchCollateral(ctx, branchID)
}

// UpsertBranchCollateral persists the (already validated, normalized) config JSON.
// updatedBy is the platform user id for platform writes, nil for staff writes.
func (r *Repos) UpsertBranchCollateral(ctx context.Context, branchID int64, config json.RawMessage, updatedBy *int64) (sqlc.BranchCollateral, error) {
	return r.q.UpsertBranchCollateral(ctx, sqlc.UpsertBranchCollateralParams{
		BranchID:                branchID,
		ConfigJson:              config,
		UpdatedByPlatformUserID: int8FromPtr(updatedBy),
	})
}
