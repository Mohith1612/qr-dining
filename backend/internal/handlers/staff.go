package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type StaffHandler struct {
	svc *services.StaffService
}

func NewStaffHandler(svc *services.StaffService) *StaffHandler {
	return &StaffHandler{svc: svc}
}

type staffAuthRequest struct {
	BranchID int64  `json:"branch_id" binding:"required"`
	PIN      string `json:"pin" binding:"required,min=4,max=8"`
}

func (h *StaffHandler) Authenticate(c *gin.Context) {
	var req staffAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	session, err := h.svc.Authenticate(c.Request.Context(), req.BranchID, req.PIN)
	if err != nil {
		if errors.Is(err, domain.ErrParticipantUnauthorized) {
			respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials")
			return
		}
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, session)
}

type createStaffRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
	Role string `json:"role" binding:"required"`
	PIN  string `json:"pin" binding:"required,min=4,max=8"`
}

// CreateStaff creates a new staff member. Only owners may create staff.
func (h *StaffHandler) CreateStaff(c *gin.Context) {
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
	if sess.Role != sqlc.StaffRoleOwner {
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners can create staff")
		return
	}

	var req createStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	role := sqlc.StaffRole(req.Role)
	staff, err := h.svc.CreateStaff(c.Request.Context(), branchID, role, req.Name, req.PIN)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        staff.ID,
		"branch_id": staff.BranchID,
		"name":      staff.Name,
		"role":      staff.Role,
	})
}

type rotatePINRequest struct {
	CurrentPIN string `json:"current_pin" binding:"required,min=4,max=8"`
	NewPIN     string `json:"new_pin" binding:"required,min=4,max=8"`
}

// RotatePIN verifies the current PIN and sets a new one.
func (h *StaffHandler) RotatePIN(c *gin.Context) {
	staffID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid staff id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.StaffID != staffID {
		// Staff may only rotate their own PIN; owners can rotate any.
		if !ok || sess.Role != sqlc.StaffRoleOwner {
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
	}

	var req rotatePINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	if err := h.svc.RotatePIN(c.Request.Context(), staffID, req.CurrentPIN, req.NewPIN); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			respondError(c, http.StatusUnauthorized, CodeUnauthorized, "current PIN is incorrect")
			return
		}
		respondInternalError(c)
		return
	}

	c.Status(http.StatusNoContent)
}

// DeactivateStaff soft-deactivates a staff member and invalidates their tokens.
// Only owners may deactivate staff.
func (h *StaffHandler) DeactivateStaff(c *gin.Context) {
	staffID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid staff id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.Role != sqlc.StaffRoleOwner {
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners can deactivate staff")
		return
	}

	if err := h.svc.Deactivate(c.Request.Context(), staffID); err != nil {
		respondInternalError(c)
		return
	}

	c.Status(http.StatusNoContent)
}
