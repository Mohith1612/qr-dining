# Phase 7 - Payment and Order Correctness

## Objective

Make order creation, idempotency, promo redemption, bill snapshots, payment settlement, and webhook processing safe under concurrency and adversarial traffic.

## Dependencies

- Phase 1 guest credentials.
- Phase 2 centralized policy.
- Phase 5 audit logging.
- Feature flag:
  - `PAYMENT_STAFF_SETTLEMENT_REQUIRED`

## Current Codebase Context

Relevant files:

- `backend/internal/services/order.go`
- `backend/internal/handlers/order.go`
- `backend/sql/queries/orders.sql`
- `backend/internal/domain/statemachine.go`
- `backend/internal/services/payment.go`
- `backend/internal/handlers/payment.go`
- `backend/sql/queries/payments.sql`
- `backend/internal/handlers/billing.go`
- `backend/internal/services/promo.go`
- `backend/internal/handlers/promo.go`
- `backend/sql/queries/promos.sql`
- `backend/migrations/000013_order_sequences.up.sql`
- `backend/migrations/000014_payment_billing.up.sql`
- `backend/migrations/000015_promos.up.sql`

Current risks:

- Order placement accepts client branch and participant IDs.
- Menu items are not fully verified against session branch.
- Idempotency key is globally unique and not request-bound.
- Webhooks are unauthenticated.
- Cash/card/UPI frontend success can happen while backend payment remains pending.
- Promo redemption cap checks are race-prone.

## Order Creation Target

Request body must exclude:

- `branch_id`
- `placed_by_participant_id`

Server derives:

- session from route
- branch from session
- participant from guest actor

Validation:

- Session is active.
- Participant belongs to session.
- All menu items belong to session branch.
- All modifiers belong to selected item.
- Item is available at order time.
- Idempotency key is scoped to session and participant.

## Idempotency Target

Add general idempotency model:

```text
idempotency_keys
  id
  scope_type
  scope_id
  actor_type
  actor_id
  key
  request_hash
  response_resource_type
  response_resource_id
  status
  created_at
  expires_at
```

Rules:

- Same key and same hash returns original result.
- Same key and different hash returns conflict.
- Scope for orders is session plus participant.
- Scope for payments is session plus actor.

## Order State Machine Hardening

Keep existing order state machine as foundation:

```text
pending -> confirmed -> preparing -> ready -> served
pending/confirmed -> cancelled
```

Mutation must include:

- branch scope
- expected current status
- authorized role/action

Audit old/new status and actor.

## Branch Order Numbering

Keep branch-local daily sequence.

Add:

- `order_business_date`
- `order_number_display`
- `order_operational_id`

Example:

```text
order_number_display = "OR1001"
order_business_date = "2026-05-21"
order_operational_id = "BLR-INDIRANAGAR-20260521-OR1001"
```

## Payment Lifecycle Target

Replace ambiguous pending/completed-only operational behavior with:

```text
requested
provider_pending
requires_staff_confirmation
completed
failed
cancelled
refunded
partially_refunded
```

Payment method behavior:

- `cash`: staff-settled.
- `card_manual`: staff-settled after POS confirmation.
- `digital`: provider-settled by verified webhook.
- `upi`: provider-settled if integrated, otherwise staff-settled.

## Bill Snapshots

Add bill snapshot model:

```text
bill_snapshots
  id
  session_id
  branch_id
  subtotal
  discount_amount
  tax_amount
  service_charge
  tip_amount
  total
  currency
  source_order_ids jsonb
  created_by_actor
  created_at
```

Payment references a bill snapshot.

Session closes only when settled payments cover all non-cancelled order totals.

## Webhook Security

Webhook handler must:

- Verify provider signature using raw body.
- Validate timestamp tolerance.
- Store raw payload and headers.
- Use external provider references, not internal payment IDs supplied by payload.
- Verify amount, currency, session, and branch.
- Mark webhook processed with the correct webhook event row.
- Audit invalid signatures and processing failures.

## Promo Redemption

Final promo redemption must:

- Occur inside order/payment transaction.
- Lock promo row or use DB-enforced counters.
- Enforce max uses atomically.
- Enforce per-phone limits against normalized identity.
- Audit redemption and failure reasons.

## Exit Criteria

- Orders cannot cross session/branch/menu boundaries.
- Idempotency is scoped and request-bound.
- Payment completion means backend-settled payment.
- Webhooks cannot be forged.
- Promo caps cannot be exceeded by concurrent requests.

## Implementation Update - 2026-05-22

Implemented in commit `48d89c9`:

- Added Phase 7 migration `000021_payment_order_correctness` for scoped idempotency keys, bill snapshots, payment lifecycle/method extensions, order operational IDs, webhook raw/header storage, and promo redemption counters.
- Regenerated sqlc query code for idempotency, orders, payments, promos, and new bill snapshot fields.
- Hardened order placement to derive branch/participant server-side, validate session/participant/menu/modifier boundaries, reject scoped idempotency hash conflicts, and generate branch-local operational order IDs.
- Hardened order status updates with expected-status mutation semantics and old/new status audit payloads.
- Added immutable bill snapshot creation at payment initiation and linked payments to snapshots.
- Added backend payment lifecycle handling for manual staff-confirmed payments, provider-pending payments, staff settlement, provider-reference webhook completion, amount/currency/session/branch verification, and stale bill snapshot rejection before closing sessions.
- Added generic HMAC SHA-256 webhook verification using `PAYMENT_WEBHOOK_SECRET_<PROVIDER>` and `PAYMENT_WEBHOOK_TIMESTAMP_TOLERANCE`.
- Hardened promo final validation by locking promo rows, using atomic redemption counters, normalizing phone identity, and incrementing redemption count inside the order transaction.
- Updated frontend payment types/copy and OpenAPI payment/webhook/settlement contracts.

Verified:

- `GOCACHE=/tmp/qr-dining-go-build go test ./...`
- `GOCACHE=/tmp/qr-dining-go-build go test -tags integration ./internal/services/... ./internal/handlers/...`
- `npm run typecheck`
- `npm run lint` exits 0 with existing unrelated warnings.

Not committed by request:

- This plan-file update remains uncommitted.

Follow-up alignment fixes implemented in commit `77f796f`:

- Added payment idempotency using the shared `idempotency_keys` model, scoped to session plus participant actor, with request hashing, replay return, and changed-body conflict handling.
- Required and sent payment `idempotency_key` through the public payment API and frontend payment flow.
- Kept order request bodies free of client-owned branch/participant IDs while preserving permissive-mode guest fallback through `X-Participant-ID`.
- Added audit events for webhook processing failures and promo redemption success/failure reasons.
- Made `cash` and `card_manual` always require staff confirmation; `digital` remains provider-settled and `upi` is provider-settled unless staff settlement is required.
- Added active authorization tests for cross-branch order status updates and cross-branch payment settlement.
- Fixed frontend order placement to stop sending now-rejected `branch_id` and `placed_by_participant_id` fields.

Verified after follow-up:

- `GOCACHE=/tmp/qr-dining-go-build go test ./...`
- `GOCACHE=/tmp/qr-dining-go-build go test -tags integration ./internal/services/... ./internal/handlers/...`
- `npm run typecheck`
- `npm run lint` exits 0 with existing unrelated warnings.

## Test Requirements

- Menu item from branch B rejected for branch A session.
- Participant from another session rejected.
- Idempotency replay with changed body rejected.
- Staff from branch A cannot update branch B order.
- Invalid webhook signature rejected.
- Manual payment requires authorized staff settlement.
- Payment cannot close session when bill snapshot no longer covers all orders.
