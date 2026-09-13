#!/bin/sh
set -eu

REPO_ROOT="/root/pathfinder-src"
GO_ROOT="${REPO_ROOT}/go"
PHASE14_VERIFY="${REPO_ROOT}/deploy/verify/phase-1.4"

APP_JAIL_ROOT="/usr/local/jails/containers/pathfinder-app"
DB_JAIL_ROOT="/usr/local/jails/containers/pathfinder-db"

BUILD_ROOT="/root/pathfinder-build/phase-1.4-final-verify"
CANDIDATE="${BUILD_ROOT}/pathfinder"
JAIL_STAGE="/var/tmp/pathfinder-phase14-final-verify"
JAIL_CANDIDATE="${APP_JAIL_ROOT}${JAIL_STAGE}/pathfinder"

EXPECTED_HEAD="0135358ecf2bdcd884e937e4ef48f4e2a11c85a1"

EXPECTED_PATHFINDER="c8dd30e3b24c14d72cc405c185eef83017a865eca4622aa5e800c7d623e287ef"
EXPECTED_ARTIFACTD="7fe5cc7fa3eb6d3de72a78fa01e8ddf30f4e5b6dc0091c2c6d39f034acd6015a"

EXPECTED_0001="8b8ca7487268ccc37c67bb7b7b85e01e7603963ea9f3b805779b47dc5bef1c5f"
EXPECTED_0002="3eeab1f592c263ca7d6fc6bb34f4b5e58ea0989759374375196a59c71dc9897c"
EXPECTED_0003="5cb4ade55d5ef03d19c231c8a8b76fe75c85782fe026eb60fb0ab1d43c2ea3c0"
EXPECTED_0004="00113453d05cdcf006b2447178111dbeab836439eeb40016736fa5c6d016ae57"
EXPECTED_0005="a6b80cde7d0ee7a1f4d652e4129e7ee1ebc7b8200d03f2a6049d9c8fdef65308"
EXPECTED_0006="77332b013cc5a37b6df83d065ee45411deab7c917169f2697b4c99f9233a85a9"
EXPECTED_0007="359c69f1ee9d08cd97c2bfbd26d5665de1a6aaf7e5a49a68be452f0c6ce7be51"

EXPECTED_CONTRACT_DOC="5a09749cecd0ab5de67c4e74f8380901b259635258a4048dbabe58f292425467"

EXPECTED_GATE_LIFECYCLE="21ef78ab7311996818d9e95bebc73924bb70eebc7cb3df97b184bff4a2b1ff39"
EXPECTED_GATE_RESILIENCE="665b30564e539b72f66653dbe03627ac5bba840b6b83891d3e6c03e985b23305"
EXPECTED_GATE_CISA="4e45a566df9f04fda60eaeb235ef2be45fb5e8498a037e5ca4630c1f954b78bf"
EXPECTED_GATE_QUERY="c5e61f3cf01ebf1326dd6ac10ab496ba8526c30bb118bfedd6657c3201253a77"
EXPECTED_GATE_FUZZ="0768123ddc9889e51b0ea45902241d81811c4331516f36dbc073f2d72b627fcb"
EXPECTED_GATE_SOURCE_RECORD="b07305f1d71c6df69eac5936a73eee33f308f19fd13d9ffe087b7d0591306b76"

LIVE_CISA="NO"

usage()
{
    cat <<'EOF'
usage: pathfinder-phase-1.4-verify.sh [--live-cisa]

Without --live-cisa, the verifier runs deterministic Phase 1.4 acceptance:
schema identity, repository hygiene, unit/integration tests, vet, bounded fuzzing,
synthetic lifecycle, resilience, SourceRecord immutability, candidate migration
idempotence, cleanup, and production non-interference.

With --live-cisa, the verifier additionally retrieves the canonical CISA KEV
feed into a disposable environment and proves the operator vulnerability query
against that real preserved snapshot.
EOF
}

if [ "$#" -gt 1 ]; then
    usage
    exit 2
fi

if [ "$#" -eq 1 ]; then
    case "$1" in
        --live-cisa)
            LIVE_CISA="YES"
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            usage
            exit 2
            ;;
    esac
fi

fail()
{
    echo "FAIL: $*" >&2
    exit 1
}

assert_equal()
{
    actual="$1"
    expected="$2"
    label="$3"

    if [ "${actual}" != "${expected}" ]; then
        echo "FAIL: ${label}" >&2
        echo "  actual:   ${actual}" >&2
        echo "  expected: ${expected}" >&2
        exit 1
    fi
}

sha_check()
{
    path="$1"
    expected="$2"
    label="$3"

    [ -f "${path}" ] || fail "${label}: missing ${path}"

    actual="$(sha256 -q "${path}")"
    assert_equal "${actual}" "${expected}" "${label} SHA-256"

    echo "PASS: ${label} exact."
}

production_counts()
{
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d pathfinder \
        -c \"SELECT
            (SELECT count(*) FROM pathfinder.processing_run) || '|' ||
            (SELECT count(*) FROM pathfinder.processing_event) || '|' ||
            (SELECT count(*) FROM pathfinder.source_record_processing) || '|' ||
            (SELECT count(*) FROM pathfinder.source_record) || '|' ||
            (SELECT count(*) FROM pathfinder.vulnerability) || '|' ||
            (SELECT count(*) FROM pathfinder.assertion) || '|' ||
            (SELECT count(*) FROM pathfinder.collector_checkpoint);\""
}

migration_ledger()
{
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d pathfinder \
        -c \"SELECT version || '|' || name || '|' || sha256
            FROM pathfinder.schema_migration
            ORDER BY version;\""
}

assert_migration_ledger()
{
    expected="$(
        cat <<'EOF'
1|source-foundation|8b8ca7487268ccc37c67bb7b7b85e01e7603963ea9f3b805779b47dc5bef1c5f
2|source-artifact-preservation|3eeab1f592c263ca7d6fc6bb34f4b5e58ea0989759374375196a59c71dc9897c
3|source-record-foundation|5cb4ade55d5ef03d19c231c8a8b76fe75c85782fe026eb60fb0ab1d43c2ea3c0
4|processing-history|00113453d05cdcf006b2447178111dbeab836439eeb40016736fa5c6d016ae57
5|vulnerability-identity|a6b80cde7d0ee7a1f4d652e4129e7ee1ebc7b8200d03f2a6049d9c8fdef65308
6|source-assertion|77332b013cc5a37b6df83d065ee45411deab7c917169f2697b4c99f9233a85a9
7|collector-checkpoint|359c69f1ee9d08cd97c2bfbd26d5665de1a6aaf7e5a49a68be452f0c6ce7be51
EOF
    )"

    actual="$(migration_ledger)"
    assert_equal "${actual}" "${expected}" "Phase 1.4 migration ledger"

    echo "PASS: Phase 1.4 migration ledger exact."
}

assert_no_disposable_environment()
{
    database_count="$(
        jexec pfdb su -m postgres -c \
            "/usr/local/bin/psql -X -A -t -d postgres \
            -c \"SELECT count(*)
                FROM pg_database
                WHERE datname = 'pathfinder_phase14_lifecycle';\""
    )"

    assert_equal "${database_count}" "0" "disposable Phase 1.4 database removal"

    hba_rules="$(
        jexec pfdb su -m postgres -c \
            "/usr/local/bin/psql -X -A -t -d postgres \
            -c \"SELECT count(*)
                FROM pg_hba_file_rules
                WHERE database = ARRAY['pathfinder_phase14_lifecycle']
                  AND user_name = ARRAY['pathfinder_app']
                  AND address = '10.77.0.10'
                  AND netmask = '255.255.255.255';\""
    )"

    assert_equal "${hba_rules}" "0" "temporary Phase 1.4 pg_hba rule removal"

    if jexec pfapp test -e /var/tmp/pathfinder-phase14-lifecycle; then
        fail "disposable Phase 1.4 jail tree remains"
    fi

    echo "PASS: no disposable Phase 1.4 database, pg_hba rule, or artifact tree remains."
}

assert_production_baseline()
{
    counts="$(production_counts)"
    assert_equal "${counts}" "0|0|0|0|0|0|0" "production Phase 1.4 semantic counts"

    jexec -U pathfinder pfapp \
        /usr/local/sbin/pathfinder \
        source-artifact reconcile \
        -config /usr/local/etc/pathfinder/pathfinder.conf

    pathfinder_sha="$(
        sha256 -q \
            "${APP_JAIL_ROOT}/usr/local/sbin/pathfinder"
    )"

    artifactd_sha="$(
        sha256 -q \
            "${APP_JAIL_ROOT}/usr/local/sbin/pathfinder-artifactd"
    )"

    assert_equal "${pathfinder_sha}" "${EXPECTED_PATHFINDER}" "installed Phase 1.3 Pathfinder binary"
    assert_equal "${artifactd_sha}" "${EXPECTED_ARTIFACTD}" "installed Phase 1.3 artifactd binary"

    echo "PASS: production remains Phase 1.3 runtime with pristine Phase 1.4 semantic tables."
}

cleanup()
{
    status="$?"

    rm -rf "${APP_JAIL_ROOT}${JAIL_STAGE}" 2>/dev/null || true

    if [ "${status}" -ne 0 ]; then
        echo
        echo "===== PHASE 1.4 FINAL VERIFIER: FAILED ====="
    fi

    exit "${status}"
}

trap cleanup EXIT INT TERM HUP

echo "===== PATHFINDER PHASE 1.4 FINAL VERIFIER ====="
echo "live_cisa=${LIVE_CISA}"

echo
echo "===== HOST / REPOSITORY IDENTITY ====="

freebsd-version -kru

git -C "${REPO_ROOT}" rev-parse --show-toplevel
HEAD="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
echo "head=${HEAD}"
assert_equal "${HEAD}" "${EXPECTED_HEAD}" "pre-commit Phase 1.4 base HEAD"

git -C "${REPO_ROOT}" diff --cached --quiet ||
    fail "Git index is not clean; final verifier must not run with staged changes"

git -C "${REPO_ROOT}" diff --check

echo "PASS: repository identity and whitespace check clean."

echo
echo "===== REPOSITORY HYGIENE ====="

TEMP_FILES="$(
    find "${REPO_ROOT}" \
        -type f \
        \( \
            -name '*.orig' -o \
            -name '*.rej' -o \
            -name '*.bak' \
        \) \
        -print
)"

[ -z "${TEMP_FILES}" ] || {
    echo "${TEMP_FILES}"
    fail "temporary patch/backup files remain"
}

FUZZ_FAILURES="$(
    find "${GO_ROOT}" \
        -type f \
        -path '*/testdata/fuzz/*' \
        -print
)"

[ -z "${FUZZ_FAILURES}" ] || {
    echo "${FUZZ_FAILURES}"
    fail "fuzz failure corpus exists"
}

echo "PASS: repository has no temporary patch debris or fuzz failure corpus."

echo
echo "===== PRODUCTION SERVICE BASELINE ====="

jexec pfdb service postgresql status
jexec pfapp service pathfinder_artifactd status
jexec pfapp service pathfinder status

assert_equal "$(
    jexec pfapp fetch -T 2 -qo - http://127.0.0.1:8080/livez
)" "live" "production livez"

assert_equal "$(
    jexec pfapp fetch -T 2 -qo - http://127.0.0.1:8080/readyz
)" "ready" "production readyz"

jexec -U pathfinder pfapp \
    /usr/local/sbin/pathfinder validate \
    -config /usr/local/etc/pathfinder/pathfinder.conf

assert_production_baseline
assert_no_disposable_environment

echo
echo "===== PHASE 1.4 SCHEMA FILE IDENTITY ====="

sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0001-source-foundation.sql" "${EXPECTED_0001}" "migration 0001"
sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0002-source-artifact.sql" "${EXPECTED_0002}" "migration 0002"
sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0003-source-record.sql" "${EXPECTED_0003}" "migration 0003"
sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0004-processing-history.sql" "${EXPECTED_0004}" "migration 0004"
sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0005-vulnerability.sql" "${EXPECTED_0005}" "migration 0005"
sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0006-assertion.sql" "${EXPECTED_0006}" "migration 0006"
sha_check "${GO_ROOT}/cmd/pathfinder/migrations/0007-collector-checkpoint.sql" "${EXPECTED_0007}" "migration 0007"

sha_check "${REPO_ROOT}/docs/PHASE-1.4-FIRST-EXTERNAL-COLLECTOR.md" "${EXPECTED_CONTRACT_DOC}" "Phase 1.4 collector contract"

assert_migration_ledger

TABLES="$(
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d pathfinder \
        -c \"SELECT tablename
            FROM pg_catalog.pg_tables
            WHERE schemaname = 'pathfinder'
            ORDER BY tablename;\""
)"

EXPECTED_TABLES="$(
    cat <<'EOF'
assertion
collector_checkpoint
processing_event
processing_run
retrieval_event
schema_migration
source
source_artifact
source_collection
source_record
source_record_processing
vulnerability
EOF
)"

assert_equal "${TABLES}" "${EXPECTED_TABLES}" "Phase 1.4 table inventory"
echo "PASS: Phase 1.4 schema table inventory exact."

echo
echo "===== ACCEPTED GATE IDENTITY ====="

sha_check "${PHASE14_VERIFY}/run-live-lifecycle-gate.sh" "${EXPECTED_GATE_LIFECYCLE}" "lifecycle gate"
sha_check "${PHASE14_VERIFY}/run-live-resilience-gate.sh" "${EXPECTED_GATE_RESILIENCE}" "resilience gate"
sha_check "${PHASE14_VERIFY}/run-live-cisa-acquisition-gate.sh" "${EXPECTED_GATE_CISA}" "canonical CISA acquisition gate"
sha_check "${PHASE14_VERIFY}/run-live-vulnerability-query-gate.sh" "${EXPECTED_GATE_QUERY}" "vulnerability query gate"
sha_check "${PHASE14_VERIFY}/run-fuzz-gate.sh" "${EXPECTED_GATE_FUZZ}" "fuzz gate"
sha_check "${PHASE14_VERIFY}/run-source-record-guard-gate.sh" "${EXPECTED_GATE_SOURCE_RECORD}" "SourceRecord identity gate"

for gate in \
    "${PHASE14_VERIFY}/run-live-lifecycle-gate.sh" \
    "${PHASE14_VERIFY}/run-live-resilience-gate.sh" \
    "${PHASE14_VERIFY}/run-live-cisa-acquisition-gate.sh" \
    "${PHASE14_VERIFY}/run-live-vulnerability-query-gate.sh" \
    "${PHASE14_VERIFY}/run-fuzz-gate.sh" \
    "${PHASE14_VERIFY}/run-source-record-guard-gate.sh"
do
    sh -n "${gate}"
done

echo "PASS: all repo-owned Phase 1.4 gate scripts have valid shell syntax."

echo
echo "===== FORMAT ====="

FORMAT_OUTPUT="$(
    cd "${GO_ROOT}"
    find cmd/pathfinder internal \
        -type f \
        -name '*.go' \
        -exec gofmt -l {} +
)"

[ -z "${FORMAT_OUTPUT}" ] || {
    echo "${FORMAT_OUTPUT}"
    fail "Go source requires gofmt"
}

echo "PASS: Go source formatted."

echo
echo "===== FULL TEST SUITE ====="

cd "${GO_ROOT}"
go test ./... -count=1

echo
echo "===== VET ====="

go vet ./...

echo
echo "===== BUILD PRE-PROMOTION SOURCE CANDIDATE ====="

mkdir -p "${BUILD_ROOT}"
rm -f "${CANDIDATE}"

go build \
    -o "${CANDIDATE}" \
    ./cmd/pathfinder

CANDIDATE_SHA="$(sha256 -q "${CANDIDATE}")"
echo "prepromotion_candidate_sha256=${CANDIDATE_SHA}"

[ -n "${CANDIDATE_SHA}" ] ||
    fail "candidate SHA-256 is empty"

echo
echo "===== CANDIDATE CLI SURFACE ====="

"${CANDIDATE}" 2>&1 || true

if ! "${CANDIDATE}" 2>&1 | grep -F 'vulnerability' >/dev/null; then
    fail "candidate CLI does not expose vulnerability command"
fi

echo "PASS: candidate CLI exposes Phase 1.4 vulnerability command."

echo
echo "===== CANDIDATE MIGRATION IDEMPOTENCE AGAINST PRODUCTION LEDGER ====="

rm -rf "${APP_JAIL_ROOT}${JAIL_STAGE}"
install -d -o root -g wheel -m 0755 "${APP_JAIL_ROOT}${JAIL_STAGE}"
install -o root -g wheel -m 0555 "${CANDIDATE}" "${JAIL_CANDIDATE}"

MIGRATE_OUTPUT="$(
    jexec pfapp \
        "${JAIL_STAGE}/pathfinder" \
        migrate \
        -config /usr/local/etc/pathfinder/pathfinder.conf
)"

echo "${MIGRATE_OUTPUT}"

for migration in \
    "0001 source-foundation" \
    "0002 source-artifact-preservation" \
    "0003 source-record-foundation" \
    "0004 processing-history" \
    "0005 vulnerability-identity" \
    "0006 source-assertion" \
    "0007 collector-checkpoint"
do
    echo "${MIGRATE_OUTPUT}" |
        grep -F "Migration ${migration}: ALREADY_APPLIED" >/dev/null ||
        fail "candidate migration idempotence missing ${migration}"
done

echo "${MIGRATE_OUTPUT}" |
    grep -F 'migrations_pending=0' >/dev/null ||
    fail "candidate reports pending production migrations"

echo "${MIGRATE_OUTPUT}" |
    grep -F 'migrations_applied=0' >/dev/null ||
    fail "candidate unexpectedly applied a production migration"

assert_migration_ledger
assert_production_baseline

rm -rf "${APP_JAIL_ROOT}${JAIL_STAGE}"

echo "PASS: current candidate recognizes the exact seven-row production migration ledger without mutation."

echo
echo "===== LIVE SYNTHETIC LIFECYCLE ====="

sh "${PHASE14_VERIFY}/run-live-lifecycle-gate.sh"
assert_no_disposable_environment
assert_production_baseline

echo
echo "===== LIVE RESILIENCE ====="

sh "${PHASE14_VERIFY}/run-live-resilience-gate.sh"
assert_no_disposable_environment
assert_production_baseline

if [ "${LIVE_CISA}" = "YES" ]; then
    echo
    echo "===== LIVE CANONICAL CISA + OPERATOR QUERY ====="

    # The vulnerability-query gate invokes the accepted canonical CISA
    # acquisition integration test before executing the operator query. This
    # proves exact-byte preservation and full provenance without performing a
    # second redundant external fetch in this final verifier.
    sh "${PHASE14_VERIFY}/run-live-vulnerability-query-gate.sh"

    assert_no_disposable_environment
    assert_production_baseline
else
    echo
    echo "===== LIVE CANONICAL CISA + OPERATOR QUERY ====="
    echo "SKIP: rerun with --live-cisa for the external-source acceptance path."
fi

echo
echo "===== IMMUTABLE SOURCE RECORD IDENTITY ====="

sh "${PHASE14_VERIFY}/run-source-record-guard-gate.sh"
assert_no_disposable_environment
assert_production_baseline

echo
echo "===== BOUNDED FUZZ ACCEPTANCE ====="

sh "${PHASE14_VERIFY}/run-fuzz-gate.sh"
assert_no_disposable_environment
assert_production_baseline

echo
echo "===== FINAL AGGREGATE CLEANUP / NON-INTERFERENCE ====="

assert_no_disposable_environment
assert_migration_ledger
assert_production_baseline

if jexec pfapp sh -c 'command -v go >/dev/null 2>&1'; then
    fail "Go is installed in the application jail"
fi

echo "PASS: Go remains absent from application jail."

HBA_JAIL_PATH="$(
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d postgres -c 'SHOW hba_file;'"
)"

HBA_HOST_PATH="${DB_JAIL_ROOT}${HBA_JAIL_PATH}"
HBA_LEFTOVERS="$(
    find "$(dirname "${HBA_HOST_PATH}")" \
        -maxdepth 1 \
        -type f \
        -name 'pg_hba.conf.phase14-*' \
        -print
)"

[ -z "${HBA_LEFTOVERS}" ] || {
    echo "${HBA_LEFTOVERS}"
    fail "Phase 1.4 temporary pg_hba backup files remain"
}

echo "PASS: no Phase 1.4 pg_hba temporary files remain."

FINAL_CANDIDATE_SHA="$(sha256 -q "${CANDIDATE}")"
assert_equal "${FINAL_CANDIDATE_SHA}" "${CANDIDATE_SHA}" "pre-promotion candidate stability"

FINAL_HEAD="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
assert_equal "${FINAL_HEAD}" "${EXPECTED_HEAD}" "repository HEAD stability"

git -C "${REPO_ROOT}" diff --cached --quiet ||
    fail "Git index changed during final verification"

git -C "${REPO_ROOT}" diff --check

echo
echo "===== FINAL REPOSITORY STATUS ====="

git -C "${REPO_ROOT}" status --short

echo
echo "===== PHASE 1.4 FINAL ACCEPTANCE SUMMARY ====="

echo "PASS: exact seven-migration Phase 1.4 schema identity and ledger."
echo "PASS: full Go suite and vet."
echo "PASS: synthetic semantic lifecycle."
echo "PASS: retry/idempotency, interruption recovery, malformed-artifact failure, and cross-artifact rejection."
if [ "${LIVE_CISA}" = "YES" ]; then
    echo "PASS: canonical CISA KEV HTTPS acquisition, exact-byte preservation, real semantic snapshot, and operator query."
else
    echo "SKIP: canonical CISA/query rerun not requested for this invocation."
fi
echo "PASS: immutable SourceRecord reuse and conflict rollback."
echo "PASS: bounded fuzzing with no invariant violation or saved failure corpus."
echo "PASS: production semantic tables remained pristine."
echo "PASS: production SourceArtifact reconciliation remained clean."
echo "PASS: installed production runtime remained exact Phase 1.3 binaries."
echo "PASS: application jail remained Go-free."
echo "PASS: no disposable DB, artifact tree, pg_hba rule, or pg_hba temp files remain."
echo "PASS: Git index stayed clean; no commit, push, install, or promotion occurred."
echo "prepromotion_candidate_sha256=${CANDIDATE_SHA}"

echo
echo "===== PATHFINDER PHASE 1.4 FINAL VERIFIER: PASS ====="
