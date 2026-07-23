package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TestSettlePaymentTriggersLoyaltyEarn exercises the real wiring: staff
// settlement completes the payment AND credits loyalty points — and when the
// loyalty gate is off, settlement still succeeds with no loyalty side effects.
func TestSettlePaymentTriggersLoyaltyEarn(t *testing.T) {
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
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, observability.NewMetrics(), nil)
	paymentSvc := services.NewPaymentService(repos, pub, observability.NewMetrics(), sessionSvc, zerolog.Nop())
	paymentSvc.SetHostAuthority(sessionSvc)
	loyaltySvc := newTestLoyaltyService(repos)
	paymentSvc.SetLoyaltyAccrual(loyaltySvc)

	staffID := seedStaff(t, pool, f.BranchID, "Sana", "waiter")
	var customerID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (restaurant_id, phone_e164, display_name) VALUES ($1, '+919812345678', 'Dev') RETURNING id`,
		f.RestaurantID).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	seedSettleablePayment := func(number, ref string) int64 {
		t.Helper()
		// One non-terminal session per table: close any prior session first.
		if _, err := pool.Exec(ctx,
			`UPDATE sessions SET status = 'closed', closed_at = NOW() WHERE table_id = $1 AND status NOT IN ('closed','abandoned','expired')`,
			f.TableID); err != nil {
			t.Fatalf("close prior sessions: %v", err)
		}
		var sessionID uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO sessions (branch_id, table_id, session_token, session_business_date, session_number, status, visit_number, customer_id)
			VALUES ($1, $2, $3, CURRENT_DATE, $4, 'payment_pending', 1, $5) RETURNING id`,
			f.BranchID, f.TableID, "tok-"+uuid.NewString(), number, customerID).Scan(&sessionID); err != nil {
			t.Fatalf("seed session: %v", err)
		}
		var paymentID int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO payments (session_id, branch_id, amount, method, status, currency,
			                      payment_reference, payment_business_date, payment_sequence, initiated_at)
			VALUES ($1, $2, 350.00, 'cash', 'requires_staff_confirmation', 'INR', $3, CURRENT_DATE, 1, $4) RETURNING id`,
			sessionID, f.BranchID, ref, time.Now().UTC()).Scan(&paymentID); err != nil {
			t.Fatalf("seed payment: %v", err)
		}
		return paymentID
	}

	// ── Gate OFF: settlement works, zero loyalty side effects ──
	payment1 := seedSettleablePayment("PL-S-001", "PL-PAY-001")
	settled, err := paymentSvc.SettlePaymentByStaff(ctx, payment1, staffID, f.BranchID)
	if err != nil {
		t.Fatalf("settle with loyalty off: %v", err)
	}
	if string(settled.Status) != "completed" {
		t.Fatalf("payment status = %s, want completed", settled.Status)
	}
	var txnCount int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM customer_loyalty_transactions`).Scan(&txnCount); err != nil {
		t.Fatal(err)
	}
	if txnCount != 0 {
		t.Fatalf("loyalty transactions with gate off = %d, want 0", txnCount)
	}

	// ── Gate ON + active program: settlement credits points ──
	enableFeatureGate(t, pool, repos, f.OrganizationID, f.BranchID, "loyalty.enabled", "loyalty")
	if _, err := loyaltySvc.PutProgram(ctx, f.BranchID, true, 1, "100.00", staffID); err != nil {
		t.Fatalf("PutProgram: %v", err)
	}
	payment2 := seedSettleablePayment("PL-S-002", "PL-PAY-002")
	settled, err = paymentSvc.SettlePaymentByStaff(ctx, payment2, staffID, f.BranchID)
	if err != nil {
		t.Fatalf("settle with loyalty on: %v", err)
	}
	if string(settled.Status) != "completed" {
		t.Fatalf("payment status = %s, want completed", settled.Status)
	}
	account, err := repos.GetLoyaltyAccountByCustomer(ctx, customerID, f.OrganizationID)
	if err != nil {
		t.Fatalf("loyalty account after settle: %v", err)
	}
	if account.PointsBalance != 3 { // floor(350/100)
		t.Fatalf("points after settle = %d, want 3", account.PointsBalance)
	}
	if !settled.SettledByStaffID.Valid || settled.SettledByStaffID.Int64 != staffID {
		t.Fatalf("settled_by_staff_id = %+v, want %d", settled.SettledByStaffID, staffID)
	}
}
