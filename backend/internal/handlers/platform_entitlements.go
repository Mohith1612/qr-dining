package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Platform plan & entitlement management. All routes sit under the /platform
// group (platform auth; staff tokens rejected). Mutations are audited via
// logPlatformAudit. Resolution itself stays resolve-only (shadow) — these APIs
// configure entitlements; they do not enforce them on operational paths.

var validPlanTiers = map[string]bool{"free": true, "standard": true, "premium": true}

var validAssignmentStatuses = map[string]bool{
	"trial": true, "active": true, "suspended": true, "cancelled": true,
}

type createPlanRequest struct {
	Name         string          `json:"name" binding:"required"`
	Tier         string          `json:"tier" binding:"required"`
	PriceMonthly string          `json:"price_monthly"`
	Features     json.RawMessage `json:"features"`
}

type updatePlanRequest struct {
	Name         *string         `json:"name"`
	PriceMonthly *string         `json:"price_monthly"`
	Features     json.RawMessage `json:"features"`
}

type planEntitlementInput struct {
	Key        string `json:"key" binding:"required"`
	Enabled    *bool  `json:"enabled"`
	LimitValue *int64 `json:"limit_value"`
}

type setPlanEntitlementsRequest struct {
	Entitlements []planEntitlementInput `json:"entitlements"`
}

type assignPlanRequest struct {
	PlanID int64  `json:"plan_id" binding:"required"`
	Status string `json:"status"`
}

type setOverrideRequest struct {
	Enabled    *bool  `json:"enabled"`
	LimitValue *int64 `json:"limit_value"`
	Reason     string `json:"reason"`
}

// ListEntitlements returns the capability/limit catalog.
// GET /platform/entitlements
func (h *PlatformHandler) ListEntitlements(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	rows, err := h.repos.ListEntitlementCatalog(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, len(rows))
	for i, e := range rows {
		out[i] = gin.H{"key": e.Key, "kind": e.Kind, "description": e.Description}
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.entitlements.list", "entitlement", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"entitlements": out})
}

// ListPlatformPlans returns all plans with their entitlement defaults.
// GET /platform/plans
func (h *PlatformHandler) ListPlatformPlans(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	plans, err := h.repos.ListPlans(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, 0, len(plans))
	for _, p := range plans {
		ents, err := h.repos.ListPlanEntitlements(c.Request.Context(), p.ID)
		if err != nil {
			respondInternalError(c)
			return
		}
		out = append(out, planResponseWithEntitlements(p, ents))
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.plans.list", "plan", "", 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"plans": out})
}

// CreatePlan creates a new subscription plan.
// POST /platform/plans — super_admin.
func (h *PlatformHandler) CreatePlan(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	var req createPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	tier := strings.ToLower(strings.TrimSpace(req.Tier))
	if !validPlanTiers[tier] {
		respondValidationError(c, "tier must be one of: free, standard, premium")
		return
	}
	price := strings.TrimSpace(req.PriceMonthly)
	if price == "" {
		price = "0"
	}
	if !validNumeric(price) {
		respondValidationError(c, "price_monthly must be a numeric value")
		return
	}
	features := req.Features
	if len(features) == 0 {
		features = json.RawMessage("{}")
	}
	plan, err := h.repos.CreatePlan(c.Request.Context(), strings.TrimSpace(req.Name), tier, price, features)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, "PLAN_TIER_EXISTS", "a plan for this tier already exists")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.plans.create", "plan", strconv.FormatInt(plan.ID, 10), 0, 0, 0, gin.H{"tier": tier})
	c.JSON(http.StatusCreated, planResponseWithEntitlements(plan, nil))
}

// UpdatePlan updates a plan's name, price, and features.
// PATCH /platform/plans/:plan_id — super_admin or billing_admin.
func (h *PlatformHandler) UpdatePlan(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleBillingAdmin)
	if !ok {
		return
	}
	planID, ok := parseInt64Param(c, "plan_id", "invalid plan id")
	if !ok {
		return
	}
	plan, err := h.repos.GetPlanByID(c.Request.Context(), planID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodePlanNotFound, "plan not found")
		return
	}
	var req updatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	name := plan.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	price := numericToString(plan.PriceMonthly)
	if req.PriceMonthly != nil {
		if !validNumeric(*req.PriceMonthly) {
			respondValidationError(c, "price_monthly must be a numeric value")
			return
		}
		price = strings.TrimSpace(*req.PriceMonthly)
	}
	features := plan.FeaturesJson
	if len(req.Features) != 0 {
		features = req.Features
	}
	updated, err := h.repos.UpdatePlan(c.Request.Context(), planID, name, price, features)
	if err != nil {
		respondInternalError(c)
		return
	}
	ents, _ := h.repos.ListPlanEntitlements(c.Request.Context(), planID)
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.plans.update", "plan", strconv.FormatInt(planID, 10), 0, 0, 0, gin.H{})
	c.JSON(http.StatusOK, planResponseWithEntitlements(updated, ents))
}

// SetPlanEntitlements replaces the entitlement defaults for a plan.
// PUT /platform/plans/:plan_id/entitlements — super_admin.
func (h *PlatformHandler) SetPlanEntitlements(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	planID, ok := parseInt64Param(c, "plan_id", "invalid plan id")
	if !ok {
		return
	}
	if _, err := h.repos.GetPlanByID(c.Request.Context(), planID); err != nil {
		respondError(c, http.StatusNotFound, CodePlanNotFound, "plan not found")
		return
	}
	var req setPlanEntitlementsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	catalog, err := h.repos.ListEntitlementCatalog(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	known := make(map[string]bool, len(catalog))
	for _, e := range catalog {
		known[e.Key] = true
	}
	for _, in := range req.Entitlements {
		if !known[in.Key] {
			respondValidationError(c, "unknown entitlement key: "+in.Key)
			return
		}
	}
	err = h.repos.WithTx(c.Request.Context(), func(tx *repository.Repos) error {
		if derr := tx.DeletePlanEntitlements(c.Request.Context(), planID); derr != nil {
			return derr
		}
		for _, in := range req.Entitlements {
			enabled := true
			if in.Enabled != nil {
				enabled = *in.Enabled
			}
			if _, uerr := tx.UpsertPlanEntitlement(c.Request.Context(), planID, in.Key, enabled, in.LimitValue); uerr != nil {
				return uerr
			}
		}
		return nil
	})
	if err != nil {
		respondInternalError(c)
		return
	}
	ents, _ := h.repos.ListPlanEntitlements(c.Request.Context(), planID)
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.plans.entitlements.set", "plan", strconv.FormatInt(planID, 10), 0, 0, 0, gin.H{"count": len(req.Entitlements)})
	c.JSON(http.StatusOK, gin.H{"plan_id": planID, "entitlements": planEntitlementResponses(ents)})
}

// GetOrganizationEntitlements returns the resolved effective entitlements for an org.
// GET /platform/organizations/:org_id/entitlements
func (h *PlatformHandler) GetOrganizationEntitlements(c *gin.Context) {
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
	eff, err := h.ent.ResolveForOrganization(c.Request.Context(), orgID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.entitlements.read", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"entitlements": eff})
}

// AssignOrganizationPlan assigns (or reassigns) a plan to an organization.
// PUT /platform/organizations/:org_id/plan — super_admin or billing_admin.
func (h *PlatformHandler) AssignOrganizationPlan(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleBillingAdmin)
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
	var req assignPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if _, err := h.repos.GetPlanByID(c.Request.Context(), req.PlanID); err != nil {
		respondError(c, http.StatusNotFound, CodePlanNotFound, "plan not found")
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "active"
	}
	if !validAssignmentStatuses[status] {
		respondValidationError(c, "status must be one of: trial, active, suspended, cancelled")
		return
	}
	uid := session.PlatformUserID
	assignment, err := h.repos.UpsertOrganizationPlanAssignment(c.Request.Context(), orgID, req.PlanID, status, &uid)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.plan.assign", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"plan_id": req.PlanID, "status": status})
	c.JSON(http.StatusOK, gin.H{
		"organization_id": assignment.OrganizationID,
		"plan_id":         assignment.PlanID,
		"status":          assignment.Status,
	})
}

// SetOrganizationEntitlementOverride sets an org-level override for one entitlement key.
// PUT /platform/organizations/:org_id/entitlements/:key — super_admin.
func (h *PlatformHandler) SetOrganizationEntitlementOverride(c *gin.Context) {
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
	if _, err := h.repos.GetEntitlement(c.Request.Context(), key); err != nil {
		respondError(c, http.StatusNotFound, "ENTITLEMENT_NOT_FOUND", "unknown entitlement key")
		return
	}
	var req setOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	uid := session.PlatformUserID
	override, err := h.repos.UpsertOrganizationEntitlementOverride(c.Request.Context(), orgID, key, req.Enabled, req.LimitValue, strings.TrimSpace(req.Reason), &uid)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.featureGate.Invalidate(c.Request.Context())
	h.logPlatformAudit(c, session.PlatformUserID, "platform.org.entitlement.override", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"key": key})
	c.JSON(http.StatusOK, gin.H{
		"organization_id": override.OrganizationID,
		"entitlement_key": override.EntitlementKey,
		"enabled":         entBoolPtr(override.Enabled),
		"limit_value":     entInt64Ptr(override.LimitValue),
		"reason":          override.Reason,
	})
}

// ── response + value helpers ─────────────────────────────────────────────────

func planResponseWithEntitlements(p sqlc.SubscriptionPlan, ents []sqlc.PlanEntitlement) gin.H {
	return gin.H{
		"id":            p.ID,
		"name":          p.Name,
		"tier":          p.Tier,
		"price_monthly": p.PriceMonthly,
		"features_json": p.FeaturesJson,
		"entitlements":  planEntitlementResponses(ents),
	}
}

func planEntitlementResponses(rows []sqlc.PlanEntitlement) []gin.H {
	out := make([]gin.H, len(rows))
	for i, r := range rows {
		out[i] = gin.H{
			"key":         r.EntitlementKey,
			"enabled":     r.Enabled,
			"limit_value": entInt64Ptr(r.LimitValue),
		}
	}
	return out
}

func entInt64Ptr(n pgtype.Int8) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func entBoolPtr(b pgtype.Bool) *bool {
	if !b.Valid {
		return nil
	}
	v := b.Bool
	return &v
}

func validNumeric(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

func numericToString(n pgtype.Numeric) string {
	v, err := n.Value()
	if err != nil || v == nil {
		return "0"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
