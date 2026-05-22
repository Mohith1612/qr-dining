package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
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
