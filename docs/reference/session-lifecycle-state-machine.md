# Session lifecycle state machine

Last verified against code: 2026-09-13.

The implemented states are `active`, `payment_pending`,
`awaiting_reactivation`, `closed`, `abandoned`, and `expired`. The last
three are terminal (`backend/internal/domain/statemachine.go:19-40`). Exact allowed
edges are centralized in `backend/internal/domain/statemachine.go:82-107`.

| State | Mutability and exit behavior |
|---|---|
| `active` | Cart/order mutations are allowed subject to identity and host rules. Payment initiation may move it to `payment_pending` (`backend/internal/services/payment.go:223-259`). |
| `payment_pending` | Cart and order writes are frozen; reads and WebSockets remain available (`backend/internal/domain/statemachine.go:43-47`, `backend/internal/handlers/ws.go:94-107`). A terminally failed/cancelled last payment returns it to `active` (`backend/internal/services/payment.go:540-562`). |
| `awaiting_reactivation` | Entered only after durable inactivity plus empty live presence and no non-terminal payment; snapshot/join can reactivate it (`backend/internal/worker/worker.go:432-468`, `backend/internal/services/session.go:567-596,682-702`). |
| `closed` | Reached by host/system close, successful settlement, or staff force-close. Close releases the table and revokes/rotates participant credentials atomically (`backend/internal/services/session.go:233-315`). |
| `abandoned` | Reached by stale/reactivation workers; the worker releases state, clears presence, publishes close, and audits the action (`backend/internal/worker/worker.go:470-503,531-560`). |
| `expired` | Defined as terminal and accepted by the transition table, but this rebuild found no service/worker mutation that writes it. Treat creation of this state as **not currently implemented** (`backend/internal/domain/statemachine.go:23-40,82-107`). |

Timeout close and warning both use latest durable participant activity, falling
back to session creation only if no participant exists
(`backend/sql/queries/workers.sql:3-14`,
`backend/sql/queries/sessions.sql:110-123`). Presence is a separate 90-second
per-field decision; host reassignment waits the configured three-minute absence
and promotes only a present active participant
(`backend/internal/redis/presence.go:13-18,151-158`,
`backend/internal/services/session.go:318-359`).

A suspended/archived tenant blocks only entry (QR, create, join); it does not
transition live sessions (`backend/internal/services/tenant_status.go:33-78`). Staff
force-close is the explicit emergency/recovery action
(`backend/internal/server/server.go:387-392`).

Terminal snapshots are readable for 60 minutes, marked `session_ended`, and then
return the terminal-read expiry error
(`backend/internal/services/session.go:670-705,759-779`).
