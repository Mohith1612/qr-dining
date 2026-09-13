package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

// GetEntitlement returns a single catalog entry, mapping not-found to
// domain.ErrEntitlementNotFound.
func (r *Repos) GetEntitlement(ctx context.Context, key string) (sqlc.Entitlement, error) {
	e, err := r.q.GetEntitlement(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Entitlement{}, domain.ErrEntitlementNotFound
	}
	return e, err
}

// CreatePlan inserts a new subscription plan. tier must be a valid plan_tier enum value.
func (r *Repos) CreatePlan(ctx context.Context, name, tier, priceMonthly string, features json.RawMessage) (sqlc.SubscriptionPlan, error) {
	var price pgtype.Numeric
	if err := price.Scan(priceMonthly); err != nil {
		return sqlc.SubscriptionPlan{}, err
	}
	return r.q.CreatePlan(ctx, sqlc.CreatePlanParams{
		Name:    name,
		Column2: sqlc.PlanTier(tier),
		Column3: price,
		Column4: features,
	})
}

// UpdatePlan updates a plan's name, price, and features.
func (r *Repos) UpdatePlan(ctx context.Context, id int64, name, priceMonthly string, features json.RawMessage) (sqlc.SubscriptionPlan, error) {
	var price pgtype.Numeric
	if err := price.Scan(priceMonthly); err != nil {
		return sqlc.SubscriptionPlan{}, err
	}
	return r.q.UpdatePlan(ctx, sqlc.UpdatePlanParams{
		ID:      id,
		Name:    name,
		Column3: price,
		Column4: features,
	})
}

// UpsertPlanEntitlement sets a single plan->entitlement default.
func (r *Repos) UpsertPlanEntitlement(ctx context.Context, planID int64, key string, enabled bool, limit *int64) (sqlc.PlanEntitlement, error) {
	return r.q.UpsertPlanEntitlement(ctx, sqlc.UpsertPlanEntitlementParams{
		PlanID:         planID,
		EntitlementKey: key,
		Enabled:        enabled,
		LimitValue:     int8FromPtr(limit),
	})
}

// DeletePlanEntitlements removes all entitlement defaults for a plan (used for replace).
func (r *Repos) DeletePlanEntitlements(ctx context.Context, planID int64) error {
	return r.q.DeletePlanEntitlements(ctx, planID)
}

// UpsertOrganizationPlanAssignment assigns (or reassigns) a plan to an organization.
func (r *Repos) UpsertOrganizationPlanAssignment(ctx context.Context, orgID, planID int64, status string, assignedBy *int64) (sqlc.OrganizationPlanAssignment, error) {
	return r.q.UpsertOrganizationPlanAssignment(ctx, sqlc.UpsertOrganizationPlanAssignmentParams{
		OrganizationID:           orgID,
		PlanID:                   planID,
		Status:                   status,
		AssignedByPlatformUserID: int8FromPtr(assignedBy),
	})
}

// UpsertOrganizationEntitlementOverride sets an org-level override for one entitlement.
func (r *Repos) UpsertOrganizationEntitlementOverride(ctx context.Context, orgID int64, key string, enabled *bool, limit *int64, reason string, createdBy *int64) (sqlc.OrganizationEntitlementOverride, error) {
	return r.q.UpsertOrganizationEntitlementOverride(ctx, sqlc.UpsertOrganizationEntitlementOverrideParams{
		OrganizationID:          orgID,
		EntitlementKey:          key,
		Enabled:                 boolToPg(enabled),
		LimitValue:              int8FromPtr(limit),
		Reason:                  reason,
		CreatedByPlatformUserID: int8FromPtr(createdBy),
	})
}

// DeleteOrganizationEntitlementOverride clears an org-level override.
func (r *Repos) DeleteOrganizationEntitlementOverride(ctx context.Context, orgID int64, key string) error {
	return r.q.DeleteOrganizationEntitlementOverride(ctx, sqlc.DeleteOrganizationEntitlementOverrideParams{
		OrganizationID: orgID,
		EntitlementKey: key,
	})
}

func int8FromPtr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func boolToPg(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}
