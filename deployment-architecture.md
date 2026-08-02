# QR-Dining — Deployment Architecture

**Date:** 2026-07-18 · **Branch:** `feature/certification-fixes-ui-redesign` @ `9dd869a` · **Companion:** `production-environment-checklist.md`
**Authority:** subordinate to `STATE-OF-THE-PROJECT.md` (architecture) — this document is the operational/deployment deep-dive.

> Scope: how the system is built, wired, deployed, observed, and backed up — as the repository actually expresses it today. Findings are descriptive; nothing here has been changed or deleted. Stale items are flagged in §8, not removed.

---

## 1. Directory map

```
qr-dining/
├── backend/                    # Go/Gin application (module github.com/Mohith1612/qr-dining)
│   ├── cmd/server/             #   entrypoint: pool → redis → migrations → hub → 6 workers → HTTP
│   ├── cmd/migrate/            #   dev-only migration CLI (NOT in the container)
│   ├── internal/               #   handlers → services → repositories → sqlc; domain/ = invariants
│   ├── migrations/             #   38 embedded golang-migrate files (auto-run at boot)
│   ├── sql/queries/            #   sqlc sources (drift-checked in CI)
│   ├── docker/Dockerfile       #   2-stage arm64 prod image (the only Dockerfile in the repo)
│   └── scripts/                #   nightly-backup.sh + lib/storage.sh + restore.sh (+ legacy backup*.sh)
│                               #   loadtest/, seed.go
├── frontend/                   # Next.js 15 app — guest/(staff)/(platform) route groups
│   └── scripts/check-prod-env.mjs   # prebuild guard (blocks localhost/http prod builds)
├── deploy/
│   ├── nginx/qr-dining.conf    #   edge config (HTTP-only; TLS terminated upstream)
│   ├── observability/          #   Prometheus + Alertmanager + blackbox + node-exporter compose,
│   │                           #   27 alert rules, Grafana dashboard JSON (import-only)
│   └── backup/                 #   systemd service+timer, backup.env.example, crontab.example, README
├── scripts/
│   ├── manual-testing-up|down.sh   # isolated 2-backend/2-frontend stack (project manual-testing)
│   └── chaos/                  #   chaos harness (redis-flap, backend-restart, webhook-replay,
│                               #   nginx-reload, wsflood/) + committed results/
├── e2e/                        # Playwright suites (106-spec matrix) — not run in CI
├── docs/                       # living docs (master-system-context, alerting/R2 setup guides,
│   ├── manual-testing/         #   testing dashboard + guides (HTML)
│   └── history/                #   archived release trail (unedited; corrections live in STATE doc)
├── .github/workflows/ci.yml    # lint · domain tests · sqlc drift · arm64 docker build
├── docker-compose.yml              # PROD base (project qr-dining): postgres + redis + app
├── docker-compose.staging.yml      # overlay: publishes ports + adds nginx (:18080) for burn-in/chaos
├── docker-compose.pilot-validation.yml  # datastores-only, ports 15432/16379 (rehearsal-era)
├── docker-compose.manual-testing.yml    # datastores-only, ports 25432/26379 (live, used by scripts/)
├── openapi.yaml                # certified API surface v2.2.0 (171 ops, 1:1 with router)
└── *.md                        # invariant docs, runbooks, checklists, historical reports (see §8)
```

### Directory purposes, interactions, production criticality

| Directory | Purpose | Interacts with | Prod-critical |
|---|---|---|---|
| `backend/` | The entire server: routes, services, state machines, workers, embedded migrations | Postgres, Redis, nginx (via compose), CI, Dockerfile | **Yes — the product** |
| `backend/docker/` | The one production Dockerfile (arm64 static binary, non-root, `/readyz` healthcheck) | `docker-compose.yml` `app` service, CI docker-build job | **Yes** |
| `backend/scripts/` | Backup/restore executables + load/seed tooling | systemd units in `deploy/backup/`, node-exporter textfile metrics, Prometheus backup alerts | **Yes** (nightly-backup.sh, lib/storage.sh, restore.sh) |
| `frontend/` | Single Next.js app, three audiences | Backend REST + WS; `check-prod-env.mjs` gates prod builds | **Yes — but has no deployment vehicle in-repo** (§4.7) |
| `deploy/nginx/` | Edge reverse proxy config (WS-aware, /metrics CIDR lock) | app:8080 over `proxy_network`; upstream TLS terminator | **Yes** |
| `deploy/observability/` | Metrics/alerting stack + dashboard | Scrapes app `/metrics`, probes `/readyz` via blackbox, reads backup textfile metrics | **Yes** (receiver still placeholder — §5) |
| `deploy/backup/` | Scheduling + host config templates for backups | Executes `backend/scripts/nightly-backup.sh` from `/opt/qr-dining` with `/etc/qr-dining/backup.env` | **Yes** |
| `scripts/` (root) | Manual-testing stack lifecycle + chaos harness | manual-testing compose, staging overlay, `/tmp` build artifacts | No (dev/validation tooling) |
| `e2e/` | Playwright behavioral matrix | Runs against local stacks; needs `API_URL`/`APP_URL`/`E2E_ADMIN_TOKEN` | No (quality gate, not runtime) |
| `docs/` | Living operational docs + archived history | Alerting/R2 setup guides are the human-setup runbooks for §5/§6 | Indirectly (runbooks) |
| `.github/` | CI (partial pyramid — §7) | backend only | Indirectly |
| `openapi.yaml` | Certified contract | redocly lint, cert reports | Reference |

---

## 2. Runtime topology

```
                    Internet
                       │  HTTPS/WSS  (TLS terminated at edge — Cloudflare/LB; NOT in this repo)
                       ▼
              nginx  (deploy/nginx/qr-dining.conf, HTTP :80)
              • upstream qr_app = app:8080 (keepalive 32)
              • /ws: Upgrade-aware, read/send timeout 3600s (> 54s app ping), no buffering, no limit
              • /:  30r/s + burst 60 per IP
              • /health, /readyz: unthrottled, access_log off
              • /metrics: allow 127.0.0.1, 172.16.0.0/12; deny all
                       │  proxy_network (external docker network — must be pre-created)
                       ▼
              app  (Go binary, stateless, :8080 exposed not published)
              boot: config → PG pool → Redis → RunMigrations (fatal on error) → hub → 6 workers → HTTP
                       │ backend network (internal: true — no egress, no published ports)
          ┌────────────┴────────────┐
          ▼                         ▼
   postgres:17-alpine        redis:7-alpine
   volume postgres_data      volume redis_data
   (authoritative)           (AOF, 256mb, allkeys-lru — disposable)

   Sidecar planes (same host):
   • observability project: prometheus(:9090) ── scrapes app:/metrics, blackbox→/readyz, node-exporter
                            alertmanager(:9093) ── receivers: PLACEHOLDER webhook-sink
   • systemd timer 02:30:  nightly-backup.sh → pg_dump → sha256 → R2/S3/local → manifest → retention
                            └─ writes .prom textfiles → node-exporter → BackupFailed/BackupTooOld alerts
```

---

## 3. Build

- **Image:** `backend/docker/Dockerfile`, two stages.
  - Builder: `golang:1.26-alpine`, `CGO_ENABLED=0 GOOS=linux GOARCH=arm64` (hardcoded — Oracle Ampere), `-trimpath -ldflags="-s -w"`, output `/bin/qr-dining` from `./cmd/server`.
  - Runtime: `alpine:3.20` + ca-certificates/wget/tzdata, non-root `appuser`, `EXPOSE 8080`, `HEALTHCHECK` wget `/readyz` (10s/5s/5 retries/20s start), `ENTRYPOINT /app/qr-dining`.
- Go is pinned to 1.26 across `backend/go.mod`, the Docker builder, and CI.
- **Migrations ship inside the binary** (embedded iofs). The app self-migrates at every boot, idempotently (chaos-validated). `cmd/migrate` is a developer CLI only — it is not in the image and production must never depend on it.
- **arm64-only consequence:** on amd64 dev hosts the container cannot run, which is why the pilot-validation and manual-testing stacks run the app as native host processes against containerized datastores.

## 4. Deployment trace (compose)

### 4.1 Production base — `docker-compose.yml` (project `qr-dining`)

| Service | Image/build | Network(s) | Ports | Volume | Restart | Healthcheck |
|---|---|---|---|---|---|---|
| postgres | postgres:17-alpine | `backend` (internal) | none published | `postgres_data` | unless-stopped | `pg_isready` 5s/5s/×10 |
| redis | redis:7-alpine (`--appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru`) | `backend` | none | `redis_data` | unless-stopped | `redis-cli ping` 5s/3s/×10 |
| app | build `./backend` + `docker/Dockerfile` | `backend` + `proxy` | `expose: 8080` (reachable only via proxy_network) | — | unless-stopped | wget `/readyz` 10s/5s/×5, start 20s |

- Networks: `backend` is `internal: true` (datastores have no route out and no published ports — good); `proxy` is **external** `proxy_network`, which must exist before `up` (`docker network create proxy_network`).
- `depends_on` app → postgres+redis with `condition: service_healthy`.
- App env: `env_file: ./.env` plus `environment: GIN_MODE: release` override.
- **⚠️ No resource limits** (cpus/mem) on any service, despite a single shared Ampere host.
- **⚠️ Boot-blocking env facts (verified):** the committed `.env` points `DATABASE_URL`/`REDIS_URL` at `localhost` (wrong inside the container — hosts must be `postgres`/`redis`), carries the placeholder password, sets localhost-only CORS, and omits `GUEST_TOKEN_SECRET`. Since compose forces `GIN_MODE=release`, `validateReleaseSecurity()` refuses the dev-default guest secret. **A production host requires a purpose-written `.env`; the committed one cannot boot the stack.** Details in the checklist doc, Appendix A.

### 4.2 Staging overlay — `docker-compose.staging.yml`

Run as `-f docker-compose.yml -f docker-compose.staging.yml` (same project). Publishes 5432/6379/8080 for local burn-in (comments explicitly forbid this in prod) and adds the **only nginx service in any compose file**: `nginx:1.27-alpine`, mounts `deploy/nginx/qr-dining.conf`, publishes `18080:80`, joins `proxy`.

**⚠️ Consequence: the production compose has no edge.** In prod, nginx (or an equivalent) must be run separately on `proxy_network` — nothing in the repo currently defines that unit. This is an unstated deployment assumption that must be made explicit before go-live.

### 4.3 TLS / certificates

**There is no TLS anywhere in the repo.** `qr-dining.conf` listens on :80 with `server_name _` and expects `X-Forwarded-Proto` from an upstream terminator (Cloudflare or LB). No certbot, no cert paths, no ACME tooling, no domain names. TLS strategy is a pending production decision (checklist §3).

### 4.4 Isolated validation stacks

| File | Project | Services | Ports | Referenced by |
|---|---|---|---|---|
| `docker-compose.pilot-validation.yml` | pilot-validation | postgres+redis only (`pilot`/`pilot_pass`/`qrdining_pilot`) | 15432/16379 | **no script** — rehearsal-era (stale candidate, §8) |
| `docker-compose.manual-testing.yml` | manual-testing | postgres+redis only (`mtest`/`mtest_pass`/`qrdining_mtest`) | 25432/26379 | `scripts/manual-testing-up|down.sh` (live) |

Port ladder is deliberate isolation: soak/prod 5432/6379/8080 · pilot 15432/16379 · manual-testing 25432/26379. Neither validation stack defines restart policies (fine — ephemeral).

> **Standing rule:** the soak stack (docker project `qr-dining` on the dev host) is never touched — no `down -v`, no restarts without `AUDIT_LOG_V2_ENABLED=true`, no volume wipes.

### 4.5 Execution order (production bring-up)

1. Provision host (arm64, docker + compose, aws-cli, postgresql-client 17) and deploy repo to `/opt/qr-dining`.
2. Write the real `/opt/qr-dining/.env` (container-host DSNs, strong secrets, https CORS, explicit rollout flags). Release mode ignores dotenv files, so injected environment remains authoritative.
3. `docker network create proxy_network`.
4. `docker compose up -d --build` (project `qr-dining`) — app waits on healthy datastores, self-migrates, `/readyz` gates healthy.
5. Start the edge: nginx with `deploy/nginx/qr-dining.conf` attached to `proxy_network` (mechanism TBD — see §4.2), TLS terminated in front of it.
6. Observability: `docker compose -f docker-compose.yml -f deploy/observability/docker-compose.observability.yml up -d` (merged — see §5 network caveat), after replacing the Alertmanager placeholder receiver.
7. Backups: install `deploy/backup/qr-dining-backup.{service,timer}`, create `/etc/qr-dining/backup.env` (600, root), `systemctl enable --now qr-dining-backup.timer`; wire `NODE_EXPORTER_TEXTFILE_DIR` to the shared `backup_textfile` volume path.
8. Frontend: deploy by whatever vehicle is chosen (§4.7) with prod `NEXT_PUBLIC_*` env; the prebuild guard will refuse localhost/http endpoints.

### 4.6 Dependency ledger

| Component | Depends on | Failure behavior |
|---|---|---|
| app | Postgres (hard: boot + readyz), Redis (hard at boot; degraded at runtime — realtime pauses, sensitive limiters fail closed), migrations (fatal), `.env` secrets (fatal in release) | `/readyz` 503 → container unhealthy; restart: unless-stopped |
| nginx | app resolvable on proxy_network | 502s; reload-safe (chaos-validated) |
| prometheus | app `/metrics` reachability (network-merge caveat §5), alert rules file | silent scrape gaps |
| alertmanager | prometheus; a REAL receiver | alerts fire into placeholder sink = nobody paged |
| backup job | DATABASE_URL reachability from host, provider creds, aws-cli | non-zero exit → `qr_dining_backup_last_run_status 1` → BackupFailed alert (only if observability is live) |
| frontend | backend URL/WS URL baked at build (NEXT_PUBLIC_*) | build-time guard; runtime dead API if misconfigured |

### 4.7 Frontend production story — undefined

No frontend Dockerfile, no compose service, no `output: standalone`/static export. Today it only runs as native `next dev`/`next start` processes (manual-testing stack: :3000/:3001). Production hosting (Vercel vs node process on the host vs container) is an **open decision** — the only guardrails are `check-prod-env.mjs` (fails builds pointing at localhost/http) and the CSP built from `NEXT_PUBLIC_API_BASE`/`NEXT_PUBLIC_R2_PUBLIC_BASE` in `next.config.ts` (neither var is set in any env file today).

---

## 5. Observability plane (`deploy/observability/`)

| Piece | Detail |
|---|---|
| prometheus v2.54.1 | :9090, 15d retention, 15s scrape/eval. Jobs: `qr-dining` (app:8080/metrics), `qr-dining-readyz` (blackbox probe of app:8080/readyz), `node` (node-exporter:9100) |
| alertmanager v0.27.0 | :9093. Route: group by alertname+severity; `severity=page` → `urgent` receiver (repeat 1h), rest → `default` (repeat 4h). **Both receivers are placeholder webhooks (`http://webhook-sink:5001/...`); Slack/email/PagerDuty blocks are commented `CHANGE_ME`.** Inhibit: page suppresses same-alertname ticket |
| blackbox v0.25.0 | module `readyz_http_2xx` (200-only ⇒ 503 = probe failure), no published port |
| node-exporter v1.8.2 | `--collector.textfile.directory=/var/lib/node_exporter/textfile` ← shared `backup_textfile` volume (backup metrics ingress) |
| alert rules | `prometheus-alerts.yml`: **27 rules, 11 page / 16 ticket**, 9 groups — availability, datastores, websocket, workers, integrity, **rollout-gates** (LegacyAuthzBypassPresent, LegacyIdentityUsagePresent, PolicyShadowMismatch… — must read zero before flag flips), enforcement, readyz-probe, backup (BackupFailed, BackupTooOld >36h with absent() guard) |
| grafana | **No Grafana service is deployed.** `grafana-dashboard.json` (uid `qr-dining-ops`, ~23 panels) is import-only into a Grafana you stand up separately |

**⚠️ Standalone-broken:** the compose header says to attach via `OBS_NETWORK`, but no such variable is wired — the file defines only its own `observability` bridge, so `app:8080` does not resolve unless the stack is **merged** with the prod compose (`-f docker-compose.yml -f deploy/observability/docker-compose.observability.yml`). Also note `prometheus-scrape-readyz.yml` is a leftover documentation snippet whose content is already merged into the live configs.

---

## 6. Backup & storage subsystem

**Design:** two fully separate R2 concerns — (a) **database backups** (`qr-dining-backups` bucket, private; this section) and (b) **app image uploads** (`qr-dining-uploads` bucket, public; configured by app-side `R2_*` vars). They intentionally use different credentials and, confusingly, different variable names (checklist Appendix A, DUPLICATE flags).

### Nightly flow (`backend/scripts/nightly-backup.sh` — the live path)

1. `pg_dump --format=custom --compress=9` → `backup_YYYYMMDD_HHMMSS.dump` (UTC).
2. `sha256sum` of the dump.
3. `storage_upload` via `backend/scripts/lib/storage.sh` → key `<BACKUP_PREFIX>/YYYY/MM/backup_<ts>.dump` (default prefix `backups`).
4. Manifest: download-append-reupload `<prefix>/manifest.jsonl` — one JSON line per backup: `{timestamp,key,bytes,sha256,provider,retention_days}`.
5. Retention: `RETENTION_DAYS` (default 14), filename-date-driven prune of remote objects + local workdir; optional bucket lifecycle rule recommended as backstop.
6. Metrics: atomically writes `qr_dining_backup_status.prom` (`qr_dining_backup_last_run_status` 0/1 — an ERR trap writes 1) and `qr_dining_backup_success.prom` (`..._last_success_timestamp_seconds`) into `$NODE_EXPORTER_TEXTFILE_DIR` (no-op if unset). Separate files so a failure never erases last-success.
7. `set -euo pipefail`: any failure exits non-zero (journal + alert visibility).

### Provider abstraction (`lib/storage.sh`)

`BACKUP_PROVIDER` = `r2` (default) | `s3` | `local`; transport is the **AWS CLI** for both cloud providers.

| Provider | Required env | Notes |
|---|---|---|
| r2 | `R2_ENDPOINT`, `R2_BUCKET`, `R2_ACCESS_KEY`, `R2_SECRET_KEY` | `--endpoint-url … --region auto`; note names differ from the app's upload vars |
| s3 | `S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (+ optional `S3_ENDPOINT_URL`, `AWS_REGION`) | same code path as r2 |
| local | `BACKUP_LOCAL_DIR` | cp/find/rm — pipeline testing without cloud; **this is the provider the restore certification exercised** |

### Scheduling

`deploy/backup/qr-dining-backup.service` (oneshot, root, `EnvironmentFile=/etc/qr-dining/backup.env`, `ExecStart=/opt/qr-dining/backend/scripts/nightly-backup.sh`, Nice=10) + `qr-dining-backup.timer` (`OnCalendar=*-*-* 02:30:00`, `Persistent=true`). Cron alternative in `crontab.example`. Both hardcode deploy-target paths (`/opt/qr-dining`, `/etc/qr-dining`) that provisioning must create.

### Restore & verification status

- `backend/scripts/restore.sh`: interactive confirm → `pg_restore --clean --if-exists --single-transaction` (atomic) → row-count validation over sessions/orders/payments/staff/menu_items/event_log.
- `restore-verification-report.md` (2026-06-09) certifies a byte-faithful round-trip — 56/56 tables row-identical, checksum match, audit-immutability trigger intact — **but via the `local` provider only, at small volume**.
- **Closed (2026-07-18): a full round-trip ran against the real production R2 bucket and PASSED** (RECOVERY.md §5), following the 4-phase procedure in `docs/r2-production-setup.html` (creds check → probe object → live nightly-backup run → live restore into a throwaway postgres:17 container). Restore duration at production data volume remains unmeasured — re-drill during week 1 of go-live.

---

## 7. CI (`.github/workflows/ci.yml`)

Push/PR to `main` and `v*` tags; backend-only; Go **1.24**. Jobs: `lint` (gofmt, vet, golangci-lint) · `test` (`go test -race ./internal/domain/...` — **domain package only**) · `sqlc-check` (drift) · `docker-build` (arm64; PRs build only, pushes to `main`/`v*` publish `ghcr.io/mohith1612/qr-dining`).

**Not in CI:** the 13 integration test files, the 106-spec Playwright matrix, any frontend lint/build/typecheck. This is the known "worst accidental debt" item (STATE Part 10) — the RC gate currently rests on local runs.

---

## 8. Stale / obsolete inventory (flagged only — nothing deleted)

### Obsolete (safe-to-remove candidates, pending owner confirmation)

| Item | Why stale | Referenced by |
|---|---|---|
| `server` (root, 32MB) | build artifact; already `git rm`-staged, `.gitignore` updated | nothing |
| `bin/server` (35MB) | untracked, gitignored orphan binary | nothing |
| `docker-compose.pilot-validation.yml` | one-time dress-rehearsal stack; superseded by manual-testing | **no script**; only history docs |
| `backend/scripts/backup.sh`, `backup_r2.sh` | legacy 2-step backup; superseded by `nightly-backup.sh` (units reference only the latter) | nothing |
| `deploy/observability/prometheus-scrape-readyz.yml` | doc snippet; content merged into live configs | nothing |
| `CODEBASE.md` | superseded by `STATE-OF-THE-PROJECT.md` | — |
| `.agents/`, `.codex/`, `frontend/features/*` (7 dirs) | empty scaffolding (STATE-flagged) | nothing |
| `scripts/chaos/results/` (2026-05-25 runs) | historical run evidence | chaos README (as examples) |

### Superseded root reports (→ belong in `docs/history/`)

`final-hardening-phase-a/b-report.md` · `final-pilot-readiness-report.md` (explicitly superseded by `docs/history/final-release-readiness-report.md`) · `pilot-load-validation-report.md` · `api-surface-certification-report.md` · `billing-ui-verification.md` · `staging-burnin-strategy.md` · `playwright-behavioral-matrix.md` · `staff-analytics-loyalty-summary-v1.md`. Aging-but-partly-live: `alerting-setup.md` (backup-metric wiring still accurate), `production-enforcement-rollout.md` (flags still pending), `e2e-impact.md`.

### Live but untracked (needs `git add`, not cleanup)

`STATE-OF-THE-PROJECT.md` · `docs/history/` · `docs/manual-testing/` · `docs/master-system-context-v1.md` and sibling living docs/HTML guides · `docker-compose.manual-testing.yml` (live scripts depend on it) · `e2e/package-lock.json`.

### Configuration drift worth noting (not files, but staleness in place)

- Chaos default container names (`qr-app-chaos`, `qr-nginx-chaos`) don't match compose-generated names (`qr-dining-app-1`, `qr-dining-nginx-1`) — harness requires `*_CONTAINER` overrides.
- Committed root `.env` is a dev file that the prod compose consumes as `env_file` — see §4.1 and the checklist.
- `TRUSTED_PROXIES` differs between `.env` (`127.0.0.1/8`) and `.env.example` (`172.16.0.0/12`).

---

## 9. Never touch

The soak stack (docker project `qr-dining` incl. `qr-app-soak`/`qr-app-chaos` containers and volumes) · `audit_log` schema/trigger · session transition table · bill-snapshot semantics · loyalty ledger constraints · the additive-migration rule. All experimentation belongs on the manual-testing stack or throwaway databases.
