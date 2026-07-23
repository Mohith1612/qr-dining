//go:build integration

package services_test

import (
	"context"
	"testing"

	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
)

// Promo is applied at payment initiation now. The handler folds the discount
// into the bill; the service records the redemption against the payment under a
// row lock. These tests drive the service directly with a pre-folded bill.
func TestPaymentPromoRedemption(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	ctx := context.Background()
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "bill_snapshots", "promo_redemptions", "promos", "idempotency_keys")
	})

	// 10%-off promo, capped at 1 use per phone.
	var promoID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO promos (branch_id, code, type, value, uses_per_phone, valid_from, valid_until, is_active)
		VALUES ($1, 'SAVE10', 'percentage', 10, 1, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 day', TRUE) RETURNING id`,
		f.BranchID).Scan(&promoID); err != nil {
		t.Fatalf("seed promo: %v", err)
	}

	phone := "+919876543210"
	// freshSession closes any prior session on the table and creates a new one.
	freshSession := func() (uuid.UUID, int64) {
		if _, err := pool.Exec(ctx, `UPDATE sessions SET status = 'closed', closed_at = NOW() WHERE table_id = $1 AND status NOT IN ('closed','abandoned','expired')`, f.TableID); err != nil {
			t.Fatalf("close prior session: %v", err)
		}
		sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Asha", "fp-"+uuid.NewString(), "")
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		return sess.Session.ID, sess.Participant.ID
	}
	initiate := func(sessionID uuid.UUID, participantID int64, idemKey string) (sqlc.Payment, error) {
		// Bill of 100 → 10% discount = 10 → total 90 (handler-folded).
		return paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
			SessionID:      sessionID,
			BranchID:       f.BranchID,
			Method:         sqlc.PaymentMethodCash,
			IdempotencyKey: idemKey,
			ActorType:      "participant",
			ActorID:        participantID,
			Bill: services.BillSnapshotInput{
				Subtotal:       100.00,
				DiscountAmount: 10.00,
				Total:          90.00,
				Currency:       "INR",
				CreatedByActor: "guest:0",
			},
			PromoCode:  ptr("SAVE10"),
			PromoPhone: ptr(phone),
		})
	}

	// ── First redemption succeeds; discount lands in the snapshot ──
	sessA, partA := freshSession()
	key1 := uuid.NewString()
	payment, err := initiate(sessA, partA, key1)
	if err != nil {
		t.Fatalf("InitiatePayment with promo: %v", err)
	}
	amt, _ := payment.Amount.Float64Value()
	if amt.Float64 != 90.00 {
		t.Errorf("payment amount = %.2f, want 90.00 (discounted)", amt.Float64)
	}
	var redemptionCount, snapDiscountCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM promo_redemptions WHERE promo_id = $1 AND payment_id = $2`, promoID, payment.ID).Scan(&redemptionCount); err != nil {
		t.Fatal(err)
	}
	if redemptionCount != 1 {
		t.Errorf("redemptions for payment = %d, want 1", redemptionCount)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM bill_snapshots WHERE discount_amount = 10.00`).Scan(&snapDiscountCount); err != nil {
		t.Fatal(err)
	}
	if snapDiscountCount != 1 {
		t.Errorf("bill snapshots with discount 10.00 = %d, want 1", snapDiscountCount)
	}

	// ── Idempotent replay (same session + key) returns the same payment ──
	replay, err := initiate(sessA, partA, key1)
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if replay.ID != payment.ID {
		t.Errorf("replay payment id = %d, want %d", replay.ID, payment.ID)
	}

	// ── Per-phone cap (1 use) blocks a second, fresh redemption with same phone ──
	sessB, partB := freshSession()
	if _, err := initiate(sessB, partB, uuid.NewString()); !errors.Is(err, domain.ErrPromoAlreadyUsed) {
		t.Fatalf("second redemption err = %v, want ErrPromoAlreadyUsed", err)
	}
}

// TestPaymentPromoMissingPhoneRejected: a per-phone-limited promo refuses to
// apply without a phone (the cap would otherwise be unenforceable).
func TestPaymentPromoMissingPhoneRejected(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	paymentSvc := newTestPaymentService(repos, pub, sessionSvc)
	ctx := context.Background()
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "payments", "bill_snapshots", "promo_redemptions", "promos", "idempotency_keys")
	})

	if _, err := pool.Exec(ctx, `
		INSERT INTO promos (branch_id, code, type, value, uses_per_phone, valid_from, valid_until, is_active)
		VALUES ($1, 'PHONE1', 'percentage', 10, 1, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 day', TRUE)`,
		f.BranchID); err != nil {
		t.Fatalf("seed promo: %v", err)
	}

	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Asha", "fp-asha", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, err = paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:      sess.Session.ID,
		BranchID:       f.BranchID,
		Method:         sqlc.PaymentMethodCash,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        sess.Participant.ID,
		Bill:           services.BillSnapshotInput{Subtotal: 100, DiscountAmount: 10, Total: 90, Currency: "INR", CreatedByActor: "guest:0"},
		PromoCode:      ptr("PHONE1"),
		// no phone
	})
	if !errors.Is(err, domain.ErrPromoPhoneRequired) {
		t.Fatalf("err = %v, want ErrPromoPhoneRequired", err)
	}
	// The failed promo must NOT have frozen the session.
	got, err := repos.GetSessionByID(ctx, sess.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if got.Status != sqlc.SessionStatusActive {
		t.Errorf("session status = %q, want active (promo failure must roll back the transition)", got.Status)
	}
}

func ptr[T any](v T) *T { return &v }
