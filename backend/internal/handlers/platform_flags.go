package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

// Platform feature-flag targeting. Management endpoints live on the /platform
// group (platform auth + audit); a single public resolve endpoint serves the
// guest frontend. This system is separate from the env-driven strict-rollout flags.

type createFlagRequest struct {
	Key            string `json:"key" binding:"required"`
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	DefaultEnabled bool   `json:"default_enabled"`
}

type updateFlagRequest struct {
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	DefaultEnabled *bool   `json:"default_enabled"`
}

type setFlagOverrideRequest struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// ListFlags returns the flag catalog with any global override value.
// GET /platform/flags
func (h *PlatformHandler) ListFlags(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	catalog, err := h.repos.ListFeatureFlags(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	globalRows, err := h.repos.ListGlobalFlagOverrides(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	globals := make(map[string]bool, len(globalRows))
	for _, g := range globalRows {
		globals[g.FlagKey] = g.Enabled
	}
	out := make([]gin.H, len(catalog))
	for i, f := range catalog {
		var globalOverride *bool
		if v, has := globals[f.Key]; has {
			globalOverride = &v
		}
		out[i] = gin.H{
			"key":             f.Key,
			"name":            f.Name,
			"description":     f.Description,
			"default_enabled": f.DefaultEnabled,
			"global_override": globalOverride,
		}
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.flags.list", "feature_flag", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"flags": out})
}

// CreateFlag adds a flag to the catalog. POST /platform/flags — super_admin.
func (h *PlatformHandler) CreateFlag(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	var req createFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		respondValidationError(c, "key is required")
		return
	}
	flag, err := h.repos.CreateFeatureFlag(c.Request.Context(), key, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), req.DefaultEnabled)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, "FLAG_EXISTS", "a flag with this key already exists")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.flags.create", "feature_flag", flag.Key, 0, 0, 0, gin.H{"default_enabled": flag.DefaultEnabled})
	c.JSON(http.StatusCreated, flagResponse(flag.Key, flag.Name, flag.Description, flag.DefaultEnabled))
}

// UpdateFlag updates a flag's name/description/default. PATCH /platform/flags/:key — super_admin.
func (h *PlatformHandler) UpdateFlag(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	existing, err := h.repos.GetFeatureFlag(c.Request.Context(), key)
	if err != nil {
		respondError(c, http.StatusNotFound, "FLAG_NOT_FOUND", "feature flag not found")
		return
	}
	var req updateFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	name, desc, def := existing.Name, existing.Description, existing.DefaultEnabled
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		desc = strings.TrimSpace(*req.Description)
	}
	if req.DefaultEnabled != nil {
		def = *req.DefaultEnabled
	}
	updated, err := h.repos.UpdateFeatureFlag(c.Request.Context(), key, name, desc, def)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.flags.update", "feature_flag", key, 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, flagResponse(updated.Key, updated.Name, updated.Description, updated.DefaultEnabled))
}

// SetGlobalFlagOverride sets the global override. PUT /platform/flags/:key/global — super_admin.
func (h *PlatformHandler) SetGlobalFlagOverride(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if _, err := h.repos.GetFeatureFlag(c.Request.Context(), key); err != nil {
		respondError(c, http.StatusNotFound, "FLAG_NOT_FOUND", "feature flag not found")
		return
	}
	var req setFlagOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	uid := session.PlatformUserID
	if _, err := h.repos.UpsertGlobalFlagOverride(c.Request.Context(), key, req.Enabled, &uid); err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.flags.global.set", "feature_flag", key, 0, 0, 0, gin.H{"enabled": req.Enabled})
	c.JSON(http.StatusOK, gin.H{"flag_key": key, "scope": "global", "enabled": req.Enabled})
}

// ClearGlobalFlagOverride removes the global override. DELETE /platform/flags/:key/global — super_admin.
func (h *PlatformHandler) ClearGlobalFlagOverride(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if err := h.repos.DeleteGlobalFlagOverride(c.Request.Context(), key); err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.flags.global.clear", "feature_flag", key, 0, 0, 0, gin.H{})
	c.Status(http.StatusNoContent)
}

// SetOrganizationFlagOverride sets an org override. PUT /platform/organizations/:org_id/flags/:key — super_admin.
func (h *PlatformHandler) SetOrganizationFlagOverride(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if _, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	if _, err := h.repos.GetFeatureFlag(c.Request.Context(), key); err != nil {
		respondError(c, http.StatusNotFound, "FLAG_NOT_FOUND", "feature flag not found")
		return
	}
	var req setFlagOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	uid := session.PlatformUserID
	if _, err := h.repos.UpsertOrganizationFlagOverride(c.Request.Context(), orgID, key, req.Enabled, strings.TrimSpace(req.Reason), &uid); err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.flag.set", "feature_flag", key, orgID, 0, 0, gin.H{"enabled": req.Enabled})
	c.JSON(http.StatusOK, gin.H{"flag_key": key, "scope": "organization", "organization_id": orgID, "enabled": req.Enabled})
}

// ClearOrganizationFlagOverride removes an org override. DELETE /platform/organizations/:org_id/flags/:key — super_admin.
func (h *PlatformHandler) ClearOrganizationFlagOverride(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if err := h.repos.DeleteOrganizationFlagOverride(c.Request.Context(), orgID, key); err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.flag.clear", "feature_flag", key, orgID, 0, 0, gin.H{})
	c.Status(http.StatusNoContent)
}

// SetBranchFlagOverride sets a branch override. PUT /platform/branches/:branch_id/flags/:key — super_admin.
func (h *PlatformHandler) SetBranchFlagOverride(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	branchID, ok := parseInt64Param(c, "branch_id", "invalid branch id")
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	branch, err := h.repos.GetBranchByID(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "branch not found")
		return
	}
	if _, err := h.repos.GetFeatureFlag(c.Request.Context(), key); err != nil {
		respondError(c, http.StatusNotFound, "FLAG_NOT_FOUND", "feature flag not found")
		return
	}
	var req setFlagOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	uid := session.PlatformUserID
	if _, err := h.repos.UpsertBranchFlagOverride(c.Request.Context(), branchID, key, req.Enabled, strings.TrimSpace(req.Reason), &uid); err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branch.flag.set", "feature_flag", key, branch.OrganizationID, branchID, 0, gin.H{"enabled": req.Enabled})
	c.JSON(http.StatusOK, gin.H{"flag_key": key, "scope": "branch", "branch_id": branchID, "enabled": req.Enabled})
}

// ClearBranchFlagOverride removes a branch override. DELETE /platform/branches/:branch_id/flags/:key — super_admin.
func (h *PlatformHandler) ClearBranchFlagOverride(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	branchID, ok := parseInt64Param(c, "branch_id", "invalid branch id")
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if err := h.repos.DeleteBranchFlagOverride(c.Request.Context(), branchID, key); err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branch.flag.clear", "feature_flag", key, 0, branchID, 0, gin.H{})
	c.Status(http.StatusNoContent)
}

// GetOrganizationFlags returns resolved flags for an org. GET /platform/organizations/:org_id/flags.
func (h *PlatformHandler) GetOrganizationFlags(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
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
	states, err := h.flag.ResolveForOrganization(c.Request.Context(), orgID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.flags.read", "feature_flag", "", orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"organization_id": orgID, "flags": states})
}

// GetBranchFlags returns resolved flags for a branch. GET /platform/branches/:branch_id/flags.
func (h *PlatformHandler) GetBranchFlags(c *gin.Context) {
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
	states, err := h.flag.ResolveForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branch.flags.read", "feature_flag", "", branch.OrganizationID, branchID, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"branch_id": branchID, "flags": states})
}

func flagResponse(key, name, description string, defaultEnabled bool) gin.H {
	return gin.H{"key": key, "name": name, "description": description, "default_enabled": defaultEnabled}
}

// ── Public resolve handler (guest frontend) ──────────────────────────────────

// FlagHandler serves the public per-branch resolved-flags endpoint.
type FlagHandler struct {
	flag  *services.FlagService
	cache *redis.Cache
}

func NewFlagHandler(flag *services.FlagService, cache *redis.Cache) *FlagHandler {
	return &FlagHandler{flag: flag, cache: cache}
}

// ResolveForBranch returns resolved flag values for a branch as a {key: enabled} map.
// GET /branches/:id/feature-flags — public (branch tenant-guarded).
func (h *FlagHandler) ResolveForBranch(c *gin.Context) {
	branchID, ok := parseInt64Param(c, "id", "invalid branch id")
	if !ok {
		return
	}
	cacheKey := "flags:branch:" + strconv.FormatInt(branchID, 10)
	var values map[string]bool
	if hit, _ := h.cache.Get(c.Request.Context(), cacheKey, &values); hit {
		c.JSON(http.StatusOK, gin.H{"feature_flags": values})
		return
	}
	states, err := h.flag.ResolveForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	values = make(map[string]bool, len(states))
	for _, s := range states {
		values[s.Key] = s.Enabled
	}
	_ = h.cache.Set(c.Request.Context(), cacheKey, values, 5*time.Minute)
	c.JSON(http.StatusOK, gin.H{"feature_flags": values})
}
