# Payment Escalation Lifecycle

Date: 2026-05-25 (Phase E)
Status: implemented, alert-only.

## Why this exists

Phase D flagged that `payment_pending` had no bound: a session whose payment never
finalizes (provider webhook never arrives, staff never settles) sat in
`payment_pending` indefinitely — cart frozen, table occupied, invisible to operators.
This is operationally important ahead of Wave R7 (`PAYMENT_STAFF_SETTLEMENT_REQUIRED`),
where a forgotten settlement strands a table.

## Design decision: ALERT-ONLY

The escalation **never mutates payment or session state**. It does not auto-settle,
auto-cancel, or auto-close. Reasons:

- Auto-settling fabricates a settlement that may not have happened → violates
  `payment-finalization-invariants` (SET/settlement integrity).
- Auto-cancelling risks cancelling a payment that actually succeeded provider-side
  but whose webhook is merely late (the exact race chaos testing cares about).
- The correct authority for resolving a stalled settlement is a human (waiter/manager),
  not a timer.

So escalation makes the stall **loud and visible** and leaves resolution to staff.

## Mechanism

Worker `RunPaymentPendingEscalation` (`internal/worker/worker.go`), scheduled every
`PAYMENT_PENDING_ESCALATION_INTERVAL` (default 1m), Redis-locked like the other
lifecycle workers.

Each tick:
1. `ListPaymentPendingStalled(olderThan)` (`repository/payment.go`) returns sessions in
   `payment_pending` whose **oldest non-terminal payment** (`pending` / `requested` /
   `provider_pending` / `requires_staff_confirmation`) was `initiated_at` before the warn
   threshold. Bounded `LIMIT 200`.
2. Per session, compute age and level:
   - `warn` — age ≥ `PAYMENT_PENDING_WARN_AFTER` (default 5m)
   - `critical` — age ≥ `PAYMENT_PENDING_CRITICAL_AFTER` (default 15m)
3. De-dupe via a Redis marker `payment_escalated:{level}:{session}` (TTL 2× critical) so
   each session fires at most once per level. Fails open (a duplicate alert beats a
   missed one).
4. Emit, without touching state:
   - metric `payment_pending_escalations_total{level}`
   - structured warn log (session_id, payment_id, branch_id, level, age_seconds)
   - `event_log` row `PAYMENT_SETTLEMENT_STALLED`
   - realtime event `PAYMENT_SETTLEMENT_STALLED` (backend-only event type; staff UI may
     adopt it later — no frontend change shipped)
   - audit event `payment.settlement.stalled` (`ActorTypeSystem`, RiskMedium)

## Thresholds (env-tunable)

| Env | Default | Meaning |
|-----|---------|---------|
| `PAYMENT_PENDING_ESCALATION_INTERVAL` | 1m | worker tick |
| `PAYMENT_PENDING_WARN_AFTER` | 5m | warn level |
| `PAYMENT_PENDING_CRITICAL_AFTER` | 15m | critical level |

Tune during the staging soak before R7; defaults are conservative starting points.

## Alerting

`PaymentPendingEscalationCritical` (page) fires on
`increase(payment_pending_escalations_total{level="critical"}[15m]) > 0`. Dashboard panel
"payment_pending escalations (alert-only)" plots warn vs critical rate.

## Operator recovery (runbook)

When a critical escalation pages:
1. Open the session; identify the non-terminal payment.
2. If the guest actually paid (provider dashboard shows success): settle it via staff
   settlement — the late webhook, if it arrives, is idempotent and will not double-count.
3. If the guest did not pay / abandoned: cancel the payment via the normal staff flow;
   the session returns to `active` (cart unfreezes) and can be closed/settled normally.
4. Only after the payment reaches a terminal state does the table free under the normal
   lifecycle. The escalation worker itself never frees it.

## Interaction with other lifecycle workers

The reactivation pipeline already refuses to abandon a session with a non-terminal
payment (TIM-1, `HasNonTerminalPaymentForSession`). So a stalled `payment_pending`
session is **not** silently abandoned by another worker — it persists, visible and
alerting, until a human resolves it. Cart stays frozen throughout
(`IsSessionCartFrozen`).

## Table recovery policy

Tables are freed only by a terminal payment + normal session close/abandon. There is no
automatic table reclamation for stalled payments — by design, since reclaiming a table
under an unresolved payment would risk losing money or double-charging. The critical
alert is the forcing function for human action.
