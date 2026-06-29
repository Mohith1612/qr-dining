package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// StalledPaymentPending is a session stuck in payment_pending whose oldest
// non-terminal payment was initiated before the escalation threshold.
type StalledPaymentPending struct {
	SessionID      uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
	PaymentID      int64
	InitiatedAt    time.Time
}

// ListPaymentPendingStalled returns payment_pending sessions whose oldest
// non-terminal payment was initiated before olderThan. Read-only; used by the
// alert-only escalation worker. Bounded to avoid a runaway sweep.
func (r *Repos) ListPaymentPendingStalled(ctx context.Context, olderThan time.Time) ([]StalledPaymentPending, error) {
	rows, err := r.db.Query(ctx, `
SELECT s.id, b.organization_id, s.branch_id, s.table_id, p.id, p.initiated_at
FROM sessions s
JOIN branches b ON b.id = s.branch_id
JOIN LATERAL (
    SELECT id, initiated_at
    FROM payments
    WHERE session_id = s.id
      AND status IN ('pending','requested','provider_pending','requires_staff_confirmation')
    ORDER BY initiated_at ASC
    LIMIT 1
) p ON TRUE
WHERE s.status = 'payment_pending'
  AND p.initiated_at < $1
ORDER BY p.initiated_at ASC
LIMIT 200`, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StalledPaymentPending
	for rows.Next() {
		var s StalledPaymentPending
		if err := rows.Scan(&s.SessionID, &s.OrganizationID, &s.BranchID, &s.TableID, &s.PaymentID, &s.InitiatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repos) CreatePayment(ctx context.Context, p sqlc.CreatePaymentParams) (sqlc.Payment, error) {
	return r.q.CreatePayment(ctx, p)
}

func (r *Repos) NextPaymentNumber(ctx context.Context, branchID int64, businessDate pgtype.Date) (int32, error) {
	return r.q.NextPaymentNumber(ctx, sqlc.NextPaymentNumberParams{
		BranchID: branchID,
		Date:     businessDate,
	})
}

func (r *Repos) CreateBillSnapshot(ctx context.Context, p sqlc.CreateBillSnapshotParams) (sqlc.BillSnapshot, error) {
	return r.q.CreateBillSnapshot(ctx, p)
}

func (r *Repos) GetBillSnapshotByID(ctx context.Context, id int64) (sqlc.BillSnapshot, error) {
	return r.q.GetBillSnapshotByID(ctx, id)
}

func (r *Repos) GetPaymentByID(ctx context.Context, id int64) (sqlc.Payment, error) {
	pay, err := r.q.GetPaymentByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Payment{}, domain.ErrPaymentNotFound
	}
	return pay, err
}

func (r *Repos) UpdatePaymentStatus(ctx context.Context, id int64, status sqlc.PaymentStatus) (sqlc.Payment, error) {
	pay, err := r.q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{
		ID:     id,
		Status: status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Payment{}, domain.ErrPaymentNotFound
	}
	return pay, err
}

func (r *Repos) UpdatePaymentStatusExpected(ctx context.Context, id int64, current, next sqlc.PaymentStatus) (sqlc.Payment, error) {
	pay, err := r.q.UpdatePaymentStatusExpected(ctx, sqlc.UpdatePaymentStatusExpectedParams{
		ID:       id,
		Status:   current,
		Status_2: next,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Payment{}, domain.ErrInvalidPaymentTransition
	}
	return pay, err
}

func (r *Repos) SettlePaymentByStaff(ctx context.Context, id, staffID, branchID int64) (sqlc.Payment, error) {
	pay, err := r.q.SettlePaymentByStaff(ctx, sqlc.SettlePaymentByStaffParams{
		ID:               id,
		SettledByStaffID: pgtype.Int8{Int64: staffID, Valid: true},
		BranchID:         branchID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Payment{}, domain.ErrInvalidPaymentTransition
	}
	return pay, err
}

func (r *Repos) ListPaymentsForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Payment, error) {
	return r.q.ListPaymentsForSession(ctx, sessionID)
}

func (r *Repos) ListPaymentsForBranchByStatus(ctx context.Context, branchID int64, status sqlc.PaymentStatus) ([]sqlc.ListPaymentsForBranchByStatusRow, error) {
	return r.q.ListPaymentsForBranchByStatus(ctx, sqlc.ListPaymentsForBranchByStatusParams{BranchID: branchID, Status: status})
}

// InsertWebhookEvent inserts a new webhook event with ON CONFLICT DO NOTHING.
// Returns (event, true) if inserted, (zero, false) if already existed (idempotent replay).
func (r *Repos) InsertWebhookEvent(ctx context.Context, externalID, provider, eventType string, payload json.RawMessage, rawPayload string, headers json.RawMessage) (sqlc.PaymentWebhookEvent, bool, error) {
	ev, err := r.q.InsertWebhookEvent(ctx, sqlc.InsertWebhookEventParams{
		ExternalEventID: externalID,
		Provider:        provider,
		EventType:       eventType,
		Payload:         payload,
		RawPayload:      pgtype.Text{String: rawPayload, Valid: rawPayload != ""},
		Headers:         headers,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING — already processed
		return sqlc.PaymentWebhookEvent{}, false, nil
	}
	if err != nil {
		return sqlc.PaymentWebhookEvent{}, false, err
	}
	return ev, true, nil
}

func (r *Repos) GetPaymentByProviderRef(ctx context.Context, provider, providerRef string) (sqlc.Payment, error) {
	pay, err := r.q.GetPaymentByProviderRef(ctx, sqlc.GetPaymentByProviderRefParams{
		Provider:           pgtype.Text{String: provider, Valid: true},
		ProviderPaymentRef: pgtype.Text{String: providerRef, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Payment{}, domain.ErrPaymentNotFound
	}
	return pay, err
}

func (r *Repos) SumCompletedPaymentsForSession(ctx context.Context, sessionID uuid.UUID) (pgtype.Numeric, error) {
	return r.q.SumCompletedPaymentsForSession(ctx, sessionID)
}

func (r *Repos) MarkWebhookProcessed(ctx context.Context, id, paymentID int64, errMsg string) error {
	var payID pgtype.Int8
	if paymentID != 0 {
		payID = pgtype.Int8{Int64: paymentID, Valid: true}
	}
	var errText pgtype.Text
	if errMsg != "" {
		errText = pgtype.Text{String: errMsg, Valid: true}
	}
	return r.q.MarkWebhookProcessed(ctx, sqlc.MarkWebhookProcessedParams{
		ID:           id,
		PaymentID:    payID,
		ErrorMessage: errText,
	})
}

func (r *Repos) ListUnprocessedWebhooks(ctx context.Context) ([]sqlc.PaymentWebhookEvent, error) {
	return r.q.ListUnprocessedWebhooks(ctx)
}
