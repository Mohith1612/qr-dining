# Final Hardening — Phase B Report

Date: 2026-05-24
Branch: main
Scope label: Phase B — auth security completion, MFA, lockouts, lifecycle
worker, support session hardening.

## 1. What This Phase Did

Phase B closes the auth-surface gaps and operational lifecycle work that
Phase A explicitly deferred. The platform now ships with TOTP MFA for
platform admins, brute-force lockout on every auth endpoint, an HttpOnly
cookie option for staff sessions, the full awaiting_reactivation worker
pipeline, support-session caps and tenant-visible audit, and an API +
frontend security-header baseline.

After Phase B, the only major work blocking strict rollout is frontend
client migration (cookie transport, ws-ticket-always, drop legacy
X-Participant-ID) and a 7-day staging burn-in.

## 2. What Was Audited

- Auth surfaces: `/staff/auth`, `/platform/auth`, MFA, session validators,
  token storage, cookie posture.
- Support session lifecycle: creation caps, expiry handling, audit
  visibility, tenant signalling.
- Session lifecycle: presence detection, awaiting_reactivation transitions,
  worker race with payment, snapshot reconnect path.
- Rate limiting and lockout: fail-open vs fail-closed surfaces, per-account
  vs per-IP limits, recovery procedures.
- Security headers: Go API (defence-in-depth) and Next.js (full CSP).
- Strict-mode compatibility paths still in code.

## 3. What Was Implemented

### 3.1 CSP + security headers

- `backend/internal/middleware/security_headers.go` — sets the strict
  API-shaped CSP `default-src 'none'; frame-ancestors 'none'`, plus
  `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`, `Permissions-Policy: ...`, COOP, CORP.
  HSTS is gated on `ENABLE_HSTS=true` because dev uses plain HTTP.
- `frontend/next.config.ts` — Next.js header pass for HTML responses with a
  production CSP that honors the API host, R2 public domain, and WS upgrade.
  `script-src 'unsafe-inline'` remains because Next.js hydration scripts are
  inline today; the nonce migration is tracked as Phase C work.

### 3.2 Brute-force lockout

- `backend/internal/redis/lockout.go` — Redis-backed sliding-window counter +
  separate lockout TTL. Two keys per identity: `counter:` (TTL = window) and
  `locked:` (TTL = lockout duration). Honest single-mistype users don't
  accumulate strikes across a shift because `Reset` is called on every
  successful login.
- Staff auth: 10 failures / 5 min / `(branch_id, staff_code)` triggers a
  15-min lockout. HTTP 423 `AUTH_LOCKED_OUT` with `Retry-After`.
- Platform auth: stricter — 5 failures / 5 min / email triggers a 30-min
  lockout.
- Lockout backend failure is fail-closed for both surfaces (matches the
  rate-limit policy added in Phase A).
- Audit events: `staff.login.failed` with `reason: locked_out` and
  `platform.auth.locked_out` for forensic timeline reconstruction.

### 3.3 MFA (TOTP) for platform users

- New migration `000025_platform_mfa.{up,down}.sql` adds
  `platform_user_mfa` (TOTP secret encrypted with AES-GCM, bcrypt'd recovery
  codes, status, enrolled/used timestamps) and `platform_mfa_challenges`
  (ephemeral 5-min challenge tokens issued between password success and TOTP
  verification).
- `backend/internal/crypto/totp.go` — RFC 6238 TOTP (SHA-1 / 30s / 6 digits),
  dependency-free implementation, drift tolerance ±1 step, format-tolerant
  verifier. Compatible with Google Authenticator, Authy, 1Password.
- `backend/internal/crypto/aesgcm.go` — AES-GCM helpers keyed by
  `MFA_ENCRYPTION_KEY`. Configuration validation fails closed if the key is
  shorter than 16 bytes.
- `backend/internal/services/platform_mfa.go` — full flow: `BeginEnrollment`,
  `ConfirmEnrollment` (returns 10 single-use recovery codes; only their
  bcrypt hashes persist), `VerifyMFA`, `DisableMFA`, `IssueMFAChallenge`,
  `CompleteMFAChallenge`. Recovery code consumption removes the matched
  code from the pool atomically.
- `Authenticate` now returns either a session or an MFA challenge via the
  new `PlatformAuthResult` envelope; the session token is briefly issued and
  revoked when MFA is active so a failed second factor leaves no zombie
  session in Redis.
- New routes: `POST /platform/auth/mfa` (complete challenge),
  `POST /platform/mfa/enroll` (begin), `POST /platform/mfa/confirm` (with
  TOTP code, returns recovery codes), `POST /platform/mfa/disable`.
- Audit events: enrolled, disabled, challenge issued, verified, failed.
- TOTP and AES-GCM both have unit tests in `internal/crypto/totp_test.go`.

### 3.4 HttpOnly cookie for staff auth

- `backend/internal/middleware/staff_auth.go` now accepts the staff token
  from EITHER the `Authorization: Bearer` header OR the new
  `qrd_staff_session` HttpOnly cookie. Both routes hit the same
  Redis/DB-backed session row, so honoring either is safe.
- Cookie issuance is gated by `AUTH_STAFF_COOKIE_ENABLED`. When on, the
  staff `Authenticate` handler sets the cookie alongside returning the JSON
  body. `Secure` is set when `GIN_MODE=release` or `ENABLE_HSTS=true`;
  `SameSite=Lax`; 8-hour TTL aligned to the existing access-token TTL.
- New endpoint `POST /staff/logout` clears the cookie. Bearer clients drop
  their local token as before — cookies need explicit clearing.
- This unblocks the frontend migration off localStorage without breaking
  any current client.

### 3.5 Support session hardening

- `CreateSupportSession` rejects durations > 24h (hard cap) outright and
  durations > 4h (soft cap) without elevated approval (HTTP 422). Reason
  text is required to be non-blank.
- `GetSupportSession` now returns HTTP 410 `SUPPORT_SESSION_EXPIRED` once
  `expires_at` has passed; the platform audit log records the attempt as
  `read_expired` even though the response was 410.
- Every successful support session read writes a tenant-visible audit event
  (`audit.ActionPlatformSupportAccess`, risk = critical). Org owners can see
  every platform support touch on their data.

### 3.6 awaiting_reactivation worker pipeline

- Migration `000026_session_reactivation.{up,down}.sql` adds
  `sessions.awaiting_reactivation_at` plus a partial index.
- New SQL queries: `TransitionSessionToAwaitingReactivation`,
  `ReactivateSession`, `ListActiveSessionsForReactivationScan`,
  `ListSessionsAwaitingReactivationExpired`, `HasNonTerminalPaymentForSession`.
- New worker `RunReactivationPipeline` runs every `PRESENCE_EXPIRY_INTERVAL`:
  1. Active sessions older than `SESSION_PRESENCE_GRACE` (default 60s) with
     no Redis presence are transitioned to `awaiting_reactivation`. Skipped
     if any non-terminal payment exists (TIM-1).
  2. Sessions in `awaiting_reactivation` older than
     `SESSION_REACTIVATION_WINDOW` (default 5m) are abandoned — table
     released, participants revoked, credential_version bumped, table
     reconciled. Again skipped if a non-terminal payment exists.
- Worker emits `SESSION_AWAITING_REACTIVATION` and
  `SESSION_ABANDONED reason=reactivation_window_expired` events.
- Snapshot endpoint `GetSnapshot` transitions
  `awaiting_reactivation → active` when a guest reconnects inside the
  window, publishing `SESSION_REACTIVATED`. Outside the window the row has
  already moved on (worker abandoned it) so the existing terminal-window
  path applies.
- `AbandonStaleSession` SQL extended to accept both `active` and
  `awaiting_reactivation`; the FOR-UPDATE scan in repository was updated to
  match.

### 3.7 Domain + error codes

- `internal/domain/errors.go`: added `ErrAuthLockedOut`, `ErrMFARequired`,
  `ErrMFAInvalidCode`.
- `internal/handlers/errors.go`: added `CodeAuthLockedOut`, `CodeMFARequired`,
  `CodeMFAInvalidCode`, `CodeRateLimiterUnavailable`.

## 4. Strict-Mode Compatibility Cleanup (Audit)

Phase B did not delete any compatibility path — the rollout document
explicitly forbids that until each flag has soaked. Remaining instrumented
compatibility surfaces (still tracked via `legacy_identity_usage_total`):

| Mechanism | Endpoint | Removal blocker |
| --- | --- | --- |
| `x_participant_id` header | cart, session join | Frontend must send `Authorization: Bearer` everywhere first |
| `body_participant_id` | assistance | Frontend assistance form still posts participant id |
| `body_placed_by_participant_id` | orders | Already rejected unconditionally — instrumentation is dead but kept until field can be dropped from the request DTO |
| `branch_id_pin` | `/staff/auth` legacy body | Staff onboarding migration to staff_code needs a UI flow |
| `ws_query_participant_id` | `GET /ws` | Frontend must always fetch `/sessions/:id/ws-ticket` first |
| `body_branch_id` | menu admin | Frontend admin form still includes branch_id in update bodies |

When each metric has been flat-zero for the documented soak period, the
corresponding strict flag flip removes the bypass without breaking traffic.

## 5. Files Changed

Backend:
- New: `backend/migrations/000025_platform_mfa.{up,down}.sql`
- New: `backend/migrations/000026_session_reactivation.{up,down}.sql`
- New: `backend/internal/crypto/{totp.go,aesgcm.go,totp_test.go}`
- New: `backend/internal/middleware/security_headers.go`
- New: `backend/internal/redis/lockout.go`
- New: `backend/internal/repository/platform_mfa.go`
- New: `backend/internal/services/platform_mfa.go`
- `backend/internal/config/config.go` (new env vars)
- `backend/internal/middleware/staff_auth.go`
- `backend/internal/middleware/ratelimit.go` (Phase A surfaces unchanged)
- `backend/internal/redis/presence.go` (added GetPresentScoped)
- `backend/internal/repository/session.go`
- `backend/internal/repository/worker.go`
- `backend/internal/services/staff.go`
- `backend/internal/services/platform.go`
- `backend/internal/services/session.go`
- `backend/internal/handlers/staff.go`
- `backend/internal/handlers/platform.go`
- `backend/internal/handlers/helpers.go`
- `backend/internal/handlers/errors.go`
- `backend/internal/server/server.go`
- `backend/internal/worker/worker.go`
- `backend/cmd/server/main.go`
- `backend/cmd/server/worker_adapter.go`
- `backend/sql/queries/sessions.sql`
- `backend/sql/queries/workers.sql`
- `backend/sql/queries/platform_mfa.sql`
- `backend/internal/domain/errors.go`

Frontend:
- `frontend/next.config.ts` (CSP + security headers)

Docs:
- `final-hardening-phase-b-report.md` (this file)

## 6. Strict-Mode Readiness Per Flag (updated)

Same flag set as Phase A; updates reflect what's now resolved.

| Flag | Status | Blockers / risk |
| --- | --- | --- |
| `AUTH_GUEST_CREDENTIALS_REQUIRED` | Ready | Frontend must send `Authorization: Bearer` everywhere; gate on legacy metric ≈ 0 for 24h. |
| `AUTH_STAFF_CODE_REQUIRED` | Ready | Staff onboarding UI for staff codes is the remaining work. |
| `AUTH_STAFF_SESSION_DB_REQUIRED` | Ready | Strict already wired (Phase A). |
| `AUTHZ_CENTRAL_POLICY_ENFORCE` | Soak | Watch `legacy_authz_bypass_total` for 7 days. |
| `TENANCY_ORGANIZATIONS_ENABLED` | Ready | No-op flip. |
| `AUDIT_LOG_V2_ENABLED` | Ready | No-op flip. |
| `WS_TICKET_AUTH_REQUIRED` | Ready | Frontend WS connect must always fetch a ticket; gate on `ws_query_participant_id` ≈ 0. |
| `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | Ready | Combined with Phase A payment_pending freeze. |
| `STRICT_BRANCH_SCOPED_MUTATIONS` | Soak | Watch `body_branch_id` metric for 7 days. |

New Phase B flags:

| Flag | Default | Effect |
| --- | --- | --- |
| `AUTH_STAFF_COOKIE_ENABLED` | false | Issue HttpOnly cookie on `/staff/auth` alongside Bearer; both transports remain accepted. |
| `ENABLE_HSTS` | false | Emit `Strict-Transport-Security` on every API response. Flip only when TLS terminates in front of the API. |

New required env vars when MFA is in use:

| Var | Default | Effect |
| --- | --- | --- |
| `MFA_ENCRYPTION_KEY` | "" | AES-GCM key for TOTP secret at rest. Required (min 16 bytes) if any platform user has `mfa_required=true`. Service errors closed otherwise. |
| `SESSION_PRESENCE_GRACE` | 60s | How long an active session may have empty Redis presence before becoming awaiting_reactivation. |
| `SESSION_REACTIVATION_WINDOW` | 5m | How long awaiting_reactivation persists before the worker abandons. |

## 7. Remaining Risks

Carried forward from Phase A (still applicable):

- Frontend cookie migration is not done. The cookie path exists on the
  backend; the frontend still uses Zustand `persist` → localStorage. Until
  the frontend moves, the headline gain from Phase B's cookie work is
  defence-in-depth (a Bearer-only client doesn't get the HttpOnly benefit).
- No nginx config in repo; HSTS / TLS termination posture is operator
  responsibility.
- Inline scripts in Next.js CSP — nonce migration is Phase C.
- Organization user auth split from staff PIN is still outstanding.

New / refined from Phase B:

- MFA enrollment is opt-in. Setting `mfa_required=true` on a user without
  prior enrollment locks them out of all writes that require MFA but does
  not auto-prompt enrollment. The provisioning UX is the next step on the
  platform admin frontend.
- Recovery codes are shown ONCE at enrollment; the admin UX must surface a
  prominent "save these now" interstitial.
- The awaiting_reactivation worker uses Redis presence as the only liveness
  signal — there is no `sessions.last_activity_at` column. A session that
  had presence + an empty WebSocket but real HTTP traffic in the last
  minute is still flagged as candidate-for-awaiting if no presence
  heartbeat was sent. The presence heartbeat path on the frontend must
  remain reliable. If this becomes a problem in production, adding
  `last_activity_at` updated by every HTTP handler is the fix.
- Brute-force counters live in Redis. A Redis outage on a fail-closed
  surface (staff auth, platform auth, payment, webhook) returns 503 and
  blocks all new logins. This is intentional — the runbook section on
  Redis outage should document the manual `Unlock` operator path that the
  `LockoutStore` exposes.
- Cookie SameSite is `Lax`, not `Strict`. Strict would break top-level
  navigations from email-linked staff dashboards. CSRF posture remains:
  bearer header is forgery-resistant; once the cookie is the only
  transport, the staff API needs an explicit CSRF token strategy (Phase C).

## 8. Recommended Phase C

1. Frontend HttpOnly cookie migration + CSRF token on every mutation.
2. CSP nonce migration (drop `'unsafe-inline'` for scripts).
3. Organization user auth (email/password + MFA) separate from staff PINs.
4. WS per-connection inbound rate limit + message size cap.
5. R2 upload validation hardening (type sniff on confirm, branch quota).
6. Per-branch grace/window overrides via `branches.settings_json`.
7. MFA enforcement UX: enrollment auto-prompt, recovery-code regeneration
   flow, force-disable with audit.
8. Begin deleting compatibility paths once each metric has soaked at zero.

## 9. Verification

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test -short ./...` — all unit tests pass.
- `go test ./internal/crypto/...` — TOTP and AES-GCM tests pass (including
  format-tolerance, wrong-key, and full TOTP roundtrip).
- `go tool sqlc generate` — clean.

Migrations:
- `000025` and `000026` are additive (column add + new tables + new index).
  `down` migrations drop the additions. No data loss path.

Manual smoke tests recommended before any flag flip in staging:
- Enroll MFA on a platform user, log out, log in with TOTP, lose TOTP, log
  in with a recovery code, observe code consumed.
- Trigger 11 failed staff logins for one staff code; confirm 423 + retry-after.
- Force Redis outage; confirm staff auth returns 503 (fail-closed).
- Start a session, kill the tab, wait `SESSION_PRESENCE_GRACE`, then
  reconnect within `SESSION_REACTIVATION_WINDOW`; snapshot should
  re-activate. Then wait past the window and confirm abandonment.
- Create a support session > 24h; confirm 422.
- Read an expired support session; confirm 410.
- Curl any API route; confirm CSP / X-Content-Type-Options / X-Frame-Options
  headers are present.
