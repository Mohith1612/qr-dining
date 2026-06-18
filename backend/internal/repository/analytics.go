package repository

import (
	"context"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

func (r *Repos) GetTopOrderedItems(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetTopOrderedItemsRow, error) {
	return r.q.GetTopOrderedItems(ctx, sqlc.GetTopOrderedItemsParams{
		BranchID:    branchID,
		CreatedAt:   from,
		CreatedAt_2: to,
	})
}

func (r *Repos) GetBusyHours(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetBusyHoursRow, error) {
	return r.q.GetBusyHours(ctx, sqlc.GetBusyHoursParams{
		BranchID:    branchID,
		CreatedAt:   from,
		CreatedAt_2: to,
	})
}

func (r *Repos) GetOrderVolume(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetOrderVolumeRow, error) {
	return r.q.GetOrderVolume(ctx, sqlc.GetOrderVolumeParams{
		BranchID:    branchID,
		CreatedAt:   from,
		CreatedAt_2: to,
	})
}

func (r *Repos) GetOrganizationTopOrderedItems(ctx context.Context, organizationID int64, from, to time.Time) ([]sqlc.GetOrganizationTopOrderedItemsRow, error) {
	return r.q.GetOrganizationTopOrderedItems(ctx, sqlc.GetOrganizationTopOrderedItemsParams{
		OrganizationID: organizationID,
		CreatedAt:      from,
		CreatedAt_2:    to,
	})
}

func (r *Repos) GetOrganizationBusyHours(ctx context.Context, organizationID int64, from, to time.Time) ([]sqlc.GetOrganizationBusyHoursRow, error) {
	return r.q.GetOrganizationBusyHours(ctx, sqlc.GetOrganizationBusyHoursParams{
		OrganizationID: organizationID,
		CreatedAt:      from,
		CreatedAt_2:    to,
	})
}

func (r *Repos) GetOrganizationOrderVolume(ctx context.Context, organizationID int64, from, to time.Time) ([]sqlc.GetOrganizationOrderVolumeRow, error) {
	return r.q.GetOrganizationOrderVolume(ctx, sqlc.GetOrganizationOrderVolumeParams{
		OrganizationID: organizationID,
		CreatedAt:      from,
		CreatedAt_2:    to,
	})
}
