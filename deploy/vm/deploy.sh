#!/usr/bin/env bash
#
# qr-dining automatic deployment.
#
# Runs ON THE VM, from cron, as the unprivileged `appuser`. It is the whole
# deploy path: nothing pushes to this box, and no credential that could reach it
# exists anywhere off it.
#
# ---------------------------------------------------------------------------
# WHY PULL AND NOT GITHUB ACTIONS -> SSH
#
# Both were on the table. The pull model won on three counts:
#
#   1. Nothing has to be reachable. An Actions-driven deploy needs sshd open to
#      GitHub's runner ranges. Port 22 on this box also fronts job-queue,
#      sketchiple, grafana and SigNoz; widening its exposure is a standing
#      liability for all of them, not just for this project.
#
#   2. There is no deploy credential to leak. The repository is public and so is
#      the GHCR package (verified: an anonymous registry token fetches the
#      manifest), so the box needs no registry login and GitHub needs no SSH
#      key. The usual answer — "fork pull_request events don't get secrets" — is
#      true but is a property you have to keep being right about forever on a
#      public repo: one pull_request_target, one workflow_run misuse, one
#      compromised third-party action in the deploy job, and the key is gone.
#      Having no key is not a thing you can get wrong later.
#
#   3. appuser has no passwordless sudo, so a systemd unit is not installable.
#      Its crontab already runs the nightly backup rootlessly. Same mechanism,
#      same owner, same log.
#
# The cost, stated: up to one poll interval of latency, and GitHub's deployment
# UI knows nothing about any of this. The Telegram report is what pays for that.
# ---------------------------------------------------------------------------
#
# THE ONE THING THIS SCRIPT WILL NOT DO
#
# It never rolls a schema back. Not on health-gate failure, not on rollback, not
# ever. docs/RECOVERY.md records that `down` does not restore durable meaning
# across migrations 16-24 and 36/38/39, and that migration 22 wedges outright on
# a populated audit_log. A binary is revertible; a schema is not. Everything
# below is built around that asymmetry:
#
#   * migrations pending  -> STOP and ask a human. Do not apply them unattended.
#   * health gate failed  -> restore the previous IMAGE. Never touch the schema.
#
# Because the migration gate blocks every unattended schema change, the
# automatic rollback path only ever runs when the schema did NOT move — which is
# what makes an automatic binary rollback safe rather than a coin flip.
#
# Usage:
#   deploy.sh --auto              resolve origin/main and deploy it if it is new (cron)
#   deploy.sh --sha <sha>         deploy a specific commit that is on origin/main
#   deploy.sh --tag <tag>         deploy an operator-supplied image tag (drills)
#   deploy.sh --rollback          restore the image the containers ran before the last deploy
#   deploy.sh --status            print what is running and exit
#   deploy.sh --dry-run           with any of the above: decide, report, change nothing
#
set -Eeuo pipefail

# ---------------------------------------------------------------------------
# Re-exec from a copy before anything else.
#
# This script lives in the checkout it is about to fast-forward. bash reads a
# script incrementally by byte offset, so rewriting the file underneath a
# running shell makes it resume at an offset into different text. Copy first,
# run the copy.
# ---------------------------------------------------------------------------
if [[ "${QRD_DEPLOY_REEXEC:-}" != "1" ]]; then
    _self_copy="$(mktemp -t qrd-deploy.XXXXXX.sh)"
    cp -- "${BASH_SOURCE[0]}" "$_self_copy"
    export QRD_DEPLOY_REEXEC=1 QRD_DEPLOY_SELF_COPY="$_self_copy"
    exec bash "$_self_copy" "$@"
fi
if [[ -n "${QRD_DEPLOY_SELF_COPY:-}" ]]; then
    trap 'rm -f -- "$QRD_DEPLOY_SELF_COPY"' EXIT
fi

# ---------------------------------------------------------------------------
# Configuration. /opt/qr-dining/deploy.env may override any of it; see
# deploy/vm/deploy.env.example.
# ---------------------------------------------------------------------------
PROJECT_DIR="${QRD_PROJECT_DIR:-/opt/qr-dining}"
if [[ -r "$PROJECT_DIR/deploy.env" ]]; then . "$PROJECT_DIR/deploy.env"; fi

REPO_DIR="${QRD_REPO_DIR:-$PROJECT_DIR/repo}"
STATE_DIR="${QRD_STATE_DIR:-$PROJECT_DIR/deploy-state}"
ENV_FILE="${QRD_ENV_FILE:-$PROJECT_DIR/.env}"
COMPOSE_PROJECT="${QRD_COMPOSE_PROJECT:-qr-dining}"

GH_REPO="${QRD_GH_REPO:-Mohith1612/qr-dining}"
GH_BRANCH="${QRD_GH_BRANCH:-main}"
IMAGE_REPO="${QRD_IMAGE_REPO:-ghcr.io/mohith1612/qr-dining}"

PG_CONTAINER="${QRD_PG_CONTAINER:-qr_dining_postgres}"
INTERNAL_NET="${QRD_INTERNAL_NET:-qr-dining_qr-dining_internal}"
NGINX_CONTAINER="${QRD_NGINX_CONTAINER:-proxy_nginx}"

# service:container:public-probe-hostname, in rolling order. The first entry is
# the canary: if it does not come up, the second is never touched.
INSTANCES=(
    "app:qr_dining_app:qr-api-beta-1.mohith16.com"
    "app2:qr_dining_app_2:qr-api-beta-2.mohith16.com"
)

HEALTH_TIMEOUT="${QRD_HEALTH_TIMEOUT:-120}"
HEALTH_INTERVAL="${QRD_HEALTH_INTERVAL:-3}"

TELEGRAM_TOKEN_FILE="${QRD_TELEGRAM_TOKEN_FILE:-$PROJECT_DIR/secrets/alertmanager/telegram_token}"
TELEGRAM_CHAT_ID="${QRD_TELEGRAM_CHAT_ID:-5488346548}"

# How long a repeating condition stays quiet after it has been reported once.
# The agent polls every two minutes and a blocked or broken main stays blocked
# until a human acts, so without this a single pending migration would send 30
# identical messages an hour. Alertmanager's own repeat_interval for a page is
# 1h and for a ticket 12h, chosen so the channel does not get muted; a deploy
# that is waiting on a human is closer to the latter.
NOTIFY_REPEAT_SECONDS="${QRD_NOTIFY_REPEAT_SECONDS:-21600}"  # 6h

COMPOSE_FILES=(
    -f "$REPO_DIR/deploy/vm/docker-compose.yml"
    -f "$REPO_DIR/deploy/vm/docker-compose.beta.yml"
)
OBS_COMPOSE_FILES=(
    "${COMPOSE_FILES[@]}"
    -f "$REPO_DIR/deploy/observability/docker-compose.observability.yml"
    -f "$REPO_DIR/deploy/vm/docker-compose.observability-vm.yml"
)

MODE="" ARG="" DRY_RUN=0
NOTIFY_SENT=0
TARGET_SHA=""

# ---------------------------------------------------------------------------
# Output. Everything the script decides goes to stdout with a timestamp; cron
# appends it to deploy.log. Telegram gets the conclusion, not the transcript.
# ---------------------------------------------------------------------------
log()  { printf '%s  %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*"; }
step() { printf '\n%s  == %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*"; }

html_escape() { python3 -c 'import html,sys; sys.stdout.write(html.escape(sys.stdin.read()))'; }

# notify_once <dedup-key> <emoji> <title> <body>
#
# For conditions that persist across polls: pending migrations, a red main, an
# unreachable API. Sends once, then stays quiet for NOTIFY_REPEAT_SECONDS unless
# the key changes — and the key always carries the target SHA, so a NEW bad
# commit is never suppressed by an older one. Transitions that happen once
# (deployed, rolled back) use notify() and are never suppressed.
notify_once() {
    local key="$1"; shift
    local marker; marker="$STATE_DIR/notified-$(printf '%s' "$key" | tr -c 'A-Za-z0-9._-' '_')"
    if [[ -f "$marker" ]]; then
        local age=$(( $(date +%s) - $(stat -c %Y "$marker") ))
        if (( age < NOTIFY_REPEAT_SECONDS )); then
            NOTIFY_SENT=1
            log "(report suppressed: '$key' already reported ${age}s ago; repeats after ${NOTIFY_REPEAT_SECONDS}s)"
            return 0
        fi
    fi
    (( DRY_RUN )) || : > "$marker"
    notify "$@"
}

# Clear suppression markers once the condition they describe is gone, so the
# NEXT occurrence is reported immediately rather than swallowed by a stale one.
notify_reset() {
    if (( ! DRY_RUN )); then
        rm -f -- "$STATE_DIR"/notified-*
    fi
}

# notify <emoji> <title> <body>
notify() {
    local emoji="$1" title="$2" body="$3"
    NOTIFY_SENT=1
    if [[ ! -r "$TELEGRAM_TOKEN_FILE" ]]; then
        log "WARN: cannot read $TELEGRAM_TOKEN_FILE — deploy report NOT delivered"
        return 0
    fi
    if (( DRY_RUN )); then
        log "DRY RUN: would send Telegram: $emoji $title"
        return 0
    fi
    local text
    text="$emoji <b>$(printf '%s' "$title" | html_escape)</b>
$(printf '%s' "$body" | html_escape)"
    if curl -sS --max-time 15 -o /dev/null \
        -X POST "https://api.telegram.org/bot$(tr -d '\n' < "$TELEGRAM_TOKEN_FILE")/sendMessage" \
        -d "chat_id=${TELEGRAM_CHAT_ID}" -d "parse_mode=HTML" \
        --data-urlencode "text=${text}"; then
        log "Telegram report delivered."
    else
        # A deploy that fails silently at 20:00 is the shape of failure this
        # whole effort has been eliminating — so say so in the log at least.
        log "ERROR: Telegram report FAILED to send. The deploy outcome above is unreported."
    fi
}

# die reports through notify_once keyed on the target SHA plus the first line of
# the message. A condition that will still be true in two minutes — a red main,
# an unreachable API, a dirty schema — must not page on every tick.
die() {
    log "FATAL: $*"
    if (( ! NOTIFY_SENT )); then
        notify_once "fail-${TARGET_SHA:-none}-$(printf '%s' "$*" | head -c 60)" \
            "🚨" "qr-dining deploy ERROR" "$*

host: $(hostname)
log:  $PROJECT_DIR/deploy.log"
    fi
    exit 1
}

# An unexpected failure must never be quieter than an expected one.
trap 'rc=$?; log "unexpected failure, rc=$rc (command: ${BASH_COMMAND})"; (( NOTIFY_SENT )) || notify "🚨" "qr-dining deploy CRASHED" "rc=$rc on $(hostname)
failed command: ${BASH_COMMAND}
Containers may be mid-deploy. Check $PROJECT_DIR/deploy.log and deploy.sh --status."' ERR

# ---------------------------------------------------------------------------
# Small helpers
# ---------------------------------------------------------------------------
dc()     { docker compose --project-directory "$PROJECT_DIR" -p "$COMPOSE_PROJECT" "${COMPOSE_FILES[@]}" "$@"; }
dc_obs() { docker compose --project-directory "$PROJECT_DIR" -p "$COMPOSE_PROJECT" "${OBS_COMPOSE_FILES[@]}" "$@"; }

container_image_ref() { docker inspect "$1" --format '{{.Config.Image}}' 2>/dev/null || true; }
container_image_id()  { docker inspect "$1" --format '{{.Image}}'        2>/dev/null || true; }

# Rewrite one KEY=VALUE in the env file, preserving mode and ownership. The file
# is 0600 and holds every production secret; never widen it, never leave a
# half-written copy behind.
set_env_var() {
    local key="$1" value="$2" tmp
    tmp="$(mktemp "${ENV_FILE}.XXXXXX")"
    chmod --reference="$ENV_FILE" "$tmp"
    if grep -qE "^${key}=" "$ENV_FILE"; then
        KEY="$key" VALUE="$value" python3 -c '
import os, sys
key, value = os.environ["KEY"], os.environ["VALUE"]
for line in sys.stdin:
    sys.stdout.write(f"{key}={value}\n" if line.startswith(key + "=") else line)
' < "$ENV_FILE" > "$tmp"
    else
        cat "$ENV_FILE" > "$tmp"
        printf '%s=%s\n' "$key" "$value" >> "$tmp"
    fi
    mv -- "$tmp" "$ENV_FILE"
}

get_env_var() { sed -nE "s/^$1=(.*)$/\1/p" "$ENV_FILE" | tail -1; }

# ---------------------------------------------------------------------------
# Checkout hygiene.
#
# /opt/qr-dining/repo is production state: Prometheus, Alertmanager and (since
# this change) the shared proxy's qr-dining vhost are all bind-mounted out of
# it. Deploying means fast-forwarding it, so anything that would make that
# fast-forward destroy uncommitted work, or silently deploy an unreviewed ref,
# stops the deploy. docs/OPERATIONS.md, "The new hazard".
# ---------------------------------------------------------------------------
assert_checkout_sane() {
    [[ -d "$REPO_DIR/.git" ]] || die "$REPO_DIR is not a git checkout"
    local branch dirty ahead
    branch="$(git -C "$REPO_DIR" rev-parse --abbrev-ref HEAD)"
    [[ "$branch" == "$GH_BRANCH" ]] || die \
"checkout is on '$branch', not '$GH_BRANCH'. That checkout is production state for
Prometheus, Alertmanager and the nginx vhost — whatever it says is what is
running. Refusing to deploy from an unreviewed ref. Fix with:
  git -C $REPO_DIR checkout $GH_BRANCH"

    dirty="$(git -C "$REPO_DIR" status --porcelain)"
    [[ -z "$dirty" ]] || die \
"checkout has local modifications. A fast-forward would silently discard them,
and they are currently LIVE monitoring/routing config. Inspect, then clean:
$(printf '%s\n' "$dirty" | head -20)"

    ahead="$(git -C "$REPO_DIR" rev-list --count "origin/${GH_BRANCH}..${GH_BRANCH}" 2>/dev/null || echo 0)"
    [[ "$ahead" == "0" ]] || die "checkout has $ahead commit(s) not on origin/$GH_BRANCH; refusing to deploy unreviewed commits"
}

# ---------------------------------------------------------------------------
# CI gate.
#
# NOT optional, and not redundant with branch protection. The docker-build job
# in .github/workflows/ci.yml has no `needs:`, so the image is pushed to GHCR on
# every push to main EVEN IF EVERY TEST FAILED. The existence of a tag proves a
# build, not a green run. Ask GitHub what the checks actually said.
#
# Unauthenticated: the repo is public, which keeps the "no credential on the
# box" property intact. It costs one API call per new commit and is subject to a
# 60/hour per-IP limit — a rate-limited or unreachable API is treated as "do not
# know", which means "do not deploy", not "probably fine".
# ---------------------------------------------------------------------------
assert_ci_green() {
    local sha="$1" body
    body="$(curl -sS --max-time 30 -H 'Accept: application/vnd.github+json' \
        "https://api.github.com/repos/$GH_REPO/commits/$sha/check-runs?per_page=100")" \
        || die "GitHub check-runs API unreachable for $sha — refusing to deploy on an unknown CI state"

    local verdict
    verdict="$(printf '%s' "$body" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception as e:
    print("ERROR|could not parse the check-runs response: %s" % e); raise SystemExit
if "check_runs" not in d:
    print("ERROR|%s" % d.get("message", "unexpected check-runs response")); raise SystemExit
runs = d["check_runs"]
if len(runs) < 10:
    print("ERROR|only %d check-runs reported; expected the full CI matrix" % len(runs)); raise SystemExit
pending = [r["name"] for r in runs if r["status"] != "completed"]
bad = [f'"'"'{r["name"]}={r["conclusion"]}'"'"' for r in runs
       if r["status"] == "completed" and r["conclusion"] not in ("success", "neutral", "skipped")]
if pending:
    print("PENDING|%s" % ", ".join(sorted(pending))); raise SystemExit
if bad:
    print("FAILED|%s" % ", ".join(sorted(bad))); raise SystemExit
print("OK|%d checks green" % len(runs))
')"

    case "${verdict%%|*}" in
        OK)      log "CI gate: ${verdict#*|} for $sha" ;;
        PENDING) log "CI gate: checks still running (${verdict#*|}); will retry next poll"; exit 0 ;;
        FAILED)  die "CI gate: $sha did NOT pass CI (${verdict#*|}). The GHCR image exists anyway — docker-build has no \`needs:\` — which is exactly why this check exists." ;;
        *)       die "CI gate: ${verdict#*|}" ;;
    esac

    # Commit STATUSES are a separate surface from check-runs, and the fifteenth
    # gate on this repo — GitGuardian — lives on it rather than on a check-run.
    # It is a pull_request-only integration, so a push-to-main commit normally
    # carries no statuses at all; an empty list therefore means "nothing to
    # check", not "pending". A status that does exist and is not green stops the
    # deploy.
    local status_body status_verdict
    status_body="$(curl -sS --max-time 30 -H 'Accept: application/vnd.github+json' \
        "https://api.github.com/repos/$GH_REPO/commits/$sha/status")" \
        || die "GitHub commit-status API unreachable for $sha — refusing to deploy on an unknown CI state"
    status_verdict="$(printf '%s' "$status_body" | python3 -c '
import json, sys
d = json.load(sys.stdin)
sts = d.get("statuses")
if sts is None:
    print("ERROR|%s" % d.get("message", "unexpected commit-status response")); raise SystemExit
if not sts:
    print("OK|no commit statuses on this SHA"); raise SystemExit
bad = ["%s=%s" % (s["context"], s["state"]) for s in sts if s["state"] != "success"]
print(("FAILED|%s" % ", ".join(sorted(bad))) if bad else ("OK|%d status(es) green" % len(sts)))
')"
    case "${status_verdict%%|*}" in
        OK)     log "CI gate: ${status_verdict#*|}" ;;
        FAILED) die "CI gate: commit statuses on $sha are not green (${status_verdict#*|})" ;;
        *)      die "CI gate: ${status_verdict#*|}" ;;
    esac
}

# ---------------------------------------------------------------------------
# Migration pre-flight. Runs BEFORE any container is touched.
#
# Two independent computations of "what would this image do to the schema", from
# two different artefacts, which must agree:
#
#   A. THE IMAGE. `/app/migrate pending` inside the new image reads the same
#      embedded migrations.FS that cmd/server applies at boot, and the same
#      schema_migrations row. Nothing is closer to the truth about what the
#      container would do on start. It writes nothing.
#
#   B. THE COMMIT. The highest NNNNNN_*.up.sql under backend/migrations/ at the
#      deploy SHA, read out of git with ls-tree (no working-tree dependency).
#
# A alone would miss a tag that does not correspond to the commit being
# deployed — a retag, a rebuild, a cache mishap. B alone would be asking a
# source tree about a binary. Disagreement means the artefacts are not the same
# release and the deploy stops.
# ---------------------------------------------------------------------------
db_schema_version() {
    docker exec "$PG_CONTAINER" psql -U "$(get_env_var POSTGRES_USER)" -d "$(get_env_var POSTGRES_DB)" \
        -tAF'|' -c 'select version, dirty from schema_migrations' 2>/dev/null | tr -d ' '
}

# migration_preflight <image> [commit-sha]
#
# Sets PENDING_COUNT and PENDING_LIST. Deliberately NOT a command substitution:
# it can call die(), and a die() inside $( ) would only kill the subshell, after
# which set -e would trip the ERR trap and page a second time for the same
# failure. Assigning globals keeps it in this shell.
migration_preflight() {
    local image="$1" sha="${2:-}" out rc

    local db_row; db_row="$(db_schema_version)"
    log "schema_migrations says: ${db_row:-<empty>}"

    # `if`, not `set +e`: a non-zero exit is the EXPECTED answer here (10 means
    # migrations are pending), and `set +e` does NOT suppress the ERR trap —
    # bash still fires it on a failed assignment-from-command-substitution. When
    # this was written with set +e, one pending migration produced three Telegram
    # messages: two spurious "deploy CRASHED" pages from the trap (one inside the
    # command substitution's subshell, one outside) and then the real one.
    # Commands in an `if` condition are exempt from both.
    if out="$(docker run --rm --network "$INTERNAL_NET" --env-file "$ENV_FILE" \
                --entrypoint /app/migrate "$image" pending 2>&1)"; then
        rc=0
    else
        rc=$?
    fi
    printf '%s\n' "$out" | sed 's/^/    migrate: /'

    if (( rc == 125 )) || grep -q 'no such file or directory' <<<"$out"; then
        die "the image does not carry /app/migrate, so the migration gate cannot run.
Refusing to deploy: an unattended deploy with no migration pre-flight is exactly
the thing this path exists to prevent. Images built before backend/docker/Dockerfile
shipped cmd/migrate cannot be auto-deployed."
    fi

    local image_highest pending_count pending_list
    image_highest="$(sed -nE 's/^image_highest=([0-9]+)$/\1/p' <<<"$out" | tail -1)"
    pending_count="$(sed -nE 's/^pending_count=([0-9]+)$/\1/p' <<<"$out" | tail -1)"
    pending_list="$(sed -nE 's/^pending=(.*)$/\1/p' <<<"$out" | tail -1)"
    [[ -n "$image_highest" && -n "$pending_count" ]] || die "could not read the migration pre-flight output (rc=$rc)"

    # Cross-check B against A.
    if [[ -n "$sha" ]]; then
        local commit_highest
        commit_highest="$(git -C "$REPO_DIR" ls-tree --name-only "$sha" backend/migrations/ \
            | sed -nE 's#.*/0*([0-9]+)_.*\.up\.sql$#\1#p' | sort -n | tail -1)"
        [[ -n "$commit_highest" ]] || die "found no migrations at commit $sha — refusing to guess"
        if [[ "$commit_highest" != "$image_highest" ]]; then
            die "ARTEFACT MISMATCH: image $image carries migrations up to $image_highest, but commit
$sha carries them up to $commit_highest. The tag does not correspond to the
commit. Refusing to deploy — this is how a schema change arrives unannounced."
        fi
        log "cross-check: commit $sha and the image agree on highest migration = $image_highest"
    else
        log "cross-check: SKIPPED (operator-supplied tag, no commit to compare against)"
    fi

    case $rc in
        0)  log "migration gate: nothing pending." ;;
        10) : ;;  # caller decides, based on approval
        11) die "migration gate: schema_migrations is DIRTY. A previous migration failed part-way and
the schema is whatever it left behind. Nothing automatic may run against it.
docs/RECOVERY.md has the inspect/force procedure; the migrate binary is now in
the image, so it is runnable on this box:
  docker run --rm --network $INTERNAL_NET --env-file $ENV_FILE \\
    --entrypoint /app/migrate $image version" ;;
        12) die "migration gate: the DATABASE IS AHEAD of this image. Deploying it would run an old
binary against a newer schema. Nothing here rolls a schema back — that is not a
capability this path has, by design — so a human must confirm the older binary
tolerates the newer schema." ;;
        *)  die "migration gate: pre-flight failed (rc=$rc)" ;;
    esac

    PENDING_COUNT="$pending_count"
    PENDING_LIST="$pending_list"
}

approval_token() { printf '%s/approved-%s' "$STATE_DIR" "$1"; }

# ---------------------------------------------------------------------------
# Health gate.
#
# /readyz returns {"checks":{"postgres":"ok","redis":"ok"},"status":"ready"}.
# A 200 is not the assertion. The handler only reaches 503 when a dependency
# ping fails outright (backend/internal/handlers/health.go); any proxy, any
# stub, any half-booted thing can return 200. Assert the body:
#   status == "ready"  AND  every value in checks == "ok".
#
# The probe goes to the container's own IP on the internal docker network, not
# through nginx and not via `docker exec`. Through nginx a probe can be answered
# by the OTHER instance and the gate would pass on the wrong container; via exec
# it fails for the wrong reason when the container is restarting. The public
# per-instance hostname is checked afterwards, separately, as a routing
# assertion rather than as the liveness gate.
# ---------------------------------------------------------------------------
readyz_ok() {
    python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception as e:
    print("unparseable body: %s" % e); raise SystemExit(1)
if d.get("status") != "ready":
    print("status=%r (want \"ready\")" % d.get("status")); raise SystemExit(1)
checks = d.get("checks")
if not isinstance(checks, dict) or not checks:
    print("no checks object in body"); raise SystemExit(1)
bad = {k: v for k, v in checks.items() if v != "ok"}
if bad:
    print("failing checks: %s" % bad); raise SystemExit(1)
print("status=ready checks=%s" % ",".join("%s:%s" % kv for kv in sorted(checks.items())))
'
}

# health_gate <container> <expected-image-id>
health_gate() {
    local container="$1" want_image="$2"
    local deadline=$(( SECONDS + HEALTH_TIMEOUT )) last="(no attempt yet)"

    while (( SECONDS < deadline )); do
        local state ip body verdict
        state="$(docker inspect "$container" --format '{{.State.Status}}' 2>/dev/null || echo missing)"
        if [[ "$state" != "running" ]]; then
            last="container state=$state"
        else
            ip="$(docker inspect "$container" --format \
                "{{with index .NetworkSettings.Networks \"$INTERNAL_NET\"}}{{.IPAddress}}{{end}}" 2>/dev/null || true)"
            if [[ -z "$ip" ]]; then
                last="container has no address on $INTERNAL_NET"
            elif ! body="$(curl -sS --max-time 5 "http://$ip:8080/readyz" 2>&1)"; then
                last="no answer from $ip:8080/readyz: $body"
            elif ! verdict="$(printf '%s' "$body" | readyz_ok)"; then
                last="/readyz body rejected: ${verdict:-$body}"
            else
                local got_image; got_image="$(container_image_id "$container")"
                if [[ "$got_image" != "$want_image" ]]; then
                    # Ready, but not the image we deployed. Never report success
                    # for a container we did not actually replace.
                    last="running image ${got_image:0:19} but expected ${want_image:0:19}"
                else
                    log "  $container: $verdict (image ${got_image:0:19}) after ${SECONDS}s"
                    return 0
                fi
            fi
        fi
        sleep "$HEALTH_INTERVAL"
    done

    log "  $container: HEALTH GATE FAILED after ${HEALTH_TIMEOUT}s — last: $last"
    docker logs --tail 25 "$container" 2>&1 | sed 's/^/      | /' || true
    return 1
}

# Routing assertion: the public per-instance hostname must reach this instance
# through the shared proxy. Separate from the gate on purpose — a routing
# failure and an application failure are different incidents.
probe_public() {
    local host="$1" body
    if ! body="$(curl -sS --max-time 10 "https://$host/readyz" 2>&1)"; then
        log "  WARN: https://$host/readyz did not answer: $body"
        return 1
    fi
    if ! printf '%s' "$body" | readyz_ok > /dev/null; then
        log "  WARN: https://$host/readyz answered but the body is not ready: $body"
        return 1
    fi
    log "  https://$host/readyz -> $body"
}

# ---------------------------------------------------------------------------
# Config deploy: fast-forward the checkout.
#
# This IS a deploy of monitoring and routing config, not a bookkeeping step. The
# nginx vhost and the Prometheus/Alertmanager files are bind-mounted out of this
# directory, so the fast-forward changes what a SHARED proxy and the alerting
# stack will load. Validate before anything can pick it up, and revert the
# checkout — not just skip the reload — if validation fails.
# ---------------------------------------------------------------------------
apply_config() {
    local target="$1" from changed
    from="$(git -C "$REPO_DIR" rev-parse HEAD)"

    if [[ "$from" == "$target" ]]; then
        log "checkout already at $target; no config change"
        return 0
    fi

    changed="$(git -C "$REPO_DIR" diff --name-only "$from" "$target")"
    log "fast-forwarding checkout $from -> $target ($(wc -l <<<"$changed") files)"
    if (( DRY_RUN )); then
        log "DRY RUN: would fast-forward and validate"
        return 0
    fi
    git -C "$REPO_DIR" merge --ff-only "$target" >/dev/null \
        || die "checkout could not fast-forward to $target"

    # nginx first: it is shared with four other projects, so a bad config here
    # is their outage. Revert the checkout rather than leave it staged to be
    # loaded by an unrelated restart later.
    if ! docker exec "$NGINX_CONTAINER" nginx -t >/dev/null 2>&1; then
        local why; why="$(docker exec "$NGINX_CONTAINER" nginx -t 2>&1 || true)"
        git -C "$REPO_DIR" reset --hard "$from" >/dev/null
        docker exec "$NGINX_CONTAINER" nginx -t >/dev/null 2>&1 \
            && die "nginx config at $target is INVALID; checkout reverted to $from and the proxy is intact.
$why" \
            || die "nginx config at $target is INVALID *and* the revert to $from did not restore a
valid config. THE SHARED PROXY IS IN A BAD STATE — grafana, job-queue and
sketchiple are affected. Fix by hand now.
$why"
    fi
    log "nginx -t passed at $target"

    if grep -q '^deploy/vm/nginx/' <<<"$changed"; then
        docker exec "$NGINX_CONTAINER" nginx -s reload
        log "nginx reloaded (routing config changed)"
    fi

    # Observability. NOT a SIGHUP: deploy/observability/* is bind-mounted file by
    # file, and a single-file bind mount pins its inode, so a container that is
    # only reloaded goes on reading the file git already replaced. Recreate.
    # docs/OPERATIONS.md, "Why the nginx mount is a directory".
    if grep -q '^deploy/observability/' <<<"$changed"; then
        log "observability config changed — RECREATING prometheus/alertmanager/blackbox"
        log "  (a reload would re-read the pre-pull inode; see docs/OPERATIONS.md)"
        dc_obs up -d --no-deps --force-recreate prometheus alertmanager blackbox
        verify_rules_loaded || log "  WARN: rule-count verification failed — see above"
    fi
}

# The 2026-09-16 signature: the file on disk and the rules Prometheus actually
# evaluates disagreeing. Read the number; do not infer health from silence.
verify_rules_loaded() {
    local in_file loaded
    in_file="$(grep -c '^      - alert:' "$REPO_DIR/deploy/observability/prometheus-alerts.yml" || true)"
    loaded="$(curl -sS --max-time 10 localhost:9090/api/v1/rules 2>/dev/null \
        | python3 -c 'import json,sys; print(sum(len(g["rules"]) for g in json.load(sys.stdin)["data"]["groups"]))' 2>/dev/null || echo "?")"
    log "  alert rules: file=$in_file loaded=$loaded"
    [[ "$in_file" == "$loaded" ]]
}

# ---------------------------------------------------------------------------
# Rolling restart.
# ---------------------------------------------------------------------------
# roll_instance <service> <container> <public-host> <image-id>
roll_instance() {
    local svc="$1" container="$2" host="$3" image_id="$4"
    log "recreating $container ($svc)"
    if (( DRY_RUN )); then log "DRY RUN: would recreate $container"; return 0; fi
    dc up -d --no-deps "$svc"
    health_gate "$container" "$image_id" || return 1
    probe_public "$host" || true
}

# restore_image <repo> <tag> <services...>
restore_image() {
    local repo="$1" tag="$2"; shift 2
    log "ROLLBACK: restoring image $repo:$tag on $*"
    log "ROLLBACK: the SCHEMA IS NOT TOUCHED. See the header of this script."
    set_env_var IMAGE_REPO "$repo"
    set_env_var IMAGE_TAG "$tag"
    dc up -d --no-deps "$@"
    local want_id ok=1 entry inst s container
    want_id="$(docker image inspect "$repo:$tag" --format '{{.Id}}' 2>/dev/null || echo unknown)"
    for entry in "$@"; do
        for inst in "${INSTANCES[@]}"; do
            IFS=: read -r s container _ <<<"$inst"
            [[ "$s" == "$entry" ]] || continue
            health_gate "$container" "$want_id" || ok=0
        done
    done
    (( ok )) || return 1
}

# ---------------------------------------------------------------------------
# Status
# ---------------------------------------------------------------------------
print_status() {
    local entry svc container host
    printf 'checkout: %s %s (%s)\n' \
        "$(git -C "$REPO_DIR" rev-parse --short HEAD)" \
        "$(git -C "$REPO_DIR" log -1 --format='%s')" \
        "$(git -C "$REPO_DIR" rev-parse --abbrev-ref HEAD)"
    printf 'env:      IMAGE_REPO=%s IMAGE_TAG=%s\n' "$(get_env_var IMAGE_REPO)" "$(get_env_var IMAGE_TAG)"
    printf 'schema:   %s\n' "$(db_schema_version)"
    for entry in "${INSTANCES[@]}"; do
        IFS=: read -r svc container host <<<"$entry"
        printf '%-16s %-46s %s  %s\n' "$container" "$(container_image_ref "$container")" \
            "$(docker inspect "$container" --format '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}-{{end}}' 2>/dev/null)" \
            "$(curl -sS --max-time 5 "https://$host/readyz" 2>&1 || echo '<no answer>')"
    done
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
while (( $# )); do
    case "$1" in
        --auto)     MODE=auto ;;
        --sha)      MODE=sha;  ARG="${2:?--sha needs a commit}"; shift ;;
        --tag)      MODE=tag;  ARG="${2:?--tag needs an image tag}"; shift ;;
        --rollback) MODE=rollback ;;
        --status)   MODE=status ;;
        --dry-run)  DRY_RUN=1 ;;
        -h|--help)  sed -n '2,60p' "$0"; exit 0 ;;
        *)          echo "unknown argument: $1" >&2; exit 2 ;;
    esac
    shift
done
[[ -n "$MODE" ]] || { echo "usage: deploy.sh --auto | --sha <sha> | --tag <tag> | --rollback | --status [--dry-run]" >&2; exit 2; }

mkdir -p "$STATE_DIR"

if [[ "$MODE" == "status" ]]; then print_status; exit 0; fi

# One deploy at a time. Two cron ticks overlapping mid-rolling-restart would
# race on .env and on the containers.
exec 9>"$STATE_DIR/deploy.lock"
flock -n 9 || { log "another deploy is running; exiting"; exit 0; }

DRY_LABEL=""
if (( DRY_RUN )); then DRY_LABEL=", DRY RUN"; fi
step "qr-dining deploy ($MODE${ARG:+ $ARG}$DRY_LABEL) on $(hostname)"

# What is serving right now. Read from the running containers, not from a state
# file: a state file can drift from reality, and this value is the rollback
# target. It has to be true.
IFS=: read -r _ CANARY_CONTAINER _ <<<"${INSTANCES[0]}"
BEFORE_REF="$(container_image_ref "$CANARY_CONTAINER")"
[[ -n "$BEFORE_REF" ]] || die "$CANARY_CONTAINER is not running, so there is no image to roll back to.
A rolling deploy needs something to roll from. Bring the stack up by hand first."
BEFORE_REPO="${BEFORE_REF%:*}"; BEFORE_TAG="${BEFORE_REF##*:}"
BEFORE_ID="$(container_image_id "$CANARY_CONTAINER")"
log "currently serving: $BEFORE_REF"
for entry in "${INSTANCES[@]}"; do
    IFS=: read -r _ c _ <<<"$entry"
    log "  before: $c = $(container_image_ref "$c")"
done

IMAGE="" TAG=""

case "$MODE" in
    rollback)
        PREV="$STATE_DIR/previous"
        [[ -r "$PREV" ]] || die "no recorded previous deploy in $PREV"
        # shellcheck disable=SC1090
        . "$PREV"
        step "Manual rollback to ${PREV_REPO:?}:${PREV_TAG:?}"
        restore_image "$PREV_REPO" "$PREV_TAG" app app2 \
            && notify "↩️" "qr-dining rolled back" "now: $PREV_REPO:$PREV_TAG
was: $BEFORE_REF
schema: $(db_schema_version) — NOT rolled back, by design" \
            || die "rollback to $PREV_REPO:$PREV_TAG did not come up healthy"
        exit 0
        ;;
    auto|sha)
        step "1/6  Resolve target"
        assert_checkout_sane
        git -C "$REPO_DIR" fetch --quiet origin "$GH_BRANCH" || die "git fetch failed"
        if [[ "$MODE" == auto ]]; then
            TARGET_SHA="$(git -C "$REPO_DIR" rev-parse "origin/$GH_BRANCH")"
        else
            TARGET_SHA="$(git -C "$REPO_DIR" rev-parse "$ARG^{commit}")" || die "unknown commit $ARG"
            git -C "$REPO_DIR" merge-base --is-ancestor "$TARGET_SHA" "origin/$GH_BRANCH" \
                || die "$ARG is not an ancestor of origin/$GH_BRANCH — refusing to deploy an unmerged commit"
        fi
        TAG="sha-$(git -C "$REPO_DIR" rev-parse --short=7 "$TARGET_SHA")"
        IMAGE="$IMAGE_REPO:$TAG"
        log "target: $TARGET_SHA -> $IMAGE"
        log "        $(git -C "$REPO_DIR" log -1 --format='%s' "$TARGET_SHA")"

        if [[ "$BEFORE_REF" == "$IMAGE" && "$(git -C "$REPO_DIR" rev-parse HEAD)" == "$TARGET_SHA" ]]; then
            log "already deployed; nothing to do"
            exit 0
        fi

        step "2/6  CI gate"
        assert_ci_green "$TARGET_SHA"

        step "3/6  Pull image"
        (( DRY_RUN )) || docker pull -q "$IMAGE" || die "docker pull $IMAGE failed"
        ;;
    tag)
        step "1/6  Operator-supplied tag"
        TAG="$ARG"
        IMAGE="${QRD_DEPLOY_IMAGE_REPO_OVERRIDE:-$IMAGE_REPO}:$TAG"
        log "target: $IMAGE (no commit; CI gate and config fast-forward are SKIPPED)"
        step "2/6  CI gate"
        log "SKIPPED — an operator-supplied tag has no commit to ask GitHub about."
        step "3/6  Pull image"
        if (( ! DRY_RUN )); then
            if ! docker pull -q "$IMAGE" 2>/dev/null; then
                docker image inspect "$IMAGE" >/dev/null 2>&1 \
                    || die "$IMAGE is neither pullable nor present locally"
                log "not in the registry; using the local image (operator-supplied tags only)"
            fi
        fi
        ;;
esac

IMAGE_ID="$(docker image inspect "$IMAGE" --format '{{.Id}}' 2>/dev/null || echo unknown)"
log "image id: $IMAGE_ID"

step "4/6  Migration pre-flight"
PENDING_COUNT=0 PENDING_LIST=""
migration_preflight "$IMAGE" "$TARGET_SHA"

if (( PENDING_COUNT > 0 )); then
    TOKEN="$(approval_token "$TAG")"
    if [[ -r "$TOKEN" ]]; then
        log "APPROVED: $(cat "$TOKEN")"
        log "consuming the approval — it is valid for exactly one deploy"
        (( DRY_RUN )) || rm -f "$TOKEN"
        APPROVED_MIGRATIONS=1
    else
        notify_once "blocked-migrations-$TAG" \
            "⛔" "qr-dining deploy BLOCKED — migrations pending" \
"$IMAGE would apply $PENDING_COUNT migration(s) at boot: $PENDING_LIST
database is at: $(db_schema_version)

Schema changes are not applied unattended. docs/RECOVERY.md records that
rollback is unsafe across 16-24 and 36/38/39, so an unattended bad migration has
no way back.

Review the SQL, then on the VM:
  $REPO_DIR/deploy/vm/deploy-approve.sh $TAG"
        log "STOPPED: $PENDING_COUNT migration(s) pending ($PENDING_LIST) and no approval token at $TOKEN"
        log "Approve with: $REPO_DIR/deploy/vm/deploy-approve.sh $TAG"
        exit 0
    fi
else
    APPROVED_MIGRATIONS=0
fi

step "5/6  Config deploy (checkout fast-forward)"
if [[ -n "$TARGET_SHA" ]]; then
    apply_config "$TARGET_SHA"
else
    log "SKIPPED — no commit associated with an operator-supplied tag."
fi

step "6/6  Rolling restart"
if (( ! DRY_RUN )); then
    set_env_var IMAGE_REPO "${IMAGE%:*}"
    set_env_var IMAGE_TAG "$TAG"
fi

ROLLED=()
FAILED_AT=""
for entry in "${INSTANCES[@]}"; do
    IFS=: read -r svc container host <<<"$entry"
    if roll_instance "$svc" "$container" "$host" "$IMAGE_ID"; then
        ROLLED+=("$svc")
        continue
    fi
    FAILED_AT="$container"
    break
done

if [[ -n "$FAILED_AT" ]]; then
    step "HEALTH GATE FAILED on $FAILED_AT — rolling the BINARY back"
    if (( ${#ROLLED[@]} < ${#INSTANCES[@]} )); then
        log "instances not yet touched: $(( ${#INSTANCES[@]} - ${#ROLLED[@]} - 1 )) — they are still serving $BEFORE_REF"
    fi
    ROLLBACK_NOTE=""
    if (( APPROVED_MIGRATIONS )); then
        # The gate normally guarantees the schema did not move, which is what
        # makes an automatic binary rollback safe. An approved migration deploy
        # breaks that guarantee and a human has to know.
        ROLLBACK_NOTE="
⚠ MIGRATIONS WERE APPLIED IN THIS DEPLOY ($PENDING_LIST). The schema is now
FORWARD of the restored binary and stays there — nothing here rolls a schema
back. Verify $BEFORE_TAG tolerates schema $(db_schema_version) NOW."
    fi
    if restore_image "$BEFORE_REPO" "$BEFORE_TAG" app app2; then
        notify "🔴" "qr-dining deploy FAILED — rolled back" \
"$FAILED_AT never became ready on $IMAGE.
restored: $BEFORE_REF (healthy)
schema:   $(db_schema_version) — NOT rolled back, by design$ROLLBACK_NOTE

log: $PROJECT_DIR/deploy.log"
        die "health gate failed on $FAILED_AT; previous image restored"
    fi
    notify "🚨" "qr-dining deploy FAILED AND SO DID THE ROLLBACK" \
"$FAILED_AT never became ready on $IMAGE, and restoring $BEFORE_REF did not come
up healthy either. THE SERVICE IS DOWN. Manual intervention required now.
schema: $(db_schema_version)$ROLLBACK_NOTE"
    die "health gate failed AND rollback failed — service is down"
fi

# Both instances up on the new image. Record the rollback target only now: a
# "previous" that was never healthy is not a rollback target.
if (( ! DRY_RUN )); then
    cat > "$STATE_DIR/previous" <<EOF
PREV_REPO=$BEFORE_REPO
PREV_TAG=$BEFORE_TAG
PREV_ID=$BEFORE_ID
RECORDED_AT=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
    cat > "$STATE_DIR/current" <<EOF
REPO=${IMAGE%:*}
TAG=$TAG
IMAGE_ID=$IMAGE_ID
SHA=${TARGET_SHA:-<operator tag>}
DEPLOYED_AT=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
fi

step "Deployed"
print_status | sed 's/^/    /'

# Whatever was being suppressed is over.
notify_reset

SUBJECT=""
if [[ -n "$TARGET_SHA" ]]; then SUBJECT="$(git -C "$REPO_DIR" log -1 --format='%s' "$TARGET_SHA")"; fi
MIGRATION_NOTE=""
if (( APPROVED_MIGRATIONS )); then
    MIGRATION_NOTE="
migrations applied: $PENDING_LIST (operator-approved)"
fi

notify "✅" "qr-dining deployed" \
"$BEFORE_REF  ->  $IMAGE
commit: ${TARGET_SHA:0:7} $SUBJECT
schema: $(db_schema_version)$MIGRATION_NOTE
instances: $(for e in "${INSTANCES[@]}"; do IFS=: read -r _ c _ <<<"$e"; printf '%s=%s ' "$c" "$(container_image_ref "$c")"; done)"
