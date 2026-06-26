package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/authz"
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
	authz       *authz.Authorizer
	audit       *audit.Writer
}

func NewOrderHandler(svc *services.OrderService, repos *repository.Repos, metrics *observability.Metrics, guestTokens *auth.GuestTokenService, flags config.FeatureFlags, authorizer *authz.Authorizer, auditWriter *audit.Writer) *OrderHandler {
	return &OrderHandler{svc: svc, repos: repos, metrics: metrics, guestTokens: guestTokens, flags: flags, authz: authorizer, audit: auditWriter}
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
	// Per-item validation: the `min=1` binding tag only checks array length, not
	// individual quantities. A zero/negative quantity would otherwise hit the
	// order_items CHECK (quantity > 0) and surface as a 500.
	for _, item := range req.Items {
		if item.MenuItemID <= 0 {
			respondValidationError(c, "each item requires a valid menu_item_id")
			return
		}
		if item.Quantity < 1 || item.Quantity > 99 {
			respondValidationError(c, "item quantity must be between 1 and 99")
			return
		}
	}
	if req.BranchID != 0 || req.PlacedByParticipantID != 0 {
		respondValidationError(c, "branch_id and placed_by_participant_id are server-derived")
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
	sess, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		sessionError(c, err)
		return
	}

	result, err := h.svc.PlaceOrder(c.Request.Context(), services.PlaceOrderRequest{
		SessionID:             sessionID,
		BranchID:              sess.BranchID,
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
		case errors.Is(err, domain.ErrPaymentInProgress):
			respondError(c, http.StatusConflict, CodePaymentInProgress, err.Error())
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
		case errors.Is(err, domain.ErrIdempotencyConflict):
			respondError(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", err.Error())
		case errors.Is(err, domain.ErrParticipantNotInSession):
			respondError(c, http.StatusForbidden, CodeForbidden, err.Error())
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
	target, err := h.svc.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(c, http.StatusNotFound, CodeOrderNotFound, err.Error())
		} else {
			respondInternalError(c)
		}
		return
	}
	actor, ok := staffActorForRequest(c, h.repos, staffSession)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, target.BranchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionOrderStatusUpdate, authz.OrderResource(target.ID, target.BranchID, target.SessionID, orgID)) {
		return
	}
	order, err := h.svc.UpdateOrderStatus(c.Request.Context(), orderID, target.BranchID, newStatus, staffSession.StaffID)
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
