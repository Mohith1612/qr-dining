# Strict Rollout Plan — Final

Date: 2026-05-25
Supersedes the sequencing intent of `production-enforcement-rollout.md` by
ratifying its wave order and layering Phase-D validation on top. Read that document
for the full per-wave checklists, blast radius, and rollback MTTR matrix — they
remain authoritative. This document states the **final order**, the **Phase-D
gating additions**, and the **blockers** that must clear before each wave.

## Principle

Flip **one wave at a time**, environment-variable change only, separate deploy per
flip, two-person rule. Never pair a flag flip with a schema change. Two pairings
flip *together inside one window* (splitting them creates an allow/deny race):

- `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS`
- `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED`

## Final staged order

| Wave | Flag(s) | Risk | Prod soak |
|------|---------|------|-----------|
| R1 | `AUDIT_LOG_V2_ENABLED` | lowest | 72h |
| R2 | `TENANCY_ORGANIZATIONS_ENABLED` | low | 72h |
| R3 | `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS` | high | 7d |
| R4 | `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED` | high | 7d |
| R5 | `WS_TICKET_AUTH_REQUIRED` | medium | 7d |
| R6 | `AUTH_GUEST_CREDENTIALS_REQUIRED` | medium | 14d |
| R7 | `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | highest operational | 14d |

Rationale for keeping this order: it matches the dependency DAG — audit visibility
first (every later flip is confirmed via V2 audit), then org scope (authz depends on
it), then the authz/scoped pair, then staff identity, then realtime ticket auth
(needs guest-credential-capable clients), then guest credentials, and finally
payment settlement (depends on staff training + waiter UI). **Do not enable
everything together.**

## Phase-D gating additions (new prerequisites layered on each wave)

These are *in addition to* the per-wave checks already in
`production-enforcement-rollout.md`.

- **All waves:** the alert in `production-alerting-baseline.md` that gates the wave
  must exist, be wired, and read its safe value for the stated soak. Thresholds set
  during the staging soak, not after the prod flip.

- **R2 — blocker:** `tenant_resolution_failures_total` does not exist yet. Add it to
  the tenant middleware; R2 cannot start until it is at zero for the soak.

- **R3 — blocker:** the shadow-decision counter `policy_shadow_mismatch_total{route,reason}`
  does not exist. R3's pre-flip check ("mismatch rate zero for 48 consecutive hours")
  is unenforceable without it. Implement before R3. Also resolve the three "Open
  Decisions Required Before R3" (§11 of the rollout doc) in writing.

- **R5 — additions:**
  - `ws_ticket_consume_failed_total` does not exist; add it at the ticket-consume
    failure path before R5.
  - **Reconnect-burst sizing (from chaos §4):** the per-session ws-ticket cap (12/min)
    and per-IP cap (60/min) were validated to throttle a single source correctly. But
    a mass reconnect of many distinct devices behind one Cloudflare/NAT egress IP can
    brush the 60/min per-IP cap. Before R5, size the per-IP cap against measured egress
    concentration and watch 429 + `rate_limiter_unavailable_total{surface="ws_ticket"}`
    in the staging soak.
  - The **WS inbound abuse hardening** shipped this phase (per-connection rate limit +
    strike budget, validated live) closes the inbound-flood gap that R5 implicitly
    relied on. New metrics `ws_inbound_dropped_total` / `ws_abusive_closes_total` are
    on the realtime dashboard.

- **R6 — addition:** `guest.token.validation_failed{reason}` does not exist; add it
  so the R6 checkpoint ("reasons should be expired/signature, not malformed") is
  observable.

- **R7 — addition:** webhook idempotency dedup (W-02) was **not** fully exercised in
  chaos (it sits behind provider-schema parsing). Add an integration test with a
  recorded provider payload and a settleable payment, and confirm
  `idempotency_replays_total{entity="webhook"}` increments on exact replay, before R7.

## Validated by Phase D (reduces risk for these waves)

- **Deploy safety (all waves):** backend restart is safe and migrations are
  idempotent (chaos §2) — rolling restarts to apply each env flip are low-risk.
- **Redis resilience:** Redis is non-authoritative; a flap does not corrupt state
  (chaos §1). Flipping flags that lean on Redis (staff session cache, ws tickets,
  rate limits) will not lose authority on a Redis blip.
- **Edge:** the new nginx config handles websockets and reloads without dropping
  service (chaos §5) — safe to reload edge config alongside a flip.

## Rollback

Unchanged from `production-enforcement-rollout.md` §5: every flag rolls back by
setting it `false` + rolling restart, MTTR < 15 min, no data corruption. Pending
payments under R7 remain pending until settled by either path. Quarterly rollback
drill under load remains required.

## Definition of "enforcement ready"

Unchanged: all of §12 of the rollout doc must hold simultaneously (R1–R7 exit gates
cleared, all `legacy_identity_usage_total` zero for 30 days, signed security review
closing every Critical/High from `operational-correctness-audit.md`, rollback drill
within 90 days). Phase D adds: the six missing observability signals in
`production-alerting-baseline.md` §7 must be implemented and populated first.
