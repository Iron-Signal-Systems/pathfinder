#!/bin/sh
# READ ONLY: collect C220 facts needed to decide whether Pathfinder C1
# development can be staged. Never creates mounts/users/SQL/clones/keys,
# copies files, starts jails, modifies firewall rules, or enables services.
set -eu
umask 077

printf '%s\n' '===== PATHFINDER C1 C220 READ-ONLY DISCOVERY ====='
[ "$(uname -s)" = FreeBSD ] || {
    echo "PATHFINDER_C1_DISCOVERY_FAIL FreeBSD host required" >&2
    exit 1
}
printf '\n%s\n' '===== FREEBSD OS / HOST ====='
freebsd-version || true
uname -m
sysctl -n hw.physmem || true

printf '\n%s\n' '===== JAIL BOOT POLICY (READ ONLY) ====='
sysrc -n jail_enable 2>/dev/null || true
sysrc -n jail_list 2>/dev/null || true

printf '\n%s\n' '===== RUNNING PATHFINDER JAILS ====='
if command -v jls >/dev/null 2>&1; then
    for jail in pathfinder-ingest pathfinder-sor; do
        if jls -j "$jail" jid >/dev/null 2>&1; then
            printf '%s\n' "RUNNING $jail"
            jls -j "$jail" jid host.hostname path || true
        else
            printf '%s\n' "MISSING_OR_STOPPED $jail"
        fi
    done
else
    echo "jls unavailable"
fi

printf '\n%s\n' '===== RELEVANT MOUNTS (NO CHANGES) ====='
mount -p | awk '
    $3=="tmpfs" || $3=="nullfs" ||
    $1 ~ /(pathfinder|cold|fast)/ ||
    $2 ~ /(pathfinder|cold|fast)/ {print}
' || true

printf '\n%s\n' '===== RELEVANT ZFS DATASETS (NO CHANGES) ====='
if command -v zfs >/dev/null 2>&1; then
    zfs list -H -o name,mountpoint,used,avail 2>/dev/null |
        awk '$1 ~ /^(cold|fast|work)(\/|$)/ || $1 ~ /pathfinder/' || true
else
    echo "zfs unavailable"
fi

printf '\n%s\n' '===== POSTGRESQL BINARY AVAILABILITY (NO CONNECTIONS) ====='
for jail in pathfinder-sor; do
    if jls -j "$jail" jid >/dev/null 2>&1; then
        if jexec "$jail" /usr/local/bin/psql --version 2>/dev/null; then
            :
        else
            echo "psql not found at /usr/local/bin/psql in $jail"
        fi
    fi
done

printf '\n%s\n' '===== SUMMARY ====='
printf '%s\n' 'PATHFINDER_C1_C220_DISCOVERY_COMPLETE read_only=true mounts_changed=0 database_connections=0 services_started=0'
printf '%s\n' 'NOTE: This is inventory, NOT permission to deploy or SQL-clone acceptance.'
