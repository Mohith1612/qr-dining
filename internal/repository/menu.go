package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) GetMenuItemByID(ctx context.Context, id int64) (sqlc.MenuItem, error) {
	item, err := r.q.GetMenuItemByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.MenuItem{}, domain.ErrMenuItemNotFound
	}
	return item, err
}

func (r *Repos) ListMenuCategoriesForBranch(ctx context.Context, branchID int64) ([]sqlc.MenuCategory, error) {
	return r.q.ListMenuCategoriesForBranch(ctx, branchID)
}

func (r *Repos) ListMenuItemsForCategory(ctx context.Context, categoryID int64) ([]sqlc.MenuItem, error) {
	return r.q.ListMenuItemsForCategory(ctx, categoryID)
}

func (r *Repos) ListModifiersForItem(ctx context.Context, itemID int64) ([]sqlc.ItemModifier, error) {
	return r.q.ListModifiersForItem(ctx, itemID)
}

func (r *Repos) GetBranchByID(ctx context.Context, id int64) (sqlc.Branch, error) {
	b, err := r.q.GetBranchByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Branch{}, domain.ErrInternalError
	}
	return b, err
}
