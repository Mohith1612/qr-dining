# Restore Verification Report — schema 40

**Date:** 2026-09-12
**Phase:** Track F — restore rehearsal and abort criteria
**Supersedes for currency:** [restore-verification-report.md](restore-verification-report.md) (2026-06-09, schema 33)
**Result:** ✅ **PASS on the verified loop** — backup → checksum → restore → compare is byte-faithful at schema 40.
**Result:** ❌ **FAIL on restore-and-replay** — a restored database cannot be migrated forward through
migration 22 while `audit_log` holds rows. This wedges the app. See Part 2.

The June report's method was followed: a live dump produced by `nightly-backup.sh`, its checksum
verified against the manifest, restored with `restore.sh` (`pg_restore --clean --if-exists
--single-transaction`) into a brand-new database, then compared table-for-table against source.
Everything below is verbatim command output.

## Environment (isolated — soak and manual-testing untouched)

| Component | Value |
|-----------|-------|
| Source DB | `qrdining_mtest` on `manual-testing-postgres` (postgres:17.10-alpine, port 25432) — **read-only: `pg_dump` only, no writes** |
| Target DBs | `restore_target`, `replay_target`, `volume_target` on a throwaway `restore-rehearsal-pg` (postgres:17.10-alpine, port 35433) |
| Backup tool | `backend/scripts/nightly-backup.sh`, `BACKUP_PROVIDER=local` (same code path as R2/S3) |
| Restore tool | `backend/scripts/restore.sh`, run in a `postgres:17-alpine` container |
| Migrate tool | `backend/cmd/migrate` built from this tree (the same `migrate.Up()` the app calls at startup) |
| Soak DB | `qr-dining-postgres-1` — found stopped, **never started, never touched** |

**The source was a live database**, with the manual-testing backend still running against it. Its
payment-escalation worker wrote 7 `payment.settlement.stalled` audit rows during the rehearsal
window (264 at dump time → 271 at the end), which is why a later `count(*)` on the source does not
match the dump. That is the realistic case, not a contamination: a nightly backup always runs
against a database that is still being written to. No step of this rehearsal wrote to
`qrdining_mtest`.

**Deviation from the June method:** June inserted sentinel rows into the source before dumping.
That would have been a write to `qrdining_mtest`, which the manual-testing stack is using, so
sentinels were replaced with a stronger check — a per-table **content checksum** (md5 over the
full ordered row text) on every money-bearing table. That proves the same thing sentinels did and
more.

## 1. Does the backup script still produce a valid dump at schema 40?

**Yes.**

```
storage: provider=local target=/bucket
[1/5] pg_dump -> /scratch/backup_20260912_155038.dump
      dump size: 276913 bytes
[2/5] checksum
      sha256: 7bcb546a225c53b8996c3b793541268ce816ec6beb9fdeb065221fab6e44b0ff
[3/5] upload -> backups/2026/09/backup_20260912_155038.dump
[4/5] manifest entry
      appended: {"timestamp":"2026-09-12T15:50:38Z","key":"backups/2026/09/backup_20260912_155038.dump","bytes":276913,"sha256":"7bcb546a225c53b8996c3b793541268ce816ec6beb9fdeb065221fab6e44b0ff","provider":"local","retention_days":14}
[5/5] retention (>14 days)
      remote pruned: 0
OK: backup complete -> backups/2026/09/backup_20260912_155038.dump (276913 bytes)

WALL_CLOCK_BACKUP=2.88 s
```

## 2. Checksum / manifest verification — actually run

Recomputed against the **stored object**, not the scratch copy:

```
key:        backups/2026/09/backup_20260912_155038.dump
manifest:   7bcb546a225c53b8996c3b793541268ce816ec6beb9fdeb065221fab6e44b0ff
recomputed: 7bcb546a225c53b8996c3b793541268ce816ec6beb9fdeb065221fab6e44b0ff
→ CHECKSUM MATCH

/bucket/backups/2026/09/backup_20260912_155038.dump: PostgreSQL custom database dump - v1.16-0
276913 bytes
```

Prometheus textfile metrics were emitted:

```
qr_dining_backup_last_run_status 0
qr_dining_backup_last_success_timestamp_seconds 1789228238
```

**Retention was exercised, not just read.** A 20-day-old dump was planted in the bucket and the
job re-run with `RETENTION_DAYS=14`:

```
[5/5] retention (>14 days)
      pruned remote: backups/2026/08/backup_20260823_020000.dump
      remote pruned: 1
```

The object is deleted but **its manifest line is not** — the manifest is append-only, so entries
older than `RETENTION_DAYS` are tombstones pointing at objects that no longer exist:

```
  OK       backups/2026/09/backup_20260912_155038.dump
  DANGLING backups/2026/08/backup_20260823_020000.dump
  OK       backups/2026/09/backup_20260912_155630.dump
```

This is by design but is not documented, and `RECOVERY.md` §2 calls the manifest "the index of
record". An operator picking any entry other than the newest can be handed a dead key. Now noted
in RECOVERY.md §2.

## 3. Does the restore complete, and how long does it take?

**Yes.** Wall-clock, on this dev host (amd64, NVMe) at the manual-testing data volume
(13 MB database, 59 tables):

| Step | Measured |
|---|---|
| `pg_dump --format=custom --compress=9` | 218 / 236 / 230 ms |
| `pg_restore --clean --if-exists --single-transaction` | 532 / 521 / 485 ms |
| `nightly-backup.sh` end-to-end in a throwaway container | 2.88 s |
| `restore.sh` end-to-end in a throwaway container | 3.18 s |

The June report deferred this: *"this proves logical correctness and schema/trigger fidelity, not
restore duration at production volume — re-time the restore once real data exists."* That is now
measured. A scratch DB was inflated to **297 MB / 500,264 `audit_log` rows / 150,159 `event_log`
rows** — roughly 90 days of single-site pilot audit volume against migration 19's own planning
figure of ~10K audit rows/day:

| Step at 297 MB | Measured |
|---|---|
| `pg_dump --compress=9` | 2,828 / 3,001 ms (dump = 16,305,820 bytes) |
| `pg_restore --clean --single-transaction` | 12,836 / 8,312 ms |
| `sha256sum` of the dump | 114 ms |

**RTO input for the abort criteria:** the mechanical restore of a 90-day pilot database is
**~10–15 s of `pg_restore`** on this hardware. The production target is an Oracle Cloud Ampere
arm64 VM sharing a host with four other projects; assume **2–3× slower, so 30–45 s**, plus a
16 MB download from R2 and the app's ~12 s restart. **Budget 5 minutes for the mechanical path.**
An incident is dominated by decision and verification time, not by `pg_restore`.

## 4. Table-for-table comparison against source

```
=== TABLE-FOR-TABLE ROW COUNT DIFF (source vs target) ===
(empty diff) — IDENTICAL across 59 tables

source tables: 59
target tables: 59
```

Schema object classes, source vs target:

```
constraints    source=214    target=214    OK
enum_types     source=14     target=14     OK
functions      source=1      target=1      OK
indexes        source=168    target=168    OK
sequences      source=36     target=36     OK
triggers       source=1      target=1      OK
views          source=0      target=0      OK
```

Per-table **content** checksums (md5 over the full ordered row text) on every money-bearing table:

```
audit_log          4fd507635cf5e5a7  4fd507635cf5e5a7  MATCH
bill_snapshots     4c9865e549e99c78  4c9865e549e99c78  MATCH
idempotency_keys   b5454d1dd3240f0b  b5454d1dd3240f0b  MATCH
order_items        4301d5036a7b892d  4301d5036a7b892d  MATCH
orders             1e46b9a56e55ff6d  1e46b9a56e55ff6d  MATCH
payment_sequences  de34b831568e51d5  de34b831568e51d5  MATCH
payments           ff511697a12c43a4  ff511697a12c43a4  MATCH
session_sequences  88c068fa4af8b1e2  88c068fa4af8b1e2  MATCH
sessions           81e2f9325d9961d4  81e2f9325d9961d4  MATCH
staff              95480fa655ea3dcc  95480fa655ea3dcc  MATCH
```

All **36 sequence `last_value`s** round-tripped (`diff` empty). This matters more than it looks:
`audit_event_reference_seq` is at 264 in both. A restore that reset sequences would reissue
receipt and audit references that already exist on paper.

Migration state, trigger fidelity, and enforcement:

```
source schema_migrations: 40|f
target schema_migrations: 40|f

trg_audit_log_immutable

ERROR:  audit_log rows are immutable: UPDATE is not permitted
CONTEXT:  PL/pgSQL function audit_log_immutable() line 3 at RAISE

idx_payments_one_non_terminal_per_session      (migration 40 — newest schema object)
idx_sessions_one_active_per_table
```

**`--single-transaction` atomicity was verified, not assumed.** The first restore attempt failed
(§6, finding R-1); afterwards the target held `0` tables and `0` enum types. A failed restore
leaves no half-restored database.

## 5. Diff against the June 2026-06-09 report

| | June (schema 33) | Now (schema 40) |
|---|---|---|
| Migrations | 33, dirty=false | 40, dirty=false |
| Tables | 56 | **59** |
| Dump size | 199,626 bytes | 276,913 bytes |
| Row-count diff | empty (56/56) | empty (59/59) |
| Non-empty tables | 6 | **42** (real session/order/payment/audit data, not just seed) |
| `audit_log` rows | 0 | **264** |
| Checksum vs manifest | MATCH | MATCH |
| Immutability trigger | present | present **and verified to enforce** |
| Sequence fidelity | not checked | **36/36 round-tripped** |
| Content checksums | not checked (sentinels instead) | **10/10 money tables byte-identical** |
| Restore duration at volume | explicitly deferred | **measured (§3)** |
| Restore-and-replay through migration 22 | not exercised | **FAILS (Part 2)** |

**Nothing in the backup/restore mechanism regressed between 33 and 40.** Every property June
verified still holds, on a database with seven more migrations, three more tables and real
transactional data instead of seed rows. The failures below are not regressions — they are
defects the June method did not exercise, plus three unsafe migrations added after June (36, 38,
39) and one new guarded up-migration (40).

## 6. Findings — things that do not work

### R-1 (high) — `restore.sh` has no `--no-owner`; a restore onto a fresh host aborts

The first restore attempt, into a server identical except that the dump's role did not exist:

```
Restoring...
pg_restore: error: could not execute query: ERROR:  role "mtest" does not exist
Command was: ALTER TYPE public.assistance_status OWNER TO mtest;
```

`--single-transaction` means this is not a partial failure — **nothing is restored at all.**

This is exactly the whole-VM-loss path in `RECOVERY.md` §4 ("Rebuild per DEPLOYMENT.md → §3b with
the newest manifest entry"): a rebuilt host has a fresh Postgres, and if the app role is not
created with the same name before the restore, the restore fails completely.

It also breaks the **certified quarterly drill as written**. `RECOVERY.md` §3a stands up
`-e POSTGRES_USER=bv -e POSTGRES_DB=bvdb` and restores a production dump (owned by `qrdining`)
into it. That is the reproduction above. Fixed in the docs by adding `--no-owner --no-privileges`
to the drill; `restore.sh` itself needs a code change (see "Needs code", below).

### R-2 (medium) — `backup.sh` cannot run on this host at all

`RECOVERY.md` §3c tells a developer to run `backend/scripts/backup.sh` on the host:

```
host pg_dump: pg_dump (PostgreSQL) 16.15 (Ubuntu 16.15-0ubuntu0.24.04.1)
server:       17.10

Starting backup → .../backup_20260912_212254.dump
pg_dump: error: aborting because of server version mismatch
pg_dump: detail: server version: 17.10; pg_dump version: 16.15
EXIT=1
```

Not a schema-40 regression — the June rehearsal ran everything in containers and never exercised
the host path. It is a live trap for anyone following §3c today. `production-environment-checklist.md`
already lists "postgresql-client **17**" as a host requirement for the VM; the same requirement
was never stated for developer machines. Now stated in RECOVERY.md §3c.

### R-3 (high) — a backup can fail without setting the failure metric

`BackupFailed` fires on `qr_dining_backup_last_run_status != 0`. Two failure modes, tested:

**Mode B — database unreachable (pg_dump fails mid-run). Handled correctly:**
```
[1/5] pg_dump -> /scratch/backup_20260912_155950.dump
pg_dump: error: connection to server ... FATAL:  database "nonexistent_db" does not exist
EXIT=1
status metric AFTER:   qr_dining_backup_last_run_status 1
success metric:        qr_dining_backup_last_success_timestamp_seconds 1789228590   (preserved)
```

**Mode A — `DATABASE_URL` missing (env file absent, unreadable, or wrong). NOT handled:**
```
current status metric (from the last GOOD run):
qr_dining_backup_last_run_status 0

ERROR: DATABASE_URL is not set
EXIT=1
status metric AFTER the failure:
qr_dining_backup_last_run_status 0
```

The guard is an explicit `exit 1` placed **before** `trap 'write_status_metric 1' ERR` is
installed, and bash does not run an `ERR` trap for `exit` anyway. The metric keeps reporting the
last good run. `BackupFailed` never fires; only `BackupTooOld` catches it, **36 hours later**.
The same 36-hour blind spot covers "cron never fired at all" — which `deploy/backup/README.md`
itself flags as likely, since the rootless path uses a user crontab that does not survive a VM
rebuild.

### R-4 (low) — the alerting comments in the deploy units are wrong

`crontab.example` claims a failed run "leaves a marker file that the BackupFailed alert watches".
Nothing watches `/var/run/qr-dining-backup.failed`. `qr-dining-backup.service` claims failure is
"surfaced to ... node_exporter's systemd collector, which the BackupFailed alert watches";
node_exporter is started with `--collector.textfile.directory` only and no systemd collector:

```
  node-exporter:
    command:
      - --collector.textfile.directory=/var/lib/node_exporter/textfile
```

The only failure signal is the textfile metric the script writes itself. Comments corrected.

### R-5 (low) — `restore.sh` aborts silently with no controlling terminal

```
WARNING: This will replace all data in the target database.
EXIT=1
```

`read -r -p` returns non-zero on EOF and `set -e` exits before the "Aborted." message. Safe
direction, but the operator sees no reason. Documented; the `echo yes |` form in RECOVERY.md §3a
already works around it.

### R-6 (low) — stale documentation pointer

`deploy/backup/README.md:122` referenced `restore-verification-report.md` relative to
`deploy/backup/`, where it does not exist. Fixed to `../../docs/history/`. The rootless section
of the same file also told the operator to source `/etc/qr-dining/backup.env` in a procedure
whose whole point is that `/etc` is unavailable; corrected to `/opt/qr-dining/backup.env`.

### Needs code (not changed here — scripts/docs/config only)

1. **`restore.sh`: add `--no-owner --no-privileges`** (or an opt-out flag). This is R-1 and it is
   the single highest-value fix in the backup path.
2. **`nightly-backup.sh`: move the `DATABASE_URL` guard after the metric helpers and write
   `write_status_metric 1` before exiting** (R-3), so an unreadable env file pages in minutes
   rather than in 36 hours.
3. **`cmd/migrate`: add a `force <version>` subcommand.** Recovering the dirty state in Part 2
   currently requires hand-written SQL against `schema_migrations` during an incident.
4. **`restore.sh`: accept `--yes`/`FORCE=1`** so a non-interactive restore states its intent
   instead of failing at a `read` (R-5).

## Part 2 — migration 22, empirically

`RECOVERY.md` §3b step 5 said: *"`docker compose start app` → app re-migrates idempotently if the
dump predates newer migrations."* That claim is false, and this is what it costs.

**Setup.** `replay_target`, restored from the same dump: schema 40, **264 `audit_log` rows**, 23
sessions, 16 payments.

**Step 1 — roll back 40 → 21.** Every down migration succeeded mechanically, as CI says they do:

```
step 1   40/false -> 39/false  rc=0  done
step 2   39/false -> 38/false  rc=0  done
...
step 19  22/false -> 21/false  rc=0  done
```

**What that cost, at version 21:**

```
audit_log_rows|264                        ← rows survive
sessions_rows|23
payments_rows|16
session_number_col_exists|0               ← every session number destroyed
payment_reference_col_exists|0            ← every payment reference destroyed
audit_event_reference_col_exists|0        ← every audit reference destroyed
session_sequences_table_exists|0
payment_sequences_table_exists|0
audit_immutability_trigger|1              ← trigger survives. This is the trap.
tables_now|37
```

**Step 2 — migrate back up. This is the failure.**

```
migrate error: migration failed in line 0: -- Phase 8: Operational UX Cleanup
  [...full text of 000022_operational_references.up.sql...]
 (details: ERROR: audit_log rows are immutable: UPDATE is not permitted (SQLSTATE P0001))
EXIT=1

=== schema_migrations after the attempt ===
22|t
```

`000022:109` runs `UPDATE audit_log SET event_reference = ...` against the
`BEFORE UPDATE OR DELETE ... RAISE EXCEPTION` trigger created at `000019:82`. Migration 22 rolled
back atomically — no partial schema, `audit_event_reference_seq exists: 0` — but golang-migrate
left **`version=22, dirty=true`**.

*(The brief cited `000022:108`; the `UPDATE` is at line 109 in this tree. Same statement.)*

**Step 3 — the consequence. The app will not start.** `RunMigrations` calls the same `m.Up()`:

```
=== retry: what happens on the NEXT app start (same m.Up() call path)? ===
migrate error: Dirty database version 22. Fix and force version.
EXIT=1

=== and can it roll itself back out? ===
migrate error: Dirty database version 22. Fix and force version.
EXIT=1
```

`cmd/migrate` only implements `up` and `down`. There is no `force`. The system is wedged with no
in-tree tool to unwedge it.

**Step 4 — clearing `dirty` by hand is not enough.** `schema_migrations` is an ordinary table:

```
UPDATE schema_migrations SET version=21, dirty=false;   → UPDATE 1
./migrate up  →  ERROR: audit_log rows are immutable: UPDATE is not permitted
now at: 22 dirty=true
```

It re-wedges immediately.

**Step 5 — the only forward path, tested end to end:**

```
UPDATE schema_migrations SET version=21, dirty=false;
DROP TRIGGER trg_audit_log_immutable ON audit_log;
./migrate up            →  done
now at: 40 dirty=false
```

then re-create it and confirm it bites again:

```
CREATE TRIGGER trg_audit_log_immutable
  BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
→ CREATE TRIGGER
UPDATE audit_log SET action='x' ...
→ ERROR:  audit_log rows are immutable: UPDATE is not permitted
```

**So: recovering from a rollback past 22 requires dropping the audit tamper-evidence guarantee,
by hand, during an incident, on a live database.** For the duration of the replay, `audit_log` is
writable. `OPERATIONS.md` §11 says "Never mutate `audit_log`" and `RUNBOOKS.md` §7 uses "an
UPDATE against audit_log must fail" as a post-restore verification step — this procedure
deliberately breaks both, then restores them. It is in `RUNBOOKS.md` §8 now as a last resort,
flagged as such.

### Why CI never caught this

```yaml
- name: Run migrations up, down, and up again
  run: |
    ... migrate ... up
    ... migrate ... down -all
    ... migrate ... up
```

against an **empty** database. With no rows in `audit_log`, `UPDATE audit_log` matches nothing,
the `FOR EACH ROW` trigger never fires, and migration 22 passes. It will pass forever. The
`RUNBOOKS.md` §8 claim that downs "are exercised in CI, so they work mechanically" is true and
dangerously incomplete: CI proves the DDL parses, not that the migration survives contact with
data.

### The other rollback hazards, verified

**Migration 36's down destroys money-adjacent data.** Two redemptions were seeded — one
payment-time, one order-time — and 36 rolled back:

```
before:  id=1 payment_time=t payment_id=1 phone=+919999900001
         id=2 payment_time=f payment_id=    phone=+919999900002

40 -> 39  rc=0 promo_redemptions=2
39 -> 38  rc=0 promo_redemptions=2
38 -> 37  rc=0 promo_redemptions=2
37 -> 36  rc=0 promo_redemptions=2
36 -> 35  rc=0 promo_redemptions=1      ← DELETE FROM promo_redemptions WHERE order_id IS NULL
```

Rolling forward does not bring it back:

```
 id | payment_time | payment_id |  phone_e164
----+--------------+------------+---------------
  2 | f            |            | +919999900002
```

The guest received the discount — it is in the immutable bill snapshot — but the record that
enforces `uses_per_phone` is gone. The same promo can be redeemed again on the same phone.

**Migration 22's regenerated references diverge under row loss.** On an untouched dataset the
backfill is deterministic and regenerated every reference identically (`diff` empty). It diverges
the moment the row population differs. Simulating one purged session in a six-session partition:

```
 printed_on_the_receipt  |      after_replay       |      verdict
-------------------------+-------------------------+--------------------
 SAFF-BND-S-20260912-001 | SAFF-BND-S-20260912-001 | same
 SAFF-BND-S-20260912-003 | SAFF-BND-S-20260912-002 | *** REASSIGNED ***
 SAFF-BND-S-20260912-004 | SAFF-BND-S-20260912-003 | *** REASSIGNED ***
 SAFF-BND-S-20260912-005 | SAFF-BND-S-20260912-004 | *** REASSIGNED ***
 SAFF-BND-S-20260912-006 | SAFF-BND-S-20260912-005 | *** REASSIGNED ***
```

Four of five visit numbers reassigned by one missing row. Any receipt already handed to a guest,
and any support ticket quoting a session number, now points at a different visit.

**Migration 20's down leaves no one-session-per-table guard.** `idx_sessions_one_active_per_table`
is created by 20 and re-created by 24; both downs drop it. Below 20 the only index on
`sessions(table_id)` is `idx_sessions_table_id` from `000001:148`, which is **not unique**. Two
active sessions can then exist on one table — two parties ordering into one bill.

**Migration 40's up is a new replay hazard since June.** `000040:19` raises
`cannot enforce one non-terminal payment per session` if any session has more than one
non-terminal payment. A dump taken *before* 40 was applied can contain exactly that — it is why
40 exists. Replaying such a dump forward fails and leaves the same `dirty=true` wedge as 22.

### Conclusion for Part 2

**The pilot rollback story is forward-fix only, and "restore an old backup and let the app
re-migrate" is not a supported path.** A restore is safe only into the schema version the dump
was taken at. This is now written into `RECOVERY.md` §3b, `RUNBOOKS.md` §8, `OPERATIONS.md` §2
and `PILOT-ABORT-CRITERIA.md` rather than living in one auditor's head.

## 7. What I would not trust in an incident

1. **`restore.sh` onto a rebuilt host** (R-1). The one scenario where you must restore from
   scratch is the one the script fails at, and it fails all-or-nothing.
2. **`BackupFailed` as proof that backups are running** (R-3). It cannot distinguish "healthy"
   from "the env file is gone" or "cron never fired". Only `BackupTooOld` can, 36 hours late.
   Treat the backup as unverified until `qr_dining_backup_last_success_timestamp_seconds` has
   been read directly.
3. **The manifest as a list of restorable objects.** It is a list of objects that *were* created.
   Entries past `RETENTION_DAYS` are tombstones.
4. **Any claim that the schema can be rolled back.** Part 2.
5. **The R2 leg, still, from this environment.** As in June, no R2 credentials were available
   here, so `local` was used. `RECOVERY.md` §5 records a real R2 round trip on 2026-07-18; that
   entry — not this report — is the R2 evidence, and it predates migrations 34–40.
6. **RPO, if a service runs past midnight.** The timer fires at 02:30 server time. A Friday
   dinner service that runs to 01:00 Saturday is backed up 90 minutes later; a failure at 02:00
   loses the entire Friday night. The nightly cadence is fine for a pilot, but "≤24h RPO" hides
   that the worst case lands exactly on the busiest service.
