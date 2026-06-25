#!/usr/bin/env bash
# Runs the full chaos suite in sequence against an already-running stack.
# See scripts/chaos/README.md for the prerequisites (stack up, optional secret,
# optional seeded session for the nginx WS test).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

run() {
  echo
  echo "════════════════════════════════════════════════════════════"
  echo "  $1"
  echo "════════════════════════════════════════════════════════════"
  bash "$HERE/$1"; echo "exit=$?"
}

run redis-flap.sh
run backend-restart.sh
[ -n "${WEBHOOK_SECRET:-}" ] && run webhook-replay.sh || echo "skip webhook-replay (set WEBHOOK_SECRET to enable)"
[ -n "${EDGE_URL:-}" ] && run nginx-reload.sh || echo "skip nginx-reload (set EDGE_URL to enable)"
echo
echo "ws flood/storm are driven via /tmp/wsflood — see README.md"
