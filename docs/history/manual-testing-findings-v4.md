# Manual Testing Findings — v4 (Certification Pass)

**Date:** 2026-06-05 · **Env:** isolated `manual-testing` stack (app-1 :8090 / app-2 :8095, frontends :3000/:3001), 3 tenants (Saffron House / Copper Pot / Urban Brew). **Soak untouched.**
**Status:** investigation only — **no fixes applied, nothing committed.** Each item below was reproduced and root-caused against current code (file:line) and/or live API tests.

> Reporter found these during a partial (not exhaustive) manual pass. All 12 are **confirmed to exist**. Severity is my assessment for a pilot.

| # | Area | Issue | Layer | Severity | Verdict |
|---|------|-------|-------|----------|---------|
| 1 | Guest menu | Beverage modifier (e.g. Lassi Sweet/Salted) allows selecting both — should be single-select | Frontend | Med | Verified |
| 2 | Guest menu | Item image/thumbnail is on the right; wanted on the left | Frontend | Low | Verified |
| 3 | Guest | "Still here?" banner reappears ~every 30s | Frontend | Med | Reproduced; mechanism found |
| 4 | Guest | Landing shows 4 tiles + 4 nav tabs; want only Menu + Help until relevant | Frontend | Low | Verified (UX) |
| 5 | Guest | Promo entry appears at order placement; should be at the bill | Frontend (+backend) | Med | Verified |
| 6 | Guest | "Come back anytime" (phone+name) save does nothing | Backend gate / data | Med | Verified |
| 7 | Manager | Manager cannot add staff | Frontend RBAC | Med | Verified (FE/BE mismatch) |
| 8 | Manager | Plan/subscription tab: "No tenant context. Set NEXT_PUBLIC_TENANT_SLUG" | Frontend/config | Med | Verified |
| 9 | Manager | Promo form: date+time field plus a separate time field (confusing) | Frontend | Low | Verified (UX) |
| 10 | Manager/Guest | Promo created & "active" but cannot be used | Backend (timezone) | **High** | Verified |
| 11 | Guest | Checking in to Table 2 (Bandra): "Something went wrong" | Backend | **High** | Verified |
| 12 | Theme | Manager theme change shows on guest but not kitchen/waiter/owner | Frontend | Med | Verified |

---

## 1. Beverage modifier should be single-select (radio), not multi-select

**Reported:** On Lassi, options are Sweet / Salted; both can be selected. Should be radio.

**Found:** Modifier selection is pure toggle/multi-select everywhere.
- `frontend/app/(guest)/session/[id]/menu/page.tsx:353` `toggleModifier()` just adds/removes the id from an array.
- `frontend/components/shared/BeverageModifierGrid.tsx` — `BeverageModifierGroup` renders each modifier as an independent toggle button (`selected.includes(mod.id)`); `is_required` only renders a `*` marker.
- The "can add" guard (`menu/page.tsx:388`) only checks that each required group has **at least one** selection — never **at most one**.

**Root cause:** No concept of a single-select / mutually-exclusive modifier group. A required group (Sweet/Salted) is still additive.

**Fix (frontend):** In `toggleModifier`, when a modifier belongs to a required (or designated single-select) group, selecting one should **deselect the others in the same group**. Apply to both the beverage grid and the standard modifier list. (Data model already carries `modifier_group` + `is_required`; treat "required group" = single-select, or add an explicit `single_select` flag later.)

---

## 2. Menu item image is on the right; want it on the left

**Reported:** Images appear on the right of each menu row; should be on the left.

**Found:** `frontend/app/(guest)/session/[id]/menu/page.tsx` (~line 55–110). The row is a flex container whose **first child is the text block** (`flex:1`) and **second child is the image/thumbnail** (`flexShrink:0`) → renders on the right. When `image_url` is absent (seeded items have none), a decorative `<Vignette>` circle is shown in that right slot — that is the "image" seen.

**Fix (frontend):** Reorder the two children (image block first), or set `flexDirection: "row-reverse"` on the row. Keep the quantity badge anchored to the thumbnail.

---

## 3. "Still here?" banner reappears about every 30 seconds

**Reported:** The "are you still here?" popup comes back every ~30s.

**Found (mechanism, not yet a pinned single trigger):**
- Backend warner runs **every 5 minutes**, warns only sessions within 15 min of timeout, and marks `warned_at` so it warns **once** per session: `backend/cmd/server/main.go:91` (`RunSessionExpiryWarner(ctx, 5*time.Minute)`), `backend/internal/worker/worker.go:146+` (`warnExpiringSessions` + `MarkSessionWarned`). So the **backend does not emit every 30s.**
- Client banner `frontend/components/shared/SessionTimeoutBanner.tsx` is visible whenever `sessionExpiringAt` is set; it's set on the `SESSION_EXPIRING_SOON` WS event (`frontend/hooks/useWebSocket.ts:30`). There is **no dedup** — a repeat of the event (or a snapshot/missed-events replay on reconnect) re-sets `sessionExpiringAt` and re-pops the banner, even right after the user dismissed it.
- The WS ping/reconnect cadence is 30s (`frontend/lib/ws/connection.ts:12` `PING_INTERVAL_MS = 30_000`, backoff up to 30s), which matches the observed period.

**Likely root cause:** For a session that has gone idle (in this env presence grace is short, so sessions slip toward `awaiting_reactivation`), the client reconnects on a ~30s cycle and the `SESSION_EXPIRING_SOON` is re-delivered via snapshot/missed-events each time, re-showing the banner.

**Fix (frontend, + verify backend):**
1. Dedup on the client — ignore a `SESSION_EXPIRING_SOON` whose `expires_at` equals the one already shown/dismissed (track last-handled expiry).
2. Ensure reconnect snapshot/`missed_events` doesn't replay an already-consumed expiry warning.
3. Confirm `warned_at` is **not** cleared on reactivation (so the warner doesn't re-warn). 
*Needs a timed live repro to confirm exactly which of these fires; the dedup in (1) fixes the user-visible symptom regardless.*

---

## 4. Guest landing shows too many options

**Reported:** On guest check-in it shows View Menu, Orders, Help, (Bill/Table); should show only View Menu and Help initially.

**Found (by current design):**
- `frontend/app/(guest)/session/[id]/page.tsx` always renders **4 tiles**: View Menu, Orders, Need Help, Pay Bill.
- `frontend/components/layout/BottomNav.tsx` always renders **4 tabs**: Menu, Orders, Bill, Help (Bill is greyed via `muted: !hasOrders` but still shown). The top bar also shows the table chip.

**Fix (frontend):** Gate **Orders** and **Bill** on there being at least one order (the store already exposes `hasOrders` in `BottomNav`; the landing page can read `useOrdersStore`). Show only Menu + Help until the party has ordered, then reveal Orders/Bill.

---

## 5. Promo code should be entered at the bill, not at order placement

**Reported:** Promo input shouldn't appear when placing the order; it should appear when finalizing the bill.

**Found:** Promo UI lives on the **cart/order** page and is applied to the order:
- `frontend/app/(guest)/session/[id]/cart/page.tsx` — `promoCode`/`appliedPromo` state, `handleApplyPromo()`, and `handlePlaceOrder()` passes the promo into `placeOrder(...)`.
- `frontend/app/(guest)/session/[id]/payment/page.tsx` — **no promo references at all.**

**Fix:** Move the promo entry to the payment/bill page (`payment/page.tsx`). Note this is **not purely cosmetic** — today the discount is bound to the order at placement; applying it at the bill means the **backend bill/payment-initiation path must accept and apply the promo** (verify `POST /sessions/:id/payments` / bill computation supports a promo, or add it). Treat as a small frontend+backend change, not just moving a widget.

---

## 6. "Come back anytime" (phone + name) does nothing

**Reported:** The post-visit "come back anytime" prompt that asks for phone & name isn't working.

**Found (live):** `frontend/components/shared/CustomerOptIn.tsx` → `customersApi.link()` → `POST /sessions/:id/customer`. Live call returns:
```
HTTP 404 {"code":"FEATURE_DISABLED","message":"feature disabled for this restaurant"}
```
The backend gates it on a restaurant setting: `backend/internal/services/customer.go:62` reads `settings_json.customer_memory_enabled`; if absent/false it returns `ErrFeatureDisabled` (`:66`/`:69`). The seeded restaurants don't set it, so the save always fails; the guest just sees an error toast.

**Fix (choose one or both):**
- **Data/seed:** enable `customer_memory_enabled` in `restaurants.settings_json` for tenants that should have loyalty (and add it to `scripts/seed.go`).
- **Frontend:** don't show the `CustomerOptIn` sheet when the feature is disabled (resolve the flag and conditionally render), so guests aren't offered a form that always errors.

---

## 7. Manager cannot add staff

**Reported:** Logged in as manager, couldn't add a staff member.

**Found — frontend/backend RBAC mismatch:**
- **Backend is owner-only:** `backend/internal/handlers/staff.go` `CreateStaff` → `if sess.Role != sqlc.StaffRoleOwner { 403 "only owners can create staff" }`; central policy agrees: `backend/internal/authz/policy.go:121` `ActionStaffCreate/UpdateRole/Deactivate → owner` only.
- **Frontend over-permits:** `frontend/app/(staff)/staff/(dashboard)/admin/page.tsx:623` `canManage = role === "owner" || role === "manager"` shows the Add-Staff form to managers and calls `staffApi.createStaff` → backend 403.

**Root cause:** UI shows an action the backend forbids for managers. Design intent (backend + handler comment) is **owner-only** for create/role-change/deactivate.

**Fix (frontend):** Gate staff **create / role-change / deactivate** to `role === "owner"` (match `policy.go`). Managers may keep allowed ops (e.g. PIN update per `policy.go:123`). Alternatively, if managers *should* manage staff, that's a backend policy decision — but currently FE and BE disagree and the FE is wrong.

---

## 8. Plan/subscription tab: "No tenant context. Set NEXT_PUBLIC_TENANT_SLUG"

**Reported:** The plan page says "no tenant context, set NEXT_PUBLIC_TENANT_SLUG."

**Found:** `frontend/app/(staff)/staff/(dashboard)/admin/page.tsx:882` returns that message when `restaurantId` is null. `restaurantId` comes from `useTenant()` (`frontend/providers/TenantProvider.tsx`), which only resolves a tenant via `NEXT_PUBLIC_TENANT_SLUG` (`config/env.ts:7`) or a subdomain of `NEXT_PUBLIC_BASE_DOMAIN`. In local dev neither is set, so `resolveTenantSlug()` returns null → no `restaurantId` → the subscription/plan tab can't load.

**Fix (pick one):**
- **Quick (testing):** set `NEXT_PUBLIC_TENANT_SLUG=saffron-house` for frontend-a (and `copper-pot-kitchen` / `urban-brew-cafe` for other instances). Caveat: this scopes the whole app to one tenant, so it's a per-instance workaround, not multi-tenant-friendly.
- **Proper (product):** in the **staff** admin, derive the restaurant/tenant from the **logged-in staff's branch** (the staff session already carries branch/org) instead of the public tenant slug. Then the plan tab works without `NEXT_PUBLIC_TENANT_SLUG`.

---

## 9. Promo form has a date+time picker *and* a separate time field

**Reported:** When creating a promo, you can set date+time in one input, but there's another separate time field too.

**Found (by design, but confusing):** `frontend/app/(staff)/staff/(dashboard)/admin/page.tsx`:
- "Valid from" / "Valid until" → `type="datetime-local"` (date **and** time) — the **validity period**.
- "Time window start (HH:MM)" / "Time window end (HH:MM)" → `type="time"` — an **optional daily active window** (e.g. happy hour 16:00–19:00).

They are different concepts, but the form doesn't make that obvious, so it reads as a duplicate. **Also note the daily window is currently broken — see #10.**

**Fix (frontend):** Group and relabel clearly, e.g. a "Validity period" section (from/until) and a separate, clearly-optional "Daily active hours (optional)" section with helper text. Consider hiding the window behind a toggle so it isn't set by accident.

---

## 10. Promo created and shows "active" but cannot be used — **timezone bug**

**Reported:** After creating a promo (shows active), a guest cannot use it.

**Found (live + query):** The lookup is `backend/internal/db/sqlc/promos.sql.go` `getPromoByCodeForUpdate`:
```sql
WHERE branch_id=$1 AND LOWER(code)=LOWER($2) AND is_active=TRUE
  AND valid_from <= now() AND valid_until >= now()
  AND (time_window_start IS NULL OR (time_window_start <= LOCALTIME AND LOCALTIME <= time_window_end))
```
- A **no-window** promo validates fine — proven live: created `TEST20` (percentage 20, no window) → `POST /sessions/:id/promos/validate` → **HTTP 200**.
- The DB runs in **UTC**: `SHOW timezone → UTC`; `SELECT LOCALTIME → 08:33` while the Bandra/IST wall clock was **14:03** (UTC+5:30).
- So a promo with a **daily time window** is compared against the **DB server's UTC `LOCALTIME`**, not the branch's local time. A window entered as the operator's local hours (e.g. 16:00–19:00 IST) will essentially never match the UTC `LOCALTIME` → the lookup returns no row → guest sees "this promo code isn't valid right now," even though it shows **active**.

**Root cause:** Time-window comparison uses DB-server `LOCALTIME` instead of the **branch timezone**. (Combined with #9, operators are nudged into setting a window, which then silently breaks the promo.)

**Fix (backend):** Compare the daily window against the **branch's local time**, not `LOCALTIME`. Either compute the branch-local time in Go and pass it as a parameter, or convert in SQL using the branch timezone (the codebase already derives branch-local time via `branchBusinessDate` in `internal/services/operational_ids.go`). Until fixed, document that promo time-windows must be entered in **UTC**, or leave the window blank.

*Side observation (separate, low priority):* the validate response returned `discount_amount: 0` for `order_total: 500` in my raw API test — likely because my test payload's total field wasn't what the handler reads. The real UI passes the cart total; worth a quick check that the validate request field name (`order_total`) matches the handler binding.

---

## 11. Checking in to Table 2 (Bandra): "Something went wrong" — **predicate mismatch**

**Reported:** Tried to check in at Table 2 of Bandra (Saffron House) → "something went wrong."

**Found (live + code) — this is a real, general bug:**
- Table 2 has a session in **`payment_pending`** (seeded; persists because pending-payment sessions are protected from abandonment).
- QR resolve `GET /tables/by-qr/:token` returns the active session via `GetActiveSessionForTable`, whose query filters **`status = 'active'` only** (`backend/internal/db/sqlc/sessions.sql.go`, used by `internal/services/menu.go:101`). For a `payment_pending` table it returns **no** session → resolve response has empty `session_id`.
- The guest page `frontend/app/(guest)/table/[token]/page.tsx` then takes the **create** path (since `session_id` is empty). Creating a session is blocked by the unique partial index `idx_sessions_one_active_per_table`, which covers **`active`, `payment_pending`, `awaiting_reactivation`** → `409 SESSION_ALREADY_ACTIVE`, surfaced to the guest as the generic "Something went wrong. Please try again."
- Proven live: resolve T2 → `session_id` empty; `POST /sessions {table_id:2}` → `HTTP 409 SESSION_ALREADY_ACTIVE`.

**Why it matters broadly:** the same dead-end happens for **any** table whose session is `payment_pending` **or `awaiting_reactivation`** — i.e. any table a party has idled on. The "joinable active session" predicate is **narrower** than the "table is occupied" constraint, leaving a gap where a guest can neither join nor create.

**Fix (backend):** Make the resolve→join lookup cover the same statuses the create-constraint blocks. Options:
- Broaden `GetActiveSessionForTable` to `status IN ('active','payment_pending','awaiting_reactivation')` and let the guest **join** the in-progress session (preferred — the party is still there), **or**
- Return a clear state to the frontend ("this table is settling up / paused") instead of forcing the create path, so the UI shows a meaningful message rather than "something went wrong."

---

## 12. Theme change reflects on guest but not kitchen/waiter/owner screens

**Reported:** Manager changed the theme; the guest screen updated, but kitchen, waiter, and owner screens kept the old theme.

**Found:** The theme is applied **per guest session**, never on staff routes:
- Guest: `frontend/app/(guest)/table/[token]/page.tsx:41-45` and `frontend/providers/SessionProvider.tsx:41` call `applyTheme(themeApi.resolveForBranch(branchId))` → sets `data-theme` + tokens on `<html>`.
- The global `TenantProvider` only resolves a theme when `NEXT_PUBLIC_TENANT_SLUG`/subdomain is set (see #8), which it isn't in local dev — so the app-wide theme path is inert.
- **Staff** dashboards (`app/(staff)/...`) never call `themeApi.resolveForBranch` / `applyTheme`, so they render with the default/persisted `next-themes` value (dark-luxury) and don't react to a branch theme change.

**Fix (frontend):** In the staff dashboard layout (`app/(staff)/staff/(dashboard)/layout.tsx`), resolve and apply the theme for the **logged-in staff's branch** (the staff session carries branch id), mirroring `SessionProvider`. Re-apply when the theme is changed so kitchen/waiter/owner reflect it too.

---

## Cross-cutting themes

- **Frontend↔backend contract drift** (the project's recurring lesson): #7 (RBAC), #5 (promo placement), #6 & #11 (UI offers an action the backend gate/predicate refuses). The fixes are mostly aligning the FE to existing BE rules.
- **Timezone correctness:** #10 is a genuine backend correctness bug (UTC vs branch tz) and is **High** — it makes a whole feature (time-windowed promos) silently fail.
- **Occupied-but-not-active tables:** #11 is **High** and broad — it breaks check-in for any idled/paying table.
- **Local-dev tenant resolution:** #8 and the inert global theme path stem from no `NEXT_PUBLIC_TENANT_SLUG`; the durable fix is deriving tenant from the staff branch rather than the public slug.

## Notes on the test environment after this investigation
During verification I created a `TEST20` promo on Bandra and a few throwaway guest sessions, and closed one Bandra T3 session to free a table. None of this affects the soak. To return to a pristine seeded state at any time: `./scripts/manual-testing-up.sh --reset`.

*Recommend re-testing items not yet covered by the reporter (kitchen lifecycle edge cases, payment settlement, multi-instance fan-out, cross-tenant isolation, MFA, support console) before sign-off.*
