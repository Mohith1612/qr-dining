#!/usr/bin/env bash
# Experiment: sustained Redis outage during operation.
#
# Validates realtime-reconciliation-invariants: Redis is NEVER authoritative.
# The app must SURVIVE a Redis outage (Postgres remains the source of truth),
# report Redis unhealthy via /readyz while it is down, and recover cleanly.
#
# Pass criteria:
#   - app process stays alive across the whole outage (no crash/fatal)
#   - /readyz returns 503 + redis:unhealthy DURING the outage
#   - /readyz returns 200 + redis:ok AFTER recovery
#
# Note (observed): redis_pubsub_connected stays 1 even during the outage because
# go-redis's Channel() abstraction reconnects transparently and never closes the
# receive channel for a transient drop. The authoritative liveness signal for an
# outage is therefore /readyz (which pings Redis directly), not that gauge.
EXPERIMENT="redis-flap"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

REDIS_CONTAINER="${REDIS_CONTAINER:-qr-dining-redis-1}"
APP_CONTAINER="${APP_CONTAINER:-qr-app-chaos}"
OUTAGE_SECONDS="${OUTAGE_SECONDS:-6}"

init_results
require_stack_up

rc=0
log "baseline scrape + readyz"
scrape baseline
curl -s -o "${RESULTS_DIR}/readyz-baseline.json" -w "%{http_code}" "${APP_URL}/readyz" >"${RESULTS_DIR}/readyz-baseline.code"

log "stopping redis for ${OUTAGE_SECONDS}s…"
docker stop "$REDIS_CONTAINER" >/dev/null
sleep "$OUTAGE_SECONDS"

during_code=$(curl -s -o "${RESULTS_DIR}/readyz-during.json" -w "%{http_code}" "${APP_URL}/readyz" || echo "000")
app_status=$(docker ps --filter "name=${APP_CONTAINER}" --format '{{.Status}}')
scrape during
log "readyz DURING outage: HTTP ${during_code} → $(cat "${RESULTS_DIR}/readyz-during.json")"
log "app container DURING outage: ${app_status:-MISSING}"

log "starting redis…"
docker start "$REDIS_CONTAINER" >/dev/null
if wait_ready 60; then ok "/readyz recovered to 200"; else fail "/readyz did not recover"; rc=1; fi
after_body=$(curl -s "${APP_URL}/readyz")
scrape after
capture_logs app redis-flap || docker logs --tail 200 "$APP_CONTAINER" >"${RESULTS_DIR}/logs-app.txt" 2>&1

echo
log "=== RESULT ==="
# 1. app survived
if [ -n "$app_status" ] && echo "$app_status" | grep -q "Up"; then
  ok "app survived the outage (no crash): ${app_status}"
else
  fail "app did not survive the outage"; rc=1
fi
# 2. 503 during outage
if [ "$during_code" = "503" ]; then
  ok "readyz reported 503 during outage"
else
  fail "expected 503 during outage, got ${during_code}"; rc=1
fi
# 3. recovered
echo "$after_body" | grep -q '"redis":"ok"' && ok "redis healthy after recovery" || { fail "redis not healthy after recovery"; rc=1; }

[ $rc -eq 0 ] && ok "PASS: Redis is non-authoritative; app degraded and recovered" || fail "experiment failed"
exit $rc
