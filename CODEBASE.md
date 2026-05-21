# qr-dining Codebase Overview

## What It Is

`qr-dining` is a **session-centric realtime dine-in hospitality operating system**. It is not a food delivery app.

The core concept: a QR code on a restaurant table creates a live **session**. Multiple guests scan the code, join the session, and collaboratively browse the menu, add to a shared cart, and place orders. Staff (waiters, kitchen, managers) see a live dashboard of all tables. All state changes propagate in real time via WebSocket.

**Target deployment:** Oracle Cloud Ampere arm64, Docker Compose, behind nginx reverse proxy.

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
| Migrations | golang-migrate (embedded in binary via `go:embed`) |

---

## Architecture

```
nginx (external, existing)
    │ port 8080
    ▼
┌──────────────────────────────────────────┐
│           app container                  │
│                                          │
│  cmd/server/main.go                      │
│  ├── Gin HTTP server                     │
│  │   ├── /health, /readyz, /metrics      │
│  │   ├── 27 HTTP routes                  │
│  │   └── /ws (WebSocket upgrade)         │
│  ├── WebSocket Hub                       │
│  │   └── rooms: session_id → clients     │
│  │   └── Redis PubSub subscriber         │
│  └── Background Workers                  │
│      ├── stale session cleaner (5m)      │
│      ├── presence expiry (60s)           │
│      └── DB pool stats poller (30s)      │
└──────────────────────────────────────────┘
         │                   │
         ▼                   ▼
  PostgreSQL 17         Redis 7
  (source of truth)    (ephemeral: pub/sub,
                        cache, presence,
                        rate limiting,
                        staff tokens)
```

### Data Flow — Order Placed

```
POST /sessions/:id/orders
  → OrderService.PlaceOrder()
    → idempotency check (return existing if key seen before)
    → validate session is active
    → batch-fetch menu items + modifiers (2 queries regardless of item count)
    → BEGIN TRANSACTION
    → CreateOrder + CreateOrderItem × N (price/modifier snapshots)
    → COMMIT
    → publisher.OrderPlaced() → Redis PUBLISH session:{id}:events
      → Hub.broadcast → WebSocket clients in room receive ORDER_PLACED
```

### Data Flow — Session Created

```
POST /sessions
  → SessionService.CreateSession()
    → BEGIN TRANSACTION
    → SET CONSTRAINTS fk_sessions_host_participant DEFERRED
    → CreateSession (host_participant_id = NULL)
    → CreateParticipant (is_host = true)
    → SetSessionHost
    → UpdateTableStatus (available → occupied)
    → COMMIT (deferred FK verified here)
    → publisher.SessionCreated()
```

---

## Layer Breakdown

### `cmd/server/main.go`
Entry point. Wires all dependencies:
1. Loads and validates config from env vars
2. Connects to PostgreSQL (pgxpool) and runs embedded migrations
3. Connects to Redis
4. Initializes Prometheus metrics, zerolog logger
5. Creates `Server` (Gin routes, middleware, services, handlers, Hub, workers)
6. Starts WebSocket Hub goroutine, background workers
7. Listens with graceful shutdown on SIGINT/SIGTERM

### `internal/config/`
Loads all config from environment variables with validation:
- Port 1–65535, RateLimitRPM > 0, GinMode ∈ {debug,test,release}
- DB pool: MaxConns 1–200, MinConns ≤ MaxConns
- ReadTimeout < WriteTimeout
- Warns to stderr when `CORS_ALLOWED_ORIGINS` empty in release mode

### `internal/db/`
- `pool.go` — creates and configures pgxpool
- `migrations.go` — runs golang-migrate with embedded SQL files from `migrations/`
- `sqlc/` — generated Go code from SQL queries (never edit manually)

### `internal/repository/`
Thin wrapper over sqlc generated code. Responsibilities:
- Map `pgx.ErrNoRows` to typed domain errors (`domain.ErrSessionNotFound` etc.)
- Provide `WithTx(ctx, fn)` for transaction scoping
- `LogEvent` — fire-and-forget audit log writes (errors logged but never propagated)

Key repos: `session`, `order`, `cart`, `assistance`, `payment`, `menu`, `staff`, `participant`, `table`, `event_log`, `worker`

### `internal/domain/`
- `errors.go` — all typed sentinel errors (ErrSessionNotFound, ErrMenuItemNotFound, etc.)
- `statemachine.go` — allowed status transitions for orders, assistance, payments. All transition validation flows through here.

### `internal/services/`
Business logic. One service per domain entity. Services:
- Call repository methods
- Enforce business rules (state machine, idempotency)
- Publish events via `events.Publisher`
- Write to audit log via `repos.LogEvent`
- Record Prometheus metrics

| Service | Key responsibilities |
|---|---|
| `SessionService` | Create/close/list sessions, host participant management |
| `OrderService` | Place orders (idempotent), update status (state machine), batch-fetch menu items |
| `CartService` | Get/create cart, add/remove items with modifier snapshots |
| `AssistanceService` | Create assistance requests, acknowledge/resolve (state machine) |
| `PaymentService` | Initiate payments, process webhooks (idempotent) |
| `StaffService` | PIN auth (bcrypt 12), token management in Redis, create/rotate/deactivate |
| `MenuService` | Read menu (Redis-cached 5 min), write categories/items, invalidate cache |
| `ParticipantService` | Join session, update presence heartbeat |

### `internal/handlers/`
HTTP handlers (one file per domain). Each handler:
1. Parses and validates path params / request body
2. Calls the appropriate service method
3. Maps domain errors to structured API error responses (`{"code":"...","message":"..."}`)

`errors.go` defines all 19 stable error code constants. **Frontend clients must key on `code`, not `message`.**

### `internal/middleware/`
- `cors.go` — CORS headers for allowed origins
- `logger.go` — zerolog request logging with request ID
- `maxbodysize.go` — 1 MB global body limit
- `metrics.go` — Prometheus HTTP request counter + duration histogram
- `ratelimit.go` — Redis fixed-window rate limiter (60 RPM default, 10 RPM on staff auth)
- `recover.go` — panic recovery
- `requestid.go` — UUID request ID injection
- `staff_auth.go` — Bearer token validation via Redis; injects `StaffSession` into Gin context

### `internal/websocket/`
- `hub.go` — single-goroutine room map owner; `Run()` processes register/unregister/broadcast events; `runSubscriber()` maintains Redis PubSub subscription with exponential-backoff reconnect (restarts automatically on unexpected channel close)
- `client.go` — per-connection struct; `writePump` goroutine drains send channel; `readPump` goroutine handles pings; slow-consumer eviction via non-blocking send
- `message.go` — event envelope type

### `internal/redis/`
- `client.go` — creates go-redis client from URL
- `cache.go` — generic JSON get/set with Prometheus cache hit/miss counters
- `presence.go` — participant heartbeat keys with TTL
- `pubsub.go` — pattern subscribe (`session:*:events`), deliver decoded messages to Hub channel
- `ratelimit.go` — `AllowWithPrefix(ctx, prefix, ip, limit)` — Redis pipeline INCR + EXPIRE

### `internal/observability/`
- `metrics.go` — custom Prometheus registry with all metrics (HTTP, WebSocket, DB pool, Redis, order lifecycle, session duration, idempotency replays, worker panics)
- `logger.go` — zerolog setup (level, pretty mode)

### `internal/worker/`
- `worker.go` — two workers: `RunStaleSessionCleaner` and `RunPresenceExpiry`
- Each tick wrapped in `safeRun()` for panic recovery; each DB op wrapped in 30s context timeout
- `runWithLock()` uses Redis NX for distributed lock (prevents duplicate runs when scaled horizontally)

### `internal/events/`
- `publisher.go` — `Publisher` wraps `PubSub.Publish`; one method per event type; encodes `{type, session_id, payload, timestamp}` envelope and publishes to `session:{id}:events`

---

## All 27 HTTP Routes

### Infrastructure (no auth)
| Method | Path | Description |
|---|---|---|
| GET | `/health` | Liveness probe → `{"status":"ok"}` |
| GET | `/readyz` | Readiness probe (checks DB + Redis) → `{"status":"ready","checks":{...}}` |
| GET | `/metrics` | Prometheus text format |

### Sessions (rate-limited)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/sessions` | — | Create session; returns session + host participant |
| GET | `/sessions/:id` | — | Get session by UUID |
| DELETE | `/sessions/:id` | `X-Participant-ID` | Close session (host only) |
| POST | `/sessions/:id/join` | — | Join session; returns participant |
| GET | `/sessions/:id/snapshot` | — | Full state snapshot for reconnect reconciliation |

### Cart (rate-limited)
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/sessions/:id/cart` | `X-Participant-ID` | Get or create cart |
| POST | `/sessions/:id/cart/items` | `X-Participant-ID` | Add item (snapshots modifier prices) |
| DELETE | `/sessions/:id/cart/items/:item_id` | `X-Participant-ID` | Remove item |

### Orders (rate-limited)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/sessions/:id/orders` | `X-Participant-ID` | Place order (idempotent via idempotency_key) |
| GET | `/sessions/:id/orders` | — | List orders for session |
| PATCH | `/orders/:id/status` | `Bearer` | Update order status (state machine validated) |

### Assistance (rate-limited)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/sessions/:id/assist` | — | Request waiter / bill / other |
| PATCH | `/assist/:id/ack` | `Bearer` | Acknowledge request |
| PATCH | `/assist/:id/resolve` | `Bearer` | Resolve request |

### Payments (rate-limited)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/sessions/:id/payments` | — | Initiate payment |
| POST | `/webhooks/payments/:provider` | — | Receive webhook (idempotent) |

### Menu & Tables (rate-limited)
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/branches/:id/menu` | — | Full menu with modifiers (Redis-cached 5 min) |
| GET | `/tables/by-qr/:token` | — | Table by QR token |

### Staff Auth (10 RPM per IP)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/staff/auth` | — | PIN auth → `{"token","staff_id","branch_id","role"}` |

### Staff Dashboard (Bearer required)
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/branches/:id/orders/active` | `Bearer` | Active kitchen queue (branch-scoped) |
| GET | `/branches/:id/sessions/active` | `Bearer` | Active table sessions (branch-scoped) |
| GET | `/branches/:id/assist/active` | `Bearer` | Pending/acknowledged assistance (branch-scoped) |

### Menu Management (Bearer, owner/manager only)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/branches/:id/menu/categories` | `Bearer` | Create category |
| POST | `/branches/:id/menu/items` | `Bearer` | Create menu item |
| PATCH | `/menu/items/:id` | `Bearer` | Update item |
| PATCH | `/menu/items/:id/availability` | `Bearer` | Toggle availability |

### Staff Management (Bearer, owner only)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/branches/:id/staff` | `Bearer` | Create staff member |
| PATCH | `/staff/:id/pin` | `Bearer` | Rotate PIN |
| PATCH | `/staff/:id/deactivate` | `Bearer` | Soft-deactivate + batch token invalidation |

### Event Log (Bearer required)
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/sessions/:id/events` | `Bearer` | Session audit timeline (max 200) |
| GET | `/branches/:id/events/recent` | `Bearer` | 100 most recent branch events |

### WebSocket
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/ws?session_id=<uuid>` | `X-Participant-ID` | Upgrade; sets `X-Reconnect-Endpoint` header |

---

## Auth

**Guest (participant):**
- Send `X-Participant-ID: <int64>` header on participant-scoped requests
- ID is returned when creating or joining a session

**Staff:**
- `POST /staff/auth` with `{"branch_id": 1, "pin": "1234"}` → returns Bearer token
- Send `Authorization: Bearer <token>` on protected routes
- Token expires after 8 hours; stored in Redis at `staff:token:{token}`
- Deactivation invalidates all tokens in batch: `SMEMBERS staff:tokens:{id}` → `DEL`

---

## WebSocket Events

Connect to `/ws?session_id=<uuid>` with `X-Participant-ID` header.

All events arrive as a JSON envelope:
```json
{
  "type": "ORDER_PLACED",
  "session_id": "...",
  "payload": {...},
  "timestamp": "2026-05-17T10:00:00Z"
}
```

| Event Type | Triggered By |
|---|---|
| `SESSION_CREATED` | POST /sessions |
| `SESSION_CLOSED` | DELETE /sessions/:id or stale cleaner |
| `PARTICIPANT_JOINED` | POST /sessions/:id/join |
| `PARTICIPANT_LEFT` | Presence expiry (future) |
| `CART_UPDATED` | POST/DELETE /sessions/:id/cart/items |
| `ORDER_PLACED` | POST /sessions/:id/orders |
| `ORDER_CONFIRMED` | PATCH /orders/:id/status → confirmed |
| `ORDER_PREPARING` | PATCH /orders/:id/status → preparing |
| `ORDER_READY` | PATCH /orders/:id/status → ready |
| `ORDER_SERVED` | PATCH /orders/:id/status → served |
| `ORDER_CANCELLED` | PATCH /orders/:id/status → cancelled |
| `ASSISTANCE_REQUESTED` | POST /sessions/:id/assist |
| `ASSISTANCE_ACKNOWLEDGED` | PATCH /assist/:id/ack |
| `ASSISTANCE_RESOLVED` | PATCH /assist/:id/resolve |
| `PAYMENT_INITIATED` | POST /sessions/:id/payments |
| `PAYMENT_COMPLETED` | POST /webhooks/payments/:provider (completed event) |

**Reconnect algorithm:**
1. Call `GET /sessions/:id/snapshot` → `{session, participants, orders, assistance, snapshot_at}`
2. Diff snapshot against local state — apply additions, removals, status changes
3. Open new WebSocket connection — real-time updates resume

The `X-Reconnect-Endpoint` header is set on every WebSocket upgrade response pointing to the snapshot URL.

---

## Redis Key Patterns

| Key | Purpose | TTL |
|---|---|---|
| `session:{id}:events` | Pub/sub channel for WebSocket fan-out | — |
| `presence:{session_id}:{participant_id}` | Participant heartbeat | 2 min |
| `menu:{branch_id}` | Full menu JSON cache | 5 min |
| `staff:token:{token}` | Staff session (role, branch_id) | 8 h |
| `staff:tokens:{staff_id}` | Set of active token keys per staff member | 8 h + 1 min |
| `ratelimit:{prefix}:{ip}` | Fixed-window rate limiter counter | 90 s |
| `worker:{name}:lock` | Redis NX distributed worker lock | 270 s |

**Redis is explicitly ephemeral.** Losing Redis degrades gracefully:
- WebSocket events stop (clients must poll REST)
- Menu cache misses fall through to DB
- Rate limiting fails open
- Staff must re-authenticate

---

## Database Schema

4 migrations, sequential:

**000001** — Full initial schema: `restaurants`, `branches`, `tables`, `staff`, `menu_categories`, `menu_items`, `item_modifiers`, `session_participants`, `sessions`, `carts`, `cart_items`, `orders`, `order_items`, `assistance_requests`, `payments`. All 8 custom ENUMs.

**000002** — `event_log` table: insert-only audit log. Fire-and-forget; errors never propagate to callers.

**000003** — `webhook_events` table with `external_event_id UNIQUE` for idempotent webhook processing.

**000004** — `ALTER TABLE staff ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE`.

**Key design decisions in schema:**
- `sessions.host_participant_id` FK is `DEFERRABLE INITIALLY DEFERRED` — breaks circular dependency between sessions and participants; both are inserted in a single transaction
- `cart_items.selected_modifiers_json` and `order_items.selected_modifiers_json` are JSONB snapshots of `[{id, name, price_delta}]` — modifier prices are captured at write time, not runtime
- `orders.idempotency_key UNIQUE` — enforces idempotency at DB level
- `webhook_events.external_event_id UNIQUE` — enforces idempotent webhook processing at DB level

---

## Error Response Contract

All non-2xx responses:
```json
{"code": "SESSION_NOT_FOUND", "message": "session not found"}
```

**Frontend MUST key on `code`. `message` may change between versions.**

| Code | HTTP Status | Meaning |
|---|---|---|
| `SESSION_NOT_FOUND` | 404 | No session with that UUID |
| `SESSION_CLOSED` | 409 | Session is already closed/abandoned |
| `SESSION_ALREADY_ACTIVE` | 409 | Table already has an active session |
| `NOT_SESSION_HOST` | 403 | Operation requires session host |
| `PARTICIPANT_NOT_FOUND` | 404 | Participant ID not in session |
| `MENU_ITEM_NOT_FOUND` | 404 | Menu item (or modifier) does not exist |
| `MENU_ITEM_UNAVAILABLE` | 422 | Menu item exists but marked unavailable |
| `CART_ITEM_NOT_FOUND` | 404 | Cart item not found in participant's cart |
| `ORDER_NOT_FOUND` | 404 | No order with that UUID |
| `INVALID_ORDER_TRANSITION` | 422 | Status change not allowed by state machine |
| `ASSISTANCE_NOT_FOUND` | 404 | No assistance request with that ID |
| `INVALID_ASSISTANCE_TRANSITION` | 422 | State machine violation |
| `PAYMENT_NOT_FOUND` | 404 | No payment with that ID |
| `INVALID_PAYMENT_TRANSITION` | 422 | State machine violation |
| `UNAUTHORIZED` | 401 | Missing or invalid Bearer token |
| `FORBIDDEN` | 403 | Authenticated but insufficient role |
| `RATE_LIMITED` | 429 | Too many requests — back off and retry |
| `VALIDATION_ERROR` | 400 | Request body failed validation |
| `INTERNAL_ERROR` | 500 | Unexpected server error — safe to retry |

Per-endpoint error codes are in `openapi.yaml`. WebSocket events are in `docs/websocket-events.md`.

---

## State Machines

All defined in `internal/domain/statemachine.go`.

**Order status:** `pending → confirmed → preparing → ready → served`; `pending/confirmed/preparing → cancelled`

**Assistance status:** `pending → acknowledged → resolved`

**Payment status:** `pending → completed | failed | refunded`

**Session status:** `active → closed | abandoned` (abandoned set by stale session cleaner)

---

## Background Workers

**Stale session cleaner** (every 5 min, distributed NX lock):
- Finds active sessions older than 2 hours with no activity
- Marks `status = abandoned`, updates table to `available`
- Publishes `SESSION_CLOSED` event

**Presence expiry** (every 60 s):
- Placeholder for `PARTICIPANT_LEFT` notifications on heartbeat timeout
- Redis TTL manages actual key expiry

**DB pool stats poller** (every 30s):
- Sets `db_pool_total_conns`, `db_pool_idle_conns`, `db_pool_acquired_conns` Prometheus gauges

---

## Observability

**Prometheus metrics at `GET /metrics`:**
- HTTP: `http_requests_total{method,path,status}`, `http_request_duration_seconds`
- WebSocket: `ws_connections_active`, `ws_messages_sent_total{event}`, `ws_client_evictions_total`, `ws_reconnects_total`
- Redis: `redis_ops_total`, `cache_hits_total`, `cache_misses_total`, `redis_pubsub_connected`, `redis_pubsub_errors_total{operation}`, `redis_reconnects_total`
- DB pool: `db_pool_total_conns`, `db_pool_idle_conns`, `db_pool_acquired_conns`, `db_pool_acquire_count_total`
- Business: `order_lifecycle_duration_seconds{from_status,to_status}`, `session_duration_seconds`, `idempotency_replays_total{entity}`
- Workers: `worker_runs_total{worker}`, `worker_panics_total{worker}`

**Logs:** zerolog structured JSON to stdout. `LOG_LEVEL=debug` shows event publish traces and SQL timing. `LOG_PRETTY=true` for readable dev output.

---

## Scaling Boundaries

| Boundary | Notes |
|---|---|
| **WebSocket Hub** | Single-process in-memory room map. Horizontal scale requires session-affinity or replacing Hub with Redis Streams consumer groups. |
| **Redis** | Ephemeral. Losing Redis degrades gracefully — no business data lost. |
| **DB pool** | Configurable 1–200 connections via `DB_MAX_CONNS`. |
| **Rate limiter** | Fixed-window (1-min buckets). At window boundaries a burst of up to 2× the limit is possible. |
| **event_log** | Fire-and-forget audit. High write volume grows the table; add partitioning for >90-day retention. |

---

## Development Commands

```bash
make build           # go build -o server ./cmd/server
make run             # go run ./cmd/server (auto-migrates at startup)
make test            # go test ./...
make migrate-up      # apply pending migrations
make migrate-down    # roll back one migration
make sqlc-generate   # regenerate internal/db/sqlc/ from sql/queries/
make seed            # insert test restaurant, tables, staff (PIN: 1234), sessions, orders
make docker-up       # docker compose up -d (all services)
make docker-down     # docker compose down
make docker-logs     # tail app container logs
make backup          # pg_dump --format=custom to ./backups/
make restore file=<path>  # pg_restore --single-transaction
```

---

## Key Files Reference

| Purpose | Path |
|---|---|
| Entry point | `cmd/server/main.go` |
| Config | `internal/config/config.go` |
| Domain errors | `internal/domain/errors.go` |
| State machines | `internal/domain/statemachine.go` |
| API error codes | `internal/handlers/errors.go` |
| WebSocket Hub | `internal/websocket/hub.go` |
| Redis PubSub | `internal/redis/pubsub.go` |
| Migrations | `migrations/` |
| SQL queries (source) | `sql/queries/` |
| Generated DB code | `internal/db/sqlc/` |
| OpenAPI spec | `openapi.yaml` |
| WebSocket event docs | `docs/websocket-events.md` |
| Reconnect guide | `docs/reconnect-guide.md` |
| Backup runbook | `docs/backup-restore.md` |
