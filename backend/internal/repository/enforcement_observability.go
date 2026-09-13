package repository

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

// Read-only aggregates for enforcement observability (observe, never enforce).

func (r *Repos) ListSubscriptionsForObservability(ctx context.Context) ([]sqlc.ListSubscriptionsForObservabilityRow, error) {
	return r.q.ListSubscriptionsForObservability(ctx)
}

func (r *Repos) ListOrgResourceCounts(ctx context.Context) ([]sqlc.ListOrgResourceCountsRow, error) {
	return r.q.ListOrgResourceCounts(ctx)
}

func (r *Repos) ListFlagOverrideCounts(ctx context.Context) ([]sqlc.ListFlagOverrideCountsRow, error) {
	return r.q.ListFlagOverrideCounts(ctx)
}

func (r *Repos) ListFlagCatalogKeys(ctx context.Context) ([]sqlc.ListFlagCatalogKeysRow, error) {
	return r.q.ListFlagCatalogKeys(ctx)
}
