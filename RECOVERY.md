# RECOVERY.md — QR-Dining Backup & Disaster Recovery

**Canonical recovery guide.** Companions: `DEPLOYMENT.md`, `OPERATIONS.md`. Historical certification: `restore-verification-report.md` (local-provider round trip, 2026-06-09).

---

## 1. What exists

- **Nightly dump** (02:30, systemd timer → `backend/scripts/nightly-backup.sh`): `pg_dump --format=custom --compress=9` → sha256 → upload to R2 → append-only `manifest.jsonl` → retention prune (14 days) → Prometheus textfile metrics. Any failure exits non-zero and sets `qr_dining_backup_last_run_status 1`.
- **Bucket layout** (`R2_BUCKET`, currently the temporary `qr-dining-backups`):
  - `backups/YYYY/MM/backup_YYYYMMDD_HHMMSS.dump` (UTC-named, one per run)
  - `backups/manifest.jsonl` — one JSON line per backup: `{timestamp,key,bytes,sha256,provider,retention_days}`. **The manifest is the index of record**: pick restore candidates from it and always verify sha256 after download.
- **Provider abstraction** (`backend/scripts/lib/storage.sh`): `BACKUP_PROVIDER=r2|s3|local`, same code path (AWS CLI) for both clouds. Nothing is bucket-name-aware beyond env.
- **RPO: ≤24h** (nightly). **RTO: minutes** for a DB-only restore at pilot volume (the verified drill below completed in seconds on a small dataset; re-measure at production volume), **~1–2h** for full-host rebuild via DEPLOYMENT.md.

## 2. Choosing a backup

```bash
export AWS_ACCESS_KEY_ID=<R2_ACCESS_KEY> AWS_SECRET_ACCESS_KEY=<R2_SECRET_KEY>
EP=--endpoint-url=https://<account_id>.r2.cloudflarestorage.com
aws s3 cp s3://$R2_BUCKET/backups/manifest.jsonl - $EP --region auto | tail -5   # newest last
aws s3 cp s3://$R2_BUCKET/<key-from-manifest> ./restore-candidate.dump $EP --region auto
sha256sum restore-candidate.dump    # MUST equal the manifest line's sha256 — stop if not
```

## 3. Restore procedures (verified pattern)

`backend/scripts/restore.sh <dump>` — **destructive**: `pg_restore --clean --if-exists --single-transaction` into `DATABASE_URL`, then row-count validation. Single-transaction = all-or-nothing.

### 3a. Drill / side-restore (non-destructive — also the quarterly drill)

Runs everything inside containers; needs no host pg tools. This exact procedure passed against the real R2 bucket on 2026-07-18 (§5):

```bash
docker network create qr-restore-drill
docker run -d --name qr-drill-db --network qr-restore-drill \
  -e POSTGRES_USER=bv -e POSTGRES_PASSWORD=drill -e POSTGRES_DB=bvdb postgres:17-alpine
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
5. `docker compose start app` → app re-migrates idempotently if the dump predates newer migrations → `/readyz` 200.
6. Verify: row counts (restore.sh prints the six core tables), one full guest journey, audit_log immutability intact (an UPDATE attempt on audit_log must fail).

## 4. Disaster scenarios

| Scenario | Action |
|---|---|
| Bad deploy / app regression | Not a restore case: pin previous `IMAGE_TAG`, `docker compose up -d app` (OPERATIONS.md §2) |
| Postgres data corruption / bad mutation | §3b. Data loss bounded by last nightly (≤24h RPO) |
| `qr-dining_postgres_data` volume lost | `docker compose up -d postgres` (fresh volume) → §3b into the empty DB |
| Redis volume lost | Nothing to restore — Redis is disposable by design; restart it, realtime self-recovers |
| Whole VM lost | Rebuild per DEPLOYMENT.md (proxy stack first) → §3b with the newest manifest entry. Secrets must come from the password manager — they are not in git |
| R2 bucket lost / creds leaked | Backups are the *copy*; the live DB is intact. Create bucket + new scoped token, update `/etc/qr-dining/backup.env`, run one manual backup, roll the leaked token |
| Backup job silently broken | `BackupTooOld` pages at >36h; `journalctl -u qr-dining-backup` for the failing step; every step is fail-hard so the journal names it |

## 5. Verification log

| Date | What | Result |
|---|---|---|
| 2026-06-09 | Full round trip, `local` provider (Phase E cert) | PASS — 56/56 tables row-identical, trigger + checksum fidelity (`restore-verification-report.md`) |
| 2026-07-18 | **Full round trip against the real R2 bucket** (temporary `qr-dining-backups`, account `16465dc4…`): containerized pg_dump 17 → upload → manifest append → fresh download → sha256 vs manifest (`fc5a2ecd…` ✓) → `restore.sh` into a second postgres:17 → content checksum source vs restored (`a2926ca4…` = `a2926ca4…` ✓) → retention pass, textfile metrics `status 0` + success timestamp | **PASS** — closes the "never tested against real R2" SEV-1. Found+fixed en route: BusyBox-incompatible `date` in the retention step (now epoch-based, portable) |

Re-run the §3a drill: within week 1 of go-live (at real data volume — record the duration to update RTO), after any bucket rename, and quarterly.
