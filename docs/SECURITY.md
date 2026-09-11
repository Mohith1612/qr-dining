# Security

The security model as the code actually implements it. Verified against `feature/signoz-observability` @ `9865a48`.

Companions: [ARCHITECTURE.md](ARCHITECTURE.md) · [OPERATIONS.md](OPERATIONS.md) · item-level gate list in [reference/security-hardening-checklist.md](reference/security-hardening-checklist.md).

> The checklist is the §-numbered contract that code comments cite (for example `internal/middleware/security_headers.go` cites §9). Its checkboxes lag its own Phase A/B status notes — treat the notes and this document as current, the checkboxes as historical.

---

## 1. Threat model

Adversaries the system is designed against:

- Unauthenticated actors hitting public routes — webhooks, QR resolution, snapshot reads.
- Compromised or shared staff devices on branch Wi-Fi.
- A guest who joins a session, captures local identifiers, and tries to act as another participant.
- XSS or a hostile browser extension on a staff or guest device.
- Replay of any observed HTTP request, including signed webhooks.
- A connection that outlives the user's intent.
- One branch reading another branch's data; one organization reaching another's.
- An organization owner escalating to platform admin.

## 2. Tenancy model

`organization → branch → table`, with sessions, staff, menus, promos and audit rows all branch- or org-scoped.

Enforcement is layered:

1. **Tenant resolution middleware** establishes org/branch context from the request.
2. **`branch_guard` middleware** verifies branch ownership on every branch-scoped route.
3. **Per-handler checks** enforce role and ownership.
4. **Redis keys are tenant-scoped in the key itself** (`org:{org_id}:branch:{branch_id}:session:{id}:…`), so a pub/sub fan-out cannot cross a tenant boundary even by accident.

**Cross-organization and cross-branch reads and writes are denied regardless of rollout-flag state.** This was an audited blocker, fixed, and is now covered by regression tests in `backend/internal/handlers/authz_scope_integration_test.go`. Investigation: [../release-certification/authz-scope-investigation-2026-08-04.md](../release-certification/authz-scope-investigation-2026-08-04.md).

A legacy `restaurants` model still exists alongside `organizations`, kept alive until strict tenancy enforcement (wave R2/R3) is metric-proven. The additive-only migration rule protects it.

## 3. Authentication — three trust domains

Tokens from one domain are **rejected** by the other two. This separation is structural, not a policy check.

### Guest

- HMAC-SHA256 signed token carrying `session_id`, `branch_id`, `table_id`, `org_id`, `participant_id`, `credential_version`, `jti`.
- Secret from `GUEST_TOKEN_SECRET`. Release mode **refuses to start** on the dev default or a short value.
- `GUEST_TOKEN_TTL` is `12h` for launch — deliberately long because no token-refresh flow exists yet.
- The server derives the participant from the token and ignores client-supplied participant IDs under strict flags.
- **Revocation:** every terminal session transition sets `session_participants.revoked_at` and bumps `credential_version`. The validator rejects revoked tokens, which is what prevents session resurrection.

Bespoke rather than a JWT library. Non-standard but sound; migrating would be cosmetic.

### Staff

- `branch_code + staff_code + PIN`. **PIN alone is not accepted** — that ambiguity was an original deployment blocker.
- PINs are bcrypt cost 12 (~100 ms per check) and are never logged. Rotation requires the current PIN and bumps `pin_version`, invalidating sessions across devices.
- Durable `staff_sessions` table holds the token hash, version and expiry. Validation re-checks `is_active`, `pin_version` and `token_version` against the database on every request.
- **Lockout:** 10 failures / 5 min per `(branch_id, staff_code)` → 15-minute lock. Returns HTTP 423 `AUTH_LOCKED_OUT` with `Retry-After`. Fails closed if Redis is unavailable.
- Optional HttpOnly cookie transport behind `AUTH_STAFF_COOKIE_ENABLED`; middleware accepts cookie or Bearer. `POST /staff/logout` clears it.
- `staff.login.success` / `staff.login.failed` are audited on every attempt.

### Platform

- Separate `platform_users` table, separate `/platform/auth`, separate middleware.
- **TOTP MFA** (RFC 6238, dependency-free): AES-GCM-encrypted secret, bcrypt-hashed recovery codes, ephemeral 5-minute challenge. Requires `MFA_ENCRYPTION_KEY`; unset means MFA fails closed. An unconfigured key returns 503, not 500.
- **Lockout:** 5 failures / 5 min per email → 30-minute lock.
- Roles: `super_admin`, `support_admin`, `billing_admin`, `read_only_auditor`.
- The first admin is created by the one-shot `bootstrap-admin` command, which refuses to overwrite an existing user, grants only `super_admin`, and marks MFA required.

## 4. Authorization

Per-handler role and ownership checks are the enforcing mechanism today. A central policy engine (`internal/authz/policy.go`) runs **in shadow** — it evaluates and records what it *would* decide, emitting mismatch metrics, without enforcing. Wave R3 flips it strict, gated on 48 hours of zero mismatches.

Organization governance — entitlements, feature flags, lifecycle, billing — is likewise **resolve-only**. It is audited but enforces nothing, and billing charges nobody.

## 5. Rate limiting

Per-route-group budgets keyed by IP, session, participant or provider as appropriate. Notable limits: staff auth 10 RPM, general API 60 RPM, with tighter per-session caps on order placement, payment initiation, assistance requests and WS ticket issuance.

The distinction that matters:

- **`RateLimitSensitive` fails CLOSED** when Redis is unreachable — staff auth, platform auth, payment initiation, webhook receipt, WS ticket issuance. A datastore outage must not become a brute-force window.
- Other routes **fail open** with a metric, so a Redis blip degrades rather than denies service.

The limiter is a fixed-window counter. At a window boundary a burst of up to 2× the limit is theoretically possible — acceptable here, but do not treat the limit as a hard ceiling.

## 6. Transport and browser hardening

The backend serves JSON only and never renders HTML, so its CSP is API-shaped:

```
default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'
```

plus `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, a restrictive `Permissions-Policy`, `Cross-Origin-Opener-Policy: same-origin` and `Cross-Origin-Resource-Policy: same-site`. HSTS is gated on `ENABLE_HSTS` — enable only where TLS terminates in front of the API, since in development the API is plain HTTP and HSTS would pin the browser to HTTPS for two years.

The frontend is a separate origin and carries its own CSP from `next.config.ts`. `NEXT_PUBLIC_API_BASE` must exactly equal the API origin or that CSP blocks API and WebSocket calls.

**CORS:** `CORS_ALLOWED_ORIGINS` is an explicit allowlist. Release mode refuses to start when it is empty — an empty list also disables the WebSocket origin allowlist, so the two failure modes are deliberately coupled.

**Body size** is capped globally at 1 MB via `http.MaxBytesReader`.

**Trusted proxies:** `SetTrustedProxies` failure is fatal. The server will not start with a misconfigured proxy list, because a wrong list silently makes every client IP the proxy's — which would break per-IP rate limiting invisibly.

## 7. WebSocket

- Upgrade requires a valid one-shot ticket (`WS_TICKET_AUTH_REQUIRED`, on for launch). Ticket issuance is itself rate-limited and fails closed.
- Origin is validated against `CORS_ALLOWED_ORIGINS` in release mode.
- Slow consumers are evicted rather than allowed to stall a room.
- Client PINGs double as the presence heartbeat; there is no separate presence endpoint to abuse.

## 8. Data protection

- **`audit_log` is append-only**, enforced by a database trigger. An `UPDATE` attempt must fail — that is a post-restore verification step.
- **Audit redaction** covers `signature`, `mfa`, `2fa`, `refresh_token`, `access_token`, `private_key`, `api_key`, `pan`, `csrf_token`, `otp`, `recovery_code`, and guest session tokens.
- **SQL injection**: every query is sqlc-generated and parameterised. There is no raw string interpolation into SQL.
- **Support reads are audited**, including a tenant-visible row for deep reads. Support console access is read-only by construction.

## 9. Secrets

- All secrets come from the environment. Nothing is in git — verified: no bucket name, domain or credential literal exists in any code path.
- Release mode **ignores dotenv files**, so a stray `.env` cannot override an injected production value.
- Release mode fails hard on a weak/missing `GUEST_TOKEN_SECRET`, empty `CORS_ALLOWED_ORIGINS`, or a malformed MFA/webhook secret.
- Production secrets live in the password manager. `deploy/vm/.env.production.example` marks every generated value `__GENERATE__` (`openssl rand -base64 48`).
- Backup credentials are a **separate scoped R2 token** from the upload credentials, and the two use different variable names on purpose.

> **Beta caveat.** The beta environment uses one shared R2 credential pair and publishes tester credentials for its seeded demo tenants. That is intentional for a throwaway environment and must not carry into production.

## 10. Vulnerability scanning

- **`govulncheck`** gates every PR (`golang.org/x/vuln/cmd/govulncheck@v1.6.0`, source mode). It fails only when a vulnerable symbol is reachable from this module's call graph, so a required-but-uncalled vulnerable module does not block the build.
- The scanner runs on the same `"1.26"` Go spec the release Dockerfile uses, so the scan matches the shipped artifact rather than the runner.
- **Dependabot** is configured (`.github/dependabot.yml`); every PR is reviewed and merged manually, with no auto-merge. Policy: [dependency-upgrades.md](dependency-upgrades.md).

> **Open:** the exact binary currently deployed to beta was built on Go 1.26.5 and an exact scan of it reports **nine reachable advisories**. That is a live release blocker tracked in [RELEASE.md §5](RELEASE.md#5-open-before-v10) — it is not covered by the PR gate, because the PR gate scans the source tree, not the deployed binary.

## 11. Security testing

- Cross-tenant denial paths are covered by integration tests (`-tags integration`), which run in CI.
- Adversarial e2e specs live in `e2e/adversarial/` and `e2e/tenancy/` — but Playwright is **not executed in CI** ([TESTING.md §5](TESTING.md#5-what-ci-actually-runs)).
- Chaos experiments exercise Redis loss, backend restart, nginx reload and webhook replay; manual only.
- The most recent independent audit is archived at [../release-certification/archive/2026-08-04/independent-audit-2026-08-04.md](../release-certification/archive/2026-08-04/independent-audit-2026-08-04.md).
