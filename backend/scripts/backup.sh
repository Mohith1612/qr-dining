#!/usr/bin/env bash
# backup.sh — PostgreSQL backup using pg_dump (custom format, compressed).
# Reads DATABASE_URL from environment. Writes to ./backups/ directory.
#
# Usage:
#   DATABASE_URL=postgres://user:pass@host:5432/dbname ./scripts/backup.sh

set -euo pipefail

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL is not set" >&2
  exit 1
fi

BACKUP_DIR="${BACKUP_DIR:-./backups}"
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
FILENAME="$BACKUP_DIR/backup_${TIMESTAMP}.dump"

echo "Starting backup → $FILENAME"

pg_dump \
  --format=custom \
  --compress=9 \
  --no-password \
  "$DATABASE_URL" \
  --file="$FILENAME"

SIZE=$(du -sh "$FILENAME" | cut -f1)
echo "Backup complete: $FILENAME ($SIZE)"

# Optional: clean up backups older than RETENTION_DAYS (default 7).
RETENTION_DAYS="${RETENTION_DAYS:-30}"
if command -v find &>/dev/null; then
  DELETED=$(find "$BACKUP_DIR" -name "backup_*.dump" -mtime +"$RETENTION_DAYS" -print -delete | wc -l)
  if [ "$DELETED" -gt 0 ]; then
    echo "Pruned $DELETED backup(s) older than ${RETENTION_DAYS} days"
  fi
fi
