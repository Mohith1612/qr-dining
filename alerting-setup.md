# Alerting Setup

**Phase:** Final Pilot Hardening — Phase F
**Goal:** turn the existing Prometheus alert *rules* into actual notifications, and
guarantee the four pilot-critical conditions page someone: **app down, database
unavailable, Redis unavailable, backup failure.**

Before this phase the repo had alert rules but **no Alertmanager** — rules would
fire inside Prometheus and route nowhere. This phase adds the routing/notification
layer and the two missing signals (readiness probe + backup).

## Components

| File | Role |
|------|------|
| `deploy/observability/prometheus.yml` | scrape app `/metrics`, `/readyz` blackbox probe, node_exporter; load rules; forward to Alertmanager |
| `deploy/observability/prometheus-alerts.yml` | alert rules (27) incl. new `ReadyzProbeFailing`, `BackupFailed`, `BackupTooOld` |
| `deploy/observability/alertmanager.yml` | routing + receivers (webhook default; Slack/email/PagerDuty examples) |
| `deploy/observability/blackbox.yml` | `/readyz` HTTP probe module |
| `deploy/observability/docker-compose.observability.yml` | runnable Prometheus+Alertmanager+blackbox+node_exporter |
| `backend/scripts/nightly-backup.sh` | emits `qr_dining_backup_*` textfile metrics |

## The four required alerts

| Condition | Alert | Signal |
|-----------|-------|--------|
| **App down** | `AppTargetDown` | `up{job="qr-dining"} == 0` (Prometheus can't scrape the app) |
| **Database unavailable** | `ReadyzProbeFailing` (+ `PostgresPoolExhausted`, `PostgresQueryErrors`) | `/readyz` returns 503 → `probe_success == 0`; the JSON body names `postgres: unhealthy` |
| **Redis unavailable** | `ReadyzProbeFailing` (+ `RedisPubSubDisconnected`) | `/readyz` returns 503 (`redis: unhealthy`); pub/sub gauge as secondary |
| **Backup failure** | `BackupFailed`, `BackupTooOld` | textfile metric `qr_dining_backup_last_run_status != 0`, or no success in >36h |

`/readyz` is the authoritative datastore-outage signal (Phase D F-1: the pub/sub
gauge stays 1 through a transient Redis blip, so it alone is unreliable). All four
are `severity: page`.

## Notification routing

`alertmanager.yml`: `page` → `urgent` receiver (repeat 1h); everything else →
`default` (repeat 4h). The default receivers are **webhooks** — the most portable
choice, and a webhook URL can target Slack, Teams, PagerDuty, Opsgenie, or a relay.
Native Slack / email / PagerDuty receiver blocks are included commented; uncomment
one and drop in the credential. An inhibit rule mutes `ticket` alerts while the same
alertname is already paging.

## Run it

```bash
# Attach to the app's network so app:8080 resolves.
docker compose -f docker-compose.yml \
  -f deploy/observability/docker-compose.observability.yml up -d

# Point node_exporter's textfile dir and the backup job at the same volume so
# backup metrics are scraped (set NODE_EXPORTER_TEXTFILE_DIR in backup.env).
```

Prometheus UI `:9090` (Status → Rules / Alerts), Alertmanager UI `:9093`.

## Verified firing path (real, not simulated)

Alertmanager + a webhook sink were run in containers; a firing `page` alert was
POSTed to Alertmanager's API and delivery to the receiver was observed:

```
POST /api/v2/alerts  [{alertname=ReadyzProbeFailing, severity=page, ...}]
→ webhook sink log:
   DELIVERED to /urgent: receiver=urgent status=firing count=1
     - ReadyzProbeFailing severity=page :: /readyz probe failing — Postgres or Redis unavailable
```

This confirms the route (`severity=page → urgent`) and the receiver fire end to
end. Config validation: `promtool check rules` → 27 rules OK; `amtool check-config`
→ OK.

Backup metric path was exercised in Phase D/E (nightly-backup.sh writes
`qr_dining_backup_status.prom` / `qr_dining_backup_success.prom`; failures go
through the `ERR` trap → status `1`).

## Honest limitations

- The verification used a webhook sink. Before go-live, set a **real** receiver
  (Slack/PagerDuty/email) and send one test alert through it (`amtool alert add`
  or the API) to confirm the credential and channel.
- Demo timing used `group_wait: 1s`; the committed config keeps the production
  `30s` group_wait.
- This is single-Prometheus / single-Alertmanager (no HA) — appropriate for a
  pilot; revisit clustering for production scale.
