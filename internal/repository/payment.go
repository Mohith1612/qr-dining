package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) CreatePayment(ctx context.Context, p sqlc.CreatePaymentParams) (sqlc.Payment, error) {
	return r.q.CreatePayment(ctx, p)
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

func (r *Repos) ListPaymentsForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Payment, error) {
	return r.q.ListPaymentsForSession(ctx, sessionID)
}

// InsertWebhookEvent inserts a new webhook event with ON CONFLICT DO NOTHING.
// Returns (event, true) if inserted, (zero, false) if already existed (idempotent replay).
func (r *Repos) InsertWebhookEvent(ctx context.Context, externalID, provider, eventType string, payload json.RawMessage) (sqlc.PaymentWebhookEvent, bool, error) {
	ev, err := r.q.InsertWebhookEvent(ctx, sqlc.InsertWebhookEventParams{
		ExternalEventID: externalID,
		Provider:        provider,
		EventType:       eventType,
		Payload:         payload,
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
