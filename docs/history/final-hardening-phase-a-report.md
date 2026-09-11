# Final Hardening — Phase A Report

Date: 2026-05-22
Branch: main
Scope label: Phase A — production correctness + security hardening (pre-rollout).

## 1. What This Phase Did

Phase A closed the unwired-flag gaps and the resurrection / freeze gaps that
made the QR Dining strict-mode flags unsafe to flip. After this phase, every
strict-enforcement flag in `backend/internal/config/config.go:172-180`
materially changes behavior when toggled, and the documented correctness
invariants from the spec docs are enforced in code.

Phase A is **not** rollout. All nine flags still default `false`. Rollout
itself is the next operational exercise, governed by
`production-enforcement-rollout.md` and the staging burn-in plan.

## 2. What Was Audited

- Session lifecycle code path (`services/session.go`, `worker/worker.go`,
  `handlers/snapshot.go`) against `session-lifecycle-state-machine.md`.
- Payment finalization (`services/payment.go`, `handlers/payment.go`, the
  webhook handler, idempotency code, bill snapshot) against
  `payment-finalization-invariants.md`.
- Realtime / WebSocket path (`handlers/ws.go`, `redis/ws_ticket.go`, the hub,
  `events.Publisher`) against `realtime-reconciliation-invariants.md`.
- Authz / branch isolation, including the five converted handlers (order,
  assistance, menu_admin, promo, customer/staff, payment) and the policy
  package, against `security-hardening-checklist.md` and the architecture
  plan.
- Rate-limit posture, fail-open/fail-closed behavior, and per-surface
  coverage.
- Audit log immutability, redaction, and writer coverage.
- Frontend token storage and WS connection paths.

## 3. What Was Fixed

### 3.1 Session lifecycle

- Migration `000023_session_lifecycle_states.up.sql` adds the three missing
  enum values `payment_pending`, `awaiting_reactivation`, `expired` to
  `session_status` and adds `session_participants.revoked_at` plus
  `revoked_reason` for credential revocation. A new partial index
  `idx_session_participants_active` keeps the lookup hot path cheap.
- Migration `000024_session_lifecycle_invariants.up.sql` rebuilds the partial
  unique index `idx_sessions_one_active_per_table` to cover the non-terminal
  statuses (`active`, `payment_pending`, `awaiting_reactivation`). This was
  the gap that allowed a second active session to appear on a table while a
  payment was in flight.
- `internal/domain/statemachine.go` adds the three new statuses, the full
  transition matrix per spec, and helpers `IsSessionTerminal` /
  `IsSessionCartFrozen`. Unit tests cover every new transition.
- `internal/services/session.go:CloseSession` now revokes every participant
  (`RevokeAllParticipants`) and increments `credential_version`
  (`BumpAllParticipantCredentialVersions`) inside the same transaction that
  closes the session and releases the table. Closed sessions are
  conclusively un-reusable: even a stored guest token from the prior session
  fails validation because its `credential_version` is stale and its
  participant row is marked `revoked_at`.
- `internal/repository/worker.go:AbandonStaleSession` does the same in the
  worker's transaction, so a session that times out without an explicit
  close still revokes credentials atomically.
- `internal/handlers/snapshot.go` and `services/session.go:GetSnapshot`
  surface a 60-minute terminal read window. Inside the window the snapshot
  returns 200 with `session_ended: true` and `close_reason`. Outside the
  window, callers receive HTTP 410 `SESSION_ENDED`.

### 3.2 Guest credential rejection

- `internal/handlers/guest_auth.go` rejects any token whose participant has
  `revoked_at` set (HTTP 401 `UNAUTHORIZED`).
- `internal/handlers/ws.go` rejects WebSocket upgrades the same way on both
  the ticket path and the legacy query-string path.

### 3.3 Payment-pending freeze

- `internal/services/payment.go:InitiatePayment` transitions the session
  `active → payment_pending` inside the existing transaction (idempotent if
  the session was already in `payment_pending`; rejects any other status with
  `ErrSessionNotActive`).
- New helper `maybeReleasePaymentPending` returns the session to `active`
  once every payment on the snapshot has terminally failed/cancelled.
- `internal/services/cart.go` and `internal/services/order.go` reject
  mutations when the session is in `payment_pending` (HTTP 409
  `PAYMENT_IN_PROGRESS`).
- New error code `CodePaymentInProgress` plus domain error
  `domain.ErrPaymentInProgress`.

### 3.4 Three previously unwired flags

- `AUTHZ_CENTRAL_POLICY_ENFORCE`: The authorizer is now constructed via
  `authz.NewEnforcingAuthorizer(...)`. `handlers.requireAuthorized` honors
  `authorizer.Enforce()`: in shadow mode it records the would-have-denied
  event (audit + new `legacy_authz_bypass_total{action,actor_role}` metric)
  but allows the request; in strict mode it returns 403 as before.
- `STRICT_BRANCH_SCOPED_MUTATIONS`: New helper
  `handlers.enforceBranchScopeFromBody` rejects (strict) or warns (shadow)
  when the client-supplied `branch_id` differs from the loaded resource's
  branch. Applied to `menu_admin` UpdateItem / ToggleAvailability /
  ToggleFeatured — the three handlers where a stale or hostile client could
  meaningfully drive cross-branch confusion.
- `AUTH_STAFF_SESSION_DB_REQUIRED`: `StaffService.validateSessionState`
  accepts a per-bootstrap `requireDBRow` flag. In strict mode, any token
  whose `staff_sessions` row is missing or stale is rejected; in shadow mode
  the Redis hit is still trusted (so flipping the flag does not invalidate
  every active shift). Tokens already re-check `is_active`, `pin_version`,
  and `token_version` against the DB regardless of flag state.

### 3.5 Rate-limit hardening

- `internal/middleware/ratelimit.go` adds two new helpers:
  - `RateLimitSensitive(rl, surface, n)` fails *closed* with HTTP 503
    `RATE_LIMITER_UNAVAILABLE` when the Redis backend is unreachable, and
    increments `rate_limiter_unavailable_total{surface}`. Applied to
    `/staff/auth`, `/platform/auth`, `/sessions/:id/payments`,
    `/webhooks/payments/:provider`, `/sessions/:id/ws-ticket`.
  - `RateLimitByKey(rl, prefix, n, keyFn)` limits by a caller-derived key
    (session id, participant id) instead of client IP. Applied per-session
    to payment initiate (6/min), WS ticket issuance (12/min), order
    placement (12/min), assistance (6/min).

### 3.6 Audit redaction

- `internal/audit/redaction.go` extends `sensitiveKeys` with `signature`,
  `webhook_signature`, `payment_signature`, `x_payment_signature`,
  `csrf_token`, `refresh_token`, `access_token`, `client_secret`,
  `private_key`, `api_key`, `otp`, `otp_code`, `mfa_code`, `recovery_code`,
  `pan`, `card_pan`.
- `sensitiveSubstrings` adds `signature`, `mfa`, `2fa`, `private_key`,
  `api_key`, `refresh_token`, `access_token`, `_pin`, `pin_` so derived
  names (e.g. `stripe_signature`, `X-Razorpay-Signature`, `mfa_secret`,
  `new_pin_hash`) are also caught without enumeration.
- New tests `TestRedact_PhaseAExpandedKeys` and
  `TestRedact_PhaseASubstrings` cover both axes.

## 4. Files Modified

Backend (build clean, `go vet` clean, unit tests green):

- New: `backend/migrations/000023_session_lifecycle_states.{up,down}.sql`
- New: `backend/migrations/000024_session_lifecycle_invariants.{up,down}.sql`
- `backend/internal/domain/statemachine.go` (+ tests)
- `backend/internal/domain/errors.go`
- `backend/sql/queries/sessions.sql` (+ regenerated sqlc)
- `backend/internal/repository/session.go`
- `backend/internal/repository/worker.go`
- `backend/internal/services/session.go`
- `backend/internal/services/payment.go`
- `backend/internal/services/cart.go`
- `backend/internal/services/order.go`
- `backend/internal/services/staff.go`
- `backend/internal/handlers/guest_auth.go`
- `backend/internal/handlers/ws.go`
- `backend/internal/handlers/snapshot.go`
- `backend/internal/handlers/cart.go`
- `backend/internal/handlers/order.go`
- `backend/internal/handlers/payment.go` (touched indirectly via service)
- `backend/internal/handlers/menu_admin.go`
- `backend/internal/handlers/authz.go`
- `backend/internal/handlers/helpers.go`
- `backend/internal/handlers/errors.go`
- `backend/internal/authz/policy.go`
- `backend/internal/middleware/ratelimit.go`
- `backend/internal/server/server.go`
- `backend/internal/audit/redaction.go` (+ tests)
- `backend/internal/observability/metrics.go`

Generated:
- `backend/internal/db/sqlc/*` (regenerated via `go tool sqlc generate`).

Docs:
- `final-hardening-phase-a-report.md` (this file).

## 5. Strict-Mode Readiness Per Flag

Definitions:
- **Ready** — flipping ON is expected to be a safe no-op for healthy traffic.
- **Soak** — code is in place but needs N days of shadow-mode telemetry
  before flipping.
- **Blocked** — there is a prerequisite outside Phase A.

| Flag | Status | Blockers / risk |
| --- | --- | --- |
| `AUTH_GUEST_CREDENTIALS_REQUIRED` | **Ready** | Frontend must send `Authorization: Bearer` on every guest call before flip. Already supported by `guestParticipantID`. Flip after `legacy_identity_usage_total{mechanism="header_participant_id"}` is zero for 24h. |
| `AUTH_STAFF_CODE_REQUIRED` | **Ready** | All active staff must have a generated `staff_code` (auto-backfilled in migration 000016). Flip after a single shift cycle where every login used the staff_code path. |
| `AUTH_STAFF_SESSION_DB_REQUIRED` | **Ready** | Flag is now wired into `validateSessionState`. Flip is a no-op for healthy traffic — every login since Phase 1 created a `staff_sessions` row. Shadow-mode soak recommended for 7 days. |
| `AUTHZ_CENTRAL_POLICY_ENFORCE` | **Soak** | Wired to `requireAuthorized`. Watch `legacy_authz_bypass_total` for 7 days at production load. If counter is zero, flipping is safe. If non-zero, the named actions need policy fixes first. |
| `TENANCY_ORGANIZATIONS_ENABLED` | **Ready** | Already strict in middleware (`TenantMiddleware`, `BranchTenantGuard`). No-op flip. |
| `AUDIT_LOG_V2_ENABLED` | **Ready** | `audit.NewWriter` already honors flag. No-op flip. |
| `WS_TICKET_AUTH_REQUIRED` | **Ready** | `ws.go` rejects query-string auth when flag is on. Frontend must always call `POST /sessions/:id/ws-ticket` before connecting. Verify zero `legacy_identity_usage_total{mechanism="ws_query_participant_id"}` over a 24h window before flip. |
| `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | **Ready** | Cash / card_manual / UPI already route to `requires_staff_confirmation` when flag is on. Combined with the new `payment_pending` freeze (Phase A), the contract holds. Flip after a staff training pass. |
| `STRICT_BRANCH_SCOPED_MUTATIONS` | **Soak** | Wired into the three highest-risk menu admin handlers. Other handlers either reject body branch_id unconditionally already (order) or do not accept it. Watch `legacy_identity_usage_total{mechanism="body_branch_id"}` for 7 days. |

## 6. Remaining Risks (Out of Phase A Scope)

These are documented and tracked but were explicitly not in scope (per the
scope decision at plan time):

- **Frontend staff token storage**: `frontend/store/staff.ts:39` still uses
  Zustand `persist` (localStorage). XSS or shared-device exposure remains.
  Mitigation: Phase B — move to HttpOnly cookie scoped to `/staff`. The
  backend cookie issuance path needs design.
- **No CSP / security-header middleware** in the Go server: the production
  reverse proxy must supply `Content-Security-Policy`, HSTS, `X-Frame-Options`,
  `Referrer-Policy`, COOP/CORP. There is no nginx config in the repo to
  validate.
- **Platform user MFA** is column-only; the verification path is not wired.
  Login at `/platform/auth` succeeds without TOTP today.
- **Brute-force lockout windows** beyond rate-limit: no per-`(branch_id,
  staff_code)` lockout, no per-username platform lockout. The Phase A
  rate-limit-fail-closed change is a partial mitigation only.
- **`awaiting_reactivation` worker grace flow**: the state exists in the enum
  and transition table, but the stale-session worker still moves
  `active → abandoned` directly. The spec calls for a
  `presence_grace → quiet_grace → reactivation_window → abandoned` pipeline.
  Out of scope here (touches presence, snapshot writes, multi-device
  reconnect — needs its own design review).
- **JTI denylist**: not implemented. Instead, `credential_version` is bumped
  on every terminal session transition, which has the same invalidation
  effect for the legitimate guest token flow. A genuine JTI-replay attack
  surface (where signature is valid but `credential_version` has been
  rotated) is closed by the `credential_version` check in the validator.
- **Organization user auth** is still bridged via staff `role=owner`. The
  spec wants an independent OAuth / magic-link flow. Out of scope.
- **WS hub inbound rate-limit per connection** (60 msg/min) — not implemented;
  current behavior closes the socket only on outbound buffer overflow (256).
- **R2 upload validation**: presign requires staff token but does not
  enforce content-type sniff on upload confirm or quota per branch.

## 7. Rollout Confidence

Medium-high. The two riskiest changes (`session.CloseSession` now revoking
participants + bumping credential version; `payment.InitiatePayment` now
transitioning the session) both run inside their pre-existing transactions
and are idempotent for already-in-target states. The new error returns
(409 `PAYMENT_IN_PROGRESS`, 410 `SESSION_ENDED`, 503
`RATE_LIMITER_UNAVAILABLE`) are additive — existing clients that never
encounter them are unaffected.

Recommendation: deploy to staging, run the burn-in plan from
`staging-burnin-strategy.md` for 7 days with all flags still `false`,
confirm the new metrics report zero or expected counts, then flip flags in
the dependency order from `production-enforcement-rollout.md` §3.

## 8. Known Remaining TODOs

- Frontend: stop sending `X-Participant-ID` and `placed_by_participant_id`.
- Frontend: always fetch `/sessions/:id/ws-ticket` before WS connect.
- Frontend: surface `PAYMENT_IN_PROGRESS` 409 as a user-facing freeze.
- Frontend: handle 410 `SESSION_ENDED` by clearing local session state.
- Backend: add awaiting_reactivation worker pipeline (Phase B).
- Backend: add CSP middleware + nginx config (Phase B).
- Backend: HttpOnly cookie path for staff token (Phase B).
- Backend: MFA enforcement for `platform_users` (Phase B).

## 9. Recommended Next Phase (B)

1. Frontend cookie migration + CSP + security headers.
2. Platform admin MFA enforcement.
3. Brute-force lockout per `(branch_id, staff_code)` and per platform user.
4. `awaiting_reactivation` worker pipeline + reconnect transitions.
5. Organization user auth split (email/password / magic link + MFA).
6. WS inbound per-connection rate limit and message size cap.

## 10. Verification

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — all unit tests pass (integration tests gated on
  `DATABASE_URL` / `REDIS_URL` env vars; not run in this session).
- `go tool sqlc generate` — clean.
- Migrations: `000023` and `000024` are additive, with `down` migrations.
  Enum values cannot be dropped on rollback — documented in 000023 down.
- New tests added:
  - `domain/statemachine_test.go`: every new transition + helpers.
  - `audit/redaction_test.go`: expanded keys + substring matches.

Manual integration smoke tests (DB-backed) should be run before flipping
any flag in staging.
