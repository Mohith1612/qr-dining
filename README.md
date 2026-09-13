# QR Dining

QR Dining is a session-centered dine-in ordering application. A table has at
most one live session, participants share a cart, and only the current host can
place its order or initiate payment
(`backend/migrations/000020_realtime_session_hardening.up.sql:3-39`,
`backend/internal/services/cart.go:48-57`,
`backend/internal/services/order.go:95-108`,
`backend/internal/services/payment.go:128-146`). Restaurant staff and platform
operators use separate authenticated surfaces
(`backend/internal/server/server.go:272-485`).

Start with [the documentation index](docs/README.md). It identifies the audience
and verification date for each maintained document. [Architecture](docs/ARCHITECTURE.md)
describes the current implementation; [invariants](docs/INVARIANTS.md) arbitrate
test-versus-code disputes; [development](docs/DEVELOPMENT.md) gives exact setup
and test commands.

## Local start

The isolated two-instance stack is the shortest reproducible route to seeded
data:

```bash
cd frontend && npm ci && cd ..
./scripts/manual-testing-up.sh --reset
```

The launcher starts isolated PostgreSQL and Redis, migrates and seeds them,
builds two backend processes, and starts two frontend processes
(`scripts/manual-testing-up.sh:19-80,82-123`). It prints a pointer to
[`docs/manual-testing/testing-dashboard.html`](docs/manual-testing/testing-dashboard.html)
after both frontends answer (`scripts/manual-testing-up.sh:111-123`). Stop it with:

```bash
./scripts/manual-testing-down.sh
```

The default teardown removes only the manual-testing compose volumes; use
`--keep-data` to retain them (`scripts/manual-testing-down.sh:21-28`). The
launcher also terminates processes matching `next dev`, so read its process and
port handling before using it beside another Next.js checkout
(`scripts/manual-testing-up.sh:88-103`).

For a native single-instance development setup and the exact verification
commands, use [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Repository map

| Path | Contents | Source |
|---|---|---|
| `backend/` | Go HTTP/WebSocket service, workers, migrations, and database queries | `backend/cmd/server/main.go:23-136` |
| `frontend/` | Next.js guest, staff, and platform route groups | `frontend/app/(guest)/table/[token]/page.tsx:1-112`; `frontend/app/(staff)/staff/login/page.tsx:1-204`; `frontend/app/(platform)/platform/login/page.tsx:1-162` |
| `marketing/` | Independently built Astro package | `marketing/package.json:1-11` |
| `e2e/` | Playwright projects and specifications; read the census before treating green output as coverage | `e2e/playwright.config.ts:1-51`; `audit/e2e-vacuity-census.md:1-42` |
| `deploy/` | VM, backup, Prometheus, nginx, and SigNoz configuration | `deploy/vm/docker-compose.yml:1-114`; `deploy/observability/docker-compose.observability.yml:1-65` |

The server embeds and applies pending migrations before accepting traffic
(`backend/cmd/server/main.go:46-58`, `backend/internal/db/migrations.go:15-44`). The
schema is forward-fix-only in operations; do not infer rollback safety from the
presence of down files. The verified constraints and recovery procedure are in
[docs/RECOVERY.md](docs/RECOVERY.md).
