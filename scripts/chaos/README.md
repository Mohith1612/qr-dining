# Chaos harness

Fault-injection experiments that validate operational correctness under real-world
failure conditions. Each script brings prerequisites up, injects a fault, captures
metric scrapes + container logs to `scripts/chaos/results/<timestamp>-<exp>/`, and
prints a PASS/FAIL verdict.

## Prerequisites

Bring the stack up. In a sandbox without image-build network access, run the app as
a host-built binary in a container on both the internal backend network and the
external proxy network (so it reaches Postgres/Redis by DNS and publishes `:8080`):

```bash
# datastores
docker network create proxy_network 2>/dev/null
docker compose -f docker-compose.yml -f docker-compose.staging.yml up -d postgres redis

# app (host-built binary; modules already cached)
cd backend && CGO_ENABLED=0 GOOS=linux go build -o /tmp/qrapp ./cmd/server && cd ..
docker run -d --name qr-app-chaos --network proxy_network --network qr-dining_backend \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://qrdining:changeme_strong_password@postgres:5432/qrdining?sslmode=disable" \
  -e REDIS_URL="redis://redis:6379/0" -e PORT=8080 -e GIN_MODE=release \
  -e PAYMENT_WEBHOOK_SECRET_MOCK="chaos-secret-123" \
  -v /tmp/qrapp:/qrapp:ro alpine:3.20 /qrapp
```

Where image builds *do* have network access, `docker compose ... up -d --build` works
directly and the app service is named `app`; set `APP_CONTAINER=qr-dining-app-1`.

## Experiments

| Script               | Fault                         | Validates |
|----------------------|-------------------------------|-----------|
| `redis-flap.sh`      | Redis stopped 6s              | Redis non-authoritative; `/readyz` 503 during, recovers |
| `backend-restart.sh` | App container restart         | Idempotent migrations; stateless backend; clean recovery |
| `webhook-replay.sh`  | Forged / stale / replayed webhook | Signature + timestamp + idempotency defenses |
| `nginx-reload.sh`    | `nginx -s reload` mid-traffic | HTTP + WebSocket survive reload |
| `wsflood` (Go tool)  | Inbound frame flood / reconnect storm | Per-connection rate limit + ws-ticket burst cap |

## WebSocket flood / storm

Build the driver and point it at a live session (create one via
`POST /sessions {table_id, display_name}` and grab `guest_access_token` + `session.id`):

```bash
cd scripts/chaos/wsflood && GOFLAGS=-mod=mod go build -o /tmp/wsflood . && cd -

# inbound flood — expect server force-close at the strike budget
/tmp/wsflood -base http://localhost:8080 -session "$SID" -token "$GT" -mode flood -n 500

# reconnect storm — expect connected≈12, throttled_429≈rest (per-session ticket cap)
/tmp/wsflood -base http://localhost:8080 -session "$SID" -token "$GT" -mode storm -c 30
```

## Run the lot

```bash
WEBHOOK_SECRET=chaos-secret-123 EDGE_URL=http://localhost:18080 bash scripts/chaos/run-all.sh
```
