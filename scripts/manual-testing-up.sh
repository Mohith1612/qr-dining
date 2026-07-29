#!/usr/bin/env bash
# Manual Testing Certification environment — bring up.
#
# Stands up a fully ISOLATED multi-tenant stack that never touches the protected
# R1 soak (compose project `qr-dining`). Components:
#   - isolated Postgres + Redis  (docker project `manual-testing`, ports 25432 / 26379)
#   - two native backend instances (app-1 :8090, app-2 :8095) sharing that DB+Redis
#   - run from /tmp so backend/.env (which points at the soak DB) is NOT loaded
#   - frontends are launched separately (see the dashboard / report)
#
# Usage:  ./scripts/manual-testing-up.sh [--reset]
#   --reset   wipe the isolated datastore volumes and re-seed from scratch.
#
# SAFETY: this script only ever addresses the `manual-testing` compose project and
# the `qrapp-mtest` binary. It never stops/restarts the soak containers.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="$ROOT/backend"
COMPOSE="$ROOT/docker-compose.manual-testing.yml"
BIN=/tmp/qrapp-mtest

export DATABASE_URL="postgres://mtest:mtest_pass@localhost:25432/qrdining_mtest"
export REDIS_URL="redis://localhost:26379/0"
# ≥32 chars: release-mode config validation (pilot hardening phase A) fails
# hard on short secrets. Fixed value so guest tokens stay valid across restarts.
GUEST_SECRET="mtest-shared-secret-0123456789abcdef0123456789abcdef"
CORS="http://localhost:3000,http://localhost:3001"

RESET=0
[ "${1:-}" = "--reset" ] && RESET=1

echo "▶ datastores (project manual-testing)…"
if [ "$RESET" = "1" ]; then
  docker compose -f "$COMPOSE" down -v
fi
docker compose -f "$COMPOSE" up -d
echo "  waiting for health…"
for _ in $(seq 1 30); do
  pg=$(docker inspect -f '{{.State.Health.Status}}' manual-testing-postgres 2>/dev/null || echo none)
  rd=$(docker inspect -f '{{.State.Health.Status}}' manual-testing-redis 2>/dev/null || echo none)
  [ "$pg" = healthy ] && [ "$rd" = healthy ] && break
  sleep 2
done
echo "  postgres=$pg redis=$rd"

echo "▶ migrate + seed…"
( cd "$BACKEND" && go run ./cmd/migrate/main.go up )
if [ "$RESET" = "1" ]; then
  ( cd "$BACKEND" && go run ./scripts/seed.go )
fi

echo "▶ build backend binary ($BIN)…"
( cd "$BACKEND" && go build -o "$BIN" ./cmd/server )

echo "▶ stop any previous test backends…"
pkill -f qrapp-mtest 2>/dev/null || true
sleep 1

echo "▶ launch app-1 :8090 / app-2 :8095 (from /tmp, no .env)…"
cd /tmp
PORT=8090 WORKER_REGION=mtest-1 GUEST_TOKEN_SECRET="$GUEST_SECRET" CORS_ALLOWED_ORIGINS="$CORS" GIN_MODE=release \
  AUDIT_LOG_V2_ENABLED=true \
  DATABASE_URL="$DATABASE_URL" REDIS_URL="$REDIS_URL" nohup "$BIN" >/tmp/qrapp-mtest-1.log 2>&1 & disown
PORT=8095 WORKER_REGION=mtest-2 GUEST_TOKEN_SECRET="$GUEST_SECRET" CORS_ALLOWED_ORIGINS="$CORS" GIN_MODE=release \
  AUDIT_LOG_V2_ENABLED=true \
  DATABASE_URL="$DATABASE_URL" REDIS_URL="$REDIS_URL" nohup "$BIN" >/tmp/qrapp-mtest-2.log 2>&1 & disown
sleep 4
echo "▶ readyz:"
curl -s --max-time 5 http://localhost:8090/readyz && echo "  <- :8090"
curl -s --max-time 5 http://localhost:8095/readyz && echo "  <- :8095"
echo "✓ backends up. Start frontends: frontend-a (repo frontend/, :3000) and frontend-b (/tmp/frontend-b, :3001)."
