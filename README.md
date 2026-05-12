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
# Unit tests (state machine validation — no DB required)
make test
# or: go test ./...

# Integration tests (require a real PostgreSQL database)
TEST_DATABASE_URL=postgres://user:pass@localhost:5432/testdb go test -tags integration ./internal/services/... ./internal/worker/...

# Run with verbose output
TEST_DATABASE_URL=... go test -tags integration -v ./internal/services/...
```

Integration tests skip automatically when `TEST_DATABASE_URL` is not set. They run migrations on the target database before executing, so they can be pointed at any empty PostgreSQL database.

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
