# Development

Last verified against the repository: 2026-09-13.

## Prerequisites

Use Go 1.26 language mode with the pinned 1.26.8 toolchain
(`backend/go.mod:1-5`). The application stack uses PostgreSQL 17 and Redis 7
(`docker-compose.yml:15-51`). Frontend and marketing builds use npm; CI installs
the current Node LTS and dependencies with `npm ci`
(`.github/workflows/frontend-ci.yml:18-57`). Docker is additionally required for
the isolated manual-testing and backup/restore harnesses
(`scripts/manual-testing-up.sh:1-16`,
`backend/scripts/tests/backup-restore-test.sh:12-18`).

## Backend

Copy `backend/.env.example` to `backend/.env` and replace its placeholder database
credentials and blank secrets. It declares PostgreSQL, Redis, server, logging,
CORS, guest-token, MFA, webhook, rate-limit, worker, and trusted-proxy settings
(`backend/.env.example:1-52`). Release mode refuses an empty CORS allowlist and
requires non-placeholder guest credentials
(`backend/internal/config/config.go:286-319`).

For a native backend, change `DATABASE_URL` and `REDIS_URL` in `backend/.env` to
use `localhost` instead of the Compose service names. Pass that file to Compose
for its PostgreSQL interpolation, then migrate, optionally seed, and run
(`backend/.env.example:1-8`, `docker-compose.yml:16-50`):

```bash
docker compose --env-file backend/.env up -d postgres redis
cd backend
go run ./cmd/migrate up
go run ./scripts/seed.go
go run ./cmd/server
```

The backend run, migration, and seed commands have matching Make targets
(`backend/Makefile:6-7,12-31`). Migration
commands require `DATABASE_URL` and support `up`, `down [N]`, `version`, and
`force <version>` (`backend/cmd/migrate/main.go:22-35,42-85`). Do not use `down`
against durable data; see [RECOVERY.md](RECOVERY.md).

Generate database bindings after changing a query or migration:

```bash
cd backend
go tool sqlc generate
git diff --exit-code internal/db/sqlc/
```

CI uses those commands as a drift gate (`.github/workflows/ci.yml:99-123`).

## Frontend and marketing

```bash
cd frontend
npm ci
npm run dev
```

The frontend provides `dev`, `build`, Cloudflare build/preview/deploy, lint, and
typecheck scripts (`frontend/package.json:5-15`). Production builds validate their
public environment before Next.js build (`frontend/package.json:7-12`).

```bash
cd marketing
npm ci
npm run dev
```

The marketing package provides Astro development, typechecked build, preview,
and check commands (`marketing/package.json:6-10`).

## Test commands

Run backend unit tests:

```bash
cd backend
go test -count=1 -race ./...
```

This matches the CI unit invocation (`.github/workflows/ci.yml:63-70`). It does
not compile files guarded by `//go:build integration`; run them separately:

```bash
cd backend
TEST_DATABASE_URL='postgres://...' TEST_REDIS_URL='redis://...' \
  go test -count=1 -p 1 -race -tags integration ./...
```

`-p 1` is mandatory because package binaries share one PostgreSQL database and
Redis instance and truncate fixtures; parallel packages create deadlocks and
foreign-key failures unrelated to product behavior
(`.github/workflows/ci.yml:197-205`,
`backend/internal/testutil/db.go:39-49`).

Run frontend and marketing checks:

```bash
cd frontend && npm run lint && npm run typecheck && npm run build
cd ../marketing && npm run build
```

The relevant scripts are `frontend/package.json:5-15` and
`marketing/package.json:6-10`. Run Playwright only against the isolated stack
described below; see [TESTING.md](TESTING.md) before interpreting its result.

`./preflight.sh` writes independent logs under `audit/` for backend build, vet,
untagged race tests, staticcheck, vulnerability/security linters, sqlc vet,
frontend typecheck/audit, and marketing audit (`preflight.sh:1-28`). It does not
run tagged integration tests, frontend lint/build, marketing build, or Playwright;
do not call it “the full suite” (`preflight.sh:13-28`).

## Isolated manual-testing stack

Install frontend dependencies first, then run:

```bash
./scripts/manual-testing-up.sh --reset
```

`--reset` deletes only the `manual-testing` compose project's volumes and reseeds;
without it, existing isolated data is retained (`scripts/manual-testing-up.sh:12-16,32-53`).
The script starts PostgreSQL and Redis on ports 25432/26379, two native Go
backends on 8090/8095, and two Next.js frontends on 3000/3001
(`scripts/manual-testing-up.sh:1-10,62-80,82-123`). It builds the backend and runs
it from `/tmp` so `backend/.env` is not loaded
(`scripts/manual-testing-up.sh:49-64`).

This environment explicitly enables `AUDIT_LOG_V2_ENABLED` and
`AUTHZ_CENTRAL_POLICY_ENFORCE` and configures the Stripe webhook test secret on
both backends (`scripts/manual-testing-up.sh:64-75`). The application default for
central role-policy enforcement is nevertheless false
(`backend/internal/config/config.go:253-263`); never generalize the manual-stack
setting to other environments.

After startup the script prints the pointer to
[manual-testing/testing-dashboard.html](manual-testing/testing-dashboard.html),
which links the active checklists and reference panels
(`scripts/manual-testing-up.sh:111-123`). Logs are `/tmp/qrapp-mtest-{1,2}.log`
and `/tmp/frontend-{a,b}.log` (`scripts/manual-testing-up.sh:121-123`).

The start script is destructive inside its named isolated scope: it uses
`docker compose down -v` for `--reset`, replaces `/tmp/frontend-b`, removes both
frontend `.next` directories, and terminates matching test processes
(`scripts/manual-testing-up.sh:35-39,58-60,88-103`). Do not point its fixed ports,
process names, or compose project at another environment.

## Environment flags that change security behavior

The code defaults guest credentials, staff code, strict staff DB sessions, and
WebSocket tickets to required. It defaults central role enforcement, organization
tenancy routing, audit v2, payment staff-settlement flag, and strict branch
mutations to false (`backend/internal/config/config.go:253-263`). Some handlers
add permanent checks independent of rollout flags; consult
[SECURITY.md](SECURITY.md) rather than inferring behavior from one environment
file.
