# SigNoz Deployment — Self-Hosted (Community Edition)

Date: 2026-08-03 · Branch: `feature/signoz-observability`
Companions: `signoz-adoption-decision.md` (why + go/no-go gate) · `docs/signoz-backend-instrumentation.md` (app changes) · `docs/signoz-rollout-runbook.md` (phases)

Skeleton files in this directory:

- `docker-compose.signoz.yml` — the stack (validates with `docker compose config -q`; image tags must be re-pinned at implementation time)
- `otel-collector-config.yaml` — collector pipelines (start from the official config of the pinned release, apply the `QR-DINING` deltas marked inside)

The stack is pinned to **SigNoz v0.129.0**, the newest release that still ships
the canonical `deploy/docker/` Compose deployment required by this rollout.
SigNoz v0.130.0 and later removed those files in favor of Foundry. The paired
upstream collector tag is `v0.144.5`; all pinned long-running images publish
`linux/arm64` manifests.

> **⚠️ SOAK SAFETY — READ FIRST.** The local dev machine hosts the long-running soak stack under compose project **`qr-dining`** on ports **5432/6379/8080** (SEV-0 RC soak pending). Never bring SigNoz up with `-p qr-dining`, never bind those host ports, never run `docker compose down -v` against anything but the `qr-dining-signoz*` projects. Same standing rules as `OPERATIONS.md` §8.

## Topology

Current SigNoz releases ship four long-running services plus one-shot schema migrators (canonical source: `https://github.com/SigNoz/signoz` → `deploy/docker/`; the layout has changed across versions — **mirror the pinned release, not memory**):

| Service | Role | arm64 |
|---|---|---|
| `clickhouse` | trace/log/metric storage | ✅ official multi-arch images |
| `zookeeper` (or `clickhouse-keeper`, per release) | ClickHouse coordination | ✅ (if the bitnami tag fails to pull post-2025 registry reorg, use the image the pinned SigNoz release references) |
| `signoz` | merged query-service + web UI (older releases split `query-service` + `frontend`) | ✅ |
| `signoz-otel-collector` | OTLP ingest + log collection → ClickHouse | ✅ |
| `schema-migrator-{sync,async}` | one-shot ClickHouse DDL | ✅ |

The whole VM is Ampere arm64 and the repo has zero amd64 fallback (CI builds `linux/arm64` only) — verify every pinned tag is multi-arch before deploying.

## Port map (binding decisions)

Existing host-port ladder (do not collide): `5432/6379/8080` soak+prod · `15432/16379` pilot-validation · `25432/26379` manual-testing DBs · `8090/8095` manual-testing backends · `3000/3001` manual-testing frontends · `18080` staging nginx · `9090/9093/9100/9115` Prometheus stack.

| Endpoint | Host binding | Rule |
|---|---|---|
| SigNoz UI | `127.0.0.1:3301` | The container serves on internal **8080 — never publish it as 8080** (conflicts with the app). Localhost-only, SSH tunnel to reach (same pattern as Prometheus `:9090`). |
| OTLP gRPC | `127.0.0.1:4317` | For a natively-run local backend. On the VM, containers use the docker network instead (below). |
| OTLP HTTP | `127.0.0.1:4318` | Same. |
| ClickHouse 9000/8123/9440, zookeeper 2181 | **no host ports** | Internal network only. |

Access from a workstation:

```bash
ssh -L 3301:127.0.0.1:3301 <vm>   # then open http://localhost:3301
```

Firewall stays 80/443-public only (`production-environment-checklist.md`); SigNoz gets **no** nginx vhost and never joins the `proxy` network.

## VM install (Phase 3 — only after the Phase 0 preflight passes)

Install path: `/opt/qr-dining/signoz/` (per-project convention; promote to a shared `/opt/signoz` only if other projects adopt it later).

```bash
sudo mkdir -p /opt/qr-dining/signoz
# copy docker-compose.signoz.yml + otel-collector-config.yaml there
sudo docker network create signoz          # once — shared external network
cd /opt/qr-dining/signoz && docker compose up -d
docker compose ps                          # all healthy; migrators Exited (0)
```

### Wiring the app to the collector

`deploy/vm/docker-compose.yml` is not modified. Create an overlay at `/opt/qr-dining/docker-compose.signoz-app.yml`:

```yaml
# Overlay: joins the app to the external "signoz" network so it can reach
# the collector at signoz-otel-collector:4317. Apply with:
#   docker compose -f docker-compose.yml -f docker-compose.signoz-app.yml up -d
services:
  app:
    networks: [signoz]
networks:
  signoz:
    external: true
```

Then in `/opt/qr-dining/.env`: `OTEL_ENABLED=true`, `OTEL_EXPORTER_OTLP_ENDPOINT=signoz-otel-collector:4317` (plain gRPC, no TLS — traffic never leaves the docker network).

### Log rotation side-fix (do this while touching the VM)

No compose file in this repo sets a logging driver, so container logs grow unbounded. Add to every service in `deploy/vm/docker-compose.yml` (or once via `x-logging` anchor) in the implementation session:

```yaml
logging:
  driver: json-file
  options: { max-size: "10m", max-file: "3" }
```

Keep `max-file: 3` generous enough that the collector's filelog receiver (which tails these files) doesn't lose data across rotations.

## Local eval (Phase 1)

```bash
docker compose -p qr-dining-signoz-eval -f deploy/signoz/docker-compose.signoz.yml up -d
```

- Project name **`qr-dining-signoz-eval`** — never `qr-dining` (soak) and distinct from the VM's `qr-dining-signoz`.
- Host ports 3301/4317/4318 are free on the ladder; nothing else is published.
- The external `signoz` network must exist locally too: `docker network create signoz` (or strip that network from a local copy — the natively-run backend uses `127.0.0.1:4317` and doesn't need it).
- Run the backend natively (`make -C backend run` pattern) with `OTEL_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317` against the **manual-testing** DBs (25432/26379) — never the soak DBs.
- On the dev machine the filelog receiver finds no `qr_dining_app` container — that's expected; local eval validates traces, not the log pipeline. Comment the filelog receiver out of the logs pipeline if collector logs get noisy.
- Teardown: `docker compose -p qr-dining-signoz-eval down -v` (the `-v` is safe **only** with this `-p`).

## Logs pipeline (zero app-code changes)

Chosen approach: the collector's **filelog receiver tails the Docker `json-file` logs** of `qr_dining_app` and parses the zerolog NDJSON body — no OTLP log exporter in the app, nothing to change in `backend/internal/observability/logger.go`.

The app's log shape (see `backend/internal/middleware/logger.go`): one `"request"` line per request with `level`, `time`, `service`, `request_id`, `method`, `route`, `action`, `client_ip`, `status`, `latency_ms`, `bytes`, plus actor enrichment (`actor_type`, `actor_id`, `organization_id`, `branch_id`) — and, after instrumentation, `trace_id`/`span_id`. The collector config parses these into log attributes so SigNoz can filter by them and pivot trace↔log on `trace_id`.

Requirements: collector mounts `/var/lib/docker/containers:ro` (VM only) and the filelog operators filter to the `qr_dining_app` container — see the marked section in `otel-collector-config.yaml`.

## Retention & disk

Set in the SigNoz UI (Settings → General → Retention Period) after first boot: **traces 7d, logs 7d** (metrics ingestion stays off/minimal — Prometheus is authoritative). Revisit only with disk evidence. ClickHouse data lives in the named volume `signoz_clickhouse_data`; treat the whole stack as disposable (no backups) through Phase 3.

## mem_limits

Every service carries an explicit `mem_limit` (shared-VM etiquette, same reason `deploy/vm/docker-compose.yml` does): clickhouse 1536m, signoz 512m, otel-collector 256m, zookeeper 256m ≈ **2.5 GB total**. If ClickHouse OOMs at 1536m under real ingest, raise deliberately and re-run the preflight math — don't remove the limit.
