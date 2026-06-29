//go:build integration

package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
)

func TestCart_AddAndRemoveItem(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	cartSvc := services.NewCartService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "carts", "cart_items")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	item, err := cartSvc.AddItem(ctx, services.AddItemRequest{
		SessionID:     sess.Session.ID,
		ParticipantID: sess.Participant.ID,
		MenuItemID:    f.MenuItemID,
		Quantity:      2,
		ModifierIDs:   nil,
		Note:          "no onions",
	})
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("expected non-zero cart item ID")
	}

	cart, err := cartSvc.GetCart(ctx, sess.Session.ID, sess.Participant.ID)
	if err != nil {
		t.Fatalf("GetCart: %v", err)
	}
	if len(cart.Items) != 1 {
		t.Fatalf("cart items after add: got %d, want 1", len(cart.Items))
	}

	if err := cartSvc.RemoveItem(ctx, sess.Session.ID, sess.Participant.ID, item.ID); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}

	cart, err = cartSvc.GetCart(ctx, sess.Session.ID, sess.Participant.ID)
	if err != nil {
		t.Fatalf("GetCart after remove: %v", err)
	}
	if len(cart.Items) != 0 {
		t.Errorf("cart items after remove: got %d, want 0", len(cart.Items))
	}
}

func TestCart_ModifierSnapshot(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	cartSvc := services.NewCartService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "carts", "cart_items")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	item, err := cartSvc.AddItem(ctx, services.AddItemRequest{
		SessionID:     sess.Session.ID,
		ParticipantID: sess.Participant.ID,
		MenuItemID:    f.MenuItemID,
		Quantity:      1,
		ModifierIDs:   []int64{f.ModifierID},
	})
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	// Modifier snapshot must be non-empty JSON array.
	if len(item.SelectedModifiersJson) == 0 {
		t.Fatal("expected non-empty modifier snapshot")
	}
	var mods []map[string]any
	if err := json.Unmarshal(item.SelectedModifiersJson, &mods); err != nil {
		t.Fatalf("unmarshal modifiers: %v", err)
	}
	if len(mods) != 1 {
		t.Errorf("modifier count: got %d, want 1", len(mods))
	}
	if _, ok := mods[0]["price_delta"]; !ok {
		t.Error("modifier snapshot missing price_delta field")
	}
}
