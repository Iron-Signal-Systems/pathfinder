#!/bin/sh
# C220 administrative diagnostic; run from the FreeBSD host as root.
# Queries committed PostgreSQL sensor observations; makes no changes.
set -eu

[ "$(id -u)" -eq 0 ] || {
    echo "STOP: run on C220 host as root" >&2
    exit 1
}

query=/fast/jails/roots/dev-pathfinder/home/jwood/pathfinder/go/queries/sensor_sni_ip_diagnostic.sql
[ -r "$query" ] || {
    echo "STOP: pull the Pathfinder development branch before running this report" >&2
    exit 1
}

jexec pathfinder-sor /bin/sh -c '
set -eu
export PGPASSFILE=/usr/local/etc/pathfinder/secrets/postgresql-app.pgpass
exec /usr/local/bin/psql \
    -X -A -t -v ON_ERROR_STOP=1 -P pager=off \
    -h 127.0.0.1 -p 5432 \
    -U pathfinder_app -d pathfinder
' < "$query"
