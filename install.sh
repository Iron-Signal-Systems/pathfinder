#!/bin/sh
#
# Pathfinder installer entry point.
#
# Phase 1 is not yet complete. The install path intentionally fails closed
# after clean-host preflight until every Phase 1 construction stage is owned by
# repository deployment tooling and clean-host validated.
#
# Usage:
#   sh install.sh --config /path/to/pathfinder.conf
#   sh install.sh --preflight --config /path/to/pathfinder.conf
#   sh install.sh --verify --config /path/to/pathfinder.conf

set -u

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEFAULT_CONFIG="${SCRIPT_DIR}/deploy/pathfinder.conf.example"
CONFIG=$DEFAULT_CONFIG
MODE=install

usage()
{
    cat <<'EOF'
Pathfinder Phase 1 installer entry point

Usage:
  sh install.sh --config FILE
  sh install.sh --preflight --config FILE
  sh install.sh --verify --config FILE

Modes:
  install       Future complete Phase 1 installation path. Currently fails
                closed because Phase 1 construction automation is incomplete.
  --preflight   Validate the clean-host starting contract only.
  --verify      Run the repository-owned Phase 1.1 verifier against an
                installed system.

The deployment configuration is non-secret. Do not place passwords, private
keys, API tokens, or other credentials in it.
EOF
}

fatal()
{
    printf 'PATHFINDER INSTALL: FAIL: %s\n' "$1" >&2
    exit 1
}

info()
{
    printf 'PATHFINDER INSTALL: %s\n' "$1"
}

require_command()
{
    command -v "$1" >/dev/null 2>&1 || fatal "required base command not found: $1"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --config)
            [ "$#" -ge 2 ] || fatal "--config requires a file path"
            CONFIG=$2
            shift 2
            ;;
        --preflight)
            MODE=preflight
            shift
            ;;
        --verify)
            MODE=verify
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            fatal "unknown argument: $1"
            ;;
    esac
done

if [ "$(id -u)" -ne 0 ]; then
    fatal "run as root"
fi

if [ ! -r "$CONFIG" ]; then
    fatal "deployment config not readable: $CONFIG"
fi

# shellcheck disable=SC1090
. "$CONFIG"

required_variables="
PATHFINDER_RELEASE
PATHFINDER_PATCH_LEVEL
POSTGRESQL_MAJOR
ZPOOL
EXT_IF
DNS_SERVER
BRIDGE_IF
PRIVATE_NET
PRIVATE_GATEWAY
APP_JAIL
APP_HOSTNAME
APP_IP
APP_EPAIR
DB_JAIL
DB_HOSTNAME
DB_IP
DB_EPAIR
JAIL_ROOT
TEMPLATE_DATASET
TEMPLATE_SNAPSHOT
APP_DATASET
DB_DATASET
ARTIFACT_DATASET
STATE_DATASET
PGDATA_DATASET
"

for variable in $required_variables; do
    eval "value=\${$variable-}"
    [ -n "$value" ] || fatal "required deployment value missing: $variable"
done

for command_name in \
    awk \
    bectl \
    fetch \
    freebsd-version \
    ifconfig \
    jail \
    jexec \
    jls \
    pfctl \
    pkg \
    pw \
    service \
    sha256 \
    sysctl \
    sysrc \
    tar \
    uname \
    zfs \
    zpool
do
    require_command "$command_name"
done

if [ "$(uname -s)" != "FreeBSD" ]; then
    fatal "unsupported operating system: $(uname -s)"
fi

if [ "$(uname -m)" != "amd64" ]; then
    fatal "unsupported architecture: $(uname -m); expected amd64"
fi

actual_version=$(freebsd-version -u 2>/dev/null || true)
expected_version="${PATHFINDER_RELEASE}-${PATHFINDER_PATCH_LEVEL}"

if [ "$actual_version" != "$expected_version" ]; then
    fatal "unsupported FreeBSD userland: ${actual_version:-NOT_KNOWN}; expected $expected_version"
fi

if ! zpool list "$ZPOOL" >/dev/null 2>&1; then
    fatal "configured ZFS root pool not found: $ZPOOL"
fi

if ! zpool status -x "$ZPOOL" 2>/dev/null | grep -q 'is healthy'; then
    fatal "configured ZFS root pool is not healthy: $ZPOOL"
fi

wheel_members=$(pw group show wheel 2>/dev/null | awk -F: '{print $4}')
wheel_non_root=0

old_ifs=$IFS
IFS=','
for member in $wheel_members; do
    case "$member" in
        ''|root)
            ;;
        *)
            wheel_non_root=1
            break
            ;;
    esac
done
IFS=$old_ifs

if [ "$wheel_non_root" -ne 1 ]; then
    fatal "clean-host contract requires at least one non-root user in wheel"
fi

if ! ifconfig "$EXT_IF" >/dev/null 2>&1; then
    fatal "configured external interface not found: $EXT_IF"
fi

info "preflight PASS"
info "FreeBSD: $actual_version"
info "architecture: amd64"
info "ZFS pool: $ZPOOL"
info "external interface: $EXT_IF"
info "PostgreSQL major: $POSTGRESQL_MAJOR"
info "deployment config: $CONFIG"

case "$MODE" in
    preflight)
        exit 0
        ;;

    verify)
        VERIFY_SCRIPT="${SCRIPT_DIR}/deploy/verify/pathfinder-verify.sh"
        [ -r "$VERIFY_SCRIPT" ] || fatal "verification script not found: $VERIFY_SCRIPT"
        exec /bin/sh "$VERIFY_SCRIPT" "$CONFIG"
        ;;

    install)
        cat >&2 <<'EOF'

PATHFINDER INSTALL: REFUSED

The stable install.sh entry point now exists, but the Phase 1 installer is not
release-complete yet. Pathfinder will not perform a partial construction and
report it as an installation.

Phase 1 install remains blocked until repository-owned automation covers and
clean-host validates the complete required construction path documented in:

  docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md

Use --preflight to validate a clean host or --verify to validate the current
Phase 1.1 reference installation.
EOF
        exit 3
        ;;

    *)
        fatal "internal mode error: $MODE"
        ;;
esac
