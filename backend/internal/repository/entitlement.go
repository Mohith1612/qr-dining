package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ListEntitlementCatalog returns the full capability/limit catalog.
func (r *Repos) ListEntitlementCatalog(ctx context.Context) ([]sqlc.Entitlement, error) {
	return r.q.ListEntitlementCatalog(ctx)
}

// GetOrganizationPlanAssignment returns the org's plan assignment, or
// domain.ErrOrgPlanNotAssigned when none exists (the resolver then bridges from
// the restaurant subscription).
func (r *Repos) GetOrganizationPlanAssignment(ctx context.Context, orgID int64) (sqlc.OrganizationPlanAssignment, error) {
	a, err := r.q.GetOrganizationPlanAssignment(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.OrganizationPlanAssignment{}, domain.ErrOrgPlanNotAssigned
	}
	return a, err
}

// ListPlanEntitlements returns the entitlement defaults for a plan.
func (r *Repos) ListPlanEntitlements(ctx context.Context, planID int64) ([]sqlc.PlanEntitlement, error) {
	return r.q.ListPlanEntitlements(ctx, planID)
}

// ListOrganizationEntitlementOverrides returns org-level overrides.
func (r *Repos) ListOrganizationEntitlementOverrides(ctx context.Context, orgID int64) ([]sqlc.OrganizationEntitlementOverride, error) {
	return r.q.ListOrganizationEntitlementOverrides(ctx, orgID)
}

// GetPlanByID returns a subscription plan, mapping not-found to domain.ErrPlanNotFound.
func (r *Repos) GetPlanByID(ctx context.Context, id int64) (sqlc.SubscriptionPlan, error) {
	p, err := r.q.GetPlanByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SubscriptionPlan{}, domain.ErrPlanNotFound
	}
	return p, err
}
