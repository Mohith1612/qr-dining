#!/usr/bin/env bash
# backup-restore-test.sh — regression tests for the four Track F defects.
#
#   1. restore.sh onto a host lacking the dump's role            (--no-owner)
#   2. nightly-backup.sh failure reporting                       (status metric)
#   3. cmd/migrate recovery from a dirty migration               (force)
#   4. restore.sh non-interactive                                (--yes)
#
# Each test reproduces the original failure first where it can, so a regression
# is visible as the OLD behaviour coming back rather than as a silent pass.
#
# Self-contained: spins up two throwaway postgres:17-alpine containers (a source
# and a SEPARATE target that deliberately does not have the source's role — that
# separation is what makes test 1 meaningful) and tears them down on exit. It
# never touches qrdining_mtest, the soak stack, or any other database.
#
# Usage:  backend/scripts/tests/backup-restore-test.sh
# Requires: docker, go. Exits 0 only if every test passes.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$(cd "$SCRIPTS_DIR/.." && pwd)"

PG_IMAGE="postgres:17-alpine"
NET="qrd-scripttest-net"
SRC="qrd-scripttest-src"
TGT="qrd-scripttest-tgt"
SRC_PORT=35444
TGT_PORT=35445
WORK="$(mktemp -d)"

PASS=0
FAIL=0

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
pass() { PASS=$((PASS+1)); printf '   \033[32mPASS\033[0m %s\n' "$*"; }
fail() { FAIL=$((FAIL+1)); printf '   \033[31mFAIL\033[0m %s\n' "$*"; }
check(){ if [ "$2" = "$3" ]; then pass "$1 ($2)"; else fail "$1: expected '$3', got '$2'"; fi; }

cleanup() {
  docker rm -f "$SRC" "$TGT" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  # The backup container runs as root and leaves root-owned files behind.
  docker run --rm -v "$WORK:/w" alpine sh -c 'rm -rf /w/* /w/.[!.]* 2>/dev/null' >/dev/null 2>&1 || true
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT

command -v docker >/dev/null || { echo "SKIP: docker not available"; exit 0; }
command -v go     >/dev/null || { echo "SKIP: go not available";     exit 0; }

# Run a script from backend/scripts inside a pg17 container on the test network.
# Everything that needs pg_dump/pg_restore/psql goes through here, because the
# host's client tools are not guaranteed to match the server major version.
in_pg() { # $1=extra docker args (string), rest = shell command
  local extra="$1"; shift
  # shellcheck disable=SC2086
  docker run --rm --network "$NET" $extra \
    -v "$SCRIPTS_DIR:/scripts:ro" -v "$WORK:/work" \
    "$PG_IMAGE" sh -c "apk add --no-cache bash >/dev/null 2>&1; $*"
}

say "setup: two independent postgres servers"
docker rm -f "$SRC" "$TGT" >/dev/null 2>&1 || true
docker network rm "$NET" >/dev/null 2>&1 || true
docker network create "$NET" >/dev/null
docker run -d --name "$SRC" --network "$NET" -p "$SRC_PORT:5432" \
  -e POSTGRES_USER=srcowner -e POSTGRES_PASSWORD=srcpw -e POSTGRES_DB=srcdb "$PG_IMAGE" >/dev/null
docker run -d --name "$TGT" --network "$NET" -p "$TGT_PORT:5432" \
  -e POSTGRES_USER=tgtowner -e POSTGRES_PASSWORD=tgtpw -e POSTGRES_DB=tgtdb "$PG_IMAGE" >/dev/null
for c in "$SRC" "$TGT"; do
  for _ in $(seq 1 40); do
    docker exec "$c" pg_isready -q && break
    sleep 0.5
  done
done
echo "   source: role 'srcowner'   target: role 'tgtowner' (source role absent — the rebuilt-host case)"

say "setup: build cmd/migrate and migrate the source schema"
MIGRATE_BIN="$WORK/migrate"
( cd "$BACKEND_DIR" && go build -o "$MIGRATE_BIN" ./cmd/migrate ) || { echo "build failed"; exit 1; }
SRC_DSN="postgres://srcowner:srcpw@localhost:$SRC_PORT/srcdb?sslmode=disable"
TGT_DSN="postgres://tgtowner:tgtpw@localhost:$TGT_PORT/tgtdb?sslmode=disable"
DATABASE_URL="$SRC_DSN" "$MIGRATE_BIN" up >/dev/null || { echo "source migration failed"; exit 1; }
SCHEMA_VERSION="$(docker exec "$SRC" psql -U srcowner -d srcdb -At -c 'SELECT version FROM schema_migrations;')"
echo "   source at schema $SCHEMA_VERSION"

# Audit rows are what makes the migration-22 wedge reproducible: 22's up runs
# UPDATE audit_log, which the immutability trigger rejects only when rows exist.
docker exec "$SRC" psql -U srcowner -d srcdb -q -c "
INSERT INTO audit_log (resource_type, action, actor_type, actor_id)
SELECT 'test', 'seed', 'system', g::text FROM generate_series(1,5) g;" >/dev/null
AUDIT_ROWS="$(docker exec "$SRC" psql -U srcowner -d srcdb -At -c 'SELECT count(*) FROM audit_log;')"
echo "   seeded $AUDIT_ROWS audit_log rows"

say "setup: produce a dump via nightly-backup.sh (provider=local)"
mkdir -p "$WORK/bucket" "$WORK/scratch" "$WORK/textfile"
chmod 777 "$WORK" "$WORK/bucket" "$WORK/scratch" "$WORK/textfile"
in_pg "-e DATABASE_URL=postgres://srcowner:srcpw@$SRC:5432/srcdb?sslmode=disable \
       -e BACKUP_PROVIDER=local -e BACKUP_LOCAL_DIR=/work/bucket \
       -e BACKUP_WORKDIR=/work/scratch -e NODE_EXPORTER_TEXTFILE_DIR=/work/textfile" \
  'bash /scripts/nightly-backup.sh' >"$WORK/backup.log" 2>&1 \
  || { echo "backup failed:"; cat "$WORK/backup.log"; exit 1; }
DUMP_REL="$(docker run --rm -v "$WORK:/w" alpine sh -c 'cd /w/bucket && find . -name "*.dump" | head -1 | sed "s|^\./||"')"
echo "   dump: $DUMP_REL"

# ---------------------------------------------------------------------------
say "TEST 1 — restore onto a server that lacks the dump's owning role"
# ---------------------------------------------------------------------------
# Old behaviour: pg_restore hits ALTER ... OWNER TO srcowner, errors, and
# --single-transaction rolls everything back. Nothing is restored at all.
docker exec "$TGT" psql -U tgtowner -d tgtdb -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null 2>&1

in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "bash /scripts/restore.sh --preserve-owner --yes /work/bucket/$DUMP_REL" >"$WORK/t1-old.log" 2>&1
RC_OLD=$?
TBL_OLD="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT count(*) FROM pg_tables WHERE schemaname='public';")"
if [ "$RC_OLD" -ne 0 ] && grep -q 'role "srcowner" does not exist' "$WORK/t1-old.log"; then
  pass "--preserve-owner still reproduces the original failure (role \"srcowner\" does not exist)"
else
  fail "--preserve-owner did not reproduce the original failure (rc=$RC_OLD)"
fi
check "nothing restored on that failure (single-transaction rollback)" "$TBL_OLD" "0"

# New behaviour: the default path restores cleanly.
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "bash /scripts/restore.sh --yes /work/bucket/$DUMP_REL" >"$WORK/t1-new.log" 2>&1
RC_NEW=$?
check "restore.sh succeeds by default onto a host without the role" "$RC_NEW" "0"

SRC_TABLES="$(docker exec "$SRC" psql -U srcowner -d srcdb -At -c "SELECT count(*) FROM pg_tables WHERE schemaname='public';")"
TGT_TABLES="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT count(*) FROM pg_tables WHERE schemaname='public';")"
check "all tables restored" "$TGT_TABLES" "$SRC_TABLES"
TGT_AUDIT="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c 'SELECT count(*) FROM audit_log;')"
check "audit rows restored" "$TGT_AUDIT" "$AUDIT_ROWS"
TGT_VER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT version||'/'||dirty FROM schema_migrations;")"
check "schema version restored, not dirty" "$TGT_VER" "$SCHEMA_VERSION/false"
OWNER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT tableowner FROM pg_tables WHERE schemaname='public' AND tablename='audit_log';")"
check "restored objects owned by the connecting role" "$OWNER" "tgtowner"
TRIG="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT count(*) FROM pg_trigger WHERE tgrelid='audit_log'::regclass AND NOT tgisinternal;")"
check "audit immutability trigger survived --no-owner" "$TRIG" "1"
if docker exec "$TGT" psql -U tgtowner -d tgtdb -c \
     "UPDATE audit_log SET action='tamper' WHERE id=(SELECT min(id) FROM audit_log);" >/dev/null 2>&1; then
  fail "audit_log is writable after restore — the trigger is not enforcing"
else
  pass "restored audit_log still rejects UPDATE"
fi

# ---------------------------------------------------------------------------
say "TEST 2 — nightly-backup.sh failure reporting"
# ---------------------------------------------------------------------------
status_metric()  { docker run --rm -v "$WORK:/w" alpine sh -c 'tail -1 /w/textfile/qr_dining_backup_status.prom 2>/dev/null' | awk '{print $2}'; }
success_metric() { docker run --rm -v "$WORK:/w" alpine sh -c 'tail -1 /w/textfile/qr_dining_backup_success.prom 2>/dev/null' | awk '{print $2}'; }

# A good run first, so the failure cases have a "last good run" to wrongly keep.
in_pg "-e DATABASE_URL=postgres://srcowner:srcpw@$SRC:5432/srcdb?sslmode=disable \
       -e BACKUP_PROVIDER=local -e BACKUP_LOCAL_DIR=/work/bucket \
       -e BACKUP_WORKDIR=/work/scratch -e NODE_EXPORTER_TEXTFILE_DIR=/work/textfile" \
  'bash /scripts/nightly-backup.sh' >/dev/null 2>&1
check "successful run writes status 0" "$(status_metric)" "0"
GOOD_TS="$(success_metric)"
[ -n "$GOOD_TS" ] && pass "successful run writes a success timestamp ($GOOD_TS)" || fail "no success timestamp written"

# 2a. The defect: DATABASE_URL unset. Used to exit before the trap existed and
#     leave the status metric reporting the previous good run.
in_pg "-e BACKUP_PROVIDER=local -e BACKUP_LOCAL_DIR=/work/bucket \
       -e BACKUP_WORKDIR=/work/scratch -e NODE_EXPORTER_TEXTFILE_DIR=/work/textfile" \
  'bash /scripts/nightly-backup.sh' >"$WORK/t2a.log" 2>&1
RC="$?"
check "missing DATABASE_URL exits non-zero" "$RC" "1"
check "missing DATABASE_URL writes status 1" "$(status_metric)" "1"
check "missing DATABASE_URL preserves the last success timestamp" "$(success_metric)" "$GOOD_TS"

# 2b. Unreachable database — already worked; assert it still does.
in_pg "-e DATABASE_URL=postgres://srcowner:srcpw@$SRC:5432/nope?sslmode=disable \
       -e BACKUP_PROVIDER=local -e BACKUP_LOCAL_DIR=/work/bucket \
       -e BACKUP_WORKDIR=/work/scratch -e NODE_EXPORTER_TEXTFILE_DIR=/work/textfile" \
  'bash /scripts/nightly-backup.sh' >"$WORK/t2b.log" 2>&1
RC="$?"
check "unreachable database exits non-zero" "$RC" "1"
check "unreachable database writes status 1" "$(status_metric)" "1"
check "unreachable database preserves the last success timestamp" "$(success_metric)" "$GOOD_TS"

# 2c. A bad provider config fails before pg_dump — also covered by the EXIT trap.
in_pg "-e DATABASE_URL=postgres://srcowner:srcpw@$SRC:5432/srcdb?sslmode=disable \
       -e BACKUP_PROVIDER=bogus -e NODE_EXPORTER_TEXTFILE_DIR=/work/textfile \
       -e BACKUP_WORKDIR=/work/scratch" \
  'bash /scripts/nightly-backup.sh' >/dev/null 2>&1
check "invalid BACKUP_PROVIDER writes status 1" "$(status_metric)" "1"

# Leave the metric healthy again so a later run is not misread.
in_pg "-e DATABASE_URL=postgres://srcowner:srcpw@$SRC:5432/srcdb?sslmode=disable \
       -e BACKUP_PROVIDER=local -e BACKUP_LOCAL_DIR=/work/bucket \
       -e BACKUP_WORKDIR=/work/scratch -e NODE_EXPORTER_TEXTFILE_DIR=/work/textfile" \
  'bash /scripts/nightly-backup.sh' >/dev/null 2>&1
check "recovery run writes status 0 again" "$(status_metric)" "0"

# ---------------------------------------------------------------------------
say "TEST 3 — wedge a migration, recover with 'migrate force'"
# ---------------------------------------------------------------------------
# The target now holds a restored copy with audit rows. Roll it back to 21 and
# migrate up: 22's UPDATE audit_log hits the immutability trigger and leaves
# schema_migrations dirty. That is the state 'force' has to get us out of.
STEPS=$(( SCHEMA_VERSION - 21 ))
DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" down "$STEPS" >/dev/null 2>&1
VER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT version||'/'||dirty FROM schema_migrations;")"
check "rolled back to 21" "$VER" "21/false"

DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" up >"$WORK/t3-wedge.log" 2>&1
RC="$?"
check "migrating up through 22 fails with audit rows present" "$RC" "1"
if grep -q 'audit_log rows are immutable' "$WORK/t3-wedge.log"; then
  pass "failed on the immutability trigger, as documented"
else
  fail "failed for an unexpected reason: $(tail -1 "$WORK/t3-wedge.log")"
fi
VER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT version||'/'||dirty FROM schema_migrations;")"
check "database is now dirty at 22" "$VER" "22/true"

# Without force, there is no way forward or back — this is the wedge.
DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" up   >"$WORK/t3-up2.log"   2>&1; RC_UP="$?"
DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" down 1 >"$WORK/t3-down2.log" 2>&1; RC_DOWN="$?"
check "up refuses while dirty" "$RC_UP" "1"
check "down refuses while dirty" "$RC_DOWN" "1"

DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" version >"$WORK/t3-version.log" 2>&1
if grep -q 'version=22 dirty=true' "$WORK/t3-version.log"; then
  pass "'migrate version' reports the wedge without needing psql"
else
  fail "'migrate version' output unexpected: $(cat "$WORK/t3-version.log")"
fi

DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" force 21 >"$WORK/t3-force.log" 2>&1
RC="$?"
check "'migrate force 21' succeeds" "$RC" "0"
VER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT version||'/'||dirty FROM schema_migrations;")"
check "dirty cleared, version set to 21" "$VER" "21/false"

# force alone is not enough — 22 still collides with the trigger. That is the
# documented reality (RUNBOOKS.md §8), so assert it rather than pretend
# otherwise, then complete the documented recovery.
DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" up >/dev/null 2>&1
VER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT version||'/'||dirty FROM schema_migrations;")"
check "re-wedges at 22 without the documented trigger step" "$VER" "22/true"

DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" force 21 >/dev/null 2>&1
docker exec "$TGT" psql -U tgtowner -d tgtdb -q -c 'DROP TRIGGER trg_audit_log_immutable ON audit_log;' >/dev/null
DATABASE_URL="$TGT_DSN" "$MIGRATE_BIN" up >"$WORK/t3-recover.log" 2>&1
RC="$?"
check "full documented recovery (force + drop trigger + up) succeeds" "$RC" "0"
VER="$(docker exec "$TGT" psql -U tgtowner -d tgtdb -At -c "SELECT version||'/'||dirty FROM schema_migrations;")"
check "back to head, clean" "$VER" "$SCHEMA_VERSION/false"
docker exec "$TGT" psql -U tgtowner -d tgtdb -q -c "
CREATE TRIGGER trg_audit_log_immutable BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();" >/dev/null
if docker exec "$TGT" psql -U tgtowner -d tgtdb -c \
     "UPDATE audit_log SET action='t' WHERE id=(SELECT min(id) FROM audit_log);" >/dev/null 2>&1; then
  fail "trigger re-created but not enforcing"
else
  pass "trigger re-created and enforcing again"
fi

# ---------------------------------------------------------------------------
say "TEST 4 — restore.sh without a terminal"
# ---------------------------------------------------------------------------
docker exec "$TGT" psql -U tgtowner -d tgtdb -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null 2>&1

# No --yes and no stdin: must fail loudly, not silently.
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "bash /scripts/restore.sh /work/bucket/$DUMP_REL < /dev/null" >"$WORK/t4-nostdin.log" 2>&1
RC="$?"
check "no confirmation and no terminal exits non-zero" "$RC" "1"
if grep -q 'no confirmation available on stdin' "$WORK/t4-nostdin.log"; then
  pass "says why it stopped instead of aborting silently"
else
  fail "no explanatory error; output was: $(tail -2 "$WORK/t4-nostdin.log")"
fi

# --yes with stdin closed: the drill/cron case.
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "bash /scripts/restore.sh --yes /work/bucket/$DUMP_REL < /dev/null" >"$WORK/t4-yes.log" 2>&1
check "--yes restores with no terminal" "$?" "0"

# FORCE=1 env equivalent.
docker exec "$TGT" psql -U tgtowner -d tgtdb -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null 2>&1
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable -e FORCE=1" \
  "bash /scripts/restore.sh /work/bucket/$DUMP_REL < /dev/null" >"$WORK/t4-force.log" 2>&1
check "FORCE=1 restores with no terminal" "$?" "0"

# The documented `echo yes |` form must keep working.
docker exec "$TGT" psql -U tgtowner -d tgtdb -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null 2>&1
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "echo yes | bash /scripts/restore.sh /work/bucket/$DUMP_REL" >"$WORK/t4-pipe.log" 2>&1
check "'echo yes |' still works (RECOVERY.md §3a)" "$?" "0"

# A refusal must still abort.
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "echo no | bash /scripts/restore.sh /work/bucket/$DUMP_REL" >"$WORK/t4-no.log" 2>&1
RC="$?"
check "answering 'no' still aborts" "$RC" "1"
grep -q 'Aborted' "$WORK/t4-no.log" && pass "prints 'Aborted.'" || fail "did not print 'Aborted.'"

# Bad usage should be rejected, not treated as a filename.
in_pg "-e DATABASE_URL=postgres://tgtowner:tgtpw@$TGT:5432/tgtdb?sslmode=disable" \
  "bash /scripts/restore.sh --bogus /work/bucket/$DUMP_REL < /dev/null" >/dev/null 2>&1
check "unknown option rejected with exit 2" "$?" "2"

printf '\n\033[1m================ %s passed, %s failed ================\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
