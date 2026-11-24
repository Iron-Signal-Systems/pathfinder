#!/bin/sh
# Fetch the C1 CI-built FreeBSD staging artifact FROM pathfinder-dev.
# No runtime jail, host mount, SQL, certificate or service is modified.
set -eu
umask 077

fail() {
  printf 'PATHFINDER_C1_DEV_FETCH_FAIL %s\n' "$*" >&2
  exit 1
}
[ "$#" -eq 1 ] || fail "usage: sh tools/pathfinder_c1_fetch_stage_from_gh.sh /ABSOLUTE/NEW/STAGING/DIR"
stage=$1
case "$stage" in
  /*) ;;
  *) fail "staging directory must be absolute" ;;
esac
case "$stage" in
  /|*/../*|*/./*) fail "unsafe staging directory" ;;
esac
[ ! -e "$stage" ] && [ ! -L "$stage" ] || fail "staging directory already exists; refuse overwrite"
[ "$(id -u)" -ne 0 ] || fail "run as non-root pathfinder-dev checkout owner"
command -v git >/dev/null 2>&1 || fail "git is required in pathfinder-dev"
command -v gh >/dev/null 2>&1 || fail "GitHub CLI gh is required in pathfinder-dev (authenticate separately)"
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
[ "$(git -C "$repo_root" status --porcelain --untracked-files=no)" = "" ] ||
  fail "tracked checkout has modifications"
source_sha=$(git -C "$repo_root" rev-parse --verify HEAD) ||
  fail "checkout HEAD not available"
case "$source_sha" in
  *[!0-9a-f]*|'') fail "invalid checkout SHA" ;;
esac
[ "${#source_sha}" -eq 40 ] || fail "checkout SHA is not 40 characters"

repository=Iron-Signal-Systems/pathfinder
branch=dev/c1-ingest-sor-20261122
workflow=pathfinder-c1-go-development.yml
artifact=pathfinder-c1-freebsd15-amd64

# The chosen run must be a completed successful PUSH workflow for EXACTLY
# this checkout, not merely the newest workflow on the development branch.
run_ids=$(gh run list -R "$repository" --workflow "$workflow" --branch "$branch" \
  --event push --limit 100 --json databaseId,headSha,conclusion \
  --jq ".[] | select(.headSha == \"$source_sha\" and .conclusion == \"success\") | .databaseId") ||
  fail "cannot query GitHub Actions (check gh auth / Actions read access)"
run_id=$(printf '%s\n' "$run_ids" | sed -n '1p')
case "$run_id" in
  *[!0-9]*|'') fail "no successful C1 staging workflow for this exact checkout" ;;
esac
run_sha=$(gh run view "$run_id" -R "$repository" --json headSha --jq .headSha) ||
  fail "cannot verify run SHA"
run_branch=$(gh run view "$run_id" -R "$repository" --json headBranch --jq .headBranch) ||
  fail "cannot verify run branch"
run_conclusion=$(gh run view "$run_id" -R "$repository" --json conclusion --jq .conclusion) ||
  fail "cannot verify run status"
[ "$run_sha" = "$source_sha" ] || fail "workflow source SHA differs from checkout"
[ "$run_branch" = "$branch" ] || fail "workflow branch differs"
[ "$run_conclusion" = success ] || fail "workflow did not succeed"

mkdir -m 0700 "$stage" || fail "cannot create new private staging directory"
gh run download "$run_id" -R "$repository" -n "$artifact" -D "$stage" ||
  fail "artifact download failed; do not use incomplete staging directory"

[ -f "$stage/BUILD-INFO" ] || fail "BUILD-INFO missing"
[ ! -L "$stage/BUILD-INFO" ] || fail "BUILD-INFO symlink rejected"
grep -Fx "source_commit=$source_sha" "$stage/BUILD-INFO" >/dev/null ||
  fail "downloaded package is from a different source SHA"
sh "$repo_root/tools/pathfinder_c1_verify_stage.sh" "$stage" ||
  fail "source-checkout package verification failed"
sh "$stage/tools/pathfinder_c1_verify_stage.sh" "$stage" ||
  fail "downloaded package verification failed"

printf 'PATHFINDER_C1_DEV_FETCH_PASS source_commit=%s run_id=%s stage=%s runtime_modified=false SQL_writes=0\n' \
  "$source_sha" "$run_id" "$stage"
printf '%s\n' 'NEXT: reconcile host-visible jail root and run C220 READ-ONLY discovery on FreeBSD host.'
