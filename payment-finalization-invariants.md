# Payment Finalization Invariants

Date: 2026-05-22
Status: Authoritative payment correctness contract. Concrete behavior in `backend/internal/services/payment.go`, `backend/internal/handlers/payment.go`, and `backend/migrations/000021_payment_order_correctness.up.sql` must satisfy every invariant here. Deviations are bugs.

## 0. Why this exists

Payments are the only part of QR Dining where double-execution loses or duplicates real money. The original audit's F-06, F-07, F-12, F-19 findings all centered on payment correctness. Hardening landed in Phase 7 with bill snapshots, signed webhooks, scoped idempotency, staff settlement, and the order state machine. This document promotes those into named invariants that must hold under network failure, concurrent actors, replay, and timeout.

## 1. Vocabulary

- **Bill snapshot**: row in `bill_snapshots` capturing immutable line items, subtotal, discount, tax, service charge, tip placeholder, total, currency, and source order IDs at the moment payment is requested.
- **Payment**: row in `payments` representing an attempt to settle a snapshot. Has a method (`cash`, `card_manual`, `upi`, `digital`) and a status.
- **Settlement**: the act of moving a payment to `completed` and, if the session balance now reaches zero, transitioning the session to `closed`.
- **Snapshot residual**: the difference between snapshot total and completed payments against the same snapshot.

## 2. Payment Status Enum

```
requested
provider_pending
requires_staff_confirmation
completed
failed
cancelled
refunded
partially_refunded
```

Allowed transitions:

```
requested -> provider_pending           (digital methods only)
requested -> requires_staff_confirmation (cash, card_manual, manual_upi)
provider_pending -> completed | failed | cancelled
requires_staff_confirmation -> completed | cancelled
completed -> refunded | partially_refunded
failed/cancelled/refunded/partially_refunded -> (terminal except refund flows below)
```

Disallowed: any reverse transition. The SQL update for status transitions must include `AND status = $expected_current` to prevent rewriting terminal rows.

## 3. Bill Snapshot Invariants

**BS-1**: A snapshot is immutable after creation. There is no UPDATE path on `bill_snapshots`. New facts about a session create new snapshots.

**BS-2**: A snapshot references the exact set of order IDs and order item rows that contributed to its totals. The `source_order_ids` and per-item subtotal must be reproducible by querying the source orders at the snapshot's create time (use the order item snapshot fields, not current menu values).

**BS-3**: Totals are integer minor units. No float arithmetic in the snapshot path. The check constraint `total = subtotal - discount_amount + tax_amount + service_charge + tip_amount` must be enforced in code prior to insert.

**BS-4**: A snapshot is created inside the same transaction that creates the first payment that references it. There is no orphan snapshot path.

**BS-5**: A new order placed against a session that already has a non-terminal payment against an active snapshot triggers `ERR_PAYMENT_IN_PROGRESS` and the order is rejected. Variant: if branch policy allows late orders, the order is accepted into a flag `pending_post_settlement` queue and is not included in the current snapshot. Default is reject.

**BS-6**: When all payments against a snapshot reach terminal state, if completed payments sum equals snapshot total the snapshot is `settled`; if less, snapshot is `partial`; if zero, snapshot is `cancelled`. This is a derived view, not a column update.

## 4. Settlement Invariants

**SET-1**: A settlement that closes the session must update payment status, write the audit event, and transition the session in a single DB transaction. The realtime publish happens after commit. No partial commit may leave session active with snapshot settled.

**SET-2**: Concurrent settlement attempts on the same payment row use `SELECT ... FOR UPDATE`. The second attempt sees the now-terminal status and exits without writing. This is enforced by the `WHERE id=$1 AND status=$expected` clause in `MarkPaymentCompleted` and equivalents.

**SET-3**: A staff settlement and a digital webhook settlement targeting the same payment row are mutually exclusive by SET-2. The loser produces a no-op response, never a duplicate completion.

**SET-4**: A successful settlement of the snapshot's total triggers a session close via `CloseSessionIfActive` in the same transaction. If the close fails (session already terminal), the settlement still succeeds and the audit notes a divergence for support attention. This avoids holding a real-money completion hostage to session state drift.

**SET-5**: If multiple payments against the same snapshot complete (split-pay), the close happens when completed sum reaches snapshot total. Excess payment is impossible because the second-to-last completion claims the residual; the last attempt that would over-pay is rejected with `ERR_OVERPAYMENT`.

**SET-6**: Tips are stored on the snapshot at request time. Adding a tip after request requires cancelling the snapshot (cancelling all non-terminal payments first) and creating a new snapshot. This is intentional — tip changes must be visible to staff before settlement.

## 5. Idempotency Invariants

**IDM-1**: Every payment-mutating endpoint accepts an `Idempotency-Key` header. The server stores it in `idempotency_keys` scoped by `session:{id}` and `actor:{participant or staff}:{id}` (see migration `000021`).

**IDM-2**: A repeated request with the same key and the same normalized request hash returns the original response (200/201), not a new payment.

**IDM-3**: A repeated request with the same key and a different request hash returns 409 `ERR_IDEMPOTENCY_CONFLICT`. No second payment is created.

**IDM-4**: A request without an idempotency key against a payment-mutating endpoint is accepted but produces a warning metric. After R7 production soak this is upgraded to 400.

**IDM-5**: Idempotency rows live at least 24 hours past payment terminal status. Older rows may be reaped by a janitor job.

## 6. Webhook Invariants

**WH-1**: Every provider webhook endpoint verifies HMAC-SHA256 over the raw body using the provider's configured secret in `cfg.Payment.WebhookSecrets`. The request body must not be parsed before verification.

**WH-2**: The webhook handler validates the `X-Webhook-Timestamp` (or provider equivalent) against `cfg.Payment.WebhookTimestampTolerance` (default 5 min). Out-of-window timestamps are rejected with 401 and audited `payment.webhook.signature_failed` with reason `stale_timestamp`.

**WH-3**: Webhook idempotency is keyed by provider event ID (e.g., Razorpay `event_id`, Stripe `id`), stored in `payment_webhook_events.external_event_id`. A repeated event ID returns 200 with the existing processed status; it does not re-run the side effects.

**WH-4**: `MarkWebhookProcessed` is called for every webhook on both success and failure path. A webhook row left `processed=false` is a bug. Reconciliation tooling alerts on rows older than 30 minutes with `processed=false`.

**WH-5**: A webhook claiming a payment ID that does not belong to the provider's account or to a known payment row is rejected as `unknown_payment_reference` and audited. It does not create a payment.

**WH-6**: A webhook for a `completed` payment with a different amount than the snapshot total is rejected and audited `payment.webhook.amount_mismatch`. The webhook row is recorded but the payment status is not transitioned.

**WH-7**: Webhook side effects (payment status change, session close) happen in one transaction with the audit write. If the transaction fails the webhook row stays `processed=false` and the provider's retry policy will redeliver.

## 7. Concurrent Payment Attempt Invariants

**CON-1**: Two participants pressing "Pay" concurrently produce at most one bill snapshot per session at any given time. The second request finds the snapshot via session-scoped lookup and either attaches a new payment row referencing it or is rejected with `ERR_PAYMENT_IN_PROGRESS` per branch policy on split-pay.

**CON-2**: Cash settlement by staff and a guest's digital provider settlement targeting the same snapshot: SET-2 guarantees one wins. The loser's payment status moves to `failed` with reason `snapshot_already_settled`, and an audit entry is written.

**CON-3**: A long-running webhook that takes the row lock past the request deadline triggers the next-attempt path to receive 409 `ERR_PAYMENT_LOCKED`. The client must retry with backoff. The lock is bounded by transaction timeout.

## 8. Promo Race Invariants

**PR-1**: Promo redemption is finalized inside the order-place transaction. `SELECT ... FOR UPDATE` on the promo row plus the conditional `IncrementPromoRedemptionCount ... WHERE current_count < max_count` makes the increment atomic.

**PR-2**: Promo validation at preview time is informational. It does not reserve capacity. Two clients seeing "valid" can both attempt to use; only one increment succeeds.

**PR-3**: Per-phone caps require a normalized phone on the participant or guest credential. If the phone is unknown the per-phone cap cannot be enforced; that promo cannot be applied (return `ERR_PROMO_REQUIRES_PHONE`).

**PR-4**: Promo applied via the cart is bound to the bill snapshot at snapshot-create time. After snapshot creation, removing the promo requires cancelling the snapshot.

## 9. Cart Mutation During Payment

**CMP-1**: Once a session enters `payment_pending`, the cart is frozen. `POST /sessions/:id/cart/items` returns 409 `ERR_PAYMENT_IN_PROGRESS`.

**CMP-2**: Going back to `active` (all payments cancelled or failed) re-allows cart mutation.

**CMP-3**: An order placement that races with payment_pending entry uses transaction ordering: the order placement transaction sees the snapshot if it has committed and is rejected; otherwise it succeeds and forces the snapshot to recompute or be cancelled by the payment service which sees the new order at commit.

**CMP-4**: There is no "modify snapshot" path. New facts produce new snapshots.

## 10. Reconnect During Payment

**RC-1**: Snapshot fetch returns the current snapshot, all payment rows, and the session state.

**RC-2**: A device that initiated payment and then lost connection sees the same payment row with its current status. It must not initiate a new payment.

**RC-3**: A second device that reconnects and tries to pay while a non-terminal payment exists sees `ERR_PAYMENT_IN_PROGRESS` unless split-pay is enabled.

**RC-4**: After payment terminal (completed/failed/cancelled) the snapshot endpoint reflects the terminal state. The frontend transitions UI accordingly without polling.

## 11. Timeout During Payment

**TIM-1**: The session-timeout worker does not transition a session in `payment_pending` to `abandoned` while any payment row is non-terminal. The worker locks the session row and exits without action if a non-terminal payment exists. The `payment_pending_stuck_threshold` (default 10 minutes) controls when to alert.

**TIM-2**: If `payment_pending_stuck_threshold` is exceeded, the worker writes `payment.stuck` audit and notifies branch staff via a realtime event. Staff has a manual `cancel_pending_payments` action that moves all non-terminal payments to `cancelled` and returns the session to active.

**TIM-3**: Timeout during `requires_staff_confirmation` defers to TIM-1; staff are still expected to settle. Cancel via TIM-2 is the staff escape hatch.

## 12. Partial Payments

**PP-1**: Partial payment support is opt-in per branch via `branches.settings_json.split_pay_enabled`. When disabled, only one non-terminal payment may exist per snapshot at any time.

**PP-2**: When enabled, each completed payment claims a portion of the residual. The server tracks `claimed_amount` per payment. The sum of claimed amounts never exceeds snapshot total (SET-5 enforces).

**PP-3**: Partial payments do not close the session until residual reaches zero. The session remains `payment_pending`.

**PP-4**: A failed partial payment frees its claim. The released amount is available to subsequent attempts.

## 13. Duplicate Settlement Attempts

**DS-1**: A staff member tapping "Settle" twice is squashed by IDM-1 if the client passes a stable key. If not, SET-2's row lock plus expected-status WHERE clause makes the second attempt a no-op.

**DS-2**: Webhook replay (same external event id) is squashed by WH-3.

**DS-3**: Cross-channel duplicate (staff settles cash; webhook later arrives for digital attempt) is squashed by SET-3, with the loser audited.

## 14. Stale Bill Snapshots

**SBS-1**: A snapshot becomes "stale" when a new non-cancelled order is placed against the session after snapshot creation. New orders are blocked by BS-5 in default mode, so this should not occur. If branch policy allows late orders, the late orders form a separate snapshot.

**SBS-2**: Settling a snapshot that is stale (i.e., where there are uncovered post-snapshot orders) does not close the session. Session remains `payment_pending` until residual covered.

**SBS-3**: The settlement handler explicitly checks `bill_snapshots.total == sum(completed payments)` AND no post-snapshot orders exist before transitioning the session to `closed`.

## 15. Refund Invariants

**REF-1**: A refund creates a new row in `payments` with method `refund` and a negative amount, referencing the original payment.

**REF-2**: Refund causes the original payment status to become `refunded` or `partially_refunded` depending on amount.

**REF-3**: Refund requires `payment.refund` policy. Audit always records actor, reason, original payment, and amount.

**REF-4**: Refunds do not reopen sessions. A refunded payment's session stays closed.

## 16. Recovery Behavior

**REC-1**: A payment stuck in `provider_pending` for longer than the provider's expected SLA (default 30 min) is moved to `requires_staff_confirmation`. Audit `payment.escalated_to_staff` written. The frontend reflects.

**REC-2**: Webhook backlog: if `payment_webhook_events.processed=false` rows older than 30 min exist, alert fires. Operations runbook covers manual reprocess via a CLI that re-verifies signature and reruns the handler.

**REC-3**: After Redis loss, no payment state lives in Redis; all payment correctness is in Postgres. Recovery is automatic.

## 17. Reconciliation Strategy

A daily reconciliation job, run at branch business-day cutover, produces:

- Sessions terminal but with non-terminal payments. Alert.
- Snapshots with `claimed_amount` sum greater than total. Critical alert (should be impossible).
- Snapshots `partial` for more than 24 hours. Operational alert.
- Webhooks unprocessed > 30 min. Critical alert.
- Payments completed without a snapshot. Critical alert (should be impossible).

Report is appended to `audit_log` as `reconciliation.daily` with a JSON body.

## 18. Adversarial Test Scenarios

| Scenario | Required outcome |
| --- | --- |
| Replay a Razorpay webhook with the same event id 10 times | One status transition, nine 200s with `already_processed` |
| Modify webhook body but keep signature header | 401, audited `signature_failed` |
| Two staff devices settle the same cash payment simultaneously | Row lock; one succeeds, one no-op |
| Guest taps "Pay" and "Pay" within 200ms with the same idempotency key | One payment row, two 201s with same body |
| Guest replays a session URL after close and POSTs /payments | 410 from session validation; payment handler never reached |
| Webhook arrives 10 minutes after staff cash settlement | Webhook recorded; payment row updated to `failed`; audit notes divergence; no double credit |
| Guest places order during payment_pending | 409 `ERR_PAYMENT_IN_PROGRESS` |
| Network drop after order placement transaction commits but before client receives response | Client retries with idempotency key; gets original order back |
| Promo cap is 100, 150 concurrent attempts | Exactly 100 succeed; 50 get `ERR_PROMO_CAP` |
| Per-phone promo replayed without phone | `ERR_PROMO_REQUIRES_PHONE`, no redemption increment |
| Refund issued, then a stale device tries to refund again | Second refund honors REF-3 and policy; cannot exceed remaining refundable amount |
| Session in `payment_pending` for 20 minutes with no progress | Alert at 10 min; staff cancel action available; on cancel, session returns to active |

## 19. Required Schema (delta vs current)

- Confirm `bill_snapshots` exists with columns named per `migrations/000021_payment_order_correctness.up.sql:48-64`.
- Confirm `payments.bill_snapshot_id` foreign key (migration `000021:67`).
- Confirm `payments.method` enum covers `cash`, `card_manual`, `digital`, `upi`, `refund`.
- Confirm `payments.status` enum matches Section 2.
- Confirm `payments.claimed_amount` exists if split-pay is supported.
- Add `payment_webhook_events.external_event_id` unique constraint.

## 20. Definition of Done

Payment correctness is "done" only when:

- Every invariant in Sections 3 through 16 is exercised by a deterministic test.
- The adversarial table in Section 18 is a Playwright + Go integration test suite that passes on every CI run.
- The daily reconciliation job runs in production for at least 30 days with zero critical alerts.
- A signed payment review (security + finance) confirms the contract holds.
