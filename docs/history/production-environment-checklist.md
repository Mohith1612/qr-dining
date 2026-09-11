# QR-Dining — Production Environment Checklist

**Date:** 2026-07-18 · **RC branch:** `feature/certification-fixes-ui-redesign` @ `9dd869a`
**Companion:** `deployment-architecture.md` · **Authority:** `STATE-OF-THE-PROJECT.md`

> Everything that must exist before production serves a paying customer. Ordered roughly by dependency; SEV labels match the standing release findings.
>
> **Updated 2026-07-18 (infrastructure-preparation phase):** deployment decisions are final (shared-VM proxy architecture, Cloudflare frontend, R2 storage) and the canonical guides now exist — `DEPLOYMENT.md`, `OPERATIONS.md`, `RECOVERY.md`. Items completed or unblocked in that phase are marked ✅ below.

---

## 1. Release engineering (gates everything)

- [ ] Create the git remote, push the branch, and obtain one green CI run; no GHCR deployment image exists until this succeeds.
- [ ] **Merge `feature/certification-fixes-ui-redesign` → `main` (`--no-ff`) and tag `v1.0.0-rc.1`** (SEV-1). Nothing else lands first.
- [x] CI unit job widened from `./internal/domain/...` to `./...` under the race detector.
- [ ] Integration suite green locally (`-tags integration`) — last commit `9dd869a` claims restoration; re-verify before tagging.
- [ ] Delete the 4 fully-absorbed branches after merge (staff-analytics-loyalty, premium-qr-collateral, pilot-readiness-remediation, platform-governance-entitlements).
- [ ] Commit the live-but-untracked operational files (STATE-OF-THE-PROJECT.md, docs/history/, docs/manual-testing/, living docs, docker-compose.manual-testing.yml, e2e/package-lock.json) — the RC tag must contain its own documentation.
- [x] Go toolchain aligned on 1.26 across `go.mod`, Docker, and CI.
- [ ] **RC soak (SEV-0):** binary built **from the tag**, run **off `/tmp`**, `AUDIT_LOG_V2_ENABLED=true`, continuous synthetic traffic + external probing for the full window, storage curve recorded. The 124h R1 soak covered the pre-redesign binary and does not certify this RC.

## 2. Host provisioning (Oracle Cloud Ampere)

- [ ] arm64 instance sized for app+PG+Redis+observability on one host; docker + compose v2.
- [ ] Host packages for the backup path: `aws` CLI, `postgresql-client` **17** (pg_dump/pg_restore version-matched to postgres:17).
- [ ] Repo deployed at **`/opt/qr-dining`** (systemd units hardcode this path).
- [ ] `docker network create proxy_network` (external network required by the prod compose).
- [x] Release mode ignores dotenv files; injected environment cannot be shadowed by a stray `.env`.
- [ ] No binaries or long-running artifacts under `/tmp` (tmp-cleaner wiped the soak binary twice).
- [ ] Firewall/security-list: only 80/443 (edge) exposed publicly; 9090/9093 (observability) private; DB/Redis never published.

## 3. Edge, TLS, DNS

- [x] ✅ **TLS termination decided & prepared (2026-07-18):** the VM's existing central proxy (`/opt/proxy` nginx + certbot, shared Let's Encrypt). Vhost template ready: `deploy/vm/qr-dining.conf.example` (passes `nginx -t`; request-time DNS so qr-dining downtime can't block shared-proxy reloads).
- [x] ✅ **nginx-in-prod decided (2026-07-18):** no per-project nginx — qr-dining is a vhost on the central proxy. Production compose is `deploy/vm/docker-compose.yml` (external `proxy` network, per VM reference).
- [ ] Real domain(s) purchased; replace every `CHANGEME-DOMAIN.com` per DEPLOYMENT.md §8 (`server_name`, cert path, `CORS_ALLOWED_ORIGINS`, frontend `NEXT_PUBLIC_*`).
- [x] ✅ `/metrics` exposure resolved: vhost restricts to 127.0.0.1; Prometheus scrapes over the internal docker network, not HTTP-through-proxy.
- [ ] `TRUSTED_PROXIES` set to the actual `proxy` network CIDR at install time (command documented in the env template).
- [ ] `ENABLE_HSTS=true` once TLS is confirmed end-to-end (template defaults it true).

## 4. Backend environment (`/opt/qr-dining/.env`)

**The committed `.env` cannot boot the prod stack** (localhost DSNs, placeholder password, no guest secret, localhost CORS). A production `.env` must be written by hand, mode 600. Full variable reference in Appendix A.

- [ ] `DATABASE_URL=postgres://<user>:<strong-pw>@postgres:5432/<db>` — **host `postgres`, not localhost**; credentials must match `POSTGRES_USER/PASSWORD/DB` (compose passes those separately to the postgres image — keep in sync).
- [ ] `REDIS_URL=redis://redis:6379/0` — host `redis`.
- [ ] `GUEST_TOKEN_SECRET` — ≥32 chars, generated, never the dev default (release mode refuses it).
- [ ] `CORS_ALLOWED_ORIGINS` — exact https origins of guest/staff/platform surfaces (release mode refuses empty; empty also disables the WS origin allowlist).
- [ ] `MFA_ENCRYPTION_KEY` — ≥16 chars (required in practice: platform login uses TOTP; unset = MFA fails closed).
- [ ] `PAYMENT_WEBHOOK_SECRET_<PROVIDER>` — per gateway, ≥16 chars (only when a digital provider is configured; pilot is cash/UPI-manual).
- [ ] `GIN_MODE=release` (compose already forces it — keep the `.env` consistent rather than contradicting it with `debug`).
- [ ] **All 9 rollout flags set EXPLICITLY** — do not rely on application defaults for a production record:
  - `AUDIT_LOG_V2_ENABLED=true` (R1 — live and soaked; must be true)
  - `TENANCY_ORGANIZATIONS_ENABLED=false` · `AUTHZ_CENTRAL_POLICY_ENFORCE=false` · `STRICT_BRANCH_SCOPED_MUTATIONS=false` · `AUTH_STAFF_CODE_REQUIRED=true` · `AUTH_STAFF_SESSION_DB_REQUIRED=true` · `WS_TICKET_AUTH_REQUIRED=true` · `AUTH_GUEST_CREDENTIALS_REQUIRED=true` · `PAYMENT_STAFF_SETTLEMENT_REQUIRED=false`
  - `GUEST_TOKEN_TTL=12h` (required while the client has no refresh endpoint)
- [ ] R2 upload vars (`R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET=qr-dining-uploads`, `R2_PUBLIC_BASE`) — all five or image upload endpoints return 503.
- [ ] Tuning reviewed for prod (defaults fine at pilot scale): `DB_MAX_CONNS` (20), rate limits (60/10 RPM), worker intervals, payment-escalation thresholds (1m/5m/15m), `LOG_PRETTY=false`.

## 5. Datastores

- [ ] Named volumes `postgres_data` / `redis_data` on adequately-sized disk; disk-usage alerting via node-exporter (audit_log growth curve from the soak).
- [ ] Strong `POSTGRES_PASSWORD` (replaces `changeme_strong_password`), consistent with `DATABASE_URL`.
- [ ] First boot verified: app self-migrates to schema v38; `/readyz` 200.
- [ ] Consider compose resource limits (none exist today) so PG/Redis/app can't starve each other on the single host.

## 6. Observability & alerting (SEV-1)

- [x] ✅ Network wiring fixed (2026-07-18): `deploy/vm/docker-compose.observability-vm.yml` joins prometheus/blackbox to the app network and binds UIs to localhost; merged invocation documented (DEPLOYMENT.md §5). Merged render verified.
- [x] ✅ Receiver prepared for one-line go-live (2026-07-18): both receivers alias a single `&notify_url` anchor in `alertmanager.yml` (`amtool check-config` clean). **Still open:** point it at a real channel and test-fire both routes.
- [ ] **Set the real notification endpoint** (the one `&notify_url` line) and test-fire `page` + `ticket` end-to-end.
- [ ] Verify the app-down path: `ReadyzProbeFailing` + `AppTargetDown` actually page.
- [ ] Grafana: stand up an instance (none is deployed) and import `grafana-dashboard.json`, or accept Prometheus-only for pilot — explicit decision.
- [ ] Keep Prometheus/Alertmanager ports (9090/9093) off the public internet.
- [ ] Rollout-gate alerts (`LegacyAuthzBypassPresent`, `LegacyIdentityUsagePresent`, …) confirmed reading zero — these are the flag-flip gates for R2+.

## 7. Backups (SEV-1)

- [x] ✅ Cloudflare R2 buckets created (2026-07-18, **temporary names**): `qr-dining-backups` + `qr-dining-uploads`, with a scoped Object-R/W token for backups. Final buckets (if renamed) are an env-only change (DEPLOYMENT.md §8).
- [ ] `/etc/qr-dining/backup.env` (600, root) on the VM from the updated `deploy/backup/backup.env.example` (now includes `NODE_EXPORTER_TEXTFILE_DIR`). ⚠️ backup var names differ from the app's R2 vars — two credential sets, don't conflate.
- [ ] Install + enable `qr-dining-backup.timer` (02:30 daily, `Persistent=true`); confirm first journal run.
- [x] ✅ **Live R2 round-trip PASSED (2026-07-18)** against the real (temporary) bucket: dump → upload → manifest → fresh download → sha256-vs-manifest match → restore into throwaway postgres:17 → content checksum identical; metrics emitted. Log: RECOVERY.md §5. (Fixed en route: BusyBox-incompatible `date` in retention.) Re-verify once on the VM with the systemd path at real volume.
- [ ] Backup metrics visible in the VM's Prometheus and `BackupFailed`/`BackupTooOld` armed (script side verified; VM textfile-volume wiring pending).
- [ ] Optional: 14-day lifecycle rule on the backups bucket as a retention backstop.

## 8. Frontend deployment

- [x] ✅ **Hosting decided & integrated (2026-07-18):** Cloudflare via the OpenNext adapter (`@opennextjs/cloudflare` + `wrangler.jsonc` + `open-next.config.ts`; scripts `build:cf`/`preview:cf`/`deploy:cf`, each chaining the prod-env guard). Production build + `wrangler deploy --dry-run` verified locally. Nothing deployed.
- [ ] Real build env from `frontend/.env.production.example` once the domain exists (`NEXT_PUBLIC_API_URL` https, `NEXT_PUBLIC_WS_URL` wss, `NEXT_PUBLIC_API_BASE` = API origin — template now includes it, `NEXT_PUBLIC_ENV=production`).
- [ ] `NEXT_PUBLIC_R2_PUBLIC_BASE` when serving images from the uploads bucket; `NEXT_PUBLIC_BASE_DOMAIN`/`NEXT_PUBLIC_GUEST_URL` per tenancy/QR model.
- [ ] First real `npm run deploy:cf` with production values (no guard bypasses) + custom-domain attachment.

## 9. Data & tenant bootstrap

- [ ] First platform super-admin created with `docker compose exec ... /app/bootstrap-admin` (DEPLOYMENT.md §10), then TOTP enrolled and recovery codes stored in a password manager.
- [ ] Restaurant #1: org + branch + tables + QR collateral printed + menu entered + staff roster with PINs + theme selected (platform onboarding flow + manual steps; ~half a day supervised).
- [ ] Billing handled **manually outside the system** (billing subsystem is shadow — do not charge through it).
- [x] Promo daily windows compare against branch-local time; integration coverage exists.

## 10. Pre-open verification gates

- [ ] `/readyz` 200 through the full edge path (TLS → nginx → app); WS connect + live cart sync from a real phone on mobile data.
- [ ] Kill-test: `docker stop` redis on a throwaway rehearsal (NOT the soak stack, NOT live prod during service) — degraded-but-serving behavior confirmed, alert received by a human.
- [ ] A full guest journey on production infra: QR scan → join → shared cart → order → kitchen → serve → bill → cash settle → session close.
- [ ] Payment-escalation drill: stall a settlement >5m in rehearsal; confirm warn alert + staff-facing `PAYMENT_SETTLEMENT_STALLED`.
- [ ] Restore drill re-run within a week of go-live using a real production nightly dump.
- [ ] Runbooks reachable and current: `operational-runbooks.md`, `support-runbooks-pilot.md`, `recovery-procedures.md`, `restaurant-go-live-checklist.md`.

## 11. Known open defects to track (not env, but pre-open awareness)

Track the `session_sequences` hot-spot ceiling (~concurrency 150, far beyond pilot).
R4–R6 are enabled for launch; guest legacy IDs are session-scoped as defense-in-depth.
Reconnect exhaustion now exposes an in-place Retry action, and promo windows use branch-local time.

---

## Appendix A — Environment variable audit

Legend: **Req** = required in production (how enforced) · Status flags: 🔴 UNSAFE-DEFAULT · 🟠 MISSING (not set in any prod-path file) · ⚪ OBSOLETE/UNUSED · 🔁 DUPLICATE-NAME.
Backend vars are all read in `backend/internal/config/config.go` (`Load()`); `GIN_MODE=release` triggers `validateReleaseSecurity()` — there is no `APP_ENV`.

### A.1 Backend — core

| Variable | Purpose | Req | Default | Current status | Prod ready? | Action |
|---|---|---|---|---|---|---|
| `DATABASE_URL` | Postgres DSN | **Yes — fails hard always** | — | `.env` points at `localhost` + placeholder pw | ❌ | Rewrite for container host `postgres`, strong pw |
| `REDIS_URL` | Redis DSN | **Yes — fails hard always** | — | `.env` points at `localhost` | ❌ | Host `redis` |
| `GIN_MODE` | debug/test/**release** (prod trigger) | Yes (=release) | `release` | 🔴 `.env` says `debug`; compose overrides to `release` | ⚠️ | Set `release` in prod `.env` for consistency |
| `GUEST_TOKEN_SECRET` | HMAC key for guest tokens | **Yes in release** (≥32, non-dev-default or boot refused) | dev sentinel | 🔴🟠 not in any `.env` | ❌ | Generate ≥32-char secret |
| `CORS_ALLOWED_ORIGINS` | CORS + WS origin allowlist | **Yes in release** (empty ⇒ boot refused) | `""` 🔴 (empty accepts all WS origins) | localhost list in `.env` | ❌ | Real https origins |
| `MFA_ENCRYPTION_KEY` | AES-GCM for TOTP secrets | Conditional (≥16 if set; unset ⇒ MFA fails closed) | `""` | 🟠 unset | ❌ (platform login needs it) | Generate ≥16-char key |
| `PAYMENT_WEBHOOK_SECRET_<PROVIDER>` | Webhook HMAC per provider | Conditional (≥16 if set) | none | 🟠 unset; 🔁 named `WEBHOOK_SECRET` in chaos, `WEBHOOK_SECRET_STRIPE` in e2e | ⚠️ N/A until gateway | Set when gateway integrated |
| `PORT` / `READ_TIMEOUT` / `WRITE_TIMEOUT` / `SHUTDOWN_TIMEOUT` | HTTP server | No | 8080 / 10s / 30s / 15s | set in `.env` | ✅ | — |
| `TRUSTED_PROXIES` | Proxy CIDRs for client IP | No | `172.16.0.0/12` | 🔁 drift: `.env` `127.0.0.1/8` vs example | ⚠️ | Set to real proxy network CIDR |
| `BASE_DOMAIN` | Subdomain tenant extraction | No | `""` | 🟠 unset | ⚠️ | Set if subdomain tenancy used |
| `ENABLE_HSTS` | HSTS header | No | `false` | unset | ⚠️ | `true` once TLS live |
| `RATE_LIMIT_RPM` / `AUTH_RATE_LIMIT_RPM` | Rate limits | No | 60 / 10 | set / unset | ✅ | — |
| `LOG_LEVEL` / `LOG_PRETTY` | Logging | No | info / false | `.env` has `LOG_PRETTY=true` | ⚠️ | `false` in prod |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` / `DB_MAX_CONN_LIFETIME` / `DB_MAX_CONN_IDLE_TIME` | PG pool | No | 20 / 2 / 1h / 30m | unset | ✅ (pilot scale) | Revisit at scale |
| `GUEST_TOKEN_TTL` | Guest token TTL | No | 12h | production template sets 12h | ✅ | Keep until refresh exists |
| `AUTH_STAFF_COOKIE_ENABLED` | HttpOnly staff cookie | No | false | unset | ✅ | — |
| `PAYMENT_WEBHOOK_TIMESTAMP_TOLERANCE` | Replay window | No | 5m | unset | ✅ | — |
| Presence/worker knobs: `HOST_ABSENCE_GRACE`, `STALE_SESSION_INTERVAL`, `PRESENCE_EXPIRY_INTERVAL`, `SESSION_RECONCILE_INTERVAL`, `WORKER_REGION`, `SESSION_PRESENCE_GRACE`, `SESSION_IDLE_GRACE`, `SESSION_REACTIVATION_WINDOW`, `PAYMENT_PENDING_ESCALATION_INTERVAL`, `PAYMENT_PENDING_WARN_AFTER`, `PAYMENT_PENDING_CRITICAL_AFTER` | Host transfer and worker/escalation timing | No | 3m/5m/60s/5m/default/60s/5m/5m/1m/5m/15m | partially set | ✅ (tuned post-rehearsal) | — |
| `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET`, `R2_PUBLIC_BASE` | App image uploads (all 5 or uploads 503) | No | `""` | 🟠 unset; 🔁 backup uses different R2 names | ⚠️ | Set for `qr-dining-uploads` bucket |

### A.2 Backend — the 9 strict rollout flags

| Flag | Wave | Set in prod-path files? | Action |
|---|---|---|---|
| `AUDIT_LOG_V2_ENABLED` | R1 ✅ live | production template: `true` | Keep `true` |
| `TENANCY_ORGANIZATIONS_ENABLED` | R2 | production template: `false` | Backfill before enabling |
| `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS` | R3 pair | production template: `false` | Keep shadow |
| `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED` | R4 pair | production template + code defaults: `true` | Keep `true` |
| `WS_TICKET_AUTH_REQUIRED` | R5 | production template + code default: `true` | Keep `true` |
| `AUTH_GUEST_CREDENTIALS_REQUIRED` | R6 | production template + code default: `true` | Keep `true`; pair with 12h TTL |
| `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | R7 | production template: `false` | Enable with a real gateway |

### A.3 Compose / infrastructure

| Variable | Purpose | Notes |
|---|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | postgres image init (compose interpolation from `.env`) | ⚪ never read by the app; 🔁 must stay consistent with creds inside `DATABASE_URL`. `changeme_strong_password` must be replaced |
| `NODE_EXPORTER_TEXTFILE_DIR` | backup script metric output | 🟠 must point at the `backup_textfile` volume path or backup alerts are blind |

### A.4 Backup job (`/etc/qr-dining/backup.env`)

| Variable | Purpose | Default | Note |
|---|---|---|---|
| `BACKUP_PROVIDER` | r2 / s3 / local | `r2` | — |
| `R2_ENDPOINT`, `R2_BUCKET`, `R2_ACCESS_KEY`, `R2_SECRET_KEY` | R2 target + creds | — | 🔁 different names from app R2 vars — separate credential set for `qr-dining-backups` |
| `S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `S3_ENDPOINT_URL` | s3 alternative | region `us-east-1` | — |
| `BACKUP_LOCAL_DIR` | local provider dir | — | testing path (used by the restore certification) |
| `RETENTION_DAYS` / `BACKUP_PREFIX` | retention / key prefix | 14 / `backups` | legacy `backup.sh` had its own 30-day default (stale script) |
| `DATABASE_URL` | dump source | — | host-reachable DSN (published port or docker network access) |

### A.5 Frontend (build-time)

| Variable | Purpose | Req | Current status | Action |
|---|---|---|---|---|
| `NEXT_PUBLIC_API_URL` | REST base | **Yes** (prebuild guard fails on unset/localhost/http) | `.env.production` = placeholder `https://api.domain.com` | Real API origin |
| `NEXT_PUBLIC_WS_URL` | WS base | **Yes** (guard; wss required) | placeholder | Real wss origin |
| `NEXT_PUBLIC_API_BASE` | CSP connect-src host | Should | 🟠 set **nowhere** → prod CSP omits API host; 🔁 duplicates API_URL conceptually | Set = API origin |
| `NEXT_PUBLIC_ENV` | env flag; `development` **bypasses the prod guard** | Yes (`production`) | set in `.env.production` | Keep `production` |
| `NEXT_PUBLIC_R2_PUBLIC_BASE` | CSP img-src for R2 | If uploads used | 🟠 unset | Set with uploads bucket |
| `NEXT_PUBLIC_BASE_DOMAIN` / `NEXT_PUBLIC_TENANT_SLUG` / `NEXT_PUBLIC_GUEST_URL` | tenancy/QR origin | Optional | unset | Per tenancy model |
| `ALLOW_LOCALHOST_BUILD` / `ALLOW_INSECURE_URLS` | guard bypasses | Never in prod | — | Must be unset |

### A.6 Obsolete / tooling-only (never set in prod)

| Variable | Where | Verdict |
|---|---|---|
| `E2E_ADMIN_TOKEN` | set in `backend/.env`; backend **never reads it** (e2e harness only) | ⚪ remove from backend/.env eventually |
| `TEST_DATABASE_URL`, `TEST_REDIS_URL`, `RUN_PHASE0_GUARDRAIL_TESTS` | integration tests | test-only |
| `APP_URL`, `EDGE_URL`, `WEBHOOK_SECRET`, `OUTAGE_SECONDS`, `APP_CONTAINER`, `REDIS_CONTAINER`, `NGINX_CONTAINER`, `PROVIDER` | chaos harness | tooling-only; 🔁 `WEBHOOK_SECRET` mirrors app's `PAYMENT_WEBHOOK_SECRET_<P>` by hand |
| `BASE_URL`, `WS_URL`, `CONCURRENCY`, `DURATION`, `WS_HOLD`, `BRANCH_ID`, `TABLE_IDS`, `MENU_ITEM_IDS`, … | loadtest | tooling-only |
| `API_URL`, `APP_URL`, `CI`, `WEBHOOK_SECRET_STRIPE` | Playwright e2e | tooling-only |

**Secret hygiene note:** no real secrets are committed anywhere — every value in tracked/untracked env files is a dev placeholder. All production secrets are net-new and must be generated at provisioning time.
