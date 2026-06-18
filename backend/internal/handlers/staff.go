package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type StaffHandler struct {
	svc     *services.StaffService
	repos   *repository.Repos
	metrics *observability.Metrics
	flags   config.FeatureFlags
	authz   *authz.Authorizer
}

func NewStaffHandler(svc *services.StaffService, repos *repository.Repos, metrics *observability.Metrics, flags config.FeatureFlags, authorizer *authz.Authorizer) *StaffHandler {
	return &StaffHandler{svc: svc, repos: repos, metrics: metrics, flags: flags, authz: authorizer}
}

type staffAuthRequest struct {
	BranchID   int64  `json:"branch_id"`
	BranchCode string `json:"branch_code"`
	StaffCode  string `json:"staff_code"`
	PIN        string `json:"pin" binding:"required,min=4,max=8"`
	DeviceName string `json:"device_name"`
}

func (h *StaffHandler) Authenticate(c *gin.Context) {
	var req staffAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	var session services.StaffSession
	var err error
	if req.BranchCode != "" || req.StaffCode != "" || h.flags.AuthStaffCodeRequired {
		if req.BranchCode == "" || req.StaffCode == "" {
			respondValidationError(c, "branch_code and staff_code are required")
			return
		}
		session, err = h.svc.AuthenticateWithCode(c.Request.Context(), req.BranchCode, req.StaffCode, req.PIN, req.DeviceName)
	} else {
		if req.BranchID == 0 {
			respondValidationError(c, "branch_id is required")
			return
		}
		recordLegacyIdentityUsage(h.metrics, legacyMechanismBranchPIN, legacyEndpointStaffAuth)
		session, err = h.svc.Authenticate(c.Request.Context(), req.BranchID, req.PIN)
	}
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
	Name      string `json:"name" binding:"required,min=1,max=100"`
	Role      string `json:"role" binding:"required"`
	StaffCode string `json:"staff_code"`
	PIN       string `json:"pin" binding:"required,min=4,max=8"`
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
	staff, err := h.svc.CreateStaff(c.Request.Context(), branchID, role, req.Name, req.StaffCode, req.PIN)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         staff.ID,
		"branch_id":  staff.BranchID,
		"name":       staff.Name,
		"role":       staff.Role,
		"staff_code": staff.StaffCode,
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
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	target, err := h.repos.GetStaffByID(c.Request.Context(), staffID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeUnauthorized, "staff not found")
		return
	}
	if sess.StaffID != staffID && sess.Role != sqlc.StaffRoleOwner {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}
	actor, ok := staffActorForRequest(c, h.repos, sess)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, target.BranchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, actor, authz.ActionStaffPinUpdate, authz.StaffResource(target.ID, target.BranchID, orgID)) {
		return
	}

	var req rotatePINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	if err := h.svc.RotatePINScoped(c.Request.Context(), staffID, target.BranchID, req.CurrentPIN, req.NewPIN); err != nil {
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
	target, err := h.repos.GetStaffByID(c.Request.Context(), staffID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeUnauthorized, "staff not found")
		return
	}
	actor, ok := staffActorForRequest(c, h.repos, sess)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, target.BranchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, actor, authz.ActionStaffDeactivate, authz.StaffResource(target.ID, target.BranchID, orgID)) {
		return
	}

	if err := h.svc.DeactivateScoped(c.Request.Context(), staffID, target.BranchID); err != nil {
		respondInternalError(c)
		return
	}

	c.Status(http.StatusNoContent)
}
