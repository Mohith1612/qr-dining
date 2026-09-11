# Staff Performance Analytics + Customer Loyalty Foundation — Delivery Summary (v1)

**Branch:** `feature/staff-analytics-loyalty` (off `premium-qr-collateral` @ `0bd230f`, unmerged)
**Date:** 2026-06-10
**Posture:** additive capability phase. No change to session lifecycle, payment lifecycle
semantics, websocket, auth, `internal/authz`, the 9 env strict-rollout flags, or the soak.

---

## 1. Architecture summary

Two new gated capabilities, both built on the existing governance architecture:

- **Staff Performance Analytics (Part A).** Read-only aggregation over data the system
  *already records*: `event_log` (order status transitions and assistance ack/resolve
  already carry staff `actor_id`), `payments` (`settled_by_staff_id`/`settled_at`),
  and `staff_sessions` (login moments + activity bounds). **No new event capture system**
  and no schema change to any operational table — the only DDL is a composite index on
  `event_log`. Operational analytics, not surveillance: per-staff activity rollups,
  windowed (daily/weekly/monthly, UTC), 5-min Redis cached, following the existing
  `services/analytics.go` conventions.
- **Customer Loyalty Foundation (Part B).** Customer → spend → points. Three new tables
  (program rule, org-scoped accounts, append-only signed points ledger). Earn rides the
  two payment-completion paths through a **nil-safe, fully error-swallowed hook**
  (`PaymentService.SetLoyaltyAccrual`, same setter pattern as `SetHostAuthority`) — a
  loyalty failure can never fail a payment. Redemption is **ledger-only**: staff record a
  points deduction (the discount is applied off-system); bill/payment math is untouched.
  No tiers, catalog, campaigns, coupons, referrals, or gamification.

**Gating model (both features):** `FeatureGate` (`services/feature_gate.go`) =
**entitlement capability** (org-level: plan grant or org override) **AND** **platform
product flag** (branch > org > global > catalog default). This realizes the required
platform controls through the existing governance plane: *enable per plan* =
`plan_entitlements`; *override per org* = entitlement override / org flag override;
*override per branch* = branch flag override. Gate results are Redis-cached 60s, so flag
flips propagate within a minute. All loyalty routes are **branch-scoped** (not under
`/orgs/:org_id/*`, which is dead until the R2 tenancy flag flips) while loyalty data
stays org-scoped — works pre- and post-R2.

## 2. Data model summary

- **Migration `000034_staff_performance_analytics`:** index
  `idx_event_log_branch_type_created` on `event_log(branch_id, event_type, created_at)`;
  seeds entitlement `analytics.staff_performance` + flag `staff_performance_analytics`
  (`default_enabled=false`).
- **Migration `000035_customer_loyalty`:**
  - `organization_loyalty_programs` — org PK; `is_active` (default false);
    `earn_rate_points`/`earn_rate_amount` ("X points per ₹Y", default 1 per 100.00).
  - `customer_loyalty_accounts` — `UNIQUE(customer_id, organization_id)`;
    `points_balance` (`CHECK >= 0`), lifetime earned/redeemed, `visit_count`,
    `lifetime_spend NUMERIC(12,2)`.
  - `customer_loyalty_transactions` — append-only signed ledger
    (`earn`/`redeem`/`adjustment`); **partial unique index on `payment_id WHERE
    type='earn'` = earn idempotency**; FK links to payment/session; staff/system actor.
  - Seeds: entitlements `loyalty.enabled`, `loyalty.redeem`, `loyalty.manual_adjustment`
    + flag `loyalty` (`default_enabled=false`).
- Points math uses exact rational arithmetic (`computeEarnPoints`, `math/big`) — no float
  drift; `floor(amount × rate_points / rate_amount)`.
- Concurrency: redemption is a guarded `UPDATE … WHERE points_balance >= X RETURNING *`
  (row lock serializes; no-row = insufficient; the CHECK constraint is the backstop) —
  verified by a concurrent-redeem integration test.

## 3. Entitlement / flag additions (all default OFF, granted to no plan)

| Kind | Key | Gates |
|---|---|---|
| capability | `analytics.staff_performance` | staff-performance endpoints |
| capability | `loyalty.enabled` | earn accrual + all loyalty surfaces |
| capability | `loyalty.redeem` | redemption endpoint |
| capability | `loyalty.manual_adjustment` | manual adjustment endpoint |
| flag | `staff_performance_analytics` | paired with the analytics capability |
| flag | `loyalty` | paired with all loyalty capabilities |

Catalog-seeding the two flags deviates deliberately from 000030's empty catalog so the
gate keys always exist (noted in the migration headers). Constants in
`services/entitlement.go` / `services/feature_gate.go`. The free-tier/subscription bridge
(`bridgeFeatures`) never auto-grants the new keys (verified).

## 4. APIs added (11 operations; OpenAPI 2.1.0, 168 total ops, redocly 0 errors)

Staff (StaffBearer, `BranchTenantGuard`; gated 403 `STAFF_ANALYTICS_DISABLED` / `LOYALTY_DISABLED`):
- `GET /branches/:id/analytics/staff/waiters|kitchen|summary` (owner/manager)
- `GET /branches/:id/analytics/loyalty` (owner/manager)
- `GET|PUT /branches/:id/loyalty/program` (owner/manager)
- `GET /branches/:id/loyalty/customers?phone=` (staff; kitchen excluded)
- `GET /branches/:id/loyalty/accounts/:account_id/transactions`
- `POST /branches/:id/loyalty/accounts/:account_id/redeem` (owner/manager/waiter; 409 `LOYALTY_INSUFFICIENT_POINTS`)
- `POST /branches/:id/loyalty/accounts/:account_id/adjust` (owner/manager; reason required)

Platform (PlatformBearer; support/billing/auditor RBAC, platform-audited, **not**
tenant-entitlement-gated — operator observability like the other platform analytics):
- `GET /platform/analytics/staff-performance?branch_id=&period=`

## 5. Frontend views added (manager-facing, minimal)

- Staff admin dashboard: **Performance** tab (`components/staff-performance/PerformanceTab.tsx`)
  — period selector, per-waiter table (sessions/tables/served/assists/response/settles/logins/active),
  kitchen cards (completed, avg prep, peak/hr), daily-activity summary; and **Loyalty** tab
  (`components/loyalty/LoyaltyTab.tsx`) — earning-rule form, weekly summary tiles + top
  customers, phone lookup → balance/ledger, redeem + manual-adjust actions.
- Both tabs gate on owner/manager and render the existing `EmptyState` upsell pattern on
  the gated 403 codes. Admin page touch: 2 imports + 2 `TABS` entries + 2 renders.
- Platform UI: **no new pages needed** — the entitlements and flags pages are
  catalog-driven, so the new keys appear automatically once migrations run.

## 6. OpenAPI changes

`openapi.yaml` bumped **2.0.0 → 2.1.0**; 11 new operations (paths above), new schemas
(`WaiterPerformanceRow`, `KitchenPerformanceRow`, `StaffDailyActivityRow`,
`BranchStaffPerformance`, `LoyaltyProgram`, `LoyaltyAccount`, `LoyaltyCustomerLookup`,
`LoyaltyTransaction`, `LoyaltyAnalytics`), gated-403 response components
(`StaffAnalyticsForbidden`, `LoyaltyForbidden`), tags `Staff Performance` and `Loyalty`.
`npx @redocly/cli lint`: **0 errors**, the same 9 pre-existing informational
`operation-4xx-response` warnings as the 2.0.0 certification. Route↔spec parity holds:
11 new routes in `server.go` = 11 new operations.

## 7. Tests executed

- **Unit (green):** feature-gate decision truth table + missing-flag fallback;
  `computeEarnPoints` (floor/boundary/fractional-rate/large-amount/invalid-numeric).
- **Integration (green on a throwaway Postgres container, never the soak):**
  - `TestStaffPerformanceAnalytics` — gate off → `STAFF_ANALYTICS_DISABLED`; gate on →
    each waiter metric (~120s response, ~60s settlement, 1 login/~1h active), kitchen
    (~300s prep, peak/hr), daily summary, platform read bypassing the tenant gate.
  - `TestLoyaltyEarnRedeemAdjust` — gate-off no-op; program-missing/inactive no-op;
    earn 250→2pts; **idempotent replay** (one ledger row); same-session second payment =
    points-yes/visit-no; new session = visit++; phone lookup; redeem capability gate;
    insufficient points; **two concurrent redeems → exactly one succeeds, no overdraw**;
    adjustment capability gate + zero floor; analytics totals/participation/top customers.
  - `TestLoyaltySkipsAnonymousSessions` — no customer link → zero loyalty rows.
  - `TestSettlePaymentTriggersLoyaltyEarn` — real `SettlePaymentByStaff` wiring: gate off
    → settles with zero side effects; gate on → settles AND credits floor(350/100)=3.
- **Suite health:** full untagged suite green (`go test -p 1 ./...` against the throwaway
  DB; serialized because packages share one DB). The `-tags integration` failures that
  exist (`session_business_date` inserts, `TestCreateSession_ConcurrentSingleActiveSession`)
  **reproduce identically on the base commit `0bd230f` with a fresh DB** — pre-existing,
  unrelated to this work (consistent with the documented "CI-green, not local-green" state).
- `go build ./...`, `go vet ./...`, `make sqlc-generate` (no drift), `tsc --noEmit`,
  `next build` — all clean.

## 8. Rollout impact assessment (Part D)

**With `analytics.staff_performance` / `loyalty.enabled` unresolved-true (the default for
every org), existing restaurant behavior is unchanged:**
- All four capabilities are granted to **no plan**; both flags are `default_enabled=false`
  → every gate resolves disabled everywhere until a platform operator deliberately grants
  an entitlement (plan or override) **and** enables the flag (global/org/branch).
- New endpoints exist but return 403 gated codes; new tabs render gated empty-states.
- Existing-path touches, exhaustively: (a) the loyalty hook after payment completion —
  nil-safe, void, error-swallowed, runs after the payment row is terminal; when disabled
  it costs one (60s-cached) gate read and does nothing; failures increment the new
  `loyalty_accrual_failures_total` counter and never propagate; (b) `server.go` wiring;
  (c) one additive `PlatformHandler` constructor parameter; (d) two admin-page tab
  entries. No websocket/session/payment/auth semantic change; no rollout-flag change;
  migrations are additive with clean downs; the soak stack was never touched (tests ran
  on a dedicated throwaway container).
- Known caveats (documented in code/API descriptions): shared kitchen logins blur
  per-staff kitchen attribution; "sessions handled" is a derived touched-sessions
  definition (no assignment system); prep-time averages exclude orders whose `preparing`
  event predates the window; flag flips propagate within ≤60s (gate cache); refunds do
  not claw back points (manual adjustment is the operational escape hatch).

**To enable for one org (operator runbook):** assign the entitlement via
`PUT /platform/organizations/:org_id/entitlements/analytics.staff_performance` (or a plan
grant), enable the flag via `PUT /platform/organizations/:org_id/flags/staff_performance_analytics`
(or a branch override), and for loyalty additionally grant `loyalty.enabled` (+`redeem`/
`manual_adjustment` as desired), enable the `loyalty` flag, then have an owner/manager
activate the earning rule in the admin Loyalty tab.

## 9. Remaining future phases (deliberately out of scope)

- **Loyalty:** bill-integrated redemption (requires a payment-correctness change — its own
  scoped phase, mirroring the promo-on-bill deferral); refund clawback; guest-facing
  balance view (CustomerOptIn exists as the anchor); rewards catalog/tiers/campaigns if
  ever wanted (explicit non-goals for Phase 1); expiry policies.
- **Staff analytics:** org-level rollups once R2 tenancy flips (`/orgs/:org_id/*` route
  group); per-staff order *claiming* if real kitchens want true individual attribution
  (would need an assignment concept — product decision first); CSV export (kept gated like
  the support console's deliberate export omission); platform UI panel for the operator
  endpoint (API-only today).
- **Governance:** these are the first consumer of entitlement enforcement beyond the
  theme gate; when entitlement enforcement generally goes strict, fold these keys into
  that staged rollout discipline.
