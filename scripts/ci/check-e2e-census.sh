#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
mapfile -d '' spec_files < <(find "$repo_root/e2e" -type f -name '*.spec.ts' -print0)

[ "${#spec_files[@]}" -gt 0 ] || { echo "No Playwright spec files found" >&2; exit 1; }

executing_count="$(grep -hEc '^\s*test\s*\(' "${spec_files[@]}" | awk '{ total += $1 } END { print total + 0 }')"
fixme_count="$(grep -hEc '^\s*test\.fixme\s*\(' "${spec_files[@]}" | awk '{ total += $1 } END { print total + 0 }')"
vacuous_count="$(grep -hEc 'VACUOUS\(sig-[0-9]+\)' "${spec_files[@]}" | awk '{ total += $1 } END { print total + 0 }')"

if [ "$executing_count" -ne 75 ] || [ "$fixme_count" -ne 59 ] || [ "$vacuous_count" -ne 59 ]; then
  echo "E2E quarantine census changed: executing=$executing_count fixme=$fixme_count VACUOUS=$vacuous_count" >&2
  echo "Expected exactly 75 executing declarations and 59 VACUOUS test.fixme declarations." >&2
  echo "Repair and re-audit a quarantine before un-skipping it; do not hide failures with a new skip." >&2
  exit 1
fi

echo "E2E quarantine census is pinned: 75 executing, 59 VACUOUS test.fixme"
