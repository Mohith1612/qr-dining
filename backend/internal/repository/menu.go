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

func (r *Repos) GetMenuItemsByIDs(ctx context.Context, ids []int64) ([]sqlc.MenuItem, error) {
	return r.q.GetMenuItemsByIDs(ctx, ids)
}

func (r *Repos) ListModifiersForItems(ctx context.Context, itemIDs []int64) ([]sqlc.ItemModifier, error) {
	return r.q.ListModifiersForItems(ctx, itemIDs)
}

func (r *Repos) InsertMenuCategory(ctx context.Context, branchID int64, name string, position int16) (sqlc.MenuCategory, error) {
	return r.q.InsertMenuCategory(ctx, sqlc.InsertMenuCategoryParams{
		BranchID: branchID,
		Name:     name,
		Position: position,
	})
}

func (r *Repos) InsertMenuItem(ctx context.Context, p sqlc.InsertMenuItemParams) (sqlc.MenuItem, error) {
	return r.q.InsertMenuItem(ctx, p)
}

func (r *Repos) UpdateMenuItem(ctx context.Context, p sqlc.UpdateMenuItemParams) (sqlc.MenuItem, error) {
	item, err := r.q.UpdateMenuItem(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.MenuItem{}, domain.ErrMenuItemNotFound
	}
	return item, err
}

func (r *Repos) UpdateMenuItemAvailability(ctx context.Context, itemID int64, available bool) error {
	return r.q.UpdateMenuItemAvailability(ctx, sqlc.UpdateMenuItemAvailabilityParams{
		ID:          itemID,
		IsAvailable: available,
	})
}

func (r *Repos) ListFeaturedMenuItems(ctx context.Context, branchID int64) ([]sqlc.MenuItem, error) {
	return r.q.ListFeaturedMenuItems(ctx, branchID)
}

func (r *Repos) UpdateMenuItemFeatured(ctx context.Context, itemID int64, featured bool, sortOrder int16) error {
	return r.q.UpdateMenuItemFeatured(ctx, sqlc.UpdateMenuItemFeaturedParams{
		ID:                itemID,
		IsFeatured:        featured,
		FeaturedSortOrder: sortOrder,
	})
}

func (r *Repos) GetBranchByID(ctx context.Context, id int64) (sqlc.Branch, error) {
	b, err := r.q.GetBranchByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Branch{}, domain.ErrInternalError
	}
	return b, err
}

func (r *Repos) GetBranchByCode(ctx context.Context, branchCode string) (sqlc.Branch, error) {
	b, err := r.q.GetBranchByCode(ctx, branchCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Branch{}, domain.ErrTenantNotFound
	}
	return b, err
}

func (r *Repos) UpdateBranchOrderPrefix(ctx context.Context, branchID int64, prefix string) error {
	return r.q.UpdateBranchOrderPrefix(ctx, sqlc.UpdateBranchOrderPrefixParams{
		ID:          branchID,
		OrderPrefix: prefix,
	})
}

func (r *Repos) ListAllMenuCategoriesForBranch(ctx context.Context, branchID int64) ([]sqlc.MenuCategory, error) {
	return r.q.ListAllMenuCategoriesForBranch(ctx, branchID)
}

func (r *Repos) ListAllMenuItemsForCategory(ctx context.Context, categoryID int64) ([]sqlc.MenuItem, error) {
	return r.q.ListAllMenuItemsForCategory(ctx, categoryID)
}

func (r *Repos) DeleteMenuItem(ctx context.Context, itemID, branchID int64) error {
	return r.q.DeleteMenuItem(ctx, sqlc.DeleteMenuItemParams{ID: itemID, BranchID: branchID})
}

func (r *Repos) CountItemsInCategory(ctx context.Context, categoryID int64) (int64, error) {
	return r.q.CountItemsInCategory(ctx, categoryID)
}

func (r *Repos) DeleteMenuCategory(ctx context.Context, categoryID, branchID int64) error {
	return r.q.DeleteMenuCategory(ctx, sqlc.DeleteMenuCategoryParams{ID: categoryID, BranchID: branchID})
}

func (r *Repos) UpdateMenuCategory(ctx context.Context, p sqlc.UpdateMenuCategoryParams) (sqlc.MenuCategory, error) {
	cat, err := r.q.UpdateMenuCategory(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.MenuCategory{}, domain.ErrCategoryNotFound
	}
	return cat, err
}

func (r *Repos) CreateItemModifier(ctx context.Context, p sqlc.CreateItemModifierParams) (sqlc.ItemModifier, error) {
	return r.q.CreateItemModifier(ctx, p)
}

func (r *Repos) DeleteItemModifier(ctx context.Context, modifierID int64) error {
	return r.q.DeleteItemModifier(ctx, modifierID)
}
