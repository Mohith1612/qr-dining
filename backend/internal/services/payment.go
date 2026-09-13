package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	hostAuth      *SessionService
	loyalty       loyaltyAccrual
	promoSvc      *PromoService
	logger        zerolog.Logger
}

// SetHostAuthority injects the session service used to enforce host-only
// payment initiation (and on-demand host reassignment). Wired after
// construction to keep the constructor signature stable.
func (s *PaymentService) SetHostAuthority(h *SessionService) { s.hostAuth = h }

// SetPromoService injects the promo validator used to re-validate and record a
// promo redemption when a promo is applied at payment initiation.
func (s *PaymentService) SetPromoService(p *PromoService) { s.promoSvc = p }

// loyaltyAccrual is the optional loyalty earn hook. Implementations must be
// fully error-isolated: the call is fire-and-forget after a payment is already
// terminal and must never affect payment semantics.
type loyaltyAccrual interface {
	AccruePointsForPayment(ctx context.Context, payment sqlc.Payment)
}

// SetLoyaltyAccrual injects the loyalty earn hook (nil-safe; wired after
// construction like SetHostAuthority).
func (s *PaymentService) SetLoyaltyAccrual(l loyaltyAccrual) { s.loyalty = l }

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
	IdempotencyKey          string
	ActorType               string
	ActorID                 int64
	// Promo, if applied at payment initiation. The handler has already folded
	// the discount into Bill (Total reduced, DiscountAmount set); the service
	// re-validates under a row lock inside the tx and records the redemption
	// against the payment. PromoCode nil ⇒ no promo.
	PromoCode  *string
	PromoPhone *string
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
	result, err := s.InitiatePaymentWithResult(ctx, req)
	return result.Payment, err
}

type InitiatePaymentResult struct {
	Payment sqlc.Payment
	Created bool
}

func (s *PaymentService) InitiatePaymentWithResult(ctx context.Context, req InitiatePaymentRequest) (InitiatePaymentResult, error) {
	created := false
	payment, err := s.initiatePayment(ctx, req, &created)
	return InitiatePaymentResult{Payment: payment, Created: created}, err
}

func (s *PaymentService) initiatePayment(ctx context.Context, req InitiatePaymentRequest, createdResult *bool) (sqlc.Payment, error) {
	actorType := req.ActorType
	if actorType == "" {
		actorType = "guest"
	}

	// Host-controlled flow: only the session host may initiate the bill/payment.
	// Checked before any idempotency-key or payment_pending side-effects so a
	// non-host request cannot freeze the session. Staff-initiated paths (if any)
	// use a non-participant actor type and are not gated here.
	if actorType == "participant" && s.hostAuth != nil {
		authorized, err := s.hostAuth.AuthorizeHostAction(ctx, req.SessionID, req.ActorID)
		if err != nil {
			return sqlc.Payment{}, err
		}
		if !authorized {
			return sqlc.Payment{}, domain.ErrNotSessionHost
		}
	}

	idemScope := repository.IdempotencyScope{
		ScopeType: "session",
		ScopeID:   req.SessionID.String(),
		ActorType: actorType,
		ActorID:   strconv.FormatInt(req.ActorID, 10),
		Key:       req.IdempotencyKey,
	}
	requestHash := hashPaymentRequest(req)
	if req.IdempotencyKey != "" {
		_, inserted, err := s.repos.CreateIdempotencyKey(ctx, idemScope, requestHash, time.Now().Add(24*time.Hour))
		if err != nil {
			return sqlc.Payment{}, err
		}
		if !inserted {
			existingKey, err := s.repos.GetIdempotencyKey(ctx, idemScope)
			if err != nil {
				return sqlc.Payment{}, err
			}
			if existingKey.RequestHash != requestHash {
				return sqlc.Payment{}, domain.ErrIdempotencyConflict
			}
			if existingKey.Status != "completed" || !existingKey.ResponseResourceID.Valid {
				return sqlc.Payment{}, domain.ErrIdempotencyInProgress
			}
			paymentID, err := strconv.ParseInt(existingKey.ResponseResourceID.String, 10, 64)
			if err != nil {
				return sqlc.Payment{}, fmt.Errorf("parse idempotent payment id: %w", err)
			}
			if s.metrics != nil && s.metrics.IdempotencyReplaysTotal != nil {
				s.metrics.IdempotencyReplaysTotal.WithLabelValues("payment").Inc()
			}
			return s.repos.GetPaymentByID(ctx, paymentID)
		}
	}
	if existing, ok, err := reusableNonTerminalPaymentForSession(ctx, s.repos, req.SessionID); err != nil {
		return sqlc.Payment{}, err
	} else if ok {
		if req.IdempotencyKey != "" {
			if err := s.repos.CompleteIdempotencyKey(ctx, idemScope, "payment", strconv.FormatInt(existing.ID, 10)); err != nil {
				return sqlc.Payment{}, fmt.Errorf("complete payment idempotency key: %w", err)
			}
		}
		return existing, nil
	}

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
	var appliedPromoID int64
	created := false
	defer func() { *createdResult = created }()
	err := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		// Move the session into payment_pending inside this transaction so cart
		// and order mutations are frozen for the duration of the payment. A
		// payment_pending session reuses its existing payment; any terminal
		// session state blocks initiation.
		sess, err := tx.GetSessionByID(ctx, req.SessionID)
		if err != nil {
			return err
		}
		switch sqlc.SessionStatus(sess.Status) {
		case sqlc.SessionStatusActive:
			if _, err := tx.TransitionSessionToPaymentPending(ctx, req.SessionID); err != nil {
				// The session left 'active' between our read and this update
				// (closed/abandoned concurrently). Surface it as a clean
				// not-active conflict rather than an opaque 500.
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrSessionNotActive
				}
				return fmt.Errorf("transition session to payment_pending: %w", err)
			}
		case sqlc.SessionStatusPaymentPending:
			existing, ok, err := nonTerminalPaymentForSession(ctx, tx, req.SessionID)
			if err != nil {
				return err
			}
			if ok {
				payment = existing
				if req.IdempotencyKey != "" {
					if err := tx.CompleteIdempotencyKey(ctx, idemScope, "payment", strconv.FormatInt(payment.ID, 10)); err != nil {
						return fmt.Errorf("complete payment idempotency key: %w", err)
					}
				}
				return nil
			}
		default:
			return domain.ErrSessionNotActive
		}
		branch, err := tx.GetBranchByID(ctx, req.BranchID)
		if err != nil {
			return fmt.Errorf("get branch: %w", err)
		}
		localDate, businessDate := branchBusinessDate(branch, time.Now())
		paymentSeq, err := tx.NextPaymentNumber(ctx, req.BranchID, businessDate)
		if err != nil {
			return fmt.Errorf("next payment number: %w", err)
		}
		paymentRef := paymentReference(branch.BranchCode, localDate, paymentSeq)

		snapshot, err := tx.CreateBillSnapshot(ctx, billSnapshotParams(req.SessionID, req.BranchID, req.Bill, currency))
		if err != nil {
			return fmt.Errorf("create bill snapshot: %w", err)
		}
		payment, err = tx.CreatePayment(ctx, sqlc.CreatePaymentParams{
			SessionID:           req.SessionID,
			OrderID:             orderID,
			Amount:              amount,
			Method:              method,
			Status:              status,
			BillSnapshotID:      pgtype.Int8{Int64: snapshot.ID, Valid: true},
			BranchID:            req.BranchID,
			Currency:            currency,
			Provider:            pgtype.Text{String: provider, Valid: provider != ""},
			ProviderPaymentRef:  pgtype.Text{String: providerRef, Valid: providerRef != ""},
			ProviderOrderRef:    pgtype.Text{String: req.ProviderOrderRef, Valid: req.ProviderOrderRef != ""},
			PaymentBusinessDate: businessDate,
			PaymentSequence:     paymentSeq,
			PaymentReference:    paymentRef,
		})
		if err != nil {
			return err
		}
		created = true
		// Record the promo redemption against this payment, inside the tx so it
		// rolls back with the snapshot/payment/transition on any failure. The
		// FOR UPDATE lock in ValidatePromo serializes concurrent redemptions of
		// the same promo, closing the per-phone/global cap race; the
		// (promo_id, payment_id) unique index guards idempotent replays.
		if req.PromoCode != nil && s.promoSvc != nil {
			preDiscountTotal := req.Bill.Total + req.Bill.DiscountAmount
			vr, perr := s.promoSvc.ValidatePromo(ctx, tx, ValidatePromoRequest{
				BranchID:   req.BranchID,
				Code:       *req.PromoCode,
				OrderTotal: preDiscountTotal,
				PhoneE164:  req.PromoPhone,
			})
			if perr != nil {
				return perr
			}
			// Store the normalized phone, not req.PromoPhone: the per-phone cap
			// query is an exact string match, so persisting the raw client
			// string would let "+91 98765 43210" evade a cap counted against
			// "+919876543210" and make the promo infinitely re-redeemable.
			if _, perr := tx.CreatePromoRedemptionForPayment(ctx, vr.PromoID, payment.ID, vr.NormalizedPhone); perr != nil {
				return fmt.Errorf("create promo redemption: %w", perr)
			}
			if perr := tx.IncrementPromoRedemptionCount(ctx, vr.PromoID); perr != nil {
				return fmt.Errorf("increment promo redemption count: %w", perr)
			}
			appliedPromoID = vr.PromoID
		}

		if req.IdempotencyKey != "" {
			if err := tx.CompleteIdempotencyKey(ctx, idemScope, "payment", strconv.FormatInt(payment.ID, 10)); err != nil {
				return fmt.Errorf("complete payment idempotency key: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		if isOneNonTerminalPaymentViolation(err) || errors.Is(err, domain.ErrSessionNotActive) {
			if existing, ok, lookupErr := nonTerminalPaymentForPaymentPendingSession(ctx, s.repos, req.SessionID); lookupErr == nil && ok {
				if req.IdempotencyKey != "" {
					_ = s.repos.CompleteIdempotencyKey(ctx, idemScope, "payment", strconv.FormatInt(existing.ID, 10))
				}
				return existing, nil
			}
		}
		if req.IdempotencyKey != "" {
			_ = s.repos.FailIdempotencyKey(ctx, idemScope)
		}
	}
	if err == nil && created {
		s.publisher.PaymentInitiated(ctx, req.SessionID, payment)
		if appliedPromoID != 0 && req.PromoCode != nil {
			s.publisher.PromoApplied(ctx, req.SessionID, map[string]any{
				"payment_id":      payment.ID,
				"promo_code":      *req.PromoCode,
				"discount_amount": req.Bill.DiscountAmount,
			})
		}
		if payment.Status == sqlc.PaymentStatusCompleted {
			if closeErr := s.maybeCloseSettledSession(ctx, payment.SessionID); closeErr != nil {
				s.logger.Warn().Err(closeErr).Str("session_id", payment.SessionID.String()).Msg("auto-close session after payment failed")
			}
		}
	}
	return payment, err
}

func nonTerminalPaymentForSession(ctx context.Context, repos *repository.Repos, sessionID uuid.UUID) (sqlc.Payment, bool, error) {
	payments, err := repos.ListPaymentsForSession(ctx, sessionID)
	if err != nil {
		return sqlc.Payment{}, false, err
	}
	for _, payment := range payments {
		if !isTerminalPaymentStatus(payment.Status) {
			return payment, true, nil
		}
	}
	return sqlc.Payment{}, false, nil
}

func reusableNonTerminalPaymentForSession(ctx context.Context, repos *repository.Repos, sessionID uuid.UUID) (sqlc.Payment, bool, error) {
	sess, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sqlc.Payment{}, false, err
	}
	if sess.Status != sqlc.SessionStatusActive && sess.Status != sqlc.SessionStatusPaymentPending {
		return sqlc.Payment{}, false, nil
	}
	return nonTerminalPaymentForSession(ctx, repos, sessionID)
}

func nonTerminalPaymentForPaymentPendingSession(ctx context.Context, repos *repository.Repos, sessionID uuid.UUID) (sqlc.Payment, bool, error) {
	sess, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sqlc.Payment{}, false, err
	}
	if sess.Status != sqlc.SessionStatusPaymentPending {
		return sqlc.Payment{}, false, nil
	}
	return nonTerminalPaymentForSession(ctx, repos, sessionID)
}

func isOneNonTerminalPaymentViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "idx_payments_one_non_terminal_per_session"
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func hashPaymentRequest(req InitiatePaymentRequest) string {
	actorType := req.ActorType
	if actorType == "" {
		actorType = "guest"
	}
	sourceOrderIDs := make([]string, 0, len(req.Bill.SourceOrderIDs))
	for _, id := range req.Bill.SourceOrderIDs {
		sourceOrderIDs = append(sourceOrderIDs, id.String())
	}
	sort.Strings(sourceOrderIDs)
	payload := struct {
		SessionID          string   `json:"session_id"`
		ActorType          string   `json:"actor_type"`
		ActorID            int64    `json:"actor_id"`
		OrderID            string   `json:"order_id,omitempty"`
		Method             string   `json:"method"`
		Total              float64  `json:"total"`
		Currency           string   `json:"currency"`
		Provider           string   `json:"provider,omitempty"`
		ProviderPaymentRef string   `json:"provider_payment_ref,omitempty"`
		ProviderOrderRef   string   `json:"provider_order_ref,omitempty"`
		PromoCode          string   `json:"promo_code,omitempty"`
		SourceOrderIDs     []string `json:"source_order_ids"`
	}{
		SessionID:          req.SessionID.String(),
		ActorType:          actorType,
		ActorID:            req.ActorID,
		Method:             string(req.Method),
		Total:              req.Bill.Total,
		Currency:           req.Bill.Currency,
		Provider:           req.Provider,
		ProviderPaymentRef: req.ProviderPaymentRef,
		ProviderOrderRef:   req.ProviderOrderRef,
		PromoCode:          derefStr(req.PromoCode),
		SourceOrderIDs:     sourceOrderIDs,
	}
	if req.OrderID != nil {
		payload.OrderID = req.OrderID.String()
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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
		s.auditWebhookFailure(ctx, req, event.ID, 0, err)
		return nil
	}

	payment, err := s.repos.GetPaymentByProviderRef(ctx, req.Provider, eventPayload.PaymentRef)
	if err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, 0, err.Error())
		s.auditWebhookFailure(ctx, req, event.ID, 0, err)
		return nil
	}
	if err := verifyWebhookPayment(payment, eventPayload); err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, err.Error())
		s.auditWebhookFailure(ctx, req, event.ID, payment.ID, err)
		return nil
	}

	if err := domain.ValidatePaymentTransition(
		domain.PaymentStatus(payment.Status),
		domain.PaymentStatus(newStatus),
	); err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, err.Error())
		s.auditWebhookFailure(ctx, req, event.ID, payment.ID, err)
		return nil
	}

	updated, err := s.repos.UpdatePaymentStatusExpected(ctx, payment.ID, payment.Status, newStatus)
	if err != nil {
		_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, err.Error())
		s.auditWebhookFailure(ctx, req, event.ID, payment.ID, err)
		return nil
	}
	_ = s.repos.MarkWebhookProcessed(ctx, event.ID, payment.ID, "")

	switch newStatus {
	case sqlc.PaymentStatusCompleted:
		s.publisher.PaymentCompleted(ctx, payment.SessionID, updated)
		sess, _ := s.repos.GetSessionByID(ctx, payment.SessionID)
		s.repos.LogEvent(ctx, payment.SessionID, sess.BranchID, "PAYMENT_COMPLETED", "system", 0, updated)
		if s.loyalty != nil {
			s.loyalty.AccruePointsForPayment(ctx, updated)
		}

		if err := s.maybeCloseSettledSession(ctx, payment.SessionID); err != nil {
			s.logger.Warn().Err(err).Str("session_id", payment.SessionID.String()).Msg("auto-close session after payment failed")
		}
	case sqlc.PaymentStatusFailed, sqlc.PaymentStatusCancelled:
		// If no non-terminal payments remain against the session, release the
		// payment_pending freeze so the guest can change the order or retry.
		if err := s.maybeReleasePaymentPending(ctx, payment.SessionID); err != nil {
			s.logger.Warn().Err(err).Str("session_id", payment.SessionID.String()).Msg("release payment_pending failed")
		}
	}

	return nil
}

// maybeReleasePaymentPending checks every payment for the session and returns
// the session to 'active' only when no non-terminal payment remains. This is
// invoked from any path that moves a payment into a terminal failed/cancelled
// state.
func (s *PaymentService) maybeReleasePaymentPending(ctx context.Context, sessionID uuid.UUID) error {
	payments, err := s.repos.ListPaymentsForSession(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, p := range payments {
		if !isTerminalPaymentStatus(p.Status) {
			return nil
		}
	}
	if _, err := s.repos.TransitionSessionToActive(ctx, sessionID); err != nil {
		// pgx.ErrNoRows means the session was not in payment_pending anymore
		// (already closed or already active); treat as advisory.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	return nil
}

func isTerminalPaymentStatus(s sqlc.PaymentStatus) bool {
	switch s {
	case sqlc.PaymentStatusCompleted,
		sqlc.PaymentStatusFailed,
		sqlc.PaymentStatusCancelled,
		sqlc.PaymentStatusRefunded,
		sqlc.PaymentStatusPartiallyRefunded:
		return true
	default:
		return false
	}
}

func (s *PaymentService) auditWebhookFailure(ctx context.Context, req ProcessWebhookRequest, eventID, paymentID int64, err error) {
	s.repos.LogAuditV2(ctx, audit.AuditEvent{
		ResourceType: audit.ResourcePayment,
		ResourceID:   strconv.FormatInt(paymentID, 10),
		Action:       audit.ActionPaymentWebhookProcess,
		Result:       audit.ResultFailure,
		ActorType:    audit.ActorTypeWebhook,
		Source:       audit.SourceWebhook,
		RiskLevel:    audit.RiskHigh,
		Metadata: map[string]any{
			"provider":          req.Provider,
			"external_event_id": req.ExternalEventID,
			"webhook_event_id":  eventID,
			"error":             err.Error(),
		},
	})
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
	if s.loyalty != nil {
		s.loyalty.AccruePointsForPayment(ctx, updated)
	}
	if err := s.maybeCloseSettledSession(ctx, payment.SessionID); err != nil {
		s.logger.Warn().Err(err).Str("session_id", payment.SessionID.String()).Msg("auto-close session after staff payment failed")
	}
	return updated, nil
}

// PaymentCancelledEvent is the PAYMENT_CANCELLED broadcast payload. The session
// status travels with the payment because cancelling the last non-terminal
// payment releases the payment_pending freeze — a connected guest learns from
// one message that the cart is writable again.
//
// The staff-authored cancellation reason is deliberately absent: it is recorded
// in the audit trail, not broadcast to the table.
type PaymentCancelledEvent struct {
	Payment       sqlc.Payment `json:"payment"`
	SessionStatus string       `json:"session_status"`
}

// CancelPaymentByStaff withdraws a payment request that never reached a terminal
// status, releasing the session's payment_pending freeze so the table can order
// again. This is the operator-driven recovery the escalation worker alerts
// towards; the worker itself stays alert-only.
//
// Which statuses are cancellable is the domain transition table's decision, not a
// second hand-maintained list here: requested, provider_pending and
// requires_staff_confirmation may move to cancelled, while completed, failed,
// refunded and cancelled may not. Cancelling an already-cancelled payment
// therefore conflicts rather than silently succeeding — every cancel carries its
// own reason and audit entry, so a second success would claim a cancellation
// that never happened.
func (s *PaymentService) CancelPaymentByStaff(ctx context.Context, paymentID, staffID, branchID int64, reason string) (sqlc.Payment, error) {
	payment, err := s.repos.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return sqlc.Payment{}, err
	}
	// Branch is derived from the payment row, never from the caller.
	if payment.BranchID != branchID {
		return sqlc.Payment{}, domain.ErrPaymentNotFound
	}
	if err := domain.ValidatePaymentTransition(
		domain.PaymentStatus(payment.Status),
		domain.PaymentStatusCancelled,
	); err != nil {
		return sqlc.Payment{}, err
	}

	updated, err := s.repos.UpdatePaymentStatusExpected(ctx, paymentID, payment.Status, sqlc.PaymentStatusCancelled)
	if err != nil {
		return sqlc.Payment{}, err
	}

	// Unfreeze through the single existing release implementation, which only
	// returns the session to active when no other non-terminal payment remains.
	if err := s.maybeReleasePaymentPending(ctx, payment.SessionID); err != nil {
		s.logger.Warn().Err(err).Str("session_id", payment.SessionID.String()).Msg("release payment_pending after staff cancel failed")
	}

	sessionStatus := ""
	if sess, err := s.repos.GetSessionByID(ctx, payment.SessionID); err == nil {
		sessionStatus = string(sess.Status)
	}
	s.publisher.PaymentCancelled(ctx, payment.SessionID, PaymentCancelledEvent{
		Payment:       updated,
		SessionStatus: sessionStatus,
	})
	s.repos.LogEvent(ctx, payment.SessionID, payment.BranchID, "PAYMENT_CANCELLED", "staff", staffID, map[string]any{
		"payment_id":      updated.ID,
		"previous_status": string(payment.Status),
		"reason":          reason,
	})
	return updated, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id int64) (sqlc.Payment, error) {
	return s.repos.GetPaymentByID(ctx, id)
}

// ListPendingForBranch returns payments in a given status for a branch, so staff
// can see which collections still need confirming. Read-only operational view.
func (s *PaymentService) ListPendingForBranch(ctx context.Context, branchID int64, status sqlc.PaymentStatus) ([]sqlc.ListPaymentsForBranchByStatusRow, error) {
	return s.repos.ListPaymentsForBranchByStatus(ctx, branchID, status)
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
		return method, sqlc.PaymentStatusRequiresStaffConfirmation
	}
	switch method {
	case sqlc.PaymentMethodCash, sqlc.PaymentMethodCardManual:
		return method, sqlc.PaymentStatusRequiresStaffConfirmation
	case sqlc.PaymentMethodUpi:
		return method, sqlc.PaymentStatusRequiresStaffConfirmation
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
	totalPaid, err := s.repos.SumCompletedPaymentsForBillSnapshot(ctx, sessionID, latestCompleted.BillSnapshotID.Int64)
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
