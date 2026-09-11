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
	"github.com/google/uuid"
)

// S2 — staff force-close session.
//
// Sessions end only by host action or by the age-based stale cleaner. A table
// that walks out without paying, or anything that happens outside the app,
// leaves the table occupied with no staff-side way to reclaim it. This route is
// that way out, and it must do everything the host close does — close, release
// the table, revoke participants, bump credential versions, publish
// SESSION_CLOSED — without weakening the host check on the guest path.

func (f *recoveryFixture) sessionHandler(enforce bool) *SessionHandler {
	return NewSessionHandler(
		f.sessionSvc, f.repos, nil, nil, nil, config.FeatureFlags{},
		authz.NewEnforcingAuthorizer(enforce), f.auditW,
	)
}

// forceClose drives POST /sessions/:id/force-close as the given staff member.
func forceClose(t *testing.T, h *SessionHandler, sess services.StaffSession, sessionID uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	return call(t, sess, http.MethodPost,
		"/sessions/"+sessionID.String()+"/force-close",
		gin.Params{{Key: "id", Value: sessionID.String()}}, body, h.ForceClose)
}

// credentialVersions returns each participant's credential_version and
// revocation state, newest id last.
func (f *recoveryFixture) participantCredentials(t *testing.T, sessionID uuid.UUID) []struct {
	Version int32
	Revoked bool
} {
	t.Helper()
	rows, err := f.pool.Query(context.Background(),
		`SELECT credential_version, revoked_at IS NOT NULL FROM session_participants WHERE session_id = $1 ORDER BY id`,
		sessionID)
	if err != nil {
		t.Fatalf("query participants: %v", err)
	}
	defer rows.Close()
	var out []struct {
		Version int32
		Revoked bool
	}
	for rows.Next() {
		var r struct {
			Version int32
			Revoked bool
		}
		if err := rows.Scan(&r.Version, &r.Revoked); err != nil {
			t.Fatalf("scan participant: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// TestStaffForceCloseSession_ClosesAndReleasesTable is the happy path: the
// session ends, the table is reclaimed, and every stored guest credential is
// invalidated so a restored browser tab cannot resurrect the session.
func TestStaffForceCloseSession_ClosesAndReleasesTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, tableID := f.openSession(t, f.a)
	before := f.participantCredentials(t, sess.Session.ID)
	if len(before) != 1 {
		t.Fatalf("precondition: got %d participants, want 1", len(before))
	}
	if got := f.tableStatus(t, tableID); got != string(sqlc.TableStatusOccupied) {
		t.Fatalf("precondition: table status is %s, want occupied", got)
	}

	rec := forceClose(t, f.sessionHandler(false), f.a.manager, sess.Session.ID,
		`{"reason":"table left without paying"}`)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("force-close: got %d, want 200/204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusClosed) {
		t.Errorf("session status: got %s, want closed", got)
	}
	if got := f.tableStatus(t, tableID); got != string(sqlc.TableStatusAvailable) {
		t.Errorf("table status: got %s, want available — the table is still blocked", got)
	}

	after := f.participantCredentials(t, sess.Session.ID)
	if len(after) != len(before) {
		t.Fatalf("participants: got %d, want %d", len(after), len(before))
	}
	for i := range after {
		if !after[i].Revoked {
			t.Errorf("participant %d: not revoked", i)
		}
		if after[i].Version <= before[i].Version {
			t.Errorf("participant %d credential_version: got %d, want > %d", i, after[i].Version, before[i].Version)
		}
	}

	if events := f.publishedEvents(t, sess.Session.ID); !containsString(events, "SESSION_CLOSED") {
		t.Errorf("published events %v: missing SESSION_CLOSED", events)
	}
}

// TestStaffForceCloseSession_CancelsOutstandingPayment — force-close works
// regardless of payment state. A non-terminal payment left on a closed session
// could never be moved again by any route and would sit in the staff pending
// queue and the stalled-payment alert forever, so it is cancelled with the
// session. It is never marked completed: no money was collected.
func TestStaffForceCloseSession_CancelsOutstandingPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, tableID, payment := f.freezeSession(t, f.a, sqlc.PaymentStatusProviderPending)

	rec := forceClose(t, f.sessionHandler(false), f.a.owner, sess.Session.ID,
		`{"reason":"POS crashed mid-settlement, table already gone"}`)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("force-close: got %d, want 200/204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusClosed) {
		t.Errorf("session status: got %s, want closed", got)
	}
	if got := f.tableStatus(t, tableID); got != string(sqlc.TableStatusAvailable) {
		t.Errorf("table status: got %s, want available", got)
	}
	if got := f.paymentStatus(t, payment.ID); got != string(sqlc.PaymentStatusCancelled) {
		t.Errorf("payment status: got %s, want cancelled — a dangling non-terminal payment is stranded forever", got)
	}
}

// TestStaffForceCloseSession_RejectsWaiter — this can discard an unpaid bill and
// revoke every guest credential, so it sits with the owner/manager writes
// (promos, branch settings, table CRUD), not with waiter service actions.
func TestStaffForceCloseSession_RejectsWaiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	for _, enforce := range []bool{false, true} {
		t.Run(fmt.Sprintf("enforce=%v", enforce), func(t *testing.T) {
			sess, tableID := f.openSession(t, f.a)
			rec := forceClose(t, f.sessionHandler(enforce), f.a.waiter, sess.Session.ID, `{"reason":"tidying up"}`)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("got %d, want 403 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusActive) {
				t.Errorf("session status: got %s, want active — denied request still closed it", got)
			}
			if got := f.tableStatus(t, tableID); got != string(sqlc.TableStatusOccupied) {
				t.Errorf("table status: got %s, want occupied", got)
			}
		})
	}
}

// TestStaffForceCloseSession_AllowedRoles — manager and owner both qualify.
func TestStaffForceCloseSession_AllowedRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	for _, actor := range []struct {
		name string
		sess services.StaffSession
	}{
		{"owner", f.a.owner},
		{"manager", f.a.manager},
	} {
		t.Run(actor.name, func(t *testing.T) {
			sess, _ := f.openSession(t, f.a)
			rec := forceClose(t, f.sessionHandler(false), actor.sess, sess.Session.ID, `{"reason":"walked out"}`)
			if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
				t.Fatalf("got %d, want 200/204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}

// TestStaffForceCloseSession_RejectsCrossBranchActor — the branch is derived
// from the session, never from the caller.
func TestStaffForceCloseSession_RejectsCrossBranchActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	for _, enforce := range []bool{false, true} {
		t.Run(fmt.Sprintf("enforce=%v", enforce), func(t *testing.T) {
			sess, tableID := f.openSession(t, f.b)
			rec := forceClose(t, f.sessionHandler(enforce), f.a.owner, sess.Session.ID, `{"reason":"not my branch"}`)
			if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
				t.Fatalf("got %d, want 403 or 404 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusActive) {
				t.Errorf("session status: got %s, want active — a foreign branch closed it", got)
			}
			if got := f.tableStatus(t, tableID); got != string(sqlc.TableStatusOccupied) {
				t.Errorf("table status: got %s, want occupied", got)
			}
		})
	}
}

// TestStaffForceCloseSession_RefusesAlreadyClosedSession — closed is terminal.
// A second force-close is a conflict, not a silent success: it would otherwise
// write an audit entry claiming a close that never happened.
func TestStaffForceCloseSession_RefusesAlreadyClosedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, _ := f.openSession(t, f.a)
	h := f.sessionHandler(false)

	rec := forceClose(t, h, f.a.manager, sess.Session.ID, `{"reason":"first"}`)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("first force-close: got %d, want 200/204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	rec = forceClose(t, h, f.a.manager, sess.Session.ID, `{"reason":"second"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second force-close: got %d, want 409 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	entries := f.auditEntries(t, "session.force_close", sess.Session.ID.String())
	var succeeded int
	for _, e := range entries {
		if e.Result == "success" {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful session.force_close audit entries: got %d, want exactly 1 (%+v)", succeeded, entries)
	}
}

// TestStaffForceCloseSession_RecordsAuditWithStaffAndReason — discarding a bill
// must be attributable.
func TestStaffForceCloseSession_RecordsAuditWithStaffAndReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, _ := f.openSession(t, f.a)
	const reason = "guests left during a power cut; bill written off by duty manager"
	rec := forceClose(t, f.sessionHandler(false), f.a.manager, sess.Session.ID, fmt.Sprintf(`{"reason":%q}`, reason))
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("force-close: got %d, want 200/204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	entries := f.auditEntries(t, "session.force_close", sess.Session.ID.String())
	if len(entries) != 1 {
		t.Fatalf("session.force_close audit entries: got %d, want 1 (%+v)", len(entries), entries)
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

// TestStaffForceCloseSession_RequiresReason — same bar as the payment cancel.
func TestStaffForceCloseSession_RequiresReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, _ := f.openSession(t, f.a)
	for _, body := range []string{`{}`, `{"reason":"   "}`} {
		rec := forceClose(t, f.sessionHandler(false), f.a.manager, sess.Session.ID, body)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body %s: got %d, want 400/422 (body: %s)", body, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusActive) {
		t.Errorf("session status: got %s, want active", got)
	}
}

// TestStaffForceCloseSession_IsNotHealedBackIntoAHost — the snapshot reconnect
// path heals host-less sessions by promoting a participant, and a force-closed
// session is still inside the terminal read window, so a returning guest does
// get a snapshot. That must never hand the closed session a fresh host.
//
// The host is deliberately transferred to the *second* participant first. The
// healing path falls back to the oldest active participant, so with the host as
// the oldest joiner a re-pick would land on the same person and the assertion
// would pass no matter what the healing path did. Transferring first makes the
// promotion observable.
func TestStaffForceCloseSession_IsNotHealedBackIntoAHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	sess, _ := f.openSession(t, f.a)
	second, err := f.sessionSvc.JoinSession(ctx, sess.Session.ID, "Second Guest", "fp-second", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	// Both guests are at the table and connected. The heartbeats are load-bearing
	// twice over: TransferHost refuses a target without live presence (90s
	// window), and pickNewHost will only ever promote a participant that is
	// present — so without one on the first guest there would be no eligible
	// successor at all and this test could not observe a wrong promotion.
	f.markPresent(t, f.a, sess.Session.ID, sess.Participant.ID)
	f.markPresent(t, f.a, sess.Session.ID, second.ID)
	if err := f.sessionSvc.TransferHost(ctx, sess.Session.ID, sess.Participant.ID, second.ID); err != nil {
		t.Fatalf("TransferHost: %v", err)
	}
	hostBefore := f.sessionHost(t, sess.Session.ID)
	if hostBefore != second.ID {
		t.Fatalf("precondition: host is %d, want the second participant %d", hostBefore, second.ID)
	}
	// Everything the close is allowed to be judged against starts here.
	eventsBefore := len(f.publishedEvents(t, sess.Session.ID))

	rec := forceClose(t, f.sessionHandler(false), f.a.manager, sess.Session.ID, `{"reason":"walked out mid-service"}`)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("force-close: got %d, want 200/204 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	// The reconnect a returning guest would make. A WebSocket reconnect writes a
	// presence heartbeat before the client calls the snapshot endpoint, so model
	// that order — closing clears presence, and without re-establishing it there
	// would be no present successor and nothing for the healing path to get
	// wrong. This is what keeps the assertion below non-vacuous.
	f.markPresent(t, f.a, sess.Session.ID, sess.Participant.ID)
	f.markPresent(t, f.a, sess.Session.ID, second.ID)

	snap, err := f.sessionSvc.GetSnapshot(ctx, sess.Session.ID, 0)
	if err != nil {
		t.Fatalf("GetSnapshot after force-close: %v", err)
	}
	if !snap.SessionEnded {
		t.Error("snapshot does not report the session as ended")
	}
	if snap.Session.HostParticipantID.Int64 != hostBefore {
		t.Errorf("snapshot host_participant_id: got %d, want %d — the closed session was healed into a new host",
			snap.Session.HostParticipantID.Int64, hostBefore)
	}

	if got := f.sessionHost(t, sess.Session.ID); got != hostBefore {
		t.Errorf("persisted host_participant_id: got %d, want %d unchanged", got, hostBefore)
	}
	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusClosed) {
		t.Errorf("session status: got %s, want closed", got)
	}
	for i, p := range f.participantCredentials(t, sess.Session.ID) {
		if !p.Revoked {
			t.Errorf("participant %d: revocation was undone", i)
		}
	}
	after := f.publishedEvents(t, sess.Session.ID)[eventsBefore:]
	if containsString(after, "HOST_CHANGED") {
		t.Errorf("events after close %v: a closed session emitted HOST_CHANGED", after)
	}
}

// TestGuestCloseStillRequiresHost is the guard on the S2 refactor: factoring the
// shared close work out must not weaken the host check on the guest path.
func TestGuestCloseStillRequiresHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	sess, tableID := f.openSession(t, f.a)
	guest, err := f.sessionSvc.JoinSession(ctx, sess.Session.ID, "Not The Host", "fp-guest", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}

	nonHost := guest.ID
	if err := f.sessionSvc.CloseSession(ctx, sess.Session.ID, &nonHost); err == nil {
		t.Fatal("CloseSession by a non-host succeeded; the host check was weakened")
	}
	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusActive) {
		t.Errorf("session status: got %s, want active", got)
	}
	if got := f.tableStatus(t, tableID); got != string(sqlc.TableStatusOccupied) {
		t.Errorf("table status: got %s, want occupied", got)
	}

	host := sess.Participant.ID
	if err := f.sessionSvc.CloseSession(ctx, sess.Session.ID, &host); err != nil {
		t.Fatalf("CloseSession by the host: %v", err)
	}
	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusClosed) {
		t.Errorf("session status: got %s, want closed", got)
	}
}
