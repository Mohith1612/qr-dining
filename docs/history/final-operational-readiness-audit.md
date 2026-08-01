# Phase D — Final Operational Readiness Audit

Date: 2026-05-25
Stance: adversarial operational review. The question is not "does the happy path
work" — it is "can this platform survive real-world production behavior under a
staged strict-flag rollout." Grounded in live chaos results
(`chaos-test-results.md`), the staging validation (`phase-d-staging-validation-report.md`),
the alert baseline (`production-alerting-baseline.md`), and the final rollout plan
(`strict-rollout-plan-final.md`).

## 1. What is solid (validated, not asserted)

- **Redis is non-authoritative.** A sustained Redis outage degrades the app
  (`/readyz` 503) but does not crash it or lose authority; it recovers cleanly.
- **Backend is stateless; deploys are safe.** Restart mid-operation re-applies
  migrations idempotently and recovers in ~12s. Rolling restarts for each flag flip
  are low-risk.
- **WebSocket inbound abuse is now bounded.** Per-connection token bucket + strike
  budget force-closes a flooding/malformed client (validated: 100 strikes → close)
  without harming the hub or peers. Reconnect bursts are capped per-session (12/min)
  and per-IP (60/min) at ticket issuance.
- **Edge handles websockets.** New nginx config upgrades WS correctly and survives a
  reload without dropping HTTP or WS.
- **Webhook forgery/replay-by-timestamp is rejected** (stale → 401, tampered → 401).
- **Lifecycle workers run** (reactivation pipeline observed firing live).

## 2. Cookie + CSRF final review

Current posture (from `internal/handlers/staff.go`, `middleware/staff_auth.go`,
`middleware/cors.go`, `middleware/security_headers.go`):

- **Transport:** Bearer token is primary. Staff *additionally* may use an HttpOnly
  session cookie, gated by `AUTH_STAFF_COOKIE_ENABLED`. Guest auth is Bearer
  (sessionStorage) only — no guest cookie. WebSocket auth is the one-time **ticket**
  (query param), not a cookie.
- **Cookie attributes:** `HttpOnly=true`, `SameSite=Lax`, `Secure` conditional, Path
  scoped, 8h TTL. Logout clears it (`MaxAge=-1`).
- **CSRF:** there is **no dedicated CSRF token**. Protection rests on (a) `SameSite=Lax`
  — cross-site POST/PATCH/DELETE do not send the cookie; (b) the CORS origin allowlist;
  (c) Bearer-primary design (a token in a header is not auto-sent cross-site).
- **Origin validation:** CORS allowlist reflects only configured origins; **dev/empty
  config allows all** (the WS origin warning seen at boot).

Assessment and required posture before enabling the staff cookie in production:

1. **`SameSite=Lax` + Bearer-primary is an adequate CSRF baseline** *provided no
   state-changing GET routes exist*. Lax sends the cookie on top-level GET
   navigation. Confirm (audit the route table) that every mutation is POST/PATCH/DELETE
   — they are in the current `server.go`. If any mutating GET is ever added, it would
   be CSRF-exposed; add a lint/test guard.
2. **`Secure` must be `true` in production** (TLS-terminated at the edge). Verify the
   flag that drives `secure` is set per-environment; never ship the cookie over plain
   HTTP.
3. **CORS must not be empty in production.** Empty origins = allow-all for both HTTP
   CORS and WS `CheckOrigin`. `CORS_ALLOWED_ORIGINS` must be the explicit PWA + staff
   origins; treat empty-in-release as a deploy blocker (it currently only warns).
4. **Consider `SameSite=Strict` for the staff cookie** (staff dashboard has no
   cross-site GET entry need) for defense-in-depth, or add a double-submit CSRF token
   if any future cookie-authenticated mutating GET is introduced.

Final production cookie posture: HttpOnly + Secure + SameSite=Lax (Strict acceptable),
explicit CORS allowlist enforced (not warned), Bearer remains primary, WS stays on
tickets. No CSRF token required at current route shape; revisit if a cookie-auth
mutating GET is ever added.

## 3. Remaining risks / blind spots

- **R-1 (medium): observability gaps block specific waves.** Five metrics the rollout
  playbook depends on do not exist (`policy_shadow_mismatch_total`,
  `tenant_resolution_failures_total`, `ws_ticket_consume_failed_total`,
  `guest.token.validation_failed`, `authz.denied` reason). Plus a `/readyz` blackbox
  probe is needed because `redis_pubsub_connected` does not reflect transient outages
  (F-1). These are hard prerequisites, not nice-to-haves. See
  `production-alerting-baseline.md` §7.
- **R-2 (medium): `payment_pending` has no timeout.** A session stuck in
  `payment_pending` (provider never calls back, staff never settles) is not auto-aged.
  Under R7 this becomes operationally important (a forgotten settlement strands a
  table). Add a bounded `payment_pending` age + alert
  (`payment.requires_staff_confirmation_age_seconds`, referenced by the rollout doc
  but not yet emitted) before R7.
- **R-3 (low): webhook idempotency dedup unproven end-to-end** (W-02). Dedup sits
  behind provider-schema parsing; cover with an integration test using a recorded
  provider payload before R7.
- **R-4 (low): reconnect-burst per-IP sizing under NAT/Cloudflare egress** — the
  60/min per-IP ws-ticket cap may be tight for mass post-deploy reconnects behind one
  egress IP. Size against real data before R5 (chaos §4).
- **R-5 (low): Redis durability gap during downtime** — events published while the
  pub/sub subscriber is impaired are not queued; clients recover via snapshot
  reconciliation on reconnect. Acceptable given Postgres authority + snapshot replay,
  but document it in the runbook so on-call expects "missed live events, recovered on
  reconnect," not data loss.
- **R-6 (cosmetic): `/readyz` JSON `status` field misleading during partial outage**
  (F-2). HTTP code is correct; align the body field.

No branch-isolation gaps, stale-authority restorations, or unsafe legacy paths were
discovered active in this phase beyond what the strict flags already gate (all default
`false`, dual-mode metered via `legacy_identity_usage_total`).

## 4. GO / NO-GO

**GO for staged rollout to begin — starting at Wave R1 (`AUDIT_LOG_V2_ENABLED`).**
The platform is operationally safe to *begin* the staged sequence: it survives the
real failure modes tested (Redis outage, restart, WS flood, reconnect storm, edge
reload, webhook forgery), deploys are reversible, and Redis is non-authoritative.

This is **NOT** a GO for production launch or for flipping everything. Conditional
gates per wave stand.

### Must be done before the corresponding wave (blockers)

- Before **R2:** implement `tenant_resolution_failures_total`.
- Before **R3:** implement `policy_shadow_mismatch_total` + run the 7-day shadow;
  answer the three §11 open decisions in writing.
- Before **R5:** implement `ws_ticket_consume_failed_total`; size the per-IP ws-ticket
  cap against egress concentration.
- Before **R6:** implement `guest.token.validation_failed{reason}`.
- Before **R7:** add `payment_pending` timeout + settlement-age metric/alert; add the
  webhook idempotency integration test.
- Before enabling the **staff cookie** in prod: enforce `Secure=true` and a non-empty
  CORS allowlist as deploy blockers (not warnings).
- Before any production paging on Redis: add a `/readyz` blackbox probe.

### Can wait until post-launch

- `/readyz` body `status` field cosmetic fix (F-2).
- `SameSite=Strict` hardening / CSRF token (only if a cookie-auth mutating GET is
  ever introduced).
- Legacy code-path removal (already 30-day-gated by the rollout doc).
- R2 backup/restore live drill, staging traffic-mirror tuning.

## 5. Bottom line

The architecture is sound and the failure behavior is correct where it matters most
(authority, recovery, abuse bounding, edge). The gap between "safe to start rollout"
and "enforcement ready" is now almost entirely **observability instrumentation** plus
the `payment_pending` timeout — concrete, scoped, and listed above. Close those per
wave and the staged rollout in `strict-rollout-plan-final.md` can proceed safely.
