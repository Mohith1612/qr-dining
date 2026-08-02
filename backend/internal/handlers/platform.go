package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/crypto"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlatformHandler struct {
	repos       *repository.Repos
	svc         *services.PlatformService
	ent         *services.EntitlementService
	flag        *services.FlagService
	analytics   *services.PlatformAnalyticsService
	theme       *services.ThemeService
	support     *services.SupportService
	billing     *services.BillingService
	obs         *services.EnforcementObservabilityService
	collateral  *services.CollateralService
	staffPerf   *services.StaffAnalyticsService
	featureGate *services.FeatureGate
	audit       *audit.Writer
}

func NewPlatformHandler(repos *repository.Repos, svc *services.PlatformService, ent *services.EntitlementService, flag *services.FlagService, analytics *services.PlatformAnalyticsService, theme *services.ThemeService, support *services.SupportService, billing *services.BillingService, obs *services.EnforcementObservabilityService, collateral *services.CollateralService, staffPerf *services.StaffAnalyticsService, featureGate *services.FeatureGate, auditWriter *audit.Writer) *PlatformHandler {
	return &PlatformHandler{repos: repos, svc: svc, ent: ent, flag: flag, analytics: analytics, theme: theme, support: support, billing: billing, obs: obs, collateral: collateral, staffPerf: staffPerf, featureGate: featureGate, audit: auditWriter}
}

type platformAuthRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	DeviceName string `json:"device_name"`
}

func (h *PlatformHandler) Authenticate(c *gin.Context) {
	var req platformAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	result, err := h.svc.AuthenticateWithMFA(c.Request.Context(), req.Email, req.Password, req.DeviceName, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		if errors.Is(err, domain.ErrAuthLockedOut) {
			h.logPlatformAudit(c, 0, "platform.auth.locked_out", "platform_user", "", 0, 0, 0, gin.H{"email": services.NormalizePlatformEmail(req.Email)})
			if retry := h.svc.LockoutRetryAfter(c.Request.Context(), req.Email); retry > 0 {
				c.Header("Retry-After", strconv.Itoa(retry))
			}
			respondError(c, http.StatusLocked, CodeAuthLockedOut, "too many failed attempts; try again later")
			return
		}
		h.logPlatformAudit(c, 0, "platform.auth.failed", "platform_user", "", 0, 0, 0, gin.H{"email": services.NormalizePlatformEmail(req.Email)})
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials")
		return
	}
	if result.Challenge != nil {
		h.logPlatformAudit(c, 0, "platform.auth.mfa_challenge_issued", "platform_user", "", 0, 0, 0, gin.H{"email": services.NormalizePlatformEmail(req.Email)})
		c.JSON(http.StatusOK, gin.H{
			"mfa_required":   true,
			"mfa_challenge":  result.Challenge.Challenge,
			"mfa_expires_at": result.Challenge.ExpiresAt,
		})
		return
	}
	session := result.Session
	h.logPlatformAudit(c, session.PlatformUserID, "platform.auth.succeeded", "platform_user", strconv.FormatInt(session.PlatformUserID, 10), 0, 0, 0, gin.H{"email": session.Email})
	c.JSON(http.StatusOK, session)
}

type platformMFACompleteRequest struct {
	Challenge  string `json:"mfa_challenge" binding:"required"`
	Code       string `json:"mfa_code" binding:"required"`
	DeviceName string `json:"device_name"`
}

// CompleteMFA finishes a login interrupted at the MFA stage: the caller
// supplies the challenge token plus the user's TOTP or recovery code; on
// success a full PlatformSession is issued just as if MFA had not been
// required.
func (h *PlatformHandler) CompleteMFA(c *gin.Context) {
	var req platformMFACompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	session, err := h.svc.CompleteMFAChallenge(c.Request.Context(), req.Challenge, req.Code, req.DeviceName)
	if err != nil {
		if errors.Is(err, domain.ErrMFAInvalidCode) {
			h.logPlatformAudit(c, 0, "platform.auth.mfa_failed", "platform_user", "", 0, 0, 0, gin.H{"reason": "invalid_code"})
			respondError(c, http.StatusUnauthorized, CodeMFAInvalidCode, "invalid mfa code")
			return
		}
		h.logPlatformAudit(c, 0, "platform.auth.mfa_failed", "platform_user", "", 0, 0, 0, gin.H{"reason": "challenge_invalid"})
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid mfa challenge")
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.auth.mfa_succeeded", "platform_user", strconv.FormatInt(session.PlatformUserID, 10), 0, 0, 0, gin.H{"email": session.Email})
	c.JSON(http.StatusOK, session)
}

// BeginMFAEnrollment starts TOTP enrollment for the authenticated platform
// user. The returned secret + otpauth URI are shown to the user once.
func (h *PlatformHandler) BeginMFAEnrollment(c *gin.Context) {
	session, ok := middleware.GetPlatformSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "platform authentication required")
		return
	}
	setup, err := h.svc.BeginMFAEnrollment(c.Request.Context(), session.PlatformUserID)
	if err != nil {
		h.logPlatformAudit(c, session.PlatformUserID, "platform.mfa.enroll_failed", "platform_user", strconv.FormatInt(session.PlatformUserID, 10), 0, 0, 0, gin.H{"error": err.Error()})
		if errors.Is(err, domain.ErrMFANotConfigured) {
			respondError(c, http.StatusServiceUnavailable, CodeMFANotConfigured, "multi-factor authentication is not available on this server")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.mfa.enroll_started", "platform_user", strconv.FormatInt(session.PlatformUserID, 10), 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, setup)
}

type platformMFAConfirmRequest struct {
	Code string `json:"code" binding:"required"`
}

// ConfirmMFAEnrollment verifies the first TOTP code and returns the recovery
// codes. The codes are shown once; only their bcrypt hashes are persisted.
func (h *PlatformHandler) ConfirmMFAEnrollment(c *gin.Context) {
	session, ok := middleware.GetPlatformSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "platform authentication required")
		return
	}
	var req platformMFAConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	result, err := h.svc.ConfirmMFAEnrollment(c.Request.Context(), session.PlatformUserID, req.Code)
	if err != nil {
		if errors.Is(err, domain.ErrMFAInvalidCode) {
			respondError(c, http.StatusUnauthorized, CodeMFAInvalidCode, "invalid code")
			return
		}
		if errors.Is(err, domain.ErrMFANotConfigured) {
			respondError(c, http.StatusServiceUnavailable, CodeMFANotConfigured, "multi-factor authentication is not available on this server")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.mfa.enrolled", "platform_user", strconv.FormatInt(session.PlatformUserID, 10), 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, result)
}

// DisableMFA removes MFA after verifying a current code or recovery code.
func (h *PlatformHandler) DisableMFA(c *gin.Context) {
	session, ok := middleware.GetPlatformSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "platform authentication required")
		return
	}
	var req platformMFAConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if err := h.svc.DisableMFA(c.Request.Context(), session.PlatformUserID, req.Code); err != nil {
		if errors.Is(err, domain.ErrMFAInvalidCode) {
			respondError(c, http.StatusUnauthorized, CodeMFAInvalidCode, "invalid code")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.mfa.disabled", "platform_user", strconv.FormatInt(session.PlatformUserID, 10), 0, 0, 0, gin.H{})
	c.Status(http.StatusNoContent)
}

func (h *PlatformHandler) Logout(c *gin.Context) {
	session, ok := middleware.GetPlatformSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "platform authentication required")
		return
	}
	token, ok := middleware.GetPlatformToken(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "platform token missing")
		return
	}
	if err := h.svc.Logout(c.Request.Context(), session, token); err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.auth.logout", "platform_session", session.SessionID.String(), 0, 0, 0, gin.H{})
	c.Status(http.StatusNoContent)
}

func (h *PlatformHandler) ListUsers(c *gin.Context) {
	session, ok := h.requirePlatformRole(c, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	users, err := h.repos.ListPlatformUsers(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, user := range users {
		roles, err := h.repos.ListPlatformRolesForUser(c.Request.Context(), user.ID)
		if err != nil {
			respondInternalError(c)
			return
		}
		out = append(out, platformUserResponse(user, roles))
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.users.list", "platform_user", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *PlatformHandler) GetUser(c *gin.Context) {
	session, ok := h.requirePlatformRole(c, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	id, ok := parseInt64Param(c, "id", "invalid platform user id")
	if !ok {
		return
	}
	user, err := h.repos.GetPlatformUserByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeUnauthorized, "platform user not found")
		return
	}
	roles, err := h.repos.ListPlatformRolesForUser(c.Request.Context(), user.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.users.read", "platform_user", strconv.FormatInt(user.ID, 10), 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, platformUserResponse(user, roles))
}

func (h *PlatformHandler) ListOrganizations(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgs, err := h.repos.ListPlatformOrganizations(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, 0, len(orgs))
	for _, org := range orgs {
		out = append(out, organizationResponse(org))
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.organizations.list", "organization", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"organizations": out})
}

type createPlatformOrganizationRequest struct {
	Code                string           `json:"code" binding:"required,min=2,max=64"`
	Name                string           `json:"name" binding:"required,min=1,max=120"`
	LegalName           string           `json:"legal_name"`
	PrimaryContactEmail string           `json:"primary_contact_email"`
	Settings            *json.RawMessage `json:"settings"`
	RestaurantSlug      string           `json:"restaurant_slug" binding:"required,min=2,max=80"`
	RestaurantName      string           `json:"restaurant_name"`
}

func (h *PlatformHandler) CreateOrganization(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	var req createPlatformOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	settings, ok := platformSettingsJSON(c, req.Settings)
	if !ok {
		return
	}
	restaurantName := strings.TrimSpace(req.RestaurantName)
	if restaurantName == "" {
		restaurantName = strings.TrimSpace(req.Name)
	}

	var org sqlc.Organization
	var restaurant sqlc.Restaurant
	err := h.repos.WithTx(c.Request.Context(), func(tx *repository.Repos) error {
		var err error
		org, err = tx.CreatePlatformOrganization(c.Request.Context(), sqlc.CreatePlatformOrganizationParams{
			Code:                normalizePlatformSlug(req.Code),
			Name:                strings.TrimSpace(req.Name),
			LegalName:           strings.TrimSpace(req.LegalName),
			PrimaryContactEmail: services.NormalizePlatformEmail(req.PrimaryContactEmail),
			SettingsJson:        settings,
		})
		if err != nil {
			return err
		}
		restaurant, err = tx.CreatePlatformRestaurant(c.Request.Context(), sqlc.CreatePlatformRestaurantParams{
			Name:           restaurantName,
			Slug:           normalizePlatformSlug(req.RestaurantSlug),
			SettingsJson:   settings,
			OrganizationID: org.ID,
		})
		return err
	})
	if err != nil {
		// A duplicate org code or restaurant slug is a client conflict, not a
		// server error — surface it so a retry after a mid-wizard failure gets a
		// clear message instead of an opaque 500.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, "ORGANIZATION_EXISTS", "An organization with that code or restaurant slug already exists. Use a different code, or check the Organizations list — it may have been created already.")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.organizations.create", "organization", strconv.FormatInt(org.ID, 10), org.ID, 0, 0, gin.H{"restaurant_id": restaurant.ID})
	c.JSON(http.StatusCreated, gin.H{"organization": organizationResponse(org), "restaurant": restaurantResponse(restaurant)})
}

func (h *PlatformHandler) GetOrganization(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	org, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.organizations.read", "organization", strconv.FormatInt(org.ID, 10), org.ID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, organizationResponse(org))
}

func (h *PlatformHandler) UpdateOrganization(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	org, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	var req updateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	params := sqlc.UpdateOrganizationSettingsParams{
		ID:                  org.ID,
		Name:                org.Name,
		LegalName:           org.LegalName,
		PrimaryContactEmail: org.PrimaryContactEmail,
		SettingsJson:        org.SettingsJson,
	}
	if req.Name != nil {
		params.Name = strings.TrimSpace(*req.Name)
	}
	if req.LegalName != nil {
		params.LegalName = strings.TrimSpace(*req.LegalName)
	}
	if req.PrimaryContactEmail != nil {
		params.PrimaryContactEmail = services.NormalizePlatformEmail(*req.PrimaryContactEmail)
	}
	if req.Settings != nil {
		settings, ok := platformSettingsJSON(c, req.Settings)
		if !ok {
			return
		}
		params.SettingsJson = settings
	}
	updated, err := h.repos.UpdateOrganizationSettings(c.Request.Context(), params)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.organizations.update", "organization", strconv.FormatInt(updated.ID, 10), updated.ID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, organizationResponse(updated))
}

type createPlatformBranchRequest struct {
	Name          string                `json:"name" binding:"required,min=1,max=120"`
	Address       string                `json:"address"`
	Timezone      string                `json:"timezone"`
	BranchCode    string                `json:"branch_code"`
	OrderPrefix   string                `json:"order_prefix"`
	LogoURL       *string               `json:"logo_url"`
	InitialTables []createTableRequest  `json:"initial_tables"`
	InitialOwner  *platformOwnerRequest `json:"initial_owner"`
}

type platformOwnerRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	StaffCode string `json:"staff_code" binding:"required,min=2,max=32"`
	PIN       string `json:"pin" binding:"required,min=4,max=8"`
}

func (h *PlatformHandler) CreateBranch(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	var req createPlatformBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	org, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	restaurant, err := h.repos.GetRestaurantByOrganizationID(c.Request.Context(), org.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	branchCode := services.NormalizePlatformCode(req.BranchCode)
	if branchCode == "" {
		branchCode = generatedBranchCode(org.Code)
	}
	orderPrefix := services.NormalizePlatformCode(req.OrderPrefix)
	if orderPrefix == "" {
		orderPrefix = "OR"
	}
	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}

	var branch sqlc.Branch
	tables := []sqlc.Table{}
	var owner *sqlc.Staff
	err = h.repos.WithTx(c.Request.Context(), func(tx *repository.Repos) error {
		var err error
		branch, err = tx.CreatePlatformBranch(c.Request.Context(), sqlc.CreatePlatformBranchParams{
			RestaurantID:   restaurant.ID,
			OrganizationID: org.ID,
			Name:           strings.TrimSpace(req.Name),
			Address:        strings.TrimSpace(req.Address),
			Timezone:       timezone,
			BranchCode:     branchCode,
			OrderPrefix:    orderPrefix,
		})
		if err != nil {
			return err
		}
		if err := tx.CreateOrganizationBranchMembership(c.Request.Context(), org.ID, branch.ID); err != nil {
			return err
		}
		for _, tableReq := range req.InitialTables {
			capacity := tableReq.Capacity
			if capacity <= 0 {
				capacity = 4
			}
			token, err := crypto.GenerateToken()
			if err != nil {
				return err
			}
			table, err := tx.CreateTable(c.Request.Context(), sqlc.CreateTableParams{
				BranchID:    branch.ID,
				Identifier:  strings.TrimSpace(tableReq.Identifier),
				Capacity:    capacity,
				QrCodeToken: token,
			})
			if err != nil {
				return err
			}
			tables = append(tables, table)
		}
		if req.InitialOwner != nil {
			hash, err := services.HashPIN(req.InitialOwner.PIN)
			if err != nil {
				return err
			}
			staff, err := tx.CreateStaff(c.Request.Context(), sqlc.CreateStaffParams{
				BranchID:  branch.ID,
				Name:      strings.TrimSpace(req.InitialOwner.Name),
				Role:      sqlc.StaffRoleOwner,
				PinHash:   hash,
				StaffCode: services.NormalizePlatformCode(req.InitialOwner.StaffCode),
			})
			if err != nil {
				return err
			}
			if _, err := tx.CreateOrganizationMember(c.Request.Context(), org.ID, staff.ID, "owner"); err != nil {
				return err
			}
			owner = &staff
		}
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, "BRANCH_CONFLICT", "That branch code or an initial owner staff code is already in use. Choose a different one.")
			return
		}
		respondInternalError(c)
		return
	}
	// Tenant logo is restaurant-scoped; set it once the branch exists.
	if req.LogoURL != nil && strings.TrimSpace(*req.LogoURL) != "" {
		if err := h.repos.UpdateRestaurantLogoByBranchID(c.Request.Context(), branch.ID, strings.TrimSpace(*req.LogoURL)); err != nil {
			respondInternalError(c)
			return
		}
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.create", "branch", strconv.FormatInt(branch.ID, 10), org.ID, branch.ID, 0, gin.H{"table_count": len(tables), "initial_owner_created": owner != nil})
	c.JSON(http.StatusCreated, gin.H{
		"branch":        platformBranchResponse(branch),
		"tables":        platformTableResponses(tables),
		"initial_owner": platformStaffResponse(owner),
	})
}

func (h *PlatformHandler) ListBranches(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	branches, err := h.repos.ListBranchesForOrganization(c.Request.Context(), orgID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.list", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"branches": branchResponses(branches)})
}

func (h *PlatformHandler) GetBranch(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	branchID, ok := parseInt64Param(c, "branch_id", "invalid branch id")
	if !ok {
		return
	}
	branch, err := h.repos.GetBranchByID(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "branch not found")
		return
	}
	resp := platformBranchResponse(branch)
	// Surface restaurant name + logo for collateral/branding consumers (additive).
	if restaurant, rerr := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID); rerr == nil {
		resp["restaurant_name"] = restaurant.Name
		resp["restaurant_slug"] = restaurant.Slug
		resp["logo_url"] = textOrEmpty(restaurant.LogoUrl)
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.read", "branch", strconv.FormatInt(branch.ID, 10), branch.OrganizationID, branch.ID, 0, gin.H{})
	c.JSON(http.StatusOK, resp)
}

type updatePlatformBranchRequest struct {
	Name        *string `json:"name"`
	Timezone    *string `json:"timezone"`
	BranchCode  *string `json:"branch_code"`
	OrderPrefix *string `json:"order_prefix"`
	LogoURL     *string `json:"logo_url"`
}

// PATCH /platform/branches/:branch_id — super_admin. Edits a branch's identity
// (including the branch code) and the tenant logo. Only sent fields change.
func (h *PlatformHandler) UpdateBranch(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	branchID, ok := parseInt64Param(c, "branch_id", "invalid branch id")
	if !ok {
		return
	}
	branch, err := h.repos.GetBranchByID(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "branch not found")
		return
	}
	var req updatePlatformBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	// Identity fields: fall back to current values for anything not sent.
	name := branch.Name
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			respondValidationError(c, "name cannot be empty")
			return
		}
		name = strings.TrimSpace(*req.Name)
	}
	timezone := branch.Timezone
	if req.Timezone != nil && strings.TrimSpace(*req.Timezone) != "" {
		timezone = strings.TrimSpace(*req.Timezone)
	}
	branchCode := branch.BranchCode
	if req.BranchCode != nil {
		bc := services.NormalizePlatformCode(*req.BranchCode)
		if len(bc) < 2 {
			respondValidationError(c, "branch code must be at least 2 characters")
			return
		}
		branchCode = bc
	}
	orderPrefix := branch.OrderPrefix
	if req.OrderPrefix != nil {
		op := services.NormalizePlatformCode(*req.OrderPrefix)
		if op == "" {
			op = branch.OrderPrefix
		}
		orderPrefix = op
	}

	updated, err := h.repos.UpdatePlatformBranch(c.Request.Context(), branchID, name, timezone, branchCode, orderPrefix)
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateBranchCode) {
			respondError(c, http.StatusConflict, "BRANCH_CODE_EXISTS", "That branch code is already in use. Choose a different one.")
			return
		}
		respondInternalError(c)
		return
	}
	if req.LogoURL != nil {
		if err := h.repos.UpdateRestaurantLogoByBranchID(c.Request.Context(), branchID, strings.TrimSpace(*req.LogoURL)); err != nil {
			respondInternalError(c)
			return
		}
	}

	resp := platformBranchResponse(updated)
	if restaurant, rerr := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID); rerr == nil {
		resp["restaurant_name"] = restaurant.Name
		resp["restaurant_slug"] = restaurant.Slug
		resp["logo_url"] = textOrEmpty(restaurant.LogoUrl)
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.update", "branch", strconv.FormatInt(updated.ID, 10), updated.OrganizationID, updated.ID, 0, gin.H{"branch_code": branchCode})
	c.JSON(http.StatusOK, resp)
}

func (h *PlatformHandler) SearchSupport(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		respondValidationError(c, "q is required")
		return
	}
	results, err := h.repos.SearchSupportReferences(c.Request.Context(), query, 20)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support.search", "support_reference", query, 0, 0, 0, gin.H{"result_count": len(results)})
	c.JSON(http.StatusOK, gin.H{"results": results})
}

const (
	supportSessionSoftCap = 4 * time.Hour
	supportSessionHardCap = 24 * time.Hour
)

type createPlatformSupportSessionRequest struct {
	OrganizationID           int64      `json:"organization_id" binding:"required"`
	BranchID                 *int64     `json:"branch_id"`
	Reason                   string     `json:"reason" binding:"required,min=8,max=500"`
	ApprovedByPlatformUserID *int64     `json:"approved_by_platform_user_id"`
	StartsAt                 *time.Time `json:"starts_at"`
	ExpiresAt                time.Time  `json:"expires_at" binding:"required"`
}

func (h *PlatformHandler) CreateSupportSession(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin)
	if !ok {
		return
	}
	var req createPlatformSupportSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	startsAt := time.Now().UTC()
	if req.StartsAt != nil {
		startsAt = req.StartsAt.UTC()
	}
	if !req.ExpiresAt.After(startsAt) {
		respondValidationError(c, "expires_at must be after starts_at")
		return
	}
	// Per security-hardening-checklist §14: soft cap 4h, hard cap 24h. The
	// hard cap is non-negotiable; the soft cap is enforced because operators
	// should not be creating multi-day support windows without an explicit
	// cap increase by Platform Engineering.
	duration := req.ExpiresAt.Sub(startsAt)
	if duration > supportSessionHardCap {
		respondValidationError(c, "expires_at exceeds 24h hard cap")
		return
	}
	if duration > supportSessionSoftCap {
		respondError(c, http.StatusUnprocessableEntity, "SUPPORT_DURATION_EXCEEDS_SOFT_CAP", "support session may not exceed 4h without elevated approval")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		respondValidationError(c, "reason is required")
		return
	}
	if _, err := h.repos.GetOrganizationByID(c.Request.Context(), req.OrganizationID); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	var branchID pgtype.Int8
	if req.BranchID != nil {
		branch, err := h.repos.GetBranchByID(c.Request.Context(), *req.BranchID)
		if err != nil || branch.OrganizationID != req.OrganizationID {
			respondValidationError(c, "branch must belong to organization")
			return
		}
		branchID = pgtype.Int8{Int64: *req.BranchID, Valid: true}
	}
	var approvedBy pgtype.Int8
	if req.ApprovedByPlatformUserID != nil {
		approvedBy = pgtype.Int8{Int64: *req.ApprovedByPlatformUserID, Valid: true}
	}
	support, err := h.repos.CreatePlatformSupportSession(c.Request.Context(), sqlc.CreatePlatformSupportSessionParams{
		PlatformUserID:           session.PlatformUserID,
		OrganizationID:           req.OrganizationID,
		BranchID:                 branchID,
		Reason:                   strings.TrimSpace(req.Reason),
		ApprovedByPlatformUserID: approvedBy,
		StartsAt:                 startsAt,
		ExpiresAt:                req.ExpiresAt.UTC(),
	})
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support_sessions.create", "platform_support_session", strconv.FormatInt(support.ID, 10), support.OrganizationID, support.BranchID.Int64, support.ID, gin.H{"reason": support.Reason})
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		OrganizationID: support.OrganizationID,
		BranchID:       support.BranchID.Int64,
		ResourceType:   audit.ResourcePlatformSupportSession,
		ResourceID:     strconv.FormatInt(support.ID, 10),
		Action:         audit.ActionPlatformSupportAccess,
		ActorType:      audit.ActorTypePlatformUser,
		ActorID:        strconv.FormatInt(session.PlatformUserID, 10),
		RiskLevel:      audit.RiskCritical,
		Result:         audit.ResultSuccess,
	})
	c.JSON(http.StatusCreated, platformSupportSessionResponse(support))
}

func (h *PlatformHandler) ListSupportSessions(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	sessions, err := h.repos.ListPlatformSupportSessions(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, 0, len(sessions))
	for _, support := range sessions {
		out = append(out, platformSupportSessionResponse(support))
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support_sessions.list", "platform_support_session", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"support_sessions": out})
}

func (h *PlatformHandler) GetSupportSession(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	id, ok := parseInt64Param(c, "id", "invalid support session id")
	if !ok {
		return
	}
	support, err := h.repos.GetPlatformSupportSessionByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "support session not found")
		return
	}
	if time.Now().UTC().After(support.ExpiresAt) {
		// Treat expired sessions as not-found from the API surface so a stale
		// link can't be used to enumerate prior support windows. The audit
		// trail still records the attempt for forensic review.
		h.logPlatformAudit(c, session.PlatformUserID, "platform.support_sessions.read_expired", "platform_support_session", strconv.FormatInt(support.ID, 10), support.OrganizationID, support.BranchID.Int64, support.ID, gin.H{})
		respondError(c, http.StatusGone, "SUPPORT_SESSION_EXPIRED", "support session has expired")
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support_sessions.read", "platform_support_session", strconv.FormatInt(support.ID, 10), support.OrganizationID, support.BranchID.Int64, support.ID, gin.H{})
	// Tenant-visible audit: organization owners can see when platform support
	// looked at their data. The platform audit log above is internal only;
	// this entry shows up in the org-scoped audit feed.
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		OrganizationID: support.OrganizationID,
		BranchID:       support.BranchID.Int64,
		ResourceType:   audit.ResourcePlatformSupportSession,
		ResourceID:     strconv.FormatInt(support.ID, 10),
		Action:         audit.ActionPlatformSupportAccess,
		ActorType:      audit.ActorTypePlatformUser,
		ActorID:        strconv.FormatInt(session.PlatformUserID, 10),
		RiskLevel:      audit.RiskCritical,
		Result:         audit.ResultSuccess,
		Metadata:       map[string]any{"support_session_id": support.ID, "expires_at": support.ExpiresAt},
	})
	c.JSON(http.StatusOK, platformSupportSessionResponse(support))
}

func (h *PlatformHandler) ListAudit(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	params, ok := auditV2PlatformFilters(c)
	if !ok {
		return
	}
	rows, err := h.repos.ListAuditLogPlatform(c.Request.Context(), params)
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditV2PlatformResponse(row))
	}
	c.JSON(http.StatusOK, gin.H{"audit": out})
	if h.audit != nil {
		h.audit.Record(c.Request.Context(), platformAuditReadEvent(session))
	}
}

func (h *PlatformHandler) requirePlatformRole(c *gin.Context, roles ...string) (services.PlatformSession, bool) {
	session, ok := middleware.GetPlatformSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "platform authentication required")
		return services.PlatformSession{}, false
	}
	if len(roles) == 0 {
		roles = []string{services.PlatformRoleSuperAdmin}
	}
	if !services.PlatformHasRole(session, roles...) {
		h.logPlatformAudit(c, session.PlatformUserID, "platform.authz.denied", "platform_route", c.FullPath(), 0, 0, 0, gin.H{"roles": session.Roles})
		respondError(c, http.StatusForbidden, CodeForbidden, "platform access denied")
		return services.PlatformSession{}, false
	}
	return session, true
}

func (h *PlatformHandler) requireAnyPlatformRole(c *gin.Context, roles ...string) (services.PlatformSession, bool) {
	return h.requirePlatformRole(c, roles...)
}

func (h *PlatformHandler) logPlatformAudit(c *gin.Context, platformUserID int64, action, targetType, targetID string, organizationID, branchID, supportSessionID int64, payload any) {
	requestID, _ := c.Get(middleware.RequestIDKey)
	h.repos.LogPlatformAudit(c.Request.Context(), repository.PlatformAuditParams{
		PlatformUserID:   platformUserID,
		Action:           action,
		TargetType:       targetType,
		TargetID:         targetID,
		OrganizationID:   organizationID,
		BranchID:         branchID,
		SupportSessionID: supportSessionID,
		RequestID:        stringValue(requestID),
		Payload:          payload,
	})
}

func platformUserResponse(user sqlc.PlatformUser, roles []string) gin.H {
	return gin.H{
		"id":           user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"status":       user.Status,
		"mfa_required": user.MfaRequired,
		"roles":        roles,
		"created_at":   user.CreatedAt,
		"updated_at":   user.UpdatedAt,
	}
}

func restaurantResponse(restaurant sqlc.Restaurant) gin.H {
	return gin.H{
		"id":              restaurant.ID,
		"organization_id": restaurant.OrganizationID,
		"name":            restaurant.Name,
		"slug":            restaurant.Slug,
		"settings":        restaurant.SettingsJson,
		"created_at":      restaurant.CreatedAt,
	}
}

// textOrEmpty returns the string value of a nullable pgtype.Text, or "" when null.
func textOrEmpty(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func platformBranchResponse(branch sqlc.Branch) gin.H {
	return gin.H{
		"id":                      branch.ID,
		"restaurant_id":           branch.RestaurantID,
		"organization_id":         branch.OrganizationID,
		"name":                    branch.Name,
		"address":                 branch.Address,
		"timezone":                branch.Timezone,
		"branch_code":             branch.BranchCode,
		"status":                  branch.Status,
		"support_metadata":        branch.SupportMetadataJson,
		"session_timeout_minutes": branch.SessionTimeoutMinutes,
		"order_prefix":            branch.OrderPrefix,
		"created_at":              branch.CreatedAt,
	}
}

func platformTableResponses(tables []sqlc.Table) []gin.H {
	out := make([]gin.H, 0, len(tables))
	for _, table := range tables {
		out = append(out, gin.H{
			"id":            table.ID,
			"branch_id":     table.BranchID,
			"identifier":    table.Identifier,
			"capacity":      table.Capacity,
			"qr_code_token": table.QrCodeToken,
			"status":        table.Status,
		})
	}
	return out
}

func platformStaffResponse(staff *sqlc.Staff) gin.H {
	if staff == nil {
		return nil
	}
	return gin.H{
		"id":         staff.ID,
		"branch_id":  staff.BranchID,
		"name":       staff.Name,
		"role":       staff.Role,
		"staff_code": staff.StaffCode,
	}
}

func platformSupportSessionResponse(s sqlc.PlatformSupportSession) gin.H {
	return gin.H{
		"id":                           s.ID,
		"platform_user_id":             s.PlatformUserID,
		"organization_id":              s.OrganizationID,
		"branch_id":                    nullableInt64(s.BranchID),
		"reason":                       s.Reason,
		"approved_by_platform_user_id": nullableInt64(s.ApprovedByPlatformUserID),
		"starts_at":                    s.StartsAt,
		"expires_at":                   s.ExpiresAt,
		"created_at":                   s.CreatedAt,
	}
}

func auditV2PlatformResponse(row sqlc.AuditLog) gin.H {
	return gin.H{
		"id":              row.ID,
		"organization_id": nullableInt64(row.OrganizationID),
		"branch_id":       nullableInt64(row.BranchID),
		"restaurant_id":   nullableInt64(row.RestaurantID),
		"session_id":      nullableUUID(row.SessionID),
		"table_id":        nullableInt64(row.TableID),
		"resource_type":   row.ResourceType,
		"resource_id":     row.ResourceID,
		"action":          row.Action,
		"result":          row.Result,
		"actor_type":      row.ActorType,
		"actor_id":        row.ActorID,
		"actor_display":   row.ActorDisplay,
		"actor_scope":     row.ActorScopeJson,
		"request_id":      row.RequestID,
		"correlation_id":  row.CorrelationID,
		"idempotency_key": row.IdempotencyKey,
		"ip":              row.Ip,
		"user_agent":      row.UserAgent,
		"source":          row.Source,
		"before":          rawJSONOrNil(row.BeforeJson),
		"after":           rawJSONOrNil(row.AfterJson),
		"metadata":        row.MetadataJson,
		"risk_level":      row.RiskLevel,
		"row_hash":        nullableText(row.RowHash),
		"previous_hash":   nullableText(row.PreviousHash),
		"created_at":      row.CreatedAt,
		"event_reference": row.EventReference,
	}
}

func platformAuditReadEvent(session services.PlatformSession) audit.AuditEvent {
	return audit.AuditEvent{
		ResourceType: audit.ResourceAuditLog,
		Action:       audit.ActionAuditRead,
		ActorType:    audit.ActorTypePlatformUser,
		ActorID:      strconv.FormatInt(session.PlatformUserID, 10),
		ActorDisplay: session.Email,
		RiskLevel:    audit.RiskLow,
		Result:       audit.ResultSuccess,
	}
}

func nullableInt64(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullableText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func nullableUUID(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := fmt.Sprintf("%x-%x-%x-%x-%x", v.Bytes[0:4], v.Bytes[4:6], v.Bytes[6:8], v.Bytes[8:10], v.Bytes[10:16])
	return &s
}

func rawJSONOrNil(v []byte) json.RawMessage {
	if len(v) == 0 {
		return nil
	}
	return json.RawMessage(v)
}

func parseInt64Param(c *gin.Context, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		respondValidationError(c, message)
		return 0, false
	}
	return id, true
}

func platformSettingsJSON(c *gin.Context, raw *json.RawMessage) (json.RawMessage, bool) {
	if raw == nil {
		return json.RawMessage(`{}`), true
	}
	var obj map[string]any
	if err := json.Unmarshal(*raw, &obj); err != nil {
		respondValidationError(c, "settings must be a JSON object")
		return nil, false
	}
	return *raw, true
}

func auditV2PlatformFilters(c *gin.Context) (sqlc.ListAuditLogPlatformParams, bool) {
	var p sqlc.ListAuditLogPlatformParams
	if raw := c.Query("organization_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			respondValidationError(c, "invalid organization_id")
			return p, false
		}
		p.OrganizationID = pgtype.Int8{Int64: id, Valid: true}
	}
	if raw := c.Query("branch_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			respondValidationError(c, "invalid branch_id")
			return p, false
		}
		p.BranchID = pgtype.Int8{Int64: id, Valid: true}
	}
	if raw := strings.TrimSpace(c.Query("session_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			respondValidationError(c, "invalid session_id")
			return p, false
		}
		p.SessionID = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	if actorType := strings.TrimSpace(c.Query("actor_type")); actorType != "" {
		p.ActorType = pgtype.Text{String: actorType, Valid: true}
	}
	if result := strings.TrimSpace(c.Query("result")); result != "" {
		p.Result = pgtype.Text{String: result, Valid: true}
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		p.Source = pgtype.Text{String: source, Valid: true}
	}
	if raw := c.Query("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			respondValidationError(c, "from must be RFC3339")
			return p, false
		}
		p.FromTime = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if raw := c.Query("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			respondValidationError(c, "to must be RFC3339")
			return p, false
		}
		p.ToTime = pgtype.Timestamptz{Time: t, Valid: true}
	}
	return p, true
}

func normalizePlatformSlug(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func generatedBranchCode(orgCode string) string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-BR-%d", services.NormalizePlatformCode(orgCode), time.Now().Unix()%100000)
	}
	return fmt.Sprintf("%s-BR-%s", services.NormalizePlatformCode(orgCode), strings.ToUpper(hex.EncodeToString(b[:])))
}
