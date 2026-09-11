//go:build integration

package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
)

func TestInitiatePaymentRequiresStaffConfirmationForEveryMethod(t *testing.T) {
	tests := []struct {
		name   string
		method sqlc.PaymentMethod
	}{
		{name: "cash", method: sqlc.PaymentMethodCash},
		{name: "card", method: sqlc.PaymentMethodCard},
		{name: "card_manual", method: sqlc.PaymentMethodCardManual},
		{name: "upi", method: sqlc.PaymentMethodUpi},
		{name: "digital", method: sqlc.PaymentMethodDigital},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testutil.OpenTestDB(t)
			f := testutil.SeedFixtures(t, pool)
			repos := testutil.NewTestRepos(pool)
			pub := events.NewNoopPublisher()
			sessionSvc := newTestSessionService(repos, pub)
			paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
			t.Cleanup(func() {
				testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "bill_snapshots", "idempotency_keys")
			})

			ctx := context.Background()
			sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}

			payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
				SessionID:      sess.Session.ID,
				BranchID:       f.BranchID,
				Method:         tt.method,
				IdempotencyKey: uuid.NewString(),
				ActorType:      "participant",
				ActorID:        sess.Participant.ID,
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
		})
	}
}

func TestInitiatePayment_AbsentParticipantCannotBecomeHost(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "bill_snapshots", "idempotency_keys")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	absentGuest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	setParticipantDBLastSeen(t, pool, created.Participant.ID, time.Now().Add(-6*time.Minute))

	_, err = paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:      created.Session.ID,
		BranchID:       f.BranchID,
		Method:         sqlc.PaymentMethodDigital,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        absentGuest.ID,
		Bill: services.BillSnapshotInput{
			Total:          50,
			Currency:       "INR",
			CreatedByActor: "guest",
		},
		Provider:           "razorpay",
		ProviderPaymentRef: "pay_absent_actor",
	})
	if !errors.Is(err, domain.ErrNotSessionHost) {
		t.Fatalf("absent guest InitiatePayment: got %v, want ErrNotSessionHost", err)
	}

	var paymentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE session_id = $1`, created.Session.ID).Scan(&paymentCount); err != nil {
		t.Fatalf("count payments: %v", err)
	}
	if paymentCount != 0 {
		t.Fatalf("payment count: got %d, want 0", paymentCount)
	}
}

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
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:      sess.Session.ID,
		BranchID:       f.BranchID,
		Method:         sqlc.PaymentMethodDigital,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        sess.Participant.ID,
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
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Place an order BEFORE initiating payment (the order path is frozen once
	// payment is in progress) and record it as the bill snapshot's source order.
	placed, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:               sess.Session.ID,
		BranchID:                f.BranchID,
		Method:                  sqlc.PaymentMethodCash,
		StaffSettlementRequired: true,
		IdempotencyKey:          uuid.NewString(),
		ActorType:               "participant",
		ActorID:                 sess.Participant.ID,
		Bill: services.BillSnapshotInput{
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
			SourceOrderIDs: []uuid.UUID{placed.Order.ID},
		},
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}
	if payment.Status != sqlc.PaymentStatusRequiresStaffConfirmation {
		t.Fatalf("payment status: got %s, want requires_staff_confirmation", payment.Status)
	}

	// Drift the active-order set after the snapshot was captured: cancelling the
	// snapshotted order makes the snapshot stale. Applied directly because the
	// order path is frozen during payment — this simulates the drift the
	// staff-settlement freshness guard must reject.
	if _, err := pool.Exec(ctx, `UPDATE orders SET status = 'cancelled' WHERE id = $1`, placed.Order.ID); err != nil {
		t.Fatalf("cancel snapshotted order: %v", err)
	}

	if _, err := paymentSvc.SettlePaymentByStaff(ctx, payment.ID, 1, f.BranchID); !isErr(err, domain.ErrBillSnapshotStale) {
		t.Fatalf("settle stale snapshot error: got %v, want ErrBillSnapshotStale", err)
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
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	payment, err := paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:      sess.Session.ID,
		BranchID:       f.BranchID,
		Method:         sqlc.PaymentMethodDigital,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        sess.Participant.ID,
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

func TestInitiatePayment_IdempotencyConflictAndReplay(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "bill_snapshots", "idempotency_keys")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	key := uuid.NewString()
	req := services.InitiatePaymentRequest{
		SessionID:      sess.Session.ID,
		BranchID:       f.BranchID,
		Method:         sqlc.PaymentMethodCash,
		IdempotencyKey: key,
		ActorType:      "participant",
		ActorID:        sess.Participant.ID,
		Bill: services.BillSnapshotInput{
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
		},
	}
	first, err := paymentSvc.InitiatePayment(ctx, req)
	if err != nil {
		t.Fatalf("first InitiatePayment: %v", err)
	}
	second, err := paymentSvc.InitiatePayment(ctx, req)
	if err != nil {
		t.Fatalf("second InitiatePayment: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("payment replay returned different payment: first=%d second=%d", first.ID, second.ID)
	}
	req.Bill.Total = 60.00
	if _, err := paymentSvc.InitiatePayment(ctx, req); !isErr(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("changed payment replay error: got %v, want idempotency conflict", err)
	}
}
