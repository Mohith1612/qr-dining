# E2E Test Failure Analysis — Pre-R1 Playwright Run

**Run:** `/tmp/playwright-pre-r1-v4.txt`  
**Results:** 84 passed, 43 failed (127 total)  
**Date:** 2026-05-25

---

## Category 1 — Backend Returns Wrong HTTP Status (Backend Bugs)

These are genuine backend issues where the server is returning an incorrect status code.

### X-03: Staff token on guest routes → 500
**Spec:** `expect([401, 403]).toContain(cartRes.status)`  
**Actual status:** 500  
**File:** `e2e/adversarial/X-03-staff-token-on-guest-routes.spec.ts`  
**Root cause:** When a staff JWT is passed to the guest auth middleware (`/sessions/:id/cart`), the backend returns 500 instead of 401/403. The guest auth middleware is likely panicking or producing an unhandled error path when encountering a JWT that is structurally valid but not a guest token.

### O-05 test 2: Zero-quantity order → 500
**Spec:** `expect([400, 422]).toContain(res.status)`  
**Actual status:** 500  
**File:** `e2e/order/O-05-empty-order-rejected.spec.ts` (line 44)  
**Root cause:** Sending `items: [{menu_item_id: id, quantity: 0}]` causes an internal server error. The binding tag `min=1` on the items array only validates array length, not individual item quantities. A zero-quantity item reaches the service or database layer and panics or hits an unhandled constraint violation.

### PT-03: MFA enroll → 500
**Spec:** `expect([200, 201, 400, 401, 404, 409]).toContain(res.status)` and `expect(res.status).not.toBe(500)`  
**Actual status:** 500  
**File:** `e2e/platform/PT-03-mfa-required-for-platform.spec.ts` (line 24)  
**Root cause:** `MFA_ENCRYPTION_KEY` is not set in the backend test `.env`. `BeginMFAEnrollment` fails to encrypt the TOTP secret and returns 500 via `respondInternalError`. The spec explicitly prohibits 500.

### T-01: Cross-org session snapshot → 200 (security issue)
**Spec:** `expect([401, 403, 404]).toContain(res.status)`  
**Actual status:** 200  
**File:** `e2e/tenancy/T-01-cross-org-session-access.spec.ts` (line 16)  
**Root cause:** A staff token from org A can successfully read a session snapshot belonging to org B. `GET /sessions/:id/snapshot` does not enforce cross-org isolation when authenticated with a staff token. This is a real cross-org data leak — the endpoint either accepts staff tokens directly or fails to verify branch/org scope.

---

## Category 2 — Spec Expects Wrong HTTP Status Code (Spec Bugs)

The backend returns a different but equally valid status code. The spec is too strict.

### X-07: WS ticket replay → spec expects 200, backend returns 201
**Spec:** `expect(ticketRes.status).toBe(200)` (line 21)  
**Actual status:** 201  
**File:** `e2e/adversarial/X-07-replay-attack-old-ws-ticket.spec.ts`  
**Fix:** Change to `expect([200, 201]).toContain(ticketRes.status)`

### R-01 test 1: ws-ticket valid token → spec expects 200, backend returns 201
**Spec:** `expect(validRes.status).toBe(200)` (line 21)  
**Actual status:** 201  
**File:** `e2e/realtime/R-01-ws-ticket-required.spec.ts`

### R-01 test 2: ticket single-use → spec expects 200, backend returns 201
**Spec:** `expect(ticketRes.status).toBe(200)` (line 56)  
**Actual status:** 201  
**File:** `e2e/realtime/R-01-ws-ticket-required.spec.ts`

### R-03: WS ticket rate limit → 201 not in [200, 429]
**Spec:** `expect(statuses.every((s) => [200, 429].includes(s))).toBe(true)` (line 32)  
**Actual status:** some tickets return 201 (not in the allowed set)  
**File:** `e2e/realtime/R-03-ws-ticket-rate-limit.spec.ts`  
**Fix:** Add 201 to the accepted set: `[200, 201, 429]`

### L-03: Reconnect ws-ticket → spec expects 200, backend returns 201
**Spec:** `expect(ticketRes.status).toBe(200)` (line 16)  
**Actual status:** 201  
**File:** `e2e/session/L-03-reconnect-awaiting-reactivation.spec.ts`

### R-06: Snapshot authoritative signal missing
**Spec:** `expect(hasEvents || hasSnapshotSignal).toBe(true)` (line 38)  
**Actual:** `snap.missed_events` is an empty array `[]` (not null, not > 0 elements), and `snap.snapshot_authoritative` is absent from response  
**File:** `e2e/realtime/R-06-snapshot-authoritative-on-large-gap.spec.ts`  
**Root cause:** The snapshot endpoint returns `missed_events: []` for a session with events at `last_sequence=0` instead of either populated events or a `snapshot_authoritative: true` signal. Spec condition `hasEvents` requires `length > 0`, and `hasSnapshotSignal` requires `null` or a truthy flag. Neither branch is satisfied.

### M-04: Session join → spec expects 200, backend returns 201
**Spec:** `expect(joinRes.status).toBe(200)` (line 31)  
**Actual status:** 201  
**File:** `e2e/multidevice/M-04-second-device-joins.spec.ts`  
**Fix:** Change to `expect([200, 201]).toContain(joinRes.status)`

### X-05: Brute-force lockout → backend returns 423, spec expects [429, 401]
**Spec:** `expect([429, 401]).toContain(lastStatus)` (line 27)  
**Actual status:** 423 (HTTP Locked)  
**File:** `e2e/adversarial/X-05-brute-force-lockout.spec.ts`  
**Root cause:** The backend uses `http.StatusLocked` (423) for auth lockout. The spec only accepts 429 (Too Many Requests) or 401.  
**Fix:** Add 423 to the expected set: `[429, 423, 401]`

### L-05: Second DELETE on closed session → returns 401, not in spec
**Spec:** `expect([200, 204, 409, 410]).toContain(close2.status)` (second delete)  
**Actual status:** 401  
**File:** `e2e/session/L-05-host-close-idempotent.spec.ts`  
**Root cause:** After the session is closed by the first DELETE, the guest token is invalidated. The second DELETE returns 401 because the token no longer authenticates.  
**Fix:** Add 401 to the second close expectation: `[200, 204, 401, 409, 410]`

### S-01 test 3: Old format accepted → spec expects rejection
**Spec:** `expect([400, 401, 422]).toContain(res.status)` (line 40)  
**Actual status:** 200 (old format accepted)  
**File:** `e2e/staff/S-01-staff-login.spec.ts`  
**Root cause:** `AUTH_STAFF_CODE_REQUIRED=false` (default). The backend's auth handler accepts the legacy `{branch_id, pin}` format when the strict flag is off. The spec comment says "when strict mode active" but the backend `.env` does not enable strict mode.  
**Fix:** Either set `AUTH_STAFF_CODE_REQUIRED=true` in test env, or add 200 to accepted statuses with a note that strict mode is disabled.

---

## Category 3 — Platform Token Used on Staff-Protected Endpoints (Spec Bugs)

These specs use `adminToken` (a platform JWT) to call endpoints in `branchStaffAPI`, which requires `StaffAuth`. Platform tokens are rejected with 401 on these routes. As a result, the action (staff creation, deactivation, item update) silently fails, and the test's subsequent assertion on the downstream effect fails.

### S-02: Staff deactivation uses adminToken
**Spec:** Uses `adminToken` to POST `/branches/:id/staff/:id/deactivate`  
**File:** `e2e/staff/S-02-inactive-staff-rejected.spec.ts`  
**Effect:** Deactivation is rejected (401/403), staff remains active, subsequent login returns 200 instead of [401, 403]

### S-03: Staff creation uses adminToken
**Spec:** Uses `adminToken` to POST `/branches/:id/staff`  
**File:** `e2e/staff/S-03-duplicate-staff-code-rejected.spec.ts`  
**Effect:** First staff creation fails with 401 (not 200/201 as spec expects at line 36)

### S-04: PIN rotation uses adminToken
**Spec:** Uses `adminToken` to PATCH `/staff/:id/pin`  
**File:** `e2e/staff/S-04-pin-rotation.spec.ts`  
**Effect:** Rotation fails with 401, not in `[200, 204, 404]`, test fails before the 404-skip guard

### A-06: Owner staff creation uses adminToken
**Spec:** Uses `adminToken` to POST `/branches/:id/staff` to create an owner, then calls `loginStaff`  
**File:** `e2e/audit/A-06-cross-org-audit-isolation.spec.ts`  
**Effect:** Staff not created → `loginStaff` returns 401 → helper throws "API POST /staff/auth → 401"

### OPID-04: Kitchen staff creation uses adminToken
**Spec:** Uses `adminToken` to POST `/branches/:id/staff` to create kitchen staff, then calls `loginStaff`  
**File:** `e2e/operational-ids/OPID-04-operational-id-kitchen-display.spec.ts`  
**Effect:** Staff not created → `loginStaff` returns 401 → helper throws

### S-08: Owner staff creation uses adminToken
**Spec:** Uses `adminToken` to POST `/branches/:id/staff` to create an owner  
**File:** `e2e/staff/S-08-owner-platform-limits.spec.ts`  
**Effect:** Staff not created → `loginStaff` returns 401 → helper throws

### O-02: Item deactivation uses adminToken
**Spec:** Uses `adminToken` to PATCH `/branches/:id/menu/items/:id` with `is_available: false`  
**File:** `e2e/order/O-02-unavailable-item-rejected.spec.ts`  
**Effect:** PATCH rejected (401), item remains available, order succeeds (201), spec expects [400, 409, 422]

**Note on fix:** All 7 specs should replace `adminToken` with the staff token obtained from `loginStaff(branch.code, staff.staffCode, staff.pin)` using the credentials already returned by `seedOrg`. For A-06, OPID-04, S-08: the `initial_owner` from `seedOrg` already has owner role and can be used directly without creating additional staff.

---

## Category 4 — `fetchAudit` Helper Returns Wrong Shape (Helper Bug)

The backend `/platform/audit` endpoint returns `{"audit": [...]}` (wrapped in a JSON object). The `fetchAudit` helper returns this object directly. Callers treat it as a raw array.

### P-01: `audit.length` is undefined
**Spec:** `expect(audit.length).toBeGreaterThan(0)` (line 47)  
**File:** `e2e/payment/P-01-cash-settlement.spec.ts`  
**Root cause:** `fetchAudit` returns `{audit: [...]}`. `.length` is undefined on an object.

### A-05 tests 1 and 2: `entries[0].id` TypeError
**Spec:** `const entryId = entries[0].id` (line 19) — TypeError: Cannot read properties of undefined (reading 'id')  
**File:** `e2e/audit/A-05-audit-entries-immutable.spec.ts`  
**Root cause:** Same shape issue — `entries` is `{audit: [...]}` not `[...]`, so `entries[0]` is undefined.

**Fix:** In `helpers/api.ts`, `fetchAudit` should extract `.audit` from the response: return `data.audit ?? data`.

---

## Category 5 — Missing Webhook Secret in Backend Config (Config Issue)

All webhook tests use `placeWebhook()` which signs payloads with `process.env.WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"`. The backend loads webhook secrets from env var `PAYMENT_WEBHOOK_SECRET_STRIPE`. This is not set in `backend/.env`, so the backend has no valid secret for "stripe" provider and rejects all signed webhooks with 401.

**Affected tests (all return 401, all expect 2xx/4xx):**

| Test | File | Spec expects |
|------|------|-------------|
| W-01 | `webhook/W-01-valid-webhook-settles-session.spec.ts` | `[200, 202]` |
| W-02 | `webhook/W-02-webhook-idempotency.spec.ts` | `[200, 202]` |
| W-05 | `webhook/W-05-unknown-event-type-accepted.spec.ts` | `[200, 202, 400]` |
| W-06 | `webhook/W-06-webhook-for-unknown-session.spec.ts` | `[200, 202, 404, 422]` |
| W-07 | `webhook/W-07-payment-failed-webhook.spec.ts` | `[200, 202, 404]` |
| P-03 | `payment/P-03-webhook-replay.spec.ts` | `[200, 204, 400, 404, 409]` |
| P-13 | `payment/P-13-webhook-amount-mismatch.spec.ts` | `[200, 400, 409, 422]` |

**Fix:** Add `PAYMENT_WEBHOOK_SECRET_STRIPE=test-webhook-secret` to `backend/.env`.

---

## Category 6 — API Response Shape Mismatch (Spec Bugs)

### O-01: Order ID not found in response
**Spec:** `expect(order.id ?? order.order_id).toBeTruthy()` (line 26)  
**File:** `e2e/order/O-01-order-placement.spec.ts`  
**Root cause:** Backend returns `PlaceOrderResult{Order: sqlc.Order{...}, OrderItems: [...]}` which serializes to `{"Order": {"id": "..."}, "OrderItems": [...]}` (capitalized field names, no json tags on `PlaceOrderResult`). Spec looks for `order.id` (flat) but the actual path is `order.Order.id`.

### O-07: Same order ID shape issue
**Spec:** `expect(o1.id ?? o1.order_id).not.toEqual(o2.id ?? o2.order_id)` — both sides are undefined  
**File:** `e2e/order/O-07-concurrent-orders.spec.ts` (line 44)  
**Root cause:** Same as O-01 — response shape is `{Order: {id:...}}` not `{id:...}`.

### PT-04: Org creation body uses old schema
**Spec:** Sends `{name: "PT04 Org", slug}` to `POST /platform/organizations` (line 16)  
**File:** `e2e/platform/PT-04-org-provisioning.spec.ts`  
**Actual status:** 400 (bad request)  
**Root cause:** Backend now requires `{code, name, restaurant_slug}`. The spec uses the old schema. Downstream step accesses `org.id` on the 400 response which is also wrong. The branch creation also sends wrong body format.

---

## Category 7 — Spec Logic Issues

### S-06 tests 1 and 2: Expects waiter role, seeded staff is owner
**Spec:** `expect(waiterCtx.role).toBe("waiter")` (line 12)  
**File:** `e2e/staff/S-06-role-separation.spec.ts`  
**Root cause:** `seedOrg` creates one `initial_owner` (owner role). The spec calls `loginStaff(branch.code, staff.staffCode, staff.pin)` and expects the role to be "waiter". The seeded staff is an owner, not a waiter. Both sub-tests fail because of this role assumption: test 1 expects the waiter to be forbidden from creating staff (but the owner role is allowed to do so), test 2 expects waiter to be forbidden from menu management.  
**Fix:** Either create a dedicated waiter staff member using the owner token, or use a seedOrg variant that returns a waiter.

### G-03: Token revocation via non-existent staff endpoint
**Spec:** POST `/sessions/:id/force-close` with staff token (line 14)  
**File:** `e2e/guest/G-03-revocation.spec.ts`  
**Root cause:** `POST /sessions/:id/force-close` does not exist in the backend. The spec accepts 404 for the close, so the close silently fails. The session remains open. The subsequent cart GET returns 200 (session active), but spec expects `[401, 410, 403]`.

### M-03: Same force-close endpoint issue
**Spec:** Similar revocation via `POST /sessions/:id/force-close` pattern  
**File:** `e2e/multidevice/M-03-revocation-propagation.spec.ts`  
**Root cause:** Same as G-03. Session is never closed, snapshot returns 200, spec expects `[401, 403]`.

---

## Category 8 — Behavioral Issues (Spec or Backend)

### P-09: Overpayment accepted instead of rejected
**Spec:** `expect([400, 409, 422]).toContain(payRes.status)` (line 30)  
**Actual status:** 201 (payment accepted)  
**File:** `e2e/payment/P-09-overpayment-rejection.spec.ts`  
**Root cause:** The backend's payment initiation endpoint does not validate that `amount` exceeds the session's bill total. An overpayment is accepted as a valid payment.

---

## Category 9 — Frontend Tests Requiring Running Frontend (Infrastructure)

These tests use Playwright's browser automation against the Next.js frontend. The frontend server is not running during the backend-focused E2E test run.

| Test | File | Error |
|------|------|-------|
| G-01 | `guest/G-01-fresh-session-and-order.spec.ts` | element(s) not found — `text=Welcome` not visible |
| L-09 | `session/L-09-stale-browser-revisit.spec.ts` | session-ended screen not visible, frontend not serving |
| L-10 | `session/L-10-back-forward-no-mutations.spec.ts` | `text=Welcome, NavUser` not visible within 10s |
| screenshots/sweep | `screenshots/sweep.spec.ts` (staff flow) | page.screenshot: Target page, context or browser has been closed |

**Note:** G-01 and screenshots/sweep#guest passed (they only check API calls), but screenshots/sweep#staff and L-09/L-10 fail because they require the frontend to render specific UI state. These tests are expected to fail in a no-frontend environment.

---

## Summary Table

| Category | Count | Fix Owner |
|----------|-------|-----------|
| Backend returns wrong status (bugs) | 4 | Backend code fix |
| Spec expects wrong status code | 9 | Spec fix |
| Platform token on staff endpoints | 7 | Spec fix |
| `fetchAudit` response shape | 3 | Helper fix |
| Missing webhook secret in env | 7 | Config fix |
| API response shape mismatch | 3 | Spec fix |
| Spec logic issues | 4 | Spec fix |
| Behavioral issues | 1 | Backend or spec |
| Frontend not running | 4 | Infrastructure |
| **Total** | **43** | |

---

## Backend Bugs (Require Code Changes)

1. **X-03** — Guest auth middleware panics/errors with 500 on staff JWT input
2. **O-05 test 2** — Zero-quantity order items cause unhandled 500 (should validate quantity > 0 in handler)
3. **PT-03** — MFA enroll 500 when `MFA_ENCRYPTION_KEY` not set (add to test env, or add graceful error)
4. **T-01** — Cross-org isolation broken: staff token from org A reads org B session snapshot (security issue)
5. **R-06** — Snapshot doesn't return `missed_events` (populated) or `snapshot_authoritative: true` for large sequence gap
6. **P-09** — Overpayment not validated against session bill total

## Config Changes Required

- Add `PAYMENT_WEBHOOK_SECRET_STRIPE=test-webhook-secret` to `backend/.env`
- Add `MFA_ENCRYPTION_KEY=<32-byte-hex>` to `backend/.env` (fixes PT-03)

## Spec Changes Required (No Backend Changes Needed)

- `helpers/api.ts`: `fetchAudit` → extract `.audit` from response object
- `R-01`, `X-07`, `L-03`, `R-03`: accept 201 for ws-ticket
- `M-04`: accept 201 for session join
- `X-05`: add 423 to lockout status list
- `L-05`: add 401 to second-delete status list
- `S-01` test 3: add 200 or enable `AUTH_STAFF_CODE_REQUIRED=true`
- `S-02`, `S-03`, `S-04`: replace `adminToken` with staff token from `seedOrg`
- `A-06`, `OPID-04`, `S-08`: use `seedOrg`'s existing owner staff token, not `adminToken`
- `O-02`: use staff token to mark item unavailable
- `S-06`: create a waiter staff member using owner staff token
- `G-03`, `M-03`: use `DELETE /sessions/:id` with guest token instead of `POST /force-close`
- `O-01`, `O-07`: access `order.Order?.id ?? order.id` to handle wrapped response
- `PT-04`: send `{code, name, restaurant_slug}` instead of `{name, slug}`
- `PT-03`: accept 500 OR add `MFA_ENCRYPTION_KEY` to env (the spec explicitly says `not.toBe(500)`, so env fix is required)
