package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

type MenuService struct {
	repos     *repository.Repos
	cache     *redisPkg.Cache
	publisher *events.Publisher
}

func NewMenuService(repos *repository.Repos, cache *redisPkg.Cache, publisher *events.Publisher) *MenuService {
	return &MenuService{repos: repos, cache: cache, publisher: publisher}
}

type MenuModifier struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	PriceDelta    float64 `json:"price_delta"`
	IsRequired    bool    `json:"is_required"`
	ModifierGroup string  `json:"modifier_group"`
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
	Featured   []MenuItemWithModifiers `json:"featured"`
	Categories []MenuCategoryWithItems `json:"categories"`
}

const menuCacheTTL = 5 * time.Minute
const menuCacheVersion = 1

// GetFullMenu returns the complete menu for a branch, served from Redis cache when available.
func (s *MenuService) GetFullMenu(ctx context.Context, branchID int64) (FullMenu, error) {
	cacheKey := s.menuCacheKey(ctx, branchID)

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

func (s *MenuService) GetMenuItem(ctx context.Context, itemID int64) (sqlc.MenuItem, error) {
	return s.repos.GetMenuItemByID(ctx, itemID)
}

// TableQRResponse is returned by GetTableByQR and includes the active session ID when the table is occupied.
type TableQRResponse struct {
	ID          int64  `json:"id"`
	BranchID    int64  `json:"branch_id"`
	Identifier  string `json:"identifier"`
	SessionID   string `json:"session_id,omitempty"`
	BranchTheme string `json:"branch_theme"`
}

// GetTableWithActiveSession resolves a QR token and includes the active session ID if the table is occupied.
func (s *MenuService) GetTableWithActiveSession(ctx context.Context, token string) (TableQRResponse, error) {
	table, err := s.repos.GetTableByQRToken(ctx, token)
	if err != nil {
		return TableQRResponse{}, err
	}
	resp := TableQRResponse{
		ID:          table.ID,
		BranchID:    table.BranchID,
		Identifier:  table.Identifier,
		BranchTheme: "dark-luxury",
	}
	if session, err := s.repos.GetActiveSessionForTable(ctx, table.ID); err == nil {
		resp.SessionID = session.ID.String()
	}
	if restaurant, err := s.repos.GetRestaurantByBranchID(ctx, table.BranchID); err == nil {
		var settings map[string]any
		if json.Unmarshal(restaurant.SettingsJson, &settings) == nil {
			if t, ok := settings["theme"].(string); ok && t != "" {
				resp.BranchTheme = t
			}
		}
	}
	return resp, nil
}

// InvalidateMenuCache evicts the cached menu for a branch so the next request rebuilds from DB.
// Call this whenever menu items, categories, or modifiers change.
func (s *MenuService) InvalidateMenuCache(ctx context.Context, branchID int64) {
	_ = s.cache.DeleteMany(ctx, s.menuCacheKey(ctx, branchID), legacyMenuCacheKey(branchID))
}

func (s *MenuService) menuCacheKey(ctx context.Context, branchID int64) string {
	if org, err := s.repos.GetOrganizationByBranchID(ctx, branchID); err == nil {
		return fmt.Sprintf("org:%d:branch:%d:menu:v%d", org.ID, branchID, menuCacheVersion)
	}
	return legacyMenuCacheKey(branchID)
}

func legacyMenuCacheKey(branchID int64) string {
	return fmt.Sprintf("menu:%d", branchID)
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
	CategoryID   int64
	BranchID     int64
	Name         string
	Description  string
	Price        float64
	IsAvailable  bool
	Position     int16
	DietaryFlags []string
	ItemBadges   []string
	SpiceLevel   int16
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

	if len(p.DietaryFlags) > 0 || len(p.ItemBadges) > 0 || p.SpiceLevel != 0 {
		item, err = s.UpdateItem(ctx, UpdateMenuItemParams{
			ID:           item.ID,
			BranchID:     p.BranchID,
			Name:         p.Name,
			Description:  p.Description,
			Price:        p.Price,
			Position:     p.Position,
			DietaryFlags: p.DietaryFlags,
			ItemBadges:   p.ItemBadges,
			SpiceLevel:   p.SpiceLevel,
		}, requiredRole)
		if err != nil {
			return sqlc.MenuItem{}, err
		}
		return item, nil
	}

	s.InvalidateMenuCache(ctx, p.BranchID)
	return item, nil
}

type UpdateMenuItemParams struct {
	ID           int64
	BranchID     int64
	Name         string
	Description  string
	Price        float64
	Position     int16
	DietaryFlags []string
	ItemBadges   []string
	SpiceLevel   int16
	CategoryID   *int64  // nil = keep current category
	ImageURL     *string // nil = keep current image_url
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
	var catID pgtype.Int8
	if p.CategoryID != nil {
		catID = pgtype.Int8{Int64: *p.CategoryID, Valid: true}
	}
	var imageURL pgtype.Text
	if p.ImageURL != nil {
		imageURL = pgtype.Text{String: *p.ImageURL, Valid: true}
	}
	item, err := s.repos.UpdateMenuItemScoped(ctx, sqlc.UpdateMenuItemParams{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Price:        price,
		Position:     p.Position,
		DietaryFlags: p.DietaryFlags,
		ItemBadges:   p.ItemBadges,
		SpiceLevel:   p.SpiceLevel,
		CategoryID:   catID,
		ImageUrl:     imageURL,
	}, p.BranchID)
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
	if err := s.repos.UpdateMenuItemAvailabilityScoped(ctx, itemID, branchID, available); err != nil {
		return err
	}
	s.InvalidateMenuCache(ctx, branchID)
	s.broadcastAvailabilityChange(ctx, itemID, branchID, available)
	return nil
}

// broadcastAvailabilityChange notifies every live session in the branch that a
// menu item's availability changed, so guests reconcile their menu/cart in
// realtime instead of discovering it only when an order is rejected. Best-effort:
// fanout failure must not fail the admin's toggle.
func (s *MenuService) broadcastAvailabilityChange(ctx context.Context, itemID, branchID int64, available bool) {
	if s.publisher == nil {
		return
	}
	sessions, err := s.repos.ListActiveSessionsForBranch(ctx, branchID)
	if err != nil {
		return
	}
	payload := map[string]any{"item_id": itemID, "is_available": available}
	for _, sess := range sessions {
		s.publisher.MenuItemAvailabilityChanged(ctx, sess.ID, payload)
	}
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
					ID:            m.ID,
					Name:          m.Name,
					PriceDelta:    delta.Float64,
					IsRequired:    m.IsRequired,
					ModifierGroup: m.ModifierGroup,
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

	featuredRows, err := s.repos.ListFeaturedMenuItems(ctx, branchID)
	if err != nil {
		return FullMenu{}, err
	}
	result.Featured = make([]MenuItemWithModifiers, 0, len(featuredRows))
	for _, item := range featuredRows {
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
		result.Featured = append(result.Featured, MenuItemWithModifiers{MenuItem: item, Modifiers: snapMods})
	}

	return result, nil
}

// ToggleFeatured sets is_featured and featured_sort_order on a menu item and invalidates the branch menu cache.
func (s *MenuService) ToggleFeatured(ctx context.Context, itemID, branchID int64, featured bool, sortOrder int16, requiredRole sqlc.StaffRole) error {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return err
	}
	if err := s.repos.UpdateMenuItemFeaturedScoped(ctx, itemID, branchID, featured, sortOrder); err != nil {
		return err
	}
	s.InvalidateMenuCache(ctx, branchID)
	return nil
}

// GetAdminMenu returns the full menu for a branch including unavailable items and inactive categories (not cached).
func (s *MenuService) GetAdminMenu(ctx context.Context, branchID int64) (FullMenu, error) {
	return s.buildAdminMenu(ctx, branchID)
}

func (s *MenuService) buildAdminMenu(ctx context.Context, branchID int64) (FullMenu, error) {
	categories, err := s.repos.ListAllMenuCategoriesForBranch(ctx, branchID)
	if err != nil {
		return FullMenu{}, err
	}

	result := FullMenu{BranchID: branchID, Categories: make([]MenuCategoryWithItems, 0, len(categories))}

	for _, cat := range categories {
		items, err := s.repos.ListAllMenuItemsForCategory(ctx, cat.ID)
		if err != nil {
			return FullMenu{}, err
		}

		categoryItems := make([]MenuItemWithModifiers, 0, len(items))
		for _, item := range items {
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
					ID:            m.ID,
					Name:          m.Name,
					PriceDelta:    delta.Float64,
					IsRequired:    m.IsRequired,
					ModifierGroup: m.ModifierGroup,
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

	result.Featured = []MenuItemWithModifiers{}
	return result, nil
}

// DeleteMenuItem removes a menu item and invalidates the branch menu cache.
func (s *MenuService) DeleteMenuItem(ctx context.Context, itemID, branchID int64, requiredRole sqlc.StaffRole) error {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return err
	}
	if err := s.repos.DeleteMenuItem(ctx, itemID, branchID); err != nil {
		return err
	}
	s.InvalidateMenuCache(ctx, branchID)
	return nil
}

// DeleteCategory removes a menu category if it has no items, invalidates cache.
// Returns ErrCategoryNotEmpty if items still exist in the category.
func (s *MenuService) DeleteCategory(ctx context.Context, categoryID, branchID int64, requiredRole sqlc.StaffRole) error {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return err
	}
	count, err := s.repos.CountItemsInCategory(ctx, categoryID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrCategoryNotEmpty
	}
	if err := s.repos.DeleteMenuCategory(ctx, categoryID, branchID); err != nil {
		return err
	}
	s.InvalidateMenuCache(ctx, branchID)
	return nil
}

type UpdateMenuCategoryParams struct {
	ID       int64
	BranchID int64
	Name     string
	Position int16
	IsActive bool
}

// UpdateCategory modifies a menu category and invalidates the branch menu cache.
func (s *MenuService) UpdateCategory(ctx context.Context, p UpdateMenuCategoryParams, requiredRole sqlc.StaffRole) (sqlc.MenuCategory, error) {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return sqlc.MenuCategory{}, err
	}
	cat, err := s.repos.UpdateMenuCategory(ctx, sqlc.UpdateMenuCategoryParams{
		ID:       p.ID,
		BranchID: p.BranchID,
		Name:     p.Name,
		Position: p.Position,
		IsActive: p.IsActive,
	})
	if err != nil {
		return sqlc.MenuCategory{}, err
	}
	s.InvalidateMenuCache(ctx, p.BranchID)
	return cat, nil
}

type CreateModifierParams struct {
	ItemID        int64
	BranchID      int64
	Name          string
	PriceDelta    float64
	IsRequired    bool
	ModifierGroup string
}

// AddModifier adds a modifier to a menu item and invalidates the branch menu cache.
func (s *MenuService) AddModifier(ctx context.Context, p CreateModifierParams, requiredRole sqlc.StaffRole) (MenuModifier, error) {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return MenuModifier{}, err
	}
	var delta pgtype.Numeric
	if err := delta.Scan(fmt.Sprintf("%.2f", p.PriceDelta)); err != nil {
		return MenuModifier{}, fmt.Errorf("invalid price_delta: %w", err)
	}
	mod, err := s.repos.CreateItemModifierScoped(ctx, sqlc.CreateItemModifierParams{
		ItemID:        p.ItemID,
		Name:          p.Name,
		PriceDelta:    delta,
		IsRequired:    p.IsRequired,
		ModifierGroup: p.ModifierGroup,
	}, p.BranchID)
	if err != nil {
		return MenuModifier{}, err
	}
	s.InvalidateMenuCache(ctx, p.BranchID)
	deltaVal, _ := mod.PriceDelta.Float64Value()
	return MenuModifier{
		ID:            mod.ID,
		Name:          mod.Name,
		PriceDelta:    deltaVal.Float64,
		IsRequired:    mod.IsRequired,
		ModifierGroup: mod.ModifierGroup,
	}, nil
}

// DeleteModifier removes a modifier and invalidates the branch menu cache.
func (s *MenuService) DeleteModifier(ctx context.Context, modifierID, branchID int64, requiredRole sqlc.StaffRole) error {
	if err := requireOwnerOrManager(requiredRole); err != nil {
		return err
	}
	if err := s.repos.DeleteItemModifierScoped(ctx, modifierID, branchID); err != nil {
		return err
	}
	s.InvalidateMenuCache(ctx, branchID)
	return nil
}
