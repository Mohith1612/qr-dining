package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	svc         *services.PaymentService
	repos       *repository.Repos
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
	paymentCfg  config.PaymentConfig
	authz       *authz.Authorizer
	audit       *audit.Writer
}

func NewPaymentHandler(svc *services.PaymentService, repos *repository.Repos, guestTokens *auth.GuestTokenService, flags config.FeatureFlags, paymentCfg config.PaymentConfig, authorizer *authz.Authorizer, auditWriter *audit.Writer) *PaymentHandler {
	return &PaymentHandler{svc: svc, repos: repos, guestTokens: guestTokens, flags: flags, paymentCfg: paymentCfg, authz: authorizer, audit: auditWriter}
}

type initiatePaymentRequest struct {
	OrderID            *uuid.UUID `json:"order_id"`
	Amount             float64    `json:"amount" binding:"required,gt=0"`
	Method             string     `json:"method" binding:"required"`
	IdempotencyKey     string     `json:"idempotency_key" binding:"required"`
	ProviderPaymentRef string     `json:"provider_payment_ref"`
	ProviderOrderRef   string     `json:"provider_order_ref"`
}

func (h *PaymentHandler) InitiatePayment(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, sessionID, 0, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}
	if participantID == 0 {
		respondValidationError(c, "guest participant is required")
		return
	}

	var req initiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	method := sqlc.PaymentMethod(req.Method)
	switch method {
	case sqlc.PaymentMethodCash, sqlc.PaymentMethodCard, sqlc.PaymentMethodDigital, sqlc.PaymentMethodCardManual, sqlc.PaymentMethodUpi:
	default:
		respondValidationError(c, "invalid payment method")
		return
	}

	// Compute the authoritative bill server-side.
	bill, billErr := ComputeBillForSession(c.Request.Context(), h.repos, sessionID)

	if billErr != nil || bill == nil {
		respondInternalError(c)
		return
	}
	// Settlement bound: a payment may not exceed the authoritative bill total.
	// There is no tip field in this flow, so any overage is an overpayment.
	// epsilon absorbs float rounding in the computed total.
	if req.Amount > bill.Total+0.01 {
		respondError(c, http.StatusUnprocessableEntity, CodePaymentAmountInvalid,
			"payment amount exceeds the bill total")
		return
	}
	sess, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		sessionError(c, err)
		return
	}

	payment, err := h.svc.InitiatePayment(c.Request.Context(), services.InitiatePaymentRequest{
		SessionID:               sessionID,
		BranchID:                sess.BranchID,
		OrderID:                 req.OrderID,
		Method:                  method,
		Bill:                    billSnapshotInputFromBill(bill),
		StaffSettlementRequired: h.flags.PaymentStaffSettlementRequired,
		IdempotencyKey:          req.IdempotencyKey,
		ActorType:               "participant",
		ActorID:                 participantID,
		ProviderPaymentRef:      req.ProviderPaymentRef,
		ProviderOrderRef:        req.ProviderOrderRef,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrIdempotencyConflict):
			respondError(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", err.Error())
		case errors.Is(err, domain.ErrIdempotencyInProgress):
			respondError(c, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", err.Error())
		case errors.Is(err, domain.ErrSessionNotFound):
			respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
		case errors.Is(err, domain.ErrSessionNotActive), errors.Is(err, domain.ErrSessionClosed):
			respondError(c, http.StatusConflict, CodeSessionClosed,
				"This session can no longer take a payment.")
		case errors.Is(err, domain.ErrNotSessionHost):
			respondError(c, http.StatusForbidden, CodeNotSessionHost,
				"Only the table host can request the bill.")
		default:
			respondInternalError(c)
		}
		return
	}
	c.JSON(http.StatusCreated, payment)
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		SessionID:      sessionID,
		ResourceType:   audit.ResourcePayment,
		ResourceID:     audit.IDStr(payment.ID),
		Action:         audit.ActionPaymentInitiate,
		ActorType:      audit.ActorTypeGuest,
		ActorID:        audit.IDStr(participantID),
		IdempotencyKey: req.IdempotencyKey,
		RiskLevel:      audit.RiskMedium,
		Result:         audit.ResultSuccess,
	})
}

func (h *PaymentHandler) Webhook(c *gin.Context) {
	provider := strings.ToLower(c.Param("provider"))
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondValidationError(c, "invalid payload")
		return
	}
	if err := verifyGenericWebhook(provider, rawBody, c.GetHeader("X-Payment-Timestamp"), c.GetHeader("X-Payment-Signature"), h.paymentCfg); err != nil {
		h.audit.Record(c.Request.Context(), audit.AuditEvent{
			ResourceType: audit.ResourcePayment,
			Action:       audit.ActionPaymentSettle,
			ActorType:    audit.ActorTypeWebhook,
			Source:       audit.SourceWebhook,
			Result:       audit.ResultDenied,
			RiskLevel:    audit.RiskHigh,
			Metadata:     map[string]any{"provider": provider, "reason": err.Error()},
		})
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid webhook signature")
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
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
		RawPayload:      string(rawBody),
		Headers:         webhookHeaders(c),
	}); err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *PaymentHandler) Settle(c *gin.Context) {
	paymentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid payment id")
		return
	}
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	payment, err := h.svc.GetPayment(c.Request.Context(), paymentID)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			respondError(c, http.StatusNotFound, CodePaymentNotFound, err.Error())
		} else {
			respondInternalError(c)
		}
		return
	}
	actor, ok := staffActorForRequest(c, h.repos, staffSession)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, payment.BranchID)
	if !ok {
		return
	}
	resource := authz.Resource{Type: authz.ResourceTypePayment, ID: strconv.FormatInt(payment.ID, 10), Scope: authz.Scope{OrganizationID: orgID, BranchID: payment.BranchID}}
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionPaymentSettleStaff, resource) {
		return
	}
	updated, err := h.svc.SettlePaymentByStaff(c.Request.Context(), paymentID, staffSession.StaffID, staffSession.BranchID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPaymentNotFound):
			respondError(c, http.StatusNotFound, CodePaymentNotFound, err.Error())
		case errors.Is(err, domain.ErrInvalidPaymentTransition), errors.Is(err, domain.ErrBillSnapshotStale):
			respondError(c, http.StatusConflict, CodeInvalidPaymentTransition, err.Error())
		default:
			respondInternalError(c)
		}
		return
	}
	c.JSON(http.StatusOK, updated)
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     payment.BranchID,
		SessionID:    payment.SessionID,
		ResourceType: audit.ResourcePayment,
		ResourceID:   audit.IDStr(payment.ID),
		Action:       audit.ActionPaymentSettle,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      strconv.FormatInt(staffSession.StaffID, 10),
		Result:       audit.ResultSuccess,
		RiskLevel:    audit.RiskMedium,
	})
}

// ListPendingForBranch returns payments awaiting staff action for a branch so a
// waiter can see which cash/card collections still need confirming. Read-only.
func (h *PaymentHandler) ListPendingForBranch(c *gin.Context) {
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

	status := c.DefaultQuery("status", string(sqlc.PaymentStatusRequiresStaffConfirmation))
	switch sqlc.PaymentStatus(status) {
	case sqlc.PaymentStatusRequiresStaffConfirmation, sqlc.PaymentStatusProviderPending:
	default:
		respondValidationError(c, "unsupported payment status filter")
		return
	}

	payments, err := h.svc.ListPendingForBranch(c.Request.Context(), branchID, sqlc.PaymentStatus(status))
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, payments)
}

func billSnapshotInputFromBill(bill *BillResponse) services.BillSnapshotInput {
	ids := make([]uuid.UUID, 0, len(bill.Orders))
	for _, order := range bill.Orders {
		if id, err := uuid.Parse(order.OrderID); err == nil {
			ids = append(ids, id)
		}
	}
	return services.BillSnapshotInput{
		Subtotal:       bill.Subtotal,
		DiscountAmount: bill.DiscountAmount,
		TaxAmount:      bill.TaxAmount,
		ServiceCharge:  bill.ServiceCharge,
		TipAmount:      bill.TipAmount,
		Total:          bill.Total,
		Currency:       bill.Currency,
		SourceOrderIDs: ids,
		CreatedByActor: services.PaymentActor("guest", 0),
	}
}

func verifyGenericWebhook(provider string, rawBody []byte, timestampHeader, signatureHeader string, cfg config.PaymentConfig) error {
	secret := cfg.WebhookSecrets[strings.ToLower(provider)]
	if secret == "" {
		return domain.ErrInvalidWebhookSignature
	}
	ts, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return domain.ErrInvalidWebhookSignature
	}
	eventTime := time.Unix(ts, 0)
	tolerance := cfg.WebhookTimestampTolerance
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	if time.Since(eventTime) > tolerance || time.Until(eventTime) > tolerance {
		return domain.ErrInvalidWebhookSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestampHeader))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signatureHeader)) {
		return domain.ErrInvalidWebhookSignature
	}
	return nil
}

func webhookHeaders(c *gin.Context) json.RawMessage {
	headers := map[string]string{
		"x-payment-timestamp": c.GetHeader("X-Payment-Timestamp"),
		"x-payment-signature": c.GetHeader("X-Payment-Signature"),
	}
	b, _ := json.Marshal(headers)
	return b
}
