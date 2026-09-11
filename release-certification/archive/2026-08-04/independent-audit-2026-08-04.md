# Independent Release Audit — qr-dining

**Date:** 2026-08-04
**Auditor role:** Principal Engineer / Release Manager / QA Lead (independent review)
**Commit audited:** `feature/signoz-observability` @ `ecfc3a6` (strict superset of the RC lineage `feature/certification-fixes-ui-redesign` @ `664d412`)
**Question:** *Can I confidently onboard Restaurant #1?*
**Answer:** **No — not today.** See §12.

Every claim below was reproduced during this audit. Prior reports, certifications
and memories were treated as unverified until independently re-tested. Where a
prior claim held up, that is stated. Where it did not, that is stated too.

**Isolation:** all testing ran against a throwaway Postgres (:15499) and Redis
(:16399) created for this audit, with the backend on :18499/:18502 and a frontend
build on :3100. The protected soak (`qr-app-soak`, `qr-dining-postgres-1`) and the
manual-testing stack (:8090/:8095/:3000/:3001) were **read-only throughout**. No
merges, no deploys, no product-logic changes.

---

## 1. What I verified green

These are not inherited claims — I ran them.

| Check | Command | Result |
|---|---|---|
| Backend build | `go build ./...` | **clean** (4.5s) |
| Static analysis | `go vet ./...` | **clean** |
| Formatting | `gofmt -l .` | **clean** |
| Full test suite | `go test -count=1 -p 1 -tags integration ./...` with real PG + Redis | **12/12 packages ok, 0 skips** |
| Same, with race detector | `... -race ...` | **12/12 packages ok** |
| Migrations | up → `down -all` → up on a virgin DB (38 migrations) | **clean round-trip** |
| Frontend build | `next build` | **clean**, 0 type errors, 0 lint errors |
| OpenAPI validity | `npx @redocly/cli lint openapi.yaml` | **valid**, 0 errors, 9 warnings |
| Route extraction | live Gin router vs. static parse of `server.go` | **177 routes, identical** |

**The prior certification's headline test claim is accurate.** `automated-test-report.md`
says the integration suite is "PASS — 12 packages ok, 0 skips" under
`-tags integration -count=1 -p 1`. I reproduced exactly that, and additionally
under `-race`. Credit where due.

I also confirmed the failure mode behind the old "red integration suite" note: the
suite fails **only** when package binaries run in parallel, because all packages
share one database and one Redis with overlapping fixtures. Deadlocks and FK
violations under default `-p` are test-isolation artifacts, not product bugs.
`README.md` documented the command **without** `-p 1` — i.e. it documented the
failing invocation. Fixed in this audit.

---

## 2. BLOCKER — Cross-organization and cross-branch write bypass

**Severity: SEV-0** (absolute blocker for multi-tenant SaaS; blocker for any pilot
restaurant with more than one branch).

### Evidence

Seeded three independent tenants. Authenticated as **owner of Saffron House /
Bandra (branch 166, restaurant 163, org 175)**. Targeted objects owned by **Copper
Pot Kitchen / Koramangala (branch 169, restaurant 164, org 176)** — a different
organization, restaurant, and branch.

Baseline set directly in SQL, then mutated only through the API:

```
BEFORE   menu_items.207     is_available=t  is_featured=f
         menu_categories.177 name='Starters'          (both belong to org 176)

PATCH /menu/items/207/featured        -> 204
PATCH /menu/items/207/availability    -> 204
PATCH /menu/categories/177            -> 200

AFTER    menu_items.207     is_available=f  is_featured=t
         menu_categories.177 name='OWNED-BY-SAFFRON'
```

Additional confirmed leaks with the same foreign token:

| Route | Result | Impact |
|---|---|---|
| `GET /sessions/{id}/events` (foreign session) | **200** | full event log of another org's dining session |
| `DELETE /menu/items/{id}` (foreign) | **500** | reaches SQL; confirms existence |
| `DELETE /menu/categories/{id}` (foreign) | **409** | reaches business logic; confirms existence |

**And the case that bites a single-tenant pilot:** a **manager** at Bandra
successfully renamed a category and marked a dish unavailable at **Indiranagar —
a sibling branch of the same organization** (`PATCH /menu/categories/167` → 200;
`PATCH /menu/items/180/availability` → 204, `is_available` t→f). A manager at one
location can 86 a dish at another location.

### Root cause (read from code, not inferred)

`backend/internal/handlers/authz.go:47-88` — `requireAuthorized()` computes the
decision, and when `Enforce()` is false it writes an `AUTHZ_DENIED` audit row and
**returns true anyway**. `Enforce()` is wired to `AUTHZ_CENTRAL_POLICY_ENFORCE`,
which ships **`false`** at `deploy/vm/.env.production.example:52`.

The policy engine is *correct* — I pulled the audit row it wrote:

```json
{"action":"order.status.serve","reason":"actor branch does not match resource branch"}
```

It detects every one of these violations and then lets them through.

Branch-param routes (`/branches/{id}/...`) survive because they carry a second,
independent per-handler ownership check — I verified those return **403** across
orders, sessions, assist, payments, menu, staff, tables, audit and customers.
Item-scoped routes (`/menu/items/{id}`, `/menu/categories/{id}`,
`/sessions/{id}/events`) have **no such fallback**. `enforceBranchScopeFromBody()`
(`internal/handlers/helpers.go:49`) does not help: it compares a *client-supplied
body* `branch_id` against the resource, never the authenticated actor's branch.

### Why the previous assessment was wrong

`final-pilot-readiness-report.md` characterised this as SEV-1: *"intra-branch role
boundaries rely on per-handler checks, not central enforcement… acceptable for a
pilot where all staff are trusted employees."*

Both halves are wrong:

1. It is not "intra-branch role boundaries" — it is **cross-organization tenant
   isolation**, the single property a multi-tenant SaaS cannot get wrong.
2. "Per-handler checks" **do not exist** on the affected routes. That is precisely
   why they leak. The report assumed a defence-in-depth layer that isn't there.

Role separation itself *is* enforced — a waiter was correctly 403'd on the same
routes. The gap is specifically **scope**, not **role**.

### Fix

Flip `AUTHZ_CENTRAL_POLICY_ENFORCE=true` (and its pair
`STRICT_BRANCH_SCOPED_MUTATIONS=true`) — **and prove it**, because it has never
been demonstrated end-to-end. Watch `legacy_authz_bypass_total` first: it counts
exactly these bypasses and tells you what will break on the flip. Nothing
currently reads it.

Belt-and-braces: add an explicit actor-branch/org check to the item-scoped
handlers so tenant isolation does not depend on a rollout flag at all.

**Blocks pilot?** If Restaurant #1 is a **single branch in a database with no other
tenants**, this is not exploitable and does not block. Any second branch, or any
second tenant on the same instance, and it is an immediate SEV-0.

---

## 3. BLOCKER — There is no production deployment

The working assumption going in was that deployments are already running. **They
are not.** What is running is local Docker: a manual-testing stack, a soak
container, and a local SigNoz evaluation stack.

| Artifact | Expected | Actual |
|---|---|---|
| Backend image `ghcr.io/mohith1612/qr-dining` | pushed by CI on main/tags | `GET /users/Mohith1612/packages/container/qr-dining` → **404 Package not found** |
| Production domain | purchased | **none** — 26 `CHANGEME` placeholders across `DEPLOYMENT.md`, `.env.production.example`, nginx conf, frontend env, alertmanager |
| Alert receiver | real channel | `http://CHANGEME-notify-endpoint:5001/alerts` — 27 rules page into a void |
| R2 credentials | live round-trip proven | `__REPLACE__` placeholders; the June SEV-1 "real R2 round-trip" is still open |

`DEPLOYMENT.md` is honest about this ("no domain is purchased yet"), but the
practical consequence is blunt: **there is nothing to onboard a restaurant onto**,
and the documented deploy path (`docker compose pull` an image tag) cannot execute
because the image has never been built.

---

## 4. BLOCKER — CI has never run. Not once.

```
$ gh api /repos/Mohith1612/qr-dining/actions/runs --jq .total_count
0
$ gh pr list --state all
(empty)
```

`ci.yml` is on `main` and registered as active — with **zero runs in the
repository's history**. Both workflows trigger only on `push`/`pull_request` to
`main`. All 187 RC commits live on feature branches, no PR has ever been opened,
and nothing has been pushed to `main` since CI was added. `frontend-ci.yml` exists
only on feature branches, so GitHub has never even registered it as a workflow.

Every quality gate in those files — lint, race tests, sqlc drift, migration
up/down, Docker build — has **never executed in CI**. The green-CI-badge model of
confidence does not apply here; nothing has been gated.

**And the gate is weaker than it looks.** `ci.yml`'s test job runs
`go test -count=1 -race ./...` with no Postgres or Redis service and no
`-tags integration`. Even once it runs, it will **silently skip the entire
integration suite** and report green. Measured statement coverage of the
non-integration path: **4.7%**.

Packages with **zero** test files include `internal/middleware` (auth, tenancy,
rate limiting, CORS, security headers), `internal/worker` (all six background
workers), `internal/server`, `internal/observability`, `internal/storage`,
`internal/events`.

---

## 5. BLOCKER — The RC is unmerged, untagged, and unsoaked

- `main` is at `2ac5ba3` (**2026-06-30**), **187 commits behind** the RC.
- The only tag in the repo is `redesign-foundation`. No `v1.0.0-rc.1`, no release tag.
- The soak container `qr-app-soak` bind-mounts `/home/mohith/qr-dining-soak/qrapp`,
  a binary with mtime **2026-06-10 00:29**. It has served 35,507 `/readyz` probes
  since — proving the *container* is healthy and proving **nothing about the RC**.

The June report named this exactly right: *"the soaked binary ≠ the pilot binary"*,
SEV-0. That was ~8 weeks ago. It is still open, and the gap has widened to include
the entire two-track UI redesign, ~18 certification fixes, migrations 036–038, and
the OpenTelemetry instrumentation.

The in-flight `rc-manual-certification-plan.md` (dated today) states this clearly
and is the right instinct. One correction: it records **"Routes: 172"**. The live
Gin router registers **177**.

---

## 6. HIGH — OpenAPI is wrong about the two most-used endpoints

The spec is structurally valid and passes Redocly. It is also **factually wrong**
on the guest loop's two hottest calls:

| Endpoint | OpenAPI 2.2.0 says | Server actually returns |
|---|---|---|
| `POST /sessions/{id}/orders` (201) | `{ "order": …, "order_items": […] }` | **`{ "Order": …, "OrderItems": […] }`** |
| `GET /sessions/{id}/cart` (200) | array of `CartItem` | **`{ "Cart": …, "Items": […] }`** |

Cause: `services.PlaceOrderResult` (`internal/services/order.go:58-61`) and the
cart result are handed straight to `c.JSON` with **no struct tags**, so Go emits
the Go field names. The frontend already compensates —
`frontend/lib/api/orders.ts:32` maps `r.Order → order` — which means the workaround
has been sitting in the codebase documenting the bug.

Same untagged-return pattern at `internal/handlers/cart.go:54`,
`loyalty.go:132`, `platform.go:171` — audit those too.

**Six routes are implemented but undocumented** (live router 177 vs. spec 171):

```
POST   /sessions/{id}/host                        (added 2026-07-20)
PATCH  /branches/{id}/promos/{promo_id}           (added 2026-08-02)
POST   /branches/{id}/promos/{promo_id}/activate  (added 2026-08-02)
PATCH  /tables/{id}                               (added 2026-08-02)
DELETE /tables/{id}                               (added 2026-08-02)
PATCH  /platform/branches/{branch_id}             (added 2026-08-02)
```

**What this says about the prior certification method.** The OpenAPI certification
proved *path + method* parity and stopped there. It never diffed a single response
body. A spec can be 100% route-complete and still lie about every payload — which
is what happened. Any generated client breaks on order placement, the most
important write in the product.

---

## 7. HIGH — The Playwright suite is not runnable as documented

106 spec files, 381 tests. It looks like the strongest asset in the repo. It is
not currently a gate.

- `e2e/helpers/api.ts:5` — `E2E_ADMIN_TOKEN` defaults to `"e2e-admin-secret"`, a
  static bearer token the backend **has never accepted**. `middleware.PlatformAuth`
  validates only real platform session tokens; the string `e2e-admin-secret`
  appears nowhere in the Go source. Every spec routed through `seedOrg()` dies at
  setup with 401.
- Default rate limits (`RATE_LIMIT_RPM=60`, `AUTH_RATE_LIMIT_RPM=10`) cannot
  sustain a suite that provisions an org per test — the rest die with 429.
- With the default config: **8 of 127** desktop tests pass.
- With a real platform token **and** raised rate limits: **93 passed / 34 failed.**

The 34 failures are predominantly **test rot**, not product regressions — I traced
three: `O-01` asserts `order.id` against the `{Order:…}` shape from §6; `S-06`
asserts `role === "waiter"` on a fixture that now seeds an *owner*; a third
asserts on a menu-item id that no longer exists. A handful (WS-ticket rate limit,
brute-force lockout) failed *because* I raised the rate limits to run the suite.

**CI's only e2e gate is `npx playwright test --list`** — test *discovery*. It
passes whether or not a single test would run. That is how a suite this large
rotted invisibly.

---

## 8. HIGH — The screenshot evidence is not evidence

`e2e/screenshots/sweep.spec.ts` seeds sessions over the API, then points the
browser at `/session/{id}/...`. The browser holds no guest credential, so every
authenticated surface redirects to the public landing page — and the sweep
screenshots it regardless.

All seven files in `e2e/screenshots/desktop/guest/` (`01-landing` …
`07-payment-pending`) are **byte-identical**, MD5
`ed843af6f6b9f45f56ffa21feeeb1976`. Seven copies of the landing page, filed as a
guest-flow walkthrough. Mobile and tablet produced nothing.

Anyone reviewing that folder would conclude the guest flow had been visually
verified. It never was.

**Real captures** — taken by driving the actual QR flow through the UI — are in
`e2e/screenshots/audit-2026-08-04/`. They show a genuinely well-built product: see §9.

---

## 9. What is genuinely strong

I want to be clear that the problems above are release-engineering problems, not a
weak product.

- **The guest UI is excellent.** Driven live through `/table/{qr_token}` → join →
  session → menu: correct tenant theming (dark-luxury for Saffron House), live
  presence ("in sync", participant avatars, host badge), real dish photography,
  a persistent order bar, sensible bottom navigation, dietary filters. This is
  well above typical internal-tool quality.
- **Release-mode config guardrails are excellent.** `validateReleaseSecurity()`
  refuses to boot on the dev guest-token secret, on a short secret, on empty CORS,
  on a malformed MFA key, on a short webhook secret — each with an actionable
  message. Dotenv override is disabled in release mode specifically so a stray
  `.env` cannot poison production.
- **Payment webhook verification is correct.** Fails closed when no secret is
  configured; bidirectional timestamp tolerance (replay *and* future-dated);
  `hmac.Equal` constant-time comparison over `timestamp + "." + body`.
- **Branch-param tenancy is real.** 403 across every branch-scoped route I probed.
- **Role separation is real.** Waiter correctly 403'd on menu and staff management.
- **The authz policy engine is correct.** It computes every denial in §2
  accurately and audits it. Only the enforcement switch is off.
- **Architecture is clean and legible.** 177 routes in a 506-line `server.go` with
  explicit trust domains (guest / staff / platform), per-endpoint rate limits with
  reasoning in comments, fail-closed sensitive limiters, host-authority
  centralised on one service.
- **Migrations are disciplined** — 38 pairs, every one reversible, verified.
- **Operational tooling exists and is real** — backup/restore scripts, 27 alert
  rules, Prometheus/Alertmanager/blackbox compose, systemd timers, runbooks.

---

## 10. Medium / low findings

| # | Sev | Finding | Evidence |
|---|---|---|---|
| M-1 | SEV-2 | `/healthz` returns **404**. Three docs reference it (`operational-runbooks.md`, `staging-burnin-strategy.md`, `docs/history/manual-local-soak-operations-guide.md`). Real endpoints are `/health` and `/readyz`. | `curl :8090/healthz` → 404 |
| M-2 | SEV-2 | `godotenv.Overload()` in non-release mode **overrides injected env vars** — a stray `.env` silently redirects `DATABASE_URL`. Correctly disabled in release; a live trap in debug/staging. | `config.go:137-139` |
| M-3 | SEV-2 | Migrations auto-apply at boot (`main.go` step 5). Fine for single-instance; needs thought before multi-replica. | `main.go:54` |
| M-4 | SEV-2 | 560 `@typescript-eslint/no-unused-vars` warnings. Build is clean (0 errors) but the lint signal is drowned. | `npx eslint . -f json` |
| M-5 | SEV-2 | No CI check for route↔OpenAPI drift — which is why §6 happened silently. | — |
| M-6 | SEV-3 | `openapi.yaml` has 9 Redocly warnings (missing 4XX responses on several operations). | `redocly lint` |
| M-7 | SEV-2 | `PAYMENT_WEBHOOK_SECRET_*` is length-checked if present but never *required* in release mode. Webhook auth then fails closed (safe), but the failure is silent at boot. | `config.go:299-303` |

---

## 11. Scores

| Dimension | Score | Reasoning |
|---|---:|---|
| **Production readiness** | **22 / 100** | Nothing is deployed. No image, no domain, no live alert receiver, no R2 round-trip, CI never executed. The software may be closer than this number suggests; the *production system* does not exist. |
| **Pilot readiness** | **55 / 100** | The product itself is credible and the operational tooling is built. But the RC has never been soaked, has never been merged or tagged, and the path to running it in front of a paying customer has not been walked once end to end. |
| **Public SaaS readiness** | **18 / 100** | §2 alone disqualifies it: cross-organization writes succeed under shipped defaults. Add shadow-only billing, `TENANCY_ORGANIZATIONS_ENABLED=false`, no signup, no self-serve provisioning. |

---

## 12. Verdict

# NO GO

**for onboarding Restaurant #1 today.**

This is not a judgement on code quality. The engineering core is genuinely good —
better than most systems at this stage. The blockers are that **the release has
never been assembled**:

1. No deployable artifact exists (GHCR 404).
2. No environment exists to deploy it to (no domain, placeholders throughout).
3. No CI run has ever validated any commit.
4. The RC is unmerged, untagged, and unsoaked — the same SEV-0 as eight weeks ago.
5. Alerts page into a void.
6. Cross-organization writes succeed under the shipped configuration.

None of these is deep or expensive. Together they mean that on the night
Restaurant #1 serves its first table, **every operational path would be running
for the first time**. That is the specific risk this audit exists to prevent.

**Path to CONDITIONAL GO** — realistically 1–2 focused weeks. See §13.

---

## 13. What I would do next, in order

**Week 1 — make the release real**

1. **Open a PR from the RC to `main`.** This alone fires CI for the first time in
   the project's history. Expect it to find things. Do not skip straight to merge.
2. **Fix the CI test job** before trusting it: add Postgres + Redis services and
   run `go test -count=1 -p 1 -race -tags integration ./...`. Right now it would
   pass while skipping every integration test.
3. **Add a route↔OpenAPI drift check to CI** (177 vs 171 today). Same script that
   produced §6.
4. **Fix the two response contracts** — add JSON tags to `PlaceOrderResult` and the
   cart result, document the 6 missing routes, then remove the
   `r.Order → order` workaround in `frontend/lib/api/orders.ts`. Do this *before*
   any external client exists.
5. **Merge and tag `v1.0.0-rc.1`.** Let CI build and push the arm64 image. Verify
   `ghcr.io/mohith1612/qr-dining:v1.0.0-rc.1` actually exists.

**Week 1 — close the security gap**

6. **Turn on `legacy_authz_bypass_total`** and drive the full manual checklist with
   it. Anything non-zero is a flow that breaks on the flip.
7. **Flip `AUTHZ_CENTRAL_POLICY_ENFORCE=true` + `STRICT_BRANCH_SCOPED_MUTATIONS=true`**,
   then re-run the §2 matrix and require **403 on every row**. Add those seven
   cases as permanent regression tests.
8. Add explicit actor-scope checks to the item-scoped handlers so isolation does
   not depend on a flag.

**Week 2 — walk the operational path once**

9. **Buy the domain.** Every `CHANGEME` is downstream of this one decision.
10. Deploy the tagged image to the VM. Point Alertmanager at a **real** channel and
    fire one alert you actually receive on your phone.
11. Run one `nightly-backup.sh` against the **production R2 bucket** and restore
    from the downloaded object. That closes the last June SEV-1.
12. **Start the soak on the tagged RC binary** and let it run its full window.
    Verify with `go version -m` that the soaked binary is the tagged one — that is
    the check that has failed twice now.

**Week 2 — restore the test assets**

13. Repair the Playwright suite: replace the fake `E2E_ADMIN_TOKEN` with real
    platform auth in `helpers/api.ts`, fix the ~34 rotted assertions, and change CI
    from `--list` to an actual run against a compose stack.
14. Fix `sweep.spec.ts` to enter via `/table/{qr_token}` and complete the join form,
    then regenerate screenshots. Delete the seven identical files first.

**Then, and only then**, run the 197-item manual certification in
`rc-manual-certification-plan.md` — with the additions in
`docs/manual-testing/audit-added-checks-2026-08-04.md` folded in. That plan is
good work; it deserves to run against a real RC on real infrastructure rather than
against a laptop.

**Deferred until after Restaurant #1 is stable:** raising unit coverage off 4.7%
(prioritise `internal/middleware` and `internal/worker`), the 560 lint warnings,
`TENANCY_ORGANIZATIONS_ENABLED`, and real billing. None of these should hold the
pilot; all of them should hold public signup.

---

## Appendix — artifacts produced by this audit

- `docs/manual-testing/audit-added-checks-2026-08-04.md` — 16 missing manual checks, reproducible
- `e2e/screenshots/audit-2026-08-04/` — real guest-flow captures (mobile)
- `README.md` — corrected the integration-test invocation (was documenting the failing command) and noted the CI gap
