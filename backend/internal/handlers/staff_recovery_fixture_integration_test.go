//go:build integration

package handlers

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Shared fixture for the two staff recovery routes — P6 (cancel a stuck payment)
// and S2 (force-close a session). Both need the same shape: two fully separate
// tenants, every staff role on each branch, and live sessions to act on.

// recoveryBranch is one branch plus a staff session per role, ready to act.
type recoveryBranch struct {
	orgID    int64
	branchID int64
	tableID  int64
	itemID   int64

	owner   services.StaffSession
	manager services.StaffSession
	waiter  services.StaffSession
	kitchen services.StaffSession
}

type recoveryFixture struct {
	pool  *pgxpool.Pool
	repos *repository.Repos

	sessionSvc *services.SessionService
	paymentSvc *services.PaymentService
	cartSvc    *services.CartService
	publisher  *events.Publisher
	auditW     *audit.Writer

	a recoveryBranch // the acting tenant
	b recoveryBranch // a different organization entirely
}

func newRecoveryFixture(t *testing.T) *recoveryFixture {
	t.Helper()
	pool := testutil.OpenTestDB(t)
	redisClient := openHandlerTestRedis(t)
	repos := testutil.NewTestRepos(pool)

	metrics := observability.NewMetrics()
	pubsub := redisPkg.NewPubSub(redisClient, zerolog.Nop(), metrics)
	publisher := events.NewPublisher(pubsub, zerolog.Nop())
	// The event store is what makes published events observable from a test:
	// every publish appends a row to session_events before hitting Redis.
	publisher.SetEventStore(repos)

	sessionSvc := services.NewSessionService(repos, publisher, metrics, nil)
	paymentSvc := services.NewPaymentService(repos, publisher, metrics, sessionSvc, zerolog.Nop())
	paymentSvc.SetHostAuthority(sessionSvc)
	paymentSvc.SetPromoService(services.NewPromoService(repos))

	f := &recoveryFixture{
		pool:       pool,
		repos:      repos,
		sessionSvc: sessionSvc,
		paymentSvc: paymentSvc,
		cartSvc:    services.NewCartService(repos, publisher),
		publisher:  publisher,
		// AUDIT_LOG_V2_ENABLED is on: these routes are money-adjacent and the
		// audit entry is part of the contract under test.
		auditW: audit.NewWriter(sqlc.New(pool), true, zerolog.Nop(), nil),
	}

	f.a = f.seedBranch(t, "A")
	f.b = f.seedBranch(t, "B")

	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "organizations", "event_log", "audit_log")
	})
	return f
}

// seedBranch builds an independent organization (SeedFixtures generates a fresh
// slug per call) with one staff member per role.
func (f *recoveryFixture) seedBranch(t *testing.T, label string) recoveryBranch {
	t.Helper()
	seed := testutil.SeedFixtures(t, f.pool)
	br := recoveryBranch{
		orgID:    seed.OrganizationID,
		branchID: seed.BranchID,
		tableID:  seed.TableID,
		itemID:   seed.MenuItemID,
	}
	br.owner = f.seedStaff(t, br, sqlc.StaffRoleOwner, label)
	br.manager = f.seedStaff(t, br, sqlc.StaffRoleManager, label)
	br.waiter = f.seedStaff(t, br, sqlc.StaffRoleWaiter, label)
	br.kitchen = f.seedStaff(t, br, sqlc.StaffRoleKitchen, label)
	return br
}

func (f *recoveryFixture) seedStaff(t *testing.T, br recoveryBranch, role sqlc.StaffRole, label string) services.StaffSession {
	t.Helper()
	var id int64
	code := label + "-" + string(role) + "-" + uuid.NewString()[:8]
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO staff (branch_id, name, role, pin_hash, staff_code) VALUES ($1, $2, $3, 'x', $4) RETURNING id`,
		br.branchID, label+" "+string(role), role, code,
	).Scan(&id); err != nil {
		t.Fatalf("seed %s staff %s: %v", label, role, err)
	}
	return services.StaffSession{
		SessionID:      uuid.New(),
		StaffID:        id,
		BranchID:       br.branchID,
		OrganizationID: br.orgID,
		Role:           role,
		StaffCode:      code,
	}
}

// newTable adds a table to the branch. Sessions are unique-per-table while
// active, so every session in a test gets its own.
func (f *recoveryFixture) newTable(t *testing.T, br recoveryBranch) int64 {
	t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO tables (branch_id, identifier, capacity, qr_code_token, status)
		 VALUES ($1, $2, 4, $3, 'available') RETURNING id`,
		br.branchID, "T-"+uuid.NewString()[:8], uuid.NewString(),
	).Scan(&id); err != nil {
		t.Fatalf("seed table: %v", err)
	}
	return id
}

// openSession opens a live session on a fresh table of the branch.
func (f *recoveryFixture) openSession(t *testing.T, br recoveryBranch) (services.CreateSessionResult, int64) {
	t.Helper()
	tableID := f.newTable(t, br)
	result, err := f.sessionSvc.CreateSession(context.Background(), tableID, "Guest", "fp-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return result, tableID
}

// freezeSession opens a session and initiates a payment on it, leaving the
// session in payment_pending with one non-terminal payment — the stuck state
// both recovery routes exist to resolve.
//
// Method drives the resulting payment status: digital lands in provider_pending
// (the state payment 6 in qrdining_mtest is wedged in), cash in
// requires_staff_confirmation.
func (f *recoveryFixture) freezeSession(t *testing.T, br recoveryBranch, method sqlc.PaymentMethod) (services.CreateSessionResult, int64, sqlc.Payment) {
	t.Helper()
	result, tableID := f.openSession(t, br)
	payment, err := f.paymentSvc.InitiatePayment(context.Background(), services.InitiatePaymentRequest{
		SessionID:      result.Session.ID,
		BranchID:       br.branchID,
		Method:         method,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        result.Participant.ID,
		Bill: services.BillSnapshotInput{
			Subtotal:       50.00,
			Total:          50.00,
			Currency:       "INR",
			CreatedByActor: "guest:0",
		},
	})
	if err != nil {
		t.Fatalf("InitiatePayment: %v", err)
	}
	return result, tableID, payment
}

// ── assertion helpers ────────────────────────────────────────────────────────

func (f *recoveryFixture) sessionStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM sessions WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read session status: %v", err)
	}
	return status
}

func (f *recoveryFixture) paymentStatus(t *testing.T, id int64) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM payments WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read payment status: %v", err)
	}
	return status
}

// sessionHost returns sessions.host_participant_id, or 0 when unset.
func (f *recoveryFixture) sessionHost(t *testing.T, id uuid.UUID) int64 {
	t.Helper()
	var host *int64
	if err := f.pool.QueryRow(context.Background(), `SELECT host_participant_id FROM sessions WHERE id = $1`, id).Scan(&host); err != nil {
		t.Fatalf("read session host: %v", err)
	}
	if host == nil {
		return 0
	}
	return *host
}

func (f *recoveryFixture) tableStatus(t *testing.T, id int64) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM tables WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read table status: %v", err)
	}
	return status
}

// auditEntry is the subset of an audit_log row these tests assert on.
type auditEntry struct {
	ActorID  string
	ActorTyp string
	Result   string
	Reason   string
}

// auditEntries returns every audit row for one action against one resource id.
func (f *recoveryFixture) auditEntries(t *testing.T, action, resourceID string) []auditEntry {
	t.Helper()
	rows, err := f.pool.Query(context.Background(),
		`SELECT actor_id, actor_type::text, result::text, COALESCE(metadata_json->>'reason', '')
		 FROM audit_log WHERE action = $1 AND resource_id = $2 ORDER BY id`,
		action, resourceID)
	if err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	defer rows.Close()
	var out []auditEntry
	for rows.Next() {
		var e auditEntry
		if err := rows.Scan(&e.ActorID, &e.ActorTyp, &e.Result, &e.Reason); err != nil {
			t.Fatalf("scan audit_log: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit_log: %v", err)
	}
	return out
}

// publishedEvents returns the event names appended for a session, in order.
func (f *recoveryFixture) publishedEvents(t *testing.T, sessionID uuid.UUID) []string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(),
		`SELECT event FROM session_events WHERE session_id = $1 ORDER BY sequence`, sessionID)
	if err != nil {
		t.Fatalf("query session_events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatalf("scan session_events: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate session_events: %v", err)
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
