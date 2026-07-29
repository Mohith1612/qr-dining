# Manual Testing Checklist — Release Candidate Certification

- **Build:** branch `feature/certification-fixes-ui-redesign`
- **Environment:** isolated manual-testing stack — Frontend A `http://localhost:3000` (backend :8090), Frontend B `http://localhost:3001` (backend :8095). Both backends share one DB/Redis, so realtime events cross instances; use guest on A and staff on B to exercise multi-instance fan-out.
- **Convention:** fill in Pass/Fail; add anything unexpected to Notes, even if it passes.
- QR tokens regenerate on every `--reset`; get current ones with:
  `docker exec manual-testing-postgres psql -U mtest -d qrdining_mtest -tAc "select branch_id, identifier, qr_code_token from tables order by 1,2;"`

---

## Guest (Frontend A, `http://localhost:3000/table/<qr_token>` — use Saffron Bandra T3, a free table)

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| G-1 | QR scan / landing | Opening `/table/<token>` shows the tenant-branded (dark-luxury for Saffron) join screen with restaurant/branch/table identity; invalid token shows a clear error, not a crash | | |
| G-2 | Join session | Entering a display name creates/joins the table session; first joiner becomes host (host badge/indicator visible) | | |
| G-3 | Browse menu | Menu renders with categories, item cards, prices, veg/non-veg markers, images/placeholders; unavailable items are visibly disabled | | |
| G-4 | Variants | An item with variants opens a variant picker; price updates per variant; chosen variant shows correctly in cart | | |
| G-5 | Cart (shared, live) | Add/increment/decrement/remove updates the cart; a second device in the same session sees the change live without refresh | | |
| G-6 | Coupon | Applying a valid promo adjusts the bill; invalid/ineligible code shows a clear rejection; removal restores the total. **Known issue:** time-windowed promos compare against UTC, not branch timezone — do not certify windowed promos | | |
| G-7 | Checkout / send order | Only the host can send the order; non-host sees a disabled/blocked action with explanation; sent order appears with status Confirmed | | |
| G-8 | Live order tracker (F-6 regression) | As kitchen advances the order (confirmed → preparing → ready) and waiter serves, the guest tracker advances live, without refresh | | |
| G-9 | Assistance | Request assistance → visible acknowledgment; resolves when waiter resolves it | | |
| G-10 | Bill request | Host requests bill; bill shows itemized totals incl. promo adjustments; non-host cannot request | | |
| G-11 | Payment | Host initiates payment (cash/manual path); session moves to payment_pending; waiter settlement completes it; guest sees confirmation | | |
| G-12 | Session expiry (F-1 regression) | Idle session drifts to awaiting_reactivation (~1 min in this env); guest sees a reactivation prompt — NOT a dead "Connection lost" screen; reactivating restores the session | | |
| G-13 | Rejoin session | Closing the tab and re-scanning the same QR rejoins the existing session with cart/order intact | | |
| G-14 | Second device | Second browser profile joining the same table shares cart, orders, and events live | | |
| G-15 | Ended session | After staff closes the session, guest tab shows the Session Ended screen (no infinite spinner) | | |
| G-16 | Token security spot-check (F-8, known open) | In devtools → Network, confirm whether any guest response exposes `session_token`. Known open until rollout wave R6 — record, don't fail the release on it | | |

## Waiter (Frontend B, `http://localhost:3001/staff/login`, branch Saffron Bandra id=1, PIN 3333)

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| W-1 | Login | PIN 3333 + branch logs in to waiter view; wrong PIN rejected; repeated failures lock out temporarily | | |
| W-2 | Table board | Board shows live table states (free / active / payment_pending / awaiting_reactivation) matching reality; updates live as guests act | | |
| W-3 | Order lifecycle | Ready items can be marked served; guest tracker reflects served state | | |
| W-4 | Assistance | Incoming assistance requests appear live; ack and resolve both work and clear the guest indicator | | |
| W-5 | Cash settlement | On the seeded payment_pending table (T2), waiter can confirm/settle payment; session closes cleanly | | |
| W-6 | Scope | Waiter sees no admin tabs (menu management, stats, staff, settings) | | |
| W-7 | Cross-instance realtime | With guest on :3000 (app-1) and waiter on :3001 (app-2), events still arrive live in both directions | | |

## Kitchen (Frontend B, PIN 4444, branch 1)

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| K-1 | Login + KDS | PIN 4444 opens the kitchen display; confirmed orders appear (10 s polling — expect up to ~10 s delay, not instant) | | |
| K-2 | Advance states | Confirmed → preparing → ready transitions work per item/order; served is NOT offered (that's the waiter's action) | | |
| K-3 | Guest sync (F-6) | Each kitchen advance shows on the guest tracker without refresh | | |
| K-4 | Cancelled orders | A cancelled order disappears from/marks clearly on the KDS | | |
| K-5 | Scope | Kitchen sees no admin surfaces | | |

## Manager (Frontend B, PIN 2222, branch 1)

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| M-1 | Login + nav | PIN 2222 shows manager view: ops surfaces plus admin tabs (menu, tables, promos, stats, performance, loyalty, collateral, settings) | | |
| M-2 | Menu admin | Create/edit/disable a menu item (incl. a variant); change reflects on the guest menu | | |
| M-3 | Tables admin | Add/rename a table; QR token/collateral available; board reflects it | | |
| M-4 | Promos | Create a non-time-windowed promo; guest can apply it. (Time-windowed promos: known UTC bug — skip or note) | | |
| M-5 | Stats | Stats/analytics tabs render with seeded data; no crashes on empty ranges | | |
| M-6 | Loyalty / performance | Tabs render. Feature-gated (entitlement AND flag, default off) — verify a clean gated/empty state, not an error | | |
| M-7 | Settings | Branch settings render and persist edits | | |
| M-8 | Scope | Manager cannot manage staff roster (owner-only) | | |

## Owner (Frontend B, PIN 1111, branch 1 — owner exists only on each tenant's primary branch)

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| O-1 | Login | PIN 1111 shows owner view: everything manager has plus staff + org surfaces | | |
| O-2 | Staff roster | List staff; add a staff member; reset a PIN; removed/reset staff can't log in with old PIN | | |
| O-3 | Org view | Cross-branch org-level view lists all 3 Saffron branches with correct data separation | | |
| O-4 | Audit | Audit surface shows recent actions (audit v2 is ON in this env) with actor/action detail | | |
| O-5 | Appearance / plan | Theme/appearance and plan surfaces render; theme change reflects on guest UI | | |

## Platform Admin (`http://localhost:3001/platform/login` or :3000)

Credentials: `admin@platform.local` / `Platform!admin1` (super_admin) · `support@platform.local` / `Support!admin1` · `billing@platform.local` / `Billing!admin1` · `auditor@platform.local` / `Auditor!admin1` (no TOTP enrolled in this seed — login is password-only until you enroll)

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| P-1 | super_admin login | Login succeeds; console shows orgs (Saffron/Copper/Urban), branches, plans, themes | | |
| P-2 | Org/branch lifecycle | Inspect an org; create a test org/branch with initial owner + tables; new branch is loginable and QR-joinable | | |
| P-3 | Entitlements & flags | Entitlement and feature-flag surfaces render; staff-analytics/loyalty gates show default-off; flipping a flag changes the staff UI gate for that tenant | | |
| P-4 | Themes | Theme editor renders; tenant theme (dark-luxury / warm-cafe / vibrant) previews correctly | | |
| P-5 | Analytics & observability | Platform analytics and observability surfaces render with seeded data | | |
| P-6 | MFA enrollment | Enroll TOTP for a platform user; next login demands the code; recovery codes shown once | | |
| P-7 | support_admin | Read-only cross-tenant search/inspection works; no billing surfaces; actions audited | | |
| P-8 | billing_admin | Subscription/billing/invoice surfaces render (shadow mode — nothing enforced); no support console | | |
| P-9 | read_only_auditor | View-only observability/audit; every mutation UI hidden or rejected | | |
| P-10 | Cross-tenant isolation | Data from one tenant never appears in another tenant's staff/guest surfaces | | |

## Cross-tenant spot checks

| # | Item | Expected behaviour | Pass/Fail | Notes |
|---|---|---|---|---|
| X-1 | Copper Pot Kitchen (branch 4, Koramangala) | Guest join on a Copper table shows warm-cafe theme; staff PINs 1111–4444 work scoped to branch 4 | | |
| X-2 | Urban Brew Café (branch 5, Cyber Hub) | Vibrant theme; free/trial plan surfaces behave (no premium features) | | |
| X-3 | Cross-branch isolation | A Saffron staff login cannot see Copper/Urban data and vice versa | | |
