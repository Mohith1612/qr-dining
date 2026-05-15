# Backup and Restore Runbook

## Overview

qr-dining uses PostgreSQL as the sole source of truth. Redis is ephemeral and does not need backup — all critical data lives in PostgreSQL.

Backups use `pg_dump` in custom format with level-9 compression. Restore uses `pg_restore`.

---

## Backup

### Manual backup

```bash
DATABASE_URL=postgres://user:pass@host:5432/qr_dining ./scripts/backup.sh
```

Creates `./backups/backup_YYYYMMDD_HHMMSS.dump`.

### Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | required |
| `BACKUP_DIR` | Directory for backup files | `./backups` |
| `RETENTION_DAYS` | Delete backups older than N days | `7` |

### Automated backup (recommended)

Add a cron job on the Oracle Cloud instance:

```cron
# Daily backup at 02:00 UTC
0 2 * * * cd /opt/qr-dining && DATABASE_URL=... ./scripts/backup.sh >> /var/log/qr-dining-backup.log 2>&1
```

### Upload to Cloudflare R2 (optional)

```bash
# After backup.sh creates the file:
R2_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com \
R2_BUCKET=qr-dining-backups \
R2_ACCESS_KEY=... \
R2_SECRET_KEY=... \
./scripts/backup_r2.sh ./backups/backup_YYYYMMDD_HHMMSS.dump
```

---

## Retention Policy

| Tier | Retention | Count |
|------|-----------|-------|
| Daily | 7 days | 7 |
| Weekly (manual archive) | 4 weeks | 4 |

The `backup.sh` script automatically prunes files older than `RETENTION_DAYS` (default 7). For weekly archives, copy the Friday backup to a separate location before the daily pruner removes it.

---

## Restore

### From a local backup file

```bash
DATABASE_URL=postgres://user:pass@host:5432/qr_dining ./scripts/restore.sh ./backups/backup_YYYYMMDD_HHMMSS.dump
```

The script will:
1. Prompt for confirmation (`type 'yes'`)
2. Run `pg_restore --clean --single-transaction`
3. Print row counts from key tables to validate the restore

### Validation checklist

After restore, verify:

```bash
# Row counts printed by restore.sh — compare with pre-failure state
# Then start the server and verify:
curl http://localhost:8080/readyz          # should return {"status":"ready"}
curl http://localhost:8080/metrics         # should return Prometheus text
make seed                                  # optional: re-seed dev data
```

---

## Disaster Recovery Checklist

1. **Stop the application** — prevent writes during restore
2. **Identify the latest clean backup** — check `./backups/` or R2
3. **Restore** — run `./scripts/restore.sh <file>`
4. **Validate row counts** — shown by restore.sh
5. **Start the application** — `make docker-up && make run`
6. **Verify `/readyz`** — should return `{"status":"ready"}`
7. **Check event_log** — `GET /branches/1/events/recent` to confirm data integrity
8. **Alert team** — document the incident and root cause

---

## Notes

- Redis does not require backup. On restart, Redis is empty; the application degrades gracefully (menu cache misses, presence TTLs reset).
- Migrations run automatically at startup. If restoring an older backup to a newer schema, migrations will re-apply cleanly due to `golang-migrate`'s idempotent runner.
- Never restore to a production database without first taking a fresh backup of the current state.
