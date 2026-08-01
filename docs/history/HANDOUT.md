# Handout

Session-state handoff log. Each section below captures a single session's state so a future Claude can resume from where the previous one stopped.

The most recent session is at the bottom.

---

## Session — 2026-05-21 22:14 IST

**Status:** paused mid-task — interrupted while updating handler files  
**If confused, start here:** Read `/home/mohith/.claude/plans/plans-phase-5-audit-logging-v2-md-this-twinkly-candy.md` first, then read `backend/internal/audit/writer.go` to understand the Writer API, then pick up at **Next concrete action** below.

---

### Original goal

Implement Phase 5 — Audit Logging V2 from `plans/phase-5-audit-logging-v2.md`. The user also requested 8 architecture improvements incorporated into both the phase plan and the implementation:

1. `result` enum field (`success`, `failure`, `denied`)
2. `correlation_id` text field (separate from `request_id`)
3. `source` enum field (`web`, `mobile`, `pwa`, `api`, `webhook`, `system`)
4. Hardened audit failure visibility (Prometheus counter `AuditWriteFailuresTotal`)
5. Standardized action naming (`resource.verb` pattern)
6. Future tamper-evident chaining placeholders (`row_hash`, `previous_hash` — NULL in this phase)
7. Security-focused audit tests (denied authz, login failure, redaction, correlation, org isolation, self-auditing)
8. Audit retention + partitioning notes (documented, not implemented)

After all code changes: run tests, verify build, then commit with a simple message and no co-authors.

---

### Plan

From the implementation plan file at `/home/mohith/.claude/plans/plans-phase-5-audit-logging-v2-md-this-twinkly-candy.md`:

- [x] Group 1: Migration 000019 (schema, 4 enums, trigger, indexes) — DONE
- [x] Group 2: SQL query file + `go tool sqlc generate` — DONE
- [x] Group 3: `backend/internal/audit/` package (event.go, redaction.go, middleware.go, writer.go) — DONE
- [x] Group 4: `AuditWriteFailuresTotal` metric added to `observability/metrics.go` — DONE
- [x] Group 5: `backend/internal/repository/audit_log.go` repo wrappers — DONE
- [ ] Group 6: Server wiring (`server.go` — Writer construction, `audit.Middleware()` registration, updated handler constructors, new routes)
- [ ] Group 7: Handler event wiring — **PARTIALLY DONE** (see below)
- [ ] Group 8: Visibility read endpoints (`GET /branches/:id/audit`, `GET /orgs/:org_id/audit`, platform `ListAudit` extension)
- [ ] Group 9: Tests (redaction unit tests + integration tests)
- [ ] Final: `go build ./...` verification, test run, commit

---

### Done this session

**Plans updated:**
- `plans/phase-5-audit-logging-v2.md` — updated Target Audit Model, Audit Writer rules, Implementation Status, Test Requirements with all 8 improvements
- `/home/mohith/.claude/plans/plans-phase-5-audit-logging-v2-md-this-twinkly-candy.md` — full implementation plan with all 8 improvements; Implementation Status section tracks progress

**Schema + generated code:**
- `backend/migrations/000019_audit_log_v2.up.sql` — created: 4 enums (`audit_actor_type`, `audit_risk_level`, `audit_result_type`, `audit_source_type`), `audit_log` table with 24 columns, immutability trigger `trg_audit_log_immutable`, 4 indexes
- `backend/migrations/000019_audit_log_v2.down.sql` — created
- Migration applied to local DB ✓ (`go run cmd/migrate/main.go up`)
- `backend/sql/queries/audit_log.sql` — created: `InsertAuditLog :exec`, `ListAuditLogForBranch :many`, `ListAuditLogForOrganization :many`, `ListAuditLogPlatform :many`
- `go tool sqlc generate` run — generated `backend/internal/db/sqlc/audit_log.sql.go` ✓, `models.go` updated with new enums + `AuditLog` struct ✓

**Audit package (`backend/internal/audit/`):**
- `event.go` — `AuditEvent` struct, `ActorType`/`RiskLevel`/`ResultType`/`SourceType` types + constants, resource constants, action constants (all `resource.verb` style)
- `redaction.go` — recursive `Redact(before, after)` function, `isSensitive()` key checker
- `middleware.go` — `AuditRequestContext`, `Middleware()` gin handler, `FromContext()`, `inferSource()` from User-Agent/X-Source header
- `writer.go` — `Writer` struct, `NewWriter()`, `Record()` (feature-flag gated, redacts before insert, logs+increments counter on failure), `MustJSON()`, `IDStr()`, `optInt8()`, `optUUID()`, `toJSON()`, `errorClass()`

**Observability:**
- `backend/internal/observability/metrics.go` — added `AuditWriteFailuresTotal *prometheus.CounterVec` (labels: `action`, `error_class`), registered in `NewMetrics()`

**Repository:**
- `backend/internal/repository/audit_log.go` — created: `LogAuditV2()`, `ListAuditLogForBranch()`, `ListAuditLogForOrganization()`, `ListAuditLogPlatform()`

**Handlers — partially updated:**
- `backend/internal/handlers/authz.go` — `requireAuthorized()` now accepts `*audit.Writer` as 4th param; writes `authz.denied` event with `ResultDenied`, `RiskMedium` on denial
- `backend/internal/handlers/staff.go` — added `audit *audit.Writer` field + updated constructor; events wired: `staff.login`, `staff.login.failed`, `staff.create`, `staff.pin.reset`, `staff.deactivate`; `requireAuthorized` calls updated
- `backend/internal/handlers/branches.go` — added `audit *audit.Writer` field + updated constructor; `branch.settings.update` event wired; tracks `updatedFields` slice; `c.Status` call reached but audit.Record not yet confirmed working (session was paused before build verification)
- `backend/internal/handlers/tables.go` — added `audit *audit.Writer` field + updated constructor; `qr_token.rotate` event wired
- `backend/internal/handlers/session.go` — added `audit *audit.Writer` field + updated constructor; `session.create` and `session.close` events wired
- `backend/internal/handlers/menu_admin.go` — added `audit *audit.Writer` field + updated constructor; all `requireAuthorized` calls updated to pass `h.audit`; **NO audit.Record() event calls added yet** (this was next)

**Handlers — NOT YET UPDATED:**
- `backend/internal/handlers/order.go` — still has old `requireAuthorized` signature (missing `h.audit` param); no `audit` field
- `backend/internal/handlers/assistance.go` — same as order.go
- `backend/internal/handlers/promo.go` — same
- `backend/internal/handlers/organization.go` — same
- `backend/internal/handlers/customer.go` — has old `requireAuthorized` signature; no `audit` field; no event calls
- `backend/internal/handlers/payment.go` — no `audit` field; no event calls
- `backend/internal/handlers/platform.go` — no `audit` field; no event calls
- `backend/internal/handlers/audit_log.go` — **not created yet**

**Server wiring — NOT started:**
- `backend/internal/server/server.go` — still uses old constructor signatures for all handlers; `audit.Writer` not constructed; `audit.Middleware()` not registered; new audit routes not added

---

### Left to do

1. **Update remaining handlers** — add `audit *audit.Writer` field + update constructors for: `order.go`, `assistance.go`, `promo.go`, `organization.go`, `customer.go`, `payment.go`, `platform.go`
2. **Update `requireAuthorized` call sites** — in: `order.go`, `assistance.go`, `promo.go`, `organization.go`, `customer.go` (pass `h.audit`)
3. **Add `audit.Record()` calls** — in: `menu_admin.go` (CreateCategory, CreateItem, UpdateItem, DeleteMenuItem, UpdateMenuCategory, DeleteMenuCategory), `customer.go` (LinkCustomer, DeleteCustomer), `payment.go` (InitiatePayment), `platform.go` (Authenticate failure, CreateSupportSession)
4. **Update `server.go`** — construct `auditWriter`, register `audit.Middleware()`, update ALL handler constructors, add `GET /branches/:id/audit` and `GET /orgs/:org_id/audit` routes
5. **Create `backend/internal/handlers/audit_log.go`** — `AuditLogHandler`, `GetBranchAuditLog`, `GetOrgAuditLog`
6. **Extend `platform.go` `ListAudit`** — also query `repos.ListAuditLogPlatform` and merge results
7. **Write tests** — `backend/internal/audit/redaction_test.go` (unit), `backend/internal/repository/audit_log_integration_test.go` (integration)
8. **Build verification** — `go build ./...` must be clean
9. **Run tests** — `go test ./internal/audit/... ./internal/repository/... -run TestAudit -v`
10. **Commit** — simple message, no co-authors

**Next concrete action:** In `backend/internal/handlers/order.go`, add `audit *audit.Writer` field to `OrderHandler` struct and update `NewOrderHandler` constructor to accept `auditWriter *audit.Writer`. Then update the `requireAuthorized` call at line 170 to pass `h.audit`. Repeat for `assistance.go`, `promo.go`, `organization.go`, `customer.go`, `payment.go`, `platform.go`. Then update `server.go`.

---

### Decisions made

- **`ip` stored as TEXT, not INET** — avoids non-standard sqlc type override for PostgreSQL INET. No FK constraints on `audit_log` — rows must survive org/branch deletion.
- **Pool-backed `audit.Writer`** — uses `sqlc.New(pool)` not a tx-backed querier. Audit writes are post-commit fire-and-forget (same pattern as `LogEvent`/`LogPlatformAudit`).
- **DB trigger for immutability** — `trg_audit_log_immutable` raises exception on any UPDATE or DELETE. DB-level, not just application convention.
- **`Redact()` called inside `Writer.Record()`** — callers pass raw state; writer scrubs it. Callers do not need to think about redaction.
- **`requireAuthorized` receives `*audit.Writer`** — uniform authz denial auditing across all handlers without duplicating the write at each call site.
- **`MustJSON` exported from audit package** — handlers call `audit.MustJSON(...)` for Before/After fields.
- **`IDStr` exported from audit package** — handlers call `audit.IDStr(id)` for string-formatted IDs.
- **`result`, `correlation_id`, `source` fields** — all added to schema and `AuditEvent`. `correlation_id` read from `X-Correlation-ID` header in `audit.Middleware()`.
- **Simple commit message, no co-authors** — user's explicit rule (from memory).

### Rules the user gave

- Simple commit messages, no co-authors in any commit.
- Plan file (`plans/phase-5-audit-logging-v2.md` and the `.claude/plans/` file) must be updated with implementation status as work progresses, but the plan file itself is NOT committed.
- Maintain clean architecture, proper version control, small logical commits.
- After all changes are done: run migrations, sqlc generate, backend build, unit tests, integration tests, verify feature flag behavior, verify immutability trigger, verify org isolation, verify audit read self-auditing, verify redaction correctness. Then stop.

### Open questions / blockers

- None from user. The sed command to bulk-update `requireAuthorized` calls was the in-flight task when the session was interrupted (user rejected it for unknown reason — possibly to trigger the handout instead).
- The session interruption happened with `sed -i` being sent. That command was rejected. The files `order.go`, `assistance.go`, `promo.go`, `organization.go`, `customer.go` still have the old `requireAuthorized` signature (`c, h.repos, h.authz, actor` — missing `h.audit`). **Do not use `sed -i` to make these changes; use the Edit tool instead.**

### Files touched

- `plans/phase-5-audit-logging-v2.md` — modified (Target Audit Model, Audit Writer, Implementation Status, Test Requirements sections updated)
- `/home/mohith/.claude/plans/plans-phase-5-audit-logging-v2-md-this-twinkly-candy.md` — created/modified (full implementation plan)
- `backend/migrations/000019_audit_log_v2.up.sql` — created
- `backend/migrations/000019_audit_log_v2.down.sql` — created
- `backend/sql/queries/audit_log.sql` — created
- `backend/internal/db/sqlc/audit_log.sql.go` — generated (do not edit)
- `backend/internal/db/sqlc/models.go` — generated (updated with new enums)
- `backend/internal/db/sqlc/querier.go` — generated (updated with new query interfaces)
- `backend/internal/audit/event.go` — created
- `backend/internal/audit/redaction.go` — created
- `backend/internal/audit/middleware.go` — created
- `backend/internal/audit/writer.go` — created
- `backend/internal/observability/metrics.go` — modified (added `AuditWriteFailuresTotal`)
- `backend/internal/repository/audit_log.go` — created
- `backend/internal/handlers/authz.go` — modified (new `requireAuthorized` signature + authz.denied event)
- `backend/internal/handlers/staff.go` — modified (audit field, constructor, events, updated requireAuthorized calls)
- `backend/internal/handlers/branches.go` — modified (audit field, constructor, branch.settings.update event, `updatedFields` tracking)
- `backend/internal/handlers/tables.go` — modified (audit field, constructor, qr_token.rotate event)
- `backend/internal/handlers/session.go` — modified (audit field, constructor, session.create + session.close events)
- `backend/internal/handlers/menu_admin.go` — modified (audit field, constructor, requireAuthorized calls updated — **no Record() calls added yet**)
- `backend/internal/server/server.go` — NOT YET modified (still uses old constructor signatures)

**NOT yet modified (still need changes):**
- `backend/internal/handlers/order.go`
- `backend/internal/handlers/assistance.go`
- `backend/internal/handlers/promo.go`
- `backend/internal/handlers/organization.go`
- `backend/internal/handlers/customer.go`
- `backend/internal/handlers/payment.go`
- `backend/internal/handlers/platform.go`

**Not yet created:**
- `backend/internal/handlers/audit_log.go`
- `backend/internal/audit/redaction_test.go`
- `backend/internal/repository/audit_log_integration_test.go`

### Environment state

- **`pwd`**: `/home/mohith/Development/projects/qr-dining/backend`
- **`git branch`**: `* main`
- **`git status`**: See above — many modified + untracked files, nothing staged
- **`git log --oneline -10`**:
  ```
  f69efde Implement phase 3 organization tenancy model
  2b0d36e Implement phase 2 RBAC ownership validation
  2367f16 Add phase 1 identity hardening
  73a47ea Add phase 0 backend guardrails
  df3d178 fix: correct backup retention default and Dockerfile base image
  ```
- **Migration applied**: `000019_audit_log_v2` is applied to local DB (`postgres://qrdining:changeme_strong_password@localhost:5432/qrdining`)
- **`sqlc generate` run**: `audit_log.sql.go` is up to date with current SQL queries
- **Code does NOT build yet** — `server.go` still calls old handler constructors; `order.go`, `assistance.go`, `promo.go`, `organization.go`, `customer.go` still have old `requireAuthorized` call signatures

### Notes for next session

- The `audit.Writer` is constructed with `audit.NewWriter(q sqlc.Querier, enabled bool, logger zerolog.Logger, failureCounter *prometheus.CounterVec)`. In `server.go`, use `sqlc.New(db)` where `db` is the `*pgxpool.Pool` (same as how repos is constructed).
- The `audit.Middleware()` must be registered AFTER `middleware.RequestID()` in `server.go` so the request ID is available in context when Middleware runs.
- For handlers like `order.go` and `assistance.go` that call `requireAuthorized` but don't have dedicated audit events planned, they still need the `audit *audit.Writer` field so they can pass it to `requireAuthorized`. Just adding the field + updating constructor + updating the call site is sufficient — no `Record()` calls needed in those handlers.
- The `menu_admin.go` needs `Record()` calls added for: `CreateCategory` (after `c.JSON`), `CreateItem` (after `c.JSON`), `UpdateItem` (after `c.Status`), `DeleteMenuItem` (after `c.Status`), `UpdateMenuCategory` (after `c.Status`), `DeleteMenuCategory` (after `c.Status`).
- Watch out: `branches.go` has the `updatedFields` slice collected BEFORE the mutations run (it checks `if req.X != nil`), but the mutations themselves are conditional too. If a validation fails mid-handler (e.g., bad tax rate), the handler returns early. The audit event is written at the end of the handler only if all mutations succeeded — this is correct behavior. However, `updatedFields` was added before any `if req.X != nil` checks are processed, which means it lists ALL requested fields, not just those that were actually applied. This is acceptable (it's a log of intent), but should be noted.
- The test for audit read self-auditing (`TestAuditLogAuditReadSelfAudit`) will need the full HTTP handler test setup — may want to simplify to just calling the repo method directly and asserting the event gets written.
- `testutil.OpenTestDB` and `testutil.TruncateTables` are the test infrastructure helpers (used in `platform_integration_test.go` and `order_integration_test.go`).

## Session — 2026-06-13 16:50 IST

**Status:** paused mid-task — partway through Phase 0 (bug fixes), working tree is CLEAN (all completed work committed). Stopped after committing C12–C14; next up is C16.
**If confused, start here:** Read the approved plan at `/home/mohith/.claude/plans/staff-performance-analytics-noble-mitten.md` (it was overwritten this session — it now describes "Certification Bug Fixes + Full UI Redesign", NOT the old staff-analytics plan despite the filename). Then read this section's **Next concrete action**.

### Original goal

User did a full manual certification pass and found ~18 bugs, then asked for **two workstreams**:
1. **Fix all the bugs** (Phase 0).
2. **Full UI redesign** (Phases 1–6) — guest pages restyled after the "Serene Hospitality" reference in `samples/guest-screens/stitch_lumina_hospitality/`; staff + platform pages restyled after `samples/guest-staff-harmony/` (a Lovable/TanStack reference app) **while keeping our dedicated kitchen/waiter/admin/platform page structure** (the reference's single-staff-page layout is explicitly NOT wanted — "I need just the UI design from them, not the overall page structure").

User's verbatim bug list covered: 30s "are you still here?" popup; landing should show View Menu / Need Help + featured carousel; menu image side (defer); beverage radio takes both inputs; promo should be at the bill (with phone prompt) not the cart; kitchen shows order ID but no item contents; no image upload + no edit-modifier in admin; featured carousel on landing; staff pages don't keep theme (resets to dark on refresh); customer name/phone at bill throws "something went wrong"; requested bill not shown / "something went wrong"; QR print PNG/SVG/PDF only shows bare QR no design; collateral form goes inactive after every keystroke; staff tab needs staff list + PIN reset; plan tab says "tenant slug not enabled"; platform missing staff-performance analytics; QR codes point at localhost:8090 (backend) not the frontend.

### Plan (the approved plan file is authoritative — `/home/mohith/.claude/plans/staff-performance-analytics-noble-mitten.md`)

**Branch:** `feature/certification-fixes-ui-redesign` (off `feature/staff-analytics-loyalty` @ e3da57b).

Phase 0 = ~20 commits (C1–C20). Phases 1–6 = the redesign. Phase 7 = e2e reconciliation (deferred).

Phase 0 progress (commit hashes in Done-this-session):
- [x] C1 presence heartbeat (the 30s popup root cause)
- [x] C2 kitchen cards render items
- [x] C3–4 promo display (`[object Object]`) + branch-timezone window fix
- [x] C5–6 promo moves to payment initiation (backend)
- [x] C7 promo UI moves cart→payment (frontend)
- [x] C8–9 modifier single_select + PATCH /menu/modifiers/:id (backend)
- [x] C10 modifier radio behavior + admin edit UI (frontend)
- [x] C11 GET/PATCH /branches/:id += restaurant_id + customer_memory_enabled (backend)
- [x] C15 GET /branches/:id/staff + POST /staff/:id/pin/reset (backend) — done early, with C11
- [x] C12 FEATURE_DISABLED message + opt-in gating (frontend)
- [x] C13 bill page retry + Try-again button (frontend)
- [x] C14 NEXT_PUBLIC_GUEST_URL for QR origin (frontend)
- [ ] **C16 staff list + PIN-reset UI in admin StaffTab; "Change my PIN" in StaffBar; hide create-staff from managers** ← NEXT
- [ ] C17 Plan tab uses GET /branches/:id → restaurant_id (drop useTenant dependency)
- [ ] C18 Platform staff-performance analytics section on the analytics page
- [ ] C19 small fixes: hoist TextField/Toggle out of CollateralConfigForm body (focus loss); wire ImageUploadField into MenuItemModal for image_url (graceful 503 when R2 unconfigured); theme-aware Avatar colors
- [ ] C20 OpenAPI 2.2.0 + `npx @redocly/cli lint` clean
- [ ] Then **Phase 0 verification on the mtest stack** (rebuild mtest with THIS branch first!)

### Done this session (all committed; 9 commits)

```
82d039c fix guest QR origin, customer opt-in gating, and bill resilience   (C12,C13,C14)
c9004ec add branch customer-memory toggle, restaurant_id, staff roster and PIN reset (C11,C15)
eff40f9 support single-select modifier groups and modifier editing in the UI (C10)
72a184b add single-select modifier groups and a modifier edit endpoint       (C8,C9)
6fce4de move promo entry from cart to the bill on the guest UI               (C7)
53d00fb move promo application from order placement to payment initiation    (C5,C6)
ebfca68 fix promo time windows: branch-local comparison and HH:MM display    (C3,C4)
f1fc0b1 render order items on kitchen tickets                                 (C2)
0a9f5b6 wire presence heartbeat into websocket ping path                     (C1)
```

**Migrations added:** `000036_promo_redemption_payment` (promo_redemptions.payment_id, order_id nullable, partial unique idx), `000037_modifier_single_select` (item_modifiers.single_select). Schema is at **v37**. Both round-trip down/up cleanly (verified).

**New/changed backend routes:** `POST /sessions/:id/payments` now accepts `promo_code` + `phone_e164` (and `POST /sessions/:id/orders` no longer takes a promo); `PATCH /menu/modifiers/:id`; `GET /branches/:id/staff`; `POST /staff/:id/pin/reset`; `GET /branches/:id` += `restaurant_id` + `customer_memory_enabled`; `PATCH /branches/:id` += `customer_memory_enabled`. New error codes `MODIFIER_CONFLICT` (422). New authz actions `ActionStaffPinReset`, `ActionStaffListRead` (owner+manager).

**New backend tests (all green on throwaway DB):** `promo_window_integration_test.go`, `payment_promo_integration_test.go`, `presence_refresh_test.go` (websocket unit), plus `TestCart_SingleSelectModifierConflict`.

### Left to do

**Immediate (finish Phase 0):** C16, C17, C18, C19, C20, then mtest verification. Details in the plan file.

**Then the big effort — Phases 1–6 (UI redesign), all not started:**
- Phase 1 (C21–24): design foundation. Migration 000038 seeds `serene` into `theme_presets` + sets it as default; rebuild `frontend/styles/themes.css` (serene = new default preset, re-derive dark-luxury/warm-cafe/vibrant on the SAME 14 token keys so backend theme governance is untouched); new `styles/surfaces.css` static `[data-surface=ops|platform]` layer; `next/font` (Hanken Grotesk for guest; Inter Tight + Instrument Serif + JetBrains Mono for staff/platform); new `components/ds/` primitives.
- Phase 2 (C25–30): guest pages (welcome+carousel, menu+item sheet, cart/review, track, bill/pay, assist).
- Phase 3 (C31–33): staff shell + kitchen KDS + waiter floor.
- Phase 4 (C34–38): **split the 2015-line `admin/page.tsx`** into per-tab files (pure-move commit first!), restyle, fix collateral print fidelity (absorbs the QR-print bug). AppearanceTab is deleted; its guest-theme picker moves into SettingsTab.
- Phase 5 (C39–41): platform redesign + actionable custom-token UX.
- Phase 6 (C42): public landing + pricing refresh (low priority).
- Phase 7: e2e reconciliation (~106 Playwright specs will break progressively — keep an `e2e-impact.md`, full pass deferred).

**Next concrete action:** Start C16 — rebuild the StaffTab in `frontend/app/(staff)/staff/(dashboard)/admin/page.tsx` (around line 618) to fetch and render the staff roster via a new `staffApi.listStaff(branchId, token)` (GET `/branches/:id/staff`, returns `{staff: [...]}`), with a per-staff "Reset PIN" modal calling a new `staffApi.resetPin(staffId, branchId, newPin, token)` (POST `/staff/:id/pin/reset`, body `{new_pin}`), a deactivate action (owner-only), and hide the create-staff form from managers (`canManage` already exists but the backend create is owner-only — gate it on `role === "owner"`). Add a "Change my PIN" entry in `frontend/components/staff/StaffBar.tsx` using the existing `PATCH /staff/:id/pin` (body `{current_pin, new_pin}`). Add the two methods to `frontend/lib/api/staff.ts`.

### Decisions made (this session + locked via AskUserQuestion)

- **Staff/platform = unified ops design, NOT tenant-branded** (tenant brand = logo/name only). This structurally eliminates the "staff theme resets to dark" bug — staff pages will get fixed ops tokens, never call `applyTheme`.
- **Guest themes rebuilt on the new Serene design**: `serene` becomes the new default preset; the other 3 presets re-derived on the same components/token keys. Platform theme governance + the 14-token custom system keep working unchanged.
- **Promo moves fully to payment initiation** (a sanctioned payment-path change — the only payment-lifecycle change allowed). Validated in the handler BEFORE any state mutation (so a bad code never freezes the session or burns the idempotency key); discount folded into the immutable bill snapshot; redemption recorded under the existing FOR-UPDATE lock; idempotency hash includes the promo code.
- **PIN reset = manager/owner reset (new endpoint, no current_pin) + self-change (existing PATCH).** Manager may reset only waiter/kitchen; owner anyone in branch; cannot reset self via the reset path. Target's tokens are invalidated on reset. No recovery codes this phase.
- **Presence wiring:** `PresenceRefresher` func is setter-injected into the websocket `Hub` (`SetPresenceRefresher` in server.go) to avoid a websocket→services import cycle — mirrors the existing `SetHostAuthority`/`SetLoyaltyAccrual` pattern. Heartbeat fires on every client PING (30s) + at WS upgrade; DB `last_seen_at` write is throttled via Redis SETNX (≤1/2min). `ReactivateSession` also clears `warned_at`.
- **single_select semantics:** a `(item, modifier_group)` group is single-select if ANY of its rows carry the flag. The admin UI writes it group-wide; the cart validates server-side (422 MODIFIER_CONFLICT).
- **QR guest origin:** `NEXT_PUBLIC_GUEST_URL` env with `window.location.origin` fallback — the fallback automatically fixes the mtest :8090 bug since the frontend serves at :3000/:3001.
- **`order.go` keeps the `promoSvc` constructor param as `_`** (dead but avoids churning the call sites/test helpers).

### Ruled out / dead ends (save the next session time)

- **`TestManualPaymentRequiresStaffSettlementAndRejectsStaleSnapshot` is a PRE-EXISTING `-tags integration` failure** — it fails on the clean baseline too (verified by stashing). Its `PlaceOrder` after `payment_pending` is blocked by the frozen-cart invariant. Do NOT think our promo changes broke it. Other known pre-existing `-tags integration` failures: `session_business_date` NULL inserts, `TestCreateSession_ConcurrentSingleActiveSession`.
- **gopls inline diagnostics go stale after `make sqlc-generate`** (they'll claim regenerated fields/methods are "undefined"). Trust `go build ./...`, not the diagnostics panel.
- The `isErr()` test helper does **string equality**, not `errors.Is` — use `errors.Is` directly for `%w`-wrapped errors.
- Integration tests creating sessions on the shared fixture table must close the prior session first (`UPDATE sessions SET status='closed' ... WHERE table_id=$1 AND status NOT IN ('closed','abandoned','expired')`) — the one-active-per-table unique index.
- Promo idempotency at payment is **session-scoped**; a true replay test must reuse the SAME session + key (a new session with the same key is a fresh payment, not a replay).

### Rules the user gave (this + carried over)

- **No co-authors / no AI attribution in commits** (from memory `feedback_commits.md`).
- Small atomic commits; verify each (`go build`/`vet`/sqlc-no-drift/integration; frontend `tsc`+`next build`).
- **Never touch the R1 soak** (docker project `qr-dining`, container `qr-app-soak`). Use the isolated mtest stack / throwaway DBs only.
- Additive migrations only; do not change session/websocket/auth/authz semantics — the ONE sanctioned exception is the promo-at-payment change.
- Keep our dedicated staff page structure in the redesign; take only the visual design from the harmony reference.
- The plan file is the source of truth; it is the only file editable during planning (now approved, so we're executing).

### Open questions / blockers

- None blocking. The remaining Phase-0 items (C16–C20) and the entire redesign (Phases 1–6) are well-specified in the plan.
- The redesign is large; the user may want to checkpoint/review after Phase 0 verification before starting Phase 1.

### Files touched (this session — all committed)

Backend (Phase 0): `internal/websocket/{hub,client,presence_refresh_test}.go`, `internal/redis/presence.go`, `internal/services/{participant,order,payment,menu,cart,staff}.go`, `internal/services/{promo_window,payment_promo,integration_test_helpers,cart}_integration_test*.go`, `internal/handlers/{promo,order,payment,cart,errors,menu_admin,branches,staff}.go`, `internal/repository/{promo,menu,staff,restaurant}.go`, `internal/domain/errors.go`, `internal/authz/{action,policy}.go`, `internal/server/server.go`, `sql/queries/{sessions,promos,menu,staff}.sql`, `migrations/000036_*`, `migrations/000037_*`, regenerated `internal/db/sqlc/*`.
Frontend (Phase 0): `app/(staff)/staff/(dashboard)/kitchen/page.tsx`, `app/(guest)/session/[id]/{cart,payment,menu}/page.tsx`, `hooks/useOrders.ts`, `lib/api/{orders,payments,staff,client}.ts`, `lib/qr.ts`, `config/env.ts`, `types/api.ts`, `components/admin/MenuItemModal.tsx`, `components/shared/BeverageModifierGrid.tsx`.

### Environment state

- **`pwd`**: `/home/mohith/Development/projects/qr-dining/frontend` (repo root is the parent)
- **`git branch`**: `* feature/certification-fixes-ui-redesign`
- **`git status`**: clean except pre-existing untracked docs/samples (`samples/`, `HANDOUT.md`, the many `*.md`/`*.html` reports, `.playwright-mcp/`). Nothing of ours is uncommitted.
- **`git log --oneline -10`**: see "Done this session".
- **Throwaway test DB**: docker container `qr-feature-test-pg` on host port **55438** (`postgres://postgres:test@localhost:55438/qrtest?sslmode=disable`) — used for `-tags integration` runs. A `qr-feature-test-redis` (port 56380) may also exist. These are disposable.
- **mtest stack**: was last brought up in the prior (certification) session — backends `:8090`/`:8095`, frontends `:3000`/`:3001`, isolated PG `:25432`/Redis `:26379` (docker project `manual-testing`). **IMPORTANT: those backends run the OLD binary from `feature/staff-analytics-loyalty`, so they do NOT have the Phase-0 fixes.** Phase-0 verification requires rebuilding mtest on THIS branch: `./scripts/manual-testing-up.sh` (no `--reset` to keep the 3 seeded tenants Saffron/Copper/Urban; staff PINs owner 1111 / mgr 2222 / waiter 3333 / kitchen 4444; platform admin@platform.local / Platform!admin1). The up-script's `GUEST_TOKEN_SECRET` was already lengthened to ≥32 chars last session.
- **The R1 soak (`qr-app-soak`) is running and must NOT be touched.**
- `ALLOW_LOCALHOST_BUILD=true npm run build` is the way to build the frontend locally (prebuild guard blocks localhost otherwise).

### Notes for next session

- **OpenAPI is still at 2.1.0** — C20 must bump it to 2.2.0 and document everything added in Phase 0 (payment promo fields, removed order promo fields, promo HH:MM times, branch restaurant_id/customer_memory_enabled, GET /branches/:id/staff, POST /staff/:id/pin/reset, PATCH /menu/modifiers/:id, modifier single_select, MODIFIER_CONFLICT). Lint with `npx @redocly/cli lint` (0 errors required) from the repo root.
- For C18 (platform staff-perf UI), the backend endpoint already exists: `GET /platform/analytics/staff-performance?branch_id=&period=`. Copy the org→branch picker pattern from the platform collateral page.
- For Phase 1, the guest design spec is `samples/guest-screens/stitch_lumina_hospitality/serene_hospitality/DESIGN.md` (alabaster #FAF9F5, deep charcoal, muted gold, Hanken Grotesk, glassmorphic floating cart, radii 8/16/24). The staff/platform token system to mirror is `samples/guest-staff-harmony/src/styles.css` + `src/components/ds/index.tsx`.
- Keep the 14 governed theme-token KEYS unchanged in Phase 1 (only preset VALUES + component styling change) so `backend/internal/services/theme.go` allowlist, the theme endpoints, and OpenAPI theme schemas stay untouched.
- Split-bill / pay-by-item appears in the bill reference design but the backend has NO split-payment support — design the Pay-Full path only; do NOT render fake split tabs.
