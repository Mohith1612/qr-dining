# Operations

Last verified against code and deployment configuration: 2026-09-18.

Deployment is no longer manual and no longer unverified: see
[Automatic deployment](#automatic-deployment), which was executed against the
production VM on 2026-09-18 and which is now the only supported path. The SigNoz
material remains configuration-derived and unexecuted.

## Deployment shape

The VM compose file defines one arm64 application image from GHCR, PostgreSQL 17,
and Redis 7. PostgreSQL and Redis use an internal network; only the application
joins the shared proxy network. All three services have memory limits, bounded
JSON logs, persistent volumes, and health checks
(`deploy/vm/docker-compose.yml`). The beta overlay adds a second instance,
`qr_dining_app_2`, against the same Postgres and Redis
(`deploy/vm/docker-compose.beta.yml`).

**Two nginx configs exist in this repository and only one of them is production.**
`deploy/vm/nginx/qr-dining-beta.conf` is the vhost the shared `proxy_nginx`
actually loads. `deploy/nginx/qr-dining.conf` is the single-app edge used by
`docker-compose.staging.yml`; it defines an `upstream qr_app { server app:8080 }`
that does not exist on the VM and has never run there. Citations about production
routing belong to the former. [How a request reaches the app](#how-a-request-reaches-the-app)
is the full path.

The application image is built for `linux/arm64`; CI pushes only on main or tag
events and emits SHA, version-tag, and default-branch `latest` tags
(`.github/workflows/ci.yml`, job `docker-build`). Production compose accepts
`IMAGE_REPO`/`IMAGE_TAG` and falls back to `ghcr.io/mohith1612/qr-dining:latest`
(`deploy/vm/docker-compose.yml`). `deploy/vm/deploy.sh` always writes an
immutable `sha-<short>` tag; `latest` is not a release pin and nothing automatic
deploys it.

Note that `docker-build` has **no `needs:`**, so a tag in GHCR proves a build
succeeded, not that the release did. See
[why a CI gate exists](#step-2-why-a-ci-gate-exists-at-all-given-branch-protection).

### Deploying by hand (break-glass only)

**The supported path is [Automatic deployment](#automatic-deployment).** Merging
to `main` deploys. The commands below exist for the case where that path is
itself broken, and they give up every guarantee it provides: no migration
pre-flight, no CI gate, no health gate, no rollback, no report, and — because
`up -d` without a service name recreates every service at once — both instances
down simultaneously rather than one at a time.

```bash
cd /opt/qr-dining
# What would the new image do to the schema? Ask BEFORE starting anything.
docker run --rm --network qr-dining_qr-dining_internal --env-file .env \
  --entrypoint /app/migrate ghcr.io/mohith1612/qr-dining:<tag> pending

# Set IMAGE_TAG in .env to the reviewed immutable tag, then, one instance at a
# time, checking /readyz in between:
docker compose pull app
docker compose up -d --no-deps app
docker compose up -d --no-deps app2
docker compose ps
curl -fsS http://localhost/readyz
```

The compose file requires a root-only `.env`, uses it both for interpolation and
the app environment, and expects the external `proxy` network to exist
(`deploy/vm/docker-compose.yml:1-6,33-59`). The environment example enumerates
the database, Redis, guest, MFA, CORS, rollout, upload, telemetry, and worker
settings (`deploy/vm/.env.production.example:19-120`). Replace every placeholder;
do not deploy that file verbatim.

The application exposes liveness, dependency readiness, and Prometheus metrics
as `/health`, `/readyz`, and `/metrics`
(`backend/internal/server/server.go:187-197`). All three answer both `GET` and
`HEAD`; see [External uptime monitors](#external-uptime-monitors) for why that
matters. Compose health uses `/readyz`
(`deploy/vm/docker-compose.yml:64-71`). Verify at least the new image tag, healthy
container state, clean schema version, and a representative guest/staff flow
before ending the change window. The repository contains no executed production
deployment report proving a stronger checklist.

### Rollback

`deploy/vm/deploy.sh --rollback` restores the image the containers ran before the
last deploy, on both instances, with the same health gate. By hand: restore the
previous `IMAGE_REPO`/`IMAGE_TAG` in `.env`, pull, and recreate each instance in
turn; the compose image reference is tag-controlled
(`deploy/vm/docker-compose.yml`).

**Never roll the schema backward**, by hand or otherwise. Migrations 16–24 and
36/38/39 destroy or fail to restore durable meaning; the verified list and
recovery procedure are in [RECOVERY.md](RECOVERY.md). The deploy path contains no
code that runs `migrate down` and will refuse to put an older binary in front of
a newer schema without a human saying so — see
[Rollback: the binary, never the schema](#rollback-the-binary-never-the-schema).

## Automatic deployment

Merging to `main` deploys to the VM. Before 2026-09-18 nothing did: CI had been
pushing images to GHCR that nothing pulled, and the box served
`qr-dining:beta-b57f746` — a locally built image from before the audit began —
for five weeks while six weeks of correctness work sat in a registry.

`deploy/vm/deploy.sh` is the whole path. It runs on the VM from `appuser`'s
crontab (`deploy/vm/deploy-crontab.example`), every two minutes.

### Why the VM pulls instead of GitHub pushing

Both were considered. The pull model won on three counts, and the third is the
one that is hardest to undo later:

1. **Nothing has to be reachable.** An Actions-driven deploy needs `sshd`
   reachable from GitHub's runner ranges. Port 22 on this box also fronts
   job-queue, sketchiple, grafana and SigNoz.
2. **There is no deploy credential to leak.** The repository is public and so is
   the GHCR package, so the VM needs no registry login and GitHub needs no SSH
   key. The usual reassurance — "fork `pull_request` events do not receive
   secrets" — is true, and it is a property you have to keep being right about
   forever on a public repo: one `pull_request_target`, one `workflow_run`
   misuse, one compromised third-party action in the deploy job. Having no key
   is not something that can be got wrong in a later PR.
3. **`appuser` has no passwordless sudo**, so a systemd unit is not installable
   and `systemctl --user` would need lingering that is off. The nightly backup
   already runs from this crontab.

What it costs: up to two minutes of latency, and GitHub's deployments UI knows
nothing about any of this. The Telegram report is what pays for that.

### The sequence

```
1  Resolve target      fetch origin/main; refuse if the checkout is not on main,
                       is dirty, or has commits origin does not
2  CI gate             GitHub check-runs for that SHA must all be green
3  Pull image          ghcr.io/mohith1612/qr-dining:sha-<short>
4  Migration pre-flight  STOP if anything is pending and unapproved
5  Config deploy       fast-forward the checkout; nginx -t; reload/recreate
6  Rolling restart     app  → health gate → app2 → health gate
                       health gate fails ⇒ restore previous image, page
```

Steps 1–4 touch nothing. The first mutation of any kind is step 5.

### Step 2: why a CI gate exists at all, given branch protection

`docker-build` in `.github/workflows/ci.yml` has no `needs:`. **The image is
pushed to GHCR on every push to `main` even if every other job failed.** A tag
existing in the registry proves that a build succeeded, not that a release did.
The gate asks GitHub what the checks actually concluded.

The gate reads two surfaces, because this repo's fifteenth check is not on the
first one. Fourteen workflow jobs report as **check-runs**; GitGuardian reports
as a **commit status**. A push-to-main commit normally carries no statuses at
all, because GitGuardian is a `pull_request`-only integration — so an empty
status list means "nothing to check" here, not "pending". Any status that does
exist and is not green stops the deploy.

Both calls are unauthenticated, which is what keeps the "no credential on the
box" property. They are subject to a 60/hour per-IP limit, and are made only
when a new commit appears — idle polling costs nothing. **A rate-limited or
unreachable API is treated as "do not know", which means "do not deploy".**

### Step 4: how pending migrations are detected

Two independent computations from two different artefacts, which must agree.

**A — the image.** `/app/migrate pending`, run from the new image against the
live database:

```bash
docker run --rm --network qr-dining_qr-dining_internal \
  --env-file /opt/qr-dining/.env \
  --entrypoint /app/migrate ghcr.io/mohith1612/qr-dining:sha-<short> pending
```

It reads the same embedded `migrations.FS` that `cmd/server` applies at boot
(`backend/migrations/embed.go`) and the same `schema_migrations` row, and writes
nothing. Nothing is closer to the truth about what the container would do on
start. Output is `key=value` lines plus an exit code that is a deploy-time API:

| exit | meaning |
| --- | --- |
| 0 | the database has applied everything this image carries |
| 10 | starting this image would apply migrations |
| 11 | `schema_migrations.dirty` is true — a migration failed part-way |
| 12 | the database is **ahead** of this image (old binary, newer schema) |

**B — the commit.** The highest `NNNNNN_*.up.sql` under `backend/migrations/` at
the deploy SHA, read with `git ls-tree` so no working tree is involved.

A alone would miss a tag that does not correspond to the commit — a retag, a
rebuild, a build-cache mishap. B alone would be asking a source tree a question
about a binary. **If they disagree the deploy stops**, because the two artefacts
are then not the same release and a schema change could arrive unannounced.

`cmd/migrate` is in the runtime image as of this change
(`backend/docker/Dockerfile`). A side effect worth knowing: `docs/RECOVERY.md`'s
wedge procedure is written as `go run ./backend/cmd/migrate force 21`, **and the
production VM has no Go toolchain** — so until now that procedure could not be
executed on the box it was written for. It can be now, via the same
`--entrypoint /app/migrate` invocation.

### Step 4: what "stop" means

The deploy exits 0, changes nothing, and pages:

> ⛔ **qr-dining deploy BLOCKED — migrations pending**
> `…:sha-abc1234` would apply 1 migration(s) at boot: 40

A human reviews the SQL and approves exactly one deploy of exactly one tag:

```bash
/opt/qr-dining/repo/deploy/vm/deploy-approve.sh sha-abc1234
```

`deploy-approve.sh` prints the pending SQL, refuses to write a token unless the
pre-flight actually exits 10 (so a stale token cannot silently authorise a later
schema change on the same tag), requires the tag typed back, and records who and
when. The next cron tick consumes it. **The token is valid for one deploy.**

This is the gate the whole design is built around. `docs/RECOVERY.md` records
that rollback is not safe across migrations 16–24 or 36/38/39, that migration 22
wedges on a populated `audit_log`, and that migration 40 raises outright if a
session already has multiple non-terminal payments. A binary can be reverted. A
schema cannot. An unattended deploy that applies a bad migration has no way
back, so it does not get to happen unattended.

### Step 5: the fast-forward IS a config deploy

`git pull` in `/opt/qr-dining/repo` changes what Prometheus, Alertmanager and —
since this change — the shared nginx proxy will load. It is not bookkeeping.

- `nginx -t` runs after every fast-forward. If it fails, the deploy **reverts
  the checkout** rather than merely skipping the reload, because a config staged
  in the working tree will be picked up by any unrelated restart later. Only if
  `deploy/vm/nginx/` actually changed is a reload issued.
- If `deploy/observability/` changed, those containers are **recreated, not
  reloaded** — see the next section for why — and the loaded rule count is
  compared against the file, the 2026-09-16 check.

### Why the nginx mount is a directory, and what that says about the others

A single-file bind mount pins the inode it was created with. `git checkout`
**replaces** files rather than rewriting them in place, so a container with a
single-file mount goes on serving content that no longer exists on disk.
Reproduced on this VM:

```
$ echo v1 > sub/f.txt; git add .; git commit -m v1
$ docker run -d --name t -v $PWD/sub/f.txt:/m/f.txt:ro -v $PWD/sub:/m/sub:ro alpine sleep 300
before:                single=v1  dir=v1
$ echo v2 > sub/f.txt; git commit -am v2      # in-place rewrite, same inode
$ git checkout HEAD~1 -- sub/f.txt            # replacement, NEW inode
after git checkout --:  single=v2  dir=v1  host=v1
                        ^^^^^^^^^  the mount is serving a version that is
                                   nowhere on disk and in no commit's worktree
```

`docker restart` re-resolves the path and picks up the new file; a **SIGHUP
reload does not**. So the practical rule is:

> **After `git pull`, a config reload is not enough for anything mounted as a
> single file. The container must be recreated or restarted.**

The nginx mount added here is a **directory** (`deploy/vm/nginx` →
`/etc/nginx/qr-dining.d`) and does not have this problem.

**The observability mounts are still single-file** — `prometheus.yml`,
`prometheus-alerts.yml`, `alertmanager.yml`, `blackbox.yml` are each mounted
individually (`deploy/vm/docker-compose.observability-vm.yml`). They were not
converted here: the paths are referenced absolutely inside `prometheus.yml`'s
`rule_files`, so changing the mount shape also changes the local and staging
invocations, and that is not a change to make in the same breath as a first
production cutover. Instead, `deploy.sh` recreates those containers whenever the
incoming range touches `deploy/observability/`.

**The residual is real and belongs to humans, not the script:** an operator who
runs `git pull` by hand in `/opt/qr-dining/repo` and then SIGHUPs Prometheus
will reload the pre-pull file and see no error. Run the rule-count check from
[Keeping the VM in step with the repo](#keeping-the-vm-in-step-with-the-repo)
afterwards, every time — it is the only thing that catches it.

### Step 6: rolling restart and the health gate

Instances are recreated one at a time, in the order `app` then `app2`. `app` is
the canary: **if it does not come up, `app2` is never touched** and is still
serving the previous image.

The gate polls `/readyz` on the container's own address on
`qr-dining_qr-dining_internal` — not through nginx (where the other instance
could answer and the gate would pass on the wrong container) and not via `docker
exec` (which fails for the wrong reason while a container is restarting). It
asserts the **body**, not the status code:

```
status == "ready"  AND  every value in checks == "ok"
```

`/readyz` returns `{"checks":{"postgres":"ok","redis":"ok"},"status":"ready"}`
and only reaches 503 when a dependency ping fails outright
(`backend/internal/handlers/health.go`). Any proxy, any stub, any half-booted
thing can return 200 — a 200 is not the assertion. The gate additionally
requires that the container is running the image ID that was just pulled, so a
container that did not actually get replaced can never be reported as deployed.

After the gate passes, the public per-instance hostname is probed through the
proxy as a separate routing assertion. It is deliberately not part of the
liveness gate: a routing failure and an application failure are different
incidents and should not produce the same page.

### Rollback: the binary, never the schema

If the gate fails, the previous image — read from the running container at the
start of the deploy, not from a state file that could have drifted — is restored
on every instance and the outcome is paged.

**The schema is never rolled back.** That constraint is in
`deploy/vm/deploy.sh`'s header and in `restore_image()`, not only here. There is
no code path in the deploy that runs `migrate down`, and the pre-flight's exit
12 exists precisely to stop an old binary being put in front of a newer schema
without a human saying so.

The reason automatic binary rollback is safe rather than a coin flip is the
migration gate: **because nothing unattended can change the schema, the automatic
rollback path only ever runs when the schema did not move.** When an operator
*has* approved a migration, that guarantee is gone, and the failure page says so
explicitly and names the migrations that were applied.

### Reporting, and why it repeats slowly

Every outcome goes to Telegram through the bot Alertmanager already uses, so
deploy reports land in the same chat as pages. It is sent directly to the Bot
API rather than injected into Alertmanager as a synthetic alert: a deploy report
is not an alert, and routing one through `group_wait`/`group_interval` would
delay the thing you most want immediately. It also means the report still
arrives when Alertmanager is the thing that broke.

**Conditions that persist are reported once, then go quiet for six hours.** The
agent polls every two minutes, and a blocked deploy stays blocked until a human
acts — without suppression, one pending migration would send thirty identical
messages an hour, which is how a channel gets muted and how the next real page
gets missed. Suppression is keyed on the target SHA, so a *new* bad commit is
never silenced by an older one, and every marker is cleared on a successful
deploy. One-time transitions — deployed, rolled back — are never suppressed.

The failure this does not cover: if the Telegram send itself fails, the deploy
outcome is only in `/opt/qr-dining/deploy.log`. There is no second channel.

### Operating it

```bash
# What is actually running, end to end.
/opt/qr-dining/repo/deploy/vm/deploy.sh --status

# Decide and report without changing anything.
/opt/qr-dining/repo/deploy/vm/deploy.sh --auto --dry-run

# A specific merged commit, or an operator-supplied tag (drills, hotfix images).
/opt/qr-dining/repo/deploy/vm/deploy.sh --sha <sha>
/opt/qr-dining/repo/deploy/vm/deploy.sh --tag <tag>

# Undo the last deploy on purpose. Binary only. Never the schema.
/opt/qr-dining/repo/deploy/vm/deploy.sh --rollback

tail -f /opt/qr-dining/deploy.log
```

### What this deploy path cannot do

Stated here so nobody has to find out during a pilot service:

- **It does not validate the application, only that it starts.** `/readyz` pings
  Postgres and Redis. An image that boots, connects, and then returns 500 on
  every order passes the gate and is reported as a successful deploy. Nothing
  here runs a guest or staff flow.
- **It cannot undo a migration.** By design — but it means an approved migration
  that turns out to be wrong is an incident, not a rollback.
- **There is a window where the two instances run different images**, between
  the two health gates. Deploys must stay backward-compatible for that window.
- **Every merge to `main` restarts both instances, even a docs-only one.** The
  image tag is per-commit, so a commit that changes no Go code still produces a
  new tag and therefore a new container. That is a deliberate trade: skipping
  the restart when the image digest is unchanged would be cheaper, but it would
  leave `docker inspect` reporting a tag that is not the deployed commit, and
  provenance is the thing this whole path exists to establish. The cost is ~10s
  per instance with the other instance serving.
- **WebSocket sessions on the restarting instance are dropped.** HTTP requests
  fail over to the other instance; an open `/ws` connection cannot. Clients
  reconnect, but a deploy during service is visible to guests as a reconnect.
- **Nothing watches the watcher.** If cron stops, the crontab is lost, or the
  checkout is left on another branch, deploys simply stop happening and the only
  symptom is silence. Alerting has a dead man's switch for exactly this; the
  deploy path has none, so "no deploy report since the last merge" is a thing a
  human has to notice. Suppression makes that quieter, not louder.
- **`proxy_nginx` itself is out of scope.** Its image is a floating tag, its
  ports and `nginx.conf` are not in this repo, and nothing here would notice if
  it stopped.
- **One VM, one database, no staging.** This is a rolling restart, not a
  blue/green: there is no environment where the new image serves real traffic
  before production does.

## How a request reaches the app

Nobody could answer this from the repository before 2026-09-18. `proxy_nginx` is
not in any compose file here and cannot be — it is the shared reverse proxy for
the whole VM. That made the first two hops of every production request invisible
to the codebase that depends on them. This section is the hops.

```
  phone / browser
        │
        │  DNS  A record, Cloudflare, DNS-ONLY (grey cloud — no CF proxy)
        │       qr-api-beta.mohith16.com    ─┐
        │       qr-api-beta-1.mohith16.com  ─┼─► 80.225.208.187   (the one VM)
        │       qr-api-beta-2.mohith16.com  ─┘
        ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │ proxy_nginx        nginx:stable-alpine, from /opt/proxy               │
  │                    host :80 and :443, the ONLY published ports        │
  │                                                                       │
  │  :80   /.well-known/acme-challenge/ → certbot webroot                 │
  │        everything else              → 301 https://$host$request_uri   │
  │                                                                       │
  │  :443  TLS TERMINATES HERE. Wildcard *.mohith16.com from              │
  │        /etc/letsencrypt/live/mohith16.com/, renewed DNS-01 via        │
  │        dns-cloudflare by the proxy_certbot sidecar. Shared with       │
  │        grafana, job-queue and sketchiple — this is not our cert.      │
  │                                                                       │
  │        Routing is by Host header (deploy/vm/nginx/qr-dining-beta.conf)│
  │          qr-api-beta-1.mohith16.com → qr_dining_app    (pinned)       │
  │          qr-api-beta-2.mohith16.com → qr_dining_app_2  (pinned)       │
  │          qr-api-beta.mohith16.com   → split_clients on $remote_addr,  │
  │                                       50/50, sticky per client IP     │
  │                                                                       │
  │        Hostnames resolve at REQUEST time via the docker resolver      │
  │        127.0.0.11, deliberately — an upstream{} block would resolve   │
  │        at config-load time and a dead app container would then stop   │
  │        this shared proxy from reloading for FOUR other projects.      │
  └───────────────────────────────┬───────────────────────────────────────┘
                                  │ docker network `proxy` (external, 172.18.0.0/16)
                                  │ plain HTTP :8080 — nothing re-encrypts here
                  ┌───────────────┴───────────────┐
                  ▼                               ▼
        ┌───────────────────┐           ┌───────────────────┐
        │ qr_dining_app     │           │ qr_dining_app_2   │
        │ gin, :8080        │           │ gin, :8080        │
        │ WORKER_REGION=beta│           │ WORKER_REGION=beta│  ← must match
        └─────────┬─────────┘           └─────────┬─────────┘
                  │       docker network `qr-dining_internal` (internal: true)
                  └───────────────┬───────────────┘
                                  ▼
                  qr_dining_postgres        qr_dining_redis
                  Neither is reachable from nginx or the internet.
```

Four consequences worth having in your head before an incident:

- **There is exactly one VM and one nginx.** `proxy_nginx` is a single point of
  failure for qr-dining, grafana, job-queue and sketchiple simultaneously.
- **`X-Forwarded-For` is set by nginx, and the app trusts it** via gin's trusted
  proxies (`TRUSTED_PROXIES=172.18.0.0/16` in `/opt/qr-dining/.env` — the
  `proxy` network CIDR). If that CIDR ever stops matching the network, every
  guest collapses into one rate-limit bucket and the audit log records the proxy
  as the client.
- **nginx replaces, rather than merges, `proxy_set_header` at the more specific
  level.** Every `location` that sets any header of its own repeats all four.
  Adding a fifth header to a location without repeating the others silently
  removes them.
- **`/metrics` is `allow 127.0.0.1; deny all;`** at the proxy. Prometheus does
  not use this path at all — it scrapes both containers directly over the docker
  network.

### Seeing access logs

nginx logs to stdout, so the proxy's log is the container's log. There is no
file to tail and no logrotate to configure.

```bash
# Everything the proxy served, all five projects, live.
docker logs -f proxy_nginx

# Just qr-dining. The Host is the only thing that distinguishes projects here,
# and it is in the default combined format's referer/host position only if you
# put it there — so filter on the request line and the upstream instead.
docker logs --since 30m proxy_nginx | grep -E 'qr-api-beta'

# Which instance served a request: the proxy log does not say. Ask the app.
docker logs --since 30m qr_dining_app   | grep <request-id>
docker logs --since 30m qr_dining_app_2 | grep <request-id>
```

### The probe endpoints do not appear in the proxy log at all

`/health` and `/readyz` carry `access_log off`
(`deploy/vm/nginx/qr-dining-beta.conf`). This is deliberate — blackbox, the
external uptime monitor and `deploy/vm/deploy.sh` hit them every few seconds and
would otherwise bury real traffic — and **it cost an evening of diagnosis**,
because it makes two very different situations look identical:

| what happened | proxy log shows |
| --- | --- |
| the probe never arrived | nothing |
| the probe arrived and returned 200 | nothing |
| the probe arrived and returned 503 | nothing |

Do not debug a probe from `docker logs proxy_nginx`. Nothing will be there and
you will conclude the request is not arriving. Go to the app:

```bash
docker logs --since 10m qr_dining_app | grep -iE 'readyz|health'
```

or take nginx out of the path entirely and ask the container directly, which is
what the deploy health gate does:

```bash
IP=$(docker inspect qr_dining_app \
      --format '{{(index .NetworkSettings.Networks "qr-dining_qr-dining_internal").IPAddress}}')
curl -s "http://$IP:8080/readyz"
```

If you need a probe temporarily logged, comment out the `access_log off` line in
`deploy/vm/nginx/qr-dining-beta.conf`, `nginx -t`, reload — and remember that
that file is now version-controlled and bind-mounted, so the edit is a commit,
not a local hack (see below).

## Backup and restore

The nightly job performs dump → checksum → upload → append-only manifest →
retention and reports last status and last success through node_exporter textfile
metrics (`backend/scripts/nightly-backup.sh:27-79,94-150`). Provider credentials
and schedule installation are described in
[deploy/backup/README.md](../deploy/backup/README.md). The Prometheus stack mounts
the backup textfile volume into node_exporter
(`deploy/observability/docker-compose.observability.yml:49-61`).

`BackupFailed` pages on a non-zero last-run status; `BackupTooOld` pages when the
last-success series is absent or older than 36 hours
(`deploy/observability/prometheus-alerts.yml:264-285`). Because the script installs
an `EXIT` trap before configuration validation, missing `DATABASE_URL`, interrupts,
and command failures report failure while preserving the prior success timestamp
(`backend/scripts/nightly-backup.sh:27-85`).

Selection, checksum verification, destructive restore, migration-22 recovery,
measured Track F timings, and the forward-fix-only constraint are all in
[RECOVERY.md](RECOVERY.md). Do not duplicate an abbreviated restore procedure in
an incident note.

## Metrics and alerts

The optional observability compose adds pinned Prometheus, Alertmanager,
blackbox-exporter, and node_exporter images. It must be merged with the app compose
and does not work standalone (`deploy/observability/docker-compose.observability.yml:1-16,29-65`).
Prometheus scrapes app metrics, readiness through blackbox, and node_exporter,
loads the alert file, and sends firing alerts to Alertmanager
(`deploy/observability/prometheus.yml:1-50`).

The application registers HTTP, database-pool, WebSocket, Redis, cache,
idempotency, worker, audit, authz, presence, payment escalation, and billing
reconciliation metrics (`backend/internal/observability/metrics.go:106-360`). The
billing reconciliation worker populates its discrepancy gauges from two exact
money comparisons without correcting financial state
(`backend/internal/worker/worker.go:132-177`,
`backend/sql/queries/payments.sql:123-258`).

Known monitoring limits:

- `db_errors_total` is declared and registered
  (`backend/internal/observability/metrics.go:169-174,338-360`), but this rebuild
  found no increment call in current backend source. Treat alerts or runbooks
  depending on that counter as **unverified/non-functional** until instrumentation
  is added and exercised.
- Cross-tenant denial alerts observe attempts that policy blocks; they cannot
  prove that no wrongly allowed cross-tenant read occurred. The alert rule records
  that blind spot (`deploy/observability/prometheus-alerts.yml:315-331`).
- Backup silence is covered by the age alerts only if node_exporter and the
  textfile volume are wired; the stack contains that mount, but this rebuild did
  not execute a production scrape test
  (`deploy/observability/docker-compose.observability.yml:49-61`,
  `deploy/observability/prometheus-alerts.yml:264-285`).

OpenTelemetry is off by default, including database and Redis instrumentation
(`backend/internal/config/config.go:214-225`). The production environment example
also leaves it off (`deploy/vm/.env.production.example:91-95`). The SigNoz compose
and rollout material is therefore an optional, unverified path—not evidence that
production traces exist. The app and collector compose files do not share a
network in their checked-in forms, so a separate, unverified overlay is required
before the collector hostname can resolve from the app container
(`deploy/vm/docker-compose.yml:33-59`,
`deploy/signoz/docker-compose.signoz.yml:11-15,129-140`).

## Alert delivery

Alerting is only real once a firing rule reaches a phone. This section is the
deploy-time half of `deploy/observability/alertmanager.yml`: the config in git
carries the routing, the grouping and the inhibition, but **none of the secret
material**, because that file is committed to a public repository with secret
scanning and push protection enabled.

### What goes where

| Thing | Secret? | Lives in |
|---|---|---|
| Telegram bot token | **Yes** — anyone holding it can post as the bot | `/opt/qr-dining/secrets/alertmanager/telegram_token` |
| Telegram chat id | No — addresses a chat, grants nothing | committed in `alertmanager.yml` |
| Dead-man's-switch ping URL | **Yes** — it is a capability: holding it lets anyone keep the watchdog quiet forever | `/opt/qr-dining/secrets/alertmanager/healthchecks_url` |

Alertmanager reads both at notify time via `bot_token_file` and `url_file`, so
neither is ever in the image, the config, `docker inspect`, or the process
environment. `deploy/observability/secrets/` is in `.gitignore`.

### Creating the secrets on the VM

Alertmanager runs as uid 65534 (`nobody`) and the VM has no passwordless sudo,
so the files must be world-readable inside a traversable directory. They are
opaque strings on a single-operator host; the protection that matters is that
they are not in git.

```bash
install -d -m 0755 /opt/qr-dining/secrets/alertmanager

# Telegram bot token from BotFather. printf, not echo: a trailing newline is
# tolerated by Alertmanager but there is no reason to add one.
printf '%s' '123456789:AA...' > /opt/qr-dining/secrets/alertmanager/telegram_token

# Dead man's switch ping URL (healthchecks.io, or an UptimeRobot heartbeat URL).
printf '%s' 'https://hc-ping.com/<uuid>' > /opt/qr-dining/secrets/alertmanager/healthchecks_url

chmod 0444 /opt/qr-dining/secrets/alertmanager/*
```

Never `cat` these into a terminal that is being recorded, and never paste them
into an issue, a PR, or a commit. If either leaks: revoke the bot token with
BotFather `/revoke`, or delete and re-create the healthcheck to get a new URL.

### Wiring the mount

Since 2026-09-16 the observability containers bind-mount their config **directly
from the git checkout** at `/opt/qr-dining/repo/deploy/observability/`. There is
no deployed copy to keep in step, so applying a config change is a pull and a
reload:

```bash
cd /opt/qr-dining/repo
git pull --ff-only                       # this IS the deploy

# Validate BEFORE reloading. A bad Alertmanager config leaves the old one
# running, but a bad rules file makes Prometheus refuse to reload — and you find
# out later, from an alert that never arrives.
docker exec qr-dining-alertmanager-1 amtool check-config /etc/alertmanager/alertmanager.yml
docker run --rm -v /opt/qr-dining/repo/deploy/observability:/o:ro \
  --entrypoint promtool prom/prometheus:v2.54.1 check rules /o/prometheus-alerts.yml

# Reload in place. SIGHUP is enough for config-only edits.
docker exec qr-dining-alertmanager-1 kill -HUP 1
docker exec qr-dining-prometheus-1   kill -HUP 1

# Confirm the reload actually took, rather than assuming it did.
curl -s localhost:9090/api/v1/rules \
  | python3 -c 'import json,sys; print(sum(len(g["rules"]) for g in json.load(sys.stdin)["data"]["groups"]), "rules loaded")'
```

A **restart** rather than a SIGHUP is needed only when a secret *file* changed —
Alertmanager reads `bot_token_file` / `url_file` at notify time, but a changed
mount needs the container recreated:

```bash
cd /opt/qr-dining
docker compose -f docker-compose.yml \
               -f repo/deploy/observability/docker-compose.observability.yml \
               -f docker-compose.observability-vm.yml up -d alertmanager
```

### The new hazard: the checkout is now production state

**Mounting the checkout removed one failure mode and created a smaller one. Know
which one you now have.**

Since 2026-09-18 this covers **the shared nginx proxy's qr-dining vhost as
well** (`deploy/vm/nginx/`, mounted at `/etc/nginx/qr-dining.d` — see
[deploy/vm/proxy-nginx/README.md](../deploy/vm/proxy-nginx/README.md)). The
stakes went up with it: `proxy_nginx` also fronts grafana, job-queue and
sketchiple, so a broken config in that directory is their outage too. This is why
`deploy/vm/deploy.sh` runs `nginx -t` after every fast-forward and reverts the
checkout rather than reloading something that fails it, and why you should never
reload that proxy by hand without `nginx -t` first.

And one correction to the sentence below, which is optimistic in a way that
matters: a `git pull` does **not** reliably change what a container reading a
**single-file** bind mount will load, even on reload, because the mount pins an
inode and git replaces files. See
[Why the nginx mount is a directory](#why-the-nginx-mount-is-a-directory-and-what-that-says-about-the-others).
The observability mounts are single-file. `deploy.sh` recreates those containers
rather than reloading them; a human doing this by hand must too.

The running observability config is whatever the working tree at
`/opt/qr-dining/repo` currently says. That means an ordinary, entirely innocent
git command in that directory silently changes what Prometheus and Alertmanager
will load on their next reload — and, because a reload can be triggered by a
container restart you did not perform (a VM reboot, `restart: unless-stopped`
after an OOM), it can take effect at a moment nobody chose:

- `git checkout <branch>` or `git switch` — swaps every rule file at once.
- `git rebase`, `git bisect`, `git stash` — same, transiently, and `bisect`
  leaves you on a detached HEAD from months ago.
- A dirty working tree from editing a file in place to "just test something".

None of these produce a warning, and `docker ps` stays green throughout.

**The rules:**

1. **`/opt/qr-dining/repo` is production state, not a scratch clone.** Do not
   experiment in it. Clone somewhere else.
2. **It stays on `main`.** Anything else is a deploy of unreviewed config.
3. **Check it before service**, as part of the daily list — this is one line and
   it is the whole guard:

   ```bash
   cd /opt/qr-dining/repo && git status -sb | head -1 && git log -1 --format='%h %s'
   ```

   Expect `## main...origin/main` with no divergence. A branch name that is not
   `main`, a `+N/-N` divergence, or a detached HEAD all mean the running alert
   config is not the reviewed one.

This trade is deliberate. The old failure mode was silent, undetectable and had
already happened — three days with five abort-criterion alerts absent and every
dashboard green. The new one is loud to anyone who looks, reversible with one
`git checkout main`, and caught by a one-line check. But it is real, and it is
new, so it is written down here rather than left to be discovered.

### Telegram

One bot, one chat, one operator. Create the bot with
[@BotFather](https://t.me/BotFather), then get the numeric chat id by sending
the bot a message and reading
`https://api.telegram.org/bot<TOKEN>/getUpdates` — `result[].message.chat.id`.
Group chat ids are negative; supergroup ids start `-100`. A `@channelname`
string will not work, `chat_id` must be numeric.

Three Telegram receivers exist and they differ only in how loud they are:

| Receiver | Marker | Phone behaviour | Cadence |
|---|---|---|---|
| `telegram-page` | 🔴 **PAGE** | notifies | 30s batch, repeats hourly while firing |
| `telegram-ticket` | 🟡 ticket | **silent** (`disable_notifications`) | 2m batch, repeats every 12h |
| `telegram-unrouted` | ⁉️ UNROUTED | notifies | should never fire — see below |

`telegram-unrouted` is the root route's catch-all. Every rule today carries
`severity: page` or `severity: ticket`; if a message arrives marked UNROUTED
then a rule was added without a severity label and is being delivered by
accident. Treat it as a config bug, not as an incident.

All three set `send_resolved: true`. A channel where alerts fire and never
visibly clear is a channel that gets muted within a week.

### The dead man's switch

Every other rule detects a problem by firing. That cannot cover the case where
Prometheus stops evaluating, Alertmanager stops notifying, or the VM loses
egress — because the result is silence, and silence is also what a healthy
system looks like.

`DeadMansSwitch` (`expr: vector(1)`) therefore fires permanently and is routed —
by alertname, on the first route, so it can never fall through to Telegram — to
a webhook receiver that POSTs to an external healthcheck every **2 minutes**.
The operator never sees this alert. They hear about it from the watchdog, once,
when the pings stop.

Set up the external check to match the 2-minute ping:

**healthchecks.io** — <https://healthchecks.io>, free tier. Create a check,
set **Period = 6 minutes** and **Grace = 4 minutes**, and copy its ping URL
(`https://hc-ping.com/<uuid>`) into the secret file above.

**UptimeRobot** — <https://dashboard.uptimerobot.com>, "+ New monitor" →
**Heartbeat**. Set the heartbeat interval to **6 minutes**; copy the generated
heartbeat URL into the same secret file. Nothing in the Alertmanager config is
provider-specific: the receiver POSTs to whatever URL the file contains.

**The real detection window is about 12 minutes, not 10, and the reason is not
obvious.** Measured end to end on 2026-09-16 by stopping Prometheus and polling
the healthchecks.io API:

| Stage | Time | Evidence / why |
|---|---|---|
| Ping cadence | **2m30s** | `repeat_interval: 2m` on a `group_interval: 30s` tick. |
| Pings continue after Prometheus dies | **132s** (bounded by 4m) | Prometheus stamps `EndsAt = now + 4m` on every alert it delivers, so Alertmanager still holds a live, unexpired `DeadMansSwitch` and keeps pinging on schedule after Prometheus is already gone. **This tail is not removable from the Alertmanager side** — it is how Prometheus says "assume this is still true unless I tell you otherwise". Prometheus stopped `10:53:52Z`, final ping `10:56:04Z`. |
| Check goes late | **+6m** (`timeout: 360`) | `10:56:04Z` → `11:02:04Z`. Two missed pings. Note the API reports this state as `status: "grace"`, not `"late"`. |
| Check goes down, operator is emailed | **+4m** (`grace: 240`) | Flip recorded at `11:06:04Z`, exactly `last_ping + 600s`. |
| **Total: Prometheus dies → operator is told** | **12m 12s** | `10:53:52Z` → `11:06:04Z`. |

Worst case is ~14 minutes, when the `EndsAt` tail runs its full four minutes
instead of the 132s measured here. **Budget for 14, not 12, and neither is 10.**

Period 6 / grace 4 tolerates two consecutive missed pings before the check even
goes late, so a transient network blip on a small VM does not cry wolf.
Tightening to period 4 / grace 2 would bring detection to roughly 8-10 minutes —
closer to abort criterion A5's window — at the cost of alarming after three
missed pings instead of four. Either is defensible; what is not defensible is
believing the number is 10 when the tail makes it 12 to 14.

Recovery is fast and needs no intervention: Prometheus restarted `11:07:01Z`,
first ping `11:08:19Z`, check back up on that ping.

> **The watchdog notifies by EMAIL, on purpose. Do not route it through the
> Telegram bot.**
>
> This is the one alert in the system whose delivery path must not share a
> component with the thing it is watching. The switch exists to catch the case
> where Prometheus, Alertmanager, the VM, or its network egress has died — and
> every Telegram notification in this deployment depends on all four. Pointing
> healthchecks.io at the bot would mean that the moment the pipeline breaks, the
> alarm about the pipeline breaking travels through the broken pipeline. It
> would appear to work in every test where you break something else, and fail
> silently in the exact case it was built for.
>
> Email is not chosen because it is good — it is slower and easier to miss than
> a push notification. It is chosen because it is **independent**. Any second
> channel works (SMS, a push app, a different chat service) as long as nothing
> on this VM is on its path. Configured 2026-09-16 as email-only for this
> reason; if you later add a channel, add it alongside, and check the same
> question first.

`send_resolved` is **false** on this receiver and must stay false. If Prometheus
dies, Alertmanager marks `DeadMansSwitch` resolved after `resolve_timeout` and
would send one final notification — which on this receiver is a ping, arriving
at exactly the moment the switch is supposed to trip.

**Consequence to know:** `ALERTS{alertstate="firing"}` is never empty on this
deployment. The daily pre-service check reads "no firing alerts *other than*
`DeadMansSwitch`".

### External uptime monitors

The dead man's switch above is a **push**: this VM tells a third party it is
alive, and silence is the alarm. It cannot tell you the service is unreachable
from the outside — if the VM is up and the ingress is broken, the pings keep
arriving. An external HTTP monitor is the complement, and it is the only signal
that sees what a guest's phone sees.

Point it at **`/readyz`**, not `/health`. Liveness only proves the process
answers; readiness pings PostgreSQL and Redis and fails closed with 503 when
either is gone (`backend/internal/handlers/health.go:26-55`). A monitor on
`/health` stays green through a total database outage.

**Either `GET` or `HEAD` is correct.** Both are registered on all three
infrastructure endpoints (`backend/internal/server/server.go:187-197`). This is
worth stating because it was not always true, and the failure was expensive:

> Until this was fixed, `/health`, `/readyz` and `/metrics` were registered with
> `r.GET` only. Gin does not derive a `HEAD` route from a `GET` registration, so
> `HEAD` returned **404** on every probe endpoint. UptimeRobot defaults to
> `HEAD` — it is that tool's recommended method, since it skips the response
> body — and a monitor configured that way reported a **6-hour outage against a
> healthy service**. `Router/OpenAPI Parity` now holds both methods in place:
> dropping either the route or its `openapi.yaml` entry fails the build.

**There is no `/healthz`.** It has never existed in this repo and 404s on every
method, `GET` included. If a monitor is pointed there, repoint it — the fix
above does not help, because the path is wrong rather than the method.

### Keeping the VM in step with the repo

**This bit the alerting rules, and "stale checkout" understates what was wrong.**

On 2026-09-16 the deployed Prometheus was evaluating **27** of the repo's 32
rules. The entire `qr-dining-pilot-abort` group — the alerts for criteria A1,
A2, A5 and A7 — had been on `main` since 2026-09-13 and did not exist on the box
at all. Nothing failed, nothing logged, every dashboard was green: **a rule that
was never loaded is indistinguishable from a rule that is not firing.** Three
days, five abort-criterion alerts, silent.

There were three drift surfaces, and all three were bad:

1. **`/opt/qr-dining/repo` was checked out on `feature/signoz-observability`** —
   not on `main`, and not even on a branch that still existed: `git ls-remote
   --heads origin feature/signoz-observability` returned nothing. The box was
   tracking a deleted ref. This is the one that makes "stale" the wrong word;
   the checkout was not behind `main`, it was pointed somewhere else. (Verified
   recoverable before moving it: `git rev-list --count origin/main..HEAD`
   returned `0`, so the branch held nothing that was not already on `main`.)
2. That checkout was **254 commits** behind `origin/main`.
3. `/opt/qr-dining/observability/` was a **hand-copy** of `deploy/observability/`
   which had then drifted from even that stale checkout. A copied file carries no
   provenance — there is no command that answers "which commit is this?".

**Surface 3 is now gone.** The containers bind-mount the checkout directly
(`deploy/vm/docker-compose.observability-vm.yml`), so there is no copy left to
drift. `git pull` is the deploy and `git log -1` answers what is running. That
change created a new, smaller hazard in exchange — see
[The new hazard](#the-new-hazard-the-checkout-is-now-production-state) above,
and do not skip it.

**Surfaces 1 and 2 remain**, because a checkout can still be on the wrong branch
or behind. They are covered by the daily one-liner in "The new hazard" and by
this check, which is worth running after any deploy or observability change:

```bash
cd /opt/qr-dining/repo
git fetch -q origin && git status -sb | head -1     # expect: ## main...origin/main

# The question that actually matters — does the running process agree with git?
echo -n "repo says:   "; grep -c '      - alert:' deploy/observability/prometheus-alerts.yml
curl -s localhost:9090/api/v1/rules \
  | python3 -c 'import json,sys; print("loaded:     ", sum(len(g["rules"]) for g in json.load(sys.stdin)["data"]["groups"]))'
```

Those two numbers disagreeing is the exact signature of the 2026-09-16 incident.
**Read the number. Do not infer health from the absence of an alert** — the
absence of an alert is what this failure looks like.

### Verifying delivery end to end

Config that looks right proves nothing. The full path is Prometheus evaluating →
Alertmanager routing → Telegram delivering, and only the last hop is exercised
by a hand-injected alert.

```bash
# Last hop only.
docker exec qr-dining-alertmanager-1 amtool --alertmanager.url=http://localhost:9093 \
  alert add TestPage severity=page '--annotation=summary=delivery drill'

# Whole path. Stopping the SECOND app instance fires AppTargetDown without
# taking the service down.
docker compose ... stop app2        # 🔴 PAGE AppTargetDown within ~90s
docker compose ... start app2       # 🟢 RESOLVED within ~2m

# Dead man's switch, the half that matters: stopping Prometheus must make the
# external check go late. Watching it succeed proves nothing.
docker compose ... stop prometheus   # "grace" at ~6m after the last ping, down at ~10m
docker compose ... start prometheus  # back up on the first ping, no intervention

# Confirm from healthchecks.io rather than from inference. A read-only API key
# is enough, and /flips/ answers the question retroactively — you do not have to
# be watching at the moment it trips.
curl -s -H "X-Api-Key: $HC_READONLY_KEY" https://healthchecks.io/api/v1/checks/
curl -s -H "X-Api-Key: $HC_READONLY_KEY" \
  https://healthchecks.io/api/v1/checks/<unique_key>/flips/
```

Alertmanager logs nothing on a successful notification at its default log
level, so "did it actually send?" is answered either by the counters —

```bash
docker exec qr-dining-alertmanager-1 wget -qO- http://localhost:9093/metrics \
  | grep -E '^alertmanager_notifications_(total|failed_total)\{integration="(telegram|webhook)"' \
  | grep -v ' 0$'
```

— or, when you want the actual receipt, by restarting it with `--log.level=debug`
for the duration of a drill. That logs one
`msg="Telegram message successfully published" message_id=… chat_id=…` per
delivered message and one `receiver=deadmanswitch … msg="Notify success"` per
dead-man's-switch ping. Put it back afterwards so the deployed command matches
the committed compose file.

To silence alerts for a planned deploy, see
[RUNBOOKS.md, Silencing alerts for planned work](RUNBOOKS.md#silencing-alerts-for-planned-work).


## Operational workers

The server starts seven worker loops at boot
(`backend/cmd/server/main.go:95-105`). Their distributed lock uses Redis `SETNX`,
and each invocation is panic-isolated
(`backend/internal/worker/worker.go:645-670`). Inspect
`background_worker_runs_total` and `background_worker_panics_total` before assuming
a lifecycle loop ran (`backend/internal/observability/metrics.go:53-57,215-229`).

Payment escalation is alert-only and points operators to staff settle/cancel or
session force-close (`backend/internal/worker/worker.go:288-310`). Billing
reconciliation is observation-only and writes only metrics/audit records
(`backend/internal/worker/worker.go:319-353`). Presence expiry currently emits no
participant-left event; it is a no-op hook because readers apply age filtering
directly (`backend/internal/worker/worker.go:233-251`,
`backend/internal/redis/presence.go:151-158`).

## Incident routing

Use [RUNBOOKS.md](RUNBOOKS.md) for symptom-specific containment and
[PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) for the stop clock. Use
[RECOVERY.md](RECOVERY.md) only for data loss/corruption or a confirmed dirty
migration; restoring a database for an application regression expands the blast
radius without undoing the binary defect.
