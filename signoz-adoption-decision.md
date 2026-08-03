# SigNoz Adoption — Decision Record

Date: 2026-08-03 · Branch: `feature/signoz-observability` (off RC `feature/certification-fixes-ui-redesign` @ f0c7008)
Companions: `deploy/signoz/README.md` (deployment) · `docs/signoz-backend-instrumentation.md` (code changes) · `docs/signoz-rollout-runbook.md` (phased rollout) · `alerting-setup.md` (existing Prometheus stack, stays authoritative)

## Verdict

**Qualified YES — adopt self-hosted SigNoz (Community Edition, no subscription), hard-gated on the Phase 0 VM sizing preflight below.**

## Why (the gaps SigNoz closes)

The project has a mature Prometheus stack — ~45 hand-rolled metrics (`backend/internal/observability/metrics.go`), 27 alert rules (`deploy/observability/prometheus-alerts.yml`), Alertmanager/blackbox/node-exporter — but three real observability gaps remain:

1. **Zero distributed tracing.** No OpenTelemetry anywhere in the codebase. For a session-centric realtime flow (QR session → WebSocket ordering → payment finalization) heading into pilot, "which layer made this request slow/fail" is unanswerable today.
2. **Zero log search.** zerolog emits structured NDJSON to stdout, but nothing collects it. Debugging means `docker logs | grep`.
3. **Unbounded container logs.** No compose file declares a `logging:` block, so Docker's default `json-file` driver grows without rotation.

Additionally, three metrics are declared but **never incremented** (`DBQueryDuration`, `DBErrorsTotal`, `RedisOpsTotal` in `metrics.go` have zero call sites) — there is currently no per-query DB timing and no Redis op signal at all. OTel auto-instrumentation (`otelpgx`, `redisotel`) fills exactly that hole with net-new signal, not duplicates.

## Cost

- **RAM: ~2.5 GB of new mem_limits** on the shared OCI Ampere arm64 VM that also hosts `invoice`, `job-queue-system`, `sketchiple`, and the central `/opt/proxy` nginx. ClickHouse dominates. The VM's total RAM is **not documented anywhere in this repo** — hence the hard gate.
- **Disk:** ClickHouse storage for traces + logs. Bounded by starting retention of 7d traces / 7d logs (R1 soak observed ~172 GB free, so disk is the lesser concern).
- **Ops burden:** one more stateful system (ClickHouse) to upgrade, back up decisions for, and watch. Mitigated by treating SigNoz as *disposable* in Phases 0–3: it holds no data of record, so "wipe and redeploy" is an acceptable recovery strategy — unlike Postgres.

### RAM budget table

| Service | mem_limit | Notes |
|---|---|---|
| **Proposed: SigNoz stack** | | `deploy/signoz/docker-compose.signoz.yml` |
| clickhouse | 1536m | the heavy part |
| signoz (query svc + UI) | 512m | merged container in current releases |
| signoz-otel-collector | 256m | |
| zookeeper / clickhouse-keeper | 256m | per pinned SigNoz release |
| **Subtotal (new)** | **≈ 2.5 GB** | |
| **Existing qr-dining** | | `deploy/vm/docker-compose.yml` |
| app | 1g | |
| postgres | 1g | |
| redis | 384m | maxmemory 256mb |
| prometheus / alertmanager / blackbox / node-exporter | *(no limits)* | `deploy/observability/` — observed small |
| **Other projects** | *(unknown)* | invoice, job-queue-system, sketchiple, /opt/proxy nginx |

## Alternatives considered

**Grafana + Loki + Tempo** covers the same gaps but means operating three new backends *plus* Grafana — more moving parts for the same outcome, and correlating trace↔log across them takes configuration that SigNoz ships out of the box. **Adding only Grafana** to the existing Prometheus (a dashboard JSON is already committed at `deploy/observability/grafana-dashboard.json`) gives nicer dashboards but zero traces and zero log search — it doesn't address the actual gap. SigNoz is the only single-stack option delivering traces + logs + UI in one deployment, with published arm64 images. If the Phase 0 preflight fails and the VM can't be upsized, the fallback below applies rather than switching stacks.

## Phase 0 preflight — the hard gate

Run on the VM before any deploy decision:

```bash
free -h                     # total / available RAM
docker stats --no-stream    # live usage of every container on the box
docker ps --format '{{.Names}}' | wc -l
df -h /var/lib/docker
```

**Go / no-go rule:** available RAM (after summing the *limits* of everything already running, not just current usage — limits are what containers may legitimately grow into) must leave **≥ 3 GB genuinely free** for the SigNoz stack (2.5 GB limits + headroom). Disk: ≥ 20 GB free for ClickHouse at 7d retention.

**If no-go:**
- **Fallback A (default):** run SigNoz as a local-eval-only stack (`deploy/signoz/README.md` §Local eval) for development debugging; production keeps the Prometheus stack unchanged. The backend instrumentation still merges (gated off by `OTEL_ENABLED=false`), so flipping production on later is a config change, not a code change.
- **Fallback B:** upsize the VM (Ampere A1 shapes resize easily) or move SigNoz to a second small VM and point `OTEL_EXPORTER_OTLP_ENDPOINT` at it over a private network.

## De-risking principles (binding on the implementation session)

- Everything is **additive and env-gated**: `OTEL_ENABLED=false` is the default; with it off, the app behavior is byte-for-byte unchanged (no-op tracer provider).
- **Prometheus stays the source of truth.** The 27 alert rules and 45 metrics are untouched through Phases 0–3; SigNoz alerting is evaluated only in Phase 4.
- The app must **never hard-depend on the collector**: exporter failures are logged, sampled, and non-fatal; startup does not block on OTLP connectivity.
- The **local soak stack is untouchable** (compose project `qr-dining`, ports 5432/6379/8080, SEV-0 RC soak pending). Local eval uses its own project name and ports — see `deploy/signoz/README.md`.

## Non-goals

- **No rewrite** of the 45 existing Prometheus metrics to OTel metrics.
- **No alert migration** off Prometheus/Alertmanager in Phases 0–3 (Phase 4 evaluates consolidation; may conclude "keep Prometheus forever").
- **Frontend RUM / browser tracing deferred.** Next.js 15 on Cloudflare Workers (OpenNext) constrains which OTel JS SDK is viable; there is no existing RUM to migrate. Revisit after Phase 4.
- **No gorilla/websocket auto-instrumentation** (none exists upstream); manual WS spans are optional Phase 3+ work.
