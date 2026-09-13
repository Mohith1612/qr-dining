package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Read-only support-console inspection endpoints. Reads are gated to support_admin
// / read_only_auditor (super_admin bypasses). Every read writes an internal
// platform_audit_log row; session and payment detail (the richest PII/financial
// views) additionally emit a tenant-visible audit_log row so organizations can see
// when platform viewed their data. NO mutations occur on any of these paths.

// GetSessionDetail — GET /platform/sessions/:id
func (h *PlatformHandler) GetSessionDetail(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	detail, err := h.support.GetSessionDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			respondError(c, http.StatusNotFound, CodeSessionNotFound, "session not found")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support.session.read", "session", id.String(),
		detail.Organization.ID, detail.Branch.ID, 0, gin.H{"session_number": detail.Session.SessionNumber})
	h.recordTenantVisibleAccess(c, session.PlatformUserID, audit.ResourceSession, id.String(), detail.Organization.ID, detail.Branch.ID)
	c.JSON(http.StatusOK, detail)
}

// GetOrderDetail — GET /platform/orders/:id
func (h *PlatformHandler) GetOrderDetail(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid order id")
		return
	}
	detail, err := h.support.GetOrderDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(c, http.StatusNotFound, CodeOrderNotFound, "order not found")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support.order.read", "order", id.String(),
		detail.Organization.ID, detail.Branch.ID, 0, gin.H{"order_operational_id": detail.Order.OrderOperationalID})
	c.JSON(http.StatusOK, detail)
}

// GetPaymentDetail — GET /platform/payments/:id
func (h *PlatformHandler) GetPaymentDetail(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return
	}
	id, ok := parseInt64Param(c, "id", "invalid payment id")
	if !ok {
		return
	}
	detail, err := h.support.GetPaymentDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			respondError(c, http.StatusNotFound, CodePaymentNotFound, "payment not found")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.support.payment.read", "payment", strconv.FormatInt(id, 10),
		detail.Organization.ID, detail.Branch.ID, 0, gin.H{"payment_reference": detail.Payment.PaymentReference})
	h.recordTenantVisibleAccess(c, session.PlatformUserID, audit.ResourcePayment, strconv.FormatInt(id, 10), detail.Organization.ID, detail.Branch.ID)
	c.JSON(http.StatusOK, detail)
}

// recordTenantVisibleAccess emits a tenant-visible, critical-risk audit_log row so
// organizations can see when a platform operator viewed their data (mirrors the
// support-session access pattern). No-op if the audit writer is disabled.
func (h *PlatformHandler) recordTenantVisibleAccess(c *gin.Context, platformUserID int64, resourceType, resourceID string, orgID, branchID int64) {
	if h.audit == nil {
		return
	}
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		OrganizationID: orgID,
		BranchID:       branchID,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Action:         audit.ActionPlatformSupportAccess,
		ActorType:      audit.ActorTypePlatformUser,
		ActorID:        strconv.FormatInt(platformUserID, 10),
		RiskLevel:      audit.RiskCritical,
		Result:         audit.ResultSuccess,
		Metadata:       map[string]any{"surface": "support_console"},
	})
}
