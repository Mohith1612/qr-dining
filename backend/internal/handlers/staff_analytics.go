package handlers

import (
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// StaffAnalyticsHandler serves manager-facing staff performance analytics.
// Owner/manager only; access is additionally gated per branch by the
// analytics.staff_performance entitlement + staff_performance_analytics flag
// (403 STAFF_ANALYTICS_DISABLED when the gate is off).
type StaffAnalyticsHandler struct {
	svc *services.StaffAnalyticsService
}

func NewStaffAnalyticsHandler(svc *services.StaffAnalyticsService) *StaffAnalyticsHandler {
	return &StaffAnalyticsHandler{svc: svc}
}

func (h *StaffAnalyticsHandler) parseBranchAndPeriod(c *gin.Context) (branchID int64, period string, ok bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return 0, "", false
	}
	p := c.DefaultQuery("period", "weekly")
	switch p {
	case "daily", "weekly", "monthly":
	default:
		respondValidationError(c, "period must be daily, weekly, or monthly")
		return 0, "", false
	}
	return id, p, true
}

// requireBranchManager enforces staff auth, branch ownership, and owner/manager role.
func (h *StaffAnalyticsHandler) requireBranchManager(c *gin.Context, branchID int64) bool {
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return false
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return false
	}
	if staffSession.Role != sqlc.StaffRoleOwner && staffSession.Role != sqlc.StaffRoleManager {
		respondError(c, http.StatusForbidden, CodeForbidden, "owner or manager role required")
		return false
	}
	return true
}

func (h *StaffAnalyticsHandler) respondStaffAnalyticsError(c *gin.Context, err error) {
	if services.IsStaffAnalyticsDisabled(err) {
		respondError(c, http.StatusForbidden, CodeStaffAnalyticsDisabled, "staff performance analytics is not enabled for this organization")
		return
	}
	respondInternalError(c)
}

// GetWaiterPerformance returns per-staff front-of-house metrics.
// GET /branches/:id/analytics/staff/waiters?period=daily|weekly|monthly
func (h *StaffAnalyticsHandler) GetWaiterPerformance(c *gin.Context) {
	branchID, period, ok := h.parseBranchAndPeriod(c)
	if !ok {
		return
	}
	if !h.requireBranchManager(c, branchID) {
		return
	}
	rows, err := h.svc.GetWaiterPerformance(c.Request.Context(), branchID, period)
	if err != nil {
		h.respondStaffAnalyticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "waiters": rows})
}

// GetKitchenPerformance returns per-staff kitchen metrics.
// GET /branches/:id/analytics/staff/kitchen?period=daily|weekly|monthly
func (h *StaffAnalyticsHandler) GetKitchenPerformance(c *gin.Context) {
	branchID, period, ok := h.parseBranchAndPeriod(c)
	if !ok {
		return
	}
	if !h.requireBranchManager(c, branchID) {
		return
	}
	rows, err := h.svc.GetKitchenPerformance(c.Request.Context(), branchID, period)
	if err != nil {
		h.respondStaffAnalyticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "kitchen": rows})
}

// GetStaffSummary returns the per-staff daily activity summary.
// GET /branches/:id/analytics/staff/summary?period=daily|weekly|monthly
func (h *StaffAnalyticsHandler) GetStaffSummary(c *gin.Context) {
	branchID, period, ok := h.parseBranchAndPeriod(c)
	if !ok {
		return
	}
	if !h.requireBranchManager(c, branchID) {
		return
	}
	rows, err := h.svc.GetStaffSummary(c.Request.Context(), branchID, period)
	if err != nil {
		h.respondStaffAnalyticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "summary": rows})
}
