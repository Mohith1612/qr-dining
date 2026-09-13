#!/usr/bin/env bash
# DEPRECATED — superseded by nightly-backup.sh + lib/storage.sh (provider
# abstraction, manifest, retention, metrics). Kept only for ad-hoc uploads.
#
# backup_r2.sh — Upload a backup file to Cloudflare R2 (S3-compatible).
# Run after backup.sh. Uses the AWS CLI with R2 endpoint configuration.
#
# Required env vars:
#   R2_ENDPOINT    — https://<account_id>.r2.cloudflarestorage.com
#   R2_BUCKET      — bucket name (always from env; bucket names are not permanent)
#   R2_ACCESS_KEY  — R2 Access Key ID
#   R2_SECRET_KEY  — R2 Secret Access Key
#
# Usage:
#   ./scripts/backup.sh   # create backup first
#   ./scripts/backup_r2.sh ./backups/backup_20260517_120000.dump

set -euo pipefail

for var in R2_ENDPOINT R2_BUCKET R2_ACCESS_KEY R2_SECRET_KEY; do
  if [ -z "${!var:-}" ]; then
    echo "ERROR: $var is not set" >&2
    exit 1
  fi
done

BACKUP_FILE="${1:-}"
if [ -z "$BACKUP_FILE" ]; then
  echo "Usage: $0 <backup_file.dump>" >&2
  exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
  echo "ERROR: Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

FILENAME=$(basename "$BACKUP_FILE")
S3_KEY="backups/$FILENAME"

echo "Uploading $FILENAME to R2 bucket $R2_BUCKET..."

AWS_ACCESS_KEY_ID="$R2_ACCESS_KEY" \
AWS_SECRET_ACCESS_KEY="$R2_SECRET_KEY" \
aws s3 cp \
  "$BACKUP_FILE" \
  "s3://${R2_BUCKET}/${S3_KEY}" \
  --endpoint-url "$R2_ENDPOINT" \
  --region auto

echo "Upload complete: s3://${R2_BUCKET}/${S3_KEY}"
