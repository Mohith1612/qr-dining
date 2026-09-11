# Testing

What exists, what runs where, and what a green result actually proves.

Companions: [DEVELOPMENT.md](DEVELOPMENT.md) · [RELEASE.md](RELEASE.md) · [SECURITY.md](SECURITY.md).

---

## 1. Test layers

| Layer | Location | Runs in CI |
|---|---|---|
| Go unit tests | `backend/**/*_test.go` | Yes, with `-race` |
| Go integration tests | `backend/**` behind `//go:build integration` | Yes, with `-race`, real Postgres + Redis services |
| Migration up/down/up | `backend/migrations/` | Yes |
| sqlc drift | `backend/internal/db/sqlc/` | Yes |
| OpenAPI lint | `openapi.yaml` | Yes |
| Frontend lint + build | `frontend/` | Yes |
| Marketing build | `marketing/` | Yes |
| Playwright e2e (106 specs) | `e2e/` | **No — discovery only** |
| Chaos experiments | `scripts/chaos/` | No — manual |
| Manual certification | `docs/manual-testing/` | No — human |

## 2. Backend tests locally

```bash
cd backend

# Unit only. This SKIPS every integration test — a green result proves little.
make test

# Full suite. -p 1 is mandatory, not tuning: all packages share one database and
# one Redis, so parallel package binaries truncate each other's fixtures and fail
# with deadlocks and FK violations that are isolation artifacts, not product bugs.
TEST_DATABASE_URL=postgres://user:pass@localhost:5432/throwaway \
TEST_REDIS_URL=redis://localhost:6379/0 \
  go test -count=1 -p 1 -race -tags integration ./...
```

Integration tests skip silently when `TEST_DATABASE_URL` is unset, and the Redis/WebSocket-ticket tests skip when `TEST_REDIS_URL` is unset. They run migrations against the target database on startup, so point them at a **throwaway** database — never a soak or production one.

## 3. End-to-end (Playwright)

106 specs under `e2e/`, organised by area: `guest`, `staff`, `platform`, `session`, `order`, `payment`, `realtime`, `multidevice`, `tenancy`, `webhook`, `audit`, `adversarial`, `operational-ids`, `frontend`, `screenshots`.

```bash
cd e2e
npm ci
npm run install-browsers
npm test                  # all projects
npm run test:desktop      # or --project=mobile
npm run test:headed
```

They run against a live stack — use the isolated manual-testing stack (`./scripts/manual-testing-up.sh`), not your dev datastores.

> **CI does not execute these.** The `e2e-typecheck` job runs `playwright test --list`, which only validates the config and proves the specs are discoverable and compile. A green Frontend CI run says nothing about whether any e2e assertion passes. Run them locally before a release gate.

## 4. Chaos experiments

```bash
./scripts/chaos/run-all.sh
```

Fault-injection scripts (Redis flap, backend restart, nginx reload, webhook replay) that bring prerequisites up, inject the fault, capture metric scrapes and container logs to `scripts/chaos/results/<timestamp>-<exp>/`, and print PASS/FAIL. See [../scripts/chaos/README.md](../scripts/chaos/README.md). Manual only — never wired into CI.

## 5. What CI actually runs

Two workflows, both triggered **only** on push to `main`, `v*` tags, PRs into `main`, or `workflow_dispatch`. A feature branch that is never PR'd into `main` gets **no CI at all**.

### `.github/workflows/ci.yml` — backend

| Job | What it does |
|---|---|
| `lint` | `gofmt -l`, `go vet`, `golangci-lint` pinned to **v2.12.2** |
| `test` | `go mod verify`, `go build ./...`, `go test -count=1 -race ./...` (unit only) |
| `govulncheck` | `golang.org/x/vuln/cmd/govulncheck@v1.6.0`, source mode. Fails only on a vulnerable symbol reachable from this module's call graph |
| `sqlc-check` | `go tool sqlc generate` then fails on any diff under `internal/db/sqlc/` |
| `migrations` | Against Postgres 17: `up` → `down -all` → `up` |
| `integration` | Postgres 17 + Redis 7 services, `go test -count=1 -p 1 -race -tags integration ./...` |
| `openapi` | `@redocly/cli@2.43.3 lint openapi.yaml` using the committed `redocly.yaml` |
| `docker-build` | `linux/arm64` build via QEMU. Pushes to `ghcr.io/mohith1612/qr-dining` on `main` pushes and `v*` tags; PRs build only |

Both the golangci-lint action major and the linter version are pinned deliberately: an RC has to lint identically on a re-run months later, and `version: latest` silently moves the gate under a rebuild of the same commit.

`govulncheck`'s Go version is deliberately the same `"1.26"` spec used by the release Dockerfile rather than an exact patch pin, so the scan matches the artifact that actually ships.

### `.github/workflows/frontend-ci.yml` — frontend

Path-filtered to `frontend/**`, `marketing/**`, `e2e/**`.

| Job | What it does |
|---|---|
| `frontend` | `npm ci`, `npm run lint`, `npm run build` with `ALLOW_LOCALHOST_BUILD=true` |
| `marketing` | `npm ci`, `npm run build` (tsc + astro check + astro build) |
| `e2e-typecheck` | `playwright test --list` — **discovery only, no execution** |

### What CI does not cover

- **Playwright assertions** — discovery only (above).
- **Frontend typecheck** — `npm run typecheck` exists but no job calls it; `next build` type-checks the app but not the whole tree.
- **OpenAPI ↔ route agreement** — the lint job proves the spec is *valid*, not that it still matches the server's routes.
- **Chaos, load, soak** — all manual.
- **Webhook exact-replay proof** — deferred, and it gates rollout wave R7.

## 6. Manual certification

The human gate before a release freeze. Materials live in `docs/manual-testing/`:

| File | Role |
|---|---|
| `testing-dashboard.html` | **Entry point** — environments, tenants, QR links, tester credentials |
| `manual-testing-user-guide.html` | Teaching manual: what each surface is meant to do |
| `manual-testing-checklist.html` | The run sheet you tick |
| `manual-testing-session-guide.html` | How to run a session |
| `how-to-use.html` | Orientation |
| `database-reference.html` | Schema reference for verification queries |
| `audit-added-checks-2026-08-04.md` | Root-cause companion for the audit-derived checks |

Scope, limits and attention plan: [../release-certification/rc-manual-certification-plan.md](../release-certification/rc-manual-certification-plan.md).
Current run and its preconditions: [../release-certification/manual-certification-preflight-2026-08-22.md](../release-certification/manual-certification-preflight-2026-08-22.md).

Where certification sits in the release sequence: [RELEASE.md](RELEASE.md).
