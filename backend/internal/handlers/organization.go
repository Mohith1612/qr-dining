package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type OrganizationHandler struct {
	repos *repository.Repos
	svc   *services.AnalyticsService
	flags config.FeatureFlags
	authz *authz.Authorizer
}

func NewOrganizationHandler(repos *repository.Repos, analyticsSvc *services.AnalyticsService, flags config.FeatureFlags, authorizer *authz.Authorizer) *OrganizationHandler {
	return &OrganizationHandler{repos: repos, svc: analyticsSvc, flags: flags, authz: authorizer}
}

type updateOrganizationRequest struct {
	Name                *string          `json:"name"`
	LegalName           *string          `json:"legal_name"`
	PrimaryContactEmail *string          `json:"primary_contact_email"`
	Settings            *json.RawMessage `json:"settings"`
}

func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	org, actor, _, ok := h.authorizeOrg(c, authz.ActionOrganizationRead, false)
	if !ok {
		return
	}
	_ = actor
	c.JSON(http.StatusOK, organizationResponse(org))
}

func (h *OrganizationHandler) UpdateOrganization(c *gin.Context) {
	org, _, _, ok := h.authorizeOrg(c, authz.ActionOrganizationUpdate, true)
	if !ok {
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
		params.Name = *req.Name
	}
	if req.LegalName != nil {
		params.LegalName = *req.LegalName
	}
	if req.PrimaryContactEmail != nil {
		params.PrimaryContactEmail = *req.PrimaryContactEmail
	}
	if req.Settings != nil {
		var settings map[string]any
		if err := json.Unmarshal(*req.Settings, &settings); err != nil {
			respondValidationError(c, "settings must be a JSON object")
			return
		}
		params.SettingsJson = *req.Settings
	}

	updated, err := h.repos.UpdateOrganizationSettings(c.Request.Context(), params)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, organizationResponse(updated))
}

func (h *OrganizationHandler) ListBranches(c *gin.Context) {
	org, _, _, ok := h.authorizeOrg(c, authz.ActionOrganizationRead, false)
	if !ok {
		return
	}
	branches, err := h.repos.ListBranchesForOrganization(c.Request.Context(), org.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"branches": branchResponses(branches)})
}

func (h *OrganizationHandler) GetTopItems(c *gin.Context) {
	org, _, _, ok := h.authorizeOrg(c, authz.ActionOrganizationRead, false)
	if !ok {
		return
	}
	period, ok := parseOrgAnalyticsPeriod(c)
	if !ok {
		return
	}
	restaurant, err := h.repos.GetRestaurantByOrganizationID(c.Request.Context(), org.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	rows, err := h.svc.GetOrganizationTopItems(c.Request.Context(), restaurant.ID, org.ID, period)
	if err != nil {
		organizationAnalyticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "items": rows})
}

func (h *OrganizationHandler) GetBusyHours(c *gin.Context) {
	org, _, _, ok := h.authorizeOrg(c, authz.ActionOrganizationRead, false)
	if !ok {
		return
	}
	period, ok := parseOrgAnalyticsPeriod(c)
	if !ok {
		return
	}
	restaurant, err := h.repos.GetRestaurantByOrganizationID(c.Request.Context(), org.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	rows, err := h.svc.GetOrganizationBusyHours(c.Request.Context(), restaurant.ID, org.ID, period)
	if err != nil {
		organizationAnalyticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "hours": rows})
}

func (h *OrganizationHandler) GetOrderVolume(c *gin.Context) {
	org, _, _, ok := h.authorizeOrg(c, authz.ActionOrganizationRead, false)
	if !ok {
		return
	}
	period, ok := parseOrgAnalyticsPeriod(c)
	if !ok {
		return
	}
	restaurant, err := h.repos.GetRestaurantByOrganizationID(c.Request.Context(), org.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	rows, err := h.svc.GetOrganizationOrderVolume(c.Request.Context(), restaurant.ID, org.ID, period)
	if err != nil {
		organizationAnalyticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "days": rows})
}

func (h *OrganizationHandler) authorizeOrg(c *gin.Context, action authz.Action, ownerOnly bool) (sqlc.Organization, authz.Actor, sqlc.OrganizationMember, bool) {
	if !h.flags.TenancyOrganizationsEnabled {
		respondError(c, http.StatusNotFound, CodeFeatureDisabled, "organization tenancy is disabled")
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}

	orgID, err := strconv.ParseInt(c.Param("org_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid organization id")
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}

	org, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID)
	if err != nil {
		if errors.Is(err, domain.ErrTenantNotFound) {
			respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
			return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
		}
		respondInternalError(c)
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}
	if org.Status != "active" {
		respondError(c, http.StatusForbidden, CodeForbidden, "organization is not active")
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}

	member, err := h.repos.GetOrganizationMembershipForStaff(c.Request.Context(), org.ID, sess.StaffID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			respondError(c, http.StatusForbidden, CodeForbidden, "organization access denied")
			return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
		}
		respondInternalError(c)
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}
	if ownerOnly && member.Role != "owner" {
		respondError(c, http.StatusForbidden, CodeForbidden, "organization owner required")
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}

	actor := authz.StaffActor(sess.StaffID, sess.Role, sess.BranchID, org.ID, sess.SessionID.String())
	if !requireAuthorized(c, h.repos, h.authz, actor, action, authz.OrganizationResource(org.ID)) {
		return sqlc.Organization{}, authz.Actor{}, sqlc.OrganizationMember{}, false
	}

	return org, actor, member, true
}

func parseOrgAnalyticsPeriod(c *gin.Context) (string, bool) {
	p := c.DefaultQuery("period", "weekly")
	switch p {
	case "daily", "weekly", "monthly":
		return p, true
	default:
		respondValidationError(c, "period must be daily, weekly, or monthly")
		return "", false
	}
}

func organizationAnalyticsError(c *gin.Context, err error) {
	if services.IsAnalyticsGated(err) {
		respondError(c, http.StatusForbidden, CodeAnalyticsGated, "analytics is not available on your current plan")
		return
	}
	respondInternalError(c)
}

func organizationResponse(org sqlc.Organization) gin.H {
	return gin.H{
		"id":                    org.ID,
		"code":                  org.Code,
		"name":                  org.Name,
		"legal_name":            org.LegalName,
		"status":                org.Status,
		"primary_contact_email": org.PrimaryContactEmail,
		"settings":              org.SettingsJson,
		"created_at":            org.CreatedAt,
		"updated_at":            org.UpdatedAt,
	}
}

func branchResponses(branches []sqlc.Branch) []gin.H {
	out := make([]gin.H, 0, len(branches))
	for _, branch := range branches {
		out = append(out, gin.H{
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
		})
	}
	return out
}
