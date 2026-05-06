package handlers

import (
	"errors"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/domain"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	var req placeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		case errors.Is(err, domain.ErrSessionNotFound), errors.Is(err, domain.ErrMenuItemNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrSessionClosed), errors.Is(err, domain.ErrMenuItemUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	orders, err := h.svc.ListOrdersForSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var req updateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newStatus := domain.OrderStatus(req.Status)
	order, err := h.svc.UpdateOrderStatus(c.Request.Context(), orderID, newStatus)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrInvalidOrderTransition):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, order)
}
