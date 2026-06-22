# Security Hardening Checklist

Date: 2026-05-22
Status: Authoritative pre-production security checklist. Every item must be either DONE with evidence in the repo, or have an owner and a date before the production gate opens.

## 0. Threat Model Summary

Adversaries assumed:

- Unauthenticated network actors hitting public routes (webhooks, QR resolve, snapshot reads).
- Compromised or shared staff devices on branch Wi-Fi.
- A guest who joins a session, captures local identifiers, and tries to act as someone else.
- A compromised browser extension or XSS injection on a staff or guest device.
- Replay of any HTTP request, including signed webhooks, after they have been observed.
- A long-lived connection (WS or HTTP) that survives the user's intent.
- A misbehaving branch trying to read another branch's data.
- An organization admin trying to escalate to platform admin.

## 1. Authentication Surface

### Staff Auth

- [x] `branch_code + staff_code + PIN` flow implemented (`AuthenticateWithCode`).
- [x] Inactive staff cannot log in.
- [x] Durable `staff_sessions` table stores token hash, version, expiry.
- [x] Token validation re-checks `is_active`, `pin_version`, `token_version` against DB.
- [ ] Rate limit per `(branch_id, staff_code)` and per IP. **OWNER + DATE REQUIRED**. Target: 10 failures per 5 minutes per `(branch_id, staff_code)`; 50 per 5 minutes per IP. Lockout 15 minutes.
- [ ] CAPTCHA escalation after lockout. Optional — record decision.
- [ ] PIN rotation triggers immediate session invalidation across devices (already via `pin_version` bump).
- [x] Audit `staff.login.success` and `staff.login.failed` written on every attempt.

### Guest Auth

- [x] Guest token is HMAC-SHA256 signed, carries session_id/branch_id/table_id/org_id/participant_id/credential_version/jti.
- [x] Server derives participant from token, ignores client-supplied participant ids when flag strict.
- [ ] Token TTL set explicitly. Recommended: access token 60 minutes; renewable via snapshot read if session still active. **CONFIRM CURRENT VALUE in `auth/guest.go`.**
- [ ] Token revocation: `session_participants.revoked_at` is checked on every validation. **CONFIRM IMPLEMENTATION.**
- [x] Token signature secret loaded from `GUEST_TOKEN_SECRET` env; production secret is not the dev default (warned in config validation).
- [ ] JWT-style `jti` denylist for revoked tokens within active TTL. **OWNER + DATE REQUIRED if not present.**

### Platform Admin Auth

- [x] Separate `platform_users` table, separate `/platform/auth`, separate middleware.
- [ ] MFA enforced for `super_admin` and `support_admin`. Currently `mfa_required` is a column; enforcement path must be wired. **OWNER + DATE REQUIRED.**
- [ ] Short access token TTL (15 min); refresh requires re-auth (no silent refresh for platform). **CONFIRM.**
- [x] Platform tokens are rejected by staff and organization middleware.
- [ ] Per-user IP allowlist for platform routes optional but recommended.

### Organization User Auth

- [ ] Separate from staff PIN flow. **CONFIRM IMPLEMENTATION STATUS.** If currently bridged through staff with `role=owner`, this is a phase-9 gap to close before strict.
- [ ] MFA for `owner` and `admin` roles.

## 2. Rate Limiting Coverage

| Surface | Limit | Window | Identifier | Implementation |
| --- | --- | --- | --- | --- |
| Staff auth | 10 fails / 50 attempts | 5 min | `(branch_id, staff_code)` and IP | **TODO** |
| Guest session create | 5 sessions | 5 min | client IP + table token | **TODO** |
| Guest session join | 10 / 5 min | 5 min | session_id + IP | **TODO** |
| Snapshot read | 60 / min | 1 min | session_id + IP | **CONFIRM** |
| Cart mutation | 60 / min | 1 min | participant_id | **CONFIRM** |
| Order placement | 12 / min | 1 min | participant_id | **TODO** |
| Payment initiation | 6 / min | 1 min | session_id | **TODO** |
| Assistance request | 6 / 15 min | 15 min | participant_id | **TODO** |
| Promo validate | 30 / min | 1 min | session_id | **TODO** |
| WS ticket issuance | 12 / min | 1 min | session_id | **TODO — critical for reconnect storm** |
| Webhook receive | 100 / min | 1 min | provider | **CONFIRM** |
| Platform auth | 5 / 5 min | 5 min | username + IP | **TODO** |
| Org governance reads | 60 / min | 1 min | org_id + user | **TODO** |
| Audit log read | 30 / min | 1 min | actor | **TODO** |

Rate limiters MUST fail closed (deny) when the rate-limit backend (Redis) is unavailable for these endpoints: staff auth, platform auth, payment, webhook. Other endpoints may fail open with a metric.

## 3. Brute Force Protection

- Staff PIN lockout after N failures (Section 1).
- Platform admin lockout after 3 failures within 5 minutes, requires manual unlock.
- Audit `auth.lockout` event with actor, IP, time, and reason.
- Periodic report of locked staff/admin accounts surfaces to ops dashboard.

## 4. WebSocket Specific

- [x] WS upgrade requires either valid ticket or legacy query auth (legacy gated by flag).
- [ ] One-shot ticket TTL: 30s. **CONFIRM in code.**
- [ ] WS ticket consume rate limit per session (see Section 2).
- [ ] Per-connection inbound rate limit: 60 messages/minute. Excess closes connection.
- [ ] Per-connection outbound buffer cap (Section 6 of `realtime-reconciliation-invariants.md`).
- [ ] Origin check on WS upgrade matches `CORS_ALLOWED_ORIGINS` in production.
- [ ] No reflective echo of arbitrary client payloads through the hub.

## 5. Upload Validation

- [ ] R2 presign endpoint requires staff with `menu.item.update` or `branch.theme.update` policy.
- [ ] Presign carries explicit MIME type and max size. Backend enforces both via signed conditions.
- [ ] Upload key path: `orgs/{org_code}/branches/{branch_code}/menu-items/{item_id}/{asset_id}.{ext}` per Phase plan section 6.4.
- [ ] Image validation: server-side type sniff after upload confirm. Reject mismatches.
- [ ] Logo replacement is versioned, never overwrites a stable path.
- [ ] Asset metadata is recorded in DB before public URL is exposed.
- [ ] Periodic janitor removes orphaned uploads with no DB references after 24h.

## 6. R2 / Signed URL Strategy

- Presigned URLs are valid 15 minutes maximum.
- Presigned URLs are bound to: bucket, key, method (PUT), content-type, content-length range, IP optional.
- Public read URLs use stable, predictable paths (Section 5). No tokenized read URLs for menu images.
- Private assets (audit exports) use short-lived (5 minute) tokenized read URLs.

## 7. Cookies / Session Storage

- [ ] Move staff token off localStorage. Use `HttpOnly; Secure; SameSite=Lax` cookie scoped to `/staff` path. **OWNER + DATE REQUIRED.**
- [ ] Guest token may remain in `sessionStorage` (tab-scoped, cleared on close), but never `localStorage`. **CONFIRM.**
- [ ] No long-lived auth state in IndexedDB.
- [ ] Frontend explicitly clears auth state on logout, on `revoked_credential` 401, and on session terminal 410.

## 8. CSRF Posture

- Bearer-token auth (staff) and signed guest token are not browser-credential-bound; they are not vulnerable to classic CSRF unless they are stored in cookies.
- Once staff token moves to cookies (Section 7), every state-changing endpoint MUST require either:
  - a `X-Requested-With` header that browsers do not forge cross-origin, or
  - a CSRF token tied to the staff session that is read from a non-cookie source on each mutation.
- Webhook endpoints are public; CSRF does not apply. Signature verification is the protection (Section 11).

## 9. CSP Posture

Production HTTP responses MUST include:

```
Content-Security-Policy:
  default-src 'self';
  script-src 'self';
  style-src 'self' 'unsafe-inline';   // if necessary; aim to remove
  img-src 'self' data: https://<r2-public-domain>;
  connect-src 'self' wss://<api-host>;
  frame-ancestors 'none';
  base-uri 'self';
  form-action 'self';
  upgrade-insecure-requests;
```

- [ ] `script-src` has no `'unsafe-inline'` or `'unsafe-eval'`.
- [ ] Nonces used for any inline scripts the Next.js bundle requires.
- [ ] Report-only mode in staging for 48h before enforcement.
- [ ] CSP violation reports collected with `report-to` directive.

## 10. nginx Security Headers

Required on the production reverse proxy:

```
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
Cross-Origin-Opener-Policy: same-origin
Cross-Origin-Resource-Policy: same-site
```

- [ ] Verify all headers on a production HEAD request.
- [ ] Verify TLS configuration: TLS 1.2+ only, modern cipher suites, OCSP stapling.

## 11. Webhook Security

- [x] HMAC-SHA256 signature verification using raw body.
- [x] Timestamp tolerance enforced (`cfg.Payment.WebhookTimestampTolerance`, default 5 min).
- [x] Provider event id unique constraint prevents replay.
- [ ] Secrets rotated quarterly via env var rotation (no downtime; support multiple secrets concurrently via the existing slice).
- [ ] Alert on 5+ signature failures from one provider in 5 minutes.
- [ ] No webhook handler reads any state from the request payload that is also derivable server-side without verification.

## 12. Replay Windows

| Surface | Replay window | Mechanism |
| --- | --- | --- |
| Webhook | 5 minutes | timestamp tolerance |
| WS ticket | 30 seconds | one-shot Redis consume |
| Guest token `jti` | within TTL | jti recorded once per request, denylist on revoke |
| Idempotency keys | matches resource window (24h after terminal) | `idempotency_keys` table |

## 13. Token Expiration

| Token | TTL | Refresh |
| --- | --- | --- |
| Staff access | 8 hours (current) | none, re-login |
| Staff refresh (future) | 30 days | optional Phase 10 |
| Guest access | 60 minutes (recommended; **confirm**) | via snapshot read while session active |
| Platform admin | 15 minutes | re-auth |
| Platform support session | duration of break-glass approval, max 4 hours | re-approve |
| WS ticket | 30 seconds | one-shot |

## 14. Support Session Expiration

- [x] `platform_support_sessions.expires_at` recorded.
- [ ] Read-only by default; write capabilities require elevated role plus reason.
- [ ] Tenant-visible audit entries (`platform.support.access`) on every read or write performed under a support session.
- [ ] Auto-expire on `expires_at`; do not auto-renew.
- [ ] Soft cap: 4 hours per session. Hard cap: 24 hours.

## 15. Audit Abuse Protection

- [x] `audit_log` is append-only via the immutable trigger.
- [ ] Application user (DB role) has INSERT/SELECT but not UPDATE/DELETE on `audit_log`. **CONFIRM grants.**
- [ ] Audit read endpoints rate-limited and require explicit scope.
- [ ] No raw PII in audit `metadata_json` — redaction in `audit/redaction.go` covers PIN, token, payment_card, secret, password, signature. Verify recursion covers nested structures.
- [ ] Audit retention policy documented (default: 365 days hot, archive after).

## 16. Input Validation

- [ ] All numeric IDs are typed at the handler. No string-to-int silent conversions accepted.
- [ ] All UUIDs validated with `uuid.Parse` and reject malformed.
- [ ] All free-text fields (display_name, item names) have size caps and Unicode normalization.
- [ ] Phone numbers normalized to E.164 before storage or comparison.
- [ ] Email addresses normalized via `citext` columns.
- [ ] JSON body size limit at the gateway (1 MB default; uploads bypass via R2 presign).

## 17. Secrets Management

- [ ] No secret in repo. Confirm with `gitleaks` or equivalent.
- [ ] Secrets in environment only. Production loads from secret manager (cloud provider).
- [ ] Rotation procedure documented for each: `GUEST_TOKEN_SECRET`, `STAFF_TOKEN_SECRET` if used, payment webhook secrets, DB passwords, R2 keys.
- [ ] Rotation procedure tested annually.

## 18. CORS

- `CORS_ALLOWED_ORIGINS` must be explicit; never `*` in production.
- Production startup warns if empty (`config.go:217` already does); upgrade to fatal in `release` mode.
- WebSocket upgrade respects the same origin list.

## 19. CSP Reporting and Violation Handling

- Production runs CSP in enforce mode after report-only validation.
- Reports land in a low-volume queue and produce a weekly report.
- Frequent violations trigger investigation, not automatic policy relaxation.

## 20. Dependency Hygiene

- [ ] `go mod tidy` clean.
- [ ] `npm audit --production` shows no high/critical for frontend.
- [ ] Automated dependency PRs every Monday; security-only PRs land within 7 days.
- [ ] Pinned base images for backend Docker; no `latest` tags in production manifests.

## 21. Build & Deploy

- [ ] Production builds reproducible — same source SHA produces same artifact hash.
- [ ] No debug binaries in production.
- [ ] No CGO unless required.
- [ ] Stripped binaries, no source paths in stack traces beyond internal modules.
- [ ] Health check endpoint does not leak version or commit info to public surface.

## 22. Logging Hygiene

- Structured JSON logs with `request_id`, actor type/id, `organization_id`, `branch_id`, `session_id` where present.
- No raw PINs, tokens, full card numbers, or webhook signatures in logs.
- `audit/redaction.go` patterns also applied to request body logging if any. Default: no request body logging in production.
- Log retention 90 days hot, 365 days archive.

## 23. Penetration Test Scope

Recommended pre-launch external penetration test covers:

- Staff auth path including rate limit / lockout.
- Guest credential flow including replay and revocation.
- Webhook signature forgery attempts.
- Cross-branch resource access attempts.
- WS ticket consume race conditions.
- Platform admin separation.
- Upload bucket policy.

Out of scope for first pen test: DDoS, social engineering of staff.

## 24. Final Sign-Off Items

Before flipping `AUTH_GUEST_CREDENTIALS_REQUIRED` and `PAYMENT_STAFF_SETTLEMENT_REQUIRED`:

- All boxes above ticked, or `TODO` items have explicit owner and date.
- External pen test report received and Critical/High findings closed.
- DR drill executed (one staff session, one guest session, one in-flight payment survive a controlled cold restart).
- Rollback drill executed for each feature flag.
- Logging and alert dashboards reviewed by both engineering and operations.
