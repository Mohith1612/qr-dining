# WebSocket Reconnect Guide

qr-dining uses a **REST-based reconciliation** strategy for WebSocket reconnects.
There is no event replay — state is always read from PostgreSQL (the source of truth)
via a single snapshot endpoint.

---

## Why REST reconciliation?

Event replay requires a persistent, ordered event log with consumer offsets.
qr-dining's `event_log` table is a fire-and-forget audit trail, not a replayable stream.

REST reconciliation is simpler, correct, and consistent:
- PostgreSQL always has the current state.
- The snapshot endpoint fetches all relevant data in parallel.
- The client simply diffs its local state against the snapshot.

---

## Reconnect Flow

```
Client                          Server
  |                               |
  |-- WebSocket disconnects ----> |
  |                               |
  |-- GET /sessions/:id/snapshot->|
  |<-- full state JSON -----------|
  |                               |
  | (client diffs local vs snapshot)
  |                               |
  |-- GET /ws?session_id=...  --->|
  |<-- 101 Switching Protocols ---|
  |   (X-Reconnect-Endpoint: ...) |
  |                               |
  | (resume normal operation)     |
```

---

## Step-by-step

### 1. Detect disconnection

Listen for the WebSocket `close` or `error` event. Implement exponential backoff
before reconnecting (start at 1s, double each attempt, cap at 30s).

### 2. Call the snapshot endpoint

```
GET /sessions/{session_id}/snapshot
```

No auth required. Returns:

```json
{
  "session": { ... },
  "participants": [ ... ],
  "orders": [ ... ],
  "assistance": [ ... ],
  "snapshot_at": "2026-05-17T10:00:00Z"
}
```

The `snapshot_at` timestamp tells you exactly when the state was captured.

### 3. Diff and reconcile

Replace your local state with the snapshot. Key things to check:

- **Session status**: if `status == "closed"`, stop reconnecting and show a "session ended" screen.
- **Orders**: diff by order ID; update statuses for any changed orders.
- **Assistance**: diff by assistance request ID; update statuses.
- **Participants**: update presence list.

The snapshot is authoritative — don't try to merge or patch; replace.

### 4. Re-establish the WebSocket

After reconciling:

```
GET /ws?session_id={session_id}&participant_id={participant_id}
```

The response headers include:

```
X-Reconnect-Endpoint: /sessions/{session_id}/snapshot
```

This header confirms where to call on the next reconnect.

### 5. Resume normal operation

You are now connected and in sync. Any events missed during the disconnect
are already reflected in the snapshot you applied in step 3.

---

## What if the session was closed during the disconnect?

`GET /sessions/:id/snapshot` returns the snapshot even for closed sessions.
Check `session.status`:

```
if snapshot.session.status == "closed" {
  // Navigate to "your session has ended" screen
  // Do not reconnect
}
```

---

## What if the snapshot endpoint is unavailable?

If the snapshot call fails (network error, 5xx), retry with backoff.
Do not reconnect the WebSocket until you have a successful snapshot —
connecting without reconciling risks displaying stale state.

---

## Timeout recommendation

- Snapshot call timeout: 10 seconds
- WebSocket reconnect backoff: 1s → 2s → 4s → 8s → 16s → 30s (capped)
- Max reconnect attempts: 10 (then show "connection lost" and require manual refresh)
