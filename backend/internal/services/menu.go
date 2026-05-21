package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/jackc/pgx/v5/pgtype"
)

type MenuService struct {
	repos *repository.Repos
	cache *redisPkg.Cache
}

func NewMenuService(repos *repository.Repos, cache *redisPkg.Cache) *MenuService {
	return &MenuService{repos: repos, cache: cache}
}

type MenuModifier struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	PriceDelta float64 `json:"price_delta"`
	IsRequired bool    `json:"is_required"`
}

type MenuItemWithModifiers struct {
	sqlc.MenuItem
	Modifiers []MenuModifier `json:"modifiers"`
}

type MenuCategoryWithItems struct {
	sqlc.MenuCategory
	Items []MenuItemWithModifiers `json:"items"`
}

type FullMenu struct {
	BranchID   int64                   `json:"branch_id"`
	Categories []MenuCategoryWithItems `json:"categories"`
}

const menuCacheTTL = 5 * time.Minute

// GetFullMenu returns the complete menu for a branch, served from Redis cache when available.
func (s *MenuService) GetFullMenu(ctx context.Context, branchID int64) (FullMenu, error) {
	cacheKey := fmt.Sprintf("menu:%d", branchID)

	var menu FullMenu
	if hit, _ := s.cache.Get(ctx, cacheKey, &menu); hit {
		return menu, nil
	}

	menu, err := s.buildMenu(ctx, branchID)
	if err != nil {
		return FullMenu{}, err
	}

	_ = s.cache.Set(ctx, cacheKey, menu, menuCacheTTL)
	return menu, nil
}

func (s *MenuService) GetTableByQRToken(ctx context.Context, token string) (sqlc.Table, error) {
	return s.repos.GetTableByQRToken(ctx, token)
}

// InvalidateMenuCache evicts the cached menu for a branch so the next request rebuilds from DB.
// Call this whenever menu items, categories, or modifiers change.
func (s *MenuService) InvalidateMenuCache(ctx context.Context, branchID int64) {
	_ = s.cache.Invalidate(ctx, fmt.Sprintf("menu:%d", branchID))
}

// CreateCategory creates a new menu category and invalidates the branch menu cache.
func (s *MenuService) CreateCategory(ctx context.Context, branchID int64, name string, position int16, requiredRole sqlc.StaffRole) (sqlc.MenuCategory, error) {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return sqlc.MenuCategory{}, err
	}
	cat, err := s.repos.InsertMenuCategory(ctx, branchID, name, position)
	if err != nil {
		return sqlc.MenuCategory{}, err
	}
	s.InvalidateMenuCache(ctx, branchID)
	return cat, nil
}

type CreateMenuItemParams struct {
	CategoryID  int64
	BranchID    int64
	Name        string
	Description string
	Price       float64
	IsAvailable bool
	Position    int16
}

// CreateItem adds a menu item and invalidates the branch menu cache.
func (s *MenuService) CreateItem(ctx context.Context, p CreateMenuItemParams, requiredRole sqlc.StaffRole) (sqlc.MenuItem, error) {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return sqlc.MenuItem{}, err
	}
	var price pgtype.Numeric
	if err := price.Scan(fmt.Sprintf("%.2f", p.Price)); err != nil {
		return sqlc.MenuItem{}, fmt.Errorf("invalid price: %w", err)
	}
	item, err := s.repos.InsertMenuItem(ctx, sqlc.InsertMenuItemParams{
		CategoryID:  p.CategoryID,
		BranchID:    p.BranchID,
		Name:        p.Name,
		Description: p.Description,
		Price:       price,
		IsAvailable: p.IsAvailable,
		Position:    p.Position,
	})
	if err != nil {
		return sqlc.MenuItem{}, err
	}
	s.InvalidateMenuCache(ctx, p.BranchID)
	return item, nil
}

type UpdateMenuItemParams struct {
	ID          int64
	BranchID    int64
	Name        string
	Description string
	Price       float64
	Position    int16
}

// UpdateItem modifies a menu item and invalidates the branch menu cache.
func (s *MenuService) UpdateItem(ctx context.Context, p UpdateMenuItemParams, requiredRole sqlc.StaffRole) (sqlc.MenuItem, error) {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return sqlc.MenuItem{}, err
	}
	var price pgtype.Numeric
	if err := price.Scan(fmt.Sprintf("%.2f", p.Price)); err != nil {
		return sqlc.MenuItem{}, fmt.Errorf("invalid price: %w", err)
	}
	item, err := s.repos.UpdateMenuItem(ctx, sqlc.UpdateMenuItemParams{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       price,
		Position:    p.Position,
	})
	if err != nil {
		return sqlc.MenuItem{}, err
	}
	s.InvalidateMenuCache(ctx, p.BranchID)
	return item, nil
}

// ToggleAvailability sets is_available on a menu item and invalidates the branch menu cache.
func (s *MenuService) ToggleAvailability(ctx context.Context, itemID, branchID int64, available bool, requiredRole sqlc.StaffRole) error {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return err
	}
	if err := s.repos.UpdateMenuItemAvailability(ctx, itemID, available); err != nil {
		return err
	}
	s.InvalidateMenuCache(ctx, branchID)
	return nil
}

// requireOwnerOrManager returns ErrUnauthorized if the role is not owner or manager.
func requireOwnerOrManager(role sqlc.StaffRole) error {
	if role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager {
		return nil
	}
	return domain.ErrUnauthorized
}

func (s *MenuService) buildMenu(ctx context.Context, branchID int64) (FullMenu, error) {
	categories, err := s.repos.ListMenuCategoriesForBranch(ctx, branchID)
	if err != nil {
		return FullMenu{}, err
	}

	result := FullMenu{BranchID: branchID, Categories: make([]MenuCategoryWithItems, 0, len(categories))}

	for _, cat := range categories {
		if !cat.IsActive {
			continue
		}
		items, err := s.repos.ListMenuItemsForCategory(ctx, cat.ID)
		if err != nil {
			return FullMenu{}, err
		}

		categoryItems := make([]MenuItemWithModifiers, 0, len(items))
		for _, item := range items {
			if !item.IsAvailable {
				continue
			}
			mods, err := s.repos.ListModifiersForItem(ctx, item.ID)
			if err != nil {
				return FullMenu{}, err
			}

			snapMods := make([]MenuModifier, 0, len(mods))
			for _, m := range mods {
				delta, err := m.PriceDelta.Float64Value()
				if err != nil {
					return FullMenu{}, fmt.Errorf("convert modifier price for id %d: %w", m.ID, err)
				}
				snapMods = append(snapMods, MenuModifier{
					ID:         m.ID,
					Name:       m.Name,
					PriceDelta: delta.Float64,
					IsRequired: m.IsRequired,
				})
			}
			categoryItems = append(categoryItems, MenuItemWithModifiers{
				MenuItem:  item,
				Modifiers: snapMods,
			})
		}

		result.Categories = append(result.Categories, MenuCategoryWithItems{
			MenuCategory: cat,
			Items:        categoryItems,
		})
	}

	return result, nil
}
