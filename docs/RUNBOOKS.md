# Incident runbooks

Last verified against code and configuration: 2026-09-13. These are concise
decision paths for the single-host Docker Compose deployment defined in
`deploy/vm/docker-compose.yml:21-114`. Shell procedures are **unverified in
production** unless [RECOVERY.md](RECOVERY.md) says a rehearsal was executed.
The pilot stop clocks and paper fallback take precedence:
[PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md).

Legacy command output may say “RUNBOOKS.md §8”; that pointer means
[Database outage, corruption, or migration failure](#database-outage-corruption-or-migration-failure).

## First minute

1. Start the applicable pilot clock; do not turn a bounded incident into an
   open-ended debugging session. The alert file binds explicit signals to the
   pilot criteria (`deploy/observability/prometheus-alerts.yml:287-368`).
2. Query `/health` and `/readyz`. Health proves only that the process responds;
   readiness pings both PostgreSQL and Redis and names either failed dependency
   in its JSON response (`backend/internal/handlers/health.go:21-55`).
3. Record the deployed `IMAGE_TAG`, container state, first symptom, and first
   relevant metric before changing anything. The app image is selected by
   `IMAGE_TAG` (`deploy/vm/docker-compose.yml:43-51`).
4. Prefer a previous application image for deploy regressions. Never use schema
   rollback; see [RECOVERY.md](RECOVERY.md).

## Authentication or authorization denials

Check which identity failed: guest bearer, staff bearer/cookie, or platform
bearer. They use separate validators
(`backend/internal/handlers/guest_auth.go:25-32`,
`backend/internal/middleware/staff_auth.go:18-52`,
`backend/internal/middleware/platform_auth.go:15-47`). Guest failures are labeled
in `guest_token_validation_failed_total`; central role/scope failures use
`authz_denied_total`, shadow bypass, and mismatch metrics
(`backend/internal/handlers/guest_auth.go:118-136`,
`backend/internal/handlers/authz.go:85-103`).

For staff role complaints, first record the environment's
`AUTHZ_CENTRAL_POLICY_ENFORCE` value. Scope violations always deny; role denials
are flag-gated (`backend/internal/handlers/authz.go:48-103`). The application
default is false, while manual testing sets true
(`backend/internal/config/config.go:253-263`,
`scripts/manual-testing-up.sh:62-75`). Do not flip the flag merely to clear an
incident without checking the shadow mismatch metric and the affected action.

If a staff member reports logging out but an old bearer still works, that is a
known code gap: logout clears only the cookie
(`backend/internal/handlers/staff.go:124-130`). PIN reset or deactivation revokes
server-side sessions (`backend/internal/services/staff.go:336-395`).

## Redis outage

`/readyz` returns 503 when Redis cannot be pinged
(`backend/internal/handlers/health.go:26-55`). Staff and platform token validation
begins with Redis cache lookup (`backend/internal/services/staff.go:233-246`,
`backend/internal/services/platform.go:208-220`); WebSocket fan-out and worker
locks also depend on Redis (`backend/internal/websocket/hub.go:21-34,196-219`,
`backend/internal/worker/worker.go:661-670`). Expect authentication, realtime,
rate-limited sensitive paths, presence, and background coordination to degrade.

Unverified recovery command:

```bash
cd /opt/qr-dining
docker compose restart redis
docker compose ps
curl -fsS http://localhost/readyz
```

After readiness returns, verify a new staff login and a guest snapshot/WebSocket
reconnect. The hub subscriber retries with exponential backoff capped at 30
seconds (`backend/internal/websocket/hub.go:196-219`).

## WebSocket or stale-client state

The server accepts realtime only for `active` and `payment_pending` sessions
(`backend/internal/handlers/ws.go:94-107`). Tickets recheck tenant scope,
participant membership, credential version, and revocation
(`backend/internal/handlers/ws.go:109-149`). Inspect ticket failure, reconnect,
pub/sub, inbound-drop, abusive-close, and slow-consumer-eviction metrics; each
corresponds to an implemented path
(`backend/internal/websocket/client.go:90-163`,
`backend/internal/websocket/hub.go:130-185`).

Clients recover through `GET /sessions/:id/snapshot`; replay is capped at 500
events and ordered by per-session sequence
(`backend/internal/repository/session.go:249-281`). If the requested replay has a
gap, the snapshot marks itself authoritative so the client replaces local state
(`backend/internal/services/session.go:759-779`). A restart of `app` is an
unverified containment action; it does not repair durable event data.

## iPhone guests see a broken page, Android guests do not

If the guest app is reachable over plain `http://` — a LAN host, a reverse proxy
with TLS switched off, or a QR batch that encoded `http://` — every Safari and
iOS guest gets a blank or unstyled page while Chrome and Android guests are
unaffected, because the app ships `upgrade-insecure-requests` whenever it
believes it is behind TLS (`frontend/next.config.ts:46-82`) and WebKit honours
that directive on any host, rewriting the app's own scripts and the guest
WebSocket to `https`/`wss` against a port that speaks neither. Recognise it by
the split: the table's iPhones fail and its Android phones work, and a Safari
console shows TLS handshake errors on `/_next/static/*` rather than 404s. Before
serving guests, `curl -sSD - -o /dev/null <guest-url>` and confirm the scheme is
`https` and that `Content-Security-Policy` either omits `upgrade-insecure-requests`
or the whole origin is genuinely on TLS; the paper fallback applies while it is
not ([PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md)).

## Stuck payment

A payment created by current initiation is
`requires_staff_confirmation`, and the session is frozen in `payment_pending`
(`backend/internal/services/payment.go:223-294,759-773`). The escalation worker is
alert-only and never settles or cancels money
(`backend/internal/worker/worker.go:288-310`).

Use governed staff routes, never direct SQL:

- settle: `PATCH /payments/:id/settle`;
- cancel and unfreeze: `PATCH /payments/:id/cancel`; or
- manager/owner terminal recovery: `POST /sessions/:id/force-close`.

All three are registered staff routes
(`backend/internal/server/server.go:379-392`). Cancellation derives branch from
the payment and records the reason
(`backend/internal/services/payment.go:651-691`). Force-close derives branch from
the session, requires manager/owner plus a reason, closes first, and reports
cancelled or stranded payment IDs
(`backend/internal/handlers/session.go:141-236`,
`backend/internal/services/session_close.go:34-81`).

After settlement, require that the session closes only against a fresh bill
snapshot and snapshot-scoped completed sum
(`backend/internal/services/payment.go:814-885`). After cancellation, require the
session to return to `active` only when no non-terminal payment remains
(`backend/internal/services/payment.go:672-685`).

## Payment webhook rejection or replay

The webhook is public and sensitive-rate-limited. Before parsing its JSON, the
handler verifies the configured provider secret, timestamp tolerance, and HMAC;
invalid verification returns 401 (`backend/internal/server/server.go:235-247`,
`backend/internal/handlers/payment.go:208-239`). A verified event is inserted by
external event ID with conflict suppression; a repeated ID returns success
without applying the payment again (`backend/internal/services/payment.go:464-477`,
`backend/sql/queries/payments.sql:70-80`).

For a rejection storm, record provider, response status, timestamp skew, and
whether the secret exists before changing configuration. For a replay storm,
inspect receipt rows and `idempotency_replays_total{scope="webhook"}`; do not
delete receipts to make the traffic quiet. Current initiation does not create a
provider-pending reference because every method enters staff confirmation
(`backend/internal/services/payment.go:759-773`), so receiving a valid provider
settlement against a newly initiated payment is not an expected current path.

## Stale session or occupied table

The stale cleaner selects active sessions by latest durable participant activity,
not session age (`backend/sql/queries/workers.sql:3-14`). The reactivation pipeline
moves an absent active session to `awaiting_reactivation`, skips any non-terminal
payment, then abandons after the configured window when no payment is in flight
(`backend/internal/worker/worker.go:432-508`). The table reconciler repairs
session/table mismatches and records its action
(`backend/internal/worker/worker.go:511-599`).

Check worker success/panic metrics and logs before intervening. Worker loops use
distributed locks and panic isolation (`backend/internal/worker/worker.go:645-670`).
If a real party needs the table reclaimed before workers resolve it, a manager or
owner can use the audited force-close route
(`backend/internal/handlers/session.go:141-236`).

## Tenant suspension complaint

Suspension is entry-only: it blocks QR resolution, session creation, and join,
while existing sessions continue (`backend/internal/services/tenant_status.go:33-78`).
It is not an emergency stop. Force-closing a live session is a separate staff
operation (`backend/internal/server/server.go:387-392`).

Backend suspension errors are distinct 403 codes, but the current QR frontend
collapses resolve into an invalid/expired message and create/join into a generic
message (`backend/internal/handlers/menu.go:36-58`,
`frontend/app/(guest)/table/[token]/page.tsx:57-60,109-113`). A report of the wrong
guest message is therefore a known frontend defect, not evidence that the backend
gate failed.

## Database outage, corruption, or migration failure

Readiness identifies PostgreSQL failure independently from Redis
(`backend/internal/handlers/health.go:26-55`). If the database is merely
unavailable, restart/repair PostgreSQL and recheck readiness before considering a
restore. If data is corrupt/lost, or the schema reports dirty, use
[RECOVERY.md](RECOVERY.md). Do not improvise a down migration.

The known version-22 wedge requires `migrate force 21`, temporarily dropping the
audit immutability trigger, replaying `up`, immediately recreating the trigger,
and proving an update fails. That whole sequence—not `force` alone—is exercised
by `backend/scripts/tests/backup-restore-test.sh:200-263`.

## Break-glass database read

Use a direct database read only when the application surfaces are unavailable
and the pilot fallback procedure requires durable facts. Stop application writes
first, record the operator, command, time, and incident, and use read-only SQL.
Orders retain placed price snapshots; a bill snapshot exists only after the bill
has been snapshotted for payment (`backend/migrations/000001_initial_schema.up.sql:218-250`,
`backend/migrations/000021_payment_order_correctness.up.sql:48-64`). Therefore do
not assume every open table has a `bill_snapshots` row. This operational access
procedure is **unverified**; it has not been rehearsed by the retained evidence.

## Backup failure

`nightly-backup.sh` writes last-run status on every exit and updates the
last-success timestamp only after a complete success
(`backend/scripts/nightly-backup.sh:27-79,147-150`). `BackupFailed` watches status;
`BackupTooOld` watches absence or 36-hour age
(`deploy/observability/prometheus-alerts.yml:264-285`). Check the job log, provider
configuration, object existence, manifest line, and stored-object checksum before
calling the database protected. The selection and verification procedure is in
[RECOVERY.md](RECOVERY.md).

## Billing discrepancy

The reconciliation worker compares collected completed payments with the latest
authoritative snapshot and recomputes that snapshot from its source order IDs
(`backend/sql/queries/payments.sql:123-258`). It never corrects financial state;
it emits a gauge and immutable audit evidence
(`backend/internal/worker/worker.go:132-177`). Follow the A1 clock in
[PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md); do not “fix” the row before the
evidence and affected session are recorded.

## After containment

Preserve logs, metrics, request IDs, audit rows, the deployed image tag, and any
manual payment record before repair. Record which pilot criterion fired, the
fallback time, the smallest confirmed blast radius, and every state-changing
action. The app emits request IDs and actor/tenant fields in request logs
(`backend/internal/middleware/logger.go:13-61`), and high-risk recovery routes
write immutable audit events with the acting staff member and reason
(`backend/internal/handlers/payment.go:405-420`,
`backend/internal/handlers/session.go:218-236`).
