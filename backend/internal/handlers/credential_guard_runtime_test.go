//go:build integration

package handlers

// Credential leak guard, part 4 of 4: what the wires actually carried.
//
// Part 3 (credential_guard_static_test.go) is a tripwire on exposure
// SURFACE — it fires when a new type that can carry a credential reaches a
// wire. It cannot fire on the other failure mode, because every fix in this
// codebase is value-level:
//
//	func guestSafeSession(s sqlc.Session) sqlc.Session { s.SessionToken = ""; return s }
//
// The returned type is sqlc.Session either way. Delete the call and the type
// graph is byte-identical; only the value changes. So this half runs the real
// routes against the real database and inspects what came out.
//
// Two independent checks on every payload, because each catches what the other
// misses:
//
//	by name  — a ruled-secret json key present with a non-empty value.
//	           Catches a raw row shipped under its own field name.
//	by value — the live credential string appearing anywhere in the bytes.
//	           Catches the same credential shipped under an innocent key, or
//	           nested inside a jsonb blob the static walk cannot see into.
//
// Coverage is all three surfaces the SESSION_CREATED leak reached:
//
//	HTTP       recorded response bodies
//	WEBSOCKET  session_events.payload — what AppendSessionEvent put on the wire
//	PERSISTED  event_log.payload — what LogEvent wrote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ── the scanner ──────────────────────────────────────────────────────────────

// jsonLeak is one secret found in a real payload.
type jsonLeak struct {
	Path   string // "$.session.session_token"
	Key    string // "session_token"
	Value  string // truncated, so a failure log never becomes a credential store
	Reason string
}

func (l jsonLeak) String() string {
	return fmt.Sprintf("%s (key %q, value %s) — %s", l.Path, l.Key, l.Value, l.Reason)
}

// scanPayloadForSecrets decodes a payload and reports every ruled-secret key
// holding a value, plus any occurrence of a known-live credential string.
//
// An empty value is not a leak: clearing the field IS the fix everywhere in
// this codebase, and a `"session_token": ""` key means the scrubber ran.
func scanPayloadForSecrets(raw []byte, liveSecrets map[string]string) []jsonLeak {
	var leaks []jsonLeak

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err == nil {
		secrets := secretJSONNames()
		walkJSON(decoded, "$", func(path, key string, value any) {
			why, ruled := secrets[key]
			if !ruled || isEmptyJSONValue(value) {
				return
			}
			leaks = append(leaks, jsonLeak{
				Path: path, Key: key, Value: redactForLog(value),
				Reason: why,
			})
		})
	}

	// Value check: catches a credential that travelled under a key the name
	// check does not know, and one buried in a jsonb blob.
	body := string(raw)
	for label, secret := range liveSecrets {
		if secret == "" {
			continue
		}
		if strings.Contains(body, secret) {
			leaks = append(leaks, jsonLeak{
				Path:   "(anywhere in the payload)",
				Key:    label,
				Value:  redactForLog(secret),
				Reason: "the live " + label + " for this session appears verbatim in the bytes, whatever key it travelled under",
			})
		}
	}
	return leaks
}

// walkJSON visits every key in a decoded JSON tree. Struct embedding is already
// flattened by this point — encoding/json did it — which is exactly why
// inspecting the serialized form catches embeds a type-level check would have
// to reason about.
func walkJSON(node any, path string, visit func(path, key string, value any)) {
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := path + "." + k
			visit(child, k, v[k])
			walkJSON(v[k], child, visit)
		}
	case []any:
		for i, item := range v {
			walkJSON(item, fmt.Sprintf("%s[%d]", path, i), visit)
		}
	case string:
		// A jsonb column re-served as a string can hold a whole nested payload.
		if len(v) > 1 && (v[0] == '{' || v[0] == '[') {
			var nested any
			if err := json.Unmarshal([]byte(v), &nested); err == nil {
				walkJSON(nested, path+"(nested)", visit)
			}
		}
	}
}

func isEmptyJSONValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case bool:
		return !t
	}
	return false
}

func redactForLog(v any) string {
	s := fmt.Sprint(v)
	if len(s) <= 8 {
		return strconv.Quote(s)
	}
	return strconv.Quote(s[:8]) + fmt.Sprintf("…(%d chars)", len(s))
}

// ── the probe ────────────────────────────────────────────────────────────────

// wirePayload is one thing that crossed a wire, with enough context to name it
// in a failure.
type wirePayload struct {
	Surface string
	Where   string // route, event name, or table row
	Raw     []byte
}

// guardProbe drives the session surfaces and collects everything they emitted.
type guardProbe struct {
	f        *recoveryFixture
	sessionH *SessionHandler
	snapH    *SnapshotHandler

	sessionID uuid.UUID
	branchID  int64

	live     map[string]string // label -> live credential value
	payloads []wirePayload
}

func newGuardProbe(t *testing.T) *guardProbe {
	t.Helper()
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)

	// A real guest token service: Create issues a guest_access_token, and
	// without one the route 500s before it ever serializes a session.
	guestTokens := auth.NewGuestTokenService("credential-leak-guard-secret-key-32-bytes!!", time.Hour)
	sessionH := NewSessionHandler(
		f.sessionSvc, f.repos, nil, guestTokens, nil, config.FeatureFlags{},
		authz.NewEnforcingAuthorizer(false), f.auditW,
	)
	snapH := NewSnapshotHandler(f.sessionSvc, f.repos, guestTokens, config.FeatureFlags{})

	return &guardProbe{
		f: f, sessionH: sessionH, snapH: snapH,
		branchID: f.a.branchID,
		live:     map[string]string{},
	}
}

func (p *guardProbe) record(surface, where string, raw []byte) {
	if len(raw) == 0 {
		return
	}
	p.payloads = append(p.payloads, wirePayload{Surface: surface, Where: where, Raw: raw})
}

// exerciseSessionLifecycle drives every session route that serializes a session
// row, then force-closes. Each response body is captured; the WebSocket and
// persisted payloads land in the database on the way and are collected after.
func (p *guardProbe) exerciseSessionLifecycle(t *testing.T) {
	t.Helper()

	// POST /sessions — the route whose service call publishes SESSION_CREATED to
	// all three surfaces at once.
	tableID := p.f.newTable(t, p.f.a)
	rec := p.callGuest(t, http.MethodPost, "/sessions", nil,
		fmt.Sprintf(`{"table_id":%d,"display_name":"Guard"}`, tableID), p.sessionH.Create)
	p.expect(t, rec, http.StatusCreated, "POST /sessions")
	p.record(surfaceHTTP, "POST /sessions", rec.Body.Bytes())

	var created struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode POST /sessions: %v (body %s)", err, rec.Body.String())
	}
	id, err := uuid.Parse(created.Session.ID)
	if err != nil {
		t.Fatalf("POST /sessions returned no usable session id: %v (body %s)", err, rec.Body.String())
	}
	p.sessionID = id

	// The live credential, read straight from the row. Everything below is
	// checked against this value as well as against key names.
	var token string
	if err := p.f.pool.QueryRow(context.Background(),
		`SELECT session_token FROM sessions WHERE id = $1`, id).Scan(&token); err != nil {
		t.Fatalf("read sessions.session_token: %v", err)
	}
	if token == "" {
		t.Fatal("precondition: the session row has no token, so nothing below can prove anything")
	}
	p.live["guest session_token"] = token

	idStr := id.String()
	params := gin.Params{{Key: "id", Value: idStr}}

	// GET /sessions/:id
	rec = p.callGuest(t, http.MethodGet, "/sessions/"+idStr, params, "", p.sessionH.Get)
	p.expect(t, rec, http.StatusOK, "GET /sessions/:id")
	p.record(surfaceHTTP, "GET /sessions/:id", rec.Body.Bytes())

	// POST /sessions/:id/join
	rec = p.callGuest(t, http.MethodPost, "/sessions/"+idStr+"/join", params,
		`{"display_name":"Second Guest"}`, p.sessionH.Join)
	p.expect(t, rec, http.StatusCreated, "POST /sessions/:id/join")
	p.record(surfaceHTTP, "POST /sessions/:id/join", rec.Body.Bytes())

	// GET /sessions/:id/snapshot — reachable without a guest credential while
	// AUTH_GUEST_CREDENTIALS_REQUIRED is false, which is why F-8 mattered.
	rec = p.callGuest(t, http.MethodGet, "/sessions/"+idStr+"/snapshot", params, "", p.snapH.GetSnapshot)
	p.expect(t, rec, http.StatusOK, "GET /sessions/:id/snapshot")
	p.record(surfaceHTTP, "GET /sessions/:id/snapshot", rec.Body.Bytes())

	// GET /sessions/:id/snapshot?last_sequence=1 — the same route again, this
	// time returning MissedEvents: session_events rows replayed verbatim, with
	// no projection of any kind (guestSafeSession covers snapshot.Session only).
	// Driving it means the bytes those rows will replay are inspected here too,
	// for every writer these flows exercise. See TestReplayPathsHaveNoProjection
	// for what that does and does not cover.
	rec = p.callGuest(t, http.MethodGet, "/sessions/"+idStr+"/snapshot?last_sequence=1", params, "", p.snapH.GetSnapshot)
	p.expect(t, rec, http.StatusOK, "GET /sessions/:id/snapshot?last_sequence=1")
	p.record(surfaceHTTP, "GET /sessions/:id/snapshot?last_sequence=1 (replays missed_events)", rec.Body.Bytes())
	if !strings.Contains(rec.Body.String(), `"missed_events"`) {
		t.Fatalf("snapshot with last_sequence=1 returned no missed_events, so the replay path was not actually exercised: %s",
			truncateBody(rec.Body.Bytes()))
	}

	// GET /branches/:id/sessions/active — as kitchen, the least-privileged staff
	// role on the branch and the one leak #2 reached.
	branchStr := strconv.FormatInt(p.branchID, 10)
	rec = call(t, p.f.a.kitchen, http.MethodGet, "/branches/"+branchStr+"/sessions/active",
		gin.Params{{Key: "id", Value: branchStr}}, "", p.sessionH.ListActiveForBranch)
	p.expect(t, rec, http.StatusOK, "GET /branches/:id/sessions/active")
	p.record(surfaceHTTP, "GET /branches/:id/sessions/active (as kitchen)", rec.Body.Bytes())

	// POST /sessions/:id/force-close — as manager, the role leak #3 reached.
	rec = forceClose(t, p.sessionH, p.f.a.manager, id, `{"reason":"credential leak guard"}`)
	p.expect(t, rec, http.StatusOK, "POST /sessions/:id/force-close")
	p.record(surfaceHTTP, "POST /sessions/:id/force-close (as manager)", rec.Body.Bytes())
}

// collectPersisted reads back what the WebSocket and event-log paths wrote.
//
// session_events.payload is not a proxy for the WebSocket payload — it IS it.
// Publisher.publish calls AppendSessionEvent, which builds the envelope and
// stores env.Payload, and then hands that same envelope to Redis
// (internal/events/events.go). Whatever is in this column went to every
// subscribed client.
func (p *guardProbe) collectPersisted(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	rows, err := p.f.pool.Query(ctx,
		`SELECT event, payload::text FROM session_events WHERE session_id = $1 ORDER BY sequence`, p.sessionID)
	if err != nil {
		t.Fatalf("query session_events: %v", err)
	}
	defer rows.Close()
	events := 0
	for rows.Next() {
		var event, payload string
		if err := rows.Scan(&event, &payload); err != nil {
			t.Fatalf("scan session_events: %v", err)
		}
		events++
		p.record(surfaceWebsocket, "session_events row for "+event, []byte(payload))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate session_events: %v", err)
	}
	if events == 0 {
		t.Fatal("no session_events rows: the WebSocket surface was never exercised, so this test proves nothing about it")
	}

	logRows, err := p.f.pool.Query(ctx,
		`SELECT event_type, payload::text FROM event_log WHERE session_id = $1 ORDER BY id`, p.sessionID)
	if err != nil {
		t.Fatalf("query event_log: %v", err)
	}
	defer logRows.Close()
	logged := 0
	for logRows.Next() {
		var eventType, payload string
		if err := logRows.Scan(&eventType, &payload); err != nil {
			t.Fatalf("scan event_log: %v", err)
		}
		logged++
		p.record(surfacePersisted, "event_log row for "+eventType, []byte(payload))
	}
	if err := logRows.Err(); err != nil {
		t.Fatalf("iterate event_log: %v", err)
	}
	if logged == 0 {
		t.Fatal("no event_log rows: the persisted surface was never exercised, so this test proves nothing about it")
	}
}

func (p *guardProbe) callGuest(t *testing.T, method, path string, params gin.Params, body string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	h(c)
	c.Writer.WriteHeaderNow()
	return rec
}

func (p *guardProbe) expect(t *testing.T, rec *httptest.ResponseRecorder, want int, what string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("%s: got %d, want %d — the guard cannot inspect a payload the route never produced (body: %s)",
			what, rec.Code, want, strings.TrimSpace(rec.Body.String()))
	}
}

// assertClean is the assertion the whole file exists for.
func (p *guardProbe) assertClean(t *testing.T, surface string) {
	t.Helper()
	checked, leaked := 0, 0
	for _, payload := range p.payloads {
		if payload.Surface != surface {
			continue
		}
		checked++
		for _, leak := range scanPayloadForSecrets(payload.Raw, p.live) {
			leaked++
			t.Errorf(`
A credential reached a %s surface.

  where:     %s
  json path: %s
  key:       %s
  value:     %s

  why this is a credential:
    %s

  The payload, in full:
    %s

  This is the value-level check: the type that carries this field is allowed to
  reach this surface (see wireInventory in credential_guard_static_test.go), so
  what changed is that the projection which empties the field stopped running.
  The scrubbers are handlers.guestSafeSession, services.credentialSafeSession
  and services.credentialSafeResult.`,
				surface, payload.Where, leak.Path, leak.Key, leak.Value, leak.Reason,
				truncateBody(payload.Raw))
		}
	}
	if checked == 0 {
		t.Fatalf("no %s payloads were captured — the guard checked nothing", surface)
	}
	if leaked == 0 {
		t.Logf("%s: %d payloads inspected, no credential present", surface, checked)
	}
}

func truncateBody(raw []byte) string {
	const max = 1500
	if len(raw) <= max {
		return string(raw)
	}
	return string(raw[:max]) + fmt.Sprintf("… (%d bytes total)", len(raw))
}

// ── the tests ────────────────────────────────────────────────────────────────

// TestHTTPResponsesCarryNoCredential covers the two staff routes that leaked
// (leak #2 and #3) plus every guest route that serializes a session row.
func TestHTTPResponsesCarryNoCredential(t *testing.T) {
	p := newGuardProbe(t)
	p.exerciseSessionLifecycle(t)
	p.assertClean(t, surfaceHTTP)
}

// TestWebSocketPayloadsCarryNoCredential covers the fan-out half of leak #1.
func TestWebSocketPayloadsCarryNoCredential(t *testing.T) {
	p := newGuardProbe(t)
	p.exerciseSessionLifecycle(t)
	p.collectPersisted(t)
	p.assertClean(t, surfaceWebsocket)
}

// TestPersistedPayloadsCarryNoCredential covers the event_log half of leak #1.
//
// This is also what stands behind the three replay read paths — GET
// /sessions/:id/events, GET /branches/:id/events and snapshot.MissedEvents.
// None of them projects; they hand back the stored bytes. Checking the stored
// bytes is therefore checking what they will replay, for every writer these
// flows exercise. See TestReplayPathsHaveNoProjection for the limit of that.
func TestPersistedPayloadsCarryNoCredential(t *testing.T) {
	p := newGuardProbe(t)
	p.exerciseSessionLifecycle(t)
	p.collectPersisted(t)
	p.assertClean(t, surfacePersisted)
}

// TestAuditRedactionCoversEverySecret checks the production denylist in
// internal/audit/redaction.go against the derived catalogue.
//
// audit_log.before_json / after_json is a fourth wire surface — the platform
// audit read API serves it — and it is the one surface defended by a
// hand-written key list rather than by a projection. That list is exactly the
// kind of artefact this guard exists to distrust, so it is checked against the
// catalogue rather than read.
//
// When this test was first written it found three escapes: session_token,
// recovery_codes and challenge_hash. All three are now in sensitiveKeys, so
// knownGaps is empty and every ruled secret must be redacted. The map stays
// because the next gap should be recordable without deleting the tripwire.
func TestAuditRedactionCoversEverySecret(t *testing.T) {
	// Ruled-secret json names that audit.Redact is knowingly allowed to pass
	// through. Empty is the correct state; an entry here is a debt with a name.
	knownGaps := map[string]string{}

	const canary = "LIVE-CREDENTIAL-VALUE"
	for name, why := range secretJSONNames() {
		before := json.RawMessage(fmt.Sprintf(`{%q:%q}`, name, canary))
		redacted, _ := audit.Redact(before, nil)
		isRedacted := !strings.Contains(string(redacted), canary)

		reason, known := knownGaps[name]
		switch {
		case isRedacted && known:
			t.Errorf("audit.Redact now redacts %q — delete it from knownGaps.\n  (it was recorded because: %s)", name, reason)
		case !isRedacted && !known:
			t.Errorf(`
audit.Redact does not redact the credential field %q.

  audit_log.before_json / after_json is served by the platform audit read API
  and is defended by the hand-written key list in internal/audit/redaction.go,
  not by a projection. isSensitive does an exact match on the whole key and
  then a substring pass, so a near-miss — a plural, a prefix, a synonym —
  silently passes the credential through.

  Add %q to sensitiveKeys there, or record it in knownGaps with the reason.

  why %q is a credential:
    %s`, name, name, name, why)
		}
	}
}

// TestAuditRedactionTreatsBothSpellingsAlike pins the property that the
// camelCase normalization in audit.isSensitive exists to establish: a key
// classifies the same whether it arrives as json-tag snake_case or as the Go
// field name an untagged struct serializes under.
//
// The gap this replaces was found while confirming the MFA recovery-code
// hashes never reach a response. isSensitive compared whole lowercased keys,
// so it matched "recovery_codes" but not "RecoveryCodes" — and
// repository.PlatformMFA, which holds those bcrypt hashes, carries no json
// tags at all. Six of the eight ruled secrets were under-redacted in that
// spelling.
//
// Measured over every key in the codebase, closing it changed exactly one
// classification (RecoveryCodes, the only untagged credential field that
// actually exists) and un-redacted nothing.
func TestAuditRedactionTreatsBothSpellingsAlike(t *testing.T) {
	const canary = "LIVE-CREDENTIAL-VALUE"
	redacts := func(key string) bool {
		before := json.RawMessage(fmt.Sprintf(`{%q:%q}`, key, canary))
		out, _ := audit.Redact(before, nil)
		return !strings.Contains(string(out), canary)
	}

	for name, why := range secretJSONNames() {
		goName := goFieldNameFor(name)
		if goName == name {
			continue // single-word names have one spelling
		}
		if !redacts(goName) {
			t.Errorf(`
audit.Redact redacts %q but not %q — the same credential under the two
spellings the same field can serialize as.

  A struct with json tags produces the first. A struct without them produces
  the second: repository.PlatformMFA is exactly that, and holds the bcrypt
  recovery-code hashes.

  why %q is a credential:
    %s`, name, goName, name, why)
		}
	}
}

// TestAuditRedactionSurfaceIsPinned lists every key in this module that
// audit.Redact will strip, and fails when that set changes.
//
// It answers the question the camelCase change raised: does widening the match
// redact something it should not? The substring rules are blunt — "mfa" and
// "pin_" match more than credentials — and normalization injects the
// underscores that make them reachable in a second spelling. That is fine
// while the set stays deliberate, and this is what keeps it deliberate.
//
// A key appearing here means audit_log.before_json / after_json will show
// "[REDACTED]" instead of the value. Losing a field from an audit trail is a
// real cost, so a new entry deserves a look rather than a shrug.
//
// Three entries are broader than the catalogue: mfa_required and
// mfa_expires_at are not credentials (a policy bool and a timestamp), and
// pin_version is a monotonic counter ruled NOT secret. All three predate the
// camelCase change — their snake_case spellings already matched — and are left
// alone here; narrowing the substring list is its own task.
func TestAuditRedactionSurfaceIsPinned(t *testing.T) {
	want := map[string]bool{
		// config / env field names
		"GuestTokenSecret": true, "MFAEncryptionKey": true, "SecretAccessKey": true,
		"WebhookSecrets": true, "mfaEncKey": true,
		// the untagged credential store — the key the camelCase change added
		"RecoveryCodes": true, "SecretEncrypted": true,
		// catalogue secrets, json spellings
		"challenge_hash": true, "password_hash": true, "pin_hash": true,
		"recovery_codes": true, "secret_encrypted": true, "session_token": true,
		"token": true, "token_hash": true,
		// denylist entries that predate all of this
		"access_token": true, "api_key": true, "card_number": true, "card_pan": true,
		"client_secret": true, "csrf_token": true, "current_pin": true,
		"cvc": true, "cvv": true, "expiry": true, "guest_access_token": true,
		"mfa_challenge": true, "mfa_code": true, "new_pin": true, "otp": true,
		"otp_code": true, "pan": true, "password": true, "payment_signature": true,
		"pin": true, "private_key": true, "recovery_code": true,
		"refresh_token": true, "secret": true, "signature": true,
		"webhook_signature": true, "x-payment-signature": true,
		"x_payment_signature": true,
		// broader than the catalogue; see the note above
		"MfaRequired": true, "mfa_required": true, "mfa_expires_at": true,
		"pin_version": true,
	}

	const canary = "LIVE-CREDENTIAL-VALUE"
	keys := moduleKeyUniverse(t)
	if len(keys) < 500 {
		t.Fatalf("only %d candidate keys found; the scan is broken, not the schema", len(keys))
	}

	got := map[string]bool{}
	for _, k := range keys {
		raw := json.RawMessage(fmt.Sprintf(`{%q:%q}`, k, canary))
		out, _ := audit.Redact(raw, nil)
		if !strings.Contains(string(out), canary) {
			got[k] = true
		}
	}

	var added, removed []string
	for k := range got {
		if !want[k] {
			added = append(added, k)
		}
	}
	for k := range want {
		if !got[k] {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)

	for _, k := range added {
		t.Errorf(`audit.Redact now strips %q, which it did not before.

  normalized form: %q
  Anything under this key is replaced with "[REDACTED]" in
  audit_log.before_json / after_json.

  If it is a credential, good — add it to want in this test. If it is not, a
  blunt rule in internal/audit/redaction.go caught it by accident and an audit
  trail just lost a field; narrow the rule instead.`, k, snakeCase(k))
	}
	for _, k := range removed {
		t.Errorf(`audit.Redact no longer strips %q.

  Either the field was renamed or deleted (drop it from want), or a redaction
  rule was narrowed and a credential is now written to audit_log in the clear.`, k)
	}

	t.Logf("%d of %d module keys are redacted by audit.Redact", len(got), len(keys))
}

// goFieldNameFor turns a catalogue json name into the Go field name an untagged
// struct would serialize under: session_token -> SessionToken.
func goFieldNameFor(jsonName string) string {
	var b strings.Builder
	for _, seg := range splitName(jsonName) {
		if seg == "" {
			continue
		}
		b.WriteString(strings.ToUpper(seg[:1]))
		b.WriteString(seg[1:])
	}
	return b.String()
}
