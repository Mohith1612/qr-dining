package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) GetRestaurantBySlug(ctx context.Context, slug string) (sqlc.Restaurant, error) {
	row, err := r.q.GetRestaurantBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Restaurant{}, domain.ErrTenantNotFound
	}
	return row, err
}

func (r *Repos) GetRestaurantByBranchID(ctx context.Context, branchID int64) (sqlc.Restaurant, error) {
	row, err := r.q.GetRestaurantByBranchID(ctx, branchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Restaurant{}, domain.ErrTenantNotFound
	}
	return row, err
}

func (r *Repos) GetRestaurantByID(ctx context.Context, id int64) (sqlc.Restaurant, error) {
	row, err := r.q.GetRestaurantByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Restaurant{}, domain.ErrTenantNotFound
	}
	return row, err
}

func (r *Repos) UpdateRestaurantLogoByBranchID(ctx context.Context, branchID int64, logoURL string) error {
	return r.q.UpdateRestaurantLogoByBranchID(ctx, sqlc.UpdateRestaurantLogoByBranchIDParams{
		ID:      branchID,
		LogoUrl: pgtype.Text{String: logoURL, Valid: true},
	})
}

func (r *Repos) UpdateRestaurantThemeByBranchID(ctx context.Context, branchID int64, theme string) error {
	return r.ExecRaw(ctx, `
		UPDATE restaurants r
		SET settings_json = settings_json || jsonb_build_object('theme', $1::text)
		FROM branches b
		WHERE b.id = $2 AND b.restaurant_id = r.id
	`, theme, branchID)
}

func (r *Repos) UpdateRestaurantBillingByBranchID(ctx context.Context, branchID int64,
	taxRate, serviceChargeRate float64, includeTaxInPrice bool) error {
	return r.ExecRaw(ctx, `
		UPDATE restaurants r
		SET settings_json = settings_json || jsonb_build_object(
			'tax_rate', $1::float8,
			'service_charge_rate', $2::float8,
			'include_tax_in_price', $3::bool
		)
		FROM branches b
		WHERE b.id = $4 AND b.restaurant_id = r.id
	`, taxRate, serviceChargeRate, includeTaxInPrice, branchID)
}
