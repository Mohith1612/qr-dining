# SigNoz Backend Instrumentation — Exact-Change Spec

Date: 2026-08-03 · Branch: `feature/signoz-observability`
Companions: `signoz-adoption-decision.md` · `deploy/signoz/README.md` · `docs/signoz-rollout-runbook.md`

This is the implementation spec for the future session that adds OpenTelemetry tracing to the Go backend. Everything here is **additive and gated by `OTEL_ENABLED` (default `false`)** — with the flag off, behavior must be byte-for-byte identical to the uninstrumented build (no-op provider, no exporter dial, no new goroutines beyond a no-op shutdown func). That property is what keeps this mergeable while the RC soak gate is open.

## 1. Dependencies (`backend/go.mod`)

Check latest stable versions at implementation time; all are pure-Go (no cgo, arm64-safe):

```
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
github.com/exaring/otelpgx
github.com/redis/go-redis/extra/redisotel/v9
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp   # outbound; optional, see §8
```

## 2. Config surface (`backend/internal/config/config.go`)

Add an `OTel` field to `Config` (struct at `config.go:13`) following the existing `LogConfig` pattern (`config.go:60`, loaded at `config.go:189-190` via the `getenv` helpers):

```go
type OTelConfig struct {
	Enabled           bool    // OTEL_ENABLED, default false
	ExporterEndpoint  string  // OTEL_EXPORTER_OTLP_ENDPOINT, default "localhost:4317" (host:port, gRPC, no scheme)
	ServiceName       string  // OTEL_SERVICE_NAME, default "qr-dining-backend"
	TracesSampleRatio float64 // OTEL_TRACES_SAMPLE_RATIO, default 0.1, clamp to [0,1]
}
```

No release-mode hard-fail validation — telemetry must never block boot. Add the four vars to `deploy/vm/.env.production.example` section ⑤ (EDGE / OBSERVABILITY / TUNING), commented out except `OTEL_ENABLED=false`, with a pointer to `deploy/signoz/README.md` (VM value: `OTEL_EXPORTER_OTLP_ENDPOINT=signoz-otel-collector:4317`).

## 3. New file: `backend/internal/observability/tracing.go`

Sibling of `logger.go` / `metrics.go`. Shape:

```go
// SetupTracing initializes the global OTel TracerProvider. When cfg.Enabled is
// false it installs nothing and returns a no-op shutdown — zero behavior change.
func SetupTracing(ctx context.Context, cfg config.OTelConfig, logger zerolog.Logger) (shutdown func(context.Context) error, err error)
```

Requirements:

- **Disabled path:** return `func(context.Context) error { return nil }, nil` immediately. Do not touch otel globals (the default global provider is already no-op).
- **Enabled path:**
  - Resource: `service.name` = cfg.ServiceName, `service.version` = build info if cheap, `deployment.environment` = `GIN_MODE` value.
  - Exporter: `otlptracegrpc` with `WithEndpoint(cfg.ExporterEndpoint)`, `WithInsecure()` (traffic stays on the docker network / localhost — see `deploy/signoz/README.md`). **Non-blocking dial** — do not use `grpc.WithBlock`; a down collector must not delay boot.
  - Sampler: `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TracesSampleRatio))`.
  - Batcher: default `sdktrace.NewBatchSpanProcessor` settings.
  - Propagator: `propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})` via `otel.SetTextMapPropagator`.
  - Error handling: `otel.SetErrorHandler` → log at `Warn` (rate-limited by otel itself); exporter failures are non-fatal by design.
  - Shutdown func flushes the batcher with the caller's context.

## 4. Wiring in `backend/cmd/server/main.go`

After step 2 (`logger := observability.Setup(cfg.Log)` at `main.go:33`) and once the root signal context exists (step 3, `main.go:35`):

```go
// 3b. Tracing (no-op unless OTEL_ENABLED=true).
otelShutdown, err := observability.SetupTracing(ctx, cfg.OTel, logger)
if err != nil {
    logger.Warn().Err(err).Msg("otel init failed; continuing without tracing")
    otelShutdown = func(context.Context) error { return nil }
}
```

Call `otelShutdown` during graceful shutdown (before the final `shutdown complete` log at `main.go:120`), with a short timeout context (~5s) so a hung exporter can't stall `SHUTDOWN_TIMEOUT`. An init error is a warning, never fatal.

## 5. HTTP middleware (`backend/internal/server/server.go`)

Middleware stack is registered at `server.go:57-65`. Insert `otelgin` **after `Recover`, before `RequestID`** so (a) panics inside otelgin are caught, and (b) the span context is on `c.Request.Context()` before the audit and logger middleware run:

```go
r.Use(middleware.Recover(logger))          // existing, line 57
r.Use(otelgin.Middleware(cfg.OTel.ServiceName))   // NEW
r.Use(middleware.RequestID())              // existing
r.Use(audit.Middleware())                  // existing
r.Use(middleware.Logger(logger))           // existing
...
```

Notes:

- `otelgin.Middleware` is safe to register unconditionally when the provider is the global no-op (near-zero overhead), but for a guaranteed-identical disabled path, register it inside `if cfg.OTel.Enabled { ... }`. **Prefer the conditional** — it makes the `OTEL_ENABLED=false` ⇒ unchanged-behavior claim trivially true for RC certification purposes.
- Exclude the infra endpoints from tracing (they're scraped every 5–15s and would be sampler noise): use otelgin's filter option (`otelgin.WithFilter` / `WithGinFilter` depending on version) to skip `/health`, `/readyz`, `/metrics`.
- Span names default to the Gin route template (bounded cardinality, same property the metrics middleware relies on via `c.FullPath()`).

## 6. DB + Redis instrumentation (the net-new signal)

These fill the dead-metric hole (`DBQueryDuration`/`DBErrorsTotal`/`RedisOpsTotal` in `metrics.go` are declared but never incremented — do NOT wire those up; spans replace them).

**`backend/internal/db/pool.go`** — `pgxpool.Config.ConnConfig.Tracer` is currently unset. In `NewPool` (after `ParseConfig` at line 15, alongside the pool tuning at lines 20-25):

```go
if cfg.OTelEnabled { // plumb a bool through config.DBConfig, or pass config.OTelConfig
    poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())
}
```

Signature change: `NewPool(ctx, cfg config.DBConfig)` needs the otel flag — either add `OTelEnabled bool` to `DBConfig` during config load or add a parameter; pick whichever touches fewer call sites (grep for `db.NewPool`). Optionally also `otelpgx.RecordStats(pool)` for pool metrics via OTel — low value while Prometheus already exports pool stats (`main.go:96-112`), skip initially.

**`backend/internal/redis/client.go`** — no hooks currently. In `NewClient` after `goredis.NewClient(opts)` (line 18):

```go
if cfg.OTelEnabled { // same plumbing pattern as DBConfig
    if err := redisotel.InstrumentTracing(client); err != nil {
        return nil, fmt.Errorf("instrument redis tracing: %w", err)
    }
}
```

Skip `redisotel.InstrumentMetrics` (Prometheus stays authoritative for metrics).

Both are gated so the disabled path constructs identical clients.

## 7. Log↔trace and audit↔trace correlation

**`backend/internal/middleware/logger.go`** — the request-scoped logger is built at lines 18-23. Enrich it with the span context (valid only when tracing is on; guard with `sc.IsValid()`):

```go
logCtx := base.With().Str("request_id", requestID.(string))
if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
    logCtx = logCtx.Str("trace_id", sc.TraceID().String()).Str("span_id", sc.SpanID().String())
}
log := logCtx.Logger()
```

This lands `trace_id`/`span_id` in every access-log line **and** every handler log that uses `middleware.GetLogger(c)` — the collector's filelog pipeline promotes them for the trace↔log pivot in SigNoz (see `deploy/signoz/otel-collector-config.yaml`).

**`backend/internal/audit/middleware.go`** — `AuditRequestContext.CorrelationID` currently only carries the inbound `X-Correlation-ID` header (line 33) and persists to the existing `audit_log.correlation_id` column. Backfill with the trace ID when no explicit header is present:

```go
CorrelationID: c.GetHeader("X-Correlation-ID"),
// after building reqCtx:
if reqCtx.CorrelationID == "" {
    if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
        reqCtx.CorrelationID = sc.TraceID().String()
    }
}
```

This gives audit-row → SigNoz-trace pivots with **no schema change**. Ordering already works: audit middleware runs after otelgin per §5. Client-supplied correlation IDs keep precedence, so existing support-search behavior (`backend/internal/repository/support_search.go`) is unchanged.

## 8. Optional / deferred (do not block Phase 2 on these)

- **Outbound HTTP (`otelhttp`):** the main outbound calls are the R2 upload client (`backend/internal/storage`, AWS SDK). Wrap its `http.Client` transport with `otelhttp.NewTransport` if R2 latency ever needs tracing. Low priority.
- **WebSocket spans:** no upstream instrumentation for gorilla/websocket. If needed in Phase 3+, hand-create spans at the hub boundary (`backend/internal/websocket/hub.go`) for connect/auth and message-broadcast batches. The existing `ws_*` Prometheus metrics already cover rates/counts.
- **Background workers:** the lifecycle workers started from `main.go` get traced only if given spans manually — defer until a real incident demands it.

## 9. Adjacent issue to flag (not SigNoz scope, fix while in the file)

`GET /metrics` is served **unauthenticated on the public router** (`server.go:169`). The VM nginx template (`deploy/vm/qr-dining.conf.example`) blocks it at the edge (`allow 127.0.0.1; deny all`), but defense-in-depth suggests binding it away from the public surface (separate listener or an allowlist middleware). Raise it in the implementation PR; do not silently fold the fix into the tracing diff.

## 10. Acceptance checks for the implementation session

1. `OTEL_ENABLED=false` (default): `go test -race ./...` green; `git diff` on generated files empty; a diff of `/health`, `/readyz`, `/metrics`, and one full session flow's responses vs. the base branch shows no change; no OTLP dial attempts in logs.
2. `OTEL_ENABLED=true` against the local eval stack (`deploy/signoz/README.md` §Local eval): one QR session flow (create session → join → order → payment) produces a trace in SigNoz containing gin server spans + pgx query spans + redis command spans, and the access-log line for the same request carries the matching `trace_id`.
3. Kill the collector while the app runs: requests keep succeeding, only `Warn` logs from the otel error handler.
4. Audit: a mutating request with no `X-Correlation-ID` header lands an `audit_log` row whose `correlation_id` equals the trace ID shown in SigNoz.
5. CI (arm64 image build) passes — all new deps are pure Go.
