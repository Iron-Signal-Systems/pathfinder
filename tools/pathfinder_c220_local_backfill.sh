#!/bin/sh
# C220-only historical Pathfinder backfill from a physical, frozen UDM-Pro archive.
# Host entry point: sh /fast/jails/roots/dev-pathfinder/home/jwood/pathfinder/tools/pathfinder_c220_local_backfill.sh STAGE
# Stages: stage | bootstrap | migrate | register | import | sni
# No inter-jail PostgreSQL connections, PF edits, or changes to the frozen source.
set -eu
umask 077

[ "$(id -u)" -eq 0 ] || { echo "STOP: run on C220 host as root" >&2; exit 1; }
[ "$#" -eq 1 ] || { echo "Usage: $0 stage|bootstrap|migrate|register|import|sni" >&2; exit 2; }
mode=$1

devroot=/fast/jails/roots/dev-pathfinder
sorroot=/fast/jails/roots/pathfinder-sor
repo="$devroot/home/jwood/pathfinder"
source=/work/pathfinder/udm-pro-20261120-0500EST
archive=/mnt/udm-pro-20261120-0500EST
target="$sorroot$archive"

test -d "$devroot" && test -d "$sorroot" || { echo "STOP: required jail roots missing"; exit 1; }
test -f "$repo/go/go.mod" || { echo "STOP: development checkout missing"; exit 1; }
test "$(jexec dev-pathfinder hostname)" = dev-pathfinder || exit 1
test "$(jexec pathfinder-sor hostname)" = pathfinder-sor || exit 1

case "$mode" in
stage)
    echo "===== BUILD AND TEST LOCAL-ONLY IMPORTER ====="
    jexec -U jwood dev-pathfinder /bin/sh <<'DEV'
set -eu
cd /home/jwood/pathfinder
test "$(git branch --show-current)" = dev/sensor-ingest-20261121
cd go
go test ./cmd/pathfinder ./cmd/sensor-sor-import -count=1
build=/tmp/pathfinder-c220-local-backfill
mkdir -p "$build"
chmod 0700 "$build"
go build -trimpath -o "$build/pathfinder" ./cmd/pathfinder
go build -trimpath -o "$build/sensor-sor-import" ./cmd/sensor-sor-import
echo "BUILD PASS"
DEV

    echo "===== STAGE BINARIES AND COMMITTED SQL IN DATABASE JAIL ====="
    install -d -m 0755 "$sorroot/usr/local/sbin" "$sorroot/usr/local/share/pathfinder/queries"
    install -o root -g wheel -m 0555 "$devroot/tmp/pathfinder-c220-local-backfill/pathfinder" \
        "$sorroot/usr/local/sbin/pathfinder"
    install -o root -g wheel -m 0555 "$devroot/tmp/pathfinder-c220-local-backfill/sensor-sor-import" \
        "$sorroot/usr/local/sbin/sensor-sor-import"
    install -o root -g wheel -m 0444 "$repo/go/queries/sensor_register.sql" \
        "$sorroot/usr/local/share/pathfinder/queries/sensor_register.sql"
    install -o root -g wheel -m 0444 "$repo/go/queries/sensor_unique_sni.sql" \
        "$sorroot/usr/local/share/pathfinder/queries/sensor_unique_sni.sql"

    echo "===== MOUNT FROZEN CAPTURE READ-ONLY FOR THIS BACKFILL ====="
    test -d "$source" || { echo "STOP: physical archive missing: $source"; exit 1; }
    mkdir -p "$target"
    test ! -L "$target" || { echo "STOP: symlink at archive mount target"; exit 1; }
    if mount -p | awk -v d="$target" '$2==d {found=1} END {exit !found}'; then
        mount -p | awk -v s="$source" -v d="$target" \
            '$1==s && $2==d && $3=="nullfs" && $4 ~ /(^|,)ro(,|$)/ {found=1} END {exit !found}' ||
            { echo "STOP: archive mount source or read-only flag conflict"; exit 1; }
    else
        test -z "$(ls -A "$target")" || { echo "STOP: archive target not empty"; exit 1; }
        mount -t nullfs -o ro "$source" "$target"
    fi
    mount -p | awk -v s="$source" -v d="$target" \
        '$1==s && $2==d && $3=="nullfs" && $4 ~ /(^|,)ro(,|$)/ {found=1} END {exit !found}' ||
        { echo "STOP: frozen source mount is not confirmed read-only"; exit 1; }
    jexec pathfinder-sor /bin/sh -c '
        set -eu
        test -r /mnt/udm-pro-20261120-0500EST/file-manifest.txt
        test -r /usr/local/share/pathfinder/queries/sensor_unique_sni.sql
        /usr/local/sbin/pathfinder version
        /usr/local/bin/psql --version
    '
    echo "STAGE PASS: executable and frozen capture are local to pathfinder-sor"
    ;;

bootstrap)
    echo "===== CREATE LOCAL-ONLY DATABASE ROLES AND BOOTSTRAP ====="
    jexec pathfinder-sor /bin/sh <<'SOR'
set -eu
umask 077
data=/var/db/pathfinder/pgdata
test "$(cat "$data/PG_VERSION")" = 18
test -L "$data/pg_wal"
test -x /usr/local/sbin/pathfinder
test -x /usr/local/bin/psql

existing=$(
    su -m postgres -c '/usr/local/bin/psql -XAt -v ON_ERROR_STOP=1 -d postgres' <<'SQL'
SELECT (SELECT count(*) FROM pg_roles WHERE rolname IN
 ('pathfinder_owner','pathfinder_migrator','pathfinder_app')) +
 (SELECT count(*) FROM pg_database WHERE datname='pathfinder');
SQL
)
[ "$existing" = 0 ] || {
    echo "STOP: Pathfinder roles or database already exist; do not overwrite or regenerate credentials"
    exit 1
}
secret=/usr/local/etc/pathfinder/secrets
mkdir -p "$secret"
chmod 0700 "$secret"
for name in postgresql-migration-password postgresql-app-password \
            postgresql-migration.pgpass postgresql-app.pgpass; do
    [ ! -e "$secret/$name" ] || { echo "STOP: existing secret $name"; exit 1; }
done

openssl rand -hex 32 > "$secret/postgresql-migration-password"
openssl rand -hex 32 > "$secret/postgresql-app-password"
chmod 0600 "$secret/postgresql-migration-password" "$secret/postgresql-app-password"
migration_password=$(cat "$secret/postgresql-migration-password")
app_password=$(cat "$secret/postgresql-app-password")
{
    echo 'BEGIN;'
    echo 'CREATE ROLE pathfinder_owner NOLOGIN;'
    printf "CREATE ROLE pathfinder_migrator LOGIN NOINHERIT PASSWORD '%s';\n" "$migration_password"
    printf "CREATE ROLE pathfinder_app LOGIN NOINHERIT PASSWORD '%s';\n" "$app_password"
    echo 'GRANT pathfinder_owner TO pathfinder_migrator WITH INHERIT FALSE, SET TRUE;'
    echo 'COMMIT;'
} | su -m postgres -c '/usr/local/bin/psql -X -q -v ON_ERROR_STOP=1 -d postgres'

su -m postgres -c '/usr/local/bin/psql -X -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE pathfinder OWNER pathfinder_owner"'
su -m postgres -c '/usr/local/bin/psql -X -q -v ON_ERROR_STOP=1 -d pathfinder' <<'SQL'
BEGIN;
CREATE SCHEMA pathfinder AUTHORIZATION pathfinder_owner;
REVOKE ALL ON DATABASE pathfinder FROM PUBLIC;
GRANT CONNECT ON DATABASE pathfinder TO pathfinder_migrator, pathfinder_app;
REVOKE ALL ON SCHEMA pathfinder FROM PUBLIC;
GRANT USAGE ON SCHEMA pathfinder TO pathfinder_app;
COMMIT;
SQL
printf '127.0.0.1:5432:pathfinder:pathfinder_migrator:%s\n' \
    "$migration_password" > "$secret/postgresql-migration.pgpass"
printf '127.0.0.1:5432:pathfinder:pathfinder_app:%s\n' \
    "$app_password" > "$secret/postgresql-app.pgpass"
chmod 0600 "$secret/postgresql-migration.pgpass" "$secret/postgresql-app.pgpass"
chmod 0640 "$secret/postgresql-app-password"
unset migration_password app_password

config=/usr/local/etc/pathfinder/frozen-backfill.conf
test ! -e "$config" || { echo "STOP: existing backfill config"; exit 1; }
cat > "$config" <<'CFG'
artifact_socket=/var/run/pathfinder-frozen-artifact.sock
artifacts_dir=/var/db/pathfinder
database_host=127.0.0.1
database_name=pathfinder
database_password_file=/usr/local/etc/pathfinder/secrets/postgresql-app-password
database_port=5432
database_user=pathfinder_app
log_dir=/var/log
readiness_listen=127.0.0.1:18099
state_dir=/var/db/pathfinder
CFG
chmod 0600 "$config"
PGPASSFILE="$secret/postgresql-migration.pgpass" \
    /usr/local/bin/psql -XAt -v ON_ERROR_STOP=1 -h 127.0.0.1 \
    -U pathfinder_migrator -d pathfinder -c \
    "BEGIN; SET ROLE pathfinder_owner; SELECT session_user, current_user; ROLLBACK;"
echo "BOOTSTRAP PASS: PostgreSQL roles and private local secrets ready"
SOR
    ;;

migrate)
    echo "===== PATHFINDER EMBEDDED MIGRATION DRY RUN ====="
    jexec pathfinder-sor /usr/local/sbin/pathfinder migrate \
        -config /usr/local/etc/pathfinder/frozen-backfill.conf -dry-run
    echo "===== APPLY EMBEDDED MIGRATIONS 0001–0009 ====="
    jexec pathfinder-sor /usr/local/sbin/pathfinder migrate \
        -config /usr/local/etc/pathfinder/frozen-backfill.conf
    echo "MIGRATE PASS"
    ;;

register)
    echo "===== REGISTER GATEWAY SENSOR WITH MIGRATION AUTHORITY ====="
    jexec pathfinder-sor /bin/sh <<'SOR'
set -eu
PGPASSFILE=/usr/local/etc/pathfinder/secrets/postgresql-migration.pgpass \
 /usr/local/bin/psql -X -v ON_ERROR_STOP=1 \
 -h 127.0.0.1 -p 5432 -U pathfinder_migrator -d pathfinder \
 -v sensor_key=Gateway -v 'sensor_name=UDM-Pro historical capture' \
 -f /usr/local/share/pathfinder/queries/sensor_register.sql
SOR
    ;;

import)
    echo "===== INDEX EXACT FROZEN ARCHIVE VIA LOCAL POSTGRESQL ====="
    jexec pathfinder-sor /bin/sh <<'SOR'
set -eu
umask 077
test -r /mnt/udm-pro-20261120-0500EST/file-manifest.txt
custody=/var/db/pathfinder/sensor-custody
if [ ! -e "$custody" ]; then mkdir -m 0700 "$custody"; fi
test -d "$custody" && test ! -L "$custody"
[ "$(stat -f %Lp "$custody")" = 700 ] || { echo "STOP: custody permissions not private"; exit 1; }
PGPASSFILE=/usr/local/etc/pathfinder/secrets/postgresql-app.pgpass \
 /usr/local/sbin/sensor-sor-import \
 -mode import \
 -root /mnt/udm-pro-20261120-0500EST \
 -sensor-key Gateway \
 -custody "$custody" \
 -psql /usr/local/bin/psql \
 -db-host 127.0.0.1 -db-port 5432 \
 -db-name pathfinder -db-user pathfinder_app \
 -expect-files 9 -expect-rows 685218 \
 -sni-query /usr/local/share/pathfinder/queries/sensor_unique_sni.sql
SOR
    ;;

sni)
    echo "===== UNIQUE SNI HOSTNAMES FROM COMMITTED POSTGRESQL ROWS ====="
    jexec pathfinder-sor /bin/sh <<'SOR'
set -eu
pass=/usr/local/etc/pathfinder/secrets/postgresql-app.pgpass
counts=$(PGPASSFILE="$pass" /usr/local/bin/psql -XAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -p 5432 -U pathfinder_app -d pathfinder -c "
SELECT concat_ws('|',
 (SELECT count(*) FROM pathfinder.sensor_observation),
 (SELECT count(*) FROM pathfinder.sensor_conntrack_event),
 (SELECT count(*) FROM pathfinder.sensor_identity_snapshot),
 (SELECT count(*) FROM pathfinder.sensor_health)
);
")
[ "$counts" = "569671|111842|2446|1259" ] || {
    echo "STOP: database counts not reconciled: $counts" >&2
    exit 1
}
PGPASSFILE="$pass" \
 /usr/local/sbin/sensor-sor-import \
 -mode sni -psql /usr/local/bin/psql \
 -db-host 127.0.0.1 -db-port 5432 \
 -db-name pathfinder -db-user pathfinder_app \
 -sni-query /usr/local/share/pathfinder/queries/sensor_unique_sni.sql
SOR
    ;;
*)
    echo "Usage: $0 stage|bootstrap|migrate|register|import|sni" >&2
    exit 2
    ;;
esac
