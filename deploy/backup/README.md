# Backup job wiring

Current recovery authority: [docs/RECOVERY.md](../../docs/RECOVERY.md). This file
only explains how the checked-in scheduler and environment templates invoke the
backup program. The installation steps below are **unverified**; retained evidence
covers the script's backup/restore behavior, not a production scheduler install.

## What the job does

`backend/scripts/nightly-backup.sh` requires `DATABASE_URL`, supports `r2`, `s3`,
or `local` storage, and defaults to a 14-day retention value
(`backend/scripts/nightly-backup.sh:9-19,82-99`,
`backend/scripts/lib/storage.sh:8-118`). It creates a custom compressed dump,
computes SHA-256, uploads the object, appends a JSONL manifest entry, and removes
objects whose filename date exceeds retention
(`backend/scripts/nightly-backup.sh:101-145`).

An `EXIT` trap is installed before configuration validation. Every invocation
that reaches the script writes last-run status when a textfile directory is set;
only a completed run updates the last-success timestamp
(`backend/scripts/nightly-backup.sh:27-85,147-150`). A scheduler or shell failure
that prevents the script from starting writes nothing; the age-based alerts are
the only checked-in detection for that silence
(`deploy/observability/prometheus-alerts.yml:278-285,362-369`).

## Environment

Install a private copy of `deploy/backup/backup.env.example` and replace every
placeholder. The example separates backup-bucket credentials from application
upload credentials and includes the node_exporter textfile directory
(`deploy/backup/backup.env.example:1-46`). The Prometheus overlay mounts the
`backup_textfile` volume read-only into node_exporter
(`deploy/observability/docker-compose.observability.yml:49-61`). The actual backup
invocation must mount or resolve the same storage at
`NODE_EXPORTER_TEXTFILE_DIR`; otherwise backup alerts receive no metric.

## Scheduler choices — unverified

The systemd unit reads `/etc/qr-dining/backup.env` and invokes the repository
script; its timer schedules 02:30 server time with persistent catch-up
(`deploy/backup/qr-dining-backup.service:1-21`,
`deploy/backup/qr-dining-backup.timer:1-11`). The cron alternative sources the
same file and appends output to `/var/log/qr-dining-backup.log`
(`deploy/backup/crontab.example:1-18`). Choose one scheduler, not both.

Before enabling either, run the exact invocation once, confirm a new remote
object, compare its checksum to the matching manifest entry, and perform the
scratch restore in [docs/RECOVERY.md](../../docs/RECOVERY.md#rehearsed-scratch-restore).
Do not call a scheduler installed or monitored until those checks have actual
operator evidence.

## Restore

Do not restore from this abbreviated wiring note. The standalone procedure,
forward-fix-only restriction, Track F timings, four repaired tool defects, and
version-22 dirty-state recovery are all in
[docs/RECOVERY.md](../../docs/RECOVERY.md).
