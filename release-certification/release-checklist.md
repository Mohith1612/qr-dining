# Release Checklist — Everything Remaining Before v1.0

Ordered by sequence, not severity. Items marked ☐ are open; ☑ were closed during this certification.

## A. Certification (this phase)

- ☑ All automated verification green on the RC branch (build, unit, race, integration, sqlc drift, gofmt, golangci-lint, frontend lint/typecheck/build, OpenAPI lint) — see `automated-test-report.md`
- ☑ Playwright e2e suite exercised against the manual-testing stack — see `automated-test-report.md`
- ☑ Manual-testing environment up, healthy, freshly seeded, credentials handed over
- ☐ **Human manual certification** using `manual-testing-checklist.md` (owner: you) — the gate for everything below

## B. Merge & tag (SEV-1 — after manual certification passes)

- ☐ Merge `feature/certification-fixes-ui-redesign` → `main` with `--no-ff`
- ☐ Confirm CI green on `main` (lint gate is newly restored — first CI run with golangci-lint v2 config)
- ☐ Tag `v1.0.0-rc.1` (tag push now triggers the GHCR image publish added to ci.yml)

## C. RC soak (SEV-0 — the #1 release blocker)

- ☐ Fresh multi-day soak of the **tagged** build: the passed 124 h R1 soak ran the pre-redesign 2026-06-10 binary; the ~18 cert fixes, migrations 000036–000038, and the entire Serene/Harmony redesign are unsoaked
- ☐ Soak binary built from the tag, hosted **off `/tmp`** (tmp-cleaner has wiped it twice), `AUDIT_LOG_V2_ENABLED=true`, continuous traffic + external probing, storage growth curve recorded
- ☐ Do NOT start until manual certification passes (explicit phase gate)

## D. Production readiness (SEV-1s and infra)

- ☐ Wire a real Alertmanager receiver (rules currently fire into a placeholder `notify_url` sink) + an app-down/`/readyz` page alert
- ☐ Provision the production host (Oracle Ampere arm64) per `DEPLOYMENT.md`; run `production-environment-checklist.md` end-to-end
- ☐ Nightly R2 backups scheduled against the production bucket (round-trip restore was proven 2026-07-18 against a temp bucket — `RECOVERY.md` §5)
- ☐ Frontend production deploy path (OpenNext → Cloudflare Workers) exercised once end-to-end with production env vars (the `check-prod-env.mjs` guard active, no localhost bypass)
- ☐ Domain purchase + DNS + TLS (deferred by explicit instruction this phase)

## E. Deferred product defects (pre-scale must-fixes, accepted for v1.0 pilot unless manual testing says otherwise)

- ☐ Promo daily-window timezone bug: windows compared in UTC, not branch tz — do not configure time-windowed promos until fixed
- ☐ Money math uses floats — convert to integer paise before scale
- ☐ F-8: tokenless session-snapshot fail-open — closes with rollout wave R6 (`AUTH_GUEST_CREDENTIALS_REQUIRED`); gates any public/multi-tenant exposure
- ☐ `session_sequences` write hot-spot (~5% 500s at concurrency ~150; clean at 50) — acceptable for pilot scale
- ☐ `godotenv.Overload()` prod footgun — stray `.env` overrides injected env
- ☐ 567 frontend `no-unused-vars` warnings + dead scaffolding cleanup

## F. CI/tooling debt

- ☐ Add integration suite (with PG/Redis service containers) and at least a Playwright smoke slice to CI — today neither runs anywhere in CI
- ☐ Align CI `setup-go` with `go.mod` go 1.26 (works today via GOTOOLCHAIN auto-download, but wastes CI time each run)
- ☐ Document the e2e runbook: suite needs a live stack, a real `E2E_ADMIN_TOKEN` from `POST /platform/auth`, and `AUTH_RATE_LIMIT_RPM`/`RATE_LIMIT_RPM` raised (default 10 RPM auth limit rate-limits the suite into mass failure)

## G. Rollout waves (post-v1.0, staged; all flags default-off, only R1 live)

- ☐ R2 `TENANCY_ORGANIZATIONS_ENABLED` → live R2 verification
- ☐ R3 `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS`
- ☐ R4 `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED`
- ☐ R5 `WS_TICKET_AUTH_REQUIRED`
- ☐ R6 `AUTH_GUEST_CREDENTIALS_REQUIRED` (closes F-8)
- ☐ R7 `PAYMENT_STAFF_SETTLEMENT_REQUIRED`
- ☐ Real payment gateway integration (currently manual/webhook-simulated only)
