# qr-dining

A session-centric realtime dine-in hospitality operating system. **Not a food-delivery app.**

A QR code on a restaurant table creates a live **session**. Several guests scan it, join that session, and share one cart in real time. Staff — waiter, kitchen, manager, owner — work a live dashboard for their branch. A separate platform control plane operates tenants. The unit of state is the session, not the user, and that single decision explains most of the architecture.

---

## Status — 2026-08-22

**Beta, mid manual certification. Nothing is in production.**

| | |
|---|---|
| Trunk | `feature/signoz-observability` @ `9865a48` — 205 commits ahead of `main`, strict superset of every other branch |
| `main` | Stale spine. PR #1 is open against it with head `b57f746` |
| Release tag | **None.** No `v*` tag exists, so no GHCR image has ever been published |
| Beta | **Live** at `qr-beta.mohith16.com`, built on the VM from `b57f746`, seeded demo tenants |
| Production | **Not provisioned.** No domain purchased |
| Manual certification | **In progress and formally BLOCKED** — see below |
| Schema | v39 (39 migrations) · OpenAPI 2.2.0, 171 operations |

**What works today:** the full guest journey (QR → join → shared cart → order → kitchen → serve → bill → settle), staff and kitchen and waiter dashboards, the platform control plane, multi-tenant isolation, realtime fan-out across two backend instances, R2 uploads, nightly backups with a verified restore, and 27 Prometheus alert rules.

**What is blocked before public release:**

1. **The release candidate has never been soaked** (SEV-0). The 124-hour soak that passed in June ran the pre-redesign binary.
2. Merge to `main` and tag — nothing has been released.
3. Alertmanager routes to a local sink, not to a human.
4. Certification is blocked on candidate provenance, broken SigNoz ingestion, and nine reachable advisories in the deployed binary.

The full open list is [docs/RELEASE.md §5](docs/RELEASE.md#5-open-before-v10). The current certification verdict is [release-certification/manual-certification-preflight-2026-08-22.md](release-certification/manual-certification-preflight-2026-08-22.md).

---

## Architecture at a glance

```
Cloudflare              OpenNext Worker (Next.js) · R2 (uploads + backups)
    │ HTTPS / WSS
shared OCI Ampere VM    nginx + certbot  (/opt/proxy, shared with other projects)
    │
app container (Go/Gin)  HTTP · WebSocket Hub · 6 background workers
    │
PostgreSQL 17           source of truth  ── Redis 7  ephemeral by design
    │
observability           Prometheus (43 metrics, 27 rules) · Alertmanager
                        SigNoz/OTel, gated on OTEL_ENABLED
```

Go 1.26 · Gin · PostgreSQL 17 · sqlc · pgx v5 · Redis 7 · gorilla/websocket · Next.js App Router · Astro (marketing).

Full detail: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Quick start

```bash
# datastores
docker compose up -d postgres redis

# backend  (make targets live in backend/, there is no root Makefile)
cd backend
cp .env.example .env        # set DATABASE_URL, REDIS_URL, GUEST_TOKEN_SECRET, MFA_ENCRYPTION_KEY
make run                    # migrations are embedded and applied at boot
make seed                   # prints org/branch/table IDs, QR tokens, staff codes and PINs

curl localhost:8080/readyz  # 200 with postgres/redis "ok"

# frontend
cd ../frontend && npm install && npm run dev    # http://localhost:3000
```

Staff sign-in needs **branch code + staff code + PIN**. PIN alone is not accepted.

For anything exploratory, prefer the isolated two-instance stack over your dev datastores:

```bash
./scripts/manual-testing-up.sh      # backends :8090/:8095, frontends :3000/:3001
./scripts/manual-testing-down.sh
```

Full setup, commands and conventions: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

---

## Documentation

Everything below is current and maintained. Anything not listed here is historical.

### Guides

| Document | Purpose |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | System shape, backend and frontend layout, data model, realtime, tenancy, known boundaries |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Local setup, commands, migrations, sqlc, conventions, safety rules |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Beta and production deployment: VM, Docker, nginx, Cloudflare, TLS, R2, env, bring-up |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Day-2: health, deploys, alerting, rollout flags, backups, cadence, onboarding, standing rules |
| [docs/RUNBOOKS.md](docs/RUNBOOKS.md) | Incident playbooks — auth, Redis, WebSocket, payments, webhooks, flags, database, drills |
| [docs/RECOVERY.md](docs/RECOVERY.md) | Backups, restore procedures, disaster scenarios, verification log |
| [docs/TESTING.md](docs/TESTING.md) | Test layers, what CI runs, what it doesn't, e2e, chaos, manual certification |
| [docs/SECURITY.md](docs/SECURITY.md) | Threat model, tenancy, the three trust domains, rate limiting, secrets, scanning |
| [docs/RELEASE.md](docs/RELEASE.md) | Branch model, release sequence, gates, everything open before v1.0, beta vs production |

### Reference

| Document | Purpose |
|---|---|
| [docs/reference/session-lifecycle-state-machine.md](docs/reference/session-lifecycle-state-machine.md) | Session states and transitions. Code must satisfy this |
| [docs/reference/payment-finalization-invariants.md](docs/reference/payment-finalization-invariants.md) | Payment correctness contract |
| [docs/reference/realtime-reconciliation-invariants.md](docs/reference/realtime-reconciliation-invariants.md) | Realtime correctness contract |
| [docs/reference/security-hardening-checklist.md](docs/reference/security-hardening-checklist.md) | §-numbered security gate list cited by code comments |
| [docs/reference/websocket-events.md](docs/reference/websocket-events.md) | WebSocket event catalogue and envelope |
| [docs/reference/reconnect-guide.md](docs/reference/reconnect-guide.md) | Client reconnect and snapshot-reconciliation algorithm |
| [openapi.yaml](openapi.yaml) | HTTP API contract, v2.2.0 |
| [docs/master-system-context-v1.md](docs/master-system-context-v1.md) | Deep code-level map, for when the guides aren't enough |

### Project state and decisions

- [STATE-OF-THE-PROJECT.md](STATE-OF-THE-PROJECT.md) — engineering handbook: roadmap, technical-debt ledger, product vision
- [docs/adr/](docs/adr/) — architecture decision records
- [docs/dependency-upgrades.md](docs/dependency-upgrades.md) — Dependabot policy
- [docs/signoz-rollout-runbook.md](docs/signoz-rollout-runbook.md) — phased tracing rollout

### Certification and history

- [release-certification/](release-certification/) — current certification round; earlier rounds under `archive/`
- [docs/manual-testing/](docs/manual-testing/) — the manual certification suite; start at `testing-dashboard.html`
- [docs/history/](docs/history/) — archived release evidence, superseded plans and prior audits. **Historical record, not operational instructions**

---

## Repository layout

```
backend/          Go service — cmd/, internal/, migrations/, scripts/, sql/
frontend/         Next.js app — (guest) (staff) (platform) route groups
marketing/        Astro marketing site (separate deploy)
e2e/              106 Playwright specs
deploy/           vm/, observability/, signoz/, backup/, nginx/
scripts/          chaos harness, manual-testing stack up/down
docs/             all active documentation
release-certification/   certification evidence
openapi.yaml      API contract
```

---

## Working in this repo

<<<<<<< HEAD
Agent instructions are in [AGENTS.md](AGENTS.md). A few rules that apply to everyone:
=======
| Variable | Default | Required | Description |
|---|---|---|---|
| `DATABASE_URL` | — | yes | PostgreSQL connection string |
| `REDIS_URL` | — | yes | Redis connection string |
| `PORT` | `8080` | no | HTTP listen port |
| `GIN_MODE` | `release` | no | `release` or `debug` |
| `READ_TIMEOUT` | `10s` | no | HTTP read timeout |
| `WRITE_TIMEOUT` | `30s` | no | HTTP write timeout |
| `SHUTDOWN_TIMEOUT` | `15s` | no | Graceful shutdown window |
| `TRUSTED_PROXIES` | `172.16.0.0/12` | no | Comma-separated CIDR list of upstream proxies |
| `RATE_LIMIT_RPM` | `60` | no | Requests per minute per IP |
| `LOG_LEVEL` | `info` | no | `debug`, `info`, `warn`, `error` |
| `LOG_PRETTY` | `false` | no | Human-readable logs (development only) |
| `CORS_ALLOWED_ORIGINS` | _(none)_ | no | Comma-separated allowed origins |
| `STALE_SESSION_INTERVAL` | `5m` | no | How often the stale session cleaner runs |
| `PRESENCE_EXPIRY_INTERVAL` | `60s` | no | How often the presence expiry worker runs |
| `HOST_ABSENCE_GRACE` | `3m` | no | Host heartbeat age required before automatic transfer |
| `SESSION_PRESENCE_GRACE` | `60s` | no | Creation grace before a session can be considered for pausing |
| `SESSION_IDLE_GRACE` | `5m` | no | Durable participant-idle age required before pausing |
| `SESSION_REACTIVATION_WINDOW` | `5m` | no | Time a paused session remains eligible for reactivation |
| `DB_MAX_CONNS` | `20` | no | pgxpool maximum connections |
| `DB_MIN_CONNS` | `2` | no | pgxpool minimum connections |
| `DB_MAX_CONN_LIFETIME` | `1h` | no | Maximum age of a DB connection |
| `DB_MAX_CONN_IDLE_TIME` | `30m` | no | Maximum idle time for a DB connection |
>>>>>>> fix/presence-host-authority

- **Additive migrations only.** This is what makes a binary rollback across a migration safe.
- **`audit_log` is append-only**, enforced by a database trigger. Never `UPDATE` it.
- **`internal/db/sqlc/` is generated.** Never hand-edit it; CI fails on drift.
- **Never touch the soak stack** (compose project `qr-dining` on the development host).
- Integration tests run migrations against `TEST_DATABASE_URL`. Point it at a throwaway database.
