# Master System Engineering Context — qr-dining (v1)

> **Purpose.** This is the master engineering context document for the `qr-dining`
> system. It exists so that future Claude sessions, future developers, and future
> implementation phases can understand the *entire current state* of the system without
> re-deriving it from scratch. It is deep engineering documentation, not a README and not
> product/marketing material.
>
> **Status of this document.** Compiled **2026-05-28**. Architectural and rollout claims
> were verified directly against source (`server.go`, `config.go`, `websocket/message.go`,
> the migration files, the WS client, and the R1/rollout status docs). Where a specific
> value matters (config defaults, enum members, route strings, worker intervals) it was
> read from the owning file rather than summarized. A handful of fine-grained method-level
> claims (exact repository/handler method inventories) are derived from dependency wiring
> in `server.go` plus exploration synthesis; treat those as "very likely" rather than
> "byte-verified," and confirm against the file before relying on them for a change.
>
> **Update 2026-05-29.** Remediation + R3 finalization landed since the 2026-05-28 compile:
> all four Category-1 e2e bugs are now fixed — T-01, X-03, O-05 in prior commits
> (`6db078e`, `8b5c386`) and **PT-03** this session (`45c93d5`: MFA-unconfigured now returns
> 503 `MFA_NOT_CONFIGURED`, not 500). The R3 policy-decisions writing blocker is **CLOSED**
> (`r3-policy-decisions-v1.md`, commits `8e193e6`/`075bbf8`). The R1 soak was interrupted by
> a ~34h app outage and **restarted with the 72h clock reset to 2026-05-28T19:29:03Z**
> (CP-4 in `r1-live-rollout-status.md`). §§2.5, 2.6, 9.3, 11, 15 updated accordingly.
>
> **Update 2026-05-29 (Platform Governance layer + Control-Plane UI + Theme adoption).**
> A new **SaaS platform control-plane** layer was built on branch
> `platform-governance-entitlements` (~24 atomic commits, **not merged to `main`**). It is
> **additive and rollout-safe**: 3 new migrations (000029 entitlements, 000030 feature flags,
> 000031 tenant theme), new platform services/handlers/routes, an operator frontend, and a
> theme-adoption integration on the guest UI. **No change** to the 9 strict-rollout flags,
> session/payment lifecycle, websocket, guest/staff auth, `internal/authz`, or workers; the R1
> soak is untouched. Entitlements and feature flags are **resolve-only/shadow** (computed +
> metered, not yet enforced on operational paths); org/branch suspend/activate flip a status
> column that nothing reads yet. **Migration count is now 31 (000001–000031).** New
> section **§16** documents this layer; §§3.4, 4, 5, 6, 11 note it inline. This work is
> branch-local — when reading `main`, the platform layer is just the pre-existing
> auth/org/branch/audit primitives (§3.4 item 3).
>
> **Update 2026-05-30 (Support Console — read-only operator observability).**
> A first **Support Console** was added to the platform layer (same branch,
> `platform-governance-entitlements`, 8 atomic commits). It lets platform operators diagnose a
> tenant — search → inspect session/order/payment/audit + tenant health — **without touching the
> DB**. **Observability, not control:** strictly additive read-only GETs + UI, **no mutations**,
> **no migration**, no flag/lifecycle/payment/websocket/auth/`authz` change; R1 soak untouched.
> Backend: a read-only `services.SupportService` assembling **sanitized** session/order/payment
> aggregates from existing repo reads (omits `session_token`/`device_fingerprint`; never uses the
> guest-token snapshot, never mutates), new `/platform/{sessions,orders,payments}/:id`, search
> extended to tables+participants, `session_id` audit filter. **Dual audit** on reads (internal
> `platform_audit_log` always; tenant-visible `audit_log` RiskCritical on session+payment detail).
> RBAC: `support_admin`/`read_only_auditor` (super bypass); `billing_admin` excluded. Documented
> in **§16.11**; migration count unchanged (still 31).
>
> **Update 2026-05-30 (Pilot-readiness remediation — the 3 dress-rehearsal findings closed).**
> The Pilot Dress Rehearsal (`pilot-dress-rehearsal-report.md`) gave a qualified GO and surfaced
> three issues; all are now **fixed and revalidated** on branch `pilot-readiness-remediation`
> (off `platform-governance-entitlements`, **3 atomic commits, unmerged**) — minimal change surface,
> R1 soak untouched. **F-8 (P0):** new `handlers.guestSafeSession()` clears the dormant
> `session_token` from **every** guest session response (snapshot + Create/Get/Join/Reactivate),
> permanent and **independent of the R6 flag** — the guest snapshot no longer leaks the credential
> (§4.3, §7.3). **F-6 (P1):** the guest `ORDER_{CONFIRMED,PREPARING,READY,SERVED,CANCELLED}`
> handlers now unwrap `(payload.order ?? payload)` to match the backend's `{order:{…}}` envelope
> (`order.go:517`), so the live order tracker advances without a refresh (§5.7, §7.4). **F-1 (P1):**
> `connection.ts openSocket()` now routes a failed ws-ticket through `scheduleReconnect()→reconnect()`
> (snapshot reactivates an `awaiting_reactivation` session server-side, then retries) instead of
> dead-ending on "Connection lost" (§5.7). No websocket/payment/session/auth/`authz`/migration change.
> Validated on the isolated `pilot-validation` stack (`:8090`/`:8091`); snapshot security matrix
> (no/valid/invalid/cross-session/cross-tenant; R6 off **and** on), live order tracker, idle-reload
> reactivation, and cross-instance propagation all pass. **Verdict: ready for Pilot Restaurant #1.**
>
> **Update 2026-05-30 (Premium QR Collateral — theme-aware print collateral).**
> A **Premium QR Collateral** system was added on branch **`premium-qr-collateral`** (off
> `pilot-readiness-remediation`, **9 atomic commits, unmerged**). It enhances the existing QR
> package generation into **theme-aware hospitality print collateral** — five premium print formats
> (`standing_card`, `table_tent`, `sticker`, `square_card`, `bulk_sheet`) that render differently per
> theme, plus optional content (welcome/subtitle/footer/tagline/WiFi/socials/branch/logo). **Additive
> + branch-local; does NOT touch payments, session, websocket, auth, realtime, analytics, or the R1
> soak.** Architectural rule honored: **no second branding system** — collateral *consumes*
> `Theme (preset+tokens) + Branch metadata + Collateral config`; the theme stays the source of truth
> and collateral owns only the *physical* concern (layout/text/logo/WiFi/exports). One new additive
> migration **000033_branch_collateral** (branch-scoped JSONB config, mirroring `tenant_themes`;
> 000032 was the separately-added subscription-billing schema) → **migration count now 33**. New
> read/write endpoints on **both** the platform (`/platform/branches/:id/{collateral,tables}`) and
> staff (`/branches/:id/collateral`) trust domains writing **one shared `branch_collateral` row**; a
> platform "Collateral" studio page; a staff-admin "Collateral" tab; client-side exports (themed
> Print→PDF, plus a QR PNG/SVG + standalone `print.html` ZIP via the existing `jszip`/`qrcode.react`,
> **no new deps**). New section **§16.12** documents it. Validated on the isolated stack (new app port
> `:8095`, isolated PG/Redis — never the soak); screenshots in `screenshots/collateral/`.
>
> **Update 2026-06-04 (R1 soak CLOSED — PASS WITH OBSERVATIONS).** A forensic closure audit
> of the R1 soak was completed after the host was shut down unexpectedly mid-soak (report:
> `r1-soak-closure-report.md`). Determination: **PASS WITH OBSERVATIONS.** The certified
> post-CP-4 run (`qr-app-chaos`, **2026-05-28T19:29:03Z → 2026-06-02T23:27:22Z**) ran
> **~124h continuously** (RestartCount 0, OOMKilled false), with **0 panics / 0 errors / 0
> `audit_write_failures`**, ending in a **graceful** shutdown — the 72h target was met with
> ~52h margin. Postgres shut down clean and restarted with **no recovery** (no corruption);
> Redis showed no eviction/OOM. The container's current `Exited (127)` is a **failed
> post-reboot restart** (the `/tmp/qrapp` binary was wiped when `/tmp` cleared on boot — the
> CP-4 deployment-fragility risk recurring), **not** a soak crash. Observations/UNKNOWNs: the
> final ~75h were **idle** (no traffic/health probes after 2026-05-30T19:52Z, though workers
> ticked every minute); **sustained-load/WebSocket longevity** and the **audit storage curve
> at real volume** remain unverified; relocate the binary off `/tmp`, add an app-down alert,
> and purge the ~20–28 stale `payment_pending` test sessions before any *production* soak.
> **Do not auto-chain R2.** §§2.5, 2.6 updated accordingly.
>
> **Update 2026-06-05 (Manual-testing certification pass — three guest-flow fixes/features committed).**
> A human manual-testing pass on the isolated 3-tenant stack (Saffron House / Copper Pot Kitchen /
> Urban Brew Café; `manual-testing-findings-v4.md`) surfaced a set of frontend↔backend contract gaps.
> Three were addressed and **committed to branch `premium-qr-collateral`** as four atomic commits
> (the first being the seed rework); all are additive, guest-scoped, and do not touch payments,
> websocket transport, auth, rollout flags, or the soak:
> 1. **Occupied-table check-in fix (`c84834b`).** `GetActiveSessionForTable` only matched
>    `status='active'`, but the one-active-per-table unique index also blocks `payment_pending` and
>    `awaiting_reactivation`; a QR scan of such a table resolved no joinable session, fell through to
>    create, and hit `SESSION_ALREADY_ACTIVE` ("something went wrong"). The lookup now matches the
>    same non-terminal statuses (newest first), and `JoinSession` resumes an `awaiting_reactivation`
>    session (reactivate + `SESSION_REACTIVATED`) and permits joining a `payment_pending` one;
>    terminal sessions are still rejected. Affected **every idled table**, so this is a real
>    operational fix (§4.10, §5.2). sqlc query `GetActiveSessionForTable` widened.
> 2. **Guest host transfer (`97d0b89`).** New `POST /sessions/:id/host` lets the current host hand
>    the role to another active participant; service `TransferHost` reuses the existing `reassignHost`
>    (persist + `HOST_CHANGED` broadcast), host-only, refused while `payment_pending`
>    (`HOST_TRANSFER_LOCKED`). Frontend adds a "Make host" control; the existing `HOST_CHANGED`
>    handler already applies it. Previously host reassignment was **automatic only** (presence-based);
>    this adds the manual path (§4.10, §5.9).
> 3. **Promo per-guest limit + phone gate (`9852ff5`).** The `uses_per_phone` cap already existed but
>    was unenforceable (validate passed no phone, the admin form hardcoded 1). Now: the form exposes
>    "Uses per guest" (explicit 0 = unlimited; omitted still defaults to 1); `/promos/validate`
>    accepts `order_total` + `phone_e164` (so the discount/min-order are correct and the per-phone cap
>    is checked at apply time); a per-phone-limited promo with no phone returns `PROMO_PHONE_REQUIRED`;
>    the guest cart reveals a phone field and carries the number into placement so the redemption
>    records against it (§4.9, §5.10).
> **Still open** from the same findings pass (not yet addressed): promo entry should move to the bill
> (finding #5), the promo daily time-window is compared against the DB's UTC `LOCALTIME` instead of
> the branch timezone (finding #10 — windowed promos silently fail), the customer "Come back anytime"
> opt-in is feature-gated off (#6), the staff admin needs a tenant context / branch-derived theme
> (#8, #12), the manager staff-add UI over-permits vs the owner-only backend (#7), and several UX
> items (modifier single-select #1, menu image side #2, recurring expiry banner #3, landing tiles #4,
> promo form date/time clarity #9).
>
> **This document is intentionally uncommitted.** It documents a snapshot of an
> in-progress rollout; do not treat it as a contract.

---

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
|---|---|---|
| **Session lifecycle / states** | `internal/services/session.go`, `migrations/000023/000024/000026`, spec `session-lifecycle-state-machine.md` | `store/session.ts`, `providers/SessionProvider.tsx`, `lib/ws/connection.ts` |
| **Payments / settlement / bill** | `internal/services/payment.go`, `internal/handlers/payment.go`, `internal/handlers/billing.go`, `migrations/000021` | `app/(guest)/session/[id]/payment/page.tsx`, `app/(staff)/staff/(dashboard)/waiter/page.tsx`, `lib/api/payments.ts`, `components/shared/BillBreakdown.tsx` |
| **Webhooks** | `internal/handlers/payment.go` (`Webhook`), `internal/services/payment.go`, payment-webhook-events table | — |
| **Waiter flow** | `internal/handlers/{order,payment,assistance}.go` (`*ForBranch`, `Settle`, `Acknowledge/Resolve`) | `app/(staff)/staff/(dashboard)/waiter/page.tsx` |
| **Kitchen rendering / order status** | `internal/handlers/order.go` (`UpdateStatus`, `ListActiveForBranch`), `internal/domain/statemachine*.go` | `app/(staff)/staff/(dashboard)/kitchen/page.tsx`, `store/orders.ts`, `hooks/useOrders.ts` |
| **WebSocket events (add/change)** | `internal/websocket/message.go`, `internal/events/*`, the publishing service | `types/ws.ts`, `hooks/useWebSocket.ts`, `lib/ws/connection.ts` |
| **Shared cart** | `internal/services/cart.go`, `internal/repository/cart.go`, `migrations/000027` | `store/cart.ts`, `hooks/useCart.ts`, `lib/api/cart.ts`, `app/(guest)/session/[id]/cart/page.tsx` |
| **Host permissions / reassignment** | `internal/services/session.go` (`AuthorizeHostAction`, host baseline), `orderSvc/paymentSvc.SetHostAuthority` in `server.go` | `hooks/useSession.ts`, `store/session.ts` (`applyHostChanged`) |
| **Reconnect / snapshot** | `internal/handlers/snapshot.go`, `internal/services/session.go` (`GetSnapshot`) | `lib/ws/connection.ts`, `lib/ws/reconciliation.ts` |
| **Menu availability** | `internal/handlers/menu_admin.go` (`ToggleAvailability`), `internal/services/menu.go` | `store/menu.ts` (`setItemAvailability`), `hooks/useFilteredMenu.ts` |
| **Admin: staff / tables / menu / promos** | `internal/handlers/{staff,table,menu_admin,promo}.go` | `app/(staff)/staff/(dashboard)/admin/page.tsx`, `components/admin/*`, `lib/api/{staff,tables,promos}.ts` |
| **Auth (guest/staff/platform)** | `internal/auth/guest.go`, `internal/middleware/{staff_auth,platform_auth}.go`, `internal/services/{staff,platform}.go` | `store/staff.ts`, `app/(staff)/staff/login/page.tsx`, `lib/api/client.ts` |
| **Authorization policy (RBAC)** | `internal/authz/*` | — |
| **Rollout flags** | `internal/config/config.go` + the gate site for each flag | (none; behavior is server-side) |
| **Routes (add an endpoint)** | `internal/server/server.go` | `lib/api/*` |
| **Workers / lifecycle jobs** | `internal/worker/worker.go`, `cmd/server/main.go` (startup), `internal/repository/worker.go` | — |
| **Migrations / schema** | `backend/migrations/`, run via `internal/db/migrations.go` or `cmd/migrate` | — |
| **Metrics / alerts** | `internal/observability/metrics.go`, `deploy/observability/prometheus-alerts.yml` | `store/ws.ts` (status), `components/shared/RealtimeIndicator.tsx` |
| **Tenant resolution** | `internal/middleware/tenant.go`, `internal/handlers/tenant.go` | `providers/TenantProvider.tsx`, `middleware.ts` |

---

## 14. Current Deployment + Operational State

### 14.1 Local / Docker
`docker-compose.yml`: `postgres:17-alpine` + `redis:7-alpine` (appendonly, 256mb LRU) +
`app` (built from `backend/docker/Dockerfile`, multi-stage, **linux/arm64**, non-root,
`/health` healthcheck). Networks: internal `backend` + external `proxy` (for nginx).
Migrations auto-run at app startup.

### 14.2 Reverse proxy
`deploy/nginx/qr-dining.conf` (authored in Phase D): WS-aware (`Upgrade`/`Connection`,
`proxy_http_version 1.1`, `proxy_buffering off`, `proxy_read_timeout 3600s` > the app's
~54s ping); `limit_req` on the HTTP API but **not** on `/ws`; `/metrics` restricted to
private CIDRs. Cloudflare idle WS timeout (~100s) is covered by the app ping — do not lower
`proxy_read_timeout` below the ping interval.

### 14.3 Startup & commands (`Makefile`)
`make build|run|test`, `make migrate-up|migrate-down`, `make sqlc-generate`,
`make docker-up|docker-down|docker-logs`, `make seed`. Frontend: standard Next.js
(`npm run dev` / `build`) with `NEXT_PUBLIC_API_URL` + `NEXT_PUBLIC_WS_URL` (defaults
`http://localhost:8080` / `ws://localhost:8080`) and `NEXT_PUBLIC_TENANT_SLUG` for local
tenancy.

### 14.4 Seed & env assumptions
`scripts/seed.go` inserts a test restaurant, branches, tables (with QR tokens), menu, and
staff (owner/manager/waiter/kitchen). Required env: `DATABASE_URL`, `REDIS_URL`,
`GUEST_TOKEN_SECRET` (dev default warns in release), `MFA_ENCRYPTION_KEY` (if any platform
user requires MFA), `CORS_ALLOWED_ORIGINS` (frontend origin). Optional Cloudflare R2 image
storage (`R2_*`).

### 14.5 Backup / restore
`backend/scripts/{backup,restore,backup_r2}.sh` (pg_dump custom format, single-transaction
restore, R2 upload, retention pruning). Runbook: `docs/backup-restore.md`.

### 14.6 Soak / DB safety (IMPORTANT)
- The **R1 soak runs on the local staging stack** (`qr-dining-postgres-1` /
  `qr-dining-redis-1`, app container historically `qr-app-chaos`) with
  `AUDIT_LOG_V2_ENABLED=true`. **Do not touch the soak DB** — do not reset it, run
  destructive migrations against it, mutate `audit_log`, or flip the flag. Restarts must
  preserve `AUDIT_LOG_V2_ENABLED=true`.
- For any testing, use a **separate throwaway database** (`TEST_DATABASE_URL`) — integration
  tests run embedded migrations against an empty DB by design.
- "Optional soak hygiene" (resolving the ~20 stale `payment_pending` test sessions) is
  soak-safe because it touches neither `audit_log` nor the flag — but treat it as optional,
  not required.

---

## 15. Final Engineering Assessment

**How mature is it?** Substantially mature. The hard architecture is settled and proven:
clean layering, strict state machines, idempotency, host-authoritative ordering, an
immutable audit trail, reversible feature-flag rollout, and Postgres-authoritative recovery.
The realtime layer is correct and self-healing via snapshot reconciliation. R1 is live and
healthy in soak.

**What feels stable.** Session/order/payment/assistance state machines; the shared-cart and
host-control model; reconnect/reconciliation; the rollout machinery (every flag reversible,
fully instrumented with page-backed alerts); the DB schema and its financial-snapshot
discipline; backup/restore and reverse-proxy topology.

**What still feels risky.**
1. The **R1 storage curve** — the one quiet long-tail unknown; must be re-derived at volume.
2. R1 has only been exercised on **single-instance local staging**, and the soak proved
   fragile in practice (a `/tmp` cleanup wiped the binary → ~34h silent outage → clock
   reset, CP-4); the multi-pod production soak with an app-down alert is still ahead.
3. **R3 is now unblocked** (policy decisions written) but still needs its 48h shadow soak
   before the paired strict flip.
   *(Resolved since 2026-05-28: the cross-org snapshot leak T-01 and the X-03/O-05/PT-03
   500-status bugs — see §9.3 and §11.)*

**Likely needed before pilot.** Finish the R1 production soak (confirm the storage
projection) and harden its deployment so an outage can't silently reset it; run the R3
shadow soak and flip the paired flags; backfill `branches.organization_id` and run the R2
shadow/soak; decide and (if needed) close the order-audit coverage gap. (T-01, the
500-status bugs, and the R3 policy-decision writing task are now done.)

**Likely needed after pilot.** Drive the remaining waves (R4–R7) with their operational
prerequisites (staff data/training, per-IP cap sizing, legacy-credential decay windows,
settlement UI on all devices); add the audit hash-chain; revisit Hub sharding only if scale
demands it.

---

## 16. Platform Governance Layer (branch `platform-governance-entitlements`)

> **Scope note.** Everything in this section lives **only on branch
> `platform-governance-entitlements`** (not merged to `main` as of 2026-05-29). It is the
> SaaS *control plane* — tenant governance, operational intelligence, and branding — built
> as a **separate trust domain** on top of the existing Platform auth primitives (§3.4 item 3).
> All of it is **additive**: additive migrations (000029–000031), additive routes/services,
> and **resolve-only/shadow** semantics — nothing here enforces on, or alters, the operational
> guest/staff domains, the 9 strict-rollout flags, or the R1 soak.

### 16.1 Bounded context & auth
All platform governance APIs sit under the existing `/platform/*` group
(`middleware.PlatformAuth`; staff/guest tokens rejected) and reuse the platform RBAC helpers
in `internal/handlers/platform.go`: `requirePlatformRole`/`requireAnyPlatformRole`
(super_admin bypasses; roles `super_admin` > `support_admin` > `billing_admin` >
`read_only_auditor`) and `logPlatformAudit` (writes `platform_audit_log` with
`platform.<resource>.<verb>` actions). Reads allow support/billing/auditor; mutations are
super_admin (billing for plan-assign/update).

### 16.2 Entitlement system (C) — migration 000029, **resolve-only/shadow**
Tables: `entitlements` (capability/limit catalog, seeded), `plan_entitlements`,
`organization_plan_assignments`, `organization_entitlement_overrides`.
`internal/services/entitlement.go` `EntitlementService.ResolveForOrganization` precedence:
**org override > org plan assignment → that plan's `plan_entitlements` > restaurant-subscription
bridge (`features_json`) > free default**. `HasCapability`/`Limit` emit the shadow metric
`entitlement_evaluations_total{capability,result}`; **no route consumes the boolean to block
anything yet.** Repo: `internal/repository/entitlement.go`. APIs: catalog + plan CRUD
(`/platform/plans*`, `/platform/entitlements`), org resolved-entitlements + plan-assign +
per-key override (`/platform/organizations/:org_id/{entitlements,plan,entitlements/:key}`).
Capability keys include `analytics.basic/advanced`, `custom.theme`, `multi_branch`,
`advanced.audit`, `support.priority`, `api.access`; limits `limit.branches/staff/tables`
(`-1` = unlimited).

### 16.3 Org/branch lifecycle (B)
`internal/handlers/platform_lifecycle.go` + repo `UpdateOrganizationStatus`/`UpdateBranchStatus`:
`POST /platform/organizations/:org_id/{suspend,activate}` and
`POST /platform/branches/:branch_id/{suspend,activate}` flip the existing
`active|suspended|archived` status column (+ audit). **No operational path reads these
statuses yet** — inert until a future enforcement phase.

### 16.4 Feature-flag targeting (D) — migration 000030, **resolve-only**
A NEW product-flag system **completely separate** from the 9 env `config.FeatureFlags`
(those remain global bootstrap booleans). Tables: `platform_feature_flags` (catalog) +
`platform_flag_global_overrides` / `platform_flag_organization_overrides` /
`platform_flag_branch_overrides`. `internal/services/flag.go` precedence **branch > org >
global > catalog default** (pure `resolveFlag`; metric `feature_flag_resolutions_total{scope}`).
APIs: catalog create/update + set/clear overrides at each scope + resolved reads
(`/platform/flags*`, `/platform/{organizations,branches}/:id/flags*`) and a public, Redis-cached
`GET /branches/:id/feature-flags`. Boolean-only (no % rollout/cohorts). No gate consumes flags yet.

### 16.5 Platform analytics (E) — no migration, on-demand
`internal/services/platform_analytics.go` + `handlers/platform_analytics.go` +
`sql/queries/platform_analytics.sql`: cross-tenant, **on-demand Postgres aggregation**
(Redis-cached 5 min, UTC bucketing, optional `?organization_id=` filter, `period=daily|weekly|monthly`).
`GET /platform/analytics/{usage,revenue,health}` — Usage (sessions/orders/payments/participant-joins
per day, active branches, active diners), Revenue (GMV + per-day/branch/org from completed payments),
Health (authz denials from `audit_log`, webhook failures from `payment_webhook_events`).
**Intentionally Postgres-only:** QR scans, websocket reconnects, worker failures, and audit-write
failures are **not** here — they remain Prometheus counters on `/metrics` + alert rules.

### 16.6 Theme/branding (G) — migration 000031, **adopted on the guest UI**
Tables: `theme_presets` (4 seeded: `dark-luxury`, `modern-minimal`, `warm-cafe`, `vibrant`) +
`tenant_themes` (restaurant-scoped `{preset, tokens_json}`). `internal/services/theme.go`:
allowlisted **hex-only** design-token keys (14, mapping 1:1 to the `--<key>` CSS vars in
`frontend/styles/themes.css`), **custom tokens gated by the org's `custom.theme` entitlement**,
no arbitrary CSS. Platform APIs `GET /platform/theme/presets`,
`GET|PUT /platform/organizations/:org_id/theme`; public `GET /branches/:id/theme`.
**Legacy bridge (back-compat):** when no `tenant_themes` row exists, `GetThemeForRestaurant`
falls back to the restaurant's legacy `settings_json.theme` (validated against `theme_presets`),
else `dark-luxury` — so legacy-only restaurants never lose theming. `GET /tenants/by-slug/:slug`
now returns an **additive** `theme:{preset,tokens}` field (ThemeService injected into
`TenantHandler`); `settings` unchanged.

### 16.7 Platform Control-Plane UI (frontend)
A new route group `frontend/app/(platform)/platform/` — its **own trust domain**, separate
from guest/staff — with `login/` (password + MFA second step) and a guarded `(dashboard)/`
shell (sidebar: Overview, Organizations, Plans, Entitlements, Analytics, Feature Flags, Themes).
New: `store/platform.ts` (Zustand + sessionStorage key `platform-auth`), `lib/api/platform.ts`,
`lib/platform-rbac.ts` (`hasPlatformRole`, super_admin bypass), `components/platform/*`. The
shared `lib/api/client.ts` gained an additive `platformToken` option + a `put` verb. Reuses
`components/ui/*`, recharts, sonner, lucide — no new deps. `middleware/cors.go` Allow-Methods
gained **PUT** (needed for the override endpoints; additive).

### 16.8 Theme adoption on the live guest frontend
The structured theme is now the **authoritative source of guest branding**, replacing the
legacy `settings_json.theme` string. `frontend/lib/theme/applyTheme.ts` applies the preset
(`data-theme`) + allowlisted hex token vars **directly on `<html>` (no localStorage write →
no cross-tenant leakage)**; `lib/api/theme.ts` calls the public branch endpoint. Resolution:
once per tenant context at boot (`TenantProvider` carries `theme`, applied by `Providers`
`TenantThemeSync`) and once per branch context (QR-resolve + `SessionProvider`, guarded per
`branch_id`). Fallback chain: structured `tenant_themes` → legacy `settings.theme` → default.

### 16.9 Known platform-layer gaps / follow-ups
- Entitlements + feature flags are **resolve-only**; no enforcement gate wired in yet.
- Org/branch suspend/activate is inert (no operational consumer of the status).
- No backend **DELETE** for an org entitlement override (only PUT upsert).
- Plan creation is limited to the 3 fixed `plan_tier` enum values (`free|standard|premium`).
- MFA **enroll/recovery-code** management UI not built (login MFA *verify* step is).
- Staff admin `AppearanceTab` still **writes** legacy `settings_json.theme` (2 presets) — the
  read path is now bridged/consistent, but unifying the staff write path is deferred.
- Whole layer is **branch-local and unmerged**; if/when merged, decide enforcement rollout
  (likely behind new flags, mirroring the staged R-wave discipline).

### 16.10 Verification posture
Backend: unit + integration tests against a **throwaway** `TEST_DATABASE_URL` (entitlement
resolution, flag precedence, analytics correctness + org filter, theme legacy bridge, lifecycle
round-trips). Frontend: `tsc`/lint/`next build` clean + Playwright smoke (auth guard, org
suspend/activate, flag precedence, theme entitlement gate, theme preset/custom/fallback) with
screenshots in `screenshots/{platform,theme}/`. All live verification used isolated throwaway
Postgres/Redis (distinct names/ports) with the app run from `/tmp` to avoid the soak-pointing
`backend/.env`; **soak containers were never touched.**

### 16.11 Support Console — read-only operator observability (no migration)
A first operator **Support Console** for diagnosing a tenant without DB access. **Observability,
not control:** additive read-only GETs + UI only — **no mutations, no migration**, nothing that
settles/closes/cancels/edits, no `authz`/lifecycle/payment/websocket/flag change.

- **Read service** `internal/services/support.go` (`SupportService`): assembles per-entity
  aggregates from existing repo reads only — `GetSessionDetail` (session + table + host +
  participants + orders(+items) + payments + assistance + event_log **lifecycle timeline**),
  `GetOrderDetail`, `GetPaymentDetail` (+ bill snapshot + **webhook history**). Returns
  **sanitized DTOs**: money stringified; the guest credential `sessions.session_token` and
  `session_participants.device_fingerprint` are deliberately **omitted** (a JSON-leak unit guard
  asserts this). It never calls the guest-token snapshot (§4.10) and never mutates.
- **APIs** `internal/handlers/platform_support.go` on the `/platform` group:
  `GET /platform/sessions/:id`, `/orders/:id`, `/payments/:id`. Reuses the existing
  `SearchSupport` (`/platform/support/search`) — now extended in `repository/support_search.go`
  with `searchTables` + `searchParticipants` (8 categories total). New read
  `ListWebhookEventsByPayment`; `/platform/audit` gained an optional `session_id` filter.
- **RBAC:** reads gated to `support_admin` | `read_only_auditor` (super_admin bypass) via
  `requireAnyPlatformRole`; **`billing_admin` is excluded** from support (keeps analytics-only);
  the audit explorer requires `read_only_auditor`. (Verified: billing_admin → 403 on support
  reads, 200 on analytics.)
- **Dual audit (preserves the transparency invariant):** every support read writes an internal
  `platform_audit_log` row (`platform.support.{search,session.read,order.read,payment.read}`);
  session-detail and payment-detail **additionally** emit a tenant-visible `audit_log` row
  (`ActionPlatformSupportAccess`, `RiskCritical`, `ActorTypePlatformUser`) so orgs see when
  platform viewed their data. (Tenant-visible rows persist only when `AUDIT_LOG_V2_ENABLED=true`.)
- **Frontend** `app/(platform)/platform/(dashboard)/support/`: search + categorized results +
  per-tenant **Health** panel (reuses analytics usage/health), `sessions/[id]`, `orders/[id]`,
  `payments/[id]`, and an `audit` explorer; "Support" nav entry. **Zero mutation affordances**
  (DOM-verified). Reuses the platform shell/ui/api-client; no new deps.
- **Verified:** backend integration test (aggregate shapes, webhook list, table/participant
  search, credential-leak guard) + live Playwright smoke (search → session/payment detail →
  audit explorer showing the dual-audit rows → RBAC 403 for billing_admin → no-mutation DOM
  scan), screenshots in `screenshots/support/`. Throwaway infra only; soak untouched.
- **Gaps/deferred:** customer cross-session history; requiring an active support-session *grant*
  for reads (future hardening — this phase is RBAC + dual-audit); order state-transition history
  beyond the event_log timeline; CSV/bulk export (intentionally omitted).

### 16.12 Premium QR Collateral (branch `premium-qr-collateral`, migration 000033)

> **Scope note.** Lives on its **own branch `premium-qr-collateral`** (off
> `pilot-readiness-remediation`, unmerged). It **enhances** the existing QR package generation into
> theme-aware hospitality collateral. **Additive + branch-local**: one new migration, additive
> routes/services/UI, **no change** to payments, session, websocket, auth, `authz`, realtime,
> analytics, the 9 strict-rollout flags, or the R1 soak.

**Architectural principle — no second branding system.** The renderer composes
`Theme (preset+tokens) + Branch metadata + Collateral config`. The **theme stays the source of
truth** for colour/typography (§16.6); collateral owns only the *physical* concern: chosen print
format, content text, logo placement, WiFi, socials, and exports. This keeps the two concerns
separate and reuses the existing theme system wholesale.

**Config model (structured, validated — no arbitrary HTML/CSS, no drag-and-drop).** A branch-scoped
`CollateralConfig` (format + 4 toggles `showLogo/showBranch/showWifi/showFooter` + 9 text fields
welcome/subtitle/footer/branchDisplay/tagline/wifi name+password/instagram/website). Stored as JSONB
in **migration 000033 `branch_collateral`** (`branch_id` PK → `branches`, `config_json`,
`updated_by_platform_user_id` nullable for staff writes) — mirrors `tenant_themes`. The number 000032
was taken by the separately-added subscription-billing schema.

- **Service** `internal/services/collateral.go` (`CollateralService`): the single validation/storage
  authority shared by both trust domains — strict JSON decode (`DisallowUnknownFields` → unknown keys
  rejected), format allowlist (`standing_card|table_tent|sticker|square_card|bulk_sheet`), per-field
  length caps, trim-normalize, sensible defaults when no row exists. Pure validators are unit-tested
  (`collateral_test.go`, 7 tests). Repo `internal/repository/collateral.go` (get + upsert).
- **APIs.** Platform (`internal/handlers/platform_collateral.go`): `GET|PUT
  /platform/branches/:branch_id/collateral` (read roles for GET; super_admin for PUT; platform-audited)
  + `GET /platform/branches/:branch_id/tables` (new read-only table list for generation). Staff
  (`internal/handlers/branches.go`): `GET|PUT /branches/:id/collateral` (owner/manager, branch-guarded).
  **Both write the same `branch_collateral` row** — one store, two trust domains (not the legacy
  two-write-path problem). `logo_url` + `restaurant_name` are now surfaced (additive) on the platform
  `GetBranch` and staff `GetBranch` responses so renderers can show the logo. Routes in `server.go`.
- **Frontend (shared, both surfaces).** `frontend/components/collateral/`: five format renderers
  (`formats/*`), `CollateralThemeScope` (applies `data-theme` + inline custom-token vars to a
  **subtree** so a preview can show a theme different from the operator's own console theme — reuses
  the `[data-theme="X"]` attribute selectors in `styles/themes.css`), `FormatRenderer` (dispatch),
  `CollateralConfigForm` (format-aware: hides blocks a format doesn't support), `CollateralStudio`
  (**controlled** component — parent owns config so it resets per branch), `CollateralPrintContainer`
  (in-document `@media print` pages, same technique as `components/admin/PrintTemplate.tsx`).
  `lib/collateral/` holds the format registry, the export (`jszip`) and `print.html` builder.
- **Surfaces.** Platform: `app/(platform)/platform/(dashboard)/collateral/page.tsx` (org→branch
  picker; loads theme via `getOrganizationTheme`, branding via `getBranchDetail`, tables via
  `listBranchTables`, config via `getBranchCollateral`) + a "Collateral" nav entry in `PlatformNav`.
  Staff: a new "Collateral" tab in `app/(staff)/staff/(dashboard)/admin/page.tsx` (loads theme via
  `themeApi.resolveForBranch`, tables via `tablesApi.list`, config via the staff collateral API). The
  existing single-card `PrintTemplate` (Tables tab) is left untouched.
- **Exports (no PDF infra, no new deps).** Print View → themed in-document pages → browser
  Save-as-PDF; Asset ZIP (`collateral-{branch}.zip`) = per-table QR **PNG** (`QRCodeCanvas`) + **SVG**
  (`XMLSerializer` on `QRCodeSVG`) + a standalone **`print.html`** with concrete (resolved-from-theme)
  colours for print vendors + `config.json`.
- **Verified** on the isolated stack: backend rebuilt to a **fresh port `:8095`** against the isolated
  `pilot-validation` Postgres/Redis (a pre-existing `:8090` pilot-app was left untouched). Migration
  applied; API CRUD + validation (bad format / unknown key / oversized → 400; no-token → 401) +
  cross-branch isolation (403) + **shared store across trust domains**; all 5 formats × 3 themes (incl.
  the custom modern-minimal); ZIP + print.html validated; **regression** (existing Tables QR cards /
  Print / Regen QR) intact. `go build/vet`, 7 unit tests, `tsc`, `next build` clean. Screenshots in
  `screenshots/collateral/`.
- **Notable fix.** The `table_tent` initially folded **upside-down** (table numbers met at the fold,
  brands at the outer edges → both faces inverted when folded). Corrected so the **heads meet at the
  ridge** (top panel rotated 180°, bottom normal) → both faces read upright once folded.
- **Gaps/deferred:** full styled-card raster PNG/SVG (would need an `html-to-image` dep — declined;
  styled output is via Print→PDF); per-table custom labels ("Window Seat") need a table-level field
  (v1 uses `identifier`); the exported `print.html` renders a simplified flat card grid, not the true
  two-panel tent geometry; multi-language; saved collateral "profiles". Branch-local + unmerged.

---

*End of master-system-context-v1.md. This document is intentionally uncommitted and reflects
the system state as of 2026-05-30. The platform-governance layer (incl. the Support Console) is
branch-local on `platform-governance-entitlements`; the Premium QR Collateral system (§16.12) is
branch-local on `premium-qr-collateral`.*
