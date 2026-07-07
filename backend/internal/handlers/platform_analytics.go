package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// Platform (cross-tenant) analytics reads. Operator-facing (platform RBAC); NOT
// entitlement-gated (entitlement gating applies to the tenant-facing
// /branches/:id/analytics/* surface). Each accepts an optional organization_id
// query param (omit = platform-wide) and a period (daily|weekly|monthly).
//
// Note: QR scans, websocket reconnects, worker failures, and audit-write failures
// are NOT served here — they are not persisted to Postgres. They remain available
// as Prometheus counters on /metrics and via the configured alert rules.

// optionalOrgID parses an optional organization_id query param. Returns (nil, true)
// when absent, (&id, true) when valid, and writes an error + (nil, false) when invalid
// or when a provided org does not exist.
func (h *PlatformHandler) optionalOrgID(c *gin.Context) (*int64, bool) {
	raw := strings.TrimSpace(c.Query("organization_id"))
	if raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		respondValidationError(c, "organization_id must be an integer")
		return nil, false
	}
	if _, err := h.repos.GetOrganizationByID(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return nil, false
	}
	return &id, true
}

func analyticsAuditPayload(orgID *int64, period, metric string) gin.H {
	p := gin.H{"metric": metric, "period": period}
	if orgID != nil {
		p["organization_id"] = *orgID
	} else {
		p["scope"] = "platform"
	}
	return p
}

// GetUsageAnalytics — GET /platform/analytics/usage
func (h *PlatformHandler) GetUsageAnalytics(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgID, ok := h.optionalOrgID(c)
	if !ok {
		return
	}
	period := services.NormalizePeriod(c.Query("period"))
	report, err := h.analytics.GetUsage(c.Request.Context(), orgID, period)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.analytics.read", "analytics", "usage", orgIDValue(orgID), 0, 0, analyticsAuditPayload(orgID, period, "usage"))
	c.JSON(http.StatusOK, report)
}

// GetRevenueAnalytics — GET /platform/analytics/revenue
func (h *PlatformHandler) GetRevenueAnalytics(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgID, ok := h.optionalOrgID(c)
	if !ok {
		return
	}
	period := services.NormalizePeriod(c.Query("period"))
	report, err := h.analytics.GetRevenue(c.Request.Context(), orgID, period)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.analytics.read", "analytics", "revenue", orgIDValue(orgID), 0, 0, analyticsAuditPayload(orgID, period, "revenue"))
	c.JSON(http.StatusOK, report)
}

// GetHealthAnalytics — GET /platform/analytics/health
func (h *PlatformHandler) GetHealthAnalytics(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	orgID, ok := h.optionalOrgID(c)
	if !ok {
		return
	}
	period := services.NormalizePeriod(c.Query("period"))
	report, err := h.analytics.GetHealth(c.Request.Context(), orgID, period)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.analytics.read", "analytics", "health", orgIDValue(orgID), 0, 0, analyticsAuditPayload(orgID, period, "health"))
	c.JSON(http.StatusOK, report)
}

func orgIDValue(orgID *int64) int64 {
	if orgID == nil {
		return 0
	}
	return *orgID
}
