package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
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
				delta, _ := m.PriceDelta.Float64Value()
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
