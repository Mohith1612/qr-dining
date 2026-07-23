package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
	hostAuth  *SessionService
}

// NewOrderService — promoSvc is accepted for call-site compatibility but no
// longer used: promos moved to payment initiation.
func NewOrderService(repos *repository.Repos, publisher *events.Publisher, metrics *observability.Metrics, _ *PromoService) *OrderService {
	return &OrderService{repos: repos, publisher: publisher, metrics: metrics}
}

// SetHostAuthority injects the session service used to enforce host-only order
// submission (and on-demand host reassignment). Wired after construction to
// avoid a constructor cycle between the order and session services.
func (s *OrderService) SetHostAuthority(h *SessionService) { s.hostAuth = h }

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
	// Promo is no longer applied at order time — it is applied at payment
	// initiation (the discount lands in the immutable bill snapshot).
}

type PlaceOrderResult struct {
	Order      sqlc.Order
	OrderItems []sqlc.OrderItem
}

// PlaceOrder creates a new order with idempotency protection.
// If the idempotency key already exists, the existing order is returned without error.
func (s *OrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResult, error) {
	// Validate session is still active.
	sess, err := s.repos.GetSessionByID(ctx, req.SessionID)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	if domain.IsSessionCartFrozen(domain.SessionStatus(sess.Status)) {
		return PlaceOrderResult{}, domain.ErrPaymentInProgress
	}
	if sess.Status != sqlc.SessionStatusActive {
		return PlaceOrderResult{}, domain.ErrSessionClosed
	}
	if req.BranchID != 0 && req.BranchID != sess.BranchID {
		return PlaceOrderResult{}, domain.ErrTenantMismatch
	}
	req.BranchID = sess.BranchID

	participant, err := s.repos.GetParticipantByID(ctx, req.PlacedByParticipantID)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	if participant.SessionID != req.SessionID {
		return PlaceOrderResult{}, domain.ErrParticipantNotInSession
	}

	// Host-controlled ordering: only the session host may submit the shared cart
	// to the kitchen. Other participants collaborate on the cart but cannot
	// finalize. Enforced server-side so a stale client cannot bypass it.
	if s.hostAuth != nil {
		authorized, err := s.hostAuth.AuthorizeHostAction(ctx, req.SessionID, req.PlacedByParticipantID)
		if err != nil {
			return PlaceOrderResult{}, err
		}
		if !authorized {
			return PlaceOrderResult{}, domain.ErrNotSessionHost
		}
	} else if !sess.HostParticipantID.Valid || sess.HostParticipantID.Int64 != req.PlacedByParticipantID {
		return PlaceOrderResult{}, domain.ErrNotSessionHost
	}

	idemScope := repository.IdempotencyScope{
		ScopeType: "session",
		ScopeID:   req.SessionID.String(),
		ActorType: "participant",
		ActorID:   strconv.FormatInt(req.PlacedByParticipantID, 10),
		Key:       req.IdempotencyKey,
	}
	requestHash := hashOrderRequest(req)
	_, inserted, err := s.repos.CreateIdempotencyKey(ctx, idemScope, requestHash, time.Now().Add(24*time.Hour))
	if err != nil {
		return PlaceOrderResult{}, err
	}
	if !inserted {
		existingKey, err := s.repos.GetIdempotencyKey(ctx, idemScope)
		if err != nil {
			return PlaceOrderResult{}, err
		}
		if existingKey.RequestHash != requestHash {
			return PlaceOrderResult{}, domain.ErrIdempotencyConflict
		}
		if existingKey.Status != "completed" || !existingKey.ResponseResourceID.Valid {
			return PlaceOrderResult{}, domain.ErrIdempotencyInProgress
		}
		orderID, err := uuid.Parse(existingKey.ResponseResourceID.String)
		if err != nil {
			return PlaceOrderResult{}, fmt.Errorf("parse idempotent order id: %w", err)
		}
		existing, err := s.repos.GetOrderByID(ctx, orderID)
		if err != nil {
			return PlaceOrderResult{}, err
		}
		if s.metrics != nil && s.metrics.IdempotencyReplaysTotal != nil {
			s.metrics.IdempotencyReplaysTotal.WithLabelValues("order").Inc()
		}
		items, err := s.repos.ListOrderItems(ctx, existing.ID)
		if err != nil {
			return PlaceOrderResult{}, fmt.Errorf("fetch order items on replay: %w", err)
		}
		return PlaceOrderResult{Order: existing, OrderItems: items}, nil
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
		if mi.BranchID != req.BranchID {
			return PlaceOrderResult{}, domain.ErrMenuItemNotFound
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

	txErr := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		// Promo discounts are applied at payment initiation, not order time —
		// the order total is the undiscounted items total.
		var totalAmount pgtype.Numeric
		if err := totalAmount.Scan(fmt.Sprintf("%.2f", totalFloat)); err != nil {
			return fmt.Errorf("encode total amount: %w", err)
		}

		var discountNumeric pgtype.Numeric
		if err := discountNumeric.Scan("0.00"); err != nil {
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
		businessDate := time.Now().In(loc)
		localDate := businessDate.Format("2006-01-02")

		seq, err := tx.NextOrderNumber(ctx, req.BranchID, localDate)
		if err != nil {
			return fmt.Errorf("next order number: %w", err)
		}
		orderNumber := fmt.Sprintf("%s%d", branch.OrderPrefix, seq)
		operationalID := fmt.Sprintf("%s-%s-%s", branch.BranchCode, businessDate.Format("20060102"), orderNumber)

		order, err := tx.CreateOrder(ctx, repository.CreateOrderParams{
			SessionID:             req.SessionID,
			BranchID:              req.BranchID,
			PlacedByParticipantID: req.PlacedByParticipantID,
			IdempotencyKey:        req.IdempotencyKey,
			TotalAmount:           totalAmount,
			OrderNumber:           orderNumber,
			OrderBusinessDate:     businessDate,
			OrderNumberDisplay:    orderNumber,
			OrderOperationalID:    operationalID,
			PromoID:               nil,
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

		result = PlaceOrderResult{Order: order, OrderItems: orderItems}
		if err := tx.CompleteIdempotencyKey(ctx, idemScope, "order", order.ID.String()); err != nil {
			return fmt.Errorf("complete idempotency key: %w", err)
		}
		return nil
	})
	if txErr != nil {
		_ = s.repos.FailIdempotencyKey(ctx, idemScope)
		return PlaceOrderResult{}, txErr
	}

	s.publisher.OrderPlaced(ctx, req.SessionID, result.Order)
	s.repos.LogEvent(ctx, req.SessionID, req.BranchID, "ORDER_PLACED", "participant", req.PlacedByParticipantID, result.Order)

	// Clear the shared session cart after a successful order — best-effort, never blocks the response.
	if cart, err := s.repos.GetOrCreateSessionCart(ctx, req.SessionID); err == nil {
		_ = s.repos.ClearCart(ctx, cart.ID)
		s.publisher.CartUpdated(ctx, req.SessionID, map[string]any{
			"action":     "clear",
			"session_id": req.SessionID,
		})
	}

	return result, nil
}

func hashOrderRequest(req PlaceOrderRequest) string {
	type hashItem struct {
		MenuItemID  int64   `json:"menu_item_id"`
		Quantity    int16   `json:"quantity"`
		ModifierIDs []int64 `json:"modifier_ids"`
		Note        string  `json:"note"`
	}
	items := make([]hashItem, 0, len(req.Items))
	for _, item := range req.Items {
		modIDs := append([]int64(nil), item.ModifierIDs...)
		sort.Slice(modIDs, func(i, j int) bool { return modIDs[i] < modIDs[j] })
		items = append(items, hashItem{
			MenuItemID:  item.MenuItemID,
			Quantity:    item.Quantity,
			ModifierIDs: modIDs,
			Note:        item.Note,
		})
	}
	payload := struct {
		SessionID     string     `json:"session_id"`
		ParticipantID int64      `json:"participant_id"`
		Items         []hashItem `json:"items"`
	}{
		SessionID:     req.SessionID.String(),
		ParticipantID: req.PlacedByParticipantID,
		Items:         items,
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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
	updated, err := s.repos.UpdateOrderStatusExpected(ctx, orderID, branchID, order.Status, sqlc.OrderStatus(newStatus))
	if err != nil {
		return sqlc.Order{}, err
	}
	if s.metrics != nil && s.metrics.OrderLifecycleDuration != nil {
		s.metrics.OrderLifecycleDuration.WithLabelValues(string(current), string(newStatus)).Observe(time.Since(start).Seconds())
	}

	s.publishOrderStatusEvent(ctx, order.SessionID, newStatus, updated)
	s.repos.LogEvent(ctx, order.SessionID, order.BranchID, "ORDER_STATUS_CHANGED", "staff", staffID,
		map[string]any{"order_id": orderID, "old_status": current, "new_status": newStatus})
	return updated, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID) (sqlc.Order, error) {
	return s.repos.GetOrderByID(ctx, orderID)
}

func (s *OrderService) ListOrdersForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Order, error) {
	return s.repos.ListOrdersForSession(ctx, sessionID)
}

// KitchenOrderItem is one line on a kitchen ticket: what to make, how many,
// the modifier/customization choices, and any free-text note.
type KitchenOrderItem struct {
	MenuItemID int64              `json:"menu_item_id"`
	Name       string             `json:"name"`
	Quantity   int16              `json:"quantity"`
	Modifiers  []ModifierSnapshot `json:"modifiers"`
	Note       string             `json:"note"`
}

// KitchenOrder embeds the existing active-order row (preserving every field the
// kitchen board already binds to) and adds the operational item detail the
// kitchen needs to actually prepare the order.
type KitchenOrder struct {
	sqlc.ListActiveOrdersForBranchRow
	Items []KitchenOrderItem `json:"items"`
}

func (s *OrderService) ListActiveForBranch(ctx context.Context, branchID int64) ([]KitchenOrder, error) {
	orders, err := s.repos.ListActiveOrdersForBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	itemRows, err := s.repos.ListActiveOrderItemsForBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}

	itemsByOrder := make(map[uuid.UUID][]KitchenOrderItem, len(orders))
	for _, row := range itemRows {
		mods := []ModifierSnapshot{}
		if len(row.SelectedModifiersJson) > 0 {
			// Best-effort: a malformed snapshot must not blank the whole ticket.
			_ = json.Unmarshal(row.SelectedModifiersJson, &mods)
		}
		itemsByOrder[row.OrderID] = append(itemsByOrder[row.OrderID], KitchenOrderItem{
			MenuItemID: row.MenuItemID,
			Name:       row.MenuItemName,
			Quantity:   row.Quantity,
			Modifiers:  mods,
			Note:       row.Note,
		})
	}

	result := make([]KitchenOrder, 0, len(orders))
	for _, o := range orders {
		items := itemsByOrder[o.ID]
		if items == nil {
			items = []KitchenOrderItem{}
		}
		result = append(result, KitchenOrder{ListActiveOrdersForBranchRow: o, Items: items})
	}
	return result, nil
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
