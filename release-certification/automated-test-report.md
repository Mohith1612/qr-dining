# Automated Test Report — Release Candidate Certification

- **Date:** 2026-07-18
- **Branch:** `feature/certification-fixes-ui-redesign`
- **Base commit at start of certification:** `9dd869a`
- **Toolchain:** Go 1.26.0 (auto-selected via `GOTOOLCHAIN=auto` from `go.mod`; host go 1.25.3), Node v22.14.0, golangci-lint v2.12.2, @redocly/cli 2.39.0, Playwright (repo-pinned)
- **Environment safety:** all verification ran against throwaway datastores (`pilot-validation` compose project, PG :15432 / Redis :16379, removed afterward) and the isolated `manual-testing` stack. The protected R1 soak stack (compose project `qr-dining`) was never touched.

## Summary

| # | Verification | Command (working dir) | Initial result | Final result |
|---|---|---|---|---|
| 1 | Go formatting | `gofmt -l .` (backend) | **FAIL** — 19 drifted files | **PASS** (after fix, committed) |
| 2 | Go vet | `go vet ./...` (backend) | PASS | PASS |
| 3 | golangci-lint | `golangci-lint run --timeout=3m` (backend) | **FAIL** — config unreadable by v2; after migration, 10 findings | **PASS — 0 issues** (config migrated + findings fixed, committed) |
| 4 | Module integrity | `go mod verify` (backend) | PASS | PASS |
| 5 | Build | `go build ./...` (backend) | PASS | PASS |
| 6 | Unit tests | `go test -count=1 ./...` (backend) | PASS | PASS |
| 7 | Race (CI parity) | `go test -count=1 -race ./internal/domain/...` | PASS | PASS |
| 8 | sqlc drift | `go tool sqlc generate && git diff --exit-code internal/db/sqlc/` | PASS — no drift | PASS |
| 9 | Integration suite | `go test -tags integration -count=1 -p 1 ./...` with `TEST_DATABASE_URL`/`TEST_REDIS_URL` (pilot-validation datastores) | **FAIL** — 1 test (`TestWSUpgradeWithTicketRejectsCredentialVersionMismatch`) | **PASS — 12 packages ok, 0 skips** (test-side fix, committed) |
| 10 | Frontend lint | `npm run lint` (frontend) | **FAIL** — 750 errors (all in `.open-next/` build output) | **PASS** — 0 errors, 567 pre-existing warnings (ignore fix, committed) |
| 11 | Frontend typecheck | `npm run typecheck` (frontend) | PASS | PASS |
| 12 | Frontend build | `NEXT_PUBLIC_ENV=development npm run build` (frontend) | PASS | PASS |
| 13 | OpenAPI lint | `npx @redocly/cli lint openapi.yaml` (root) | PASS — 0 errors, 9 warnings | PASS |
| 14 | Playwright e2e | `npx playwright test` (e2e) against manual-testing stack | _see below_ | _see below_ |

## Detail

### 1. Go formatting (initial FAIL → PASS)
`gofmt -l .` reported 19 files with drift (misaligned const/struct blocks from manual edits). This
would fail CI's lint job at its first step. All files reformatted with `gofmt -w`; no semantic
changes. Committed as `372e3be`.

### 3. golangci-lint (initial FAIL → PASS)
Two-layer failure:
1. `backend/.golangci.yml` was in v1 format. golangci-lint v2 — which CI's `version: latest`
   resolves to — refuses to run with it (`unsupported version of the configuration`). The CI lint
   gate therefore could not run at all. Config migrated with the official `golangci-lint migrate`.
2. The restored gate reported 10 findings: 3 errcheck (unchecked `json.Unmarshal` in a test,
   unchecked `SetReadDeadline` in the loadtest script), 1 ineffassign
   (`internal/services/entitlement.go`), 2 staticcheck deprecations (legacy Prometheus collector
   constructors in `internal/observability/metrics.go`), 4 unused symbols (dead code in
   `internal/handlers/platform.go`, `internal/handlers/session.go`, `internal/redis/cache.go`).
   All fixed; re-run reports **0 issues**. Committed as `ff23778`.

### 9. Integration suite (initial FAIL → PASS)
Run serially (`-p 1`) against dedicated throwaway datastores (never the soak DB), with both
`TEST_DATABASE_URL` and `TEST_REDIS_URL` set — **0 skipped tests**, so the Redis/WS-ticket tests
executed too. One failure: `TestWSUpgradeWithTicketRejectsCredentialVersionMismatch` used a raw
`INSERT INTO sessions` predating migration 000022's NOT NULL columns
(`session_business_date`, `visit_number`, `session_number`). Same test-rot class as commit
`9dd869a`; fixed using the established pattern from `session_hardening_integration_test.go`.
Full-suite re-run: **12 packages ok, 0 failures, 0 skips**. Committed as `f493dc9`.

### 10. Frontend lint (initial FAIL → PASS)
The new `.open-next/` Cloudflare build output (from the infrastructure phase) was not in the ESLint
ignore list, so minified bundles were being linted: 19,457 problems, 750 errors. All 750 errors were
inside `.open-next/**`; real sources contained only warnings. Added `.open-next/**` and
`.wrangler/**` to `frontend/eslint.config.mjs` ignores. Re-run: **0 errors, 567 warnings** — the
documented pre-existing `no-unused-vars` warning baseline (tracked as debt in
STATE-OF-THE-PROJECT.md, not a certification blocker). Committed as `a91cb7e`.

### 13. OpenAPI lint (PASS)
`openapi.yaml` valid; 0 errors, 9 warnings — all from the intentionally-downgraded
`operation-4xx-response` rule in `redocly.yaml`.

### 14. Playwright e2e (127 desktop tests: 99 passed / 28 failed — all 28 documented pre-existing spec-side)

Run history (desktop project, all 106 spec files / 127 tests):
1. **Run 1 — 14 passed / 113 failed.** Root cause: default backend rate limits (10 RPM on
   `/staff/auth` + `/platform/auth`, 60 RPM general) throttle the suite's per-test org seeding into
   mass 429s. Not a product defect — the config comment on `AUTH_RATE_LIMIT_RPM` explicitly says to
   raise it in test environments; the manual-testing stack simply doesn't.
2. **Run 2 (rate limits raised) — 91 passed / 36 failed.** The webhook/payment subset failed
   because (a) no `PAYMENT_WEBHOOK_SECRET_STRIPE` was configured, and (b) the e2e helper had
   drifted from the certified webhook contract (wrong header names, `v1=` prefix, missing required
   event `id`). Helper + 2 specs fixed (commit `439de14`, test-side only — verified against
   `openapi.yaml` and the handler).
3. **Final run (secret + MFA key configured, helper fixed) — 99 passed / 28 failed.**

The remaining 28 failures map one-to-one onto the pre-existing, documented spec-cleanup backlog in
`docs/history/e2e-failure-analysis.md` (deferred to the R4 wave by
`docs/history/non-blocking-e2e-cleanup-plan.md`): specs using the platform token on staff-only
endpoints (S-02/03/04, S-06×2, S-08, A-06, OPID-04, O-01/O-02/O-07, PT-01, PT-04, T-03, T-05,
G-03, G-07, M-03, M-04), status-code lists missing legitimate codes (X-05→423, L-05→401,
R-01×2/R-03/X-07/L-03→201-vs-200), strict-mode expectations requiring R4 flags (S-01), and one
screenshot-sweep selector timeout (staff login — the same flow works when driven manually; see
exploration screenshots). **None indicates a product regression.**

Required e2e environment (now documented): live stack + `API_URL`/`APP_URL`; real
`E2E_ADMIN_TOKEN` minted via `POST /platform/auth` (the static `e2e-admin-secret` bypass was
removed by security hardening); backend started with `RATE_LIMIT_RPM`/`AUTH_RATE_LIMIT_RPM` raised,
`PAYMENT_WEBHOOK_SECRET_STRIPE=test-webhook-secret`, `MFA_ENCRYPTION_KEY=<32 chars>`. Note the
hardcoded `payment_init` sensitive limiter (30/min/IP) can still trip back-to-back payment-heavy
runs from one IP.

## Exploratory inspection summary (Playwright-driven, screenshots in `screenshots/`)

Full multi-role sweep on the freshly-reseeded stack; **no new product defects found**. Highlights:
- **Guest:** QR join (new + "Join the party" rejoin + invalid-token error page), menu/filters,
  variant pricing, shared cart, order placement, live tracker, assistance, bill, promo apply
  (₹180→₹162 with created CERT10), cash payment, session-ended screen with payment confirmation.
- **F-6 regression PASS:** kitchen advanced confirmed→preparing→ready→served (KDS on app-2 :8095),
  guest tracker (app-1 :8090) advanced live each step with no refresh — cross-instance WS fan-out
  proven.
- **F-1 regression PASS:** session drifted to `awaiting_reactivation` after presence dropped;
  returning guest transparently reactivated it (DB verified back to `active`), full UI intact — no
  dead-end.
- **Waiter:** ready-to-serve, payments-to-collect (seeded + live), assistance ack/resolve — board
  updates live; **Kitchen:** KDS columns + no serve action (correct role split).
- **Manager:** admin tabs render; menu availability toggle round-trip; promo creation (required
  date-range validation via toast); stats charts; loyalty tab shows clean gated state (the 403s in
  console are the expected gate).
- **Owner:** staff roster with add-staff + deactivate (owner-only), plan/entitlements.
- **Platform:** super_admin overview with accurate live metrics (GMV ₹380 = the two settled
  payments), org detail (plan/capabilities/limits/branches/suspend), feature-flag console
  (loyalty + staff_performance_analytics, global/targeted precedence), support_admin login with
  read-only audited diagnostics console.
- **Cross-tenant:** three distinct themes render (Saffron dark-luxury, Copper Pot warm-cafe,
  Urban Brew vibrant).
- **F-8 spot check:** `session_token` is redacted (`""`) in all guest snapshot responses;
  tokenless snapshot access still returns 200 (fail-open) — exactly the documented known-open
  state until wave R6.

## Notes / observations
- **Go version skew (informational):** `go.mod` declares `go 1.26.0`; CI pins `go-version: "1.24"`.
  Because `GOTOOLCHAIN=auto`, both local and CI transparently download and use go1.26.0, so nothing
  breaks — but CI wastes time fetching a toolchain every run. Recommend aligning `setup-go` to 1.26.
  Recorded in issues-found.md.
- **CI coverage gap (pre-existing, tracked):** CI runs lint, domain unit tests, sqlc drift, and a
  docker build only. The integration suite and Playwright suite run nowhere in CI. Recorded in
  release-checklist.md.
- The previously staged deletion of the obsolete 32 MB `server` binary at the repo root (staged
  before certification began) was committed together with the gofmt commit `372e3be`.
