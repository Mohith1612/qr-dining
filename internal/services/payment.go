package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type PaymentService struct {
	repos     *repository.Repos
	publisher *events.Publisher
	metrics   *observability.Metrics
}

func NewPaymentService(repos *repository.Repos, publisher *events.Publisher, metrics *observability.Metrics) *PaymentService {
	return &PaymentService{repos: repos, publisher: publisher, metrics: metrics}
}

type InitiatePaymentRequest struct {
	SessionID uuid.UUID
	OrderID   *uuid.UUID // nil for session-level payments
	Amount    float64
	Method    sqlc.PaymentMethod
}

func (s *PaymentService) InitiatePayment(ctx context.Context, req InitiatePaymentRequest) (sqlc.Payment, error) {
	var amount pgtype.Numeric
	if err := amount.Scan(fmt.Sprintf("%.2f", req.Amount)); err != nil {
		return sqlc.Payment{}, fmt.Errorf("encode amount: %w", err)
	}

	var orderID pgtype.UUID
	if req.OrderID != nil {
		orderID = pgtype.UUID{Bytes: [16]byte(*req.OrderID), Valid: true}
	}

	payment, err := s.repos.CreatePayment(ctx, sqlc.CreatePaymentParams{
		SessionID: req.SessionID,
		OrderID:   orderID,
		Amount:    amount,
		Method:    req.Method,
	})
	if err != nil {
		return sqlc.Payment{}, err
	}

	s.publisher.PaymentInitiated(ctx, req.SessionID, payment)
	return payment, nil
}

type ProcessWebhookRequest struct {
	Provider        string
	ExternalEventID string
	EventType       string
	Payload         json.RawMessage
}

// ProcessWebhook handles an inbound payment webhook with idempotency protection.
func (s *PaymentService) ProcessWebhook(ctx context.Context, req ProcessWebhookRequest) error {
	// INSERT ON CONFLICT DO NOTHING — returns false if already processed.
	_, inserted, err := s.repos.InsertWebhookEvent(ctx, req.ExternalEventID, req.Provider, req.EventType, req.Payload)
	if err != nil {
		return fmt.Errorf("insert webhook event: %w", err)
	}
	if !inserted {
		// Already processed — idempotent replay, nothing to do.
		s.metrics.IdempotencyReplaysTotal.WithLabelValues("webhook").Inc()
		return nil
	}

	// Parse the payment ID from the payload (provider-specific; simplified here).
	paymentID, newStatus, err := s.parseWebhookPayload(req.EventType, req.Payload)
	if err != nil {
		// Record the error but don't fail the webhook receipt.
		_ = s.repos.MarkWebhookProcessed(ctx, 0, 0, err.Error())
		return nil
	}

	payment, err := s.repos.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return err
	}

	if err := domain.ValidatePaymentTransition(
		domain.PaymentStatus(payment.Status),
		domain.PaymentStatus(newStatus),
	); err != nil {
		return err
	}

	updated, err := s.repos.UpdatePaymentStatus(ctx, paymentID, newStatus)
	if err != nil {
		return err
	}

	if newStatus == sqlc.PaymentStatusCompleted {
		s.publisher.PaymentCompleted(ctx, payment.SessionID, updated)
		s.repos.LogEvent(ctx, payment.SessionID, 0, "PAYMENT_COMPLETED", "system", 0, updated)
	}

	return nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id int64) (sqlc.Payment, error) {
	return s.repos.GetPaymentByID(ctx, id)
}

func (s *PaymentService) ListForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Payment, error) {
	return s.repos.ListPaymentsForSession(ctx, sessionID)
}

// parseWebhookPayload extracts the payment_id and new status from the provider payload.
// This is a simplified implementation — real providers have their own event schemas.
func (s *PaymentService) parseWebhookPayload(eventType string, payload json.RawMessage) (int64, sqlc.PaymentStatus, error) {
	var p struct {
		PaymentID int64  `json:"payment_id"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return 0, "", fmt.Errorf("parse webhook payload: %w", err)
	}
	if p.PaymentID == 0 {
		return 0, "", fmt.Errorf("missing payment_id in payload")
	}

	var status sqlc.PaymentStatus
	switch eventType {
	case "payment.captured", "payment.success", "charge.succeeded":
		status = sqlc.PaymentStatusCompleted
	case "payment.failed", "charge.failed":
		status = sqlc.PaymentStatusFailed
	case "refund.created", "payment.refunded":
		status = sqlc.PaymentStatusRefunded
	default:
		return 0, "", fmt.Errorf("unknown event type: %s", eventType)
	}

	return p.PaymentID, status, nil
}
