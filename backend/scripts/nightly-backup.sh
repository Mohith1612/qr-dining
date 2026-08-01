#!/usr/bin/env bash
# nightly-backup.sh — pilot-grade automated PostgreSQL backup to object storage.
#
# Flow:  pg_dump (custom, compressed) -> sha256 -> upload -> manifest entry -> retention
#
# Designed to be run from cron / a systemd timer (see deploy/backup/). Exits non-zero
# on any failure so the scheduler / alerting can detect a missed backup.
#
# Required env:
#   DATABASE_URL                  postgres connection string
#   BACKUP_PROVIDER               r2 (default) | s3 | local   (+ its provider env, see lib/storage.sh)
# Optional env:
#   BACKUP_PREFIX                 key prefix in the bucket            (default: backups)
#   RETENTION_DAYS                delete backups older than N days    (default: 14)
#   BACKUP_WORKDIR                local scratch dir for the dump      (default: ./backups)
#
# Bucket layout:
#   <prefix>/YYYY/MM/backup_<YYYYMMDD_HHMMSS>.dump      one dump per run
#   <prefix>/manifest.jsonl                            append-only index of all backups

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/storage.sh
source "$SCRIPT_DIR/lib/storage.sh"

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL is not set" >&2
  exit 1
fi

BACKUP_PREFIX="${BACKUP_PREFIX:-backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
BACKUP_WORKDIR="${BACKUP_WORKDIR:-./backups}"
mkdir -p "$BACKUP_WORKDIR"

# Emit Prometheus metrics for the node_exporter textfile collector so the
# BackupFailed / BackupTooOld alerts have a signal. No-op if the dir is unset.
# Status and last-success are separate files so a failure (trap) does not erase
# the last successful timestamp.
write_status_metric() { # $1=status(0|1)
  [ -n "${NODE_EXPORTER_TEXTFILE_DIR:-}" ] || return 0
  local f="$NODE_EXPORTER_TEXTFILE_DIR/qr_dining_backup_status.prom"
  {
    echo "# HELP qr_dining_backup_last_run_status Last backup run status (0=ok,1=failed)."
    echo "# TYPE qr_dining_backup_last_run_status gauge"
    echo "qr_dining_backup_last_run_status $1"
  } > "$f.tmp" && mv "$f.tmp" "$f"
}
write_success_metric() {
  [ -n "${NODE_EXPORTER_TEXTFILE_DIR:-}" ] || return 0
  local f="$NODE_EXPORTER_TEXTFILE_DIR/qr_dining_backup_success.prom"
  {
    echo "# HELP qr_dining_backup_last_success_timestamp_seconds Unix time of last successful backup."
    echo "# TYPE qr_dining_backup_last_success_timestamp_seconds gauge"
    echo "qr_dining_backup_last_success_timestamp_seconds $(date +%s)"
  } > "$f.tmp" && mv "$f.tmp" "$f"
}
trap 'write_status_metric 1' ERR

storage_init

TIMESTAMP="$(date -u +%Y%m%d_%H%M%S)"
YEAR="${TIMESTAMP:0:4}"
MONTH="${TIMESTAMP:4:2}"
FILENAME="backup_${TIMESTAMP}.dump"
LOCAL_FILE="$BACKUP_WORKDIR/$FILENAME"
REMOTE_KEY="$BACKUP_PREFIX/$YEAR/$MONTH/$FILENAME"

echo "[1/5] pg_dump -> $LOCAL_FILE"
pg_dump --format=custom --compress=9 --no-password "$DATABASE_URL" --file="$LOCAL_FILE"
BYTES="$(stat -c %s "$LOCAL_FILE" 2>/dev/null || stat -f %z "$LOCAL_FILE")"
echo "      dump size: $BYTES bytes"

echo "[2/5] checksum"
SHA256="$(sha256sum "$LOCAL_FILE" | awk '{print $1}')"
echo "      sha256: $SHA256"

echo "[3/5] upload -> $REMOTE_KEY"
storage_upload "$LOCAL_FILE" "$REMOTE_KEY"

echo "[4/5] manifest entry"
MANIFEST_KEY="$BACKUP_PREFIX/manifest.jsonl"
MANIFEST_TMP="$(mktemp)"
# Pull the existing manifest if present; start fresh otherwise.
storage_download "$MANIFEST_KEY" "$MANIFEST_TMP" 2>/dev/null || : > "$MANIFEST_TMP"
ENTRY="$(printf '{"timestamp":"%s","key":"%s","bytes":%s,"sha256":"%s","provider":"%s","retention_days":%s}' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$REMOTE_KEY" "$BYTES" "$SHA256" "$BACKUP_PROVIDER" "$RETENTION_DAYS")"
echo "$ENTRY" >> "$MANIFEST_TMP"
storage_upload "$MANIFEST_TMP" "$MANIFEST_KEY"
cp "$MANIFEST_TMP" "$BACKUP_WORKDIR/manifest.jsonl"
rm -f "$MANIFEST_TMP"
echo "      appended: $ENTRY"

echo "[5/5] retention (>$RETENTION_DAYS days)"
# Epoch math keeps this portable across GNU (-d @), BusyBox (-d @), and BSD (-r)
# date — the GNU-only "-N days" form broke when run inside alpine containers.
CUTOFF_EPOCH="$(( $(date -u +%s) - RETENTION_DAYS * 86400 ))"
CUTOFF="$(date -u -d "@${CUTOFF_EPOCH}" +%Y%m%d 2>/dev/null || date -u -r "${CUTOFF_EPOCH}" +%Y%m%d)"
PRUNED=0
while IFS= read -r key; do
  [ -z "$key" ] && continue
  base="$(basename "$key")"
  # Extract YYYYMMDD from backup_YYYYMMDD_HHMMSS.dump
  date_part="$(echo "$base" | sed -n 's/^backup_\([0-9]\{8\}\)_.*/\1/p')"
  [ -z "$date_part" ] && continue
  if [ "$date_part" -lt "$CUTOFF" ]; then
    storage_delete "$key" && PRUNED=$((PRUNED+1))
    echo "      pruned remote: $key"
  fi
done < <(storage_list "$BACKUP_PREFIX/")
# Prune local scratch copies too.
find "$BACKUP_WORKDIR" -name 'backup_*.dump' -mtime +"$RETENTION_DAYS" -print -delete 2>/dev/null | sed 's/^/      pruned local: /' || true
echo "      remote pruned: $PRUNED"

write_status_metric 0
write_success_metric
echo "OK: backup complete -> $REMOTE_KEY ($BYTES bytes)"
