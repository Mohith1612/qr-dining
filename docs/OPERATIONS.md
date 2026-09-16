# Operations

Last verified against code and deployment configuration: 2026-09-13. Deployment
commands below are configuration-derived; this rebuild found no retained evidence
that the complete production cutover or SigNoz path has been executed. Treat them
as **unverified procedures** until an operator records a successful run.

## Deployment shape

The VM compose file defines one arm64 application image from GHCR, PostgreSQL 17,
and Redis 7. PostgreSQL and Redis use an internal network; only the application
joins the shared proxy network. All three services have memory limits, bounded
JSON logs, persistent volumes, and health checks
(`deploy/vm/docker-compose.yml:21-114`). Nginx proxies HTTP and WebSocket traffic
to `app:8080`; `/metrics` is restricted to loopback and private Docker CIDRs
(`deploy/nginx/qr-dining.conf:26-82`).

The application image is built for `linux/arm64`; CI pushes only on main or tag
events and emits SHA, version-tag, and default-branch `latest` tags
(`.github/workflows/ci.yml:225-275`). Production compose accepts `IMAGE_TAG` and
falls back to `latest` (`deploy/vm/docker-compose.yml:43-51`). Use an immutable
SHA/version tag for a reviewed deployment; the example's `latest` value is not a
release pin (`deploy/vm/.env.production.example:14-17`).

### Unverified deploy procedure

```bash
cd /opt/qr-dining
# Set IMAGE_TAG in .env to the reviewed immutable tag.
docker compose pull app
docker compose up -d
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
(`backend/internal/server/server.go:187-190`). Compose health uses `/readyz`
(`deploy/vm/docker-compose.yml:64-71`). Verify at least the new image tag, healthy
container state, clean schema version, and a representative guest/staff flow
before ending the change window. The repository contains no executed production
deployment report proving a stronger checklist.

### Rollback

Roll back the application by restoring the previous `IMAGE_TAG`, pulling it, and
recreating `app`; the compose image reference is tag-controlled
(`deploy/vm/docker-compose.yml:43-51`). Never roll the schema backward. Migrations
16–24 and 36/38/39 destroy or fail to restore durable meaning; the verified list
and recovery procedure are in [RECOVERY.md](RECOVERY.md).

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
