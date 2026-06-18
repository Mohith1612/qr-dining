package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestFixtures holds IDs of seeded test data.
type TestFixtures struct {
	OrganizationID int64
	RestaurantID   int64
	RestaurantSlug string
	BranchID       int64
	TableID        int64
	MenuItemID     int64
	ModifierID     int64
}

// SeedFixtures inserts minimal test data (restaurant → branch → table → menu item)
// and returns the resulting IDs. Uses ON CONFLICT to be idempotent within a test run.
func SeedFixtures(t testing.TB, pool *pgxpool.Pool) TestFixtures {
	t.Helper()
	ctx := context.Background()

	var f TestFixtures
	f.RestaurantSlug = "test-restaurant-" + randomHex(4)

	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (code, name, settings_json) VALUES ($1, 'Test Restaurant', '{}')
		 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		f.RestaurantSlug,
	).Scan(&f.OrganizationID)
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO restaurants (name, slug, settings_json, organization_id) VALUES ('Test Restaurant', $1, '{}', $2)
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, organization_id = EXCLUDED.organization_id RETURNING id`,
		f.RestaurantSlug, f.OrganizationID,
	).Scan(&f.RestaurantID)
	if err != nil {
		t.Fatalf("seed restaurant: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO branches (restaurant_id, organization_id, name, address, timezone, branch_code) VALUES ($1, $2, 'Test Branch', '1 Test St', 'UTC', $3) RETURNING id`,
		f.RestaurantID, f.OrganizationID, "TEST-"+randomHex(4),
	).Scan(&f.BranchID)
	if err != nil {
		t.Fatalf("seed branch: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
		 VALUES ($1, 'T1', 4, $2, 'available') RETURNING id`,
		f.BranchID, randomHex(16),
	).Scan(&f.TableID)
	if err != nil {
		t.Fatalf("seed table: %v", err)
	}

	var categoryID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO menu_categories (branch_id, name, position) VALUES ($1, 'Mains', 1) RETURNING id`,
		f.BranchID,
	).Scan(&categoryID)
	if err != nil {
		t.Fatalf("seed menu category: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO menu_items (category_id, branch_id, name, price, is_available) VALUES ($1, $2, 'Test Item', 10.00, true) RETURNING id`,
		categoryID, f.BranchID,
	).Scan(&f.MenuItemID)
	if err != nil {
		t.Fatalf("seed menu item: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO item_modifiers (item_id, name, price_delta) VALUES ($1, 'Extra', 1.50) RETURNING id`,
		f.MenuItemID,
	).Scan(&f.ModifierID)
	if err != nil {
		t.Fatalf("seed item modifier: %v", err)
	}

	return f
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
