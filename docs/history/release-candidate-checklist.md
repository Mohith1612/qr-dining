# Release Candidate Checklist — qr-dining → Restaurant #1

**Candidate:** `feature/certification-fixes-ui-redesign` @ HEAD · **Compiled:** 2026-06-25
**Companion:** `final-release-readiness-report.md` (rationale + evidence)

Legend: `[x]` done & verified · `[ ]` outstanding · **SEV-0** blocks first paying dinner ·
**SEV-1** should precede pilot · **SEV-2/3** after pilot. Items marked *(verified this session)*
were exercised during the 2026-06-25 certification.

---

## Engineering

- [x] Backend builds, vets, unit-tests clean — Go 1.26, all 10 pkgs `ok` *(verified this session)*
- [x] Frontend type-checks and production-builds (`ALLOW_LOCALHOST_BUILD=true`) *(verified)*
- [x] sqlc generated code matches SQL (no drift) *(verified)*
- [x] Migrations round-trip up→down→up with no orphans (v38, 59 tables) *(verified)*
- [x] Architecture invariants hold — audit/tenant/entitlement/authz, 0 stubs *(verified)*
- [x] Full pilot UX works live on this branch — guest/staff/platform *(verified)*
- [ ] **SEV-1 — Repair the red integration suite (all failures are test-side; 0 product regressions). Get `go test -tags integration -p 1 ./...` green:**
  - [ ] `internal/services/session_hardening_integration_test.go` — raw inserts must set the NOT-NULL `session_business_date` (product already sets it).
  - [ ] `internal/worker/worker_integration_test.go` — remove the non-existent `sessions.updated_at` from the stale raw insert.
  - [ ] `internal/services/theme_integration_test.go` — update default-preset assertions from `dark-luxury` → `serene` (intentional, per mig 000038).
  - [ ] `internal/services/assistance_integration_test.go:86` — `pending→resolved` is now valid (matches `statemachine_test.go:108`); fix/retire `TestAssistance_InvalidTransition`.
  - [ ] `internal/services/session_integration_test.go` — close the prior fixture session so the one-active-per-table index doesn't collide (shared-fixture isolation).
- [ ] **SEV-2 — Document `POST /sessions/{id}/host` (host transfer) in `openapi.yaml`** (only route↔spec gap).
- [ ] SEV-2 — Add `audit.Record` for order placement (`PlaceOrder` currently emits no audit row).
- [ ] SEV-2 — Resolve WS reconnect dead-end after 10 attempts.
- [ ] SEV-2 — Replace float money accumulation with integer/decimal math (scale correctness).
- [ ] SEV-3 — Clear 567 `no-unused-vars` lint warnings (dead imports from the redesign refactor).
- [ ] SEV-3 — Fold `AppearanceTab` into `SettingsTab` per the redesign plan (cosmetic), or accept as-is.

## Operations

- [ ] **SEV-0 — Fresh soak of THIS branch's RC build** (the running soak is the Jun-10 pre-redesign binary):
  - [ ] Build from the RC tag; deploy binary **off `/tmp`** *(already corrected on soak host)*.
  - [ ] `AUDIT_LOG_V2_ENABLED=true`; **continuous synthetic traffic + external `/readyz` probing** for the full window.
  - [ ] **Purge stale `payment_pending` test sessions** so a real settlement stall isn't masked by alert noise.
  - [ ] Re-derive per-row audit size at realistic volume; confirm the 60-day storage projection.
- [ ] **SEV-1 — Live R2 backup round-trip:** run `deploy/backup/nightly-backup.sh` against the production bucket, then restore the downloaded object with `restore.sh`.
- [ ] **SEV-1 — Real alert receiver:** point Alertmanager at Slack/PagerDuty/email; fire one test alert; wire an **app-down / `/readyz` liveness** alert.
- [x] Automated nightly backup pipeline (R2/S3/local + manifest + retention) — built
- [x] Restore verified byte-faithful on a throwaway DB (checksum + sentinel + trigger fidelity)
- [x] Container healthchecks target `/readyz` (readiness), not `/health`
- [x] Release mode fails hard on insecure secret/origin defaults
- [ ] SEV-2 — Ensure no `.env` ships in the prod image/host (`godotenv.Overload()` footgun).
- [ ] SEV-2 — Raise `DB_MAX_CONNS` (~40–50) before any high-volume (multi-restaurant) scale.

## Documentation

- [x] `final-release-readiness-report.md` — produced *(this session)*
- [x] `release-candidate-checklist.md` — this file
- [x] `manual-testing-user-guide.html` / `-checklist.html` / `-session-guide.html` updated to v38 + redesign + Phase-0 *(this session)*
- [x] `testing-dashboard.html` already current (Jun 14)
- [ ] SEV-2 — Refresh `final-pilot-readiness-report.md` (Jun-10, premium-qr-collateral) to point at this report as the superseding cert.
- [ ] SEV-3 — Update `restaurant-go-live-checklist.md` / `pilot-onboarding-checklist.md` if any step changed with the redesign.

## Manual Testing

- [x] Guest serene flow (landing→welcome→menu→item sheet→cart→order→bill) *(verified)*
- [x] Promo entry at the bill + Pay-Full Cash/Card/UPI *(verified)*
- [x] Staff admin roster + Reset PIN + Change PIN (C16) *(verified)*
- [x] Kitchen KDS renders item contents (C2) *(verified)*
- [x] Platform Control Plane + cross-tenant analytics *(verified)*
- [ ] SEV-1 — Run the **full** updated `manual-testing-checklist.html` against the RC build on the mtest stack (this session smoke-covered the critical path; the long-tail categories — MFA, multi-instance, recovery, cross-tenant isolation — still need a sign-off pass).
- [ ] SEV-2 — Re-run the ~106 Playwright e2e specs against the RC (full reconciliation was deferred; the 6 DOM specs pass).
- [ ] SEV-2 — Purge leftover E2E test branches/sessions from any shared seed data before formal sign-off.

## Infrastructure

- [ ] **SEV-1 — Merge `feature/certification-fixes-ui-redesign` → `main` (no-ff); tag RC (e.g. `v1.0.0-rc.1`); build soak/pilot binary from the tag.**
- [ ] SEV-1 — Prune the merged feature branches (staff-analytics-loyalty, premium-qr-collateral, platform-governance-entitlements, pilot-readiness-remediation).
- [x] `deploy/` assets present: systemd backup timer/service, nginx conf, observability stack
- [ ] SEV-1 — Provision the production host (Oracle Cloud Ampere arm64) and dependencies (Postgres 17, Redis 7) per `operational-runbooks.md`.
- [ ] SEV-1 — Inject production secrets (GUEST_TOKEN_SECRET ≥32, CORS origins, MFA/webhook secrets); confirm release-mode boot succeeds.
- [x] Frontend prod build blocked from shipping localhost endpoints (prebuild guard)

## Monitoring

- [x] Prometheus scrape + alert rules, Alertmanager routing, Grafana dashboard, blackbox probe — in `deploy/observability/`
- [x] `ReadyzProbeFailing` + `Backup*` alerts defined
- [ ] **SEV-1 — Connect alerts to a real receiver and verify end-to-end** (see Operations).
- [ ] SEV-2 — Confirm the audit-write-failure counter + payment-stall escalation alerts are visible on the dashboard with the RC build.
- [ ] SEV-2 — Stand up external uptime/liveness probing for the pilot host (the prior soak ran ~75h unprobed).

## Customer Onboarding (Restaurant #1)

- [ ] SEV-1 — Create the org/restaurant/branch via the platform onboarding flow *(flow verified working this session)*.
- [ ] SEV-1 — Seed the real menu (categories, items, modifiers, single-select groups, featured/popular).
- [ ] SEV-1 — Create tables + print themed QR collateral (verify QR origin points at the guest frontend, not the API — `NEXT_PUBLIC_GUEST_URL`).
- [ ] SEV-1 — Create staff accounts (owner/manager/waiter/kitchen) + hand off PINs; confirm Reset/Change-PIN.
- [ ] SEV-1 — Pick the tenant theme preset (serene default or one of dark-luxury/modern-minimal/warm-cafe/vibrant).
- [ ] SEV-2 — Brief staff with `support-runbooks-pilot.md`; agree the manual-billing arrangement for the pilot.
- [ ] SEV-2 — Confirm `restaurant-go-live-checklist.md` is walked on-site before first service.

---

### Gate summary

| SEV | Open items | Blocks |
|---|---|---|
| SEV-0 | Fresh soak of the RC build | First paying dinner |
| SEV-1 | Green integration suite · merge+tag · live R2/alert · prod host+secrets · onboarding · full manual pass | Pilot start |
| SEV-2 | Host-transfer doc · order audit · WS reconnect · float math · e2e reconciliation · scale tuning | Post-pilot |
| SEV-3 | Lint debt · AppearanceTab · doc tidy-ups | Future |
