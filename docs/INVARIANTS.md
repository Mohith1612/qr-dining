# Product invariants

Last verified against code: 2026-09-13. This document is the arbiter when a
test expectation conflicts with implementation. “Broken” means the current code
does not satisfy the stated product rule; it is not permission to weaken the
rule.

## Host and authority

**H1 — one host per session. Holds on service-managed paths.** Session creation
inserts one host and sets `sessions.host_participant_id` in the same transaction
(`backend/internal/services/session.go:130-175`). Reassignment clears every other
participant's `is_host` flag and updates the session pointer in one transaction
(`backend/internal/repository/participant.go:51-66`,
`backend/sql/queries/participants.sql:25-30`).

**H2 — explicit transfer is host-only, targets an active participant in the same
session, requires that target to be present, and is locked during payment.
Holds.** (`backend/internal/services/session.go:362-396`)

**H3 — automatic transfer may promote only a present, active participant after
the current host is absent. Holds.** The acting non-host is checked for session
membership, revocation, and current presence before promotion
(`backend/internal/services/session.go:318-359`). Snapshot healing selects the
oldest non-revoked participant that is present
(`backend/internal/services/session.go:519-564`).

**H4 — a participant who stops heartbeating becomes absent. Holds.** Presence is
calculated per hash field: a participant is present only when that field's
timestamp is no more than 90 seconds old. The Redis hash TTL is storage cleanup,
not the presence decision (`backend/internal/redis/presence.go:13-18,57-76,151-158`).

**H5 — automatic host transfer waits three minutes of host absence and is
visible to clients. Holds.** The default grace is three minutes; the stored
heartbeat history lives for grace plus the 90-second presence threshold
(`backend/internal/redis/presence.go:13-18,30-43`). The service considers Redis
and durable timestamps, fails conservatively on read errors, and adds the
two-minute database-heartbeat throttle when Redis history is missing
(`backend/internal/services/session.go:399-440`). A completed transfer publishes
`HOST_CHANGED` (`backend/internal/services/session.go:443-453`).

**H6 — only the host places orders, initiates payment, or closes the guest
session. Holds.** Order and payment use the same host authority
(`backend/internal/services/order.go:95-108`,
`backend/internal/services/payment.go:128-146`); guest close rejects a non-host
before closing (`backend/internal/handlers/session.go:91-138`). Staff recovery is
a separate governed path described by S2.

## Cart and orders

**C1 — one shared cart per session; any authenticated participant may add or
remove. Holds.** A partial unique index permits at most one cart with
`participant_id IS NULL` per session
(`backend/migrations/000027_shared_session_cart.up.sql:1-10`). Cart mutations use
that shared cart and do not impose a host check
(`backend/internal/services/cart.go:60-110,114-150`).

**C2 — only the host turns the cart into an order. Holds.**
(`backend/internal/services/order.go:95-108`)

**C3 — a session may accumulate orders; payment covers all non-cancelled session
orders, not one order. Holds.** Order creation records undiscounted item totals
and applies no promo (`backend/internal/services/order.go:222-266`). Bill
freshness compares the snapshot's source order IDs with every current
non-cancelled order in the session (`backend/internal/services/payment.go:849-885`).

**C4 — cart and order mutations are frozen while payment is pending. Holds.**
The shared status predicate defines `payment_pending` as frozen
(`backend/internal/domain/statemachine.go:43-47`); cart and order services apply
it before mutation (`backend/internal/services/cart.go:60-72,114-124`,
`backend/internal/services/order.go:70-80`).

## Payment

**P1 — at most one non-terminal payment exists per session. Holds.** Migration
40 rejects pre-existing duplicates, then creates a partial unique index covering
`pending`, `requested`, `provider_pending`, and `requires_staff_confirmation`
(`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`).

**P2 — repeated payment initiation returns the existing request. Holds.** The
service reuses a non-terminal payment before creating one, also handles the
`payment_pending` transaction race, and converges a unique-index race onto the
existing record (`backend/internal/services/payment.go:148-190,223-255,331-338`).
The HTTP handler returns 200 for reused and 201 for newly created payments
(`backend/internal/handlers/payment.go:186-193`).

**P3 — payment amount equals the complete server-computed bill; partial payment
is unsupported. Holds.** The handler computes the bill and rejects a supplied
amount differing by more than half a currency cent with 422
(`backend/internal/handlers/payment.go:105-149`).

**P4 — settlement closes against one immutable bill snapshot, not the sum of
unrelated payments. Holds.** Snapshot and payment are created in one transaction
(`backend/internal/services/payment.go:271-294`). Close checks snapshot freshness
and sums completed payments for that snapshot only
(`backend/internal/services/payment.go:814-845`,
`backend/sql/queries/payments.sql:100-105`).

**P5 — every supported payment method waits for staff confirmation; initiation
does not reach `provider_pending`. Holds.** `card` normalizes to `card_manual`,
and every handler-accepted method returns `requires_staff_confirmation`
(`backend/internal/handlers/payment.go:97-103`,
`backend/internal/services/payment.go:759-773`). The
`PAYMENT_STAFF_SETTLEMENT_REQUIRED` value is still passed to an unread parameter,
so it has no effect on this behavior (`backend/internal/handlers/payment.go:152-165`,
`backend/internal/services/payment.go:759-773`).

**P6 — waiter, manager, or owner may cancel an outstanding payment and release
the frozen session. Holds.** The route is registered
(`backend/internal/server/server.go:379-392`); authorization checks the payment's
branch and roles before calling the service
(`backend/internal/handlers/payment.go:369-430`). The service applies the payment
transition table, tries to release `payment_pending`, publishes the result, and
records the reason (`backend/internal/services/payment.go:639-691`).

## Session lifecycle

**S1 — timeout warning and stale closure use last participant activity, falling
back to session creation only when there are no participants. Holds.** The closer
uses `COALESCE(MAX(last_seen_at), created_at)`
(`backend/sql/queries/workers.sql:3-14`), and the 15-minute warner uses the same
expression (`backend/sql/queries/sessions.sql:110-123`). This corrects the stale
status in `audit/Invariance.md`.

**S2 — manager or owner may force-close a same-branch non-terminal session.
Holds.** The handler requires a reason, derives scope from the session, checks
branch and role, and writes a high-risk audit event
(`backend/internal/handlers/session.go:141-236`). The service closes first, then
cancels transition-eligible outstanding payments and reports any it cannot
transition (`backend/internal/services/session_close.go:34-81,84-126`).

**S3 — terminal sessions do not accept later transitions. Holds in the domain
transition table.** Closed, abandoned, and expired have no outgoing transitions
(`backend/internal/domain/statemachine.go:82-107`).

## Tenancy

**T1 — a suspended or archived organization or branch refuses QR resolution,
new sessions, and joins; existing sessions continue. Holds.** The gate admits
only `active`, fails closed on lookup errors, and defines suspension as entry-only
(`backend/internal/services/tenant_status.go:13-23,33-78`). Session create and
join call it (`backend/internal/services/session.go:99-115,567-596`); QR resolve
calls it before returning the table (`backend/internal/services/menu.go:98-112`).

**T2 — the guest sees a clear suspension message. Broken in the current
frontend.** The backend distinguishes organization and branch suspension as 403
responses (`backend/internal/handlers/menu.go:36-58`,
`backend/internal/handlers/session.go:430-453`), but the QR page converts every
resolve failure to “invalid or expired” and every create/join failure except an
invalid phone to a generic error
(`frontend/app/(guest)/table/[token]/page.tsx:57-60,109-113`).

**T3 — emergency closure of live sessions is separate from suspension. Holds.**
The tenant gate is entry-only (`backend/internal/services/tenant_status.go:49-51`),
while force-close is a separate staff route
(`backend/internal/server/server.go:387-392`).

## Known implementation gaps adjacent to these invariants

- Staff logout clears the cookie but does not revoke the server-side staff
  session (`backend/internal/handlers/staff.go:124-130`); PIN reset does revoke
  durable and cached sessions (`backend/internal/services/staff.go:336-380`).
- Idempotency rows receive an expiry timestamp, but lookup ignores it and no
  query reaps expired rows (`backend/internal/repository/idempotency.go:18-35`,
  `backend/sql/queries/idempotency.sql:1-26`).
- The presence-expiry worker is currently a notification no-op; per-field
  readers still enforce presence age (`backend/internal/worker/worker.go:233-251`,
  `backend/internal/redis/presence.go:151-158`).
