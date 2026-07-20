#!/usr/bin/env bash
# storage.sh — backup storage provider abstraction.
#
# Source this file, call storage_init, then use the storage_* functions. The
# provider is selected by BACKUP_PROVIDER:
#
#   r2     (default) — Cloudflare R2 via the S3-compatible API (aws CLI).
#   s3     — AWS S3 or any S3-compatible store via the aws CLI.
#   local  — copy into a local directory (for testing the pipeline without cloud).
#
# Both r2 and s3 use the same `aws s3` code path; R2 just adds --endpoint-url and
# --region auto. Adding another S3-compatible provider is a matter of setting the
# endpoint — no new code path. This keeps the system pilot-simple while remaining
# portable.
#
# Required env per provider:
#   r2:    R2_ENDPOINT, R2_BUCKET, R2_ACCESS_KEY, R2_SECRET_KEY
#   s3:    S3_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY [, S3_ENDPOINT_URL, AWS_REGION]
#   local: BACKUP_LOCAL_DIR

BACKUP_PROVIDER="${BACKUP_PROVIDER:-r2}"

# Internal: populated by storage_init.
_STORAGE_BUCKET=""
_STORAGE_AWS_ARGS=()

storage_init() {
  case "$BACKUP_PROVIDER" in
    r2)
      _storage_require R2_ENDPOINT R2_BUCKET R2_ACCESS_KEY R2_SECRET_KEY
      _storage_require_cmd aws
      _STORAGE_BUCKET="$R2_BUCKET"
      _STORAGE_AWS_ARGS=(--endpoint-url "$R2_ENDPOINT" --region auto)
      export AWS_ACCESS_KEY_ID="$R2_ACCESS_KEY"
      export AWS_SECRET_ACCESS_KEY="$R2_SECRET_KEY"
      ;;
    s3)
      _storage_require S3_BUCKET AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
      _storage_require_cmd aws
      _STORAGE_BUCKET="$S3_BUCKET"
      _STORAGE_AWS_ARGS=(--region "${AWS_REGION:-us-east-1}")
      if [ -n "${S3_ENDPOINT_URL:-}" ]; then
        _STORAGE_AWS_ARGS+=(--endpoint-url "$S3_ENDPOINT_URL")
      fi
      ;;
    local)
      _storage_require BACKUP_LOCAL_DIR
      mkdir -p "$BACKUP_LOCAL_DIR"
      ;;
    *)
      echo "ERROR: unknown BACKUP_PROVIDER=$BACKUP_PROVIDER (expected r2|s3|local)" >&2
      return 1
      ;;
  esac
  echo "storage: provider=$BACKUP_PROVIDER target=${_STORAGE_BUCKET:-$BACKUP_LOCAL_DIR}"
}

# storage_upload <local_file> <remote_key>
storage_upload() {
  local src="$1" key="$2"
  if [ "$BACKUP_PROVIDER" = "local" ]; then
    local dest="$BACKUP_LOCAL_DIR/$key"
    mkdir -p "$(dirname "$dest")"
    cp "$src" "$dest"
  else
    aws s3 cp "$src" "s3://${_STORAGE_BUCKET}/${key}" "${_STORAGE_AWS_ARGS[@]}"
  fi
}

# storage_download <remote_key> <local_file>  (returns non-zero if missing)
storage_download() {
  local key="$1" dest="$2"
  if [ "$BACKUP_PROVIDER" = "local" ]; then
    [ -f "$BACKUP_LOCAL_DIR/$key" ] || return 1
    cp "$BACKUP_LOCAL_DIR/$key" "$dest"
  else
    aws s3 cp "s3://${_STORAGE_BUCKET}/${key}" "$dest" "${_STORAGE_AWS_ARGS[@]}"
  fi
}

# storage_list <prefix>  — prints one remote key per line (recursive).
storage_list() {
  local prefix="$1"
  if [ "$BACKUP_PROVIDER" = "local" ]; then
    ( cd "$BACKUP_LOCAL_DIR" 2>/dev/null && find "$prefix" -type f 2>/dev/null ) || true
  else
    aws s3 ls "s3://${_STORAGE_BUCKET}/${prefix}" --recursive "${_STORAGE_AWS_ARGS[@]}" 2>/dev/null \
      | awk '{ $1=""; $2=""; $3=""; sub(/^ +/, ""); print }'
  fi
}

# storage_delete <remote_key>
storage_delete() {
  local key="$1"
  if [ "$BACKUP_PROVIDER" = "local" ]; then
    rm -f "$BACKUP_LOCAL_DIR/$key"
  else
    aws s3 rm "s3://${_STORAGE_BUCKET}/${key}" "${_STORAGE_AWS_ARGS[@]}"
  fi
}

_storage_require() {
  local missing=0
  for var in "$@"; do
    if [ -z "${!var:-}" ]; then
      echo "ERROR: $var is required for BACKUP_PROVIDER=$BACKUP_PROVIDER" >&2
      missing=1
    fi
  done
  [ "$missing" -eq 0 ] || return 1
}

_storage_require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command '$1' not found in PATH" >&2
    return 1
  fi
}
