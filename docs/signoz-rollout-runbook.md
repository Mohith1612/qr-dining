# SigNoz Rollout Runbook

Date: 2026-08-03 · Branch: `feature/signoz-observability`
Companions: [adr/0001-signoz-adoption.md](adr/0001-signoz-adoption.md) (verdict + preflight detail) · [../deploy/signoz/README.md](../deploy/signoz/README.md) (deploy mechanics) · [history/signoz-backend-instrumentation.md](history/signoz-backend-instrumentation.md) (the code spec — since implemented)

> **Status (2026-08-22).** OpenTelemetry **is** implemented and gated on `OTEL_ENABLED` (default `false`). SigNoz is deployed on the beta VM but its ingestion path is currently down — ClickHouse is exceeding its memory limit and the collector is dropping data. That is an open certification blocker: [RELEASE.md §5](RELEASE.md#5-open-before-v10).

Phases are strictly ordered; each has entry criteria, success criteria, and a rollback. Nothing in Phases 0–3 touches the Prometheus/Alertmanager stack, the 27 alert rules, or the local soak.

## Standing soak-safety rules (apply to every phase)

- The local soak stack is compose project **`qr-dining`** on ports **5432/6379/8080** — never target it, never share its project name, never `down -v` anything except the `qr-dining-signoz*` projects.
- Any restart of the soak/prod app keeps `AUDIT_LOG_V2_ENABLED=true` (R1 is live).
- All SigNoz work uses project names `qr-dining-signoz` (VM) / `qr-dining-signoz-eval` (local) and host ports 3301/4317/4318 only.

## Phase 0 — Sizing preflight (go/no-go)

**Entry:** decision doc approved; VM SSH access.
**Do:** run the preflight from [adr/0001-signoz-adoption.md](adr/0001-signoz-adoption.md) §Phase 0 (`free -h`, `docker stats --no-stream`, `df -h /var/lib/docker`), summing container *limits*, not usage.
**Go:** ≥ 3 GB genuinely free RAM and ≥ 20 GB free disk → proceed to Phase 1.
**No-go:** record the numbers in this file's log (below), adopt Fallback A (local-eval-only; Phases 1–2 still proceed, Phase 3 blocked) or Fallback B (upsize / second VM), and re-run.

## Phase 1 — Local eval

**Entry:** Phase 0 executed (any outcome — Phase 1 is local and free).
**Do:**
1. `docker network create signoz` (once), then bring up `qr-dining-signoz-eval` per `deploy/signoz/README.md` §Local eval. First boot: wait for migrators to exit 0; set retention 7d/7d in the UI (`http://localhost:3301`).
2. Implement the backend instrumentation per `docs/signoz-backend-instrumentation.md` (this can equally happen in Phase 2 order — eval just needs a build with the code in it).
3. Run the backend natively with `OTEL_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 OTEL_TRACES_SAMPLE_RATIO=1.0` against the **manual-testing** DBs (25432/26379).
4. Drive one full session flow (create → join → menu → order → payment) via the manual-testing frontend or curl.

**Success:** the flow's trace in SigNoz shows gin + pgx + redis spans stitched under one trace ID; `/health`/`/readyz`/`/metrics` produce no spans; killing the collector mid-run causes no request failures.
**Rollback:** `docker compose -p qr-dining-signoz-eval down -v`. Nothing else to undo.

## Phase 2 — Instrumentation merged, dark

**Entry:** Phase 1 success.
**Do:** land the instrumentation PR with `OTEL_ENABLED=false` everywhere (`deploy/vm/.env.production.example` documents the vars, default off). Run the §10 acceptance checks from the instrumentation spec — the disabled-path checks are the merge gate.
**Success:** CI green (lint, `go test -race`, sqlc drift, arm64 image build); disabled-path diff vs. base is behaviorally empty; RC soak untouched (this branch's binary is not the soak binary — the soak runs its pinned build).
**Rollback:** revert the PR; with the flag off it was inert, so this is a pure code revert with no operational steps.

## Phase 3 — VM deploy, dual-run

**Entry:** Phase 2 merged **and** Phase 0 was a GO. Coordinate with the RC certification state — do not combine this deploy with an app-version bump in the same change window.
**Do:**
1. Install the SigNoz stack at `/opt/qr-dining/signoz/` per `deploy/signoz/README.md` (network, compose up, migrators, retention 7d/7d).
2. Add the `docker-compose.signoz-app.yml` overlay; add the `logging:` rotation block to the app compose (the one sanctioned edit to `deploy/vm/docker-compose.yml` — do it in its own commit).
3. Set in `/opt/qr-dining/.env`: `OTEL_ENABLED=true`, `OTEL_EXPORTER_OTLP_ENDPOINT=signoz-otel-collector:4317`, `OTEL_TRACES_SAMPLE_RATIO=0.1`. Recreate the app (`docker compose ... up -d`); confirm `/readyz` healthy.
4. Verify the logs pipeline: filelog receiver ingesting `qr_dining_app` only (other projects' containers filtered out — check ingest volume in SigNoz).
5. Watch for 72h alongside the existing Prometheus alerts (which remain the alerting authority throughout).

**Success (all four, sustained 72h):**
- Trace→log pivot works in SigNoz (click a span → matching `trace_id` log lines).
- VM free RAM stays above the Phase 0 threshold; no container on the box OOM-kills; SigNoz services respect their mem_limits.
- App error rate / latency unchanged vs. pre-deploy Prometheus baselines (`HighServerErrorRate` etc. stay quiet).
- An `audit_log` row's `correlation_id` resolves to a real trace in SigNoz.

Then tune: raise `OTEL_TRACES_SAMPLE_RATIO` (0.1 → 0.3 → 1.0) only with RAM/disk evidence at each step.

**Rollback (fast, ~2 min):** set `OTEL_ENABLED=false`, recreate the app → tracing dark, app back to certified behavior. If the stack itself misbehaves: `docker compose -p qr-dining-signoz down` (keep volumes unless corrupt). The app never hard-depends on the collector, so stack-down without flag-off is also safe — just noisy Warn logs.

## Phase 4 — Later / optional (each its own decision)

- **Metrics dual-scrape:** add a `prometheus` receiver to the collector scraping `app:8080/metrics`, view in SigNoz alongside Prometheus. Only after 2+ weeks of stable Phase 3.
- **Alert consolidation:** evaluate migrating the 27 rules to SigNoz alerting. Acceptable outcome: "no — Prometheus stays forever." Do not migrate the alerts until Alertmanager's `CHANGEME` receiver is resolved anyway (see [OPERATIONS.md §3](OPERATIONS.md#3-alerting)) — an alert stack that pages nobody is the real gap there.
- **Frontend RUM / browser traces:** revisit the Cloudflare-Workers-compatible OTel JS options; propagate `traceparent` from the Next.js app so guest-visible latency joins backend traces.
- **Manual WS + worker spans:** per `docs/signoz-backend-instrumentation.md` §8, only if incidents demand.

## Phase log (append entries here as phases execute)

| Date | Phase | Outcome | Notes |
|---|---|---|---|
| 2026-08-03 | Phase 1 — Local eval | **PASS** | `qr-dining-signoz-eval` healthy; v0.129.0/v0.144.5 arm64-capable images. Full isolated manual-testing flow (create → join → cart/menu → order → payment) produced Gin + pgx + Redis spans. Payment trace/access log/audit correlation matched `368fb151c3909a3146da5f8790eb9945`. Infra endpoints were excluded; collector-stop request returned 200 with only a non-fatal exporter warning. `go test -race ./...` and a CGO-free linux/arm64 backend build passed. `OTEL_ENABLED=false` booted without an OTLP dial and preserved untraced health/readiness/metrics responses. Phase 3 remains blocked pending an explicit Phase 0 GO. |
