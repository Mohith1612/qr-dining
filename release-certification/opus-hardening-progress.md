# Opus Hardening Loop — Progress

**Session start:** 2026-09-04
**Baseline SHA:** `9865a488709464296f888bcba9476a79b3f5423f` (`feature/signoz-observability`)
**Objective:** evidence-driven hardening before formal manual certification. Not deployment, not doc consolidation.

---

## 0. Pre-existing working tree (NOT created by this session — preserved)

`git status` at session start showed 88 entries belonging to an **in-progress documentation
consolidation** from a previous session. This session did **not** create, stage, commit,
reset, or overwrite any of it.

Inventory:

| Class | Count | Examples |
|---|---|---|
| Staged renames/deletes (docs reorg) | 50 files, 1851 deletions | `CODEBASE.md`→deleted, `DEPLOYMENT.md`→`docs/DEPLOYMENT.md`, root reports→`docs/history/`, `release-certification/*`→`release-certification/archive/2026-*` |
| Unstaged modifications | 29 files | `README.md`, `STATE-OF-THE-PROJECT.md`, `docs/master-system-context-v1.md`, `deploy/**`, `docs/manual-testing/*.html`, `marketing/package*.json` |
| Untracked new files | 17 | `AGENTS.md`, `CLAUDE.md`, `CANONICAL-ENGINEERING-EVIDENCE-BANK.md`, `docs/{ARCHITECTURE,DEVELOPMENT,RELEASE,RUNBOOKS,SECURITY,TESTING}.md`, `docs/agents/*`, `release-certification/manual-certification-preflight-2026-08-22.md` |

No stashes exist. This file (`opus-hardening-progress.md`) is the only artifact this session
adds to `release-certification/`.

---

## 1. Verified baseline facts (established from the repo, not from reports)

| Fact | Verified value | How |
|---|---|---|
| Branch / HEAD | `feature/signoz-observability` @ `9865a48` | `git rev-parse HEAD` |
| origin/feature/signoz-observability | `b57f746` — **4 commits behind local** | `git rev-parse origin/...` |
| Unpushed commits | `03eb13c` (PostHog), `8ffd6e0`, `5af1ffa`, `9865a48` | `git log origin/..HEAD` |
| Open PR | #1, head `b57f746`, base `main` | `gh pr list` |
| Last CI run | 2026-08-04, PR #1 @ `b57f746`, **all green** | `gh run list` |
| **CI coverage of HEAD** | **ZERO.** No workflow has ever run on `03eb13c`..`9865a48` | run list vs. commit dates |
| Local Go toolchain | resolves to **go1.26.0** (`go.mod: go 1.26.0`, no `toolchain` directive, `GOTOOLCHAIN=auto`) | `go version` in `backend/` |
| CI Go toolchain | `go-version: "1.26"` → latest 1.26.x at run time | `.github/workflows/ci.yml` |
| Deployed beta binary | go1.26.5 (per 2026-08-22 preflight) | historical, not re-verified |
| `go build ./...` | PASS | run |
| `go test -count=1 -race ./...` | PASS — but only **10 of 24 packages have any test files** | run; see §1.1 |
| Throwaway test stack | `qrd-opus-pg` :15599, `qrd-opus-redis` :16699 (created this session, to be removed) | `docker run` |
| Protected stacks | soak (`qr-app-soak`, `qr-dining-postgres-1`) and manual-testing containers all **Exited**; untouched | `docker ps -a` |

### 1.1 Unit-suite package coverage (`go test -race ./...`)

Has tests: `audit`, `auth`, `authz`, `config`, `crypto`, `domain`, `handlers`, `repository`,
`services`, `websocket`.

**No test files at all:** `cmd/server`, `cmd/migrate`, `cmd/bootstrap-admin`, `internal/db`,
`internal/db/sqlc`, `internal/events`, `internal/middleware`, `internal/observability`,
`internal/redis`, `internal/server`, `internal/storage`, `internal/worker`,
`scripts/loadtest`.

`internal/middleware` (730 LOC — rate limiting, CORS, security headers, proxy trust, branch
guard, staff auth) and `internal/worker` (553 LOC — background execution) have **zero
non-integration tests**. `internal/worker` has one integration test file.

### 1.2 CI truthfulness notes (from reading both workflows)

- `ci.yml` and `frontend-ci.yml` trigger only on `push:[main]`, `tags:v*`, `pull_request:[main]`,
  `workflow_dispatch`. Work on a feature branch with no open PR at that head is **never** gated.
- `frontend-ci.yml` runs `npm run lint` + `npm run build`. It does **not** run a standalone
  `tsc --noEmit`; typecheck coverage depends on whether `next.config` sets
  `typescript.ignoreBuildErrors`. **To verify.**
- `e2e-typecheck` job runs `playwright test --list` only — discovery, not execution. Correctly
  labelled in the workflow.
- `docker-build` builds `linux/arm64` only.
- `govulncheck` is inherently time-varying: the same commit passes today and fails next month
  as new stdlib advisories land. It passed 2026-08-04; the 2026-08-22 preflight reported 26
  reachable advisories from source. **To re-run.**

---

## 2. Findings

Status legend: `OPEN` · `REPRODUCED` · `FIXED` · `FALSE-POSITIVE` · `DEFERRED` · `PRE-EXISTING-OK`

_(populated as the loop runs)_

---

## 3. Commands run

_(appended incrementally)_
