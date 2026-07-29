package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/audit"
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
	audit   *audit.Writer
}

func NewStaffHandler(svc *services.StaffService, repos *repository.Repos, metrics *observability.Metrics, flags config.FeatureFlags, authorizer *authz.Authorizer, auditWriter *audit.Writer) *StaffHandler {
	return &StaffHandler{svc: svc, repos: repos, metrics: metrics, flags: flags, authz: authorizer, audit: auditWriter}
}

// staffLockoutIdentity mirrors the identity key used by StaffService so the
// handler can ask the service for the remaining lockout duration.
func staffLockoutIdentity(req staffAuthRequest) string {
	if req.BranchCode != "" || req.StaffCode != "" {
		return "code:" + strings.ToUpper(strings.TrimSpace(req.BranchCode)) + ":" + strings.ToUpper(strings.TrimSpace(req.StaffCode))
	}
	return "branch:" + strconv.FormatInt(req.BranchID, 10)
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
		switch {
		case errors.Is(err, domain.ErrAuthLockedOut):
			h.audit.Record(c.Request.Context(), audit.AuditEvent{
				BranchID:     req.BranchID,
				ResourceType: audit.ResourceStaff,
				Action:       audit.ActionStaffLoginFailed,
				Result:       audit.ResultDenied,
				ActorType:    audit.ActorTypeStaff,
				RiskLevel:    audit.RiskHigh,
				Metadata:     map[string]any{"reason": "locked_out", "staff_code": req.StaffCode, "branch_code": req.BranchCode},
			})
			identity := staffLockoutIdentity(req)
			if retry := h.svc.LockoutRetryAfter(c.Request.Context(), identity); retry > 0 {
				c.Header("Retry-After", strconv.Itoa(retry))
			}
			respondError(c, http.StatusLocked, CodeAuthLockedOut, "too many failed attempts; try again later")
			return
		case errors.Is(err, domain.ErrParticipantUnauthorized):
			h.audit.Record(c.Request.Context(), audit.AuditEvent{
				BranchID:     req.BranchID,
				ResourceType: audit.ResourceStaff,
				Action:       audit.ActionStaffLoginFailed,
				Result:       audit.ResultFailure,
				ActorType:    audit.ActorTypeStaff,
				RiskLevel:    audit.RiskHigh,
				Metadata:     map[string]any{"reason": "invalid_credentials"},
			})
			respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials")
			return
		default:
			respondInternalError(c)
			return
		}
	}

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:       session.BranchID,
		OrganizationID: session.OrganizationID,
		ResourceType:   audit.ResourceStaff,
		ResourceID:     audit.IDStr(session.StaffID),
		Action:         audit.ActionStaffLogin,
		Result:         audit.ResultSuccess,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        audit.IDStr(session.StaffID),
		RiskLevel:      audit.RiskMedium,
	})
	issueStaffCookieIfEnabled(c, session.Token)
	c.JSON(http.StatusOK, session)
}

// Logout invalidates the current staff session and clears the HttpOnly cookie
// if one was set. Bearer-token-only clients can simply drop the token on the
// frontend; the cookie path needs explicit clearing.
func (h *StaffHandler) Logout(c *gin.Context) {
	clearStaffCookie(c)
	c.Status(http.StatusNoContent)
}

// issueStaffCookieIfEnabled sets the HttpOnly staff session cookie when the
// rollout flag is on. Honors release-mode TLS via the Secure flag so dev (HTTP
// localhost) and production (HTTPS) both work.
func issueStaffCookieIfEnabled(c *gin.Context, token string) {
	if !activeAuthConfig.StaffCookieEnable {
		return
	}
	secure := activeServerConfig.GinMode == "release" || activeServerConfig.EnableHSTS
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.StaffCookieName, token, 8*60*60, middleware.StaffCookiePath, "", secure, true /* HttpOnly */)
}

func clearStaffCookie(c *gin.Context) {
	secure := activeServerConfig.GinMode == "release" || activeServerConfig.EnableHSTS
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.StaffCookieName, "", -1, middleware.StaffCookiePath, "", secure, true)
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

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:       branchID,
		OrganizationID: sess.OrganizationID,
		ResourceType:   audit.ResourceStaff,
		ResourceID:     audit.IDStr(staff.ID),
		Action:         audit.ActionStaffCreate,
		Result:         audit.ResultSuccess,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        audit.IDStr(sess.StaffID),
		RiskLevel:      audit.RiskMedium,
		After:          audit.MustJSON(map[string]any{"name": staff.Name, "role": string(staff.Role)}),
	})
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
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionStaffPinUpdate, authz.StaffResource(target.ID, target.BranchID, orgID)) {
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

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     target.BranchID,
		ResourceType: audit.ResourceStaff,
		ResourceID:   audit.IDStr(staffID),
		Action:       audit.ActionStaffPINReset,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      audit.IDStr(sess.StaffID),
		RiskLevel:    audit.RiskHigh,
	})
	c.Status(http.StatusNoContent)
}

// ListStaff returns the active staff roster for a branch (owner/manager),
// without pin hashes. GET /branches/:id/staff
func (h *StaffHandler) ListStaff(c *gin.Context) {
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
	actor, ok := staffActorForRequest(c, h.repos, sess)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, branchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionStaffListRead, authz.BranchResource(branchID, orgID)) {
		return
	}
	roster, err := h.repos.ListStaffRosterForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"staff": roster})
}

type resetPINRequest struct {
	NewPIN string `json:"new_pin" binding:"required,min=4,max=8"`
}

// ResetPIN sets a new PIN for another staff member WITHOUT the current PIN —
// the manager/owner forgotten-PIN path. POST /staff/:id/pin/reset
// Rules: you cannot reset your own PIN here (use the self-change endpoint);
// a manager may reset only waiters/kitchen; an owner may reset anyone in branch.
func (h *StaffHandler) ResetPIN(c *gin.Context) {
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
	if sess.StaffID == staffID {
		respondError(c, http.StatusBadRequest, CodeValidationError, "use the change-PIN option for your own PIN")
		return
	}
	target, err := h.repos.GetStaffByID(c.Request.Context(), staffID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeUnauthorized, "staff not found")
		return
	}
	if target.BranchID != sess.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}
	// A manager may only reset waiters/kitchen; owners may reset anyone.
	if sess.Role == sqlc.StaffRoleManager &&
		(target.Role == sqlc.StaffRoleOwner || target.Role == sqlc.StaffRoleManager) {
		respondError(c, http.StatusForbidden, CodeForbidden, "managers can only reset waiter and kitchen PINs")
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
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionStaffPinReset, authz.StaffResource(target.ID, target.BranchID, orgID)) {
		return
	}
	var req resetPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if err := h.svc.ResetPINScoped(c.Request.Context(), staffID, target.BranchID, req.NewPIN); err != nil {
		respondInternalError(c)
		return
	}
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     target.BranchID,
		ResourceType: audit.ResourceStaff,
		ResourceID:   audit.IDStr(staffID),
		Action:       audit.ActionStaffPINReset,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      audit.IDStr(sess.StaffID),
		RiskLevel:    audit.RiskHigh,
	})
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
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionStaffDeactivate, authz.StaffResource(target.ID, target.BranchID, orgID)) {
		return
	}

	if err := h.svc.DeactivateScoped(c.Request.Context(), staffID, target.BranchID); err != nil {
		respondInternalError(c)
		return
	}

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:       target.BranchID,
		OrganizationID: actor.Scope.OrganizationID,
		ResourceType:   audit.ResourceStaff,
		ResourceID:     audit.IDStr(staffID),
		Action:         audit.ActionStaffDeactivate,
		Result:         audit.ResultSuccess,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        audit.IDStr(sess.StaffID),
		ActorScope: map[string]any{
			"org_id":    actor.Scope.OrganizationID,
			"branch_id": target.BranchID,
			"role":      string(sess.Role),
		},
		RiskLevel: audit.RiskHigh,
		After:     audit.MustJSON(map[string]any{"is_active": false}),
	})
	c.Status(http.StatusNoContent)
}
