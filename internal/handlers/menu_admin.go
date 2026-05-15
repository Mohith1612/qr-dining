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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch id"})
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch id"})
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var req createMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	Name        string  `json:"name" binding:"required,min=1,max=100"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,min=0"`
	Position    int16   `json:"position"`
	BranchID    int64   `json:"branch_id" binding:"required"`
}

func (h *MenuAdminHandler) UpdateItem(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	var req updateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	item, err := h.svc.UpdateItem(c.Request.Context(), services.UpdateMenuItemParams{
		ID:          itemID,
		BranchID:    req.BranchID,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Position:    req.Position,
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	var req toggleAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != req.BranchID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if err := h.svc.ToggleAvailability(c.Request.Context(), itemID, req.BranchID, req.Available, sess.Role); err != nil {
		menuAdminError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func menuAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrMenuItemNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
