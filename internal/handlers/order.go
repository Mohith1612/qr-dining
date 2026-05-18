package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrderHandler struct {
	svc *services.OrderService
}

func NewOrderHandler(svc *services.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

type placeOrderRequest struct {
	BranchID              int64                `json:"branch_id" binding:"required"`
	PlacedByParticipantID int64                `json:"placed_by_participant_id" binding:"required"`
	IdempotencyKey        string               `json:"idempotency_key" binding:"required"`
	Items                 []services.OrderItem `json:"items" binding:"required,min=1"`
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

	result, err := h.svc.PlaceOrder(c.Request.Context(), services.PlaceOrderRequest{
		SessionID:             sessionID,
		BranchID:              req.BranchID,
		PlacedByParticipantID: req.PlacedByParticipantID,
		IdempotencyKey:        req.IdempotencyKey,
		Items:                 req.Items,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSessionNotFound):
			respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
		case errors.Is(err, domain.ErrMenuItemNotFound):
			respondError(c, http.StatusNotFound, CodeMenuItemNotFound, err.Error())
		case errors.Is(err, domain.ErrSessionClosed):
			respondError(c, http.StatusConflict, CodeSessionClosed, err.Error())
		case errors.Is(err, domain.ErrMenuItemUnavailable):
			respondError(c, http.StatusUnprocessableEntity, CodeMenuItemUnavailable, err.Error())
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
	staffSession, _ := middleware.GetStaffSession(c)
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
	if ok && staffSession.BranchID != branchID {
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
