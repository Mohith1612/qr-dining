# Development

Local setup and day-to-day commands. Architecture background: [ARCHITECTURE.md](ARCHITECTURE.md). Tests and gates: [TESTING.md](TESTING.md).

---

## 1. Prerequisites

- Go **1.26+**
- Node.js LTS (CI uses `lts/*`)
- Docker + Docker Compose v2.24+
- PostgreSQL 17 client tools, only if you run backup/restore on the host

`make` targets live in `backend/Makefile`, so **run `make` from `backend/`**, not the repo root. There is no root Makefile.

## 2. Backend

```bash
# 1. Datastores (from the repo root)
docker compose up -d postgres redis

# 2. Environment
cd backend
cp .env.example .env
#    DATABASE_URL / REDIS_URL must point at the local instances above.
#    GUEST_TOKEN_SECRET and MFA_ENCRYPTION_KEY must be set — the server refuses
#    to start in release mode with weak or missing values.

# 3. Run (migrations are embedded and applied at boot)
make run          # == go run ./cmd/server

# 4. Verify
curl localhost:8080/health   # {"status":"ok"}
curl localhost:8080/readyz   # 200 with postgres/redis "ok"

# 5. Seed development data
make seed         # prints org/branch/table IDs, QR tokens, staff codes and PINs
```

`make seed` prints the credentials it creates. Staff sign-in needs **branch code + staff code + PIN** — PIN alone is not accepted.

### Environment variables

`backend/.env.example` is the authoritative list and is kept in sync with `internal/config/`. The production template with every rollout flag is `deploy/vm/.env.production.example`.

Two behaviours worth knowing before you fight them:

- **Release mode (`GIN_MODE=release`) ignores dotenv files.** Injected environment variables are authoritative, so a stray `.env` cannot override a production value.
- **Release mode fails hard** on a weak/missing `GUEST_TOKEN_SECRET`, an empty `CORS_ALLOWED_ORIGINS`, or a malformed MFA/webhook secret. A boot loop almost always names the offending variable in the first log line.

### Migrations

The server applies all pending migrations at startup via `go:embed` — never run a separate migration step in a deployed environment. For local control:

```bash
make migrate-up          # go run ./cmd/migrate up
make migrate-down        # roll back exactly one
```

Migrations are **additive-only** by project rule. That is what makes rolling a binary back across a migration safe, so do not write a destructive one without deciding to give that property up.

### sqlc

`internal/db/sqlc/` is generated. Never hand-edit it.

```bash
make sqlc-generate       # go tool sqlc generate
```

CI fails on drift, so regenerate and commit whenever you touch `sql/queries/`.

## 3. Frontend

```bash
cd frontend
npm install
npm run dev              # http://localhost:3000
npm run lint
npm run typecheck
npm run build
```

Point it at the backend with `NEXT_PUBLIC_API_BASE`. `NEXT_PUBLIC_API_BASE` must equal the API origin exactly, or the CSP blocks API and WebSocket calls.

A `prebuild` guard rejects production builds that point at localhost or plain HTTP. For a local production build, set `ALLOW_LOCALHOST_BUILD=true` (this is what CI does).

Do not run `build:cf` / `deploy:cf` while `next dev` is serving the same checkout — the production build rewrites `.next/` underneath the dev server and corrupts it.

## 4. Marketing site

```bash
cd marketing
npm install
npm run dev
npm run build            # tsc + astro check + astro build
```

Separate Astro site for the public marketing domain. It shares no code with `frontend/`.

## 5. Isolated manual-testing stack

For anything exploratory, use the isolated stack rather than your dev datastores:

```bash
./scripts/manual-testing-up.sh [--reset]
./scripts/manual-testing-down.sh
```

It stands up its own Postgres (`:25432`) and Redis (`:26379`) under the `manual-testing` compose project, two backend instances (`:8090`, `:8095`) sharing them, and two frontends (`:3000`, `:3001`). Two instances against one datastore is the point: it exercises cross-instance realtime fan-out. `--reset` wipes volumes and re-seeds, which regenerates QR tokens.

The script prints the path to `docs/manual-testing/testing-dashboard.html`, the entry point for tenants, QR links and tester credentials.

## 6. Conventions

- **Routes** are all registered in `internal/server/server.go`. That file is the authoritative route table; keep it that way.
- **Errors**: return typed `domain` errors from services; map to HTTP in handlers. Clients key on the `code` field, never on `message`.
- **Transactions** are opened in services via `repository.WithTx`, never in handlers.
- **Events** are published after the transaction commits, never inside it.
- **Audit**: `audit_log` is append-only and trigger-protected. Never `UPDATE` it.
- **Formatting**: `gofmt` is a CI gate. `golangci-lint` is pinned to **v2.12.2** — match it locally or you will see a different result than CI.
- **Generated code** (`internal/db/sqlc/`) and **`openapi.yaml`** are both checked for drift/validity in CI.
- Behaviour changes that touch session lifecycle, payments, or realtime must stay inside the contracts in [reference/](reference/).

## 7. Standing safety rules

- **Never point local work at the soak stack** (compose project `qr-dining` on the development host): no `down -v`, no restarts, no volume wipes.
- Integration tests run migrations against whatever `TEST_DATABASE_URL` names. Point them at a **throwaway** database, never a soak or production one.
- Process-killing helpers must anchor their patterns (`pkill -f '^/tmp/qrapp-mtest$'`) so they cannot match a protected container's process.
