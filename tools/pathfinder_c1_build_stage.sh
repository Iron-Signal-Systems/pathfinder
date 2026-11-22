#!/bin/sh
# Deterministic FreeBSD/amd64 C1 test-stage builder.
# Run in a clean checkout of the development branch, normally GitHub Actions.
# This builds files only; no service, database, mount or jail changes.
set -eu
umask 022

[ "$#" -eq 2 ] || { echo "usage: sh tools/pathfinder_c1_build_stage.sh EXACT_SOURCE_SHA OUTPUT_DIRECTORY" >&2; exit 2; }
want_sha=$1
destination=$2
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
actual_sha=$(git rev-parse --verify HEAD)
[ "$actual_sha" = "$want_sha" ] ||
    { echo "PATHFINDER_C1_BUILD_FAIL checkout SHA differs from requested source" >&2; exit 1; }
[ "$(git status --porcelain --untracked-files=no | wc -l | tr -d ' ')" -eq 0 ] ||
    { echo "PATHFINDER_C1_BUILD_FAIL tracked files not clean" >&2; exit 1; }
mkdir -p "$destination/bin" "$destination/tools" "$destination/docs"
stage=$(CDPATH= cd -- "$destination" && pwd)
command -v go >/dev/null 2>&1 ||
    { echo "PATHFINDER_C1_BUILD_FAIL Go compiler missing" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 ||
    { echo "PATHFINDER_C1_BUILD_FAIL GNU sha256sum missing on build runner" >&2; exit 1; }

for entry in pathfinder-c1-ingest-dev pathfinder-c1-sor-dev \
    pathfinder-c1-verifier-dev pathfinder-c1-ram-probe-dev; do
    printf 'C1_FREEBSD_BUILD %s\n' "$entry"
    (
      cd "$repo_root/go"
      GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build \
        -tags pathfinder_c1_dev \
        -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
        -o "$stage/bin/$entry" "./cmd/$entry"
    )
    chmod 0755 "$stage/bin/$entry"
done
for entry in pathfinder_c1_c220_preflight.sh \
    pathfinder_c1_c220_discover.sh pathfinder_c1_verify_stage.sh; do
    cp "$repo_root/tools/$entry" "$stage/tools/$entry"
    chmod 0755 "$stage/tools/$entry"
done
cp "$repo_root/docs/PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md" "$stage/docs/"
printf 'source_commit=%s\ntarget=freebsd/amd64\nstatus=unreleased_development_test_only\nreceiver_included=false\n' \
    "$want_sha" > "$stage/BUILD-INFO"

(
 cd "$stage"
 LC_ALL=C sha256sum \
   bin/pathfinder-c1-ingest-dev \
   bin/pathfinder-c1-sor-dev \
   bin/pathfinder-c1-verifier-dev \
   bin/pathfinder-c1-ram-probe-dev \
   tools/pathfinder_c1_c220_preflight.sh \
   tools/pathfinder_c1_c220_discover.sh \
   tools/pathfinder_c1_verify_stage.sh \
   docs/PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md > SHA256SUMS
 sh tools/pathfinder_c1_verify_stage.sh "$stage"
)
printf 'PATHFINDER_C1_FREEBSD_STAGE_BUILD_PASS source=%s target=freebsd/amd64 binaries=4 SQL_writes=0 services_started=0\n' "$want_sha"
