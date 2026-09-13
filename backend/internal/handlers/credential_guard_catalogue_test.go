package handlers

// Credential leak guard, part 1 of 4: what counts as a secret.
//
// Three leaks of one shape have been found by accident in this codebase, each
// after it shipped:
//
//   - SESSION_CREATED published sqlc.Session whole — to the WebSocket wire, to
//     event_log, and to session_events (F-27).
//   - GET /branches/:id/sessions/active returned it to every staff client,
//     kitchen included, because SessionWithTable embeds sqlc.Session.
//   - POST /sessions/:id/force-close returned it to the closing manager,
//     because ForceCloseResult.Session is a bare sqlc.Session.
//
// The mechanism is always the same: an sqlc row reaches a wire surface and
// silently contributes every json tag it carries, session_token included.
//
// This file answers "which field names are credentials?" and — this is the
// point — it derives the candidate set from the generated sqlc models rather
// than from a list someone typed. If a migration adds a column whose name is
// secret-shaped, sqlc regenerates, and this test fails until a human rules on
// it. A hand-written list would have gone stale silently.
//
// credential_guard_static_test.go uses the rulings to decide which types may
// reach a wire surface. credential_guard_runtime_test.go uses them to inspect
// real payloads.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// secretWords are the snake_case segments that make a field name
// "secret-shaped" — worth a human ruling, not necessarily a secret.
//
// Segment matching, not substring matching: "monkey" must not match "key" and
// "shipping" must not match "pin". Being over-inclusive here is free (the cost
// is one ruling below); being under-inclusive is how a credential ships.
var secretWords = map[string]bool{
	"auth": true, "cert": true, "certificate": true, "cred": true,
	"credential": true, "credentials": true, "cvc": true, "cvv": true,
	"hash": true, "key": true, "keys": true, "mfa": true, "nonce": true,
	"otp": true, "pan": true, "passwd": true, "password": true, "pin": true,
	"private": true, "pwd": true, "recovery": true, "salt": true, "secret": true,
	"seed": true, "sig": true, "signature": true, "token": true, "tokens": true,
	"totp": true, "2fa": true,
}

// isSecretShaped reports whether a field name deserves a ruling. It accepts
// both snake_case json names and CamelCase Go names.
func isSecretShaped(name string) bool {
	for _, seg := range splitName(name) {
		if secretWords[seg] {
			return true
		}
	}
	return false
}

// splitName lowercases and splits a name into segments on underscores and
// camelCase boundaries: "QrCodeToken" and "qr_code_token" both yield
// [qr code token].
func splitName(name string) []string {
	var segs []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			segs = append(segs, strings.ToLower(cur.String()))
			cur.Reset()
		}
	}
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == '.':
			flush()
		case r >= 'A' && r <= 'Z':
			// Break before an uppercase run's start, and before the last
			// uppercase of a run followed by lowercase ("QRToken" -> qr, token).
			if i > 0 && (runes[i-1] < 'A' || runes[i-1] > 'Z') {
				flush()
			} else if i > 0 && i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
				flush()
			}
			cur.WriteRune(r)
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return segs
}

// ruling is a decision about one secret-shaped json field name, with the
// evidence behind it. The why is not decoration: the previous sweep reached
// several of these conclusions and the next person has to be able to check
// them without re-deriving the whole thing.
type ruling struct {
	secret bool
	why    string
}

// secretRulings classifies every secret-shaped json field name that appears in
// the generated sqlc models, plus the ones that appear only in hand-written
// wire types. Two tests keep it honest:
//
//   - TestSecretCatalogue_CoversEverySqlcField fails if sqlc grows a
//     secret-shaped field with no ruling here.
//   - TestNoSecretReachesAWireSurface fails if a secret-shaped name with no
//     ruling here reaches an HTTP, WebSocket or persisted-payload surface.
//
// Verified against the code, not inherited from the earlier sweep — see each
// why for where the evidence is.
var secretRulings = map[string]ruling{
	// ── credentials ──────────────────────────────────────────────────────────
	"session_token":    {true, "sessions.session_token: the guest bearer credential for a table's session. All three known leaks were this field."},
	"pin_hash":         {true, "staff.pin_hash: bcrypt verifier for a staff PIN (services/staff.go hashes with bcrypt on create and compares on auth)."},
	"password_hash":    {true, "platform_users.password_hash: bcrypt verifier for a platform operator password."},
	"token_hash":       {true, "staff_sessions/platform_sessions.token_hash: sha256 of the live bearer token, and the row's lookup key. A verifier for a credential that is still valid."},
	"challenge_hash":   {true, "platform_mfa_challenges.challenge_hash: sha256 of the ephemeral MFA challenge and its lookup key (services/platform_mfa.go IssueMFAChallenge/CompleteMFAChallenge)."},
	"secret_encrypted": {true, "platform_user_mfa.secret_encrypted: the TOTP shared secret at rest. crypto.DecryptSecret turns it back into the standing second factor."},
	"recovery_codes":   {true, "platform_user_mfa.recovery_codes: bcrypt(cost 12) hashes of MFA recovery codes (services/platform_mfa.go generateRecoveryCodes stores hashes, returns plaintext). See the exception for the one-time enrollment response."},
	"token":            {true, "Bare `token` on a hand-written wire type is a live bearer credential (services.StaffSession.Token, services.PlatformSession.Token). Legitimate only in the response that mints it — see wireInventory."},

	// ── not credentials ──────────────────────────────────────────────────────
	"qr_code_token":      {false, "tables.qr_code_token is the content of the QR sticker on the physical table, resolved by the PUBLIC route GET /api/tables/by-qr/:token (server.go:261). It is published by construction, and rotatable (PATCH /staff/tables/:id/qr-refresh)."},
	"mfa_required":       {false, "platform_users.mfa_required: a bool policy flag saying whether this operator must enroll a second factor. Says nothing about the factor itself."},
	"credential_version": {false, "session_participants.credential_version: a monotonic int32 revocation counter. It is the mechanism that invalidates guest credentials, not a credential; clients already carry it in their WebSocket ticket."},
	"token_version":      {false, "staff.token_version: monotonic int32 revocation counter, same role as credential_version."},
	"pin_version":        {false, "staff.pin_version: monotonic int32 counter bumped on PIN rotation. Not derived from the PIN."},
	"row_hash":           {false, "audit_log.row_hash: SHA-256 over canonical row fields for tamper-evidence (migration 000019_audit_log_v2). Exposed on purpose through the platform audit read API so an auditor can verify the chain. Still NULL in this phase."},
	"previous_hash":      {false, "audit_log.previous_hash: the predecessor row's row_hash, same chain, same reasoning."},
	"request_hash":       {false, "idempotency_keys.request_hash: sha256 of the request body, used to detect a key replayed with a different body. Not invertible and unlocks nothing."},
	"idempotency_key":    {false, "Caller-supplied dedup handle, scoped by (scope_type, scope_id, actor_type, actor_id) — see GetOrderByScopedIdempotencyKeyParams. Possession alone grants nothing."},
	"entitlement_key":    {false, "A plan capability name such as `loyalty.enabled`. Config identifier."},
	"flag_key":           {false, "A feature flag name. Config identifier."},
	"key":                {false, "entitlements.key / platform_feature_flags.key / theme_presets.key — the primary key of a config row, and idempotency_keys.key, the caller's own dedup handle."},
	"tokens_json":        {false, "tenant_themes.tokens_json: design tokens (colours, radii, fonts). Name collision with credential `token` only."},
	"challenge":          {false, "services.PlatformMFAChallenge.Challenge: the plaintext ephemeral challenge, returned to the caller who just proved their password. The caller's own credential, and the hash is what is stored."},
	"otpauth_uri":        {false, "services.PlatformMFASetup.OTPAuthURI embeds the TOTP secret, but the enrollment response is the only way the operator can provision their authenticator. Shown once, by design. Ruled with `secret` below."},
	"secret":             {false, "services.PlatformMFASetup.Secret: the TOTP secret at enrollment. Shown exactly once to the enrolling operator by design; there is no other channel for it."},
	"private_ip":         {false, "Diagnostic address field in the observability payloads. Not a credential."},
}

// secretJSONNames returns the ruled-secret field names. Used by the static and
// runtime guards.
func secretJSONNames() map[string]string {
	out := map[string]string{}
	for name, r := range secretRulings {
		if r.secret {
			out[name] = r.why
		}
	}
	return out
}

// ── derivation from the generated models ─────────────────────────────────────

// sqlcField is one struct field in the generated sqlc package.
type sqlcField struct {
	Struct   string
	GoName   string
	JSONName string
	GoType   string
	File     string
	Line     int
}

// scanSqlcFields parses every generated file in internal/db/sqlc and returns
// each struct field. This is the authority for "what columns exist": the sqlc
// models are generated from the migrations, so anything in the schema that Go
// can serialize shows up here.
func scanSqlcFields(t *testing.T) []sqlcField {
	t.Helper()
	dir := filepath.Join(moduleRoot(t), "internal", "db", "sqlc")
	entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no generated sqlc files under %s (err=%v)", dir, err)
	}

	fset := token.NewFileSet()
	var out []sqlcField
	for _, path := range entries {
		af, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, fld := range st.Fields.List {
				jsonName := ""
				if fld.Tag != nil {
					if unquoted, err := strconv.Unquote(fld.Tag.Value); err == nil {
						jsonName = strings.Split(reflect.StructTag(unquoted).Get("json"), ",")[0]
					}
				}
				for _, nm := range fld.Names {
					jn := jsonName
					if jn == "" {
						jn = nm.Name
					}
					pos := fset.Position(nm.Pos())
					out = append(out, sqlcField{
						Struct:   ts.Name.Name,
						GoName:   nm.Name,
						JSONName: jn,
						GoType:   exprText(fld.Type),
						File:     filepath.Base(pos.Filename),
						Line:     pos.Line,
					})
				}
			}
			return true
		})
	}
	if len(out) == 0 {
		t.Fatal("scanned the sqlc package and found no struct fields — the scanner is broken, not the schema")
	}
	return out
}

// TestSecretCatalogue_CoversEverySqlcField is the tripwire on the schema. Add a
// column named *_token, *_hash, *_secret, *_key and this fails until someone
// writes down whether it is a credential.
func TestSecretCatalogue_CoversEverySqlcField(t *testing.T) {
	fields := scanSqlcFields(t)

	// json name -> where it was seen, for the failure message.
	shaped := map[string][]sqlcField{}
	for _, f := range fields {
		if isSecretShaped(f.JSONName) || isSecretShaped(f.GoName) {
			shaped[f.JSONName] = append(shaped[f.JSONName], f)
		}
	}

	var unruled []string
	for name := range shaped {
		if _, ok := secretRulings[name]; !ok {
			unruled = append(unruled, name)
		}
	}
	sort.Strings(unruled)

	for _, name := range unruled {
		f := shaped[name][0]
		var where []string
		seen := map[string]bool{}
		for _, occ := range shaped[name] {
			if !seen[occ.Struct] {
				seen[occ.Struct] = true
				where = append(where, occ.Struct)
			}
		}
		sort.Strings(where)
		t.Errorf(`unclassified secret-shaped sqlc field: %q

  Go field:   sqlc.%s.%s (%s)
  declared:   internal/db/sqlc/%s:%d
  appears in: %s

  A column with this name reached the generated models and nobody has said
  whether it is a credential. Add an entry to secretRulings in
  credential_guard_catalogue_test.go:

      %q: {true,  "<why this is a credential>"},
   or %q: {false, "<why this is safe on a wire, with the evidence>"},

  If it IS a credential, also check every wire type that carries this row —
  TestNoSecretReachesAWireSurface will tell you which ones.`,
			name, f.Struct, f.GoName, f.GoType, f.File, f.Line, strings.Join(where, ", "), name, name)
	}
}

// TestSecretCatalogue_HasNoDeadRulings keeps the table from accumulating
// entries for fields that no longer exist. A stale ruling is a small lie about
// the schema, and the next person will trust it.
func TestSecretCatalogue_HasNoDeadRulings(t *testing.T) {
	// Names ruled on here but sourced from hand-written wire types rather than
	// sqlc. They are exercised by the static guard, not by this test.
	nonSqlc := map[string]bool{
		"token": true, "challenge": true, "secret": true,
		"otpauth_uri": true, "private_ip": true,
	}

	inSqlc := map[string]bool{}
	for _, f := range scanSqlcFields(t) {
		inSqlc[f.JSONName] = true
	}
	var dead []string
	for name := range secretRulings {
		if !inSqlc[name] && !nonSqlc[name] {
			dead = append(dead, name)
		}
	}
	sort.Strings(dead)
	if len(dead) > 0 {
		t.Errorf("secretRulings has entries for fields that exist neither in sqlc nor in the "+
			"hand-written wire types: %s\n\nDrop them, or add them to the nonSqlc set above with a reason.",
			strings.Join(dead, ", "))
	}
}

// TestSecretCatalogue_Report prints the derived catalogue. Run with -v when you
// want to see what the guard is actually enforcing:
//
//	go test ./internal/handlers -run TestSecretCatalogue_Report -v
func TestSecretCatalogue_Report(t *testing.T) {
	fields := scanSqlcFields(t)
	byName := map[string][]sqlcField{}
	for _, f := range fields {
		byName[f.JSONName] = append(byName[f.JSONName], f)
	}

	var shaped []string
	for name := range byName {
		if isSecretShaped(name) {
			shaped = append(shaped, name)
		}
	}
	sort.Strings(shaped)

	var secret, notSecret []string
	for _, name := range shaped {
		if secretRulings[name].secret {
			secret = append(secret, name)
		} else {
			notSecret = append(notSecret, name)
		}
	}

	t.Logf("scanned %d struct fields in internal/db/sqlc; %d distinct secret-shaped json names", len(fields), len(shaped))
	t.Log("")
	t.Logf("SECRET (%d) — must never reach a wire surface:", len(secret))
	for _, name := range secret {
		t.Logf("  %-20s %s", name, structsFor(byName[name]))
		t.Logf("  %-20s   %s", "", secretRulings[name].why)
	}
	t.Log("")
	t.Logf("NOT SECRET (%d) — secret-shaped, ruled safe:", len(notSecret))
	for _, name := range notSecret {
		t.Logf("  %-20s %s", name, structsFor(byName[name]))
		t.Logf("  %-20s   %s", "", secretRulings[name].why)
	}
}

func structsFor(fields []sqlcField) string {
	seen := map[string]bool{}
	var names []string
	for _, f := range fields {
		if !seen[f.Struct] {
			seen[f.Struct] = true
			names = append(names, f.Struct)
		}
	}
	sort.Strings(names)
	if len(names) > 5 {
		return fmt.Sprintf("%s … (+%d more)", strings.Join(names[:5], ", "), len(names)-5)
	}
	return strings.Join(names, ", ")
}

// exprText renders a type expression for a failure message.
func exprText(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprText(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprText(t.X)
	case *ast.ArrayType:
		return "[]" + exprText(t.Elt)
	case *ast.MapType:
		return "map[" + exprText(t.Key) + "]" + exprText(t.Value)
	}
	return "?"
}

// moduleRoot walks up from the test's working directory to the directory
// holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
