package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
)

type CartService struct {
	repos     *repository.Repos
	publisher *events.Publisher
}

func NewCartService(repos *repository.Repos, publisher *events.Publisher) *CartService {
	return &CartService{repos: repos, publisher: publisher}
}

type ModifierSnapshot struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	PriceDelta float64 `json:"price_delta"`
}

type AddItemRequest struct {
	SessionID     uuid.UUID
	ParticipantID int64
	MenuItemID    int64
	Quantity      int16
	ModifierIDs   []int64
	Note          string
}

// CartResponse is returned verbatim by GET /sessions/{id}/cart, so its tags are
// the wire contract. Without them Go exported the field names as `Cart`/`Items`,
// which contradicted every other response in the API — the same accidental
// leakage as PlaceOrderResult (F-24, F-26). snake_case, like the rest.
type CartResponse struct {
	Cart  sqlc.Cart               `json:"cart"`
	Items []sqlc.ListCartItemsRow `json:"items"`
}

func (s *CartService) GetCart(ctx context.Context, sessionID uuid.UUID, participantID int64) (CartResponse, error) {
	cart, err := s.repos.GetOrCreateSessionCart(ctx, sessionID)
	if err != nil {
		return CartResponse{}, err
	}
	items, err := s.repos.ListCartItems(ctx, cart.ID)
	if err != nil {
		return CartResponse{}, err
	}
	return CartResponse{Cart: cart, Items: items}, nil
}

func (s *CartService) AddItem(ctx context.Context, req AddItemRequest) (sqlc.CartItem, error) {
	// Reject mutations once the session has entered payment_pending or any
	// terminal state. Reads remain allowed via GetCart.
	sess, err := s.repos.GetSessionByID(ctx, req.SessionID)
	if err != nil {
		return sqlc.CartItem{}, err
	}
	if domain.IsSessionCartFrozen(domain.SessionStatus(sess.Status)) {
		return sqlc.CartItem{}, domain.ErrPaymentInProgress
	}
	if domain.IsSessionTerminal(domain.SessionStatus(sess.Status)) {
		return sqlc.CartItem{}, domain.ErrSessionClosed
	}

	// Validate menu item availability.
	menuItem, err := s.repos.GetMenuItemByID(ctx, req.MenuItemID)
	if err != nil {
		return sqlc.CartItem{}, err
	}
	if !menuItem.IsAvailable {
		return sqlc.CartItem{}, domain.ErrMenuItemUnavailable
	}

	// Snapshot modifier details at add-to-cart time.
	modifiers, err := s.snapshotModifiers(ctx, req.MenuItemID, req.ModifierIDs)
	if err != nil {
		return sqlc.CartItem{}, fmt.Errorf("snapshot modifiers: %w", err)
	}

	modJSON, err := json.Marshal(modifiers)
	if err != nil {
		return sqlc.CartItem{}, fmt.Errorf("marshal modifiers: %w", err)
	}

	cart, err := s.repos.GetOrCreateSessionCart(ctx, req.SessionID)
	if err != nil {
		return sqlc.CartItem{}, err
	}

	item, err := s.repos.AddCartItem(ctx, cart.ID, req.MenuItemID, req.Quantity, modJSON, req.Note)
	if err != nil {
		return sqlc.CartItem{}, err
	}

	s.publisher.CartUpdated(ctx, req.SessionID, map[string]any{
		"action":     "add",
		"cart_item":  item,
		"session_id": req.SessionID,
	})
	// menuItem.BranchID is already fetched above — no extra query needed.
	s.repos.LogEvent(ctx, req.SessionID, menuItem.BranchID, "CART_UPDATED", "participant", req.ParticipantID, map[string]any{"action": "add", "item_id": item.ID})
	return item, nil
}

func (s *CartService) RemoveItem(ctx context.Context, sessionID uuid.UUID, participantID, itemID int64) error {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if domain.IsSessionCartFrozen(domain.SessionStatus(sess.Status)) {
		return domain.ErrPaymentInProgress
	}
	if domain.IsSessionTerminal(domain.SessionStatus(sess.Status)) {
		return domain.ErrSessionClosed
	}

	cart, err := s.repos.GetOrCreateSessionCart(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify the item belongs to this cart.
	cartItem, err := s.repos.GetCartItem(ctx, itemID)
	if err != nil {
		return err
	}
	if cartItem.CartID != cart.ID {
		return domain.ErrCartItemNotFound
	}

	if err := s.repos.RemoveCartItem(ctx, cart.ID, itemID); err != nil {
		return err
	}

	s.publisher.CartUpdated(ctx, sessionID, map[string]any{
		"action":     "remove",
		"item_id":    itemID,
		"session_id": sessionID,
	})
	s.repos.LogEvent(ctx, sessionID, sess.BranchID, "CART_UPDATED", "participant", participantID, map[string]any{"action": "remove", "item_id": itemID})
	return nil
}

func (s *CartService) snapshotModifiers(ctx context.Context, itemID int64, modifierIDs []int64) ([]ModifierSnapshot, error) {
	if len(modifierIDs) == 0 {
		return []ModifierSnapshot{}, nil
	}

	allModifiers, err := s.repos.ListModifiersForItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	idSet := make(map[int64]sqlc.ItemModifier, len(allModifiers))
	for _, m := range allModifiers {
		idSet[m.ID] = m
	}

	// A modifier group is single-select if ANY of its rows carry the flag.
	singleSelectGroup := make(map[string]bool)
	for _, m := range allModifiers {
		if m.SingleSelect {
			singleSelectGroup[m.ModifierGroup] = true
		}
	}

	snapshots := make([]ModifierSnapshot, 0, len(modifierIDs))
	groupChosen := make(map[string]int64)
	for _, id := range modifierIDs {
		m, ok := idSet[id]
		if !ok {
			return nil, fmt.Errorf("%w: id %d", domain.ErrModifierNotFound, id)
		}
		// Enforce single-select: at most one option per single-select group.
		if singleSelectGroup[m.ModifierGroup] {
			if prev, dup := groupChosen[m.ModifierGroup]; dup && prev != id {
				return nil, fmt.Errorf("%w: group %q", domain.ErrModifierConflict, m.ModifierGroup)
			}
			groupChosen[m.ModifierGroup] = id
		}
		delta, err := m.PriceDelta.Float64Value()
		if err != nil {
			return nil, fmt.Errorf("convert modifier price for id %d: %w", id, err)
		}
		snapshots = append(snapshots, ModifierSnapshot{
			ID:         m.ID,
			Name:       m.Name,
			PriceDelta: delta.Float64,
		})
	}
	return snapshots, nil
}
