# Fixed During Certification

Every fix below satisfies the certification fix policy: root cause understood, low risk, no
architectural change, no feature work. Each was committed individually to the RC branch and the
affected verification re-run.

## 1. `372e3be` — gofmt drift across 19 backend files

- **Before:** `gofmt -l .` reported 19 files (misaligned const/struct alignment from manual edits).
  CI's lint job fails on any gofmt output, so the CI gate was red on this branch.
- **Fix:** `gofmt -w` on the flagged files. Purely mechanical; zero semantic change.
- **After:** `gofmt -l .` empty; `go build ./...` and full unit suite pass.
- **Note:** the previously-staged deletion of the obsolete 32 MB `server` binary (staged during the
  infrastructure phase, before certification began) rode along in this commit.

## 2. `ff23778` — golangci-lint gate restored (config v1 → v2) + 10 lint findings fixed

- **Before:** `backend/.golangci.yml` was v1-format. golangci-lint v2 — what CI's
  `version: latest` resolves to — refuses to start with it, so the CI lint gate could not run at
  all. Once migrated, the gate reported 10 findings.
- **Fix:** official `golangci-lint migrate`; then: checked `json.Unmarshal` errors in
  `internal/audit/redaction_test.go`; `_ =` on `SetReadDeadline` in `scripts/loadtest/main.go`;
  removed ineffectual assignment in `internal/services/entitlement.go`; replaced deprecated
  `prometheus.NewGoCollector`/`NewProcessCollector` with the `collectors` package in
  `internal/observability/metrics.go`; deleted dead symbols `platformAuditResponse`,
  `platformError` (`internal/handlers/platform.go`), `closeSessionRequest`
  (`internal/handlers/session.go`), `defaultCacheTTL` (`internal/redis/cache.go`).
- **After:** `golangci-lint run` → **0 issues**; build + unit + race suites pass.
- **Reason:** restores the only lint gate CI has; dead-code removal and error-checking are
  strictly risk-reducing.

## 3. `a91cb7e` — ESLint no longer lints `.open-next/` build output

- **Before:** `npm run lint` → 19,457 problems (750 errors), all errors inside the new
  `.open-next/` Cloudflare build directory (created by the infrastructure phase, never added to
  ignores).
- **Fix:** added `.open-next/**` and `.wrangler/**` to `frontend/eslint.config.mjs` ignores.
- **After:** 0 errors, 567 warnings — exactly the documented pre-existing warning baseline.

## 4. `f493dc9` — integration-test rot: WS ticket test vs migration 000022

- **Before:** `TestWSUpgradeWithTicketRejectsCredentialVersionMismatch` failed: its raw
  `INSERT INTO sessions` predates the NOT NULL `session_business_date`/`visit_number`/
  `session_number` columns. Only red test in the whole `-tags integration` suite.
- **Fix:** supply the required columns, copying the established pattern from
  `session_hardening_integration_test.go`. Test-side only.
- **After:** full integration suite (serial, real PG + Redis, 0 skips) → **12 packages ok, 0
  failures**.

## 5. `439de14` — e2e webhook specs realigned to the certified payment-webhook contract

- **Before:** every "valid webhook accepted" spec failed (rejection specs passed). The e2e helper
  sent `X-Webhook-Timestamp`/`X-Webhook-Signature: v1=<hex>` and events without the required `id`
  field. The certified OpenAPI contract (and handler) require
  `X-Payment-Timestamp`/`X-Payment-Signature`, raw hex HMAC of `timestamp.raw_body`, and a payload
  `id` (the idempotency/dedupe key).
- **Fix (test-side only):** `e2e/helpers/api.ts placeWebhook` uses the correct headers and defaults
  a stable event `id`; `P-03` (hand-rolled request) aligned the same way; `X-08` accepts 422
  `PAYMENT_AMOUNT_INVALID` (amount is validated against the bill before the idempotency key is
  consulted — the replay is rejected either way).
- **After:** webhook + payment spec group: 26/28 → after fixes 100% of the contract-drift subset
  passes; full desktop suite improved from 91 → 99 passed. Remaining failures are the documented
  deferred backlog (see issues-found.md I-7).

## Not fixed (deliberately)

- The remaining 28 Playwright failures — all map to the pre-existing, documented spec-cleanup
  backlog (`docs/history/e2e-failure-analysis.md`, deferred by
  `docs/history/non-blocking-e2e-cleanup-plan.md` to the R4 wave). They are spec bugs (wrong token
  types, status-code lists, response shapes, strict-mode expectations), not product regressions.
  Rewriting ~25 specs mid-certification would violate the conservative-change policy.
- Anything listed as a known deferred defect (promo timezone, float money, F-8, etc.) — recorded in
  `issues-found.md` / `release-checklist.md`.
