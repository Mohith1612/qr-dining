package handlers

import (
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// Enforcement observability — read-only operator surfaces that show where enforcement
// WOULD bite (expiring/suspended subscriptions, over-limit tenants, flag override usage).
// Reads only; nothing here enforces. RBAC: support/billing/auditor (super bypass).

// GetSubscriptionObservability lists subscriptions that would be affected by enforcement.
// GET /platform/observability/subscriptions
func (h *PlatformHandler) GetSubscriptionObservability(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	report, err := h.obs.SubscriptionObservability(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.observability.subscriptions.read", "observability", "subscriptions", 0, 0, 0, gin.H{"alerts": len(report.Alerts)})
	c.JSON(http.StatusOK, report)
}

// GetEntitlementObservability lists tenants over their resolved entitlement limits.
// GET /platform/observability/entitlements
func (h *PlatformHandler) GetEntitlementObservability(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	report, err := h.obs.EntitlementObservability(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.observability.entitlements.read", "observability", "entitlements", 0, 0, 0, gin.H{"breaches": len(report.Breaches)})
	c.JSON(http.StatusOK, report)
}

// GetFlagObservability reports feature-flag override usage and orphaned catalog flags.
// GET /platform/observability/flags
func (h *PlatformHandler) GetFlagObservability(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	report, err := h.obs.FlagObservability(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.observability.flags.read", "observability", "flags", 0, 0, 0, gin.H{"in_use": len(report.InUse), "orphaned": len(report.Orphaned)})
	c.JSON(http.StatusOK, report)
}
