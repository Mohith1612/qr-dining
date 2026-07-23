package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

func newTestLoyaltyService(repos *repository.Repos) *services.LoyaltyService {
	return services.NewLoyaltyService(repos, newTestFeatureGate(repos), nil, zerolog.Nop(), observability.NewMetrics())
}

func seedLoyaltySession(t *testing.T, pool *pgxpool.Pool, f testutil.TestFixtures, customerID int64, number string) uuid.UUID {
	t.Helper()
	// One non-terminal session per table (idx_sessions_one_active_per_table):
	// close any prior session on the fixture table first.
	if _, err := pool.Exec(context.Background(),
		`UPDATE sessions SET status = 'closed', closed_at = NOW() WHERE table_id = $1 AND status NOT IN ('closed','abandoned','expired')`,
		f.TableID); err != nil {
		t.Fatalf("close prior sessions: %v", err)
	}
	var sessionID uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO sessions (branch_id, table_id, session_token, session_business_date, session_number, status, visit_number, customer_id)
		VALUES ($1, $2, $3, CURRENT_DATE, $4, 'active', 1, $5) RETURNING id`,
		f.BranchID, f.TableID, "tok-"+uuid.NewString(), number, customerID).Scan(&sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return sessionID
}

func seedCompletedPayment(t *testing.T, pool *pgxpool.Pool, f testutil.TestFixtures, sessionID uuid.UUID, amount string, ref string) sqlc.Payment {
	t.Helper()
	var paymentID int64
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO payments (session_id, branch_id, amount, method, status, currency,
		                      payment_reference, payment_business_date, payment_sequence, initiated_at, completed_at)
		VALUES ($1, $2, $3, 'upi', 'completed', 'INR', $4, CURRENT_DATE, 1, $5, $5) RETURNING id`,
		sessionID, f.BranchID, amount, ref, time.Now().UTC()).Scan(&paymentID); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	repos := repository.New(pool, zerolog.Nop())
	payment, err := repos.GetPaymentByID(context.Background(), paymentID)
	if err != nil {
		t.Fatalf("fetch payment: %v", err)
	}
	return payment
}

func TestLoyaltyEarnRedeemAdjust(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"customer_loyalty_transactions",
		"customer_loyalty_accounts",
		"organization_loyalty_programs",
		"organization_entitlement_overrides",
		"platform_flag_branch_overrides",
		"platform_flag_organization_overrides",
		"platform_flag_global_overrides",
	)
	f := testutil.SeedFixtures(t, pool)
	t.Cleanup(func() {
		// Leave the shared test DB the way later tests expect it (e.g. the
		// platform-analytics test asserts zero GMV): drop the sessions (and
		// cascaded payments/event_log) plus loyalty rows created here.
		testutil.TruncateTables(t, pool,
			"customer_loyalty_transactions",
			"customer_loyalty_accounts",
			"organization_loyalty_programs",
			"sessions",
			"event_log",
			"staff_sessions",
		)
	})
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	svc := newTestLoyaltyService(repos)

	staffID := seedStaff(t, pool, f.BranchID, "Mira", "manager")

	var customerID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (restaurant_id, phone_e164, display_name) VALUES ($1, '+919876543210', 'Asha') RETURNING id`,
		f.RestaurantID).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	sessionID := seedLoyaltySession(t, pool, f, customerID, "LO-S-001")
	payment1 := seedCompletedPayment(t, pool, f, sessionID, "250.00", "LO-PAY-001")

	// ── Gate off: accrual is a silent no-op, program API refuses ──
	svc.AccruePointsForPayment(ctx, payment1)
	if _, err := repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID); err == nil {
		t.Fatal("account created while gate off")
	}
	if _, err := svc.GetProgram(ctx, f.BranchID); !services.IsLoyaltyDisabled(err) {
		t.Fatalf("GetProgram gate off: err = %v, want ErrLoyaltyDisabled", err)
	}

	// ── Enable loyalty.enabled (+flag); program still missing → accrual no-op ──
	enableFeatureGate(t, pool, repos, f.OrganizationID, f.BranchID, "loyalty.enabled", "loyalty")
	svc.AccruePointsForPayment(ctx, payment1)
	if _, err := repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID); err == nil {
		t.Fatal("account created without an active program")
	}

	// ── Inactive program → still no accrual ──
	if _, err := svc.PutProgram(ctx, f.BranchID, false, 1, "100.00", staffID); err != nil {
		t.Fatalf("PutProgram(inactive): %v", err)
	}
	svc.AccruePointsForPayment(ctx, payment1)
	if _, err := repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID); err == nil {
		t.Fatal("account created while program inactive")
	}

	// ── Active program: 1 point per ₹100 → 250.00 earns 2 points ──
	program, err := svc.PutProgram(ctx, f.BranchID, true, 1, "100.00", staffID)
	if err != nil {
		t.Fatalf("PutProgram(active): %v", err)
	}
	if !program.IsActive || program.EarnRatePoints != 1 {
		t.Fatalf("program = %+v", program)
	}
	svc.AccruePointsForPayment(ctx, payment1)
	account, err := repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID)
	if err != nil {
		t.Fatalf("account after earn: %v", err)
	}
	if account.PointsBalance != 2 || account.LifetimePointsEarned != 2 || account.VisitCount != 1 {
		t.Fatalf("account = %+v, want balance 2, earned 2, visits 1", account)
	}

	// ── Idempotent replay: same payment accrues nothing more ──
	svc.AccruePointsForPayment(ctx, payment1)
	account, _ = repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID)
	if account.PointsBalance != 2 {
		t.Fatalf("replay changed balance: %+v", account)
	}
	txns, err := svc.ListTransactions(ctx, f.BranchID, account.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txns) != 1 || txns[0].Type != "earn" || txns[0].Points != 2 {
		t.Fatalf("transactions after replay = %+v, want exactly one earn of 2", txns)
	}

	// ── Second payment in the SAME session: points yes, new visit no ──
	payment2 := seedCompletedPayment(t, pool, f, sessionID, "120.00", "LO-PAY-002")
	svc.AccruePointsForPayment(ctx, payment2)
	account, _ = repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID)
	if account.PointsBalance != 3 || account.VisitCount != 1 {
		t.Fatalf("after second payment: %+v, want balance 3, visits still 1", account)
	}

	// ── New session: visit increments ──
	session2 := seedLoyaltySession(t, pool, f, customerID, "LO-S-002")
	payment3 := seedCompletedPayment(t, pool, f, session2, "400.00", "LO-PAY-003")
	svc.AccruePointsForPayment(ctx, payment3)
	account, _ = repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID)
	if account.PointsBalance != 7 || account.VisitCount != 2 {
		t.Fatalf("after third payment: %+v, want balance 7, visits 2", account)
	}

	// ── Lookup by phone ──
	lookup, err := svc.LookupCustomer(ctx, f.BranchID, "+919876543210")
	if err != nil {
		t.Fatalf("LookupCustomer: %v", err)
	}
	if lookup.Account == nil || lookup.Account.PointsBalance != 7 {
		t.Fatalf("lookup = %+v, want account with 7 points", lookup)
	}

	// ── Redeem requires loyalty.redeem ──
	if _, err := svc.Redeem(ctx, f.BranchID, account.ID, 2, "counter discount", staffID); !services.IsLoyaltyDisabled(err) {
		t.Fatalf("redeem without capability: err = %v, want ErrLoyaltyDisabled", err)
	}
	enableFeatureGate(t, pool, repos, f.OrganizationID, f.BranchID, "loyalty.redeem", "loyalty")
	redeemed, err := svc.Redeem(ctx, f.BranchID, account.ID, 2, "counter discount", staffID)
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if redeemed.PointsBalance != 5 || redeemed.LifetimePointsRedeemed != 2 {
		t.Fatalf("after redeem: %+v, want balance 5, redeemed 2", redeemed)
	}

	// ── Insufficient points ──
	if _, err := svc.Redeem(ctx, f.BranchID, account.ID, 100, "too much", staffID); !services.IsLoyaltyInsufficientPoints(err) {
		t.Fatalf("over-redeem: err = %v, want ErrLoyaltyInsufficientPoints", err)
	}

	// ── Concurrent redeems never overdraw (balance 5, two redeems of 4) ──
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Redeem(ctx, f.BranchID, account.ID, 4, "concurrent", staffID)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	var successes, insufficient int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case services.IsLoyaltyInsufficientPoints(err):
			insufficient++
		default:
			t.Fatalf("concurrent redeem unexpected error: %v", err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent redeems: %d succeeded / %d insufficient, want 1/1", successes, insufficient)
	}
	account, _ = repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID)
	if account.PointsBalance != 1 {
		t.Fatalf("balance after concurrent redeems = %d, want 1", account.PointsBalance)
	}

	// ── Adjust requires loyalty.manual_adjustment; floors at zero ──
	if _, err := svc.Adjust(ctx, f.BranchID, account.ID, 5, "correction", staffID); !services.IsLoyaltyDisabled(err) {
		t.Fatalf("adjust without capability: err = %v, want ErrLoyaltyDisabled", err)
	}
	enableFeatureGate(t, pool, repos, f.OrganizationID, f.BranchID, "loyalty.manual_adjustment", "loyalty")
	adjusted, err := svc.Adjust(ctx, f.BranchID, account.ID, 5, "correction", staffID)
	if err != nil {
		t.Fatalf("Adjust(+5): %v", err)
	}
	if adjusted.PointsBalance != 6 {
		t.Fatalf("after +5 adjust: %+v, want balance 6", adjusted)
	}
	if _, err := svc.Adjust(ctx, f.BranchID, account.ID, -100, "bad correction", staffID); !services.IsLoyaltyInsufficientPoints(err) {
		t.Fatalf("floor adjust: err = %v, want ErrLoyaltyInsufficientPoints", err)
	}

	// ── Analytics ──
	report, err := svc.GetLoyaltyAnalytics(ctx, f.BranchID, "weekly")
	if err != nil {
		t.Fatalf("GetLoyaltyAnalytics: %v", err)
	}
	if report.PointsIssued != 7 {
		t.Errorf("PointsIssued = %d, want 7", report.PointsIssued)
	}
	if report.PointsRedeemed != 6 { // 2 + 4
		t.Errorf("PointsRedeemed = %d, want 6", report.PointsRedeemed)
	}
	if report.TotalAccounts != 1 || report.ActiveCustomers != 1 {
		t.Errorf("accounts = %d / active = %d, want 1/1", report.TotalAccounts, report.ActiveCustomers)
	}
	if report.LoyaltySessions != 2 || report.TotalSessions < 2 {
		t.Errorf("participation = %d/%d, want 2 loyalty sessions of >=2", report.LoyaltySessions, report.TotalSessions)
	}
	if len(report.TopCustomers) != 1 || report.TopCustomers[0].CustomerID != customerID {
		t.Errorf("top customers = %+v", report.TopCustomers)
	}
}

func TestLoyaltySkipsAnonymousSessions(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"customer_loyalty_transactions",
		"customer_loyalty_accounts",
		"organization_loyalty_programs",
		"organization_entitlement_overrides",
		"platform_flag_branch_overrides",
	)
	f := testutil.SeedFixtures(t, pool)
	t.Cleanup(func() {
		// Leave the shared test DB the way later tests expect it (e.g. the
		// platform-analytics test asserts zero GMV): drop the sessions (and
		// cascaded payments/event_log) plus loyalty rows created here.
		testutil.TruncateTables(t, pool,
			"customer_loyalty_transactions",
			"customer_loyalty_accounts",
			"organization_loyalty_programs",
			"sessions",
			"event_log",
			"staff_sessions",
		)
	})
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	svc := newTestLoyaltyService(repos)
	staffID := seedStaff(t, pool, f.BranchID, "Omar", "owner")

	enableFeatureGate(t, pool, repos, f.OrganizationID, f.BranchID, "loyalty.enabled", "loyalty")
	if _, err := svc.PutProgram(ctx, f.BranchID, true, 1, "100.00", staffID); err != nil {
		t.Fatalf("PutProgram: %v", err)
	}

	// Session with NO linked customer → accrual silently skips.
	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO sessions (branch_id, table_id, session_token, session_business_date, session_number, status, visit_number)
		VALUES ($1, $2, $3, CURRENT_DATE, 'AN-S-001', 'active', 1) RETURNING id`,
		f.BranchID, f.TableID, "tok-"+uuid.NewString()).Scan(&sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	payment := seedCompletedPayment(t, pool, f, sessionID, "500.00", "AN-PAY-001")
	svc.AccruePointsForPayment(ctx, payment)

	var count int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM customer_loyalty_transactions`).Scan(&count); err != nil {
		t.Fatalf("count txns: %v", err)
	}
	if count != 0 {
		t.Fatalf("anonymous session produced %d loyalty transactions, want 0", count)
	}
}
