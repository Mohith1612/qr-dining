# Phase 8 - Operational UX Cleanup

## Objective

Reduce operational friction and support burden by introducing human-readable identifiers, clear visibility boundaries, and support-oriented observability.

## Dependencies

- Phase 3 organization and branch codes.
- Phase 6 session/realtime hardening.
- Phase 7 order/payment operational IDs.

## Scope

This phase can include frontend and API representation work, but only after backend trust boundaries are hardened.

## Human-Readable Identifiers

Add or expose:

- `organization.code`
- `branch.code`
- `staff.staff_code`
- `sessions.session_number` or `visit_number`
- `orders.order_operational_id`
- `payments.payment_reference`
- `refunds.refund_reference`
- `audit.event_reference`

Rules:

- UUID remains primary key.
- Human-readable IDs are unique within their intended scope.
- Staff and support UI should prefer readable IDs.
- Diagnostic views may still show UUIDs.

## Operational ID Examples

```text
Session: BLR-INDIRANAGAR-S-20260521-018
Order:   BLR-INDIRANAGAR-20260521-OR1042
Payment: BLR-INDIRANAGAR-PAY-20260521-0031
```

## Visibility Boundaries

Branch staff:

- Default branch only.
- No cross-branch data unless explicitly authorized.

Organization admins:

- Cross-branch dashboards and reports.
- Must choose branch scope before operational mutation.

Platform admins:

- Platform namespace only.
- Support actions visible in audit.

## Observability

Logs should include:

- request_id
- actor type/id
- organization_id
- branch_id
- session_id where relevant
- action
- policy decision reason

Metrics should avoid high-cardinality labels.

Good labels:

- route
- status_code
- action
- actor_type
- payment_provider
- worker_name
- event_type

Avoid labels:

- session_id
- participant_id
- order_id
- customer phone

## Deliverables

- Operational ID generation plan.
- API response migration plan for readable IDs.
- Staff dashboard UUID reduction plan.
- Support search plan by organization, branch, order, session, and payment reference.
- Admin visibility boundary documentation.

## Implementation Status

Implemented in the Phase 8 code change:

- Added additive operational reference schema:
  - `sessions.session_business_date`, `sessions.visit_number`, `sessions.session_number`
  - `payments.payment_business_date`, `payments.payment_sequence`, `payments.payment_reference`
  - `audit_log.event_reference`
  - `session_sequences` and `payment_sequences` for branch/date scoped numbering
- Backfilled existing sessions and payments with deterministic branch/date references.
- Kept UUID/BIGSERIAL primary keys unchanged.
- Generated new session references in `SessionService.CreateSession` using branch-local business dates.
- Generated new payment references in `PaymentService.InitiatePayment` using branch-local business dates.
- Exposed generated fields through regenerated sqlc models and existing API responses.
- Added table identifier and session number to active assistance responses for staff operations.
- Added `/platform/support/search?q=...` for platform support/read-only auditor lookup across:
  - organization code/name/id
  - branch code/name/id
  - session number/session UUID
  - order operational ID/order number/order UUID
  - payment reference/provider payment reference/payment id
  - audit event reference/request/correlation/id
- Updated request logs to include `request_id`, route/action, actor context when authenticated, organization/branch scope where available, status, and latency.
- Updated staff UI labels to prefer:
  - session numbers in admin active sessions
  - order operational IDs in kitchen
  - table identifiers in waiter requests

Verification completed:

- `GOCACHE=/tmp/qr-dining-go-cache go test ./...`
- `npm run lint` (passed with pre-existing warnings only)
- `npm run build`
- Full migration chain applied successfully to disposable Docker Postgres database `qr_dining_phase8_test`.
- Disposable migrated schema confirmed new `audit_log.event_reference`, `payments.payment_reference`, `sessions.session_number`, and `sessions.visit_number` columns.
- Disposable database removed after verification.

Not implemented / intentionally deferred:

- `refunds.refund_reference`: no refunds table or refund domain exists in the current schema.
- Dedicated admin visibility boundary documentation beyond this phase status note remains pending.
- Organization-admin branch selector UX for cross-branch operational mutations remains pending because current staff mutation routes are branch-scoped and already enforce branch membership.

## Exit Criteria

- Staff can operate without raw UUIDs.
- Support can trace incidents by readable references.
- Organization and branch selectors respect policy.
- UUIDs remain available in diagnostics but are no longer primary operational labels.
