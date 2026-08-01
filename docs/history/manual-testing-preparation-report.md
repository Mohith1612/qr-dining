# Manual Testing Preparation — Final Report

**Date:** 2026-05-31 · **Branch:** `premium-qr-collateral` · **Author:** preparation pass (Claude)

This report closes out the Manual Testing Preparation phase: the local machine was cleaned of disposable processes, a fully isolated multi-instance environment was stood up and verified, and the testing documentation was generated. **No features were built, no schema or business logic changed, and the protected R1 soak stack was never touched.**

---

## 1. Cleanup Summary

### Protected (soak — never touched)
| Component | Identity | State after |
|---|---|---|
| Soak backend | container `qr-app-chaos` → host `:8080`, `AUDIT_LOG_V2_ENABLED=true` | Up, `/health` 200 |
| Soak Postgres | `qr-dining-postgres-1` (project `qr-dining`, internal `5432/tcp`) | Up (healthy) |
| Soak Redis | `qr-dining-redis-1` (internal `6379/tcp`) | Up (healthy) |
| Host OS Postgres/Redis | systemd `127.0.0.1:5432` / `127.0.0.1:6379` | Untouched |

### Disposable (terminated)
| Process / stack | Was | Action |
|---|---|---|
| `pilot-app` | `:8090`, `:8091` (native Go) | Stopped (earlier this session) |
| `pilot-app-new` | `:8095` (native Go) | Killed |
| `next dev` | `:3001`, `:3002` | Killed (incl. stale leftovers) |
| `pilot-validation` stack | `pilot-validation-postgres` `:15432`, `pilot-validation-redis` `:16379` | `docker compose … down -v` (containers + volumes removed) |

**Before:** pilot-validation PG/Redis running; pilot-app-new on `:8095`; next dev on `:3001`/`:3002`; soak up.
**After:** all disposable ports (`3000–3002`, `8090`, `8091`, `8095`, `15432`, `16379`) freed; soak still up. (Ports `8090`/`8095` were then deliberately reused for the new isolated backends — see below.)

---

## 2. Environment Summary

Two backend instances share **one isolated Postgres + Redis + guest-token secret** (so guest tokens and realtime events cross instances), each with its own frontend.

| Service | How it runs | Port | Connects to |
|---|---|---|---|
| `manual-testing-postgres` | docker, project `manual-testing` | `25432`→5432 | — |
| `manual-testing-redis` | docker, project `manual-testing` | `26379`→6379 | — |
| **app-1** | native binary `/tmp/qrapp-mtest` | **8090** | mtest PG+Redis, `WORKER_REGION=mtest-1` |
| **app-2** | native binary `/tmp/qrapp-mtest` | **8095** | same PG+Redis, `WORKER_REGION=mtest-2` |
| **frontend-a** | `next dev` in repo `frontend/` | **3000** | → app-1 `:8090` |
| **frontend-b** | `next dev` in copy `/tmp/frontend-b/` | **3001** | → app-2 `:8095` |

New file: `docker-compose.manual-testing.yml` (project `manual-testing`, network + volumes isolated from soak and pilot).

**Verification performed (all passed):**
- `app-1` and `app-2` `/readyz` → `{postgres:ok, redis:ok}`.
- Isolation: backends connect only to `25432`/`26379`; `docker inspect` confirms the `manual-testing` network/volumes; soak DB never connected. (`config.go` uses `godotenv.Overload()`, so I ran the binary from `/tmp` where there is **no** `.env`; inline env wins and `backend/.env` — which points at the soak IP — is not read.)
- **Cross-instance auth:** a session created on app-1 was read back on app-2 using app-1's guest token → shared DB + shared `GUEST_TOKEN_SECRET` confirmed. Menu reads identically from both instances.
- Both frontends serve HTTP 200; the served chunks confirm **frontend-a→:8090** and **frontend-b→:8095** (separate `.next` build dirs prevent cross-clobber).
- CSP allows `connect-src` to `:8090`/`:8095` so the browser can reach both backends.
- Soak intact throughout (`qr-app-chaos` + `qr-dining-postgres-1`/`redis-1` up; `:8080/health` 200).

---

## 3. URLs

- Guest / Staff / Platform UI (instance A): **http://localhost:3000**
- Same UI on instance B (cross-instance tests): **http://localhost:3001**
- Backend app-1: http://localhost:8090 (`/health`, `/readyz`, `/metrics`)
- Backend app-2: http://localhost:8095
- Guest entry: `http://localhost:3000/table/<qr_token>`
- Staff login: `http://localhost:3000/staff/login` → `/staff/{kitchen,waiter,admin}`
- Platform login: `http://localhost:3000/platform/login`
- Postgres: `postgres://mtest:mtest_pass@localhost:25432/qrdining_mtest`
- Redis: `redis://localhost:26379/0`

---

## 4. Credentials

| Actor | Login | Secret |
|---|---|---|
| Staff — owner (Owner Sam) | branch `DEMO-MAIN` | PIN `1234` |
| Staff — waiter (Waiter Alex) | branch `DEMO-MAIN` | PIN `1234` |
| Staff — kitchen (Chef Jordan) | branch `DEMO-MAIN` | PIN `1234` |
| Platform super_admin | `admin@mtest.local` | `Mtest!admin123` |

**Table QR tokens** (guest entry `…/table/<token>`):
- T1 `6d251dc7479e4832e2a79b696a3477100ea5521ef5a5fabad2ee0a9a62fe9ff2`
- T2 `359cae15c9c5b8ca422e3d183d88e0f9597f2f71e4b0fdcb11c103469f37df45`
- T3 `caf95fff7243553e0521ca2a6b49e37fb23ad93752d2926a69190f1dcf837e15`

Re-list tokens any time: `docker exec manual-testing-postgres psql -U mtest -d qrdining_mtest -tAc "SELECT identifier, qr_code_token FROM tables ORDER BY id;"`

Seed baseline: org `demo-restaurant`, branch `DEMO-MAIN`, 3 tables (available), 3 staff, 5 menu categories / 15 items / 6 modifiers, 3 subscription plans, platform admin. 0 active sessions (clean).

---

## 5. Generated Documentation Files

| File | Purpose |
|---|---|
| `manual-testing-user-guide.html` | Role-by-role testing handbook (URL · login · permissions · actions · expected outcome) for guest, participant, waiter, kitchen, manager, org owner, platform admin + sub-roles, plus the cross-instance realtime procedure. |
| `database-reference.html` | **Regenerated** from the live schema — 55 tables (grouped by domain), 158 indexes, 100 FKs, 20 unique constraints, 423 checks (incl. NOT NULL), 1 immutability trigger, 0 views, 14 enums, data-flow + ERD overview. Supersedes the stale 23-table version. |
| `manual-testing-checklist.html` | Print-friendly certification checklist (Test · Expected · Pass · Notes) across all required categories incl. cross-tenant isolation, realtime, multi-instance, security, billing, themes, collateral, analytics, support console, and the three pilot-finding regressions. |
| `docker-compose.manual-testing.yml` | Isolated Postgres+Redis datastores (project `manual-testing`). |
| `manual-testing-preparation-report.md` | This report. |

---

## 6. Known Risks / Notes

1. **Seed demo-session step fails (cosmetic).** `scripts/seed.go` aborts at the end creating 2 demo sessions: `null value in column "session_business_date"` (a migration added a NOT-NULL column the seed doesn't populate). **All entities testers need were created** (org/branch/tables/staff/menu/plans/admin); testers create their own sessions. Worth fixing in `seed.go` later, but out of scope here.
2. **`backend/.env` points at the soak DB IP** (`172.20.0.2:5432`) and `config.go` uses `godotenv.Overload()` (overrides shell env). The binary is therefore run from `/tmp` (no `.env`) so inline env wins. **Do not run `go run ./cmd/server` from inside `backend/` for these instances** — it would load `backend/.env` and could point at the soak DB. Use the documented `/tmp/qrapp-mtest` invocation.
3. **`frontend/.env.local` was changed** from `:8095` (dead pilot port) to `:8090` for frontend-a, and shows as modified in git. It is a local env file; revert with `git checkout frontend/.env.local` when done.
4. **frontend-b runs from `/tmp/frontend-b`** (a copy with `node_modules` symlinked, own `.env.local`→`:8095`). If `/tmp` is cleared, recreate it (see §7) — the same `/tmp` fragility that caused the earlier soak outage applies; keep `/tmp` intact during the pass.
5. **CSP pins backend ports.** The frontend CSP `connect-src` only allows `:8080`/`:8090`/`:8095`. Do not move the test backends off `8090`/`8095` without updating `frontend/next.config.ts`, or the browser will silently block API/WS.
6. **Shared secret / Redis by design.** Both instances use `GUEST_TOKEN_SECRET=mtest-shared-secret` and Redis db 0 — required for the cross-instance test. This is a dev secret, fine for local testing only.
7. **Strict-rollout flags are off.** Behaviour matches pre-enforcement defaults; platform entitlements/feature-flags/suspension are resolve-only (shadow). To test enforcement paths, set the relevant flag on a test instance (never the soak).

---

## 7. Recommended Manual Testing Order

1. **Guest happy path** (frontend A): QR → session → cart → order → live tracking.
2. **Kitchen + waiter ops loop**: advance order through the kitchen KDS, then serve from the waiter queue.
3. **Payment settlement**: host initiates cash/UPI → waiter settles → guest sees completion → session closes.
4. **Reconnect & realtime**: reload mid-session; idle into reactivation and recover; toggle menu availability live.
5. **Multi-instance fan-out** (two browsers): guest on A (`:3000`/app-1), staff on B (`:3001`/app-2) — confirm events cross instances both ways.
6. **Manager / owner admin**: menu, staff, tables, promos, customers, analytics, audit.
7. **Platform + support + billing**: org/branch/plan/entitlement/subscription/billing/flags/themes/collateral/observability; support console read-only inspection; sub-role separation.
8. **Cross-tenant isolation**: branch/guest/snapshot/analytics scoping (incl. T-01).
9. **Security**: trust-domain rejection, token tampering, lockout, rate limits, server-side host gating, body-size limit.
10. **Pilot-finding regressions**: F-8 (token never leaked), F-6 (live tracker), F-1 (reconnect reactivation).

### Re-creating frontend-b (if /tmp is cleared)
```bash
SRC=/home/mohith/Development/projects/qr-dining/frontend; DST=/tmp/frontend-b
rm -rf "$DST"; mkdir -p "$DST"
for i in app components config features hooks lib providers public store styles middleware.ts next.config.ts next-env.d.ts package.json tsconfig.json postcss.config.mjs components.json eslint.config.mjs .env.development .env.production; do cp -a "$SRC/$i" "$DST/$i"; done
ln -s "$SRC/node_modules" "$DST/node_modules"
printf 'NEXT_PUBLIC_API_URL=http://localhost:8095\nNEXT_PUBLIC_WS_URL=ws://localhost:8095\n' > "$DST/.env.local"
```

### Teardown when finished (never affects soak)
```bash
pkill -f qrapp-mtest                  # stop both backends
pkill -f 'next dev'                   # stop both frontends
docker compose -f docker-compose.manual-testing.yml down -v   # remove isolated PG/Redis + volumes
git checkout frontend/.env.local      # revert the local env tweak
```
