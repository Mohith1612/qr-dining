# Rollout Blocker Remediation Report (pre-R1)

Date: 2026-05-26
Scope: fix ONLY genuine P0 rollout blockers from the pre-R1 Playwright sweep
(43 failures, `e2e-failure-analysis.md`). Spec/helper/env drift intentionally left alone.

## 1. Git status findings (start of pass)

Modified (tracked, uncommitted) — **all from the abandoned Sonnet verification sweep or
prior phases; all PRESERVED, nothing discarded** (none were experimental/harmful):

- `e2e/helpers/api.ts` — **required** drift fix. `seedOrg` rewritten to the current
  platform API (`{code,name,restaurant_slug}`, branch creation with
  `initial_owner`/`initial_tables`, staff-token menu seeding, `forceCloseSession` →
  `DELETE /sessions/:id`). Without this, every `seedOrg`-based spec fails at setup.
- `backend/internal/config/config.go` + `internal/server/server.go` — adds
  `AUTH_RATE_LIMIT_RPM` override (default unchanged at 10). A test-env knob, not a
  weakening. Low risk.
- `docker-compose.staging.yml` — local pg/redis port publishing (Phase E artifact).
- `e2e/{audit/A-01, guest/G-04, order/O-06, platform/PT-02, session/L-04, L-08, L-09}`
  (7 specs) — spec-drift fixes from the sweep. None overlap the 5 P0 specs.

Untracked: the Phase C/D/E report `.md`s, `plans/`, `e2e/node_modules`,
`e2e/artifacts`, `e2e/screenshots`, `e2e-failure-analysis.md`, and the two reference
HTML files. Left untracked.

**Preserved vs discarded:** everything preserved. Nothing discarded — the sweep made
coherent drift fixes, no experimental junk. The P0 commits below touch ONLY backend
source; the e2e/helper/env changes remain as uncommitted working-tree changes.

### Environment note (caused initial false readings)
A stale `go run ./cmd/server/` (host process from the sweep) was holding `:8080` with
**pre-fix** code. Early verification curls hit it and showed old behavior. It was
stopped and replaced with a freshly-built binary of the fixed code before final
verification. (A `| tail` pipe had also masked a build failure earlier — the binary
under test must be confirmed fresh.)

## 2. Triage of all 43 failures

| Class | Count | Action |
|-------|-------|--------|
| **P0 rollout blockers** | 5 | **FIXED** (this pass): T-01, X-03, O-05, P-09, R-06 |
| Spec expects wrong status (200v201, 423v429, 401 on 2nd close) | 9 | spec drift — left alone |
| Platform token used on staff endpoints (adminToken) | 7 | spec drift — left alone |
| `fetchAudit` wrong shape (helper) | 3 | helper drift — left alone |
| Missing `PAYMENT_WEBHOOK_SECRET_STRIPE` / `MFA_ENCRYPTION_KEY` env | 7+1 | env/config — left alone (PT-03 MFA 500 is env-only, not a code bug) |
| API response shape (`order.Order.id`, org schema) | 3 | spec drift — left alone |
| Spec logic (waiter vs owner, non-existent force-close) | 4 | spec drift — left alone |
| Frontend not running | 4 | infra — left alone |

Only 5 are genuine blockers. PT-03 (MFA enroll 500) is **not** a code blocker — it is
purely a missing `MFA_ENCRYPTION_KEY` in the test env; the handler behaves correctly
when the key is set. Left for the env/spec cleanup pass.

## 3. P0 fixes — root cause, fix, regression

### T-01 — Cross-org snapshot leak (CRITICAL, security) + X-03 — auth 500 (same root)
**Root cause:** `guestParticipantID` (`internal/handlers/guest_auth.go`) failed OPEN for
a *present-but-malformed* bearer token in non-strict mode (`return legacyParticipantID,
true`). A staff JWT is malformed-as-guest, so:
- snapshot (`requireGuestSession`) authorized the caller → any session readable across
  orgs → **T-01 (200)**;
- cart (`GetCart`) proceeded with participant `0` → service error →
  `respondInternalError` → **X-03 (500)**.

**Fix:** when a bearer token is present and `Validate` returns any error, reject 401 —
remove the malformed fail-open. The `token == ""` legacy/anonymous path is unchanged, so
legacy reconnects aren't broken; expired/revoked tokens were already rejected.

**Regression:** `internal/handlers/guest_auth_test.go` (DB-free) — present-invalid token
→ 401 for both `required=false` and `true`; no-token non-strict still passes through.
**Live:** T-01 → 401, X-03 → 401, valid-token cart GET → 200.

### O-05 — Zero-quantity order → 500
**Root cause:** `placeOrderRequest.Items` binding `required,min=1` validates array length
only; `quantity:0` hit the `order_items` CHECK `(quantity > 0)` (migration 000001) →
unmatched repo error → `default: respondInternalError` → 500.
**Fix:** `PlaceOrder` now validates each item (`MenuItemID > 0`, `1 ≤ Quantity ≤ 99`) and
returns 400 before the service call.
**Live:** qty=0 → 400; qty=2 happy-path → 201.

### P-09 — Overpayment accepted → 201 (financial correctness)
**Root cause:** `InitiatePayment` (`internal/handlers/payment.go`) computed the
authoritative bill server-side but **ignored `req.Amount`**, silently accepting an
overpayment.
**Fix:** after `ComputeBillForSession`, reject with 422 `PAYMENT_AMOUNT_INVALID` when
`req.Amount > bill.Total + 0.01` (rounding epsilon). No tip field exists in this flow,
so any overage is an overpayment. Stored amount remains the server-computed total.
**Live:** amount 99999 → 422 `PAYMENT_AMOUNT_INVALID`; exact-amount pay → 201.

### R-06 — Snapshot authority ambiguity
**Root cause:** `SessionSnapshot` had no authority signal, and `last_sequence=0` skipped
the missed-events fetch → the client could not tell a full reconcile from an empty
incremental replay.
**Fix:** added `snapshot_authoritative` (bool). Set `true` when `last_sequence == 0`
(no incremental basis) or when a replay gap is detected (first returned event
`Sequence > last_sequence+1`, i.e. older events pruned). Matches
`realtime-reconciliation-invariants` (Postgres-authoritative full reconcile).
**Live:** `last_sequence=0` → `snapshot_authoritative: true`.

## 4. Commits (backend only; no co-authors)

```
6db078e Reject present-but-invalid guest tokens instead of failing open (T-01, X-03)
8b5c386 Validate per-item order quantity to prevent 500 on zero quantity (O-05)
463e72f Reject overpayment exceeding bill total (P-09)
54b72f6 Add snapshot_authoritative signal for full-reconcile snapshots (R-06)
```

`go build`, `go vet`, `gofmt`, and `go test ./internal/handlers/` all clean. The report,
`api.ts`, the `AUTH_RATE_LIMIT_RPM` change, `docker-compose.staging.yml`, and the e2e
spec edits were intentionally NOT committed (out of P0 scope; preserved in the tree).

## 5. Verification performed

- Go unit test (guest_auth) green.
- Live reproductions against a freshly-built binary in the staging stack — all five P0s
  now return the correct status; two happy-path + spot-checks confirm no regression
  (see table in §3).
- The 5 Playwright P0 specs encode the same assertions and are CI-runnable once the
  platform-admin bootstrap is available (a seeded platform user + a Redis-backed
  `e2e-admin-secret` token, which `PlatformAuth.ValidateToken` requires). The full
  Playwright run was NOT executed here because that bootstrap is not wired in this
  sandbox; direct curl reproductions were used instead and are authoritative for the
  code paths fixed.

## 6. Remaining (non-blocking) failures

38 of 43 remain and are NOT rollout blockers: spec status drift, `adminToken`→staff-token
spec edits, `fetchAudit` shape, missing webhook/MFA env vars, wrapped response shapes,
and frontend-required specs. They are tracked in `e2e-failure-analysis.md` with fix
owners and should be cleared in a separate spec/helper/env cleanup pass (and the suite
re-run in CI). None affect production correctness.

## 7. Residual risks

- **Snapshot legacy no-token path:** with `AUTH_GUEST_CREDENTIALS_REQUIRED=false`, a
  request with NO bearer token can still read a snapshot given the (unguessable) session
  UUID — the intended legacy/anonymous reconnect, removed at Wave R6. T-01 (a *presented*
  staff token) is fully closed. The no-token path is unchanged and R6-gated; not a new risk.
- **e2e suite green** still depends on the separate spec/helper/env cleanup + the
  platform-admin bootstrap. Not a code risk.
- **Overpayment bound** assumes no tip in the initiation flow (true today). If a tip
  field is ever added, the bound must include it.

## 8. Recommendation

**GO for R1.** All five P0 blockers are fixed and verified live, including the critical
cross-org snapshot leak (T-01) — staff tokens are now rejected, financial overpayment is
rejected, and the two unhandled-500 paths return correct 4xx. No auth or tenancy
semantics were weakened. The remaining 38 failures are spec/helper/env/frontend drift and
do not block rollout; clear them in a separate non-blocking cleanup and re-run the full
suite in CI before relying on it as a gate.
