//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Tenant isolation regression suite for the item-scoped staff routes — the ones that
// carry no branch path parameter and therefore no BranchTenantGuard.
//
// Every case runs twice: with AUTHZ_CENTRAL_POLICY_ENFORCE off (the shipped default) and
// on. Both must deny. That second dimension is the point of this file: these routes once
// returned 200/204 for a foreign organization's owner precisely because the denial was
// computed, audited, and then allowed through while the flag was off.
// See release-certification/authz-scope-investigation-2026-08-04.md.

type scopeFixture struct {
	repos *repository.Repos
	menu  *services.MenuService

	// Tenant A — two branches under one organization.
	orgA          int64
	branchA1      int64
	branchA2      int64
	itemA2        int64
	categoryA2    int64
	modifierA2    int64
	staffA2Waiter int64
	sessionA2     uuid.UUID

	// Tenant B — a separate organization entirely.
	orgB          int64
	branchB1      int64
	itemB1        int64
	categoryB1    int64
	modifierB1    int64
	staffB1Waiter int64
	sessionB1     uuid.UUID

	// The actor: an owner at branch A1.
	actorA1 services.StaffSession
	// Own-branch objects, for the positive control.
	itemA1     int64
	categoryA1 int64
	sessionA1  uuid.UUID
}

func newScopeFixture(t *testing.T, pool *pgxpool.Pool) scopeFixture {
	t.Helper()
	ctx := context.Background()

	// SeedFixtures generates a fresh slug per call, so two calls yield two fully
	// independent organizations.
	a := testutil.SeedFixtures(t, pool)
	b := testutil.SeedFixtures(t, pool)

	f := scopeFixture{
		repos:      testutil.NewTestRepos(pool),
		orgA:       a.OrganizationID,
		branchA1:   a.BranchID,
		itemA1:     a.MenuItemID,
		orgB:       b.OrganizationID,
		branchB1:   b.BranchID,
		itemB1:     b.MenuItemID,
		modifierB1: b.ModifierID,
	}

	redisClient := openHandlerTestRedis(t)
	metrics := observability.NewMetrics()
	cache := redisPkg.NewCache(redisClient, nil, nil)
	pubsub := redisPkg.NewPubSub(redisClient, zerolog.Nop(), metrics)
	publisher := events.NewPublisher(pubsub, zerolog.Nop())
	publisher.SetEventStore(f.repos)
	f.menu = services.NewMenuService(f.repos, cache, publisher)

	must := func(label string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}

	// A second branch under tenant A's organization — the sibling-branch case that bites
	// a single-tenant pilot with more than one location.
	must("branch A2", pool.QueryRow(ctx,
		`INSERT INTO branches (restaurant_id, organization_id, name, address, timezone, branch_code)
		 VALUES ($1, $2, 'Sibling Branch', '2 Test St', 'UTC', $3) RETURNING id`,
		a.RestaurantID, a.OrganizationID, "SIB-"+uuid.NewString()[:8],
	).Scan(&f.branchA2))

	// A2 needs its own table: sessions are unique-per-table while active.
	var tableA2 int64
	must("table A2", pool.QueryRow(ctx,
		`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
		 VALUES ($1, 'A2-T1', 4, $2, 'available') RETURNING id`,
		f.branchA2, uuid.NewString(),
	).Scan(&tableA2))

	must("category A1", pool.QueryRow(ctx,
		`INSERT INTO menu_categories (branch_id, name, position) VALUES ($1, 'A1 Cat', 1) RETURNING id`,
		f.branchA1).Scan(&f.categoryA1))
	must("category A2", pool.QueryRow(ctx,
		`INSERT INTO menu_categories (branch_id, name, position) VALUES ($1, 'A2 Cat', 1) RETURNING id`,
		f.branchA2).Scan(&f.categoryA2))
	must("item A2", pool.QueryRow(ctx,
		`INSERT INTO menu_items (category_id, branch_id, name, price, is_available) VALUES ($1, $2, 'A2 Dish', 10.00, true) RETURNING id`,
		f.categoryA2, f.branchA2).Scan(&f.itemA2))
	must("modifier A2", pool.QueryRow(ctx,
		`INSERT INTO item_modifiers (item_id, name, price_delta) VALUES ($1, 'A2 Extra', 1.50) RETURNING id`,
		f.itemA2).Scan(&f.modifierA2))

	// Tenant B's category is the one SeedFixtures created for its menu item.
	must("category B1", pool.QueryRow(ctx,
		`SELECT category_id FROM menu_items WHERE id = $1`, f.itemB1).Scan(&f.categoryB1))

	must("staff A2", pool.QueryRow(ctx,
		`INSERT INTO staff (branch_id, name, role, pin_hash, staff_code) VALUES ($1, 'A2 Waiter', 'waiter', 'x', $2) RETURNING id`,
		f.branchA2, "A2W").Scan(&f.staffA2Waiter))
	must("staff B1", pool.QueryRow(ctx,
		`INSERT INTO staff (branch_id, name, role, pin_hash, staff_code) VALUES ($1, 'B1 Waiter', 'waiter', 'x', $2) RETURNING id`,
		f.branchB1, "B1W").Scan(&f.staffB1Waiter))

	var staffA1 int64
	must("staff A1", pool.QueryRow(ctx,
		`INSERT INTO staff (branch_id, name, role, pin_hash, staff_code) VALUES ($1, 'A1 Owner', 'owner', 'x', $2) RETURNING id`,
		f.branchA1, "A1O").Scan(&staffA1))

	f.sessionA1 = seedScopeSession(t, pool, f.branchA1, a.TableID, "A1")
	f.sessionA2 = seedScopeSession(t, pool, f.branchA2, tableA2, "A2")
	f.sessionB1 = seedScopeSession(t, pool, f.branchB1, b.TableID, "B1")

	f.actorA1 = services.StaffSession{
		SessionID:      uuid.New(),
		StaffID:        staffA1,
		BranchID:       f.branchA1,
		OrganizationID: f.orgA,
		Role:           sqlc.StaffRoleOwner,
	}

	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "organizations", "event_log", "audit_log")
	})
	return f
}

func seedScopeSession(t *testing.T, pool *pgxpool.Pool, branchID, tableID int64, label string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO sessions (branch_id, table_id, session_token, status, session_business_date, visit_number, session_number)
		 VALUES ($1, $2, $3, 'active', CURRENT_DATE, 1, $4) RETURNING id`,
		branchID, tableID, uuid.NewString(), label+"-"+uuid.NewString()[:8],
	).Scan(&id); err != nil {
		t.Fatalf("seed session %s: %v", label, err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO event_log (session_id, branch_id, event_type, actor_type, payload)
		 VALUES ($1, $2, 'session.created', 'guest', '{"private":"tenant-payload"}')`,
		id, branchID); err != nil {
		t.Fatalf("seed event %s: %v", label, err)
	}
	return id
}

// call drives one handler through a gin test context with the actor's staff session
// already installed, exactly as middleware.StaffAuth would.
func call(t *testing.T, sess services.StaffSession, method, path string, params gin.Params, body string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	c.Set(middleware.StaffSessionKey, sess)
	h(c)
	// c.Status() only records the code on the ResponseWriter; outside a real gin engine
	// nothing flushes it, so a 204 would otherwise read back as the recorder's default 200.
	c.Writer.WriteHeaderNow()
	return rec
}

type scopeCase struct {
	name   string
	invoke func(t *testing.T, f scopeFixture, menuH *MenuAdminHandler, eventsH *EventLogHandler, target scopeTarget) *httptest.ResponseRecorder
}

// scopeTarget names the objects a case should act on, so the same case body can be
// pointed at a foreign organization, a sibling branch, or the actor's own branch.
type scopeTarget struct {
	item      int64
	category  int64
	modifier  int64
	sessionID uuid.UUID
}

func idParam(v int64) gin.Params { return gin.Params{{Key: "id", Value: strconv.FormatInt(v, 10)}} }

func scopeCases() []scopeCase {
	return []scopeCase{
		{"menu item update", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodPatch, "/menu/items/"+strconv.FormatInt(tg.item, 10), idParam(tg.item),
				`{"name":"RENAMED","price":99}`, m.UpdateItem)
		}},
		{"menu item availability", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodPatch, "/menu/items/x/availability", idParam(tg.item),
				`{"available":false}`, m.ToggleAvailability)
		}},
		{"menu item featured", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodPatch, "/menu/items/x/featured", idParam(tg.item),
				`{"is_featured":true}`, m.ToggleFeatured)
		}},
		{"menu item delete", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodDelete, "/menu/items/x", idParam(tg.item), "", m.DeleteMenuItem)
		}},
		{"menu category update", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodPatch, "/menu/categories/x", idParam(tg.category),
				`{"name":"RENAMED","is_active":true}`, m.UpdateMenuCategory)
		}},
		{"menu category delete", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodDelete, "/menu/categories/x", idParam(tg.category), "", m.DeleteMenuCategory)
		}},
		{"menu modifier add", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodPost, "/menu/items/x/modifiers", idParam(tg.item),
				`{"name":"INJECTED","price_delta":0}`, m.AddItemModifier)
		}},
		{"menu modifier update", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodPatch, "/menu/modifiers/x", idParam(tg.modifier),
				`{"name":"INJECTED","price_delta":0}`, m.UpdateItemModifier)
		}},
		{"menu modifier delete", func(t *testing.T, f scopeFixture, m *MenuAdminHandler, _ *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodDelete, "/menu/modifiers/x", idParam(tg.modifier), "", m.DeleteItemModifier)
		}},
		{"session events read", func(t *testing.T, f scopeFixture, _ *MenuAdminHandler, e *EventLogHandler, tg scopeTarget) *httptest.ResponseRecorder {
			return call(t, f.actorA1, http.MethodGet, "/sessions/x/events",
				gin.Params{{Key: "id", Value: tg.sessionID.String()}}, "", e.GetSessionEvents)
		}},
	}
}

func (f scopeFixture) handlers(enforce bool) (*MenuAdminHandler, *EventLogHandler) {
	authorizer := authz.NewEnforcingAuthorizer(enforce)
	writer := audit.NewWriter(sqlc.New(nil), false, zerolog.Nop(), nil)
	return NewMenuAdminHandler(f.menu, f.repos, authorizer, writer), NewEventLogHandler(f.repos)
}

// TestCrossTenantWritesAreDeniedRegardlessOfEnforcementFlag is the core regression guard.
// A foreign organization's owner, and an owner at a sibling branch of the same
// organization, must be refused on every item-scoped route — with the central
// enforcement flag both off and on.
func TestCrossTenantWritesAreDeniedRegardlessOfEnforcementFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testutil.OpenTestDB(t)
	f := newScopeFixture(t, pool)

	targets := map[string]scopeTarget{
		"cross-organization": {item: f.itemB1, category: f.categoryB1, modifier: f.modifierB1, sessionID: f.sessionB1},
		"cross-branch":       {item: f.itemA2, category: f.categoryA2, modifier: f.modifierA2, sessionID: f.sessionA2},
	}

	for _, enforce := range []bool{false, true} {
		for scope, tg := range targets {
			for _, tc := range scopeCases() {
				name := fmt.Sprintf("enforce=%v/%s/%s", enforce, scope, tc.name)
				t.Run(name, func(t *testing.T) {
					menuH, eventsH := f.handlers(enforce)
					rec := tc.invoke(t, f, menuH, eventsH, tg)
					if rec.Code != http.StatusForbidden {
						t.Fatalf("got %d, want 403 — %s reachable by a foreign actor (body: %s)",
							rec.Code, scope, strings.TrimSpace(rec.Body.String()))
					}
				})
			}
		}
	}
}

// TestSameBranchOperationsStillSucceed is the positive control: the guards above must not
// have broken the legitimate path they sit on.
func TestSameBranchOperationsStillSucceed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testutil.OpenTestDB(t)
	f := newScopeFixture(t, pool)

	for _, enforce := range []bool{false, true} {
		t.Run(fmt.Sprintf("enforce=%v", enforce), func(t *testing.T) {
			menuH, eventsH := f.handlers(enforce)

			rec := call(t, f.actorA1, http.MethodPatch, "/menu/items/x", idParam(f.itemA1),
				`{"name":"Legitimately Renamed","price":15}`, menuH.UpdateItem)
			if rec.Code != http.StatusOK {
				t.Fatalf("own-branch item update: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}

			rec = call(t, f.actorA1, http.MethodPatch, "/menu/items/x/availability", idParam(f.itemA1),
				`{"available":false}`, menuH.ToggleAvailability)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("own-branch availability toggle: got %d, want 204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}

			rec = call(t, f.actorA1, http.MethodPatch, "/menu/categories/x", idParam(f.categoryA1),
				`{"name":"Own Category","is_active":true}`, menuH.UpdateMenuCategory)
			if rec.Code != http.StatusOK {
				t.Fatalf("own-branch category update: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}

			rec = call(t, f.actorA1, http.MethodGet, "/sessions/x/events",
				gin.Params{{Key: "id", Value: f.sessionA1.String()}}, "", eventsH.GetSessionEvents)
			if rec.Code != http.StatusOK {
				t.Fatalf("own-branch session events: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			var events []map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
				t.Fatalf("decode events: %v", err)
			}
			if len(events) == 0 {
				t.Fatal("own-branch session events returned an empty timeline; the positive control proves nothing")
			}
		})
	}
}

// TestCrossTenantWritesLeaveResourcesUntouched proves the denials are real refusals and
// not merely a 403 returned after the mutation already landed.
func TestCrossTenantWritesLeaveResourcesUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testutil.OpenTestDB(t)
	f := newScopeFixture(t, pool)
	ctx := context.Background()

	type snapshot struct {
		name        string
		isAvailable bool
		isFeatured  bool
	}
	read := func(itemID int64) snapshot {
		t.Helper()
		var s snapshot
		if err := pool.QueryRow(ctx,
			`SELECT name, is_available, is_featured FROM menu_items WHERE id = $1`, itemID,
		).Scan(&s.name, &s.isAvailable, &s.isFeatured); err != nil {
			t.Fatalf("read item %d: %v", itemID, err)
		}
		return s
	}

	for _, tc := range []struct {
		scope  string
		itemID int64
	}{
		{"cross-organization", f.itemB1},
		{"cross-branch", f.itemA2},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			before := read(tc.itemID)
			menuH, _ := f.handlers(false) // shipped default: enforcement off

			call(t, f.actorA1, http.MethodPatch, "/menu/items/x", idParam(tc.itemID), `{"name":"OWNED","price":99}`, menuH.UpdateItem)
			call(t, f.actorA1, http.MethodPatch, "/menu/items/x/availability", idParam(tc.itemID), `{"available":false}`, menuH.ToggleAvailability)
			call(t, f.actorA1, http.MethodPatch, "/menu/items/x/featured", idParam(tc.itemID), `{"is_featured":true}`, menuH.ToggleFeatured)
			call(t, f.actorA1, http.MethodDelete, "/menu/items/x", idParam(tc.itemID), "", menuH.DeleteMenuItem)

			if after := read(tc.itemID); after != before {
				t.Fatalf("resource mutated by a foreign actor: before=%+v after=%+v", before, after)
			}
		})
	}
}
