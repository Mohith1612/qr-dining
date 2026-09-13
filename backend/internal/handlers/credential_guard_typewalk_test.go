package handlers

// Credential leak guard, part 2 of 4: loading the module's types, and walking a
// type the way encoding/json would.
//
// Why not plain reflection, which is the obvious idea and the one the sweep
// that found the three leaks suggested:
//
// Reflection is fine for WALKING a type once you hold it. It cannot ENUMERATE
// the types that reach a wire surface in this codebase, because at every one of
// the three surfaces the static type is an empty interface:
//
//   - 82 of the 157 c.JSON calls pass gin.H, which is map[string]any. The
//     element type is `any`; reflect.TypeOf gives you interface{} and stops.
//   - Every publish entry point is `func(ctx, uuid.UUID, payload any)`
//     (internal/events/events.go). Same wall.
//   - Repos.LogEvent and Repos.AppendSessionEvent both take `payload any`.
//
// So a reflection-only guard needs a hand-maintained list of types to walk —
// which is the exact artefact that goes stale, and the exact reason these
// leaks kept being found by accident. The enumeration has to come from the
// call sites.
//
// This file therefore type-checks the module with go/types and resolves the
// static type of each payload expression, descending into gin.H literals so
// `gin.H{"session": sess}` yields sess's real type. The walk then uses
// go/types' own struct and tag API, which models embedding and json tags
// exactly as encoding/json does.
//
// No new module dependency: go/types plus the stdlib gc importer reading the
// export data that `go list -export` already produces.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// ── module loading ───────────────────────────────────────────────────────────

// guardPackage is one of our own packages, parsed and type-checked.
type guardPackage struct {
	ImportPath string
	Files      []*ast.File
	Info       *types.Info
}

type guardModule struct {
	Fset     *token.FileSet
	Packages []*guardPackage

	declOnce  sync.Once
	declIndex map[string]*declRef
}

var (
	loadOnce   sync.Once
	loadedMod  *guardModule
	loadErrMsg string
)

// loadModule type-checks every non-test file of every package in this module.
//
// It is deliberately fail-closed: if the toolchain cannot produce export data
// the guard reports a failure rather than skipping, because a guard that
// quietly no-ops is worth less than no guard at all.
func loadModule(t *testing.T) *guardModule {
	t.Helper()
	loadOnce.Do(func() {
		mod, err := doLoadModule()
		if err != nil {
			loadErrMsg = err.Error()
			return
		}
		loadedMod = mod
	})
	if loadedMod == nil {
		t.Fatalf("credential leak guard could not load the module: %s\n\n"+
			"The guard needs `go list` and the build cache. It does not skip on error: a\n"+
			"guard that no-ops when the toolchain hiccups would have let all three known\n"+
			"leaks through.", loadErrMsg)
	}
	return loadedMod
}

func doLoadModule() (*guardModule, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return nil, fmt.Errorf("no go.mod above the test working directory")
		}
		root = parent
	}

	// Export data for the full dependency graph. `go list -export` reuses the
	// build cache, which the test binary's own compile has already warmed.
	exportOut, err := runGoList(root, "-deps", "-export", "-f", "{{.ImportPath}}\t{{.Export}}")
	if err != nil {
		return nil, fmt.Errorf("go list -export: %w", err)
	}
	exports := map[string]string{}
	for _, line := range strings.Split(exportOut, "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && parts[1] != "" {
			exports[parts[0]] = parts[1]
		}
	}
	if len(exports) == 0 {
		return nil, fmt.Errorf("go list -export produced no export data")
	}

	// Our own packages, with build constraints already applied by go list.
	ownOut, err := runGoList(root, "-f", `{{.ImportPath}}	{{.Dir}}	{{join .GoFiles " "}}`)
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %q", path)
		}
		return os.Open(file)
	})

	mod := &guardModule{Fset: fset}
	for _, line := range strings.Split(ownOut, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
			continue
		}
		importPath, dir := parts[0], parts[1]
		var files []*ast.File
		for _, name := range strings.Fields(parts[2]) {
			af, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, fmt.Errorf("parse %s/%s: %w", importPath, name, err)
			}
			files = append(files, af)
		}
		info := &types.Info{
			Types:      map[ast.Expr]types.TypeAndValue{},
			Uses:       map[*ast.Ident]types.Object{},
			Defs:       map[*ast.Ident]types.Object{},
			Selections: map[*ast.SelectorExpr]*types.Selection{},
		}
		conf := types.Config{Importer: imp, Error: func(error) {}}
		if _, err := conf.Check(importPath, fset, files, info); err != nil {
			return nil, fmt.Errorf("type-check %s: %w", importPath, err)
		}
		mod.Packages = append(mod.Packages, &guardPackage{ImportPath: importPath, Files: files, Info: info})
	}
	if len(mod.Packages) == 0 {
		return nil, fmt.Errorf("go list found no packages in this module")
	}
	return mod, nil
}

func runGoList(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", append([]string{"list"}, append(args, "./...")...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("%w: %s", err, stderr)
	}
	return string(out), nil
}

// ── walking a type the way encoding/json serializes it ───────────────────────

// exposure is one secret-shaped field reachable from a root type, with the
// route it takes to get there. The route is the point: "leak detected" costs
// the next person an hour, "$.session_token via the embedded sqlc.Session"
// costs them a minute.
type exposure struct {
	Root     string // root type, e.g. "…/internal/services.SessionWithTable"
	Field    string // json name of the secret field, e.g. "session_token"
	JSONPath string // "$.session.session_token"
	GoPath   string // "services.SessionWithTable → sqlc.Session (embedded) → SessionToken string"
}

// blindSpot is a place the type walk cannot see through. Recorded rather than
// ignored, and reported by the static guard, because an unreported blind spot
// reads as a clean result.
type blindSpot struct {
	Root     string
	JSONPath string
	Reason   string
}

type walkResult struct {
	Exposures  []exposure
	BlindSpots []blindSpot
}

const walkMaxDepth = 12

// walkTypeForSecrets reports every secret-shaped json field reachable from t
// through encoding/json's rules: embedded structs promote their fields,
// json:"-" drops them, unexported non-embedded fields never serialize.
func walkTypeForSecrets(t types.Type, secrets map[string]string) walkResult {
	w := &typeWalker{secrets: secrets, root: shortType(t)}
	w.walk(t, "$", shortType(t), map[string]bool{}, 0)
	return walkResult{Exposures: w.exposures, BlindSpots: w.blind}
}

type typeWalker struct {
	secrets   map[string]string
	root      string
	exposures []exposure
	blind     []blindSpot
}

func (w *typeWalker) walk(t types.Type, jsonPath, goPath string, onPath map[string]bool, depth int) {
	if t == nil || depth > walkMaxDepth {
		return
	}

	// Named types first: cycle-breaking and the two cases where a type's wire
	// shape is not its field list. Only then switch on the underlying type —
	// switching on t directly would skip every named slice and named map
	// (gin.H is a named map, and missing it would silence a whole surface).
	if named, ok := t.(*types.Named); ok {
		key := named.String() + "@" + jsonPath
		if onPath[key] {
			return
		}
		onPath[key] = true
		defer delete(onPath, key)

		// json.RawMessage is a []byte alias; catch it before the byte-slice case
		// so the reason names the real thing.
		if named.String() == "encoding/json.RawMessage" {
			w.blind = append(w.blind, blindSpot{w.root, jsonPath,
				"json.RawMessage — a payload serialized upstream, opaque to a type walk"})
			return
		}
		// A custom marshaller replaces the struct's shape, so its fields say
		// nothing about what ships. Only worth reporting when there was
		// something underneath to hide: pgtype.Text and time.Time are scalar
		// wrappers and reporting them would bury the real blind spots in noise.
		if hasMarshalJSON(named) {
			if w.underlyingHidesSecrets(named) {
				w.blind = append(w.blind, blindSpot{w.root, jsonPath,
					shortType(named) + " implements json.Marshaler — its wire shape is decided by MarshalJSON, not by the fields below it"})
			}
			return
		}
	}

	switch u := t.Underlying().(type) {
	case *types.Pointer:
		w.walk(u.Elem(), jsonPath, goPath, onPath, depth+1)
		return
	case *types.Slice:
		if isByteSlice(u) {
			w.blind = append(w.blind, blindSpot{w.root, jsonPath,
				"raw JSON bytes (" + shortType(t) + ") — contents are opaque to a type walk"})
			return
		}
		w.walk(u.Elem(), jsonPath+"[]", goPath+"[]", onPath, depth+1)
		return
	case *types.Array:
		w.walk(u.Elem(), jsonPath+"[]", goPath+"[]", onPath, depth+1)
		return
	case *types.Map:
		w.walk(u.Elem(), jsonPath+".*", goPath+"[*]", onPath, depth+1)
		return
	case *types.Interface:
		if u.NumMethods() == 0 {
			w.blind = append(w.blind, blindSpot{w.root, jsonPath,
				"`any` (" + shortType(t) + ") — the concrete type is only known at the call site"})
		}
		return
	case *types.Basic, *types.Signature, *types.Chan:
		return
	}

	st := structOf(t)
	if st == nil {
		return
	}

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := reflect.StructTag(st.Tag(i)).Get("json")
		name, _, _ := strings.Cut(tag, ",")

		if name == "-" && !strings.HasPrefix(tag, "-,") {
			continue
		}
		// encoding/json promotes the exported fields of an embedded struct even
		// when the embedded field itself is unexported; every other unexported
		// field is dropped.
		if !f.Exported() && !f.Embedded() {
			continue
		}
		if !f.Exported() && f.Embedded() && structOf(f.Type()) == nil {
			continue
		}

		// An embedded field with no json name is flattened into the parent
		// object. This is exactly how SessionWithTable shipped every
		// sqlc.Session field, session_token included, as a top-level key.
		if f.Embedded() && name == "" {
			w.walk(f.Type(), jsonPath, goPath+" → "+shortType(f.Type())+" (embedded)", onPath, depth+1)
			continue
		}
		if name == "" {
			name = f.Name()
		}

		childJSON := jsonPath + "." + name
		childGo := goPath + " → " + f.Name() + " " + shortType(f.Type())

		if _, isSecret := w.secrets[name]; isSecret {
			w.exposures = append(w.exposures, exposure{
				Root: w.root, Field: name, JSONPath: childJSON, GoPath: childGo,
			})
			continue
		}
		w.walk(f.Type(), childJSON, childGo, onPath, depth+1)
	}
}

// underlyingHidesSecrets reports whether a json.Marshaler's field list could
// have reached a secret had the marshaller not intercepted it. Used to keep the
// blind-spot report to the ones that matter.
func (w *typeWalker) underlyingHidesSecrets(t types.Type) bool {
	st := structOf(t)
	if st == nil {
		return false
	}
	probe := &typeWalker{secrets: w.secrets, root: w.root}
	for i := 0; i < st.NumFields(); i++ {
		probe.walk(st.Field(i).Type(), "$", "", map[string]bool{}, 0)
	}
	if len(probe.exposures) > 0 {
		return true
	}
	for i := 0; i < st.NumFields(); i++ {
		if isSecretShaped(st.Field(i).Name()) {
			return true
		}
	}
	return false
}

func structOf(t types.Type) *types.Struct {
	if t == nil {
		return nil
	}
	st, _ := t.Underlying().(*types.Struct)
	return st
}

func isByteSlice(s *types.Slice) bool {
	b, ok := s.Elem().Underlying().(*types.Basic)
	return ok && (b.Kind() == types.Byte || b.Kind() == types.Uint8)
}

// hasMarshalJSON reports whether a type controls its own wire shape. Checked by
// method name rather than against the json.Marshaler interface, because
// go/types has no runtime interface to assert against.
func hasMarshalJSON(t types.Type) bool {
	for _, candidate := range []types.Type{t, types.NewPointer(t)} {
		ms := types.NewMethodSet(candidate)
		for i := 0; i < ms.Len(); i++ {
			if ms.At(i).Obj().Name() == "MarshalJSON" {
				return true
			}
		}
	}
	return false
}

// shortType renders a type without the module path noise:
// "github.com/Mohith1612/qr-dining/internal/services.SessionWithTable" reads as
// "services.SessionWithTable".
func shortType(t types.Type) string {
	s := types.TypeString(t, func(p *types.Package) string { return p.Name() })
	return s
}

// fullType keeps the import path, for inventory keys that must not collide.
func fullType(t types.Type) string {
	return types.TypeString(t, nil)
}

// ── self-test ────────────────────────────────────────────────────────────────

// TestTypeWalker_FollowsEmbedsAndTags pins the walker's own behaviour. The
// guard's whole value rests on this walk matching encoding/json, so it is
// checked against real marshalled output rather than by inspection.
func TestTypeWalker_FollowsEmbedsAndTags(t *testing.T) {
	type inner struct {
		Secret string `json:"session_token"`
		Plain  string `json:"plain"`
	}
	type dropped struct {
		Secret string `json:"session_token"`
	}
	type embedder struct {
		inner            // promoted: fields land at the top level
		Nested  inner    `json:"nested"`
		Skipped dropped  `json:"-"`
		List    []inner  `json:"list"`
		Ptr     *inner   `json:"ptr"`
		ByName  []string `json:"by_name"`
	}

	got := walkTypeForSecrets(embedderType(t), map[string]string{"session_token": "test"})

	want := map[string]bool{
		"$.session_token":        true, // via the embed, flattened
		"$.nested.session_token": true,
		"$.list[].session_token": true,
		"$.ptr.session_token":    true,
	}
	gotPaths := map[string]bool{}
	for _, e := range got.Exposures {
		gotPaths[e.JSONPath] = true
	}
	for path := range want {
		if !gotPaths[path] {
			t.Errorf("walker missed %s (found: %v)", path, keysOf(gotPaths))
		}
	}
	for path := range gotPaths {
		if !want[path] {
			t.Errorf("walker invented %s", path)
		}
	}

	// Cross-check against encoding/json: every path the walker claims must
	// actually appear in the marshalled object, and the json:"-" field must not.
	raw, err := json.Marshal(embedder{
		inner:   inner{Secret: "AAA"},
		Nested:  inner{Secret: "BBB"},
		Skipped: dropped{Secret: "CCC"},
		List:    []inner{{Secret: "DDD"}},
		Ptr:     &inner{Secret: "EEE"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, live := range []string{"AAA", "BBB", "DDD", "EEE"} {
		if !strings.Contains(body, live) {
			t.Errorf("encoding/json did not emit %s, so the walker's path for it is wrong: %s", live, body)
		}
	}
	if strings.Contains(body, "CCC") {
		t.Errorf(`encoding/json emitted the json:"-" field: %s`, body)
	}
}

// embedderType type-checks the snippet above as source, because the walker
// operates on go/types and its self-test has to hand it the same thing the
// real scan does.
func embedderType(t *testing.T) types.Type {
	t.Helper()
	const src = `package p
type inner struct {
	Secret string ` + "`json:\"session_token\"`" + `
	Plain  string ` + "`json:\"plain\"`" + `
}
type dropped struct {
	Secret string ` + "`json:\"session_token\"`" + `
}
type embedder struct {
	inner
	Nested  inner    ` + "`json:\"nested\"`" + `
	Skipped dropped  ` + "`json:\"-\"`" + `
	List    []inner  ` + "`json:\"list\"`" + `
	Ptr     *inner   ` + "`json:\"ptr\"`" + `
	ByName  []string ` + "`json:\"by_name\"`" + `
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	pkg, err := (&types.Config{}).Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("check snippet: %v", err)
	}
	return pkg.Scope().Lookup("embedder").Type()
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ── function index, for following a call into its body ───────────────────────

type declRef struct {
	Decl *ast.FuncDecl
	Pkg  *guardPackage
}

// funcDecl returns the declaration of one of our own functions, so the scanner
// can see what an opaque helper actually builds. The index is built once per
// module and lives on it, not in a package-level variable.
func (m *guardModule) funcDecl(fn *types.Func) (*ast.FuncDecl, *guardPackage) {
	m.declOnce.Do(func() {
		m.declIndex = map[string]*declRef{}
		for _, pkg := range m.Packages {
			for _, file := range pkg.Files {
				for _, d := range file.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok {
						continue
					}
					obj, ok := pkg.Info.Defs[fd.Name].(*types.Func)
					if !ok {
						continue
					}
					m.declIndex[obj.String()] = &declRef{Decl: fd, Pkg: pkg}
				}
			}
		}
	})
	if ref, ok := m.declIndex[fn.String()]; ok {
		return ref.Decl, ref.Pkg
	}
	return nil, nil
}

// lookupType finds a named type in one of this module's packages, so a test can
// walk a specific response type by name.
func (m *guardModule) lookupType(t *testing.T, pkgPath, name string) types.Type {
	t.Helper()
	for _, pkg := range m.Packages {
		if pkg.ImportPath != pkgPath {
			continue
		}
		for ident, obj := range pkg.Info.Defs {
			tn, ok := obj.(*types.TypeName)
			if ok && ident.Name == name && tn.Pkg() != nil && tn.Pkg().Path() == pkgPath {
				return tn.Type()
			}
		}
		t.Fatalf("type %s not found in %s", name, pkgPath)
	}
	t.Fatalf("package %s not loaded", pkgPath)
	return nil
}

// blindSpotPaths walks a type and returns the json paths the walk cannot see
// into, keyed by path.
func blindSpotPaths(t types.Type, secrets map[string]string) map[string]string {
	out := map[string]string{}
	for _, b := range walkTypeForSecrets(t, secrets).BlindSpots {
		out[b.JSONPath] = b.Reason
	}
	return out
}
