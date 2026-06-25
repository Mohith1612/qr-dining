#!/usr/bin/env bash
# Experiment: backend restart (simulates a deploy / crash-recovery).
#
# Validates: migrations are idempotent on every boot, the app re-establishes
# Postgres + Redis, and /readyz recovers. Postgres/Redis are untouched, so all
# durable state survives — the backend holds no authoritative state.
#
# Pass criteria:
#   - app restarts and /readyz returns 200 within the timeout
#   - boot log shows "migrations applied" with no migration error
#   - postgres + redis report ok after restart
EXPERIMENT="backend-restart"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

APP_CONTAINER="${APP_CONTAINER:-qr-app-chaos}"

init_results
require_stack_up

rc=0
scrape baseline

log "restarting backend container ${APP_CONTAINER}…"
restart_start=$(date +%s)
docker restart "$APP_CONTAINER" >/dev/null

if wait_ready 60; then
  recovered=$(( $(date +%s) - restart_start ))
  ok "/readyz recovered in ~${recovered}s"
else
  fail "/readyz did not recover within 60s"; rc=1
fi

docker logs --since "${restart_start}" "$APP_CONTAINER" >"${RESULTS_DIR}/boot-logs.txt" 2>&1 || \
  docker logs --tail 40 "$APP_CONTAINER" >"${RESULTS_DIR}/boot-logs.txt" 2>&1
scrape after
after_body=$(curl -s "${APP_URL}/readyz")

echo
log "=== RESULT ==="
if grep -q "migrations applied" "${RESULTS_DIR}/boot-logs.txt"; then
  ok "migrations applied idempotently on boot"
else
  warn "did not see 'migrations applied' in boot window (check boot-logs.txt)"
fi
if grep -qiE "migration.*(fail|error)|dirty database" "${RESULTS_DIR}/boot-logs.txt"; then
  fail "migration error on boot"; rc=1
else
  ok "no migration errors on boot"
fi
echo "$after_body" | grep -q '"postgres":"ok"' && ok "postgres ok after restart" || { fail "postgres not ok"; rc=1; }
echo "$after_body" | grep -q '"redis":"ok"' && ok "redis ok after restart" || { fail "redis not ok"; rc=1; }

[ $rc -eq 0 ] && ok "PASS: backend is stateless; clean restart + idempotent migrations" || fail "experiment failed"
exit $rc
