# Session Lifecycle State Machine

Date: 2026-05-22
Status: Authoritative specification. Code must converge to this. Differences between this document and `backend/internal/services/session.go` or `backend/internal/worker/worker.go` are bugs.

Phase A implementation status (2026-05-22):
- New states (`payment_pending`, `awaiting_reactivation`, `expired`) added to the enum (migration `000023`).
- One-non-terminal-per-table partial unique index extended (migration `000024`).
- `payment_pending` entry/exit, cart/order freeze, and resurrection prevention (revoked_at + credential_version bump on close) are wired.
- Snapshot 60-minute terminal read window is wired (`services/session.go:GetSnapshot`, returns HTTP 410 outside the window).

Phase B implementation status (2026-05-24):
- `awaiting_reactivation` worker pipeline is wired in `RunReactivationPipeline` (migration `000026` adds the timestamp column). Worker uses Redis presence as the liveness signal and skips sessions with a non-terminal payment (TIM-1).
- Snapshot endpoint transitions `awaiting_reactivation → active` on reconnect within `SESSION_REACTIVATION_WINDOW` (default 5m).
- The worker requires the newest durable participant `last_seen_at` to be older than `SESSION_IDLE_GRACE` (default 5m), in addition to empty live Redis presence and the existing creation grace. A broader per-session HTTP `last_activity_at` signal remains a deferred refinement (Phase C if needed).

## 0. Why this exists

QR Dining sessions are long-lived (often 45-120 minutes), shared across multiple devices, and naturally bridge crashes, network drops, browser history revisits, multi-tab restoration, and abrupt staff actions. The current code stores session status as a freeform text column with values `active`, `closed`, `abandoned`. That undersells the actual lifecycle. This document defines the canonical states, transitions, triggers, side effects, and adversarial scenarios. It is migration-aware: the canonical state must be derivable from the existing columns without a destructive schema change.

## 1. Canonical States

```
                +-----------+
                |  active   |
                +-----------+
                  |  |  |
   payment_initiate|  |  |timeout_warn
                  v  |  |
       +-----------------+
       | payment_pending |--+ failed_or_cancelled (back to active)
       +-----------------+  |
                |           |
                | completed |
                v           |
            +-------+       |
            |closed |<------+
            +-------+
                                +-----------------------+
   active or payment_pending -->| awaiting_reactivation |  (grace window after disconnect)
                                +-----------------------+
                                            |
                       grace expires        |
                                            v
                                       +-----------+
                                       | abandoned |
                                       +-----------+
                                            |
                                            v
                                       +---------+
                                       | expired |
                                       +---------+
```

Six states:

- `active`
- `payment_pending`
- `awaiting_reactivation`
- `abandoned`
- `expired`
- `closed`

`closed` is terminal. `expired` and `abandoned` are also terminal but distinguish system-initiated end (`abandoned`) from end after timeout grace fully consumed (`expired`).

### Mapping to current schema

`sessions.status` continues as the durable enum. Migration target:

```
status text in ('active','payment_pending','awaiting_reactivation','abandoned','expired','closed')
```

Existing rows: `active` and `closed` stay. Existing `abandoned` stay. The new states are derived going forward; the worker writes them. No backfill needed for terminal historical rows.

## 2. Definitions

### 2.1 active

- Default state at create/join.
- Carts, orders, assistance, snapshots all permitted subject to authz.
- Table is `occupied`.
- WebSocket subscriptions allowed.
- Worker may move to `awaiting_reactivation` on stale liveness or to `expired` if timeout configured and reached.

### 2.2 payment_pending

- A bill snapshot has been created (`bill_snapshots.id` set), at least one payment row exists with status `requested` or `provider_pending` or `requires_staff_confirmation`.
- Cart additions are rejected. New orders that would change the bill total are rejected with `ERR_PAYMENT_IN_PROGRESS`.
- Existing in-flight orders that were already accepted before payment_pending are still allowed to advance through kitchen states; they form a separate, second bill snapshot if settlement is staged.
- WebSocket events flow. Reconnect allowed.
- Transition out:
  - `payment_completed` -> `closed`
  - `payment_failed` or `payment_cancelled` (all pending payments terminal-failed) -> `active`
  - `host_close` blocked while any non-terminal payment exists.

### 2.3 awaiting_reactivation

- All connected sockets have been gone for longer than `presence_grace_seconds` AND there is no new HTTP traffic from any participant for at least `quiet_grace_seconds`.
- Table remains `occupied`.
- The session is still resumable: a reconnect with a valid guest credential within `reactivation_window_seconds` transitions back to `active` (or `payment_pending` if a payment was in flight).
- New joins are blocked while in this state to avoid resurrecting a session that the table may have been physically vacated from. Staff "re-attach" is allowed (see 4.5).

### 2.4 abandoned

- The stale-session worker has decided the table is no longer in use.
- Table is released to `available` in the same transaction.
- Guest credentials for the session are invalidated (`session_participants.revoked_at` set).
- No further mutations allowed. Reads return 410 Gone.
- Audit event `session.abandon.system` written with reason and last activity timestamp.

### 2.5 expired

- The branch session timeout has been exceeded and the worker has decided to close.
- Differs from `abandoned` only by reason: `expired` means timeout policy fired with the table technically occupied right up to expiry; `abandoned` means liveness signals were already lost.
- For all practical purposes identical to `abandoned` except for the audit reason.

### 2.6 closed

- Host or staff has explicitly closed, or payment has fully completed.
- Table is `available`.
- Guest credentials invalidated.
- All future mutations rejected. Reads return read-only snapshot for a bounded window then 410.

## 3. Transition Matrix

| From | To | Trigger | Side effects |
| --- | --- | --- | --- |
| (new) | active | `POST /sessions` | row inserted, table -> occupied, host credential issued |
| active | payment_pending | `POST /sessions/:id/payments` accepted | bill snapshot created, cart frozen, audit `payment.requested` |
| payment_pending | active | all payments terminal-failed | bill snapshot retained for audit, cart unfrozen, audit `payment.cancelled` |
| payment_pending | closed | terminal payment succeeded | table -> available, credentials revoked, audit `payment.completed` + `session.close.payment` |
| active | awaiting_reactivation | stale liveness | presence keys removed, audit `session.activity_lost` |
| payment_pending | awaiting_reactivation | stale liveness | as above; payment work continues server-side |
| awaiting_reactivation | active | reconnect with valid credential within grace | resume; audit `session.reactivated` |
| awaiting_reactivation | payment_pending | as above when payment was in flight | resume payment state, audit `session.reactivated` |
| awaiting_reactivation | abandoned | grace expired | table -> available, credentials revoked, audit `session.abandon.system` |
| active or awaiting_reactivation | expired | timeout policy exceeded | identical to abandoned, reason=`timeout` |
| active | closed | host close (`DELETE /sessions/:id`) | table -> available, credentials revoked, audit `session.close.host` |
| any non-terminal | closed | staff close | as above with `session.close.staff` |
| terminal | (terminal) | no transitions out of `closed`, `abandoned`, `expired` | -- |

## 4. Reconnect Rules

### 4.1 Active reconnect

A client with a valid guest credential and a still-active session may reconnect.

- HTTP read: allowed.
- HTTP mutation: subject to authz and the active state's own rules.
- WebSocket: must issue a fresh ticket via `POST /sessions/:id/ws-ticket`. Tickets are one-shot, 30s TTL, single-use. The legacy query-string auth must be considered absent for any client that has already obtained a credential.

### 4.2 Payment-pending reconnect

Same as active, plus:

- The first snapshot reply must include the `payment_pending` flag and the current payment row(s) with their state.
- The frontend cart store must lock immediately to reflect server state; any local optimistic cart edits since payment started are discarded with a user-visible toast.

### 4.3 Awaiting-reactivation reconnect

- Within `reactivation_window_seconds`: the snapshot endpoint flips the state back to active or payment_pending. This is the only HTTP endpoint allowed to write a state transition out of awaiting_reactivation.
- Outside the window: the snapshot endpoint returns 410 with reason `session_abandoned`.
- A reconnect with a credential issued before host transfer (older `credential_version`) is rejected even within the window.

### 4.4 Stale credential

A credential whose `credential_version` is below the participant's current `credential_version`, or whose `revoked_at` is set, is rejected on every endpoint including read. Any UI must respond by clearing local state and redirecting to QR rescan.

### 4.5 Staff re-attach

Staff with `branch.session.read` can view any session in their branch. Staff cannot resurrect a terminal session. If staff needs to reopen a closed session for support, they must create a new session — never reuse the old session ID.

## 5. Multi-Tab and Browser-Restore Behavior

Adversarial assumptions: a participant may have the session open in 3 tabs, two on phone (one in incognito), one on laptop. The browser may restore tabs after restart.

Rules:

- Every tab uses the same `participant_id` but may hold a different access token expiry; tabs MUST refresh credential via `GET /sessions/:id/snapshot` on focus.
- Cart actions are last-writer-wins server side; the cart is per-participant, so concurrent tabs may produce inconsistent local state until WS event `CART_UPDATED` reconciles them. The server is authoritative.
- Order placement is participant-scoped idempotent (`idempotency_keys.scope = session:{id}`, `actor = participant:{id}`). Two tabs submitting the same order with the same client-generated idempotency key get one order. Two tabs submitting different idempotency keys can place two orders, intentional.
- Reading a stale snapshot in a restored tab: the tab MUST refresh on visibility change and discard cart state if the server says the session is terminal.

## 6. Stale Browser Revisit

A user closes the browser without paying, returns next day, the browser restores the tab.

- The session is `abandoned` or `expired` by then.
- The snapshot endpoint returns 410 with reason and a CTA payload (`requeue_qr_url`).
- The frontend MUST NOT auto-redirect to a generic landing; it must show "this session has ended" with a single explicit "scan QR to start new session" button.
- The browser localStorage/sessionStorage entries for the old session are cleared on this response.

## 7. Resurrection Prevention

Closed sessions must never be reusable. Mechanisms in code:

- `CloseSessionIfActive` only matches `WHERE id = $1 AND status = 'active'` (or the equivalent allowed prior state for system close). It returns `pgx.ErrNoRows` if the row is already terminal, which is treated as idempotent.
- The partial unique index `idx_sessions_one_active_per_table ON sessions(table_id) WHERE status='active'` prevents recreating an active row on the same table while one exists; combined with terminal status, an attempt to "reopen" the same row never matches the WHERE clause.
- Guest credentials carry the session ID and the `credential_version`. On terminal transitions, `session_participants.revoked_at` is set, so the credential validator rejects even technically valid signatures.
- All terminal transitions emit `qr_token.rotate` audit when the table QR token is rotated as part of close. Even if a stale QR were used, the table token validator rejects it.

## 8. Table Release Rules

Table status transitions strictly mirror session terminality:

- session enters `active` -> table `occupied`.
- session enters `payment_pending` -> table remains `occupied`.
- session enters `awaiting_reactivation` -> table remains `occupied`.
- session enters terminal (`closed`, `abandoned`, `expired`) -> table `available`, in the same transaction.

The reconciliation worker (`backend/internal/worker/worker.go`) runs every `SESSION_RECONCILE_INTERVAL` and corrects three classes of drift:

1. Table `occupied` but no non-terminal session on it -> release table, audit `worker.table_released_orphan`.
2. Session non-terminal but table not `occupied` -> mark session `abandoned` with reason `table_state_drift`.
3. Two non-terminal sessions on the same table -> impossible due to the partial unique index, but the worker logs a critical alert if observed (would imply index corruption).

## 9. Grace Periods (defaults; per-branch override permitted)

| Knob | Default | Configurable per branch |
| --- | --- | --- |
| `presence_grace_seconds` | 60 | yes |
| `quiet_grace_seconds` | 180 | yes |
| `reactivation_window_seconds` | 300 | yes |
| `branch.session_timeout_minutes` | already present, default 120 | yes |
| Payment-pending stuck threshold | 600 | yes |

Branches with bar/lounge usage should raise `reactivation_window_seconds` to ~600. Quick-service should lower it.

## 10. Payment Interruption Behavior

Scenarios:

- Guest network drops during `POST /sessions/:id/payments`: idempotency key on the request makes retry safe. Server returns existing pending payment on retry.
- Guest closes app between request and webhook: payment remains `provider_pending` or `requires_staff_confirmation`. Reconnect within `reactivation_window_seconds` resumes; otherwise the next snapshot returns 410 with bill state preserved for staff dispute handling.
- Staff settles cash but webhook for a digital attempt arrives later: the digital payment moves to `failed` because the bill snapshot already has a winning settlement; webhook is recorded but does not transition session.
- Two participants initiate payment concurrently: bill snapshot creation is serialized by transaction; the second request returns the existing snapshot. Two payment rows can exist if they reference the same snapshot, but only one can succeed and settle the snapshot; the loser is marked `cancelled`.

## 11. Credential Invalidation Rules

- Terminal session -> all participants' credentials revoked.
- Host transfer -> outgoing host's `credential_version` incremented, new credential issued; previous host's old credential is rejected.
- Participant removed by host or staff -> that participant's `revoked_at` set.
- Branch suspension -> all sessions in the branch transitioned to `abandoned` with reason `branch_suspended`, all credentials revoked.

## 12. Stale Websocket Reconnect Storm

If a Redis hiccup causes mass WS disconnect:

- Tickets are short-lived; only valid clients can reissue. There is no thundering herd risk to the API beyond ticket issuance.
- The ticket endpoint must rate-limit per session at most N requests per minute (see `security-hardening-checklist.md`). Clients that exceed it get exponential backoff via 429.
- The hub must accept reconnection with an `If-Last-Sequence` parameter; missed events between disconnect and reconnect are replayed from `session_events` if the gap is small, or the snapshot endpoint is the recovery path otherwise.

## 13. Closed/Abandoned Reads

For 60 minutes after terminal transition the snapshot endpoint returns 200 with a read-only payload marked `session_ended=true` plus the closing reason. After 60 minutes it returns 410. This window lets staff dispute handling and the customer's own receipt screen work without immediately killing requests.

## 14. Adversarial Scenarios and Required Outcomes

| Scenario | Required outcome |
| --- | --- |
| Guest replays an old session URL after `abandoned` | 410 with `session_ended` reason |
| Guest uses a credential whose participant was removed | 401 `credential_revoked`, even if signature is valid |
| Two QR scans land at the table in the same second | One creates a session; the other gets `TABLE_OCCUPIED` 409 with a `join` CTA |
| Stale browser tab restored after close | snapshot returns 410, frontend clears state |
| Network flap during webhook | webhook idempotency key + `MarkWebhookProcessed` prevent double-settle |
| Staff manually closes during payment_pending | rejected unless all payments are terminal; explicit `force_close` requires `payment.refund` role and audits |
| Worker tries to abandon a session while a payment webhook is in-flight | worker uses `SELECT ... FOR UPDATE`; webhook commits first; worker re-checks status and skips |
| Host transfer races with a non-host trying to close | Non-host close requires `session.close.host`; non-host loses on policy check; host transfer increments `credential_version` and revokes old close attempts mid-flight |

## 15. Implementation Tasks Required

These are deltas vs. current code:

- Add `payment_pending`, `awaiting_reactivation`, `expired` to the `sessions.status` enum/check constraint in a new migration. Backfill is identity.
- Update `services/session.go` to write the new statuses where appropriate.
- Update worker logic in `backend/internal/worker/worker.go` to use `awaiting_reactivation -> abandoned` rather than direct `active -> abandoned` in cases where presence has been lost but quiet grace has not.
- Add `session_participants.revoked_at` if not present; current schema may already have it.
- Update guest token validator to reject when `revoked_at IS NOT NULL`.
- Update snapshot handler to return 410 with reason on terminal sessions older than the 60-minute read window.
- Add unit tests for every transition in the matrix.

## 16. Definition of Done

The state machine is "done" only when:

- Every transition above is exercised by an integration test in `backend/internal/services/`.
- The worker reconciliation loop has been run under chaos (forced Redis drop, forced webhook delay) and the invariants hold.
- The frontend respects the closed/abandoned read window and CTA.
- An attempt to resurrect a closed session via any combination of stored credential, replayed URL, or admin endpoint fails closed.
