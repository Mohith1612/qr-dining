package handlers

import (
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct {
	svc *services.AnalyticsService
}

func NewAnalyticsHandler(svc *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

func (h *AnalyticsHandler) parseBranchAndPeriod(c *gin.Context) (branchID int64, period string, ok bool) {
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

// restaurantIDFromContext returns the tenant restaurant_id when enforcement is active,
// or 0 when running in local dev mode (no BASE_DOMAIN).
func restaurantIDFromContext(c *gin.Context) int64 {
	id, _ := middleware.GetTenantRestaurantID(c)
	return id
}

// GetTopItems returns the most ordered menu items for the branch.
// GET /branches/:id/analytics/top-items?period=daily|weekly|monthly
func (h *AnalyticsHandler) GetTopItems(c *gin.Context) {
	branchID, period, ok := h.parseBranchAndPeriod(c)
	if !ok {
		return
	}

	// Staff auth check — staff can only access their own branch.
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	restaurantID := restaurantIDFromContext(c)
	rows, err := h.svc.GetTopItems(c.Request.Context(), restaurantID, branchID, period)
	if err != nil {
		if services.IsAnalyticsGated(err) {
			respondError(c, http.StatusForbidden, CodeAnalyticsGated, "analytics is not available on your current plan")
			return
		}
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "items": rows})
}

// GetBusyHours returns order volume per hour of day for the branch.
// GET /branches/:id/analytics/busy-hours?period=daily|weekly|monthly
func (h *AnalyticsHandler) GetBusyHours(c *gin.Context) {
	branchID, period, ok := h.parseBranchAndPeriod(c)
	if !ok {
		return
	}

	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	restaurantID := restaurantIDFromContext(c)
	rows, err := h.svc.GetBusyHours(c.Request.Context(), restaurantID, branchID, period)
	if err != nil {
		if services.IsAnalyticsGated(err) {
			respondError(c, http.StatusForbidden, CodeAnalyticsGated, "analytics is not available on your current plan")
			return
		}
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "hours": rows})
}

// GetOrderVolume returns daily order count and revenue for the branch.
// GET /branches/:id/analytics/order-volume?period=daily|weekly|monthly
func (h *AnalyticsHandler) GetOrderVolume(c *gin.Context) {
	branchID, period, ok := h.parseBranchAndPeriod(c)
	if !ok {
		return
	}

	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	restaurantID := restaurantIDFromContext(c)
	rows, err := h.svc.GetOrderVolume(c.Request.Context(), restaurantID, branchID, period)
	if err != nil {
		if services.IsAnalyticsGated(err) {
			respondError(c, http.StatusForbidden, CodeAnalyticsGated, "analytics is not available on your current plan")
			return
		}
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "days": rows})
}
