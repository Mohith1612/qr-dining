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

- Alertmanager's receiver is a placeholder webhook URL, so notification delivery
  is **not configured by the checked-in file**
  (`deploy/observability/alertmanager.yml:17-38`).
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
