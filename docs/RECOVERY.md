# RECOVERY.md — QR-Dining Backup & Disaster Recovery

**Canonical backup and disaster-recovery guide.** Companions: [DEPLOYMENT.md](DEPLOYMENT.md) · [OPERATIONS.md](OPERATIONS.md) · [RUNBOOKS.md](RUNBOOKS.md).

Certification evidence: [history/restore-verification-report-2026-09-12.md](history/restore-verification-report-2026-09-12.md) (**current** — schema 40, local-provider round trip, restore timed at volume, restore-and-replay failure documented) · [history/restore-verification-report.md](history/restore-verification-report.md) (schema 33, 2026-06-09, original Phase E certification).

> **Forward-fix only.** A restore is safe **only into the schema version the dump was taken at**.
> Restoring an older dump and letting the app migrate it forward is **not** a supported path:
> migration 22 cannot be re-applied to a populated `audit_log`, and the failure leaves
> `schema_migrations` dirty so the app will not start. Verified empirically 2026-09-12.
> See [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) §0 and [RUNBOOKS.md §8](RUNBOOKS.md#8-migration-rollback).

---

## 1. What exists

- **Nightly dump** (02:30, systemd timer → `backend/scripts/nightly-backup.sh`): `pg_dump --format=custom --compress=9` → sha256 → upload to R2 → append-only `manifest.jsonl` → retention prune (14 days) → Prometheus textfile metrics. Any failure exits non-zero. Most failures also set `qr_dining_backup_last_run_status 1` — **but not all**: the `DATABASE_URL` guard is an explicit `exit 1` placed before the `ERR` trap is installed, so a missing or unreadable env file leaves the status metric reporting the last good run and `BackupFailed` never fires (finding R-3, verified 2026-09-12). Only `BackupTooOld` catches that, 36h later.
- **Bucket layout** (`R2_BUCKET`, currently the temporary `qr-dining-backups`):
  - `backups/YYYY/MM/backup_YYYYMMDD_HHMMSS.dump` (UTC-named, one per run)
  - `backups/manifest.jsonl` — one JSON line per backup: `{timestamp,key,bytes,sha256,provider,retention_days}`. **The manifest is the index of record**: pick restore candidates from it and always verify sha256 after download.
- **Provider abstraction** (`backend/scripts/lib/storage.sh`): `BACKUP_PROVIDER=r2|s3|local`, same code path (AWS CLI) for both clouds. Nothing is bucket-name-aware beyond env.
- **RPO: ≤24h** (nightly). Note the worst case lands where it hurts most: the timer fires at 02:30 server time, so a Friday service running to 01:00 is not backed up until 90 minutes later, and a failure at 02:00 loses the whole Friday night.
- **RTO: minutes**, now measured rather than estimated. At ~90 days of single-site pilot volume (297 MB DB, 500K `audit_log` rows, 16 MB dump) on an amd64 dev host: `pg_dump` ~3 s, `pg_restore` 8–13 s, sha256 0.1 s. Assume 2–3× on the Ampere VM; **budget 5 minutes** for the mechanical path including download and the app's ~12 s restart. **~1–2h** for a full-host rebuild via [DEPLOYMENT.md](DEPLOYMENT.md).

## 2. Choosing a backup

```bash
export AWS_ACCESS_KEY_ID=<R2_ACCESS_KEY> AWS_SECRET_ACCESS_KEY=<R2_SECRET_KEY>
EP=--endpoint-url=https://<account_id>.r2.cloudflarestorage.com
aws s3 cp s3://$R2_BUCKET/backups/manifest.jsonl - $EP --region auto | tail -5   # newest last
aws s3 cp s3://$R2_BUCKET/<key-from-manifest> ./restore-candidate.dump $EP --region auto
sha256sum restore-candidate.dump    # MUST equal the manifest line's sha256 — stop if not
```

> **The manifest lists objects that *were* created, not objects that still exist.** It is
> append-only; retention deletes the object but leaves its line behind. Any entry older than
> `RETENTION_DAYS` (14) is a tombstone whose key will 404. Verified 2026-09-12. Prefer the newest
> entries, and treat a missing object as expected rather than as a lost backup.

## 3. Restore procedures (verified pattern)

`backend/scripts/restore.sh <dump>` — **destructive**: `pg_restore --clean --if-exists --single-transaction` into `DATABASE_URL`, then row-count validation. Single-transaction = all-or-nothing.

### 3a. Drill / side-restore (non-destructive — also the quarterly drill)

Runs everything inside containers; needs no host pg tools. This exact procedure passed against the real R2 bucket on 2026-07-18 (§5):

> **Create the dump's role on the drill server first, or the restore fails completely.**
> `restore.sh` does not pass `--no-owner`, so `ALTER ... OWNER TO qrdining` aborts the restore if
> that role does not exist — and `--single-transaction` means **nothing** is restored, not part of
> it. The `CREATE ROLE` below is what makes this drill work against a production dump; it was
> missing until 2026-09-12 (finding R-1). Substitute the production role name.

```bash
docker network create qr-restore-drill
docker run -d --name qr-drill-db --network qr-restore-drill \
  -e POSTGRES_USER=bv -e POSTGRES_PASSWORD=drill -e POSTGRES_DB=bvdb postgres:17-alpine
# The dump's owner role must exist on the target before pg_restore runs:
sleep 5 && docker exec qr-drill-db psql -U bv -d bvdb -c "CREATE ROLE qrdining LOGIN PASSWORD 'drill';"
docker run --rm --network qr-restore-drill \
  --env-file /etc/qr-dining/backup.env \
  -e DATABASE_URL=postgres://bv:drill@qr-drill-db:5432/bvdb \
  -v /opt/qr-dining/repo/backend/scripts:/scripts:ro -v /tmp/drill:/work \
  postgres:17-alpine bash -c '
    apk add --no-cache bash aws-cli >/dev/null
    export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY AWS_SECRET_ACCESS_KEY=$R2_SECRET_KEY
    KEY=$(aws s3 cp s3://$R2_BUCKET/backups/manifest.jsonl - --endpoint-url $R2_ENDPOINT --region auto | tail -1 | sed -n "s/.*\"key\":\"\([^\"]*\)\".*/\1/p")
    aws s3 cp s3://$R2_BUCKET/$KEY /work/candidate.dump --endpoint-url $R2_ENDPOINT --region auto
    sha256sum /work/candidate.dump     # compare to manifest before proceeding
    echo yes | bash /scripts/restore.sh /work/candidate.dump'
# inspect, then: docker rm -f qr-drill-db && docker network rm qr-restore-drill
```

### 3b. Production restore (destructive — real incident only)

1. Freeze traffic: `cd /opt/qr-dining && docker compose stop app` (proxy 502s; other VM projects unaffected).
2. If any current data might matter, take a pre-restore dump first (`nightly-backup.sh` manually — it uploads and manifests it).
3. Download + sha256-verify the chosen dump (§2).
4. `DATABASE_URL=postgres://<user>:<pw>@localhost-or-container/qrdining bash restore.sh candidate.dump` — from a postgres:17 container on `qr-dining_internal` (attach with `docker run --network qr-dining_qr-dining_internal …`) since the DB is not exposed to the host.
5. `docker compose start app` → `/readyz` 200.
   > **Only restore a dump taken at the schema version the running binary expects.** The previous
   > wording here claimed the app "re-migrates idempotently if the dump predates newer
   > migrations". That is false and was never tested. Migration 22 runs `UPDATE audit_log`
   > (`000022:109`) against the immutability trigger from `000019:82`; with even one audit row
   > present it aborts and golang-migrate leaves `schema_migrations` at `22, dirty=true`, after
   > which the app cannot start and `cmd/migrate` cannot unwedge it. Migration 40's up likewise
   > refuses to apply if any session has two non-terminal payments. If you must replay across
   > those migrations, [RUNBOOKS.md §8](RUNBOOKS.md#8-migration-rollback) has the tested
   > last-resort procedure and what it costs.
6. Verify: row counts (restore.sh prints the six core tables), one full guest journey, audit_log immutability intact (an UPDATE attempt on audit_log must fail), and `SELECT version, dirty FROM schema_migrations` reads the expected version with `dirty = false`.

## 3c. Local / ad-hoc backups (development)

The nightly job above is the production path. For a one-off dump on a development or throwaway database:

> **Requires a host `pg_dump` whose major version matches the server (17).** On a host carrying
> only PostgreSQL 16 client tools this fails outright — `pg_dump: error: aborting because of
> server version mismatch` — and no dump is written. Either install `postgresql-client-17` or run
> it in a container the way §3a does. Verified 2026-09-12 (finding R-2).

```bash
cd backend
DATABASE_URL=postgres://user:pass@host:5432/db ./scripts/backup.sh
# writes $BACKUP_DIR/backup_YYYYMMDD_HHMMSS.dump   (BACKUP_DIR defaults to ./backups)

DATABASE_URL=postgres://user:pass@host:5432/db ./scripts/restore.sh ./backups/backup_YYYYMMDD_HHMMSS.dump
```

`backup.sh` prunes local dumps older than `RETENTION_DAYS`. Note that the script's own comment says the default is 7 while the code uses **30** — trust the code, and set the variable explicitly if the value matters to you. This local pruning is unrelated to the 14-day R2 retention used by `nightly-backup.sh`.

These local dumps carry **no manifest and no checksum**, so they are not restore candidates for an incident. Use §2 for that.

## 4. Disaster scenarios

| Scenario | Action |
|---|---|
| Bad deploy / app regression | Not a restore case: pin previous `IMAGE_TAG`, `docker compose up -d app` ([OPERATIONS.md §2](OPERATIONS.md#2-deploying-a-new-version)) |
| Postgres data corruption / bad mutation | §3b. Data loss bounded by last nightly (≤24h RPO) |
| `qr-dining_postgres_data` volume lost | `docker compose up -d postgres` (fresh volume) → §3b into the empty DB |
| Redis volume lost | Nothing to restore — Redis is disposable by design; restart it, realtime self-recovers |
| Whole VM lost | Rebuild per [DEPLOYMENT.md](DEPLOYMENT.md) (proxy stack first) → §3b with the newest manifest entry. Secrets must come from the password manager — they are not in git |
| R2 bucket lost / creds leaked | Backups are the *copy*; the live DB is intact. Create bucket + new scoped token, update `/etc/qr-dining/backup.env`, run one manual backup, roll the leaked token |
| Backup job silently broken | `BackupTooOld` pages at >36h; `journalctl -u qr-dining-backup` for the failing step; every step is fail-hard so the journal names it |

## 5. Verification log

| Date | What | Result |
|---|---|---|
| 2026-06-09 | Full round trip, `local` provider (Phase E cert) | PASS — 56/56 tables row-identical, trigger + checksum fidelity ([history/restore-verification-report.md](history/restore-verification-report.md)) |
| 2026-07-18 | **Full round trip against the real R2 bucket** (temporary `qr-dining-backups`, account `16465dc4…`): containerized pg_dump 17 → upload → manifest append → fresh download → sha256 vs manifest (`fc5a2ecd…` ✓) → `restore.sh` into a second postgres:17 → content checksum source vs restored (`a2926ca4…` = `a2926ca4…` ✓) → retention pass, textfile metrics `status 0` + success timestamp | **PASS** — closes the "never tested against real R2" SEV-1. Found+fixed en route: BusyBox-incompatible `date` in the retention step (now epoch-based, portable) |
| 2026-09-12 | **Re-run at schema 40** (Track F), `local` provider, source = manual-testing DB with real transactional data: `nightly-backup.sh` → sha256 vs manifest ✓ → `restore.sh` into a fresh DB → 59/59 tables row-identical, 168 indexes / 214 constraints / 36 sequences / 14 enums / 1 trigger identical, 10/10 money tables content-checksum identical, all 36 sequence `last_value`s preserved, immutability trigger verified to enforce. Retention pruning and `--single-transaction` atomicity exercised, not assumed. Restore timed at 297 MB / 500K audit rows. | **PASS** on the backup→restore loop. **FAIL** on restore-and-replay: migration 22 cannot be re-applied to a populated `audit_log` and wedges the app (`dirty=true`). Found: R-1 `restore.sh` lacks `--no-owner` (a rebuilt host cannot be restored to); R-2 host `backup.sh` broken by pg_dump 16 vs server 17; R-3 backup failure metric not written when `DATABASE_URL` is unset. [history/restore-verification-report-2026-09-12.md](history/restore-verification-report-2026-09-12.md) |

Re-run the §3a drill: within week 1 of go-live (at real data volume — record the duration to update RTO), after any bucket rename, and quarterly.

**Abort criteria that depend on this file:** [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md)
A4 (data loss / stale backup) and §5 (recovery ladder). The backup alerts have a known blind
spot — `qr_dining_backup_last_run_status` keeps reporting the last good run when `DATABASE_URL`
is unset, so read `qr_dining_backup_last_success_timestamp_seconds` directly rather than trusting
the absence of a page.
