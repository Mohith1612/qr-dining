# WebSocket events

Last verified against code: 2026-09-13.

The endpoint is `GET /ws` (`backend/internal/server/server.go:484-485`). The standard
JSON envelope contains `event_id`, per-session `sequence`, organization and branch
scope, `session_id`, event name, optional `payload`, and timestamp
(`backend/internal/websocket/message.go:63-93`). Events are persisted before Redis
fan-out and replayed in sequence order with a limit of 500
(`backend/internal/events/events.go:46-83`,
`backend/internal/repository/session.go:200-281`).

These event types have publisher helpers and current call sites:

| Event | Current producer |
|---|---|
| `SESSION_CREATED`, `PARTICIPANT_JOINED`, `HOST_CHANGED`, `SESSION_REACTIVATED`, `SESSION_CLOSED` | Session service (`backend/internal/services/session.go:182-185,313-314,443-453,567-610`). |
| `CART_UPDATED` | Cart add/remove and post-order clear (`backend/internal/services/cart.go:99-110,140-149`, `backend/internal/services/order.go:301-311`). |
| `ORDER_PLACED`, `ORDER_CONFIRMED`, `ORDER_PREPARING`, `ORDER_READY`, `ORDER_SERVED`, `ORDER_CANCELLED` | Order placement/status transition (`backend/internal/services/order.go:301-302,348-372,438-451`). |
| `ASSISTANCE_REQUESTED`, `ASSISTANCE_ACKNOWLEDGED`, `ASSISTANCE_RESOLVED` | Assistance service (`backend/internal/services/assistance.go:28-98`). |
| `PAYMENT_INITIATED`, `PROMO_APPLIED`, `PAYMENT_COMPLETED`, `PAYMENT_CANCELLED` | Payment initiation, webhook/staff settlement, and staff cancellation (`backend/internal/services/payment.go:330-359,464-537,596-624,627-691`). |
| `PAYMENT_SETTLEMENT_STALLED` | Alert-only escalation worker (`backend/internal/worker/worker.go:355-417`). |
| `SESSION_EXPIRING_SOON` | Reactivation and timeout-warning worker paths (`backend/internal/worker/worker.go:432-468,605-623`). |
| `MENU_ITEM_AVAILABILITY_CHANGED` | Availability fan-out to every live branch session (`backend/internal/services/menu.go:270-298`). |
| `PING` / `PONG` | Client PING is parsed by the connection and produces PONG while refreshing presence (`backend/internal/websocket/client.go:124-145`). |

`ITEM_ADDED`, `ITEM_REMOVED`, and `PARTICIPANT_LEFT` are declared constants, and
`PARTICIPANT_LEFT` has a publisher helper, but this rebuild found no current producer
call. Do not build a client contract around them
(`backend/internal/websocket/message.go:13-19`,
`backend/internal/events/events.go:101-115`). The presence-expiry worker is
explicitly a no-op notification hook (`backend/internal/worker/worker.go:233-251`).

`SESSION_CREATED` uses a credential-safe session projection; its stored
`session_token` is cleared before the event and event log are written
(`backend/internal/services/session.go:68-96,182-185`).

For authentication, reconnect, and snapshot replacement rules, see
[realtime-reconciliation-invariants.md](realtime-reconciliation-invariants.md).
