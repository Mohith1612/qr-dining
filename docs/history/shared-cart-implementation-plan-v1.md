# Shared Session Cart + Host-Controlled Ordering — Implementation Plan v1

Status: planning artifact (uncommitted). Grounds the final major workflow phase before pilot prep.

## 1. Goal & scope

Move the table from "multiple independent shopping carts" to **one collaborative dining session**:

- **Shared session cart** — a single cart per session, collaboratively edited by all participants,
  synchronized in realtime, reconnect-safe and backend-authoritative.
- **Host-controlled ordering** — the first participant is the host; only the host submits orders to
  the kitchen and initiates the bill/payment. Other participants edit the shared cart but cannot
  finalize.
- **Host reassignment** — if the host is lost, authority transfers automatically so a table never
  deadlocks.
- **Participant visibility** — host is visibly identified in the session UI.
- **Optional phone at join** — display name required, phone optional with "continue without phone".

**Out of scope (locked):** live cursors, item-ownership tracking, merge-conflict/locking systems,
granular permissions, WebSocket architecture rewrites, frontend/backend rewrites.

## 2. Current state (verified against code)

- `session_participants.is_host` and `sessions.host_participant_id` already exist; the creator is
  inserted with `is_host=true` and set as host (`services/session.go:90-97`).
- Host-only authority is already enforced for session close via `domain.ErrNotSessionHost`
  (`services/session.go:150-154`), which maps to **403 / `NOT_SESSION_HOST`** (`handlers/session.go:294`).
- Carts are **per-participant**: `GetOrCreateCart` keys on `(session_id, participant_id)`
  (`sql/queries/carts.sql:1-5`, `repository/cart.go:15-20`), constraint `UNIQUE(session_id, participant_id)`
  (`migrations/000001`).
- `CART_UPDATED` already broadcasts to the whole session room (`services/cart.go:100,140`).
- Cart already rides reconnect: snapshot reconciliation honors `snapshot_authoritative`
  (`services/session.go:354-369`, `frontend/lib/ws/connection.ts:136`). Cart is fetched from the
  backend on `CART_UPDATED` (`frontend/hooks/useWebSocket.ts`).
- Order submission only checks participant-in-session, not host (`services/order.go:76-82`).
- Payment initiation has no host check.
- `RunPresenceExpiry` is a no-op stub (`worker/worker.go:171-189`); presence lives in Redis
  (`redis/presence.go`, 90s TTL). No host reassignment exists anywhere.

## 3. State topology changes

### 3.1 Cart: per-participant → per-session

- The shared cart is the single `carts` row with `participant_id IS NULL` for a session.
- **Constraint gap:** `UNIQUE(session_id, participant_id)` does NOT enforce one shared cart because
  Postgres treats NULLs as distinct. Add a **partial unique index**:
  `CREATE UNIQUE INDEX carts_session_shared_uniq ON carts (session_id) WHERE participant_id IS NULL;`
- New query `GetOrCreateSessionCart`:
  ```sql
  INSERT INTO carts (session_id, participant_id) VALUES ($1, NULL)
  ON CONFLICT (session_id) WHERE participant_id IS NULL
  DO UPDATE SET session_id = EXCLUDED.session_id
  RETURNING *;
  ```
- `CartService.GetCart/AddItem/RemoveItem` resolve the cart by **session only**. `participantID`
  stays in the signatures for event-log attribution but no longer scopes the cart.
- `cart_items` are intentionally anonymous (no `added_by`) — item-ownership tracking is out of scope.

### 3.2 Host: already modeled, now load-bearing

- `host_participant_id` becomes the single submit/pay authority. No schema change for host itself.
- New WS event `HOST_CHANGED` carries the new host participant for live UI updates.

### 3.3 Participant identity

- Add nullable `phone_e164 TEXT` to `session_participants` (additive). Optional, format-if-present.

## 4. Reconciliation strategy

- Cart stays **backend-authoritative**. Clients never trust local cart state across a reconnect.
- On `CART_UPDATED`, clients refetch `GET /sessions/:id/cart` (already the behavior) — now returns the
  one shared cart for everyone, so all devices converge on identical state.
- On reconnect, snapshot reconciliation rehydrates session + participants + orders, honoring
  `snapshot_authoritative` (wholesale replace) vs. incremental replay. The cart is NOT carried in the
  snapshot payload, so `reconcileSnapshot` additionally refetches the shared cart from
  `GET /sessions/:id/cart` and replaces the local cart store — keeping it backend-authoritative and
  convergent after a disconnect. Host identity comes from `session.host_participant_id` +
  per-participant `is_host` in the snapshot, so host survives reconnect with no extra work; baseline
  host reassignment is also evaluated during `GetSnapshot`.
- Idempotency on order placement is unchanged; combined with single-host submit authority this
  removes the duplicate-submission surface.

## 5. Host semantics

- **Assignment:** first participant = host (unchanged).
- **Authority:** host-only `PlaceOrder` and payment initiation, enforced in the service and surfaced
  as 403; gated in the UI. Non-hosts retain full cart edit rights and bill *viewing*.
- **Persistence:** `host_participant_id` in `sessions`; `is_host` flag per participant. Survives
  reconnect via snapshot.

## 6. Reassignment rules (chosen: baseline + on-demand)

`EnsureHost(ctx, sessionID, actingParticipantID)` promotes a host, persists `host_participant_id` and
flips `is_host` in a single tx, then broadcasts `HOST_CHANGED`.

- **Baseline (permanent loss), evaluated at `GetSnapshot`:** if the current host is gone for good —
  host credential revoked or host participant no longer active — promote the **oldest active
  (non-revoked) participant**. This is the natural reconnect re-evaluation point and causes no
  flapping during transient drops.
- **On-demand (unblock a stranded table), evaluated before the host gate in `PlaceOrder` / payment:**
  if the acting participant is not the host AND the current host is **absent from live presence**
  (Redis presence lookup), promote the acting participant, then allow the action to proceed.
- **Guards:** only reassign when a valid active candidate exists; never strip a host with no
  successor. "Oldest" = earliest `joined_at` among non-revoked participants (matches
  `ListParticipantsBySession ORDER BY joined_at ASC`).
- **Deadlock avoidance:** because any present participant's action (or reconnect) re-evaluates host,
  a dead host device can never permanently block the table.

## 7. Reconnect behavior

- No new reconnect machinery. The existing ticketed reconnect + backoff + snapshot path covers cart,
  host, and participants. `HOST_CHANGED` is additive and idempotent (re-applying the same host is a
  no-op). After any reconnect the client adopts the snapshot's host + the single shared cart.

## 8. Migration strategy

- `migrations/000027_shared_session_cart.{up,down}.sql`:
  - up: `CREATE UNIQUE INDEX carts_session_shared_uniq ON carts (session_id) WHERE participant_id IS NULL;`
  - down: `DROP INDEX carts_session_shared_uniq;`
- `migrations/000028_participant_phone.{up,down}.sql`:
  - up: `ALTER TABLE session_participants ADD COLUMN phone_e164 TEXT;`
  - down: `ALTER TABLE session_participants DROP COLUMN phone_e164;`
- Both additive and safe on the live R1 soak. No NULL-participant carts exist today, so the new index
  has zero conflicts at creation.
- Regenerate sqlc after query/schema changes.
- Existing per-participant cart rows are left intact and simply unused by the new code path
  (pre-pilot, ephemeral pre-order state — no backfill required).

## 9. Rollback / risk analysis

- **Riskiest: cart topology (Phase 1).** Isolate and verify before host gating. Rollback = revert
  `CartService` to `GetOrCreateCart`; the partial index is inert when unused and can be dropped.
- **Host reassignment correctness (Phase 3).** Wrong trigger → host flapping or stranded table.
  Mitigated by baseline-on-loss + on-demand-promotion (no presence-driven background churn) and the
  successor guard.
- **Migrations** are additive with clean down-migrations; R1 soak untouched.
- **FE gating is convenience only** — backend 403 is the source of truth, so a stale client cannot
  bypass host authority.

## 10. Implementation sequencing (small atomic commits)

1. **Shared session cart** — migration + `GetOrCreateSessionCart` + sqlc + repo + `CartService`
   session-scoping. Verify multi-device sync before moving on.
2. **Host-only submission + payment** — backend host check in `PlaceOrder` and payment initiation
   (403 mapping in order/payment handlers); FE gating on `isHost` with a clear non-host message.
3. **Host reassignment** — `EnsureHost` + baseline (snapshot) + on-demand (gate) triggers;
   `HOST_CHANGED` event + FE handler.
4. **Visibility + optional phone** — host badge in the guest participant list; `phone_e164` migration
   threaded through create/join handlers + the join form with "continue without phone".
5. **Verify** — multi-device walkthrough + `go test ./...` + clean build.

## 11. Verification checklist

- Two guests on one table: host badge correct, count = 2, one shared cart, edits sync both ways.
- Only the host can send-to-kitchen and initiate payment; non-host blocked in UI and by backend 403.
- Refresh / WS drop: cart + host reconcile from snapshot; no divergence, no phantom items.
- Host disconnect: next active participant promoted (baseline on their reconnect, or on-demand when a
  present participant acts); ordering continues.
- Join with and without phone both succeed; display name still required.
- `cd backend && go test ./...` green; clean build after migrations + sqlc regen.
