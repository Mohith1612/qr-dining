package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BillingHandler struct {
	repos *repository.Repos
}

func NewBillingHandler(repos *repository.Repos) *BillingHandler {
	return &BillingHandler{repos: repos}
}

type modifierSnapshot struct {
	Name       string  `json:"name"`
	PriceDelta float64 `json:"price_delta"`
}

// BillItem is a single line item in the bill.
type BillItem struct {
	Name      string   `json:"name"`
	Quantity  int16    `json:"quantity"`
	UnitPrice float64  `json:"unit_price"`
	Modifiers []string `json:"modifiers"`
	LineTotal float64  `json:"line_total"`
}

// BillOrder groups items that belong to a single order.
type BillOrder struct {
	OrderID     string     `json:"order_id"`
	OrderNumber string     `json:"order_number"`
	PlacedAt    time.Time  `json:"placed_at"`
	Items       []BillItem `json:"items"`
	OrderTotal  float64    `json:"order_total"`
}

// BillResponse is the full itemized bill for a session.
type BillResponse struct {
	SessionID         string      `json:"session_id"`
	Orders            []BillOrder `json:"orders"`
	Subtotal          float64     `json:"subtotal"`
	TaxRate           float64     `json:"tax_rate"`
	TaxAmount         float64     `json:"tax_amount"`
	ServiceChargeRate float64     `json:"service_charge_rate"`
	ServiceCharge     float64     `json:"service_charge"`
	DiscountAmount    float64     `json:"discount_amount"`
	TipAmount         float64     `json:"tip_amount"`
	Total             float64     `json:"total"`
	Currency          string      `json:"currency"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// GET /sessions/:id/bill — public, rate-limited.
func (h *BillingHandler) GetBill(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	bill, err := ComputeBillForSession(c.Request.Context(), h.repos, sessionID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, bill)
}

// ComputeBillForSession computes the full itemized bill for a session.
// Reused by the payment handler to persist breakdown at payment initiation.
func ComputeBillForSession(ctx context.Context, repos *repository.Repos, sessionID uuid.UUID) (*BillResponse, error) {
	// Get session to find branch_id.
	sess, err := repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Read billing config from restaurant settings_json.
	var taxRate, serviceChargeRate float64
	var includeTaxInPrice bool
	if restaurant, err := repos.GetRestaurantByBranchID(ctx, sess.BranchID); err == nil {
		var settings map[string]any
		if json.Unmarshal(restaurant.SettingsJson, &settings) == nil {
			if v, ok := settings["tax_rate"].(float64); ok {
				taxRate = v
			}
			if v, ok := settings["service_charge_rate"].(float64); ok {
				serviceChargeRate = v
			}
			if v, ok := settings["include_tax_in_price"].(bool); ok {
				includeTaxInPrice = v
			}
		}
	}

	// Get all non-cancelled orders.
	orders, err := repos.ListOrdersForSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	// Fetch items for all non-cancelled orders in one pass and collect unique menu item IDs.
	type orderWithItems struct {
		order sqlc.Order
		items []sqlc.OrderItem
	}
	activeOrders := make([]orderWithItems, 0, len(orders))
	itemIDSet := make(map[int64]struct{})

	for _, o := range orders {
		if o.Status == "cancelled" {
			continue
		}
		items, err := repos.ListOrderItems(ctx, o.ID)
		if err != nil {
			return nil, fmt.Errorf("list order items for %s: %w", o.ID, err)
		}
		for _, it := range items {
			itemIDSet[it.MenuItemID] = struct{}{}
		}
		activeOrders = append(activeOrders, orderWithItems{order: o, items: items})
	}

	itemIDs := make([]int64, 0, len(itemIDSet))
	for id := range itemIDSet {
		itemIDs = append(itemIDs, id)
	}

	itemNameByID := make(map[int64]string)
	if len(itemIDs) > 0 {
		menuItems, err := repos.GetMenuItemsByIDs(ctx, itemIDs)
		if err == nil {
			for _, mi := range menuItems {
				itemNameByID[mi.ID] = mi.Name
			}
		}
	}

	// Build bill orders.
	billOrders := make([]BillOrder, 0, len(activeOrders))
	var subtotal float64
	var totalDiscount float64

	for _, owi := range activeOrders {
		o := owi.order
		items := owi.items

		billItems := make([]BillItem, 0, len(items))
		var orderTotal float64

		for _, it := range items {
			unitPrice, _ := it.UnitPrice.Float64Value()
			basePrice := unitPrice.Float64

			// Parse modifiers from stored JSON snapshot.
			var mods []modifierSnapshot
			_ = json.Unmarshal(it.SelectedModifiersJson, &mods)

			modNames := make([]string, 0, len(mods))
			var modTotal float64
			for _, m := range mods {
				modNames = append(modNames, fmt.Sprintf("%s +₹%.0f", m.Name, m.PriceDelta))
				modTotal += m.PriceDelta
			}

			lineTotal := round2((basePrice + modTotal) * float64(it.Quantity))
			orderTotal += lineTotal

			name := itemNameByID[it.MenuItemID]
			if name == "" {
				name = fmt.Sprintf("Item #%d", it.MenuItemID)
			}

			billItems = append(billItems, BillItem{
				Name:      name,
				Quantity:  it.Quantity,
				UnitPrice: basePrice,
				Modifiers: modNames,
				LineTotal: lineTotal,
			})
		}

		orderNumber := ""
		if o.OrderNumber.Valid {
			orderNumber = o.OrderNumber.String
		}

		billOrders = append(billOrders, BillOrder{
			OrderID:     o.ID.String(),
			OrderNumber: orderNumber,
			PlacedAt:    o.CreatedAt,
			Items:       billItems,
			OrderTotal:  round2(orderTotal),
		})
		subtotal += orderTotal

		// Accumulate per-order promo discounts.
		if d, err := o.DiscountAmount.Float64Value(); err == nil && d.Valid {
			totalDiscount += d.Float64
		}
	}

	subtotal = round2(subtotal)
	totalDiscount = round2(totalDiscount)

	var taxAmount float64
	if !includeTaxInPrice && taxRate > 0 {
		taxAmount = round2(subtotal * taxRate)
	}
	svcCharge := round2(subtotal * serviceChargeRate)

	total := round2(subtotal + taxAmount + svcCharge - totalDiscount)

	return &BillResponse{
		SessionID:         sessionID.String(),
		Orders:            billOrders,
		Subtotal:          subtotal,
		TaxRate:           taxRate,
		TaxAmount:         taxAmount,
		ServiceChargeRate: serviceChargeRate,
		ServiceCharge:     svcCharge,
		DiscountAmount:    totalDiscount,
		TipAmount:         0,
		Total:             total,
		Currency:          "INR",
	}, nil
}
