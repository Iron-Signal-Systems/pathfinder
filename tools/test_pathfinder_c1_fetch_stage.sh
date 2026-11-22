#!/bin/sh
# CI simulation of exact-commit pathfinder-dev retrieval using a local mock gh.
# Source staging artifact is built independently by the CI stage builder.
set -eu
[ "$#" -eq 1 ] || { echo "usage: test_pathfinder_c1_fetch_stage.sh BUILD_STAGE" >&2; exit 2; }
source_stage=$1
base=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/mock"
cat > "$tmp/mock/gh" <<'MOCK'
#!/bin/sh
set -eu
case "$1:$2" in
  run:list)
    printf '%s\n' 77777777 ;;
  run:view)
    for arg in "$@"; do
      case "$arg" in
        .headSha) printf '%s\n' "${PF_C1_TEST_RUN_SHA_OVERRIDE:-$PF_C1_TEST_SOURCE_SHA}"; exit 0 ;;
        .headBranch) printf '%s\n' dev/c1-ingest-sor-20261122; exit 0 ;;
        .conclusion) printf '%s\n' success; exit 0 ;;
      esac
    done
    exit 1 ;;
  run:download)
    destination=''
    while [ "$#" -gt 0 ]; do
      if [ "$1" = '-D' ]; then
        destination=$2
        break
      fi
      shift
    done
    [ -n "$destination" ] || exit 1
    cp -R "$PF_C1_TEST_SOURCE_STAGE/." "$destination/"
    if [ "${PF_C1_TEST_TAMPER:-0}" = 1 ]; then
      printf '\nTAMPER\n' >> "$destination/tools/pathfinder_c1_verify_stage.sh"
    fi ;;
  *) exit 1 ;;
esac
MOCK
chmod 0700 "$tmp/mock/gh"
export PATH="$tmp/mock:$PATH"
export PF_C1_TEST_SOURCE_STAGE="$source_stage"
export PF_C1_TEST_SOURCE_SHA
PF_C1_TEST_SOURCE_SHA=$(git -C "$base" rev-parse HEAD)
target_script="$base/tools/pathfinder_c1_fetch_stage_from_gh.sh"

sh "$target_script" "$tmp/valid" >/dev/null ||
  { echo "C1 MOCK FETCH FAIL: valid CI artifact rejected" >&2; exit 1; }
[ -f "$tmp/valid/SHA256SUMS" ] ||
  { echo "C1 MOCK FETCH FAIL: verified staging directory missing" >&2; exit 1; }
if PF_C1_TEST_RUN_SHA_OVERRIDE=0000000000000000000000000000000000000000 \
  sh "$target_script" "$tmp/wrong-sha" >/dev/null 2>&1; then
  echo "C1 MOCK FETCH FAIL: wrong workflow SHA accepted" >&2
  exit 1
fi
[ ! -e "$tmp/wrong-sha" ] ||
  { echo "C1 MOCK FETCH FAIL: wrong SHA created output" >&2; exit 1; }
if PF_C1_TEST_TAMPER=1 sh "$target_script" "$tmp/tamper" >/dev/null 2>&1; then
  echo "C1 MOCK FETCH FAIL: changed downloaded package accepted" >&2
  exit 1
fi
if sh "$target_script" "$tmp/valid" >/dev/null 2>&1; then
  echo "C1 MOCK FETCH FAIL: existing destination overwritten" >&2
  exit 1
fi
printf '%s\n' 'PATHFINDER_C1_DEV_FETCH_MOCK_PASS verified=1 wrong_run_rejected=1 checksum_tamper_rejected=1 overwrite_rejected=1'
