# SigNoz deployment inputs

The files in this directory are an optional self-hosted SigNoz stack. They are
**not verified as deployed**: the retained repository evidence contains no
successful production cutover or ingestion run. Current operational status is in
[docs/OPERATIONS.md](../../docs/OPERATIONS.md#metrics-and-alerts), and the decision
record is [ADR 0001](../../docs/adr/0001-signoz-adoption.md).

## Checked-in shape

The compose file defines ClickHouse, ZooKeeper, SigNoz, a schema migrator, and an
OTel collector. It binds the UI to loopback port 3301, OTLP to loopback ports
4317/4318, and applies explicit memory limits to the long-running services
(`deploy/signoz/docker-compose.signoz.yml:23-141`). ClickHouse, SQLite, and
ZooKeeper use named volumes (`deploy/signoz/docker-compose.signoz.yml:17-21`).

The collector accepts OTLP traces and metrics plus Docker JSON logs, filters log
records to `service="qr-dining"`, and exports to the SigNoz ClickHouse databases
(`deploy/signoz/otel-collector-config.yaml:8-40,52-79,118-163`). The compose
collector runs as root and mounts the configurable Docker containers directory
read-only because the file-log input reads container log files
(`deploy/signoz/docker-compose.signoz.yml:113-140`).

## Application side

The backend defaults tracing off. When enabled, it creates an insecure gRPC OTLP
exporter at the configured endpoint, parent-based ratio sampling, W3C trace and
baggage propagation, and a non-fatal export error handler
(`backend/internal/config/config.go:214-225`,
`backend/internal/observability/tracing.go:18-57`). Startup also treats tracing
initialization failure as non-fatal (`backend/cmd/server/main.go:39-44`).

There is no checked-in production connection between the two compose projects:
the app belongs to `proxy` and `qr-dining_internal`, while the collector belongs
to `signoz` and `signoz_internal`
(`deploy/vm/docker-compose.yml:33-59`,
`deploy/signoz/docker-compose.signoz.yml:11-15,129-140`). The production env
example leaves tracing off and only comments a collector hostname
(`deploy/vm/.env.production.example:91-95`). An operator must supply and verify
the missing network overlay before describing production ingestion as working.

## Unverified evaluation procedure

The stack requires the external `signoz` network
(`deploy/signoz/docker-compose.signoz.yml:11-15`):

```bash
docker network create signoz
docker compose -p qr-dining-signoz-eval \
  -f deploy/signoz/docker-compose.signoz.yml up -d
```

Before any non-local use, replace the literal tokenizer secret in the compose
file, verify every image for the target architecture, set an explicit Docker
containers directory, and prove traces and only QR Dining logs arrive. The
checked-in secret is visibly a local-evaluation placeholder
(`deploy/signoz/docker-compose.signoz.yml:77-90`), and the log mount defaults to
`/var/lib/docker/containers` unless overridden
(`deploy/signoz/docker-compose.signoz.yml:129-140`). No successful result for
these steps is retained.
