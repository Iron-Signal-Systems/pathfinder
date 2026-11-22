#!/bin/sh
# C220 low-level developer/admin diagnostic: unique TLS SNI hostnames.
# Run from the FreeBSD host as root; reads already committed PostgreSQL rows.
# Prints one unique hostname per line and the total count. No import or writes.
set -eu

[ "$(id -u)" -eq 0 ] || {
    echo "STOP: run on C220 host as root" >&2
    exit 1
}

jexec pathfinder-sor /bin/sh -c '
set -eu
export PGPASSFILE=/usr/local/etc/pathfinder/secrets/postgresql-app.pgpass
test -r /usr/local/share/pathfinder/queries/sensor_unique_sni.sql
test -x /usr/local/sbin/sensor-sor-import
exec /usr/local/sbin/sensor-sor-import \
    -mode sni \
    -psql /usr/local/bin/psql \
    -db-host 127.0.0.1 -db-port 5432 \
    -db-name pathfinder -db-user pathfinder_app \
    -sni-query /usr/local/share/pathfinder/queries/sensor_unique_sni.sql
'
