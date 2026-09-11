//go:build integration

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// P6 — staff cancel payment.
//
// A non-terminal payment freezes its session: payment_pending means the cart is
// frozen and the table cannot place new orders. Staff settlement only accepts
// requires_staff_confirmation and the escalation worker only alerts, so before
// this route the sole escape was closing the session outright. These tests pin
// the effect — on the payment, the session, the cart, the broadcast and the
// audit trail — not just the status code.

func (f *recoveryFixture) paymentHandler(enforce bool) *PaymentHandler {
	return NewPaymentHandler(
		f.paymentSvc, f.repos, nil, config.FeatureFlags{}, config.PaymentConfig{},
		authz.NewEnforcingAuthorizer(enforce), f.auditW, services.NewPromoService(f.repos),
	)
}

// cancelPayment drives PATCH /payments/:id/cancel as the given staff member.
func cancelPayment(t *testing.T, h *PaymentHandler, sess services.StaffSession, paymentID int64, body string) *httptest.ResponseRecorder {
	t.Helper()
	return call(t, sess, http.MethodPatch,
		"/payments/"+strconv.FormatInt(paymentID, 10)+"/cancel",
		idParam(paymentID), body, h.Cancel)
}

// TestStaffCancelPayment_UnfreezesSession is the happy path: from either
// cancellable non-terminal status the payment ends cancelled, the session
// returns to active, the guests are told, and the table can order again.
func TestStaffCancelPayment_UnfreezesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	cases := []struct {
		name   string
		method sqlc.PaymentMethod
		want   sqlc.PaymentStatus
	}{
		{"provider_pending", sqlc.PaymentMethodDigital, sqlc.PaymentStatusProviderPending},
		{"requires_staff_confirmation", sqlc.PaymentMethodCash, sqlc.PaymentStatusRequiresStaffConfirmation},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess, _, payment := f.freezeSession(t, f.a, tc.method)
			if payment.Status != tc.want {
				t.Fatalf("precondition: payment status is %s, want %s", payment.Status, tc.want)
			}
			if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusPaymentPending) {
				t.Fatalf("precondition: session status is %s, want payment_pending", got)
			}

			rec := cancelPayment(t, f.paymentHandler(false), f.a.waiter, payment.ID,
				`{"reason":"guest walked out before the terminal arrived"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("cancel: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}

			if got := f.paymentStatus(t, payment.ID); got != string(sqlc.PaymentStatusCancelled) {
				t.Errorf("payment status: got %s, want cancelled", got)
			}
			if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusActive) {
				t.Errorf("session status: got %s, want active — the cart is still frozen", got)
			}

			// Connected guests must learn the cart is live again, otherwise the
			// freeze is only lifted server-side.
			events := f.publishedEvents(t, sess.Session.ID)
			if !containsString(events, "PAYMENT_CANCELLED") {
				t.Errorf("published events %v: missing PAYMENT_CANCELLED", events)
			}

			// The point of the whole route: the table can order again.
			if _, err := f.cartSvc.AddItem(ctx, services.AddItemRequest{
				SessionID:     sess.Session.ID,
				ParticipantID: sess.Participant.ID,
				MenuItemID:    f.a.itemID,
				Quantity:      1,
			}); err != nil {
				t.Errorf("AddItem after cancel: %v — session did not actually unfreeze", err)
			}
		})
	}
}

// TestStaffCancelPayment_AllowedRoles pins the role decision: waiters collect at
// the table and are already trusted to settle, so they may also withdraw the
// request. Owner and manager follow.
func TestStaffCancelPayment_AllowedRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	for _, actor := range []struct {
		name string
		sess services.StaffSession
	}{
		{"owner", f.a.owner},
		{"manager", f.a.manager},
		{"waiter", f.a.waiter},
	} {
		t.Run(actor.name, func(t *testing.T) {
			_, _, payment := f.freezeSession(t, f.a, sqlc.PaymentMethodDigital)
			rec := cancelPayment(t, f.paymentHandler(false), actor.sess, payment.ID, `{"reason":"terminal declined"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}

// TestStaffCancelPayment_RejectsKitchenRole — kitchen never touches money.
// Asserted with the central enforcement flag both off and on: a brand-new route
// has no legacy traffic to learn from, so it must deny on the shipped default.
func TestStaffCancelPayment_RejectsKitchenRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	for _, enforce := range []bool{false, true} {
		t.Run(fmt.Sprintf("enforce=%v", enforce), func(t *testing.T) {
			_, _, payment := f.freezeSession(t, f.a, sqlc.PaymentMethodDigital)
			rec := cancelPayment(t, f.paymentHandler(enforce), f.a.kitchen, payment.ID, `{"reason":"nope"}`)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("got %d, want 403 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			if got := f.paymentStatus(t, payment.ID); got != string(sqlc.PaymentStatusProviderPending) {
				t.Errorf("payment status: got %s, want provider_pending — denied request still mutated state", got)
			}
		})
	}
}

// TestStaffCancelPayment_RejectsCrossBranchActor — the branch is derived from
// the payment, never from the caller, so branch A staff cannot reach into B.
func TestStaffCancelPayment_RejectsCrossBranchActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	for _, enforce := range []bool{false, true} {
		t.Run(fmt.Sprintf("enforce=%v", enforce), func(t *testing.T) {
			_, _, payment := f.freezeSession(t, f.b, sqlc.PaymentMethodDigital)
			rec := cancelPayment(t, f.paymentHandler(enforce), f.a.owner, payment.ID, `{"reason":"not mine to cancel"}`)
			if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
				t.Fatalf("got %d, want 403 or 404 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			if got := f.paymentStatus(t, payment.ID); got != string(sqlc.PaymentStatusProviderPending) {
				t.Errorf("payment status: got %s, want provider_pending — a foreign branch cancelled it", got)
			}
		})
	}
}

// TestStaffCancelPayment_RefusesCompletedPayment — completed is terminal in the
// domain transition table. Cancelling settled money must never be possible.
func TestStaffCancelPayment_RefusesCompletedPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	_, _, payment := f.freezeSession(t, f.a, sqlc.PaymentMethodDigital)
	if _, err := f.repos.UpdatePaymentStatusExpected(ctx, payment.ID,
		sqlc.PaymentStatusProviderPending, sqlc.PaymentStatusCompleted); err != nil {
		t.Fatalf("mark payment completed: %v", err)
	}

	rec := cancelPayment(t, f.paymentHandler(false), f.a.manager, payment.ID, `{"reason":"changed my mind"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	if got := f.paymentStatus(t, payment.ID); got != string(sqlc.PaymentStatusCompleted) {
		t.Errorf("payment status: got %s, want completed", got)
	}
}

// TestStaffCancelPayment_SecondCancelConflicts documents the chosen behaviour
// for a repeat cancel: explicitly conflicting, not silently idempotent. Each
// cancel carries its own reason and audit entry, so a second 200 would claim a
// cancellation that never happened.
func TestStaffCancelPayment_SecondCancelConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	_, _, payment := f.freezeSession(t, f.a, sqlc.PaymentMethodDigital)
	h := f.paymentHandler(false)

	if rec := cancelPayment(t, h, f.a.waiter, payment.ID, `{"reason":"first cancel"}`); rec.Code != http.StatusOK {
		t.Fatalf("first cancel: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	rec := cancelPayment(t, h, f.a.waiter, payment.ID, `{"reason":"second cancel"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second cancel: got %d, want 409 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	entries := f.auditEntries(t, "payment.cancel", strconv.FormatInt(payment.ID, 10))
	var succeeded int
	for _, e := range entries {
		if e.Result == "success" {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful payment.cancel audit entries: got %d, want exactly 1 (%+v)", succeeded, entries)
	}
}

// TestStaffCancelPayment_RecordsAuditWithStaffAndReason — this is a
// money-adjacent action, so who did it and why must both be recoverable.
func TestStaffCancelPayment_RecordsAuditWithStaffAndReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	_, _, payment := f.freezeSession(t, f.a, sqlc.PaymentMethodDigital)
	const reason = "UPI app never confirmed; guest paid cash instead"
	rec := cancelPayment(t, f.paymentHandler(false), f.a.manager, payment.ID,
		fmt.Sprintf(`{"reason":%q}`, reason))
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	entries := f.auditEntries(t, "payment.cancel", strconv.FormatInt(payment.ID, 10))
	if len(entries) != 1 {
		t.Fatalf("payment.cancel audit entries: got %d, want 1 (%+v)", len(entries), entries)
	}
	got := entries[0]
	if got.ActorID != strconv.FormatInt(f.a.manager.StaffID, 10) {
		t.Errorf("audit actor_id: got %q, want %d", got.ActorID, f.a.manager.StaffID)
	}
	if got.ActorTyp != "staff" {
		t.Errorf("audit actor_type: got %q, want staff", got.ActorTyp)
	}
	if got.Reason != reason {
		t.Errorf("audit reason: got %q, want %q", got.Reason, reason)
	}
}

// TestStaffCancelPayment_RequiresReason — an unexplained cancellation of a live
// bill is not acceptable on a money-adjacent route.
func TestStaffCancelPayment_RequiresReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	_, _, payment := f.freezeSession(t, f.a, sqlc.PaymentMethodDigital)
	for _, body := range []string{`{}`, `{"reason":"   "}`} {
		rec := cancelPayment(t, f.paymentHandler(false), f.a.waiter, payment.ID, body)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body %s: got %d, want 400/422 (body: %s)", body, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
	if got := f.paymentStatus(t, payment.ID); got != string(sqlc.PaymentStatusProviderPending) {
		t.Errorf("payment status: got %s, want provider_pending", got)
	}
}
