# QR-Dining — State of the Project

**Engineering Handbook · Chief Architect Review**
**Date:** 2026-07-18 · **Branch reviewed:** `feature/certification-fixes-ui-redesign` @ `9dd869a` · **Schema:** v38

> This document was reconstructed from the full repository — backend, frontend, migrations, sqlc, OpenAPI, deployment, scripts, and every strategic/readiness/testing document (now archived under `docs/` and `docs/history/`). It supersedes no invariant docs, but where it contradicts an older report, this document is correct and the older report is stale (each case is called out in Part 8).

---

## Part 1 — Executive Summary

### What this product actually is today

QR-Dining is a **session-centric, realtime, multi-tenant dine-in operating system**. A guest scans a QR code at a physical table, and from that moment the *table session* — not the individual diner, not the order — is the unit of truth. Multiple guests join the same session, share a live collaborative cart over WebSockets, and order under a host-controlled model. Staff (waiter, kitchen, manager, owner) run the operational side from a PIN-authenticated console. A separate platform trust domain (super admin, support, billing, auditor) governs tenants, entitlements, feature flags, themes, billing, and support — almost all of it deliberately in **shadow/observe-only mode**.

Concretely, it is:

- A **Go/Gin backend** (254 Go files, 172 routes, 38 migrations, 58 tables) with strict state machines for sessions, orders, payments, and assistance; Redis for pub/sub, presence, rate limiting, lockout, WS tickets, and worker locks; six background workers; and an immutable audit log.
- A **single Next.js 15 frontend** serving three audiences via route groups: guest (`(guest)` — the tenant-branded "Serene" experience), staff (`(staff)` — the unbranded "Harmony" ops surface), and platform (`(platform)` — the governance console).
- A **certified API**: OpenAPI 3.1, v2.2.0, 171 operations, kept 1:1 with the router and lint-clean.
- A **hardened but not-yet-launched system**: it has survived chaos testing, a 124-hour soak (of an older binary — see the SEV-0 below), a 3-tenant pilot dress rehearsal, and four rounds of manual certification. It has never served a paying customer.

### Who it serves

1. **Dine-in restaurants in India** (INR default, UPI/cash/card-manual payment reality, Oracle Cloud Ampere arm64 cost profile) that want guests ordering from their phones without an app install.
2. **Groups of diners** — the collaborative shared-cart/host model is built for tables of several people, not solo QR menus.
3. **A future platform operator** (you) — the platform console, entitlements, billing, and support tooling are the skeleton of a SaaS business, currently running in shadow.

### What problems it solves

- Order capture without waiters transcribing; kitchen routing; assistance requests; bill assembly with promos; split payment settlement — all tied to a session lifecycle that cannot strand a table (reconciler workers, reactivation pipeline, alert-only payment escalation with a human as final authority).
- Multi-guest coordination at one table — the genuinely differentiated part. Most QR-menu products are single-phone catalogs; this is a realtime shared session with presence, host transfer, and backend-authoritative recovery.
- Tenant governance without redeploys — entitlements + feature flags + theming resolved per org/branch at runtime.

### What differentiates it

1. **The session state machine as the core abstraction** (6 states, enforced transitions, cart freezing during `payment_pending`, reactivation instead of silent expiry).
2. **Collaborative ordering** (shared cart, host-gated submission, live presence).
3. **Operational honesty by design**: payment escalation never mutates state; audit log is trigger-immutable; every risky behavior sits behind one of 9 strict flags with a documented rollout wave and rollback (<5 min MTTR).
4. **Rollout discipline as a feature**: shadow-before-strict, per-wave gates, metrics that must read zero before a flag flips. This is unusual maturity for a pre-launch product.

### The single most important fact

**The pilot candidate has never been soaked.** The 124h R1 soak that passed (2026-06-04) ran the pre-redesign June-10 binary. Everything since — ~18 certification bug fixes, migrations 000036–000038, the entire Serene/Harmony redesign — is unsoaked. This is the standing SEV-0 and the first item in every recommendation list in this document.

---

## Part 2 — Architecture

### 2.1 System shape

```
Guest phone ──HTTPS/WSS──┐
Staff tablet ──HTTPS─────┤   nginx (WS-aware, /metrics CIDR-locked)
Platform console ─HTTPS──┘        │
                                  ▼
                        Go/Gin app (single binary, stateless)
                        handlers → services → repositories → sqlc
                          │                │
                          ▼                ▼
                    PostgreSQL 17     Redis 7 (AOF, 256MB LRU)
                    (authoritative)   pub/sub · presence · cache ·
                          ▲           rate-limit · lockout · ws-tickets ·
                          │           worker locks · escalation dedup
                    6 workers (Redis-lock-guarded, panic-isolated)
```

**Postgres is authoritative for everything; Redis is disposable.** This was chaos-validated: sustained Redis outage degrades gracefully (realtime pauses, HTTP continues, sensitive rate limiter fails closed) and recovers automatically. App restart recovery is ~12s with idempotent migrations at boot.

### 2.2 Backend

- **Layout:** module `github.com/Mohith1612/qr-dining` under `backend/`. Entry points `backend/cmd/server/main.go` (boots pool, Redis, hub, services, 6 workers + a DB-stats goroutine) and `backend/cmd/migrate/main.go`.
- **Routing:** all 172 routes wired in `backend/internal/server/server.go` (489 lines) — one constructor wires services → handlers → middleware → groups. Groups: public `api` (rate-limited), staff (`staffAPI`, `branchStaffAPI`, `orgAPI`), platform (`platformAPI`, separate trust domain), plus `/health`, `/readyz`, `/metrics`, `/ws`.
- **Middleware chain (order matters):** `Recover → RequestID → audit → Logger → Metrics → SecurityHeaders → CORS → MaxBodySize(1MB) → Tenant`; then per-group rate limits and auth (`StaffAuth`, `PlatformAuth`, `BranchTenantGuard`).
- **Layering:** handlers (~47 files) → ~30 services (`internal/services/`) → thin repositories → sqlc-generated queries (`backend/sql/queries/*.sql` → `internal/db/sqlc/`, drift-checked in CI). Correctness-critical invariants live in `internal/domain/` (state machines) — the best-tested code in the repo.

### 2.3 Database

PostgreSQL 17, 38 embedded golang-migrate migrations (all with down-migrations, round-trip verified), 58 tables, 14 enums, 1 immutability trigger (audit log), 100+ FKs, 400+ check constraints. Detail in Part 5.

### 2.4 Redis — six distinct roles

| Role | File | Notes |
|---|---|---|
| Session event fanout | `internal/redis/pubsub.go` | feeds the WS hub; subscriber auto-reconnects with backoff |
| Presence | `presence.go` | TTL keys, refreshed by WS pings |
| Cache | `cache.go` | menu, feature-gate decisions (60s), analytics |
| Rate limiting | `ratelimit.go` | fixed-window per-IP; sensitive endpoints **fail closed** on Redis outage |
| Auth lockout | `lockout.go` | sliding-window brute-force counter for PINs |
| WS tickets | `ws_ticket.go` | short-lived, single-use WebSocket admission |

Plus worker distributed locks (`SetNX`) and payment-escalation dedup markers.

### 2.5 Workers (`backend/internal/worker/worker.go`, 552 lines)

All Redis-lock-guarded (multi-pod safe) and panic-isolated:

1. **StaleSessionCleaner** — abandons sessions past branch timeout.
2. **SessionExpiryWarner** — emits `SESSION_EXPIRING_SOON` (5 min ahead).
3. **PresenceExpiry** — **no-op stub** (debug log only; real presence-loss handling lives in the reactivation pipeline).
4. **SessionTableReconciler** — repairs session↔table drift.
5. **ReactivationPipeline** — drives `active → awaiting_reactivation → abandoned`; never bypasses `payment_pending`.
6. **PaymentPendingEscalation** — **alert-only** (warn @5m, critical @15m): metric + log + event_log + WS `PAYMENT_SETTLEMENT_STALLED` + audit. Never mutates payment or session state — a human settles or cancels. (`docs/payment-escalation-lifecycle.md`)

### 2.6 Realtime

Covered in depth in Part 6. Summary: single-goroutine hub (`internal/websocket/hub.go`), rooms keyed by session, Redis-pub/sub-fed, ~25 event types, ticket-gated admission, snapshot-based reconnect reconciliation. Guests are realtime; **staff/kitchen deliberately poll REST every 10s**.

### 2.7 Authentication — three trust domains, never mixed

| Domain | Mechanism | Notes |
|---|---|---|
| **Guest** | Bespoke HMAC-SHA256 signed token (`internal/auth/guest.go`) — `base64url(claims).base64url(sig)`, audience `qr-dining-guest`, claims: session/branch/tenant/org/participant/credential-version/exp/jti | Not a standard JWT (no `alg` header — immune to alg-confusion, but nonstandard). Enforcement flag `AUTH_GUEST_CREDENTIALS_REQUIRED` is **off** (wave R6) — snapshot currently fails open (finding F-8, see Part 11). WS additionally requires a Redis ticket. |
| **Staff** | PIN login → opaque DB-backed session token (`staff_sessions`) | Brute-force lockout + 10-RPM sensitive limiter (fails closed). Roles: owner/manager/waiter/kitchen. Role checks mostly per-handler; central authz engine (`internal/authz/`) exists but runs in **shadow** until R3. |
| **Platform** | Email login + TOTP MFA (AES-GCM-encrypted secrets), opaque platform session | `middleware/platform_auth.go` explicitly rejects staff tokens. Roles: `super_admin` (global bypass), `support_admin`, `billing_admin`, `read_only_auditor`. |

Tenant scoping: host/`BASE_DOMAIN` resolution middleware + `BranchTenantGuard` on branch-scoped routes; frontend sends `X-Tenant-Slug` in local dev and keeps three separate bearer tokens that are never mixed (`frontend/lib/api/client.ts`).

### 2.8 Payments

- Host-only initiation: `POST /sessions/:id/payments` freezes a **bill snapshot** (subtotal/discount/tax/service/tip/total + source order IDs). Promos apply **at payment initiation**, not order placement (deliberate — migration 000036).
- Method routing (`normalizePaymentMethodStatus`): `cash`/`card_manual` → `requires_staff_confirmation` (staff settles via `PATCH /payments/:id/settle`); `digital` → `provider_pending` via a **generic webhook** endpoint (`POST /webhooks/payments/:provider`) with signature, amount/currency/session verification and idempotency table — no hard-wired gateway SDK; `upi` → either, by config.
- **Split payments** supported: multiple payment rows per session; session auto-closes only when completed payments cover the bill (`maybeCloseSettledSession`).
- Loyalty accrual rides payment completion but is error-isolated and "never alters payment semantics."
- Session cart freezes in `payment_pending`; stalled settlements escalate (alert-only, above).

### 2.9 Sessions

The core aggregate. Six states: `active`, `payment_pending`, `awaiting_reactivation`, `closed`, `abandoned`, `expired`, with an enforced transition table (`internal/domain/statemachine.go`) and invariant docs (`session-lifecycle-state-machine.md`, `payment-finalization-invariants.md`). One active session per table (constraint); reactivation instead of silent death; per-session monotonic sequence numbers for realtime ordering; snapshot endpoint is the recovery source of truth.

### 2.10 Governance: entitlements, flags, billing, support, analytics

This is the largest *built-but-dormant* subsystem:

- **Entitlements** (mig 000029): capability catalog → plan grants → org overrides. `HasCapability(org, key)`.
- **Feature flags** (mig 000030): resolution hierarchy **branch > org > global > catalog default** — distinct from the 9 env-level strict rollout flags.
- **FeatureGate** (`services/feature_gate.go`): `decision = entitlement AND flag`, 60s cache. **The only place governance actually bites today** — it gates staff analytics and loyalty, both default-off.
- **Lifecycle:** org/branch suspend/activate endpoints exist but are "inert until enforcement"; subscription mutations are "recorded/audited, not enforced." A read-only **enforcement observability** surface (`GET /platform/observability/*`) shows where enforcement *would* bite.
- **Billing** (mig 000032): plans CRUD, subscriptions (activate/suspend/renew/cancel/trial/change-plan), billing profiles, full invoice lifecycle. **Shadow-only. Pilot billing must be manual — do not charge anyone through this yet.**
- **Support console:** read-only search + session/order/payment inspection + audit explorer ("observability, not control").
- **Platform analytics:** cross-tenant usage/revenue/health/staff-performance.

### 2.11 Themes

- Backend: `theme_presets` catalog + `tenant_themes` (preset + allowlist-validated `tokens_json`; custom tokens gated by `custom.theme` entitlement). Mig 000031; serene became default in 000038.
- Frontend: `GET /branches/:id/theme` → `applyTheme.ts` writes `data-theme` + inline CSS vars on `<html>` — hex-validated, allowlisted keys mirroring `services/theme.go`, **never persisted** (no cross-tenant leak), cleared on branch switch. Guest QR payload carries a legacy preset for instant paint before the authoritative theme arrives.
- **Scoping decision (locked):** tenant theming applies to the **guest surface only**. Staff/platform run the static "Harmony" ops design (`styles/surfaces.css`, `[data-surface="ops"]`) — which structurally killed the "staff theme resets" bug class.

### 2.12 Staff analytics & loyalty

Both fully merged into the working tree, both FeatureGated off by default:

- **Staff analytics** (mig 000034, additive/derived): waiter/kitchen/summary metrics derived from `event_log` + `payments.settled_by_staff_id` + `staff_sessions`; no new event capture; entitlement `analytics.staff_performance` + flag `staff_performance_analytics`.
- **Loyalty** (mig 000035): org-level earn rule; append-only **signed-points ledger** (`customer_loyalty_transactions`: earn>0, redeem<0); balance `CHECK >= 0`; earn idempotent per payment (partial unique index). **Redemption is ledger-only** — a staff-witnessed points deduction that never touches bill/payment math. Gated by `loyalty.redeem`.

### 2.13 Frontend

Next.js 15.5 / React 19 / App Router / TS / Tailwind v4 / zustand / framer-motion / recharts. One app, three route groups:

- **(guest):** `table/[token]` (QR landing) → `session/[id]/` (welcome, menu, cart, orders, payment, assist). Serene design: alabaster `#FAF9F5`, charcoal ink, Hanken Grotesk + Cormorant Garamond.
- **(staff):** PIN login → waiter (tables, ready-to-serve, settlement queues), kitchen (10s polling display), admin (12 tabs: sessions, menu, tables, collateral, staff, stats, performance, loyalty, plan, promos, appearance, settings).
- **(platform):** dashboard, organizations (+billing), plans, entitlements, feature-flags, themes, collateral studio, analytics, observability, onboarding, support (search/audit/detail).

State: zustand stores (`store/`) + providers (`SessionProvider` owns snapshot load, WS, theme, terminal-state screens). API: thin fetch wrapper with per-domain tokens and a rich error-code→message map. Prebuild guard (`scripts/check-prod-env.mjs`) blocks insecure prod endpoints.

### 2.14 QR collateral

Migration 000033: `branch_collateral` holds **only physical/content config** (format, welcome text, WiFi, socials) — never theme tokens (theme stays single-source). Full studio in the platform console + a mirror tab in restaurant admin; print/PDF-faithful renderers for sticker/table-tent/square/standing/bulk formats; ZIP export.

### 2.15 Deployment

- **Build:** 2-stage Dockerfile, `golang:1.24-alpine` builder hardcoded `GOARCH=arm64` (Oracle Ampere), static binary, non-root, `HEALTHCHECK /readyz`. ⚠️ `go.mod` says `go 1.26.0` — mismatch with Docker/CI 1.24.
- **Runtime:** `docker-compose.yml` (prod: postgres:17 + redis:7 AOF/LRU + app on external proxy network), plus staging / pilot-validation / manual-testing compose files. nginx `deploy/nginx/qr-dining.conf` (WS-aware, 3600s read timeout > 54s app ping).
- **Observability stack committed:** `deploy/observability/` — Prometheus 2.54 + Alertmanager 0.27 + blackbox + node-exporter + Grafana dashboard + 27 promtool-clean alert rules.
- **Backups:** nightly systemd timer (`deploy/backup/`), R2/S3/local providers, manifest + retention; restore certified byte-faithful (56/56 tables) — and **verified end-to-end against the real production R2 bucket (2026-07-18, RECOVERY.md §5)**.

---

## Part 3 — User Roles

### Guest (anonymous diner)
- **Capabilities:** scan QR → create or join a session (name + optional phone); browse menu; add to the *shared* cart; request assistance; view live order status; view bill.
- **Restrictions:** cannot send the order or request the bill unless host (`NOT_SESSION_HOST`); cart frozen during `payment_pending`; cannot see other sessions (branch/tenant guarded); WS requires a ticket.
- **Lifecycle:** exists only within a session; token carries participant identity + credential version; on session close/abandon/expire the guest sees a terminal "session ended" screen. Optional phone links to a `customers` record (loyalty opt-in).
- **Interactions:** peers (shared cart, presence avatars), host (transfer), kitchen/waiter indirectly via orders and assistance.

### Participant vs Host
- Every guest in a session is a **participant**; exactly one is **host** (defaults to session creator). Host gates order submission, bill requests, payment initiation, and can hand off (`POST /sessions/:id/host` → `HOST_CHANGED` broadcast). This is a deliberate anti-chaos design for group dining.

### Waiter (staff)
- **Capabilities:** PIN login; table board; serve ready items; settle cash/card-manual payments; acknowledge/resolve assistance; witness loyalty redemptions.
- **Restrictions:** branch-scoped; cannot manage staff or menu; PIN lockout on brute force.
- **Lifecycle:** provisioned by owner/manager; `staff.is_active` toggles; opaque DB sessions revocable.

### Kitchen (staff)
- **Capabilities:** kitchen display (confirmed → preparing → ready), item-level detail with modifiers.
- **Restrictions:** no payment or table authority; "served" is the waiter's action (removed from kitchen board deliberately).
- **Interactions:** consumes `ORDER_*` transitions; 10s polling, not WS.

### Manager (staff)
- **Capabilities:** waiter+kitchen powers plus admin tabs: menu, tables, promos, stats, performance, loyalty config, collateral, settings.
- **Restrictions:** cannot add staff (owner-only — certified as intended RBAC, though it surprised manual testing as finding v4); branch-scoped.

### Organization Owner (staff)
- **Capabilities:** everything a manager has plus staff roster management, PIN resets, plan/appearance, org-level view across branches (org memberships).
- **Restrictions:** no platform powers; tenant-scoped absolutely.

### Platform Support (`support_admin`)
- Read-only cross-tenant search and inspection (sessions/orders/payments/audit). Explicitly "observability, not control." Support sessions are themselves audited.

### Platform Billing (`billing_admin`)
- Plans, subscriptions, invoices, billing profiles — all currently **shadow** (recorded, audited, unenforced).

### Platform Admin / Super Admin (`super_admin`)
- Full platform console: org/branch lifecycle, entitlements, flags, themes, collateral, analytics, observability, onboarding. TOTP MFA required. `PlatformHasRole` grants super_admin a global bypass. Separate trust domain — staff tokens rejected outright.
- **Gap:** no first-super-admin bootstrap UI (seeding is manual — dress-rehearsal finding).

### Read-only Auditor (`read_only_auditor`)
- Platform console in view-only mode; UI gates affordances (`lib/platform-rbac.ts`), backend authoritative.

---

## Part 4 — Feature Inventory

Legend: **PR** = production ready · **PI** = pilot ready · **EX** = experimental · **DF** = deferred · 🚩 = behind strict env flag · 🎫 = entitlement-gated (FeatureGate = entitlement AND flag)

### Core Restaurant
| Feature | Status | Gating |
|---|---|---|
| QR → table resolution → session create/join | **PR** | — |
| Shared session cart (multi-guest, live) | **PR** | — |
| Host-controlled ordering + host transfer | **PR** | — |
| Menu (categories, modifiers incl. single-select, availability, featured, images) | **PR** | — |
| Order lifecycle (pending→confirmed→preparing→ready→served / cancelled) | **PR** | — |
| Kitchen display | **PI** (10s polling; fine ≤10 restaurants) | — |
| Waiter board (serve + settlement queues) | **PI** | — |
| Assistance requests | **PR** | — |
| Bill snapshot + promos-at-payment | **PR** | — |
| Split payments; cash/card-manual staff settlement | **PR** | — |
| Digital payment via generic webhook | **PI** (no real gateway integrated yet) | — |
| Payment escalation (alert-only) | **PR** | — |
| Session reactivation / "still here?" | **PR** (timings tuned post-rehearsal) | — |
| Human-readable operational IDs, business dates | **PR** | — |

### Platform & Governance
| Feature | Status | Gating |
|---|---|---|
| Org/branch model + memberships | **PR** (data), enforcement 🚩 R2 | 🚩 `TENANCY_ORGANIZATIONS_ENABLED` |
| Central authz policy engine | **DF** (shadow) | 🚩 R3 pair |
| Entitlements (catalog/plans/overrides) | **PI** (resolve-only) | — |
| Feature-flag targeting (branch>org>global) | **PI** | — |
| Org/branch suspend-activate | **DF** (inert until enforcement) | — |
| Subscription billing + invoices | **DF** (shadow; pilot bills manually) | — |
| Enforcement observability endpoints | **PR** | — |
| Platform onboarding workflow | **PI** | — |
| Platform MFA (TOTP) | **PR** | — |

### Operations
| Feature | Status |
|---|---|
| Immutable audit log v2 | **PR** — 🚩 `AUDIT_LOG_V2_ENABLED` (R1, **live & soaked**) |
| Prometheus metrics + 27 alerts + Grafana | **PR** (rules committed; receiver/human setup pending) |
| Nightly backups + verified restore | **PR** (real-R2 round-trip passed 2026-07-18) |
| Chaos harness (`scripts/chaos/`) | **PR** |
| Manual-testing multi-tenant stack | **PR** (docs in `docs/manual-testing/`) |

### Billing · Analytics · Branding · Support · Experimental
| Feature | Status | Gating |
|---|---|---|
| Staff performance analytics | **PI** | 🎫 `analytics.staff_performance` + flag |
| Customer loyalty (ledger, earn/redeem) | **PI** | 🎫 `loyalty.*` + flag |
| Restaurant stats (busy hours, top items, volume) | **PR** | — |
| Platform cross-tenant analytics | **PI** | — |
| Tenant themes (5 presets + custom tokens) | **PR** | 🎫 `custom.theme` for custom tokens |
| Serene guest design system | **PR** (unsoaked) | — |
| Harmony ops design system | **PR** (unsoaked) | — |
| Premium QR collateral studio + print/ZIP | **PI** | — |
| Support console (read-only) | **PR** | — |
| Presence-expiry worker | **EX** (stub) | — |
| Audit hash-chain (`row_hash`) | **DF** (columns exist, NULL) | — |

### The 9 strict rollout flags (all default `false`; only R1 flipped)
`AUDIT_LOG_V2_ENABLED` (R1 ✅ live) · `TENANCY_ORGANIZATIONS_ENABLED` (R2) · `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS` (R3, paired) · `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED` (R4, paired) · `WS_TICKET_AUTH_REQUIRED` (R5) · `AUTH_GUEST_CREDENTIALS_REQUIRED` (R6) · `PAYMENT_STAFF_SETTLEMENT_REQUIRED` (R7).

---

## Part 5 — Data Model

38 migrations (000001–000038, all reversible, round-trip verified at v38), 58 tables.

### Aggregates

**Tenancy (dual-model — the biggest schema debt):**
- New: `organizations` → `branches` (+ `organization_members`, `organization_branch_memberships`). Branch = isolation boundary; org = governance boundary. Added migration 000017.
- Legacy: `restaurants`, `restaurant_subscriptions` still live and still routed (`/restaurants/:id/subscription`). Do not drop until R2/R3 enforcement is metric-proven — but do not build new features on it either.

**Dining operations:**
- `tables` → `sessions` (6-state) → `session_participants`, `session_events`, `session_sequences` (per-session monotonic ordering), `carts`/`cart_items` (shared, mig 000027), `orders`/`order_items` (+ `order_sequences` for human-readable IDs), `assistance_requests`.

**Menu:** `menu_categories`, `menu_items` (featured/metadata/images), `item_modifiers` (single-select enforcement, mig 000037).

**Money:** `payments` (9-state), `bill_snapshots` (frozen bill at initiation — the payment-correctness cornerstone), `payment_webhook_events` (idempotency), `payment_sequences`, `promos`/`promo_redemptions` (redeemed at payment, mig 000036), `idempotency_keys`.

**Identity/audit:** `staff` + `staff_sessions`, `customers` (`UNIQUE(restaurant_id, phone_e164)`), `platform_users`/`platform_user_roles`/`platform_sessions`/`platform_user_mfa`/`platform_mfa_challenges`/`platform_support_sessions`, `audit_log` (trigger-immutable) + `platform_audit_log` + `event_log`.

**Governance:** `entitlements`, `plan_entitlements`, `organization_plan_assignments`, `organization_entitlement_overrides`, `platform_feature_flags` + global/org/branch override tables, `subscription_plans`, `organization_subscriptions`, `subscription_invoices`, `subscription_payments`, `organization_billing_profiles`.

**Branding/loyalty/analytics:** `theme_presets`, `tenant_themes`, `branch_collateral`, `organization_loyalty_programs`, `customer_loyalty_accounts`, `customer_loyalty_transactions`.

### Critical invariants — never change casually

1. **Session transition table** (`internal/domain/statemachine.go`) — every worker, handler, and the frontend's terminal-state handling assume it. `payment_pending` can never be bypassed by lifecycle automation.
2. **One active session per table** — the joinability predicate vs this constraint was the source of cert finding #11; keep them aligned.
3. **Bill snapshot immutability** — the bill a guest agreed to is frozen; promos/loyalty never retroactively edit it.
4. **Audit log immutability trigger** — the R1 soak certified it; any migration touching `audit_log` invalidates the soak lineage.
5. **Loyalty ledger:** append-only, signed points, `CHECK (points_balance >= 0)`, earn-idempotency partial unique index per payment.
6. **Webhook idempotency** (`payment_webhook_events`) and payment state machine terminal states.
7. **Additive migrations only** — the standing rule since Phase 3; enum values are added via `ALTER TYPE ... ADD VALUE`, never removed.

### Migration maturity
High. Numbered, reversible, embedded, auto-run at boot (idempotent — chaos-validated), sqlc drift-checked in CI, full round-trip verified at v38. The discipline here is one of the project's strongest assets.

---

## Part 6 — Realtime

### Lifecycle
1. Guest authenticates → requests a **WS ticket** (short-lived, Redis, single-use) → connects to `/ws` with it.
2. Hub (`internal/websocket/hub.go`, single goroutine, no mutexes — rooms map owned by one loop) registers the client into its session room.
3. Events flow: service → Redis pub/sub → hub subscriber → room broadcast. ~25 event types (`SESSION_*`, `PARTICIPANT_JOINED/LEFT`, `ITEM_ADDED/REMOVED`, `CART_UPDATED`, `ORDER_PLACED/CONFIRMED/PREPARING/READY/SERVED/CANCELLED`, `ASSISTANCE_*`, `PAYMENT_INITIATED/COMPLETED/SETTLEMENT_STALLED`, `HOST_CHANGED`, `PROMO_APPLIED`, `MENU_ITEM_AVAILABILITY_CHANGED`, `PING/PONG`).

### Ordering & reconciliation
Per-session monotonic sequence numbers (`session_sequences`). The client tracks the last seen sequence; on reconnect it fetches `GET /sessions/:id/snapshot` (the server advertises this via `X-Reconnect-Endpoint`) and reconciles (`frontend/lib/ws/reconciliation.ts`) — **the snapshot is authoritative**, events are advisory. This "backend-authoritative recovery" is the realtime design's core principle.

### Reconnect
Client: exponential backoff 1→30s, max 10 attempts, 30s ping. Reconnect storms are bounded server-side (12/min per session admitted, rest 429 — chaos-validated). **Known debt:** after 10 failed attempts the client dead-ends ("Connection lost") with no manual retry affordance; and reconnect into an `awaiting_reactivation` session 409s on the ticket instead of routing through reactivation (dress-rehearsal F-1) — verify this fix landed in the cert branch before pilot.

### Presence
WS pings double as presence heartbeats (Redis TTL keys). Loss of presence feeds the reactivation pipeline (60s+60s windows — tuned after the rehearsal found them too aggressive for real dining). The dedicated presence-expiry worker is a stub.

### Payment sync
`PAYMENT_INITIATED` freezes the cart across all participants' UIs; `PAYMENT_COMPLETED` (webhook or staff settlement) → possible session close → terminal screens everywhere; `PAYMENT_SETTLEMENT_STALLED` reaches both guests and staff.

### Failure recovery
- Redis down: WS pauses, HTTP continues, hub subscriber auto-reconnects with backoff (max 30s), sensitive limiters fail closed. Chaos-validated.
- Slow consumers: evicted immediately (non-blocking send) — a slow phone cannot stall a room.
- Inbound abuse: 100-strike force-close; per-IP connection caps (sizing before R5 still open).
- App restart: ~12s; clients reconcile via snapshot.

### Deliberate asymmetry
Staff and kitchen poll REST every 10s instead of using WS. This was a conscious scale/simplicity decision (the hub is guest-session-scoped; a branch-scoped staff channel is deferred). Cost: up to 10s kitchen latency. Fine for pilot; revisit at ~50 restaurants.

---

## Part 7 — Operational Readiness

### Deployment model (today)
Single Oracle Cloud Ampere arm64 host, docker-compose (app + Postgres 17 + Redis 7), nginx in front, migrations auto-run at boot, stateless app (~12s restart recovery), all strict flags via env. The soak/staging stack runs locally under docker project `qr-dining` (`qr-app-soak`/`qr-app-chaos` + postgres + redis).

> **Standing rule: never touch the soak stack.** No `compose down -v`, no backend restart without `-e AUDIT_LOG_V2_ENABLED=true`, no writes to `audit_log`, no volume wipes. All experimentation belongs on the isolated `manual-testing` stack (backends :8090/:8095, frontends :3000/:3001) or throwaway DBs.

### Backup / restore
- **Complete:** nightly systemd timer, provider abstraction (R2/S3/local), manifests + retention, and a certified byte-faithful restore (pg_restore --clean --single-transaction; 56/56 tables row-identical, trigger + checksum fidelity — `restore-verification-report.md`).
- **Closed 2026-07-18:** the full round-trip against the **real production R2 bucket** PASSED (RECOVERY.md §5) — the former SEV-1. Still required on the prod host: backup-metric textfile wiring into node-exporter.

### Alerting / monitoring
- **Complete:** 27 promtool-clean rules (page vs ticket severities), rollout-gate alerts that must read zero before flag flips (`LegacyAuthzBypassPresent`, `LegacyIdentityUsagePresent`), Grafana dashboard, blackbox probes, full compose for the observability stack; app metrics cover WS, workers, escalations, cache, DB pool, audit failures, rate limits, tenant resolution.
- **Human setup still required** (`docs/alerting-production-setup.html`): real Alertmanager receiver (current config points at a placeholder webhook sink), production scrape targets, `/metrics` CIDR for the real network, standing the stack up persistently, an app-down/`/readyz` page alert.

### Health checks
`/health` (liveness), `/readyz` (503 on PG/Redis loss — wired into container healthcheck and LB), `/metrics`. All validated in staging topology.

### Secrets
Fail-hard on missing prod secrets (pilot-readiness fix); MFA encryption key, guest-token secret, webhook secrets via env. ⚠️ `godotenv.Overload()` means a stray `.env` on the host **overrides** injected env — ensure no `.env` ships to prod.

### Load & soak
- Load: validated at concurrency 50 — p99 ~200ms, ~0.4% errors (pilot-scale proof). Known ceiling: `session_sequences` write hot-spot produces ~5% 500s at concurrency ~150 (single-branch worst case; far beyond pilot).
- Soak: R1 audit-writer soak **passed** 2026-06-04 — ~124h continuous, RestartCount=0, zero panics/audit failures. Qualifiers: final ~75h ran unprobed/idle; the binary ran from bind-mounted `/tmp` and was wiped once by tmp-cleanup (34h outage, clock reset) — **relocate the binary off `/tmp` before any future soak**. And the SEV-0: this soak does **not** cover the current RC.

### What is complete vs pending — summary
| Area | Complete | Requires human |
|---|---|---|
| Deploy tooling | ✅ compose, Dockerfile, nginx, migrations | provision prod host, inject secrets |
| Backups | ✅ automation + verified restore + real-R2 round-trip (2026-07-18) | textfile-metric wiring on prod host |
| Alerting | ✅ rules/dashboards/probes committed | receiver, targets, stand up stack |
| Soak | ✅ methodology + one passed soak | **fresh soak of the RC build** (SEV-0) |
| Release eng | ✅ branch is RC-quality | merge→main, tag `v1.0.0-rc.1`, green integration suite |

---

## Part 8 — Development History

### Arc
1. **Prototype era** → `operational-correctness-audit.md` (2026-05-21) declared it **not production-ready** with 6 deployment blockers (PIN-only ambiguous staff identity, trusted `X-Participant-ID` guest identity, inconsistent branch isolation, unsafe payment finalization, stranded tables, session-by-UUID exposure). That audit is the spine of everything since.
2. **Hardening phases 0–8** (plans in `docs/history/plans/`): guardrails → staff identity → RBAC/ownership → org model → platform trust domain → immutable audit v2 → realtime/session hardening (WS tickets, state machine, reactivation) → payment/order correctness (bill snapshots, idempotency, webhook safety) → operational UX (readable IDs, business dates).
3. **Stabilization phases A–E:** lifecycle states → reactivation → **Phase C behavioral convergence** (killed 13 FE/BE contract divergences; built the 106-spec Playwright matrix) → **Phase D staging/chaos validation** (6 chaos experiments, all PASS) → **Phase E enforcement instrumentation** (6 rollout metrics, escalation worker, `/readyz` fix).
4. **Rollout waves R1–R7** defined (`docs/master-system-context-v1.md` §2.5); pre-R1 e2e sweep found 43 failures → 5 genuine P0s fixed; **R1 flipped 2026-05-25**, soak closed **PASS 2026-06-04**.
5. **Second arc — SaaS expansion:** platform governance/entitlements (shadow), support console, pilot-readiness remediation, premium QR collateral, staff analytics + loyalty.
6. **Third arc — certification + redesign (current branch):** a full manual certification pass found ~18 bugs (findings v4) → Phase 0 fixes C1–C20 (migrations 000036–000038) → the two-track "Serene" (guest) + "Harmony" (ops) redesign → merges of both tracks → e2e reconciliation fixes. This produced the current RC candidate.

### Rollout philosophy (the project's defining decision)
**Shadow before strict. One wave at a time. Metrics gate flags. Flags are reversible in <5 minutes. Humans stay the authority for money.** Nothing risky is enabled by default; enforcement follows observation.

### Deliberately deferred (with reasons)
- Governance enforcement & billing charging — observe first; a pilot with one trusted restaurant doesn't need it.
- Central authz enforcement (R3) — shadow until 48h of zero mismatches.
- Guest credential enforcement (R6) — needs legacy decay window; but see F-8 in Part 11.
- Staff WS realtime — polling is adequate at pilot scale; hub sharding/branch channels deferred.
- Audit hash-chain, e2e full reconciliation post-redesign, webhook exact-replay CI proof (pre-R7), multi-instance WS soak.

### Stale documents — corrections of record
These remain in `docs/`/`docs/history/` unedited; **this section is the correction**:
1. `docs/project-strategic-context-v1.md` §13 still lists the T-01 cross-org snapshot leak as open. **It was fixed 2026-05-29 (`6db078e`).** (F-8, the *no-token* fail-open, is a different, still-open issue.)
2. `final-production-readiness-assessment.md`, `final-rollout-gates-status.md`, `post-remediation-rollout-status.md` (05-25/26) list the R3 policy decisions as the last hard blocker. **Closed 2026-05-29** (`r3-policy-decisions-v1.md` — all three decisions ratified existing behavior).
3. `final-pilot-readiness-report.md` (06-10) is superseded by `docs/history/final-release-readiness-report.md` (06-25).
4. `docs/history/manual-testing-findings-v4.md` reads as 12 open findings; **most were fixed** in cert Phase 0 (promo timezone bug remains open).
5. `docs/history/HANDOUT.md` describes the redesign as "next action C16"; the branch has since completed the entire redesign through schema v38.
6. "R1 is soaked" ≠ "the pilot code is soaked" — the soak covered the pre-redesign binary only.

---

## Part 9 — Current Branch Strategy

### Topology (verified via merge-base ancestry)

| Branch | vs main | Status |
|---|---|---|
| `main` @ `075bbf8` | — | **Stale spine.** Ends at R3 governance regression tests. ~136 commits behind reality. |
| `feature/certification-fixes-ui-redesign` @ `9dd869a` | +136 / −0 | **The de-facto trunk and RC.** Strict superset of every other branch. |
| `feature/staff-analytics-loyalty` | +92 | Fully absorbed into HEAD (ancestor). Prunable. |
| `premium-qr-collateral` | +75 | Fully absorbed. Prunable. |
| `pilot-readiness-remediation` | +45 | Fully absorbed. Prunable. |
| `platform-governance-entitlements` | +42 | Fully absorbed (shares a tip lineage with pilot-readiness). Prunable. |

`feature/redesign-guest` and its worktree no longer exist — absorbed and pruned. There are no remotes configured.

### Recommendation

1. **Merge `feature/certification-fixes-ui-redesign` → `main` with `--no-ff`** (preserves the merge point), then **tag `v1.0.0-rc.1`**. This is already an open SEV-1 on the release checklist; nothing else should land first.
2. **Delete the four absorbed branches** after the merge (they add nothing and mislead readers into thinking work is stranded on them).
3. **Build the soak/pilot binary from the tag**, never from a branch tip.
4. Going forward: `main` is the only long-lived branch; short-lived feature branches merge back promptly. The multi-month divergence pattern that produced this situation should not be repeated — it worked only because one person held all context.
5. Nothing should remain isolated. There is no branch left whose content is experimental enough to warrant quarantine; the flag system, not branches, is the correct isolation mechanism now.

---

## Part 10 — Technical Debt

### Intentional debt (documented, defensible)
- **Governance/billing built but unenforced** — by rollout design; do not "finish" it by flipping enforcement casually.
- **Staff/kitchen 10s polling** — deliberate; revisit at scale.
- **Dual restaurant/organization model** — legacy kept alive until strict enforcement is metric-proven; additive-only rule protects it.
- **Bespoke HMAC guest token** — nonstandard but sound; migrating to a JWT library is cosmetic, not urgent.
- **Audit hash-chain columns NULL**, **e2e post-redesign reconciliation partial**, **webhook exact-replay proof deferred to pre-R7 CI**.

### Accidental debt (fix-worthy)
1. **CI gap (worst item):** CI runs lint + `internal/domain` unit tests + sqlc drift + docker build only. The 13 integration files and 106 Playwright specs never run in CI. The integration suite is currently **red** (11 test-side failures; harness deadlocks `pgxpool.Close()`).
2. **Float money math** — accumulation in floats; should be integer paise. Pre-scale must-fix.
3. **Promo daily-window timezone bug** — windows compared against DB UTC, not branch tz; windowed promos silently fail. Open since findings v4.
4. **WS reconnect dead-end** after 10 attempts (no retry affordance).
5. **`go.mod` go 1.26 vs Docker/CI go 1.24.**
6. **`godotenv.Overload()`** prod footgun.
7. **567 frontend `no-unused-vars` warnings** (redesign residue); empty `frontend/features/*` scaffolding; `AppearanceTab` not folded into `SettingsTab`; stale `seed.go` demo step.
8. **`server.go` 489-line constructor** — wiring will become unmanageable; fine now, refactor when it hurts.
9. **`/tmp`-hosted soak binary** — bitten twice; relocate.
10. **Two collateral surfaces** (platform studio + admin tab) — watch for config-schema divergence.

### Never touch casually
Session transition table · bill snapshot semantics · audit-log schema/trigger · loyalty ledger constraints · additive-migration rule · the soak stack · payment escalation's alert-only property.

### Deserves redesign after public launch
Money as integers end-to-end · staff realtime (branch-scoped WS channel) · hub sharding for multi-pod WS · dropping the legacy restaurant model · consolidated `server.go` wiring (DI or per-module registration) · CI running the full pyramid.

---

## Part 11 — Pilot Readiness

**Scenario: Restaurant #1 onboards tomorrow.**

### Onboarding (works, with manual steps)
Platform onboarding flow exists (org → branches → tables → QR package → readiness summary). Manual: super-admin bootstrap (no UI — seed by hand), staff roster + PINs, menu entry, printed collateral from the studio, tenant theme selection. Realistic effort: half a day with you present. Billing: **manual invoice outside the system** (subsystem is shadow).

### Daily operations
Guests order collaboratively; kitchen works the (polled) display; waiter serves and settles cash/card-manual; escalation alerts catch stalled settlements; sessions self-heal via reconciler/reactivation. The workflows were validated end-to-end in manual testing v3 and the dress rehearsal. Payment reality for India: UPI-as-staff-confirmed or cash — no live gateway integration yet, which is fine for pilot.

### Support
You are the support organization. Tools are genuinely good: read-only support console, audit explorer, observability endpoints, runbooks (`operational-runbooks.md`, `support-runbooks-pilot.md`, `recovery-procedures.md`). Escalation paths assume a human with psql access for anything the console can't do.

### What could realistically go wrong
1. **Unsoaked RC misbehaves under real load** — the SEV-0. A memory leak or redesign-era regression would surface mid-service. *Mitigation: run the soak first. Non-negotiable.*
2. **Alerting isn't actually wired** — rules fire into a placeholder webhook sink. An outage would be discovered by the restaurant, not you. *Mitigation: real receiver + app-down alert before day 1 (SEV-1).*
3. **F-8 exposure:** with R6 off, a session snapshot can be fetched tokenlessly (session token + participant PII in the payload). Low risk in a single-tenant supervised pilot (UUIDs unguessable), **gates any public/multi-tenant exposure**.
4. **Reconnect edge cases** (F-1 lineage): a guest who backgrounds their phone through reactivation may dead-end and need a QR re-scan. Annoying, recoverable.
5. **Promo timezone bug** — if Restaurant #1 uses time-windowed promos, they'll silently not apply. Don't configure windowed promos, or fix first.
6. **Host-model confusion** — non-host guests hitting "only the host can send the order" needs staff able to explain host transfer. Training item.
7. **`/tmp`/env footguns on the prod host** — binary placement and stray `.env` files have both bitten before; the deployment checklist covers them — follow it literally.

### Burden estimate
- **Operational:** ~1–2 h/day during week 1 (monitoring dashboards, settling escalations questions, DB spot-checks), decaying to <30 min/day.
- **Support:** front-loaded staff training (host model, settlement flow, PIN discipline); expect most tickets in week 1 to be workflow questions, not bugs — manual testing v3/v4 already burned down the workflow-bug class.

**Verdict: conditional GO, matching `final-release-readiness-report.md` (~93%).** Conditions: fresh soak of the RC tag, real alert receiver, merge+tag. (Two conditions since met: R2 backup round-trip passed 2026-07-18; integration suite green at `439de14`.)

---

## Part 12 — Public Launch Roadmap

### Pilot → 10 restaurants
- **Must change:** flip R2 (org tenancy) after backfill; enable R4/R5 (staff code + WS tickets) after decay windows; real Alertmanager paging; onboarding runbook so a second human could do it; fix promo tz bug and reconnect dead-end.
- **Can stay:** single host, compose deployment, polling staff UIs, manual billing, single Postgres.

### 10 → 50
- **Must change:** flip R3 (central authz) and R6 (guest credentials — closes F-8) after shadow windows; integrate one real payment gateway behind the existing webhook abstraction; self-serve-ish onboarding (super-admin bootstrap UI, menu import); CI runs integration + smoke e2e; support rotation beyond one person; billing enforcement pilot (invoices generated, still human-approved).
- **Watch:** Postgres sizing (`DB_MAX_CONNS`), disk curves from real audit volume.

### 50 → 500
- **Must change:** multi-instance app (the architecture is ready: stateless app, Redis-locked workers, pub/sub-fed hub — but needs a real multi-pod WS soak); managed/dedicated Postgres with replicas; staff realtime (branch-scoped WS) replaces polling; integer money migration completed; observability as a managed service; entitlement enforcement on (the business needs plans to mean something); drop legacy restaurant model.
- **Organizational:** an on-call rotation, SLOs, status page.

### Thousands
- Postgres partitioning (sessions/events/audit by branch or date), hub sharding or a dedicated realtime tier, per-region deployment, a real billing/invoicing integration (or the shadow system graduates), data-retention/archival policy for the audit log, formal tenant data-export/deletion (compliance).
- **What never needed changing:** the session state machine, bill-snapshot semantics, the trust-domain split, the flag-gated rollout method — these were built scale-agnostic.

---

## Part 13 — Product Vision (grounded in what exists)

**6 months:** 5–15 live restaurants in one city. Loyalty and staff analytics flipped on for tenants who want them (the FeatureGate plumbing makes this a config change, not a project). One payment gateway live. QR collateral becomes an onboarding delight ("we print your table kit"). The support console proves its worth as the churn-prevention tool.

**1 year:** 50–100 restaurants. The governance layer graduates from shadow to enforced: plans, entitlements, and billing become the actual business model (tiers: core ordering / +loyalty / +analytics / +custom themes). Platform analytics becomes a sellable owner-facing product (cross-branch comparisons for small chains — the org model already supports multi-branch). Staff realtime lands.

**3 years:** A vertical dine-in OS for the Indian market: realtime table operations + loyalty + analytics + branded guest experience, sold per-branch with entitlement-tiered pricing. Realistic adjacencies *already latent in the codebase*: multi-branch chain tooling (org layer), a customer graph from phone-linked loyalty (CRM-lite), payment-gateway revenue share, and white-label theming (the token system is already tenant-safe). Not latent, would be new builds: delivery/takeaway, POS-hardware integration, inventory — treat those as partnerships, not roadmap, unless the market demands otherwise.

---

## Part 14 — Overall Assessment

| Axis | Score | Rationale |
|---|---|---|
| Architecture | **9/10** | Session-centric core, trust-domain separation, Postgres-authoritative/Redis-disposable, flag-gated enforcement. Deducted: dual tenant model, monolithic wiring. |
| Engineering | **8.5/10** | Strict state machines, sqlc discipline, additive migrations, invariant docs. Deducted: float money, bespoke token, lint residue. |
| Testing | **6.5/10** | 106-spec e2e matrix + chaos + integration suites *exist* and are strong; but the integration suite was red until the certification fixes (green at `439de14`), e2e post-redesign reconciliation is partial, and **CI runs almost none of it**. The gap is execution, not coverage. |
| Security | **8/10** | Three trust domains, MFA, immutable audit, ticket-gated WS, fail-closed limiters, webhook verification. Deducted: F-8 open until R6, per-handler authz until R3. |
| Operations | **7.5/10** | Backups+restore verified (incl. real-R2 round-trip 2026-07-18), 27 alerts, chaos-proven degradation, runbooks. Deducted: alert receiver is a placeholder, `/tmp` incident ×2. |
| Developer Experience | **7/10** | Excellent docs and scripts; manual-testing stack is superb. Deducted: doc sprawl (now archived), 489-line constructor, dead scaffolding. |
| Maintainability | **8/10** | Layering, invariants isolated in `domain/`, migrations exemplary. |
| Scalability | **7/10** | Stateless app + locked workers + pub/sub are multi-pod-ready but unproven multi-instance; known sequence hot-spot; polling staff UIs. |
| Product Maturity | **7/10** | Guest+staff flows are certified and polished (post-redesign); governance/billing are shadow; zero real-world usage. |
| Operational Maturity | **6.5/10** | Method is excellent; live proof is thin (single-instance local soak, no production host yet). |
| **Pilot Readiness** | **8/10 — conditional GO** | ~93% per the June release audit; blocked only by soak + release-engineering conditions, not by product gaps. |
| **Public Launch Readiness** | **4/10** | R2–R7 unflipped, F-8 open, no gateway, no CI safety net, one-person ops. By design — the roadmap to close it is already written. |

**One-line verdict:** an unusually disciplined pre-launch system whose engineering risk is genuinely retired; what remains is operational proof (soak), release engineering (merge/tag/CI), and go-to-market courage.

---

## Part 15 — Final Recommendations

### Next 30 days — Must Do
1. **Merge → `main` (no-ff), tag `v1.0.0-rc.1`, delete the 4 absorbed branches.** (Part 9.)
2. ~~Fix the red integration suite~~ — **done during certification** (`f493dc9`; suite green at `439de14`).
3. **Run the RC soak:** binary built from the tag, off `/tmp`, `AUDIT_LOG_V2` on, continuous synthetic traffic + external probing for the full window, storage curve recorded.
4. **Wire real alerting:** actual receiver, app-down/`/readyz` alert, production scrape targets.
5. ~~Real-R2 backup round-trip~~ — **done 2026-07-18** (PASS; RECOVERY.md §5).
6. **Provision the prod host** (secrets injected, no `.env`, binary placement per checklist).
7. Onboard Restaurant #1 supervised; bill manually.

### Next 30 days — Should Do
- Fix the promo timezone bug and the WS reconnect dead-end before the pilot's first weekend.
- Align `go.mod` with Docker/CI Go version.
- Add integration suite + a 10-spec e2e smoke pack to CI.

### Next 90 days — Must Do
1. Pilot retro → burn down the top field findings.
2. Flip **R2** (org tenancy) after `branches.organization_id` backfill + soak; start **R3 shadow** (48h zero-mismatch gate) and **R4** staff-code decay window.
3. Establish the weekly cadence: one wave at a time, never chained.
4. CI runs the full integration suite on every PR.

### Next 90 days — Should Do
- Integrate one real payment gateway (UPI-capable) behind the existing webhook layer.
- Super-admin bootstrap UI + onboarding runbook (make onboarding repeatable by someone who isn't you).
- Restaurants #2–#5.
- e2e full reconciliation with the redesigned DOM.

### Next year — Must Do
1. Complete the wave ladder through **R6** (closes F-8 — precondition for any public/multi-tenant exposure) and **R7** (with the webhook-replay CI proof).
2. **Integer money migration.**
3. Multi-instance deployment with a real multi-pod WS soak.
4. Graduate billing from shadow to enforced (start with invoice generation + human approval).
5. Second operator: on-call, runbook-driven support, SLOs.

### Next year — Should Do
- Staff realtime (branch-scoped WS channel); retire kitchen polling.
- Drop the legacy `restaurants` model once R2/R3 metrics prove it dead.
- Audit hash-chain activation; data-retention policy for `audit_log`.
- Refactor `server.go` wiring into per-module registration.

### Nice To Have (any horizon)
- Standard JWT library for guest tokens; menu import tooling; owner-facing multi-branch analytics packaging; white-label theme marketplace; collateral-surface consolidation; fold `AppearanceTab` into `SettingsTab`; prune the 567 lint warnings.

---

## Appendix — Document Map (post-cleanup, 2026-07-18)

- **`docs/`** (living): `master-system-context-v1.md` (canonical map; §2.5 wave ledger), `project-strategic-context-v1.md` (strategy; §13 stale — see Part 8), `payment-escalation-lifecycle.md`, `production-alerting-baseline.md`, `alerting-production-setup.html`, `r2-production-setup.html`.
- **`docs/manual-testing/`**: `testing-dashboard.html` (entry point — `scripts/manual-testing-up.sh` prints this path), guides, checklist, `database-reference.html`.
- **`docs/history/`** (archived release trail, unedited): hardening/stabilization phase reports, all readiness/audit/gate reports, R1 soak docs, chaos + e2e analyses, manual-testing findings v1–v4, implementation plans, `plans/` (hardening phase plans 0–9), `HANDOUT.md` (superseded session log).
- **Repo root** (tracked reference docs kept in place): `openapi.yaml` (v2.2.0), invariant docs (`session-lifecycle-state-machine.md`, `payment-finalization-invariants.md`, `realtime-reconciliation-invariants.md`), runbooks, checklists, `r3-policy-decisions-v1.md`, `restore-verification-report.md`, certification reports.
- **Deleted** (ephemeral, regenerable): all root screenshots/renders, Playwright output dirs, standalone design mockups, redesign sample screens.
