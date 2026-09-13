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

func (r *Repos) UpdateMenuItemScoped(ctx context.Context, p sqlc.UpdateMenuItemParams, branchID int64) (sqlc.MenuItem, error) {
	row := r.db.QueryRow(ctx, `
UPDATE menu_items
SET name = $3, description = $4, price = $5, position = $6,
    dietary_flags = $7, item_badges = $8, spice_level = $9,
    category_id = COALESCE($10, category_id),
    image_url = COALESCE($11, image_url)
WHERE id = $1 AND branch_id = $2
RETURNING id, category_id, branch_id, name, description, price, is_available, position, is_featured, featured_sort_order, dietary_flags, item_badges, spice_level, image_url
`,
		p.ID,
		branchID,
		p.Name,
		p.Description,
		p.Price,
		p.Position,
		p.DietaryFlags,
		p.ItemBadges,
		p.SpiceLevel,
		p.CategoryID,
		p.ImageUrl,
	)
	var item sqlc.MenuItem
	err := row.Scan(
		&item.ID,
		&item.CategoryID,
		&item.BranchID,
		&item.Name,
		&item.Description,
		&item.Price,
		&item.IsAvailable,
		&item.Position,
		&item.IsFeatured,
		&item.FeaturedSortOrder,
		&item.DietaryFlags,
		&item.ItemBadges,
		&item.SpiceLevel,
		&item.ImageUrl,
	)
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

func (r *Repos) UpdateMenuItemAvailabilityScoped(ctx context.Context, itemID, branchID int64, available bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE menu_items SET is_available = $3 WHERE id = $1 AND branch_id = $2`, itemID, branchID, available)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMenuItemNotFound
	}
	return nil
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

func (r *Repos) UpdateMenuItemFeaturedScoped(ctx context.Context, itemID, branchID int64, featured bool, sortOrder int16) error {
	tag, err := r.db.Exec(ctx, `UPDATE menu_items SET is_featured = $3, featured_sort_order = $4 WHERE id = $1 AND branch_id = $2`, itemID, branchID, featured, sortOrder)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMenuItemNotFound
	}
	return nil
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

func (r *Repos) GetMenuCategoryByID(ctx context.Context, id int64) (sqlc.MenuCategory, error) {
	row := r.db.QueryRow(ctx, `SELECT id, branch_id, name, position, is_active FROM menu_categories WHERE id = $1`, id)
	var cat sqlc.MenuCategory
	err := row.Scan(&cat.ID, &cat.BranchID, &cat.Name, &cat.Position, &cat.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.MenuCategory{}, domain.ErrCategoryNotFound
	}
	return cat, err
}

func (r *Repos) CreateItemModifier(ctx context.Context, p sqlc.CreateItemModifierParams) (sqlc.ItemModifier, error) {
	return r.q.CreateItemModifier(ctx, p)
}

func (r *Repos) CreateItemModifierScoped(ctx context.Context, p sqlc.CreateItemModifierParams, branchID int64) (sqlc.ItemModifier, error) {
	mod, err := r.q.CreateItemModifierScoped(ctx, sqlc.CreateItemModifierScopedParams{
		ItemID:        p.ItemID,
		BranchID:      branchID,
		Name:          p.Name,
		PriceDelta:    p.PriceDelta,
		IsRequired:    p.IsRequired,
		ModifierGroup: p.ModifierGroup,
		SingleSelect:  p.SingleSelect,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ItemModifier{}, domain.ErrMenuItemNotFound
	}
	return mod, err
}

// UpdateItemModifierScoped edits an existing modifier, scoped to the branch.
func (r *Repos) UpdateItemModifierScoped(ctx context.Context, modifierID, branchID int64, p sqlc.CreateItemModifierParams) (sqlc.ItemModifier, error) {
	mod, err := r.q.UpdateItemModifierScoped(ctx, sqlc.UpdateItemModifierScopedParams{
		ID:            modifierID,
		BranchID:      branchID,
		Name:          p.Name,
		PriceDelta:    p.PriceDelta,
		IsRequired:    p.IsRequired,
		ModifierGroup: p.ModifierGroup,
		SingleSelect:  p.SingleSelect,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ItemModifier{}, domain.ErrModifierNotFound
	}
	return mod, err
}

func (r *Repos) DeleteItemModifier(ctx context.Context, modifierID int64) error {
	return r.q.DeleteItemModifier(ctx, modifierID)
}

type ModifierWithItemBranch struct {
	sqlc.ItemModifier
	BranchID int64
}

func (r *Repos) GetModifierWithItemBranch(ctx context.Context, modifierID int64) (ModifierWithItemBranch, error) {
	row := r.db.QueryRow(ctx, `
SELECT im.id, im.item_id, im.name, im.price_delta, im.is_required, im.modifier_group, im.single_select, mi.branch_id
FROM item_modifiers im
JOIN menu_items mi ON mi.id = im.item_id
WHERE im.id = $1
`, modifierID)
	var mod ModifierWithItemBranch
	err := row.Scan(&mod.ID, &mod.ItemID, &mod.Name, &mod.PriceDelta, &mod.IsRequired, &mod.ModifierGroup, &mod.SingleSelect, &mod.BranchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ModifierWithItemBranch{}, domain.ErrModifierNotFound
	}
	return mod, err
}

func (r *Repos) DeleteItemModifierScoped(ctx context.Context, modifierID, branchID int64) error {
	tag, err := r.db.Exec(ctx, `
DELETE FROM item_modifiers
USING menu_items
WHERE item_modifiers.id = $1
  AND item_modifiers.item_id = menu_items.id
  AND menu_items.branch_id = $2
`, modifierID, branchID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrModifierNotFound
	}
	return nil
}
