# Production Enforcement Rollout

Date: 2026-05-22
Status: Pre-rollout. All strict-enforcement flags default `false` in `backend/internal/config/config.go:172-180`. This document defines the staged sequence, dependencies, observability checkpoints, and rollback procedure for moving the platform from compatibility/dual-mode to strict enforcement.

## 1. Scope

In scope:

- `AUTH_GUEST_CREDENTIALS_REQUIRED`
- `AUTH_STAFF_CODE_REQUIRED`
- `AUTH_STAFF_SESSION_DB_REQUIRED`
- `AUTHZ_CENTRAL_POLICY_ENFORCE`
- `TENANCY_ORGANIZATIONS_ENABLED`
- `AUDIT_LOG_V2_ENABLED`
- `WS_TICKET_AUTH_REQUIRED`
- `PAYMENT_STAFF_SETTLEMENT_REQUIRED`
- `STRICT_BRANCH_SCOPED_MUTATIONS`

Out of scope: legacy column drops, route removals, frontend re-skin. Those follow only after a flag has been strict for the stated soak period.

## 2. Operating Mode Before Rollout

- Backend serves both the new and the legacy code paths.
- Every legacy entry point is metered via `legacy_identity_usage_total{mechanism,endpoint_class}` (`backend/internal/observability/metrics.go`).
- Audit log V2 writer is wired but only emits when `AUDIT_LOG_V2_ENABLED=true`.
- Strict SQL scopes already exist (`WHERE id = $1 AND branch_id = $2`) on the converted handlers; `STRICT_BRANCH_SCOPED_MUTATIONS` only controls whether unconverted/legacy handler shapes still accept client-supplied scope.

Pre-rollout audit baseline must capture, per environment:

- 24h count of each `legacy_identity_usage_total` mechanism.
- 24h count of `authz.denied` events bucketed by `reason`.
- 24h count of `webhook.signature.failed`.
- 24h count of session reconciliation worker corrective actions.
- 7d distribution of staff session age.

If any of these are unknown the rollout has not started.

## 3. Flag Dependency Map

```
AUDIT_LOG_V2_ENABLED
  ^
  | (informational dependency: every later flip relies on V2 audit to confirm behavior)
  |
TENANCY_ORGANIZATIONS_ENABLED
  ^
  | resolves organization_id in tenant middleware, required by authz scope
  |
AUTHZ_CENTRAL_POLICY_ENFORCE  <-->  STRICT_BRANCH_SCOPED_MUTATIONS
  (paired: central policy denials and scoped-SQL denials must match)
  ^
  |
AUTH_STAFF_SESSION_DB_REQUIRED
  ^
  | DB-backed staff session must be the source of truth before code-only auth is rejected
  |
AUTH_STAFF_CODE_REQUIRED
  ^
  | branch_code + staff_code must be issued for every active staff
  |
WS_TICKET_AUTH_REQUIRED
  ^
  | depends on AUTH_GUEST_CREDENTIALS_REQUIRED-capable clients (ticket issuance requires guest cred)
  |
AUTH_GUEST_CREDENTIALS_REQUIRED
  ^
  |
PAYMENT_STAFF_SETTLEMENT_REQUIRED
  (last; depends on staff workflow training and waiter UI completion)
```

Two pairings must be flipped together inside the same maintenance window:

- `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS`
- `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED`

Splitting either pair leaves a window where one layer says "deny" and the other says "allow", producing inconsistent behavior under concurrent traffic.

## 4. Rollout Sequence

Each wave runs in `staging` for at least the soak period, then in `production` after the gate passes. No wave starts until the previous wave has cleared its soak.

### Wave R1: AUDIT_LOG_V2_ENABLED

- Risk: lowest. Pure write path. Read path is unaffected.
- Soak: 48 hours staging, 72 hours production.
- Pre-flip checks:
  - `audit_log` immutable trigger present (`trg_audit_log_immutable`).
  - `AuditWriteFailuresTotal` is documented in dashboards.
  - Disk headroom: at least 60 days of expected audit volume.
- Flip: set `AUDIT_LOG_V2_ENABLED=true`. Restart backend rolling.
- Observability checkpoints (every 15 min for first 2 hours, then hourly):
  - `audit_log` row count growth in line with API request volume.
  - `AuditWriteFailuresTotal` rate near zero.
  - p99 request latency for writes that audit (POST /staff/auth, POST /orders, POST /payments) within +10ms of baseline.
- Rollback: set `AUDIT_LOG_V2_ENABLED=false` and restart. No data corruption risk. Existing audit rows stay; readers gracefully degrade.
- Exit gate: 72h production with zero immutable trigger violations and no latency regression.

### Wave R2: TENANCY_ORGANIZATIONS_ENABLED

- Risk: low. Adds `organization_id` to actor scope; does not yet deny any operation.
- Soak: 48 hours staging, 72 hours production.
- Pre-flip checks:
  - All `restaurants.organization_id` and `branches.organization_id` are NOT NULL in production (migration `000017` enforces; verify post-deploy).
  - `organization_members` has at least one `owner` per active organization.
  - `/orgs/:org_id/...` routes return 200 for owner test accounts.
- Flip: set `TENANCY_ORGANIZATIONS_ENABLED=true`.
- Observability checkpoints:
  - `authz.denied{reason=missing_org_scope}` rate at zero.
  - Per-route `tenant_resolution_failures_total` near zero.
- Rollback: set false. No state change required.
- Exit gate: 72h with zero `missing_org_scope` denials on routes that should resolve org context.

### Wave R3: AUTHZ_CENTRAL_POLICY_ENFORCE + STRICT_BRANCH_SCOPED_MUTATIONS (paired)

- Risk: high. Real denials start. Misconfigured routes will surface.
- Soak: 72 hours staging, 7 days production.
- Pre-flip checks:
  - Run paired shadow report for 7 days: every request runs both legacy decision and policy decision; mismatches are logged with `policy_shadow_mismatch_total{route,reason}`. Mismatch rate must be zero for 48 consecutive hours.
  - `repository/order.go`, `repository/menu.go`, `repository/staff.go`, `repository/promo.go` all expose scoped variants and are called from every mutation handler.
  - `handlers/authz.go:37-64 requireAuthorized` audits denials.
- Flip: set both true together. Coordinated rolling restart.
- Observability checkpoints:
  - `authz.denied{reason=branch_mismatch}` from staff agents stays under a rate baseline that has been validated to represent actual misuse (set the threshold from shadow data, not from intuition).
  - 4xx rate on staff handlers within +0.5% of baseline.
- Rollback: set both false. Existing denials do not reverse; staff retry succeeds.
- Exit gate: 7d production, mismatch rate zero, no staff support tickets attributing failures to authz.

### Wave R4: AUTH_STAFF_CODE_REQUIRED + AUTH_STAFF_SESSION_DB_REQUIRED (paired)

- Risk: high. Every active staff member must have a `staff_code` and an issued PIN under the new shape.
- Soak: 72 hours staging, 7 days production after staff rollout.
- Pre-flip checks (do not skip any):
  - `SELECT count(*) FROM staff WHERE is_active = true AND (staff_code IS NULL OR staff_code = '')` returns 0 in production.
  - `branches.branch_code` populated and unique within organization.
  - Staff training: every active staff has logged in via new flow at least once during shadow week, captured by `staff.login{auth_method='code'}` audit events.
  - `legacy_identity_usage_total{mechanism='staff_branch_pin'}` per branch decays to zero for 7 consecutive days.
- Flip: both true together.
- Observability checkpoints:
  - `staff.login.failed{reason='legacy_payload_rejected'}` triages to specific branches; field response procedure on standby.
  - Staff session creation succeeding, sessions visible in `staff_sessions` table.
- Rollback: set both false. Old PIN-only flow re-enabled. Existing `staff_sessions` rows remain valid.
- Exit gate: 7d with zero `legacy_payload_rejected` on production.

### Wave R5: WS_TICKET_AUTH_REQUIRED

- Risk: medium. Requires every WS client to issue a ticket via `POST /sessions/:id/ws-ticket`.
- Soak: 48 hours staging, 7 days production.
- Pre-flip checks:
  - `legacy_identity_usage_total{mechanism='ws_query_participant_id'}` is zero for 7 consecutive days.
  - Ticket consume failures (`ws_ticket_consume_failed_total`) trend below baseline.
  - Frontend reconnect logic uses ticket exclusively (verify via deployed bundle version inventory; reject WS connect with missing `?ticket` param triggers user-visible reconnect with new ticket fetch).
- Flip: set true.
- Observability checkpoints:
  - WS connection error rate within +0.2% of baseline.
  - Reconnect storm protection: per-minute ticket issuance rate per session capped (see `security-hardening-checklist.md`).
- Rollback: set false. Legacy query auth re-enabled. Connected sessions unaffected since current sockets remain open.
- Exit gate: 7d production with zero legacy WS auth observations.

### Wave R6: AUTH_GUEST_CREDENTIALS_REQUIRED

- Risk: medium. Guest token must be issued by every active client.
- Soak: 48 hours staging, 14 days production (longer because of QR re-share habits).
- Pre-flip checks:
  - `legacy_identity_usage_total{mechanism in (x_participant_id, body_participant_id, body_placed_by_participant_id)}` zero for 14 consecutive days.
  - Active sessions older than the guest token TTL have all been re-issued during normal use; spot check 100 active sessions for token freshness.
  - `auth/guest.go` token rotation under host transfer tested.
- Flip: set true.
- Observability checkpoints:
  - `guest.token.validation_failed{reason}` distribution stable; reasons should be expired/signature, not malformed.
  - Order/cart/assist 4xx rate within +0.3% of baseline.
- Rollback: set false. Existing tokens still valid; legacy participant id paths re-enabled.
- Exit gate: 14d production zero legacy.

### Wave R7: PAYMENT_STAFF_SETTLEMENT_REQUIRED

- Risk: highest operational. Cash/card/UPI cannot complete without staff settlement action.
- Soak: 7 days staging, 14 days production after staff training.
- Pre-flip checks:
  - Staff settlement UI deployed to every branch device.
  - Average time-to-settle measured below 90 seconds on staging.
  - `payments.status=requires_staff_confirmation` rows being cleared on staging within SLA.
  - Out-of-hours settlement workflow defined (who closes a session if night manager forgets).
- Flip: set true.
- Observability checkpoints:
  - `payment.requires_staff_confirmation_age_seconds` p95 monitored, alert if > 5 minutes.
  - Session timeout abandonments on tables with pending payment alerted distinctly.
- Rollback: set false. Cash flows resume implicit completion. Outstanding pending payments unaffected.
- Exit gate: 14d with no support escalation tied to settlement workflow.

## 5. Rollback Matrix

| Flag | Rollback action | Data implications | Estimated MTTR |
| --- | --- | --- | --- |
| `AUDIT_LOG_V2_ENABLED` | Set false, rolling restart | None | <5 min |
| `TENANCY_ORGANIZATIONS_ENABLED` | Set false, rolling restart | None | <5 min |
| `AUTHZ_CENTRAL_POLICY_ENFORCE` | Set false, rolling restart | None; in-flight denials stand | <5 min |
| `STRICT_BRANCH_SCOPED_MUTATIONS` | Same as above; flip with authz | None | <5 min |
| `AUTH_STAFF_CODE_REQUIRED` | Set false, rolling restart | None; existing `staff_sessions` reusable | <10 min |
| `AUTH_STAFF_SESSION_DB_REQUIRED` | Same as above | None; Redis cache still serves | <10 min |
| `WS_TICKET_AUTH_REQUIRED` | Set false, rolling restart | None; new WS connects use legacy query | <5 min |
| `AUTH_GUEST_CREDENTIALS_REQUIRED` | Set false, rolling restart | None; existing tokens still accepted, legacy ids re-accepted | <10 min |
| `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | Set false, rolling restart | Pending payments remain pending until settled by either path | <15 min |

Rollback drill: a quarterly exercise must confirm each rollback under load and that no `_failed_to_rollback` alert fires.

## 6. Per-Flag Blast Radius

- `AUDIT_LOG_V2_ENABLED`: write amplification (one extra INSERT per audited action). Storage growth. No request behavior change.
- `TENANCY_ORGANIZATIONS_ENABLED`: tenant middleware does an extra lookup. Adds `organization_id` to logs and audit. No request behavior change.
- `AUTHZ_CENTRAL_POLICY_ENFORCE`: any handler missing a `requireAuthorized` call will degrade to default-allow (if shadow agreed) or default-deny; review each handler. Misconfigured `actor_scope_resolver` produces 403.
- `STRICT_BRANCH_SCOPED_MUTATIONS`: any handler still using the unscoped mutation helper rejects valid traffic. Mitigated by shadow week.
- `AUTH_STAFF_CODE_REQUIRED`: every staff device missing a `staff_code` cannot log in. Single biggest support risk on flip day.
- `AUTH_STAFF_SESSION_DB_REQUIRED`: existing Redis-only sessions invalidated. Stranded staff must re-login.
- `WS_TICKET_AUTH_REQUIRED`: stale frontend bundles cannot connect WebSocket. Falls back to polling/refresh.
- `AUTH_GUEST_CREDENTIALS_REQUIRED`: guests who scanned a QR before the flip and never refreshed cannot mutate (snapshot/read may still work). Mitigated by token TTL design plus the 14-day soak.
- `PAYMENT_STAFF_SETTLEMENT_REQUIRED`: any payment method needing staff cannot self-complete. Maximum disruption to live service if staff unprepared.

## 7. Observability Plan

Per flag, the following dashboards must exist before flip:

1. **Flip-impact dashboard**: 4xx rate, 5xx rate, p99 latency, request volume, and audit volume on the routes the flag affects, with overlay markers for flip time.
2. **Legacy decay dashboard**: `legacy_identity_usage_total` by mechanism, by branch, by hour. Must hit zero before flip.
3. **Denial reason dashboard**: `authz.denied` and `auth.failed` by reason, by branch.
4. **Staff workflow dashboard** (R4 and R7): login success/failure, settlement latency, settlement abandonment.
5. **Realtime dashboard** (R5): WS connect attempts, ticket consume failures, reconnect rate per session.
6. **Payment dashboard** (R7): pending-payment age histogram, settlement throughput, webhook signature failures.

Alert thresholds must be set during the staging soak, not after production flip.

## 8. Soak Period Rules

- Staging soak counts only if staging traffic mirror is at minimum 25% of production volume.
- Production soak counts only after the flip is on 100% of pods.
- Pause and extend the soak by 24h on any of: alert firing tied to the flag, support escalation tied to the flag, anomaly in flip-impact dashboard.
- If the soak is paused twice for the same flag, re-do the shadow week.

## 9. Legacy Path Removal

Legacy code paths must remain in the repo for at least 30 days after the corresponding flag has been strict in production. Removal requires:

- Zero legacy usage metric for 30 consecutive days.
- A separate PR per legacy mechanism, named `chore(legacy-removal): drop <mechanism>`.
- The PR removes both code and the associated metric counter, but keeps the audit event constants until the next major version.

Order of removal mirrors the rollout order (oldest flip removed first).

## 10. Cross-Cutting Requirements

- Every flag flip is a separate deploy with a separate change ticket.
- Two-person rule: at least one engineer and one operations contact must acknowledge the flip start.
- The flip is automated through environment variable change only — no code edit in the same window.
- Migration changes must be deployed in a previous release; never pair a flag flip with a schema change.
- All flips happen during the lowest-traffic local window for the predominant timezone of the affected branches.

## 11. Open Decisions Required Before R3

These must be answered in writing in this repo before the R3 paired flip:

- Are organization owners who do not also have a `staff` record allowed to mutate branch operational resources, or only to read/configure?
- Is `assistance.create.self` allowed for revoked-but-not-yet-expired guest credentials, or must it 401?
- Does `audit.read.organization` aggregate platform support session activity into the org timeline, or is that platform-only?

If unanswered, R3 must not flip.

## 12. Definition of "Enforcement Ready"

All of the following are true at the same time:

- Waves R1 through R7 have cleared their exit gates in production.
- All `legacy_identity_usage_total` counters are zero for 30 consecutive days.
- All `authz.denied` counters trend at expected operational baselines (no spikes attributable to flips).
- A signed security review has compared the original 30-finding audit (`operational-correctness-audit.md`) to the post-rollout state and marks each Critical/High as Closed.
- Rollback drill has been executed in production within the last 90 days for each flag.

Only when all twelve conditions hold may legacy code begin to be removed.
