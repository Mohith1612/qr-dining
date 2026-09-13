#!/usr/bin/env bash
# Manual Testing Certification environment — bring up.
#
# Stands up a fully ISOLATED multi-tenant stack that never touches the protected
# R1 soak (compose project `qr-dining`). Components:
#   - isolated Postgres + Redis  (docker project `manual-testing`, ports 25432 / 26379)
#   - two native backend instances (app-1 :8090, app-2 :8095) sharing that DB+Redis
#   - run from /tmp so backend/.env (which points at the soak DB) is NOT loaded
#   - two frontends: app-a (repo frontend/ → :8090) on :3000, app-b (/tmp/frontend-b
#     → :8095) on :3001 — started automatically at the end of this script
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
WEBHOOK_SECRET="test-webhook-secret"
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
  PAYMENT_WEBHOOK_SECRET_STRIPE="$WEBHOOK_SECRET" \
  RATE_LIMIT_RPM=10000 AUTH_RATE_LIMIT_RPM=10000 \
  AUDIT_LOG_V2_ENABLED=true \
  AUTHZ_CENTRAL_POLICY_ENFORCE=true \
  DATABASE_URL="$DATABASE_URL" REDIS_URL="$REDIS_URL" nohup "$BIN" >/tmp/qrapp-mtest-1.log 2>&1 & disown
PORT=8095 WORKER_REGION=mtest-2 GUEST_TOKEN_SECRET="$GUEST_SECRET" CORS_ALLOWED_ORIGINS="$CORS" GIN_MODE=release \
  PAYMENT_WEBHOOK_SECRET_STRIPE="$WEBHOOK_SECRET" \
  RATE_LIMIT_RPM=10000 AUTH_RATE_LIMIT_RPM=10000 \
  AUDIT_LOG_V2_ENABLED=true \
  AUTHZ_CENTRAL_POLICY_ENFORCE=true \
  DATABASE_URL="$DATABASE_URL" REDIS_URL="$REDIS_URL" nohup "$BIN" >/tmp/qrapp-mtest-2.log 2>&1 & disown
sleep 4
echo "▶ readyz:"
curl -s --max-time 5 http://localhost:8090/readyz && echo "  <- :8090"
curl -s --max-time 5 http://localhost:8095/readyz && echo "  <- :8095"
echo "✓ backends up."

# ── Frontends ────────────────────────────────────────────────────────────────
# frontend-a = the repo (process env explicitly points at :8090) on :3000.
# frontend-b = a synced copy in /tmp pointing at :8095, on :3001 (cross-instance tests).
FE_A="$ROOT/frontend"
FE_B=/tmp/frontend-b

echo "▶ stop any previous test frontends…"
for p in 3000 3001; do
  pid=$(ss -ltnp 2>/dev/null | grep ":$p " | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2 || true)
  [ -n "${pid:-}" ] && kill "$pid" 2>/dev/null || true
done
pkill -f "next dev" 2>/dev/null || true
sleep 1

echo "▶ sync frontend-b ($FE_B → :8095)…"
mkdir -p "$FE_B"
rsync -a --delete --exclude node_modules --exclude .next --exclude .env.local "$FE_A"/ "$FE_B"/
ln -sfn "$FE_A/node_modules" "$FE_B/node_modules"
printf 'NEXT_PUBLIC_API_URL=http://localhost:8095\nNEXT_PUBLIC_WS_URL=ws://localhost:8095\n' > "$FE_B/.env.local"

# Clean dev build dirs so a stale .next can't wedge `next dev`.
rm -rf "$FE_A/.next" "$FE_B/.next"

echo "▶ launch frontend-a :3000 (→:8090) / frontend-b :3001 (→:8095)…"
( cd "$FE_A" && NEXT_PUBLIC_API_URL=http://localhost:8090 NEXT_PUBLIC_WS_URL=ws://localhost:8090 NEXT_PUBLIC_API_BASE=http://localhost:8090 \
    nohup npx next dev -p 3000 >/tmp/frontend-a.log 2>&1 ) & disown
( cd "$FE_B" && NEXT_PUBLIC_API_URL=http://localhost:8095 NEXT_PUBLIC_WS_URL=ws://localhost:8095 NEXT_PUBLIC_API_BASE=http://localhost:8095 \
    nohup npx next dev -p 3001 >/tmp/frontend-b.log 2>&1 ) & disown

echo "  waiting for frontends (first compile can take ~20s)…"
fa=000; fb=000
for _ in $(seq 1 40); do
  fa=$(curl -s -o /dev/null -w '%{http_code}' --max-time 4 http://localhost:3000/staff/login || echo 000)
  fb=$(curl -s -o /dev/null -w '%{http_code}' --max-time 4 http://localhost:3001/staff/login || echo 000)
  [ "$fa" = "200" ] && [ "$fb" = "200" ] && break
  sleep 3
done
echo "  frontend-a(:3000)=$fa  frontend-b(:3001)=$fb"
echo ""
echo "✓ stack up. Open the dashboard:  docs/manual-testing/testing-dashboard.html"
echo "   Frontend A → http://localhost:3000   (app-1 :8090)"
echo "   Frontend B → http://localhost:3001   (app-2 :8095)"
echo "   logs: /tmp/qrapp-mtest-{1,2}.log · /tmp/frontend-{a,b}.log"
