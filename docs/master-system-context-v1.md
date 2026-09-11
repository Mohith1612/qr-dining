# Master System Engineering Context — qr-dining

> **Purpose.** The deep code-level map: where things live, what owns what, and which file to open when you need to change X. It is the layer below [ARCHITECTURE.md](ARCHITECTURE.md), which explains the shape of the system without enumerating it.
>
> **Compiled 2026-08-22** against `feature/signoz-observability` @ `9865a48`. Package inventories, migration names, worker names and route groups were read from the tree rather than summarized from memory. Where a claim is inference rather than direct reading, it says so.
>
> This file replaces the 2026-05-28 edition, which predated the organization model reaching its current shape, the certification fixes, the redesign, and OpenTelemetry.

---

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
