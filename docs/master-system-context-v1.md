# Master System Engineering Context — qr-dining

> **Purpose.** The deep code-level map: where things live, what owns what, and which file to open when you need to change X. It is the layer below [ARCHITECTURE.md](ARCHITECTURE.md), which explains the shape of the system without enumerating it.
>
> **Compiled 2026-08-22** against `feature/signoz-observability` @ `9865a48`. Package inventories, migration names, worker names and route groups were read from the tree rather than summarized from memory. Where a claim is inference rather than direct reading, it says so.
>
> This file replaces the 2026-05-28 edition, which predated the organization model reaching its current shape, the certification fixes, the redesign, and OpenTelemetry.

---

<<<<<<< HEAD
## 1. Orientation

| | |
|---|---|
| Module | `github.com/Mohith1612/qr-dining` |
| Go | 1.26 |
| Schema | v39 — 39 migrations, embedded via `go:embed`, applied at boot |
| HTTP surface | ~177 route registrations, all in `internal/server/server.go` |
| API contract | `openapi.yaml`, v2.2.0, 171 operations |
| Metrics | 43 collectors in `internal/observability/metrics.go` |
| Alert rules | 27 in `deploy/observability/prometheus-alerts.yml` |
| e2e specs | 106 under `e2e/` |
| Trunk | `feature/signoz-observability`, 205 commits ahead of `main` |

Read [ARCHITECTURE.md](ARCHITECTURE.md) first if you have not. This document assumes you already know that sessions are the unit of state, Postgres is the source of truth, and Redis is disposable.

## 2. Repository map

```
backend/
  cmd/server/            entry point and dependency wiring
  cmd/migrate/           standalone migration CLI (development)
  cmd/bootstrap-admin/   one-shot first super_admin
  internal/              see §3
  migrations/            000001 … 000039, up + down
  sql/queries/           sqlc source — edit here, never in internal/db/sqlc/
  scripts/               seed, backup, restore, nightly-backup, loadtest
  docker/Dockerfile      multi-stage, linux/arm64, non-root, healthcheck
frontend/                Next.js App Router — see §7
marketing/               Astro static site, separate deploy
e2e/                     Playwright, 106 specs
deploy/                  vm/ observability/ signoz/ backup/ nginx/
scripts/                 chaos harness, manual-testing stack scripts
docs/                    active documentation
release-certification/   certification evidence
```

## 3. Backend packages (`backend/internal/`)

| Package | Owns |
|---|---|
| `config` | Env loading and validation. Release-mode hard failures live here |
| `server` | **The route table.** Every route, its middleware, its rate-limit budget |
| `handlers` | HTTP layer: DTOs, validation, error mapping. One file per surface |
| `services` | Business logic and transaction boundaries. One file per domain |
| `repository` | sqlc wrapper; maps `pgx.ErrNoRows` to domain errors; `WithTx` |
| `db` | `pool.go`, `migrations.go`, and generated `sqlc/` |
| `domain` | `errors.go` (typed errors) and `statemachine.go` (transition tables) |
| `auth` | Guest token issuance/validation, staff and platform session handling |
| `authz` | Central policy engine — **evaluates in shadow, does not enforce** |
| `audit` | Append-only audit writer and redaction |
| `events` | Event construction and publication |
| `websocket` | Hub, Client, rooms, ticket auth, slow-consumer eviction |
| `redis` | Pub/sub, presence, rate limiter, lockout store, cache |
| `worker` | Six background loops |
| `observability` | Prometheus collectors and OTel setup |
| `storage` | R2/S3 presigned upload abstraction |
| `crypto` | HMAC guest tokens, AES-GCM secret sealing, TOTP |
| `middleware` | 13 middlewares — see §5 |
| `testutil` | Integration-test fixtures and harness |

### Services

`session` `participant` `cart` `order` `payment` `assistance` `menu` `promo` `staff` `customer` `loyalty` `theme` `collateral` `operational_ids` — the core product.

`platform` `platform_mfa` `platform_analytics` `support` `subscription` `billing` `entitlement` `flag` `feature_gate` `enforcement_observability` `analytics` `staff_analytics` — the platform and governance layer.

`feature_gate.go` is the one to understand before touching gated capability: a feature is available only when **entitlement AND platform flag** both allow it.

### Handlers

Guest-facing: `session` `snapshot` `cart` `order` `payment` `assistance` `menu` `promo` `tables` `customer` `guest_auth` `tenant` `ws`.

Staff-facing: `staff` `branches` `menu_admin` `upload` `organization` `subscription` `loyalty` `staff_analytics` `analytics`.

Platform-facing: `platform` `platform_billing` `platform_collateral` `platform_entitlements` `platform_flags` `platform_lifecycle` `platform_observability` `platform_support` `platform_theme` `platform_analytics`.

Infrastructure: `health` `errors` `helpers` `audit_log` `event_log` `authz` `legacy_metrics`.

## 4. Route groups (`internal/server/server.go`)

Read this file top to bottom once; it is the authoritative map of the HTTP surface and it carries the reasoning for each rate-limit budget in comments.

| Group | Auth | Notes |
=======
## 1. Executive Overview

**What the product is.** `qr-dining` is a **session-centric, realtime, collaborative
dine-in ordering operating system** for restaurants. A guest scans a QR code on their
table, joins a *table session*, and the whole table collaborates on a **single shared
cart**. The **session host** (first joiner, reassignable) is the only participant
authorized to submit orders and initiate payment. Kitchen staff see orders on a kitchen
display, advance them through a cooking state machine, and mark them ready; waiters serve
ready orders and settle payments. Everything updates live over WebSockets.

It is **session-centric, not user-centric**: identity is anchored to a table session and
its participants, not to persistent user accounts. This matches the real-world dine-in
domain (people sit down, order together, leave) and is the single most important
architectural decision shaping the whole system.

**Market/context signals.** Currency defaults to **INR**, participants carry an optional
`phone_e164`, and the deployment target is **Oracle Cloud Ampere arm64** — this is built
for the Indian dine-in market on cost-efficient ARM infrastructure.

**Maturity.** The system is **feature-complete and substantially hardened**. It has been
through a long sequence of hardening phases (0–8) and stabilization phases (A–E) covering
identity, RBAC, multi-tenancy, an immutable audit trail, the session lifecycle state
machine, payment correctness, realtime reconciliation, and rollout instrumentation. It is
**not yet pilot-live**: it is in the **staged strict-enforcement rollout** stage, gated
behind feature flags.

**Current operational state (as of this writing).** The rollout sequence has **started at
R1**. `AUDIT_LOG_V2_ENABLED` is **ON and in soak**, executed on a single-instance **local
staging** stack — healthy, no rollback. Waves R2–R7 are instrumentation-ready but not yet
flipped. The R3 policy-decisions writing task (the prior sole hard blocker) is now
**closed** (`r3-policy-decisions-v1.md`); no hard blocker remains in the sequence — the
remainder is operational (soak time, backfills).

**What phase the project is in.** "Post-hardening, pre-pilot, staged rollout in progress."
The engineering risk is largely retired; the remaining work is *operational* (soak time,
staff data backfills, UI rollout, disk-headroom validation) plus a short list of real
defects surfaced by the e2e suite (see §11).

---

## 2. Current System Status

### 2.1 Backend stabilization
Stable. Clean layered architecture (handlers → services → repositories), strict state
machines for sessions/orders/payments/assistance, idempotency on mutating endpoints,
Redis-locked background workers safe for multi-pod, fail-closed rate limiting on sensitive
surfaces, and a comprehensive Prometheus metric surface. All nine strict-enforcement
feature flags exist and default `false`, so production behavior is unchanged until a flag
is intentionally flipped.

### 2.2 Frontend stabilization
Stable and contract-aligned with the backend. Next.js 15 App Router + React 19 +
TypeScript + Zustand + Tailwind 4. The frontend was explicitly aligned to the backend
contract (`docs/frontend-contract-stabilization.md`) and went through a polish plan
(`frontend-production-polish-plan-v1.md`). Realtime, reconnect, shared-cart, host-gating,
and payment flows are implemented.

### 2.3 Realtime status
Stable. Redis pub/sub fans events to an in-process WebSocket Hub that broadcasts to
per-session rooms. Clients reconnect with exponential backoff and reconcile via an
authoritative snapshot endpoint. Events are **at-least-once**; clients dedup by sequence.

### 2.4 Operational workflow maturity
The end-to-end operational loop (guest order → kitchen cook → waiter serve → payment
collect → session close) was walked through and verified in
`manual-testing-findings-v3.md`: the waiter ready-to-serve queue works, payment collection
works (no fake success — guest waits for real confirmation), the assist flow works, and
the guest lifecycle "feels operationally correct." Earlier P0 issues from
`manual-testing-findings-v1/v2` are resolved.

### 2.5 Rollout status (the central operational fact)

Nine strict flags, sequenced into seven waves R1–R7. All instrumentation prerequisites are
met. Per `final-rollout-gates-status.md`:

| Wave | Flag(s) | Code/instrumentation | Operational gate remaining | Hard blocker | Flipped? |
|------|---------|----------------------|----------------------------|--------------|----------|
| **R1** | `AUDIT_LOG_V2_ENABLED` | ✅ | disk headroom + 72h prod soak | — | **YES — staging soak CLOSED 2026-06-04, PASS w/ OBS** |
| **R2** | `TENANCY_ORGANIZATIONS_ENABLED` | ✅ | all `branches.organization_id` NOT NULL; soak to zero resolution failures | — | no |
| **R3** | `AUTHZ_CENTRAL_POLICY_ENFORCE` **+** `STRICT_BRANCH_SCOPED_MUTATIONS` (paired) | ✅ | shadow week → `policy_shadow_mismatch_total`=0 for 48h | ✅ **closed** (`r3-policy-decisions-v1.md`) | no |
| **R4** | `AUTH_STAFF_CODE_REQUIRED` **+** `AUTH_STAFF_SESSION_DB_REQUIRED` (paired) | ✅ | staff_code backfill + staff training + legacy PIN decay 7d | — | no |
| **R5** | `WS_TICKET_AUTH_REQUIRED` | ✅ | per-IP ws-ticket cap sizing + legacy decay 7d | — | no |
| **R6** | `AUTH_GUEST_CREDENTIALS_REQUIRED` | ✅ | 14d legacy participant-id decay + 14d soak | — | no |
| **R7** | `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | ✅ | staff settlement UI on every device + CI webhook test + soak | — | no |

**Dependency/pairing rules:** flips proceed in order; two pairs must flip *together* —
(AUTHZ + STRICT branch) and (STAFF_CODE + STAFF_SESSION_DB). Every flag is reversible
(set `false` + restart, MTTR <5 min, no data risk).

### 2.6 Soak status (R1)
Per `r1-live-rollout-status.md`: R1 flipped **2026-05-25T19:36:34Z**, soak start
**19:37:42Z**, target 48h staging → 72h prod at 100%. Through checkpoints CP-1..CP-3:
`audit_write_failures_total`=0, latency within +10ms budget, DB immutability trigger
enforced live (UPDATE/DELETE rejected), pool stable, disk ample (171.8 GB free, >60-day
headroom). Observed benign artifacts: a **one-time backlog burst** of
`payment.settlement.stalled`/`session.abandon` audit rows from ~20 accumulated test
sessions (Redis dedup confirmed working — counts flat across worker ticks), and two short
intentional restart gaps to ship CORS fixes (`AUDIT_LOG_V2_ENABLED` preserved across both).

**CP-4 (2026-05-28) — soak interrupted & restarted, 72h clock RESET.** The soak app
(`qr-app-chaos`, binary bind-mounted from host `/tmp/qrapp`) exited 127 and was down ~34h
after a `/tmp` cleanup wiped the binary; it had run only ~37h continuously, so the original
72h gate was never met. Recovered per operator decision: rebuilt the static binary from
HEAD (`075bbf8`, incl. the PT-03 fix), restarted with `AUDIT_LOG_V2_ENABLED=true` preserved
— `/readyz` 200, `audit_write_failures_total`=0, `policy_shadow_mismatch_total`=0; the
Postgres volume (immutable `audit_log`) was never reset. **New R1 soak start =
2026-05-28T19:29:03Z** (72h target ≈ 2026-05-31T19:29Z). The bind-from-`/tmp` deployment is
not outage-resilient (no app-down alert) — relocate the binary / add a liveness page before
a real production soak.

**Two caveats the soak itself flags as the residual R1 risk:**
1. **Storage curve** — per-row audit size must be re-derived at real volume (the n=2
   53 kB/row figure is a fixed-page-overhead artifact) and the 60-day projection confirmed.
2. **Audit coverage gap** — order placement does **not** emit an audit row; only
   `session.create`, `payment.initiate`, `session.abandon`, `payment.settlement.stalled`,
   authz denials, etc. do. R1 only turns the writer *on*; *what* is audited is a separate
   (non-R1) coverage concern. (`order.go` has no audit action.)

**CP-5 (2026-06-04) — staging soak CLOSED, verdict PASS WITH OBSERVATIONS.** The host was
shut down unexpectedly mid-soak; a forensic closure audit (`r1-soak-closure-report.md`)
certified the post-CP-4 run from **2026-05-28T19:29:03Z** (the CP-4 start, byte-matched in the
app log) to a **graceful** shutdown at **2026-06-02T23:27:22Z** — **~124h (5.16d) continuous,
`RestartCount=0`, `OOMKilled=false`**, so the 72h gate cleared with ~52h margin. Stability
within the certified window: **0 panics, 0 error-level lines, 0 `audit_write_failures`**; the
5346 `warn`/`critical` lines are all the documented alert-only `payment_pending settlement
stalled` escalations (worker never mutated state). Postgres shut down clean and restarted with
**no recovery** (no corruption); Redis showed no eviction/OOM/pressure; disk/memory flat
(~172 GB free). The container's present `Exited (127)` is a **failed post-reboot restart** —
the bind-mount source `/tmp/qrapp` was wiped when `/tmp` cleared on boot — i.e. the CP-4
deployment-fragility risk **recurred**, and is **not** a soak crash.

**Two soak qualifiers remained UNKNOWN (not failed), gating a future *production* R1 soak:**
1. **Idle tail / load coverage** — the last external request/health probe was
   **2026-05-30T19:52Z**; the final ~75h ran with no traffic and no `/readyz` probing (the
   process stayed alive — workers ticked every minute — but "healthy" was *inferred*, not
   *asserted*). Sustained-load and long-lived-WebSocket longevity were therefore not exercised
   for the full window.
2. **Storage curve** — still unproven at real volume (the original residual caveat below).

**Before a production R1 soak:** relocate the binary off `/tmp` + add an app-down/`/readyz`
liveness alert (closes the recurring CP-4 risk); run with continuous synthetic traffic +
external probing for the full window; re-derive per-row audit size at real volume; and purge
the ~20–28 stale `payment_pending` test sessions so a real stall is not masked. Leave the
current soak containers untouched (the Postgres `audit_log` volume is intact — do not reset).

Discipline note from the docs: **do not chain R2** — R1 must clear its full soak first. The
staging soak is now closed PASS w/ OBS; R2 still proceeds only on its own gate by deliberate
decision.

---

## 3. Full Architecture Overview

### 3.1 Backend architecture
Go + Gin HTTP, PostgreSQL (via `pgx/pgxpool`) as the **authoritative source of truth**,
Redis for ephemeral state (pub/sub, presence, rate limits, WS tickets, lockout, cache),
and an in-process WebSocket Hub. Go module: `github.com/Mohith1612/qr-dining`.

Layering:
- **Handlers** (`internal/handlers`) — HTTP request/response, validation, auth extraction,
  audit recording. No business rules.
- **Services** (`internal/services`) — business logic, state-machine enforcement,
  idempotency, host authority, event publishing, transactions.
- **Repositories** (`internal/repository`) — data access over sqlc-generated queries
  (`internal/db/sqlc`), plus a `WithTx` transaction wrapper.
- **Cross-cutting** — `internal/middleware`, `internal/auth` (guest tokens),
  `internal/authz` (central policy), `internal/audit` (immutable audit writer),
  `internal/events` (publisher), `internal/redis` (rate limiter, cache, presence, tickets,
  lockout), `internal/websocket` (Hub/Client/Envelope), `internal/observability`
  (logger + metrics), `internal/worker` (background jobs), `internal/config`,
  `internal/storage` (Cloudflare R2), `internal/domain` (state machine), `internal/crypto`
  (TOTP).

It is **event-driven**: every state mutation publishes a typed event through
`events.Publisher` → Redis → Hub → session room.

### 3.2 Frontend architecture
Next.js 15 App Router. Layering:
- **Providers** (`providers/`) — cross-cutting context: tenant resolution, theme, session
  lifecycle, error boundary.
- **Stores** (`store/`) — Zustand slices for session, cart, menu, orders, assistance,
  staff, ws-status.
- **Hooks** (`hooks/`) — domain logic over stores + API (`useSession`, `useCart`,
  `useOrders`, `useAssistance`, `useWebSocket`, `useFilteredMenu`).
- **API client** (`lib/api/`) — thin typed `fetch` wrapper + per-domain modules.
- **Realtime client** (`lib/ws/`) — `WSConnection` + snapshot `reconciliation`.
- **Routes** (`app/`) — route groups `(guest)` and `(staff)`.

State approach: backend is authoritative; the frontend holds an optimistic local mirror in
Zustand and **reconciles from an authoritative snapshot** on every reconnect.

### 3.3 Realtime architecture
`service → events.Publisher → Redis pub/sub → ws.Hub.runSubscriber → broadcast channel →
per-session room → client.writePump → WebSocket`. The Hub is single-process; rooms are a
`map[sessionID]map[clientID]*Client` mutated only inside the Hub's `Run` goroutine (no
lock). Clients connect via an **ephemeral WS ticket** (`POST /sessions/:id/ws-ticket` →
`GET /ws?ticket=...`). Reconnect uses `GET /sessions/:id/snapshot?last_sequence=N`.

### 3.4 Auth architecture — three distinct trust domains
1. **Guest** (`internal/auth/guest.go`) — stateless HMAC-SHA256 token (base64 payload +
   base64 signature) carrying session/branch/table/org/participant/role + credential
   version + expiry. Secret `GUEST_TOKEN_SECRET`; TTL `GUEST_TOKEN_TTL` (**default 2h**).
2. **Staff** (`middleware/staff_auth.go`, `services/staff.go`) — opaque token (Redis +
   optionally DB), via `Authorization: Bearer` or HttpOnly cookie
   (`AUTH_STAFF_COOKIE_ENABLED`). Roles: `owner`, `manager`, `waiter`, `kitchen`. Lockout
   on repeated failures (`redis.LockoutStore`).
3. **Platform** (`middleware/platform_auth.go`, `services/platform.go`) — a separate
   super-admin trust domain (org/branch governance). **Staff tokens are rejected** on
   platform routes. Optional TOTP MFA (AES-GCM-encrypted secret; `MFA_ENCRYPTION_KEY`).

Authorization is centralized in `internal/authz` (policy/action/actor/resource/scope),
constructed as `NewEnforcingAuthorizer(AuthzCentralPolicyEnforce)` — **shadow mode** by
default (log + metric, allow), strict when the R3 flag is on.

### 3.5 Session lifecycle (6-state machine)
`active`, `payment_pending`, `awaiting_reactivation`, `abandoned`, `expired`, `closed`.
Authoritative spec: `session-lifecycle-state-machine.md`. Terminal states
(`closed`/`abandoned`/`expired`) remain **readable for 60 minutes** after close for dispute
handling. See §6 and §8.

### 3.6 Payment lifecycle
`payment_status`: `pending`, `requested`, `provider_pending`, `requires_staff_confirmation`,
`completed`, `failed`, `refunded`, `cancelled`, `partially_refunded`. `payment_method`:
`cash`, `card`, `digital`, `card_manual`, `upi`. Initiating payment freezes the cart
(`payment_pending`); an **immutable bill snapshot** (`bill_snapshots`) is captured at
initiation. See §6 and §8.

### 3.7 Role system & tenancy
Roles `owner > manager > waiter > kitchen`. Tenancy is **organization → restaurant →
branch**; `tables/staff/menu/sessions/orders` are branch-scoped. Org scoping is gated by
`TENANCY_ORGANIZATIONS_ENABLED` (R2) and subdomain extraction via `BASE_DOMAIN`
(`middleware/tenant.go`). Branch ownership is enforced by `middleware/branch_guard.go`.

### 3.8 State management approach
**Postgres authoritative; Redis ephemeral & reconstructible.** If Redis is lost, presence/
tickets/rate-limits reset but no business data is lost; clients re-snapshot. This makes
recovery straightforward and is why the WS layer can be single-process for now.

---

## 4. Backend Codebase Map

> Root: `backend/`. Module `github.com/Mohith1612/qr-dining`.

### 4.1 Bootstrap / server lifecycle
- `cmd/server/main.go` — loads config, logger (zerolog), root context (SIGINT/SIGTERM),
  connects pgxpool + Redis, **runs embedded migrations before accepting traffic**
  (`db.RunMigrations`), wires metrics/pubsub/presence/publisher, creates the Hub, builds
  repos, constructs the HTTP server, then starts: `go hub.Run(ctx)` and **6 worker
  goroutines** (see 4.11), then blocks on the HTTP server.
- `cmd/server/worker_adapter.go` — adapts `repository.Repos` to the worker's `Querier`.
- `cmd/migrate/main.go` — standalone migrate CLI (`up`/`down`/steps).

### 4.2 Routing — `internal/server/server.go`
The single source of truth for the HTTP surface. Defines the global middleware stack, wires
**all** services and handlers (this file is the best top-down index of the backend), and
registers routes into groups:
- **Infra (no auth/rate-limit):** `GET /health`, `GET /readyz`, `GET /metrics`.
- **Public API** (`api`, global `RATE_LIMIT_RPM`=60): sessions, cart, orders, assist,
  payments, webhooks, snapshot, menu, tenant/plans, promo validate, customer opt-in.
- **Auth group** (`RateLimitSensitive "auth"`, `AUTH_RATE_LIMIT_RPM`=10, fail-closed):
  `POST /staff/auth`, `POST /platform/auth`, `POST /platform/auth/mfa`.
- **Platform API** (`/platform/*`, `PlatformAuth`): logout, MFA enroll/confirm/disable,
  users, organizations, branches, support search/sessions, audit.
- **Staff API** (`StaffAuth`): order status, payment settle, assist ack/resolve, plus
  branch-scoped dashboards (`/branches/:id/...`, `BranchTenantGuard`), org governance
  (`/orgs/:org_id/...`), menu admin, tables, promos, staff mgmt, analytics, audit reads,
  image-upload presign, subscription, customer history/delete.
- **WebSocket:** `GET /ws` (ticket or legacy guest token resolved in the handler).

Notable per-route protections (verified): `ws-ticket` = sensitive 60 + per-session 12;
`orders` = per-session 12; `assist` = per-session 6; `payments` = sensitive 30 +
per-session 6; `webhooks` = sensitive 200.

### 4.3 Handlers — `internal/handlers/`
HTTP boundary. Constructed in `server.go`. Important ones and their responsibility:
- `session.go` (`SessionHandler`) — Create/Get/Close/Join/Reactivate/IssueWSTicket/
  ListActiveForBranch.
- `cart.go` (`CartHandler`) — GetCart/AddItem/RemoveItem (shared cart).
- `order.go` (`OrderHandler`) — PlaceOrder/ListOrders/UpdateStatus/ListActiveForBranch.
- `payment.go` (`PaymentHandler`) — InitiatePayment/Settle/Webhook/ListPendingForBranch.
- `assistance.go`, `menu.go` (public), `menu_admin.go`, `staff.go`, `platform.go`,
  `ws.go` (`Upgrade`), `snapshot.go`, `promo.go`, `analytics.go`, `customer.go`,
  `table.go`, `branch.go`, `billing.go` (`GetBill`), `tenant.go`, `subscription.go`,
  `event_log.go`, `audit_log.go`, `organization.go`, `upload.go` (R2 presign), `health.go`.
- Shared helpers (`helpers.go`-style): global setters `SetActiveFeatureFlags`,
  `SetActiveAuthConfig`, `SetActiveServerConfig`, `SetHandlerMetrics`; error mappers
  (`sessionError`, `respondValidationError`, …); `requireGuestSession` guard.

### 4.4 Services — `internal/services/`
Business logic. Constructed/wired in `server.go` (lines ~80–102):
- `session.go` (`SessionService`) — lifecycle, **host authority** (`AuthorizeHostAction`),
  presence-aware on-demand host reassignment, snapshot assembly, reactivation. The largest
  and most central service.
- `participant.go` — join/leave, credential revocation.
- `cart.go` — shared cart ops + availability validation + modifier snapshotting.
- `order.go` — **host-gated** placement, idempotency, status transitions; injected promo
  service; host authority set via `orderSvc.SetHostAuthority(sessionSvc)`.
- `payment.go` — **host-gated** initiation, bill snapshot, state transitions, webhook
  handling, staff settlement; `paymentSvc.SetHostAuthority(sessionSvc)`.
- `promo.go`, `menu.go` (cache-backed), `staff.go` (auth/lockout/`SetRequireSessionDBRow`),
  `platform.go` (MFA/org/users), `analytics.go` (plan-gated), `customer.go`,
  `subscription.go`, plus `operational_ids.go` (human-readable order/payment references).

Cross-cutting service patterns: **idempotency** via the `idempotency_keys` table
(scope+actor+key+request_hash); **transactions** via `repos.WithTx`; **event publishing**
on every mutation; **host authority** as the single gate for order/payment submission.

### 4.5 Repositories — `internal/repository/`
Data access over `internal/db/sqlc` (sqlc-generated, type-safe). `repos.go` is the factory
+ `WithTx` wrapper (a `*Repos` bound to a tx implements the same interface). One file per
aggregate: `session.go`, `participant.go`, `order.go`, `cart.go`, `payment.go`,
`assistance.go`, `menu.go`, `promo.go`, `staff.go`, `platform.go`, `organization.go`,
`table.go`, `restaurant.go`, `customer.go`, `idempotency.go`, `event_log.go`,
`audit_log.go`, `worker.go` (worker-specific queries), `analytics.go`, `support_search.go`.
SQL queries live alongside sqlc input; regenerate with `make sqlc-generate`.

**Invariant:** the session↔host circular FK is handled with a `DEFERRABLE INITIALLY
DEFERRED` constraint so a session and its first (host) participant insert in one
transaction (see migration 000001).

### 4.6 WebSocket Hub — `internal/websocket/`
- `hub.go` (`Hub`) — rooms registry; `Run` goroutine owns all room mutations;
  register/unregister (buffered 32) and broadcast (buffered 512) channels;
  `runSubscriber` keeps a Redis psubscribe alive with exponential backoff.
- `client.go` (`Client`) — `readPump`/`writePump`; **slow-consumer eviction** when the
  send buffer is full; inbound-abuse limits (`client_abuse_test.go`).
- `message.go` — `EventType` constants (**25 event types**, see §7) + `Envelope`
  (`event_id`, `sequence`, org/branch/session IDs, `event`, `payload`, UTC `timestamp`)
  + `NewEnvelope`.

### 4.7 Middleware — `internal/middleware/`
`staff_auth.go`, `platform_auth.go`, `tenant.go` (subdomain→restaurant when `BASE_DOMAIN`
set), `branch_guard.go` (`BranchTenantGuard`), `ratelimit.go` (`RateLimit`,
`RateLimitSensitive` = **fail-closed** on Redis outage, `RateLimitByKey` = per-session),
`logger.go`, `metrics.go`, `recover.go`, `requestid.go`, `security_headers.go` (HSTS via
`ENABLE_HSTS`), `cors.go`, `maxbodysize.go` (1 MB). Audit context is injected via
`audit.Middleware()`.

### 4.8 Auth & authz — `internal/auth/`, `internal/authz/`
- `auth/guest.go` — `GuestTokenService` (issue/validate HMAC tokens; credential-version
  check enables revocation).
- `authz/{policy,action,actor,resource,scope}.go` — central RBAC; `roleAllowed(role,
  action)` table; `Decision{Allowed, Reason, …}`; shadow vs strict via the R3 flag.
  Emits `authz_denied_total` and `policy_shadow_mismatch_total`.

### 4.9 Payments — `internal/services/payment.go` (+ `handlers/payment.go`)
Host-gated initiation → freeze cart (`payment_pending`) → capture `bill_snapshots` row →
insert payment. Provider flows go `provider_pending` and resolve via
`POST /webhooks/payments/:provider` (signature + timestamp-tolerance + dedup via the
payment-webhook-events table). Cash/manual flows go `requires_staff_confirmation` and are
resolved by `PATCH /payments/:id/settle` (staff). Completion publishes `PAYMENT_COMPLETED`
and typically closes the session. Webhook secrets come from env keys
`PAYMENT_WEBHOOK_SECRET_<provider>`; tolerance `PAYMENT_WEBHOOK_TIMESTAMP_TOLERANCE`
(default 5m).

### 4.10 Session lifecycle — `internal/services/session.go`
Create (lock table row, insert session + host participant in one tx via deferred FK, mark
table occupied, publish `SESSION_CREATED`), Join, Close (revoke credentials, free table,
publish `SESSION_CLOSED`), host reassignment (`ensureHostBaseline` / presence check →
`HOST_CHANGED`), Reactivate, and `GetSnapshot` (auto-reactivates an
`awaiting_reactivation` session on reconnect within the window; returns 410-style terminal
read window after 60 min).

### 4.11 Workers — `internal/worker/worker.go`
Six goroutines, each Redis-locked (distributed, multi-pod safe) and panic-guarded
(`safeRun`). Verified names + startup wiring (`main.go` lines 90–95):
| Worker | Tick (default) | Purpose |
|--------|----------------|---------|
| `RunStaleSessionCleaner` | `STALE_SESSION_INTERVAL` (5m) | abandon sessions past branch timeout |
| `RunSessionExpiryWarner` | **hardcoded 5m** | emit `SESSION_EXPIRING_SOON` |
| `RunPresenceExpiry` | `PRESENCE_EXPIRY_INTERVAL` (60s) | reserved participant-left notification hook; field age determines presence |
| `RunSessionTableReconciler` | `SESSION_RECONCILE_INTERVAL` (5m) | reconcile session↔table occupancy |
| `RunReactivationPipeline` | `PRESENCE_EXPIRY_INTERVAL` (60s); creation grace=60s, idle grace=5m, reactivation window=5m | active→awaiting_reactivation→abandoned |
| `RunPaymentPendingEscalation` | `PAYMENT_PENDING_ESCALATION_INTERVAL` (1m); warn 5m / critical 15m | **alert-only** stalled-payment escalation |

The escalation worker **never mutates state** (see §12). The reactivation pipeline
**refuses to abandon sessions with a non-terminal payment**.

### 4.12 Migrations — `internal/db/migrations.go`, `cmd/migrate/`, `backend/migrations/`
`golang-migrate` + `iofs` embedded FS; auto-run at startup. 28 forward migrations
(000001–000028), each with `.up`/`.down`. See §6.

### 4.13 Observability — `internal/observability/`
- `logger.go` — zerolog structured JSON; request-scoped, tenant-enriched.
- `metrics.go` — **custom Prometheus registry** (not the global default). HTTP, WS, DB
  pool, Redis/cache, business (active sessions, orders, idempotency replays), workers,
  audit, and the Phase-E rollout metrics: `authz_denied_total`,
  `policy_shadow_mismatch_total`, `tenant_resolution_failures_total`,
  `guest_token_validation_failed_total`, `ws_ticket_consume_failed_total`,
  `payment_pending_escalations_total`, `audit_write_failures_total`, plus
  `legacy_identity_usage_total{mechanism}` for legacy-decay tracking.
- Health: `GET /health` (liveness, always 200) and `GET /readyz` (composite DB+Redis;
  Phase E fixed its `status` JSON field to track the HTTP code).

### 4.14 Config / feature flags — `internal/config/config.go`
`Load()` reads env (optionally `.env` via godotenv), validates, returns `*Config`. The
**nine** `FeatureFlags` (all `parseBool(..., false)`): `AuthGuestCredentialsRequired`,
`AuthStaffCodeRequired`, `AuthStaffSessionDBRequired`, `AuthzCentralPolicyEnforce`,
`TenancyOrganizationsEnabled`, `AuditLogV2Enabled`, `WSTicketAuthRequired`,
`PaymentStaffSettlementRequired`, `StrictBranchScopedMutations`. Verified non-obvious
defaults: `GUEST_TOKEN_TTL`=2h, `DB_MAX_CONNS`=20/`DB_MIN_CONNS`=2,
`DB_MAX_CONN_LIFETIME`=1h, `DB_MAX_CONN_IDLE_TIME`=30m, `RATE_LIMIT_RPM`=60,
`AUTH_RATE_LIMIT_RPM`=10, worker intervals as in 4.11. R2 (Cloudflare) image storage is
optional; upload endpoints 503 if unconfigured. In release mode the loader warns if CORS is
empty or the dev guest-token secret is still in use.

---

## 5. Frontend Codebase Map

> Root: `frontend/`. Next.js 15 App Router, React 19, TS, Zustand, Tailwind 4.

### 5.1 App shell, providers, routing
- `app/layout.tsx` — root layout; mounts `providers/Providers.tsx` (`TenantProvider` →
  `ThemeProvider` → theme sync → `Toaster`).
- `middleware.ts` (frontend) — extracts tenant slug from subdomain → header.
- `config/env.ts` — `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`, `NEXT_PUBLIC_ENV`,
  `NEXT_PUBLIC_TENANT_SLUG`, `NEXT_PUBLIC_BASE_DOMAIN`.
- Route groups: `app/(guest)/…` and `app/(staff)/staff/…`. Public marketing: `app/page.tsx`,
  `app/pricing/page.tsx`.

### 5.2 Guest flow
- `app/(guest)/table/[token]/page.tsx` — QR entry: resolve token (`menuApi.resolveQrToken`),
  collect name + **optional phone**, then create or join a session.
- `app/(guest)/session/[id]/layout.tsx` — validates `sessionStorage` (session_id,
  participant_id, guest_access_token), wraps in `providers/SessionProvider.tsx`.
- `session/[id]/page.tsx` (dashboard), `menu/page.tsx`, `cart/page.tsx`, `orders/page.tsx`,
  `payment/page.tsx`, `assist/page.tsx`.
- Layout chrome: `components/layout/{Shell,TopBar,BottomNav}.tsx`.

### 5.3 Waiter flow — `app/(staff)/staff/(dashboard)/waiter/page.tsx`
Three queues: assistance (ack/resolve), ready-to-serve orders (filter `status==="ready"` →
`ordersApi.updateStatus(id,"served")`), pending payments
(`paymentsApi.listPendingForBranch` → `paymentsApi.settle`). Driven by `ASSISTANCE_*`,
`ORDER_READY`, `PAYMENT_COMPLETED` events plus initial fetch.

### 5.4 Kitchen flow — `app/(staff)/staff/(dashboard)/kitchen/page.tsx`
KDS Kanban (pending/confirmed/preparing/ready). Advances orders pending→confirmed→
preparing→ready; **cannot mark served** (waiter-only, enforced server-side). Elapsed-time
color coding. Driven by `ORDER_*` events + `staffApi.getActiveOrders`.

### 5.5 Admin flow — `app/(staff)/staff/(dashboard)/admin/page.tsx`
Tabs: sessions, menu (CRUD + availability/featured/modifiers via `menu_admin` endpoints),
staff (create/PIN-rotate/deactivate), tables, promos, analytics, QR printing
(`components/admin/{QRCard,PrintTemplate,MenuItemModal,ImageUploadField}.tsx`,
`components/analytics/*`).
- `app/(staff)/staff/login/page.tsx` — staff auth; `(dashboard)/layout.tsx` is the staff
  auth guard (checks `store/staff.ts` hydration + token) and renders
  `components/staff/StaffBar.tsx`.

### 5.6 Session state & stores — `store/`
`session.ts` (session/participant/participants/isHost/completedPayment/sessionExpiringAt/
isReactivating; actions `setSession`, `setFromSnapshot`, `addParticipant`,
`applyHostChanged`, `markClosed`, `markPaused`, `applyReactivated`, …), `cart.ts`,
`menu.ts` (incl. `setItemAvailability`), `orders.ts`, `assistance.ts`, `staff.ts`
(persisted to sessionStorage with a hydration flag), `ws.ts` (connection status + attempt).

### 5.7 WebSocket handling & reconnect — `lib/ws/`, `hooks/useWebSocket.ts`
`lib/ws/connection.ts` (`WSConnection`, **verified**): backoff
`[1,2,4,8,16,30]s`, `MAX_ATTEMPTS=10`, ping every 30s. Connect = fetch WS ticket
(`sessionsApi.wsTicket`) → open `${wsUrl}/ws?ticket=…`. On close → `scheduleReconnect`. A **failed
ws-ticket** (e.g. 409 when the session has idled into `awaiting_reactivation`) also routes through
`scheduleReconnect()` (remediation F-1, 2026-05-30) — the older code dead-ended on "Connection
lost"; only a genuinely missing guest token still fails. On reconnect, fetch
`sessionsApi.snapshot(id, token, lastSequence)`:
- terminal status → set disconnected, surface `SESSION_CLOSED`;
- `awaiting_reactivation` → `markPaused()` and keep retrying;
- active + `snapshot_authoritative` → take snapshot as whole truth (skip replay);
- active + not authoritative → replay `missed_events`, then reconcile.
`lib/ws/reconciliation.ts` (`reconcileSnapshot`, **verified**): sets session/orders/
assistance from the snapshot and **refetches the cart separately** because the shared cart
is backend-authoritative and not carried in the snapshot payload.

### 5.8 Cart & shared-cart semantics — `store/cart.ts`, `hooks/useCart.ts`, `lib/api/cart.ts`
One shared cart per session. Any participant adds/removes; `CART_UPDATED` triggers a
`cartApi.getCart` refetch to converge (cart is **refetch-driven**, not pushed). Optimistic
remove with revert on error. Modifiers snapshotted per item.

### 5.9 Host-controlled ordering (UI gate) — `session/[id]/cart/page.tsx`, `hooks/useSession.ts`
Non-hosts can edit the shared cart but the "send order" / payment-initiate affordances are
disabled with a "only the host can send orders" message. `HOST_CHANGED` →
`applyHostChanged` (toast if you become host). The gate is **UX only**; the server is the
real authority.

### 5.10 Payment UI — `session/[id]/payment/page.tsx`, `components/shared/BillBreakdown.tsx`
Fetch bill, host-only initiate, method selection (cash/card/upi/digital); for
`requires_staff_confirmation` shows "awaiting confirmation" and waits for
`PAYMENT_COMPLETED`. Promo input (`lib/api/promos.ts`). Optional customer opt-in
(`components/shared/CustomerOptIn.tsx`, `lib/api/customers.ts`).

### 5.11 Reconnect / lifecycle UI — `components/shared/`
`ReconnectingBanner`, `SessionReactivatingBanner`, `SessionTimeoutBanner`,
`SessionEndedScreen`, `RealtimeIndicator`. `SESSION_EXPIRING_SOON` →
`setSessionExpiringAt` → timeout banner with a reactivate button.

### 5.12 API client & types — `lib/api/`, `types/`
`lib/api/client.ts` (typed fetch wrapper, Bearer + `X-Tenant-Slug` for local dev, maps
error codes to friendly messages, `ApiError`). Modules: `sessions, cart, orders, menu,
payments, assistance, staff, analytics, promos, tables, customers, plans, upload`.
`lib/idempotency.ts` generates keys for order/payment. Types in `types/api.ts`,
`types/ws.ts`.

### 5.13 Menu availability — `store/menu.ts`, `hooks/useFilteredMenu.ts`
`MENU_ITEM_AVAILABILITY_CHANGED` → `setItemAvailability` across featured + categories;
toast if an affected item is in the cart. No polling needed.

> **Realtime contract nuance (verify before relying on it):** the backend defines **no
> `SESSION_REACTIVATED` WS event** (`message.go` has 25 events; reactivation is recognized
> by the *client* via the snapshot on reconnect, not via a pushed event). If frontend code
> registers a `SESSION_REACTIVATED` handler, it is currently dead/defensive. Treat snapshot
> reconciliation — not a reactivation event — as the source of truth for resuming a paused
> session.

---

## 6. Database + Migration Overview

### 6.1 Schema philosophy
- **Status enums** for lifecycle (table/session/order/payment/assistance) rather than
  boolean soup; partial indexes target non-terminal rows.
- **Snapshotted money:** `order_items.unit_price` + `selected_modifiers_json` capture price
  at order time; `bill_snapshots` capture the bill at payment-initiation. Menu price
  changes never retroactively alter placed orders or initiated bills — correct financial
  behavior.
- **JSONB only for schemaless config** (`restaurants.settings_json`, item metadata) — never
  for data that needs filtering/aggregation.
- **Deferred circular FK:** `sessions.host_participant_id → session_participants` is
  `DEFERRABLE INITIALLY DEFERRED`.
- **Idempotency:** a general `idempotency_keys` table (scope+actor+key+request_hash);
  orders' original unique `idempotency_key` was replaced (migration 021) by scoped indexes.

### 6.2 Core entities (from 000001 + later alters)
`restaurants → branches → {tables, staff, menu_categories → menu_items → item_modifiers}`;
`sessions → session_participants`, `carts → cart_items`, `orders → order_items`,
`assistance_requests`, `payments → bill_snapshots`; plus `event_log`, payment webhook
events, `customers`, `promos`/`promo_redemptions`, `organizations`/`organization_members`,
`platform_users`/`platform_audit_log`/`platform_user_mfa`, `audit_log`, `order_sequences`,
`idempotency_keys`, subscriptions.

### 6.3 Lifecycle enum members (verified across migrations)
- **session_status:** `active`, `closed`, `abandoned` (000001) + `payment_pending`,
  `awaiting_reactivation`, `expired` (000023) = **6**.
- **order_status:** `pending, confirmed, preparing, ready, served, cancelled` (000001).
- **payment_status:** `pending, completed, failed, refunded` (000001) + `requested,
  provider_pending, requires_staff_confirmation, cancelled, partially_refunded` (000021)
  = **9**.
- **payment_method:** `cash, card, digital` (000001) + `card_manual, upi` (000021).
- **assistance_status:** `pending, acknowledged, resolved`. **assistance_type:** `waiter,
  bill, other`. **table_status:** `available, occupied, reserved`. **staff_role:** `owner,
  manager, waiter, kitchen`.

### 6.4 Participant & host semantics
`session_participants` carries `is_host`, `joined_at`, `last_seen_at`, and (from 000023)
`revoked_at`/`revoked_reason` — revocation prevents closed-session resurrection and is the
"active participant" predicate (`WHERE revoked_at IS NULL`). Host is denormalized on
`sessions.host_participant_id`; reassignment updates both.

### 6.5 The 28 migrations (grouped)
- **Foundation (001–015):** initial schema (001); event_log (002); webhook events (003);
  staff `is_active` (004); subscriptions (005); branch session config (006); session
  `warned_at` (007); menu featured/metadata/modifier-group (008–010); customers (011);
  image URLs (012); order_sequences (013); payment billing (014); promos (015).
- **Hardening (016–022):** identity/branch_code (016); **organization model** (017);
  **platform trust domain** (018); **audit_log v2 + immutability trigger** (019); realtime/
  session hardening (020); **payment/order correctness** — new payment enums,
  `idempotency_keys`, `bill_snapshots`, operational order fields, scoped idempotency,
  payment provider/settlement columns, promo redemption (021); operational UX/IDs (022).
- **Lifecycle/realtime (023–028):** session lifecycle states + participant revocation
  (023); non-terminal-per-table unique index extension (024); platform MFA/TOTP (025);
  session reactivation timestamp (026); **shared session cart** partial-unique index (027);
  **participant phone_e164** (028).

**Rollout/hardening-critical migrations:** 017 (R2 org tenancy), 019 (R1 audit v2), 023/024
(session lifecycle states + invariants), 021 (payment correctness + bill snapshots), 027
(shared cart), 028 (optional phone).

---

## 7. Realtime / Event Architecture

### 7.1 Connection & event flow
1. Client `POST /sessions/:id/ws-ticket` (sensitive 60 RPM + per-session 12) → ephemeral
   ticket in Redis (`WSTicketStore`).
2. Client `GET /ws?ticket=…`; `WSHandler.Upgrade` validates ticket (or legacy guest token
   until R5/R6 strict) and registers the `Client` into the session room.
3. A service mutation calls `events.Publisher` → Redis pub/sub → `Hub.runSubscriber` →
   broadcast channel → room fan-out → `client.writePump`.

### 7.2 Event catalog (25 types, verified in `websocket/message.go`)
Session: `SESSION_CREATED`, `SESSION_CLOSED`, `SESSION_EXPIRING_SOON`, `HOST_CHANGED`.
Participants: `PARTICIPANT_JOINED`, `PARTICIPANT_LEFT`. Cart: `ITEM_ADDED`, `ITEM_REMOVED`,
`CART_UPDATED`. Orders: `ORDER_PLACED/CONFIRMED/PREPARING/READY/SERVED/CANCELLED`.
Assistance: `ASSISTANCE_REQUESTED/ACKNOWLEDGED/RESOLVED`. Payments: `PAYMENT_INITIATED`,
`PAYMENT_COMPLETED`, `PAYMENT_SETTLEMENT_STALLED`. Menu: `MENU_ITEM_AVAILABILITY_CHANGED`.
Promo: `PROMO_APPLIED`. Heartbeat: `PING`, `PONG`. **There is no `SESSION_REACTIVATED`
event** (see §5.13 nuance).

`Envelope` carries `event_id`, `sequence`, org/branch/session IDs, `payload`, UTC
`timestamp`. Delivery is **at-least-once**; dedup by sequence.

### 7.3 Snapshot reconciliation (authoritative recovery)
`GET /sessions/:id/snapshot?last_sequence=N` returns full session state (session,
participants, orders, assistance) plus either contiguous `missed_events` or
`snapshot_authoritative: true` when the gap is too large to replay. The client
(`reconnect()` in `connection.ts`) replays-or-replaces accordingly. Terminal sessions
remain readable for **60 minutes** post-close. The serialized `session` has its
`session_token` **stripped** by `handlers.guestSafeSession()` (remediation F-8, 2026-05-30) — the
guest credential is never exposed here (or on Create/Get/Join/Reactivate), regardless of the R6 flag.

### 7.4 What is WebSocket-driven vs not
- **WS-push driven:** order lifecycle, assistance lifecycle, participant join/leave, host
  change, payment initiated/completed/stalled, session created/closed/expiring, menu
  availability, promo applied.
- **Refetch-on-event:** the **shared cart** — `CART_UPDATED` triggers a `getCart` refetch;
  the cart payload is *not* pushed and is *not* in the snapshot (it is fetched separately on
  reconcile). This keeps the cart strictly backend-authoritative.
- **Snapshot/HTTP driven:** initial state, all reconnect recovery, and **session
  reactivation** (no dedicated event).

### 7.5 Staff realtime
Staff dashboards (kitchen/waiter/admin) combine an initial REST fetch with live events;
they are not purely push-driven and use refresh affordances. This is acceptable for the
operational surfaces but is a known "staff realtime is lighter than guest realtime"
limitation.

### 7.6 Durability limitations (documented, acceptable)
The Hub is single-process; events can be lost during a Redis pub/sub impairment. Clients
recover via snapshot reconciliation. `redis_pubsub_connected` is a gauge; `/readyz` is the
authoritative outage signal (phase-d findings F-1/F-2). Slow consumers are evicted rather
than allowed to block the Hub.

---

## 8. Operational Workflow Semantics (intended behavior)

- **Guest ordering:** scan QR → join/create session → browse menu → add to **shared
  cart** → host submits the order → track status live.
- **Collaborative session:** all participants share one cart and one live view; any
  participant edits the cart; only the host submits/pays.
- **Session host:** first joiner is host. If the host loses presence (Redis) when someone
  tries to order/pay, the acting participant is **promoted on-demand** (avoids deadlock);
  `HOST_CHANGED` is published. Host also reassigns if the host leaves.
- **Waiter:** works three queues — assistance (ack→resolve), ready-to-serve (mark served),
  pending payments (settle cash/manual). Serving and settlement are **waiter-side**.
- **Kitchen:** advances orders pending→confirmed→preparing→ready. Kitchen **does not**
  serve.
- **Serving:** only after kitchen marks `ready` does the order appear in the waiter
  ready-to-serve queue; waiter marks `served`.
- **Payment collection:** host initiates → cart freezes (`payment_pending`) → bill snapshot
  captured. Provider payments resolve by webhook; cash/manual go
  `requires_staff_confirmation` and a waiter settles. On completion the session typically
  closes. **No fake success** — the guest UI waits for real `PAYMENT_COMPLETED`.
- **Reactivation:** a quiet session moves to `awaiting_reactivation`; a returning guest's
  snapshot/reconnect within `SESSION_REACTIVATION_WINDOW` (default 5m) reactivates it.
- **Inactivity handling:** `RunReactivationPipeline` moves active→awaiting_reactivation
  only after the 60s `SESSION_PRESENCE_GRACE` from creation, the newest durable
  participant heartbeat is older than `SESSION_IDLE_GRACE` (5m), and live Redis
  presence is empty; it then moves →abandoned after the reactivation window.
  `RunStaleSessionCleaner` abandons sessions past the branch timeout;
  abandoned→expired on terminal timeout.
- **Table lifecycle:** occupied on session create, freed on close; `RunSessionTableReconciler`
  repairs drift (fixing the old "abandoned session strands an occupied table" bug).
- **Stalled payments:** `RunPaymentPendingEscalation` emits warn/critical alerts (and the
  `PAYMENT_SETTLEMENT_STALLED` event) but **never** auto-settles or auto-cancels — a human
  resolves (see §12). The reactivation pipeline won't abandon a session with a non-terminal
  payment.

---

## 9. Testing + Validation Infrastructure

### 9.1 Backend unit tests (no DB)
State machine (`internal/domain/statemachine_test.go`), guest tokens
(`internal/auth/guest_test.go`), TOTP (`internal/crypto/totp_test.go`), authz policy
(`internal/authz/policy_test.go`), audit redaction (`internal/audit/redaction_test.go`),
operational IDs, platform logic, webhook signature, guest-auth middleware, platform audit.

### 9.2 Backend integration tests (`//go:build integration`, `TEST_DATABASE_URL`)
Run embedded migrations against any **empty/throwaway** Postgres, then exercise repos/
services/workers/ws-tickets (org, audit immutability, platform/MFA, session hardening,
cart, order, payment, **webhook idempotency**, assistance, reactivation/escalation worker,
ws-ticket store). **Known harness issue:** locally `pgxpool.Close()` deadlocks on teardown,
so the suite can't go green on this sandbox; it runs green in **CI against a dedicated DB**.
This is the safe pattern: **integration/CI uses a throwaway DB — never the soak DB.**

### 9.3 Playwright e2e (`e2e/`, ~106 specs)
Domains: guest/session lifecycle, orders, payments, webhooks, staff/RBAC, realtime,
tenancy, audit, platform/admin, multi-device, operational IDs, adversarial/security,
frontend/screenshots. Latest pre-R1 run (`e2e-failure-analysis.md`): **84 pass / 43 fail**.
Failures split into:
- **Category 1 — real backend bugs (all now FIXED):** staff token on guest route → 500
  (X-03) and **cross-org session snapshot → 200** (T-01) both fixed in `6db078e`
  (present-but-invalid guest tokens are rejected, not failed-open); zero-quantity order →
  500 (O-05) fixed in `8b5c386` (per-item quantity validation); MFA enroll → 500 when
  `MFA_ENCRYPTION_KEY` unset (PT-03) fixed in `45c93d5` (now 503 `MFA_NOT_CONFIGURED`).
  A re-audit (2026-05-29) confirmed T-01/X-03/O-05 complete; only PT-03 was still open.
- **Category 2 — over-strict specs:** several expect 200 where the backend correctly
  returns 201; one snapshot test expects populated `missed_events`/`snapshot_authoritative`
  signal it isn't asserting correctly (R-06).

### 9.4 Soak, staging, chaos
- **Soak:** `manual-local-soak-operations-guide.md`, `r1-soak-monitoring-guide.md`,
  `r1-activation-runbook.md`, `r1-live-rollout-status.md` (the live ledger).
- **Staging validation:** `phase-d-staging-validation-report.md` (topology sound; authored
  `deploy/nginx/qr-dining.conf` WS-aware reverse proxy; app stateless; `/readyz` is the
  outage signal).
- **Chaos:** `chaos-test-results.md` (reconnect storms, pool behavior, per-IP egress
  reconnect risk feeding the R5 cap-sizing requirement).

### 9.5 Observability / metrics / alerts
Custom Prometheus registry on `/metrics`; **24 alert rules**
(`deploy/observability/prometheus-alerts.yml`, `promtool`-clean) including the page-backed
rollout gates (`AuditWriteFailures`, `TenantResolutionFailures`,
`PolicyShadowMismatchPresent`/`AuthzDeniedSpike`, `GuestTokenValidationFailures`,
`WSTicketConsumeFailures`, `PaymentPendingEscalationCritical`, `ReadyzProbeFailing`).
Grafana dashboard JSON committed.

### 9.6 Known testing gaps
The 4 Category-1 e2e bugs (incl. the cross-org leak); e2e spec strictness; the local
integration teardown deadlock (CI-only green); audit-coverage gap (order.place unaudited);
no live Grafana in sandbox (validated via `/metrics` presence).

---

## 10. Rollout + Hardening History

### 10.1 Hardening phases (implemented; code + migrations)
0 baseline re-audit → 1 identity (staff code + guest tokens) → 2 RBAC/ownership (central
policy, branch-scoped SQL) → 3 organization model → 4 platform super-admin trust → 5
**audit log v2** (immutable trigger) → 6 realtime/session hardening (state machine, WS
tickets, reactivation) → 7 payment/order correctness (bill snapshots, payment enums) → 8
operational UX (operational IDs, business dates).

### 10.2 Stabilization phases A–E
A session lifecycle states (023/024) → B session reactivation (026) → C behavioral
convergence (`phase-c-behavioral-convergence-report.md`) → D staging validation
(`phase-d-staging-validation-report.md`) → E enforcement readiness (six rollout metrics,
alert-only payment escalation, `/readyz` fix, webhook idempotency test, +6 alerts;
`phase-e-enforcement-readiness-report.md`).

### 10.3 Rollout sequencing
`production-enforcement-rollout.md` (wave defs + dependency graph),
`strict-rollout-plan-final.md` (soak durations + exit gates),
`final-production-readiness-assessment.md` (GO to start at R1),
`final-rollout-gates-status.md` (per-wave ledger), `r1-*` (R1 execution/soak),
`post-remediation-rollout-status.md`, `rollout-blocker-remediation-report.md`.

### 10.4 Major risks solved
The `operational-correctness-audit.md` "not production-ready" blockers are the spine of the
hardening work: ambiguous PIN-only staff identity, unauthenticated guest identity
(X-Participant-ID trust), inconsistent branch isolation, unsafe payment finalization
(unauth webhooks / fake success), and stranded-occupied-table cleanup. Each maps to a
hardening phase + flag. What stabilized: the session lifecycle state machine, the payment
finalization invariants, realtime reconciliation, and the frontend↔backend contract.

---

## 11. Current Known Gaps

### Critical — RESOLVED this cycle (2026-05-29)
- **Cross-org snapshot leak (T-01)** + **staff token on guest route → 500 (X-03)** — FIXED
  (`6db078e`): present-but-invalid guest tokens are rejected (401/403) instead of failing
  open. **Zero-quantity order → 500 (O-05)** — FIXED (`8b5c386`). **MFA enroll → 500
  without `MFA_ENCRYPTION_KEY` (PT-03)** — FIXED (`45c93d5`; now 503 `MFA_NOT_CONFIGURED`).
- **R3 policy decisions (was ⛔ hard blocker) — CLOSED:** the three §11 questions
  (org-owner = governance-only; revoked-credential handling; org audit aggregation) are
  answered in writing in `r3-policy-decisions-v1.md` (commits `8e193e6`/`075bbf8`); all
  three ratify existing behavior, so no code-behavior change was needed.
- **No critical gap remains open.** R3 still needs its 48h shadow soak (operational) before
  the paired strict flip.

### Moderate
- **Audit coverage gap:** order placement isn't audited (decide intended coverage; if it
  should be, add the action — separate from R1).
- **e2e spec strictness** (201-vs-200 expectations; snapshot signal assertions).
- **R1 storage curve** must be re-derived at real volume and the 60-day projection
  confirmed.
- **Operational backfills before their waves:** `branches.organization_id` NOT NULL (R2);
  `staff_code` + staff training + legacy PIN decay (R4); per-IP ws-ticket cap sizing +
  legacy decay (R5); guest legacy-credential decay (R6); settlement UI on every device +
  CI webhook test (R7).

### Future enhancement
- Deferred `quiet_grace_seconds` HTTP-traffic gate for reactivation (Phase C if needed).
- Real multi-pod **production** soak (R1 so far is single-instance local staging).
- Audit hash-chain (`row_hash`/`previous_hash`) tamper-evidence.
- Hub sharding if a single process becomes a ceiling.

---

## 12. Important Current Product Decisions (with WHY)

- **Shared cart (one cart/session, `participant_id IS NULL`).** Single source of truth →
  no per-person merge conflicts; everyone sees the same table order. Enforced by the
  `carts_session_shared_uniq` partial index (migration 027) so `GetOrCreateSessionCart` can
  rely on `ON CONFLICT`.
- **Host-controlled ordering (server-enforced).** Prevents chaos/replay from many devices
  and gives a single accountable submitter. On-demand host reassignment (presence-based)
  avoids a deadlock where an absent host blocks the table. The UI gate is convenience; the
  server is authority.
- **Optional phone (`phone_e164`, migration 028).** Lowers join friction ("continue without
  phone") while enabling waiter/kitchen identification and a future loyalty/CRM anchor.
- **Session-centric (not user-centric) model.** Matches the real dine-in domain; avoids
  forcing account creation; identity is scoped to the session and expires with it.
- **Waiter/kitchen separation.** Kitchen cooks (→ready); waiter serves + settles. Mirrors
  real restaurant roles and keeps the KDS uncluttered by front-of-house actions.
- **Payment confirmation = real, escalation = alert-only.** No fake success; cash/manual
  needs explicit staff settlement. The escalation worker **never** auto-settles (would
  fabricate revenue) or auto-cancels (would risk cancelling a late-but-successful webhook);
  a human is the correct authority (`payment-escalation-lifecycle.md`).

---

## 13. File-Level Navigation Guide ("modify X → look at Y")

| If you need to change… | Backend | Frontend |
>>>>>>> fix/presence-host-authority
|---|---|---|
| Infrastructure | none | `/health`, `/readyz`, `/metrics`. No rate limit |
| Public API (`api`) | guest token where applicable | Sessions, cart, orders, assistance, payments, promo validation, customer opt-in |
| `branchPublicAPI` | none | `/branches/:id/menu` and friends; tenant-guarded when `BASE_DOMAIN` is set |
| Snapshot | guest token | `/sessions/:id/snapshot` — the reconnect reconciliation endpoint |
| `authGroup` | none | `/staff/auth` at a strict 10 RPM. **Fails closed** |
| `platformAPI` | platform token | `/platform/*`. Staff tokens are rejected outright |
| `staffAPI` | staff token | Staff-protected routes |
| `branchStaffAPI` | staff token + branch guard | Branch-scoped operational views |
| `orgAPI` | staff token | `/orgs/:org_id`, feature-flagged in handler |
| WebSocket | ticket | `/ws` upgrade |

Notable design points recorded in that file:

- **Host-controlled ordering.** Only the session host may submit orders or close the session.
- **Promos apply at payment initiation**, not at order placement.
- **Client WS PINGs are the presence heartbeat.** There is no separate presence endpoint.
- **Payments and webhooks fail closed** on rate-limit backend outage.
- **Loyalty earn rides payment completion** but is nil-safe and error-isolated — a loyalty failure must never fail a payment.

## 5. Middleware

Order: `requestid → logger → metrics → recover → security headers → max body size → CORS → tenant → rate limit → (auth + branch guard)`.

| Middleware | Note |
|---|---|
| `tenant.go` | Resolves org/branch context; failures counted by `tenant_resolution_failures_total` |
| `branch_guard.go` | Verifies branch ownership on branch-scoped routes |
| `staff_auth.go` / `platform_auth.go` | Separate trust domains; each rejects the other's tokens |
| `ratelimit.go` | `RateLimitSensitive` fails **closed**; the general limiter fails open with a metric |
| `security_headers.go` | API-shaped CSP (`default-src 'none'`); HSTS gated on config |

## 6. Data model and migrations

39 migrations, additive-only. The arc is legible from their names:

- **000001–000015** — the product: schema, event log, webhooks, staff, subscriptions, menu, modifiers, customers, order sequences, billing, promos.
- **000016–000022** — the hardening phases: identity, organization model, platform trust domain, **audit log v2**, realtime/session hardening, payment/order correctness, operational UX.
- **000023–000028** — lifecycle: session states, lifecycle invariants, platform MFA, reactivation, shared cart, participant phone.
- **000029–000035** — the SaaS layer: entitlements, feature flags, theme config, subscription billing, collateral, staff analytics, loyalty.
- **000036–000039** — certification fixes and the redesign: promo redemption on payment, single-select modifiers, the serene theme preset, promo phone normalization.

Invariants that migrations protect, and that you should not casually change: one non-terminal session per table (partial unique index); append-only `audit_log` (trigger); immutable bill snapshots; price and modifier snapshots on order items; ledger-only loyalty balances.

## 7. Frontend map (`frontend/`)

| Path | Owns |
|---|---|
| `app/(guest)/` | QR landing, join, menu, cart, order tracker, bill, payment |
| `app/(staff)/` | Login, admin, kitchen display, waiter view |
| `app/(platform)/` | Platform login, onboarding, support console, billing, flags, analytics |
| `app/pricing/` | Public pricing page |
| `middleware.ts` | Tenant resolution at the edge |
| `hooks/`, `lib/ws/` | WebSocket client, reconnect/backoff, snapshot reconciliation |
| `store/` | Client state |
| `providers/` | Context providers including product analytics |
| `styles/`, `config/` | Theme tokens; `serene` is the default preset |
| `scripts/check-prod-env.mjs` | The prebuild guard that rejects localhost/http production builds |

## 8. Realtime internals

- Channel: `org:{org_id}:branch:{branch_id}:session:{session_id}:events`
- Presence: `org:{org_id}:branch:{branch_id}:session:{session_id}:presence`
- Tenant scoping is **in the key**, so fan-out cannot cross tenants by mistake.
- `session_events` carries a durable sequence; clients detect gaps and reconcile.
- No replay. Reconnect is snapshot-based — [reference/reconnect-guide.md](reference/reconnect-guide.md).
- Staff dashboards poll (~10s) rather than hold WebSockets. Deliberate.

Contract: [reference/realtime-reconciliation-invariants.md](reference/realtime-reconciliation-invariants.md).

## 9. Workers

`RunStaleSessionCleaner` · `RunSessionExpiryWarner` · `RunPresenceExpiry` · `RunReactivationPipeline` · `RunPaymentPendingEscalation` · `RunSessionTableReconciler`

All Redis-`NX`-lock guarded and panic-isolated. Health signals: `background_worker_runs_total`, `background_worker_panics_total`.

`RunPaymentPendingEscalation` is **alert-only** and must stay that way — a machine never decides money.

## 10. Rollout flags

Nine environment flags stage enforcement. Current posture and change procedure: [OPERATIONS.md §4](OPERATIONS.md#4-rollout-flags-the-enforcement-ladder).

Shadow-mode observability is what gates them: `legacy_authz_bypass_total`, `legacy_identity_usage_total`, `policy_shadow_mismatch_total`, `tenant_resolution_failures_total`. A wave flips when its gate metric has read zero for the required window.

## 11. Testing infrastructure

- Unit tests alongside the code; integration tests behind `//go:build integration`, sharing one DB and one Redis — hence the mandatory `-p 1`.
- `internal/testutil/` holds fixtures and the harness. Integration tests run migrations on startup, so they can target any empty database — always a throwaway one.
- 106 Playwright specs in `e2e/`, organized by area. **Not executed in CI.**
- Chaos harness in `scripts/chaos/`.

Full picture, including what CI does and does not cover: [TESTING.md](TESTING.md).

## 12. "I need to change X — where do I look?"

| Change | Start here |
|---|---|
| Add or modify a route | `internal/server/server.go`, then `handlers/`, then `openapi.yaml` |
| Change business behaviour | `internal/services/<domain>.go` |
| Change a SQL query | `sql/queries/`, then `make sqlc-generate`. Never edit `internal/db/sqlc/` |
| Change the schema | New migration in `backend/migrations/`. **Additive only** |
| Change session states | `domain/statemachine.go` + [reference/session-lifecycle-state-machine.md](reference/session-lifecycle-state-machine.md) |
| Change payment behaviour | `services/payment.go` + [reference/payment-finalization-invariants.md](reference/payment-finalization-invariants.md) |
| Change realtime behaviour | `internal/websocket/`, `internal/events/`, `internal/redis/pubsub.go` + [reference/realtime-reconciliation-invariants.md](reference/realtime-reconciliation-invariants.md) |
| Add a metric | `internal/observability/metrics.go`, then an alert rule if it should page |
| Add an alert | `deploy/observability/prometheus-alerts.yml`, `promtool check rules` |
| Gate a new capability | `services/feature_gate.go` — entitlement AND flag |
| Change auth | `internal/auth/`, `internal/middleware/*_auth.go` + [SECURITY.md](SECURITY.md) |
| Change rate limits | `internal/server/server.go` (budgets) and `middleware/ratelimit.go` (behaviour) |
| Change a guest screen | `frontend/app/(guest)/` |
| Change themes | `frontend/styles/`, `frontend/config/`, `services/theme.go` |
| Deploy or operate | [DEPLOYMENT.md](DEPLOYMENT.md) · [OPERATIONS.md](OPERATIONS.md) · [RUNBOOKS.md](RUNBOOKS.md) |

## 13. Known gaps

These are known, deliberate, and tracked — not discoveries.

| Gap | Status |
|---|---|
| RC never soaked | **SEV-0.** [RELEASE.md](RELEASE.md#5-open-before-v10) |
| Central authz enforcement (R3) | Shadow. Gated on 48h zero mismatches |
| Tenancy organizations (R2) | Off. Needs org backfill + soak |
| Staff settlement required (R7) | Off. Needs webhook exact-replay proof in CI |
| Governance and billing | Built, audited, **enforcing nothing** by design |
| Playwright in CI | Discovery only |
| Money as float | Convert to integer paise before scale |
| `session_sequences` hot-spot | ~5% 5xx at concurrency ~150; clean at 50 |
| Audit hash-chain columns | Present, NULL |
| Multi-instance WS soak | Not done |
| Dual `restaurants`/`organizations` | Legacy kept until R2/R3 are metric-proven |

## 14. Where else to look

- Strategy, roadmap, technical-debt ledger, product vision — [../STATE-OF-THE-PROJECT.md](../STATE-OF-THE-PROJECT.md)
- Decisions and their reasoning — [adr/](adr/)
- How the system got here — [history/README.md](history/README.md)
- Current certification state — [../release-certification/](../release-certification/)
