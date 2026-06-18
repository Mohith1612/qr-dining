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
	promoSvc  *PromoService
}

func NewOrderService(repos *repository.Repos, publisher *events.Publisher, metrics *observability.Metrics, promoSvc *PromoService) *OrderService {
	return &OrderService{repos: repos, publisher: publisher, metrics: metrics, promoSvc: promoSvc}
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
	PromoCode             *string
	PhoneE164             *string
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
		if s.metrics != nil && s.metrics.IdempotencyReplaysTotal != nil {
			s.metrics.IdempotencyReplaysTotal.WithLabelValues("order").Inc()
		}
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

	// Accumulate items total as float; promo discount applied inside TX.
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

	var result PlaceOrderResult
	var appliedPromoID *int64
	var discountAmount float64

	txErr := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		// Validate and apply promo inside transaction for atomicity.
		finalTotal := totalFloat
		if req.PromoCode != nil && s.promoSvc != nil {
			promoResult, err := s.promoSvc.ValidatePromo(ctx, tx, ValidatePromoRequest{
				BranchID:   req.BranchID,
				Code:       *req.PromoCode,
				OrderTotal: finalTotal,
				PhoneE164:  req.PhoneE164,
			})
			if err != nil {
				return err
			}
			discountAmount = promoResult.DiscountAmount
			finalTotal -= discountAmount
			promoIDVal := promoResult.PromoID
			appliedPromoID = &promoIDVal
		}

		var totalAmount pgtype.Numeric
		if err := totalAmount.Scan(fmt.Sprintf("%.2f", finalTotal)); err != nil {
			return fmt.Errorf("encode total amount: %w", err)
		}

		var discountNumeric pgtype.Numeric
		if err := discountNumeric.Scan(fmt.Sprintf("%.2f", discountAmount)); err != nil {
			return fmt.Errorf("encode discount amount: %w", err)
		}

		branch, err := tx.GetBranchByID(ctx, req.BranchID)
		if err != nil {
			return fmt.Errorf("get branch: %w", err)
		}

		loc, err := time.LoadLocation(branch.Timezone)
		if err != nil || loc == nil {
			loc = time.UTC
		}
		localDate := time.Now().In(loc).Format("2006-01-02")

		seq, err := tx.NextOrderNumber(ctx, req.BranchID, localDate)
		if err != nil {
			return fmt.Errorf("next order number: %w", err)
		}
		orderNumber := fmt.Sprintf("%s%d", branch.OrderPrefix, seq)

		order, err := tx.CreateOrder(ctx, repository.CreateOrderParams{
			SessionID:             req.SessionID,
			BranchID:              req.BranchID,
			PlacedByParticipantID: req.PlacedByParticipantID,
			IdempotencyKey:        req.IdempotencyKey,
			TotalAmount:           totalAmount,
			OrderNumber:           orderNumber,
			PromoID:               appliedPromoID,
			DiscountAmount:        discountNumeric,
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

		// Record promo redemption within the same transaction.
		if appliedPromoID != nil {
			if _, err := tx.CreatePromoRedemption(ctx, *appliedPromoID, order.ID, req.PhoneE164); err != nil {
				return fmt.Errorf("create promo redemption: %w", err)
			}
		}

		result = PlaceOrderResult{Order: order, OrderItems: orderItems}
		return nil
	})
	if txErr != nil {
		return PlaceOrderResult{}, txErr
	}

	s.publisher.OrderPlaced(ctx, req.SessionID, result.Order)
	s.repos.LogEvent(ctx, req.SessionID, req.BranchID, "ORDER_PLACED", "participant", req.PlacedByParticipantID, result.Order)

	if appliedPromoID != nil && req.PromoCode != nil {
		s.publisher.PromoApplied(ctx, req.SessionID, map[string]any{
			"order_id":        result.Order.ID,
			"promo_code":      *req.PromoCode,
			"discount_amount": discountAmount,
		})
	}

	// Clear the participant's cart after a successful order — best-effort, never blocks the response.
	if cart, err := s.repos.GetOrCreateCart(ctx, req.SessionID, req.PlacedByParticipantID); err == nil {
		_ = s.repos.ClearCart(ctx, cart.ID)
	}

	return result, nil
}

// UpdateOrderStatus applies a state machine-validated status transition.
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, branchID int64, newStatus domain.OrderStatus, staffID int64) (sqlc.Order, error) {
	order, err := s.repos.GetOrderByID(ctx, orderID)
	if err != nil {
		return sqlc.Order{}, err
	}

	current := domain.OrderStatus(order.Status)
	if err := domain.ValidateOrderTransition(current, newStatus); err != nil {
		return sqlc.Order{}, err
	}

	start := time.Now()
	updated, err := s.repos.UpdateOrderStatusScoped(ctx, orderID, branchID, sqlc.OrderStatus(newStatus))
	if err != nil {
		return sqlc.Order{}, err
	}
	if s.metrics != nil && s.metrics.OrderLifecycleDuration != nil {
		s.metrics.OrderLifecycleDuration.WithLabelValues(string(current), string(newStatus)).Observe(time.Since(start).Seconds())
	}

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
