# Task B — Pin the Go toolchain and clear the stdlib vulnerabilities

**Separate branch, separate commit from Task A.** Do not touch `e2e/`.

## The actual diagnosis — read this before acting

This is not "we are behind on Go". The repo is already consistent on the 1.26 line:

- `backend/go.mod:3` → `go 1.26.0`
- `backend/docker/Dockerfile:8` → `golang:1.26-alpine`
- `.github/workflows/ci.yml` → `go-version: "1.26"` at all seven jobs

`audit/govulncheck.log` reports 26 stdlib vulnerabilities, every one of the form
`Found in: <pkg>@go1.26` / `Fixed in: <pkg>@go1.26.6`. The scan ran on exactly
**1.26.0**, because the `go 1.26.0` directive with no `toolchain` line pins the
toolchain to 1.26.0 — even though the local `go version` is 1.25.3, which triggers a
toolchain download of precisely 1.26.0 and no higher.

Meanwhile CI's `"1.26"` is a floating minor spec that resolves to the newest 1.26.x,
so CI is probably already building on a patched toolchain. **Confirm that** rather
than assume it: the consequence, if true, is that local and CI builds differ in
security posture and our vulnerability evidence came from the worse of the two.

So the fix is small and the report is the point.

## What to do

1. Determine the current latest 1.26.x and confirm ≥ 1.26.6 clears all 26.
2. Add an explicit `toolchain` directive to `backend/go.mod` so the version is stated,
   not inferred. Do not raise the `go` directive — that is a language-version change
   and a separate decision.
3. Pin the same patch version explicitly in `.github/workflows/ci.yml` and
   `backend/docker/Dockerfile`. We want the three to be pinned and equal, so that a
   floating resolution can never again mean CI and a developer machine disagree about
   which stdlib they are testing.
4. Re-run `govulncheck` and write the new output to `audit/govulncheck.log`.
5. Run the full Go test suite, both tagged and untagged, and report results verbatim —
   including failures. A toolchain bump that "passes" without you showing the output
   is not accepted.

## Report

- Which toolchain produced the original 26-vuln scan, and which CI actually used.
- The remaining vulnerability count and, for any that remain, whether our code calls
  the affected path.
- Any test that changed behaviour under the new toolchain.

Do not fix unrelated findings that surface. List them.
