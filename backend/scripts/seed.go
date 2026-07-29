//go:build ignore

// Seed builds a realistic MULTI-TENANT manual-testing dataset:
//   - 4 platform users (super_admin / support_admin / billing_admin / read_only_auditor)
//   - 3 subscription plans (free / standard / premium) + entitlements
//   - 3 tenants: Saffron House (premium, 3 branches), Copper Pot Kitchen (standard),
//     Urban Brew Café (free/trial) — each with org, restaurant, subscription, theme,
//     billing profile, branches, tables (+QR), menus, staff (owner/manager/waiter/kitchen),
//     and per-branch collateral.
//   - Live data on each tenant's primary branch (active sessions, orders, assistance,
//     a payment awaiting staff confirmation) so dashboards are populated on first open.
//
// This is THROWAWAY TEST TOOLING — no business logic. It writes only to the database
// pointed at by DATABASE_URL. Run with:  DATABASE_URL=... go run ./scripts/seed.go
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// ─────────────────────────── data model for the seed ────────────────────────

type modSpec struct {
	name   string
	delta  string
	req    bool
	group  string // modifier_group; empty = plain add-on
	single bool   // single_select; group renders as a radio "pick one"
}
type itemSpec struct {
	name  string
	price string
	mods  []modSpec
}

// stockImageURLs gives every seeded dish a representative stock photo so image
// layouts are visible in manual testing. Keyed by item name; unmatched items
// simply render the vignette placeholder. All URLs verified to resolve.
var stockImageURLs = map[string]string{
	"Paneer Tikka":         "https://images.unsplash.com/photo-1567188040759-fb8a883dc6d8?w=400&q=60",
	"Veg Spring Rolls":     "https://images.unsplash.com/photo-1625220194771-7ebdea0b70b9?w=400&q=60",
	"Chicken 65":           "https://images.unsplash.com/photo-1599487488170-d11ec9c172f0?w=400&q=60",
	"Dal Makhani":          "https://images.unsplash.com/photo-1546069901-ba9599a7e63c?w=400&q=60",
	"Butter Chicken":       "https://images.unsplash.com/photo-1603894584373-5ac82b2ae398?w=400&q=60",
	"Veg Biryani":          "https://images.unsplash.com/photo-1589302168068-964664d93dc0?w=400&q=60",
	"Butter Naan":          "https://images.unsplash.com/photo-1601050690597-df0568f70950?w=400&q=60",
	"Garlic Roti":          "https://images.unsplash.com/photo-1565557623262-b51c2513a641?w=400&q=60",
	"Laccha Paratha":       "https://images.unsplash.com/photo-1626074353765-517a681e40be?w=400&q=60",
	"Sweet Lassi":          "https://images.unsplash.com/photo-1571091718767-18b5b1457add?w=400&q=60",
	"Masala Chai":          "https://images.unsplash.com/photo-1596797038530-2c107229654b?w=400&q=60",
	"Mango Shake":          "https://images.unsplash.com/photo-1551024506-0bccd828d307?w=400&q=60",
	"Gulab Jamun":          "https://images.unsplash.com/photo-1631452180519-c014fe946bc7?w=400&q=60",
	"Ice Cream":            "https://images.unsplash.com/photo-1563805042-7684c019e1cb?w=400&q=60",
	"Kheer":                "https://images.unsplash.com/photo-1541696432-82c6da8ce7bf?w=400&q=60",
	"Espresso":             "https://images.unsplash.com/photo-1517244683847-7456b63c5969?w=400&q=60",
	"Cappuccino":           "https://images.unsplash.com/photo-1585937421612-70a008356fbe?w=400&q=60",
	"Cold Brew":            "https://images.unsplash.com/photo-1461023058943-07fcbe16d735?w=400&q=60",
	"Green Tea":            "https://images.unsplash.com/photo-1512621776951-a57141f2eefd?w=400&q=60",
	"Grilled Veg Sandwich": "https://images.unsplash.com/photo-1504674900247-0877df9cc836?w=400&q=60",
	"Chicken Club":         "https://images.unsplash.com/photo-1540189549336-e6e99c3679fe?w=400&q=60",
	"Butter Croissant":     "https://images.unsplash.com/photo-1555507036-ab1f4038808a?w=400&q=60",
	"Chocolate Muffin":     "https://images.unsplash.com/photo-1607958996333-41aef7caefaa?w=400&q=60",
	"Iced Latte":           "https://images.unsplash.com/photo-1551782450-a2132b4ba21d?w=400&q=60",
	"Lemonade":             "https://images.unsplash.com/photo-1523677011781-c91d1bbe2f9e?w=400&q=60",
}
type catSpec struct {
	name  string
	items []itemSpec
}

type branchSpec struct {
	name        string
	code        string
	address     string
	timezone    string
	orderPrefix string
}

type tenantSpec struct {
	logical          string // Alpha / Beta / Gamma
	orgCode          string
	orgName          string
	restSlug         string
	restName         string
	planTier         string // free / standard / premium
	subStatus        string // trial / active
	themePreset      string
	themeTokens      string // JSON object of design tokens
	collateralFormat string
	gstNumber        string
	billingEmail     string
	wifiSSID         string
	wifiPass         string
	welcomeText      string
	footerText       string
	branches         []branchSpec
	menu             []catSpec
}

// ─────────────────────────── result accounting ──────────────────────────────

type seededTable struct {
	identifier string
	token      string
}
type seededBranch struct {
	id          int64
	name        string
	code        string
	orderPrefix string
	tables      []seededTable
}
type seededTenant struct {
	spec     tenantSpec
	orgID    int64
	restID   int64
	branches []seededBranch
}

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
		fatal("connect", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)

	platformUsers := seedPlatformUsers(ctx, q)
	planIDByTier, freePlanID := seedPlans(ctx, pool)

	tenants := tenantCatalog()
	results := make([]seededTenant, 0, len(tenants))
	for _, t := range tenants {
		results = append(results, seedTenant(ctx, pool, q, planIDByTier, freePlanID, t))
	}

	printSummary(platformUsers, results)
}

// ─────────────────────────── platform users ─────────────────────────────────

type seededPlatformUser struct {
	email string
	pass  string
	role  string
}

func seedPlatformUsers(ctx context.Context, q *sqlc.Queries) []seededPlatformUser {
	users := []seededPlatformUser{
		{"admin@platform.local", "Platform!admin1", services.PlatformRoleSuperAdmin},
		{"support@platform.local", "Support!admin1", services.PlatformRoleSupportAdmin},
		{"billing@platform.local", "Billing!admin1", services.PlatformRoleBillingAdmin},
		{"auditor@platform.local", "Auditor!admin1", services.PlatformRoleReadOnlyAuditor},
	}
	for _, u := range users {
		hash, err := services.HashPlatformPassword(u.pass)
		if err != nil {
			fatal("hash platform password "+u.email, err)
		}
		admin, err := q.UpsertPlatformUser(ctx, sqlc.UpsertPlatformUserParams{
			Email:        services.NormalizePlatformEmail(u.email),
			DisplayName:  displayNameForRole(u.role),
			PasswordHash: hash,
			Status:       "active",
			MfaRequired:  false,
		})
		if err != nil {
			fatal("upsert platform user "+u.email, err)
		}
		if err := q.AddPlatformUserRole(ctx, sqlc.AddPlatformUserRoleParams{
			PlatformUserID: admin.ID,
			Role:           u.role,
		}); err != nil {
			fmt.Printf("add role %s/%s: %v\n", u.email, u.role, err)
		}
		fmt.Printf("platform user id=%d email=%s role=%s\n", admin.ID, admin.Email, u.role)
	}
	return users
}

func displayNameForRole(role string) string {
	switch role {
	case services.PlatformRoleSuperAdmin:
		return "Platform Super Admin"
	case services.PlatformRoleSupportAdmin:
		return "Support Operator"
	case services.PlatformRoleBillingAdmin:
		return "Billing Operator"
	case services.PlatformRoleReadOnlyAuditor:
		return "Read-Only Auditor"
	}
	return "Platform User"
}

// ─────────────────────────── plans + entitlements ───────────────────────────

func seedPlans(ctx context.Context, pool *pgxpool.Pool) (map[string]int64, int64) {
	plans := []struct{ name, tier, price, features string }{
		{"Free", "free", "0.00", `{"max_branches":1,"max_tables":10,"analytics":false,"multi_branch":false}`},
		{"Standard", "standard", "29.00", `{"max_branches":3,"max_tables":-1,"analytics":true,"multi_branch":false}`},
		{"Premium", "premium", "79.00", `{"max_branches":-1,"max_tables":-1,"analytics":true,"multi_branch":true}`},
	}
	planIDByTier := map[string]int64{}
	var freePlanID int64
	for _, p := range plans {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
			 VALUES ($1, $2::plan_tier, $3::numeric, $4::jsonb)
			 ON CONFLICT (tier) DO UPDATE SET name = EXCLUDED.name, features_json = EXCLUDED.features_json
			 RETURNING id`,
			p.name, p.tier, p.price, p.features,
		).Scan(&id)
		if err != nil {
			fatal("insert plan "+p.tier, err)
		}
		planIDByTier[p.tier] = id
		if p.tier == "free" {
			freePlanID = id
		}
		fmt.Printf("plan id=%d tier=%s price=%s\n", id, p.tier, p.price)
	}

	type planEnt struct {
		key   string
		limit any
	}
	lim := func(v int64) any { return v }
	planEnts := map[string][]planEnt{
		"free":     {{"limit.branches", lim(1)}, {"limit.tables", lim(10)}, {"limit.staff", nil}},
		"standard": {{"analytics.basic", nil}, {"limit.branches", lim(3)}, {"limit.tables", nil}, {"limit.staff", nil}},
		"premium":  {{"analytics.basic", nil}, {"analytics.advanced", nil}, {"multi_branch", nil}, {"limit.branches", nil}, {"limit.tables", nil}, {"limit.staff", nil}},
	}
	for tier, ents := range planEnts {
		planID := planIDByTier[tier]
		if planID == 0 {
			continue
		}
		for _, e := range ents {
			if _, err := pool.Exec(ctx,
				`INSERT INTO plan_entitlements (plan_id, entitlement_key, enabled, limit_value)
				 VALUES ($1, $2, TRUE, $3)
				 ON CONFLICT (plan_id, entitlement_key)
				 DO UPDATE SET enabled = EXCLUDED.enabled, limit_value = EXCLUDED.limit_value, updated_at = NOW()`,
				planID, e.key, e.limit,
			); err != nil {
				fmt.Printf("plan entitlement %s/%s: %v\n", tier, e.key, err)
			}
		}
	}
	return planIDByTier, freePlanID
}

// ─────────────────────────── per-tenant seeding ─────────────────────────────

func seedTenant(ctx context.Context, pool *pgxpool.Pool, q *sqlc.Queries, planIDByTier map[string]int64, freePlanID int64, spec tenantSpec) seededTenant {
	fmt.Printf("\n── Tenant %s: %s ──\n", spec.logical, spec.restName)
	res := seededTenant{spec: spec}

	// Organization + restaurant.
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (code, name, settings_json) VALUES ($1, $2, '{}'::jsonb)
		 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		spec.orgCode, spec.orgName,
	).Scan(&res.orgID); err != nil {
		fatal("org "+spec.orgCode, err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO restaurants (name, slug, settings_json, organization_id) VALUES ($1, $2, '{}'::jsonb, $3)
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, organization_id = EXCLUDED.organization_id RETURNING id`,
		spec.restName, spec.restSlug, res.orgID,
	).Scan(&res.restID); err != nil {
		fatal("restaurant "+spec.restSlug, err)
	}
	fmt.Printf("org id=%d restaurant id=%d\n", res.orgID, res.restID)

	// Theme (tenant_themes is restaurant-scoped).
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_themes (restaurant_id, preset, tokens_json) VALUES ($1, $2, $3::jsonb)
		 ON CONFLICT (restaurant_id) DO UPDATE SET preset = EXCLUDED.preset, tokens_json = EXCLUDED.tokens_json, updated_at = NOW()`,
		res.restID, spec.themePreset, spec.themeTokens,
	); err != nil {
		fmt.Printf("theme: %v\n", err)
	}

	// Subscriptions (org + restaurant) + billing profile.
	planID := planIDByTier[spec.planTier]
	if planID == 0 {
		planID = freePlanID
	}
	seedSubscriptions(ctx, pool, res.orgID, res.restID, planID, spec)

	// Branches.
	for i, b := range spec.branches {
		sb := seedBranch(ctx, pool, q, res.orgID, res.restID, b, spec, i == 0)
		res.branches = append(res.branches, sb)
	}

	// Live data on the primary branch only.
	if len(res.branches) > 0 {
		seedLiveData(ctx, pool, q, res.branches[0])
	}
	return res
}

func seedSubscriptions(ctx context.Context, pool *pgxpool.Pool, orgID, restID, planID int64, spec tenantSpec) {
	now := time.Now().UTC()
	var trialEnds, expires any
	switch spec.subStatus {
	case "trial":
		trialEnds = now.Add(30 * 24 * time.Hour)
		expires = nil
	default: // active
		trialEnds = nil
		expires = now.Add(365 * 24 * time.Hour)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO organization_subscriptions (organization_id, plan_id, status, started_at, trial_ends_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (organization_id) DO UPDATE SET plan_id = EXCLUDED.plan_id, status = EXCLUDED.status,
		   started_at = EXCLUDED.started_at, trial_ends_at = EXCLUDED.trial_ends_at, expires_at = EXCLUDED.expires_at, updated_at = NOW()`,
		orgID, planID, spec.subStatus, now, trialEnds, expires,
	); err != nil {
		fmt.Printf("org subscription: %v\n", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO restaurant_subscriptions (restaurant_id, plan_id, status, trial_ends_at, current_period_start, current_period_end)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (restaurant_id) DO UPDATE SET plan_id = EXCLUDED.plan_id, status = EXCLUDED.status,
		   trial_ends_at = EXCLUDED.trial_ends_at, updated_at = NOW()`,
		restID, planID, spec.subStatus, trialEnds, now, expires,
	); err != nil {
		fmt.Printf("restaurant subscription: %v\n", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO organization_billing_profiles (organization_id, business_name, gst_number, billing_email, currency)
		 VALUES ($1, $2, $3, $4, 'INR')
		 ON CONFLICT (organization_id) DO UPDATE SET business_name = EXCLUDED.business_name, gst_number = EXCLUDED.gst_number,
		   billing_email = EXCLUDED.billing_email, updated_at = NOW()`,
		orgID, spec.orgName+" Pvt Ltd", spec.gstNumber, spec.billingEmail,
	); err != nil {
		fmt.Printf("billing profile: %v\n", err)
	}
}

func seedBranch(ctx context.Context, pool *pgxpool.Pool, q *sqlc.Queries, orgID, restID int64, b branchSpec, spec tenantSpec, isPrimary bool) seededBranch {
	out := seededBranch{name: b.name, code: b.code, orderPrefix: b.orderPrefix}
	if err := pool.QueryRow(ctx,
		`INSERT INTO branches (restaurant_id, organization_id, name, address, timezone, branch_code, order_prefix)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (branch_code) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		restID, orgID, b.name, b.address, b.timezone, b.code, b.orderPrefix,
	).Scan(&out.id); err != nil {
		// branch_code may not be the conflict target; fall back to lookup.
		if e2 := pool.QueryRow(ctx, `SELECT id FROM branches WHERE branch_code = $1`, b.code).Scan(&out.id); e2 != nil {
			fatal("branch "+b.code, err)
		}
	}
	fmt.Printf("branch id=%d code=%s name=%q\n", out.id, b.code, b.name)

	// Collateral (per branch).
	collateral := fmt.Sprintf(
		`{"format":%q,"content_toggles":{"welcome_text":true,"wifi":true,"footer_text":true},"welcome_text":%q,"footer_text":%q,"wifi_info":{"ssid":%q,"password":%q}}`,
		spec.collateralFormat, spec.welcomeText, spec.footerText, spec.wifiSSID, spec.wifiPass,
	)
	if _, err := pool.Exec(ctx,
		`INSERT INTO branch_collateral (branch_id, config_json) VALUES ($1, $2::jsonb)
		 ON CONFLICT (branch_id) DO UPDATE SET config_json = EXCLUDED.config_json, updated_at = NOW()`,
		out.id, collateral,
	); err != nil {
		fmt.Printf("collateral: %v\n", err)
	}

	// Tables (3 per branch).
	for i := 1; i <= 3; i++ {
		token := mustToken()
		ident := fmt.Sprintf("T%d", i)
		if _, err := pool.Exec(ctx,
			`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
			 VALUES ($1, $2, 4, $3, 'available')
			 ON CONFLICT (branch_id, identifier) DO UPDATE SET qr_code_token = EXCLUDED.qr_code_token`,
			out.id, ident, token,
		); err != nil {
			fatal("table "+ident, err)
		}
		// Re-read the token actually stored (handles the conflict-update path).
		var stored string
		_ = pool.QueryRow(ctx, `SELECT qr_code_token FROM tables WHERE branch_id=$1 AND identifier=$2`, out.id, ident).Scan(&stored)
		out.tables = append(out.tables, seededTable{identifier: ident, token: stored})
	}

	// Staff: every branch gets manager/waiter/kitchen; the primary branch also gets the org owner.
	type staffSpec struct {
		role sqlc.StaffRole
		name string
		pin  string
		code string
	}
	roster := []staffSpec{
		{sqlc.StaffRoleManager, "Manager " + b.code, "2222", b.code + "-MGR"},
		{sqlc.StaffRoleWaiter, "Waiter " + b.code, "3333", b.code + "-WTR"},
		{sqlc.StaffRoleKitchen, "Chef " + b.code, "4444", b.code + "-KIT"},
	}
	if isPrimary {
		roster = append([]staffSpec{{sqlc.StaffRoleOwner, "Owner " + spec.logical, "1111", b.code + "-OWN"}}, roster...)
	}
	for _, s := range roster {
		hash, err := services.HashPIN(s.pin)
		if err != nil {
			fatal("hash pin", err)
		}
		staff, err := q.CreateStaff(ctx, sqlc.CreateStaffParams{
			BranchID:  out.id,
			Name:      s.name,
			Role:      s.role,
			PinHash:   hash,
			StaffCode: s.code,
		})
		if err != nil {
			fmt.Printf("staff %s may already exist: %v\n", s.code, err)
			continue
		}
		if s.role == sqlc.StaffRoleOwner {
			if _, err := pool.Exec(ctx,
				`INSERT INTO organization_members (organization_id, staff_id, role, status)
				 VALUES ($1, $2, 'owner', 'active')
				 ON CONFLICT (organization_id, staff_id) DO UPDATE SET role = EXCLUDED.role, status = EXCLUDED.status`,
				orgID, staff.ID,
			); err != nil {
				fmt.Printf("org member: %v\n", err)
			}
		}
		fmt.Printf("  staff id=%d %q role=%s pin=%s code=%s\n", staff.ID, s.name, s.role, s.pin, s.code)
	}

	// Menu (per branch — restaurant menus are branch-scoped in this schema).
	seedMenu(ctx, pool, out.id, spec.menu)
	return out
}

func seedMenu(ctx context.Context, pool *pgxpool.Pool, branchID int64, cats []catSpec) {
	for pos, cat := range cats {
		var catID int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO menu_categories (branch_id, name, position) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING RETURNING id`,
			branchID, cat.name, pos,
		).Scan(&catID); err != nil {
			if e2 := pool.QueryRow(ctx, `SELECT id FROM menu_categories WHERE branch_id=$1 AND name=$2`, branchID, cat.name).Scan(&catID); e2 != nil {
				fmt.Printf("category %s: %v\n", cat.name, err)
				continue
			}
		}
		for ipos, item := range cat.items {
			var itemID int64
			var imageURL *string
			if u, ok := stockImageURLs[item.name]; ok {
				imageURL = &u
			}
			if err := pool.QueryRow(ctx,
				`INSERT INTO menu_items (category_id, branch_id, name, price, position, image_url)
				 VALUES ($1, $2, $3, $4::numeric, $5, $6)
				 ON CONFLICT DO NOTHING RETURNING id`,
				catID, branchID, item.name, item.price, ipos, imageURL,
			).Scan(&itemID); err != nil {
				if e2 := pool.QueryRow(ctx, `SELECT id FROM menu_items WHERE branch_id=$1 AND name=$2`, branchID, item.name).Scan(&itemID); e2 != nil {
					fmt.Printf("item %s: %v\n", item.name, err)
					continue
				}
			}
			for _, mod := range item.mods {
				_, _ = pool.Exec(ctx,
					`INSERT INTO item_modifiers (item_id, name, price_delta, is_required, modifier_group, single_select)
					 VALUES ($1, $2, $3::numeric, $4, $5, $6) ON CONFLICT DO NOTHING`,
					itemID, mod.name, mod.delta, mod.req, mod.group, mod.single,
				)
			}
		}
	}
}

// seedLiveData populates the primary branch with active sessions / orders / assistance
// and one payment awaiting staff confirmation, so kitchen/waiter/guest dashboards are
// non-empty immediately. Uses raw INSERTs that set the operational-id columns the sqlc
// CreateSession/CreateOrder helpers omit (the cause of the prior seed's failure).
func seedLiveData(ctx context.Context, pool *pgxpool.Pool, q *sqlc.Queries, branch seededBranch) {
	if len(branch.tables) < 2 {
		return
	}
	now := time.Now()
	dateStr := now.Format("20060102")

	// Resolve table ids + first two menu items.
	tableID := func(ident string) int64 {
		var id int64
		_ = pool.QueryRow(ctx, `SELECT id FROM tables WHERE branch_id=$1 AND identifier=$2`, branch.id, ident).Scan(&id)
		return id
	}
	var itemIDs []int64
	rows, _ := pool.Query(ctx, `SELECT id FROM menu_items WHERE branch_id=$1 ORDER BY id LIMIT 2`, branch.id)
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		itemIDs = append(itemIDs, id)
	}
	rows.Close()
	var itemPrice pgtype.Numeric
	if len(itemIDs) > 0 {
		_ = pool.QueryRow(ctx, `SELECT price FROM menu_items WHERE id=$1`, itemIDs[0]).Scan(&itemPrice)
	}

	// ── T1: active session + confirmed order + pending assistance ──
	t1 := tableID("T1")
	if t1 != 0 && !tableHasLiveSession(ctx, pool, t1) {
		sess := createSession(ctx, pool, branch, t1, dateStr, "active")
		host := createHost(ctx, pool, q, sess, "Riya")
		markOccupied(ctx, pool, t1)
		orderID := createOrder(ctx, pool, branch, sess, host, dateStr, "confirmed", "220.00")
		if orderID != "" && len(itemIDs) > 0 {
			_, _ = q.CreateOrderItem(ctx, sqlc.CreateOrderItemParams{
				OrderID: pgUUID(orderID), MenuItemID: itemIDs[0], Quantity: 2,
				UnitPrice: itemPrice, SelectedModifiersJson: []byte("[]"),
			})
		}
		if _, err := q.CreateAssistanceRequest(ctx, sqlc.CreateAssistanceRequestParams{
			SessionID: pgUUID(sess), TableID: t1, ParticipantID: pgtype.Int8{Int64: host, Valid: true}, Type: sqlc.AssistanceTypeWaiter,
		}); err != nil {
			fmt.Printf("  assistance: %v\n", err)
		}
		fmt.Printf("  live T1: session=%s order=confirmed assistance=pending\n", sess)
	}

	// ── T2: payment_pending session + order + payment awaiting staff confirmation ──
	t2 := tableID("T2")
	if t2 != 0 && !tableHasLiveSession(ctx, pool, t2) {
		sess := createSession(ctx, pool, branch, t2, dateStr, "payment_pending")
		host := createHost(ctx, pool, q, sess, "Dev")
		markOccupied(ctx, pool, t2)
		orderID := createOrder(ctx, pool, branch, sess, host, dateStr, "served", "180.00")
		if orderID != "" && len(itemIDs) > 0 {
			_, _ = q.CreateOrderItem(ctx, sqlc.CreateOrderItemParams{
				OrderID: pgUUID(orderID), MenuItemID: itemIDs[0], Quantity: 1,
				UnitPrice: itemPrice, SelectedModifiersJson: []byte("[]"),
			})
		}
		// Allocate from payment_sequences so the reference matches the app and never collides.
		paySeq := nextSeq(ctx, pool, "payment_sequences", branch.id)
		ref := fmt.Sprintf("%s-PAY-%s-%04d", branch.code, dateStr, paySeq)
		if _, err := pool.Exec(ctx,
			`INSERT INTO payments (session_id, order_id, amount, method, status, branch_id, currency,
			   payment_business_date, payment_sequence, payment_reference)
			 VALUES ($1, $2, 180.00, 'cash', 'requires_staff_confirmation', $3, 'INR', CURRENT_DATE, $4, $5)`,
			sess, orderID, branch.id, paySeq, ref,
		); err != nil {
			fmt.Printf("  payment: %v\n", err)
		}
		fmt.Printf("  live T2: session=%s payment=requires_staff_confirmation ref=%s\n", sess, ref)
	}
}

// nextSeq advances and returns the per-branch/day counter from one of the operational
// sequence tables (session_sequences / order_sequences / payment_sequences), exactly as
// the app's NextSessionNumber/NextOrderNumber/NextPaymentNumber do. Seeding through these
// counters guarantees seeded operational references never collide with app-generated ones.
func nextSeq(ctx context.Context, pool *pgxpool.Pool, table string, branchID int64) int32 {
	var seq int32
	q := fmt.Sprintf(
		`INSERT INTO %s (branch_id, date, last_seq) VALUES ($1, CURRENT_DATE, 1)
		 ON CONFLICT (branch_id, date) DO UPDATE SET last_seq = %s.last_seq + 1
		 RETURNING last_seq`, table, table)
	if err := pool.QueryRow(ctx, q, branchID).Scan(&seq); err != nil {
		fatal("next seq "+table, err)
	}
	return seq
}

func tableHasLiveSession(ctx context.Context, pool *pgxpool.Pool, tableID int64) bool {
	var id string
	err := pool.QueryRow(ctx,
		`SELECT id FROM sessions WHERE table_id=$1 AND status IN ('active','payment_pending','awaiting_reactivation') LIMIT 1`,
		tableID,
	).Scan(&id)
	return err == nil
}

func createSession(ctx context.Context, pool *pgxpool.Pool, branch seededBranch, tableID int64, dateStr, status string) string {
	var id string
	seq := nextSeq(ctx, pool, "session_sequences", branch.id)
	num := fmt.Sprintf("%s-S-%s-%03d", branch.code, dateStr, seq)
	if err := pool.QueryRow(ctx,
		`INSERT INTO sessions (branch_id, table_id, session_token, status, session_business_date, visit_number, session_number)
		 VALUES ($1, $2, $3, $4::session_status, CURRENT_DATE, $5, $6) RETURNING id`,
		branch.id, tableID, mustToken(), status, seq, num,
	).Scan(&id); err != nil {
		fatal("create session", err)
	}
	return id
}

func createHost(ctx context.Context, pool *pgxpool.Pool, q *sqlc.Queries, sessionID string, name string) int64 {
	p, err := q.CreateParticipant(ctx, sqlc.CreateParticipantParams{
		SessionID: pgUUID(sessionID), DisplayName: name, IsHost: true,
	})
	if err != nil {
		fatal("create participant", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sessions SET host_participant_id=$1 WHERE id=$2`, p.ID, sessionID); err != nil {
		fatal("set host", err)
	}
	return p.ID
}

func markOccupied(ctx context.Context, pool *pgxpool.Pool, tableID int64) {
	_, _ = pool.Exec(ctx, `UPDATE tables SET status='occupied' WHERE id=$1`, tableID)
}

func createOrder(ctx context.Context, pool *pgxpool.Pool, branch seededBranch, sessionID string, host int64, dateStr, status, total string) string {
	var id string
	seq := nextSeq(ctx, pool, "order_sequences", branch.id)
	disp := fmt.Sprintf("%s%d", branch.orderPrefix, seq) // matches app: orderPrefix + seq
	opID := fmt.Sprintf("%s-%s-%s", branch.code, dateStr, disp)
	if err := pool.QueryRow(ctx,
		`INSERT INTO orders (session_id, branch_id, placed_by_participant_id, status, idempotency_key,
		   total_amount, order_number, order_business_date, order_number_display, order_operational_id)
		 VALUES ($1, $2, $3, $4::order_status, $5, $6::numeric, $7, CURRENT_DATE, $7, $8) RETURNING id`,
		sessionID, branch.id, host, status, "seed-"+opID, total, disp, opID,
	).Scan(&id); err != nil {
		fmt.Printf("  create order: %v\n", err)
		return ""
	}
	return id
}

// ─────────────────────────── tenant catalog ─────────────────────────────────

func tenantCatalog() []tenantSpec {
	return []tenantSpec{
		{
			logical: "Alpha", orgCode: "saffron-house", orgName: "Saffron House Hospitality",
			restSlug: "saffron-house", restName: "Saffron House",
			planTier: "premium", subStatus: "active",
			themePreset: "dark-luxury", themeTokens: `{"color.primary":"#C8A24B","color.surface":"#1A1410"}`,
			collateralFormat: "standing_card", gstNumber: "27AAACS1234A1Z5", billingEmail: "billing@saffronhouse.example",
			wifiSSID: "SaffronGuest", wifiPass: "saffron2026",
			welcomeText: "Welcome to Saffron House", footerText: "Scan • Order • Relax",
			branches: []branchSpec{
				{"Bandra", "SAFF-BND", "Linking Road, Bandra West, Mumbai", "Asia/Kolkata", "BND"},
				{"Indiranagar", "SAFF-IND", "100 Feet Road, Indiranagar, Bengaluru", "Asia/Kolkata", "IND"},
				{"Connaught Place", "SAFF-CP", "Block A, Connaught Place, New Delhi", "Asia/Kolkata", "CP"},
			},
			menu: restaurantMenu(),
		},
		{
			logical: "Beta", orgCode: "copper-pot", orgName: "Copper Pot Kitchen",
			restSlug: "copper-pot-kitchen", restName: "Copper Pot Kitchen",
			planTier: "standard", subStatus: "active",
			themePreset: "warm-cafe", themeTokens: `{"color.primary":"#B5651D","color.surface":"#FBF3E7"}`,
			collateralFormat: "table_tent", gstNumber: "29BBBCS5678B1Z4", billingEmail: "billing@copperpot.example",
			wifiSSID: "CopperPotWiFi", wifiPass: "copper123",
			welcomeText: "Welcome to Copper Pot Kitchen", footerText: "Fresh • Local • Honest",
			branches: []branchSpec{
				{"Koramangala", "COPR-KOR", "5th Block, Koramangala, Bengaluru", "Asia/Kolkata", "KOR"},
			},
			menu: restaurantMenu(),
		},
		{
			logical: "Gamma", orgCode: "urban-brew", orgName: "Urban Brew Cafe",
			restSlug: "urban-brew-cafe", restName: "Urban Brew Café",
			planTier: "free", subStatus: "trial",
			themePreset: "vibrant", themeTokens: `{"color.primary":"#E63946","color.surface":"#FFFFFF"}`,
			collateralFormat: "sticker", gstNumber: "06CCCUB9012C1Z3", billingEmail: "billing@urbanbrew.example",
			wifiSSID: "UrbanBrewFree", wifiPass: "brewlove",
			welcomeText: "Welcome to Urban Brew Café", footerText: "Good coffee, good vibes",
			branches: []branchSpec{
				{"Cyber Hub", "BREW-CYB", "DLF Cyber Hub, Gurugram", "Asia/Kolkata", "CYB"},
			},
			menu: cafeMenu(),
		},
	}
}

func restaurantMenu() []catSpec {
	return []catSpec{
		{"Starters", []itemSpec{
			{"Paneer Tikka", "180", []modSpec{{"Extra Sauce", "20", false, "", false}}},
			{"Veg Spring Rolls", "140", nil},
			{"Chicken 65", "210", nil},
		}},
		{"Mains", []itemSpec{
			{"Dal Makhani", "220", nil},
			{"Butter Chicken", "280", []modSpec{{"Extra Butter", "30", false, "", false}}},
			{"Veg Biryani", "200", nil},
		}},
		{"Breads", []itemSpec{
			{"Butter Naan", "45", nil},
			{"Garlic Roti", "35", nil},
			{"Laccha Paratha", "50", nil},
		}},
		{"Beverages", []itemSpec{
			{"Sweet Lassi", "80", []modSpec{{"Sweet", "0", true, "style", true}, {"Salted", "0", false, "style", true}}},
			{"Masala Chai", "30", nil},
			{"Mango Shake", "90", nil},
		}},
		{"Desserts", []itemSpec{
			{"Gulab Jamun", "60", nil},
			{"Ice Cream", "80", []modSpec{{"Chocolate", "10", false, "flavour", true}, {"Vanilla", "0", false, "flavour", true}}},
			{"Kheer", "70", nil},
		}},
	}
}

func cafeMenu() []catSpec {
	return []catSpec{
		{"Coffee", []itemSpec{
			{"Espresso", "120", nil},
			{"Cappuccino", "160", []modSpec{{"Oat Milk", "30", false, "", false}}},
			{"Cold Brew", "180", nil},
		}},
		{"Tea", []itemSpec{
			{"Masala Chai", "90", nil},
			{"Green Tea", "100", nil},
		}},
		{"Sandwiches", []itemSpec{
			{"Grilled Veg Sandwich", "150", nil},
			{"Chicken Club", "190", nil},
		}},
		{"Bakery", []itemSpec{
			{"Butter Croissant", "120", nil},
			{"Chocolate Muffin", "110", nil},
		}},
		{"Cold Drinks", []itemSpec{
			{"Iced Latte", "180", []modSpec{{"Hazelnut", "20", false, "", false}}},
			{"Lemonade", "100", nil},
		}},
	}
}

// ─────────────────────────── summary + helpers ──────────────────────────────

func printSummary(users []seededPlatformUser, tenants []seededTenant) {
	fmt.Println("\n══════════════════ SEED COMPLETE ══════════════════")
	fmt.Println("\nPLATFORM USERS (POST /platform/auth — email + password):")
	for _, u := range users {
		fmt.Printf("  %-26s %-16s %s\n", u.email, u.pass, u.role)
	}
	fmt.Println("\nSTAFF PINs (POST /staff/auth — branch_id + pin):")
	fmt.Println("  owner=1111  manager=2222  waiter=3333  kitchen=4444")
	for _, t := range tenants {
		fmt.Printf("\nTENANT %s — %s  [org=%d restaurant=%d plan=%s/%s theme=%s]\n",
			t.spec.logical, t.spec.restName, t.orgID, t.restID, t.spec.planTier, t.spec.subStatus, t.spec.themePreset)
		for _, b := range t.branches {
			fmt.Printf("  branch %q  code=%s  id=%d\n", b.name, b.code, b.id)
			for _, tb := range b.tables {
				fmt.Printf("    %s qr=%s\n", tb.identifier, tb.token)
			}
		}
	}
}

func mustToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func pgUUID(s string) uuid.UUID {
	return uuid.MustParse(s)
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}
