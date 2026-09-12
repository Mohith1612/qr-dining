#!/usr/bin/env bash
# restore.sh — Restore a PostgreSQL backup produced by nightly-backup.sh (or backup.sh).
#
# Usage:
#   DATABASE_URL=postgres://user:pass@host:5432/dbname ./scripts/restore.sh [options] <backup.dump>
#
# Options:
#   -y, --yes           Skip the interactive confirmation. Required for drills,
#                       cron and CI, which have no terminal to answer the prompt.
#   --preserve-owner    Restore original object ownership and privileges
#                       (pg_restore default). See "Ownership" below — you almost
#                       never want this.
#   -h, --help          Show this help.
#
# Environment:
#   DATABASE_URL  (required)  target DSN — a scratch DB, never production blindly.
#   FORCE=1                   same as --yes.
#
# Ownership: this script passes --no-owner --no-privileges by default. The dump
# records "ALTER ... OWNER TO <role>" for the role that owned the objects on the
# source server. If that role does not exist on the target, pg_restore errors and
# --single-transaction rolls the whole thing back, so NOTHING is restored — which
# is exactly the rebuilt-host case you need a restore for. Restored objects are
# owned by the connecting role instead, which for this single-role schema is the
# same end state. Use --preserve-owner only when restoring onto a server that
# already has the original roles and you need the exact grants back.
#
# WARNING: This replaces all data in the target database. Back up before restoring.

set -euo pipefail

usage() {
  sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
}

ASSUME_YES="${FORCE:-0}"
PRESERVE_OWNER=0
BACKUP_FILE=""

while [ $# -gt 0 ]; do
  case "$1" in
    -y|--yes)        ASSUME_YES=1 ;;
    --preserve-owner) PRESERVE_OWNER=1 ;;
    -h|--help)       usage; exit 0 ;;
    --)              shift; break ;;
    -*)              echo "ERROR: unknown option: $1" >&2; usage >&2; exit 2 ;;
    *)
      if [ -n "$BACKUP_FILE" ]; then
        echo "ERROR: unexpected extra argument: $1" >&2
        exit 2
      fi
      BACKUP_FILE="$1"
      ;;
  esac
  shift
done
if [ -z "$BACKUP_FILE" ] && [ $# -gt 0 ]; then
  BACKUP_FILE="$1"
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL is not set" >&2
  exit 1
fi

if [ -z "$BACKUP_FILE" ]; then
  echo "ERROR: no backup file given" >&2
  usage >&2
  exit 2
fi

if [ ! -f "$BACKUP_FILE" ]; then
  echo "ERROR: Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

OWNERSHIP_ARGS=(--no-owner --no-privileges)
OWNERSHIP_NOTE="objects will be owned by the connecting role (--no-owner --no-privileges)"
if [ "$PRESERVE_OWNER" = "1" ]; then
  OWNERSHIP_ARGS=()
  OWNERSHIP_NOTE="preserving original ownership/privileges — the dump's roles MUST exist on the target"
fi

echo "Restoring from: $BACKUP_FILE"
echo "Target: $DATABASE_URL"
echo "Ownership: $OWNERSHIP_NOTE"
echo ""

if [ "$ASSUME_YES" = "1" ]; then
  echo "Proceeding without confirmation (--yes)."
else
  echo "WARNING: This will replace all data in the target database."
  # `echo yes | restore.sh ...` still works. With no stdin at all — cron, a
  # systemd unit, CI — read hits EOF; say so instead of exiting silently, which
  # is what the bare `read` used to do under `set -e`.
  if ! read -r -p "Type 'yes' to confirm: " CONFIRM; then
    echo "" >&2
    echo "ERROR: no confirmation available on stdin (not a terminal)." >&2
    echo "       Re-run with --yes (or FORCE=1) if this is a drill or an automated restore." >&2
    exit 1
  fi
  if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 1
  fi
fi

echo "Restoring..."
pg_restore \
  --format=custom \
  --no-password \
  --clean \
  --if-exists \
  --single-transaction \
  "${OWNERSHIP_ARGS[@]+"${OWNERSHIP_ARGS[@]}"}" \
  --dbname="$DATABASE_URL" \
  "$BACKUP_FILE"

echo ""
echo "Restore complete. Validating row counts:"

psql "$DATABASE_URL" --no-password --tuples-only <<'SQL'
SELECT
  'sessions'              AS table_name, COUNT(*) AS rows FROM sessions
UNION ALL SELECT 'orders',              COUNT(*) FROM orders
UNION ALL SELECT 'payments',            COUNT(*) FROM payments
UNION ALL SELECT 'staff',               COUNT(*) FROM staff
UNION ALL SELECT 'menu_items',          COUNT(*) FROM menu_items
UNION ALL SELECT 'event_log',           COUNT(*) FROM event_log;
SQL

echo ""
echo "Migration state (a restored DB must not be dirty, and the running binary"
echo "must expect exactly this version — see docs/RUNBOOKS.md §8):"
psql "$DATABASE_URL" --no-password --tuples-only \
  -c "SELECT 'schema_migrations version=' || version || ' dirty=' || dirty FROM schema_migrations;"
