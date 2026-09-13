#!/usr/bin/env bash
# preflight.sh — deterministic checks, each independent, output kept for the record
root=$(git rev-parse --show-toplevel)
mkdir -p "$root/audit"

run() {
  local name=$1; shift
  echo "--- $name"
  ( "$@" ) > "$root/audit/$name.log" 2>&1
  echo "    exit $? -> audit/$name.log"
}

cd "$root/backend"
run go-build   go build ./...
run go-vet     go vet ./...
run go-test    go test ./... -race -count=1
run staticcheck staticcheck ./...
run govulncheck govulncheck ./...
run gosec      gosec -exclude-generated ./...
run golangci   golangci-lint run --timeout 5m
run sqlc-vet   sqlc vet

cd "$root/frontend"
run tsc        npx tsc --noEmit
run npm-audit-fe npm audit --omit=dev

cd "$root/marketing"
run npm-audit-mkt npm audit --omit=dev
