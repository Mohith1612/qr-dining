# API Surface Certification Report — qr-dining

> **Purpose.** Certify that `openapi.yaml` is an accurate, authoritative map of the HTTP API
> as actually coded, ahead of the comprehensive manual-testing phase. This report is the audit
> deliverable; the spec edits it drives land in subsequent atomic commits.
>
> **Scope.** Documentation/certification only — no feature, route, handler, or business-logic
> changes. Source of truth: `backend/internal/server/server.go` (all 157 routes registered in
> one file), with DTOs/validation read from `internal/handlers/*` and `internal/services/*`.
>
> **Branch.** `premium-qr-collateral` (sits atop platform-governance, billing/subscription,
> support-console, theme, pilot-remediation, and collateral work — so the full surface is present).
>
> **Compiled.** 2026-05-30.

---

## 1. Route Inventory

**Totals by trust domain (verified line-by-line against `server.go`):**

| Trust domain | Auth | Routes | Documented (pre-audit) |
|---|---|---:|---:|
| Infrastructure (`/health`, `/readyz`, `/metrics`, `/ws`) | none | 4 | 3 |
| Public / Guest | `RateLimit` 60; guest Bearer per-handler | 24 | ~17 |
| Auth endpoints | `RateLimitSensitive("auth")` 10 | 3 | 1 |
| Staff | `StaffAuth` (+`BranchTenantGuard`) | 52 | ~12 |
| Platform | `PlatformAuth` | 74 | 0 |
| **Total** | | **157** | **~36** |

### 1.1 Infrastructure (no auth, no rate limit)
| Method | Path | Handler |
|---|---|---|
| GET | `/health` | `HealthHandler.Health` |
| GET | `/readyz` | `HealthHandler.Readiness` |
| GET | `/metrics` | promhttp (custom registry) |
| GET | `/ws` | `WSHandler.Upgrade` (ticket or legacy guest token resolved in handler) |

### 1.2 Public / Guest (`RateLimit` 60 RPM; guest credential enforced per-handler via `requireGuestSession`)
| Method | Path | Handler | Notes |
|---|---|---|---|
| POST | `/sessions` | `SessionHandler.Create` | |
| GET | `/sessions/:id` | `SessionHandler.Get` | guest-safe (no `session_token`) |
| DELETE | `/sessions/:id` | `SessionHandler.Close` | host only |
| POST | `/sessions/:id/join` | `SessionHandler.Join` | |
| POST | `/sessions/:id/reactivate` | `SessionHandler.Reactivate` | **undocumented** |
| POST | `/sessions/:id/ws-ticket` | `SessionHandler.IssueWSTicket` | sensitive 60 + per-session 12; **undocumented** |
| GET | `/sessions/:id/cart` | `CartHandler.GetCart` | shared cart |
| POST | `/sessions/:id/cart/items` | `CartHandler.AddItem` | |
| DELETE | `/sessions/:id/cart/items/:item_id` | `CartHandler.RemoveItem` | |
| POST | `/sessions/:id/orders` | `OrderHandler.PlaceOrder` | per-session 12; host-gated |
| GET | `/sessions/:id/orders` | `OrderHandler.ListOrders` | |
| POST | `/sessions/:id/assist` | `AssistanceHandler.Request` | per-session 6 |
| POST | `/sessions/:id/payments` | `PaymentHandler.InitiatePayment` | sensitive 30 + per-session 6; host-gated |
| GET | `/sessions/:id/bill` | `BillingHandler.GetBill` | **undocumented** |
| POST | `/webhooks/payments/:provider` | `PaymentHandler.Webhook` | sensitive 200; HMAC signed |
| POST | `/sessions/:id/customer` | `CustomerHandler.LinkCustomer` | **undocumented** |
| POST | `/sessions/:id/promos/validate` | `PromoHandler.ValidatePromo` | **undocumented** |
| GET | `/branches/:id/menu` | `MenuHandler.GetMenu` | `BranchTenantGuard` |
| GET | `/branches/:id/feature-flags` | `FlagHandler.ResolveForBranch` | `BranchTenantGuard`; **undocumented** |
| GET | `/branches/:id/theme` | `ThemeHandler.ResolveForBranch` | `BranchTenantGuard`; **undocumented** |
| GET | `/tables/by-qr/:token` | `MenuHandler.GetTableByQR` | |
| GET | `/sessions/:id/snapshot` | `SnapshotHandler.GetSnapshot` | reconnect reconciliation; guest-safe |
| GET | `/tenants/by-slug/:slug` | `TenantHandler.GetBySlug` | returns additive `theme`; **undocumented** |
| GET | `/plans` | `SubscriptionHandler.ListPlans` | **undocumented** |

### 1.3 Auth endpoints (`RateLimitSensitive("auth")` 10 RPM, fail-closed)
| Method | Path | Handler |
|---|---|---|
| POST | `/staff/auth` | `StaffHandler.Authenticate` |
| POST | `/platform/auth` | `PlatformHandler.Authenticate` (**undocumented**) |
| POST | `/platform/auth/mfa` | `PlatformHandler.CompleteMFA` (**undocumented**) |

### 1.4 Staff (`StaffAuth`; branch-scoped group adds `BranchTenantGuard`) — 52 routes
Operational: `POST /staff/logout`, `PATCH /orders/:id/status`, `PATCH /payments/:id/settle`,
`PATCH /assist/:id/ack`, `PATCH /assist/:id/resolve`.
Branch dashboard (`/branches/:id/*`, tenant-guarded): `orders/active`, `sessions/active`,
`assist/active`, `payments`, `menu/full`, `POST menu/categories`, `POST menu/items`, `POST staff`,
`events/recent`, `analytics/{top-items,busy-hours,order-volume}`, `GET|POST tables`,
`GET|PUT collateral`, `GET|PATCH ""` (branch detail/update), `customers`, `audit`,
`GET|POST promos`, `DELETE promos/:promo_id`.
Org governance (`/orgs/:org_id/*`): `GET|PATCH ""`, `branches`, `analytics/{top-items,busy-hours,order-volume}`, `audit`.
Item-scoped menu admin: `PATCH|DELETE /menu/items/:id`, `PATCH /menu/items/:id/availability`,
`PATCH /menu/items/:id/featured`, `POST /menu/items/:id/modifiers`,
`DELETE|PATCH /menu/categories/:id`, `DELETE /menu/modifiers/:id`.
Misc: `POST /upload/menu-item-image`, `POST /upload/restaurant-logo`, `PATCH /tables/:id/qr-refresh`,
`PATCH /staff/:id/pin`, `PATCH /staff/:id/deactivate`, `GET /sessions/:id/events`,
`GET /restaurants/:id/subscription`, `GET /customers/:id/history`, `DELETE /customers/:id`.
**Documented pre-audit:** only ~12 (auth, create/pin/deactivate staff, 3 dashboards, 2 menu create,
item update/availability, 2 event-log). All else **undocumented**.

### 1.5 Platform (`PlatformAuth`; RBAC enforced in handlers) — 74 routes, **0 documented**
- **Identity/MFA:** `POST /platform/auth/logout`, `/mfa/enroll`, `/mfa/confirm`, `/mfa/disable`,
  `GET /users`, `GET /users/:id`.
- **Organizations:** `GET|POST /organizations`, `GET|PATCH /organizations/:org_id`,
  `GET|POST /organizations/:org_id/branches`.
- **Branches:** `GET /branches/:branch_id`, `GET|POST /branches/:branch_id/tables`,
  `GET|PUT /branches/:branch_id/collateral`.
- **Support console:** `GET /support/search`, `POST|GET /support/sessions`, `GET /support/sessions/:id`,
  `GET /audit`, `GET /sessions/:id`, `GET /orders/:id`, `GET /payments/:id`.
- **Plans & entitlements:** `GET /entitlements`, `GET|POST /plans`, `PATCH /plans/:plan_id`,
  `PUT /plans/:plan_id/entitlements`, `GET /organizations/:org_id/entitlements`,
  `PUT /organizations/:org_id/plan`, `PUT /organizations/:org_id/entitlements/:key`.
- **Subscriptions & billing:** `GET /organizations/:org_id/subscription` + 6 lifecycle POSTs
  (activate/suspend/renew/cancel/extend-trial/plan); `GET|PUT .../billing-profile`;
  `GET|POST .../payments`; `GET|POST .../invoices`, `GET .../invoices/:invoice_id`,
  3 invoice lifecycle POSTs (issue/mark-paid/cancel).
- **Org/branch lifecycle:** `POST /organizations/:org_id/{suspend,activate}`,
  `POST /branches/:branch_id/{suspend,activate}`.
- **Feature flags:** `GET|POST /flags`, `PATCH /flags/:key`, `PUT|DELETE /flags/:key/global`,
  `PUT|DELETE /organizations/:org_id/flags/:key`, `GET /organizations/:org_id/flags`,
  `PUT|DELETE /branches/:branch_id/flags/:key`, `GET /branches/:branch_id/flags`.
- **Enforcement observability:** `GET /observability/{subscriptions,entitlements,flags}`.
- **Analytics:** `GET /analytics/{usage,revenue,health}`.
- **Themes:** `GET /theme/presets`, `GET|PUT /organizations/:org_id/theme`.

---

## 2. Gap Report

```
Routes found (server.go):          157
Documented (pre-audit openapi):    ~36
Missing / undocumented:            ~121
```

**Entirely undocumented domains (0 coverage):** the whole Platform surface (74) — organizations,
branches, plans, entitlements, feature flags, platform analytics, themes, support console, billing,
subscriptions, collateral, MFA, users, lifecycle, observability.

**Undocumented public/guest routes (7):** `reactivate`, `ws-ticket`, `bill`, `customer`,
`promos/validate`, `theme`, `feature-flags`, plus `tenants/by-slug` and public `plans`.

**Undocumented staff routes (~40):** all branch-scoped dashboards beyond the 3 listed, all org
governance, item-scoped menu admin (featured/modifiers/category CRUD/delete), tables, qr-refresh,
upload presign, promos, branch analytics, audit reads, subscription read, customer history/delete,
staff collateral.

**Stale/incorrect in what *is* documented:**
- **Auth model is wrong** (see §3) — `ParticipantID` presented as the credential; no Bearer scheme
  for guest/platform.
- `/payments/{id}/settle` → `security: [{StaffAuth: []}]` references a **scheme that does not exist**
  in `components.securitySchemes` (broken `$ref`-like reference; only `StaffBearer`/`ParticipantID` defined).
- `Session` schema advertises `session_token` — but every guest response now strips it (F-8). Stale/leaky doc.
- `Session.status` enum lists only `[active, closed, abandoned]` — missing `payment_pending`,
  `awaiting_reactivation`, `expired` (migration 000023; 6 states total).
- `info.version` `1.0.0` understates a major surface expansion (bump → `2.0.0`).
- Missing top-level tags for every new domain.

---

## 3. Auth Documentation Audit

**Reality (from `internal/auth/guest.go`, `middleware/{staff_auth,platform_auth}.go`,
`frontend/lib/api/client.ts:52`):** all three trust domains use `Authorization: Bearer <token>`
with three distinct token types. `X-Participant-ID` is still read (`session.go:307`) as a
**non-credential participant identifier** on some guest mutations, not as the auth credential.

| Domain | Credential (actual) | Issued by | Spec today | Fix |
|---|---|---|---|---|
| Public | none | — | none | keep |
| Guest | `Authorization: Bearer <guest HMAC token>` (gated by R6 `AUTH_GUEST_CREDENTIALS_REQUIRED`; present-but-invalid tokens always rejected) | `POST /sessions`, `/join` | `ParticipantID` header (wrong) | add `GuestBearer`; keep `ParticipantID` only where genuinely sent, annotated non-credential |
| Staff | `Authorization: Bearer <staff token>` (or HttpOnly cookie when `AUTH_STAFF_COOKIE_ENABLED`) | `POST /staff/auth` | `StaffBearer` (ok) | keep; fix `StaffAuth` ref |
| Platform | `Authorization: Bearer <platform token>` (separate trust domain; staff tokens rejected; optional TOTP MFA) | `POST /platform/auth` (+`/mfa`) | none | add `PlatformBearer` |

**Platform RBAC (enforced in handlers, to be documented per-operation):**
`super_admin` (bypass) > `support_admin` > `billing_admin` > `read_only_auditor`. Reads generally
allow support/billing/auditor; mutations are `super_admin`; billing mutations require `billing_admin`;
support reads exclude `billing_admin`; audit explorer requires `read_only_auditor`.

---

## 4. Risk Findings (implementation vs documentation disagreements)

- **R-1 (auth model, high).** Spec's documented auth (`X-Participant-ID` credential, no Bearer for
  guest/platform) does not match code. A reader testing from the spec would send the wrong header.
  *Resolved by this certification (Commit 2).*
- **R-2 (broken security ref).** `/payments/{id}/settle` references undefined `StaffAuth`.
  *Resolved (Commit 2/3).*
- **R-3 (F-8 credential leak in doc).** `Session` schema lists `session_token`, which the code now
  strips from every guest response (`handlers.guestSafeSession()`). The doc must not advertise it.
  *Resolved (Commit 2/3).*
- **R-4 (support-session grant — doc says "deferred", code ships it).** master-system-context §16.11
  lists "requiring an active support-session *grant*" as deferred, but the code implements
  `POST /platform/support/sessions` (`platform.go:609`) with reason (min 8 / max 500), org/branch
  scoping, **4h soft cap → 422 `SUPPORT_DURATION_EXCEEDS_SOFT_CAP`**, **24h hard cap → 400**, plus
  `GET /support/sessions` and `GET /support/sessions/:id` (**410 `SUPPORT_SESSION_EXPIRED`** when past
  `expires_at`). Nuance: the support **detail reads** (`/platform/{sessions,orders,payments}/:id`) do
  **not require** an active grant — they are gated by role + dual-audit only. So grants are *recorded*
  but not yet *enforced* on reads. **Document the code as implemented**; flag that reads don't consume
  the grant. *Captured here; spec documents actual behavior (Commit 8).*
- **R-5 (resolve-only/inert surfaces).** Entitlements & feature flags are computed but enforce nothing;
  org/branch suspend/activate flips a status column no operational path reads. Not a doc bug, but each
  such endpoint's `description` must say so to avoid testers assuming enforcement. *(Commits 5–8.)*
- **R-6 (audit-coverage gap, pre-existing).** Order placement emits no audit row (`order.go`). Out of
  scope to fix; noted so manual testing doesn't expect an `order.place` audit entry.

---

## 5. Manual Testing Recommendations

Areas deserving special attention in the upcoming pass, ranked by risk/novelty:

1. **Guest auth transition (R6).** Verify `Authorization: Bearer` is accepted and that a
   present-but-invalid guest token is rejected (401/403), not failed-open (the T-01/X-03 fix), under
   `AUTH_GUEST_CREDENTIALS_REQUIRED` both off and on. Confirm `X-Participant-ID` alone is not treated
   as a credential.
2. **F-8 credential non-leak.** Assert no `session_token` appears in Create/Get/Join/Reactivate/snapshot
   responses, regardless of R6.
3. **Support console RBAC + dual-audit.** `billing_admin` → 403 on support reads but 200 on analytics;
   each session/payment detail read produces a tenant-visible `audit_log` row; support-session grant
   caps (4h→422, 24h→400, expired→410).
4. **Billing/subscription/invoice state machines.** Exercise every transition and confirm 409
   `INVALID_TRANSITION` on illegal ones; confirm money fields are strings.
5. **Collateral validation.** Bad `format` / unknown JSON key / oversized text → 400; cross-branch
   isolation (403); platform and staff writes land in the *same* `branch_collateral` row.
6. **Theme entitlement gate.** Custom tokens without `custom.theme` → 403; non-hex token value → 400;
   unknown token key → 400.
7. **Snapshot reconnect contract.** `missed_events` vs `snapshot_authoritative`; terminal-session
   60-minute read window then gone.
8. **MFA-unconfigured path (PT-03).** MFA enroll with `MFA_ENCRYPTION_KEY` unset → 503
   `MFA_NOT_CONFIGURED` (not 500).
9. **Rate-limit fail-closed.** Payment-init/webhook/auth limits behave correctly (and fail closed) under
   Redis pressure; per-session caps on orders/assist/ws-ticket.
10. **Tenant guard.** `BranchTenantGuard` cross-tenant access denied on `/branches/:id/*` when
    `TENANCY_ORGANIZATIONS_ENABLED`.

---

## 6. OpenAPI Changes (filled in as commits land)

- Commit 2 — auth schemes (`GuestBearer`/`StaffBearer`/`PlatformBearer`), fixed refs, version bump, tags, shared schemas.
- Commit 3 — guest/public surface corrected + completed; F-8 + snapshot contract.
- Commit 4 — full staff surface.
- Commit 5 — platform governance + identity + lifecycle.
- Commit 6 — plans, entitlements, feature flags.
- Commit 7 — billing, subscriptions, analytics, observability.
- Commit 8 — theme, support console, collateral.
- Commit 9 — `redocly.yaml` + validation; final counts below.

## 7. Validation Results

_To be completed in Commit 9 (`npx @redocly/cli lint openapi.yaml`)._

## 8. Final Coverage

_To be completed in Commit 9: documented-vs-actual route count and any intentional omissions._
