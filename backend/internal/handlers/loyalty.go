package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// LoyaltyHandler serves the staff-facing loyalty surfaces. All routes are
// branch-scoped (StaffAuth + BranchTenantGuard); the loyalty data itself is
// org-scoped and resolved from the branch. Every operation is gated by the
// loyalty entitlements + flag (403 LOYALTY_DISABLED when off).
//
// Redemption is ledger-only: it records a staff-witnessed points deduction
// and never touches bill or payment math.
type LoyaltyHandler struct {
	svc *services.LoyaltyService
}

func NewLoyaltyHandler(svc *services.LoyaltyService) *LoyaltyHandler {
	return &LoyaltyHandler{svc: svc}
}

// requireBranchStaff enforces staff auth + branch match, and optionally
// owner/manager role.
func (h *LoyaltyHandler) requireBranchStaff(c *gin.Context, managerOnly bool) (services.StaffSession, int64, bool) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return services.StaffSession{}, 0, false
	}
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return services.StaffSession{}, 0, false
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return services.StaffSession{}, 0, false
	}
	if managerOnly && staffSession.Role != sqlc.StaffRoleOwner && staffSession.Role != sqlc.StaffRoleManager {
		respondError(c, http.StatusForbidden, CodeForbidden, "owner or manager role required")
		return services.StaffSession{}, 0, false
	}
	if staffSession.Role == sqlc.StaffRoleKitchen {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return services.StaffSession{}, 0, false
	}
	return staffSession, branchID, true
}

func (h *LoyaltyHandler) respondLoyaltyError(c *gin.Context, err error) {
	switch {
	case services.IsLoyaltyDisabled(err):
		respondError(c, http.StatusForbidden, CodeLoyaltyDisabled, "loyalty is not enabled for this organization")
	case services.IsLoyaltyInsufficientPoints(err):
		respondError(c, http.StatusConflict, CodeLoyaltyInsufficientPoints, "insufficient loyalty points")
	case services.IsLoyaltyAccountNotFound(err):
		respondError(c, http.StatusNotFound, CodeLoyaltyAccountNotFound, "loyalty account not found")
	case errors.Is(err, domain.ErrCustomerNotFound):
		respondError(c, http.StatusNotFound, CodeCustomerNotFound, "customer not found")
	case errors.Is(err, domain.ErrInvalidPhone):
		respondError(c, http.StatusBadRequest, CodeInvalidPhone, "invalid phone number")
	case errors.Is(err, domain.ErrInvalidLoyaltyRequest):
		respondValidationError(c, "invalid loyalty request")
	default:
		respondInternalError(c)
	}
}

// GetProgram — GET /branches/:id/loyalty/program (owner/manager)
func (h *LoyaltyHandler) GetProgram(c *gin.Context) {
	_, branchID, ok := h.requireBranchStaff(c, true)
	if !ok {
		return
	}
	program, err := h.svc.GetProgram(c.Request.Context(), branchID)
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, program)
}

type putLoyaltyProgramRequest struct {
	IsActive       bool   `json:"is_active"`
	EarnRatePoints int64  `json:"earn_rate_points" binding:"required"`
	EarnRateAmount string `json:"earn_rate_amount" binding:"required"`
}

// PutProgram — PUT /branches/:id/loyalty/program (owner/manager)
func (h *LoyaltyHandler) PutProgram(c *gin.Context) {
	staffSession, branchID, ok := h.requireBranchStaff(c, true)
	if !ok {
		return
	}
	var req putLoyaltyProgramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	program, err := h.svc.PutProgram(c.Request.Context(), branchID, req.IsActive, req.EarnRatePoints, req.EarnRateAmount, staffSession.StaffID)
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, program)
}

// LookupCustomer — GET /branches/:id/loyalty/customers?phone=
func (h *LoyaltyHandler) LookupCustomer(c *gin.Context) {
	_, branchID, ok := h.requireBranchStaff(c, false)
	if !ok {
		return
	}
	phone := c.Query("phone")
	if phone == "" {
		respondValidationError(c, "phone query parameter is required")
		return
	}
	result, err := h.svc.LookupCustomer(c.Request.Context(), branchID, phone)
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListTransactions — GET /branches/:id/loyalty/accounts/:account_id/transactions
func (h *LoyaltyHandler) ListTransactions(c *gin.Context) {
	_, branchID, ok := h.requireBranchStaff(c, false)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid account id")
		return
	}
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 32)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)
	rows, err := h.svc.ListTransactions(c.Request.Context(), branchID, accountID, int32(limit), int32(offset))
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": rows})
}

type loyaltyRedeemRequest struct {
	Points int64  `json:"points" binding:"required"`
	Reason string `json:"reason"`
}

// Redeem — POST /branches/:id/loyalty/accounts/:account_id/redeem
// (owner/manager/waiter; ledger-only deduction)
func (h *LoyaltyHandler) Redeem(c *gin.Context) {
	staffSession, branchID, ok := h.requireBranchStaff(c, false)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid account id")
		return
	}
	var req loyaltyRedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if req.Points <= 0 {
		respondValidationError(c, "points must be a positive integer")
		return
	}
	account, err := h.svc.Redeem(c.Request.Context(), branchID, accountID, req.Points, req.Reason, staffSession.StaffID)
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

type loyaltyAdjustRequest struct {
	PointsDelta int64  `json:"points_delta" binding:"required"`
	Reason      string `json:"reason" binding:"required"`
}

// Adjust — POST /branches/:id/loyalty/accounts/:account_id/adjust
// (owner/manager only; signed correction, floors at zero)
func (h *LoyaltyHandler) Adjust(c *gin.Context) {
	staffSession, branchID, ok := h.requireBranchStaff(c, true)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid account id")
		return
	}
	var req loyaltyAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	account, err := h.svc.Adjust(c.Request.Context(), branchID, accountID, req.PointsDelta, req.Reason, staffSession.StaffID)
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

// GetLoyaltyAnalytics — GET /branches/:id/analytics/loyalty?period= (owner/manager)
func (h *LoyaltyHandler) GetLoyaltyAnalytics(c *gin.Context) {
	_, branchID, ok := h.requireBranchStaff(c, true)
	if !ok {
		return
	}
	period := c.DefaultQuery("period", "weekly")
	switch period {
	case "daily", "weekly", "monthly":
	default:
		respondValidationError(c, "period must be daily, weekly, or monthly")
		return
	}
	report, err := h.svc.GetLoyaltyAnalytics(c.Request.Context(), branchID, period)
	if err != nil {
		h.respondLoyaltyError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}
