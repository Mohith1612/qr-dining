package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type MenuAdminHandler struct {
	svc *services.MenuService
}

func NewMenuAdminHandler(svc *services.MenuService) *MenuAdminHandler {
	return &MenuAdminHandler{svc: svc}
}

type createCategoryRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Position int16  `json:"position"`
}

func (h *MenuAdminHandler) CreateCategory(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	cat, err := h.svc.CreateCategory(c.Request.Context(), branchID, req.Name, req.Position, sess.Role)
	if err != nil {
		menuAdminError(c, err)
		return
	}

	c.JSON(http.StatusCreated, cat)
}

type createMenuItemRequest struct {
	CategoryID  int64   `json:"category_id" binding:"required"`
	Name        string  `json:"name" binding:"required,min=1,max=100"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,min=0"`
	IsAvailable bool    `json:"is_available"`
	Position    int16   `json:"position"`
}

func (h *MenuAdminHandler) CreateItem(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	var req createMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	item, err := h.svc.CreateItem(c.Request.Context(), services.CreateMenuItemParams{
		CategoryID:  req.CategoryID,
		BranchID:    branchID,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		IsAvailable: req.IsAvailable,
		Position:    req.Position,
	}, sess.Role)
	if err != nil {
		menuAdminError(c, err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

type updateMenuItemRequest struct {
	Name         string   `json:"name" binding:"required,min=1,max=100"`
	Description  string   `json:"description"`
	Price        float64  `json:"price" binding:"required,min=0"`
	Position     int16    `json:"position"`
	BranchID     int64    `json:"branch_id" binding:"required"`
	DietaryFlags []string `json:"dietary_flags"`
	ItemBadges   []string `json:"item_badges"`
	SpiceLevel   int16    `json:"spice_level"`
	CategoryID   *int64   `json:"category_id"` // optional: move item to a different category
	ImageURL     *string  `json:"image_url"`   // optional: set or update item image
}

func (h *MenuAdminHandler) UpdateItem(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid item id")
		return
	}

	var req updateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	validDietary := map[string]bool{"vegetarian": true, "vegan": true, "jain": true, "egg": true, "non-veg": true}
	validBadges := map[string]bool{"chef-special": true, "bestseller": true, "seasonal": true, "new": true}
	for _, f := range req.DietaryFlags {
		if !validDietary[f] {
			respondValidationError(c, "invalid dietary_flag: "+f)
			return
		}
	}
	for _, b := range req.ItemBadges {
		if !validBadges[b] {
			respondValidationError(c, "invalid item_badge: "+b)
			return
		}
	}
	if req.SpiceLevel < 0 || req.SpiceLevel > 3 {
		respondValidationError(c, "spice_level must be 0-3")
		return
	}

	if req.DietaryFlags == nil {
		req.DietaryFlags = []string{}
	}
	if req.ItemBadges == nil {
		req.ItemBadges = []string{}
	}

	item, err := h.svc.UpdateItem(c.Request.Context(), services.UpdateMenuItemParams{
		ID:           itemID,
		BranchID:     req.BranchID,
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		Position:     req.Position,
		DietaryFlags: req.DietaryFlags,
		ItemBadges:   req.ItemBadges,
		SpiceLevel:   req.SpiceLevel,
		CategoryID:   req.CategoryID,
		ImageURL:     req.ImageURL,
	}, sess.Role)
	if err != nil {
		menuAdminError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

type toggleAvailabilityRequest struct {
	Available bool  `json:"available"`
	BranchID  int64 `json:"branch_id" binding:"required"`
}

func (h *MenuAdminHandler) ToggleAvailability(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid item id")
		return
	}

	var req toggleAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	if err := h.svc.ToggleAvailability(c.Request.Context(), itemID, req.BranchID, req.Available, sess.Role); err != nil {
		menuAdminError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

type toggleFeaturedRequest struct {
	Featured          bool  `json:"is_featured"`
	FeaturedSortOrder int16 `json:"featured_sort_order"`
	BranchID          int64 `json:"branch_id" binding:"required"`
}

func (h *MenuAdminHandler) ToggleFeatured(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid item id")
		return
	}

	var req toggleFeaturedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	if err := h.svc.ToggleFeatured(c.Request.Context(), itemID, req.BranchID, req.Featured, req.FeaturedSortOrder, sess.Role); err != nil {
		menuAdminError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func menuAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrMenuItemNotFound):
		respondError(c, http.StatusNotFound, CodeMenuItemNotFound, err.Error())
	case errors.Is(err, domain.ErrCategoryNotFound):
		respondError(c, http.StatusNotFound, CodeCategoryNotFound, err.Error())
	case errors.Is(err, domain.ErrCategoryNotEmpty):
		respondError(c, http.StatusConflict, CodeCategoryNotEmpty, err.Error())
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrForbidden):
		respondError(c, http.StatusForbidden, CodeForbidden, err.Error())
	default:
		respondInternalError(c)
	}
}

func (h *MenuAdminHandler) GetAdminMenu(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	menu, err := h.svc.GetAdminMenu(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, menu)
}

func (h *MenuAdminHandler) DeleteMenuItem(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid item id")
		return
	}

	branchID, err := strconv.ParseInt(c.Query("branch_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "branch_id query param required")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	if err := h.svc.DeleteMenuItem(c.Request.Context(), itemID, branchID, sess.Role); err != nil {
		menuAdminError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *MenuAdminHandler) DeleteMenuCategory(c *gin.Context) {
	categoryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid category id")
		return
	}

	branchID, err := strconv.ParseInt(c.Query("branch_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "branch_id query param required")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	if err := h.svc.DeleteCategory(c.Request.Context(), categoryID, branchID, sess.Role); err != nil {
		menuAdminError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

type updateMenuCategoryRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Position int16  `json:"position"`
	IsActive bool   `json:"is_active"`
	BranchID int64  `json:"branch_id" binding:"required"`
}

func (h *MenuAdminHandler) UpdateMenuCategory(c *gin.Context) {
	categoryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid category id")
		return
	}

	var req updateMenuCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	cat, err := h.svc.UpdateCategory(c.Request.Context(), services.UpdateMenuCategoryParams{
		ID:       categoryID,
		BranchID: req.BranchID,
		Name:     req.Name,
		Position: req.Position,
		IsActive: req.IsActive,
	}, sess.Role)
	if err != nil {
		menuAdminError(c, err)
		return
	}

	c.JSON(http.StatusOK, cat)
}

type addModifierRequest struct {
	Name          string  `json:"name" binding:"required,min=1,max=100"`
	PriceDelta    float64 `json:"price_delta"`
	IsRequired    bool    `json:"is_required"`
	ModifierGroup string  `json:"modifier_group"`
	BranchID      int64   `json:"branch_id" binding:"required"`
}

func (h *MenuAdminHandler) AddItemModifier(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid item id")
		return
	}

	var req addModifierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	mod, err := h.svc.AddModifier(c.Request.Context(), services.CreateModifierParams{
		ItemID:        itemID,
		BranchID:      req.BranchID,
		Name:          req.Name,
		PriceDelta:    req.PriceDelta,
		IsRequired:    req.IsRequired,
		ModifierGroup: req.ModifierGroup,
	}, sess.Role)
	if err != nil {
		menuAdminError(c, err)
		return
	}

	c.JSON(http.StatusCreated, mod)
}

func (h *MenuAdminHandler) DeleteItemModifier(c *gin.Context) {
	modifierID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid modifier id")
		return
	}

	branchID, err := strconv.ParseInt(c.Query("branch_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "branch_id query param required")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	if err := h.svc.DeleteModifier(c.Request.Context(), modifierID, branchID, sess.Role); err != nil {
		menuAdminError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
