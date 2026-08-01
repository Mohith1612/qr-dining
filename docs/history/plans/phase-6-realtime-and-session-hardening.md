# Phase 6 - Realtime and Session Hardening

## Objective

Make session lifecycle, table occupancy, WebSocket auth, Redis namespacing, and realtime reconnect behavior production-safe.

## Dependencies

- Phase 1 guest credentials.
- Phase 2 ownership validation.
- Feature flag:
  - `WS_TICKET_AUTH_REQUIRED`

## Current Codebase Context

Relevant files:

- `backend/internal/services/session.go`
- `backend/internal/handlers/session.go`
- `backend/internal/handlers/ws.go`
- `backend/internal/websocket/hub.go`
- `backend/internal/websocket/client.go`
- `backend/internal/events/events.go`
- `backend/internal/redis/pubsub.go`
- `backend/internal/redis/presence.go`
- `backend/internal/worker/worker.go`
- `backend/sql/queries/sessions.sql`
- `backend/sql/queries/workers.sql`
- `frontend/lib/ws/connection.ts`
- `frontend/lib/ws/reconciliation.ts`

Current risks:

- Duplicate active sessions per table can race.
- Stale cleanup abandons sessions without freeing tables.
- WebSocket auth uses query `session_id` and `participant_id`.
- Redis keys are not organization/branch scoped.
- Realtime events are ephemeral and only partially reconciled through snapshot.

## Session Creation Correctness

Target invariant:

```text
unique active session per table:
  unique(table_id) where status = 'active'
```

Session creation should:

1. Lock table or rely on partial unique active-session constraint.
2. Create session.
3. Create host participant.
4. Set table occupied.
5. Issue guest credential.
6. Audit.
7. Publish event after commit.

## Stale Cleanup Correctness

Worker cleanup must:

- Mark session abandoned.
- Set table available.
- Delete Redis presence.
- Publish session closed/abandoned event.
- Audit system closure.
- Run inside a transaction for DB state.

Add reconciliation worker for:

- occupied table with no active session
- active session with available table
- duplicate active sessions

## WebSocket Ticket Auth

Target handshake:

```text
POST /sessions/:id/ws-ticket
GET /ws?ticket=one_time_ticket
```

Ticket rules:

- Issued only after guest credential validation.
- TTL around 30 seconds.
- Bound to session, branch, participant, credential version, and `jti`.
- Consumed once in Redis.
- Cannot be reused after reconnect.

## Event Sequencing

Add envelope fields:

```text
event_id
sequence
organization_id
branch_id
session_id
event
payload
timestamp
```

Recommended:

- DB-backed `session_events` for replayable operational events.
- Redis Pub/Sub remains delivery bus only.
- Client reconnect sends `last_sequence`.
- Server returns missed events or authoritative snapshot.

## Redis Namespacing

Target keys:

```text
org:{org_id}:branch:{branch_id}:session:{session_id}:presence
org:{org_id}:branch:{branch_id}:session:{session_id}:events
org:{org_id}:branch:{branch_id}:menu:v{version}
worker:{region}:{job}:lock
```

## Exit Criteria

- Duplicate active sessions per table are impossible.
- Stale session cleanup does not strand occupied tables.
- WebSocket cannot be joined with spoofed participant query params.
- Reconnect can reconcile event ordering.
- Redis keys are inspectable by organization/branch/session scope.

## Implementation Plan

1. Add additive schema guardrails:
   - partial unique index on active sessions by `table_id`
   - `session_events` table with per-session monotonic sequence
2. Harden session lifecycle:
   - create sessions under table row lock and rely on the unique index for races
   - close sessions and release tables in one DB transaction
   - abandon stale sessions and release tables in one DB transaction
3. Harden WebSocket entry:
   - add `POST /sessions/:id/ws-ticket`
   - issue tickets only from valid guest credentials
   - consume tickets once from Redis during `/ws?ticket=...`
   - keep legacy query-param WebSocket entry available while `WS_TICKET_AUTH_REQUIRED=false`
4. Scope realtime infrastructure:
   - publish Redis events on `org:{org_id}:branch:{branch_id}:session:{session_id}:events`
   - preserve legacy subscription compatibility during rollout
   - write scoped presence keys while deleting legacy keys on close
5. Update guest frontend:
   - persist guest credentials returned from session create/join
   - fetch a one-time WebSocket ticket before opening the socket
   - use guest credentials for snapshot reconciliation

## Implementation Status

- Backend code implemented:
  - migration `000020_realtime_session_hardening`
  - active-session partial unique index
  - replayable `session_events` append path
  - sequenced WebSocket envelopes
  - scoped Redis Pub/Sub channels
  - one-time WebSocket ticket issue/consume flow
  - transactional close and stale-abandon table release
- Frontend code implemented:
  - guest token persistence after create/join
  - ticket-based WebSocket connection when a guest token exists
  - token-backed snapshot fetches
- Verification completed:
  - `env GOCACHE=/tmp/qr-dining-go-cache go test ./...`
  - `npm run lint` (existing warnings only)
  - `npm run typecheck`

Remaining follow-up:

- Decide when to enable `WS_TICKET_AUTH_REQUIRED` in production.

Completion follow-up implemented after commit `9b9cedc`:

- Stale cleanup now only frees a table when the target active session was actually abandoned under row locks.
- Stale cleanup deletes scoped and legacy presence, writes system audit, and records legacy operational events.
- A session/table reconciliation worker repairs occupied-without-active, active-with-available, and duplicate-active drift.
- Snapshot reconciliation accepts `last_sequence` and returns missed sequenced `session_events`.
- Frontend reconnect tracks the latest sequence and requests missed events before applying the authoritative snapshot.
- Worker locks and menu cache keys use scoped Redis names.
- Migration `000020` now repairs duplicate active sessions and table status drift before creating the partial unique index.
- Added backend unit/integration coverage for concurrent creation, stale race handling, reconciliation, event sequencing, and WebSocket ticket rejection cases.

## Test Requirements

- Concurrent session creation creates at most one active session.
- Stale worker frees table.
- Expired/consumed WebSocket ticket is rejected.
- Ticket for session A cannot join session B.
- Revoked participant cannot reconnect.
- Event sequence is monotonic.
