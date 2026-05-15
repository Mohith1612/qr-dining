#!/usr/bin/env bash
# restore.sh — Restore a PostgreSQL backup created by backup.sh.
# Reads DATABASE_URL from environment. Accepts backup file as first argument.
#
# Usage:
#   DATABASE_URL=postgres://user:pass@host:5432/dbname ./scripts/restore.sh ./backups/backup_20260517_120000.dump
#
# WARNING: This will DROP and recreate the target database. Back up production before restoring.

set -euo pipefail

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL is not set" >&2
  exit 1
fi

BACKUP_FILE="${1:-}"
if [ -z "$BACKUP_FILE" ]; then
  echo "Usage: $0 <backup_file.dump>" >&2
  exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
  echo "ERROR: Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

echo "Restoring from: $BACKUP_FILE"
echo "Target: $DATABASE_URL"
echo ""
echo "WARNING: This will replace all data in the target database."
read -r -p "Type 'yes' to confirm: " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
  echo "Aborted."
  exit 1
fi

echo "Restoring..."
pg_restore \
  --format=custom \
  --no-password \
  --clean \
  --if-exists \
  --single-transaction \
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
