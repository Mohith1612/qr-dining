# Phase D — Production Alerting Baseline

Date: 2026-05-25
Artifacts: `deploy/observability/prometheus-alerts.yml` (18 rules, validated with
`promtool check rules`), `deploy/observability/grafana-dashboard.json`.

This is the **minimum production alerting set**. Every rule references a metric the
app actually emits (`internal/observability/metrics.go`) — no aspirational series.
Severities: `page` = wake someone; `ticket` = next business hours.

## 1. Availability

| Alert | Expr (summary) | Sev | Why |
|-------|----------------|-----|-----|
| AppTargetDown | `up{job="qr-dining"} == 0` for 1m | page | scrape/process down |
| HighServerErrorRate | 5xx ratio > 5% for 10m | page | broad failure / dependency outage |

## 2. Datastores

| Alert | Expr (summary) | Sev | Why |
|-------|----------------|-----|-----|
| PostgresPoolExhausted | `idle==0 and acquired>=total` for 3m | page | requests queue on acquire; leak or slow query |
| PostgresQueryErrors | `rate(db_errors_total)>0` for 10m | ticket | sustained query failures |
| RedisPubSubDisconnected | `redis_pubsub_connected==0` for 1m | page | fan-out down (secondary signal — see note) |
| RedisPubSubReconnectStorm | `rate(redis_pubsub_reconnects_total)>0.2` for 10m | ticket | Redis flapping |

**Note (from chaos F-1):** `redis_pubsub_connected` stays `1` during a *transient*
Redis flap because go-redis reconnects transparently. It only drops to 0 on
subscriber teardown. For a true Redis outage, the primary page comes from a probe on
`/readyz` (which pings Redis directly) — wire a blackbox/exporter probe on `/readyz`
in addition to this rule. Do not rely on the gauge alone.

## 3. WebSocket + abuse hardening (new this phase)

| Alert | Expr (summary) | Sev | Why |
|-------|----------------|-----|-----|
| WSAbusiveCloses | `rate(ws_abusive_closes_total)>0` for 5m | ticket | connections force-closed for inbound abuse |
| WSInboundDropSpike | `rate(ws_inbound_dropped_total)>5` for 10m | ticket | per-connection rate limit dropping frames |
| WSClientEvictionSpike | `rate(ws_client_evictions_total)>1` for 10m | ticket | slow consumers / fan-out hotspot |
| WSReconnectStorm | `rate(ws_reconnects_total)>10` for 10m | ticket | edge instability / deploy flapping |

## 4. Integrity

| Alert | Expr (summary) | Sev | Why |
|-------|----------------|-----|-----|
| AuditWriteFailures | `rate(audit_write_failures_total)>0` for 5m | page | audit gaps under AUDIT_LOG_V2 — compliance risk |
| RateLimiterUnavailable | `rate(rate_limiter_unavailable_total)>0` for 5m | page | fail-closed surfaces 503ing (Redis down) |
| WebhookReplaySpike | `rate(idempotency_replays_total{entity="webhook"})>1` for 10m | ticket | provider re-delivery / replay attempt |

## 5. Lifecycle workers

| Alert | Expr (summary) | Sev | Why |
|-------|----------------|-----|-----|
| BackgroundWorkerPanic | `increase(background_worker_panics_total[10m])>0` | page | lifecycle cleanup stalled → stuck sessions/tables |
| StaleSessionCleanerStalled | no `stale_session_cleaner` success in 15m | ticket | abandoned sessions not freed |
| ReactivationPipelineStalled | no `reactivation_pipeline` success in 15m | ticket | lifecycle transitions stalled |

## 6. Rollout gates (must read zero before flipping the matching flag)

| Alert | Metric | Gates flag |
|-------|--------|-----------|
| LegacyAuthzBypassPresent | `legacy_authz_bypass_total` | AUTHZ_CENTRAL_POLICY_ENFORCE |
| LegacyIdentityUsagePresent | `legacy_identity_usage_total` | AUTH_GUEST_CREDENTIALS_REQUIRED, WS_TICKET_AUTH_REQUIRED |

A non-zero value means flipping the strict flag **will break live traffic**. These
double as the "legacy decay" dashboards required by `production-enforcement-rollout.md` §7.

## 7. Observability gaps — RESOLVED in Phase E

The rollout playbook referenced metrics that did not exist at the end of Phase D.
Phase E implemented all of them (emitted at authoritative paths, bounded labels),
wired alerts (§1–6 above) and dashboard panels, and added the `/readyz` probe config.

| Signal | Blocks wave | Status |
|--------|-------------|--------|
| `policy_shadow_mismatch_total{route,reason}` | R3 (authz) | DONE — `handlers/authz.go` shadow branch; alert `PolicyShadowMismatchPresent` |
| `authz_denied_total{reason}` | R3, R4 | DONE — `handlers/authz.go` enforced branch; alert `AuthzDeniedSpike` |
| `tenant_resolution_failures_total{stage}` | R2 (tenancy) | DONE — `middleware/tenant.go`; alert `TenantResolutionFailures` |
| `ws_ticket_consume_failed_total{reason}` | R5 (ws ticket) | DONE — `handlers/ws.go` consume path; alert `WSTicketConsumeFailures` |
| `guest_token_validation_failed_total{reason}` | R6 (guest creds) | DONE — `handlers/guest_auth.go`; alert `GuestTokenValidationFailures` |
| `/readyz` blackbox probe | Redis-down paging | DONE — `prometheus-scrape-readyz.yml` + `ReadyzProbeFailing` (gauge insufficient, F-1) |
| `payment_pending` escalation visibility | R7 (settlement) | DONE — `payment_pending_escalations_total{level}`; alert `PaymentPendingEscalationCritical` |

Each still requires its thresholds tuned during the staging soak before the
matching wave flips, but the instrumentation prerequisite is now met.

## 8. Dashboard

`grafana-dashboard.json` (uid `qr-dining-ops`) has rows for Availability, Rollout
gates (the two legacy-decay panels), WebSocket + abuse, Datastores, and Lifecycle
workers + payments. Import and bind to the Prometheus datasource. It is the
flip-impact + legacy-decay surface for the rollout.
