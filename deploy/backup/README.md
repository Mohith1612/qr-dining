# qr-dining Backup System

Pilot-grade automated PostgreSQL backups to object storage (Cloudflare R2 by
default, any S3-compatible store via the same code path).

## Components

| File | Purpose |
|------|---------|
| `backend/scripts/nightly-backup.sh` | Orchestrator: dump → compress → upload → manifest → retention |
| `backend/scripts/lib/storage.sh` | Provider abstraction (`r2` \| `s3` \| `local`) |
| `backend/scripts/restore.sh` | Restore a `.dump` into a target database |
| `deploy/backup/qr-dining-backup.{service,timer}` | systemd nightly schedule |
| `deploy/backup/crontab.example` | cron alternative |
| `deploy/backup/backup.env.example` | environment template (root-only) |

## Flow

```
pg_dump (custom format, --compress=9)
   ↓  sha256
   ↓  upload  →  <prefix>/YYYY/MM/backup_<UTC-ts>.dump
   ↓  manifest entry appended → <prefix>/manifest.jsonl
   ↓  retention: prune dumps older than RETENTION_DAYS (remote + local)
```

The script exits non-zero on any failure so the scheduler/alerting detects a
missed backup (see `alerting-setup.md`, `BackupFailed`).

## Bucket layout

```
qr-dining-backups/
  backups/
    manifest.jsonl                         # append-only index, one JSON object per backup
    2026/06/backup_20260609_023000.dump     # one dump per nightly run, partitioned by year/month
    2026/06/backup_20260610_023000.dump
    ...
```

Each `manifest.jsonl` line:
```json
{"timestamp":"2026-06-09T02:30:00Z","key":"backups/2026/06/backup_20260609_023000.dump","bytes":52384,"sha256":"…","provider":"r2","retention_days":14}
```

## Retention

`RETENTION_DAYS` (default **14**) — dumps whose filename timestamp is older than
the cutoff are deleted from both the bucket and the local scratch dir on every
run. Retention is filename-driven (provider-agnostic), so it works identically on
R2, S3, and local. Optionally also set an R2/S3 bucket lifecycle rule as a
belt-and-suspenders backstop.

## Setup (production)

```bash
# 1. Install scripts (e.g. to /opt/qr-dining) and the AWS CLI (v2).
# 2. Configure env, root-only:
sudo mkdir -p /etc/qr-dining
sudo install -m 600 deploy/backup/backup.env.example /etc/qr-dining/backup.env
sudo $EDITOR /etc/qr-dining/backup.env        # set DATABASE_URL + R2_* creds

# 3a. systemd (preferred):
sudo cp deploy/backup/qr-dining-backup.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now qr-dining-backup.timer
systemctl list-timers qr-dining-backup.timer   # confirm next run

# 3b. or cron: install deploy/backup/crontab.example into /etc/cron.d/
```

Run once on demand to validate:
```bash
set -a; . /etc/qr-dining/backup.env; set +a
backend/scripts/nightly-backup.sh
```

## Restore procedure

See `restore-verification-report.md` for a verified end-to-end run. Summary:

```bash
# 1. Find the backup key in the manifest:
aws s3 cp s3://qr-dining-backups/backups/manifest.jsonl - \
  --endpoint-url "$R2_ENDPOINT" --region auto | tail

# 2. Download the chosen dump:
aws s3 cp s3://qr-dining-backups/backups/2026/06/backup_20260609_023000.dump ./restore.dump \
  --endpoint-url "$R2_ENDPOINT" --region auto

# 3. Verify checksum against the manifest entry's sha256:
sha256sum ./restore.dump

# 4. Restore into a TARGET database (never restore over production blindly;
#    restore into a fresh DB, verify, then cut over):
DATABASE_URL=postgres://user:pass@host:5432/restore_target ./backend/scripts/restore.sh ./restore.dump
```

`restore.sh` uses `pg_restore --clean --if-exists --single-transaction`, so a
failed restore rolls back atomically.

## RPO / RTO (pilot)

- **RPO:** ≤ 24h (nightly). For a single-restaurant pilot a day's worst-case loss
  is acceptable; tighten to hourly WAL archiving post-pilot if needed.
- **RTO:** minutes — download + `pg_restore` of a single small DB.
