#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ] || [ ! -s "$1" ]; then
  echo "usage: $0 <non-empty go-test-json-file>" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
results_file="$1"

shopt -s nullglob
guard_files=("$repo_root"/backend/internal/handlers/credential_guard_*_test.go)
if [ "${#guard_files[@]}" -lt 4 ]; then
  echo "credential guard suite is incomplete: found ${#guard_files[@]} file(s), expected at least 4" >&2
  exit 1
fi

mapfile -t required_tests < <(
  sed -nE 's/^func (Test[^ (]+)\(.*/\1/p' \
    "${guard_files[@]}"
)
if [ "${#required_tests[@]}" -lt 16 ]; then
  echo "credential guard suite is incomplete: found ${#required_tests[@]} test(s), expected at least 16" >&2
  exit 1
fi

# These prove that the integration-tagged PostgreSQL tests, Redis tests, and
# the two incident-specific wire guards executed instead of calling t.Skip.
required_tests+=(
  TestListActiveSessions_CarriesNoCredential
  TestForceCloseSession_CarriesNoCredential
  TestWSTicketStoreConsumeOnce
)

missing=0
for test_name in "${required_tests[@]}"; do
  if ! jq --slurp --exit-status --arg test_name "$test_name" \
    'any(.[]; .Action == "pass" and .Test == $test_name)' \
    "$results_file" >/dev/null; then
    echo "required test did not pass: $test_name" >&2
    missing=$((missing + 1))
  fi
done

if [ "$missing" -ne 0 ]; then
  echo "$missing required integration/credential test(s) did not run and pass" >&2
  exit 1
fi

echo "Required DB, Redis, and ${#required_tests[@]} credential/sentinel tests ran and passed"
