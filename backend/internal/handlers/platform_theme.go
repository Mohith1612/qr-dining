package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// Structured theme/branding management (platform-operator-managed). Presets are
// always allowed; custom design tokens require the org's custom.theme entitlement.
// No arbitrary CSS — token keys are allowlisted and values must be hex colors.

type setThemeRequest struct {
	Preset string            `json:"preset" binding:"required"`
	Tokens map[string]string `json:"tokens"`
}

// ListThemePresets returns the preset catalog plus the allowlisted custom-token keys.
// GET /platform/theme/presets
func (h *PlatformHandler) ListThemePresets(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	presets, err := h.repos.ListThemePresets(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, len(presets))
	for i, p := range presets {
		out[i] = gin.H{"key": p.Key, "name": p.Name, "description": p.Description}
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.theme.presets.list", "theme_preset", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"presets": out, "allowed_token_keys": services.AllowedThemeTokenKeys()})
}

// GetOrganizationTheme returns the resolved structured theme for an org.
// GET /platform/organizations/:org_id/theme
func (h *PlatformHandler) GetOrganizationTheme(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	restaurant, err := h.repos.GetRestaurantByOrganizationID(c.Request.Context(), orgID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	theme, err := h.theme.GetThemeForRestaurant(c.Request.Context(), restaurant.ID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.theme.read", "tenant_theme", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"theme": theme})
}

// SetOrganizationTheme sets the structured theme for an org.
// PUT /platform/organizations/:org_id/theme — super_admin. Custom tokens require custom.theme.
func (h *PlatformHandler) SetOrganizationTheme(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	if _, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	var req setThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	uid := session.PlatformUserID
	theme, err := h.theme.SetTheme(c.Request.Context(), orgID, strings.TrimSpace(req.Preset), req.Tokens, &uid)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrThemePresetNotFound):
			respondError(c, http.StatusNotFound, "THEME_PRESET_NOT_FOUND", "unknown theme preset")
		case errors.Is(err, domain.ErrCustomThemeNotEntitled):
			respondError(c, http.StatusForbidden, CodeForbidden, "custom theme tokens require the custom.theme entitlement")
		case errors.Is(err, domain.ErrInvalidThemeToken):
			respondValidationError(c, err.Error())
		default:
			respondInternalError(c)
		}
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.theme.set", "tenant_theme", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"preset": theme.Preset, "custom_token_count": len(theme.Tokens)})
	c.JSON(http.StatusOK, gin.H{"theme": theme})
}

// ── Public resolve handler (guest frontend) ──────────────────────────────────

// ThemeHandler serves the public per-branch resolved-theme endpoint.
type ThemeHandler struct {
	theme *services.ThemeService
	repos *repository.Repos
}

func NewThemeHandler(theme *services.ThemeService, repos *repository.Repos) *ThemeHandler {
	return &ThemeHandler{theme: theme, repos: repos}
}

// ResolveForBranch returns the structured theme plus branding (logo + name) for
// a branch's restaurant. GET /branches/:id/theme — public (branch tenant-guarded).
func (h *ThemeHandler) ResolveForBranch(c *gin.Context) {
	branchID, ok := parseInt64Param(c, "id", "invalid branch id")
	if !ok {
		return
	}
	theme, err := h.theme.GetThemeForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "branch not found")
		return
	}
	var logoURL, restaurantName string
	if restaurant, rerr := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID); rerr == nil {
		logoURL = textOrEmpty(restaurant.LogoUrl)
		restaurantName = restaurant.Name
	}
	c.JSON(http.StatusOK, gin.H{"theme": theme, "logo_url": logoURL, "restaurant_name": restaurantName})
}
