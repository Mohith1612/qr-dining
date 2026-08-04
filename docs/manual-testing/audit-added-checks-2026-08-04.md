# Manual-testing checks added by the independent audit — 2026-08-04

**Why this is a separate file:** `manual-testing-checklist.html`,
`manual-testing-user-guide.html` and `testing-dashboard.html` all had uncommitted
in-flight edits when this audit ran. Rather than collide with that work, the
checks below are staged here for the checklist author to fold in. Each one covers
a gap the audit found that the current 197-item run sheet does **not** test.

Every check below was reproduced against a throwaway stack during the audit
(scratch Postgres on :15499, scratch Redis on :16399, backend on :18499/:18502,
frontend on :3100). None of it touched the protected soak.

---

## Section 8 · Cross-tenant isolation — ADD these

The existing section tests branch-scoped routes (`/branches/{id}/...`), which are
correctly guarded. It does **not** test the item-scoped routes that carry no branch
in the path. Those are the ones that leak.

| # | Severity | Check | Expected | Actual on `ecfc3a6` |
|---|---|---|---|---|
| 8.9 | **SEV-0** | Authenticate as **owner or manager of Branch A**. `PATCH /menu/categories/{id}` where `{id}` is a category belonging to **a different organization**. | 403 | **200 — the rename persists** |
| 8.10 | **SEV-0** | Same actor, `PATCH /menu/items/{id}/featured` on a **different organization's** item. | 403 | **204 — `is_featured` flips** |
| 8.11 | **SEV-0** | Same actor, `PATCH /menu/items/{id}/availability` on a **different organization's** item. | 403 | **204 — `is_available` flips** |
| 8.12 | **SEV-1** | Same actor, `GET /sessions/{id}/events` for a session in **another organization**. | 403/404 | **200 — full event log returned** |
| 8.13 | **SEV-1** | Same actor, `DELETE /menu/items/{id}` on a foreign item. | 403 | **500** (reaches SQL; leaks existence) |
| 8.14 | **SEV-1** | Same actor, `DELETE /menu/categories/{id}` on a foreign category. | 403 | **409** (reaches business logic; leaks existence) |
| 8.15 | **SEV-0** | **Multi-branch operator case:** manager of Branch A marks a dish unavailable at sibling **Branch B of the same organization**. | 403 | **204 — the dish is 86'd at the wrong branch** |

> **STATUS 2026-08-04 — FIXED.** All rows above now return **403 regardless of the
> flag state**. See `release-certification/authz-scope-investigation-2026-08-04.md`
> for the full re-test. Two corrections to the root cause below: check **8.12**
> (`GET /sessions/{id}/events`) had **no authorization check of any kind** and was
> therefore *not* fixed by flipping the flag; and a foreign owner could also
> `PATCH /staff/{id}/deactivate` any staff member in any organization, which this
> table did not list. Both are now closed.

**Root cause (verified in code, not inferred):** these handlers gate only on
`requireAuthorized(...)`, and `authz.Authorizer.Enforce()` returns
`AUTHZ_CENTRAL_POLICY_ENFORCE`, which ships **`false`** in
`deploy/vm/.env.production.example:52`. In shadow mode
(`backend/internal/handlers/authz.go:54-88`) the policy engine correctly computes
the denial, writes an `AUTHZ_DENIED` audit row with
`reason: "actor branch does not match resource branch"`, and then **allows the
request anyway**. Branch-param routes survive because they have a second,
independent per-handler ownership check; item-scoped routes have no such fallback.

**Reproduce (copy-paste):**

```bash
API=http://localhost:8080
# Owner at Branch A (org 1)
TOKEN=$(curl -s -X POST $API/staff/auth -H 'Content-Type: application/json' \
  -d '{"branch_code":"SAFF-BND","staff_code":"SAFF-BND-OWN","pin":"1111"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")

# FOREIGN_CAT = a menu_categories.id whose branch belongs to a different org
curl -s -o /dev/null -w "%{http_code}\n" -X PATCH \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"CROSS-TENANT-PROOF"}' $API/menu/categories/$FOREIGN_CAT
# then confirm in the DB that the name actually changed
```

**How to verify the fix:** run the table **twice** — once with
`AUTHZ_CENTRAL_POLICY_ENFORCE=false` (the shipped default) and once with `=true`.
Every row must be **403 in both runs**, and the underlying DB row must be unchanged.
Tenant isolation no longer depends on the flag, so a 403 that only appears when the
flag is on is a regression, not a pass.

The permanent guard is
`backend/internal/handlers/authz_scope_integration_test.go`, which asserts exactly
this at both flag settings.

---

## Section 0 · Environment sanity — ADD these

| # | Severity | Check | Why |
|---|---|---|---|
| 0.8 | SEV-1 | Confirm the binary under test was built from the commit named in the certification header (`go version -m <binary>` + `git rev-parse HEAD`). | The soak has been running a **2026-06-10** binary for eight weeks while the RC moved 296 commits ahead. Certifying a binary you did not build from the RC is the failure mode this gate exists to prevent. |
| 0.9 | SEV-1 | `curl -sf $API/health` **and** `curl -sf $API/readyz` both return 200. | `/healthz` does **not** exist (404). Three docs still reference it: `operational-runbooks.md`, `staging-burnin-strategy.md`, `docs/history/manual-local-soak-operations-guide.md`. A healthcheck copied from those docs will always fail. |
| 0.10 | SEV-2 | Confirm the backend was started with `GIN_MODE=release`, or that no `.env` exists in its working directory. | `config.Load()` calls `godotenv.**Overload**()` when `GIN_MODE != release` (`backend/internal/config/config.go:137-139`) — a stray `.env` silently **overrides injected environment variables**, including `DATABASE_URL`. Correctly disabled in release mode; a live trap in staging/debug. |

---

## Section 10 · Observability — ADD these

| # | Severity | Check | Why |
|---|---|---|---|
| 10.11 | SEV-0 | Fire a real alert and confirm a **human receives it** on the channel that will be used in production. | `deploy/observability/alertmanager.yml:32` still points at `http://CHANGEME-notify-endpoint:5001/alerts`. All 27 rules currently page into a void. |
| 10.12 | SEV-2 | Confirm `legacy_authz_bypass_total` is **0** across the whole run. | Since the 2026-08-04 fix this counter tracks **role-policy** shadow bypasses only — tenant scope denials are enforced unconditionally and land in `authz_denied_total{reason="branch_mismatch"\|"org_mismatch"}` instead. A non-zero value still means the R3 enforcement flip will break a working flow, so it remains the pre-flip risk signal. |

---

## Section 11 · Regression sweep — ADD these (API contract)

The OpenAPI certification verified that every path+method exists. It never
compared **response bodies**. Two core guest-loop endpoints are documented wrong:

| # | Severity | Check | Documented | Actual |
|---|---|---|---|---|
| 11.10 | SEV-1 | `POST /sessions/{id}/orders` → 201 body keys | `{ "order": …, "order_items": […] }` | **`{ "Order": …, "OrderItems": […] }`** |
| 11.11 | SEV-1 | `GET /sessions/{id}/cart` → 200 body | array of `CartItem` | **`{ "Cart": …, "Items": […] }`** |

Cause: `services.PlaceOrderResult` (`backend/internal/services/order.go:58-61`) and
the cart equivalent are returned straight to `c.JSON` with **no struct tags**, so Go
serializes the exported field names. The frontend already works around it —
`frontend/lib/api/orders.ts:32` maps `r.Order → order`. The product is fine; the
**published contract is wrong**, so any generated client breaks.

Also check the same class on the other two untagged returns:
`internal/handlers/cart.go:54`, `internal/handlers/loyalty.go:132`,
`internal/handlers/platform.go:171`.

### Routes implemented but absent from `openapi.yaml` (2.2.0)

Verified by diffing the live Gin router (177 routes) against the spec (171 operations):

```
POST   /sessions/{id}/host
PATCH  /branches/{id}/promos/{promo_id}
POST   /branches/{id}/promos/{promo_id}/activate
PATCH  /tables/{id}
DELETE /tables/{id}
PATCH  /platform/branches/{branch_id}
```

Add a route-vs-spec diff to CI so this cannot drift again.

---

## Section 9 · Failure & recovery — ADD this

| # | Severity | Check | Why |
|---|---|---|---|
| 9.26 | SEV-1 | Run the Playwright suite and record pass/fail counts in the certification record. | The suite is **not runnable as documented**: `E2E_ADMIN_TOKEN` defaults to `"e2e-admin-secret"` (`e2e/helpers/api.ts:5`), a value the backend has never accepted — `PlatformAuth` only validates real platform session tokens. Every spec that calls `seedOrg()` dies at setup. It also needs raised rate limits. Correctly configured, the desktop project scores **93 passed / 34 failed**; the 34 are predominantly test rot (stale response shapes, fixture role drift), not product regressions. CI only runs `playwright test --list`, which never executes a test. |

**Working invocation** (record this in the guide):

```bash
TOK=$(curl -s -X POST $API/platform/auth -H 'Content-Type: application/json' \
  -d '{"email":"admin@platform.local","password":"Platform!admin1"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
# backend must be started with RATE_LIMIT_RPM and AUTH_RATE_LIMIT_RPM raised
E2E_ADMIN_TOKEN=$TOK API_URL=$API APP_URL=$APP npx playwright test --project=desktop
```

---

## Do not trust `e2e/screenshots/` as visual evidence

`e2e/screenshots/sweep.spec.ts` seeds sessions over the API and then navigates the
browser straight to `/session/{id}/...`. The browser has no guest credential in
`sessionStorage`, so every authenticated surface redirects to the public landing
page — and the sweep screenshots it anyway.

Verified on this run: all seven files in `e2e/screenshots/desktop/guest/`
(`01-landing` … `07-payment-pending`) are **byte-identical**, MD5
`ed843af6f6b9f45f56ffa21feeeb1976` — seven copies of the landing page presented as
a guest-flow walkthrough. Mobile and tablet produced nothing at all.

Real captures taken by driving the QR flow through the UI are in
`e2e/screenshots/audit-2026-08-04/`. Fix the sweep to enter via
`/table/{qr_token}` and complete the join form before adding it back to any
evidence pack.
