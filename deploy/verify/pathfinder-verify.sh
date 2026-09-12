#!/bin/sh
#
# Pathfinder Phase 1.1 runtime/platform verification.
# See ../../LICENSE and ../../docs/PHASE-1.1-PLATFORM-FOUNDATION.md.

set -u

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEFAULT_CONFIG="${SCRIPT_DIR}/../pathfinder.conf.example"
CONFIG=${1:-$DEFAULT_CONFIG}

PASS_COUNT=0
FAIL_COUNT=0

pass()
{
    PASS_COUNT=$((PASS_COUNT + 1))
    printf '[PASS] %s\n' "$1"
}

fail()
{
    FAIL_COUNT=$((FAIL_COUNT + 1))
    printf '[FAIL] %s\n' "$1"
}

check_eq()
{
    description=$1
    actual=$2
    expected=$3

    if [ "$actual" = "$expected" ]; then
        pass "$description"
    else
        fail "$description (expected: $expected; actual: $actual)"
    fi
}

if [ ! -r "$CONFIG" ]; then
    printf '[FAIL] deployment config not readable: %s\n' "$CONFIG" >&2
    exit 2
fi

# shellcheck disable=SC1090
. "$CONFIG"

if [ "$(id -u)" -ne 0 ]; then
    printf '[FAIL] run as root; PF, jail, and service verification require host authority\n' >&2
    exit 2
fi

printf 'Pathfinder Phase 1.1 verification\n'
printf 'Config: %s\n\n' "$CONFIG"

version=$(freebsd-version -u 2>/dev/null || true)
case "$version" in
    "${PATHFINDER_RELEASE}-${PATHFINDER_PATCH_LEVEL}")
        pass "FreeBSD ${PATHFINDER_RELEASE}-${PATHFINDER_PATCH_LEVEL}"
        ;;
    *)
        fail "FreeBSD version (expected ${PATHFINDER_RELEASE}-${PATHFINDER_PATCH_LEVEL}; actual ${version:-NOT_KNOWN})"
        ;;
esac

if zpool status -x "$ZPOOL" 2>/dev/null | grep -q "is healthy"; then
    pass "ZFS pool $ZPOOL healthy"
else
    fail "ZFS pool $ZPOOL healthy"
fi

if jls -j "$DB_JAIL" >/dev/null 2>&1; then
    pass "database jail $DB_JAIL running"
else
    fail "database jail $DB_JAIL running"
fi

if jls -j "$APP_JAIL" >/dev/null 2>&1; then
    pass "application jail $APP_JAIL running"
else
    fail "application jail $APP_JAIL running"
fi

if ifconfig "$BRIDGE_IF" 2>/dev/null | grep -q "inet ${PRIVATE_GATEWAY} "; then
    pass "$BRIDGE_IF has ${PRIVATE_GATEWAY}"
else
    fail "$BRIDGE_IF has ${PRIVATE_GATEWAY}"
fi

if ifconfig "$BRIDGE_IF" 2>/dev/null | grep -q 'member: epair10a'; then
    pass "$BRIDGE_IF contains epair10a"
else
    fail "$BRIDGE_IF contains epair10a"
fi

if ifconfig "$BRIDGE_IF" 2>/dev/null | grep -q 'member: epair20a'; then
    pass "$BRIDGE_IF contains epair20a"
else
    fail "$BRIDGE_IF contains epair20a"
fi

check_eq "IPv4 forwarding enabled" \
    "$(sysctl -n net.inet.ip.forwarding 2>/dev/null || echo NOT_KNOWN)" \
    "1"

check_eq "bridge member PF inspection enabled" \
    "$(sysctl -n net.link.bridge.pfil_member 2>/dev/null || echo NOT_KNOWN)" \
    "1"

check_eq "bridge-level double filtering disabled" \
    "$(sysctl -n net.link.bridge.pfil_bridge 2>/dev/null || echo NOT_KNOWN)" \
    "0"

check_eq "bridge local-physical PF inspection enabled" \
    "$(sysctl -n net.link.bridge.pfil_local_phys 2>/dev/null || echo NOT_KNOWN)" \
    "1"

if pfctl -s info 2>/dev/null | grep -q '^Status: Enabled'; then
    pass "PF enabled"
else
    fail "PF enabled"
fi

if pfctl -s nat 2>/dev/null | grep -Fq "nat on ${EXT_IF} inet from ${PRIVATE_NET} to any -> (${EXT_IF})"; then
    pass "Pathfinder NAT active"
else
    fail "Pathfinder NAT active"
fi

if pfctl -s rules 2>/dev/null | grep -Fq "from ${APP_IP} to ${DB_IP} port = postgresql"; then
    pass "app to database PostgreSQL rule present"
else
    fail "app to database PostgreSQL rule present"
fi

if jexec "$APP_JAIL" ifconfig epair10b 2>/dev/null | grep -q "inet ${APP_IP} "; then
    pass "$APP_JAIL address ${APP_IP}"
else
    fail "$APP_JAIL address ${APP_IP}"
fi

if jexec "$DB_JAIL" ifconfig epair20b 2>/dev/null | grep -q "inet ${DB_IP} "; then
    pass "$DB_JAIL address ${DB_IP}"
else
    fail "$DB_JAIL address ${DB_IP}"
fi

if jexec "$APP_JAIL" fetch -T 5 -qo /dev/null https://download.freebsd.org/ >/dev/null 2>&1; then
    pass "$APP_JAIL public HTTPS egress"
else
    fail "$APP_JAIL public HTTPS egress"
fi

if jexec "$DB_JAIL" fetch -T 5 -qo /dev/null https://download.freebsd.org/ >/dev/null 2>&1; then
    fail "$DB_JAIL public HTTPS denied"
else
    pass "$DB_JAIL public HTTPS denied"
fi

if jexec "$APP_JAIL" /rescue/nc -z -w 2 "$DB_IP" 5432 >/dev/null 2>&1; then
    pass "$APP_JAIL to $DB_JAIL PostgreSQL connectivity"
else
    fail "$APP_JAIL to $DB_JAIL PostgreSQL connectivity"
fi

check_eq "PostgreSQL boot enabled" \
    "$(jexec "$DB_JAIL" sysrc -n postgresql_enable 2>/dev/null || echo NOT_KNOWN)" \
    "YES"

if jexec "$DB_JAIL" service postgresql status >/dev/null 2>&1; then
    pass "PostgreSQL running"
else
    fail "PostgreSQL running"
fi

if jexec "$DB_JAIL" sockstat -4 -l 2>/dev/null | grep -q "${DB_IP}:5432"; then
    pass "PostgreSQL listener restricted to ${DB_IP}:5432"
else
    fail "PostgreSQL listener restricted to ${DB_IP}:5432"
fi

pg_version=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d postgres -Atc 'SHOW server_version;'" 2>/dev/null || true)
case "$pg_version" in
    18.*)
        pass "PostgreSQL 18 runtime"
        ;;
    *)
        fail "PostgreSQL 18 runtime (actual: ${pg_version:-NOT_KNOWN})"
        ;;
esac

check_eq "PostgreSQL data checksums enabled" \
    "$(jexec "$DB_JAIL" su - postgres -c "/usr/local/bin/psql -d postgres -Atc 'SHOW data_checksums;'" 2>/dev/null || echo NOT_KNOWN)" \
    "on"

check_eq "Pathfinder boot enabled" \
    "$(jexec "$APP_JAIL" sysrc -n pathfinder_enable 2>/dev/null || echo NOT_KNOWN)" \
    "YES"

if jexec "$APP_JAIL" service pathfinder status >/dev/null 2>&1; then
    pass "Pathfinder service running"
else
    fail "Pathfinder service running"
fi

child_pid=$(jexec "$APP_JAIL" cat /var/run/pathfinder/pathfinder.pid 2>/dev/null || true)
if [ -n "$child_pid" ] && \
    [ "$(jexec "$APP_JAIL" ps -p "$child_pid" -o user= 2>/dev/null | awk '{print $1}')" = "pathfinder" ]; then
    pass "Pathfinder child runs as pathfinder"
else
    fail "Pathfinder child runs as pathfinder"
fi

if jexec "$APP_JAIL" sockstat -4 -l 2>/dev/null | grep -q '127.0.0.1:8080'; then
    pass "Pathfinder health listener loopback-only"
else
    fail "Pathfinder health listener loopback-only"
fi

if [ "$(jexec "$APP_JAIL" fetch -T 2 -qo - http://127.0.0.1:8080/livez 2>/dev/null || true)" = "live" ]; then
    pass "Pathfinder /livez"
else
    fail "Pathfinder /livez"
fi

if [ "$(jexec "$APP_JAIL" fetch -T 2 -qo - http://127.0.0.1:8080/readyz 2>/dev/null || true)" = "ready" ]; then
    pass "Pathfinder /readyz"
else
    fail "Pathfinder /readyz"
fi

if jexec -U pathfinder "$APP_JAIL" \
    test -r /usr/local/etc/pathfinder/secrets/postgresql-password; then
    pass "Pathfinder service can read runtime DB credential"
else
    fail "Pathfinder service can read runtime DB credential"
fi

if jexec -U pathfinder "$APP_JAIL" \
    test -r /usr/local/etc/pathfinder/secrets/postgresql-migration-password; then
    fail "Pathfinder service cannot read migration credential"
else
    pass "Pathfinder service cannot read migration credential"
fi

if jexec "$APP_JAIL" /usr/local/sbin/pathfinder migrate \
    -config /usr/local/etc/pathfinder/pathfinder.conf >/dev/null 2>&1; then
    pass "Pathfinder migration authority preflight"
else
    fail "Pathfinder migration authority preflight"
fi

if jexec -U pathfinder "$APP_JAIL" /usr/local/sbin/pathfinder migrate \
    -config /usr/local/etc/pathfinder/pathfinder.conf >/dev/null 2>&1; then
    fail "Pathfinder service denied migration command"
else
    pass "Pathfinder service denied migration command"
fi

if jexec "$APP_JAIL" sh -c 'command -v go >/dev/null 2>&1'; then
    fail "Go toolchain absent from runtime jail"
else
    pass "Go toolchain absent from runtime jail"
fi

printf '\nPASS=%d FAIL=%d\n' "$PASS_COUNT" "$FAIL_COUNT"

if [ "$FAIL_COUNT" -ne 0 ]; then
    printf 'PATHFINDER PHASE 1.1: FAIL\n'
    exit 1
fi

printf 'PATHFINDER PHASE 1.1: PASS\n'
exit 0
