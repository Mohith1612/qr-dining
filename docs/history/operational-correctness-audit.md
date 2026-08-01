# QR Dining Operational Correctness Audit

Audit date: 2026-05-21

Scope: backend, frontend, WebSocket architecture, Redis coupling, auth/RBAC, session lifecycle, timeout workers, payment and promo flows, cart/order lifecycle, table occupancy lifecycle, image upload, QR system, event streaming, admin CMS, kitchen/waiter workflows, customer memory, branch isolation, transactions, idempotency, retries, optimistic updates, and reference docs (`how-to-use.html`, `database-reference.html`).

This was an audit-only pass. No production code, schema, UI, or package changes were made.

## 1. Executive Summary

QR Dining is not production-ready. The product has a coherent late-stage architecture, but operational correctness depends on several client-trusted identifiers and route-level assumptions that are not safe under real restaurant usage, multi-device behavior, or adversarial traffic.

The largest deployment blockers are:

- Staff identity is ambiguous and PIN-only. `StaffService.Authenticate` scans branch staff ordered by name and accepts the first matching PIN, and it uses `ListStaffForBranch`, which does not filter inactive staff (`backend/internal/services/staff.go:49`, `backend/sql/queries/staff.sql:5`).
- Guest identity is not authenticated. HTTP endpoints trust `X-Participant-ID` or JSON `participant_id`; session/order/cart/payment/bill APIs do not require a bearer session token or signed participant credential.
- Branch isolation is inconsistent. Several staff mutations either omit object ownership checks or trust a client-supplied `branch_id` rather than verifying the target object's branch.
- Payment finalization is unsafe. Webhooks are unauthenticated, payment initiation accepts unauthenticated guest requests, successful webhook events are never marked processed, and cash/card/UPI frontend success does not correspond to a completed backend payment.
- Session cleanup can strand occupied tables. The stale-session worker marks sessions abandoned but does not reset table status to available.
- Public session APIs expose complete session state by UUID alone. Guessing UUIDs is hard, but QR/session URLs are shared in crowded environments, and all participant operations are spoofable once a session ID is known.

Recommended next step: stop feature work and harden auth/session/payment/branch boundaries first. Do not deploy until Critical and High findings are fixed and covered by integration tests.

## 2. Severity Classification

Critical:

- Staff PIN collision and inactive-staff login.
- Client-spoofable guest participant identity.
- Cross-branch staff mutations on item-scoped and operational routes.
- Unauthenticated, replay-prone, incomplete payment lifecycle.
- Session timeout worker leaves tables occupied.
- Session creation race can create duplicate active sessions for one table.
- Menu/order branch mismatch can place orders against the wrong branch.

High:

- Staff token validity depends entirely on Redis and is not rechecked against staff active state.
- WebSocket auth uses query-string participant IDs and no signed session credential.
- Promo redemption caps are race-prone and validation preview is misleading.
- Public bill/snapshot/order/session APIs leak session data by UUID.
- Event log and customer history endpoints can leak cross-branch/cross-restaurant data.
- Assistance lifecycle trusts client-supplied table and participant IDs.
- Payment amount and bill snapshot are not protected from stale concurrent orders.
- Object ownership checks are missing for menu categories/modifiers.
- QR print/export is operationally brittle.

Medium:

- Staff auth UX cannot disambiguate people and overflows on mobile PIN cells.
- Landing page exposes orders/payment hierarchy when no orders exist.
- Promo is in the cart/order stage rather than billing/payment stage.
- Active session identifiers expose UUID fragments.
- Session timeout UX is abrupt and warning delivery is best-effort only.
- Frontend state persists staff tokens in localStorage.
- WebSocket messages are ephemeral and reconciliation is partial.
- Theme management is stored as ad hoc JSON, not an operational layer.
- Kitchen/waiter dashboards poll and optimistically mutate without conflict visibility.
- Menu image placement and bill divider/visual hierarchy issues remain.

Low:

- Hardcoded floor overview tables in waiter UI.
- Error handling often collapses operationally distinct failures into generic messages.
- Reference docs are useful but encode unsafe assumptions as normal workflows.

## 3. Detailed Findings

### F-01: Staff PIN Collision Authenticates the Wrong Person

Severity: Critical

Affected systems: staff auth, RBAC, audit logs, admin CMS, kitchen/waiter identity.

Root cause: `Authenticate` loads all staff for a branch and compares the supplied PIN against each bcrypt hash until the first match (`backend/internal/services/staff.go:49-60`). `ListStaffForBranch` orders by name and does not filter inactive staff (`backend/sql/queries/staff.sql:5`).

Reproduction scenario: create two active staff in one branch with the same PIN. Log in with branch ID + PIN. The alphabetically first staff member is returned, regardless of who is actually at the device. If an inactive staff member's PIN matches and sorts first, that inactive identity can be issued a token.

Why it is dangerous: staff actions can be attributed to the wrong person, role can be escalated accidentally if an owner/manager shares a PIN with lower-privilege staff, and deactivated staff may still authenticate. Audit logs become untrustworthy.

Recommended fix direction: replace branch ID + PIN with a non-ambiguous login identifier plus PIN, enforce unique active login identities, filter inactive staff, and return staff identity confirmation before issuing operational access. Add tests for duplicate PINs, inactive staff, and role separation.

Estimated complexity: Medium.

### F-02: Guest Participant Identity Is Fully Client-Spoofable

Severity: Critical

Affected systems: cart, orders, assistance, payment, bill, session close.

Root cause: HTTP routes trust client-provided participant IDs. Orders accept `placed_by_participant_id` from JSON (`backend/internal/handlers/order.go:23-53`). Cart routes use `X-Participant-ID` with no signed credential. Assistance accepts `participant_id` in JSON (`backend/internal/handlers/assistance.go:24-56`). Payment/bill accepts optional participant ID but does not validate it.

Reproduction scenario: join a session as participant A, inspect participant IDs from snapshot, then submit order/cart/assist requests using participant B's ID. The backend will write operations as B if the ID is accepted by foreign keys.

Why it is dangerous: guests can place orders as each other, close a session if they know the host participant ID, remove items from another participant cart, and pollute audit logs. This is not just a UI issue; it is a backend trust flaw.

Recommended fix direction: issue signed guest session credentials on create/join containing session ID, participant ID, and role/host flag. Require that credential on all participant routes and derive participant ID server-side. Add ownership checks for carts, orders, assistance, bill, payment, and close.

Estimated complexity: High.

### F-03: Cross-Branch Staff Order and Assistance Mutations Are Possible

Severity: Critical

Affected systems: kitchen workflow, waiter workflow, RBAC, event logs.

Root cause: item-scoped staff routes do not verify the target order/request belongs to the authenticated staff branch. `UpdateStatus` retrieves staff session but calls `UpdateOrderStatus` without branch comparison (`backend/internal/handlers/order.go:102-133`). Assistance acknowledge/resolve similarly pass only request ID and staff ID (`backend/internal/handlers/assistance.go:59-96`).

Reproduction scenario: authenticate as staff in branch A, obtain or guess an order UUID/request ID from branch B through logs or UI leakage, then PATCH `/orders/:id/status` or `/assist/:id/ack`. The service updates the row without comparing `order.branch_id` or assistance table branch to `staffSession.BranchID`.

Why it is dangerous: staff from one branch can advance, cancel, acknowledge, or resolve another branch's operations. In a multi-branch restaurant this breaks tenant isolation and can disrupt live service.

Recommended fix direction: all item-scoped staff mutations must load the target, compare target branch to authenticated staff branch, and enforce role-specific action policy before mutation. Add branch mismatch tests for every staff route.

Estimated complexity: Medium.

### F-04: Menu Admin Mutations Trust Client Branch IDs Instead of Object Ownership

Severity: Critical

Affected systems: admin CMS, menu correctness, branch isolation.

Root cause: update/toggle requests compare `sess.BranchID` to request body `branch_id`, then update by item ID only. SQL updates for `UpdateMenuItem`, `UpdateMenuItemAvailability`, and `UpdateMenuItemFeatured` do not include `branch_id` in the WHERE clause (`backend/sql/queries/menu.sql:45-55`, `backend/sql/queries/menu.sql:72-73`). Creation also allows a category ID that may not belong to the submitted branch (`backend/sql/queries/menu.sql:40-43`).

Reproduction scenario: staff in branch A sends `{branch_id: A}` while targeting an item ID that belongs to branch B. The handler accepts the request because the body branch matches staff branch, and SQL updates the target item by ID only.

Why it is dangerous: branch A can alter branch B's menu items if item IDs are known. Item moves can link items to categories in other branches, creating inconsistent menus and cached wrong data.

Recommended fix direction: verify the loaded item/category/modifier belongs to `staffSession.BranchID`, and update/delete with `WHERE id = $1 AND branch_id = $2`. For category moves, verify the destination category belongs to the same branch. Add DB constraints or service checks to keep `menu_items.category_id` and `menu_items.branch_id` consistent.

Estimated complexity: Medium.

### F-05: Promo Management Lacks Staff Branch Authorization

Severity: Critical

Affected systems: promotions, billing, tenant isolation.

Root cause: promo list/create/deactivate handlers parse `branchID` from the URL and perform no `GetStaffSession` branch comparison inside the handler (`backend/internal/handlers/promo.go:81-198`). The route group is staff-protected and tenant-guarded, but tenant guard only checks restaurant ownership, not staff's branch.

Reproduction scenario: a staff token from branch A calls `/branches/B/promos` where branch B is in the same tenant or tenant enforcement is disabled. The handler lists or mutates branch B promos.

Why it is dangerous: staff can inspect, create, or deactivate promotions outside their branch, changing revenue and guest pricing.

Recommended fix direction: enforce `staffSession.BranchID == branchID` and owner/manager-only policy in every promo management handler. Add explicit RBAC tests.

Estimated complexity: Low.

### F-06: Payment Webhook Endpoint Is Unauthenticated and Does Not Mark Success Processed

Severity: Critical

Affected systems: payments, session closure, webhook idempotency, audit events.

Root cause: `/webhooks/payments/:provider` accepts arbitrary JSON and does not verify provider signature (`backend/internal/handlers/payment.go:77-110`). `ProcessWebhook` inserts an idempotency row but never calls `MarkWebhookProcessed` after successful processing (`backend/internal/services/payment.go:110-157`). Parse errors call `MarkWebhookProcessed(ctx, 0, 0, err)` which updates no row (`backend/internal/services/payment.go:123-127`).

Reproduction scenario: POST `{id:"x", event:"payment.captured", payment_id:<valid>, status:"completed"}` to the webhook endpoint. The payment can be marked completed and the session auto-closed. The webhook row remains `processed=false`, so operational replay tooling would see it as unprocessed.

Why it is dangerous: anyone with network access can mark payments complete or refunded, close sessions, and generate false audit trails. Idempotency bookkeeping is incomplete, making reconciliation unreliable.

Recommended fix direction: require provider-specific signature verification, bind external provider payment IDs rather than internal IDs, mark webhook rows processed on success/failure with the correct webhook row ID, and reject events that do not match the expected amount/session/provider reference.

Estimated complexity: High.

### F-07: Cash/Card/UPI Frontend Success Does Not Complete Backend Payment

Severity: Critical

Affected systems: payment flow, session lifecycle, table turnover.

Root cause: payment initiation only creates a pending payment (`backend/internal/services/payment.go:57-99`). Session closure happens only after a completed webhook (`backend/internal/services/payment.go:147-154`). Frontend immediately shows "Payment recorded" after initiation (`frontend/app/(guest)/session/[id]/payment/page.tsx`) even for cash/card/UPI methods that have no real provider integration in this code.

Reproduction scenario: guest taps Cash. Backend inserts a pending payment; frontend displays success. No webhook arrives, so payment remains pending and session remains active until timeout.

Why it is dangerous: guests and staff can believe payment was completed when backend state remains unpaid. Tables may remain occupied, analytics/revenue are wrong, and staff has no reconciliation workflow.

Recommended fix direction: split "request payment" from "payment completed". Cash/card should create staff-visible payment requests that staff can settle with RBAC. Digital should create provider payment intents and only show final success after provider confirmation.

Estimated complexity: High.

### F-08: Stale Session Cleanup Does Not Free Tables

Severity: Critical

Affected systems: timeout worker, table occupancy, QR entry, staff sessions dashboard.

Root cause: worker `cleanStaleSessions` marks sessions abandoned and deletes presence, but does not update `tables.status` to available (`backend/internal/worker/worker.go:149-158`). SQL `AbandonStaleSession` only updates the session row (`backend/sql/queries/workers.sql:12-15`).

Reproduction scenario: create a session, wait past branch timeout, let worker abandon it. The session is no longer active, but the table remains `occupied`.

Why it is dangerous: staff dashboards and QR entry can show tables as occupied without an active session. Guests may not be able to start a new session cleanly, and QR regeneration may be blocked by stale occupancy.

Recommended fix direction: abandon session and update table status in one transaction, publish a session-closed event, and add worker integration tests asserting table status recovery.

Estimated complexity: Medium.

### F-09: Session Creation Has a Race Window for Duplicate Active Sessions

Severity: Critical

Affected systems: QR entry, carts, orders, table lifecycle.

Root cause: `CreateSession` checks active sessions before starting the transaction (`backend/internal/services/session.go:44-52`), then inserts a new active session without locking the table or relying on a partial unique index for active table sessions. The schema indexes `sessions.table_id` but does not prevent multiple active sessions per table.

Reproduction scenario: two guests scan the same available QR and submit within milliseconds. Both preflight checks can observe no active session, and both insert active sessions.

Why it is dangerous: one physical table can have multiple active sessions, separate carts/orders/payments, and conflicting table state. Staff sees inconsistent service context.

Recommended fix direction: enforce a partial unique index on `(table_id) WHERE status='active'`, or lock the table row with `SELECT ... FOR UPDATE` inside the transaction and re-check. Handle unique conflicts as "session already active".

Estimated complexity: Medium.

### F-10: Orders Can Be Created with Mismatched Session, Branch, Participant, and Menu Items

Severity: Critical

Affected systems: order placement, billing, kitchen routing, analytics.

Root cause: order placement accepts branch ID and participant ID from client JSON (`backend/internal/handlers/order.go:23-53`). `PlaceOrder` validates the session is active but does not enforce `req.BranchID == sess.BranchID`, does not verify participant belongs to the session, and does not verify all menu items belong to that branch/session (`backend/internal/services/order.go:70-143`).

Reproduction scenario: a client submits session ID from branch A, `branch_id` for branch B, participant ID from any session, and menu item IDs from branch B. The order can be stored under branch B while attached to session A.

Why it is dangerous: kitchen dashboards, billing, analytics, promo validation, and event logs diverge. Cross-branch order injection becomes possible.

Recommended fix direction: derive branch ID from the session server-side, require participant credential, verify participant belongs to the session, and verify every menu item/category/modifier belongs to the session branch.

Estimated complexity: Medium.

### F-11: Idempotency Keys Are Globally Scoped and Replays Are Not Request-Bound

Severity: High

Affected systems: orders, retries, multi-device clients.

Root cause: `orders.idempotency_key` is globally unique and `GetOrderByIdempotencyKey` returns the existing order without verifying same session, participant, branch, or request body (`backend/internal/services/order.go:55-65`).

Reproduction scenario: a buggy or malicious client reuses another order's idempotency key. The API returns the prior order rather than rejecting a mismatched replay.

Why it is dangerous: clients can receive or confirm the wrong order, and idempotency can become a cross-session data leak.

Recommended fix direction: scope idempotency keys by session/participant, store a hash of the request body, and reject replay attempts whose request hash or principal differs.

Estimated complexity: Medium.

### F-12: Promo Redemption Caps Are Race-Prone and Preview Is Misleading

Severity: High

Affected systems: promotions, billing, revenue leakage.

Root cause: promo validation counts existing redemptions and later inserts a redemption without locking the promo row or enforcing unique/cap constraints in the DB (`backend/internal/services/promo.go`). The public validation endpoint passes `OrderTotal: 0`, so min-order validation is not a real preview (`backend/internal/handlers/promo.go:53-57`). Frontend promo flow appears before final billing (`frontend/app/(guest)/session/[id]/cart/page.tsx`).

Reproduction scenario: two clients apply the final remaining promo at the same time. Both count below max and both insert redemptions. Separately, a guest can see a promo preview that later fails at order placement because cart total was not validated.

Why it is dangerous: promo caps can be exceeded, guest expectations break, and discounts are offered at the wrong lifecycle stage.

Recommended fix direction: validate promos at billing/order finalization using locked rows or DB constraints; move promo entry to billing/payment; compute preview against authoritative cart/order totals; require phone/customer identity for per-phone limits.

Estimated complexity: Medium.

### F-13: Public Session Snapshot, Orders, Bill, and Session Reads Are UUID-Only

Severity: High

Affected systems: privacy, session lifecycle, guest recovery.

Root cause: public routes expose session, order list, bill, and snapshot by session UUID alone. Snapshot returns participants, orders, assistance, and session state without participant authorization (`backend/internal/services/session.go:198-247`).

Reproduction scenario: anyone with a session URL or UUID can fetch `/sessions/:id/snapshot`, `/sessions/:id/orders`, and `/sessions/:id/bill`.

Why it is dangerous: QR sessions are naturally shared at tables and can leak through browser history, screenshots, logs, and support messages. UUID entropy helps against blind guessing but does not solve bearer-link privacy.

Recommended fix direction: require signed participant credentials for session-scoped reads, and expose limited unauthenticated state only through QR token resolution.

Estimated complexity: Medium.

### F-14: Staff Tokens Are Redis-Only and Not Revalidated Against Staff State

Severity: High

Affected systems: staff auth, deactivation, incident response.

Root cause: `ValidateToken` only reads a cached session object from Redis and returns it (`backend/internal/services/staff.go:85-95`). It does not re-check `staff.is_active`, role, branch, or updated PIN version in PostgreSQL. Deactivation invalidation is best-effort and logs failures as non-fatal (`backend/internal/services/staff.go:129-146`).

Reproduction scenario: Redis token set tracking fails during login or deactivation. A deactivated staff token remains valid until TTL. Role changes or branch moves would not reflect in existing tokens.

Why it is dangerous: revoked staff can retain access for up to 8 hours, and emergency deactivation is unreliable.

Recommended fix direction: include a staff token version/session ID in PostgreSQL, validate active state or token version on sensitive routes, and make invalidation durable. At minimum, re-check active staff on token validation.

Estimated complexity: Medium.

### F-15: WebSocket Auth Is Query-String Based and Not Bound to a Signed Session

Severity: High

Affected systems: realtime state, reconnect, privacy.

Root cause: frontend opens `/ws?session_id=...&participant_id=...` (`frontend/lib/ws/connection.ts:25-28`), and backend validates only that the participant ID belongs to the session (`backend/internal/handlers/ws.go:22-60`). There is no signed participant credential.

Reproduction scenario: a participant ID observed from snapshot can be used to open another WebSocket as that participant.

Why it is dangerous: realtime events are private table context. Query parameters can land in logs and are easy to replay.

Recommended fix direction: use a signed guest access token in `Sec-WebSocket-Protocol` or a short-lived token endpoint, and validate it server-side before upgrade. Avoid sensitive query params.

Estimated complexity: Medium.

### F-16: Event Log Session Timeline Is Not Branch-Checked

Severity: High

Affected systems: audit log, operational debugging.

Root cause: `GetSessionEvents` fetches events for any session ID and does not compare the session/event branch to authenticated staff branch (`backend/internal/handlers/event_log.go:18-35`).

Reproduction scenario: staff from branch A requests `/sessions/{branchBSession}/events`.

Why it is dangerous: audit logs contain operational detail and customer/session context. This leaks branch data and weakens audit boundaries.

Recommended fix direction: load session or event branch, compare with `staffSession.BranchID`, and restrict to manager/owner where appropriate.

Estimated complexity: Low.

### F-17: Customer History Ignores Restaurant Scope

Severity: High

Affected systems: customer memory, privacy, PII.

Root cause: `GetCustomerHistory` accepts `restaurantID` but calls `repos.GetCustomerSessionHistory(customerID)`; the SQL query filters only by `s.customer_id = $1` and not restaurant ID (`backend/internal/services/customer.go`, `backend/sql/queries/customers.sql`).

Reproduction scenario: staff from restaurant A guesses or obtains customer ID from restaurant B and requests `/customers/:id/history`. Handler passes restaurant A ID, but service/query does not use it.

Why it is dangerous: cross-restaurant customer visit history and spend can leak.

Recommended fix direction: join customers and filter by `customers.restaurant_id = $2` in history queries; return 404/403 if not scoped.

Estimated complexity: Low.

### F-18: Assistance Requests Trust Client Table and Participant IDs

Severity: High

Affected systems: waiter workflow, table routing.

Root cause: assistance request handler accepts `table_id` and `participant_id` from JSON and passes them directly to insert (`backend/internal/handlers/assistance.go:24-56`). The service loads the session but does not verify table ID equals the session table or participant belongs to session.

Reproduction scenario: a guest in table 1 sends an assistance request with table_id 9. Waiter dashboard routes it to table 9.

Why it is dangerous: staff can be sent to the wrong table, creating real operational confusion during service.

Recommended fix direction: derive table ID from the session and participant ID from signed participant auth; reject mismatches.

Estimated complexity: Low.

### F-19: Payment Amount and Bill Snapshot Can Race with New Orders

Severity: High

Affected systems: billing correctness, payments, session closure.

Root cause: payment initiation computes bill, creates a pending payment, and later webhook closes the session. There is no transaction that freezes billable orders or prevents new order placement while payment is pending (`backend/internal/handlers/payment.go:49-69`, `backend/internal/services/payment.go:57-99`).

Reproduction scenario: guest A initiates payment, guest B places another order before webhook completion. Payment may complete and close the session against an outdated bill snapshot.

Why it is dangerous: unpaid orders can be served or bill totals can change after a guest has initiated payment.

Recommended fix direction: introduce bill/payment state: lock session for settlement, reject new orders after payment initiation, or require staff confirmation of late orders. Store immutable payment line items.

Estimated complexity: High.

### F-20: Billing Uses Float Math and Recomputes Item Names from Current Menu

Severity: Medium

Affected systems: billing, receipts, itemized bills.

Root cause: bill computation converts numerics to floats and rounds (`backend/internal/handlers/billing.go`). Order item prices are snapshotted, but item names are read from current menu items instead of stored on order items.

Reproduction scenario: rename a menu item after an order is placed. The bill shows the new name for an old order. Rounding can drift over many items/modifiers.

Why it is dangerous: receipts are not immutable records of what was ordered; financial calculations should not rely on float.

Recommended fix direction: snapshot item names and modifier names/prices on order items and compute using integer minor units or decimal arithmetic.

Estimated complexity: Medium.

### F-21: Staff Login UX Is Operationally Weak

Severity: Medium

Affected systems: staff login, shift operations.

Root cause: UI asks only for branch ID and PIN and renders six fixed 48px PIN boxes (`frontend/app/(staff)/staff/login/page.tsx:100-132`).

Reproduction scenario: on narrow mobile widths, six boxes plus gaps can overflow. Operationally, a shared PIN gives no way to choose the intended staff member.

Why it is dangerous: the UI normalizes ambiguous identity and can be hard to use on staff phones.

Recommended fix direction: add staff identifier selection/search, responsive PIN input, identity confirmation, lockout messaging, and manager-managed credential reset.

Estimated complexity: Medium.

### F-22: Staff Tokens Are Persisted in localStorage

Severity: Medium

Affected systems: staff auth frontend, browser security.

Root cause: Zustand persist stores staff token under `staff-auth` in localStorage (`frontend/store/staff.ts:17-44`).

Reproduction scenario: XSS, shared tablet access, browser extension compromise, or unattended staff device exposes a live bearer token.

Why it is dangerous: staff tokens authorize admin, kitchen, waiter, and customer data routes.

Recommended fix direction: prefer httpOnly secure same-site cookies or encrypted platform storage, shorten token TTL, add idle timeout/lock, and implement explicit server-side logout/revocation.

Estimated complexity: Medium.

### F-23: QR Print/Export Is Brittle and Can Print Blank Pages

Severity: Medium

Affected systems: QR management, table setup.

Root cause: print template is hidden by default and relies on dynamic client-only QR rendering plus `setTimeout(() => window.print(), 100)` in admin (`frontend/components/admin/PrintTemplate.tsx`, `frontend/app/(staff)/staff/(dashboard)/admin/page.tsx`). If QR SVG is not mounted before print, output can be blank.

Reproduction scenario: click print immediately after setting `printTable` on a slower device. Browser print snapshot can occur before dynamic QR component is rendered.

Why it is dangerous: restaurants can deploy invalid or blank QR cards, blocking guest ordering.

Recommended fix direction: render a dedicated print route with QR ready state, wait for QR component mount before printing, and provide PDF/export fallback.

Estimated complexity: Medium.

### F-24: Active Session and Kitchen UI Expose UUID Fragments

Severity: Medium

Affected systems: admin sessions, kitchen display, operational UX.

Root cause: active sessions display `s.id.slice(0, 8)` and kitchen falls back to `order.id.slice(0, 8)` (`frontend/app/(staff)/staff/(dashboard)/admin/page.tsx`, `frontend/app/(staff)/staff/(dashboard)/kitchen/page.tsx`).

Reproduction scenario: staff sees `#a1b2c3d4` as a session/order identifier when a human-readable table/order number is unavailable.

Why it is dangerous: UUID fragments are not operational identifiers. They are hard to communicate and leak implementation details.

Recommended fix direction: always display table identifier, order number, and short human sequence. Hide UUIDs behind detail views for support/debug only.

Estimated complexity: Low.

### F-25: Session Timeout UX Is Abrupt and Warning Delivery Is Best-Effort

Severity: Medium

Affected systems: guest experience, session lifecycle, Redis pub/sub.

Root cause: warning worker publishes `SESSION_EXPIRING_SOON` once and marks warned even though pub/sub is ephemeral (`backend/internal/worker/worker.go:171-185`). If the user is disconnected, frontend learns only on later snapshot/reconnect. Actual timeout closes/abandons without user-controlled extension workflow.

Reproduction scenario: guest loses network during warning window, reconnects after abandonment, and sees ended session without a recovery path.

Why it is dangerous: active diners can be abruptly cut off during long meals or bad Wi-Fi.

Recommended fix direction: persist warning state, expose `expires_at` in snapshots, provide guest/staff extension actions, and reset timeout based on meaningful activity if product requirements allow.

Estimated complexity: Medium.

### F-26: WebSocket Event Delivery Is Ephemeral and Partial Reconciliation Can Miss Details

Severity: Medium

Affected systems: realtime frontend, Redis pub/sub, reconnect.

Root cause: Redis pub/sub events are not durable; publisher logs errors but business operations continue (`backend/internal/events/events.go`). Snapshot reconciliation restores sessions, participants, orders, and assistance, but cart state is only refreshed on `CART_UPDATED` for the current participant (`frontend/hooks/useWebSocket.ts`).

Reproduction scenario: Redis briefly disconnects during order placement. Order persists, but connected clients may miss live updates until polling/reconnect. Cart events dropped during a bad connection leave local cart stale.

Why it is dangerous: kitchen/waiter/guest views can diverge under network instability.

Recommended fix direction: add event sequence numbers and durable outbox/event log replay, reconcile all critical stores after reconnect, and make dashboards poll or subscribe to branch-level durable streams.

Estimated complexity: High.

### F-27: Theme Management Is an Ad Hoc JSON Setting, Not an Operational Layer

Severity: Medium

Affected systems: admin settings, QR entry, frontend theming.

Root cause: theme is stored in restaurant `settings_json` and exposed via branch QR resolution. Only two hardcoded values are accepted in branch handler. There is no preview, rollout, branch override model, or asset validation.

Reproduction scenario: updating theme for one branch modifies restaurant settings, affecting every branch sharing that restaurant record.

Why it is dangerous: visual identity and accessibility can change globally by accident.

Recommended fix direction: create explicit theme configuration with branch/restaurant inheritance, preview, validation, and rollback.

Estimated complexity: Medium.

### F-28: Operational Frontend Known Issues Remain

Severity: Medium

Affected systems: guest landing, menu, cart, bill, payment.

Root cause: UI workflows still show actions out of lifecycle order and visual inconsistencies remain.

Known issues included in this audit:

- Landing page shows Orders and Pay Bill before orders exist.
- Menu item image placement is still inconsistent with the intended left-image layout.
- Promo entry appears before order placement and belongs in billing/payment.
- Bill UI has divider and visual hierarchy inconsistencies.
- QR print/export can render blank pages.

Reproduction scenario: follow the reference manual happy paths from `how-to-use.html` on a fresh session and inspect first-viewport actions, cart promo placement, bill page, and QR print flow.

Why it is dangerous: these are not just polish issues; they guide staff/guests toward premature or confusing operational actions.

Recommended fix direction: fix after security/lifecycle blockers. Align UI state with backend lifecycle: no bill/payment emphasis until billable orders exist, promo at settlement, consistent item layout, robust print route.

Estimated complexity: Low to Medium.

### F-29: Waiter Floor Overview Is Hardcoded

Severity: Low

Affected systems: waiter dashboard.

Root cause: waiter page defines `KNOWN_TABLES = [{id:1,label:"T1"}, ...]` instead of using branch table data.

Reproduction scenario: branch has table IDs that differ from 1-3. The floor overview shows wrong tables.

Why it is dangerous: staff can trust a false floor map during service.

Recommended fix direction: load branch tables from `/branches/:id/tables` and map assistance requests by table identifier.

Estimated complexity: Low.

### F-30: Error States Are Too Generic for Operations

Severity: Low

Affected systems: frontend, staff workflows, guest workflows.

Root cause: many catches show "Something went wrong" or silent refresh failures; backend often maps distinct data problems to internal errors.

Reproduction scenario: Redis outage, payment bill compute failure, stale cart item, branch mismatch, or inactive session can all produce generic messages.

Why it is dangerous: staff cannot distinguish retryable network errors, permission problems, stale state, or real backend incidents.

Recommended fix direction: expand typed API errors, log correlation IDs in UI, and add staff-facing remediation messages.

Estimated complexity: Medium.

## 4. Architecture Risk Areas

### Auth

Current staff auth is PIN-only and ambiguous. Guest auth is effectively bearer-by-session-UUID plus client-asserted participant ID. Production needs two distinct credential models:

- staff: unique login identifier, PIN/password/manager reset, active-state validation, durable revocation, role-scoped tokens;
- guest: signed session participant token issued on create/join and required for all session actions.

### WebSocket Lifecycle

WebSocket subscriptions are session-scoped and query-param authenticated. Pub/sub is ephemeral and has no durable replay or sequence numbers. Reconnect snapshots help, but they do not fully cover cart/payment/staff dashboard branch streams.

### Redis Coupling

Redis is required at startup and used for tokens, rate limiting, cache, presence, locks, and pub/sub. Rate limiting fails open. Token revocation is best-effort. Worker locks depend on Redis but do not protect all lifecycle state transitions transactionally.

### DB Transaction Consistency

Transactions exist for session creation and order placement, but important invariants are outside DB constraints:

- one active session per table;
- item/category same branch;
- order/session/branch/participant consistency;
- promo max-use/per-phone caps;
- payment locks/frozen bill snapshots;
- stale cleanup session/table update atomicity.

### Frontend State Architecture

Frontend state is split across Zustand, sessionStorage, localStorage, and WebSocket event handlers. It assumes the stored participant ID is valid and durable. Staff tokens persist across browser restarts. Reconciliation does not reset all stores comprehensively.

### Optimistic UI Strategy

Cart/order/menu/admin flows use optimistic updates but generally lack conflict detection. Kitchen/waiter dashboards mutate local state after status updates but do not show when another device already changed the status.

### Session Lifecycle Management

Create, join, close, timeout, payment, and table occupancy are not modeled as a single authoritative state machine. Table status is a denormalized lifecycle flag and can desync. Payment does not reliably drive closure. Timeout is abrupt and not recoverable.

## 5. Operational Readiness Score

Frontend maturity: 6/10

The frontend is feature-rich and mostly coherent, but it embeds unsafe identity assumptions, persists staff tokens in localStorage, has lifecycle hierarchy issues, and has operationally fragile QR printing/payment states.

Backend maturity: 5/10

The backend has clear services/repositories, sqlc, migrations, Redis helpers, and tests, but critical branch/session/payment invariants are enforced inconsistently or not at all.

Operational correctness: 3/10

Major real-world flows can desync: duplicate table sessions, stale occupied tables, spoofed participants, pending payments shown as completed, promo cap races, and cross-branch mutations.

Security posture: 2/10

Staff auth ambiguity, client-spoofable guest identity, unauthenticated webhooks, inconsistent RBAC, and UUID-only public session reads are production blockers.

Deployment readiness: 2/10

Do not deploy to production until Critical and High findings are fixed, tested, and re-audited.

## 6. Recommended Hardening Roadmap

1. Auth and identity boundary

- Replace PIN-only staff login with unambiguous staff identity + PIN.
- Filter inactive staff and enforce active-state token validation.
- Add signed guest participant tokens and remove client-trusted participant IDs.

2. Branch isolation and RBAC

- Add target-object branch checks to every staff mutation.
- Fix menu item/category/modifier branch constraints.
- Enforce promo branch and owner/manager checks.
- Add integration tests for branch mismatch on all protected routes.

3. Session/table lifecycle

- Add DB invariant for one active session per table.
- Make session create/close/abandon/table-status updates transactional.
- Add staff/guest session extension or recovery flow.

4. Payment correctness

- Authenticate provider webhooks and bind external references.
- Separate pending payment requests from completed payments.
- Add settlement state and freeze bill/order creation during payment.
- Mark webhook rows processed correctly and add reconciliation tooling.

5. Order/cart/promo correctness

- Derive branch and participant server-side.
- Verify menu items/modifiers belong to session branch.
- Bind idempotency keys to participant/session/request body.
- Lock or constrain promo cap redemption.

6. Realtime and Redis resilience

- Add durable event outbox or sequence-based replay.
- Reconcile all client stores after reconnect.
- Decide fail-open/fail-closed policy for rate limiting and token checks.

7. Operational UI cleanup

- Move promo to billing/payment.
- Hide payment until billable orders exist.
- Replace UUID fragments with human identifiers.
- Fix QR print/export with a dedicated route.
- Replace hardcoded waiter floor map with branch table data.

8. Re-audit and deployment gate

- Run backend integration tests covering concurrency, branch isolation, RBAC, payment, promo caps, stale worker, and WebSocket reconnect.
- Run mobile/desktop Playwright scenarios for staff login, QR printing, bill/payment, timeout, and dashboard workflows.
- Perform a final security review before production.
