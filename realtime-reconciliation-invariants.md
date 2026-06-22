# Realtime Reconciliation Invariants

Date: 2026-05-22
Status: Authoritative realtime correctness contract. Concrete behavior must match. The relevant code surfaces are `backend/internal/websocket/`, `backend/internal/events/`, `backend/internal/redis/pubsub.go`, `backend/internal/redis/presence.go`, `backend/internal/worker/worker.go`, and `frontend/hooks/useWebSocket.ts` plus `frontend/lib/ws/`.

## 0. Why this exists

WebSocket-driven UIs in QR Dining are central to live service. Phase 6 added ticket auth, scoped Redis keys, session_events sequencing, and reconciliation. Reaching production strict mode requires those guarantees written down so that any new WebSocket-touching code is judged against them, and so reconnect storms, Redis flaps, multi-tab tabs, and delayed pubsub all collapse to defined behaviors.

## 1. Vocabulary

- **Hub**: in-process Go struct in `backend/internal/websocket/hub.go` mapping `session_id` to a set of connected `Client`s.
- **Client**: a single WebSocket connection bound to one participant.
- **Pubsub**: Redis pubsub channel `org:{org_id}:branch:{branch_id}:session:{session_id}:events` that fans an event from one app instance to all instances hosting clients for that session.
- **session_events**: durable Postgres table storing every realtime event with a monotonic per-session sequence.
- **Snapshot**: the `GET /sessions/:id/snapshot` payload — authoritative session state.

## 2. Event Envelope

Every realtime event is published with this envelope:

```
{
  "event_id": "uuid",
  "sequence": "int64 (per-session monotonic, gap-free)",
  "organization_id": "int64",
  "branch_id": "int64",
  "session_id": "uuid",
  "event": "string (type)",
  "payload": "object",
  "ts": "RFC3339 timestamp"
}
```

`sequence` is allocated inside the same transaction that persists the underlying state change. There is no "publish without persist" path.

## 3. Delivery Semantics

**DEL-1 (at-least-once with replay)**: A client that maintains a WebSocket connection during normal operation sees every event for its session at least once. Duplicates may occur after reconnect; clients are required to be idempotent on `event_id`.

**DEL-2 (no out-of-order delivery)**: Sequence numbers are monotonic per session. The frontend MUST treat a received `sequence < last_applied_sequence` as a duplicate and discard. A `sequence > last_applied_sequence + 1` (a gap) triggers replay (DEL-4).

**DEL-3 (no loss during normal operation)**: As long as a client's WebSocket is connected and Redis pubsub is up, every committed event reaches it.

**DEL-4 (replay on reconnect)**: On reconnect, the client sends `last_sequence` in the ticket request payload or via the first WS frame. The server replays missed events from `session_events WHERE sequence > $last AND session_id = $sid ORDER BY sequence`. If the gap exceeds `event_replay_window` (default 200 events) the server returns a single `SNAPSHOT_AUTHORITATIVE` event and the client refetches snapshot.

**DEL-5 (snapshot authority)**: The snapshot endpoint reflects all committed state for the session. When a client's last applied sequence is lower than what the snapshot's events would imply, the snapshot wins. Any subsequent realtime event with `sequence <= snapshot_last_sequence` is discarded.

## 4. Event Sequence Invariants

**SEQ-1**: `session_events.sequence` is unique per `session_id`. Enforced by the `(session_id, sequence)` unique index in migration `000020`.

**SEQ-2**: Sequence is assigned by `SELECT COALESCE(MAX(sequence), 0) + 1 FROM session_events WHERE session_id = $1 FOR UPDATE` inside the same transaction as the underlying mutation. No application-level counter, no Redis counter.

**SEQ-3**: A failed transaction never consumes a sequence; the row is never inserted. Therefore there are no gaps in committed sequences.

**SEQ-4**: A duplicate publish (e.g., the publisher retries because Redis ack timed out) is harmless because the `event_id` is unique and the client deduplicates.

## 5. Pubsub Channels and Keys

- Events: `org:{org_id}:branch:{branch_id}:session:{session_id}:events`
- Presence: `org:{org_id}:branch:{branch_id}:session:{session_id}:presence`
- Branch staff feed (kitchen, waiter dashboards): `org:{org_id}:branch:{branch_id}:staff:feed`

Legacy unscoped keys (`session:{session_id}:events`) are still subscribed in compatibility mode; new publishes always go to the scoped channel. Listeners merge legacy + scoped during transition; once `TENANCY_ORGANIZATIONS_ENABLED` is strict for 30 days, the legacy subscribe is removed.

## 6. Hub Behavior

**HUB-1**: The hub holds Clients in-memory only. On app restart all sockets disconnect and clients reconnect — DEL-4 covers correctness.

**HUB-2**: A Client's outbound write buffer is bounded (default 256 messages). Overflow disconnects the client with code 1009. Reconnect path is the recovery.

**HUB-3**: A Client's PING/PONG keepalive runs every 30s. Missed PONG for 60s disconnects.

**HUB-4**: The hub does NOT replay events to a newly connected client. Replay is the responsibility of the snapshot endpoint or the explicit `?since=<sequence>` query on connect.

## 7. Reconnect Protocol

1. Client detects WebSocket close. Local store records `last_applied_sequence`.
2. Client calls `POST /sessions/:id/ws-ticket` with body `{ "since": last_applied_sequence }`. Server validates guest credential, issues a one-shot ticket, and stores `since` alongside it.
3. Client connects `GET /ws?ticket=...`. Server consumes ticket, recovers `since`.
4. Server emits up to `event_replay_window` events with `sequence > since`. If more than the window, server emits `SNAPSHOT_AUTHORITATIVE` and stops.
5. Client either applies replayed events in order or refetches snapshot.
6. Hub starts normal live delivery.

Backoff: client uses exponential backoff with jitter (1s, 2s, 4s, 8s, capped at 30s). The ticket endpoint is rate-limited per session.

## 8. Duplicate Event Handling (frontend)

The frontend MUST:

- Maintain `last_applied_sequence` per session.
- Discard any event with `sequence <= last_applied_sequence`.
- Apply events strictly in sequence order. A gap triggers reconnect with `since=last_applied`.
- Treat `event_id` as the dedup key for application-level idempotency in case of replay overlap.

The frontend MUST NOT:

- Re-apply business logic on duplicate `event_id`.
- Optimistically accept a higher-sequence event when a gap exists below it.

## 9. Delayed Pubsub Handling

A Redis pubsub message that arrives a long time after the underlying commit is still safe to deliver because the envelope carries `sequence`. Clients that have already moved past will discard via DEL-2. Clients with a gap will replay.

If pubsub backs up beyond several seconds, the publisher itself logs a warning and writes a `realtime_publisher_lag` metric. App instances with lag > 30 seconds should be drained.

## 10. Snapshot Reconciliation

**SNP-1**: `GET /sessions/:id/snapshot` returns:

- session row (with status)
- participants
- non-cancelled orders with current status
- assistance requests
- payment rows + bill snapshot if any
- `last_event_sequence` (max sequence applied to this session)
- session timeline summary (counts of events, not the full event log)

**SNP-2**: After snapshot, a client may safely set `last_applied_sequence = last_event_sequence`. Subsequent live events with `sequence <= last_event_sequence` are duplicates.

**SNP-3**: Snapshot may be returned for terminal sessions within the 60-minute read window (see `session-lifecycle-state-machine.md`). The snapshot is marked `session_ended=true`. Realtime subscription is disallowed for terminal sessions; the WS ticket endpoint returns 410.

## 11. Multi-Device Synchronization

**MDS-1**: All devices for a participant see the same session events. Per-device state diverges only in local UI (cart input, navigation).

**MDS-2**: An action by participant A produces an event seen by B. B's UI updates without polling.

**MDS-3**: A revoked participant immediately stops receiving events. The hub drops the connection on credential revocation (the server publishes a control event `PARTICIPANT_REVOKED` and the hub closes that participant's sockets after delivering it).

## 12. Redis Recovery Assumptions

**REC-1**: Redis is not the source of truth for any business state. Redis stores: pubsub bus, presence keys (TTL), WS tickets (TTL), idempotency cache (replaceable), rate-limit counters.

**REC-2**: After Redis loss:

- Pubsub messages in flight are lost. DEL-4 covers — clients reconnect and replay.
- Presence keys are lost. Worker reconciliation rebuilds from clients' next heartbeat.
- WS tickets are lost. Clients fetch fresh tickets.
- Rate-limit counters reset; this is acceptable post-incident.

**REC-3**: After Redis recovery, the publisher resumes. Subscribers re-subscribe with backoff. There is no "Redis recovery dance" required by application code; the realtime layer is designed to be self-healing.

**REC-4**: Sustained Redis outage (> 5 min) triggers the operational runbook (`operational-runbooks.md` section on Redis outage). The frontend degrades to polling the snapshot endpoint at 5s intervals.

## 13. Snapshot vs Live Authority

When a snapshot and a live event disagree:

- If snapshot `last_event_sequence` >= event's `sequence`, the event is a duplicate; discard.
- If snapshot is older (its `last_event_sequence` < event's sequence), apply the event normally. The snapshot is older.
- If a snapshot read happens during a write transaction, the snapshot returned reflects whichever side of the transaction the read landed on (Postgres MVCC). The next live event picks up the difference.

There is no case where snapshot and live events "conflict" in a way the client must resolve manually.

## 14. Branch Isolation in Events

**ISO-1**: Every published envelope carries `organization_id` and `branch_id`. Subscribers must filter by these even if the channel name already encodes them. Defense in depth against misrouted publishes.

**ISO-2**: Staff branch dashboards subscribe to `org:{org}:branch:{branch}:staff:feed`. A staff member is bound to one branch's feed by their staff session. Cross-branch organization admins use distinct dashboards, not staff feeds.

**ISO-3**: Cross-organization data must never appear on a channel scoped to a different organization. The publisher writes the channel name from the session's persisted org/branch; client-side channel choice is ignored.

## 15. Cart Event Specifics

**CART-1**: `CART_UPDATED` events are delivered to the entire session (so other participants see additions in shared-cart mode and so multiple devices of the same participant sync).

**CART-2**: `CART_UPDATED` is participant-scoped in payload. A device for participant A receives B's `CART_UPDATED` only if `branches.settings_json.shared_cart_enabled` is true.

**CART-3**: When the session enters `payment_pending`, cart events are no longer emitted because the cart is frozen. The first event after re-opening is `CART_UNFROZEN`.

## 16. Order Event Specifics

**ORD-1**: `ORDER_CREATED`, `ORDER_STATUS_CHANGED`, and `ORDER_CANCELLED` carry the order row and the new status. The branch staff feed receives a normalized version with sensitive guest fields stripped.

**ORD-2**: Order status changes use the expected-status SQL clause; on a no-op (already at target) no event is emitted. This prevents idle dashboards from flapping.

## 17. Assistance Event Specifics

**ASSIST-1**: `ASSISTANCE_REQUESTED` is published to both the session feed and the branch staff feed.

**ASSIST-2**: `ASSISTANCE_ACK` and `ASSISTANCE_RESOLVED` carry the acting staff identifier (operational reference, not raw UUID).

## 18. Payment Event Specifics

**PAY-1**: `PAYMENT_REQUESTED` is published when the session transitions to payment_pending. Carries snapshot summary.

**PAY-2**: `PAYMENT_STATUS_CHANGED` is published on every payment status transition. Carries payment id, method, status, and snapshot residual.

**PAY-3**: `PAYMENT_COMPLETED` plus `SESSION_CLOSED` are sequenced: payment status changes before session closes, so frontend can render "paid" before the session ends.

## 19. Session Event Specifics

**SESS-1**: `SESSION_CREATED`, `SESSION_REACTIVATED`, `SESSION_ABANDONED`, `SESSION_EXPIRED`, `SESSION_CLOSED` are published at the respective transitions. The transition itself is in `sessions.status`; the event is the realtime notification.

**SESS-2**: `SESSION_EXPIRING_SOON` is published once when the session enters the warning window. It is persisted in `session_events` so reconnecting clients still see it. The old behavior of marking `warned_at` and dropping the event on reconnect is no longer correct.

## 20. Adversarial Scenarios and Required Outcomes

| Scenario | Required outcome |
| --- | --- |
| Client receives sequence 10, then 12 | Client requests `since=10` reconnect; server replays 11, 12; client applies in order |
| Client receives sequence 8 after applying 9 | Client discards 8 (DEL-2) |
| Redis flap during order placement | Order persists; event sequence persists; pubsub publish fails or succeeds — clients reconnect and replay |
| Two app instances both publish same event_id due to retry | Clients dedup on event_id; sequence collision is impossible because sequence is allocated inside the persisting transaction |
| Client connects with stale ticket reused | Server rejects (`ErrWSTicketInvalid`); client fetches new ticket |
| Stale frontend bundle connects with legacy query auth after WS_TICKET_AUTH_REQUIRED | Server rejects 401; client redirects to refresh |
| Participant revoked mid-session | Hub closes that participant's sockets after delivering `PARTICIPANT_REVOKED`; further reconnect attempts 401 |
| Hub-local memory leak across reconnect storms | Hub bounds buffer per client; closes overflowing clients; clients reconnect with bounded backoff |
| Pubsub message arrives 60 seconds late | Clients with newer state discard; clients with gap replay |
| Subscriber app instance dies mid-message | Other instances still deliver. Dead instance's clients reconnect to a healthy instance |
| Cross-branch event misroute attempt | ISO-3: subscriber filters by `branch_id`; mismatch logged + dropped |
| Multi-tab client receives the same event on 2 tabs | Both apply; sequence-based dedup means it's harmless because tabs have separate stores |

## 21. Implementation Notes

- The current `session_events` table created in migration `000020` is the durable replay source. Ensure `sequence` is BIGINT with monotonic per-session uniqueness.
- The publisher in `backend/internal/events/events.go` must always write to `session_events` before publishing to Redis. Order matters because Redis publish is best-effort.
- The hub on `Subscribe` should read `session_events` if a `since` is provided. If the gap exceeds the window, send `SNAPSHOT_AUTHORITATIVE` and stop.
- Frontend `useWebSocket.ts` must maintain `last_applied_sequence` and use it on reconnect.
- `useWebSocket` must drop events with `sequence <= last_applied_sequence`.

## 22. Definition of Done

Realtime correctness is "done" only when:

- Every invariant above is exercised by an automated test (integration or Playwright).
- A chaos test forces Redis loss for 60s, then 5 min, and the system self-heals to consistent state in both.
- A reconnect-storm test with 200 concurrent clients reconnecting to the same session passes without dropping events.
- Cross-organization isolation is fuzzed (random session_ids across orgs) without leak.
- A multi-device test where two devices for the same participant act concurrently produces identical end-state on both after reconciliation.
