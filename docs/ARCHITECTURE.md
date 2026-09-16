# Architecture

Last verified against code: 2026-09-13.

The repository separates a Next.js client from the Go HTTP application
(`frontend/package.json:1-14`, `backend/cmd/server/main.go:23-136`). The production
compose file defines one Go app replica with PostgreSQL and Redis and joins the
app to an external proxy network (`deploy/vm/docker-compose.yml:33-114`).
PostgreSQL owns durable business state. Redis is used for cache, rate limiting,
presence, WebSocket fan-out, short-lived tickets, and worker locks; the server
wires those components at startup (`backend/cmd/server/main.go:75-105`).

## Durable model

The core transaction chain is:

```text
organization → restaurant → branch → table → session → participant
                                      │          ├── shared cart → cart item
                                      │          ├── order → order item
                                      │          ├── assistance request
                                      │          └── bill snapshot ← payment
                                      └── staff → staff session
```

The initial schema defines restaurants and branches, tables and staff, menu
categories/items/modifiers, sessions and participants, carts and cart items,
orders and order items, assistance requests, and payments
(`backend/migrations/000001_initial_schema.up.sql:21-126,136-174,184-292`).
Organizations were added above restaurants/branches by migration 17
(`backend/migrations/000017_organization_model.up.sql:1-45`). Platform users and
sessions form a separate control-plane trust domain
(`backend/migrations/000018_platform_trust_domain.up.sql:1-47`).

Order items retain unit price and selected modifier JSON so placed orders do not
change with the menu (`backend/migrations/000001_initial_schema.up.sql:235-249`).
Payment initiation persists a bill snapshot and its payment in one database
transaction (`backend/internal/services/payment.go:271-294`). Audit rows are
append-only by trigger and deliberately have no foreign keys so tenant deletion
does not delete them (`backend/migrations/000019_audit_log_v2.up.sql:65-89`).

Two database constraints close the high-consequence concurrency races: one live
session per table and one non-terminal payment per session
(`backend/migrations/000020_realtime_session_hardening.up.sql:3-39`,
`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`). A
third partial index gives each session one shared cart
(`backend/migrations/000027_shared_session_cart.up.sql:1-10`).

## HTTP surface by identity class

The router is the source of truth for this summary.

| Identity | Surface | Enforcement |
|---|---|---|
| None | `/health`, `/readyz`, `/metrics` | No authentication or API rate-limit middleware; each registered for `GET` and `HEAD` so uptime monitors probing with `HEAD` are not answered 404 (`backend/internal/server/server.go:187-197`). |
| Guest participant | Session create/join/read/close/reactivate/host transfer; cart; order; assistance; bill/payment; snapshot; customer and promo actions | Public group is rate-limited; action handlers recover participant identity and host-only services enforce authority (`backend/internal/server/server.go:199-271`, `backend/internal/services/session.go:318-359`). |
| Public | Branch menu/flags/theme, QR resolve, tenant resolution, plans, payment webhook | Public group plus branch tenant guard where registered; payment initiation and webhook add sensitive limits (`backend/internal/server/server.go:242-277`). |
| Staff | Order/payment/assistance mutation; recovery; branch operations, menu, tables, staff, analytics, loyalty, promos, audit and customer data | Staff middleware applies to the group; resource handlers add branch, role, and central-policy checks (`backend/internal/server/server.go:386-492`). |
| Platform | Tenant lifecycle, support, audit, plans, billing records, flags, analytics and themes | Platform token middleware protects the entire `/platform` group; staff tokens are a different validator (`backend/internal/server/server.go:288-384`, `backend/internal/middleware/platform_auth.go:15-46`). |

`POST /staff/auth`, `POST /platform/auth`, and `POST /platform/auth/mfa` are
separately sensitive-rate-limited authentication endpoints
(`backend/internal/server/server.go:279-286`). WebSockets upgrade at `/ws`; the
ticket path validates session, organization, branch, participant, credential
version, and revocation before upgrade (`backend/internal/server/server.go:491-492`,
`backend/internal/handlers/ws.go:109-149`).

## State machines

The domain package defines every accepted transition
(`backend/internal/domain/statemachine.go:73-154`). The concise graphs are:

```text
session:
  active ↔ payment_pending
  active/payment_pending ↔ awaiting_reactivation
  any live state → closed | abandoned | expired
  terminal states → nowhere

order:
  pending → confirmed → preparing → ready → served
      └──────── cancellation is allowed through preparing only

assistance:
  pending → acknowledged → resolved
      └──────────────────→ resolved

payment:
  requested → provider_pending | requires_staff_confirmation | failed | cancelled
  provider_pending/requires_staff_confirmation → completed | failed | cancelled
  completed → partially_refunded | refunded
  partially_refunded → refunded
```

The exact session and order edges are
`backend/internal/domain/statemachine.go:73-107`; assistance and payment edges are
`backend/internal/domain/statemachine.go:109-125`. Although the payment state
machine retains provider states, current initiation accepts five methods and
normalizes every one to `requires_staff_confirmation`; `provider_pending` is not
reached by initiation (`backend/internal/handlers/payment.go:97-103`,
`backend/internal/services/payment.go:759-773`).

## Session and presence

Session creation locks the table and creates the session, first host participant,
host pointer, and occupied table state in one transaction
(`backend/internal/services/session.go:99-175`). A suspended or archived
organization or branch blocks QR resolve, creation, and joining but does not stop
an existing meal (`backend/internal/services/tenant_status.go:33-78`,
`backend/internal/services/session.go:99-115,567-596`,
`backend/internal/services/menu.go:98-112`).

Presence stores one timestamp per participant and refreshes the hash TTL on each
heartbeat. Readers decide presence per field at 90 seconds, while unfiltered
timestamps remain available for the longer host-absence calculation
(`backend/internal/redis/presence.go:13-23,46-76,95-127,151-158`). Host transfer
waits a default three-minute absence and requires the replacement participant to
be active and present (`backend/internal/services/session.go:318-359,399-440`).

Both timeout queries now use latest durable participant activity, falling back to
session creation only for a participant-less session: the cleaner closes after
the branch timeout and the warner selects the 15-minute window
(`backend/sql/queries/workers.sql:3-14`,
`backend/sql/queries/sessions.sql:110-123`).

## Cart, ordering, and money path

Participants collaborate on one session cart; only the host may submit it as an
order (`backend/internal/services/cart.go:48-57,60-150`,
`backend/internal/services/order.go:95-108`). Order placement validates branch,
menu availability, modifiers, and prices before writing price snapshots in one
transaction (`backend/internal/services/order.go:151-218,222-294`). A successful
order clears the shared cart best-effort after the order transaction commits
(`backend/internal/services/order.go:301-311`).

Payment initiation follows this sequence:

1. The handler computes the authoritative session bill, applies an optional
   validated promo, and rejects any client amount unequal to the resulting total
   with 422 (`backend/internal/handlers/payment.go:105-149`).
2. The service performs the host check before idempotency or session-state effects
   (`backend/internal/services/payment.go:128-146`).
3. An existing non-terminal payment is returned; a fresh one moves the session to
   `payment_pending`, then creates the snapshot and payment together
   (`backend/internal/services/payment.go:148-190,223-294`).
4. A racing insert rejected by migration 40 is converted into the same existing
   payment response (`backend/internal/services/payment.go:331-338`). New records
   return 201; reused records return 200
   (`backend/internal/handlers/payment.go:186-193`).
5. Staff settlement accepts only `requires_staff_confirmation`
   (`backend/sql/queries/payments.sql:45-54`). Closure verifies that the snapshot's
   source-order set is still current and sums completed money only for that
   snapshot (`backend/internal/services/payment.go:814-885`,
   `backend/sql/queries/payments.sql:100-105`).

The escalation worker never settles or cancels money. It emits metrics, logs,
session events, WebSocket events, and audit records, leaving recovery to staff
settle/cancel and force-close routes (`backend/internal/worker/worker.go:288-310,355-417`).
The billing reconciliation worker is likewise read-only with respect to money;
it compares snapshot-to-orders and collected-to-snapshot, writing only metrics
and audit evidence (`backend/internal/worker/worker.go:132-177,319-353`,
`backend/sql/queries/payments.sql:123-258`).

## Realtime

Each event has an ID, per-session sequence, tenant scope, event type, timestamp,
and JSON payload (`backend/internal/websocket/message.go:44-75`). The publisher
appends to `session_events` before Redis publication; append failure suppresses
publication, while Redis failure is logged and cannot roll back the already
committed business operation (`backend/internal/events/events.go:46-83`).
Per-session advisory locking makes sequence allocation serial, and replay returns
at most 500 later events in order (`backend/internal/repository/session.go:200-281`).

One hub goroutine owns connection rooms and receives messages from a Redis
subscriber; slow consumers are evicted, and the subscriber reconnects with
bounded exponential backoff (`backend/internal/websocket/hub.go:21-34,88-127,158-219`).
Clients reconcile after reconnect through the snapshot endpoint. A snapshot is
authoritative when the client has no sequence basis or replay begins after a gap;
terminal sessions remain readable for 60 minutes
(`backend/internal/services/session.go:651-681,739-779`). See
[reference/websocket-events.md](reference/websocket-events.md) for the emitted
event catalog.

## Background workers

Startup launches seven loops: stale-session cleaner, 15-minute expiry warner,
presence-expiry hook, session/table reconciler, reactivation pipeline,
payment-pending escalation, and billing reconciliation
(`backend/cmd/server/main.go:95-105`). Each loop uses a Redis `SETNX` lock and is
panic-isolated (`backend/internal/worker/worker.go:645-670`). The presence-expiry
loop currently logs a tick but publishes no departure notification
(`backend/internal/worker/worker.go:233-251`).

Worker intervals and lifecycle thresholds come from configuration, including
three-minute host absence, five-minute idle grace, five-minute reactivation, and
the payment and billing intervals (`backend/internal/config/config.go:238-251`).
