//go:build integration

package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Invariant T1: a suspended organization accepts no new sessions and no new
// joins, and a QR scan says so plainly — but a session already in progress
// finishes normally. Suspension is a billing and compliance action, not an
// emergency stop.

// setTenantStatus flips an organization or branch lifecycle column directly.
// The platform endpoint is exercised end-to-end by e2e/tenancy/T-03; here we
// only need the state the endpoint produces.
func setTenantStatus(t *testing.T, pool *pgxpool.Pool, table string, id int64, status string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"UPDATE "+table+" SET status = $1 WHERE id = $2", status, id); err != nil {
		t.Fatalf("set %s %d status=%s: %v", table, id, status, err)
	}
}

// tenantStatusHarness wires the services under test with a real gate. The cache
// is nil so every call reads the database — the tests assert the decision, and
// the caching layer is covered separately by TestTenantStatusGate_CacheAndInvalidate.
type tenantStatusHarness struct {
	pool    *pgxpool.Pool
	f       testutil.TestFixtures
	repos   *repository.Repos
	session *services.SessionService
	menu    *services.MenuService
	order   *services.OrderService
	qrToken string
}

func newTenantStatusHarness(t *testing.T, cache *redisPkg.Cache) tenantStatusHarness {
	t.Helper()
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()

	gate := services.NewTenantStatusGate(repos, cache)
	sessionSvc := newTestSessionService(repos, pub)
	sessionSvc.SetTenantStatusGate(gate)
	menuSvc := services.NewMenuService(repos, cache, pub)
	menuSvc.SetTenantStatusGate(gate)

	var qrToken string
	if err := pool.QueryRow(context.Background(),
		`SELECT qr_code_token FROM tables WHERE id = $1`, f.TableID).Scan(&qrToken); err != nil {
		t.Fatalf("read table qr token: %v", err)
	}

	t.Cleanup(func() {
		setTenantStatus(t, pool, "organizations", f.OrganizationID, "active")
		setTenantStatus(t, pool, "branches", f.BranchID, "active")
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items")
	})

	return tenantStatusHarness{
		pool:    pool,
		f:       f,
		repos:   repos,
		session: sessionSvc,
		menu:    menuSvc,
		order:   newTestOrderService(repos, pub),
		qrToken: qrToken,
	}
}

func TestSuspendedOrganization_BlocksCreateJoinAndQRResolve(t *testing.T) {
	h := newTenantStatusHarness(t, nil)
	ctx := context.Background()

	// A session opened while the org was healthy — it must survive the flip.
	live, err := h.session.CreateSession(ctx, h.f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession before suspension: %v", err)
	}

	setTenantStatus(t, h.pool, "organizations", h.f.OrganizationID, "suspended")

	// Create is refused. Use a second table so the refusal can't be confused
	// with the one-active-session-per-table constraint.
	var secondTableID int64
	if err := h.pool.QueryRow(ctx,
		`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
		 VALUES ($1, 'T-SUSPEND', 4, $2, 'available') RETURNING id`,
		h.f.BranchID, "qr-suspend-"+live.Session.ID.String(),
	).Scan(&secondTableID); err != nil {
		t.Fatalf("seed second table: %v", err)
	}
	if _, err := h.session.CreateSession(ctx, secondTableID, "Bob", "fp-bob", ""); !errors.Is(err, domain.ErrOrganizationSuspended) {
		t.Errorf("CreateSession on suspended org: got %v, want ErrOrganizationSuspended", err)
	}

	// Join is refused, even though the session it targets is still active.
	if _, err := h.session.JoinSession(ctx, live.Session.ID, "Carol", "fp-carol", ""); !errors.Is(err, domain.ErrOrganizationSuspended) {
		t.Errorf("JoinSession on suspended org: got %v, want ErrOrganizationSuspended", err)
	}

	// QR resolve is refused — the guest learns the restaurant isn't serving
	// rather than walking into the menu and failing later.
	if _, err := h.menu.GetTableWithActiveSession(ctx, h.qrToken); !errors.Is(err, domain.ErrOrganizationSuspended) {
		t.Errorf("GetTableWithActiveSession on suspended org: got %v, want ErrOrganizationSuspended", err)
	}
}

func TestSuspendedOrganization_LiveSessionKeepsOrdering(t *testing.T) {
	h := newTenantStatusHarness(t, nil)
	ctx := context.Background()

	live, err := h.session.CreateSession(ctx, h.f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	setTenantStatus(t, h.pool, "organizations", h.f.OrganizationID, "suspended")

	// The party mid-meal is untouched: they can still read their session and
	// place an order. Stranding them would be a worse problem than it solves.
	if _, err := h.session.GetSession(ctx, live.Session.ID); err != nil {
		t.Fatalf("GetSession on suspended org: %v", err)
	}
	placed, err := h.order.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             live.Session.ID,
		BranchID:              h.f.BranchID,
		PlacedByParticipantID: live.Participant.ID,
		IdempotencyKey:        "idem-suspended-live",
		Items: []services.OrderItem{
			{MenuItemID: h.f.MenuItemID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder on live session of suspended org: %v", err)
	}
	if placed.Order.ID.String() == "" {
		t.Fatal("expected a placed order")
	}
}

func TestSuspendedBranch_BlocksEntryWithDistinctError(t *testing.T) {
	h := newTenantStatusHarness(t, nil)
	ctx := context.Background()

	// Branch status is its own gate: one location can be closed while the
	// organization that owns it is perfectly healthy.
	setTenantStatus(t, h.pool, "branches", h.f.BranchID, "suspended")

	if _, err := h.session.CreateSession(ctx, h.f.TableID, "Alice", "fp-alice", ""); !errors.Is(err, domain.ErrBranchSuspended) {
		t.Errorf("CreateSession on suspended branch: got %v, want ErrBranchSuspended", err)
	}
	if _, err := h.menu.GetTableWithActiveSession(ctx, h.qrToken); !errors.Is(err, domain.ErrBranchSuspended) {
		t.Errorf("GetTableWithActiveSession on suspended branch: got %v, want ErrBranchSuspended", err)
	}
}

func TestReactivation_RestoresCreateJoinAndQRResolve(t *testing.T) {
	h := newTenantStatusHarness(t, nil)
	ctx := context.Background()

	setTenantStatus(t, h.pool, "organizations", h.f.OrganizationID, "suspended")
	if _, err := h.session.CreateSession(ctx, h.f.TableID, "Alice", "fp-alice", ""); !errors.Is(err, domain.ErrOrganizationSuspended) {
		t.Fatalf("precondition: CreateSession should be blocked, got %v", err)
	}

	setTenantStatus(t, h.pool, "organizations", h.f.OrganizationID, "active")

	if _, err := h.menu.GetTableWithActiveSession(ctx, h.qrToken); err != nil {
		t.Errorf("GetTableWithActiveSession after reactivation: %v", err)
	}
	created, err := h.session.CreateSession(ctx, h.f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession after reactivation: %v", err)
	}
	if _, err := h.session.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", ""); err != nil {
		t.Errorf("JoinSession after reactivation: %v", err)
	}
}

// TestTenantStatusGate_CacheAndInvalidate covers the caching contract the hot
// path depends on: a decision is reused within the TTL (so a QR scan costs no
// query), and Invalidate makes an operator's flip visible at once.
func TestTenantStatusGate_CacheAndInvalidate(t *testing.T) {
	redisClient := openServiceTestRedis(t)
	cache := redisPkg.NewCache(redisClient, nil, nil)
	h := newTenantStatusHarness(t, cache)
	ctx := context.Background()

	gate := services.NewTenantStatusGate(h.repos, cache)
	if err := gate.EnsureBranchAdmitsGuests(ctx, h.f.BranchID); err != nil {
		t.Fatalf("healthy branch: %v", err)
	}

	// Flip the column behind the gate's back: the cached "active" decision is
	// still served, which is exactly the staleness the TTL bounds.
	setTenantStatus(t, h.pool, "organizations", h.f.OrganizationID, "suspended")
	if err := gate.EnsureBranchAdmitsGuests(ctx, h.f.BranchID); err != nil {
		t.Fatalf("within TTL the cached decision should still admit: %v", err)
	}

	// What the platform endpoint does on suspend.
	gate.Invalidate(ctx)
	if err := gate.EnsureBranchAdmitsGuests(ctx, h.f.BranchID); !errors.Is(err, domain.ErrOrganizationSuspended) {
		t.Errorf("after Invalidate: got %v, want ErrOrganizationSuspended", err)
	}

	setTenantStatus(t, h.pool, "organizations", h.f.OrganizationID, "active")
	gate.Invalidate(ctx)
	if err := gate.EnsureBranchAdmitsGuests(ctx, h.f.BranchID); err != nil {
		t.Errorf("after reactivation + Invalidate: %v", err)
	}
}
