# Phase 0 Guardrail Matrix

Date: 2026-05-21

Status: Baseline artifact with Phase 0 and Phase 1 backend implementation notes. Rows still identify future ownership where strict enforcement remains incomplete.

Implemented updates:

- Phase 0 added backend rollout flags, legacy identity metrics, and pending guardrail tests.
- Phase 1 added staff-code authentication, durable staff sessions, guest token issuance/validation, and permissive guest credential checks.

## Auth Boundary Map

| Route | Current boundary | Tenant / branch source | Resource | Owning phase |
| --- | --- | --- | --- | --- |
| `GET /health`, `GET /readyz`, `GET /metrics` | public infrastructure | none | runtime health/metrics | Phase 9 |
| `POST /sessions` | public guest | table lookup | session/table | Phase 1, Phase 7 |
| `GET /sessions/:id` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session row or guest token | session | Phase 1 |
| `DELETE /sessions/:id` | guest token accepted; legacy `X-Participant-ID` allowed unless strict flag is enabled | session row + guest/legacy participant | session/table | Phase 1, Phase 7 |
| `POST /sessions/:id/join` | public guest; returns signed guest token | session row | participant | Phase 1 |
| `GET /sessions/:id/cart` | guest token accepted; legacy `X-Participant-ID` allowed unless strict flag is enabled | token participant or legacy participant ID | cart | Phase 1, Phase 2 |
| `POST /sessions/:id/cart/items` | guest token accepted; legacy `X-Participant-ID` allowed unless strict flag is enabled | token participant or legacy participant ID + menu item | cart/menu | Phase 1, Phase 2 |
| `DELETE /sessions/:id/cart/items/:item_id` | guest token accepted; legacy `X-Participant-ID` allowed unless strict flag is enabled | token participant or legacy participant ID + cart item | cart | Phase 1, Phase 2 |
| `POST /sessions/:id/orders` | guest token accepted; legacy body identifiers allowed unless strict flag is enabled | token participant/session or legacy body branch/participant | orders/menu/promos | Phase 1, Phase 2, Phase 7 |
| `GET /sessions/:id/orders` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session row or guest token | orders | Phase 1 |
| `POST /sessions/:id/assist` | guest token accepted; legacy body participant allowed unless strict flag is enabled | token participant/session or legacy body table/participant | assistance | Phase 1, Phase 2 |
| `POST /sessions/:id/payments` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session bill calculation | payments | Phase 1, Phase 7 |
| `GET /sessions/:id/bill` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session row or guest token | bill/orders/payments | Phase 1, Phase 7 |
| `POST /webhooks/payments/:provider` | public webhook, no provider signature | payload | payments/webhooks | Phase 7 |
| `POST /sessions/:id/customer` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session row + phone | customer | Phase 1, Phase 2 |
| `POST /sessions/:id/promos/validate` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session + body promo data | promos | Phase 2, Phase 7 |
| `GET /branches/:id/menu` | public tenant guard when `BASE_DOMAIN` is set | path branch + tenant subdomain | menu | Phase 2, Phase 3 |
| `GET /tables/by-qr/:token` | public QR token | table token | table | Phase 1 |
| `GET /sessions/:id/snapshot` | guest token accepted; legacy UUID-only allowed unless strict flag is enabled | session row or guest token | session snapshot | Phase 1, Phase 6 |
| `GET /tenants/by-slug/:slug` | public | slug | tenant/restaurant | Phase 3 |
| `GET /plans` | public | none | subscription plans | Phase 8 |
| `POST /staff/auth` | staff-code auth supported; legacy branch ID + PIN allowed unless strict flag is enabled | branch code or legacy branch ID | staff session | Phase 1 |
| `PATCH /orders/:id/status` | staff token only | order row, no staff branch comparison | orders | Phase 2 |
| `PATCH /assist/:id/ack`, `PATCH /assist/:id/resolve` | staff token only | assistance row, no staff branch comparison | assistance | Phase 2 |
| `GET /branches/:id/orders/active` | staff token + branch param comparison + tenant guard | path branch | orders | Phase 2 |
| `GET /branches/:id/sessions/active` | staff token + branch param comparison + tenant guard | path branch | sessions | Phase 2 |
| `GET /branches/:id/assist/active` | staff token + branch param comparison + tenant guard | path branch | assistance | Phase 2 |
| `GET /branches/:id/menu/full` | staff token + tenant guard | path branch | menu | Phase 2 |
| `POST /branches/:id/menu/categories`, `POST /branches/:id/menu/items` | staff token + handler branch comparison | path branch | menu | Phase 2 |
| `POST /branches/:id/staff` | staff token + owner role + branch comparison | path branch | staff | Phase 1, Phase 2 |
| `GET /branches/:id/events/recent` | staff token + tenant guard | path branch | event log | Phase 5 |
| `GET /branches/:id/analytics/top-items` | staff token + handler branch comparison + tenant guard | path branch | analytics | Phase 2, Phase 8 |
| `GET /branches/:id/analytics/busy-hours` | staff token + handler branch comparison + tenant guard | path branch | analytics | Phase 2, Phase 8 |
| `GET /branches/:id/analytics/order-volume` | staff token + handler branch comparison + tenant guard | path branch | analytics | Phase 2, Phase 8 |
| `GET /branches/:id/tables` | staff token + handler branch comparison + tenant guard | path branch | tables | Phase 2 |
| `POST /branches/:id/tables` | staff token + handler branch comparison + tenant guard | path branch | tables | Phase 2 |
| `GET /branches/:id` | staff token + handler branch comparison + tenant guard | path branch | branch | Phase 2 |
| `PATCH /branches/:id` | staff token + handler branch comparison + tenant guard | path branch | branch | Phase 2, Phase 8 |
| `GET /branches/:id/customers` | staff token + handler branch comparison + tenant guard | path branch | customers | Phase 2 |
| `GET /branches/:id/promos`, `POST /branches/:id/promos`, `DELETE /branches/:id/promos/:promo_id` | staff token + tenant guard; staff branch comparison is incomplete | path branch | promos | Phase 2 |
| `PATCH /menu/items/:id` | staff token; body branch check with item target ID | target item ID + client branch data | menu | Phase 2 |
| `DELETE /menu/items/:id` | staff token; query branch check with item target ID | target item ID + client branch data | menu | Phase 2 |
| `PATCH /menu/items/:id/availability` | staff token; body branch check with item target ID | target item ID + client branch data | menu | Phase 2 |
| `PATCH /menu/items/:id/featured` | staff token; body branch check with item target ID | target item ID + client branch data | menu | Phase 2 |
| `POST /menu/items/:id/modifiers` | staff token; body branch check with item target ID | target item ID + client branch data | menu modifiers | Phase 2 |
| `DELETE /menu/categories/:id` | staff token; query branch check with category target ID | target category ID + client branch data | menu categories | Phase 2 |
| `PATCH /menu/categories/:id` | staff token; body branch check with category target ID | target category ID + client branch data | menu categories | Phase 2 |
| `DELETE /menu/modifiers/:id` | staff token; query branch check with modifier target ID | target modifier ID + client branch data | menu modifiers | Phase 2 |
| `POST /upload/menu-item-image`, `POST /upload/restaurant-logo` | staff token | staff branch | uploads | Phase 2 |
| `PATCH /tables/:id/qr-refresh` | staff token + handler ownership check | table row | tables | Phase 2 |
| `PATCH /staff/:id/pin`, `PATCH /staff/:id/deactivate` | staff token + role/self checks | staff row | staff | Phase 1, Phase 2 |
| `GET /sessions/:id/events` | staff token only | session row | event log | Phase 2, Phase 5 |
| `GET /restaurants/:id/subscription` | staff token + restaurant/branch comparison | restaurant row | subscription | Phase 3, Phase 8 |
| `GET /customers/:id/history`, `DELETE /customers/:id` | staff token + branch/restaurant checks vary | customer row | customers | Phase 2 |
| `GET /ws` | guest token accepted; legacy query `participant_id` allowed unless strict guest/ws flags are enabled | guest token or query params + participant lookup | realtime session | Phase 1, Phase 6 |

## Branch Ownership Rules

| Resource | Current source of truth | Phase 0 risk | Future enforcement point |
| --- | --- | --- | --- |
| Orders | `orders.branch_id`, `orders.session_id` | status updates do not compare order branch to staff branch; placement trusts body branch | policy/service check before create/update |
| Assistance | session/table relationship plus assistance row | requests trust body table/participant; staff ack/resolve lack branch comparison | policy/service check before create/update |
| Menu categories/items/modifiers | menu row branch and category relationship | item-scoped mutations can trust body/query branch or update by ID only | branch-scoped SQL and service ownership validation |
| Promos | `promos.branch_id` | staff branch comparison is incomplete around branch route | handler/service branch authorization |
| Sessions | `sessions.branch_id`, `sessions.table_id`, host participant | guest tokens now supported; legacy UUID/header access remains until strict flag rollout | guest credential enforcement and session service checks |
| Payments | `payments.session_id`, webhook rows | guest initiation unauthenticated; webhook unsigned and internal payment IDs trusted | provider verification and payment state policy |
| Customers | customer restaurant/session history | customer operations rely on handler-specific ownership checks | centralized branch/restaurant policy |
| Uploads | authenticated staff branch | uploaded object intent is staff-asserted | staff branch policy and object association validation |
| Websocket | session + participant row | guest token accepted; query auth remains during compatibility window | signed guest websocket ticket / strict flag rollout |

## Required Enforcement Ownership

| Audit blocker | Current confirmation | Owning phase |
| --- | --- | --- |
| Duplicate PIN and inactive staff login | Phase 1 now filters active staff and rejects ambiguous legacy PIN matches; staff-code login resolves exact staff identity | Phase 1 implemented |
| Guest participant spoofing | Phase 1 now accepts signed guest tokens and rejects token/legacy participant mismatches; legacy inputs remain until strict flag rollout | Phase 1 implemented, frontend rollout pending |
| Cross-branch staff mutations | order and assistance item routes mutate target IDs without staff branch comparison | Phase 2 |
| Menu ownership gaps | item/category/modifier mutations mix target IDs with body/query branch data | Phase 2 |
| Promo branch authorization gaps | promo management relies on tenant guard more than staff branch policy | Phase 2 |
| Unsigned payment webhook | webhook handler accepts JSON payload without provider signature | Phase 7 |
| Webhook idempotency bookkeeping | successful processing does not mark webhook rows processed | Phase 7 |
| Stale session table release | stale worker abandons sessions without releasing table status | Phase 7 |
| Duplicate active session race | active-session precheck is outside a DB-enforced single-active-session constraint | Phase 7 |
| Websocket query auth | Phase 1 accepts guest tokens for websocket; legacy query auth remains until `AUTH_GUEST_CREDENTIALS_REQUIRED` or `WS_TICKET_AUTH_REQUIRED` is enabled | Phase 6 |
| Redis-only staff token validation | Phase 1 revalidates staff active state, token version, PIN version, and durable session state | Phase 1 implemented |

## Phase 0 Instrumentation

Phase 0 records legacy identity usage with `legacy_identity_usage_total{mechanism,endpoint_class}`. The counter is observational only and must not block requests.

Mechanisms:

- `x_participant_id`
- `body_participant_id`
- `body_placed_by_participant_id`
- `branch_id_pin`
- `ws_query_participant_id`

Endpoint classes:

- `guest_cart`
- `guest_session`
- `guest_order`
- `guest_assistance`
- `staff_auth`
- `websocket`
