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
	"github.com/jackc/pgx/v5/pgxpool"
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

func TestPlaceOrder_IdempotencyConflictAndScopedReplay(t *testing.T) {
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
	firstSession, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession first: %v", err)
	}
	secondTableID := seedTable(t, pool, f.BranchID, "T2")
	secondSession, err := sessionSvc.CreateSession(ctx, secondTableID, "Bob", "fp-bob")
	if err != nil {
		t.Fatalf("CreateSession second: %v", err)
	}

	key := uuid.NewString()
	req := services.PlaceOrderRequest{
		SessionID:             firstSession.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: firstSession.Participant.ID,
		IdempotencyKey:        key,
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	}
	if _, err := orderSvc.PlaceOrder(ctx, req); err != nil {
		t.Fatalf("first PlaceOrder: %v", err)
	}
	req.Items[0].Quantity = 2
	if _, err := orderSvc.PlaceOrder(ctx, req); !isErr(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("changed replay error: got %v, want idempotency conflict", err)
	}

	_, err = orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             secondSession.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: secondSession.Participant.ID,
		IdempotencyKey:        key,
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("same key in different scope: %v", err)
	}
}

func TestPlaceOrder_RejectsCrossBranchMenuAndParticipant(t *testing.T) {
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
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	otherTableID := seedTable(t, pool, f.BranchID, "T2")
	otherSess, err := sessionSvc.CreateSession(ctx, otherTableID, "Bob", "fp-bob")
	if err != nil {
		t.Fatalf("CreateSession other: %v", err)
	}
	otherBranchID, otherItemID := seedSecondBranchMenuItem(t, pool, f.RestaurantID, f.OrganizationID)

	_, err = orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: otherItemID, Quantity: 1}},
	})
	if !isErr(err, domain.ErrMenuItemNotFound) {
		t.Fatalf("cross-branch menu error: got %v", err)
	}

	_, err = orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              otherBranchID,
		PlacedByParticipantID: otherSess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if !isErr(err, domain.ErrTenantMismatch) {
		t.Fatalf("cross-branch/participant error: got %v", err)
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
	_, err = orderSvc.UpdateOrderStatus(ctx, result.Order.ID, f.BranchID, domain.OrderStatusServed, 1)
	if err == nil {
		t.Fatal("expected ErrInvalidOrderTransition, got nil")
	}
	if !isErr(err, domain.ErrInvalidOrderTransition) {
		t.Errorf("expected ErrInvalidOrderTransition, got %v", err)
	}
}

func seedTable(t testing.TB, pool *pgxpool.Pool, branchID int64, identifier string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
		 VALUES ($1, $2, 4, $3, 'available') RETURNING id`,
		branchID, identifier, uuid.NewString(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed table: %v", err)
	}
	return id
}

func seedSecondBranchMenuItem(t testing.TB, pool *pgxpool.Pool, restaurantID, organizationID int64) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var branchID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO branches (restaurant_id, organization_id, name, address, timezone, branch_code)
		 VALUES ($1, $2, 'Other Branch', '2 Test St', 'UTC', $3) RETURNING id`,
		restaurantID, organizationID, "OTHER-"+uuid.NewString(),
	).Scan(&branchID); err != nil {
		t.Fatalf("seed second branch: %v", err)
	}
	var categoryID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO menu_categories (branch_id, name, position) VALUES ($1, 'Mains', 1) RETURNING id`,
		branchID,
	).Scan(&categoryID); err != nil {
		t.Fatalf("seed second category: %v", err)
	}
	var itemID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO menu_items (category_id, branch_id, name, price, is_available) VALUES ($1, $2, 'Other Item', 10.00, true) RETURNING id`,
		categoryID, branchID,
	).Scan(&itemID); err != nil {
		t.Fatalf("seed second item: %v", err)
	}
	return branchID, itemID
}
