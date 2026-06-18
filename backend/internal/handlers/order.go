package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrderHandler struct {
	svc         *services.OrderService
	repos       *repository.Repos
	metrics     *observability.Metrics
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
}

func NewOrderHandler(svc *services.OrderService, repos *repository.Repos, metrics *observability.Metrics, guestTokens *auth.GuestTokenService, flags config.FeatureFlags) *OrderHandler {
	return &OrderHandler{svc: svc, repos: repos, metrics: metrics, guestTokens: guestTokens, flags: flags}
}

type placeOrderRequest struct {
	BranchID              int64                `json:"branch_id"`
	PlacedByParticipantID int64                `json:"placed_by_participant_id"`
	IdempotencyKey        string               `json:"idempotency_key" binding:"required"`
	Items                 []services.OrderItem `json:"items" binding:"required,min=1"`
	PromoCode             *string              `json:"promo_code"`
	PhoneE164             *string              `json:"phone_e164"`
}

func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	var req placeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if req.PlacedByParticipantID != 0 {
		recordLegacyIdentityUsage(h.metrics, legacyMechanismBodyPlacedByParticipantID, legacyEndpointOrder)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, sessionID, req.PlacedByParticipantID, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}
	if participantID == 0 {
		respondValidationError(c, "placed_by_participant_id is required")
		return
	}
	if req.BranchID == 0 {
		sess, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
		if err != nil {
			sessionError(c, err)
			return
		}
		req.BranchID = sess.BranchID
	}

	result, err := h.svc.PlaceOrder(c.Request.Context(), services.PlaceOrderRequest{
		SessionID:             sessionID,
		BranchID:              req.BranchID,
		PlacedByParticipantID: participantID,
		IdempotencyKey:        req.IdempotencyKey,
		Items:                 req.Items,
		PromoCode:             req.PromoCode,
		PhoneE164:             req.PhoneE164,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSessionNotFound):
			respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
		case errors.Is(err, domain.ErrMenuItemNotFound):
			respondError(c, http.StatusNotFound, CodeMenuItemNotFound, err.Error())
		case errors.Is(err, domain.ErrModifierNotFound):
			respondError(c, http.StatusNotFound, CodeMenuItemNotFound, err.Error())
		case errors.Is(err, domain.ErrSessionClosed):
			respondError(c, http.StatusConflict, CodeSessionClosed, err.Error())
		case errors.Is(err, domain.ErrMenuItemUnavailable):
			respondError(c, http.StatusUnprocessableEntity, CodeMenuItemUnavailable, err.Error())
		case errors.Is(err, domain.ErrPromoNotFound):
			respondError(c, http.StatusNotFound, CodePromoNotFound, "This promo code isn't valid right now.")
		case errors.Is(err, domain.ErrMinOrderNotMet):
			respondError(c, http.StatusUnprocessableEntity, CodeMinOrderNotMet, err.Error())
		case errors.Is(err, domain.ErrPromoExhausted):
			respondError(c, http.StatusConflict, CodePromoExhausted, "This offer has been claimed by too many guests.")
		case errors.Is(err, domain.ErrPromoAlreadyUsed):
			respondError(c, http.StatusConflict, CodePromoAlreadyUsed, "You've already used this offer.")
		default:
			respondInternalError(c)
		}
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, sessionID, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	orders, err := h.svc.ListOrdersForSession(c.Request.Context(), sessionID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, orders)
}

type updateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid order id")
		return
	}

	var req updateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	newStatus := domain.OrderStatus(req.Status)
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	order, err := h.svc.UpdateOrderStatus(c.Request.Context(), orderID, newStatus, staffSession.StaffID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			respondError(c, http.StatusNotFound, CodeOrderNotFound, err.Error())
		case errors.Is(err, domain.ErrInvalidOrderTransition):
			respondError(c, http.StatusUnprocessableEntity, CodeInvalidOrderTransition, err.Error())
		default:
			respondInternalError(c)
		}
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) ListActiveForBranch(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
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

	orders, err := h.svc.ListActiveForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, orders)
}
