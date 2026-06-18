//go:build integration

package services_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
)

func TestPlaceOrder_HappyPath(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	orderSvc := newTestOrderService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	result, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items: []services.OrderItem{
			{MenuItemID: f.MenuItemID, Quantity: 2, ModifierIDs: []int64{f.ModifierID}},
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if result.Order.ID == (uuid.UUID{}) {
		t.Fatal("expected non-zero order ID")
	}
	if len(result.OrderItems) != 1 {
		t.Errorf("order items: got %d, want 1", len(result.OrderItems))
	}

	// Total = (10.00 + 1.50) * 2 = 23.00
	total, _ := result.Order.TotalAmount.Float64Value()
	if total.Float64 != 23.00 {
		t.Errorf("total amount: got %.2f, want 23.00", total.Float64)
	}
}

func TestPlaceOrder_Idempotent(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	orderSvc := newTestOrderService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
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

	second, err := orderSvc.PlaceOrder(ctx, req)
	if err != nil {
		t.Fatalf("second PlaceOrder: %v", err)
	}

	if first.Order.ID != second.Order.ID {
		t.Errorf("idempotency violated: first=%s, second=%s", first.Order.ID, second.Order.ID)
	}

	// Only one row in DB.
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE idempotency_key = $1`, key).Scan(&count); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if count != 1 {
		t.Errorf("order count: got %d, want 1", count)
	}
}

func TestUpdateOrderStatus_InvalidTransition(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	orderSvc := newTestOrderService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	result, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// pending → served is an invalid transition.
	_, err = orderSvc.UpdateOrderStatus(ctx, result.Order.ID, domain.OrderStatusServed, 1)
	if err == nil {
		t.Fatal("expected ErrInvalidOrderTransition, got nil")
	}
	if !isErr(err, domain.ErrInvalidOrderTransition) {
		t.Errorf("expected ErrInvalidOrderTransition, got %v", err)
	}
}
