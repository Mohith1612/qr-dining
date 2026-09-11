# Architecture

How QR Dining is built. Verified against the tree at `feature/signoz-observability` @ `9865a48` (schema v39, OpenAPI 2.2.0).

Companions: [DEVELOPMENT.md](DEVELOPMENT.md) · [DEPLOYMENT.md](DEPLOYMENT.md) · [SECURITY.md](SECURITY.md) · deep code map in [master-system-context-v1.md](master-system-context-v1.md).

---

## 1. What the system is

A **session-centric realtime dine-in operating system**, not a food-delivery app.

A QR code on a table creates a live **session**. Several guests scan it, join the same session, and share one cart. Staff — waiter, kitchen, manager, owner — work a live dashboard of that branch. A separate platform control plane operates tenants. PostgreSQL is the sole source of truth; everything realtime is a projection of it.

The unit of state is the **session**, not the user. That single decision explains the auth model, the WebSocket design, and the reconnect strategy.

## 2. System shape

```
                      Cloudflare
        ┌──────────────────────────────────────┐
        │  OpenNext Worker (Next.js frontend)  │   guest · staff · platform
        │  R2 (uploads + backups)              │
        └────────────────┬─────────────────────┘
                         │ HTTPS / WSS
        ┌────────────────▼─────────────────────┐
        │   shared OCI Ampere VM (arm64)       │
        │   /opt/proxy  nginx + certbot        │   central proxy, shared with
        └────────────────┬─────────────────────┘   other projects on the VM
                         │ proxy network
        ┌────────────────▼─────────────────────┐
        │  app container (Go / Gin)            │
        │   ├── HTTP: ~177 route registrations │
        │   ├── /health /readyz /metrics       │
        │   ├── WebSocket Hub (in-process)     │
        │   └── 6 background workers           │
        └────────┬────────────────┬────────────┘
                 │  internal network (no host ports)
        ┌────────▼──────┐  ┌──────▼──────────┐
        │ PostgreSQL 17 │  │    Redis 7      │
        │ source of     │  │  ephemeral by   │
        │ truth         │  │  design         │
        └───────────────┘  └─────────────────┘
                 │
        ┌────────▼──────────────────────────┐
        │ Observability: Prometheus (43      │
        │ metrics, 27 alert rules),          │
        │ Alertmanager, blackbox, node-exp.  │
        │ SigNoz/OTel — gated, OTEL_ENABLED  │
        └────────────────────────────────────┘
```

## 3. Technology

| Layer | Choice |
|---|---|
| Language | Go 1.26 (`backend/go.mod`, module `github.com/Mohith1612/qr-dining`) |
| HTTP | Gin |
| Database | PostgreSQL 17, 39 migrations, embedded via `go:embed` and applied at boot |
| Query layer | sqlc (pinned via Go tool directive); generated code in `internal/db/sqlc/` is never hand-edited |
| Driver | pgx v5 / pgxpool |
| Cache, pub/sub, presence, rate limit | Redis 7 (go-redis/v9) |
| WebSockets | gorilla/websocket |
| Metrics | Prometheus (43 collectors in `internal/observability/metrics.go`) |
| Tracing | OpenTelemetry → SigNoz, **off unless `OTEL_ENABLED=true`** |
| Logging | zerolog, structured JSON to stdout |
| Frontend | Next.js (App Router) + TypeScript, deployed as an OpenNext Cloudflare Worker |
| Marketing site | Astro + Tailwind, static, separate deploy |

## 4. Backend layout (`backend/`)

| Path | Responsibility |
|---|---|
| `cmd/server/` | Entry point: config → pgxpool → migrations → Redis → metrics → routes → Hub → workers → graceful shutdown |
| `cmd/migrate/` | Standalone migration CLI for development |
| `cmd/bootstrap-admin/` | One-shot creation of the first `super_admin`; refuses to overwrite an existing user |
| `internal/config/` | Env-var loading and validation. Release mode **fails hard** on weak secrets or empty CORS |
| `internal/server/` | All route registration in one file — the authoritative route table |
| `internal/handlers/` | HTTP handlers, request/response DTOs, validation |
| `internal/services/` | Business logic, transaction boundaries, lifecycle rules |
| `internal/repository/` | Thin wrapper over sqlc; maps `pgx.ErrNoRows` to typed domain errors; `WithTx` scoping |
| `internal/domain/` | Typed errors, enums, state machines |
| `internal/auth/`, `internal/authz/` | Credential issuance/validation and the central policy engine |
| `internal/audit/` | Append-only audit log (v2), DB-trigger protected |
| `internal/events/`, `internal/websocket/`, `internal/redis/` | Event publication, Hub and rooms, pub/sub + presence + rate-limit keys |
| `internal/worker/` | Six background loops (below) |
| `internal/storage/` | R2/S3 presigned upload abstraction |
| `internal/crypto/` | HMAC guest tokens, AES-GCM MFA secret sealing |
| `internal/middleware/` | request-id, logger, metrics, recover, CORS, security headers, max body size, rate limit, tenant resolution, branch guard, staff auth, platform auth |

### Middleware order matters

`requestid → logger → metrics → recover → security headers → max body size → CORS → tenant resolution → rate limit → (route-specific auth + branch guard)`. Rate limiting is applied per route group with different budgets; sensitive groups **fail closed** when Redis is unreachable.

### Background workers

All six are Redis-`NX`-lock guarded (safe with multiple app instances) and panic-isolated:

| Worker | Purpose |
|---|---|
| Stale session cleaner | Abandons inactive sessions, frees tables, emits `SESSION_CLOSED` |
| Session expiry warner | Warns before a session ages out |
| Presence expiry | Reaps expired participant heartbeats |
| Reactivation pipeline | Drives `awaiting_reactivation`, using Redis presence as the liveness signal |
| Payment-pending escalation | **Alert-only.** Bounds `payment_pending`; a human always settles |
| Session/table reconciler | Repairs table status drift against session state |

## 5. Data model

PostgreSQL is authoritative for every business fact. Key aggregates: organizations → branches → tables; sessions → participants → carts → orders → order items; payments → bill snapshots; menu categories → items → modifiers; staff; platform users; promos and loyalty ledger; `audit_log`.

Invariants that must not be broken casually:

- **One non-terminal session per table**, enforced by a partial unique index.
- **Price and modifier snapshots** are written at order time — menu edits never retroactively change a placed order.
- **Bill snapshots** are immutable once a payment is initiated.
- **`audit_log` is append-only**, enforced by a database trigger. Never `UPDATE` it.
- **Migrations are additive-only.** This is what makes a binary rollback across a migration safe.
- Loyalty redemption is **ledger-only** — balances are derived, never stored as a mutable number.

Contracts the code is required to satisfy live in [reference/](reference/): [session lifecycle](reference/session-lifecycle-state-machine.md), [payment finalization](reference/payment-finalization-invariants.md), [realtime reconciliation](reference/realtime-reconciliation-invariants.md).

## 6. Realtime

- **Hub** — an in-process map of `session_id → {clients}`. Single-process by design.
- **Pub/sub** — Redis channel `org:{org_id}:branch:{branch_id}:session:{session_id}:events`. Tenant-scoped in the key itself, so a fan-out cannot cross a tenant boundary even by mistake.
- **Presence** — `org:{org_id}:branch:{branch_id}:session:{session_id}:presence`, refreshed by client WebSocket PINGs. The heartbeat *is* the ping; there is no separate presence call.
- **Ordering** — `session_events` carries a durable sequence so clients can detect gaps.
- **Reconnect** — there is **no event replay**. A reconnecting client calls `GET /sessions/:id/snapshot` and diffs. See [reference/reconnect-guide.md](reference/reconnect-guide.md).
- **Slow consumers** are evicted rather than allowed to stall a room.
- **Staff dashboards poll** (~10s) instead of using WebSockets. Deliberate: adequate at pilot scale, and it keeps the Hub single-purpose.

Event catalogue: [reference/websocket-events.md](reference/websocket-events.md).

## 7. Tenancy and trust domains

Three trust domains that are **never mixed** — a token from one is rejected by the others:

| Domain | Credential | Surface |
|---|---|---|
| **Guest** | HMAC-SHA256 signed guest token bound to session + participant, with `credential_version` and `revoked_at` | `/sessions/*`, `/branches/:id/menu`, `/tables/by-qr/*` |
| **Staff** | branch code + staff code + PIN (bcrypt cost 12) → DB-backed session token | `/staff/*`, `/branches/:id/*` operational views |
| **Platform** | email + password + TOTP MFA → platform session token | `/platform/*` control plane |

Tenant resolution runs as middleware; `branch_guard` verifies branch ownership on every branch-scoped route. Cross-organization and cross-branch access is denied regardless of rollout-flag state — see [release-certification/authz-scope-investigation-2026-08-04.md](../release-certification/authz-scope-investigation-2026-08-04.md) and the regression tests in `backend/internal/handlers/authz_scope_integration_test.go`.

Full model and boundaries: [SECURITY.md](SECURITY.md).

## 8. Redis is disposable

Redis holds **only** ephemeral operational data: pub/sub channels, presence, menu cache, staff/platform session tokens, rate-limit counters, worker locks. Losing Redis costs realtime updates, cached menus and active logins — no business data. Losing PostgreSQL is the only real data-loss event.

The one exception to "degrade gracefully": rate limiting on **sensitive** routes (auth, payments, webhooks, WS tickets) **fails closed**, so a Redis outage cannot become a brute-force window.

## 9. Governance layer (built, deliberately unenforced)

Entitlements, platform feature flags, organization/branch lifecycle, and subscription billing all exist and are audited, but run **resolve-only / shadow**: they record what *would* happen without enforcing it. Billing charges nobody; money is handled manually outside the system. This is a rollout decision, not an incomplete feature — see [OPERATIONS.md](OPERATIONS.md) for the flag ladder.

## 10. Frontend (`frontend/`)

Next.js App Router with three route groups — `(guest)`, `(staff)`, `(platform)` — plus a public `pricing` page. Shared `components/`, `hooks/`, `lib/`, `store/`, `providers/`, and a tenant-resolving `middleware.ts`.

- Themes are tenant-configured and server-driven; `serene` is the default preset.
- The WebSocket client owns reconnect/backoff and calls the snapshot endpoint to reconcile.
- Product analytics is PostHog, a hard no-op unless both `NEXT_PUBLIC_POSTHOG_KEY` and `NEXT_PUBLIC_POSTHOG_HOST` are set — see [../frontend/docs/product-analytics.md](../frontend/docs/product-analytics.md).
- A prebuild guard **refuses** to produce a production build pointing at localhost or plain HTTP.

## 11. Known architectural boundaries

| Boundary | Consequence |
|---|---|
| WebSocket Hub is single-process | Horizontal scale-out needs session-affinity or a sharded pub/sub. Not needed at pilot scale |
| `session_sequences` write hot-spot | Measured ~5% 5xx at concurrency ~150 on a single branch — far beyond pilot load |
| Dual `restaurants` / `organizations` model | Legacy kept alive until strict tenancy enforcement is metric-proven; additive-only rule protects it |
| Bespoke HMAC guest token | Non-standard but sound; a JWT-library migration would be cosmetic |
| Money as decimal, not integer | Revisit only if invariant testing or a real bill exposes drift |

Roadmap, technical-debt ledger and current status: [../STATE-OF-THE-PROJECT.md](../STATE-OF-THE-PROJECT.md).
