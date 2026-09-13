# Realtime reconciliation invariants

Last verified against code: 2026-09-13.

1. **Durable before fan-out.** A service event is appended to `session_events`
   before Redis publication. An append error stops publication; a Redis error is
   logged and cannot roll back business state (`backend/internal/events/events.go:46-83`).
2. **Per-session order.** Event append takes a transaction-scoped advisory lock,
   allocates the next sequence within that session, and stores tenant scope
   (`backend/internal/repository/session.go:200-246`).
3. **Bounded replay.** Replay selects events after `last_sequence` in ascending
   order with a 500-event limit (`backend/internal/repository/session.go:249-281`).
4. **Snapshot wins when continuity is unknown.** `last_sequence=0` or a replay
   whose first event is beyond the next expected sequence marks the full snapshot
   authoritative (`backend/internal/services/session.go:759-779`).
5. **Terminal reads are bounded.** Closed, abandoned, and expired sessions return a
   read-only snapshot for 60 minutes, then the service returns the terminal-read
   expiry error (`backend/internal/services/session.go:670-705,775-779`).
6. **Realtime remains available during settlement.** WebSockets accept `active`
   and `payment_pending`, but not `awaiting_reactivation` or terminal states
   (`backend/internal/handlers/ws.go:94-107`).
7. **Connections cannot block a room.** The single-owner hub evicts a client whose
   send channel is full and reconnects its Redis subscriber with exponential
   backoff (`backend/internal/websocket/hub.go:88-106,158-219`).
8. **Presence is independent per participant.** PING refreshes the participant's
   presence, and readers filter each timestamp at 90 seconds
   (`backend/internal/websocket/client.go:124-145`,
   `backend/internal/redis/presence.go:70-76,151-158`).

Consumers must tolerate duplicate delivery: replay can return an event a client
processed before losing its last sequence, while live Redis fan-out is not itself
a delivery acknowledgement. Clients should apply events by sequence and reconcile
from the authoritative snapshot; the server exposes both mechanisms
(`backend/internal/repository/session.go:249-281`,
`backend/internal/services/session.go:739-779`).
