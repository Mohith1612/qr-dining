# Issues Found — Release Candidate Certification

Severity scale: Critical / High / Medium / Low / Enhancement.
Issues fixed during certification are cross-referenced to `fixed-during-certification.md` (F-n).

---

## I-1 · High · CI lint gate could not run (golangci config v1 vs v2) — FIXED (F-2)

- **Repro:** `golangci-lint run` (v2.x, what CI's `version: latest` installs) in `backend/` →
  `can't load config: unsupported version of the configuration`.
- **Root cause:** `.golangci.yml` was v1-format; golangci-lint v2 refuses v1 configs.
- **Affected:** CI `lint` job (`.github/workflows/ci.yml`), local lint parity.
- **Recommendation:** done — config migrated; consider pinning the action to a major version to
  avoid silent future format breaks.

## I-2 · Medium · 19 files failing gofmt; 10 golangci findings — FIXED (F-1, F-2)

- **Repro:** `gofmt -l .` in `backend/`.
- **Root cause:** manual edits without a format pass on a branch CI never ran on (CI triggers only
  on push/PR to `main`).
- **Affected:** CI lint job; code hygiene.
- **Recommendation:** done. Consider a pre-commit hook or running CI on feature branches.

## I-3 · Medium · Integration test rot: WS ticket test vs migration 000022 — FIXED (F-4)

- **Repro:** full `-tags integration` run with `TEST_DATABASE_URL`+`TEST_REDIS_URL`; 1 failure in
  `internal/handlers`.
- **Root cause:** raw SQL insert missing NOT NULL columns added by migration 000022.
- **Affected:** integration gate credibility (the suite commit `9dd869a` restored was green only
  when the Redis-dependent tests were skipped — without `TEST_REDIS_URL` this test never ran).
- **Recommendation:** done. Run the suite with BOTH env vars set in any future gate.

## I-4 · Medium · ESLint linted `.open-next/` build output (750 errors) — FIXED (F-3)

- **Repro:** `npm run lint` in `frontend/` with a `.open-next/` build present.
- **Root cause:** new Cloudflare OpenNext output dir absent from ESLint ignores.
- **Affected:** frontend lint gate.
- **Recommendation:** done.

## I-5 · Medium · E2E suite unusable against a default-configured backend — PARTIALLY FIXED / DOCUMENTED

- **Repro:** `npx playwright test` against the manual-testing stack → mass 429
  (`RATE_LIMITED`) failures from `POST /staff/auth` / `POST /platform/organizations`.
- **Root cause:** the suite seeds an org + staff login per test from one IP; default limits
  (60 RPM general, **10 RPM auth**) throttle it immediately. Additionally the sensitive
  `payment_init` limiter (30/min/IP) is hardcoded, so payment-heavy spec groups can trip it even
  with env overrides.
- **Affected:** e2e usability; CI adoption of the suite.
- **Recommendation:** document the required e2e env (done in the artifacts here):
  `RATE_LIMIT_RPM`/`AUTH_RATE_LIMIT_RPM` raised, `PAYMENT_WEBHOOK_SECRET_STRIPE=test-webhook-secret`,
  `MFA_ENCRYPTION_KEY` set, real `E2E_ADMIN_TOKEN` minted via `POST /platform/auth`. Longer term,
  consider making sensitive limiter thresholds configurable for test environments (enhancement —
  not done).

## I-6 · Medium · E2E webhook helper/specs drifted from the certified payment-webhook contract — FIXED (F-5)

- **Repro:** all "valid webhook" specs 401/400 while rejection specs pass.
- **Root cause:** wrong header names (`X-Webhook-*` vs `X-Payment-*`), a `v1=` signature prefix the
  backend never strips, and missing required payload `id`.
- **Affected:** e2e coverage of the entire payment-settlement path.
- **Recommendation:** done (helper + P-03 + X-08).

## I-7 · Low · 28 remaining Playwright failures — all documented pre-existing spec-side backlog — NOT FIXED (deliberate)

- **Repro:** full desktop run (99 passed / 28 failed) with the e2e env of I-5.
- **Root cause:** documented per-spec in `docs/history/e2e-failure-analysis.md`: specs using the
  platform token on staff-only endpoints (S-02/03/04, S-06, S-08, A-06, OPID-04, O-02, PT-01,
  T-03, T-05, G-07), status-code lists missing legitimate codes (X-05→423, L-05→401,
  R-01/R-03/X-07/L-03/M-04→201), response-shape drift (O-01/O-07, PT-04, G-03/M-03), and
  strict-mode expectations that require the R4 flags (S-01). Deferred by
  `docs/history/non-blocking-e2e-cleanup-plan.md` to be revisited **with R4**.
- **Affected:** e2e signal quality only; no product regression implied by any of them.
- **Recommendation:** execute the documented cleanup plan alongside the R4 wave; add the suite to
  CI afterwards.

## I-8 · Low · `go.mod` (go 1.26.0) vs CI `setup-go` (1.24) skew — NOT FIXED

- **Repro:** inspect `backend/go.mod` vs `.github/workflows/ci.yml`.
- **Root cause:** toolchain bumped locally; CI pin not updated. Harmless today —
  `GOTOOLCHAIN=auto` downloads go1.26.0 transparently in both places — but CI re-downloads a
  toolchain every run.
- **Recommendation:** bump CI `go-version` to `"1.26"` when next touching ci.yml (left alone
  because ci.yml carries uncommitted infra-phase changes that are out of certification scope).

## I-9 · Low · Screenshot-sweep staff flow spec times out — NOT FIXED (spec-side suspicion, manually cross-checked)

- **Repro:** `screenshots/sweep.spec.ts` "staff flow: login → dashboard" hits the 30 s test
  timeout.
- **Root cause (suspected):** selector drift against the redesigned Harmony staff login; the same
  flow works when driven manually (verified during exploratory inspection — see
  screenshots `staff-*`).
- **Affected:** screenshot sweep only.
- **Recommendation:** update the sweep spec's staff-login selectors with the R4-wave e2e cleanup.

## Known, deliberately-carried defects (recorded, not certification findings)

- **F-8 (P1, gates public exposure):** tokenless session-snapshot fail-open until wave R6
  (`AUTH_GUEST_CREDENTIALS_REQUIRED`). Verify during manual testing that this stays acceptable for
  the supervised pilot only.
- **Promo daily-window timezone bug:** windows evaluated in UTC, not branch tz. Do not configure
  time-windowed promos.
- **Float money math** — convert to integer paise before scale.
- **`session_sequences` hot-spot** — ~5% 500s at concurrency ~150 (clean at pilot scale).
- **`godotenv.Overload()`** prod env-override footgun.
- **567 frontend lint warnings** (unused vars, dead scaffolding).

## Exploration findings (automated multi-role UI sweep)

**No new Critical/High/Medium product defects were found during exploration.** All exercised flows
behaved correctly (see `automated-test-report.md` exploration summary and `screenshots/`).
Regressions F-1 and F-6 both PASS; the session-ended screen (L-09 fix) works; F-8 behaves exactly
as documented (token redacted, tokenless read fail-open until R6).

Minor observations (Enhancement — recorded only, per policy not fixed):

- **E-1 · Enhancement:** promo-create validation errors surface only as a transient toast; the
  form gives no inline field-level indication of what's missing (Code/Value/date range). Easy to
  read as a silent failure.
- **E-2 · Enhancement:** the staff header shows the literal label "Restaurant / Branch 1" rather
  than the branch's display name ("Saffron House — Bandra"); with multiple tenants a staff member
  can't tell which restaurant they're signed into from the header alone.
- **E-3 · Enhancement:** the loyalty admin tab logs expected 403s to the browser console when the
  feature is gated off; probing entitlement first (or swallowing the expected status) would keep
  the console clean for support/debug sessions.
- **E-4 · Enhancement:** guest join screen shows only the table identifier ("Table T3"), not the
  restaurant/branch name — a guest scanning in a multi-venue food hall gets no venue confirmation
  before joining.

---

## Manual-testing remediation pass — 2026-07-21

First human certification pass (45/54 items) surfaced the findings below. Four read-only
investigations traced each to root cause; all were fixed on `feature/certification-fixes-ui-redesign`
(commits `ae5722c`, `8b5537f`, `e58311d`, `d3387b0`). Backend gates (build/vet/golangci/sqlc/race/unit),
frontend gates (typecheck/lint/build:cf), and targeted Playwright (guest+frontend desktop 12/2 — the
certified baseline, G-03/G-07 spec-side) all green afterward.

### Confirmed bugs — fixed
- **M-1 (G-10) Non-host could request the bill.** The guest "Request Bill" fired an assistance
  request of type `bill` with no host guard, unlike order/payment. Now gated on `AuthorizeHostAction`
  (backend) and the option is disabled for non-hosts (frontend).
- **M-2 Host not reassigned when the host goes inactive (deadlock).** `ensureHostBaseline` only
  reassigned a revoked/unset host, so a remaining guest was stranded and couldn't order. Now it also
  reassigns a live-but-absent host to a present participant (churn-safe: requires a present co-host).
- **M-3 (G-13) Rejoin via `/session/<id>` bounced to landing.** Guest credentials lived only in
  per-tab sessionStorage. Now mirrored to localStorage keyed by session id and recovered on rejoin.
- **M-4 QR regen "occupied" with 0 active sessions + cross-table flips.** The 5-min reconciler
  released any occupied table lacking an `active` session, prematurely freeing payment_pending /
  awaiting_reactivation tables. Reconciler and the staff sessions board now count all live statuses.
- **M-5 Loyalty/Performance gate "enabled itself" after repeated opens.** The feature-gate decision
  was cached in Redis 60s with no invalidation on flag/entitlement change. Added cache invalidation
  on every operator mutation.
- **B1 Order/review (cart) page showed placeholder circles.** `image_url` wasn't carried through
  `ListCartItems`; now selected, typed, and rendered (falls back to the vignette).
- **B2 Print button produced a blank tinted sheet.** The print template was nested under the ops
  layout its own print CSS hides; now portalled to `<body>`.
- **B3 "Couldn't load staff roster" after a PIN reset.** The reset never refetched and errors were
  swallowed; now refetches and retries transient failures.
- **B4 Platform MFA enrollment was unwired end-to-end.** Backend was complete; added the API methods,
  a Security page (enroll → confirm → recovery codes, disable), and a nav entry.
- **B5 Platform nav showed dead-end links to non-super roles ("felt buggy").** Nav is now
  role-filtered.
- **B6 Org creation was hard to find + a mid-wizard failure 500'd on retry.** Added a "Create
  organization" entry point on the Organizations page and a clear duplicate-code 409.

### Missing features — added
- **F-1 Table rename / edit / delete** (delete blocked while a session is in progress).
- **F-2 Order cancellation** from the kitchen board (pending/confirmed/preparing → cancelled;
  waiter/manager/kitchen permitted; guest tracker already handles the cancelled state).
- **F-3 Promo activate/deactivate toggle and edit** of mutable fields (code/type immutable).

### Polish
- Order-ready copy reworded ("will be served shortly"); sold-out dishes sink to the bottom of each
  menu category; change-PIN modal gains a confirm field + validation; staff lockout shortened to
  2 min with a live countdown on the login screen; table QR rendered theme-aware on a transparent
  background; platform Overview degrades per-widget instead of blanking to zeros.

### Not a bug / discoverability (no code change beyond guidance)
- **W-1 PIN lockout refusing the correct PIN** was the 15-min lockout working as designed; shortened
  and made visible (see Polish) rather than "fixed".
- **O-3 owner cross-branch view** does not exist on the owner side (platform-only at
  `/platform/organizations`); **O-4 audit** lives at `/platform/support/audit`; **P-2 create
  org** is the `/platform/onboarding` wizard. The host badge (not "crown") and ~10s staff-board
  polling are expected behaviour.
