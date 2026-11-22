#!/bin/sh
# Validate a CI-generated Pathfinder C1 FreeBSD STAGING package.
# Purely read-only: no services, SQL, mount, or jail changes.
set -eu
if [ "$#" -gt 0 ]; then stage=$1; else stage=.; fi
case "$stage" in
    /*) ;;
    *) stage=$(CDPATH= cd -- "$stage" && pwd) ;;
esac
[ -d "$stage" ] || { echo "PATHFINDER_C1_STAGE_FAIL package directory missing" >&2; exit 1; }
[ ! -L "$stage" ] || { echo "PATHFINDER_C1_STAGE_FAIL package directory symlink" >&2; exit 1; }

for path in bin/pathfinder-c1-ingest-dev bin/pathfinder-c1-sor-dev \
    bin/pathfinder-c1-verifier-dev bin/pathfinder-c1-ram-probe-dev \
    bin/pathfinder-c1-frozen-check \
    tools/pathfinder_c1_c220_preflight.sh \
    tools/pathfinder_c1_c220_discover.sh \
    tools/pathfinder_c1_verify_stage.sh \
    tools/pathfinder_c1_dev_pki.sh \
    docs/PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md \
    docs/PATHFINDER-C1-C220-STAGING-HANDOFF.md \
    BUILD-INFO SHA256SUMS; do
    [ -f "$stage/$path" ] && [ ! -L "$stage/$path" ] ||
        { echo "PATHFINDER_C1_STAGE_FAIL missing or symlink $path" >&2; exit 1; }
done
[ -x "$stage/bin/pathfinder-c1-ingest-dev" ] ||
    { echo "PATHFINDER_C1_STAGE_FAIL binary permissions" >&2; exit 1; }
[ "$(wc -l < "$stage/SHA256SUMS" | tr -d ' ')" -eq 11 ] ||
    { echo "PATHFINDER_C1_STAGE_FAIL checksum manifest item count" >&2; exit 1; }

# Exactly eleven named files. No dynamic paths, traversal, or shell evaluation.
count=0
while IFS= read -r line || [ -n "$line" ]; do
    expected=$(printf '%s\n' "$line" | awk '{print $1}')
    relative=$(printf '%s\n' "$line" | awk '{print $2}')
    case "$relative" in
        bin/pathfinder-c1-ingest-dev|bin/pathfinder-c1-sor-dev|\
bin/pathfinder-c1-verifier-dev|bin/pathfinder-c1-ram-probe-dev|\
bin/pathfinder-c1-frozen-check|\
tools/pathfinder_c1_c220_preflight.sh|tools/pathfinder_c1_c220_discover.sh|\
tools/pathfinder_c1_verify_stage.sh|tools/pathfinder_c1_dev_pki.sh|\
docs/PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md|\
docs/PATHFINDER-C1-C220-STAGING-HANDOFF.md) ;;
        *) echo "PATHFINDER_C1_STAGE_FAIL unrecognized manifest path" >&2; exit 1 ;;
    esac
    case "$expected" in
        *[!0-9a-f]*|'') echo "PATHFINDER_C1_STAGE_FAIL invalid checksum" >&2; exit 1 ;;
    esac
    [ "$(printf %s "$expected" | wc -c | tr -d ' ')" -eq 64 ] ||
        { echo "PATHFINDER_C1_STAGE_FAIL checksum width" >&2; exit 1; }
    if command -v sha256 >/dev/null 2>&1; then
        actual=$(sha256 -q "$stage/$relative")
    elif command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$stage/$relative" | awk '{print $1}')
    else
        echo "PATHFINDER_C1_STAGE_FAIL missing SHA256 implementation" >&2
        exit 1
    fi
    [ "$actual" = "$expected" ] ||
        { echo "PATHFINDER_C1_STAGE_FAIL checksum mismatch: $relative" >&2; exit 1; }
    count=$((count+1))
done < "$stage/SHA256SUMS"
[ "$count" -eq 11 ] ||
    { echo "PATHFINDER_C1_STAGE_FAIL incomplete checksums" >&2; exit 1; }
# Ensure each required file is included once (no duplicate manifest entries).
expected_names=$(printf '%s\n' \
    bin/pathfinder-c1-ingest-dev bin/pathfinder-c1-sor-dev \
    bin/pathfinder-c1-verifier-dev bin/pathfinder-c1-ram-probe-dev \
    bin/pathfinder-c1-frozen-check \
    tools/pathfinder_c1_c220_preflight.sh \
    tools/pathfinder_c1_c220_discover.sh \
    tools/pathfinder_c1_verify_stage.sh \
    tools/pathfinder_c1_dev_pki.sh \
    docs/PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md \
    docs/PATHFINDER-C1-C220-STAGING-HANDOFF.md | LC_ALL=C sort)
actual_names=$(awk '{print $2}' "$stage/SHA256SUMS" | LC_ALL=C sort)
[ "$actual_names" = "$expected_names" ] ||
    { echo "PATHFINDER_C1_STAGE_FAIL missing or duplicate checksum entry" >&2; exit 1; }

grep -q '^target=freebsd/amd64$' "$stage/BUILD-INFO" ||
    { echo "PATHFINDER_C1_STAGE_FAIL build target mismatch" >&2; exit 1; }
printf 'PATHFINDER_C1_STAGE_VERIFY_PASS target=freebsd/amd64 files=11 services_started=0 SQL_writes=0\n'
