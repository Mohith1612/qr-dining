# Spec — Promo per-guest limit + phone-gated redemption, and Guest host transfer (v1)

**Date:** 2026-06-05 · **Branch:** `premium-qr-collateral` · **Status:** spec (host transfer being implemented first)

Two related guest-facing features. Both lean heavily on machinery that already exists.

---

## Design decision — whose phone for a promo? → the **host's**

A promo applies to the **whole bill**, and a redemption is recorded as **one phone per order**
(`CreatePromoRedemption(promoID, orderID, phone)` — `backend/internal/services/order.go:312`). The
**host** is the only participant who can apply a promo or pay (host-gated, server-authoritative). So:

- **One redemption = the host's phone.** Don't collect every guest's number — there's no model for
  "this discount belongs to 4 phones," and burning the per-phone limit for guests who didn't benefit
  would be wrong.

**Known limitation:** an unverified phone is *soft* enforcement. With host-transfer (below) a group
could move host to a member with an unused number to dodge the per-phone cap. Acceptable for pilot;
OTP verification is the only real fix and is out of scope here.

---

## Spec 1 — Promo: settable per-guest limit + phone-gated redemption

### Already exists (no work)
- `promos.uses_per_phone` column.
- `ValidatePromo` enforces the per-phone cap when a phone is present (`backend/internal/services/promo.go:61`).
- Redemption recorded with phone at order time (`order.go:312`).
- `placeOrder(items, promoCode, phoneE164)` already accepts a phone (`frontend/hooks/useOrders.ts:24`).

### Broken / missing
- Admin form hardcodes `uses_per_phone: 1` (`frontend/app/(staff)/staff/(dashboard)/admin/page.tsx:1645`).
- `validate` endpoint passes `OrderTotal: 0` and **no phone** (`backend/internal/handlers/promo.go:68`),
  so min-order and per-phone are not checked at apply time.
- The per-phone check is **silently skipped** when no phone is supplied → cap is unenforceable.

### Backend changes
1. **Expose the limit (no schema change).** `createPromoRequest.UsesPerPhone` already exists
   (`handlers/promo.go:131`). Semantics: `0/blank` = unlimited per guest (no phone needed); `>0` =
   phone required.
2. **Make `validate` real.** Add `order_total float64` and `phone_e164 *string` to
   `validatePromoRequest`; pass them into `ValidatePromoRequest` instead of hardcoded `OrderTotal: 0`.
3. **Explicit "phone required" signal.** In `ValidatePromo`, after loading the promo:
   ```go
   if promo.UsesPerPhone > 0 && req.PhoneE164 == nil {
       return ValidatePromoResult{}, domain.ErrPromoPhoneRequired
   }
   ```
   Add `domain.ErrPromoPhoneRequired` → handler code `PROMO_PHONE_REQUIRED`.
4. **Recording** already works; ensure the apply-time phone flows into `PlaceOrder` (and, once promo
   moves to the bill per finding #5, into payment initiation).

### Frontend changes
5. **Admin form:** replace hardcoded `uses_per_phone: 1` with an input **"Uses per guest"**
   (1 / 2 / 3 / unlimited); relabel the existing field **"Max total uses"**.
6. **Guest apply (on the bill/payment page, per finding #5):**
   - call `validate` with `order_total` + host phone if known;
   - on `PROMO_PHONE_REQUIRED`, show a small **phone (+ optional name)** prompt (reuse `CustomerOptIn`
     styling; prefill from `participant.phone_e164`);
   - re-validate with the phone; on success carry the phone into payment/order.
7. **Errors:** `PROMO_PHONE_REQUIRED` → "Add your phone number to use this offer."; `PROMO_ALREADY_USED`
   → "You've already used this code."

**Effort:** 1 new domain error, ~10 lines in validate handler/service, one admin input, one
conditional phone prompt. Name optional (phone is the key).

---

## Spec 2 — Guest-initiated host transfer  *(implementing first)*

### Already exists
- `reassignHost(sess, newHost)` persists `host_participant_id` + `is_host` flags and broadcasts
  `HOST_CHANGED` (`backend/internal/services/session.go:288`) — but only called automatically by
  `ensureHostBaseline` when the host leaves/is revoked.
- Frontend already consumes `HOST_CHANGED` (`applyHostChanged`, "you're now host" toast —
  `frontend/hooks/useWebSocket.ts:70`).

We only need to expose a **manual** path.

### Backend changes
1. **`SessionService.TransferHost(ctx, sessionID, actingParticipantID, targetParticipantID)`:**
   - acting participant must be the **current host** → else `ErrNotHost` (403);
   - target must be an **active, non-revoked** participant of the **same** session → else 400/404;
   - block if session is **`payment_pending`** (host owns an in-flight bill);
   - call existing `reassignHost(...)` → persists + broadcasts `HOST_CHANGED`.
2. **Route** `POST /sessions/:id/host` in the guest API group (`server.go`), `requireGuestSession`,
   per-session rate-limited like `assist`. Body `{ "participant_id": <int> }`; resolve acting
   participant from the guest token.
3. **Handler** `handlers/session.go TransferHost` + audit row (`HOST_CHANGED`, risk low).

### Frontend changes
4. **Participant list** (`app/(guest)/session/[id]/page.tsx`, "At this table" card): when you are host,
   render **"Make host"** on each other participant → `sessionsApi.transferHost(...)`.
5. **No new realtime work** — existing `HOST_CHANGED` updates badges and gives the new host the
   order/pay affordances. Add a confirm + toast.
6. **API client:** add `sessionsApi.transferHost(sessionId, participantId, guestToken)`
   (`lib/api/sessions.ts`).

### Edge cases
- Target left/revoked between render and click → backend rejects ("That guest has left").
- Can't transfer to yourself (no-op) or to a non-member.
- Transfer blocked while `payment_pending`.

**Effort:** one service method wrapping `reassignHost`, one route + handler, one API method, one
button. Realtime already done.
