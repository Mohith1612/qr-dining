package handlers

// Credential leak guard, part 3 of 4: which types reach a wire.
//
// Requirement: cover all three surfaces the SESSION_CREATED leak reached at
// once. It published sqlc.Session to the WebSocket fan-out AND to event_log AND
// to session_events; a guard that only read HTTP bodies would have caught none
// of the three.
//
//	HTTP       — (*gin.Context).JSON and friends
//	WEBSOCKET  — the events.Publisher methods and ws.NewEnvelope
//	PERSISTED  — Repos.LogEvent (event_log) and Repos.AppendSessionEvent
//	             (session_events)
//
// audit_log.before_json / after_json is a fourth surface, defended by a
// denylist rather than a projection and opaque to a type walk. It is checked
// separately by TestAuditRedactionCoversEverySecret in part 4.
//
// What this part does: resolve the static type of every payload expression at
// those call sites and walk it for secret-shaped fields. It is a tripwire on
// EXPOSURE SURFACE — the moment a new type that can carry a credential reaches
// a wire, this fails, whether or not any test ever exercises that route. That
// is the failure mode described as "a future struct embed reintroduces the leak
// with nothing to catch it".
//
// What this part deliberately does NOT do: prove that the existing sites scrub.
// It cannot. Every fix in this codebase is value-level — guestSafeSession and
// credentialSafeSession return the same sqlc.Session type with the field zeroed
// — so the type is identical before and after the fix and a type-level check
// cannot tell them apart. Deleting a scrubber call is caught by the runtime
// half, in credential_guard_runtime_test.go.
//
// The static and runtime parts are complementary and neither is redundant:
//
//	new type reaches a wire, never exercised  -> static catches, runtime cannot
//	scrubber deleted from an existing site    -> runtime catches, static cannot

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ── surfaces ─────────────────────────────────────────────────────────────────

const (
	surfaceHTTP      = "HTTP"
	surfaceWebsocket = "WEBSOCKET"
	surfacePersisted = "PERSISTED"
)

// ginContext is the receiver every HTTP response method hangs off.
const ginContext = "github.com/gin-gonic/gin.Context"

// matchChokepoint decides whether a resolved call is a wire surface, and which
// argument carries the payload. Matching is on the resolved receiver and
// method, not on source text, so a renamed import or an aliased helper is still
// caught.
//
// Returns (surface, payload arg index, what the surface is, ok).
func matchChokepoint(fn *types.Func, call *ast.CallExpr) (string, int, string, bool) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return "", 0, "", false
	}
	recv := ""
	if r := sig.Recv(); r != nil {
		rt := r.Type()
		if p, isPtr := rt.(*types.Pointer); isPtr {
			rt = p.Elem()
		}
		recv = types.TypeString(rt, nil)
	}

	switch {
	// ── HTTP response bodies ─────────────────────────────────────────────────
	case recv == ginContext:
		switch fn.Name() {
		case "JSON", "IndentedJSON", "PureJSON", "AsciiJSON", "SecureJSON":
			if len(call.Args) == 2 {
				return surfaceHTTP, 1, "HTTP response body", true
			}
		case "AbortWithStatusJSON":
			if len(call.Args) == 2 {
				return surfaceHTTP, 1, "HTTP response body (abort)", true
			}
		case "JSONP":
			if len(call.Args) == 2 {
				return surfaceHTTP, 1, "HTTP response body (jsonp)", true
			}
		}

	// ── WebSocket fan-out ────────────────────────────────────────────────────
	//
	// Matched structurally: every publish method is (ctx, uuid.UUID, any), so a
	// new event type added to events.Publisher is covered the day it is written
	// without anyone remembering to update this list.
	case strings.HasSuffix(recv, "/internal/events.Publisher"):
		if sig.Params().Len() == 3 && isEmptyInterface(sig.Params().At(2).Type()) && len(call.Args) == 3 {
			return surfaceWebsocket, 2, "WebSocket event payload (" + fn.Name() + ")", true
		}

	case strings.HasSuffix(recv, "/internal/websocket.") || fn.Name() == "NewEnvelope":
		if fn.Pkg() != nil && strings.HasSuffix(fn.Pkg().Path(), "/internal/websocket") && len(call.Args) == 3 {
			return surfaceWebsocket, 2, "WebSocket envelope payload", true
		}

	// ── persisted payloads ───────────────────────────────────────────────────
	//
	// Matched by method name across any receiver declared in this module, because
	// both of these are also reached through interfaces: events.EventStore wraps
	// AppendSessionEvent and the worker declares its own LogEvent interface.
	case fn.Name() == "LogEvent" && inThisModule(fn):
		if idx := lastEmptyInterfaceParam(sig); idx >= 0 && len(call.Args) > idx {
			return surfacePersisted, idx, "event_log.payload (persisted, replayed by GET /sessions/:id/events and GET /branches/:id/events)", true
		}

	case fn.Name() == "AppendSessionEvent" && inThisModule(fn):
		if idx := lastEmptyInterfaceParam(sig); idx >= 0 && len(call.Args) > idx {
			return surfacePersisted, idx, "session_events.payload (persisted, replayed by snapshot.MissedEvents)", true
		}
	}
	return "", 0, "", false
}

func inThisModule(fn *types.Func) bool {
	return fn.Pkg() != nil && strings.HasPrefix(fn.Pkg().Path(), "github.com/Mohith1612/qr-dining/")
}

func isEmptyInterface(t types.Type) bool {
	i, ok := t.Underlying().(*types.Interface)
	return ok && i.NumMethods() == 0
}

func lastEmptyInterfaceParam(sig *types.Signature) int {
	for i := sig.Params().Len() - 1; i >= 0; i-- {
		if isEmptyInterface(sig.Params().At(i).Type()) {
			return i
		}
	}
	return -1
}

// ── the scan ─────────────────────────────────────────────────────────────────

// finding is one secret-shaped field reachable from one payload type at one
// surface. Findings are keyed without the source position: a type moving down a
// file is not a new leak, and an inventory keyed on line numbers would churn.
type finding struct {
	Surface     string
	SurfaceDesc string
	PayloadType string // full import path, for a unique key
	ShortType   string // for humans
	Field       string
	JSONPath    string
	GoPath      string
	Sites       []string // file:line of every call site that reaches it
}

func (f finding) key() string {
	return f.Surface + "|" + f.PayloadType + "|" + f.JSONPath
}

// payloadRef is a concrete payload type resolved at a call site, with the json
// prefix it sits under. gin.H{"session": x} gives x a prefix of "$.session".
type payloadRef struct {
	Type      types.Type
	Prefix    string
	Forwarder bool // the payload is this function's own `any` parameter
}

// resolveCtx carries what it takes to resolve a payload expression: the module
// (to follow calls into other packages), the package's type info, and the
// enclosing function (to follow local variables).
type resolveCtx struct {
	mod  *guardModule
	pkg  *guardPackage
	encl *ast.FuncDecl
	seen map[string]bool
}

// payloads turns a payload argument expression into the concrete types it
// contributes. Four descents matter, and each exists because the static type at
// the call site is uninformative — which is the whole reason a reflection-only
// guard cannot work here:
//
//  1. Composite map literals. gin.H is map[string]any, so gin.H{"session": s}
//     has element type `any`. 82 of the 157 HTTP call sites are this shape.
//  2. Local variables. `payload := map[string]any{"order": order}` then
//     publish(..., payload) — the name is opaque, the literal is not.
//     (internal/services/order.go:442, internal/worker/worker.go:387)
//  3. Calls returning an opaque type. platformStaffResponse(staff) returns
//     gin.H; the projection that makes it safe is inside the callee. Without
//     following the call, changing that body to gin.H{"staff": staff} would
//     ship staff.pin_hash with the guard reporting nothing.
//  4. Forwarders. events.Publisher.publish hands its own `any` parameter to
//     the store; the payload originates at the caller, which is scanned
//     separately, so this is noted and not counted as a gap.
func (rc *resolveCtx) payloads(expr ast.Expr, prefix string, depth int) []payloadRef {
	if depth > 8 {
		return nil
	}
	switch e := expr.(type) {
	case *ast.CompositeLit:
		if refs, ok := rc.compositeLit(e, prefix, depth); ok {
			return refs
		}
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return rc.payloads(e.X, prefix, depth+1)
		}
	case *ast.ParenExpr:
		return rc.payloads(e.X, prefix, depth+1)
	case *ast.CallExpr:
		if refs, followed := rc.followCall(e, prefix, depth); followed {
			return refs
		}
	case *ast.Ident:
		if refs, followed := rc.followLocal(e, prefix, depth); followed {
			return refs
		}
	}

	tv, ok := rc.pkg.Info.Types[expr]
	if !ok || tv.Type == nil {
		return nil
	}
	return []payloadRef{{Type: tv.Type, Prefix: prefix}}
}

func (rc *resolveCtx) compositeLit(e *ast.CompositeLit, prefix string, depth int) ([]payloadRef, bool) {
	tv, ok := rc.pkg.Info.Types[e]
	if !ok {
		return nil, false
	}
	switch tv.Type.Underlying().(type) {
	case *types.Map:
		var out []payloadRef
		for _, elt := range e.Elts {
			kv, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}
			key := "*"
			if lit, isLit := kv.Key.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
				if unquoted, err := strconv.Unquote(lit.Value); err == nil {
					key = unquoted
				}
			}
			out = append(out, rc.payloads(kv.Value, prefix+"."+key, depth+1)...)
		}
		return out, true
	case *types.Slice, *types.Array:
		var out []payloadRef
		for _, elt := range e.Elts {
			out = append(out, rc.payloads(elt, prefix+"[]", depth+1)...)
		}
		return out, true
	}
	return nil, false
}

// isOpaqueType reports whether a type's declaration says nothing about what
// will be serialized — an `any`, or a container of `any`.
func isOpaqueType(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Interface:
		return u.NumMethods() == 0
	case *types.Map:
		return isOpaqueType(u.Elem())
	case *types.Slice:
		return isOpaqueType(u.Elem())
	case *types.Array:
		return isOpaqueType(u.Elem())
	case *types.Pointer:
		return isOpaqueType(u.Elem())
	}
	return false
}

func (rc *resolveCtx) exprType(e ast.Expr) types.Type {
	if tv, ok := rc.pkg.Info.Types[e]; ok {
		return tv.Type
	}
	return nil
}

// followCall resolves a call to one of our own functions whose result is
// opaque, by looking at what the body actually builds.
func (rc *resolveCtx) followCall(call *ast.CallExpr, prefix string, depth int) ([]payloadRef, bool) {
	var name *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		name = fn
	case *ast.SelectorExpr:
		name = fn.Sel
	default:
		return nil, false
	}
	obj, ok := rc.pkg.Info.Uses[name].(*types.Func)
	if !ok || !inThisModule(obj) {
		return nil, false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Results().Len() == 0 || !isOpaqueType(sig.Results().At(0).Type()) {
		return nil, false
	}
	if rc.seen[obj.String()] {
		return nil, true // recursive helper; stop rather than loop
	}
	rc.seen[obj.String()] = true
	defer delete(rc.seen, obj.String())

	decl, declPkg := rc.mod.funcDecl(obj)
	if decl == nil || decl.Body == nil {
		return nil, false
	}
	inner := &resolveCtx{mod: rc.mod, pkg: declPkg, encl: decl, seen: rc.seen}

	var out []payloadRef
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		ret, isRet := n.(*ast.ReturnStmt)
		if !isRet || len(ret.Results) == 0 {
			return true
		}
		out = append(out, inner.payloads(ret.Results[0], prefix, depth+1)...)
		return true
	})
	return out, true
}

// followLocal resolves an identifier whose own type is opaque by finding what
// was assigned, appended or indexed into it inside the enclosing function.
// A parameter is instead reported as a forwarder.
func (rc *resolveCtx) followLocal(ident *ast.Ident, prefix string, depth int) ([]payloadRef, bool) {
	t := rc.exprType(ident)
	if t == nil || !isOpaqueType(t) || rc.encl == nil || rc.encl.Body == nil {
		return nil, false
	}
	obj, ok := rc.pkg.Info.Uses[ident].(*types.Var)
	if !ok {
		return nil, false
	}
	if isParamOf(rc.encl, rc.pkg, obj) {
		return []payloadRef{{Type: t, Prefix: prefix, Forwarder: true}}, true
	}

	key := "local:" + obj.String() + "@" + fmt.Sprint(obj.Pos())
	if rc.seen[key] {
		return nil, true
	}
	rc.seen[key] = true
	defer delete(rc.seen, key)

	var out []payloadRef
	ast.Inspect(rc.encl.Body, func(n ast.Node) bool {
		assign, isAssign := n.(*ast.AssignStmt)
		if !isAssign {
			return true
		}
		for i, lhs := range assign.Lhs {
			if i >= len(assign.Rhs) {
				// `a, b := f()` — nothing literal to descend into.
				continue
			}
			switch target := lhs.(type) {
			case *ast.Ident:
				// payload := map[string]any{…}  /  payload = append(payload, …)
				if rc.pkg.Info.Uses[target] != obj && rc.pkg.Info.Defs[target] != obj {
					continue
				}
				if call, isCall := assign.Rhs[i].(*ast.CallExpr); isCall {
					if fn, isIdent := call.Fun.(*ast.Ident); isIdent {
						if fn.Name == "append" && len(call.Args) > 1 {
							for _, arg := range call.Args[1:] {
								out = append(out, rc.payloads(arg, prefix+"[]", depth+1)...)
							}
							continue
						}
						// `out := make([]gin.H, 0, n)` allocates; it carries no
						// payload, and treating it as one would re-open the very
						// blind spot the append below closes.
						if _, isBuiltin := rc.pkg.Info.Uses[fn].(*types.Builtin); isBuiltin {
							continue
						}
					}
				}
				out = append(out, rc.payloads(assign.Rhs[i], prefix, depth+1)...)
			case *ast.IndexExpr:
				// out[i] = gin.H{…}  /  out["key"] = value
				base, isIdent := target.X.(*ast.Ident)
				if !isIdent || rc.pkg.Info.Uses[base] != obj {
					continue
				}
				itemPrefix := prefix + "[]"
				if lit, isLit := target.Index.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
					if unquoted, err := strconv.Unquote(lit.Value); err == nil {
						itemPrefix = prefix + "." + unquoted
					}
				}
				out = append(out, rc.payloads(assign.Rhs[i], itemPrefix, depth+1)...)
			}
		}
		return true
	})
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func isParamOf(decl *ast.FuncDecl, pkg *guardPackage, obj types.Object) bool {
	lists := []*ast.FieldList{decl.Type.Params, decl.Recv}
	for _, fl := range lists {
		if fl == nil {
			continue
		}
		for _, field := range fl.List {
			for _, name := range field.Names {
				if pkg.Info.Defs[name] == obj {
					return true
				}
			}
		}
	}
	return false
}

type scanResult struct {
	Findings   []finding
	BlindSpots map[string][]string // reason -> sites
	Forwarders []string            // sites that hand on a caller's payload
	CallSites  int
}

// scanWireSurfaces walks every call site on every surface and reports the
// secret-shaped fields reachable from the payload types.
func scanWireSurfaces(t *testing.T) scanResult {
	t.Helper()
	mod := loadModule(t)
	secrets := secretJSONNames()

	byKey := map[string]*finding{}
	blind := map[string]map[string]bool{}
	forwarders := map[string]bool{}
	sites := 0

	for _, pkg := range mod.Packages {
		for _, file := range pkg.Files {
			// Track the enclosing function so a payload held in a local
			// variable can be resolved back to what was put in it.
			var encl *ast.FuncDecl
			ast.Inspect(file, func(n ast.Node) bool {
				if fd, isFunc := n.(*ast.FuncDecl); isFunc {
					encl = fd
					return true
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				fn, ok := pkg.Info.Uses[sel.Sel].(*types.Func)
				if !ok {
					return true
				}
				surface, argIdx, desc, matched := matchChokepoint(fn, call)
				if !matched {
					return true
				}
				sites++
				pos := mod.Fset.Position(call.Lparen)
				site := fmt.Sprintf("%s:%d", trimRoot(pos.Filename), pos.Line)

				rc := &resolveCtx{mod: mod, pkg: pkg, encl: encl, seen: map[string]bool{}}
				for _, ref := range rc.payloads(call.Args[argIdx], "$", 0) {
					if ref.Forwarder {
						forwarders[site] = true
						continue
					}
					res := walkTypeForSecrets(ref.Type, secrets)
					for _, e := range res.Exposures {
						f := finding{
							Surface:     surface,
							SurfaceDesc: desc,
							PayloadType: fullType(ref.Type),
							ShortType:   shortType(ref.Type),
							Field:       e.Field,
							JSONPath:    strings.Replace(e.JSONPath, "$", ref.Prefix, 1),
							GoPath:      e.GoPath,
						}
						if existing, seen := byKey[f.key()]; seen {
							existing.Sites = appendUnique(existing.Sites, site)
						} else {
							f.Sites = []string{site}
							byKey[f.key()] = &f
						}
					}
					for _, b := range res.BlindSpots {
						if blind[b.Reason] == nil {
							blind[b.Reason] = map[string]bool{}
						}
						blind[b.Reason][site] = true
					}
				}
				return true
			})
		}
	}

	out := scanResult{BlindSpots: map[string][]string{}, CallSites: sites}
	for _, f := range byKey {
		sort.Strings(f.Sites)
		out.Findings = append(out.Findings, *f)
	}
	sort.Slice(out.Findings, func(i, j int) bool {
		if out.Findings[i].Surface != out.Findings[j].Surface {
			return out.Findings[i].Surface < out.Findings[j].Surface
		}
		if out.Findings[i].ShortType != out.Findings[j].ShortType {
			return out.Findings[i].ShortType < out.Findings[j].ShortType
		}
		return out.Findings[i].JSONPath < out.Findings[j].JSONPath
	})
	for reason, siteSet := range blind {
		var list []string
		for s := range siteSet {
			list = append(list, s)
		}
		sort.Strings(list)
		out.BlindSpots[reason] = list
	}
	for site := range forwarders {
		out.Forwarders = append(out.Forwarders, site)
	}
	sort.Strings(out.Forwarders)
	return out
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

func trimRoot(path string) string {
	if i := strings.Index(path, "/backend/"); i >= 0 {
		return path[i+len("/backend/"):]
	}
	return path
}

// TestWireSurfaceReport prints every payload type that can carry a credential.
// Not an assertion — run it when you want the map:
//
//	go test ./internal/handlers -run TestWireSurfaceReport -v
func TestWireSurfaceReport(t *testing.T) {
	res := scanWireSurfaces(t)
	t.Logf("scanned %d wire call sites across %d packages", res.CallSites, len(loadModule(t).Packages))
	t.Logf("%d (surface, type, field) exposures:", len(res.Findings))
	for _, f := range res.Findings {
		t.Logf("  [%s] %s", f.Surface, f.ShortType)
		t.Logf("      field %s at %s", f.Field, f.JSONPath)
		t.Logf("      go:   %s", f.GoPath)
		t.Logf("      at:   %s", strings.Join(f.Sites, ", "))
	}
	t.Log("")
	t.Logf("%d forwarder sites (payload is the function's own `any` parameter; the origin is scanned at the caller):", len(res.Forwarders))
	for _, site := range res.Forwarders {
		t.Logf("  %s", site)
	}
	t.Log("")
	t.Logf("%d blind spots:", len(res.BlindSpots))
	for reason, sites := range res.BlindSpots {
		t.Logf("  %s", reason)
		for _, site := range sites {
			t.Logf("      %s", site)
		}
	}
}

// ── the inventory ────────────────────────────────────────────────────────────

// wireEntry is one accepted (surface, payload type, json path) exposure, with
// what makes it safe. Keyed without a line number so moving code around does
// not churn the table.
type wireEntry struct {
	Surface  string
	Type     string // full import path, as the scanner reports it
	JSONPath string
	Scrubber string // the projection that empties the field, or "" if the value is meant to ship
	Why      string
}

func (e wireEntry) key() string { return e.Surface + "|" + e.Type + "|" + e.JSONPath }

const sessionsPkg = "github.com/Mohith1612/qr-dining/internal/services"
const sqlcPkg = "github.com/Mohith1612/qr-dining/internal/db/sqlc"

// wireInventory is every place a credential-bearing type is allowed to reach a
// wire, and why. Two kinds of entry:
//
//   - Scrubber != "": the type carries the field but the value is emptied
//     before it ships. Whether the scrubber is still being CALLED is not
//     something this file can check — see the note at the top — and is what
//     credential_guard_runtime_test.go verifies against real payloads.
//   - Scrubber == "": the value genuinely ships, because the recipient is the
//     party the credential belongs to.
//
// Adding a row here is a deliberate act. If TestNoSecretReachesAWireSurface
// points at a row you are about to add, the question to answer first is whether
// the new type should carry the row at all.
var wireInventory = []wireEntry{
	// ── guest session_token: scrubbed before every wire ──────────────────────
	{
		Surface: surfaceHTTP, Type: sqlcPkg + ".Session", JSONPath: "$.session_token",
		Scrubber: "handlers.guestSafeSession",
		Why:      "GET /sessions/:id and the reactivate route return the session row to the guest. guestSafeSession clears session_token; guests authenticate with their HMAC guest_access_token and never need it (F-8).",
	},
	{
		Surface: surfaceHTTP, Type: sqlcPkg + ".Session", JSONPath: "$.session.session_token",
		Scrubber: "handlers.guestSafeSession",
		Why:      "POST /sessions and POST /sessions/:id/join nest the same scrubbed row under `session`.",
	},
	{
		Surface: surfaceHTTP, Type: sessionsPkg + ".SessionSnapshot", JSONPath: "$.session.session_token",
		Scrubber: "handlers.guestSafeSession",
		Why:      "GET /sessions/:id/snapshot. Reachable without a guest token while AUTH_GUEST_CREDENTIALS_REQUIRED is false, so the strip at snapshot.go is the flag-independent fix (F-8). Note this covers snapshot.Session only — see TestWireSurfaceGaps for MissedEvents.",
	},
	{
		Surface: surfaceHTTP, Type: "[]" + sessionsPkg + ".SessionWithTable", JSONPath: "$[].session_token",
		Scrubber: "services.credentialSafeSession",
		Why:      "GET /branches/:id/sessions/active. SessionWithTable EMBEDS sqlc.Session, so every field lands as a top-level key — this is leak #2, and the reason it reached kitchen clients that need a table identifier and nothing else.",
	},
	{
		Surface: surfaceHTTP, Type: sessionsPkg + ".ForceCloseResult", JSONPath: "$.session.session_token",
		Scrubber: "services.credentialSafeSession",
		Why:      "POST /sessions/:id/force-close echoes the closed session to the manager who ended it — leak #3.",
	},
	{
		Surface: surfaceWebsocket, Type: sessionsPkg + ".CreateSessionResult", JSONPath: "$.session.session_token",
		Scrubber: "services.credentialSafeResult",
		Why:      "SESSION_CREATED fan-out — leak #1, the one that hit all three surfaces at once.",
	},
	{
		Surface: surfacePersisted, Type: sessionsPkg + ".CreateSessionResult", JSONPath: "$.session.session_token",
		Scrubber: "services.credentialSafeResult",
		Why:      "The same SESSION_CREATED payload written to event_log, where branch staff can read it back through GET /sessions/:id/events (F-27).",
	},

	// ── credentials that genuinely ship, to their own owner ──────────────────
	{
		Surface: surfaceHTTP, Type: sessionsPkg + ".StaffSession", JSONPath: "$.token",
		Why: "POST /staff/auth response. The bearer token just minted for the caller who passed the PIN check; there is no other channel for it.",
	},
	{
		Surface: surfaceHTTP, Type: sessionsPkg + ".StaffSession", JSONPath: "$.session_token",
		Why: "Same response, same value: services.StaffSession sets Token and SessionToken to the identical string (services/staff.go createSession) and serializes both for client compatibility. Worth knowing that a second field with this name exists — it is NOT the guest sessions.session_token, and services.StaffSession reaches exactly one response (staff.go:121). If it ever reaches a second, this test will say so.",
	},
	{
		Surface: surfaceHTTP, Type: sessionsPkg + ".PlatformSession", JSONPath: "$.token",
		Why: "POST /platform/auth/mfa response — the platform session minted after the second factor.",
	},
	{
		Surface: surfaceHTTP, Type: "*" + sessionsPkg + ".PlatformSession", JSONPath: "$.token",
		Why: "POST /platform/auth response on the no-MFA path (PlatformAuthResult.Session is a pointer).",
	},
	{
		Surface: surfaceHTTP, Type: sessionsPkg + ".PlatformMFAEnrollmentResult", JSONPath: "$.recovery_codes",
		Why: "POST /platform/mfa/confirm. The plaintext recovery codes, shown exactly once at enrollment; only their bcrypt hashes are stored (services/platform_mfa.go generateRecoveryCodes). There is no second chance to display them.",
	},
}

// TestNoSecretReachesAWireSurface is the static half of the guard.
//
// It fails when a credential-bearing type reaches an HTTP response, a WebSocket
// payload or a persisted payload without a row in wireInventory — which is what
// happens the moment somebody embeds an sqlc row in a new response type.
func TestNoSecretReachesAWireSurface(t *testing.T) {
	res := scanWireSurfaces(t)

	known := map[string]wireEntry{}
	for _, e := range wireInventory {
		if dup, exists := known[e.key()]; exists {
			t.Fatalf("wireInventory has two rows for %s (%q and %q)", e.key(), dup.Why, e.Why)
		}
		known[e.key()] = e
	}

	seen := map[string]bool{}
	for _, f := range res.Findings {
		seen[f.key()] = true
		if _, ok := known[f.key()]; ok {
			continue
		}
		t.Errorf(`
A credential can reach a wire surface from a type nobody has signed off on.

  secret field: %s
  surface:      %s — %s
  payload type: %s
  json path:    %s
  go path:      %s
  call site(s): %s

  why %s is a credential:
    %s

  This is the shape of all three known leaks: a type that reaches a wire
  inherits every json tag underneath it, including the ones on an embedded
  sqlc row.

  Fix it, or — if the value genuinely has to ship — add a row to
  wireInventory in credential_guard_static_test.go:

      {
          Surface: %s, Type: %q, JSONPath: %q,
          Scrubber: "<the projection that empties it, or omit if it must ship>",
          Why:      "<why this is safe>",
      },
`,
			f.Field, f.Surface, f.SurfaceDesc, f.PayloadType, f.JSONPath, f.GoPath,
			strings.Join(f.Sites, ", "),
			f.Field, secretRulings[f.Field].why,
			surfaceConst(f.Surface), f.PayloadType, f.JSONPath)
	}

	// A row that no longer matches anything is a stale claim about the code.
	for key, e := range known {
		if !seen[key] {
			t.Errorf(`
wireInventory has a row for an exposure that no longer exists:

  %s
  why it was there: %s

  Either the type stopped carrying the field (good — delete the row), or the
  call site moved somewhere this scan does not look (bad — find out where).`,
				key, e.Why)
		}
	}
}

func surfaceConst(s string) string {
	switch s {
	case surfaceHTTP:
		return "surfaceHTTP"
	case surfaceWebsocket:
		return "surfaceWebsocket"
	case surfacePersisted:
		return "surfacePersisted"
	}
	return "surface" + s
}

// TestWireSurfaceGaps pins what the static scan cannot see, so the gaps are a
// recorded fact rather than something the next reader has to rediscover.
//
// It asserts the gap list does not GROW silently. A new blind-spot category
// means a new kind of opaque payload reached a wire.
func TestWireSurfaceGaps(t *testing.T) {
	res := scanWireSurfaces(t)

	// Each key is a blind-spot reason the scan is known to produce.
	acknowledged := map[string]string{
		"json.RawMessage — a payload serialized upstream, opaque to a type walk": "" +
			"Columns already stored as jsonb (settings_json, metadata_json, support_metadata, " +
			"event_log.payload, session_events.payload) and re-served verbatim. A type walk " +
			"cannot see inside them. Covered instead by the runtime half, which inspects the " +
			"stored bytes: see TestPersistedPayloadsCarryNoCredential.",
		"raw JSON bytes ([]byte) — contents are opaque to a type walk": "" +
			"audit_log.before_json / after_json, served by the platform audit read API. " +
			"Written through audit.Redact, whose own key list is checked by " +
			"TestAuditRedactionCoversEverySecret.",
	}

	for reason, sites := range res.BlindSpots {
		if _, ok := acknowledged[reason]; ok {
			continue
		}
		t.Errorf(`
A new kind of payload reached a wire surface that the type walk cannot see into.

  blind spot: %s
  sites:      %s

  The scan resolved the payload but cannot tell what is inside it, so it can
  neither clear nor condemn these call sites. Decide which it is, then either
  fix the call site or add the reason to the acknowledged map in
  TestWireSurfaceGaps with a note on what covers it instead.`,
			reason, strings.Join(sites, ", "))
	}

	for reason, why := range acknowledged {
		if _, ok := res.BlindSpots[reason]; !ok {
			t.Errorf("acknowledged blind spot %q no longer occurs — delete it.\n  (it was there because: %s)", reason, why)
		}
	}

	t.Logf("static scan covers %d wire call sites; %d forwarders; %d acknowledged blind-spot categories",
		res.CallSites, len(res.Forwarders), len(res.BlindSpots))
}

// TestReplayPathsHaveNoProjection records a structural fact the guard cannot
// close, so that it is a checked statement rather than a comment someone wrote
// once.
//
// Three read paths hand back stored event payloads verbatim:
//
//	GET /sessions/:id/events    c.JSON(200, []sqlc.EventLog)     event_log.go:51
//	GET /branches/:id/events    c.JSON(200, []sqlc.EventLog)     event_log.go:79
//	snapshot.MissedEvents       []ws.Envelope inside the snapshot snapshot.go:65
//
// In each case the bytes on the wire are the bytes in the column. There is no
// projection between them and nothing for a type walk to inspect: the field is
// json.RawMessage, which is opaque by construction.
//
// They are clean today because every writer into those columns is clean, and
// the writers ARE covered — by wireInventory statically and by
// TestPersistedPayloadsCarryNoCredential and
// TestWebSocketPayloadsCarryNoCredential at runtime. That is coverage of the
// writers, not of the readers: a row written by something this guard does not
// see would be replayed unexamined.
//
// This test fails if any of the three gains or loses its verbatim shape, which
// is when this note needs rewriting.
func TestReplayPathsHaveNoProjection(t *testing.T) {
	mod := loadModule(t)
	secrets := secretJSONNames()

	const sqlcPath = "github.com/Mohith1612/qr-dining/internal/db/sqlc"
	const servicesPath = "github.com/Mohith1612/qr-dining/internal/services"

	cases := []struct {
		what     string
		typ      types.Type
		wantPath string
		readers  []string
	}{
		{
			what:     "GET /sessions/:id/events and GET /branches/:id/events replay event_log rows",
			typ:      mod.lookupType(t, sqlcPath, "EventLog"),
			wantPath: "$.payload",
			readers:  []string{"internal/handlers/event_log.go:51", "internal/handlers/event_log.go:79"},
		},
		{
			what:     "snapshot.MissedEvents replays session_events rows",
			typ:      mod.lookupType(t, servicesPath, "SessionSnapshot"),
			wantPath: "$.missed_events[].payload",
			readers:  []string{"internal/handlers/snapshot.go:65"},
		},
	}

	scan := scanWireSurfaces(t)
	rawMessageSites := map[string]bool{}
	for reason, sites := range scan.BlindSpots {
		if !strings.Contains(reason, "json.RawMessage") {
			continue
		}
		for _, s := range sites {
			rawMessageSites[s] = true
		}
	}

	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			paths := blindSpotPaths(tc.typ, secrets)
			if _, ok := paths[tc.wantPath]; !ok {
				var have []string
				for p := range paths {
					have = append(have, p)
				}
				sort.Strings(have)
				t.Errorf(`%s no longer replays verbatim at %s.

  The opaque json.RawMessage this test expects is gone. Either a projection was
  added (good — say so here and drop the case) or the payload moved (find where
  it went, because the guard is no longer looking at it).

  opaque paths now: %s`, shortType(tc.typ), tc.wantPath, strings.Join(have, ", "))
			}
			for _, reader := range tc.readers {
				if !rawMessageSites[reader] {
					t.Errorf("expected %s to serve an opaque json.RawMessage payload, but the scan does not report it there.\n"+
						"  Either the route moved, or it gained a projection — either way this note is now wrong.", reader)
				}
			}
		})
	}

	// snapshot.Session is scrubbed; MissedEvents beside it is not. Pin the
	// asymmetry so it is not mistaken for coverage.
	snapshot := mod.lookupType(t, servicesPath, "SessionSnapshot")
	exposures := walkTypeForSecrets(snapshot, secrets).Exposures
	var sessionCovered bool
	for _, e := range exposures {
		if e.JSONPath == "$.session.session_token" {
			sessionCovered = true
		}
	}
	if !sessionCovered {
		t.Error("SessionSnapshot no longer carries $.session.session_token — guestSafeSession at snapshot.go may have become unnecessary; re-read this test.")
	}
	t.Log("snapshot: $.session is projected through guestSafeSession; $.missed_events[].payload is not projected at all — " +
		"it is clean only because every writer into session_events is clean.")
}

// TestInventoryScrubbersExist keeps the Scrubber column from decaying into a
// comment. Every row that claims a projection makes it safe must name a
// function that is still there — deleting the scrubber outright should not
// leave a table row quietly asserting it runs.
//
// That the named function is still CALLED is a value-level question, and the
// runtime part answers it.
func TestInventoryScrubbersExist(t *testing.T) {
	mod := loadModule(t)

	declared := map[string]bool{} // "handlers.guestSafeSession"
	for _, pkg := range mod.Packages {
		for ident, obj := range pkg.Info.Defs {
			fn, ok := obj.(*types.Func)
			if !ok || fn.Pkg() == nil {
				continue
			}
			if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
				continue
			}
			declared[fn.Pkg().Name()+"."+ident.Name] = true
		}
	}

	for _, e := range wireInventory {
		if e.Scrubber == "" {
			continue
		}
		if !declared[e.Scrubber] {
			t.Errorf(`wireInventory names a scrubber that does not exist: %s

  row:  %s
  why:  %s

  The row claims this projection is what makes the exposure safe. Either the
  function was renamed (update the row) or it was deleted (the exposure is
  now unguarded — this is the bug).`, e.Scrubber, e.key(), e.Why)
		}
	}
}

// TestCredentialStoresNeverReachAWire names the types that exist to HOLD
// credentials and asserts none of them reaches any surface.
//
// The inventory above is an allowlist: it proves nothing about a type that is
// simply absent. Absence can mean "correctly never serialized" or "the scan
// failed to resolve the call site", and those are not the same claim. These
// types are important enough to assert positively.
//
// repository.PlatformMFA is the reason this test exists. It holds the bcrypt
// recovery-code hashes and the encrypted TOTP secret, and it carries NO json
// tags — so if it ever reached a response it would ship as "RecoveryCodes" and
// "SecretEncrypted", not the snake_case names the catalogue is keyed on. The
// walker normalizes for exactly this case; this test is what keeps that honest.
func TestCredentialStoresNeverReachAWire(t *testing.T) {
	mod := loadModule(t)

	const repoPath = "github.com/Mohith1612/qr-dining/internal/repository"
	const sqlcPath = "github.com/Mohith1612/qr-dining/internal/db/sqlc"

	stores := []struct {
		pkg, name, holds string
	}{
		{repoPath, "PlatformMFA", "bcrypt hashes of the MFA recovery codes, and the encrypted TOTP secret"},
		{sqlcPath, "PlatformUserMfa", "the platform_user_mfa row: recovery_codes (bcrypt hashes) and secret_encrypted"},
		{sqlcPath, "PlatformUser", "password_hash"},
		{sqlcPath, "Staff", "pin_hash"},
		{sqlcPath, "StaffSession", "token_hash — sha256 of a live staff bearer token"},
		{sqlcPath, "PlatformSession", "token_hash — sha256 of a live platform bearer token"},
		{sqlcPath, "PlatformMfaChallenge", "challenge_hash"},
	}

	// Every payload type the scan resolved, by full type string, with the sites.
	scan := scanWireSurfaces(t)
	reached := map[string][]finding{}
	for _, f := range scan.Findings {
		bare := strings.TrimPrefix(strings.TrimPrefix(f.PayloadType, "[]"), "*")
		reached[bare] = append(reached[bare], f)
	}

	for _, store := range stores {
		full := store.pkg + "." + store.name
		t.Run(store.name, func(t *testing.T) {
			// The type must still carry what we think it carries — otherwise
			// this test passes for the wrong reason.
			typ := mod.lookupType(t, store.pkg, store.name)
			if len(walkTypeForSecrets(typ, secretJSONNames()).Exposures) == 0 {
				t.Errorf("%s no longer exposes any credential field.\n"+
					"  It was listed here because it holds %s.\n"+
					"  Either the field moved (find where) or this row is obsolete (delete it).",
					shortType(typ), store.holds)
				return
			}

			if hits, ok := reached[full]; ok {
				var where []string
				for _, f := range hits {
					where = append(where, fmt.Sprintf("%s at %s (%s)", f.JSONPath, strings.Join(f.Sites, ", "), f.Surface))
				}
				t.Errorf(`
%s reached a wire surface.

  it holds:  %s
  reached:   %s

  This type exists to store credentials. It has no business on any wire —
  project the fields you need into a response type instead.`,
					shortType(typ), store.holds, strings.Join(where, "; "))
			}
		})
	}
}
