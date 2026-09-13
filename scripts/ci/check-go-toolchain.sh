#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"

fail() {
  echo "Go toolchain pin check failed: $*" >&2
  exit 1
}

module_version="$(awk '$1 == "toolchain" { sub(/^go/, "", $2); print $2 }' "$repo_root/backend/go.mod")"
[ -n "$module_version" ] || fail "backend/go.mod has no toolchain directive"

mapfile -t docker_versions < <(
  sed -nE 's/^FROM .*golang:([0-9]+\.[0-9]+\.[0-9]+)-.*/\1/p' \
    "$repo_root/backend/docker/Dockerfile"
)
[ "${#docker_versions[@]}" -gt 0 ] || fail "backend/docker/Dockerfile has no pinned golang:X.Y.Z image"

mapfile -t workflow_versions < <(
  sed -nE 's/^[[:space:]]+go-version:[[:space:]]*"?([0-9]+\.[0-9]+\.[0-9]+)"?.*/\1/p' \
    "$repo_root"/.github/workflows/*.yml
)
[ "${#workflow_versions[@]}" -gt 0 ] || fail "workflows have no exact go-version pins"

for version in "${docker_versions[@]}"; do
  [ "$version" = "$module_version" ] || fail "go.mod=$module_version but Dockerfile=$version"
done

for version in "${workflow_versions[@]}"; do
  [ "$version" = "$module_version" ] || fail "go.mod=$module_version but a workflow uses $version"
done

runtime_version="$(cd "$repo_root/backend" && go env GOVERSION)"
[ "$runtime_version" = "go$module_version" ] || \
  fail "go.mod=$module_version but the active backend toolchain is $runtime_version"

echo "Go toolchain pins agree: go.mod, Dockerfile, workflows, and runtime use $module_version"
