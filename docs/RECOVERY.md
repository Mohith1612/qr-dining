# Recovery

Last verified: 2026-09-13.

This is the destructive database-recovery procedure. A bad application deploy is
an image rollback, not a database restore; the production compose file pins the
application image through `IMAGE_TAG` (`deploy/vm/docker-compose.yml:43-51`).

Legacy backup-test output may say “RECOVERY.md §3a”; that pointer means
[Rehearsed scratch restore](#rehearsed-scratch-restore).

## Non-negotiable constraint: forward fixes only

Do not roll the production schema down. The down migrations from 16 through 24
remove identity, tenancy, audit, realtime, idempotency, billing-snapshot, and
session-lifecycle state (`backend/migrations/000016_identity_hardening.down.sql:1-17`,
`backend/migrations/000017_organization_model.down.sql:1-19`,
`backend/migrations/000018_platform_trust_domain.down.sql:1-13`,
`backend/migrations/000019_audit_log_v2.down.sql:1-7`,
`backend/migrations/000020_realtime_session_hardening.down.sql:1-2`,
`backend/migrations/000021_payment_order_correctness.down.sql:1-34`,
`backend/migrations/000022_operational_ux_cleanup.down.sql:1-21`,
`backend/migrations/000023_session_lifecycle_states.down.sql:1-8`,
`backend/migrations/000024_session_lifecycle_invariants.down.sql:1-5`). Migration
36 deletes payment-time promo redemptions, migration 38 deletes the Serene theme
preset and assignments, and migration 39 has no reversible down operation
(`backend/migrations/000036_promo_redemption_payment.down.sql:1-9`,
`backend/migrations/000038_serene_theme_preset.down.sql:1-4`,
`backend/migrations/000039_normalize_promo_redemption_phones.down.sql:1-4`).

Restore a dump at the schema version it contains. Do not roll an existing
database backward to meet an older binary. The restore script prints the
restored version and dirty flag after its row-count check
(`backend/scripts/restore.sh:119-136`).

## Backup artifact and selection

`nightly-backup.sh` creates a compressed custom-format dump, computes SHA-256,
uploads the dump, appends its key, size, checksum, provider, and retention to
`manifest.jsonl`, then prunes old objects (`backend/scripts/nightly-backup.sh:94-145`).
The manifest is append-only while retention deletes objects, so old manifest
lines can be tombstones; confirm the chosen object exists before downloading it
(`backend/scripts/nightly-backup.sh:113-145`).

Before any restore:

1. Put the restaurant on its documented fallback and stop application writes.
   The pilot stop/abort thresholds are in
   [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md).
2. Select an existing dump object, download it, and recompute SHA-256. Compare
   that value with the same object's manifest entry. The backup writes the
   checksum over the dump before upload (`backend/scripts/nightly-backup.sh:101-124`).
3. Record the dump's `schema_migrations` version after restore and do not start
   a binary whose expected schema differs (`backend/scripts/restore.sh:132-136`).

The storage adapter supports `r2`, `s3`, and `local`; each provider's required
variables and commands are implemented in `backend/scripts/lib/storage.sh:8-118`.

## Rehearsed scratch restore

Use a disposable PostgreSQL 17 target. The regression harness uses two isolated
`postgres:17-alpine` containers and deliberately gives the target a different
owner role (`backend/scripts/tests/backup-restore-test.sh:12-18`,
`backend/scripts/tests/backup-restore-test.sh:65-87`). Run:

```bash
cd backend
scripts/tests/backup-restore-test.sh
```

The harness skips successfully when Docker or Go is absent, so a zero exit is
not sufficient by itself; read its output and require a non-zero pass count and
zero failures (`backend/scripts/tests/backup-restore-test.sh:51-52`,
`backend/scripts/tests/backup-restore-test.sh:310-311`). It verifies:

- default restore onto a host without the source owner, including table and
  audit-row counts, clean schema version, object ownership, and the audit trigger
  (`backend/scripts/tests/backup-restore-test.sh:109-149`);
- failure metrics for missing configuration, an unreachable database, and an
  invalid provider (`backend/scripts/tests/backup-restore-test.sh:151-198`);
- the migration-22 dirty wedge and its full recovery
  (`backend/scripts/tests/backup-restore-test.sh:200-263`); and
- explicit behavior with no terminal, `--yes`, `FORCE=1`, piped confirmation,
  refusal, and bad options (`backend/scripts/tests/backup-restore-test.sh:265-308`).

## Production restore

This operation replaces the target database. `restore.sh` uses `--clean`,
`--if-exists`, and `--single-transaction` (`backend/scripts/restore.sh:108-117`).

1. Stop the application so no writes race the restore:

   ```bash
   cd /opt/qr-dining
   docker compose stop app
   ```

2. Take a best-effort pre-restore dump if PostgreSQL is readable, and retain the
   incident copy separately. The supported automated backup invocation and its
   required `DATABASE_URL` are defined by
   `backend/scripts/nightly-backup.sh:6-15,82-102`.

3. Restore the verified artifact using PostgreSQL 17 client tools:

   ```bash
   DATABASE_URL="$TARGET_DATABASE_URL" backend/scripts/restore.sh --yes /path/to/backup.dump
   ```

   `--yes` (or `FORCE=1`) is required without an interactive input stream
   (`backend/scripts/restore.sh:7-17,89-105`). The default omits original ownership
   and privileges so a rebuilt host need not contain the source role; use
   `--preserve-owner` only when those roles exist and exact grants are required
   (`backend/scripts/restore.sh:19-26,77-82`).

4. Require the restore command to exit zero. Review its counts for `sessions`,
   `orders`, `payments`, `staff`, `menu_items`, and `event_log`, and require
   `dirty=false` in the printed migration state (`backend/scripts/restore.sh:119-136`).

5. Verify the audit trigger before reopening. This statement must fail:

   ```sql
   UPDATE audit_log
   SET action = 'recovery-trigger-check'
   WHERE id = (SELECT min(id) FROM audit_log);
   ```

   The trigger rejects every update or delete
   (`backend/migrations/000019_audit_log_v2.up.sql:75-84`). If `audit_log` is empty,
   verify the trigger exists in `pg_trigger` instead; do not insert synthetic
   production audit data.

6. Start the application, then require `/readyz` to succeed. The route is wired
   separately from `/health` (`backend/internal/server/server.go:187-197`).

## Known dirty-migration wedge at version 22

Migration 22 updates all existing `audit_log` rows
(`backend/migrations/000022_operational_ux_cleanup.up.sql:102-113`), while migration
19 installed a trigger that rejects all updates and deletes
(`backend/migrations/000019_audit_log_v2.up.sql:75-84`). Replaying migration 22
against a populated log therefore fails and leaves golang-migrate dirty. Empty-DB
CI does not exercise that population-dependent collision; its migration job
creates a fresh database before `up`, `down`, and `up`
(`.github/workflows/ci.yml:125-157`).

Use this only for the confirmed `version=22 dirty=true` case:

```bash
# Inspect; do not guess the recorded state.
DATABASE_URL="$DATABASE_URL" go run ./backend/cmd/migrate version

# Take an incident backup first. Then change only the recorded version.
DATABASE_URL="$DATABASE_URL" go run ./backend/cmd/migrate force 21
```

`force` changes only migration metadata; it neither applies nor undoes SQL
(`backend/cmd/migrate/main.go:55-80,96-110`). Running `up` now without addressing
the trigger simply wedges at 22 again; the regression test asserts that behavior
(`backend/scripts/tests/backup-restore-test.sh:241-246`). Record the start of the
tamper-evidence gap, then:

```sql
DROP TRIGGER trg_audit_log_immutable ON audit_log;
```

```bash
DATABASE_URL="$DATABASE_URL" go run ./backend/cmd/migrate up
```

Immediately restore and test the trigger:

```sql
CREATE TRIGGER trg_audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

-- This must fail.
UPDATE audit_log SET action = 'x'
WHERE id = (SELECT min(id) FROM audit_log);
```

The complete drop/replay/re-create/test sequence is exercised by
`backend/scripts/tests/backup-restore-test.sh:248-263`. Keep the restaurant closed
until enforcement is restored, and record the exact trigger-free window in the
incident report.

Migration 40 has a different data-dependent guard: it raises when a session
already has multiple non-terminal payments before creating the partial unique
index (`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`).
Do not `force` through that guard; reconcile the duplicate payments with recorded
operator decisions before applying the migration.

## Track F verification record

The 2026-09-12 rehearsal used a live schema-40 source, generated a dump with the
production backup script, recomputed the stored object's checksum, restored into
a fresh PostgreSQL 17 database, and compared source and target
(`docs-before-rebuild:docs/history/restore-verification-report-2026-09-12.md:15-37,39-71`).
The measured small-database timings were 218/236/230 ms for `pg_dump`,
532/521/485 ms for `pg_restore`, 2.88 s for the complete backup script, and
3.18 s for the complete restore script
(`docs-before-rebuild:docs/history/restore-verification-report-2026-09-12.md:102-113`).
On an inflated 297 MB scratch database, `pg_dump` took 2.828/3.001 s,
`pg_restore` took 12.836/8.312 s, and SHA-256 took 114 ms
(`docs-before-rebuild:docs/history/restore-verification-report-2026-09-12.md:114-124`).
These are measurements from one development host, not production RTO guarantees
(`docs-before-rebuild:docs/history/restore-verification-report-2026-09-12.md:102-130`).

The restored copy matched all 59 source-table row counts, all inspected schema
object counts, all 10 money-bearing table content hashes, and all 36 sequence
values; the audit trigger and migration-40 payment index were present, and a
failed single-transaction restore left no partial schema
(`docs-before-rebuild:docs/history/restore-verification-report-2026-09-12.md:132-190`).

That rehearsal found four tool defects: fresh-host ownership failure, missing
failure metrics for pre-validation exits, silent no-stdin restore failure, and no
in-tree dirty-version recovery command. The current fixes are respectively the
default ownership flags, an early `EXIT` trap, explicit `--yes`/`FORCE=1` plus an
EOF error, and `migrate version`/`force`
(`backend/scripts/restore.sh:7-26,77-105`,
`backend/scripts/nightly-backup.sh:27-79`,
`backend/cmd/migrate/main.go:42-80`). Incorrect deploy comments about the failure
signal were an additional documentation defect and are corrected to point at the
textfile metric emitted by the script (`backend/scripts/nightly-backup.sh:27-75`).

The original evidence, including command output, remains retrievable from the
immutable baseline tag:

```bash
git show docs-before-rebuild:docs/history/restore-verification-report-2026-09-12.md
```
