# Frontend Production Polish + Contract Alignment Plan — v1

## Context

The backend workflow-stabilization phase is complete and its contracts are authoritative
(`docs/frontend-contract-stabilization.md`, `docs/websocket-events.md`). This phase aligns the
**frontend** to those now-stable contracts and brings it to production feel — correctness, realtime
UX, operational clarity, responsive ergonomics, visual consistency. **No backend redesign**, no
rollout/architecture work. Giant rewrites are explicitly out — work is sequenced and incremental.

Primary inputs: `manual-testing-findings-v1.md`, `docs/frontend-contract-stabilization.md`, the
existing frontend (`/frontend`, Next.js 15 app-router, React 19, Zustand, Tailwind v4, base-ui/shadcn,
framer-motion), and the **Maison Saffron** HTML redesign as *design/UX direction only* (warm
cream/saffron/terracotta palette, Cormorant Garamond display + Inter body + JetBrains Mono;
operational correctness outranks static visual fidelity).

### How the audit was done

Three parallel code audits of the frontend mapped: (a) the realtime/session/contract core
(`lib/ws/*`, `store/*`, `hooks/useWebSocket.ts`, `providers/SessionProvider.tsx`, `types/ws.ts`),
(b) the guest commerce flows (payment, menu/modifiers, cart, orders, promo), and (c) the staff
operational screens + routing + responsive/visual surface.

### Headline result

The frontend is **structurally solid** (clean store/hook/api separation, a real WS layer with
ticketed connect + reconnect + snapshot reconcile, a coherent design system). The gaps are
**targeted contract drift and operational UX**, not architecture — so this is a polish pass, not a
rebuild. Two findings from the manual report are already correct in code (multi-join routing; promo
at order placement) and need only a manual re-walkthrough, not changes.

**One blocking dependency surfaced:** the waiter-settlement workflow the backend enabled
(`PATCH /payments/:id/settle`, now waiter-allowed) is **not usable from the FE** because there is no
endpoint to *list* pending payments for a branch (`ListPaymentsForSession` is per-session only; no
`branchStaffAPI` payments route). This is the single place this phase must touch the backend — a
small **additive read endpoint**, mirroring the existing `GET /branches/:id/orders/active`. Flagged
in §3, §5 (Phase 2), and §9.

---

## 1. Executive Summary

- **Current FE maturity:** good bones. App-router structure, Zustand stores, ticketed WebSocket with
  reconnect + snapshot reconciliation, per-domain API clients, a consistent hospitality design
  system, and mostly-correct guest routing/persistence. Production feel is achievable by alignment
  and polish, not rewrite.
- **Major remaining weaknesses:**
  1. **Contract drift** — payment optimistic UI flips to "paid" before staff confirmation; the new
     `MENU_ITEM_AVAILABILITY_CHANGED` event is unhandled; `snapshot_authoritative` is ignored; the
     "still ordering" action calls *waiter-assist* instead of the new reactivation endpoint.
  2. **Operational UX** — kitchen offers a "served" action it's now forbidden to use, and renders
     **no item detail** despite the enriched payload; the waiter has **no settlement UI** at all.
  3. **Commerce UX** — modifier groups allow multi-select (should be single-select per group); cart
     has no quantity editing.
  4. **Responsive/visual** — kitchen kanban is a fixed 900px desktop layout with sub-44px tap
     targets; image placement is inconsistent (menu right, cart placeholder-left, featured top);
     reconnect/loading/error states are inconsistent and absent on staff screens.
- **Alignment with stabilized backend contracts:** **partial.** Routing (multi-join) and promo
  placement already conform. Reactivation, payment lifecycle, menu-availability realtime, kitchen
  role/payload, and waiter settlement do **not** yet conform — these are the core of this phase.

## 2. Issue Classification Matrix

Dimensions: **Sev** (P0 blocking pilot / P1 high / P2 polish) · **Type** · **Workflow** · **UX** ·
**Realtime** · **Mobile**. All issues are FE-owned unless marked *(needs additive BE read endpoint)*.

| ID | Issue | Sev | Type | Workflow | UX | Realtime | Mobile |
|----|-------|-----|------|----------|----|----------|--------|
| FC-4 | "Still ordering" calls assist, not `POST /sessions/:id/reactivate`; `reactivate()` missing from API | **P0** | stale contract + realtime | High (guest blocked, 409 on next order) | High | High | — |
| FC-6 | Kitchen UI offers `ready→served` (now 403) | **P0** | stale contract / operational | High (confusing failure) | High | — | — |
| FC-7 | Kitchen renders no items/modifiers/notes despite enriched payload | **P0** | stale contract / operational | Critical (kitchen blind) | High | — | Med |
| FC-8 | Waiter has no payment-settlement UI; `paymentsApi.settle` missing *(needs additive BE list endpoint)* | **P0** | operational + contract | Critical (payment stuck `payment_pending`) | High | Med | Med |
| FC-1 | Payment shows "paid" immediately for cash/card (should await confirmation) | **P1** | optimistic UI | High (false "paid") | High | High | — |
| FC-2 | `MENU_ITEM_AVAILABILITY_CHANGED` unhandled; cart not reconciled | **P1** | realtime / state sync | Med | Med | High | — |
| FC-3 | `snapshot_authoritative` ignored on reconcile | **P1** | state sync | Med (stale local state) | Med | High | — |
| FC-5 | No friendly handling of `SESSION_CLOSED`/`PAYMENT_IN_PROGRESS`/`MENU_ITEM_UNAVAILABLE` | **P1** | stale contract | Med | Med | Med | — |
| CX-1 | Modifier groups allow multi-select (should be single-select per group) | **P1** | operational UX | Med (wrong order data) | Med | — | Med |
| OP-2 | No per-page role gating (kitchen can open /admin) | **P1** | routing/navigation | Med | Med | — | — |
| RM-1 | Kitchen kanban fixed 900px, unresponsive; ~36px tap targets | **P1** | layout/responsive | Med (tablet/phone unusable) | Med | — | High |
| OP-3 | Reconnect/loading/error states inconsistent; no reconnect banner on staff | **P1** | realtime UX | Med | High | High | Med |
| CX-2 | Cart has no quantity inc/dec (remove-only) | **P2** | operational UX | Low | Med | — | Med |
| VC-1 | Image layout inconsistent; cart shows placeholders, never real images | **P2** | visual consistency | Low | Med | — | Med |
| RM-2 | PIN input alignment + narrow-screen overflow on login | **P2** | layout/responsive | Low | Low | — | Med |
| OP-1 | Kitchen/waiter are 10s polling (no realtime) | **P2** | realtime (BE-limited) | Med | Med | High | — |
| VC-2 | Mixed inline-style vs component styling | **P2** | visual consistency | Low | Low | — | Low |
| — | Multi-join routing | — | verified CORRECT | — | — | — | — |
| — | Promo at order placement | — | verified COMPLIANT | — | — | — | — |

## 3. Contract Alignment Audit

**Stale FE assumptions (now contradicted by backend):**
- **Reactivation** — `components/shared/SessionTimeoutBanner.tsx` "still ordering" calls
  `assistanceApi.request(... "waiter")`; `lib/api/sessions.ts` has no `reactivate()`. The session
  stays `awaiting_reactivation` locally and the next order 409s. Must call
  `POST /sessions/:id/reactivate` and apply the returned session.
- **Kitchen role** — `app/(staff)/staff/(dashboard)/kitchen/page.tsx` `NEXT_STATUS`/`NEXT_LABEL`
  include `ready→served`. Backend now 403s kitchen on serve.
- **Kitchen payload** — kitchen `Order` type / `lib/api/staff.ts|orders.ts` don't carry `items`;
  the card renders only number + elapsed time though the payload now includes
  `items[{menu_item_id,name,quantity,modifiers[],note}]`.

**Incorrect optimistic UI:**
- `app/(guest)/session/[id]/payment/page.tsx:~86` calls `setPaid(method)` immediately regardless of
  `payment.status`. Cash/card are `requires_staff_confirmation` and must not read as "paid". The
  `PAYMENT_COMPLETED` handler already writes `useSessionStore.completedPayment`
  (`hooks/useWebSocket.ts`), but the payment page never reads it — so it never reconciles to truth.

**Missing realtime reconciliation:**
- `MENU_ITEM_AVAILABILITY_CHANGED` is absent from `types/ws.ts` and `hooks/useWebSocket.ts`.
  `store/menu.ts` already has an unused `markItemUnavailable()`; cart reconciliation doesn't exist.
- `lib/ws/connection.ts` reconcile path (~line 136) calls `reconcileSnapshot()` without honoring
  `snapshot_authoritative` (wholesale-replace signal).

**Unsafe / missing error handling:**
- `app/(guest)/session/[id]/cart/page.tsx` + `lib/api/client.ts` map promo errors but fall through
  to a generic message for `SESSION_CLOSED` / `PAYMENT_IN_PROGRESS` / `MENU_ITEM_UNAVAILABLE`.

**Local state duplication risk:** payment page keeps a local `paid` flag divorced from the
authoritative `completedPayment` in the session store (the root of FC-1).

**Backend dependency (the one additive exception):** waiter settlement needs
`GET /branches/:id/payments?status=requires_staff_confirmation` (branch-staff), mirroring
`GET /branches/:id/orders/active`. Additive read, no semantic change.

**Verified already-correct (no change, re-walkthrough only):** multi-join
(`app/(guest)/table/[token]/page.tsx` joins when `session_id` present, creates otherwise — no
redirect-home bug); promo entered/validated in the cart at order placement.

## 4. Frontend Stabilization Buckets

- **Bucket A — Contract alignment (correctness):** FC-4, FC-1, FC-2, FC-3, FC-5, FC-6.
- **Bucket B — Operational workflow UX (staff):** FC-7 (kitchen ticket detail), FC-8 (waiter
  settlement + its BE list endpoint), OP-2 (role gating).
- **Bucket C — Commerce UX (guest):** CX-1 (modifier single-select), CX-2 (cart quantity).
- **Bucket D — Responsive/mobile:** RM-1 (kitchen mobile + tap targets), RM-2 (PIN/login).
- **Bucket E — Visual consistency:** VC-1 (image-left across menu/cart/featured, real cart images),
  VC-2 (style consolidation — opportunistic, bundled with screens already being touched).
- **Bucket F — Realtime UX + state-consistency polish:** OP-3 (reconnect/loading/error consistency,
  reconnect banner on staff), OP-1 (perceived latency / polling cadence; note WS limitation).

## 5. Recommended Implementation Order

No giant rewrites — each step is small, screen-scoped, and independently shippable. Correctness and
trust first, then operational workflows, then commerce, then responsive, then visual, then polish.

**Phase 0 — Verify & unblock (no UI yet)**
- Confirm the kitchen active-orders API client surfaces the new `items` (extend FE type + mapping).
- Add the additive backend read endpoint for branch pending-settlement payments (the one BE
  exception). Without it, FC-8 is not buildable. Treat as a prerequisite for Phase 2's waiter work.

**Phase 1 — Bucket A: contract alignment** (small, low-risk, high-trust; do these first)
1. FC-4 reactivation: add `sessionsApi.reactivate()`; wire `SessionTimeoutBanner` to call it and
   apply the returned session (clear `isReactivating`, restore `active`) before re-enabling ordering.
2. FC-6 remove kitchen `served` action (drop `ready→served` from `NEXT_STATUS`/`NEXT_LABEL`).
3. FC-1 payment UI: gate on status — cash/card show "awaiting staff confirmation"; flip to "paid"
   only on `PAYMENT_COMPLETED` (read `useSessionStore.completedPayment`) or `status==="completed"`.
4. FC-2 menu availability: add the event type + handler → `markItemUnavailable()` + cart reconcile;
   never treat as connection error.
5. FC-3 honor `snapshot_authoritative` (wholesale replace) in the reconcile path.
6. FC-5 friendly messages for `SESSION_CLOSED`/`PAYMENT_IN_PROGRESS`/`MENU_ITEM_UNAVAILABLE`.

**Phase 2 — Bucket B: operational workflow UX**
7. FC-7 kitchen ticket detail: render items, quantity, modifiers, notes per order.
8. FC-8 waiter settlement: `paymentsApi.settle()` + a pending-payments queue with "confirm
   cash/card collected" (depends on Phase 0 endpoint).
9. OP-2 per-page role gating (redirect/deny on unauthorized role).

**Phase 3 — Bucket C: commerce UX**
10. CX-1 single-select per `modifier_group` (beverage grid + non-beverage list, grouped).
11. CX-2 cart quantity inc/dec (re-add semantics if no cart-item PATCH exists; verify in Phase 0).

**Phase 4 — Bucket D: responsive/mobile**
12. RM-1 kitchen responsive layout (stack/list under `md`, ≥44px tap targets).
13. RM-2 PIN/login alignment + narrow-screen sizing.

**Phase 5 — Bucket E: visual consistency**
14. VC-1 consistent image-left across menu/cart/featured; render real images in cart.
15. VC-2 consolidate inline styles into components on screens already touched above.

**Phase 6 — Bucket F: realtime UX + polish**
16. OP-3 shared loading/error patterns; reconnect banner on staff screens.
17. OP-1 reduce perceived latency (optimistic-local on staff actions, sensible polling cadence);
    document the session-scoped-WS limitation for staff.

Rationale: Bucket A removes user-visible falsehoods (false "paid", dead reactivation, stale menu)
with tiny diffs; Bucket B fixes the two unusable operational screens; the rest is incremental UX/
visual polish that can't regress correctness.

## 6. Realtime UX Audit

- **WebSocket timing perception:** guest realtime is genuine (ticketed WS, event handlers for
  session/participant/cart/order/assistance/payment). Gaps: `MENU_ITEM_AVAILABILITY_CHANGED` and
  `snapshot_authoritative` unhandled; `PAYMENT_INITIATED`/`SESSION_CREATED` ignored (informational —
  low priority).
- **Reconnect UX:** `lib/ws/connection.ts` does exponential backoff (max ~30s, ~10 attempts) and a
  `ReconnectingBanner` exists — but **only on guest screens**. Staff screens (kitchen/waiter/admin)
  poll every ~10s and fail **silently**, so staff get no "connection lost" signal. Add the banner +
  consistent error surfacing to staff (OP-3).
- **Loading states:** ad hoc — kitchen uses a skeleton, waiter a spinner, admin per-tab spinners,
  guest mixed. Standardize on shared loading components.
- **Session recovery:** reload path is sound (`sessionStorage` holds session/participant/token; the
  guest layout guards and `SessionProvider` fetches the snapshot). The missing piece is
  `awaiting_reactivation` recovery via the explicit endpoint (FC-4).
- **Propagation ergonomics:** staff screens are **not** on the realtime bus (session-scoped fanout
  has no branch channel), so kitchen/waiter see ~10s lag. True staff realtime needs a backend
  branch-level channel — **out of scope** here; mitigate with optimistic-local updates + a tighter
  cadence and flag for a later backend phase (OP-1, §9).

## 7. Mobile / Responsive Audit

- **Guest:** generally mobile-first (bottom sheets, single-column). Watch tap-target sizes and the
  menu/cart image inconsistency (VC-1).
- **Kitchen (worst offender):** fixed `minWidth: 900` 4-column kanban with ~36px buttons — unusable
  on phones and awkward on tablets. Needs a responsive collapse (list/stacked under `md`) and
  ≥44px targets (RM-1). Update `KitchenSkeleton` to match.
- **Waiter:** good responsive grid (`grid-cols-1 md:grid-cols-[1.1fr_1fr]`); minor overflow handling
  for long table/status text. The settlement queue (FC-8) must be designed mobile-first.
- **Admin:** acceptable (`overflow-x-auto` tabs, stacking cards).
- **Login:** 6×48px PIN boxes can overflow <384px and don't baseline-align with the full-width
  button (RM-2).
- **Keyboard:** PIN uses a hidden input overlay — verify numeric keypad (`inputmode="numeric"`) and
  focus behavior on mobile.

## 8. Visual Consistency Audit

- **Menu layout:** images are **right/circular** in menu rows, **top** in the featured carousel, and
  the cart shows **left placeholders with no real image**. Target: consistent **image-left** with
  real images in the cart (VC-1).
- **Typography/spacing/color:** the design system is consistent and good (eyebrow/serif/display/body/
  mono scale; `--ok/--warn/--alert/--info/--accent` tokens; hospitality theme). Align toward the
  Maison Saffron direction (Cormorant Garamond display, Inter body, warm saffron/terracotta) where it
  doesn't fight operational clarity.
- **Styling approach:** mixed inline styles vs shared components (kitchen card inline vs
  `HospitalityCard`; login underline inputs vs admin `Input`). Consolidate opportunistically on
  touched screens (VC-2) — not a standalone refactor.
- **Operational clarity:** kitchen tickets must read at a glance (item, qty, modifiers, allergen
  notes prominent) — the FC-7 detail rendering is as much visual hierarchy as data.

## 9. Remaining Product Risks

- **Waiter settlement is blocked without the additive branch-payments read endpoint** (Phase 0). If
  it's not added, the cash/card workflow the backend enabled stays unreachable from the UI and
  sessions can hang in `payment_pending`. Highest-leverage risk to clear early.
- **Staff screens are not truly realtime** (session-scoped WS, no branch channel). Kitchen/waiter lag
  ~10s. Acceptable for a small pilot with optimistic-local updates, but a real backend branch-channel
  is a likely follow-up phase — call it out before pilot.
- **Modifier single-select is a product decision** — the backend can't distinguish single vs multi
  groups. Interim FE convention (single-select per group) may misclassify a genuinely multi-select
  group; confirm the rule with the operator and revisit if a backend `selection_type` lands.
- **Cart quantity editing** may require remove+re-add if there's no cart-item update endpoint —
  verify in Phase 0; if re-add is used, ensure idempotency/cart-merge behaves.
- **Re-walkthrough after Phase 1–2** — multi-join and reactivation especially should get a fresh
  manual multi-device walkthrough, since the original findings predate these fixes and the code paths
  changed.

---

### Execution rules for this phase
Small, atomic, human-readable commits; no co-author/AI attribution; no noisy formatting-only commits
(bundle style consolidation with the screen being changed); keep the working tree clean. Verify each
screen in a browser (golden path + reconnect/error) before marking done. This planning document
stays **uncommitted** unless instructed otherwise.
