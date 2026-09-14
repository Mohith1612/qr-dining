//go:build integration

package services_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// idempotency_keys.expires_at is written at reservation (now + 24h) but was
// never consulted on lookup, so a key replayed to its original resource
// forever. These tests pin the expiry down as enforced, in both write paths
// that reserve keys — payments and orders.
//
// "Expired" means the key is a fresh request that supersedes the stale row,
// not a conflict: the guest tapping pay a day later must be able to pay, and
// must not silently receive the old payment. It also has to be the same answer
// whether or not the reaper has swept the row yet, or reaper scheduling would
// become semantically load-bearing.

// expireIdempotencyKey backdates a reserved key past its expiry, standing in
// for the 24h of wall-clock the test cannot wait for.
func expireIdempotencyKey(t *testing.T, pool *pgxpool.Pool, key string) {
	t.Helper()
	tag, err := pool.Exec(context.Background(),
		`UPDATE idempotency_keys SET expires_at = NOW() - INTERVAL '1 hour' WHERE key = $1`, key)
	if err != nil {
		t.Fatalf("expire idempotency key: %v", err)
	}
	if tag.RowsAffected() == 0 {
		t.Fatalf("expire idempotency key %q: no such row", key)
	}
}

func TestInitiatePayment_ExpiredIdempotencyKeyDoesNotReplay(t *testing.T) {
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
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Asha", "fp-asha", "")
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
			Subtotal:       50.00,
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
		},
	}

	first, err := paymentSvc.InitiatePayment(ctx, req)
	if err != nil {
		t.Fatalf("first InitiatePayment: %v", err)
	}

	// Retire the first attempt so the session is payable again. Without this the
	// session-level reuse path would hand back the same non-terminal payment and
	// the assertion below could not tell replay from reuse.
	if _, err := pool.Exec(ctx, `UPDATE payments SET status = 'cancelled' WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("cancel first payment: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sessions SET status = 'active' WHERE id = $1`, sess.Session.ID); err != nil {
		t.Fatalf("reactivate session: %v", err)
	}

	expireIdempotencyKey(t, pool, key)

	// A day later the guest taps pay again and the client re-sends its stored
	// key. The expired key is no longer evidence of a retry.
	second, err := paymentSvc.InitiatePayment(ctx, req)
	if err != nil {
		t.Fatalf("replay of expired key: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expired idempotency key replayed to the original payment %d; expected a fresh payment", first.ID)
	}
	if second.Status == sqlc.PaymentStatusCancelled {
		t.Fatalf("guest received a cancelled payment (id=%d) from an expired key", second.ID)
	}
}

func TestInitiatePayment_LiveIdempotencyKeyStillReplays(t *testing.T) {
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
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Asha", "fp-asha", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	req := services.InitiatePaymentRequest{
		SessionID:      sess.Session.ID,
		BranchID:       f.BranchID,
		Method:         sqlc.PaymentMethodCash,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        sess.Participant.ID,
		Bill: services.BillSnapshotInput{
			Subtotal:       50.00,
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
		t.Fatalf("live idempotency key did not replay: first=%d second=%d", first.ID, second.ID)
	}
}

func TestPlaceOrder_ExpiredIdempotencyKeyDoesNotReplay(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	orderSvc := newTestOrderService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items", "idempotency_keys")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Asha", "fp-asha", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	key := uuid.NewString()
	req := services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        key,
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	}

	first, err := orderSvc.PlaceOrder(ctx, req)
	if err != nil {
		t.Fatalf("first PlaceOrder: %v", err)
	}

	expireIdempotencyKey(t, pool, key)

	second, err := orderSvc.PlaceOrder(ctx, req)
	if err != nil {
		t.Fatalf("replay of expired key: %v", err)
	}
	if second.Order.ID == first.Order.ID {
		t.Fatalf("expired idempotency key replayed to the original order %s; expected a fresh order", first.Order.ID)
	}
	if len(second.OrderItems) != 1 {
		t.Fatalf("fresh order items: got %d, want 1", len(second.OrderItems))
	}
}
