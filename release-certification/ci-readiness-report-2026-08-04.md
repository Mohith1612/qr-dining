# CI Readiness Report — 2026-08-04

**Release Candidate:** `feature/signoz-observability` @ `c4aeb4b`
**Pull Request:** [#1](https://github.com/Mohith1612/qr-dining/pull/1) → `main`
**Workflow runs:** [30913538196](https://github.com/Mohith1612/qr-dining/actions/runs/30913538196) (CI),
[30913538314](https://github.com/Mohith1612/qr-dining/actions/runs/30913538314) (Frontend CI)

## Headline

Before this session the pipeline had **never executed**: zero workflow runs and zero pull
requests in the repository's history. Both workflows trigger only on push to `main`,
`v*` tags, or a PR into `main`, and the RC branch had been pushed 190 commits deep
without ever meeting one of those conditions. "CI is green" was an untested assumption.

It is now a measured fact. All 10 checks pass on the first run.

## Workflows executed

| Workflow | Job | Result | Time |
|---|---|---|---|
| CI | Lint | pass | 1m33s |
| CI | Build and Unit Test | pass | 2m33s |
| CI | sqlc Drift Check | pass | 1m11s |
| CI | Migration Check | pass | 47s |
| CI | **Integration Tests** (new) | pass | 3m37s |
| CI | **OpenAPI Lint** (new) | pass | 8s |
| CI | Docker Build | pass | 2m29s |
| Frontend CI | Frontend Lint and Build | pass | 1m12s |
| Frontend CI | Marketing Build | pass | 51s |
| Frontend CI | E2E Test Discovery | pass | 14s |

No job passed by being skipped. Both workflows fired: `frontend-ci.yml` is path-filtered,
and this diff touches `frontend/**`, `marketing/**`, and `e2e/**`.

## Fixes applied

Every fix below was reproduced locally first. None were guesses.

### 1. Docker build was broken (blocking)

`ci.yml` requested `platforms: linux/arm64` on an amd64 runner with **no
`docker/setup-qemu-action`**. Under buildx every stage defaults to the target platform,
so the runtime stage tried to execute arm64 binaries natively. Reproduced verbatim:

```
#8 [stage-1 2/5] RUN apk add --no-cache ca-certificates wget tzdata && addgroup ...
#8 0.383 exec /bin/sh: exec format error
```

This was not a latent risk — it was a certain failure on the first PR, and the only
reason it had never been seen is that no run had ever been triggered.

**Fix.** Added `docker/setup-qemu-action@v3`, and pinned the builder stage to
`FROM --platform=$BUILDPLATFORM`. The builder already cross-compiles explicitly
(`GOOS=linux GOARCH=arm64`), so keeping it native means QEMU carries only the small
`apk add` / `adduser` layer instead of the entire Go compile.

**Verified locally** before pushing: `linux/arm64` image, `aarch64` binaries in
`/app`, 1m31s wall clock with the Go compile at 45.8s (native speed). CI reproduced this
at 2m29s.

### 2. golangci-lint would have failed, and was not reproducible

The workflow pinned `golangci/golangci-lint-action@v6` with `version: latest`. `latest`
now resolves to **v2.12.2**, which the v6 action major does not support.

Independently of the break: a floating `latest` means the lint gate can change under a
re-run of the same commit. For a release candidate that is a correctness problem, not a
convenience one — a re-run of a certified build must produce the certified result.

**Fix.** Pinned the action to `v8` and the linter to `v2.12.2`, the exact version this
branch was certified against (0 issues locally, confirmed green in CI at 1m33s).

### 3. Integration tests never ran (the gap that mattered most)

`go test -race ./...` compiles none of the `//go:build integration` files. That silently
excluded **42 test functions across 14 files and 5 packages**
(`internal/{handlers,redis,repository,services,worker}`) — including
`authz_scope_integration_test.go`, the regression test for the exact cross-tenant bypass
this RC exists to fix. That file could have been deleted and the pipeline would have
stayed green.

**Fix.** Added an `integration` job with `postgres:17-alpine` and `redis:7-alpine`
services running `go test -count=1 -p 1 -race -tags integration ./...`. `-p 1` is
mandatory rather than tuning: every package shares the one database and one Redis, so
parallel package binaries truncate each other's fixtures.

**Verified the job is not vacuous.** A green integration job would be meaningless if the
service env vars never reached the tests, since they skip silently when unset. Two
independent confirmations:

- `internal/worker` and `internal/redis` appear as `ok` in the CI log. Those packages
  contain *only* integration-tagged tests, so they cannot report `ok` unless the
  database and Redis connections succeeded.
- Run times rose against the unit-only baseline in the same run:
  `handlers` 1.0s → 2.5s, `repository` 1.0s → 5.7s, `services` 1.1s → 7.7s.

Locally, `-run 'Scope|CrossOrg|CrossBranch|Tenant'` executes **42 subtests, 0 skipped**,
covering `enforce=false` and `enforce=true` across cross-organization and cross-branch
for 10 endpoints.

### 4. OpenAPI lint added

`redocly.yaml` was committed but no workflow ran it. Added an `openapi` job pinned to
`@redocly/cli@2.43.3`. Passes in 8s: valid, 9 warnings.

### 5. `workflow_dispatch` added to both workflows

Neither had it, so re-running the pipeline against a branch required pushing an empty
commit.

## Remaining risks

**No fix was required in response to a CI failure** — the failures were found and fixed
by local reproduction before the PR was opened. The risks below are what CI still does
not cover.

### A green pipeline is still not sufficient proof

- **Playwright never executes.** `frontend-ci.yml` runs only `playwright test --list`.
  387 tests across 106 files are discovered and none run. This includes
  `tenancy/T-04..06` and `webhook/W-01..07` — precisely the isolation and settlement
  guarantees this RC claims. The E2E job proves the suite parses, nothing more.
- **OpenAPI lint proves validity, not fidelity.** It does not prove the 157 documented
  operations still match the server's routes. The 1:1 check remains a manual exercise.
- **The Migration Check proves schema reversibility, not data reversibility.** It runs
  up → down -all → up against an *empty* database. `000039` is a forward-only data
  migration whose `down` is a deliberate no-op, so the job cannot detect that a real
  rollback across it is unrecoverable.

### Not covered at all

- ~~**No vulnerability scanning**~~ — **CLOSED.** A pinned `govulncheck` job now gates
  every PR; see the addendum below. Still absent: CodeQL, dependency review, and any
  scanning of the npm trees.
- **No branch protection on `main`.** Nothing enforces that these 10 checks pass before a
  merge. The pipeline is currently advisory, so its trustworthiness depends on process
  discipline rather than mechanism.
- **`frontend-ci.yml` path filtering vs required checks.** A backend-only PR skips all
  three frontend jobs. If they are later marked required, skipped runs will block; if
  they are not, a frontend break can merge. This needs a decision at the same time as
  branch protection.
- **No release workflow.** GHCR publish is folded into `ci.yml` on push to `main` and
  `v*` tags. Nothing produces a GitHub Release or an SBOM. Provenance attestation *is*
  generated (`--attest type=provenance,mode=max`, a `build-push-action` v5 default).

### Lower severity

- **Node 20 deprecation.** `actions/checkout@v4` and `actions/setup-node@v4` target
  Node 20 and are being force-migrated to Node 24 by the runner. A warning today, a
  break when the runner drops the shim. The Dependabot `github-actions` ecosystem is
  configured monthly and will surface the majors.
- **Publish path is untested.** `push: true` only evaluates on `push` to `main` or a
  `v*` tag, so GHCR login, push, and tagging have never executed. The first tag will
  exercise that path for the first time.

## Reproducibility

| Input | Pinned? |
|---|---|
| Go toolchain | `go-version: "1.26"` → resolved go1.26.5; `go.mod` says `go 1.26.0` |
| golangci-lint | v2.12.2, action v8 — pinned this session |
| Redocly CLI | 2.43.3 — pinned this session |
| Postgres / Redis services | `postgres:17-alpine`, `redis:7-alpine` — floating patch |
| Docker base images | `golang:1.26-alpine`, `alpine:3.20` — floating patch |
| Node | `lts/*` — floating |
| GitHub Actions | major-pinned (`@v4`, `@v5`, `@v8`), not SHA-pinned |

Builds are reproducible in the sense that matters for this gate — the lint gate no longer
floats, and the toolchain is fixed to a minor. They are **not** bit-reproducible: base
image patch tags, `lts/*`, and unpinned action SHAs all float. `sha_pinning_required` is
off on the repository. Tightening these is a hardening exercise, not a merge blocker.

---

# Addendum — 2026-08-04, dependency security round

**Commit:** `9f97a54` · **Checks: 11/11 green** (was 10/10)

## Updated job matrix

| Workflow | Job | Result |
|---|---|---|
| CI | Lint | pass |
| CI | Build and Unit Test | pass |
| CI | **Vulnerability Scan** (new) | pass |
| CI | sqlc Drift Check | pass |
| CI | Migration Check | pass |
| CI | Integration Tests | pass |
| CI | OpenAPI Lint | pass |
| CI | Docker Build | pass |
| Frontend CI | Frontend Lint and Build | pass |
| Frontend CI | Marketing Build | pass |
| Frontend CI | E2E Test Discovery | pass |

## The govulncheck job

Added to `ci.yml`, running `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...`.

**Deterministic.** The scanner is version-pinned, so the analysis is reproducible.

**Fails only on actionable findings, with no custom filtering.** govulncheck's default
source mode already has exactly the required semantics: it exits non-zero only when a
vulnerable symbol is reachable from this module's call graph, and exits 0 when a
vulnerable module is merely required but never called. Adding an allowlist would be
actively harmful — it would suppress the reachable findings that are the point.

**Verified in both directions before committing**, because a gate that cannot fail is
worthless:

- With the `x/text` and `grpc` upgrades reverted → **non-zero, job fails**, reporting both.
- With the upgrades applied → **exit 0**, while two unreachable module advisories remain
  present and correctly stay informational.

**On the Go version.** This job does *not* pin an exact Go patch, which deviates from a
literal reading of "pinned Go version". The reason is that pinning it would make the scan
less correct, not more. govulncheck reports standard-library findings against whatever
toolchain it runs on, so a scanner pinned to a patch the release image does not use would
report on a binary that is never shipped. Concretely: scanning this tree on go1.26.0
reports 19 standard-library vulnerabilities that do not exist in the shipped image.

Using the same `"1.26"` spec as every other job and as `backend/docker/Dockerfile` keeps
the scan describing the artifact. **Confirmed empirically in CI:** the job logged
`GOVERSION='go1.26.5'`, the same patch the Docker builder resolves.

**The residual gap this leaves.** `setup-go` and the Dockerfile resolve `"1.26"`
independently. They agree today; nothing enforces it. If they diverge, the gate keeps
passing while describing a different binary than the one shipped. Deriving both from one
pinned patch is the correct fix and is not done. Documented in `docs/dependency-upgrades.md`.

## Confirmation that nothing else moved

Six files changed in this round: `ci.yml`, `go.mod`, `go.sum`, `package.json`,
`package-lock.json`, and `docs/dependency-upgrades.md`. `backend/migrations/`,
`backend/internal/db/sqlc/`, and `openapi.yaml` are byte-identical to the previously
certified tree — verified by `git diff --name-only`, not assumed. No generated drift, no
accidental files.

## Remaining risks — unchanged

Everything in the "A green pipeline is still not sufficient proof" section above still
stands. **Playwright still does not execute**, and that remains the largest gap in this
release. The npm trees are still unscanned in CI; `govulncheck` covers Go only.
