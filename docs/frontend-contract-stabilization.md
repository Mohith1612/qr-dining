# Frontend Contract Stabilization

Contracts the backend guarantees as of the workflow-stabilization pass, written for
the upcoming frontend production-polish phase. Each section states the backend
behavior the frontend must build against. Items marked **FE-deferred** have no
backend change in this pass — the contract is documented so the frontend can be
polished without future backend thrashing.

---

## 1. Cash / Card payment lifecycle (finding #6)

The backend has **never** auto-settled cash or card. The "immediate paid" the manual
test observed was optimistic frontend UI. Authoritative lifecycle:

1. **Guest requests payment** — `POST /sessions/:id/payments` with `method` in
   `cash | card | upi | digital | card_manual`. The session moves to
   `payment_pending` (cart/orders frozen).
   - `cash` and `card` are normalized to status **`requires_staff_confirmation`**.
     (`card` is stored as `card_manual`.) The payment is **not** complete yet.
   - `upi` / `digital` move to `provider_pending` and complete via provider webhook.
2. **Waiter collects physically**, then confirms: `PATCH /payments/:id/settle`.
   - As of this pass, **waiter, manager, and owner** may settle (previously
     owner/manager only).
   - Settle transitions `requires_staff_confirmation → completed`.
3. **Payment complete** — only after step 2 (cash/card) or webhook (upi/digital).

**FE contract:** for cash/card, show "awaiting staff confirmation" after step 1, and
flip to "paid" only on the `PAYMENT_COMPLETED` event or a payment whose `status` is
`completed`. Do **not** show "paid" at initiation. **FE-deferred** (rendering only).

**Error contract (fixed this pass):** initiating a payment on a session that is no
longer active now returns `404 SESSION_NOT_FOUND` or `409 SESSION_CLOSED`, never a
500. Treat these as "this session can no longer take a payment."

---

## 2. Modifier selection (finding #10) — **FE-deferred + future schema note**

Backend schema today: `item_modifiers(id, item_id, name, price_delta, is_required,
modifier_group)`. There is **no** field expressing single- vs multi-select. The
backend stores whatever `modifier_ids` the order sends and snapshots them onto the
order item — it does not enforce select counts.

**FE contract (interim):** group an item's modifiers by `modifier_group`. The
frontend owns single-select-per-group behavior. Without a backend flag the frontend
cannot distinguish a single-select group ("Size") from a multi-select group ("Extra
toppings"), so the interim convention is: treat a group as single-select unless a
product decision says otherwise.

**Recommended future backend addition (not in this pass):** add a per-group
selection rule — e.g. a `selection_type TEXT ('single'|'multi')` or
`max_select INT` — so the contract is explicit and the backend can validate. This is
an additive migration; defer to the polish phase once the product rule is settled.

---

## 3. Promo placement (finding #11) — **FE-deferred + future rework note**

Backend today applies promo **at order placement**: `POST /sessions/:id/orders`
accepts `promo_code`, the discount is computed and stored on the order
(`orders.promo_id`, `orders.discount_amount`), and a redemption row is written keyed
by `(promo_id, order_id)` (unique). The bill aggregates per-order discounts.

The desired end-state (promo applied once against the whole bill at payment time)
requires backend rework: redemption would key on session/bill rather than order, and
the discount would live on the bill snapshot. That is **out of scope** for this
stabilization pass.

**FE contract (interim):** keep promo entry where the backend accepts it — at order
placement — until the backend redemption model is reworked. Do not move promo to the
bill screen on the frontend before the backend supports bill-level redemption, or
redemptions and discounts will diverge from what the backend records.

---

## 4. Kitchen vs waiter responsibilities (findings #3, #5)

Locked this pass:
- **Kitchen** advances orders `pending → confirmed → preparing → ready`. The kitchen
  role is **denied** `ready → served` (403 under enforcement).
- **Waiter / manager / owner** perform `ready → served`.
- The kitchen active-orders payload (`GET /branches/:id/orders/active`) now includes,
  per order, an `items` array: `{menu_item_id, name, quantity, modifiers[], note}`.
  All previously-returned top-level order fields are unchanged.

---

## 5. Session reactivation + multi-join (findings #1, #2)

- **Multi-join works** at the backend: multiple participants may join one active
  session. "Redirect home" is a frontend routing bug. **FE contract:** when
  `GET /tables/by-qr/:token` returns an active `session_id`, the frontend must
  `POST /sessions/:id/join` — never create a new session (which 409s on an occupied
  table) and never redirect home.
- **Reactivation** has an explicit endpoint now: `POST /sessions/:id/reactivate`
  (requires the guest credential). The "still ordering" action must call it and apply
  the returned session before re-enabling ordering, so the next order does not 409.
  Idempotent for already-active sessions; terminal sessions return `409`.

---

## 6. Menu availability realtime (finding #8)

Toggling a menu item now emits `MENU_ITEM_AVAILABILITY_CHANGED`
(`{item_id, is_available}`) to every live session in the branch. **FE contract:**
update local menu state and flag/remove the item from cart if present. This event is
informational — it MUST NOT be treated as a session or connection error. See
`docs/websocket-events.md`.
