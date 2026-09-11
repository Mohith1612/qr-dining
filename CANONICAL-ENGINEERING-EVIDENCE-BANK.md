# QR Dining forensic engineering evidence bank

## Purpose

This is a research record for preparing for Canonical's written engineering assessment. It is source material, not a set of polished application answers. Findings distinguish direct repository evidence from inference and unknown personal context.

## 1. Evidence rules and authorship boundary

### Directly established

- **FACT:** The repository contains 504 commits across all local refs.
- **FACT:** 471 commits are authored as `Mohith1612 <mohith.dev16@gmail.com>`.
- **FACT:** 33 commits are authored as `T3 Code <t3code@users.noreply.github.com>`.
- **FACT:** The Git remote is `github.com/Mohith1612/qr-dining`.
- **FACT:** The working tree was clean during this investigation.
- **FACT:** Mohith-authored history spans backend, frontend, database, E2E, infrastructure, release certification, observability, marketing, and operations.
- **FACT:** Locally available branches include platform governance, pilot remediation, analytics/loyalty, collateral, UI redesign, and observability tracks.
- **FACT:** No local PR metadata, review comments, issue tracker, or stakeholder correspondence is available.

### Attribution rule

`Mohith1612` commits are treated as evidence of work under the user's Git identity. That establishes committed authorship, but not necessarily whether every line was written unaided.

T3 checkpoint commits are not attributed as personal engineering accomplishments without corroborating Mohith-authored commits.

### Important unknowns

- **UNKNOWN — PERSONAL CONTEXT NEEDED:** Confirm that `Mohith1612 <mohith.dev16@gmail.com>` is the user's identity.
- **UNKNOWN:** Whether this was solo work, employment work, coursework, a startup, or a personal product.
- **UNKNOWN:** Which changes originated from independent reasoning versus tickets, external audits, AI assistance, or stakeholder instructions.
- **UNKNOWN:** Whether any branches corresponded to reviewed PRs.
- **UNKNOWN:** Whether the system ever had real restaurant users. The repository says it had never served a paying customer as of 18 July 2026.
- **UNKNOWN:** Production or business impact beyond test, pilot, beta, and certification evidence.

---

# 2. Product reconstruction

## What QR Dining is

**FACT:** QR Dining is a session-centric, realtime dine-in hospitality system, not a delivery application.

The central aggregate is a physical table session:

1. A guest scans a table QR code.
2. The backend resolves the table, branch, restaurant, and tenant context.
3. The guest creates or joins the table's non-terminal session.
4. Multiple participants share one collaborative cart.
5. Exactly one participant is the host.
6. The host submits orders and initiates payment.
7. Kitchen and floor staff operate from role-specific dashboards.
8. Payment settlement closes or reactivates the session.
9. The table becomes available again.

This is supported by current code, migrations, state-machine definitions, API routes, E2E tests, and `STATE-OF-THE-PROJECT.md`.

## Users and roles

### Guests

- Scan QR codes without installing an app.
- Create or join a table session.
- Browse a tenant-branded menu.
- Add items and modifiers to a shared cart.
- See participant presence and order progress.
- Request waiter assistance.
- View the bill.
- Optionally supply a phone number for customer memory, promo caps, and loyalty.

### Session host

The host is a participant with additional authority:

- Submit the shared cart.
- Initiate payment.
- Request the bill.
- Transfer host status.
- Prevent multiple diners from independently submitting or settling the table.

### Waiter

- View live tables and assistance requests.
- Acknowledge and resolve assistance.
- Serve ready orders.
- Settle cash or manually confirmed card payments.
- Participate in loyalty operations where enabled.

### Kitchen

- See active orders with item and modifier detail.
- Advance `confirmed → preparing → ready`.
- Cannot mark food served; that was deliberately moved to front-of-house.

### Manager

- Operational access plus menu, tables, promos, analytics, loyalty configuration, collateral, and restaurant settings.
- Branch-scoped.
- Does not have all owner privileges.

### Owner

- Staff roster and PIN administration.
- Restaurant/organization administration.
- Broader configuration and plan/appearance access.
- Still isolated from platform administration.

### Platform roles

A separate trust domain supports:

- `super_admin`
- `support_admin`
- `billing_admin`
- `read_only_auditor`

The platform console manages organizations, branches, plans, entitlements, feature flags, themes, billing records, onboarding, support inspection, and cross-tenant analytics.

## Restaurant workflow

A restaurant or platform operator:

- Creates an organization, restaurant, and branch.
- Configures branch codes, table identifiers, QR tokens, menu, modifiers, tax/service charges, time zone, branding, session timeout, promos, and staff.
- Prints tenant-themed QR collateral.
- Runs kitchen, waiter, and management views.
- Reviews analytics and operational state.
- Uses the platform onboarding/readiness workflow for setup.

## Order lifecycle

Current order transitions in `backend/internal/domain/statemachine.go`:

```text
pending → confirmed → preparing → ready → served
    └──────────────→ cancelled
```

Key correctness properties:

- Order submission is idempotent.
- Menu prices and modifier prices are snapshotted.
- Item and modifier ownership is validated.
- Human-readable per-branch/day operational IDs are generated.
- The shared cart is cleared after submission.
- Redis publishes the event after the database transaction commits.
- WebSocket clients reconcile through an authoritative snapshot after reconnect.

## Session lifecycle

```text
active
  ├─→ payment_pending
  ├─→ awaiting_reactivation
  ├─→ closed
  ├─→ abandoned
  └─→ expired
```

Terminal states are `closed`, `abandoned`, and `expired`.

During `payment_pending`, cart and order mutations are frozen. A partial unique index prevents more than one non-terminal session per table. Detailed design is recorded in `session-lifecycle-state-machine.md`.

## Payment and checkout

### Implemented

- Immutable bill snapshots.
- Tax, service charge, tip, discount, and total.
- Cash and manual-card settlement by staff.
- Generic signed payment webhooks.
- Webhook replay protection.
- Amount, currency, session, and signature validation.
- Split payments.
- Overpayment rejection.
- Concurrent settlement protection.
- Session closure only once completed payments cover the bill.
- Alert-only escalation for stuck payments.

### Not established as complete

- A live Razorpay, Stripe, Cashfree, or other gateway integration.
- Automated subscription charging.

Platform billing is principally a lifecycle/bookkeeping foundation and was intentionally shadow-only.

---

# 3. Current architecture

```text
Guest / Staff / Platform browser
              │ HTTPS/WSS
              ▼
       nginx reverse proxy
              │
              ▼
       Go/Gin application
 handlers → services → repositories → sqlc
     │             │              │
 WebSocket hub   workers       audit/events
     │             │
     └──── Redis pub/sub, presence, locks,
           rate limits, cache, auth, WS tickets
                    │
                    ▼
             PostgreSQL 17
             source of truth
```

## Backend

- Go/Gin modular monolith.
- Explicit handlers, services, repositories, domain rules, and generated sqlc queries.
- Embedded migrations.
- Structured API errors.
- Central state machines.
- 177 route registrations in the current server.
- 53 Go test files.

## Frontend

- Next.js 15 / React 19 / TypeScript.
- App Router route groups for guest, staff, and platform.
- Zustand state.
- WebSocket connection manager and snapshot reconciliation.
- Tenant-specific guest themes.
- Static operations/platform visual surfaces.
- PWA manifest.
- Responsive kitchen, waiter, admin, and guest experiences.

## Database

- 39 forward migrations.
- 58 explicitly created tables.
- Relational constraints, foreign keys, check constraints, partial unique indexes, enums, and an immutable audit-log trigger.
- PostgreSQL is authoritative; Redis is deliberately treated as disposable.

## Redis

Used for:

- WebSocket event fanout.
- Presence.
- Menu and governance caching.
- Rate limiting.
- Brute-force lockout.
- Staff/platform session support.
- Single-use WebSocket tickets.
- Distributed worker locks.
- Escalation deduplication.

## Background operations

- Stale-session cleanup.
- Expiry warnings.
- Table/session reconciliation.
- Reactivation processing.
- Payment-pending escalation.
- Operational metrics and health checks.

## Infrastructure

- Docker and Docker Compose.
- PostgreSQL and Redis.
- nginx.
- Cloudflare Workers/OpenNext frontend deployment.
- GHCR backend images.
- R2/S3-compatible backup storage.
- Prometheus, Alertmanager, Grafana, and blackbox exporter.
- Optional SigNoz/OpenTelemetry tracing.
- GitHub Actions CI.

---

# 4. Architectural and product evolution

## Rough timeline

| Period | Evolution |
|---|---|
| 1–20 May | Backend-first modular monolith, full relational schema, Redis/WebSockets, services, state machines, integration testing, operational hardening |
| 21 May | Repository reorganized; Next.js frontend introduced |
| 22–26 May | End-to-end guest/staff product, early tenant resolution, subscriptions/analytics, major UI rebuilds, QR administration, CMS, session timeouts |
| June | Images, operational IDs, bills, promos, customer memory; then substantial security, organization, realtime, payment, and lifecycle hardening |
| 23–29 June | Guest credentials replaced raw participant IDs; MFA and lockout; authoritative reconnect; shared cart and host authority; staff workflow refined |
| July | Entitlements, feature flags, themes, support, billing, onboarding, collateral, analytics, loyalty, OpenAPI certification, backup/load/alerting validation |
| Late July | Guest and staff/platform redesign tracks; product-flow corrections from manual testing |
| 1–5 August | Deployment hardening, release certification, security investigation, dependency remediation, telemetry, beta deployment fixes |
| 12 August | Latest T3 checkpoint; no later Mohith-authored feature commit found |

## Important evolution chains

### Backend-only system → full product

- **FACT:** The project began with Go, PostgreSQL, Redis, WebSockets, and Docker on 1 May.
- **FACT:** Next.js was introduced on 21 May after the backend already had services, tests, state machines, reconnect snapshots, and operational tooling.
- **STRONG INFERENCE:** This was API/domain-first development rather than a UI prototype later given a backend.

### Restaurant/branch schema → tenant routing → organization platform

1. Initial schema had restaurants and branches.
2. `8f51c39` added tenant middleware, branch guards, and restaurant resolution.
3. `99e2728` introduced organizations and ownership relationships.
4. Platform users and platform sessions became a separate trust domain.
5. Entitlements, flags, themes, billing, and support were layered above organizations.
6. `b79f91e` later repaired a serious enforcement gap between policy calculation and tenant isolation.

### Per-participant carts → shared table cart

- Initial carts belonged to participants.
- `5580c33` replaced that with one shared cart per session.
- A partial unique index was necessary because PostgreSQL permits multiple `NULL` values in a normal unique constraint.
- Host-only order/payment authority was added to prevent group-ordering races.
- Later testing found inactive-host deadlocks and prompted host reassignment refinements.

### Simple active/closed sessions → operational lifecycle

- Original states: `active`, `closed`, `abandoned`.
- Per-branch timeouts and warnings were added in `1cce930`.
- Payment and reconnect requirements exposed missing intermediate states.
- Migrations `000023` and `000024` introduced `payment_pending`, `awaiting_reactivation`, and `expired`, plus a matching unique-index invariant.
- Cart freezing, credential revocation, reactivation workers, and terminal read windows followed.
- Later bugs occurred when queries and reconcilers still used the old definition of “active.”

### Raw participant identity → revocable guest credentials

- Original guest APIs trusted `X-Participant-ID`.
- Later hardening introduced signed guest credentials containing session, participant, tenant, organization, branch, credential version, expiry, and audience.
- `9d7650f` removed the old header from frontend call sites.
- WebSocket admission moved to short-lived, single-use Redis tickets.
- Participant credential versions are bumped on terminal transitions.

### Promo applied at order → promo applied to immutable payment snapshot

- Initial promo redemption happened during order submission.
- `054f568` moved redemption to payment initiation.
- This aligned the discount with the bill snapshot and payment that actually consumed it.
- Migration `000036` allowed payment-linked redemptions.
- Later canonical-phone fixes ensured per-guest caps were enforceable.

### Optimistic realtime → authoritative reconciliation

- Redis pub/sub provided low-latency events.
- A snapshot endpoint was added for reconnect recovery.
- Pub/sub reconnect and WebSocket close races were fixed.
- Monotonic session event sequences and `snapshot_authoritative` semantics were introduced.
- The frontend learned to replace rather than merge state when instructed.
- Staff/kitchen ultimately used REST polling intentionally, while guest interactions remained realtime.

## Replaced or abandoned approaches

- Raw `X-Participant-ID` guest trust boundary: replaced.
- Per-participant carts: replaced by a shared session cart.
- Promo redemption at order placement: replaced by payment-time redemption.
- Staff tokens in broader persistence: moved to `sessionStorage`/cookie-oriented handling.
- Legacy WebSocket fallback: explicitly removed.
- Tenant theming across all surfaces: constrained to guest surfaces; staff/platform styling pinned.
- “Active means status exactly `active`”: replaced by a non-terminal/live-state concept.
- Automatic stuck-payment mutation: rejected in favor of alert-only escalation.
- Subscription/lifecycle enforcement: built but deliberately left shadow/observe-only.
- Presence-expiry worker behavior: current project record describes it as a no-op stub, with recovery handled elsewhere.

---

# 5. Ownership assessment

## High-confidence ownership

Repeated, longitudinal Mohith-authored commits support high-confidence ownership of:

- Core backend architecture and domain model.
- Sessions, carts, orders, assistance, payments, menu, and staff services.
- PostgreSQL schema and migration evolution.
- Redis/WebSocket realtime architecture.
- Guest, staff, kitchen, admin, and platform frontend flows.
- Multi-tenant organization/platform architecture.
- Authentication and authorization hardening.
- Testing, CI, OpenAPI, release certification, and developer tooling.
- Deployment, backups, alerting, load testing, and observability.
- Product redesign and manual-testing remediation.

This is not based on one commit: the same identity created these systems and returned to repair and extend them over approximately three months.

## Moderate-confidence ownership

- Product strategy and pricing.
- SaaS billing and entitlement policy.
- UX decisions such as host authority, collateral formats, and restaurant operating conventions.
- SigNoz and PostHog adoption decisions.

The code and authored documentation show implementation ownership, but not whether business decisions were independently initiated or supplied externally.

## Low-confidence or unsupported

- Team leadership.
- Stakeholder communication.
- Customer collaboration.
- PR-review quality.
- Mentoring.
- Production incident response.
- Revenue, adoption, or customer impact.

These should not be claimed from this repository alone.

---

# 6. Ranked Canonical written-test story bank

Scores are relative, from 1–5:

- E: evidence strength
- T: technical depth
- O: ownership
- P: problem-solving
- J: judgment
- V: versatility

## 1. Tenant isolation worked in policy but failed in enforcement

**Scores:** E5 T5 O5 P5 J5 V5  
**Ownership:** High confidence

**Situation/problem:** An independent release audit alleged cross-organization and cross-branch authorization bypasses.

**My role:** Mohith-authored commits reproduced, investigated, fixed, tested, and documented the issue.

**Investigation:** A three-tenant attack matrix was constructed. Enabling `AUTHZ_CENTRAL_POLICY_ENFORCE` closed 25 of 27 rows, but two leaks remained. This disproved the proposed “flip the flag” fix.

**Root cause:**

- Shadow-mode authorization allowed tenant-scope denials through.
- Session-event reads had no authorization check.
- Staff deactivation checked role but not branch.
- Some SQL scope parameters came from the target resource, making them tautological rather than actor-scoped.

**Action:** Distinguished tenant-scope violations from role-policy denials and made scope violations unconditional. Added handler checks and regression tests.

**Technical depth:** Required understanding policy evaluation, rollout configuration, handler behavior, resource lookup, SQL scoping, roles, tenant boundaries, audit behavior, and test isolation.

**Decision/tradeoff:** Preserve shadow rollout for potentially incompatible role policy while treating cross-tenant ownership violations as never legitimate.

**Result:** Cross-org and cross-branch attack matrices returned denials under both flag states while legitimate same-branch operations continued working.

**Evidence:** `b79f91e`, `e91c9f1`, `8e18c75`; `release-certification/authz-scope-investigation-2026-08-04.md`; `backend/internal/authz/policy.go`.

**Canonical qualities:** Security reasoning, debugging, skepticism, systems thinking, ownership, verification.

**Possible written-test themes:** Hardest bug; challenging an assumption; security; failure/recovery; testing strategy; engineering judgment.

**Missing personal context:** Who commissioned the audit? How long did the investigation take? What prior security knowledge was needed?

## 2. Rebuilding the session lifecycle around payment and reconnect reality

**Scores:** E5 T5 O5 P5 J5 V5  
**Ownership:** High confidence

**Situation/problem:** The original `active/closed/abandoned` lifecycle could not safely represent payment settlement, temporary disconnects, credential revocation, or table reuse.

**My role:** Authored the cross-layer migrations, state machine, service behavior, credential revocation, workers, frontend states, tests, and invariant documents.

**Investigation:** Payment settlement and reconnect recovery required non-terminal intermediate states. Database uniqueness, application lookups, worker behavior, and guest credentials all had to agree on which states were live or terminal.

**Action:**

- Added `payment_pending`, `awaiting_reactivation`, and `expired`.
- Extended the one-live-session-per-table partial unique index.
- Froze cart/order writes during payment.
- Added participant revocation and credential-version bumps.
- Added reactivation and expiration workers.
- Bounded terminal-session reads.
- Documented formal invariants.

**Technical depth:** State machines spanned database enums, indexes, workers, services, auth credentials, payment transitions, WebSocket behavior, and frontend terminal screens.

**Decision/tradeoff:** Preserve sessions during recoverable disconnects without letting closed credentials resurrect them; keep payment settlement authoritative without stranding tables.

**Result:** A six-state lifecycle with explicit transitions and recovery semantics.

**Evidence:** `f5fd66d`, `994ba62`, `7c5c07a`, `1279a04`, `3d19399`, `8b58a9d`; migrations `000023–000026`; `session-lifecycle-state-machine.md`.

**Canonical qualities:** Systems thinking, state modelling, correctness, architecture, ambiguity.

**Possible themes:** Complex design; architectural change; correctness; cross-layer ownership; learning.

**Missing personal context:** What concrete scenario first exposed the inadequacy of the original states?

## 3. Turning individual carts into a collaborative table cart

**Scores:** E5 T5 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** Per-participant carts did not model a group dining table as one operational unit.

**My role:** Reworked database constraints, services, authorization, presence-based host handling, event propagation, frontend behavior, and tests.

**Investigation:** A shared cart created a new concurrency and authority question: all diners should contribute, but only one should submit or pay.

**Action:** `5580c33` moved to one session-level cart, added host-only submission/payment, presence-aware host reassignment, cart convergence broadcasts, and optional participant phone identity.

**Technical depth:** PostgreSQL's normal uniqueness semantics allow multiple `NULL` participant IDs. A partial unique index on `carts(session_id) WHERE participant_id IS NULL` guaranteed a single shared cart.

**Decision/tradeoff:** Collaboration versus accidental duplicate orders. The host model serialized consequential actions while allowing everyone to contribute.

**Result:** The central product model became a shared table session rather than several independent phone orders.

**Failure/recovery:** Manual testing later found absent-host deadlocks; `5f5b1ef` and `35929b2` refined reassignment and order takeover.

**Evidence:** `5580c33`, `001a823`, migration `000027`, `5f5b1ef`, `35929b2`.

**Canonical qualities:** Product thinking, data modelling, concurrency reasoning, iteration, ownership.

**Possible themes:** Product-led architecture; changing a design; ambiguity; failure and recovery.

**Missing personal context:** Was shared ordering user-driven, self-initiated, or discovered during testing?

## 4. Payment and order correctness hardening

**Scores:** E5 T5 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** The early payment/order paths needed production invariants beyond basic CRUD.

**My role:** Authored a 2,269-line change across 33 files, followed by correctness fixes and E2E certification.

**Action:** Added scoped idempotency keys, immutable bill snapshots, expanded payment states and methods, split-payment support, settlement correctness, webhook verification and replay protection, order locking, transactional validation, and integration tests.

**Technical depth:** Transaction boundaries, payment state transitions, immutable pricing, concurrent settlement, webhook replay, provider events, overpayment, and session closure had to compose safely.

**Decision/tradeoff:** Keep payment providers generic while preserving internal correctness. PostgreSQL remained authoritative; external events became idempotent inputs rather than state owners.

**Result:** Payments could be retried, split, confirmed manually, or settled by verified webhook without duplicating side effects.

**Evidence:** `028e544`, `0f5d514`, migration `000021`; payment E2E specifications; `payment-finalization-invariants.md`.

**Canonical qualities:** Technical depth, transactional reasoning, reliability, system boundaries.

**Possible themes:** Difficult implementation; correctness; API design; external integration; risk management.

**Missing personal context:** Which payment failures were observed versus anticipated through design review?

## 5. Repairing the occupied-table but no joinable-session dead end

**Scores:** E5 T4 O5 P5 J4 V5  
**Ownership:** High confidence

**Symptom:** A QR scan on a table in `payment_pending` or `awaiting_reactivation` found no `active` session, attempted to create another, then failed with `SESSION_ALREADY_ACTIVE`.

**Investigation:** The lookup defined “active” as one enum value, while the unique index defined “live” as three states. Application and database invariants disagreed.

**Hypothesis/root cause:** `GetActiveSessionForTable` used stale lifecycle semantics.

**Action:** Broadened lookup to all non-terminal states. Reactivated `awaiting_reactivation` sessions and allowed joins during `payment_pending`.

**Technical depth:** Required connecting guest QR behavior to SQL lookup semantics, database uniqueness, session states, and reactivation events.

**Decision/tradeoff:** Rejoin an existing live session while still rejecting terminal ones.

**Result/verification:** The lookup matched the exact statuses protected by the index, eliminating the create-then-conflict dead end.

**Evidence:** `37a62cb`; session queries and service.

**Canonical qualities:** Debugging, model consistency, database reasoning, concise corrective design.

**Possible themes:** Hard bug; root-cause analysis; small change with large effect; learning from invariant mismatch.

**Missing personal context:** How was it reproduced and how much time was spent isolating it?

## 6. Finding and fixing a WebSocket double-close race

**Scores:** E5 T4 O5 P5 J4 V4  
**Ownership:** High confidence

**Situation/symptom:** Multiple hub/client paths could close the same send channel.

**Investigation/root cause:** Concurrent unregister/error paths did not share an idempotent close operation.

**Action:** Added `sync.Once` around client send-channel closure and routed hub closure through it.

**Technical depth:** Go channels, goroutine lifecycle, hub ownership, and concurrent disconnect/error behavior.

**Decision/tradeoff:** Centralize shutdown idempotency in the client rather than relying on every caller to coordinate perfectly.

**Result:** Channel shutdown became safe under concurrent lifecycle events.

**Evidence:** `8e728d3`; `backend/internal/websocket/client.go`; `hub.go`.

**Canonical qualities:** Concurrency debugging, Go knowledge, reliability.

**Possible themes:** Difficult bug; race conditions; testing under concurrency; technical learning.

**Missing personal context:** Was it found by the race detector, panic logs, code review, or manual testing?

## 7. Making Redis pub/sub recover instead of silently losing realtime

**Scores:** E5 T4 O5 P5 J4 V4  
**Ownership:** High confidence

**Situation/problem:** If the Redis subscriber exited unexpectedly, realtime propagation stopped even though HTTP and PostgreSQL remained healthy.

**Investigation/hypothesis:** The hub treated subscriber exit as terminal instead of a recoverable dependency failure.

**Action:** Reworked the hub to restart the subscriber with backoff and moved WebSocket upgrader ownership into the hub.

**Technical depth:** Redis subscription lifecycle, goroutine ownership, backoff, hub state, and eventual client reconciliation.

**Decision/tradeoff:** Preserve HTTP availability and recover realtime asynchronously rather than treating Redis loss as total application failure.

**Result:** Redis/pub-sub failure became recoverable; PostgreSQL remained source of truth with snapshot reconciliation after gaps.

**Evidence:** `e0a745a`; later `2c678b5` Redis-flap chaos harness; realtime invariant documentation.

**Canonical qualities:** Failure handling, resilience, component boundaries, systems thinking.

**Possible themes:** Reliability; external dependency failure; recovery; architectural ownership.

**Missing personal context:** What failure first revealed the missing restart behavior?

## 8. Designing authoritative reconnect reconciliation

**Scores:** E5 T5 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** WebSocket delivery is not durable. Reconnects can miss events, replay stale state, or merge state incorrectly.

**My role:** Implemented backend snapshots and sequencing, admission tickets, frontend connection behavior, reactivation UI, and reconciliation semantics.

**Action:**

- Added session snapshots.
- Added sequence-aware events.
- Added `snapshot_authoritative`.
- Required short-lived WebSocket tickets.
- Removed legacy fallback paths.
- Taught the frontend to replace local state when a snapshot is authoritative.
- Added reconnect/reactivation UI.

**Technical depth:** Event ordering, disconnected clients, Redis pub/sub gaps, token admission, client state replacement, and backend authority.

**Decision/tradeoff:** Use WebSockets for latency, not truth; accept a reconciliation read rather than attempting full event sourcing.

**Result:** Reconnect behavior had an explicit recovery contract.

**Evidence:** `4068aae`, `45e2458`, `acad3cd`, `8c26af5`, `d121194`; `realtime-reconciliation-invariants.md`.

**Canonical qualities:** Distributed-systems reasoning, API contracts, failure recovery.

**Possible themes:** Systems design; unreliable networks; state consistency; tradeoffs.

**Missing personal context:** Which mobile/network conditions motivated the design?

## 9. Replacing raw guest IDs with revocable credentials and WS tickets

**Scores:** E5 T5 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** `X-Participant-ID` was an identifier, not an authorization credential. Possession or guessing could cross trust boundaries.

**Action:**

- Introduced HMAC-signed guest tokens with session, branch, tenant, organization, participant, expiry, audience, and credential version.
- Revoked credentials on lifecycle completion.
- Replaced raw-ID frontend APIs in `9d7650f`.
- Added short-lived, single-use Redis WebSocket tickets.
- Added adversarial tests for forged tokens and replay.

**Technical depth:** Credential claims, signing, revocation without centralized guest sessions, HTTP/WS integration, replay, expiry, and tenant scope.

**Decision/tradeoff:** A bespoke signed token simplified the constrained guest domain but is nonstandard compared with JWT.

**Result:** Guest actions became session-scoped and revocable across HTTP and WebSockets.

**Evidence:** `45e2458`, `1279a04`, `9d7650f`; `backend/internal/auth/guest.go`; tenancy/adversarial E2E specs.

**Canonical qualities:** Security, trust-boundary reasoning, cross-layer migration.

**Possible themes:** Security improvement; changing architecture; learning; risk reduction.

**Missing personal context:** Why was a bespoke token selected rather than standard JWT or PASETO?

## 10. Moving promos to the payment boundary

**Scores:** E5 T4 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** Redeeming a promo at order placement did not guarantee the discount corresponded to the eventual immutable bill/payment.

**Investigation:** The financial effect belonged to the bill that was settled, not an earlier order that might not represent final table consumption.

**Action:** `054f568` removed order-time consumption, added payment-time validation/redemption, linked redemptions to payments, and added 166 lines of targeted integration tests.

**Technical depth:** Moving a business invariant across service and transaction boundaries while preserving existing data and API behavior.

**Decision/tradeoff:** Payment initiation becomes more complex, but bill calculation and redemption become atomic and explainable.

**Result:** The payment snapshot became the canonical location for promo effects.

**Evidence:** `054f568`, migration `000036`, payment-promo integration tests; frontend move `9b5d0d8`.

**Canonical qualities:** Domain modelling, transactional boundaries, changing one's mind.

**Possible themes:** Redesign; business invariants; database migration; engineering judgment.

**Missing personal context:** What failure or requirement caused the change?

## 11. Closing the phone-format promo-cap bypass

**Scores:** E5 T4 O5 P5 J4 V5  
**Ownership:** High confidence

**Symptom:** Per-phone promo limits could be bypassed with punctuation variants.

**Root cause:**

- Payment initiation stored a raw client string.
- The normalizer produced multiple representations.
- Cap queries used exact equality against canonical input.

**Action:** Canonicalized writes, corrected normalization, and added migration `000039` to normalize historical rows.

**Technical depth:** The fix had to cover future writes and existing data. The down migration was intentionally non-reversing because reversal would reopen the bypass.

**Decision/tradeoff:** Accept an irreversible data normalization because the original formatting was not valuable and restoration would violate the cap invariant.

**Result:** Historical and new redemptions use the same cap key.

**Evidence:** `0da5f26`; `backend/migrations/000039_normalize_promo_redemption_phones.up.sql`.

**Canonical qualities:** Data repair, security/business-rule correctness, migration judgment.

**Possible themes:** Subtle bug; backwards compatibility; irreversible migration; failure recovery.

**Missing personal context:** How was the bypass discovered?

## 12. Modelling multi-restaurant SaaS governance incrementally

**Scores:** E5 T5 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation:** The system grew from restaurant/branch records into an organization-controlled platform.

**Action:** Added tenant middleware and branch guards; organization ownership and memberships; separate platform users/sessions; entitlement catalog and plan assignments; branch/org/global feature-flag resolution; structured theme configuration; subscription and invoice lifecycle; support and onboarding workflows.

**Technical depth:** Cross-layer tenant context, database ownership relationships, trust-domain separation, runtime configuration, cache resolution, and management UI.

**Decision/tradeoff:** Governance was rolled out additively and often resolve/observe-only before enforcement. Legacy theme/subscription data was bridged rather than immediately deleted.

**Result:** Future platform capabilities could be enabled without schema redesign or tenant-specific deployments.

**Evidence:** `8f51c39`, `99e2728`, migrations `000017–000018`, `000029–000033`; July platform commits.

**Canonical qualities:** Systems thinking, extensibility, migration planning, product architecture.

**Possible themes:** Scaling architecture; abstraction; long-term design; ambiguity.

**Missing personal context:** Which abstractions were anticipated versus required by an explicit roadmap?

## 13. Eliminating an N+1 query in order placement

**Scores:** E5 T4 O5 P4 J4 V4  
**Ownership:** High confidence

**Situation/problem:** Order placement fetched menu items and modifiers repeatedly as the cart grew.

**Investigation:** The query count was proportional to order size even though all required identifiers were available in advance.

**Action:** Added batch queries and changed the service to fetch menu items and modifiers in two queries regardless of item count.

**Technical depth:** SQL query design, generated sqlc layer, repository abstraction, service validation, and result mapping.

**Decision/tradeoff:** Slightly more complex batching and mapping for predictable database work.

**Result:** Query count became bounded rather than proportional to order size.

**Evidence:** `201094c`; 114 insertions and 14 deletions across SQL, generated sqlc, repository, and service layers.

**Canonical qualities:** Performance, data-access reasoning, focused refactoring.

**Possible themes:** Performance improvement; profiling/review; technical debt.

**Missing personal context:** Was the N+1 measured, observed under load, or found by inspection?

## 14. Characterizing capacity instead of guessing

**Scores:** E5 T4 O5 P5 J5 V5  
**Ownership:** High confidence

**Situation:** A review raised concern that the default 20-connection pool might be insufficient.

**Action:** Built a 323-line synthetic driver exercising the full guest flow and ran isolated 50- and 150-concurrency scenarios.

**Investigation/findings:**

- At pilot-scale concurrency, write p99 was approximately 190–230 ms with roughly 0.4% genuine errors.
- At 150 continuous single-branch writers, p99 degraded to approximately 0.8–1.7 seconds and genuine 500s appeared.
- Redis and the pool did not show exhaustion.
- The report identified the per-branch/business-date sequence row as a likely serialization hotspot, explicitly as a hypothesis.

**Technical depth:** Realistic flow generation, rate-limit behavior, WebSocket connection, latency percentiles, error classification, database pool metrics, and interpretation of contention.

**Decision/tradeoff:** Accepted the 20-connection default for a single-restaurant pilot while recording scale limitations and follow-up work.

**Result:** Replaced an unverified capacity concern with a measured pilot envelope and an honest ceiling.

**Evidence:** `7434f93`; `pilot-load-validation-report.md`.

**Canonical qualities:** Evidence-based decision-making, performance testing, intellectual honesty.

**Possible themes:** Scalability; challenging assumptions; measurement; tradeoffs.

**Missing personal context:** Did the sequence-row hypothesis receive later confirmation?

## 15. Building and actually verifying backup/restore

**Scores:** E5 T4 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** A backup script is not evidence that recovery works.

**Action:** Built nightly backup automation with local/R2/S3 abstraction, retention, manifests, and systemd scheduling. Restored a real dump into a throwaway database, compared all 56 tables, and verified sentinel data, migration state, checksum, and the audit immutability trigger.

**Technical depth:** PostgreSQL custom dumps, atomic restore, storage abstraction, integrity manifests, trigger/schema fidelity, retention, and isolated verification.

**Decision/tradeoff:** Used a single transaction to avoid half-restored databases and explicitly separated logical correctness from untested production-volume restore time.

**Result:** Source and target matched; limitations around production R2 and production-scale duration were recorded rather than hidden.

**Evidence:** `647419d`, `08aa4e0`; `restore-verification-report.md`; `docs/backup-restore.md`.

**Canonical qualities:** Operational ownership, reliability, verification, risk management.

**Possible themes:** Going beyond implementation; disaster recovery; initiative; full-system ownership.

**Missing personal context:** Whether the real R2 round trip noted as outstanding was later performed.

## 16. Choosing alert-only handling for stuck payments

**Scores:** E5 T4 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation/problem:** A session can remain in `payment_pending`, but an automated worker cannot safely guess whether money moved.

**Investigation:** Timeout alone cannot distinguish delayed provider confirmation, successful payment with lost notification, abandonment, or genuine failure.

**Action:** Added 5- and 15-minute escalation through metrics, logs, audit records, event logs, WebSocket events, and deduplication, but no automatic payment/session mutation.

**Technical depth:** Distributed timers, worker locks, deduplication, audit/metrics/event fanout, and payment/session state boundaries.

**Decision/tradeoff:** Slower human recovery versus avoiding false settlement, double charge, or premature table closure.

**Result:** The system detects and surfaces ambiguous settlement without manufacturing financial truth.

**Evidence:** `cd9f81f`; `backend/internal/worker/worker.go`; payment escalation documentation.

**Canonical qualities:** Engineering judgment, safety, human-in-the-loop design, systems reasoning.

**Possible themes:** Ambiguity; risk tradeoff; when not to automate; reliability.

**Missing personal context:** Who would operationally receive and resolve the alert?

## 17. Using manual certification to find cross-layer product defects

**Scores:** E5 T4 O5 P5 J4 V5  
**Ownership:** High confidence

**Situation:** A human certification pass found workflow problems not covered by prior automated gates.

**Findings included:** Non-host bill requests; host absence deadlocking a table; rejoin identity loss; table reconciler disagreeing with live-session states; stale feature-gate caches; missing cart images; blank print output; MFA UI not wired; dead platform navigation; onboarding retry failures.

**Action:** `5f5b1ef` fixed seven cross-layer trust/state issues; subsequent commits repaired UI and operational flows. Documentation separated confirmed bugs, missing features, intended behavior, and enhancements.

**Technical depth:** Backend authorization, presence and host state, browser storage, workers and SQL, Redis cache invalidation, API payloads, and frontend UX.

**Decision/tradeoff:** Prioritized correctness and dead-end flows, while explicitly leaving lower-severity enhancements recorded rather than expanding scope indefinitely.

**Result:** Backend, frontend, state reconciliation, cache invalidation, and usability were corrected from one test cycle.

**Evidence:** `5f5b1ef`, `914d902`, `eef468b`, `0207ad6`, `35929b2`; `release-certification/issues-found.md`.

**Canonical qualities:** Testing, product thinking, ownership, failure recovery.

**Possible themes:** Responding to feedback; quality; mistakes; prioritization.

**Missing personal context:** Who performed the manual pass, and how were fixes prioritized?

## 18. Correcting branch-time-zone promo windows

**Scores:** E5 T4 O5 P5 J4 V4  
**Ownership:** High confidence

**Symptom:** Windowed promos silently failed at expected local restaurant hours; the staff UI displayed raw time values.

**Investigation/root cause:** SQL compared against database-server `LOCALTIME`, UTC in the environment, rather than the branch's configured time zone. `pgtype.Time` also reached the frontend without formatting.

**Action:** Converted comparisons to branch-local time, formatted API output as `HH:MM`, and added an integration test.

**Technical depth:** PostgreSQL time semantics, tenant configuration, SQL, generated types, API serialization, and UI behavior.

**Decision/tradeoff:** Treat branch time zone as business truth instead of deployment-local server time.

**Result:** Promo behavior became tenant-location-correct and API output became frontend-safe.

**Evidence:** `08d3d76`; `promo_window_integration_test.go`.

**Canonical qualities:** Time-zone reasoning, data/API debugging, multi-tenant product awareness.

**Possible themes:** Unexpected behavior; difficult domain bug; testing.

**Missing personal context:** Which restaurant or test scenario exposed it?

## 19. Building reproducible CI and fixing the arm64 build correctly

**Scores:** E5 T4 O5 P5 J5 V5  
**Ownership:** High confidence

**Symptom:** GitHub's amd64 runner tried to execute arm64 binaries and failed with `exec format error`. The lint action also had incompatible floating versions.

**Investigation:** The Docker workflow requested `linux/arm64` without QEMU; the builder could compile natively for arm64 but the runtime stage still needed emulation.

**Action:** Added QEMU, pinned the builder to `$BUILDPLATFORM`, kept explicit arm64 cross-compilation, pinned the lint action and linter version, and recorded why floating build gates undermine reproducibility. Added integration, migration round-trip, sqlc drift, OpenAPI, race, vulnerability, and Docker gates.

**Technical depth:** OCI platforms, build/runtime stages, emulation, Go cross-compilation, GitHub Actions, generated-code drift, shared test databases, and reproducible tooling.

**Decision/tradeoff:** Use native-speed compilation and limit QEMU to the small runtime layer.

**Result:** Locally verified arm64 image and binaries with a 1m31s build.

**Evidence:** `8243b5c`, `c4aeb4b`, `f0af2e6`; `.github/workflows/ci.yml`.

**Canonical qualities:** CI/CD, architecture portability, reproducibility, debugging.

**Possible themes:** Infrastructure problem; independent learning; build systems; ownership.

**Missing personal context:** Whether the corrected workflow subsequently ran and passed on GitHub.

## 20. Adding observability without leaking credentials

**Scores:** E5 T4 O5 P5 J5 V5  
**Ownership:** High confidence

**Situation:** Backend tracing was introduced with OpenTelemetry/SigNoz.

**Action:** Instrumented HTTP, PostgreSQL, Redis, request IDs, and audit context; added deployment/evaluation assets and rollout documentation.

**Failure discovered:** Redis tracing included session-token-bearing keys or attributes.

**Recovery:** `dabb770` kept session tokens out of trace spans.

**Technical depth:** Trace propagation, database/Redis instrumentation, correlation IDs, deployment configuration, sensitive attributes, and observability/security interaction.

**Decision/tradeoff:** Preserve useful datastore tracing while explicitly redacting or avoiding authentication material.

**Result:** Better distributed visibility without turning observability into a credential leakage channel.

**Evidence:** `5b67264`, `d4f8a95`, `209a267`, `5444c8b`, `dabb770`; `signoz-adoption-decision.md`.

**Canonical qualities:** Learning, observability, security, correcting an introduced risk.

**Possible themes:** Adopting unfamiliar technology; mistake and recovery; security judgment.

**Missing personal context:** How was the leakage noticed, and what did SigNoz provide beyond Prometheus/logging?

## 21. Chaos testing dependency and process failures

**Scores:** E5 T4 O5 P4 J5 V4  
**Ownership:** High confidence

**Situation:** Unit and integration tests did not establish behavior during infrastructure faults.

**Action:** Added runnable scenarios for Redis flap, backend restart, webhook replay, nginx reload, and WebSocket flood/storm.

**Technical depth:** The harness exercised recovery properties across process, network, event, and abuse boundaries.

**Decision/tradeoff:** Test failures in isolated environments rather than relying on architectural claims or risking the soak stack.

**Result:** Repeatable failure injection supported the design that PostgreSQL remains authoritative while Redis/realtime can recover.

**Evidence:** `2c678b5`; `scripts/chaos/`.

**Canonical qualities:** Reliability, initiative, systems thinking, adversarial testing.

**Possible themes:** Going beyond assigned work; resilience; testing strategy.

**Missing personal context:** Which chaos scenarios failed initially and what fixes resulted?

## 22. Applying platform governance in shadow mode

**Scores:** E5 T4 O5 P4 J5 V5  
**Ownership:** High confidence

**Situation:** Plans, entitlements, feature flags, subscriptions, and organization lifecycle could break legitimate tenants if enforced before scope data and policies were trustworthy.

**Action:** Introduced additive schemas, hierarchical resolution, observability surfaces, cache-backed feature gates, and documented rollout flags.

**Technical depth:** Policy resolution, tenant hierarchy, cache keys/invalidation, auditability, platform APIs, and compatibility with legacy data.

**Decision/tradeoff:** Separate resolve, observe, and enforce. Subscriptions and lifecycle were recorded and audited before being allowed to block operations.

**Failure/learning:** The authorization bypass later showed that tenant isolation cannot be treated like an ordinary shadow policy. `b79f91e` split scope violations from role rollout.

**Result:** A nuanced lesson: shadow rollout is useful for business policy, but inappropriate for fundamental ownership boundaries.

**Evidence:** `7ad7f48` through `d9d9ee6`, `r3-policy-decisions-v1.md`, `b79f91e`.

**Canonical qualities:** Rollout judgment, learning, changing a design, systems thinking.

**Possible themes:** Ambiguity; staged migration; mistake; balancing safety and compatibility.

**Missing personal context:** Was the rollout strategy independently formulated?

---

# 7. Additional evidence worth retaining

These are credible contributions but weaker standalone written-test stories:

- Initial full relational model with deferred session/host foreign key: `413d5a9`, `cc8c03e`.
- Explicit order/assistance/payment state machines: `b72d9ef`, `93a1317`.
- Structured API errors and OpenAPI certification: `7dab77a`, `db05a6e`, July API audit sequence.
- TOTP RFC 6238 and AES-GCM primitives: `561dc60`.
- Redis-backed brute-force lockout: `7550bf2`, `2d86b05`.
- WebSocket abuse rate/strike budget: `1dacc79`.
- Accessible bottom-sheet focus trap and reduced-motion handling: `2fdc5c1`, `f5eff70`.
- Dynamic QR-code import to reduce initial bundle: `7c2c316`.
- Tenant theme allowlisting and guest-only theme scope.
- Premium QR collateral with separation of theme and physical-print configuration.
- Staff analytics derived from existing event logs instead of duplicating event capture.
- Loyalty append-only signed ledger with idempotent payment accrual.
- Marketing/demo flow and Cloudflare delivery.
- Dependency security remediation and `govulncheck` gating: `b16f855`, `f0022d2`, `f0af2e6`.
- Beta deployment validation and follow-up corrections: `8ffd6e0`, `5af1ffa`.

---

# 8. Canonical evidence categories

## Technical depth

Strongest stories:

1. Tenant authorization bypass.
2. Session lifecycle redesign.
3. Payment/order correctness.
4. Reconnect reconciliation.
5. Guest credential migration.
6. Shared collaborative cart.
7. WebSocket race and Redis recovery.

## Ownership

Strongest evidence:

- Created core backend and schema.
- Built the frontend and role-specific experiences.
- Returned repeatedly to fix bugs in those subsystems.
- Extended work through infrastructure, recovery, CI, and certification.
- Authored both implementation and operational reasoning documents.

## Problem solving

Strongest stories:

- Authorization audit disproving the suggested fix.
- Occupied-table/live-state mismatch.
- Promo time-zone failure.
- WebSocket double close.
- arm64 build failure.
- Phone-normalization promo bypass.

## Engineering judgment

Strongest stories:

- Alert-only payment escalation.
- PostgreSQL truth plus snapshot reconciliation.
- Shared cart with host serialization.
- Resolve/observe/enforce platform rollout.
- Payment-time promo application.
- Verified restore rather than backup-only confidence.

## Initiative

Repository-supported candidates:

- Chaos harness.
- Load-test driver.
- Backup verification.
- OpenAPI certification.
- Observability evaluation.
- Dependency-vulnerability CI gate.
- Operational runbooks and recovery procedures.

**Caution:** The repository cannot prove these were unassigned. Personal memory must establish whether they were independently identified and initiated.

## Learning

Promising areas needing personal context:

- Go concurrency and WebSocket lifecycle.
- PostgreSQL partial indexes and deferred constraints.
- Payment idempotency and settlement.
- TOTP/AES-GCM.
- OpenTelemetry/SigNoz.
- Cloudflare Workers/OpenNext.
- arm64 cross-builds/QEMU.
- R2/S3 backup automation.
- Multi-tenant authorization.

## Ambiguity

Best examples:

- Stuck payment: refuse to fabricate settlement.
- Host disappearance: decide who can continue a table.
- Realtime gaps: define authoritative recovery.
- Shadow enforcement: distinguish rollout-compatible policy from absolute isolation.
- Subscription governance: build future structure without prematurely enforcing it.

## Collaboration

No strong repository-only story.

Branch names and merge commits show parallel “track A” and “track B” redesign work, but they do not establish other contributors or collaboration. Do not claim a collaboration example without memory or external records.

## Failure and recovery

Strong examples:

- Shadow authorization allowed tenant bypass.
- Observability risked token leakage.
- Original lifecycle queries disagreed with new database invariants.
- Per-participant carts were replaced.
- Promo timing and redemption boundaries were redesigned.
- WebSocket fallback and raw participant identity were removed.
- Manual testing exposed host, table, cache, and rejoin failures.

## Systems thinking

Strongest examples:

- Session lifecycle spanning database, workers, authentication, payment, realtime, and UI.
- Multi-tenant platform governance.
- Payment correctness.
- Recovery/backup.
- Snapshot reconciliation.
- Authorization boundary repair.
- Capacity characterization.

## Product thinking

Strongest examples:

- Shared cart and host authority.
- Kitchen versus waiter state responsibility.
- Bill snapshot and payment-time promotions.
- Branch-local promo windows.
- Tenant branding limited to customer-facing surfaces.
- QR collateral separated from theme configuration.
- Human settlement authority for ambiguous payments.

---

# 9. Story coverage view

| Story | Debugging | Ownership | Judgment | Learning | Failure/recovery | Systems | Product | Security |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Tenant isolation bypass | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  | ✓ |
| Session lifecycle redesign | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Shared collaborative cart | ✓ | ✓ | ✓ |  | ✓ | ✓ | ✓ |  |
| Payment/order correctness | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Occupied-table dead end | ✓ | ✓ | ✓ |  | ✓ | ✓ | ✓ |  |
| Authoritative reconnect | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Guest credentials |  | ✓ | ✓ | ✓ | ✓ | ✓ |  | ✓ |
| Payment-time promos | ✓ | ✓ | ✓ |  | ✓ | ✓ | ✓ |  |
| Promo phone bypass | ✓ | ✓ | ✓ |  | ✓ | ✓ | ✓ | ✓ |
| Capacity validation | ✓ | ✓ | ✓ | ✓ |  | ✓ |  |  |
| Backup/restore |  | ✓ | ✓ | ✓ | ✓ | ✓ |  | ✓ |
| Alert-only payment escalation |  | ✓ | ✓ |  | ✓ | ✓ | ✓ |  |
| Manual certification remediation | ✓ | ✓ | ✓ |  | ✓ | ✓ | ✓ | ✓ |
| arm64 CI repair | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |  |
| Observability/token leakage | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  | ✓ |
| Shadow governance rollout | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Most reusable stories

1. **Tenant isolation bypass** — strongest overall; usable for debugging, security, skepticism, ownership, testing, judgment, and changing an incomplete proposed solution.
2. **Session lifecycle redesign** — strongest systems-design story; usable for architecture, ambiguity, state modelling, failure recovery, and cross-layer ownership.
3. **Shared collaborative cart** — strongest combined product/technical story.
4. **Payment/order correctness** — strongest transaction and reliability story.
5. **Authoritative reconnect reconciliation** — strongest distributed-state story.
6. **Manual certification remediation** — strongest iterative quality and feedback story.
7. **Capacity characterization** — strongest measurement and evidence-based tradeoff story.
8. **Backup/restore verification** — strongest initiative and operational-ownership story.
9. **Shadow governance rollout** — strongest changed-my-mind-after-new-evidence story.
10. **Observability token leakage** — compact mistake/learning/recovery story.

---

# 10. Questions to recover from memory

Answering these later will materially improve the evidence bank without inventing facts:

1. Is the Mohith Git identity unquestionably yours?
2. What was the project's real context and your formal responsibility?
3. Did you originate the QR Dining idea and the shared-session model?
4. Which features were assigned versus self-initiated?
5. Did anyone review your architecture or commits?
6. What prompted the June production-hardening phase?
7. What prompted the shared-cart rewrite?
8. How did you discover the WebSocket double-close race?
9. Which bugs caused the most investigation time?
10. Which technologies were new to you?
11. Was the independent authorization audit performed by another person or tool?
12. How did you communicate the authorization risk and remediation?
13. Did the load-test sequence-row hypothesis receive follow-up confirmation?
14. Was a real production R2 recovery test eventually performed?
15. Did beta deployment reach external users?
16. Were restaurant operators involved in manual testing?
17. Which design decision did you initially resist or later reverse?
18. Which implementation are you least satisfied with now?
19. What work was deliberately deferred because of time or pilot scope?
20. What direct evidence exists outside Git: PRs, issues, messages, demos, or deployment logs?

---

# Final evidentiary assessment

The repository provides unusually strong evidence for technical depth, longitudinal ownership, system design, debugging, product reasoning, testing, security hardening, and operational initiative.

It provides weak evidence for collaboration, stakeholder communication, mentoring, customer feedback, and real-world business impact. Those categories require personal context or external records and should not be inferred from code alone.

This document should be used as a lookup bank when a written-test question is known. Select the strongest evidenced story for the quality being tested, then add only personal context that can be remembered and defended accurately.
