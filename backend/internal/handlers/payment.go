package handlers

import (
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	svc *services.PaymentService
}

func NewPaymentHandler(svc *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

type initiatePaymentRequest struct {
	OrderID *uuid.UUID `json:"order_id"`
	Amount  float64    `json:"amount" binding:"required,gt=0"`
	Method  string     `json:"method" binding:"required"`
}

func (h *PaymentHandler) InitiatePayment(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	var req initiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	method := sqlc.PaymentMethod(req.Method)
	switch method {
	case sqlc.PaymentMethodCash, sqlc.PaymentMethodCard, sqlc.PaymentMethodDigital:
	default:
		respondValidationError(c, "invalid payment method")
		return
	}

	payment, err := h.svc.InitiatePayment(c.Request.Context(), services.InitiatePaymentRequest{
		SessionID: sessionID,
		OrderID:   req.OrderID,
		Amount:    req.Amount,
		Method:    method,
	})
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusCreated, payment)
}

func (h *PaymentHandler) Webhook(c *gin.Context) {
	provider := c.Param("provider")

	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondValidationError(c, "invalid payload")
		return
	}

	externalID, _ := payload["id"].(string)
	if externalID == "" {
		respondValidationError(c, "missing event id in payload")
		return
	}

	eventType, _ := payload["event"].(string)

	rawPayload, err := marshalPayload(payload)
	if err != nil {
		respondValidationError(c, "invalid payload")
		return
	}

	if err := h.svc.ProcessWebhook(c.Request.Context(), services.ProcessWebhookRequest{
		Provider:        provider,
		ExternalEventID: externalID,
		EventType:       eventType,
		Payload:         rawPayload,
	}); err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
