#!/usr/bin/env bash
#
# Approve ONE deploy that would apply schema migrations.
#
# deploy.sh stops rather than apply migrations unattended, because a bad
# migration has no way back: docs/RECOVERY.md records that rollback is not safe
# across 16-24 or 36/38/39, and that migration 22 wedges outright on a populated
# audit_log with recovery requiring `migrate force` plus dropping a trigger by
# hand. Reverting a binary is a `docker compose up`. Reverting a schema is an
# incident with a restaurant in it.
#
# This is the human in that loop. It writes a token that deploy.sh consumes
# exactly once, for exactly the tag named. It does not deploy anything.
#
#   deploy-approve.sh sha-a1b2c3d
#   deploy-approve.sh --show
#   deploy-approve.sh --revoke sha-a1b2c3d
#
set -Eeuo pipefail

PROJECT_DIR="${QRD_PROJECT_DIR:-/opt/qr-dining}"
if [[ -r "$PROJECT_DIR/deploy.env" ]]; then . "$PROJECT_DIR/deploy.env"; fi
STATE_DIR="${QRD_STATE_DIR:-$PROJECT_DIR/deploy-state}"
REPO_DIR="${QRD_REPO_DIR:-$PROJECT_DIR/repo}"
IMAGE_REPO="${QRD_IMAGE_REPO:-ghcr.io/mohith1612/qr-dining}"
ENV_FILE="${QRD_ENV_FILE:-$PROJECT_DIR/.env}"
INTERNAL_NET="${QRD_INTERNAL_NET:-qr-dining_qr-dining_internal}"

mkdir -p "$STATE_DIR"

case "${1:-}" in
    "")
        sed -n '2,20p' "$0"; exit 2 ;;
    -h|--help)
        sed -n '2,20p' "$0"; exit 0 ;;
    --show)
        shopt -s nullglob
        found=0
        for f in "$STATE_DIR"/approved-*; do
            found=1; printf '%s\n' "--- ${f##*/approved-}"; sed 's/^/    /' "$f"
        done
        (( found )) || echo "no outstanding approvals"
        exit 0 ;;
    --revoke)
        tag="${2:?--revoke needs a tag}"
        rm -f -- "$STATE_DIR/approved-$tag" && echo "revoked $tag"; exit 0 ;;
esac

TAG="$1"
IMAGE="$IMAGE_REPO:$TAG"
TOKEN="$STATE_DIR/approved-$TAG"

docker image inspect "$IMAGE" >/dev/null 2>&1 || docker pull -q "$IMAGE" \
    || { echo "cannot obtain $IMAGE" >&2; exit 1; }

echo "== What this image would do to the schema"
set +e
docker run --rm --network "$INTERNAL_NET" --env-file "$ENV_FILE" \
    --entrypoint /app/migrate "$IMAGE" pending 2>&1 | sed 's/^/    /'
rc=${PIPESTATUS[0]}
set -e
if (( rc != 10 )); then
    echo
    echo "Nothing to approve: the pre-flight exited $rc, not 10 (pending)."
    echo "Approving a deploy that is not blocked would leave a token lying around"
    echo "that silently authorises a LATER schema change on this tag. Not doing that."
    exit 1
fi

# Show the SQL. An approval given without reading the migration is not an
# approval, it is a rubber stamp with an audit trail.
if [[ -d "$REPO_DIR" ]]; then
    DB_V="$(docker run --rm --network "$INTERNAL_NET" --env-file "$ENV_FILE" \
        --entrypoint /app/migrate "$IMAGE" pending 2>/dev/null | sed -nE 's/^db_version=([0-9]+)$/\1/p')"
    echo
    echo "== The SQL you are approving (backend/migrations, above version ${DB_V:-?})"
    for f in "$REPO_DIR"/backend/migrations/*.up.sql; do
        n="$(basename "$f")"; v="$((10#${n%%_*}))"
        (( v > ${DB_V:-0} )) || continue
        echo; echo "--- $n"; sed 's/^/    /' "$f"
    done
fi

cat <<EOF

== Read this before answering
Schema rollback is NOT available. docs/RECOVERY.md:
  * migrations 16-24 and 36/38/39 destroy or fail to restore durable meaning
  * migration 22 wedges on a populated audit_log
  * migration 40 raises if a session already has multiple non-terminal payments
If one of these fails part-way, schema_migrations goes dirty and every instance
refuses to boot until a human fixes it by hand.

Take a backup first if you have not today:  $PROJECT_DIR/run-backup.sh
EOF

read -r -p "Type the tag ($TAG) to approve, anything else to abort: " answer
[[ "$answer" == "$TAG" ]] || { echo "aborted; nothing written"; exit 1; }

cat > "$TOKEN" <<EOF
tag=$TAG
image=$IMAGE
approved_by=$(id -un)@$(hostname)
approved_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
echo
echo "Approved. Written to $TOKEN."
echo "It is consumed by the next deploy of $TAG and by no other. The following"
echo "cron tick (within ~2 minutes) will pick it up; nothing further is needed."
