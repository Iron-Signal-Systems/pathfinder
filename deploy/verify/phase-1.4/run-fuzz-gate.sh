#!/bin/sh
set -eu

REPO_ROOT="/root/pathfinder-src"
GO_ROOT="${REPO_ROOT}/go"
DISPOSABLE_DB="pathfinder_phase14_lifecycle"
DISPOSABLE_TREE="/var/tmp/pathfinder-phase14-lifecycle"

EXPECTED_PATHFINDER="c8dd30e3b24c14d72cc405c185eef83017a865eca4622aa5e800c7d623e287ef"
EXPECTED_ARTIFACTD="7fe5cc7fa3eb6d3de72a78fa01e8ddf30f4e5b6dc0091c2c6d39f034acd6015a"

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

run_fuzz()
{
    package="$1"
    target="$2"

    echo
    echo "===== ${target} ====="

    GOMAXPROCS=2 go test \
        "${package}" \
        -run='^$' \
        -fuzz="^${target}$" \
        -fuzztime=5s \
        -parallel=2
}

echo "===== PHASE 1.4 FUZZING ACCEPTANCE GATE ====="

echo
echo "===== PRIOR DISPOSABLE ENVIRONMENT MUST BE GONE ====="

DB_EXISTS="$(
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d postgres \
        -c \"SELECT count(*)
            FROM pg_database
            WHERE datname = '${DISPOSABLE_DB}';\""
)"

echo "disposable_database_count=${DB_EXISTS}"

[ "${DB_EXISTS}" = "0" ] || {
    echo "FAIL: prior disposable database remains."
    exit 1
}

HBA_RULES="$(
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d postgres \
        -c \"SELECT count(*)
            FROM pg_hba_file_rules
            WHERE database = ARRAY['pathfinder_phase14_lifecycle']
              AND user_name = ARRAY['pathfinder_app']
              AND address = '10.77.0.10'
              AND netmask = '255.255.255.255';\""
)"

echo "temporary_hba_rules=${HBA_RULES}"

[ "${HBA_RULES}" = "0" ] || {
    echo "FAIL: prior temporary pg_hba rule remains."
    exit 1
}

if jexec pfapp test -e "${DISPOSABLE_TREE}"; then
    echo "FAIL: prior disposable artifact/runtime tree remains."
    exit 1
fi

echo "PASS: prior disposable query environment fully removed."

echo
echo "===== PRODUCTION BEFORE ====="

PROD_BEFORE="$(production_counts)"
echo "${PROD_BEFORE}"

if [ "${PROD_BEFORE}" != "0|0|0|0|0|0|0" ]; then
    echo "FAIL: production Phase 1.4 semantic tables are not pristine."
    exit 1
fi

echo
echo "===== FORMAT ====="

cd "${GO_ROOT}"

FUZZ_FILES="
internal/collectors/cisakev/kev_fuzz_test.go
internal/collectors/cisakev/acquire_fuzz_test.go
internal/processinghistory/processinghistory_fuzz_test.go
internal/semantictransaction/transaction_fuzz_test.go
internal/sourceartifact/storage_reference_fuzz_test.go
cmd/pathfinder/vulnerability_fuzz_test.go
"

# shellcheck disable=SC2086
FORMATTING="$(gofmt -l ${FUZZ_FILES})"
if [ -n "${FORMATTING}" ]; then
    echo "FAIL: fuzz files require gofmt."
    echo "${FORMATTING}"
    exit 1
fi

echo "PASS: fuzz files formatted."

echo
echo "===== SEED CORPUS / NORMAL TEST SUITE ====="

go test ./... -count=1

echo
echo "===== VET ====="

go vet ./...

echo
echo "===== BOUNDED FUZZ RUNS ====="

run_fuzz ./internal/collectors/cisakev FuzzParse
run_fuzz ./internal/collectors/cisakev FuzzBoundedReadCloser
run_fuzz ./internal/processinghistory FuzzProcessingHistoryValidators
run_fuzz ./internal/semantictransaction FuzzBuildSuccessSQL
run_fuzz ./internal/sourceartifact FuzzExpectedStorageReference
run_fuzz ./cmd/pathfinder FuzzVulnerabilityCVEPattern

echo
echo "===== NORMAL SUITE AFTER FUZZING ====="

go test ./... -count=1

echo
echo "===== PRODUCTION AFTER ====="

PROD_AFTER="$(production_counts)"
echo "${PROD_AFTER}"

if [ "${PROD_AFTER}" != "${PROD_BEFORE}" ]; then
    echo "FAIL: production semantic tables changed during fuzzing."
    exit 1
fi

echo
echo "===== PRODUCTION SOURCEARTIFACT RECONCILIATION ====="

jexec -U pathfinder pfapp \
    /usr/local/sbin/pathfinder \
    source-artifact reconcile \
    -config /usr/local/etc/pathfinder/pathfinder.conf

echo
echo "===== INSTALLED PRODUCTION BINARIES ====="

PATHFINDER_SHA="$(
    sha256 -q \
        /usr/local/jails/containers/pathfinder-app/usr/local/sbin/pathfinder
)"
ARTIFACTD_SHA="$(
    sha256 -q \
        /usr/local/jails/containers/pathfinder-app/usr/local/sbin/pathfinder-artifactd
)"

echo "pathfinder=${PATHFINDER_SHA}"
echo "artifactd=${ARTIFACTD_SHA}"

[ "${PATHFINDER_SHA}" = "${EXPECTED_PATHFINDER}" ] || {
    echo "FAIL: installed Pathfinder changed."
    exit 1
}

[ "${ARTIFACTD_SHA}" = "${EXPECTED_ARTIFACTD}" ] || {
    echo "FAIL: installed artifactd changed."
    exit 1
}

echo
echo "===== REPOSITORY CHECK ====="

git -C "${REPO_ROOT}" diff --check
git -C "${REPO_ROOT}" status --short

echo
echo "PASS: six Phase 1.4 fuzz targets completed without crash or invariant violation."
echo "PASS: canonical SHA-256 storage references reject non-lowercase-hex input."
echo "PASS: production semantic tables unchanged."
echo "PASS: production SourceArtifact reconciliation clean."
echo "PASS: installed Phase 1.3 binaries unchanged."

echo
echo "===== PHASE 1.4 FUZZING ACCEPTANCE GATE: PASS ====="
