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

	// ── Restaurant ────────────────────────────────────────────────────────────
	restaurant, err := pool.QueryRow(ctx,
		`INSERT INTO restaurants (name, slug, settings_json) VALUES ($1, $2, $3)
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id, name, slug, settings_json, created_at`,
		"Demo Restaurant", "demo-restaurant", []byte(`{}`),
	).Values()
	if err != nil {
		fatal("insert restaurant", err)
	}
	restaurantID := restaurant[0].(int64)
	fmt.Printf("restaurant id=%d\n", restaurantID)

	// ── Branch ────────────────────────────────────────────────────────────────
	var branchID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO branches (restaurant_id, name, address, timezone) VALUES ($1, $2, $3, $4)
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		restaurantID, "Main Branch", "123 Main St", "Asia/Kolkata",
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
			BranchID: branchID,
			Name:     staffNames[i],
			Role:     role,
			PinHash:  hash,
		})
		if err != nil {
			fmt.Printf("staff %s may already exist: %v\n", role, err)
			continue
		}
		fmt.Printf("staff id=%d name=%q role=%s (PIN: %s)\n", staff.ID, staff.Name, staff.Role, pin)
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
