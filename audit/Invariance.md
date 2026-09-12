# Intended behaviour — invariants

Stated by the product owner, September 2026. These are the target. Where the code differs today,
the code is wrong and the finding ID is noted.

**Status markers re-verified against code 2026-09-12.** The numbered invariants and the open
decisions are the product owner's and are unchanged; only the *Holds / Broken / Not built* lines
were corrected, each against a cited source. One line is marked *Reported fixed* — H4 — a closure claimed
elsewhere that I could not confirm from code alone. Every other status line on this page is
verified against the cited source.

Each numbered invariant should end up with a test asserting it.

---

## Host and authority

**H1.** Exactly one participant is host at any time.

**H2.** The host may transfer the role explicitly to another participant who is **present**.
*Holds today* — `ReassignHost` now requires the target to be present
(`backend/internal/services/session.go:382`).

**H3.** If the host disconnects, the role transfers automatically to another **present**
participant. A participant who is not present may never become host.
*Fixed* — F-16 closed. Promotion now requires the host absent **and** the acting participant
present (`backend/internal/services/session.go:332`, `:342`). This is how the ₹300
double-collection happened.

**H4.** A host who has genuinely left must be detectable as absent.
*Reported fixed* — F-06 closed with the presence work; presence is now read per participant
(`backend/internal/services/session.go:397`, `:477`). **Needs a citation for the TTL change
itself** before this line is treated as verified.

**H5.** Automatic transfer requires the host to have been absent for a grace period, and the
change must be visible to participants. **Decided 2026-09-12: three minutes.**
*Implemented as decided* — `hostAbsenceGrace` defaults to `3 * time.Minute`
(`backend/internal/services/session.go:39`), overridable via `SetHostAbsenceGrace`.

**The effective floor is roughly five minutes, not three, and that is intended.** The durable
`last_seen_at` fallback is throttled to one write per two minutes
(`presenceDBSyncInterval`, `backend/internal/services/participant.go:16`), so when the Redis
presence read falls back to the durable heartbeat the newest available timestamp can already be
two minutes stale before the three-minute grace starts counting. Test 2A asserts that floor.
Anyone tuning the three-minute constant downward should know they are tuning a number the
throttle already dominates.

**H6.** Only the host may place orders, request the bill, initiate payment, and close the
session.
*Holds today* for orders and payment. The H3 caveat that used to qualify this line is closed.

---

## Cart and orders

**C1.** One cart per session, shared by all participants. Any participant may add and remove
items.
*Holds today* (migration 27, `carts.sql:7`).

**C2.** Only the host converts the cart into an order.
*Holds today.*

**C3.** A session accumulates many orders over the meal. Orders are never paid individually.
*Holds today* — order placement sets no promo or discount, and the bill is computed across all
non-cancelled session orders.

**C4.** While a payment is outstanding the cart is frozen: no add, no remove, no new orders.
*Holds today* — this is the `payment_pending` state.

→ **G-07, O-07 and P-08 are wrong tests.** They assert per-participant carts and
participant-independent ordering and payment. Rewrite them against C1, C2 and H6.

---

## Payment

**P1.** A session has **at most one non-terminal payment at a time**. The bill covers every
non-cancelled order in the session.
*Fixed* — migration 40 adds a partial unique index on `payments(session_id)` over the four
non-terminal statuses, so a second concurrent payment is rejected by the database
(`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql`).

**P2.** Tapping "pay" again returns the existing payment request. It never creates a second one.
*Fixed at the service layer*, which is the right layer. A second initiation on a session that
already holds a non-terminal payment returns that payment
(`reusableNonTerminalPaymentForSession`, `backend/internal/services/payment.go:182`, `:375`);
the handler maps the not-created case to 200 rather than 201
(`backend/internal/handlers/payment.go:186`). This holds even when the caller supplies a **fresh**
idempotency key per tap — the exact frontend behaviour this invariant was written against.
Migration 40 is the concurrency backstop, not the mechanism: a racing insert that trips
`idx_payments_one_non_terminal_per_session` is caught and converges on the same existing payment
(`payment.go:332`). `P-08` asserts 200 with the identical payment ID and passes for this reason.

**P3.** The payment amount is always the full bill total. Partial payment is out of scope.
*Fixed* — the handler now rejects any amount that is not the full bill total
(`backend/internal/handlers/payment.go:148`).

**P4.** A session closes when its bill is settled once — not when the sum of unrelated payments
happens to reach the total.
*Fixed* — the sum is scoped to the bill snapshot
(`SumCompletedPaymentsForBillSnapshot`, `backend/sql/queries/payments.sql:100`).

**P5.** Payment method (cash / card / UPI) is a signal to staff about how the guest intends to
pay. It is not a settlement integration. Every method waits for staff confirmation.
*Fixed* — `normalizePaymentMethodStatus` (`backend/internal/services/payment.go:759`) returns
`requires_staff_confirmation` for every method: `card` is rewritten to `card_manual`, and `cash`,
`card_manual`, `digital` and `upi` are all handled explicitly. All five `PaymentMethod` enum
values are matched before the `provider_pending` default, so that state is unreachable from
initiation. `PAYMENT_STAFF_SETTLEMENT_REQUIRED` no longer has any effect.

**Residual, not a defect:** the dead flag is still threaded through three layers — `config.go:260`
→ `handlers/payment.go:158` → `services.InitiatePaymentRequest.StaffSettlementRequired`
(`payment.go:85`) → the unread `staffRequired` parameter at `payment.go:759`. Go does not warn on
unused function parameters, so the branch this invariant describes can be reintroduced by
accident. Deleting the parameter and its plumbing is a small, safe follow-up.

**P6.** Waiters and managers can cancel a payment request for a session, which unfreezes it and
lets the table continue ordering.
*Built* — `PATCH /payments/:id/cancel`, waiter/manager/owner, branch derived from the payment,
audited; repeat cancel returns 409. (`backend/internal/server/server.go:391`, commit `7ffe4e4`.)

---

## Session lifecycle

**S1.** A session ends when the bill is settled, when the host closes it, or when it ages out via
the stale-session cleaner.
*Does not hold as intended today.* **Decided 2026-09-12: the trigger is last activity, not age.**
Rationale: `created_at + branches.session_timeout_minutes` (default 120) abandons a table two
hours into a long dinner on the same schedule as one that emptied after five minutes, and a
restaurant pilot will hit that.

The replacement is `MAX(session_participants.last_seen_at)` — durable, already written, and
already the predicate the reactivation worker uses for `SESSION_IDLE_GRACE`
(`ListActiveSessionsForReactivationScan`, `backend/sql/queries/sessions.sql:100`). So this is a
change to the cleaner's **query**, not to configuration.

**Not yet implemented. It needs its own task with a failing test first.** Scope note for whoever
writes that task: the age predicate appears in **two** queries, not one, and they must move
together —

- `ListExpiredSessions` (`backend/sql/queries/workers.sql:9`) — the closer.
- `ListSessionsExpiringSoon` (`backend/sql/queries/sessions.sql:116`) — the 15-minute warner
  that sets `warned_at`.

Changing only the closer leaves the warning firing on age while the close fires on inactivity:
tables get warned that will not be closed, and tables get closed that were never warned.

**S2.** Restaurant staff can force-close any session on their branch — for tables that left
without paying, or where something happened outside the app.
*Built* — `POST /sessions/:id/force-close`, manager/owner, branch derived from the session.
(`backend/internal/server/server.go:392`, commit `7ffe4e4`.) `L-07-force-close-by-staff` and
`PT-02-force-close-any-session` were written against a then-unregistered endpoint and must now be
re-checked against the real one rather than assumed to pass.

**S3.** Should be audited with the acting staff member and a reason, since it can discard an
unpaid bill.

---

## Tenancy

**T1.** A suspended organization accepts **no new sessions and no new joins**. Sessions already in
progress finish normally.
*Fixed* — F-05 closed. A `TenantStatusGate` is consulted on the guest entry paths
(`backend/internal/services/session.go:30`, `:51`, `backend/internal/services/tenant_status.go`).

**T2.** A guest scanning a suspended restaurant's QR sees a clear message, not a generic error.

**T3.** Emergency stop — closing live sessions — is a separate action from suspension, not folded
into it. Not currently needed.

---

## Decisions — all closed

1. **H5** — how long must a host be absent before automatic transfer?
   **Three minutes**, decided 2026-09-12, already shipped. Effective floor ~5 min; see H5.
2. **S1** — session timeout on age or on last activity?
   **Last activity**, decided 2026-09-12. Not yet implemented; see S1 for the two-query scope.
3. **P6 / S2** — which roles for cancel-payment and force-close?
   **Answered in code by `7ffe4e4`:** cancel-payment is waiter/manager/owner; force-close is
   manager/owner. Recorded here so the question is not reopened.

No open decisions remain on this page.

---

## What this changes in the plan

**New work, not previously in the ledger:**
- ~~Staff cancel-payment route and service method (P6)~~ — built, `7ffe4e4`. Remaining: a staff
  affordance that reaches it.
- ~~Staff force-close-session route (S2)~~ — built, `7ffe4e4`. Same remaining gap.
- ~~Host-transfer grace period (H5)~~ — decided at three minutes and shipped. **UI visibility of
  the transfer is not confirmed** and is still worth checking separately.
- Stale-session trigger moved from age to last activity (S1) — decided, **not yet built**.
- ~~Suspension checks (T1)~~ — built. Guest-facing message (T2) — verify separately.

**Removed from scope** by "no merchant connected": F-17, F-18, F-04, webhook retry, receipt
reconciliation, refund reversal.

**Tests to rewrite rather than chase:** G-07, O-07, P-08.