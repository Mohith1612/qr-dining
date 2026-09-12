# ADR 0003 — Tenant suspension is enforced at guest entry only

**Status:** Accepted · **Date:** 2026-09-12 · **Outcome:** organization and branch lifecycle status are now read by three guest entry points; live sessions are deliberately untouched; one asymmetry is left open (§7)

> **Context.** Until this change the `organizations.status` and `branches.status`
> columns were inert on every guest path. `handlers/platform_lifecycle.go` said so in
> a comment: *"No operational path reads these statuses yet — the flip is inert until
> a future enforcement phase."* Staff login **did** read them
> (`services/staff.go` `createSession`), so suspending an organization locked the
> restaurant out of its own system while guests kept scanning QR codes and ordering
> into it. The asymmetry — not the missing check — was the defect (T-03 / F-05,
> invariant T1).

---

## 1. Decision

A non-`active` organization or branch status refuses **entry** to a tenant and nothing
else. Three call sites, all on the path a guest takes to get *in*:

| Entry point | Call site |
|---|---|
| QR resolve | `services/menu.go` `GetTableWithActiveSession` |
| Create session | `services/session.go` `CreateSession` (before the transaction) |
| Join session | `services/session.go` `JoinSession` |

The gate is `services.TenantStatusGate` (`services/tenant_status.go`), wired at startup
via `SetTenantStatusGate` on `SessionService` and `MenuService`.

Nothing on a live session's path consults it. Ordering, cart, payment, snapshot,
WebSocket and close are all unchanged for a party already seated.

---

## 2. Organization status and branch status are two gates, not one

They share a vocabulary (`active | suspended | archived`) and are checked in the same
function, which makes it tempting to treat them as one condition. They are not:

- **Organization status** is the billing and compliance state of the **whole tenant**.
  Suspending it stops every branch the organization owns. It is the lever a platform
  operator pulls for non-payment or a compliance hold.
- **Branch status** is the operational state of **one location**. A branch can be
  suspended for a seasonal close or a fit-out while its organization is entirely
  healthy and its other branches keep trading.

They therefore get **distinct error codes** (§3), and the organization is checked
first because it is the broader statement about the tenant.

**Table status is not a third lifecycle gate and is deliberately excluded.**
`sqlc.TableStatus` is `available | occupied | reserved` — an **occupancy** enum
describing whether a table is in use, not whether it is permitted to serve. Occupancy
is already enforced by the one-active-session-per-table constraint, and reading
`occupied` as a suspension would break every join, since joining an occupied table is
the normal case. The naming collision with the lifecycle columns is a trap; this ADR
exists partly to name it.

---

## 3. 403, not 503 and not 404

A suspended tenant answers **403** with `ORGANIZATION_SUSPENDED` or `BRANCH_SUSPENDED`
(`handlers/errors.go`).

- **Not 503.** 503 says *"try again shortly"* — it implies a transient, self-healing
  outage. Suspension is a durable policy decision that persists until an operator
  reverses it, so 503 would be a lie to the client and, more expensively, would make
  a deliberate business action indistinguishable from an availability incident in
  alerting and uptime reporting. Suspending a tenant must not page anyone.
- **Not 404.** The table exists and the guest is standing at it. Telling them the QR
  code is unknown sends them to look for a different code, or to staff who cannot log
  in either. `TABLE_NOT_FOUND` was what QR resolve returned before this change — for
  every error, because the handler collapsed them all.
- **403** is the accurate statement: the request is well-formed and understood, and
  the server refuses to authorize it. Two distinct codes so a guest client can say
  *"this restaurant isn't serving right now"* rather than showing a generic failure.

Both messages are written for a guest, not an operator, and neither discloses the
reason for the suspension.

---

## 4. Entry-only scope: suspension is not an emergency stop

Invariant T1: *a suspended organization accepts no new sessions and no new joins;
sessions already in progress finish normally.*

Suspension is a billing and compliance action. A restaurant is suspended for an unpaid
invoice, not because its food is dangerous. Cutting off a party mid-meal — cart frozen,
bill unreachable, no way to pay and leave — creates a worse and more immediate problem
than the one suspension is solving, and it lands on guests who have no relationship
with the billing dispute.

So the gate is called from entry points only, and `JoinSession` refuses a **new**
device even though the session it would join keeps running for the people already in
it. That looks inconsistent at a glance and is the intended reading: joining is entry,
not continuation.

This is enforced by test, not just by convention —
`TestSuspendedOrganization_LiveSessionKeepsOrdering` opens a session, suspends the
organization, and asserts the party can still read the session and place an order.

If a true emergency stop is ever needed (a food-safety incident, a compromised
tenant), it must be a **separate** mechanism with its own status and its own explicit
decision about stranded sessions. Do not overload suspension to get it.

---

## 5. Short-TTL cache plus explicit invalidation

Every QR scan, create and join consults the gate, so the uncached path (two row reads:
branch, then its organization) must not run per request.

The gate follows the pattern already established by `services/feature_gate.go`:

- Redis key `tenantstatus:branch:<id>`, TTL **60s** (`tenantStatusCacheTTL`).
- `TenantStatusGate.Invalidate` flushes `tenantstatus:*` and is called by all four
  lifecycle endpoints — organization suspend/activate and branch suspend/activate
  (`handlers/platform_lifecycle.go`).

**Why a full pattern flush rather than targeted eviction.** An organization-level flip
fans out to every branch that organization owns, and those branch ids are not known at
the call site. The `tenantstatus` keyspace is small and operator lifecycle mutations
are rare and low-volume, so flushing it is both correct and simpler than maintaining a
reverse index. Same reasoning as `FeatureGate.Invalidate`.

**What the TTL is actually for.** Because every mutation invalidates, suspension and
re-activation both take effect immediately on the instance handling the request. The
TTL is the multi-instance backstop: it bounds how long any *other* instance can serve
a stale decision if a flush is missed. It is not the primary propagation mechanism.

**The gate never fails open.** A lookup error is returned to the caller rather than
swallowed into "admit". A nil cache is supported (uncached, straight to the database)
so tests and any non-Redis deployment degrade to correct-but-slower, never to
permissive.

---

## 6. Alternatives rejected

- **Enforce in `TenantMiddleware`.** It already resolves the organization, so it looks
  like the natural home. Rejected: it is a **complete no-op** in every current
  deployment because it returns early when `BASE_DOMAIN` is empty, which is the
  default (`config.go`). Putting the only guest-facing suspension check behind an
  unset environment variable would have produced a gate that reads as enforced and
  enforces nothing.
- **A database-level check (view or trigger on `sessions`).** Would catch inserts but
  could not distinguish entry from continuation, could not produce a guest-facing
  error code, and would not cover QR resolve — which is the call a guest makes
  *before* any row is written, and the right place to tell them.
- **Reuse `ErrParticipantUnauthorized`** (what staff login returns for the same
  condition). Rejected: it maps to a generic 403 `FORBIDDEN`, which is exactly the
  undistinguished error this change exists to replace.

---

## 7. Open gap — staff remain locked out at login

**This ADR does not close the asymmetry it was written about; it closes one half.**

`services/staff.go` `createSession` still returns `ErrParticipantUnauthorized` when
the branch or organization is not `active`, so **no staff member can log in to a
suspended tenant**. An existing staff session keeps working — the check is at login
only, and `staffTokenTTL` is 8 hours — so a suspension mid-shift does not eject anyone
already working.

The failure mode is a **shift boundary during a suspension**:

1. An organization is suspended while a party is mid-meal (allowed to continue, §4).
2. The staff on shift log out, or their 8-hour token expires.
3. The incoming shift cannot authenticate.
4. The in-progress session is still live and still ordering — with nobody able to see
   the kitchen display, confirm orders, or settle the bill.

The session survives, which was the goal, but it becomes unserveable. §4's promise
that a party "finishes normally" depends on staff being present to serve them, and
that is not currently guaranteed.

**Deliberately not fixed here.** Changing staff login behaviour is a separate decision
with its own blast radius, and the resolution is not obvious. At least three options:

- Let staff authenticate but restrict them to sessions that predate the suspension.
- Let staff authenticate for a bounded drain window after suspension.
- Keep the lockout and require platform operators to close or migrate open sessions
  *before* suspending — making suspension a two-step operator workflow rather than a
  single status flip.

The third is the only one that needs no code change but the most operator discipline.
Whichever is chosen should be recorded as a superseding ADR.

---

## 8. Relationship to existing records

No conflict with [ADR 0002](0002-r3-policy-decisions.md). That record governs the
**central authorizer** (`AUTHZ_CENTRAL_POLICY_ENFORCE`) — role and scope policy for
*staff and platform* actors. This gate is a tenant-lifecycle check on *guest* entry
and runs in the service layer, below and independent of the authorizer. The two never
evaluate the same condition, and this gate is not behind a rollout flag: it is
unconditional, because an inert suspension was the defect.

---

## 9. Verification

- `services/tenant_status_integration_test.go` — suspended organization blocks create,
  join and QR resolve; suspended branch blocks entry with its own distinct error;
  re-activation restores all three; a live session keeps ordering throughout; the
  cache serves within TTL and `Invalidate` makes a flip visible at once.
- `e2e/tenancy/T-03-org-suspension.spec.ts` — flipped from failing to passing without
  being modified.
