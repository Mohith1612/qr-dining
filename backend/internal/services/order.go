package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type OrderService struct {
	repos     *repository.Repos
	publisher *events.Publisher
	metrics   *observability.Metrics
}

func NewOrderService(repos *repository.Repos, publisher *events.Publisher, metrics *observability.Metrics) *OrderService {
	return &OrderService{repos: repos, publisher: publisher, metrics: metrics}
}

type OrderItem struct {
	MenuItemID  int64           `json:"menu_item_id"`
	Quantity    int16           `json:"quantity"`
	ModifierIDs []int64         `json:"modifier_ids"`
	Modifiers   json.RawMessage `json:"modifiers,omitempty"`
	Note        string          `json:"note"`
}

type PlaceOrderRequest struct {
	SessionID             uuid.UUID
	BranchID              int64
	PlacedByParticipantID int64
	IdempotencyKey        string
	Items                 []OrderItem
}

type PlaceOrderResult struct {
	Order      sqlc.Order
	OrderItems []sqlc.OrderItem
}

// PlaceOrder creates a new order with idempotency protection.
// If the idempotency key already exists, the existing order is returned without error.
func (s *OrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResult, error) {
	// Idempotency check before starting the transaction.
	existing, err := s.repos.GetOrderByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		s.metrics.IdempotencyReplaysTotal.WithLabelValues("order").Inc()
		items, err := s.repos.ListOrderItems(ctx, existing.ID)
		if err != nil {
			return PlaceOrderResult{}, fmt.Errorf("fetch order items on replay: %w", err)
		}
		return PlaceOrderResult{Order: existing, OrderItems: items}, nil
	}
	if !errors.Is(err, domain.ErrOrderNotFound) {
		return PlaceOrderResult{}, err
	}

	// Validate session is still active.
	sess, err := s.repos.GetSessionByID(ctx, req.SessionID)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	if sess.Status != sqlc.SessionStatusActive {
		return PlaceOrderResult{}, domain.ErrSessionClosed
	}

	// Batch-fetch all menu items and modifiers in two queries instead of 2N.
	uniqueItemIDs := make([]int64, 0, len(req.Items))
	seen := make(map[int64]struct{}, len(req.Items))
	for _, item := range req.Items {
		if _, ok := seen[item.MenuItemID]; !ok {
			seen[item.MenuItemID] = struct{}{}
			uniqueItemIDs = append(uniqueItemIDs, item.MenuItemID)
		}
	}

	menuItems, err := s.repos.GetMenuItemsByIDs(ctx, uniqueItemIDs)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("fetch menu items: %w", err)
	}
	menuItemByID := make(map[int64]sqlc.MenuItem, len(menuItems))
	for _, mi := range menuItems {
		menuItemByID[mi.ID] = mi
	}

	allModifiers, err := s.repos.ListModifiersForItems(ctx, uniqueItemIDs)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("fetch modifiers: %w", err)
	}
	modifiersByItemID := make(map[int64][]sqlc.ItemModifier, len(uniqueItemIDs))
	for _, m := range allModifiers {
		modifiersByItemID[m.ItemID] = append(modifiersByItemID[m.ItemID], m)
	}

	// Resolve menu items and compute total.
	type resolvedItem struct {
		MenuItem  sqlc.MenuItem
		Modifiers []ModifierSnapshot
		Req       OrderItem
	}
	resolved := make([]resolvedItem, 0, len(req.Items))
	var totalAmount pgtype.Numeric

	// Accumulate as float, convert at end.
	var totalFloat float64

	for _, item := range req.Items {
		mi, ok := menuItemByID[item.MenuItemID]
		if !ok {
			return PlaceOrderResult{}, domain.ErrMenuItemNotFound
		}
		if !mi.IsAvailable {
			return PlaceOrderResult{}, domain.ErrMenuItemUnavailable
		}

		priceF, err := mi.Price.Float64Value()
		if err != nil {
			return PlaceOrderResult{}, fmt.Errorf("convert price for item %d: %w", mi.ID, err)
		}
		itemTotal := priceF.Float64 * float64(item.Quantity)

		mods, err := snapshotModifiersFromPreloaded(modifiersByItemID[item.MenuItemID], item.ModifierIDs)
		if err != nil {
			return PlaceOrderResult{}, err
		}
		for _, m := range mods {
			itemTotal += m.PriceDelta * float64(item.Quantity)
		}
		totalFloat += itemTotal

		resolved = append(resolved, resolvedItem{MenuItem: mi, Modifiers: mods, Req: item})
	}

	if err := totalAmount.Scan(fmt.Sprintf("%.2f", totalFloat)); err != nil {
		return PlaceOrderResult{}, fmt.Errorf("encode total amount: %w", err)
	}

	var result PlaceOrderResult

	txErr := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		order, err := tx.CreateOrder(ctx, repository.CreateOrderParams{
			SessionID:             req.SessionID,
			BranchID:              req.BranchID,
			PlacedByParticipantID: req.PlacedByParticipantID,
			IdempotencyKey:        req.IdempotencyKey,
			TotalAmount:           totalAmount,
		})
		if err != nil {
			return fmt.Errorf("create order: %w", err)
		}

		orderItems := make([]sqlc.OrderItem, 0, len(resolved))
		for _, ri := range resolved {
			modJSON, _ := json.Marshal(ri.Modifiers)
			price := ri.MenuItem.Price

			oi, err := tx.CreateOrderItem(ctx, sqlc.CreateOrderItemParams{
				OrderID:               order.ID,
				MenuItemID:            ri.MenuItem.ID,
				Quantity:              ri.Req.Quantity,
				UnitPrice:             price,
				SelectedModifiersJson: modJSON,
				Note:                  ri.Req.Note,
			})
			if err != nil {
				return fmt.Errorf("create order item: %w", err)
			}
			orderItems = append(orderItems, oi)
		}

		result = PlaceOrderResult{Order: order, OrderItems: orderItems}
		return nil
	})
	if txErr != nil {
		return PlaceOrderResult{}, txErr
	}

	s.publisher.OrderPlaced(ctx, req.SessionID, result.Order)
	s.repos.LogEvent(ctx, req.SessionID, req.BranchID, "ORDER_PLACED", "participant", req.PlacedByParticipantID, result.Order)

	// Clear the participant's cart after a successful order — best-effort, never blocks the response.
	if cart, err := s.repos.GetOrCreateCart(ctx, req.SessionID, req.PlacedByParticipantID); err == nil {
		_ = s.repos.ClearCart(ctx, cart.ID)
	}

	return result, nil
}

// UpdateOrderStatus applies a state machine-validated status transition.
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, newStatus domain.OrderStatus, staffID int64) (sqlc.Order, error) {
	order, err := s.repos.GetOrderByID(ctx, orderID)
	if err != nil {
		return sqlc.Order{}, err
	}

	current := domain.OrderStatus(order.Status)
	if err := domain.ValidateOrderTransition(current, newStatus); err != nil {
		return sqlc.Order{}, err
	}

	start := time.Now()
	updated, err := s.repos.UpdateOrderStatus(ctx, orderID, sqlc.OrderStatus(newStatus))
	if err != nil {
		return sqlc.Order{}, err
	}
	s.metrics.OrderLifecycleDuration.WithLabelValues(string(current), string(newStatus)).Observe(time.Since(start).Seconds())

	s.publishOrderStatusEvent(ctx, order.SessionID, newStatus, updated)
	s.repos.LogEvent(ctx, order.SessionID, order.BranchID, "ORDER_STATUS_CHANGED", "staff", staffID,
		map[string]any{"order_id": orderID, "new_status": newStatus})
	return updated, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID) (sqlc.Order, error) {
	return s.repos.GetOrderByID(ctx, orderID)
}

func (s *OrderService) ListOrdersForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Order, error) {
	return s.repos.ListOrdersForSession(ctx, sessionID)
}

func (s *OrderService) ListActiveForBranch(ctx context.Context, branchID int64) ([]sqlc.ListActiveOrdersForBranchRow, error) {
	return s.repos.ListActiveOrdersForBranch(ctx, branchID)
}

func (s *OrderService) publishOrderStatusEvent(ctx context.Context, sessionID uuid.UUID, status domain.OrderStatus, order sqlc.Order) {
	payload := map[string]any{"order": order}
	switch status {
	case domain.OrderStatusConfirmed:
		s.publisher.OrderConfirmed(ctx, sessionID, payload)
	case domain.OrderStatusPreparing:
		s.publisher.OrderPreparing(ctx, sessionID, payload)
	case domain.OrderStatusReady:
		s.publisher.OrderReady(ctx, sessionID, payload)
	case domain.OrderStatusServed:
		s.publisher.OrderServed(ctx, sessionID, payload)
	case domain.OrderStatusCancelled:
		s.publisher.OrderCancelled(ctx, sessionID, payload)
	}
}

func snapshotModifiersFromPreloaded(all []sqlc.ItemModifier, modifierIDs []int64) ([]ModifierSnapshot, error) {
	if len(modifierIDs) == 0 {
		return []ModifierSnapshot{}, nil
	}
	byID := make(map[int64]sqlc.ItemModifier, len(all))
	for _, m := range all {
		byID[m.ID] = m
	}
	snapshots := make([]ModifierSnapshot, 0, len(modifierIDs))
	for _, id := range modifierIDs {
		m, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("%w: id %d", domain.ErrModifierNotFound, id)
		}
		delta, err := m.PriceDelta.Float64Value()
		if err != nil {
			return nil, fmt.Errorf("convert modifier price for id %d: %w", id, err)
		}
		snapshots = append(snapshots, ModifierSnapshot{ID: m.ID, Name: m.Name, PriceDelta: delta.Float64})
	}
	return snapshots, nil
}
