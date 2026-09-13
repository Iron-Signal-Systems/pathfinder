#!/bin/sh
set -eu

REPO_ROOT="/root/pathfinder-src"
GO_ROOT="${REPO_ROOT}/go"
BUILD_ROOT="/root/pathfinder-build/phase-1.4-source-record-guard"
JAIL_ROOT="/usr/local/jails/containers/pathfinder-app"
DB_JAIL_ROOT="/usr/local/jails/containers/pathfinder-db"
TEST_ROOT="/var/tmp/pathfinder-phase14-lifecycle"
TEST_DB="pathfinder_phase14_lifecycle"
TEST_BINARY="${BUILD_ROOT}/pathfinder-phase14-source-record-guard.test"
ARTIFACTD_BINARY="${BUILD_ROOT}/pathfinder-artifactd-phase14-source-record-guard"
JAIL_TEST_BINARY="${JAIL_ROOT}${TEST_ROOT}/pathfinder-phase14-source-record-guard.test"
JAIL_ARTIFACTD_BINARY="${JAIL_ROOT}${TEST_ROOT}/pathfinder-artifactd-phase14-source-record-guard"
PROD_CONFIG="${JAIL_ROOT}/usr/local/etc/pathfinder/pathfinder.conf"
TEST_CONFIG="${JAIL_ROOT}${TEST_ROOT}/pathfinder.conf"

EXPECTED_LIFECYCLE_HELPER="97e092d466ae62eb68b71bbdb39e99e6c2080c9663604690754df931cde2c647"
EXPECTED_PATHFINDER="c8dd30e3b24c14d72cc405c185eef83017a865eca4622aa5e800c7d623e287ef"
EXPECTED_ARTIFACTD="7fe5cc7fa3eb6d3de72a78fa01e8ddf30f4e5b6dc0091c2c6d39f034acd6015a"

HBA_FILE=""
HBA_BACKUP=""

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

cleanup()
{
    status="$?"

    if [ -n "${HBA_FILE}" ] && [ -n "${HBA_BACKUP}" ] && [ -f "${HBA_BACKUP}" ]; then
        cp -p "${HBA_BACKUP}" "${HBA_FILE}" || true
        rm -f "${HBA_BACKUP}" || true
        jexec pfdb su -m postgres -c \
            "/usr/local/bin/psql -X -A -t -d postgres -c 'SELECT pg_reload_conf();'" \
            >/dev/null 2>&1 || true
    fi

    if jexec pfapp test -f "${TEST_ROOT}/artifactd.pid" 2>/dev/null; then
        pid="$(jexec pfapp cat "${TEST_ROOT}/artifactd.pid" 2>/dev/null || true)"
        [ -z "${pid}" ] || jexec pfapp kill "${pid}" 2>/dev/null || true
    fi

    jexec pfdb su -m postgres -c \
        "/usr/local/bin/dropdb --if-exists ${TEST_DB}" \
        >/dev/null 2>&1 || true

    rm -rf "${JAIL_ROOT}${TEST_ROOT}"

    if [ "${status}" -ne 0 ]; then
        echo
        echo "===== PHASE 1.4 SOURCE RECORD IDENTITY GUARD GATE FAILED ====="
    fi

    exit "${status}"
}

trap cleanup EXIT INT TERM HUP

echo "===== PHASE 1.4 SOURCE RECORD IDENTITY GUARD GATE ====="

echo
echo "===== PRIOR DISPOSABLE ENVIRONMENT MUST BE GONE ====="

DB_EXISTS="$(jexec pfdb su -m postgres -c "/usr/local/bin/psql -X -A -t -d postgres -c \"SELECT count(*) FROM pg_database WHERE datname='${TEST_DB}';\"")"
echo "disposable_database_count=${DB_EXISTS}"
[ "${DB_EXISTS}" = "0" ] || { echo "FAIL: disposable database remains."; exit 1; }

echo
echo "===== PRODUCTION BEFORE ====="
PROD_BEFORE="$(production_counts)"
echo "${PROD_BEFORE}"
[ "${PROD_BEFORE}" = "0|0|0|0|0|0|0" ] || { echo "FAIL: production semantic tables are not pristine."; exit 1; }

echo
echo "===== ACCEPTED HELPER ====="
[ "$(sha256 -q "${GO_ROOT}/cmd/pathfinder/phase14_lifecycle_integration_test.go")" = "${EXPECTED_LIFECYCLE_HELPER}" ] || {
    echo "FAIL: lifecycle helper drifted."
    exit 1
}
echo "PASS: accepted lifecycle helper exact."

echo
echo "===== NORMAL SUITE / VET ====="
cd "${GO_ROOT}"
go test ./... -count=1
go vet ./...

echo
echo "===== BUILD DISPOSABLE TEST BINARIES ====="
mkdir -p "${BUILD_ROOT}"
go test -c -o "${TEST_BINARY}" ./cmd/pathfinder
go build -o "${ARTIFACTD_BINARY}" ./cmd/pathfinder-artifactd

echo
echo "===== CREATE DISPOSABLE DATABASE ====="
jexec pfdb su -m postgres -c "/usr/local/bin/dropdb --if-exists ${TEST_DB}"
jexec pfdb su -m postgres -c "/usr/local/bin/createdb -T template0 ${TEST_DB}"
jexec pfdb su -m postgres -c "/usr/local/bin/pg_dump --schema-only --no-owner pathfinder | /usr/local/bin/psql -X -v ON_ERROR_STOP=1 -d ${TEST_DB}" >/dev/null

echo
echo "===== TEMPORARY DATABASE ACCESS ====="
HBA_JAIL_PATH="$(jexec pfdb su -m postgres -c "/usr/local/bin/psql -X -A -t -d postgres -c 'SHOW hba_file;'")"
HBA_FILE="${DB_JAIL_ROOT}${HBA_JAIL_PATH}"
HBA_BACKUP="${HBA_FILE}.phase14-source-record-guard.$$"
cp -p "${HBA_FILE}" "${HBA_BACKUP}"
HBA_TMP="${HBA_FILE}.phase14-source-record-guard.$$.tmp"
{
    echo "# Pathfinder Phase 1.4 SourceRecord identity guard - temporary"
    echo "host    pathfinder_phase14_lifecycle    pathfinder_app    10.77.0.10/32    scram-sha-256"
    cat "${HBA_BACKUP}"
} > "${HBA_TMP}"
HBA_UID="$(stat -f '%u' "${HBA_FILE}")"
HBA_GID="$(stat -f '%g' "${HBA_FILE}")"
HBA_MODE="$(stat -f '%Lp' "${HBA_FILE}")"
chown "${HBA_UID}:${HBA_GID}" "${HBA_TMP}"
chmod "${HBA_MODE}" "${HBA_TMP}"
mv "${HBA_TMP}" "${HBA_FILE}"
jexec pfdb su -m postgres -c "/usr/local/bin/psql -X -A -t -d postgres -c 'SELECT pg_reload_conf();'" >/dev/null

echo
echo "===== CREATE DISPOSABLE JAIL RUNTIME ====="
rm -rf "${JAIL_ROOT}${TEST_ROOT}"
jexec pfapp install -d -o root -g wheel -m 0755 "${TEST_ROOT}"
jexec pfapp install -d -o pfartifact -g pfartifact -m 0750 "${TEST_ROOT}/artifacts" "${TEST_ROOT}/artifacts/objects" "${TEST_ROOT}/run"
jexec pfapp install -d -o pathfinder -g pathfinder -m 0750 "${TEST_ROOT}/state" "${TEST_ROOT}/log"
install -o root -g wheel -m 0555 "${TEST_BINARY}" "${JAIL_TEST_BINARY}"
install -o root -g wheel -m 0555 "${ARTIFACTD_BINARY}" "${JAIL_ARTIFACTD_BINARY}"

awk -F= '
BEGIN { OFS = "=" }
$1 == "artifact_socket" { print "artifact_socket", "/var/tmp/pathfinder-phase14-lifecycle/run/preserve.sock"; next }
$1 == "artifacts_dir" { print "artifacts_dir", "/var/tmp/pathfinder-phase14-lifecycle/artifacts"; next }
$1 == "database_name" { print "database_name", "pathfinder_phase14_lifecycle"; next }
$1 == "log_dir" { print "log_dir", "/var/tmp/pathfinder-phase14-lifecycle/log"; next }
$1 == "readiness_listen" { print "readiness_listen", "127.0.0.1:18085"; next }
$1 == "state_dir" { print "state_dir", "/var/tmp/pathfinder-phase14-lifecycle/state"; next }
{ print }
' "${PROD_CONFIG}" > "${TEST_CONFIG}"
chown root:wheel "${TEST_CONFIG}"
chmod 0644 "${TEST_CONFIG}"

echo
echo "===== START DISPOSABLE ARTIFACTD ====="
jexec pfapp /usr/sbin/daemon \
    -p "${TEST_ROOT}/artifactd.pid" \
    -o "${TEST_ROOT}/artifactd.log" \
    -u pfartifact \
    "${TEST_ROOT}/pathfinder-artifactd-phase14-source-record-guard" \
    -objects "${TEST_ROOT}/artifacts/objects" \
    -socket "${TEST_ROOT}/run/preserve.sock"

attempt=0
while [ "${attempt}" -lt 50 ]; do
    jexec pfapp test -S "${TEST_ROOT}/run/preserve.sock" && break
    attempt=$((attempt + 1))
    sleep 0.1
done
jexec pfapp test -S "${TEST_ROOT}/run/preserve.sock" || { echo "FAIL: disposable artifactd unavailable."; exit 1; }

echo
echo "===== RUN LIVE SOURCE RECORD GUARD ====="
jexec -U pathfinder pfapp env \
    PATHFINDER_PHASE14_SOURCE_RECORD_GUARD=YES \
    PATHFINDER_PHASE14_SOURCE_RECORD_GUARD_CONFIG="${TEST_ROOT}/pathfinder.conf" \
    "${TEST_ROOT}/pathfinder-phase14-source-record-guard.test" \
    -test.run '^TestPhase14SourceRecordIdentityGuard$' \
    -test.v

echo
echo "===== DISPOSABLE FINAL COUNTS ====="
COUNTS="$(jexec pfdb su -m postgres -c "/usr/local/bin/psql -X -A -t -d ${TEST_DB} -c \"SELECT
    (SELECT count(*) FROM pathfinder.processing_run) || '|' ||
    (SELECT count(*) FROM pathfinder.processing_event) || '|' ||
    (SELECT count(*) FROM pathfinder.source_record_processing) || '|' ||
    (SELECT count(*) FROM pathfinder.source_record) || '|' ||
    (SELECT count(*) FROM pathfinder.vulnerability) || '|' ||
    (SELECT count(*) FROM pathfinder.assertion) || '|' ||
    (SELECT count(*) FROM pathfinder.collector_checkpoint);\"")"
echo "${COUNTS}"
[ "${COUNTS}" = "4|4|4|2|1|2|2" ] || { echo "FAIL: unexpected disposable guard counts."; exit 1; }

echo
echo "===== PRODUCTION AFTER ====="
PROD_AFTER="$(production_counts)"
echo "${PROD_AFTER}"
[ "${PROD_AFTER}" = "${PROD_BEFORE}" ] || { echo "FAIL: production changed."; exit 1; }

echo
echo "===== PRODUCTION SOURCEARTIFACT RECONCILIATION ====="
jexec -U pathfinder pfapp /usr/local/sbin/pathfinder source-artifact reconcile -config /usr/local/etc/pathfinder/pathfinder.conf

echo
echo "===== INSTALLED PRODUCTION BINARIES ====="
PATHFINDER_SHA="$(sha256 -q /usr/local/jails/containers/pathfinder-app/usr/local/sbin/pathfinder)"
ARTIFACTD_SHA="$(sha256 -q /usr/local/jails/containers/pathfinder-app/usr/local/sbin/pathfinder-artifactd)"
echo "pathfinder=${PATHFINDER_SHA}"
echo "artifactd=${ARTIFACTD_SHA}"
[ "${PATHFINDER_SHA}" = "${EXPECTED_PATHFINDER}" ] || { echo "FAIL: Pathfinder production binary changed."; exit 1; }
[ "${ARTIFACTD_SHA}" = "${EXPECTED_ARTIFACTD}" ] || { echo "FAIL: artifactd production binary changed."; exit 1; }

echo
echo "===== REPOSITORY CHECK ====="
git -C "${REPO_ROOT}" diff --check
git -C "${REPO_ROOT}" status --short

echo
echo "PASS: exact immutable SourceRecord metadata can be reused across processing versions."
echo "PASS: conflicting external_record_id is rejected and rolled back."
echo "PASS: conflicting source_marking is rejected and rolled back."
echo "PASS: rejected conflict runs create no SRP, Assertion, or CollectorCheckpoint rows."
echo "PASS: original SourceRecord identity remains unchanged."
echo "PASS: production semantic tables unchanged."
echo "PASS: production SourceArtifact reconciliation clean."
echo "PASS: installed Phase 1.3 binaries unchanged."

echo
echo "===== PHASE 1.4 SOURCE RECORD IDENTITY GUARD GATE: PASS ====="
