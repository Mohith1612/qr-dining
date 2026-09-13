#!/usr/bin/env bash
# Manual Testing Certification environment — tear down.
# Stops ONLY the manual-testing stack. NEVER touches the soak (project `qr-dining`).
#
# Usage:  ./scripts/manual-testing-down.sh [--keep-data]
#   --keep-data   leave the isolated Postgres/Redis volumes intact (default removes them).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE="$ROOT/docker-compose.manual-testing.yml"

echo "▶ stopping native test backends…"
pkill -f qrapp-mtest 2>/dev/null || true
echo "▶ stopping test frontends (:3000/:3001)…"
for p in 3000 3001; do
  pid=$(ss -ltnp 2>/dev/null | grep ":$p " | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2 || true)
  [ -n "${pid:-}" ] && kill "$pid" 2>/dev/null || true
done
pkill -f 'next dev' 2>/dev/null || true

if [ "${1:-}" = "--keep-data" ]; then
  echo "▶ stopping datastores (volumes kept)…"
  docker compose -f "$COMPOSE" down
else
  echo "▶ stopping datastores + removing volumes…"
  docker compose -f "$COMPOSE" down -v
fi
echo "✓ manual-testing stack down. Soak (project qr-dining) untouched."
