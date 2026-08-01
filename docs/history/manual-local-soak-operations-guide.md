# Manual Local Soak Operations Guide

A step-by-step, beginner-safe guide to run, access, and manually test the whole product
**during the R1 soak** without destabilizing the rollout.

> **R1 soak in one sentence:** the backend container `qr-app-chaos` is running with
> `AUDIT_LOG_V2_ENABLED=true`, writing immutable rows to `audit_log` in the Postgres
> volume with zero write failures. Keep that true and the soak stays valid.

---

## 0. TL;DR — the golden rules

- ✅ **Safe:** browsing/using the app, starting/stopping the **frontend**, querying the DB
  read-only, closing Claude, editing code, rebuilding.
- ⚠️ **Safe only if you re-include the flag:** restarting the backend — always pass
  `-e AUDIT_LOG_V2_ENABLED=true` (see §3 for the exact command).
- ⛔ **Breaks the soak:** restarting the backend *without* the flag, truncating/dropping
  `audit_log`, or wiping the Postgres volume (`docker compose down -v`).
- The backend talks to the **container** Postgres. Always inspect data with
  `docker exec qr-dining-postgres-1 psql ...` — **not** a host `psql` (that hits a
  different, empty DB).

---

## 1. Current service state (verified)

| Service | What/where | Status | Notes |
|---------|-----------|--------|-------|
| Postgres | container `qr-dining-postgres-1` | Up, healthy | `restart: unless-stopped` (survives reboot). Holds `audit_log` (the soak data). Port 5432 is **not** published to host. |
| Redis | container `qr-dining-redis-1` | Up, healthy | `restart: unless-stopped`. |
| Backend (API+WS) | container `qr-app-chaos` (host-built Go binary in `alpine`) | Up, `AUDIT_LOG_V2_ENABLED=true` | Published on **http://localhost:8080**. **No restart policy** → does NOT auto-start after reboot (see §3). |
| Frontend (Next.js) | not running | — | Start it yourself: §2. Runs on **http://localhost:3000**. |
| Grafana / Prometheus | config files only (`deploy/observability/`) | not running | Not stood up locally. Check metrics via `curl :8080/metrics` (§8). |

Verify the soak is alive any time:
```bash
docker ps --format '{{.Names}}\t{{.Status}}' | grep qr-dining
docker inspect qr-app-chaos --format '{{range .Config.Env}}{{println .}}{{end}}' | grep AUDIT_LOG_V2   # must show =true
curl -s localhost:8080/readyz                                                                          # {"status":"ready",...} 200
```

---

## 2. Start the frontend (the only thing you need to start)

The backend + datastores are already running. You only need the UI:
```bash
cd /home/mohith/Development/projects/qr-dining/frontend
npm run dev          # node_modules already installed; Next.js dev server
```
Expected: `▲ Next.js … - Local: http://localhost:3000` and `Ready in …`.
`frontend/.env.local` already points at the backend:
`NEXT_PUBLIC_API_URL=http://localhost:8080`, `NEXT_PUBLIC_WS_URL=ws://localhost:8080`,
`NEXT_PUBLIC_TENANT_SLUG=demo-restaurant`.

Health checks:
```bash
curl -s localhost:8080/readyz        # backend + deps: {"postgres":"ok","redis":"ok"}
curl -s localhost:8080/healthz 2>/dev/null; curl -s localhost:8080/health   # liveness
# WS readiness (needs a valid session+ticket; see §5). Quick sanity: the upgrade route exists:
curl -s -o /dev/null -w "%{http_code}\n" "localhost:8080/ws"   # 400/401 = route alive (not 404)
# audit logging still active during soak:
docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -t -A -c "SELECT count(*) FROM audit_log;"  # grows over time
```

### (Optional) If the backend is NOT running — start it WITH the flag
Only if `qr-app-chaos` is missing (e.g. after a reboot):
```bash
cd /home/mohith/Development/projects/qr-dining/backend
CGO_ENABLED=0 GOOS=linux go build -o /tmp/qrapp ./cmd/server
docker rm -f qr-app-chaos 2>/dev/null
docker run -d --name qr-app-chaos --restart unless-stopped \
  --network proxy_network --network qr-dining_backend -p 8080:8080 \
  -e DATABASE_URL="postgres://qrdining:changeme_strong_password@postgres:5432/qrdining?sslmode=disable" \
  -e REDIS_URL="redis://redis:6379/0" -e PORT=8080 -e GIN_MODE=release \
  -e AUDIT_LOG_V2_ENABLED=true \
  -e CORS_ALLOWED_ORIGINS="http://localhost:3000,http://127.0.0.1:3000" \
  -v /tmp/qrapp:/qrapp:ro alpine:3.20 /qrapp
```
(`--restart unless-stopped` added so it survives future reboots.) If the datastores are
also down: `docker compose -f docker-compose.yml -f docker-compose.staging.yml up -d postgres redis` first. **Never** add `-v` to a `down` — that deletes the audit data.

> **CORS is required for the browser frontend.** Without `CORS_ALLOWED_ORIGINS` the API
> serves requests but emits no `Access-Control-Allow-Origin` header, so the browser at
> `http://localhost:3000` blocks every call (the CORS error you hit). The value must match
> the frontend origin exactly. Setting it also lets the WebSocket origin check accept
> `:3000`. (`CORS_ALLOWED_ORIGINS` is independent of the soak — but always keep
> `AUDIT_LOG_V2_ENABLED=true` on the same command.)

> **Verifying endpoints — use the right HTTP method.** Routes are registered for specific
> methods only, so `curl -I` (HEAD) and `curl -X OPTIONS` return **404** even though the
> endpoint is healthy. Use:
> `curl http://localhost:8080/readyz` (GET → 200) and
> `curl -X POST http://localhost:8080/staff/auth -H 'Content-Type: application/json' -d '{"branch_code":"DEMO-MAIN","staff_code":"STAFF01","pin":"1234"}'` (POST → token).
> A bare `OPTIONS` preflight returns **204** (with the Allow-Origin header) only when CORS
> origins are set — that's expected, not an error.

---

## 3. Keeping R1 active — practical operator answers

**What keeps the soak meaningful:** `qr-app-chaos` running continuously with
`AUDIT_LOG_V2_ENABLED=true`, `audit_log` accumulating in the Postgres volume, and
`audit_write_failures_total` staying 0. Time-on-flag is what you're accruing.

| Action | Safe? | Why |
|--------|-------|-----|
| **Close Claude / this session** | ✅ Yes | Containers run independently of Claude. |
| **Reboot the machine** | ⚠️ Mostly | Postgres+Redis auto-restart (`unless-stopped`). `qr-app-chaos` (as originally started) does **not** auto-restart — re-run it WITH the flag (§2). If you used the `--restart unless-stopped` command above, it survives. |
| **Stop the frontend only** | ✅ Yes | Frontend is not part of the soak. `Ctrl+C` the `npm run dev` terminal anytime. |
| **Stop the backend temporarily** | ⚠️ Yes if brief + restarted WITH the flag | A short restart is like a deploy. Auditing pauses while it's down; accumulated rows persist; resume by restarting with `AUDIT_LOG_V2_ENABLED=true`. Long downtime just reduces soak coverage. |
| **Ctrl+C** | ✅ Affects only foreground processes | The backend/pg/redis are detached containers; `Ctrl+C` in your shell won't stop them. Only your `npm run dev` (frontend) stops. |
| **Code changes / rebuild** | ✅ Yes | Edit and rebuild freely. Redeploy the backend WITH the flag. **Avoid migrations that alter `audit_log`.** |
| **DB reset / `compose down -v` / TRUNCATE audit_log** | ⛔ No | Destroys the immutable audit trail = soak reset. Don't. |
| **Restart backend WITHOUT the flag** | ⛔ No | Auditing turns off = soak broken. Always include `-e AUDIT_LOG_V2_ENABLED=true`. |

**If you accidentally break it:** re-run the backend with the flag (§2), note the gap in
`r1-live-rollout-status.md`, and restart the soak clock. It's cheap — don't panic.

---

## 4. Role access flows

All roles share backend **http://localhost:8080** and frontend **http://localhost:3000**.
Current seeded data (branch 1 of "Demo Restaurant"):

- Restaurant slug: `demo-restaurant` · Organization id `1`
- Branch: **`DEMO-MAIN`** (id 1)
- Staff (PIN **`1234`** for all): `STAFF01` = owner, `STAFF02` = waiter, `STAFF03` = kitchen
- Tables T1/T2/T3 with QR tokens (see §5)

> Read the **live** values anytime (authoritative — the DB may have extra test data):
> ```bash
> docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -c \
>  "SELECT b.branch_code, s.role, s.staff_code FROM staff s JOIN branches b ON b.id=s.branch_id WHERE s.branch_id=1 AND s.is_active ORDER BY s.role;"
> docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -c \
>  "SELECT identifier, qr_code_token FROM tables WHERE branch_id=1 ORDER BY id;"
> ```

### Guest
- **UI:** `http://localhost:3000/table/<QR_TOKEN>` → joins/creates a session → redirects to
  `/session/<id>/menu`. (See §5 for the QR-less path.)
- No login; identity is a guest token issued on session join (stored client-side).

### Waiter (`STAFF02`)
- **UI:** `http://localhost:3000/staff/login` → branch code `DEMO-MAIN`, staff code
  `STAFF02`, PIN `1234` → dashboard `http://localhost:3000/staff/waiter`.
- Sees active orders/sessions for the branch; settles payments.

### Kitchen (`STAFF03`)
- **UI:** `/staff/login` with `STAFF03` / `1234` → `http://localhost:3000/staff/kitchen`.
- Realtime order queue (order status transitions).

### Branch admin / owner (`STAFF01`)
- **UI:** `/staff/login` with `STAFF01` / `1234` → `http://localhost:3000/staff/admin`.
- Menu/table/staff management for the branch.

### Platform / super admin (API-only locally)
- There is **no dedicated platform UI** beyond the branch `admin` page; platform admin is
  **API-only** here.
- Login: `POST http://localhost:8080/platform/auth` with `{ "email": ..., "password": ... }`
  → returns a platform token (Bearer) for `/platform/*` routes.
- A known platform admin must be seeded with a password you choose (the existing
  `e2e-admin@local` has an unknown password). Create one — **soak-safe**, see §6.

---

## 5. Guest access without a QR PNG (exact, reproducible)

The QR code just encodes the URL `…/table/<qr_code_token>`. You don't need the image.

1. **Get a table's QR token** (container DB):
   ```bash
   docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -t -A -c \
     "SELECT identifier, qr_code_token FROM tables WHERE branch_id=1 ORDER BY id LIMIT 3;"
   ```
   Example (current): T1 = `ef67129d5c050daa6a1f33abc4ae518d1f7ab0e35199c6f8fc81822bec5c6b9f`.
2. **Open it in a browser** (this is exactly what scanning the QR does):
   `http://localhost:3000/table/ef67129d5c050daa6a1f33abc4ae518d1f7ab0e35199c6f8fc81822bec5c6b9f`
3. The page resolves the table and starts a session, landing you on the guest menu.

**Pure-API alternative** (no frontend needed — useful for scripted checks):
```bash
# resolve a table by its QR token
curl -s "localhost:8080/tables/by-qr/<QR_TOKEN>"
# create a session directly (table_id from the query above)
curl -s -X POST localhost:8080/sessions -H 'Content-Type: application/json' \
  -d '{"table_id":1,"display_name":"Manual Tester"}'
# → returns { session: { id }, participant, guest_access_token }
# use the guest_access_token as `Authorization: Bearer <token>` for /sessions/:id/* calls
```
A second device/tab "joins" the same table's active session via the same `/table/<token>`
URL (multi-device) — good for testing realtime sync.

---

## 6. Test data / bootstrap (soak-safe)

The container DB is **already seeded** (Demo Restaurant, branch `DEMO-MAIN`, staff
`STAFF01–03`, tables T1–T3, a menu). You usually don't need to seed.

**Inspect what exists** (container DB only):
```bash
docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -c \
 "SELECT 'restaurants' t, count(*) FROM restaurants UNION ALL
  SELECT 'branches', count(*) FROM branches UNION ALL
  SELECT 'tables', count(*) FROM tables UNION ALL
  SELECT 'staff', count(*) FROM staff UNION ALL
  SELECT 'menu_items', count(*) FROM menu_items;"
```

**Re-seed / add a known platform admin (idempotent, soak-safe** — uses `ON CONFLICT`, does
**not** touch `audit_log` or the flag). Because the container Postgres isn't published to
the host, run the seed *inside the backend network*:
```bash
cd /home/mohith/Development/projects/qr-dining/backend
CGO_ENABLED=0 GOOS=linux go build -o /tmp/qrseed ./scripts/seed.go
docker run --rm --network qr-dining_backend \
  -e DATABASE_URL="postgres://qrdining:changeme_strong_password@postgres:5432/qrdining?sslmode=disable" \
  -e PLATFORM_ADMIN_EMAIL="admin@local.test" -e PLATFORM_ADMIN_PASSWORD="ChangeMe12345" \
  -v /tmp/qrseed:/qrseed:ro alpine:3.20 /qrseed
```
This (re)creates: a super-admin (`admin@local.test` / `ChangeMe12345`), Demo Restaurant,
branch `DEMO-MAIN`, tables, staff (`STAFF01–03`, PIN `1234`), and the menu. Manual table
creation if ever needed:
```bash
docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -c \
 "INSERT INTO tables(branch_id,identifier,qr_code_token) VALUES (1,'T-MANUAL','manual-token-123');"
```

---

## 7. Recommended manual test flows (soak-safe)

Best setup: Chrome with DevTools; use a **normal window** for the guest and an
**incognito window** for staff (separate sessions). Toggle mobile viewport
(DevTools → device toolbar, iPhone 12) for the guest experience.

1. **Guest ordering:** open `/table/<token>` → browse menu → add to cart →
   place order. Confirm it appears under `/session/<id>/orders`.
2. **Kitchen realtime:** keep `/staff/kitchen` (STAFF03) open in another window; place a
   guest order → it should appear/advance in real time (WebSocket).
3. **Waiter settlement / payment_pending:** as guest, go to `/session/<id>/payment` and
   initiate a **cash** payment → session enters `payment_pending` (cart frozen). As waiter
   (STAFF02) on `/staff/waiter`, settle it → session resumes/closes. *Safe:* leaving a
   payment_pending session is fine — the escalation worker only **alerts** (no auto-action).
4. **Reconnect:** on a guest session tab, DevTools → Network → toggle **Offline** then
   **Online** (or just reload `/session/<id>`). The client re-opens the WebSocket and
   reconciles via the snapshot endpoint — no lost state.
5. **Stale tab / reactivation:** leave a guest tab idle; the lifecycle worker moves it to
   `awaiting_reactivation` after the presence grace, then back to active on return (or to
   abandoned after the window). Revisiting the tab should recover gracefully.
6. **Mobile viewport:** run the full guest flow in the iPhone 12 viewport to validate the
   PWA layout.

These flows generate normal audit rows (session.create, payment.initiate, etc.) — that's
expected and *good* soak signal, not a problem.

---

## 8. Observability access (local)

Grafana/Prometheus aren't running locally; the artifacts are in `deploy/observability/`.
Practical checks during the soak:

```bash
# the soak's primary signal — must stay zero:
curl -s localhost:8080/metrics | grep -E '^audit_write_failures_total' || echo "0 (no failures)"
# audit volume + storage growth:
docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining -c \
 "SELECT count(*) rows, pg_size_pretty(pg_total_relation_size('audit_log')) size FROM audit_log;"
# DB pool + readiness:
curl -s localhost:8080/metrics | grep -E '^db_pool_(idle|total|acquired)_conns'
curl -s -o /dev/null -w "readyz %{http_code}\n" localhost:8080/readyz
```
Validate the alert rules render: `promtool check rules deploy/observability/prometheus-alerts.yml`.

**Optional — view the dashboard:** run Prometheus (scrape `localhost:8080/metrics` + load
`prometheus-alerts.yml`) and Grafana (import `grafana-dashboard.json`) in their own
containers. Not required for the soak.

**What matters during R1:** `audit_write_failures_total` (the only page-worthy R1 signal —
expected **0**), `audit_log` row growth vs disk, audited-route p99, `db_pool_*`.
**Expected vs concerning:** a brief readiness blip during any backend restart is expected;
sustained `audit_write_failures_total > 0`, p99 regression > +10 ms, or pool exhaustion is
concerning → consult `r1-soak-monitoring-guide.md §5` (rollback vs pause).

---

## 9. If something looks wrong

- Don't reset the DB. Don't flip the flag off "to debug".
- Capture: `curl -s localhost:8080/metrics | grep -E 'audit_write_failures|db_pool'`, the
  `audit_log` count, and recent `docker logs qr-app-chaos --tail 100`.
- Decide using `r1-soak-monitoring-guide.md §5` (rollback triggers vs pause/escalate) and
  record it in `r1-live-rollout-status.md §10`.
- Rollback (only if a trigger is met): re-run the backend with
  `AUDIT_LOG_V2_ENABLED=false`. MTTR <5 min, zero data loss (rows are immutable). This ends
  the soak — note it in the ledger.

---

### Quick reference card
| Thing | Value |
|-------|-------|
| Frontend | http://localhost:3000 |
| Backend API / WS | http://localhost:8080 · ws://localhost:8080/ws |
| Guest entry | /table/`<qr_code_token>` |
| Staff login | /staff/login → `DEMO-MAIN` / `STAFF01`(owner)·`STAFF02`(waiter)·`STAFF03`(kitchen) / PIN `1234` |
| Staff dashboards | /staff/waiter · /staff/kitchen · /staff/admin |
| Platform auth | POST /platform/auth `{email,password}` (API only; seed an admin via §6) |
| DB access | `docker exec qr-dining-postgres-1 psql -U qrdining -d qrdining` |
| Soak flag | `AUDIT_LOG_V2_ENABLED=true` on `qr-app-chaos` |
| Soak data | `audit_log` table (immutable) in the Postgres volume |
