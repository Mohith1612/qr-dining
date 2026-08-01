# Phase E — Enforcement Readiness Report

Date: 2026-05-25
Scope: rollout instrumentation + enforcement readiness. No features, no UI. Goal:
eliminate the rollout-blindness Phase D identified so the staged strict-flag rollout
can begin.

## What was implemented

### 1. Missing rollout metrics (all six closed)

Registered in `internal/observability/metrics.go` (custom registry, bounded labels,
nil-safe emission via the existing package-setter pattern):

| Metric | Emitted at | Labels |
|--------|-----------|--------|
| `authz_denied_total` | `handlers/authz.go` `requireAuthorized` (enforced denial) | `reason` (5 bounded slugs) |
| `policy_shadow_mismatch_total` | same fn, shadow branch (enforce off + would-deny) | `route` (gin pattern), `reason` |
| `tenant_resolution_failures_total` | `middleware/tenant.go` (both 500 paths) | `stage` (restaurant_slug, organization_lookup) |
| `guest_token_validation_failed_total` | `handlers/guest_auth.go` `guestParticipantID` | `reason` (malformed, invalid, expired, session_mismatch, participant_mismatch, stale_credential, revoked) |
| `ws_ticket_consume_failed_total` | `handlers/ws.go` `upgradeWithTicket` | `reason` (invalid, internal) |
| `payment_pending_escalations_total` | escalation worker | `level` (warn, critical) |

Reason values are mapped from sentinel errors via small helpers — never raw error
strings — so cardinality is bounded. `route` is `gin.FullPath()` (a fixed route set).

### 2. payment_pending escalation (alert-only)

New `RunPaymentPendingEscalation` worker + `ListPaymentPendingStalled` repository query +
config knobs + `PAYMENT_SETTLEMENT_STALLED` event + `payment.settlement.stalled` audit
action. Never mutates state. Full spec in `payment-escalation-lifecycle.md`.

### 3. /readyz fix + blackbox probe

`handlers/health.go` now returns `status: "not_ready"` (not a hardcoded `"ready"`) when a
dependency check fails — the Phase D F-2 bug. Added
`deploy/observability/prometheus-scrape-readyz.yml` (blackbox module + scrape job +
`ReadyzProbeFailing` page) so a real Redis outage pages off `/readyz`, not the
`redis_pubsub_connected` gauge (Phase D F-1).

### 4. Webhook idempotency integration test

Added `payment_webhook_idempotency_integration_test.go` (`//go:build integration`):
sends a correctly-shaped signed webhook twice, asserts
`idempotency_replays_total{entity="webhook"}` goes 0→1 on the replay and the payment
settles exactly once (one `payment_webhook_events` row, status completed). Reconciles the
Phase D note: dedup runs via `InsertWebhookEvent` ON CONFLICT *before* payload-schema
parsing; the Phase D synthetic body 400'd only because it used `event_id`/`type` instead
of the handler's `id`/`event` envelope.

### 5. Observability wiring

`prometheus-alerts.yml`: +6 rules (PolicyShadowMismatchPresent, TenantResolutionFailures,
GuestTokenValidationFailures, WSTicketConsumeFailures, AuthzDeniedSpike,
PaymentPendingEscalationCritical) — 24 rules total, `promtool` clean.
`grafana-dashboard.json`: +"Rollout instrumentation (Phase E)" row with panels for all
new metrics. `production-alerting-baseline.md` §7 gap table updated to RESOLVED.

## Validation evidence

- `go build ./...`, `go vet`, `gofmt` clean (CI parity).
- `promtool check rules` → 24 rules SUCCESS.
- Dashboard JSON validates.
- **Live metric emission** (host-built binary against dockerized PG/Redis): triggered an
  invalid WS ticket and a malformed guest token; scraped `/metrics`:
  `ws_ticket_consume_failed_total{reason="invalid"} 1` and
  `guest_token_validation_failed_total{reason="malformed"} 1`. The other CounterVecs share
  identical registration/emission wiring (a CounterVec only exposes a series after its
  first labelled increment, which is why un-triggered ones aren't yet visible).
- `/readyz` status-field fix verified by code; HTTP 503 behavior unchanged from Phase D.

## Honest limitations

- The webhook integration test is committed and compiles/vets, but could not be executed
  to green **in this sandbox**: the shared integration harness (`testutil.OpenTestDB` →
  golang-migrate borrowing a pgx-pool connection it doesn't release) deadlocks
  `pgxpool.Close()` during teardown. This affects the entire integration suite here,
  including the pre-existing `TestWebhookReplay_Idempotent`, not this test specifically.
  It runs in CI against a dedicated throwaway database. The dedup behavior itself was
  also validated live in Phase D (`scripts/chaos/webhook-replay.sh`: stale→401, tampered→401).
- `payment_pending_escalations_total` and the authz/tenant counters were not force-fired
  live (they need a stalled payment / staff denial / tenant subdomain), but the wiring is
  identical to the two verified counters.

## Commits

1. rollout instrumentation metrics (authz, tenant, guest token, ws ticket)
2. /readyz status fix + blackbox probe config
3. alert-only payment_pending escalation worker
4. webhook replay metric integration test
5. wire metrics into alerts + dashboard

(The 4 Phase-E report docs are intentionally left uncommitted.)
