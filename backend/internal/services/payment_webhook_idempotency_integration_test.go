//go:build integration

package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// webhookReplayCount reads idempotency_replays_total{entity="webhook"} from the
// service's own registry so the test can assert the replay metric, not just DB state.
func webhookReplayCount(t *testing.T, m *observability.Metrics) float64 {
	t.Helper()
	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != "idempotency_replays_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			for _, lp := range metric.GetLabel() {
				if lp.GetName() == "entity" && lp.GetValue() == "webhook" {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

// TestWebhookReplay_IncrementsReplayMetricNoDoubleSettle closes the Phase D gap:
// it proves that an exact webhook replay (a) increments
// idempotency_replays_total{entity="webhook"} and (b) does not double-settle —
// exactly one webhook_events row and the payment ends completed exactly once.
//
// Signature + timestamp rejection (W-03/W-04) live in the HTTP handler
// (verifyGenericWebhook) and are validated live in scripts/chaos/webhook-replay.sh.
func TestWebhookReplay_IncrementsReplayMetricNoDoubleSettle(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	m := observability.NewMetrics()
	sessionSvc := services.NewSessionService(repos, pub, m, nil)
	paymentSvc := services.NewPaymentService(repos, pub, m, sessionSvc, zerolog.Nop())
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
		ProviderPaymentRef: "pay_metric_1",
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"payment_ref": "pay_metric_1",
		"status":      "completed",
		"amount":      50.00,
		"currency":    "INR",
		"session_id":  sess.Session.ID.String(),
		"branch_id":   f.BranchID,
	})
	req := services.ProcessWebhookRequest{
		Provider:        "razorpay",
		ExternalEventID: uuid.NewString(),
		EventType:       "payment.captured",
		Payload:         payload,
		RawPayload:      string(payload),
		Headers:         json.RawMessage(`{}`),
	}

	before := webhookReplayCount(t, m)
	if err := paymentSvc.ProcessWebhook(ctx, req); err != nil {
		t.Fatalf("first ProcessWebhook: %v", err)
	}
	if got := webhookReplayCount(t, m); got != before {
		t.Fatalf("first delivery must not count as replay: before=%v after=%v", before, got)
	}

	if err := paymentSvc.ProcessWebhook(ctx, req); err != nil {
		t.Fatalf("replay ProcessWebhook: %v", err)
	}
	if got := webhookReplayCount(t, m); got != before+1 {
		t.Fatalf("replay must increment idempotency_replays_total{webhook}: got %v, want %v", got, before+1)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payment_webhook_events WHERE external_event_id = $1`, req.ExternalEventID,
	).Scan(&count); err != nil {
		t.Fatalf("count webhook events: %v", err)
	}
	if count != 1 {
		t.Errorf("webhook_events rows: got %d, want 1 (no duplicate processing)", count)
	}

	updated, err := paymentSvc.GetPayment(ctx, payment.ID)
	if err != nil {
		t.Fatalf("GetPayment: %v", err)
	}
	if updated.Status != sqlc.PaymentStatusCompleted {
		t.Errorf("payment status after replay: got %q, want completed (settled exactly once)", updated.Status)
	}
}
