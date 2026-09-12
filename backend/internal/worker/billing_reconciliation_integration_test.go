//go:build integration

package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/Mohith1612/qr-dining/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type billingFixture struct {
	pool       *pgxpool.Pool
	worker     *worker.Worker
	metrics    *observability.Metrics
	sessionID  uuid.UUID
	orderID    uuid.UUID
	snapshotID int64
	paymentID  int64
	branchID   int64
}

type billingTestQuerier struct {
	worker.Querier
	repos *repository.Repos
}

func (q *billingTestQuerier) ListBillingReconciliationDiscrepancies(ctx context.Context, windowStart, windowEnd time.Time) ([]worker.BillingReconciliationDiscrepancy, error) {
	return q.repos.ListBillingReconciliationDiscrepancies(ctx, windowStart, windowEnd)
}

func newBillingFixture(t *testing.T) billingFixture {
	t.Helper()
	pool := testutil.OpenTestDB(t)
	seed := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	metrics := observability.NewMetrics()
	// A1 discrepancy audits are required even while the general audit rollout
	// flag is off; the detector must never emit only an anonymous metric.
	auditWriter := audit.NewWriter(sqlc.New(pool), false, zerolog.Nop(), metrics.AuditWriteFailuresTotal)
	w := worker.New(pool, &billingTestQuerier{repos: repos}, nil, nil, nil, metrics, auditWriter, "billing-test", zerolog.Nop())
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "audit_log", "sessions", "orders", "payments", "bill_snapshots")
	})

	ctx := context.Background()
	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO sessions (
			branch_id, table_id, session_token, status, created_at, closed_at,
			session_business_date, visit_number, session_number
		) VALUES (
			$1, $2, $3, 'closed', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '2 minutes',
			CURRENT_DATE, 1, $4
		) RETURNING id`,
		seed.BranchID, seed.TableID, "billing-session-"+uuid.NewString(), "billing-session-"+uuid.NewString(),
	).Scan(&sessionID); err != nil {
		t.Fatalf("insert settled session: %v", err)
	}

	var orderID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (
			session_id, branch_id, status, idempotency_key, total_amount,
			order_business_date, order_number_display, order_operational_id, created_at, updated_at
		) VALUES (
			$1, $2, 'served', $3, 150.00,
			CURRENT_DATE, $4, $5, NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '10 minutes'
		) RETURNING id`,
		sessionID, seed.BranchID, uuid.NewString(), "1001", "order-"+uuid.NewString(),
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price, selected_modifiers_json)
		VALUES ($1, $2, 1, 150.00, '[]')`, orderID, seed.MenuItemID); err != nil {
		t.Fatalf("insert order item: %v", err)
	}

	var snapshotID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO bill_snapshots (
			session_id, branch_id, subtotal, discount_amount, tax_amount,
			service_charge, tip_amount, total, currency, source_order_ids,
			created_by_actor, created_at
		) VALUES (
			$1, $2, 150.00, 0, 0,
			0, 0, 150.00, 'INR', jsonb_build_array($3::text),
			'system:test', NOW() - INTERVAL '5 minutes'
		) RETURNING id`, sessionID, seed.BranchID, orderID,
	).Scan(&snapshotID); err != nil {
		t.Fatalf("insert bill snapshot: %v", err)
	}
	paymentID := insertCompletedPayment(t, pool, sessionID, seed.BranchID, snapshotID, "150.00")
	insertCancelledPayment(t, pool, sessionID, seed.BranchID, snapshotID, "150.00")

	return billingFixture{
		pool:       pool,
		worker:     w,
		metrics:    metrics,
		sessionID:  sessionID,
		orderID:    orderID,
		snapshotID: snapshotID,
		paymentID:  paymentID,
		branchID:   seed.BranchID,
	}
}

func insertCompletedPayment(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, branchID, snapshotID int64, amount string) int64 {
	t.Helper()
	var paymentID int64
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO payments (
			session_id, amount, method, status, bill_snapshot_id, branch_id,
			currency, initiated_at, completed_at, payment_business_date,
			payment_sequence, payment_reference
		) VALUES (
			$1, $2::numeric, 'cash', 'completed', $3, $4,
			'INR', NOW() - INTERVAL '4 minutes', NOW() - INTERVAL '3 minutes', CURRENT_DATE,
			1, $5
		) RETURNING id`,
		sessionID, amount, snapshotID, branchID, "PAY-"+uuid.NewString(),
	).Scan(&paymentID); err != nil {
		t.Fatalf("insert completed payment: %v", err)
	}
	return paymentID
}

func insertCancelledPayment(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, branchID, snapshotID int64, amount string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payments (
			session_id, amount, method, status, bill_snapshot_id, branch_id,
			currency, initiated_at, payment_business_date, payment_sequence, payment_reference
		) VALUES (
			$1, $2::numeric, 'cash', 'cancelled', $3, $4,
			'INR', NOW() - INTERVAL '6 minutes', CURRENT_DATE, 2, $5
		)`, sessionID, amount, snapshotID, branchID, "PAY-"+uuid.NewString()); err != nil {
		t.Fatalf("insert cancelled payment: %v", err)
	}
}

func TestBillingReconciliation_CorrectlySettledSessionHasNoDiscrepancy(t *testing.T) {
	f := newBillingFixture(t)

	findings, err := f.worker.ReconcileBilling(context.Background(), time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
}

func TestBillingReconciliation_CollectedExceedsSnapshot(t *testing.T) {
	f := newBillingFixture(t)
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE payments SET amount = 175.00 WHERE session_id = $1 AND status = 'completed'`,
		f.sessionID,
	); err != nil {
		t.Fatalf("increase collected amount: %v", err)
	}

	findings, err := f.worker.ReconcileBilling(context.Background(), time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	finding := requireBillingFinding(t, findings, "collected_vs_snapshot")
	if finding.ExpectedAmount != "150.00" || finding.ActualAmount != "175.00" || finding.Difference != "25.00" {
		t.Fatalf("finding amounts = expected %s, actual %s, difference %s; want 150.00, 175.00, 25.00",
			finding.ExpectedAmount, finding.ActualAmount, finding.Difference)
	}
	if got := gaugeValue(t, f.metrics.Registry, "billing_reconciliation_discrepancies", "comparison", "collected_vs_snapshot"); got != 1 {
		t.Fatalf("collected_vs_snapshot gauge = %v, want 1", got)
	}

	var comparison, expected, actual, difference, riskLevel string
	if err := f.pool.QueryRow(context.Background(), `
		SELECT
			metadata_json->>'comparison',
			metadata_json->>'expected_amount',
			metadata_json->>'actual_amount',
			metadata_json->>'difference',
			risk_level::text
		FROM audit_log
		WHERE session_id = $1 AND action = 'billing.reconciliation.discrepancy'`,
		f.sessionID,
	).Scan(&comparison, &expected, &actual, &difference, &riskLevel); err != nil {
		t.Fatalf("read reconciliation audit: %v", err)
	}
	if comparison != "collected_vs_snapshot" || expected != "150.00" || actual != "175.00" || difference != "25.00" || riskLevel != "critical" {
		t.Fatalf("audit = comparison %q, expected %q, actual %q, difference %q, risk %q",
			comparison, expected, actual, difference, riskLevel)
	}

	if _, err := f.worker.ReconcileBilling(context.Background(), time.Now().Add(-24*time.Hour), time.Now()); err != nil {
		t.Fatalf("second ReconcileBilling: %v", err)
	}
	var auditCount int
	if err := f.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM audit_log
		WHERE session_id = $1 AND action = 'billing.reconciliation.discrepancy'`,
		f.sessionID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count reconciliation audits: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("reconciliation audit count after unchanged rescan = %d, want 1", auditCount)
	}
}

func gaugeValue(t *testing.T, gatherer prometheus.Gatherer, metricName, labelName, labelValue string) float64 {
	t.Helper()
	families, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != metricName {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == labelName && label.GetValue() == labelValue {
					return metric.GetGauge().GetValue()
				}
			}
		}
	}
	t.Fatalf("metric %s{%s=%q} not found", metricName, labelName, labelValue)
	return 0
}

func TestBillingReconciliation_SnapshotDisagreesWithOrderLines(t *testing.T) {
	f := newBillingFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE bill_snapshots SET subtotal = 140.00, total = 140.00 WHERE id = $1`, f.snapshotID); err != nil {
		t.Fatalf("corrupt bill snapshot: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE payments SET amount = 140.00 WHERE session_id = $1 AND status = 'completed'`, f.sessionID); err != nil {
		t.Fatalf("match payment to corrupted snapshot: %v", err)
	}

	findings, err := f.worker.ReconcileBilling(ctx, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	finding := requireBillingFinding(t, findings, "snapshot_vs_orders")
	if finding.ExpectedAmount != "150.00" || finding.ActualAmount != "140.00" || finding.Difference != "-10.00" {
		t.Fatalf("finding amounts = expected %s, actual %s, difference %s; want 150.00, 140.00, -10.00",
			finding.ExpectedAmount, finding.ActualAmount, finding.Difference)
	}
}

func TestBillingReconciliation_PostSnapshotOrderCancellationDoesNotRewriteBill(t *testing.T) {
	f := newBillingFixture(t)
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = $1`,
		f.orderID,
	); err != nil {
		t.Fatalf("cancel order after settlement: %v", err)
	}

	findings, err := f.worker.ReconcileBilling(context.Background(), time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("post-snapshot cancellation findings = %+v, want none", findings)
	}
}

func TestBillingReconciliation_ForceClosedUnpaidBillIsIgnored(t *testing.T) {
	f := newBillingFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE payments SET amount = 50.00 WHERE session_id = $1 AND status = 'completed'`, f.sessionID); err != nil {
		t.Fatalf("record partial collection before force-close: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO event_log (session_id, branch_id, event_type, actor_type, actor_id, payload)
		VALUES ($1, $2, 'SESSION_CLOSED', 'staff', '42', '{"reason":"guest left"}')`,
		f.sessionID, f.branchID,
	); err != nil {
		t.Fatalf("record force-close marker: %v", err)
	}

	findings, err := f.worker.ReconcileBilling(ctx, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("force-closed write-off findings = %+v, want none", findings)
	}
}

func TestBillingReconciliation_MidPaymentSessionIsIgnored(t *testing.T) {
	f := newBillingFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE sessions SET status = 'payment_pending', closed_at = NULL WHERE id = $1`, f.sessionID); err != nil {
		t.Fatalf("reopen session as payment_pending: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE payments SET amount = 50.00 WHERE session_id = $1 AND status = 'completed'`, f.sessionID); err != nil {
		t.Fatalf("record partial completed collection: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO payments (
			session_id, amount, method, status, bill_snapshot_id, branch_id,
			currency, initiated_at, payment_business_date, payment_sequence, payment_reference
		) VALUES (
			$1, 100.00, 'cash', 'requires_staff_confirmation', $2, $3,
			'INR', NOW() - INTERVAL '30 seconds', CURRENT_DATE, 2, $4
		)`, f.sessionID, f.snapshotID, f.branchID, "PAY-"+uuid.NewString()); err != nil {
		t.Fatalf("insert in-flight payment: %v", err)
	}

	findings, err := f.worker.ReconcileBilling(ctx, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("mid-payment findings = %+v, want none", findings)
	}
}

func TestBillingReconciliation_AppliedPromoReconcilesCleanly(t *testing.T) {
	f := newBillingFixture(t)
	ctx := context.Background()
	var promoID int64
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO promos (
			branch_id, code, type, value, min_order_amount, uses_per_phone,
			valid_from, valid_until, redeemed_count
		) VALUES (
			$1, $2, 'flat_amount', 10.00, 0, 1,
			NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 hour', 1
		) RETURNING id`, f.branchID, "PROMO-"+uuid.NewString(),
	).Scan(&promoID); err != nil {
		t.Fatalf("insert promo: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO promo_redemptions (promo_id, payment_id, phone_e164)
		VALUES ($1, $2, '+919876543210')`, promoID, f.paymentID); err != nil {
		t.Fatalf("insert payment promo redemption: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `
		UPDATE order_items
		SET unit_price = 140.00,
			selected_modifiers_json = '[{"name":"Extra","price_delta":10.00}]'
		WHERE order_id = $1`, f.orderID); err != nil {
		t.Fatalf("add snapshotted modifier: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `
		UPDATE bill_snapshots
		SET discount_amount = 10.00, tax_amount = 15.00,
			service_charge = 7.50, total = 162.50
		WHERE id = $1`, f.snapshotID); err != nil {
		t.Fatalf("apply promo to snapshot: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE payments SET amount = 162.50 WHERE id = $1`, f.paymentID); err != nil {
		t.Fatalf("match collected amount to discounted snapshot: %v", err)
	}

	findings, err := f.worker.ReconcileBilling(ctx, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("promo settlement findings = %+v, want none", findings)
	}
	for _, comparison := range []string{"snapshot_vs_orders", "collected_vs_snapshot"} {
		if got := gaugeValue(t, f.metrics.Registry, "billing_reconciliation_discrepancies", "comparison", comparison); got != 0 {
			t.Fatalf("billing reconciliation gauge %q = %v, want 0", comparison, got)
		}
	}
}

func TestBillingReconciliation_HistoricalTwoFullPaymentsCollects300For150Bill(t *testing.T) {
	f := newBillingFixture(t)
	ctx := context.Background()

	var secondSnapshotID int64
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO bill_snapshots (
			session_id, branch_id, subtotal, discount_amount, tax_amount,
			service_charge, tip_amount, total, currency, source_order_ids,
			created_by_actor, created_at
		) VALUES (
			$1, $2, 150.00, 0, 0,
			0, 0, 150.00, 'INR', jsonb_build_array($3::text),
			'participant:2', NOW() - INTERVAL '4 minutes'
		) RETURNING id`, f.sessionID, f.branchID, f.orderID,
	).Scan(&secondSnapshotID); err != nil {
		t.Fatalf("insert second historical bill snapshot: %v", err)
	}
	insertCompletedPayment(t, f.pool, f.sessionID, f.branchID, secondSnapshotID, "150.00")

	findings, err := f.worker.ReconcileBilling(ctx, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("ReconcileBilling: %v", err)
	}
	finding := requireBillingFinding(t, findings, "collected_vs_snapshot")
	if finding.ExpectedAmount != "150.00" || finding.ActualAmount != "300.00" || finding.Difference != "150.00" {
		t.Fatalf("historical double collection = expected %s, actual %s, difference %s; want 150.00, 300.00, 150.00",
			finding.ExpectedAmount, finding.ActualAmount, finding.Difference)
	}
}

func requireBillingFinding(t *testing.T, findings []worker.BillingReconciliationDiscrepancy, comparison string) worker.BillingReconciliationDiscrepancy {
	t.Helper()
	for _, finding := range findings {
		if finding.Comparison == comparison {
			return finding
		}
	}
	t.Fatalf("no %s finding in %+v", comparison, findings)
	return worker.BillingReconciliationDiscrepancy{}
}
