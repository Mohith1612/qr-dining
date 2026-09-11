# QR Dining Manual Certification Preflight — 2026-08-22

## Exact URLs and Entry Points

| Surface | URL | Purpose |
|---|---|---|
| Guest | <https://qr-beta.mohith16.com/table/ae8670b51c04907372fa8c621bea8c6644b3b9a6b0bbc73a1b5d1caa1e36e6f8> | Saffron Bandra T3 QR ordering; currently available |
| Platform | <https://qr-beta.mohith16.com/platform/login> | Platform administration |
| Staff | <https://qr-beta.mohith16.com/staff/login> | Owner, manager, kitchen and waiter login |
| API | <https://qr-api-beta.mohith16.com> | Client-IP-balanced API |
| API instance 1 | <https://qr-api-beta-1.mohith16.com> | Deterministic backend 1 testing |
| API instance 2 | <https://qr-api-beta-2.mohith16.com> | Deterministic backend 2 testing |
| API readiness 1 / 2 | <https://qr-api-beta-1.mohith16.com/readyz> · <https://qr-api-beta-2.mohith16.com/readyz> | PostgreSQL/Redis readiness |
| SigNoz | <http://127.0.0.1:3301/login> | Traces/logs after `ssh -L 3301:127.0.0.1:3301 appuser` |
| Testing dashboard | `docs/manual-testing/testing-dashboard.html` | QR links and intentionally published tester credentials |
| Local frontend A / B | <http://localhost:3000> · <http://localhost:3001> | Local two-instance support stack |
| Local backend A / B | <http://localhost:8090/readyz> · <http://localhost:8095/readyz> | Local deterministic backends |

Login entry points:

- Guest: no login; open a current `/table/<qr_token>` link and enter a display name.
- Owner/manager/waiter/kitchen: `/staff/login`; enter branch code, staff code and PIN. The beta rejects PIN-only login. Use the intentionally published values in the testing dashboard; this report does not duplicate passwords/PINs.
- Owner and manager land at `/staff/admin`; waiter at `/staff/waiter`; kitchen at `/staff/kitchen`.
- Platform roles: `/platform/login`; use the intentionally published super-admin/support/billing/auditor accounts in the testing dashboard/readiness document. The dedicated beta admin requires MFA enrollment.

## Certification Readiness

**BLOCKED.** The beta product surfaces are usable for exploratory/manual testing, but a formal release-certification run must not be signed off yet:

1. The certification checkout is `9865a48`, four commits ahead of GitHub, while both beta backend containers run image `qr-dining:beta-b57f746` from repository commit `b57f746`. The frontend artifact has no commit label and cannot be proven to be the same candidate as either commit.
2. SigNoz is reachable, but ClickHouse is exceeding its 1.35 GiB limit. The collector is rejecting/dropping logs, traces and metrics, and the live preflight request could not be found.
3. The exact deployed backend binary uses Go `1.26.5`; its exact scan reports nine reachable vulnerabilities (see Automated Preflight Results).
4. Alertmanager routes only to the local alert-sink container, not to a human/on-call channel.

No product code, production state, DNS, release tag, branch, PR, protected soak, or shared-VM application was changed during this preflight.

## Current Certification State

| Item | Verified state |
|---|---|
| Branch / local HEAD | `feature/signoz-observability` / `9865a488709464296f888bcba9476a79b3f5423f` |
| Working tree | Four pre-existing/unrelated untracked documentation paths and concurrent changes under `marketing/`; preserved. Evidence is stored in an ignored screenshot directory. |
| Pushed | **No.** Local branch is four commits ahead of `origin/feature/signoz-observability`; GitHub does not know `9865a48`. |
| Merged | **No.** PR #1 is open and cleanly mergeable, but its head is `b57f746`. |
| Tagged | **No.** No tag contains local HEAD. |
| Beta VM build | Repository and backend image at `b57f746`; image `sha256:1b2ef5ad…`, started 2026-08-12. This is not local HEAD. |
| Frontend build | Cloudflare-hosted frontend is live, but its deployed artifact has no verifiable commit label. Prior documentation says it includes local-only PostHog work; exact provenance remains unproven. |
| Frontend | HTTPS 200; real page renders. HTTP redirects to HTTPS. |
| API / instances | Balanced and both pinned APIs return 200 for `/health` and `/readyz`. |
| PostgreSQL | Healthy; schema v39, dirty=false; seeded multi-tenant data present. |
| Redis | Healthy; shared by both backends; `PING` succeeds. |
| WebSockets | WSS ticket flow, ping/pong, instance-1→instance-2 and instance-2→instance-1 `CART_UPDATED` propagation passed. |
| R2 | App bucket and backup bucket accessible; live presign/PUT/GET byte round trip passed; latest backup checksum matches manifest. |
| Nginx / Cloudflare | Config valid; API DNS-only/direct to nginx; frontend Cloudflare-proxied. Routing and forwarding work. |
| SigNoz | UI/health up, but ingestion/query path is not functional due ClickHouse memory exhaustion. |
| Can manual certification begin? | Exploratory functional testing may begin. **Formal certification is blocked** until one immutable candidate is named/deployed and SigNoz/security gates are cleared. |

## Verified

- Beta DNS, redirects, HTTPS and certificate chains: frontend certificate valid through 2026-11-02; API wildcard valid through 2026-10-27.
- Both backend instances are healthy, use the same PostgreSQL/Redis, and use `WORKER_REGION=beta`.
- Real client IP forwarding: an intentionally spoofed inbound `X-Forwarded-For` was not trusted. Application logs and login audit rows recorded the real tester IP, not nginx/Docker and not the spoofed address.
- `Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto` and request IDs are set/preserved by effective nginx configuration. `nginx -t` passed.
- CORS permits the beta frontend origin with credentials and rejects an unrelated origin. API security headers and HSTS are present. Public `/metrics` returns 403.
- Rate limiting is active. A 100-request burst to a normal API route yielded 60×200, 21×429 and 19×503. The mixed rejection codes are a warning below.
- Real seeded owner, manager, waiter and kitchen credentials authenticate against beta with correct role/branch; platform seeded super-admin authenticates. No secret tokens were recorded.
- Cross-instance state proof: create through instance 1; join/read through instance 2; cart mutations in both directions; each peer observed shared state. The session was closed afterwards.
- Cross-instance realtime proof: WSS connected to instance 2 and received an instance-1 cart event, then WSS connected to instance 1 and received an instance-2 cart event. Both instances reported the expected WS counters.
- Recent worker logs show both instances participating without a simultaneously duplicated dangerous execution. Existing stalled-payment escalation repeats periodically by design and does not mutate payment/session state.
- Beta data is usable: 3 organizations, 5 branches, 15 tables, 25 menu categories, 71 available menu items, 18 active staff, 3 organization owners, 5 platform users, 2 product flags (default off), 3 plans, 14 entitlements, 3 organization subscriptions and 3 restaurant subscriptions.
- Subscription mapping is coherent: Saffron/Premium/active, Copper Pot/Standard/active, Urban Brew/Free/trial.
- R2 credentials are supplied only via ignored/runtime environment files; no access key was printed or found committed. The latest backup (2026-08-21 21:00 UTC) is present; its downloaded SHA-256 exactly matches the manifest. Retention is 14 days.
- Local isolated manual stack is left running: PostgreSQL `:25432`, Redis `:26379`, backends `:8090/:8095`, frontends `:3000/:3001`. It was started without reset and did not touch compose project `qr-dining`.

## Failed

| Check | Classification | Result |
|---|---|---|
| Immutable candidate provenance | Release/environment failure | Local HEAD, GitHub PR head, beta backend image and frontend artifact are not one provable commit. |
| Fresh SigNoz trace/log lookup | Environment failure | Live request ID `preflight-signoz-live-20260822`, trace `3e08a5d7490d8dd1f808b23648281c23`, returned API 200 but cannot be located; ClickHouse queries themselves fail with code 241 memory exhaustion. |
| Telemetry ingestion | Environment failure | Collector reports retry, queue-full rejection and dropped items for traces/logs/metrics. |
| Human alert delivery | Environment/configuration failure | Effective Alertmanager receivers point only to `http://alert-sink:5001/alerts`; no human/on-call channel is wired. |
| Vulnerability gate | Likely release/security defect | The exact deployed beta binary scan found 9 reachable vulnerabilities. No remediation was attempted. |

## Environment Problems

1. **SigNoz/ClickHouse — blocking.** UI container health is green, but ClickHouse reaches its 1.35 GiB memory ceiling. The collector is dropping telemetry. A green login page is not evidence that observability works.
2. **Candidate identity — blocking.** `9865a48` is unpushed and not the beta backend. The frontend artifact is not labeled with a source SHA.
3. **Rate-limit response split — warning.** Application limiting returns a structured 429 with `Retry-After: 60`, while nginx burst exhaustion returns its default 503. Manual checks must verify the guest/staff UX for both paths.
4. **Human alert delivery — blocking for release.** Prometheus targets/rules and Alertmanager are up, but both effective receivers route to the local `alert-sink` container. A firing alert is not delivered to a person.
5. **Documentation/dashboard drift — addressed after preflight.** The preflight found stale branch, migration, table-state, localhost, PIN-only and SigNoz labels. The beta dashboard, session guide, user guide and interactive checklist were updated on 2026-08-22; `how-to-use.html` is now explicitly labeled local-only.
6. **Retention sizing — warning.** SigNoz tables use 15-day retention on a shared, memory-constrained VM, while existing beta guidance proposed 7 days. This needs an operator decision, not an in-session change.

## Suspected Product / Release Defects

### SEC-01 — deployed Go toolchain advisories

- Severity: High pending security triage.
- Reproduction: run `govulncheck` against the exact `/app/qr-dining` binary from `qr_dining_app`.
- Expected: zero reachable known vulnerabilities for the release candidate.
- Actual: the exact beta binary scan reports 9 reachable vulnerabilities: GO-2026-6218, GO-2026-6091, GO-2026-6090, GO-2026-6089, GO-2026-6088, GO-2026-5972, GO-2026-5942, GO-2026-5932 and GO-2026-5026. Its embedded Go version is `go1.26.5`; most listed standard-library fixes require `go1.26.6`, and `x/net` GO-2026-5942 requires v0.56.0.
- Affected role/tenant: all backend users and tenants.
- Evidence: deployed binary module metadata plus vulnerability output from this run.
- Relevant component: backend build image/toolchain, not one UI route.

### WEB-01 — frontend CSP blocks Cloudflare Analytics beacon

- Severity: Low.
- Reproduction: open the beta landing page with browser console visible.
- Expected: configured first-party/edge analytics loads without CSP violations, or is intentionally omitted.
- Actual: `static.cloudflareinsights.com/beacon.min.js` is blocked by `script-src` CSP; an uncaught console error is emitted. The page otherwise loads.
- Affected role/tenant: all frontend users/all tenants.
- Evidence: browser console during `01-beta-frontend-landing.png` capture.
- Relevant component: frontend security headers / Cloudflare Analytics injection.

### DOC-01 — published API contract remains incomplete

- Severity: Medium documentation/client-generation defect.
- Reproduction: compare live Gin routes with `openapi.yaml`.
- Expected: every live operation is documented with correct response shape.
- Actual: the spec validates with 9 warnings, but still omits `POST /sessions/{id}/host`, `PATCH /tables/{id}`, and `DELETE /tables/{id}`. Audit addenda also document stale cart/order response shapes.
- Affected role/tenant: API consumers/all tenants.
- Evidence: static route/spec comparison; `docs/manual-testing/audit-added-checks-2026-08-04.md`.
- Relevant component: `openapi.yaml` and server route registration.

## Automated Preflight Results

| Gate | Result |
|---|---|
| `gofmt` drift | PASS — none |
| `go mod verify` | PASS |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `go test -count=1 -race ./...` | PASS |
| Disposable migration up/down/up | PASS through v39; dirty=false |
| Isolated integration tests with race detector | PASS |
| SQLC regeneration diff | PASS — no generated drift |
| Frontend typecheck | PASS |
| Frontend lint | PASS with 0 errors, 563 warnings (2 fixable); warning debt remains |
| Frontend production build | PASS; 24 routes built |
| OpenAPI lint | PASS with 9 missing-4xx-response warnings |
| Playwright discovery | PASS — 129 tests in 106 files discovered across configured projects |
| Playwright isolated smoke slice | PASS — 7/7 desktop tests: strict staff login, order idempotency/conflict, concurrent cart mutation, realtime propagation and cross-org denial |
| Full Playwright suite | NOT RUN — repository audit already identifies broad fixture/helper rot; the bounded live slice was used to avoid spending the preflight on unrelated failures |
| Source `govulncheck` | FAIL — local source/toolchain scan reported 26 reachable standard-library advisories |
| Exact deployed-binary `govulncheck -mode=binary` | FAIL — 9 reachable vulnerabilities in the beta binary (8 advisory entries involving the standard library, plus affected `x/net`/`x/crypto` module symbols); embedded Go `1.26.5` |
| Remote health/readiness | PASS on balanced, instance 1 and instance 2 |

## Manual Materials Review

- The HTML checklist now has exactly 214 unique items and no duplicate IDs.
- Independent-audit gaps are folded into the interactive checklist as ENV-08–11, TEN-09–15, FAI-26, OBS-11–12 and REG-10–12.
- `manual-testing-session-guide.html`, `manual-testing-user-guide.html` and `testing-dashboard.html` are beta-first. `how-to-use.html` is explicitly local-development-only.
- `testing-dashboard.html` has current QR tokens, timestamped table-state guidance, v39/build warnings and the current SigNoz failure.
- Existing screenshot sweeps are not trusted as guest-flow evidence; the audit proved several were byte-identical redirects. This preflight used fresh, explicitly driven captures.
- The actual frontend routes agree with the role surfaces listed above. No undocumented product feature was added to this runbook.

## Evidence

Stored in `e2e/screenshots/manual-certification-preflight-2026-08-22/`:

1. `01-beta-frontend-landing.png` — public beta frontend rendered.
2. `02-beta-guest-qr-entry-mobile.png` — real Saffron T3 QR landing at a phone viewport.
3. `03-beta-staff-login.png` — strict three-field staff entry point.
4. `04-beta-platform-login.png` — platform entry point.
5. `05-signoz-login-via-tunnel.png` — SigNoz UI reachable (not proof of ingestion).
6. `06-api-instance-1-readyz.png` — pinned instance readiness response.
7. `07-testing-dashboard.png` — certification dashboard as reviewed.
8. `08-api-instance-2-readyz.png` — pinned instance 2 reports the same dependency readiness.

API/DB/log evidence is summarized in this report because screenshots would be less meaningful than exact status codes, IDs and queries.

# Manual Testing Checklist

Use a clean normal browser profile for Guest Host, incognito/another phone for Guest Participant, and separate staff/platform profiles. Use the testing dashboard for intentionally published credentials. Record timestamps in IST and copy request/correlation IDs from the Network panel. The 197-item HTML remains the detailed sign-off sheet; the ordered runbook below groups it into executable checkpoints and adds the independent-audit gaps.

## Phase A — Environment sanity

**ID:** A-01  
**Area:** Candidate identity  
**Role/device:** Release operator / laptop  
**URL:** Repository, PR #1, VM  
**Action:** Before testing, record local SHA, PR head, backend image tag/ID and frontend deployment version. Require one approved SHA across all surfaces.  
**Expected result:** One pushed, immutable, unambiguously deployed candidate.  
**What to watch for:** Current split `9865a48` vs `b57f746`.  
**Evidence to capture:** Command output and deployment metadata.  
**Pass / Fail:** ____  
**Notes:** Blocking preflight failure until reconciled.

**ID:** A-02  
**Area:** Stack readiness  
**Role/device:** Operator / laptop  
**URL:** Both pinned `/health` and `/readyz`; beta frontend  
**Action:** Load all four endpoints and frontend; confirm HTTP→HTTPS redirect and 200 responses.  
**Expected result:** `/health` and `/readyz` 200 on both instances; frontend renders.  
**What to watch for:** `/healthz` is not a valid route.  
**Evidence to capture:** Status, body, timestamp, certificate expiry.  
**Pass / Fail:** ____  
**Notes:** Covers missing audit ENV-09.

**ID:** A-03  
**Area:** Data/services  
**Role/device:** Operator / SSH  
**URL:** VM internal Docker networks  
**Action:** Check compose status, schema v39/dirty=false, PostgreSQL, Redis and both app environment regions without printing secrets.  
**Expected result:** All healthy; both backends share DB/Redis; `GIN_MODE=release`; `WORKER_REGION=beta`.  
**What to watch for:** Stray `.env` override, dirty migration, wrong worker region.  
**Evidence to capture:** Sanitized compose/ready output.  
**Pass / Fail:** ____  
**Notes:** Covers ENV-01–05 and missing audit ENV-08/10.

**ID:** A-04  
**Area:** Baseline data  
**Role/device:** Operator / laptop + dashboard  
**URL:** `testing-dashboard.html` and beta DB  
**Action:** Compare every QR token and table state with the DB; choose an available T3.  
**Expected result:** Tokens match; current state is explicitly corrected in notes.  
**What to watch for:** Dashboard state labels are stale although tokens match.  
**Evidence to capture:** Token prefix, branch/table and DB status—never credentials.  
**Pass / Fail:** ____  
**Notes:** Do not reset.

## Phase B — Platform Admin

**ID:** B-01  
**Area:** Platform authentication/RBAC  
**Role/device:** Each platform role / separate laptop profiles  
**URL:** `/platform/login`  
**Action:** Log in as super admin, support, billing and auditor; verify landing/navigation boundaries.  
**Expected result:** Each role sees only its documented console; staff token is rejected by platform APIs.  
**What to watch for:** Mutation buttons on read-only roles, cross-trust token acceptance.  
**Evidence to capture:** One screenshot and rejected request per role.  
**Pass / Fail:** ____  
**Notes:** PLT-01/02/27–30.

**ID:** B-02  
**Area:** Platform overview/support  
**Role/device:** Super admin + support admin / laptop  
**URL:** `/platform`, `/platform/support`, `/platform/support/audit`  
**Action:** Reconcile KPIs with seeded tenants; search a real session/order/payment; inspect audit correlation.  
**Expected result:** Scoped, sanitized records; no guest credentials/device fingerprints; opening support detail is audited.  
**What to watch for:** Credential leakage and missing cross-tenant filter labels.  
**Evidence to capture:** Sanitized response and audit row.  
**Pass / Fail:** ____  
**Notes:** SUP-01–06.

**ID:** B-03  
**Area:** MFA  
**Role/device:** Dedicated beta admin / authenticator + private browser  
**URL:** `/platform/security`  
**Action:** Enroll TOTP, store recovery codes securely, log out, and prove next login demands a valid code.  
**Expected result:** Secret/recovery codes shown once; invalid/missing TOTP fails closed.  
**What to watch for:** Password-only bypass after enrollment.  
**Evidence to capture:** Redacted enrollment and challenge screens.  
**Pass / Fail:** ____  
**Notes:** Never include TOTP seed/recovery codes in findings.

## Phase C — Organization / Branch management

**ID:** C-01  
**Area:** Onboarding  
**Role/device:** Super admin / laptop  
**URL:** `/platform/onboarding`  
**Action:** Create a uniquely named certification tenant, primary branch, owner and one table; immediately test its staff login and QR.  
**Expected result:** All resources are coherent and usable; actions audited.  
**What to watch for:** Partial creation, duplicate codes, wrong org ownership.  
**Evidence to capture:** IDs/codes and QR landing; no PIN.  
**Pass / Fail:** ____  
**Notes:** Record cleanup objects for Phase T.

**ID:** C-02  
**Area:** Governance state  
**Role/device:** Super admin / laptop  
**URL:** `/platform/organizations/<id>`  
**Action:** Suspend/reactivate the test org and branch; inspect audit and product behavior.  
**Expected result:** State records and audit change, while current documented governance enforcement remains intentionally inert.  
**What to watch for:** Accidental traffic outage or un-audited state change.  
**Evidence to capture:** Before/after state and unchanged guest reachability.  
**Pass / Fail:** ____  
**Notes:** Never use a seeded tenant for this mutation.

**ID:** C-03  
**Area:** Plans/subscriptions  
**Role/device:** Super admin + billing admin / laptop  
**URL:** `/platform/plans`, test organization billing page  
**Action:** Assign a plan to only the test tenant; verify resolved entitlements and billing shadow records.  
**Expected result:** Plan/entitlement resolution updates immediately; no real charge occurs.  
**What to watch for:** Limits accidentally enforced or billing role crossing into support.  
**Evidence to capture:** Plan assignment, resolved entitlements, audit.  
**Pass / Fail:** ____  
**Notes:** BIL-01–05, PLT-09–13.

## Phase D — Owner

**ID:** D-01  
**Area:** Owner scope/navigation  
**Role/device:** Saffron owner / separate laptop profile  
**URL:** `/staff/login` → `/staff/admin`  
**Action:** Log in with branch/staff code/PIN; visit all 12 admin tabs and organization branch view.  
**Expected result:** Owner sees branch admin plus allowed org governance for the three Saffron branches only.  
**What to watch for:** Copper/Urban data, broken empty states, tenant theme on staff UI.  
**Evidence to capture:** Admin home and org branch list.  
**Pass / Fail:** ____  
**Notes:** ADM-01/15–18/28/30.

**ID:** D-02  
**Area:** Owner-only staff controls  
**Role/device:** Owner / laptop  
**URL:** `/staff/admin` Staff tab  
**Action:** Create a uniquely coded waiter, rotate its PIN, verify old PIN rejected/new PIN accepted, deactivate it, verify immediate revocation.  
**Expected result:** Full lifecycle succeeds and is audited; deactivated session stops working.  
**What to watch for:** Old session remaining authorized or staff code collision.  
**Evidence to capture:** Redacted roster/audit and rejection status.  
**Pass / Fail:** ____  
**Notes:** Restore/remove only the test staff in Phase T.

**ID:** D-03  
**Area:** Audit  
**Role/device:** Owner / laptop  
**URL:** `/staff/admin` audit surface  
**Action:** Locate the owner login, staff lifecycle and subsequent menu action by request/correlation ID.  
**Expected result:** Correct actor, branch, IP, action and immutable record.  
**What to watch for:** Proxy IP, another tenant’s rows, missing correlation.  
**Evidence to capture:** Sanitized audit details.  
**Pass / Fail:** ____  
**Notes:** Cross-reference Phase R.

## Phase E — Manager

**ID:** E-01  
**Area:** Manager navigation/scope  
**Role/device:** Saffron manager / separate profile  
**URL:** `/staff/login` → `/staff/admin`  
**Action:** Visit menu, tables, promos, stats, appearance and settings; directly attempt owner-only staff creation.  
**Expected result:** Branch administration works; owner-only mutation is hidden and server-rejected.  
**What to watch for:** UI-only enforcement or sibling-branch access.  
**Evidence to capture:** 403 code and visible tab set.  
**Pass / Fail:** ____  
**Notes:** ADM-03–30 boundaries.

**ID:** E-02  
**Area:** PIN authorization  
**Role/device:** Manager / laptop + disposable waiter  
**URL:** Staff tab / Change PIN  
**Action:** Reset a waiter/kitchen PIN, attempt owner/manager/self reset, then rotate manager’s own PIN through self-service.  
**Expected result:** Only waiter/kitchen reset is permitted; protected roles denied; self-change works.  
**What to watch for:** Manager escalating another manager/owner.  
**Evidence to capture:** Status codes and audit rows; redact PINs.  
**Pass / Fail:** ____  
**Notes:** Restore access before continuing.

**ID:** E-03  
**Area:** Analytics/empty ranges  
**Role/device:** Manager / laptop  
**URL:** `/staff/admin` Stats/Performance/Loyalty  
**Action:** Inspect seeded and empty date ranges with gates off.  
**Expected result:** Stats reconcile; gated surfaces show intentional unavailable states; no NaN/crash.  
**What to watch for:** Data leakage and role/error messaging.  
**Evidence to capture:** One populated and one empty/gated state.  
**Pass / Fail:** ____  
**Notes:** Gate truth table occurs in Phase O.

## Phase F — Kitchen

**ID:** F-01  
**Area:** Kitchen authentication/KDS  
**Role/device:** Saffron kitchen / laptop profile  
**URL:** `/staff/login` → `/staff/kitchen`  
**Action:** Log in and leave KDS open at least 20 minutes while Phase H creates an order.  
**Expected result:** Only branch orders; stable board; new order appears within one poll cycle.  
**What to watch for:** Poll delay over 10 seconds, stale cards, admin/payment controls.  
**Evidence to capture:** Login landing and timestamped new-order card.  
**Pass / Fail:** ____  
**Notes:** KIT-01/02/08–10.

**ID:** F-02  
**Area:** Kitchen order state machine  
**Role/device:** Kitchen / laptop  
**URL:** `/staff/kitchen`  
**Action:** Advance the Phase H order pending/confirmed → preparing → ready; attempt an invalid transition separately.  
**Expected result:** Legal transitions succeed; illegal transition is server-rejected; notes/modifiers remain legible.  
**What to watch for:** Duplicate transitions or guest tracker lag.  
**Evidence to capture:** Network statuses and each KDS column.  
**Pass / Fail:** ____  
**Notes:** Do not mark served from kitchen.

**ID:** F-03  
**Area:** Kitchen branch isolation  
**Role/device:** Saffron kitchen / laptop  
**URL:** KDS plus direct Copper order URL/API  
**Action:** Confirm Copper/Urban orders are absent; attempt a known foreign order mutation directly.  
**Expected result:** Foreign data absent and direct mutation denied 403/404.  
**What to watch for:** ID-based route leakage.  
**Evidence to capture:** Rejection response and unchanged foreign order.  
**Pass / Fail:** ____  
**Notes:** Complements Phase P.

## Phase G — Waiter

**ID:** G-01  
**Area:** Waiter floor queues  
**Role/device:** Saffron waiter / laptop profile  
**URL:** `/staff/login` → `/staff/waiter`  
**Action:** Log in; reconcile assistance, ready-to-serve and pending-payment queues with DB/guest actions.  
**Expected result:** Only branch items, no admin controls, current queues only.  
**What to watch for:** Already-settled items or cross-branch rows.  
**Evidence to capture:** Queue screenshot and counts.  
**Pass / Fail:** ____  
**Notes:** WTR-01/02/08–10/13.

**ID:** G-02  
**Area:** Serve/assistance  
**Role/device:** Waiter + guest phone  
**URL:** Waiter dashboard and guest session  
**Action:** Mark Phase F ready order served; acknowledge and resolve Phase K assistance.  
**Expected result:** Guest updates without refresh; resolved items leave active queues.  
**What to watch for:** One-directional realtime or duplicate side effects.  
**Evidence to capture:** Both devices before/after with timestamps.  
**Pass / Fail:** ____  
**Notes:** WTR-03/04.

**ID:** G-03  
**Area:** Session revocation/lockout  
**Role/device:** Waiter / private browser  
**URL:** Staff login/dashboard  
**Action:** Log out and replay old token; separately exercise documented wrong-PIN threshold on a disposable staff account.  
**Expected result:** Old token immediately rejected; lockout returns clear bounded error and later recovers.  
**What to watch for:** Shared-IP lockout affecting unrelated staff.  
**Evidence to capture:** Status/code/Retry-After; no credential values.  
**Pass / Fail:** ____  
**Notes:** WTR-11/12/14.

## Phase H — Guest QR flow

**ID:** H-01  
**Area:** QR/theme/join  
**Role/device:** Guest host / physical phone on cellular  
**URL:** Saffron Bandra T3 guest URL above  
**Action:** Open QR URL, verify branch/table identity and dark-luxury theme, enter a display name and join. Also test one invalid token.  
**Expected result:** First guest is host; valid QR enters session; invalid QR fails clearly.  
**What to watch for:** Wrong tenant/theme, layout clipping, console/CSP errors.  
**Evidence to capture:** Phone screenshots and URL/token prefix.  
**Pass / Fail:** ____  
**Notes:** GST-01–05/27.

**ID:** H-02  
**Area:** Menu/modifiers/cart  
**Role/device:** Guest host / phone  
**URL:** Session menu/cart  
**Action:** Browse categories/images, select an item with a single-select modifier, add/update/remove items.  
**Expected result:** Correct price/variant/availability and arithmetic; single-select enforces exactly one.  
**What to watch for:** R2 image failure, stale availability, duplicate cart lines.  
**Evidence to capture:** Item dialog, cart, network status.  
**Pass / Fail:** ____  
**Notes:** GST-06–09/28.

**ID:** H-03  
**Area:** Host authority/order  
**Role/device:** Host + participant phone  
**URL:** Shared session  
**Action:** Participant attempts order and is denied; host submits once, then rapidly repeats the same action.  
**Expected result:** Only host succeeds; exactly one order is created; empty cart cannot order.  
**What to watch for:** Duplicate order or participant escalation.  
**Evidence to capture:** Both UI states, idempotency key/request statuses, order ID.  
**Pass / Fail:** ____  
**Notes:** GST-10–14.

**ID:** H-04  
**Area:** Persistence/reactivation  
**Role/device:** Guest / phone  
**URL:** Guest session  
**Action:** Refresh at menu/cart/order/payment states; close/reopen; allow a disposable session to enter reactivation window.  
**Expected result:** Exact server state restores; reactivation is offered; no privilege gain from cleared storage.  
**What to watch for:** Dead reconnect screen, resurrected closed session.  
**Evidence to capture:** Before/after snapshots and recovery duration.  
**Pass / Fail:** ____  
**Notes:** GST-26/29 and failure checks.

## Phase I — Multi-device / realtime

**ID:** I-01  
**Area:** Same-session concurrency  
**Role/device:** Host phone + participant incognito/second phone  
**URL:** Same QR/session  
**Action:** Join both, mutate cart concurrently in both directions, transfer host, and keep order tracker open.  
**Expected result:** One converged cart, monotonic events, correct host powers, no refresh.  
**What to watch for:** Lost/duplicate events and sequence gaps.  
**Evidence to capture:** Side-by-side video/screenshots with timestamps.  
**Pass / Fail:** ____  
**Notes:** GST-04/08/09/14/30, FAI-16/22.

**ID:** I-02  
**Area:** Deterministic two-instance HTTP  
**Role/device:** Operator / API client  
**URL:** API instance 1 and instance 2  
**Action:** Create/mutate through instance 1 and read through 2, then mutate through 2 and read through 1.  
**Expected result:** Shared session/cart/order state in both directions.  
**What to watch for:** Instance-local cache/session state.  
**Evidence to capture:** Request IDs, resource ID and response bodies.  
**Pass / Fail:** ____  
**Notes:** Automated preflight already passed; retain human evidence.

**ID:** I-03  
**Area:** Deterministic two-instance WebSocket  
**Role/device:** Operator + two browser profiles  
**URL:** WSS `/ws` on both pinned hosts  
**Action:** Redeem a fresh one-use ticket on instance 2, mutate via 1; repeat with socket on 1/mutation via 2.  
**Expected result:** Correct event arrives both ways; ticket replay fails; snapshots reconcile after reconnect.  
**What to watch for:** Origin rejection, missing fan-out, replay acceptance.  
**Evidence to capture:** WS frames, request IDs and instance hostnames.  
**Pass / Fail:** ____  
**Notes:** FAI-07/11/12.

## Phase J — Orders

**ID:** J-01  
**Area:** End-to-end order lifecycle  
**Role/device:** Guest, kitchen, waiter / three profiles  
**URL:** Guest session, KDS, waiter dashboard  
**Action:** Place order; advance through every legal state; mark served.  
**Expected result:** Each role sees the same state with correct operational ID; guest updates live.  
**What to watch for:** Poll delay, invalid owner of transition, duplicate ID.  
**Evidence to capture:** One screenshot per role and state timestamps.  
**Pass / Fail:** ____  
**Notes:** GST-15, KIT-02–07, WTR-02/03.

**ID:** J-02  
**Area:** Order validation/idempotency  
**Role/device:** Guest/API client  
**URL:** `/sessions/<id>/orders`  
**Action:** Repeat same idempotency key/body; repeat key with changed body; submit unavailable/foreign item and empty order.  
**Expected result:** One order; changed body conflicts; invalid items/order rejected without mutation.  
**What to watch for:** Cross-branch item acceptance and undocumented response casing.  
**Evidence to capture:** Status/body and DB order count.  
**Pass / Fail:** ____  
**Notes:** Automated O-04 passed.

**ID:** J-03  
**Area:** Cancellation/frozen bill  
**Role/device:** Kitchen/owner + guest  
**URL:** Order and payment surfaces  
**Action:** Cancel a separate order before payment; then test cancellation after bill freeze according to documented policy.  
**Expected result:** Guest informed; frozen financial snapshot never silently changes.  
**What to watch for:** Payment total drift.  
**Evidence to capture:** Order/payment before/after and audit.  
**Pass / Fail:** ____  
**Notes:** FAI-23; this is release-critical.

## Phase K — Assistance

**ID:** K-01  
**Area:** Assistance lifecycle  
**Role/device:** Guest phone + waiter laptop  
**URL:** `/session/<id>/assist`, waiter dashboard  
**Action:** Guest requests assistance; waiter acknowledges and resolves.  
**Expected result:** Request appears promptly and every state reaches guest.  
**What to watch for:** Duplicate requests or another branch’s waiter seeing it.  
**Evidence to capture:** Both devices and timestamps.  
**Pass / Fail:** ____  
**Notes:** GST-16/WTR-04.

**ID:** K-02  
**Area:** Assistance abuse limit  
**Role/device:** Guest / phone/API client  
**URL:** Assistance endpoint  
**Action:** Repeat requests to the documented per-session limit.  
**Expected result:** Friendly structured 429; existing assistance remains valid.  
**What to watch for:** Raw nginx 503, shared-IP collateral throttling.  
**Evidence to capture:** Status, error code and Retry-After.  
**Pass / Fail:** ____  
**Notes:** GST-17.

## Phase L — Payments / settlement

**ID:** L-01  
**Area:** Bill correctness/freeze  
**Role/device:** Guest host + owner  
**URL:** Guest payment and admin menu/promo  
**Action:** Verify itemized total; initiate payment; then change price/deactivate promo behind it.  
**Expected result:** Frozen bill remains byte-for-byte/amount-for-amount unchanged; cart is frozen for all guests.  
**What to watch for:** Any total mutation—SEV-0.  
**Evidence to capture:** Before/after bill JSON/screenshots and audit.  
**Pass / Fail:** ____  
**Notes:** GST-18–23, ADM-21.

**ID:** L-02  
**Area:** Staff settlement  
**Role/device:** Guest host + waiter  
**URL:** Guest payment/waiter pending payments  
**Action:** Initiate cash, then a separate card-manual/UPI-staff-confirmed payment; waiter settles.  
**Expected result:** Only waiter settlement closes payment/session/table; guest sees terminal screen.  
**What to watch for:** Auto-settlement or table remaining occupied.  
**Evidence to capture:** Payment reference, state transitions, terminal screen.  
**Pass / Fail:** ____  
**Notes:** WTR-05/06, GST-24.

**ID:** L-03  
**Area:** Payment concurrency/splits  
**Role/device:** Two waiter clients + guest  
**URL:** Same pending payment  
**Action:** Concurrently settle the same payment; separately execute a valid split and overpayment attempt.  
**Expected result:** Exactly one settlement side effect; split remains open until covered; overpayment rejected.  
**What to watch for:** Duplicate audit/payment rows or early close.  
**Evidence to capture:** Both response statuses and final DB state.  
**Pass / Fail:** ____  
**Notes:** WTR-07, GST-25.

**ID:** L-04  
**Area:** Escalation invariant  
**Role/device:** Operator + waiter  
**URL:** Pending payment, Prometheus/alerts/logs  
**Action:** Leave a disposable payment pending past warning/critical thresholds.  
**Expected result:** Alerts/metrics/logs fire; payment/session state changes **nothing**.  
**What to watch for:** Double execution across workers or auto-cancel/settle.  
**Evidence to capture:** Alert timestamp plus unchanged DB state.  
**Pass / Fail:** ____  
**Notes:** FAI-10; release-critical.

## Phase M — Menu / themes / media

**ID:** M-01  
**Area:** Menu CRUD/live cache  
**Role/device:** Manager + seated guest  
**URL:** Admin Menu and guest Menu  
**Action:** Create/edit/category/item/modifier, toggle featured/availability, then delete only disposable content.  
**Expected result:** Guest reflects edits promptly; unavailable item cannot order; active cart handles change safely.  
**What to watch for:** Redis cache delay and foreign ID mutation.  
**Evidence to capture:** Admin/guest before-after and request IDs.  
**Pass / Fail:** ____  
**Notes:** ADM-03–08.

**ID:** M-02  
**Area:** R2 media UI  
**Role/device:** Owner / laptop + guest phone  
**URL:** Admin menu upload and guest menu  
**Action:** Upload a valid small PNG to a disposable menu item; reload guest from another network/profile.  
**Expected result:** Upload succeeds and public R2 URL renders with correct content type.  
**What to watch for:** Presign expiry, CSP/image-domain rejection, tenant path collision.  
**Evidence to capture:** Admin success, public image, sanitized object path.  
**Pass / Fail:** ____  
**Notes:** Automated byte round trip passed; UI render remains manual.

**ID:** M-03  
**Area:** Theme/collateral  
**Role/device:** Owner + platform admin + phone/printer  
**URL:** Admin Appearance/Collateral; Platform Themes/Collateral  
**Action:** Change only the test tenant theme/collateral, verify guest-only repaint, export PNG/SVG/PDF/ZIP, print and scan one QR.  
**Expected result:** Surfaces agree; staff stays unthemed; output matches preview; printed QR opens HTTPS frontend.  
**What to watch for:** Competing write paths, unsafe token input, print safe-area.  
**Evidence to capture:** Preview/export/printed scan.  
**Pass / Fail:** ____  
**Notes:** Restore theme in Phase T.

## Phase N — Staff / PIN / authorization

**ID:** N-01  
**Area:** Strict staff login  
**Role/device:** All four staff roles / separate profiles  
**URL:** `/staff/login`  
**Action:** Authenticate with branch+staff code+PIN; retry old branch-ID/PIN-only shape.  
**Expected result:** New shape works and routes by role; old shape has no token.  
**What to watch for:** UI copy still says branch ID + PIN.  
**Evidence to capture:** Role landing and old-format rejection.  
**Pass / Fail:** ____  
**Notes:** Automated F-06 passed.

**ID:** N-02  
**Area:** Server-side role policy  
**Role/device:** Kitchen, waiter, manager / API client  
**URL:** Representative forbidden admin/payment/platform endpoints  
**Action:** Call forbidden mutations directly with each valid token.  
**Expected result:** 401/403 regardless of hidden UI.  
**What to watch for:** Shadow-only authorization bypass.  
**Evidence to capture:** Route, actor role, status/code, unchanged resource.  
**Pass / Fail:** ____  
**Notes:** Check `legacy_authz_bypass_total` in Phase R.

**ID:** N-03  
**Area:** Credential/session lifecycle  
**Role/device:** Disposable staff / two browsers  
**URL:** Staff login and protected staff API  
**Action:** Rotate PIN and deactivate while a second browser has an active token.  
**Expected result:** Old PIN and existing DB-backed session are immediately rejected.  
**What to watch for:** Token valid until expiry despite revocation.  
**Evidence to capture:** Before 200 / after rejection and audit.  
**Pass / Fail:** ____  
**Notes:** Restore/cleanup disposable staff only.

## Phase O — Feature flags

**ID:** O-01  
**Area:** Gate truth table  
**Role/device:** Platform admin + owner  
**URL:** Platform flags/entitlements and Staff Performance/Loyalty  
**Action:** On the test tenant only, test entitlement/flag combinations 00, 10, 01 and 11 for each gated feature.  
**Expected result:** Only 11 enables; propagation immediate; 00 restored afterward.  
**What to watch for:** Flag-alone or entitlement-alone activation.  
**Evidence to capture:** Four-cell result matrix and audit.  
**Pass / Fail:** ____  
**Notes:** GAT-01–06/12.

**ID:** O-02  
**Area:** Flag precedence/isolation  
**Role/device:** Platform admin + two tenant owners  
**URL:** Platform Feature Flags  
**Action:** Prove branch > org > global > default precedence and that another tenant remains unchanged.  
**Expected result:** Exact documented precedence with no leak.  
**What to watch for:** Cache persistence after override removal.  
**Evidence to capture:** Resolved value at every scope.  
**Pass / Fail:** ____  
**Notes:** PLT-14/15, GAT-06.

**ID:** O-03  
**Area:** Loyalty invariants  
**Role/device:** Owner + waiter + guest  
**URL:** Loyalty and payment surfaces  
**Action:** With both gates on for test tenant, settle one payment, replay earn, redeem/adjust within allowed roles, then induce a loyalty failure.  
**Expected result:** Earn idempotent; balance never negative; redemption ledger-only; loyalty failure never breaks payment.  
**What to watch for:** Bill movement or duplicate earn.  
**Evidence to capture:** Ledger/payment rows and balances.  
**Pass / Fail:** ____  
**Notes:** GAT-07–11; restore gates.

## Phase P — Tenant isolation / security

**ID:** P-01  
**Area:** Trust-domain isolation  
**Role/device:** API client  
**URL:** Guest, staff and platform APIs  
**Action:** Use staff token on platform, guest A token on session B, and a tampered/expired guest token.  
**Expected result:** All rejected 401/403/404; no resource existence leakage.  
**What to watch for:** 500/409 on foreign IDs.  
**Evidence to capture:** Sanitized request/status/body.  
**Pass / Fail:** ____  
**Notes:** TEN-01/03–06.

**ID:** P-02  
**Area:** Item-scoped tenant guards  
**Role/device:** Saffron owner/manager / API client  
**URL:** Foreign category/item/staff/session-event endpoints  
**Action:** Execute audit-added TEN-09–15 against Copper resources: category rename, featured/availability flips, deletes, event read and staff deactivation; repeat with central policy off/on only in an isolated local stack.  
**Expected result:** 403 in both modes; every foreign row unchanged.  
**What to watch for:** Authorization depending on rollout flag.  
**Evidence to capture:** Before/after DB rows and all statuses.  
**Pass / Fail:** ____  
**Notes:** Never flip beta enforcement flags.

**ID:** P-03  
**Area:** Branch/org read isolation  
**Role/device:** Saffron/Copper/Urban staff / separate profiles  
**URL:** Menus, themes, audit, KDS, waiter/admin APIs  
**Action:** Cross-call sibling and foreign branch resources, then inspect each UI for leaked data.  
**Expected result:** Only authorized branch/org scope appears.  
**What to watch for:** Same-org sibling branch mistaken as authorized.  
**Evidence to capture:** Rejections and three tenant theme/menu screenshots.  
**Pass / Fail:** ____  
**Notes:** Automated cross-org session denial passed.

**ID:** P-04  
**Area:** Audit/client IP/security headers  
**Role/device:** Phone on Wi-Fi and cellular + laptop  
**URL:** API via beta frontend/direct host  
**Action:** Make identified requests from two networks, try spoofed XFF, inspect audit/log IP/request ID/CORS/CSP.  
**Expected result:** Actual client IP recorded, spoof ignored, correct request correlation and origin policy.  
**What to watch for:** Shared proxy IP or attacker-controlled XFF.  
**Evidence to capture:** Redacted IP comparison and request IDs.  
**Pass / Fail:** ____  
**Notes:** Automated forwarding check passed from one network.

## Phase Q — R2 / backups

**ID:** Q-01  
**Area:** R2 application path  
**Role/device:** Owner + guest  
**URL:** Menu image upload/public image URL  
**Action:** Presign, upload a valid PNG, read it publicly and render it in guest menu.  
**Expected result:** Exact bytes/content type, tenant-scoped path, public URL works.  
**What to watch for:** Credential exposure or cross-tenant overwrite.  
**Evidence to capture:** SHA-256, object path and rendered image.  
**Pass / Fail:** ____  
**Notes:** Delete only the disposable test object if documented cleanup supports it.

**ID:** Q-02  
**Area:** Backup presence/integrity  
**Role/device:** Operator / SSH  
**URL:** Backup R2 bucket  
**Action:** List latest dump/manifest, download to a disposable local file and compare SHA-256.  
**Expected result:** Latest scheduled backup present; checksum exact; retention 14 days.  
**What to watch for:** Stale manifest, zero-byte/truncated dump.  
**Evidence to capture:** Object timestamp/size/checksum only.  
**Pass / Fail:** ____  
**Notes:** Automated check passed; no restore into beta DB.

**ID:** Q-03  
**Area:** Secret hygiene  
**Role/device:** Operator / repository and VM  
**URL:** Environment configuration  
**Action:** Confirm required R2 variables are set at runtime and secret values are absent from tracked files/logs/report.  
**Expected result:** Runtime-only secrets; least necessary bucket access.  
**What to watch for:** `.env` tracked or keys printed in client bundles.  
**Evidence to capture:** SET/UNSET names only.  
**Pass / Fail:** ____  
**Notes:** Never capture secret values.

## Phase R — Observability / SigNoz

**ID:** R-01  
**Area:** Fresh trace pivot  
**Role/device:** Tester + SigNoz / laptop  
**URL:** Beta API and SigNoz  
**Action:** Send a real guest/order request with known request ID, find its HTTP trace, inspect PostgreSQL/Redis child spans, then pivot to correlated logs/audit.  
**Expected result:** Fresh end-to-end trace within seconds; service names identify backend instance; logs correlate.  
**What to watch for:** Missing/old data or blank trace IDs.  
**Evidence to capture:** Trace screenshot, trace/request/correlation IDs.  
**Pass / Fail:** ____  
**Notes:** Currently blocked by ClickHouse memory exhaustion.

**ID:** R-02  
**Area:** Instance/service isolation  
**Role/device:** Operator / SigNoz  
**URL:** Services/logs/traces  
**Action:** Query both pinned instances and confirm both report independently under correct QR Dining names; inspect recent service list for unrelated VM apps.  
**Expected result:** Both beta instances present; no unrelated project ingestion into this SigNoz deployment.  
**What to watch for:** Stale local `mtest-*` names and shared collector leakage.  
**Evidence to capture:** Service list/filter and two traces.  
**Pass / Fail:** ____  
**Notes:** Full proof is currently impossible because aggregate queries OOM.

**ID:** R-03  
**Area:** Metrics/alerts  
**Role/device:** Operator / Prometheus, Alertmanager, human channel  
**URL:** SSH-only monitoring endpoints  
**Action:** Confirm both app targets and 27 rules; observe business/WS/worker counters; fire a safe test alert and confirm a human receives it.  
**Expected result:** Targets/rules healthy, counters move, real receiver notified.  
**What to watch for:** Placeholder alert sink and `legacy_authz_bypass_total > 0`.  
**Evidence to capture:** Target/rule state, counter deltas, receipt timestamp.  
**Pass / Fail:** ____  
**Notes:** Adds missing OBS-11/12; do not treat the local sink as human delivery.

## Phase S — Failure / recovery testing

Run only after happy paths, with explicit operator supervision. Never stop PostgreSQL/Redis or the shared proxy on the VM. Use the isolated local stack for datastore outages; beta may be used only for a single app-instance restart if separately authorized.

**ID:** S-01  
**Area:** Client network/reconnect  
**Role/device:** Two guest devices  
**URL:** Active disposable guest session  
**Action:** Toggle airplane mode/slow 3G, sleep/resume, refresh and navigate back.  
**Expected result:** Bounded reconnect, authoritative snapshot, no duplicate mutations or privilege gain.  
**What to watch for:** Infinite spinner/event replay gap.  
**Evidence to capture:** Recovery times and WS frames.  
**Pass / Fail:** ____  
**Notes:** FAI-01/02/11/16–19/21.

**ID:** S-02  
**Area:** Local Redis/PostgreSQL recovery  
**Role/device:** Operator / isolated local stack only  
**URL:** Local backends `:8090/:8095`  
**Action:** Following existing runbook, pause one dependency at a time; verify `/health` vs `/readyz`, then resume.  
**Expected result:** Health process remains up; readiness reports dependency; Redis recovery resumes realtime; PostgreSQL recovery restores readiness.  
**What to watch for:** Data loss, manual restart requirement.  
**Evidence to capture:** Endpoint transition/recovery times.  
**Pass / Fail:** ____  
**Notes:** Do not run on beta without new authorization.

**ID:** S-03  
**Area:** Backend/collector failure  
**Role/device:** Operator / isolated local stack  
**URL:** Local instance B and collector  
**Action:** Restart only local backend B during a session; separately stop local collector.  
**Expected result:** Instance A unaffected; B reconnects/reconciles; collector loss never affects app requests.  
**What to watch for:** Split-client has no beta failover; do not infer otherwise.  
**Evidence to capture:** Recovery duration and state snapshot.  
**Pass / Fail:** ____  
**Notes:** FAI-06/08.

**ID:** S-04  
**Area:** Rate limits/webhooks/audit invariants  
**Role/device:** API client / isolated stack  
**URL:** Auth, assistance, WS-ticket, webhook and audit routes  
**Action:** Exercise caps, bad/replayed webhook signatures/amount/currency/session, and attempt audit UPDATE/DELETE.  
**Expected result:** Structured rejection, idempotent replay, no payment mutation; audit trigger rejects writes.  
**What to watch for:** Nginx 503 versus app 429 and any mutable audit row.  
**Evidence to capture:** Status/code, unchanged rows, trigger errors.  
**Pass / Fail:** ____  
**Notes:** Includes FAI-13–15/20 and audit-added FAI-26 (run Playwright and record counts).

## Phase T — Final certification

**ID:** T-01  
**Area:** Restore test configuration  
**Role/device:** Owner/platform admin/operator  
**URL:** All mutated test surfaces  
**Action:** Restore themes, flags, entitlements, settings, promos and test-only menu/table/staff objects; leave seeded fixtures as documented.  
**Expected result:** All gates default off, seeded themes restored, no orphaned sessions/stuck tables.  
**What to watch for:** Cleanup crossing tenant boundaries.  
**Evidence to capture:** Before/after inventory and current table/session query.  
**Pass / Fail:** ____  
**Notes:** Never reset beta DB.

**ID:** T-02  
**Area:** Regression sweep  
**Role/device:** Guest, kitchen, waiter / phone+laptop  
**URL:** Fresh available T3 and staff surfaces  
**Action:** Repeat QR→cart→order→kitchen→serve→payment→settlement after all configuration tests.  
**Expected result:** Complete happy path still passes; session closes/table frees.  
**What to watch for:** Residual flags/themes/menu changes.  
**Evidence to capture:** End-to-end timeline and terminal screen.  
**Pass / Fail:** ____  
**Notes:** REG-01–06.

**ID:** T-03  
**Area:** API contract/security regression  
**Role/device:** Operator / laptop  
**URL:** Repository + isolated stack  
**Action:** Run full appropriate automated suite; record exact Playwright counts; verify documented response shapes/live route coverage and exact deployed binary vulnerability result.  
**Expected result:** Approved gates green; known test defects separated from product failures; contract matches live API.  
**What to watch for:** Treating discovery-only CI as execution.  
**Evidence to capture:** Machine-readable reports and command versions.  
**Pass / Fail:** ____  
**Notes:** Adds REG-10/11 and FAI-26.

**ID:** T-04  
**Area:** Final operational review  
**Role/device:** Release operator / laptop  
**URL:** VM, SigNoz, Prometheus, R2  
**Action:** Recheck both instances, DB/Redis, fresh telemetry, alerts, latest backup, disk/memory, error budget and protected soak unchanged.  
**Expected result:** No blockers, no fresh critical errors, evidence tied to approved candidate.  
**What to watch for:** Green container health masking failed ingestion.  
**Evidence to capture:** Final timestamped status pack.  
**Pass / Fail:** ____  
**Notes:** REG-07/08 and observability exit gate.

**ID:** T-05  
**Area:** Verdict  
**Role/device:** Certifier / report  
**URL:** Certification artifacts  
**Action:** Export checklist raw JSON/Markdown; list every failure/N/A; assign severity/owner; write GO, CONDITIONAL GO or NO-GO against the exact SHA.  
**Expected result:** Zero SEV-0 for GO; all SEV-1s resolved or explicitly owned under the published exit criteria.  
**What to watch for:** Blank checks, generous passes, missing N/A rationale.  
**Evidence to capture:** Signed verdict, artifact hashes and candidate SHA.  
**Pass / Fail:** ____  
**Notes:** Do not merge/tag/deploy as part of certification itself.

## First Manual Test

Do **A-01 first**: obtain one approved candidate SHA and prove that exact SHA labels the backend image and frontend deployment. Once that blocker is cleared, the first product interaction is H-01: on a phone using cellular data, open the Saffron Bandra T3 Guest URL in the URL table, confirm the page says Saffron House/Bandra/T3 with the dark-luxury theme, enter a disposable display name, and capture the resulting host/session screen plus timestamp.
