# DEPLOYMENT.md — QR-Dining Production Deployment Guide

**Canonical deployment guide.** Companions: `OPERATIONS.md` (day-2), `RECOVERY.md` (restore/disaster). Architecture background: `deployment-architecture.md`, `STATE-OF-THE-PROJECT.md`.

**Model:** backend on the shared OCI Ampere VM behind the central `/opt/proxy` nginx+certbot stack (per the VM Project Deployment Reference); frontend on Cloudflare (OpenNext worker + static assets); storage on Cloudflare R2.

> **Placeholders:** no domain is purchased yet. Every `CHANGEME-DOMAIN.com` in this repo is a placeholder. Bucket names `qr-dining-backups` / `qr-dining-uploads` are **temporary** and appear only in env files — see §8 for the go-live substitution list.

---

## 1. Artifact map

| Repo file | Deploys to | Purpose |
|---|---|---|
| `deploy/vm/docker-compose.yml` | `/opt/qr-dining/docker-compose.yml` | app + postgres 17 + redis 7 |
| `deploy/vm/.env.production.example` | `/opt/qr-dining/.env` (mode 600) | all runtime config/secrets |
| `deploy/vm/qr-dining.conf.example` | `/opt/proxy/nginx/conf.d/qr-dining.conf` | central-proxy vhost (TLS, WS, rate limit) |
| `deploy/observability/` (dir) | `/opt/qr-dining/observability/` | Prometheus/Alertmanager/blackbox/node-exporter |
| `deploy/vm/docker-compose.observability-vm.yml` | `/opt/qr-dining/` | VM overlay: network join + localhost-only UIs |
| `deploy/backup/qr-dining-backup.{service,timer}` | `/etc/systemd/system/` | nightly backup schedule (02:30) |
| `deploy/backup/backup.env.example` | `/etc/qr-dining/backup.env` (600, root) | backup credentials/config |
| `backend/scripts/nightly-backup.sh` + `lib/` + `restore.sh` | run from `/opt/qr-dining/repo` or a container | backup/restore executables |
| `frontend/` (OpenNext) | Cloudflare Workers | guest/staff/platform frontend |

Backend image: **`ghcr.io/mohith1612/qr-dining`** — built and pushed by CI on pushes to `main` and `v*` tags (`sha-<short>`, `<tag>`, `latest`). Deploy by pinning `IMAGE_TAG` in `/opt/qr-dining/.env`. arm64-only (Ampere).

## 2. VM prerequisites (one-time)

The shared proxy stack (`/opt/proxy`: nginx + certbot + external `proxy` network + `letsencrypt`/`certbot_webroot` volumes) must already be live — it is, for the other projects.

Additional for qr-dining:

```bash
# Postgres 17 client tools (Ubuntu 24.04 ships 16 — needs the PGDG repo):
sudo apt install postgresql-common && sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh
sudo apt install postgresql-client-17
# AWS CLI v2 (backup upload transport)
# docker compose v2.24+ (the observability overlay uses `ports: !override`)
```

> Alternative without host pg tools: run backup/restore inside `postgres:17-alpine` containers (`apk add bash aws-cli`) — this pattern is fully verified; see RECOVERY.md §3.

## 3. Backend bring-up

```bash
sudo mkdir -p /opt/qr-dining && sudo chown appuser:appuser /opt/qr-dining
cd /opt/qr-dining
# copy from a repo checkout (or scp):
cp <repo>/deploy/vm/docker-compose.yml .
cp <repo>/deploy/vm/docker-compose.observability-vm.yml .
cp -r <repo>/deploy/observability ./observability
install -m 600 <repo>/deploy/vm/.env.production.example .env
vi .env      # fill every __GENERATE__ (openssl rand -base64 48) and CHANGEME
             # set IMAGE_TAG to a specific tag for real deploys, never latest
docker compose pull
docker compose up -d
docker compose ps                        # postgres/redis/app all healthy
docker compose logs app | grep -i migrat # migrations applied at boot (schema v38)
```

Notes:
- The app self-migrates at every boot (embedded migrations, idempotent). Never run a separate migration step.
- All nine rollout flags are explicit in the env template. `AUDIT_LOG_V2_ENABLED=true` is mandatory (R1 is live); the rest stay `false` until their wave gates pass (`OPERATIONS.md` §4).
- Release mode fails hard on weak/missing `GUEST_TOKEN_SECRET` or empty `CORS_ALLOWED_ORIGINS` — if the app restarts in a loop, check `docker compose logs app` for the named missing variable.
- Never place a stray `.env` in any directory a backend process might run from — `godotenv.Overload()` lets it override the real environment.

## 4. Edge (central proxy)

```bash
cp <repo>/deploy/vm/qr-dining.conf.example /opt/proxy/nginx/conf.d/qr-dining.conf
vi /opt/proxy/nginx/conf.d/qr-dining.conf   # replace api.CHANGEME-DOMAIN.com + cert path
docker compose -f /opt/proxy/docker-compose.yml exec nginx nginx -t
docker compose -f /opt/proxy/docker-compose.yml exec nginx nginx -s reload
```

- The vhost uses request-time DNS (`resolver 127.0.0.11` + `set $qr_upstream`) **deliberately** — a static upstream would make the shared proxy fail config loads whenever qr-dining is down. Keep that pattern.
- DNS + cert (after domain purchase): point `api.<domain>` A-record at the VM, issue/extend the Let's Encrypt cert (webroot method via the existing certbot container), confirm the cert path in the vhost.
- WS timeouts (3600s) must stay above the app's 54s ping. `/metrics` stays 127.0.0.1-only.

## 5. Observability

```bash
cd /opt/qr-dining
docker compose -f docker-compose.yml \
               -f observability/docker-compose.observability.yml \
               -f docker-compose.observability-vm.yml up -d
```

- One compose project: prometheus/blackbox join `qr-dining_internal` so `app:8080` resolves; UIs bind to localhost only (reach via `ssh -L 9090:localhost:9090 …`).
- **Alert delivery:** `observability/alertmanager.yml` has a single `&notify_url` anchor — the one line to change at go-live (`OPERATIONS.md` §3). Until then alerts route to a placeholder.
- Grafana is not deployed; import `observability/grafana-dashboard.json` into any Grafana when wanted.

## 6. Backups

```bash
sudo mkdir -p /etc/qr-dining
sudo install -m 600 <repo>/deploy/backup/backup.env.example /etc/qr-dining/backup.env
sudo vi /etc/qr-dining/backup.env    # DSN, R2 creds (backups token — separate from uploads!),
                                     # NODE_EXPORTER_TEXTFILE_DIR = mountpoint of qr-dining_backup_textfile
sudo cp <repo>/deploy/backup/qr-dining-backup.{service,timer} /etc/systemd/system/
# The unit ExecStart expects the repo at /opt/qr-dining/repo (or edit the path):
sudo systemctl daemon-reload && sudo systemctl enable --now qr-dining-backup.timer
sudo systemctl start qr-dining-backup.service && journalctl -u qr-dining-backup -n 30
```

Confirm `qr_dining_backup_last_run_status 0` appears in Prometheus afterward. The full flow (pg_dump → sha256 → R2 → manifest → retention → metrics) was verified end-to-end against the real R2 bucket on 2026-07-18 — see RECOVERY.md §5.

## 7. Frontend (Cloudflare)

The frontend deploys as an OpenNext worker with static assets (`frontend/wrangler.jsonc`, worker name `qr-dining-frontend`).

```bash
cd frontend
cp .env.production.example .env.production   # real https URLs — the prebuild guard rejects localhost/http
npx wrangler login                            # once per machine
npm run deploy:cf                             # guard → next build → OpenNext bundle → wrangler deploy
```

- `build:cf` / `deploy:cf` run the prod-env guard explicitly (OpenNext bypasses npm's `prebuild` hook). The guard loads `.env.production` itself (already-exported env vars take precedence, matching Next).
- Do **not** run `build:cf`/`deploy:cf` while a `next dev` server is serving from the same `frontend/` checkout — the production build rewrites `.next/` under the dev server and corrupts it (symptom: `Cannot find module './vendor-chunks/...'`). Stop the dev server first, or build from a separate clone.
- Until the domain exists, the worker serves from its `workers.dev` URL; attach the real custom domain in the Cloudflare dashboard (or `wrangler.jsonc` routes) after purchase, then rebuild with final `NEXT_PUBLIC_*` values (they are baked at build time).
- `NEXT_PUBLIC_API_BASE` must equal the API origin or the CSP blocks API/WS calls.

## 8. Go-live substitution list (when domain/buckets are final)

| Placeholder | Where | Replace with |
|---|---|---|
| `api.CHANGEME-DOMAIN.com` | proxy vhost, `/opt/qr-dining/.env` (CORS), `frontend/.env.production` | real API subdomain |
| `app.CHANGEME-DOMAIN.com` | CORS list, `NEXT_PUBLIC_GUEST_URL` | real frontend domain |
| cert path `live/CHANGEME-DOMAIN.com` | proxy vhost | issued cert dir |
| `qr-dining-backups` (temp bucket) | `/etc/qr-dining/backup.env` → `R2_BUCKET` | final bucket (copy manifest+dumps over, or start fresh) |
| `qr-dining-uploads` (temp bucket) | `/opt/qr-dining/.env` → `R2_BUCKET`, `R2_PUBLIC_BASE`, `NEXT_PUBLIC_R2_PUBLIC_BASE` | final bucket + public base |
| `&notify_url` placeholder | `observability/alertmanager.yml` | real webhook endpoint |

No code changes are required for any of these — all are env/config lines (verified: no bucket or domain literal exists in any code path).

## 9. First-boot verification gate

1. `curl -fsS https://api.<domain>/readyz` → 200 through the full proxy+TLS path.
2. WS smoke: open the guest app from a phone on mobile data; confirm live cart sync between two devices.
3. `docker compose ps` all healthy; Prometheus targets all UP; test-fire an alert (`amtool alert add ...`) and confirm delivery.
4. Trigger one manual backup; confirm metric + object in bucket.
5. Full guest journey: QR → join → shared cart → order → kitchen → serve → bill → cash settle → session closed.

> **Release preconditions still standing (do not deploy real traffic before):** merge → `main` + tag `v1.0.0-rc.1`; fresh soak of the tagged build (SEV-0); real alert receiver. See `production-environment-checklist.md`.
