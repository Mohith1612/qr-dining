#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ] || [ ! -s "$1" ]; then
  echo "usage: $0 <non-empty backup-restore log>" >&2
  exit 2
fi

if grep -q '^SKIP:' "$1"; then
  echo "Backup/restore drill reported SKIP; CI requires a real drill" >&2
  exit 1
fi

if ! grep -Eq '[0-9]+ passed, 0 failed' "$1"; then
  echo "Backup/restore drill did not complete with zero failures" >&2
  exit 1
fi

echo "Backup/restore drill ran and completed with zero failures"
