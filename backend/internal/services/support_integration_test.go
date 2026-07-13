package services_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestSupportSessionAndPaymentDetail(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	// Clear any prior session graph so fixed references don't collide on re-run.
	testutil.TruncateTables(t, pool, "sessions")
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	svc := services.NewSupportService(repos)

	secretToken := "guest-secret-" + uuid.NewString()

	// ── Seed a session graph (raw SQL; deferred host FK handled via UPDATE) ──
	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO sessions (branch_id, table_id, session_token, session_business_date, session_number, status, visit_number)
		VALUES ($1, $2, $3, CURRENT_DATE, 'TEST-S-001', 'active', 1) RETURNING id`,
		f.BranchID, f.TableID, secretToken).Scan(&sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	var participantID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO session_participants (session_id, display_name, phone_e164, device_fingerprint, is_host)
		VALUES ($1, 'Aanya', '+919999999999', 'fp-secret-xyz', TRUE) RETURNING id`,
		sessionID).Scan(&participantID); err != nil {
		t.Fatalf("seed participant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sessions SET host_participant_id = $1 WHERE id = $2`, participantID, sessionID); err != nil {
		t.Fatalf("set host: %v", err)
	}
	var orderID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (session_id, branch_id, idempotency_key, total_amount, status,
		                    order_operational_id, order_number_display, order_business_date)
		VALUES ($1, $2, $3, 250.00, 'served', 'TEST-O-001', 'OR-1', CURRENT_DATE) RETURNING id`,
		sessionID, f.BranchID, uuid.NewString()).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price)
		VALUES ($1, $2, 2, 125.00)`, orderID, f.MenuItemID); err != nil {
		t.Fatalf("seed order item: %v", err)
	}
	var billID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO bill_snapshots (session_id, branch_id, subtotal, total, created_by_actor)
		VALUES ($1, $2, 250.00, 250.00, 'host') RETURNING id`,
		sessionID, f.BranchID).Scan(&billID); err != nil {
		t.Fatalf("seed bill snapshot: %v", err)
	}
	var paymentID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO payments (session_id, branch_id, amount, method, status, currency,
		                      payment_reference, payment_business_date, payment_sequence, bill_snapshot_id)
		VALUES ($1, $2, 250.00, 'upi', 'completed', 'INR', 'TEST-PAY-001', CURRENT_DATE, 1, $3) RETURNING id`,
		sessionID, f.BranchID, billID).Scan(&paymentID); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment_webhook_events (external_event_id, provider, event_type, payload, processed, payment_id, error_message)
		VALUES ($1, 'razorpay', 'payment.captured', '{}'::jsonb, TRUE, $2, NULL)`,
		"evt-"+uuid.NewString(), paymentID); err != nil {
		t.Fatalf("seed webhook: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO assistance_requests (session_id, table_id, type, status)
		VALUES ($1, $2, 'waiter', 'pending')`, sessionID, f.TableID); err != nil {
		t.Fatalf("seed assistance: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event_log (session_id, branch_id, event_type, actor_type, payload)
		VALUES ($1, $2, 'SESSION_CREATED', 'system', '{}'::jsonb)`, sessionID, f.BranchID); err != nil {
		t.Fatalf("seed event_log: %v", err)
	}

	// ── Session detail aggregate ──
	detail, err := svc.GetSessionDetail(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionDetail: %v", err)
	}
	if detail.Session.SessionNumber != "TEST-S-001" || detail.Session.Status != "active" {
		t.Fatalf("session = %+v", detail.Session)
	}
	if detail.Organization.ID != f.OrganizationID || detail.Branch.ID != f.BranchID {
		t.Fatalf("org/branch context wrong: %+v %+v", detail.Organization, detail.Branch)
	}
	if len(detail.Participants) != 1 || detail.Participants[0].Phone != "+919999999999" || !detail.Participants[0].IsHost {
		t.Fatalf("participants = %+v", detail.Participants)
	}
	if len(detail.Orders) != 1 || len(detail.Orders[0].Items) != 1 || detail.Orders[0].Items[0].Quantity != 2 {
		t.Fatalf("orders = %+v", detail.Orders)
	}
	if detail.Orders[0].TotalAmount != "250.00" {
		t.Errorf("order total = %q, want 250.00", detail.Orders[0].TotalAmount)
	}
	if len(detail.Payments) != 1 || detail.Payments[0].Status != "completed" {
		t.Fatalf("payments = %+v", detail.Payments)
	}
	if len(detail.Assistance) != 1 || len(detail.Timeline) < 1 {
		t.Fatalf("assistance=%d timeline=%d", len(detail.Assistance), len(detail.Timeline))
	}

	// ── Credential / PII-leak guard: the response must NOT contain the guest
	//    token or device fingerprint anywhere. ──
	raw, _ := json.Marshal(detail)
	js := string(raw)
	for _, forbidden := range []string{secretToken, "session_token", "device_fingerprint", "fp-secret-xyz"} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("session detail JSON leaked %q", forbidden)
		}
	}

	// ── Payment detail aggregate ──
	pd, err := svc.GetPaymentDetail(ctx, paymentID)
	if err != nil {
		t.Fatalf("GetPaymentDetail: %v", err)
	}
	if pd.Payment.PaymentReference != "TEST-PAY-001" || pd.BillSnapshot == nil || pd.BillSnapshot.Total != "250.00" {
		t.Fatalf("payment detail = %+v / bill %+v", pd.Payment, pd.BillSnapshot)
	}
	if len(pd.Webhooks) != 1 || pd.Webhooks[0].Provider != "razorpay" || !pd.Webhooks[0].Processed {
		t.Fatalf("webhooks = %+v", pd.Webhooks)
	}

	// ── Search now covers tables + participants ──
	tableRows, err := repos.SearchSupportReferences(ctx, "T1", 20)
	if err != nil {
		t.Fatalf("search tables: %v", err)
	}
	if !hasType(tableRows, "table") {
		t.Errorf("search 'T1' returned no table result: %+v", tableRows)
	}
	partRows, err := repos.SearchSupportReferences(ctx, "Aanya", 20)
	if err != nil {
		t.Fatalf("search participants: %v", err)
	}
	if !hasType(partRows, "participant") {
		t.Errorf("search 'Aanya' returned no participant result: %+v", partRows)
	}
}

func hasType(rows []repository.SupportSearchResult, typ string) bool {
	for _, r := range rows {
		if r.Type == typ {
			return true
		}
	}
	return false
}
