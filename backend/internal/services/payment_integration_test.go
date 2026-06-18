//go:build integration

package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
)

func TestWebhookReplay_Idempotent(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "payment_webhook_events")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID: sess.Session.ID,
		Amount:    50.00,
		Method:    sqlc.PaymentMethodDigital,
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"payment_id": payment.ID,
		"status":     "completed",
	})
	externalID := uuid.NewString()
	req := services.ProcessWebhookRequest{
		Provider:        "razorpay",
		ExternalEventID: externalID,
		EventType:       "payment.captured",
		Payload:         payload,
	}

	// First call — should process.
	if err := paymentSvc.ProcessWebhook(ctx, req); err != nil {
		t.Fatalf("first ProcessWebhook: %v", err)
	}

	// Second call — must be idempotent.
	if err := paymentSvc.ProcessWebhook(ctx, req); err != nil {
		t.Fatalf("second ProcessWebhook: %v", err)
	}

	// Exactly one row in payment_webhook_events.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payment_webhook_events WHERE external_event_id = $1`, externalID,
	).Scan(&count); err != nil {
		t.Fatalf("count webhook events: %v", err)
	}
	if count != 1 {
		t.Errorf("webhook_events count: got %d, want 1", count)
	}

	// Payment status updated to completed.
	updated, err := paymentSvc.GetPayment(ctx, payment.ID)
	if err != nil {
		t.Fatalf("GetPayment: %v", err)
	}
	if updated.Status != sqlc.PaymentStatusCompleted {
		t.Errorf("payment status: got %q, want completed", updated.Status)
	}
}
