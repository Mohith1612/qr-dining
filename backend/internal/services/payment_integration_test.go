//go:build integration

package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
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
		BranchID:  f.BranchID,
		Method:    sqlc.PaymentMethodDigital,
		Bill: services.BillSnapshotInput{
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
		},
		Provider:           "razorpay",
		ProviderPaymentRef: "pay_test_1",
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"payment_ref": "pay_test_1",
		"status":      "completed",
		"amount":      50.00,
		"currency":    "INR",
		"session_id":  sess.Session.ID.String(),
		"branch_id":   f.BranchID,
	})
	externalID := uuid.NewString()
	req := services.ProcessWebhookRequest{
		Provider:        "razorpay",
		ExternalEventID: externalID,
		EventType:       "payment.captured",
		Payload:         payload,
		RawPayload:      string(payload),
		Headers:         json.RawMessage(`{}`),
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

func TestManualPaymentRequiresStaffSettlementAndRejectsStaleSnapshot(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	orderSvc := newTestOrderService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "bill_snapshots", "orders", "order_items", "idempotency_keys")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:               sess.Session.ID,
		BranchID:                f.BranchID,
		Method:                  sqlc.PaymentMethodCash,
		StaffSettlementRequired: true,
		Bill: services.BillSnapshotInput{
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
		},
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}
	if payment.Status != sqlc.PaymentStatusRequiresStaffConfirmation {
		t.Fatalf("payment status: got %s, want requires_staff_confirmation", payment.Status)
	}
	if _, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if _, err := paymentSvc.SettlePaymentByStaff(ctx, payment.ID, 1, f.BranchID); !isErr(err, domain.ErrBillSnapshotStale) {
		t.Fatalf("settle stale snapshot error: got %v", err)
	}
}

func TestWebhookRejectsAmountMismatch(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "payment_webhook_events", "bill_snapshots")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID: sess.Session.ID,
		BranchID:  f.BranchID,
		Method:    sqlc.PaymentMethodDigital,
		Bill: services.BillSnapshotInput{
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
		},
		Provider:           "razorpay",
		ProviderPaymentRef: "pay_test_mismatch",
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"payment_ref": "pay_test_mismatch",
		"status":      "completed",
		"amount":      49.00,
		"currency":    "INR",
		"session_id":  sess.Session.ID.String(),
		"branch_id":   f.BranchID,
	})
	if err := paymentSvc.ProcessWebhook(ctx, services.ProcessWebhookRequest{
		Provider:        "razorpay",
		ExternalEventID: uuid.NewString(),
		EventType:       "payment.captured",
		Payload:         payload,
		RawPayload:      string(payload),
		Headers:         json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("ProcessWebhook: %v", err)
	}
	updated, err := paymentSvc.GetPayment(ctx, payment.ID)
	if err != nil {
		t.Fatalf("GetPayment: %v", err)
	}
	if updated.Status == sqlc.PaymentStatusCompleted {
		t.Fatal("amount-mismatched webhook completed payment")
	}
}
