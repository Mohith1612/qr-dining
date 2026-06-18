package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// SessionCloser is a narrow interface to close a session; avoids circular import with services package.
type SessionCloser interface {
	CloseSession(ctx context.Context, id uuid.UUID, requesterID *int64) error
}

type PaymentService struct {
	repos         *repository.Repos
	publisher     *events.Publisher
	metrics       *observability.Metrics
	sessionCloser SessionCloser
	logger        zerolog.Logger
}

func NewPaymentService(
	repos *repository.Repos,
	publisher *events.Publisher,
	metrics *observability.Metrics,
	sessionCloser SessionCloser,
	logger zerolog.Logger,
) *PaymentService {
	return &PaymentService{
		repos:         repos,
		publisher:     publisher,
		metrics:       metrics,
		sessionCloser: sessionCloser,
		logger:        logger,
	}
}

type InitiatePaymentRequest struct {
	SessionID               uuid.UUID
	BranchID                int64
	OrderID                 *uuid.UUID
	Method                  sqlc.PaymentMethod
	Bill                    BillSnapshotInput
	StaffSettlementRequired bool
	Provider                string
	ProviderPaymentRef      string
	ProviderOrderRef        string
}

type BillSnapshotInput struct {
	Subtotal       float64
	DiscountAmount float64
	TaxAmount      float64
	ServiceCharge  float64
	TipAmount      float64
	Total          float64
	Currency       string
	SourceOrderIDs []uuid.UUID
	CreatedByActor string
}

func (s *PaymentService) InitiatePayment(ctx context.Context, req InitiatePaymentRequest) (sqlc.Payment, error) {
	var amount pgtype.Numeric
	if err := amount.Scan(fmt.Sprintf("%.2f", req.Bill.Total)); err != nil {
		return sqlc.Payment{}, fmt.Errorf("encode amount: %w", err)
	}

	var orderID pgtype.UUID
	if req.OrderID != nil {
		orderID = pgtype.UUID{Bytes: [16]byte(*req.OrderID), Valid: true}
	}

	method, status := normalizePaymentMethodStatus(req.Method, req.StaffSettlementRequired)
	currency := req.Bill.Currency
	if currency == "" {
		currency = "INR"
	}
	provider := req.Provider
	providerRef := req.ProviderPaymentRef
	if status == sqlc.PaymentStatusProviderPending {
		if provider == "" {
			provider = "generic"
		}
		if providerRef == "" {
			providerRef = "pay_" + uuid.NewString()
		}
	}

	var payment sqlc.Payment
	err := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		snapshot, err := tx.CreateBillSnapshot(ctx, billSnapshotParams(req.SessionID, req.BranchID, req.Bill, currency))
		if err != nil {
			return fmt.Errorf("create bill snapshot: %w", err)
		}
		payment, err = tx.CreatePayment(ctx, sqlc.CreatePaymentParams{
			SessionID:          req.SessionID,
			OrderID:            orderID,
			Amount:             amount,
			Method:             method,
			Status:             status,
			BillSnapshotID:     pgtype.Int8{Int64: snapshot.ID, Valid: true},
			BranchID:           req.BranchID,
			Currency:           currency,
			Provider:           pgtype.Text{String: provider, Valid: provider != ""},
			ProviderPaymentRef: pgtype.Text{String: providerRef, Valid: providerRef != ""},
			ProviderOrderRef:   pgtype.Text{String: req.ProviderOrderRef, Valid: req.ProviderOrderRef != ""},
		})
		return err
	})
	if err == nil {
		s.publisher.PaymentInitiated(ctx, req.SessionID, payment)
		if payment.Status == sqlc.PaymentStatusCompleted {
			if closeErr := s.maybeCloseSettledSession(ctx, payment.SessionID); closeErr != nil {
				s.logger.Warn().Err(closeErr).Str("session_id", payment.SessionID.String()).Msg("auto-close session after payment failed")
			}
		}
	}
	return payment, err
}

type ProcessWebhookRequest struct {
	Provider        string
	ExternalEventID string
	EventType       string
	Payload         json.RawMessage
	RawPayload      string
	Headers         json.RawMessage
}

// ProcessWebhook handles an inbound payment webhook with idempotency protection.
func (s *PaymentService) ProcessWebhook(ctx context.Context, req ProcessWebhookRequest) error {
	// INSERT ON CONFLICT DO NOTHING — returns false if already processed.
	event, inserted, err := s.repos.InsertWebhookEvent(ctx, req.ExternalEventID, req.Provider, req.EventType, req.Payload, req.RawPayload, req.Headers)
	if err != nil {
		return fmt.Errorf("insert webhook event: %w", err)
	}
	if !inserted {
		// Already processed — idempotent replay, nothing to do.
		if s.metrics != nil && s.metrics.IdempotencyReplaysTotal != nil {
			s.metrics.IdempotencyReplaysTotal.WithLabelValues("webhook").Inc()
		}
		return nil
	}

	// Parse the payment ID from the payload (provider-specific; simplified here).
	eventPayload, newStatus, err := s.parseWebhookPayload(req.EventType, req.Payload)
	if err != nil {
		// Record the error but don't fail the webhook receipt.
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, 0, err.Error())
		return nil
	}

	payment, err := s.repos.GetPaymentByProviderRef(ctx, req.Provider, eventPayload.PaymentRef)
	if err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, 0, err.Error())
		return nil
	}
	if err := verifyWebhookPayment(payment, eventPayload); err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, err.Error())
		return nil
	}

	if err := domain.ValidatePaymentTransition(
		domain.PaymentStatus(payment.Status),
		domain.PaymentStatus(newStatus),
	); err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, err.Error())
		return nil
	}

	updated, err := s.repos.UpdatePaymentStatusExpected(ctx, payment.ID, payment.Status, newStatus)
	if err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, err.Error())
		return nil
	}
	_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, "")

	if newStatus == sqlc.PaymentStatusCompleted {
		s.publisher.PaymentCompleted(ctx, payment.SessionID, updated)
		sess, _ := s.repos.GetSessionByID(ctx, payment.SessionID)
		s.repos.LogEvent(ctx, payment.SessionID, sess.BranchID, "PAYMENT_COMPLETED", "system", 0, updated)

		if err := s.maybeCloseSettledSession(ctx, payment.SessionID); err != nil {
			s.logger.Warn().Err(err).Str("session_id", payment.SessionID.String()).Msg("auto-close session after payment failed")
		}
	}

	return nil
}

func (s *PaymentService) SettlePaymentByStaff(ctx context.Context, paymentID, staffID, branchID int64) (sqlc.Payment, error) {
	payment, err := s.repos.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return sqlc.Payment{}, err
	}
	if payment.BranchID != branchID {
		return sqlc.Payment{}, domain.ErrPaymentNotFound
	}
	if payment.Status != sqlc.PaymentStatusRequiresStaffConfirmation {
		return sqlc.Payment{}, domain.ErrInvalidPaymentTransition
	}
	if payment.BillSnapshotID.Valid {
		if err := s.ensureSnapshotFresh(ctx, payment); err != nil {
			return sqlc.Payment{}, err
		}
	}
	updated, err := s.repos.SettlePaymentByStaff(ctx, paymentID, staffID, branchID)
	if err != nil {
		return sqlc.Payment{}, err
	}
	s.publisher.PaymentCompleted(ctx, payment.SessionID, updated)
	s.repos.LogEvent(ctx, payment.SessionID, payment.BranchID, "PAYMENT_COMPLETED", "staff", staffID, updated)
	if err := s.maybeCloseSettledSession(ctx, payment.SessionID); err != nil {
		s.logger.Warn().Err(err).Str("session_id", payment.SessionID.String()).Msg("auto-close session after staff payment failed")
	}
	return updated, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id int64) (sqlc.Payment, error) {
	return s.repos.GetPaymentByID(ctx, id)
}

func (s *PaymentService) ListForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Payment, error) {
	return s.repos.ListPaymentsForSession(ctx, sessionID)
}

// parseWebhookPayload extracts the payment_id and new status from the provider payload.
// This is a simplified implementation — real providers have their own event schemas.
type webhookPaymentPayload struct {
	PaymentRef string
	Amount     float64
	Currency   string
	SessionID  uuid.UUID
	BranchID   int64
}

func (s *PaymentService) parseWebhookPayload(eventType string, payload json.RawMessage) (webhookPaymentPayload, sqlc.PaymentStatus, error) {
	var p struct {
		PaymentRef string  `json:"payment_ref"`
		Status     string  `json:"status"`
		Amount     float64 `json:"amount"`
		Currency   string  `json:"currency"`
		SessionID  string  `json:"session_id"`
		BranchID   int64   `json:"branch_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return webhookPaymentPayload{}, "", fmt.Errorf("parse webhook payload: %w", err)
	}
	if p.PaymentRef == "" {
		return webhookPaymentPayload{}, "", fmt.Errorf("missing payment_ref in payload")
	}
	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		return webhookPaymentPayload{}, "", fmt.Errorf("invalid session_id in payload")
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
		return webhookPaymentPayload{}, "", fmt.Errorf("unknown event type: %s", eventType)
	}

	return webhookPaymentPayload{
		PaymentRef: p.PaymentRef,
		Amount:     p.Amount,
		Currency:   p.Currency,
		SessionID:  sessionID,
		BranchID:   p.BranchID,
	}, status, nil
}

func normalizePaymentMethodStatus(method sqlc.PaymentMethod, staffRequired bool) (sqlc.PaymentMethod, sqlc.PaymentStatus) {
	switch method {
	case sqlc.PaymentMethodCard:
		method = sqlc.PaymentMethodCardManual
	case sqlc.PaymentMethodDigital:
		method = sqlc.PaymentMethodUpi
	}
	switch method {
	case sqlc.PaymentMethodCash, sqlc.PaymentMethodCardManual:
		if staffRequired {
			return method, sqlc.PaymentStatusRequiresStaffConfirmation
		}
		return method, sqlc.PaymentStatusCompleted
	case sqlc.PaymentMethodUpi:
		if staffRequired {
			return method, sqlc.PaymentStatusRequiresStaffConfirmation
		}
		return method, sqlc.PaymentStatusProviderPending
	default:
		return method, sqlc.PaymentStatusProviderPending
	}
}

func billSnapshotParams(sessionID uuid.UUID, branchID int64, bill BillSnapshotInput, currency string) sqlc.CreateBillSnapshotParams {
	orderIDs := make([]string, 0, len(bill.SourceOrderIDs))
	for _, id := range bill.SourceOrderIDs {
		orderIDs = append(orderIDs, id.String())
	}
	source, _ := json.Marshal(orderIDs)
	return sqlc.CreateBillSnapshotParams{
		SessionID:      sessionID,
		BranchID:       branchID,
		Subtotal:       numeric(bill.Subtotal),
		DiscountAmount: numeric(bill.DiscountAmount),
		TaxAmount:      numeric(bill.TaxAmount),
		ServiceCharge:  numeric(bill.ServiceCharge),
		TipAmount:      numeric(bill.TipAmount),
		Total:          numeric(bill.Total),
		Currency:       currency,
		SourceOrderIds: source,
		CreatedByActor: bill.CreatedByActor,
	}
}

func numeric(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", v))
	return n
}

func verifyWebhookPayment(payment sqlc.Payment, payload webhookPaymentPayload) error {
	amount, _ := payment.Amount.Float64Value()
	if !amount.Valid || fmt.Sprintf("%.2f", amount.Float64) != fmt.Sprintf("%.2f", payload.Amount) {
		return domain.ErrPaymentVerificationFailed
	}
	if payment.Currency != payload.Currency || payment.SessionID != payload.SessionID || payment.BranchID != payload.BranchID {
		return domain.ErrPaymentVerificationFailed
	}
	return nil
}

func (s *PaymentService) maybeCloseSettledSession(ctx context.Context, sessionID uuid.UUID) error {
	payments, err := s.repos.ListPaymentsForSession(ctx, sessionID)
	if err != nil {
		return err
	}
	var latestCompleted *sqlc.Payment
	for i := range payments {
		if payments[i].Status == sqlc.PaymentStatusCompleted && payments[i].BillSnapshotID.Valid {
			if latestCompleted == nil || payments[i].CompletedAt.Time.After(latestCompleted.CompletedAt.Time) {
				latestCompleted = &payments[i]
			}
		}
	}
	if latestCompleted == nil {
		return nil
	}
	if err := s.ensureSnapshotFresh(ctx, *latestCompleted); err != nil {
		return err
	}
	snapshot, err := s.repos.GetBillSnapshotByID(ctx, latestCompleted.BillSnapshotID.Int64)
	if err != nil {
		return err
	}
	totalPaid, err := s.repos.SumCompletedPaymentsForSession(ctx, sessionID)
	if err != nil {
		return err
	}
	paid, _ := totalPaid.Float64Value()
	total, _ := snapshot.Total.Float64Value()
	if paid.Valid && total.Valid && paid.Float64+0.001 >= total.Float64 {
		return s.sessionCloser.CloseSession(ctx, sessionID, nil)
	}
	return nil
}

func (s *PaymentService) ensureSnapshotFresh(ctx context.Context, payment sqlc.Payment) error {
	snapshot, err := s.repos.GetBillSnapshotByID(ctx, payment.BillSnapshotID.Int64)
	if err != nil {
		return err
	}
	var snapIDs []string
	if err := json.Unmarshal(snapshot.SourceOrderIds, &snapIDs); err != nil {
		return err
	}
	current, err := activeOrderIDSet(ctx, s.repos, payment.SessionID)
	if err != nil {
		return err
	}
	if len(snapIDs) != len(current) {
		return domain.ErrBillSnapshotStale
	}
	for _, id := range snapIDs {
		if _, ok := current[id]; !ok {
			return domain.ErrBillSnapshotStale
		}
	}
	return nil
}

func activeOrderIDSet(ctx context.Context, repos *repository.Repos, sessionID uuid.UUID) (map[string]struct{}, error) {
	orders, err := repos.ListOrdersForSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(orders))
	for _, order := range orders {
		if order.Status == sqlc.OrderStatusCancelled {
			continue
		}
		set[order.ID.String()] = struct{}{}
	}
	return set, nil
}

func PaymentActor(actorType string, actorID int64) string {
	if actorID == 0 {
		return actorType
	}
	return actorType + ":" + strconv.FormatInt(actorID, 10)
}

func IsPaymentStaleError(err error) bool {
	return errors.Is(err, domain.ErrBillSnapshotStale)
}
