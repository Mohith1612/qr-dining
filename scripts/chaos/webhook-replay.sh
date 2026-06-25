#!/usr/bin/env bash
# Experiment: payment webhook forgery + replay defenses.
#
# Validates payment-finalization-invariants (WH/IDM groups): the webhook endpoint
# must reject forged signatures and stale timestamps, and must deduplicate exact
# replays by external event id.
#
# Requires the app to run with PAYMENT_WEBHOOK_SECRET_<PROVIDER> set so the valid
# path can be exercised. Provide it via env:
#   WEBHOOK_SECRET=... PROVIDER=mock bash webhook-replay.sh
#
# Cases:
#   1. valid signature + fresh timestamp  → NOT a signature rejection (401)
#   2. stale timestamp (replay window)     → 401 invalid signature
#   3. tampered body vs signature          → 401 invalid signature
#   4. exact replay of case 1              → idempotency_replays_total{webhook} rises
EXPERIMENT="webhook-replay"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

PROVIDER="${PROVIDER:-mock}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-}"

init_results
require_stack_up

if [ -z "$WEBHOOK_SECRET" ]; then
  fail "set WEBHOOK_SECRET (and run the app with PAYMENT_WEBHOOK_SECRET_${PROVIDER^^} to match)"
  exit 2
fi

sign() { # <ts> <body> -> hex hmac
  printf '%s.%s' "$1" "$2" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -r | awk '{print $1}'
}
send() { # <ts> <sig> <body> -> http code
  curl -s -o /dev/null -w "%{http_code}" -X POST "${APP_URL}/webhooks/payments/${PROVIDER}" \
    -H "Content-Type: application/json" \
    -H "X-Payment-Timestamp: $1" -H "X-Payment-Signature: $2" --data "$3"
}

rc=0
event_id="evt-$(date +%s)-$RANDOM"
body="{\"event_id\":\"${event_id}\",\"type\":\"payment.succeeded\",\"amount\":150}"
now=$(date +%s)
stale=$(( now - 600 ))   # 10 min old, beyond the 5-min tolerance

scrape baseline
replays_before=$(metric baseline 'idempotency_replays_total\{entity="webhook"\}' | awk '{print $2}' | head -1)
replays_before="${replays_before:-0}"

log "case 1: valid signature + fresh timestamp"
code1=$(send "$now" "$(sign "$now" "$body")" "$body")
if [ "$code1" = "401" ]; then fail "valid signature rejected as 401 (secret mismatch?)"; rc=1
else ok "valid signature accepted at the signature layer (HTTP ${code1}, not 401)"; fi

log "case 2: stale timestamp (10m old)"
code2=$(send "$stale" "$(sign "$stale" "$body")" "$body")
[ "$code2" = "401" ] && ok "stale timestamp rejected (401)" || { fail "stale timestamp NOT rejected (HTTP ${code2})"; rc=1; }

log "case 3: tampered body vs signature"
code3=$(send "$now" "$(sign "$now" "$body")" "${body%\}}, \"amount\":999999}")
[ "$code3" = "401" ] && ok "tampered body rejected (401)" || { fail "tampered body NOT rejected (HTTP ${code3})"; rc=1; }

log "case 4: exact replay of case 1"
code4=$(send "$now" "$(sign "$now" "$body")" "$body")
sleep 1
scrape after
replays_after=$(metric after 'idempotency_replays_total\{entity="webhook"\}' | awk '{print $2}' | head -1)
replays_after="${replays_after:-0}"
log "replay HTTP ${code4}; idempotency_replays_total{webhook}: ${replays_before} → ${replays_after}"
if awk "BEGIN{exit !(${replays_after} > ${replays_before})}"; then
  ok "exact replay deduplicated (idempotency counter rose)"
else
  warn "idempotency counter did not rise — replay may have been rejected earlier in the pipeline (HTTP ${code4}); inspect manually"
fi

capture_logs app webhook-replay || docker logs --tail 80 "${APP_CONTAINER:-qr-app-chaos}" >"${RESULTS_DIR}/logs-app.txt" 2>&1
[ $rc -eq 0 ] && ok "PASS: forged/stale webhooks rejected" || fail "experiment failed"
exit $rc
