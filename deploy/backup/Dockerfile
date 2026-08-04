# Backup runner: Postgres 17 client tools + the AWS CLI in one image.
#
# WHY THIS EXISTS
#
# The backup job needs pg_dump/pg_restore v17 and `aws`. Installing them on the
# host needs root, which the shared OCI VM's appuser does not have. The obvious
# workaround — `docker run postgres:17-alpine sh -c 'apk add aws-cli && ...'` —
# cannot work either: the job must join qr-dining_internal to reach Postgres,
# that network is `internal: true`, and apk therefore has no egress. The failure
# is also misleading, surfacing as "aws-cli (no such package)" rather than as a
# network error.
#
# Baking the tools in fixes all of it, and makes the nightly run faster and
# independent of package-repo availability.
#
# Build (on a network WITH egress — i.e. the default bridge, not the app's):
#   docker build -t qr-dining-backup:1 deploy/backup
#
# Run (on the internal network, where "postgres" resolves):
#   docker run --rm --network qr-dining_qr-dining_internal \
#     --env-file /opt/qr-dining/backup.env \
#     -v /opt/qr-dining/repo/backend/scripts:/scripts:ro \
#     -v qr-dining_backup_local:/var/backups/qr-dining \
#     -v qr-dining_backup_textfile:/var/lib/node_exporter/textfile \
#     qr-dining-backup:1
FROM postgres:17-alpine

RUN apk add --no-cache bash aws-cli

ENTRYPOINT ["bash", "/scripts/nightly-backup.sh"]
