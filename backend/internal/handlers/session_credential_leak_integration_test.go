//go:build integration

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// sessions.session_token is the guest credential. Every other surface in this
// codebase strips it — guestSafeSession on the four guest session routes and the
// snapshot handler, credentialSafeSession on the SESSION_CREATED event and log
// paths (F-27), toSupportSession on the platform support serializer. The two
// staff routes below embed sqlc.Session in their response struct and so shipped
// it verbatim.
//
// Both routes also contradict their own OpenAPI contract: each responds with
// #/components/schemas/Session, which documents session_token as intentionally
// never returned.
//
// These assert on the serialized HTTP body, not on the Go struct, because the
// json tag on the embedded row is what does the leaking.

// bodyMentions reports whether the raw response body contains the token
// anywhere — key, nested value, or embedded struct.
func bodyMentions(t *testing.T, rec *httptest.ResponseRecorder, token string) bool {
	t.Helper()
	if token == "" {
		t.Fatal("precondition: session has no token to look for")
	}
	return strings.Contains(rec.Body.String(), token)
}

// listActiveSessions drives GET /branches/:id/sessions/active as the given staff member.
func listActiveSessions(t *testing.T, h *SessionHandler, f *recoveryFixture, br recoveryBranch) *httptest.ResponseRecorder {
	t.Helper()
	id := strconv.FormatInt(br.branchID, 10)
	return call(t, br.kitchen, http.MethodGet,
		"/branches/"+id+"/sessions/active",
		gin.Params{{Key: "id", Value: id}}, "", h.ListActiveForBranch)
}

// GET /branches/:id/sessions/active returns every active session on the branch
// to every authenticated staff client — kitchen included, which needs a table
// identifier and a status and nothing else. SessionWithTable embeds sqlc.Session,
// so each element carried the live guest credential for that table.
func TestListActiveSessions_CarriesNoCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, _ := f.openSession(t, f.a)
	token := sess.Session.SessionToken
	if token == "" {
		t.Fatal("precondition: CreateSession returned no token")
	}

	rec := listActiveSessions(t, f.sessionHandler(false), f, f.a)
	if rec.Code != http.StatusOK {
		t.Fatalf("list active sessions: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	// The route must still do its job.
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	if len(out) != 1 {
		t.Fatalf("active sessions: got %d, want 1 (body: %s)", len(out), rec.Body.String())
	}
	if out[0]["id"] != sess.Session.ID.String() {
		t.Errorf("session id: got %v, want %s", out[0]["id"], sess.Session.ID)
	}
	if _, ok := out[0]["table_identifier"]; !ok {
		t.Errorf("response dropped table_identifier, which is why this projection exists: %v", out[0])
	}

	if got, leaked := out[0]["session_token"]; leaked && got != "" {
		t.Errorf("GET /branches/:id/sessions/active returns the guest credential: session_token=%v", got)
	}
	if bodyMentions(t, rec, token) {
		t.Error("GET /branches/:id/sessions/active body contains the guest credential")
	}
}

// POST /sessions/:id/force-close echoes the closed session back to the
// manager/owner who ended it. ForceCloseResult.Session is a bare sqlc.Session,
// so the response carried the credential of the session just closed.
func TestForceCloseSession_CarriesNoCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	sess, _ := f.openSession(t, f.a)
	token := sess.Session.SessionToken
	if token == "" {
		t.Fatal("precondition: CreateSession returned no token")
	}

	rec := forceClose(t, f.sessionHandler(false), f.a.manager, sess.Session.ID,
		`{"reason":"table left without paying"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("force-close: got %d, want 200 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	var out struct {
		Session             map[string]any `json:"session"`
		CancelledPaymentIDs []int64        `json:"cancelled_payment_ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	if out.Session == nil {
		t.Fatalf("response has no `session` key: %s", rec.Body.String())
	}
	if out.Session["id"] != sess.Session.ID.String() {
		t.Errorf("session id: got %v, want %s", out.Session["id"], sess.Session.ID)
	}
	if out.Session["status"] != "closed" {
		t.Errorf("session status: got %v, want closed", out.Session["status"])
	}

	if got, leaked := out.Session["session_token"]; leaked && got != "" {
		t.Errorf("POST /sessions/:id/force-close returns the guest credential: session_token=%v", got)
	}
	if bodyMentions(t, rec, token) {
		t.Error("POST /sessions/:id/force-close body contains the guest credential")
	}
}
