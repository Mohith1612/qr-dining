//go:build ignore

// Seed inserts a test restaurant, branch, tables, staff, and menu into the database.
// Run with: go run ./scripts/seed.go
// Requires DATABASE_URL to be set.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := sqlc.New(pool)

	// ── Organization / Restaurant ─────────────────────────────────────────────
	var organizationID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO organizations (code, name, settings_json) VALUES ($1, $2, $3)
		 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
		"demo-restaurant", "Demo Restaurant", []byte(`{}`),
	).Scan(&organizationID)
	if err != nil {
		fatal("insert organization", err)
	}
	fmt.Printf("organization id=%d\n", organizationID)

	var restaurantID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO restaurants (name, slug, settings_json, organization_id) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (slug) DO UPDATE SET
		   name = EXCLUDED.name,
		   organization_id = EXCLUDED.organization_id
		 RETURNING id`,
		"Demo Restaurant", "demo-restaurant", []byte(`{}`), organizationID,
	).Scan(&restaurantID)
	if err != nil {
		fatal("insert restaurant", err)
	}
	fmt.Printf("restaurant id=%d\n", restaurantID)

	// ── Subscription Plans ────────────────────────────────────────────────────
	plans := []struct {
		name     string
		tier     string
		price    string
		features string
	}{
		{
			name:     "Free",
			tier:     "free",
			price:    "0.00",
			features: `{"max_branches":1,"max_tables":10,"analytics":false,"multi_branch":false}`,
		},
		{
			name:     "Standard",
			tier:     "standard",
			price:    "29.00",
			features: `{"max_branches":3,"max_tables":-1,"analytics":true,"multi_branch":false}`,
		},
		{
			name:     "Premium",
			tier:     "premium",
			price:    "79.00",
			features: `{"max_branches":-1,"max_tables":-1,"analytics":true,"multi_branch":true}`,
		},
	}

	var freePlanID int64
	for _, p := range plans {
		var planID int64
		err = pool.QueryRow(ctx,
			`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
			 VALUES ($1, $2::plan_tier, $3::numeric, $4::jsonb)
			 ON CONFLICT (tier) DO UPDATE SET name = EXCLUDED.name, features_json = EXCLUDED.features_json
			 RETURNING id`,
			p.name, p.tier, p.price, p.features,
		).Scan(&planID)
		if err != nil {
			fatal("insert plan "+p.tier, err)
		}
		fmt.Printf("plan id=%d tier=%s price=%s\n", planID, p.tier, p.price)
		if p.tier == "free" {
			freePlanID = planID
		}
	}

	// Link demo-restaurant to the free plan (trial).
	if freePlanID > 0 {
		_, err = pool.Exec(ctx,
			`INSERT INTO restaurant_subscriptions (restaurant_id, plan_id, status, trial_ends_at)
			 VALUES ($1, $2, 'trial', NOW() + INTERVAL '30 days')
			 ON CONFLICT (restaurant_id) DO NOTHING`,
			restaurantID, freePlanID,
		)
		if err != nil {
			fmt.Printf("subscription link: %v\n", err)
		} else {
			fmt.Printf("subscription: restaurant %d on free plan (trial, 30 days)\n", restaurantID)
		}
	}

	// ── Branch ────────────────────────────────────────────────────────────────
	var branchID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO branches (restaurant_id, organization_id, name, address, timezone, branch_code) VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		restaurantID, organizationID, "Main Branch", "123 Main St", "Asia/Kolkata", "DEMO-MAIN",
	).Scan(&branchID)
	if err != nil {
		// Try selecting existing.
		err = pool.QueryRow(ctx,
			`SELECT id FROM branches WHERE restaurant_id = $1 LIMIT 1`, restaurantID,
		).Scan(&branchID)
		if err != nil {
			fatal("get branch", err)
		}
	}
	fmt.Printf("branch id=%d\n", branchID)

	// ── Tables ────────────────────────────────────────────────────────────────
	tableTokens := make([]string, 3)
	for i := 1; i <= 3; i++ {
		token := mustToken()
		var tableID int64
		err = pool.QueryRow(ctx,
			`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
			 VALUES ($1, $2, 4, $3, 'available')
			 ON CONFLICT (branch_id, identifier) DO UPDATE SET qr_code_token = $3
			 RETURNING id`,
			branchID, fmt.Sprintf("T%d", i), token,
		).Scan(&tableID)
		if err != nil {
			fatal(fmt.Sprintf("insert table %d", i), err)
		}
		tableTokens[i-1] = token
		fmt.Printf("table %d id=%d qr_token=%s\n", i, tableID, token)
	}

	// ── Staff ─────────────────────────────────────────────────────────────────
	staffRoles := []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen}
	staffNames := []string{"Owner Sam", "Waiter Alex", "Chef Jordan"}
	pin := "1234"
	hash, err := services.HashPIN(pin)
	if err != nil {
		fatal("hash pin", err)
	}

	for i, role := range staffRoles {
		staff, err := q.CreateStaff(ctx, sqlc.CreateStaffParams{
			BranchID:  branchID,
			Name:      staffNames[i],
			Role:      role,
			PinHash:   hash,
			StaffCode: fmt.Sprintf("STAFF%02d", i+1),
		})
		if err != nil {
			fmt.Printf("staff %s may already exist: %v\n", role, err)
			continue
		}
		fmt.Printf("staff id=%d name=%q role=%s (PIN: %s)\n", staff.ID, staff.Name, staff.Role, pin)
		if role == sqlc.StaffRoleOwner {
			_, err = pool.Exec(ctx,
				`INSERT INTO organization_members (organization_id, staff_id, role, status)
				 VALUES ($1, $2, 'owner', 'active')
				 ON CONFLICT (organization_id, staff_id) DO UPDATE SET role = EXCLUDED.role, status = EXCLUDED.status`,
				organizationID, staff.ID,
			)
			if err != nil {
				fmt.Printf("organization membership for owner may already exist: %v\n", err)
			}
		}
	}

	// ── Menu ──────────────────────────────────────────────────────────────────
	categories := []struct {
		name  string
		items []struct {
			name  string
			price string
			mods  []struct {
				name  string
				delta string
				req   bool
			}
		}
	}{
		{
			name: "Starters",
			items: []struct {
				name  string
				price string
				mods  []struct {
					name  string
					delta string
					req   bool
				}
			}{
				{name: "Paneer Tikka", price: "180", mods: []struct {
					name  string
					delta string
					req   bool
				}{{name: "Extra Sauce", delta: "20"}}},
				{name: "Veg Spring Rolls", price: "140", mods: nil},
				{name: "Soup of the Day", price: "90", mods: nil},
			},
		},
		{
			name: "Mains",
			items: []struct {
				name  string
				price string
				mods  []struct {
					name  string
					delta string
					req   bool
				}
			}{
				{name: "Dal Makhani", price: "220", mods: nil},
				{name: "Butter Chicken", price: "280", mods: []struct {
					name  string
					delta string
					req   bool
				}{{name: "Extra Butter", delta: "30"}}},
				{name: "Veg Biryani", price: "200", mods: nil},
			},
		},
		{
			name: "Breads",
			items: []struct {
				name  string
				price string
				mods  []struct {
					name  string
					delta string
					req   bool
				}
			}{
				{name: "Butter Naan", price: "45"},
				{name: "Garlic Roti", price: "35"},
				{name: "Paratha", price: "50"},
			},
		},
		{
			name: "Beverages",
			items: []struct {
				name  string
				price string
				mods  []struct {
					name  string
					delta string
					req   bool
				}
			}{
				{name: "Lassi", price: "80", mods: []struct {
					name  string
					delta string
					req   bool
				}{{name: "Sweet", delta: "0", req: true}, {name: "Salted", delta: "0"}}},
				{name: "Masala Chai", price: "30"},
				{name: "Mango Shake", price: "90"},
			},
		},
		{
			name: "Desserts",
			items: []struct {
				name  string
				price string
				mods  []struct {
					name  string
					delta string
					req   bool
				}
			}{
				{name: "Gulab Jamun", price: "60"},
				{name: "Ice Cream", price: "80", mods: []struct {
					name  string
					delta string
					req   bool
				}{{name: "Chocolate", delta: "10"}, {name: "Vanilla", delta: "0"}}},
				{name: "Kheer", price: "70"},
			},
		},
	}

	for pos, cat := range categories {
		var catID int64
		err = pool.QueryRow(ctx,
			`INSERT INTO menu_categories (branch_id, name, position) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING RETURNING id`,
			branchID, cat.name, pos,
		).Scan(&catID)
		if err != nil {
			err = pool.QueryRow(ctx,
				`SELECT id FROM menu_categories WHERE branch_id = $1 AND name = $2`,
				branchID, cat.name,
			).Scan(&catID)
			if err != nil {
				fmt.Printf("category %s: %v\n", cat.name, err)
				continue
			}
		}

		for ipos, item := range cat.items {
			var price pgtype.Numeric
			_ = price.Scan(item.price)

			var itemID int64
			err = pool.QueryRow(ctx,
				`INSERT INTO menu_items (category_id, branch_id, name, price, position)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT DO NOTHING RETURNING id`,
				catID, branchID, item.name, price, ipos,
			).Scan(&itemID)
			if err != nil {
				err = pool.QueryRow(ctx,
					`SELECT id FROM menu_items WHERE branch_id = $1 AND name = $2`,
					branchID, item.name,
				).Scan(&itemID)
				if err != nil {
					fmt.Printf("item %s: %v\n", item.name, err)
					continue
				}
			}

			for _, mod := range item.mods {
				var delta pgtype.Numeric
				_ = delta.Scan(mod.delta)
				_, _ = pool.Exec(ctx,
					`INSERT INTO item_modifiers (item_id, name, price_delta, is_required)
					 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
					itemID, mod.name, delta, mod.req,
				)
			}
		}
	}

	// ── Active Sessions, Orders, Assistance ──────────────────────────────────
	// Retrieve table IDs for tables 1 and 2 to open sessions on them.
	var tableIDs [2]int64
	for i := 0; i < 2; i++ {
		err = pool.QueryRow(ctx,
			`SELECT id FROM tables WHERE branch_id = $1 AND identifier = $2`,
			branchID, fmt.Sprintf("T%d", i+1),
		).Scan(&tableIDs[i])
		if err != nil {
			fatal(fmt.Sprintf("get table T%d", i+1), err)
		}
	}

	// Retrieve the first two menu_item IDs to seed orders.
	var seedItemIDs [2]int64
	rows, err := pool.Query(ctx,
		`SELECT id FROM menu_items WHERE branch_id = $1 ORDER BY id LIMIT 2`, branchID,
	)
	if err != nil {
		fatal("list menu items for seed orders", err)
	}
	i := 0
	for rows.Next() && i < 2 {
		_ = rows.Scan(&seedItemIDs[i])
		i++
	}
	rows.Close()

	displayNames := []string{"Riya's Table", "Dev's Table"}

	for idx := 0; idx < 2; idx++ {
		tableID := tableIDs[idx]

		// Skip if table already has an active session.
		var existingSessionID string
		checkErr := pool.QueryRow(ctx,
			`SELECT id FROM sessions WHERE table_id = $1 AND status = 'active' LIMIT 1`, tableID,
		).Scan(&existingSessionID)
		if checkErr == nil {
			fmt.Printf("table T%d already has active session %s — skipping\n", idx+1, existingSessionID)
			continue
		}

		sess, err := q.CreateSession(ctx, sqlc.CreateSessionParams{
			BranchID:     branchID,
			TableID:      tableID,
			SessionToken: mustToken(),
		})
		if err != nil {
			fatal(fmt.Sprintf("create session %d", idx+1), err)
		}

		// Mark table occupied.
		_, err = pool.Exec(ctx,
			`UPDATE tables SET status = 'occupied' WHERE id = $1`, tableID,
		)
		if err != nil {
			fatal("mark table occupied", err)
		}

		// Create host participant.
		participant, err := q.CreateParticipant(ctx, sqlc.CreateParticipantParams{
			SessionID:   sess.ID,
			DisplayName: displayNames[idx],
			IsHost:      true,
		})
		if err != nil {
			fatal(fmt.Sprintf("create participant %d", idx+1), err)
		}

		// Set host_participant_id (DEFERRABLE FK).
		_, err = pool.Exec(ctx,
			`UPDATE sessions SET host_participant_id = $1 WHERE id = $2`,
			participant.ID, sess.ID,
		)
		if err != nil {
			fatal("set session host", err)
		}

		fmt.Printf("session id=%s table=T%d participant=%q\n", sess.ID, idx+1, displayNames[idx])

		// Seed one confirmed order.
		if seedItemIDs[0] != 0 {
			var itemPrice pgtype.Numeric
			_ = pool.QueryRow(ctx,
				`SELECT price FROM menu_items WHERE id = $1`, seedItemIDs[0],
			).Scan(&itemPrice)

			var total pgtype.Numeric
			_ = total.Scan("220.00") // fixed total for seed simplicity

			ikey := fmt.Sprintf("seed-order-%s", sess.ID)
			order, err := q.CreateOrder(ctx, sqlc.CreateOrderParams{
				SessionID:             sess.ID,
				BranchID:              branchID,
				PlacedByParticipantID: pgtype.Int8{Int64: participant.ID, Valid: true},
				IdempotencyKey:        ikey,
				TotalAmount:           total,
			})
			if err != nil {
				fmt.Printf("create order for session %s: %v\n", sess.ID, err)
			} else {
				// Update order to confirmed status.
				_, err = pool.Exec(ctx,
					`UPDATE orders SET status = 'confirmed' WHERE id = $1`, order.ID,
				)
				if err != nil {
					fmt.Printf("confirm order %s: %v\n", order.ID, err)
				}
				// Create one order item.
				_, _ = q.CreateOrderItem(ctx, sqlc.CreateOrderItemParams{
					OrderID:               order.ID,
					MenuItemID:            seedItemIDs[0],
					Quantity:              2,
					UnitPrice:             itemPrice,
					SelectedModifiersJson: []byte("[]"),
				})
				fmt.Printf("  order id=%s status=confirmed\n", order.ID)
			}
		}

		// Seed one pending assistance request.
		ar, err := q.CreateAssistanceRequest(ctx, sqlc.CreateAssistanceRequestParams{
			SessionID:     sess.ID,
			TableID:       tableID,
			ParticipantID: pgtype.Int8{Int64: participant.ID, Valid: true},
			Type:          sqlc.AssistanceTypeWaiter,
		})
		if err != nil {
			fmt.Printf("create assistance for session %s: %v\n", sess.ID, err)
		} else {
			fmt.Printf("  assistance id=%d type=waiter status=pending\n", ar.ID)
		}
	}

	fmt.Println("\n── Seed complete ──")
	fmt.Printf("Branch ID:    %d\n", branchID)
	fmt.Println("Table QR tokens:")
	for i, t := range tableTokens {
		fmt.Printf("  T%d: %s\n", i+1, t)
	}
	fmt.Printf("Staff PIN:    %s\n", pin)
}

func mustToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}
