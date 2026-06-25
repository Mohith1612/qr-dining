#!/usr/bin/env bash
# Shared helpers for the qr-dining chaos harness.
#
# Each experiment script sources this file, brings prerequisites up, injects a
# fault, and captures observed behavior (metric scrapes + container logs) to a
# timestamped directory under scripts/chaos/results/.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP_URL="${APP_URL:-http://localhost:8080}"
COMPOSE="docker compose -f ${REPO_ROOT}/docker-compose.yml -f ${REPO_ROOT}/docker-compose.staging.yml"

RESULTS_DIR="${REPO_ROOT}/scripts/chaos/results/$(date +%Y%m%d-%H%M%S)-${EXPERIMENT:-run}"

log()  { printf '\033[1;34m[chaos]\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m[ ok ]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[FAIL]\033[0m %s\n' "$*"; }

init_results() {
  mkdir -p "$RESULTS_DIR"
  log "results → $RESULTS_DIR"
}

# scrape <name> — write the full /metrics snapshot to the results dir.
scrape() {
  local name="$1"
  curl -fsS "${APP_URL}/metrics" >"${RESULTS_DIR}/metrics-${name}.txt" 2>/dev/null \
    || warn "metrics scrape '${name}' failed (app may be mid-restart)"
}

# metric <substr> — print matching sample lines from the latest scrape on stdout.
metric() {
  local file="$1" pattern="$2"
  grep -E "^${pattern}" "${RESULTS_DIR}/metrics-${file}.txt" 2>/dev/null || true
}

# wait_ready <seconds> — poll /readyz until 200 or timeout. Returns non-zero on timeout.
wait_ready() {
  local timeout="${1:-60}" start now
  start=$(date +%s)
  while true; do
    if curl -fsS -o /dev/null "${APP_URL}/readyz"; then return 0; fi
    now=$(date +%s)
    if (( now - start > timeout )); then return 1; fi
    sleep 1
  done
}

# capture_logs <service> <name> — dump recent logs for a compose service.
capture_logs() {
  local svc="$1" name="$2"
  $COMPOSE logs --no-color --tail=200 "$svc" >"${RESULTS_DIR}/logs-${name}-${svc}.txt" 2>&1 || true
}

require_stack_up() {
  if ! curl -fsS -o /dev/null "${APP_URL}/health"; then
    fail "app not reachable at ${APP_URL}. Start it first:"
    echo "    docker network create proxy_network 2>/dev/null; ${COMPOSE} up -d --build"
    exit 1
  fi
}
