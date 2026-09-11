# qr-dining

A production-grade, realtime dine-in hospitality operating system. Not a food delivery app.

A QR code on a restaurant table creates a live session. Multiple guests at the table collaborate on orders in real time. Staff — waiters, kitchen, managers — see a live dashboard. Everything is session-centric, not user-centric.

---

## Architecture

```
┌───────────────────────────────────────────────────────────┐
│                     proxy_network (external)               │
│                        nginx (existing)                    │
└──────────────────────────┬────────────────────────────────┘
                           │ port 8080 (via container name)
┌──────────────────────────▼────────────────────────────────┐
│                        app container                       │
│  cmd/server/main.go                                        │
│  ├── Gin HTTP server (middleware stack)                    │
│  │   ├── /health, /readyz, /metrics                       │
│  │   ├── /sessions, /orders, /assist, /payments ...       │
│  │   └── /ws (WebSocket upgrade)                          │
│  ├── WebSocket Hub                                         │
│  │   └── room map: session_id → {clients}                 │
│  └── Background Workers                                    │
│      ├── stale session cleaner (every 5 min)              │
│      └── presence expiry (every 5 min)                    │
└─────────────┬─────────────────────┬──────────────────────┘
              │   backend_network   │   (internal, no external access)
   ┌──────────▼──────────┐  ┌──────▼───────────┐
   │   PostgreSQL 17      │  │    Redis 7        │
   │   (source of truth) │  │  (pub/sub, cache, │
   │                     │  │   presence, RL)   │
   └─────────────────────┘  └──────────────────┘
```

**Order placed — data flow:**
```
POST /sessions/:id/orders
  → service.PlaceOrder()
    → idempotency check (return existing if key seen before)
    → BEGIN TRANSACTION
    → CreateOrder + CreateOrderItems (price snapshot)
    → COMMIT
    → publisher.OrderPlaced() → Redis PUBLISH session:{id}:events
      → Hub.broadcast → WebSocket clients in room receive ORDER_PLACED
```

**Session created — data flow:**
```
POST /sessions
  → service.CreateSession()
    → BEGIN TRANSACTION
    → SET CONSTRAINTS fk_sessions_host_participant DEFERRED
    → CreateSession (host_participant_id = NULL)
    → CreateParticipant (is_host = true)
    → SetSessionHost (sets host_participant_id)
    → UpdateTableStatus (available → occupied)
    → COMMIT (deferred FK verified here)
    → publisher.SessionCreated()
```

---

## Technology Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP router | Gin |
| Database | PostgreSQL 17 |
| Query layer | sqlc v1.31.1 (pinned via Go tool directive) |
| DB driver | pgx v5 / pgxpool |
| Cache / pub-sub | Redis 7 (go-redis/v9) |
| WebSockets | gorilla/websocket |
| Metrics | Prometheus |
| Logging | zerolog (structured JSON) |
| Migrations | golang-migrate (embedded in binary) |

---

## Prerequisites

- Docker + Docker Compose
- Go 1.26+
- `make` (optional, all commands listed below)

---

## Local Development

```bash
# 1. Start PostgreSQL and Redis
docker compose -f docker/docker-compose.yml up -d postgres redis

# 2. Copy env
cp .env.example .env
# Edit DATABASE_URL and REDIS_URL to point to local instances:
#   DATABASE_URL=postgres://qrdining:changeme_strong_password@localhost:5432/qrdining
#   REDIS_URL=redis://localhost:6379/0

# 3. Run the server (auto-runs migrations at startup)
make run
# or: go run ./cmd/server

# 4. Verify
curl localhost:8080/health   # {"status":"ok"}
curl localhost:8080/readyz   # {"status":"ready","checks":{"postgres":"ok","redis":"ok"}}

# 5. Seed test data
make seed
# Prints: restaurant/branch/table IDs, QR tokens, staff IDs, PIN: 1234
```

---

## Migrations

The server runs all pending migrations automatically at startup via `go:embed`. No external CLI required in production.

For manual control during development:

```bash
make migrate-up       # apply all pending migrations
make migrate-down     # roll back one migration
# or directly:
go run ./cmd/migrate up
go run ./cmd/migrate down 1
```

---

## Environment Variables

| Variable | Default | Required | Description |
|---|---|---|---|
| `DATABASE_URL` | — | yes | PostgreSQL connection string |
| `REDIS_URL` | — | yes | Redis connection string |
| `PORT` | `8080` | no | HTTP listen port |
| `GIN_MODE` | `release` | no | `release` or `debug` |
| `READ_TIMEOUT` | `10s` | no | HTTP read timeout |
| `WRITE_TIMEOUT` | `30s` | no | HTTP write timeout |
| `SHUTDOWN_TIMEOUT` | `15s` | no | Graceful shutdown window |
| `TRUSTED_PROXIES` | `172.16.0.0/12` | no | Comma-separated CIDR list of upstream proxies |
| `RATE_LIMIT_RPM` | `60` | no | Requests per minute per IP |
| `LOG_LEVEL` | `info` | no | `debug`, `info`, `warn`, `error` |
| `LOG_PRETTY` | `false` | no | Human-readable logs (development only) |
| `CORS_ALLOWED_ORIGINS` | _(none)_ | no | Comma-separated allowed origins |
| `STALE_SESSION_INTERVAL` | `5m` | no | How often the stale session cleaner runs |
| `PRESENCE_EXPIRY_INTERVAL` | `60s` | no | How often the presence expiry worker runs |
| `HOST_ABSENCE_GRACE` | `3m` | no | Host heartbeat age required before automatic transfer |
| `SESSION_PRESENCE_GRACE` | `60s` | no | Creation grace before a session can be considered for pausing |
| `SESSION_IDLE_GRACE` | `5m` | no | Durable participant-idle age required before pausing |
| `SESSION_REACTIVATION_WINDOW` | `5m` | no | Time a paused session remains eligible for reactivation |
| `DB_MAX_CONNS` | `20` | no | pgxpool maximum connections |
| `DB_MIN_CONNS` | `2` | no | pgxpool minimum connections |
| `DB_MAX_CONN_LIFETIME` | `1h` | no | Maximum age of a DB connection |
| `DB_MAX_CONN_IDLE_TIME` | `30m` | no | Maximum idle time for a DB connection |

---

## API Reference

### Infrastructure (no auth)

```
GET  /health    liveness probe → {"status":"ok"}
GET  /readyz    readiness probe (checks DB + Redis) → {"status":"ready","checks":{...}}
GET  /metrics   Prometheus text format
```

### Public API (rate limited at RATE_LIMIT_RPM)

**Sessions**
```
POST   /sessions                    Create session; returns session + host participant
GET    /sessions/:id                Get session by ID
DELETE /sessions/:id                Close session (host only; X-Participant-ID header)
POST   /sessions/:id/join           Join an active session
```

**Cart** (`X-Participant-ID` header required)
```
GET    /sessions/:id/cart                  Get or create cart for participant
POST   /sessions/:id/cart/items            Add item (snapshots modifier prices)
DELETE /sessions/:id/cart/items/:item_id   Remove item
```

**Orders**
```
POST   /sessions/:id/orders         Place order (idempotent via idempotency_key)
GET    /sessions/:id/orders         List orders for session
```

**Assistance**
```
POST   /sessions/:id/assist         Request waiter / bill / other
```

**Payments**
```
POST   /sessions/:id/payments               Initiate payment
POST   /webhooks/payments/:provider         Receive payment webhook (idempotent)
```

**Menu & Tables**
```
GET    /branches/:id/menu           Full menu with modifiers (Redis-cached 5 min)
GET    /tables/by-qr/:token         Table info by QR code token
```

**Staff Auth**
```
POST   /staff/auth                  PIN auth → {"token":"...","staff_id":1,"branch_id":1,"role":"owner"}
```

### Staff-Protected Routes (Bearer token required)

```
PATCH  /orders/:id/status                   Update order status (state machine validated)
PATCH  /assist/:id/ack                      Acknowledge assistance request
PATCH  /assist/:id/resolve                  Resolve assistance request

GET    /branches/:id/orders/active          Active orders for kitchen queue
GET    /branches/:id/sessions/active        Active table sessions
GET    /branches/:id/assist/active          Pending/acknowledged assistance requests
```

---

## Auth

**Guest (participant):** Send `X-Participant-ID: <int64>` header with each request. The ID is returned when creating or joining a session.

**Staff:** `POST /staff/auth` with `{"branch_id": 1, "pin": "1234"}` returns a bearer token. Send as `Authorization: Bearer <token>` on protected routes. Token expires after 8 hours; stored in Redis.

---

## WebSocket

Connect to `/ws?session_id=<uuid>` with `X-Participant-ID` header.

The Hub maintains a room per `session_id`. Events are published to `session:{id}:events` in Redis and fanned out to all WebSocket clients in the room.

**Event types:** `SESSION_CREATED`, `SESSION_CLOSED`, `PARTICIPANT_JOINED`, `PARTICIPANT_LEFT`, `CART_UPDATED`, `ORDER_PLACED`, `ORDER_CONFIRMED`, `ORDER_PREPARING`, `ORDER_READY`, `ORDER_SERVED`, `ORDER_CANCELLED`, `ASSISTANCE_REQUESTED`, `ASSISTANCE_ACKNOWLEDGED`, `ASSISTANCE_RESOLVED`, `PAYMENT_INITIATED`, `PAYMENT_COMPLETED`.

Each event is delivered as a JSON envelope:
```json
{"type":"ORDER_PLACED","session_id":"...","payload":{...},"timestamp":"..."}
```

**Reconnect:** On disconnect, reconnect to `/ws` with the same session and participant IDs. The Hub will route new events immediately. There is no replay of events missed during disconnection — clients should call `GET /sessions/:id/orders` to reconcile state on reconnect.

**Slow consumer eviction:** Clients that cannot drain their send buffer within the write timeout are disconnected to protect other room members.

---

## Redis Role

Redis is **explicitly ephemeral** infrastructure. Losing Redis degrades gracefully — the server continues serving HTTP with DB fallbacks for cached data.

| Key pattern | Purpose | TTL |
|---|---|---|
| `session:{id}:events` | Pub/sub channel for WebSocket fan-out | — |
| `presence:{session_id}:{participant_id}` | Participant heartbeat tracking | 2 min |
| `menu:{branch_id}` | Full menu JSON cache | 5 min |
| `staff:token:{token}` | Staff session (role, branch_id) | 8 h |
| `ratelimit:{ip}` | Sliding window rate limiter | 1 min |
| `worker:{name}:lock` | Redis NX lock for background workers | 270 s |

---

## Background Workers

Two goroutines run on the configured intervals (default 5 min):

**Stale session cleaner:** Finds active sessions older than 2 hours with no recent activity. Marks them `abandoned`, updates table status to `available`, and publishes `SESSION_CLOSED` events. Uses a Redis NX lock (`worker:stale_session_cleaner:lock`) to prevent duplicate runs when multiple instances are deployed.

**Presence expiry:** Placeholder for future `PARTICIPANT_LEFT` notifications on heartbeat expiry. Redis TTL manages the actual expiry.

---

## Observability

**Prometheus:** `GET /metrics` returns text format. Metrics include HTTP request rate/latency, WebSocket client count, DB pool stats, Redis cache hit/miss, and worker run counts.

**Logs:** zerolog structured JSON to stdout. Set `LOG_LEVEL=debug` to see event publish traces and SQL query timing. Set `LOG_PRETTY=true` for human-readable output during development.

**Grafana/Loki:** Prometheus metrics and JSON logs are compatible with a standard Grafana stack.

---

## Testing

```bash
# Unit tests only (no DB required). NOTE: this silently SKIPS every integration
# test — a green `make test` proves very little on its own.
make test
# or: go test ./...

# Full suite (unit + integration). `-p 1` is REQUIRED: all packages share one
# database and one Redis, so running package test binaries in parallel makes
# them truncate each other's fixtures. Without -p 1 the suite fails with
# deadlocks and foreign-key violations that are test-isolation artifacts, not
# product bugs.
TEST_DATABASE_URL=postgres://user:pass@localhost:5432/testdb \
TEST_REDIS_URL=redis://localhost:6379/0 \
  go test -count=1 -p 1 -tags integration ./...

# Same, with the race detector (what a release gate should run)
TEST_DATABASE_URL=... TEST_REDIS_URL=... go test -count=1 -p 1 -race -tags integration ./...

# Verbose, single package
TEST_DATABASE_URL=... go test -tags integration -v -p 1 ./internal/services/...
```

Integration tests skip automatically when `TEST_DATABASE_URL` is not set, and the
Redis/WebSocket-ticket tests skip when `TEST_REDIS_URL` is not set. They run
migrations on the target database before executing, so they can be pointed at any
empty PostgreSQL database — always a **throwaway** one, never a soak or production DB.

> **CI gap (as of 2026-08-04):** `.github/workflows/ci.yml` runs `go test -race ./...`
> with no Postgres/Redis service and no `-tags integration`. The integration suite
> therefore never executes in CI, and a green CI run does not cover it. Run the
> command above locally before tagging a release.

---

## Deployment

**Target:** Oracle Cloud Ampere (arm64) via Docker Compose, behind an existing nginx reverse proxy.

```bash
# Build and start all services
make docker-up

# Stop
make docker-down

# Follow app logs
make docker-logs
```

The `app` container is on two Docker networks:
- `backend` — internal network with `postgres` and `redis` containers (no host exposure)
- `proxy_network` — shared network with nginx; nginx forwards to `app:8080`

No host-bound ports are required. The multi-stage Dockerfile builds for `linux/arm64`, runs as a non-root user, and includes a healthcheck.

---

## Recovery Philosophy

**PostgreSQL is the source of truth.** All business state lives in PostgreSQL. Redis holds ephemeral operational data (pub/sub channels, presence, cache, tokens).

If Redis is lost:
- WebSocket events stop — clients miss real-time updates but can poll REST endpoints
- Menu cache misses fall through to DB — no data loss, just higher DB load
- Staff sessions expire — staff must re-authenticate
- Rate limiting fails open — all requests pass through

If the app restarts:
- Migrations run automatically at startup
- The stale session cleaner runs within the first interval after restart
- WebSocket clients must reconnect — the Hub state is in-memory only
- All durable state (sessions, orders, payments) is intact in PostgreSQL

The `event_log` table records key operational facts for debugging and auditing. It is insert-only and does not drive application state.

---

## Development Commands

```bash
make build          # go build -o server ./cmd/server
make run            # go run ./cmd/server
make test           # go test ./...
make migrate-up     # apply pending migrations
make migrate-down   # roll back one migration
make sqlc-generate  # regenerate internal/db/sqlc/ from sql/queries/
make docker-up      # docker compose up -d (all services)
make docker-down    # docker compose down
make docker-logs    # tail app container logs
make seed           # insert test restaurant, tables, staff, and menu
```

---

## Reconnect Strategy

When a WebSocket client disconnects, the server sets an `X-Reconnect-Endpoint` header on the upgrade response pointing to `GET /sessions/:id/snapshot`. Clients should use this endpoint to reconcile state on reconnect rather than replaying events.

**Three-step algorithm:**
1. Call `GET /sessions/:id/snapshot` — returns `{session, participants, orders, assistance, snapshot_at}`
2. Diff the snapshot against local state — apply additions, removals, and status changes
3. Open a new WebSocket connection — real-time updates resume from that point forward

The snapshot endpoint is read-only and requires no authentication. See `docs/reconnect-guide.md` for timeout/backoff recommendations and the full diff algorithm.

---

## Error Codes

All error responses follow `{"code": "...", "message": "..."}`. Key on `code` in client logic — `message` is human-readable and may change between versions.

| Code | Meaning |
|------|---------|
| `SESSION_NOT_FOUND` | No session with that UUID |
| `SESSION_CLOSED` | Session is already closed |
| `SESSION_ALREADY_ACTIVE` | Table already has an active session |
| `NOT_SESSION_HOST` | Operation requires the session host |
| `PARTICIPANT_NOT_FOUND` | Participant ID not found in session |
| `MENU_ITEM_NOT_FOUND` | Menu item does not exist |
| `MENU_ITEM_UNAVAILABLE` | Menu item exists but is marked unavailable |
| `ORDER_NOT_FOUND` | No order with that UUID |
| `INVALID_ORDER_TRANSITION` | Status change not allowed by state machine |
| `ASSISTANCE_NOT_FOUND` | No assistance request with that UUID |
| `INVALID_ASSISTANCE_TRANSITION` | State machine violation for assistance status |
| `PAYMENT_NOT_FOUND` | No payment with that UUID |
| `INVALID_PAYMENT_TRANSITION` | State machine violation for payment status |
| `UNAUTHORIZED` | Missing or invalid Bearer token |
| `FORBIDDEN` | Authenticated but insufficient role |
| `RATE_LIMITED` | Too many requests — back off and retry |
| `VALIDATION_ERROR` | Request body failed schema validation |
| `INTERNAL_ERROR` | Unexpected server error — safe to retry |

Per-endpoint error codes are listed in `openapi.yaml`. WebSocket event error shapes are documented in `docs/websocket-events.md`.

---

## Backup & Restore

```bash
# Create a timestamped compressed dump (pg_dump --format=custom)
DATABASE_URL=postgres://user:pass@host/db make backup
# Output: ./backups/backup_YYYYMMDD_HHMMSS.dump

# Restore from a dump (runs inside a single transaction, prints row counts after)
DATABASE_URL=postgres://user:pass@host/db make restore file=./backups/backup_20260517_120000.dump

# Upload latest backup to Cloudflare R2 (S3-compatible)
./scripts/backup_r2.sh
```

Backups are pruned automatically after `RETENTION_DAYS` days (default: 7). See `docs/backup-restore.md` for the full runbook: retention policy, cron schedule example, and disaster recovery checklist.

---

## Security Notes

- **Staff auth brute-force protection:** `POST /staff/auth` is on a separate 10 RPM rate limit group keyed per IP. Failed attempts log the token prefix (first 8 chars) and client IP.
- **PIN hashing:** bcrypt cost 12 (~100 ms per check). Current PIN is verified before rotation is allowed. PINs are never logged.
- **Token invalidation:** On staff deactivation, all Redis tokens for that staff member are invalidated in a batch delete (`SMEMBERS staff:tokens:{id}` → `DEL`). The `is_active = FALSE` DB flag is set first.
- **Body size limit:** All requests are capped at 1 MB via `http.MaxBytesReader` applied globally.
- **Trusted proxies:** `SetTrustedProxies` failure is fatal — the server will not start with a misconfigured proxy list.
- **WebSocket origin validation:** In production (`GIN_MODE=release`), only origins listed in `CORS_ALLOWED_ORIGINS` are accepted. An empty list triggers a startup warning and allows all origins (development only).
- **SQL injection:** All DB queries are generated by sqlc and use parameterized placeholders — no raw string interpolation.

---

## Scaling Boundaries

| Boundary | Notes |
|----------|-------|
| **WebSocket Hub** | Single-process, in-memory room map. Horizontal scale-out requires partitioning sessions to the same process or replacing the hub with a sharded pub/sub (e.g., Redis Streams with consumer groups). |
| **Redis** | Explicitly ephemeral. Losing Redis degrades gracefully: real-time events stop, menu cache misses fall through to DB, rate limiting fails open, staff must re-authenticate. No business data is lost. |
| **DB pool** | Configurable via `DB_MAX_CONNS` (1–200) and `DB_MIN_CONNS`. Pool stats are exposed as Prometheus gauges (`db_pool_total_conns`, `db_pool_acquired_conns`, etc.). |
| **Rate limiter** | Fixed-window counter (1-minute buckets). At window boundaries, a burst of up to 2× the limit is theoretically possible. Acceptable for typical API traffic; switch to a sliding-window counter for stricter guarantees. |
| **event_log** | Fire-and-forget — errors are logged but never propagate to callers. High write volume will increase table size; add a partition or archival job if retention beyond 90 days is needed. |
| **Order placement** | Menu items and modifiers are batch-fetched in two queries regardless of item count. |
