# Backend Workflow Stabilization Plan — v1

> **Status: IMPLEMENTED.** All 7 steps in §5 are complete — commits `017b3ee`, `bac813b`,
> `2e8a666`, `e38aa94`, `abf5b22`, `f42a609`, `12922d6`. Backend builds and the non-integration
> test suite is green. Remaining work is the frontend production-polish phase, which should build
> against `docs/frontend-contract-stabilization.md`. Soak-watch: payment 4xx mapping, the
> reactivation endpoint, and the new `event_publish_duration_seconds` metric.

## Context

This document is the output of the **Backend Workflow Stabilization Phase** — one final backend
stabilization pass to lock down operational correctness, workflow semantics, and frontend/backend
contracts *before* the frontend production-polish sprint, so contracts stop thrashing afterward.

The backend architecture is mature: auth hardening, lifecycle hardening, WebSocket hardening,
rollout governance, and observability are all complete. R1 soak is active
(`AUDIT_LOG_V2_ENABLED=true`). The backend is **no longer in architecture-redesign territory** —
the remaining gaps are operational workflow correctness, frontend/backend synchronization,
lifecycle edge cases, role-responsibility correctness, and API-contract stabilization.

The primary input is `manual-testing-findings-v1.md` (real multi-role manual walkthroughs —
guest, waiter, kitchen, admin, realtime, lifecycle, payment, mobile viewport).

### Key insight from code exploration (drives everything below)

A deep pass over the Go backend (`backend/`) revealed that **several "P0/P1" findings contradict the
current backend code** — the backend already does the operationally-correct thing, which means the
defect is **frontend/contract drift, not a backend bug**:

- **Multi-guest join** (finding #1): no DB unique constraint and no host-only guard prevents a
  second participant. `JoinSession` (`internal/services/session.go:206`) creates another participant
  on an `active` session without complaint. The "redirects to home" symptom is almost certainly a
  **frontend QR→join routing bug** or a **guest-credential (`credential_version` / `revoked_at`)
  rejection**, not a backend block.
- **Cash/card "immediately settles"** (finding #6): backend already routes cash →
  `requires_staff_confirmation` and card → `card_manual` → `requires_staff_confirmation`
  (`normalizePaymentMethodStatus`, `internal/services/payment.go:491`). Settlement only completes
  via `PATCH /payments/:id/settle`. The "immediate settle" is **frontend optimistic UI**, not
  backend behavior.
- **Kitchen can mark served** (finding #5): this one *is* a real backend gap — `roleAllowed` grants
  `ActionOrderStatusUpdate` to `kitchen` for **every** transition (`internal/authz/policy.go:112`),
  with no transition-aware restriction.

**Handling approach:**
- Frontend-only / stale-contract items are **classified + contract-specced only** here; FE rendering
  fixes are deferred to the frontend-polish phase.
- Every contradiction between the manual findings and the code gets a **verify-first task**
  (reproduce against current code before any fix).

Confirmed genuine backend gaps:
- `ActionOrderStatusUpdate` lets `kitchen` perform `ready→served` (`policy.go:112`).
- `ActionPaymentSettleStaff` is restricted to **owner/manager only** (`policy.go:125`) — a **waiter
  cannot confirm cash/card collection**, directly contradicting the desired workflow in finding #6.
- The menu admin toggle publishes **no WebSocket event** (`internal/services/menu.go:251`) — guests
  never learn of availability changes in realtime (root of finding #8).

---

## 1. Executive Summary

- **Backend maturity:** Core architecture, auth, lifecycle state machine, WebSocket fanout, payment
  state machine, and observability are stable and production-oriented. No architectural redesign is
  warranted by these findings.
- **What remains unstable:** (a) a small number of **authz / role-boundary** gaps, (b) **contract
  drift** where the frontend assumes behavior the backend already changed, (c) **missing realtime
  events** for menu changes, and (d) **reactivation ergonomics** (no explicit reactivation endpoint;
  it only fires as a side-effect of `GET snapshot`).
- **Why this is likely the final backend pass:** the remaining defects are workflow-correctness and
  contract-alignment, not foundation. Once role boundaries, the menu-event contract, and the
  reactivation contract are locked, the frontend can be polished aggressively without backend churn.

## 2. Issue Classification Matrix

| # | Finding | Severity | Ownership | Workflow impact | Production risk | Root-cause hypothesis |
|---|---------|----------|-----------|-----------------|-----------------|-----------------------|
| 1 | Same-table multi-user join redirects home | P0 | **FE / shared** (verify-first) | Collaborative ordering broken | High | Backend allows join (no constraint, `session.go:206`). FE QR→join routing or guest-credential (`credential_version`/revoked) rejection → redirect. |
| 2 | 409 after reactivation until refresh | P0 | **Shared contract** | Guest can't order post-timeout | High | Reactivation only fires on `GET /sessions/:id/snapshot` (`session.go:257`). "Still ordering" button doesn't hit it → `PlaceOrder` sees non-active status → `ErrSessionClosed` 409 (`order.go:59`). No explicit reactivation endpoint. |
| 3 | Kitchen screen missing item/modifier/note detail | P0 | **BE** | Kitchen unusable | High | `ListActiveOrdersForBranch` (`sql/queries/orders.sql:60`) returns order metadata only — no items/modifiers/notes join. Items live in `order_items.selected_modifiers_json` + `.note`. |
| 4 | Payment methods intermittent 500 | P0 | **BE / shared** (verify-first) | Payment may fail | High | Candidates: missing/`null` `bill_snapshot_id` on settle (`payment.go:415`), float-precision amount compare (`payment.go:540`), missing `payment_ref`/`session_id` in webhook, card→`card_manual` normalization, idempotency-key edge. Needs reproduction + log capture. |
| 5 | Kitchen can mark order "served" | P1 | **BE** | Role-boundary violation | Medium | `roleAllowed` grants `ActionOrderStatusUpdate` to kitchen for all transitions (`policy.go:112`); no transition-aware guard. |
| 6 | Cash/card workflow incorrect (auto-settles) | P1 | **FE display + BE authz** (verify-first) | Payment-collection integrity | Medium | Backend already requires staff confirmation (`payment.go:491`). FE shows premature "paid". AND waiter blocked from settling — `ActionPaymentSettleStaff` is owner/manager only (`policy.go:125`). |
| 7 | WebSocket realtime delay (seconds) | P1 | **BE (investigate) / infra** | Operational responsiveness | Medium | Nominal path is non-blocking (`hub.go` single-goroutine, `pubsub.go:57` wildcard PSubscribe). Each event does a synchronous `AppendSessionEvent` DB write (`events/events.go:48`) before publish — likely DB/proxy latency, not architecture. Verify with metrics. |
| 8 | Admin toggle → persistent "connection lost" | P1 | **BE + shared contract** | Guest session appears dead | High | `ToggleAvailability` updates DB + invalidates cache but publishes **no WS event** (`menu.go:251`). Guests keep stale menu; ordering disabled item → `MENU_ITEM_UNAVAILABLE`, which FE misreads as connection loss. No menu-change event contract exists. |
| 9 | Staff PIN input alignment | P2 | **FE only** | Cosmetic | Low | FE layout. Classify only; defer. |
| 10 | Beverage modifier multi-select (should be single) | P2 | **FE + contract** | Wrong order data | Low–Med | FE selection logic. Backend stores `selected_modifiers_json` as-is. Need contract: define modifier `min/max`/`single_select` field if not present, else FE-only. |
| 11 | Promo at order time vs bill time | P2 | **FE + contract** | Promo UX | Low | Promo currently applied at order placement; desired at bill/payment. Verify whether backend `promo_id` lives on order vs payment/bill snapshot; define contract. |
| 12 | Ready-to-send screen missing qty controls | P2 | **FE only** | Pre-send editing | Low | FE cart UI. Backend accepts quantities at placement. Classify only; defer. |
| 13 | Menu image layout inconsistent | P2 | **FE only** | Cosmetic | Low | FE. Classify only; defer. |
| 14 | Staff list missing in admin | P3 | BE+FE (expansion) | Admin mgmt | Low | Out of scope this phase (expansion). |
| 15 | Table management missing | P3 | BE+FE (expansion) | Admin mgmt | Low | Out of scope this phase. |
| 16 | QR customization missing | P3 | FE+BE (expansion) | Branding | Low | Out of scope this phase. |
| 17 | Platform dashboard | P3 | Platform (expansion) | Multi-tenant ops | Low | Explicitly out of scope (no super-admin yet). |

## 3. Stabilization Buckets

**Bucket A — Session lifecycle stabilization** (findings 1, 2)
- A0 (verify): reproduce multi-join "redirect home" with two clients on one table; capture whether
  the redirect is FE routing or a guest-auth (`credential_version`/`revoked_at`) rejection.
- A1: define + add an **explicit reactivation contract**. Either a dedicated
  `POST /sessions/:id/reactivate` endpoint, or document that the FE "still ordering" action MUST
  call `GET /sessions/:id/snapshot` (which already reactivates, `session.go:257`) and apply the
  returned authoritative state before re-enabling ordering. Recommend the explicit endpoint for a
  clean contract.
- A2: contract spec for FE QR→join (when `by-qr` returns a `session_id`, FE joins; never redirect
  home on an active session).

**Bucket B — Kitchen/waiter operational separation** (findings 3, 5)
- B1: add a **kitchen order-detail payload** — extend the kitchen list query (or add a batched
  detail query) to include items, quantities, `selected_modifiers_json`, and `note` per order.
  Define the kitchen-order DTO as a stable contract.
- B2: make `ActionOrderStatusUpdate` **transition-aware** in `roleAllowed` (policy.go) — kitchen may
  do `pending/confirmed→preparing→ready`; `ready→served` restricted to waiter/manager/owner.

**Bucket C — Payment workflow stabilization** (findings 4, 6)
- C0 (verify): reproduce the cash/card/UPI 500s with logging; confirm exact origin from the
  hypotheses in the matrix.
- C1: allow **waiter** to settle staff-confirmed payments — add `StaffRoleWaiter` to
  `ActionPaymentSettleStaff` in `roleAllowed` (policy.go:125).
- C2: document the canonical cash/card lifecycle contract (guest requests → waiter collects →
  waiter confirms via `PATCH /payments/:id/settle` → completed) so FE stops showing premature
  "paid". FE display fix deferred to polish phase.

**Bucket D — WebSocket / event synchronization** (findings 7, 8)
- D1: publish a **menu-change WebSocket event** on `ToggleAvailability` (and related menu mutations)
  — define `MENU_ITEM_AVAILABILITY_CHANGED` (or `MENU_UPDATED`) with payload `{item_id, is_available}`.
  Define the FE handling contract: reconcile local menu/cart, do NOT treat as connection loss.
- D2 (investigate): instrument/confirm the realtime-delay source (DB append latency vs proxy);
  decide if any change is needed. Likely no code change beyond measurement.

**Bucket E — API contract alignment** (findings 10, 11; contract portions of others)
- E1: modifier single-select — confirm whether `menu_modifiers`/`item_modifiers` carry
  `min_select`/`max_select`/`single_select`; if absent and needed, define the field as a contract
  addition; otherwise FE-only.
- E2: promo-at-bill-time — determine whether promo belongs on order vs bill snapshot/payment;
  define the contract. Implementation of FE move deferred.

**Bucket F — Deferred (P3 expansion: findings 14–17)** — explicitly out of scope this phase.

## 4. Dependency Graph

- **Must happen before frontend polish (contract-locking):** A1/A2 (reactivation + join contract),
  B1 (kitchen payload DTO), B2 (role boundaries), C1/C2 (payment authz + lifecycle contract),
  D1 (menu-change event contract), E1/E2 (modifier + promo contracts).
- **Can happen independently / low contract risk:** D2 (latency investigation), C0/A0/B-verify steps.
- **Risks contract drift if deferred:** D1 (no menu event = FE keeps guessing), A1 (reactivation),
  B1 (kitchen DTO shape). These define new payloads the FE will bind to — lock them first.
- **No drift risk (pure FE, deferred):** findings 9, 12, 13, and the rendering halves of 6/10/11.

## 5. Recommended Implementation Order

Safest sequencing — low-risk authz/contract changes that unblock the FE first, verification before
any change to "already-correct" areas, observation-only items last:

1. ✅ **DONE** (`017b3ee`) — **B2 + C1 — authz boundary fixes** (kitchen cannot serve; waiter can
   settle). New `ActionOrderMarkServed`, transition-aware in the order handler; waiter added to
   `ActionPaymentSettleStaff`. Policy unit tests added.
2. ✅ **DONE** (`bac813b`) — **B1 — kitchen order-detail payload.** Additive `items` array (name,
   qty, modifiers, note) via one batched query; existing order fields preserved.
3. ✅ **DONE** (`2e8a666`) — **D1 — menu-change WS event + FE handling contract.**
   `MENU_ITEM_AVAILABILITY_CHANGED` fanned out to every active session on toggle; FE handling
   contract documented in `docs/websocket-events.md`.
4. ✅ **DONE** (`e38aa94`) — **A1/A2 — reactivation + join contract.** A0 verified (multi-join works;
   "redirect home" is FE). Added `POST /sessions/:id/reactivate`; join contract documented.
5. ✅ **DONE** (`abf5b22`) — **C0 → C-fix — payment 500.** Confirmed cause: handler collapsed every
   non-idempotency error into 500. Now returns 404/409 for non-active sessions; concurrent
   payment_pending race translated to a clean conflict.
6. ✅ **DONE** (`f42a609`) — **C2 + E1 + E2 — contract documentation.**
   `docs/frontend-contract-stabilization.md` covers cash lifecycle, modifier-select (+ future schema
   note), and promo-placement (+ future rework note). FE implementation deferred to polish phase.
7. ✅ **DONE** (`12922d6`) — **D2 — realtime latency investigation.** Added
   `event_publish_duration_seconds` (by event type + outcome) to measure the server-side publish
   path during soak. Instrumentation only; no behavior change.

Rationale: no "random fixes" — each step is either an additive contract (safe) or a verified,
narrowly-scoped correction. Authz first because it's the smallest change with the clearest
operational-correctness payoff and zero contract surface for the FE.

## 6. Frontend Contract Stability Assessment

After this pass, frontend contracts should become **stable enough for aggressive polish** because
every payload/endpoint the FE binds to is locked:
- Kitchen order DTO (B1) — fixed shape with items/modifiers/notes.
- Reactivation contract (A1) — explicit, deterministic, no refresh required.
- Menu-change event (D1) — FE has a defined realtime signal instead of guessing.
- Payment lifecycle + settle authz (C1/C2) — FE knows exactly when "paid" is true.
- Order-status role matrix (B2) — FE can hide "served" for kitchen role with confidence.

Remaining FE-only work (layout, qty controls, PIN alignment, modifier rendering, promo placement)
touches **presentation, not contracts** — safe to do in the polish phase without backend coupling.

## 7. Rollout / Soak Considerations

- **Soak-safe (additive, no behavior regression):** B1 (additive query/DTO), D1 (new event type —
  old clients ignore unknown events), C1/B2 (tightening/loosening role checks — verify no current
  client depends on kitchen-served or owner-only-settle), contract docs (C2/E1/E2).
- **Requires careful rollout / verify-first:** A1 reactivation endpoint (changes recovery semantics
  during active R1 soak — guard behind the existing rollout/flag pattern if one applies), C-fix for
  payment 500 (touches money path — stage + observe).
- **May need additional soak/observation:** D2 latency change (if any), payment fix (watch
  `payment` metrics + `audit_log` for a soak window before declaring stable).
- All changes follow existing version-control discipline: small atomic commits, plain human commit
  messages, no co-author / no AI attribution, clean tree.

## 8. Estimated Remaining Backend Instability

- **Still feels volatile:** the **payment 500 path** (until C0 reproduction pins the cause) and the
  **reactivation contract** (semantics are correct but ergonomics/triggering are awkward). The
  **menu→realtime** path is a known gap (D1).
- **Stable now:** session state machine, order/payment state machines, WebSocket fanout
  architecture, auth/credential model, idempotency + bill-snapshot immutability, worker pipelines.
- **Likely will NOT change again:** the core state-machine definitions, Redis pub/sub fanout design,
  embedded-migration model, and the session-as-primary-entity model. These are foundation and the
  findings give no reason to touch them.

## Verification (for the implementation phase)

End-to-end checks to run once fixes land:
- **Multi-join:** two browser clients on one table QR → both reach the live session (no redirect).
- **Reactivation:** force `awaiting_reactivation` (worker/presence), tap "still ordering", place an
  order → succeeds with no refresh, no 409.
- **Kitchen:** kitchen dashboard shows items+modifiers+notes; kitchen role cannot `ready→served`
  (403/authz), waiter can.
- **Payment:** cash/card → `requires_staff_confirmation`; waiter `PATCH /payments/:id/settle`
  completes; no 500 across cash/card/UPI; UPI webhook completes.
- **Menu toggle:** admin disables item → guests get a realtime event, menu/cart reconcile, no
  "connection lost"; re-enable propagates too.
- Run backend unit/integration tests (`authz` policy tests, payment/order state-machine tests) and
  the existing e2e suite where applicable.
