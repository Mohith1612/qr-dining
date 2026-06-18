package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) GetOrganizationByID(ctx context.Context, id int64) (sqlc.Organization, error) {
	org, err := r.q.GetOrganizationByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Organization{}, domain.ErrTenantNotFound
	}
	return org, err
}

func (r *Repos) GetOrganizationByCode(ctx context.Context, code string) (sqlc.Organization, error) {
	org, err := r.q.GetOrganizationByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Organization{}, domain.ErrTenantNotFound
	}
	return org, err
}

func (r *Repos) GetOrganizationByBranchID(ctx context.Context, branchID int64) (sqlc.Organization, error) {
	org, err := r.q.GetOrganizationByBranchID(ctx, branchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Organization{}, domain.ErrTenantNotFound
	}
	return org, err
}

func (r *Repos) GetOrganizationByRestaurantID(ctx context.Context, restaurantID int64) (sqlc.Organization, error) {
	org, err := r.q.GetOrganizationByRestaurantID(ctx, restaurantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Organization{}, domain.ErrTenantNotFound
	}
	return org, err
}

func (r *Repos) GetRestaurantByOrganizationID(ctx context.Context, organizationID int64) (sqlc.Restaurant, error) {
	restaurant, err := r.q.GetRestaurantByOrganizationID(ctx, organizationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Restaurant{}, domain.ErrTenantNotFound
	}
	return restaurant, err
}

func (r *Repos) GetOrganizationMembershipForStaff(ctx context.Context, organizationID, staffID int64) (sqlc.OrganizationMember, error) {
	member, err := r.q.GetOrganizationMembershipForStaff(ctx, sqlc.GetOrganizationMembershipForStaffParams{
		OrganizationID: organizationID,
		StaffID:        staffID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.OrganizationMember{}, domain.ErrForbidden
	}
	return member, err
}

func (r *Repos) ListBranchesForOrganization(ctx context.Context, organizationID int64) ([]sqlc.Branch, error) {
	return r.q.ListBranchesForOrganization(ctx, organizationID)
}

func (r *Repos) UpdateOrganizationSettings(ctx context.Context, p sqlc.UpdateOrganizationSettingsParams) (sqlc.Organization, error) {
	org, err := r.q.UpdateOrganizationSettings(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Organization{}, domain.ErrTenantNotFound
	}
	return org, err
}
