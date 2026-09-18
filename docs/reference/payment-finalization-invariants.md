# Payment finalization invariants

Last verified against code: 2026-09-13. The product-level payment rules are P1–P6
in [../INVARIANTS.md](../INVARIANTS.md); this file exists because payment service
comments reference its narrow concurrency contract.

1. **One outstanding request.** A session has at most one payment in `pending`,
   `requested`, `provider_pending`, or `requires_staff_confirmation`; migration 40
   enforces this in PostgreSQL
   (`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`).
2. **Retry converges.** A second initiation returns the existing payment, including
   when it uses a fresh idempotency key or loses an insert race
   (`backend/internal/services/payment.go:148-190,223-255,331-338`). HTTP uses
   200 for reuse and 201 for creation (`backend/internal/handlers/payment.go:186-193`).
3. **The server owns the amount.** The handler computes the bill and rejects a
   caller amount unequal to the complete discounted total with 422
   (`backend/internal/handlers/payment.go:105-149`). Partial payment is not an
   implemented mode.
4. **Freeze and snapshot are transactional.** Moving the session to
   `payment_pending`, creating the bill snapshot, and creating the payment happen in
   one transaction (`backend/internal/services/payment.go:223-294`). Cart and order
   mutations reject the frozen state
   (`backend/internal/services/cart.go:60-72,114-124`,
   `backend/internal/services/order.go:70-80`).
5. **Every current method is manual.** Every handler-accepted payment method enters
   `requires_staff_confirmation`; `provider_pending` is unreachable from initiation
   (`backend/internal/handlers/payment.go:97-103`,
   `backend/internal/services/payment.go:759-773`).
6. **Settlement is snapshot-scoped.** Staff settlement first verifies snapshot
   freshness; closure compares completed payments for the same snapshot with that
   snapshot's total (`backend/internal/services/payment.go:596-624,814-885`,
   `backend/sql/queries/payments.sql:100-105`).
7. **Machines do not repair money.** Payment escalation and billing reconciliation
   are observation-only for financial/session state
   (`backend/internal/worker/worker.go:288-310,319-353`). Staff use settle,
   cancel, or force-close (`backend/internal/server/server.go:386-399`).

Known adjacent defect: payment and order idempotency rows record expiry, but lookup
ignores it and no reaper query exists
(`backend/internal/repository/idempotency.go:18-35`,
`backend/sql/queries/idempotency.sql:1-26`).
