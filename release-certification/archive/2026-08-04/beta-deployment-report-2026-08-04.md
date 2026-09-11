# Beta Deployment & Infrastructure Validation Report

**Date:** 2026-08-04 · **Environment:** internal beta (NOT production) · **VM:** Oracle Ampere arm64, 4 vCPU / 23 GB
**Branch deployed:** `feature/signoz-observability` · **Backend built from:** `b57f746`
**Status:** deployed, validated, and ready for manual certification — with three items to read before you start.

---

## 1. What this environment is

A production-shaped deployment of the RC on the shared OCI VM, reachable from real devices over real TLS.
It is not production: it uses beta hostnames, seeded data, a local alert sink, and one shared R2 credential pair.

| Surface | URL |
|---|---|
| Frontend (guest / staff / platform) | `https://qr-beta.mohith16.com` |
| API — load balanced across both instances | `https://qr-api-beta.mohith16.com` |
| API — pinned to instance 1 | `https://qr-api-beta-1.mohith16.com` |
| API — pinned to instance 2 | `https://qr-api-beta-2.mohith16.com` |
| Prometheus / Alertmanager | `ssh -L 9090:127.0.0.1:9090 -L 9093:127.0.0.1:9093 appuser` |
| SigNoz | `ssh -L 3301:127.0.0.1:3301 appuser` → http://localhost:3301 |

13 containers, all healthy: 2 backends, Postgres 17, Redis 7, Prometheus, Alertmanager, blackbox, node-exporter,
alert sink, and the 4-service SigNoz stack. 4.6 GB of 23 GB RAM; 19 GB disk free.

---

## 2. Two facts that shaped the deployment

**The container image had never been published.** `ghcr.io/mohith1612/qr-dining` returns *Package not found* —
`ci.yml` only pushes on `main` and `v*` tags, and PR builds run `push: false`. `DEPLOYMENT.md` §3's
`docker compose pull` could not have worked. The image is now built natively on the arm64 VM (2m21s, no QEMU)
and tagged `qr-dining:beta-<sha>` — deliberately *not* under the `ghcr.io` name, so a local artifact never
masquerades as a published one. Documented as `DEPLOYMENT.md` §3b.

**The RC's HEAD commit is not on the remote.** `03eb13c` (PostHog analytics, 39 files) exists only on your
laptop; `origin` is at `b57f746`. Two consequences:

- **PR #1 does not contain the PostHog work**, so the "CI green" you have is for the branch *without* it.
- The commit is **frontend-only** (verified: zero changes under `backend/`), so the backend image built from
  `b57f746` is byte-identical to the RC's backend. The frontend was built from your local checkout and *does*
  include PostHog — confirmed present in the deployed bundle.

Nothing was pushed, merged, or tagged.

---

## 3. Defects found and fixed

All five were found by running the thing, not by reading it. Each is fixed, verified on the live stack, and
committed (`5af1ffa`).

### 3.1 nginx dropped the real client IP — *also present in the production template*

`proxy_set_header` at server level is inherited by a `location` **only if that location sets no
`proxy_set_header` of its own**; nginx replaces the set rather than merging it. `location /` sets `Connection`
and `location = /ws` sets `Upgrade`/`Connection`, so both silently discarded `Host`, `X-Real-IP` and
`X-Forwarded-For`.

Every request logged `client_ip: 172.18.0.2` — the nginx container. Impact: **all guests share one rate-limit
bucket** (one noisy client can throttle a whole restaurant) and audit logs record the proxy instead of the actor.

Fixed in both `qr-dining-beta.conf.example` and `qr-dining.conf.example`. After: real client IP logged, and 80
concurrent requests produce exactly 60×200 then 429 — matching `RATE_LIMIT_RPM=60`.

### 3.2 Background workers ran twice — a configuration trap worth knowing about

`internal/worker/worker.go` locks on `worker:<region>:<name>:lock`, so `WORKER_REGION` is the workers'
**coordination domain, not an instance label**. Two instances with different regions take different locks and
both run every sweep — duplicate payment escalations, duplicate reactivation transitions.

My overlay initially copied `manual-testing-up.sh`'s `mtest-1`/`mtest-2`, which exists precisely to *simulate two
regions*. Observed before the fix: both instances ran the reactivation pipeline independently. After setting both
to `WORKER_REGION=beta`: three runs in 20 minutes, each on exactly one instance.

The app's locking is correct; the trap is that the misconfiguration is silent.

### 3.3 SigNoz logs: collected nothing, and would have been wrong if they had

Three separate faults, each of which looks like "the app is just quiet":

- Docker's data-root here is **`/data/docker`**, not `/var/lib/docker`. The hardcoded mount produced an empty
  directory rather than an error. Host path is now configurable.
- The collector runs as uid 10001; container log files are root-owned `0600`.
- `trace_id`/`span_id` were parsed into *attributes* but never promoted onto the log record, so SigNoz's
  trace↔logs pivot returned nothing while a log search looked perfectly healthy.

Also corrected: the log filter matched `resource.attributes["container.name"]`, which the filelog receiver never
sets — and being guarded by `!= nil`, it dropped **nothing**. On this shared VM every other project (grafana,
loki, tempo, job-queue, sketchiple, clickhouse) would have been ingested into qr-dining's telemetry. It now
matches the app's own `service="qr-dining"` stamp: correct, container-agnostic, and automatically covers both
instances. Verified: only `qr-dining` present.

### 3.4 The documented observability bring-up could not start

With multiple `-f` files, Compose resolves relative paths against the **project directory**, not each file's own
directory — so `./prometheus.yml` resolved to `/opt/qr-dining/prometheus.yml` and the container died with
"Are you trying to mount a directory onto a file". `DEPLOYMENT.md` §5's invocation was broken as written.
Fixed with absolute-path overrides in the VM overlay, leaving the base file usable locally.

### 3.5 The backup job needed an image and two networks

`postgres:17-alpine` has neither `pg_dump`-plus-`aws` nor egress on the app's internal network, so the runtime
`apk add` failed as the misleading `aws-cli (no such package)`. Added `deploy/backup/Dockerfile`.

The job also needs Postgres (internal, **no egress**) and R2 (**egress**) simultaneously; the first symptom is an
opaque "Could not connect to the endpoint URL" *after* a successful dump. It now attaches to a dedicated
`qr-dining_backup_egress` network alongside the internal one.

---

## 4. Validation results

| Check | Result |
|---|---|
| `/readyz` over public TLS, all three hostnames | 200, cert verified (`*.mohith16.com`, valid to 2026-10-27) |
| Migrations | applied at boot, **schema v39**, not dirty, 59 tables |
| Guest flow | QR → table resolve → session → signed guest token |
| Staff auth | works with `branch_code` + `staff_code` + PIN |
| Platform auth | works (seeded admin + bootstrapped super-admin) |
| **Redis fan-out across instances** | cart write via instance 1 → `CART_UPDATED` on a socket attached to **instance 2** |
| **WS ticket portability** | ticket minted on instance 1 **redeemed on instance 2**; single-use enforced (reuse → 401) |
| Worker coordination | 3 runs / 20 min, each on exactly one instance |
| Rate limiting | exactly 60×200 then 429 — per real client IP |
| CORS | allowed origin echoed; `evil.example.com` correctly gets no ACAO header |
| R2 upload | presign → PUT → public fetch, **byte-identical** round-trip |
| Backup | dump → sha256 → R2 → manifest → retention prune (2 stale objects) |
| Restore | downloaded, **checksum matched**, restored to scratch DB, **all 11 table row-counts identical** |
| Alert delivery | synthetic alert **observed arriving** at the sink; real `BackupTooOld` fired then cleared after the backup |
| Prometheus | 5/5 targets up, both instances scraped separately, 27 rules loaded |
| Traces | spans in ClickHouse from `qr-dining-backend` |
| Logs + pivot | log records join to real spans incl. DB `pool.acquire` child spans |
| Frontend | all routes 200; API origin and PostHog present in the deployed bundle; CSP allows API + WSS + R2 |
| Shared proxy | grafana / queue-api / canvas-ws unaffected across every reload |

---

## 5. A correction to an earlier claim of mine

During the audit I reported that **both R2 buckets were empty**, casting doubt on `DEPLOYMENT.md` §6's claim of a
verified live round-trip on 2026-07-18.

**That was wrong.** I checked with `wrangler r2 object get`, and wrangler 4.x defaults to *local simulation*
unless `--remote` is passed. Checked properly via the S3 API, the backups bucket held two real dumps from
2026-07-18 and a manifest — exactly as documented. The prior verification was genuine. (Those two objects have
since been pruned by the 14-day retention policy, working as designed.)

The uploads bucket was genuinely empty; it now holds the round-trip test object.

---

## 6. Deferred — with reasons

| Item | Why | Effort to close |
|---|---|---|
| **PostHog is inert** | No project key was supplied, so the SDK is a deliberate no-op. Instrumentation code is deployed and confirmed in the bundle. | Add `NEXT_PUBLIC_POSTHOG_KEY` and redeploy the frontend |
| **SigNoz retention left at default** | The v0.129 login API moved; `/api/v1/login` now serves the SPA, and I time-boxed reverse-engineering it. Traces retain **15 days**; 106 MB used against 19 GB free, so this is not a beta risk. | Settings → Retention in the UI, ~20 seconds |
| **TLS auto-renewal is dead** | `proxy_certbot` has been `Exited (137)` for ~4 months, and its entrypoint runs `certbot renew --webroot`, which cannot renew a DNS-01 wildcard; `certbot/certbot` lacks the plugin. **Pre-existing, affects all four projects on this VM.** Cert is valid to **2026-10-27**, so the beta window is safe. | Switch `/opt/proxy` to `certbot/dns-cloudflare` with `--dns-cloudflare-credentials` |
| **One R2 credential pair for both buckets** | Only one was supplied. Docs recommend two scoped tokens so a leaked uploads key cannot reach backups. | Create a second scoped token at go-live |
| **`split_clients` has no automatic failover** | Deliberate: a static `upstream{}` resolves at config-load, so a down app would break config reloads for the three *other* projects on this shared proxy. That property is worth more here. | Revisit for production, where qr-dining likely owns its own edge |
| **GitGuardian check failing on PR #1** | Every GitHub Actions workflow passes; the failing check is the third-party app. Merge-blocking, not beta-blocking. | Review the flagged finding before merge |
| **Commits not pushed** | Two commits (`8ffd6e0`, `5af1ffa`) are local. Pushing updates an open PR and triggers CI — an outward-facing act I did not take unasked. | `git push` when you want them on the PR |

---

## 7. Operating the beta

```bash
# redeploy after a fix (data survives; only the backends restart)
ssh appuser
cd /opt/qr-dining/repo && git pull
SHA=$(git rev-parse --short HEAD)
docker build -f backend/docker/Dockerfile -t qr-dining:beta-$SHA -t qr-dining:beta ./backend
sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=beta-$SHA/" /opt/qr-dining/.env
cd /opt/qr-dining && docker compose -f docker-compose.yml -f docker-compose.beta.yml \
  -f observability/docker-compose.observability.yml -f docker-compose.observability-vm.yml \
  -f docker-compose.observability-beta.yml up -d
```

Note the frontend is built from your **laptop** (it carries the unpushed PostHog commit), not from the VM:
`cd /tmp/qr-frontend-beta && npx wrangler deploy`.

Backups run nightly at 02:30 via the `appuser` crontab → `/opt/qr-dining/run-backup.sh`, logging to
`/opt/qr-dining/backup.log`. Run it by hand any time with that same script.

**Secrets** live only in `/opt/qr-dining/.env` and `/opt/qr-dining/backup.env` (both mode 600, appuser-owned)
and in the gitignored `/tmp/qr-frontend-beta/.env.production`. None are committed, and none appear in this report.
