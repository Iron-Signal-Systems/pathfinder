#!/bin/sh
#
# Pathfinder Phase 1.1 platform verification.
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
    printf '[FAIL] run as root; PF and jail verification require host authority\n' >&2
    exit 2
fi

printf 'Pathfinder Phase 1.1 platform verification\n'
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

if jexec "$APP_JAIL" fetch -qo /dev/null https://download.freebsd.org/ >/dev/null 2>&1; then
    pass "$APP_JAIL HTTPS egress"
else
    fail "$APP_JAIL HTTPS egress"
fi

if jexec "$DB_JAIL" fetch -qo /dev/null https://download.freebsd.org/ >/dev/null 2>&1; then
    pass "$DB_JAIL HTTPS egress (temporary provisioning allowance)"
else
    fail "$DB_JAIL HTTPS egress (temporary provisioning allowance)"
fi

printf '\nPASS=%d FAIL=%d\n' "$PASS_COUNT" "$FAIL_COUNT"

if [ "$FAIL_COUNT" -ne 0 ]; then
    printf 'PATHFINDER PLATFORM: FAIL\n'
    exit 1
fi

printf 'PATHFINDER PLATFORM: PASS\n'
exit 0
