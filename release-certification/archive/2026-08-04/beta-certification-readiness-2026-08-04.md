# Manual Certification Readiness — Beta

**Date:** 2026-08-04 · Companion to `beta-deployment-report-2026-08-04.md`

> **Superseded for the 2026-08-22 run.** Use
> [`manual-certification-preflight-2026-08-22.md`](../../manual-certification-preflight-2026-08-22.md) and
> [`docs/manual-testing/testing-dashboard.html`](../../../docs/manual-testing/testing-dashboard.html).
> Product-flow testing may continue, but the current formal readiness verdict is **BLOCKED** by candidate
> provenance, failed SigNoz ingestion, nine reachable advisories in the deployed backend binary, and the lack
> of a human Alertmanager receiver. The historical statement below describes the 2026-08-04 deployment check;
> it is not the current certification verdict.

## Can manual certification begin?

**Yes.** Every operational component is deployed and verified end-to-end: backend (two instances), frontend,
TLS, WebSockets, uploads, R2, traces, logs, metrics, alert delivery, backup, and a checksum-verified restore.
Nothing required to exercise the product is missing.

Three things to know before you start — none block testing:

1. **PostHog records nothing.** No project key was supplied, so the SDK is a deliberate no-op. If analytics is
   in scope for certification, add `NEXT_PUBLIC_POSTHOG_KEY` and redeploy the frontend first.
2. **Staff login needs three fields, not two.** `AUTH_STAFF_CODE_REQUIRED=true`, so PIN alone returns
   `400 "branch_code and staff_code are required"`. The docs previously said `branch_id + pin`; corrected.
3. **PR #1 is missing your latest commit.** `03eb13c` (PostHog) is unpushed, so the PR — and its green CI —
   cover the branch without it. The beta *does* run it on the frontend.

---

## Access

| What | Where |
|---|---|
| App (guest, staff, platform) | **https://qr-beta.mohith16.com** |
| API (balanced) | https://qr-api-beta.mohith16.com |
| API pinned — instance 1 / 2 | https://qr-api-beta-1.mohith16.com · https://qr-api-beta-2.mohith16.com |
| Prometheus + Alertmanager | `ssh -L 9090:127.0.0.1:9090 -L 9093:127.0.0.1:9093 appuser` |
| SigNoz | `ssh -L 3301:127.0.0.1:3301 appuser` → http://localhost:3301 |

Works on any device, any network — real Let's Encrypt cert, valid to 2026-10-27.

## Credentials

**Platform console** (`/platform/login`)

| Email | Password | Role |
|---|---|---|
| `admin@platform.local` | `Platform!admin1` | super_admin |
| `support@platform.local` | `Support!admin1` | support_admin |
| `billing@platform.local` | `Billing!admin1` | billing_admin |
| `auditor@platform.local` | `Auditor!admin1` | read_only_auditor |
| `beta-admin@mohith16.com` | `Beta-6mhVxHv2Pu8qYsfmRFhB-Adm1!` | super_admin — **created via the production `bootstrap-admin` path, MFA required; enroll TOTP on first login** |

**SigNoz:** `beta-admin@mohith16.com` / `BetaSignoz!2026`

**Staff** (`/staff/login`) — PIN **plus** branch code **plus** staff code:

| Role | PIN | Staff code pattern |
|---|---|---|
| owner | 1111 | `<BRANCH>-OWN` |
| manager | 2222 | `<BRANCH>-MGR` |
| waiter | 3333 | `<BRANCH>-WTR` |
| kitchen | 4444 | `<BRANCH>-KIT` |

e.g. branch `SAFF-BND` + code `SAFF-BND-OWN` + PIN `1111`. Owner exists only on each tenant's primary branch.

## Restaurants

| Tenant | Plan / theme | Branches |
|---|---|---|
| **Saffron House** (org 1) | premium / dark-luxury | `SAFF-BND` Bandra (1, primary, live data) · `SAFF-IND` Indiranagar (2) · `SAFF-CP` Connaught Place (3) |
| **Copper Pot Kitchen** (org 2) | standard / warm-cafe | `COPR-KOR` Koramangala (4, primary, live data) |
| **Urban Brew Café** (org 3) | free-trial / vibrant | `BREW-CYB` Cyber Hub (5, primary, live data) |

15 tables, 3 per branch. **Live QR links are in `docs/manual-testing/testing-dashboard.html`** — regenerated for
this deployment, so open that file rather than reusing older links. T1/T2 of each primary branch already carry
seeded live state (a served order, a payment awaiting staff confirmation); T3 is free.

---

## Suggested order

1. **Smoke** — open the app on your phone, hit a T3 QR link, confirm the menu renders and the theme matches the tenant.
2. **Full guest journey** on one device: QR → join → shared cart → order → kitchen → serve → bill → cash settle → closed.
3. **Two devices, same table** — shared-cart convergence and live order status.
4. **Cross-instance** (the reason there are two backends): put device A on wifi and device B on mobile data so
   they hash to different instances, or drive the API directly via `qr-api-beta-1` / `qr-api-beta-2`. Confirm a
   write on one instance appears on the other. *(Automated proof already passed; this is the human confirmation.)*
5. **Staff surfaces** — all four roles, on the seeded live sessions.
6. **Platform console** — MFA enrollment on `beta-admin@mohith16.com`, then tenant/branch governance.
7. **Cross-tenant isolation** — the three tenants exist precisely for this.
8. **Uploads** — menu-item image through the staff admin UI; confirm it renders from the R2 public base.
9. **Observability** — find one of your own requests as a trace in SigNoz and pivot to its logs.

Run the checklist in `docs/manual-testing/manual-testing-checklist.html`; it has been repointed at the beta.

## Watch for

- **Duplicate side effects** (double payment escalations, sessions closing twice) — worker coordination is fixed
  and verified, but this is the class of bug two instances expose and the one worth staying alert to.
- **Anything keyed on client IP** — rate limiting was silently bucketing every guest together until today's nginx
  fix. It is correct now; if throttling behaves oddly, this is the first place to look.
- **WebSocket behaviour after an instance restart** — `split_clients` gives no automatic failover by design, so a
  restart will drop the clients hashed to that instance until they reconnect.

## Cleanup when finished

```bash
ssh appuser
cd /opt/qr-dining && docker compose -f docker-compose.yml -f docker-compose.beta.yml \
  -f observability/docker-compose.observability.yml -f docker-compose.observability-vm.yml \
  -f docker-compose.observability-beta.yml down
cd /opt/qr-dining/signoz && docker compose -f docker-compose.signoz.yml down
```

Add `-v` only if you also want the seeded data gone. Leave `/opt/proxy` alone — it serves three other projects.
