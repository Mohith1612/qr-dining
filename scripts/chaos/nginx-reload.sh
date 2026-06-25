#!/usr/bin/env bash
# Experiment: nginx reload during active traffic.
#
# Validates that an `nginx -s reload` (config reload during a deploy) does NOT
# drop service: HTTP keeps serving and WebSocket upgrades keep working across the
# reload. nginx keeps old worker processes alive until their in-flight
# connections close, so established WS sessions survive a reload.
#
# Requires nginx running with deploy/nginx/qr-dining.conf on the proxy network,
# with the app reachable as upstream `app:8080`. EDGE_URL points at the nginx
# listener (default http://localhost:18080). NGINX_CONTAINER names it.
#
# Pass criteria:
#   - HTTP through nginx returns 200 before and after reload
#   - a WebSocket upgrade through nginx succeeds before and after reload
EXPERIMENT="nginx-reload"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

EDGE_URL="${EDGE_URL:-http://localhost:18080}"
NGINX_CONTAINER="${NGINX_CONTAINER:-qr-nginx-chaos}"
SESSION="${SESSION:-}"
TOKEN="${TOKEN:-}"

init_results

code_before=$(curl -s -o /dev/null -w "%{http_code}" "${EDGE_URL}/readyz" || echo 000)
log "HTTP via nginx before reload: ${code_before}"

ws_before="n/a"
if [ -n "$SESSION" ] && [ -n "$TOKEN" ] && [ -x /tmp/wsflood ]; then
  ws_before=$(/tmp/wsflood -base "$EDGE_URL" -session "$SESSION" -token "$TOKEN" -mode storm -c 1 | sed 's/.*connected=\([0-9]*\).*/\1/')
  log "WS via nginx before reload: connected=${ws_before}"
fi

log "reloading nginx…"
docker exec "$NGINX_CONTAINER" nginx -s reload 2>&1 | tee "${RESULTS_DIR}/reload.txt"
sleep 1

code_after=$(curl -s -o /dev/null -w "%{http_code}" "${EDGE_URL}/readyz" || echo 000)
log "HTTP via nginx after reload: ${code_after}"

ws_after="n/a"
if [ -n "$SESSION" ] && [ -n "$TOKEN" ] && [ -x /tmp/wsflood ]; then
  ws_after=$(/tmp/wsflood -base "$EDGE_URL" -session "$SESSION" -token "$TOKEN" -mode storm -c 1 | sed 's/.*connected=\([0-9]*\).*/\1/')
  log "WS via nginx after reload: connected=${ws_after}"
fi

docker logs --tail 50 "$NGINX_CONTAINER" >"${RESULTS_DIR}/logs-nginx.txt" 2>&1 || true

echo
log "=== RESULT ==="
rc=0
[ "$code_before" = "200" ] && ok "HTTP 200 before reload" || { fail "HTTP not 200 before reload (${code_before})"; rc=1; }
[ "$code_after" = "200" ] && ok "HTTP 200 after reload" || { fail "HTTP not 200 after reload (${code_after})"; rc=1; }
if [ "$ws_before" != "n/a" ]; then
  [ "$ws_before" = "1" ] && ok "WS upgrade worked before reload" || { fail "WS failed before reload"; rc=1; }
  [ "$ws_after" = "1" ] && ok "WS upgrade worked after reload" || { fail "WS failed after reload"; rc=1; }
fi
[ $rc -eq 0 ] && ok "PASS: nginx reload preserved HTTP + WS service" || fail "experiment failed"
exit $rc
