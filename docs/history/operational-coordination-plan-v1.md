# Operational Coordination + Collaborative Session Semantics — Plan v1

## Context

The system is structurally stable (auth, lifecycle, WebSocket architecture, payment lifecycle,
frontend/backend contracts). The remaining gaps are **operational coordination** and **collaborative
dining semantics**, not architecture. This document is an analysis + design pass; it changes no code.

The manual testing findings (`manual-testing-findings-v1.md`, `-v2.md`) were verified against the
current code and runtime flow rather than taken at face value. One of their central assumptions —
"waiter realtime works for assist but not for ready/payment" — turned out to be misleading, and that
correction reframes the whole operational diagnosis below.

**Confirmed product decisions:** (1) host-controlled, shared-cart ordering; (2) optional phone number
at join; (3) staff realtime via a polling-coverage fix this phase, deferring a branch-scoped WS
channel to a later phase.

---

## 1. Executive Summary

- **Operationally stable now:** guest realtime reactivity (ticketed WS, reconnect + exponential
  backoff, snapshot reconcile honoring `snapshot_authoritative`), session lifecycle, payment
  lifecycle, the guest→kitchen order flow, and the host *data model* — which already exists in the
  schema and is simply unused.
- **Still operationally weak:**
  - The **waiter has no operational surface** beyond assist requests. Ready-to-serve and
    payment-confirmation are invisible to them.
  - **Collaborative ordering has no ownership** — any participant can independently submit orders,
    producing duplicates and chaos.
  - **Participant identity is name-only**, and the count appears stuck at `1`.
- **Central diagnosis (corrects the manual findings):** the WebSocket hub is **session-scoped only**
  (`backend/internal/websocket/hub.go:21`; the WS upgrade requires guest session auth in
  `backend/internal/handlers/ws.go`). **Staff have no realtime channel at all** — kitchen and waiter
  run entirely on **~10s REST polling**. "Assist reaches the waiter" is *not* evidence that waiter
  realtime works; it works only because the waiter page happens to poll the assist endpoint. The
  governing rule is simply: **what the waiter page polls, the waiter sees; what it does not poll is
  invisible.**

## 2. Operational Event Flow Audit

WebSocket events are emitted to the **guest session room** — which is correct, because guests are the
ones who need them. Staff operate by polling branch-level REST endpoints. Mapping each operational
flow against emission and polling:

| Flow | Event emitted | Waiter poll endpoint | Endpoint exists? | Waiter page polls it? | Status |
|------|---------------|----------------------|------------------|-----------------------|--------|
| **Assist** | `ASSISTANCE_REQUESTED` → session (`services/assistance.go:32`) | `GET /branches/:id/assist/active` (`server.go:257`) | yes | **yes** (`waiter/page.tsx:130`) | ✅ works |
| **Ready-to-serve** | `ORDER_READY` → session (`services/order.go:498`) | `GET /branches/:id/orders/active` (`server.go:255`; query **includes `ready`**) | yes | **no** | ❌ invisible |
| **Payment request** | `PAYMENT_INITIATED` → session (`handlers/payment.go:220`) | `GET /branches/:id/payments` (`server.go:258`, `ListPendingForBranch`) | **yes (already added)** | **no** | ❌ invisible |

### Guest flow
Guest order-status reactivity is genuine and sound: `ORDER_PLACED/CONFIRMED/PREPARING/READY/SERVED`
each update the Zustand orders store, and the orders UI reads it reactively
(`hooks/useWebSocket.ts:79-107`, `store/orders.ts`). Initial state and reconnect are reconciled from
the session snapshot.

### Kitchen flow
Kitchen polls `/orders/active` (~10s) and advances `pending → confirmed → preparing → ready`. It is
correctly 403'd from `ready → served` per the locked contract. Consequence: once an order is `ready`,
the kitchen can no longer act on it — and **no other surface picks it up**.

### Waiter flow
The waiter page (`frontend/app/(staff)/staff/(dashboard)/waiter/page.tsx`) polls **only**
`assistanceApi.getActive` (lines 127–143) and renders an assist queue plus a hardcoded floor
overview. It does **not** poll orders or payments and has **no serve or settle UI**.

### Payment / serve / assist / reconnect
- **Serve** has no operational home: kitchen can't (403), waiter doesn't see ready orders.
- **Payment**: the branch payments endpoint **already exists** (`server.go:258`) — the
  frontend-polish plan's "missing endpoint" note is **outdated**. There is no
  `paymentsApi.listPendingForBranch` client method and no settlement UI, so guest cash/card requests
  sit in `requires_staff_confirmation` unseen.
- **Assist** works end-to-end purely via polling.
- **Reconnect** is robust on the guest side (ticketed reconnect, backoff, snapshot reconcile honoring
  `snapshot_authoritative` at `lib/ws/connection.ts:136`).

### Identified gaps
- **Missing transitions:** `ready → served` and payment settlement have no usable operator path.
- **Missing notifications:** none on the staff side are realtime; coverage gaps (not emission bugs)
  hide ready orders and pending payments.
- **Stale propagation:** `served` is **doubly delayed** — it depends on a waiter action the waiter
  currently can't see; there is **no guest polling fallback** if WS degrades;
  `PAYMENT_INITIATED` is received but ignored on the guest (`useWebSocket.ts` has no handler) — minor.
- **FE filtering / count:** `PARTICIPANT_JOINED` *is* handled (`useWebSocket.ts:58`) and
  `addParticipant` appends correctly (`store/session.ts:54`). The "stuck at 1" is most likely a
  join-path race — `setSession` resets `participants` to `[self]` (`store/session.ts:34-41`) and may
  run after the snapshot populated the full list — and/or the count UI not reading
  `participants.length`. Small fix, no schema change; verify during implementation.

## 3. Collaborative Ordering Models

**Current state.** Carts are **per-participant** (`migrations/000001:184-189`,
`UNIQUE(session_id, participant_id)`). Order submission validates only that the participant belongs to
the session, **not** that they are the host (`services/order.go:76-82`). Orders already record
`placed_by_participant_id` (`migrations/000001:222`). `CART_UPDATED` already broadcasts to the whole
session (`events/events.go:100`).

| Model | Operational simplicity | Realtime complexity | UX clarity | Restaurant realism | Abuse resistance |
|-------|------------------------|---------------------|------------|--------------------|------------------|
| **Host-controlled, shared cart (chosen)** | High (one cart, one bill) | Low (`CART_UPDATED` already broadcasts) | High (clear authority) | High (one table → one order) | High (single submit point) |
| Hybrid (per-person orders) | Med | Low | Med | Med (everyone-pays-own) | Med |
| Fully collaborative (current) | Low | Low | Low | Low | Low (duplicate/accidental orders) |

**Recommendation (confirmed): host-controlled, shared cart.** The first participant is the host —
`is_host=true` is already set at `services/session.go:90` and `sessions.host_participant_id` already
exists. All participants edit one **session-level shared cart**; only the host submits to the kitchen
and requests the bill. This is the only model in which "everyone collaborates on one cart, one person
sends it" is coherent, and it requires moving the cart from per-participant to a single session cart
(the schema already permits `participant_id NULL`).

**Required robustness rule — host reassignment.** If the host disconnects or leaves, promote the
oldest active participant to host and broadcast the change, so ordering never deadlocks on one device.

## 4. Participant Identity Recommendations

- **Name:** required (current `display_name`, `handlers/session.go:38`). Keep.
- **Phone:** **optional**, with an explicit "continue without phone" path. Add a nullable
  `phone_e164` column to `session_participants` (additive migration). Phone is collected today only at
  order time, not on the participant. This improves waiter/kitchen identification ("Aanya · host"
  beats "Guest 1"), enables future loyalty/CRM, and preserves low-friction onboarding.
- **Visibility:** show the participant list with a host badge on the guest session screen
  (`app/(guest)/session/[id]/page.tsx`, which already receives `is_host`), and surface participant
  names on kitchen tickets and waiter cards for coordination.
- **Future extensibility:** an optional phone is the natural anchor for later loyalty, receipts, and
  CRM without forcing accounts now.

## 5. Recommended Operational Workflow Semantics

- **Order submission → host only.** Non-hosts add/edit the shared cart; the "send to kitchen" CTA is
  host-only, gated on the frontend and enforced by a backend authorization check in `PlaceOrder`.
- **Bill request → host only**, same authority model.
- **Mark served → waiter / manager / owner only** (already enforced; kitchen is 403'd). The waiter
  needs the UI to actually perform it.
- **Waiter notifications → polling-coverage fix.** The waiter page gains two additional polled queues
  alongside assist — ready-to-serve (with a **Serve** action) and pending payments (with a **Confirm
  cash/card collected** action) — on a single, tightened poll loop.
- **Collaborative editing → shared cart**, broadcast via the existing `CART_UPDATED` event; reconnect
  rehydrates from the session snapshot (cart is already part of snapshot reconciliation).

## 6. Realtime Coordination Risks

- **Staff are not truly realtime (accepted for now).** Polling yields ~10s latency. The chosen
  polling-coverage fix closes the *coverage* gap (ready + payments) without touching WS infra; true
  staff push (a branch-scoped WS channel) is a flagged later phase.
- **Host single-point-of-control.** Mitigated by host reassignment (§3). Without it, a dead host phone
  blocks the entire table.
- **Cart topology migration is the riskiest change.** Switching per-participant → shared session cart
  touches `GetCart`/`AddToCart` and the cart sync path; isolate and verify it before layering host
  gating on top.
- **Guest WS degradation has no fallback.** A low-frequency guest poll fallback would be cheap
  insurance (optional, low priority).

## 7. Recommended Implementation Order

Sequenced to ship value early and isolate the one risky change. Each step is small and independently
verifiable. **This is for the next phase — not this document.**

1. **Waiter operational surface (FE-only, highest leverage, zero backend work).** Add a ready-to-serve
   queue (poll `/orders/active`, filter `ready`, **Serve** via the existing `ready → served`) and a
   pending-payments queue (poll `/payments`, add `paymentsApi.listPendingForBranch` + **Confirm
   collected** via the existing `PATCH /payments/:id/settle`). Both backend endpoints already exist.
   This alone unblocks the two stuck P0 flows.
2. **Participant count + identity surfacing (FE-only).** Fix the join-path race / count display;
   render the participant list with a host badge; show names on staff views.
3. **Optional phone (additive migration + join form).** Nullable `phone_e164` on participants; an
   optional input with a "continue without phone" path.
4. **Shared session cart (backend cart topology).** Move the cart to session-scoped
   (`participant_id NULL`); keep the `CART_UPDATED` broadcast. Isolated and verified before step 5.
5. **Host-gated submission + bill request (backend auth + FE gating).** Host-only "send to kitchen"
   and bill request; host badge/affordances; **host reassignment** on host departure.
6. **Polish.** Handle `PAYMENT_INITIATED` (or drop it intentionally); optional guest WS poll fallback.

Steps 1–3 are low-risk and shippable immediately; steps 4–5 carry the deliberate workflow change and
are sequenced to avoid a giant rewrite.

## 8. Stability Assessment — risks remaining before pilot

After this planning pass and the sequenced implementation above:

- **Highest residual risk:** the shared-cart migration (step 4) — concurrent edits and reconnect
  rehydration must be verified across multiple devices.
- **Host reassignment** must be correct, or a dead host can strand a table.
- **Staff latency (~10s polling)** is acceptable for a small pilot but should be revisited (a
  branch-scoped WS channel) before scale.
- **Multi-join + reactivation** need a fresh multi-device manual walkthrough — the original findings
  predate the fixes.
- Everything else (auth, lifecycle, payment lifecycle, guest realtime) is pilot-grade.

## Verification (for the implementation phase)

- Multi-device manual walkthrough: two guests join one table → host badge correct, count = 2, shared
  cart edits sync both ways, only the host can send to kitchen.
- Waiter: place order → kitchen marks ready → waiter sees it and serves → guest sees `served`; guest
  requests cash payment → waiter sees the pending payment → confirms → guest sees `completed`.
- Host disconnect → reassignment promotes the next participant; ordering still works.
- Run backend tests and a clean build after the additive migration.
