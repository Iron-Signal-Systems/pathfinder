#!/bin/sh
set -eu

REPO_ROOT="/root/pathfinder-src"
GO_ROOT="${REPO_ROOT}/go"
BUILD_ROOT="/root/pathfinder-build/phase-1.4-lifecycle"
JAIL_ROOT="/usr/local/jails/containers/pathfinder-app"
DB_JAIL_ROOT="/usr/local/jails/containers/pathfinder-db"
TEST_ROOT="/var/tmp/pathfinder-phase14-lifecycle"
HBA_FILE=""
HBA_BACKUP=""
TEST_DB="pathfinder_phase14_lifecycle"
TEST_BINARY="${BUILD_ROOT}/pathfinder-phase14-lifecycle.test"
ARTIFACTD_BINARY="${BUILD_ROOT}/pathfinder-artifactd-phase14-lifecycle"
JAIL_TEST_BINARY="${JAIL_ROOT}${TEST_ROOT}/pathfinder-phase14-lifecycle.test"
JAIL_ARTIFACTD_BINARY="${JAIL_ROOT}${TEST_ROOT}/pathfinder-artifactd-phase14-lifecycle"
PROD_CONFIG="${JAIL_ROOT}/usr/local/etc/pathfinder/pathfinder.conf"
TEST_CONFIG="${JAIL_ROOT}${TEST_ROOT}/pathfinder.conf"

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

    if [ -n "${HBA_FILE}" ] && [ -n "${HBA_BACKUP}" ] && \
       [ -f "${HBA_BACKUP}" ]; then
        cp -p "${HBA_BACKUP}" "${HBA_FILE}" || true
        rm -f "${HBA_BACKUP}" || true

        jexec pfdb su -m postgres -c \
            "/usr/local/bin/psql -X -A -t -d postgres -c 'SELECT pg_reload_conf();'" \
            >/dev/null 2>&1 || true
    fi

    if jexec pfapp test -f "${TEST_ROOT}/artifactd.pid" 2>/dev/null; then
        pid="$(jexec pfapp cat "${TEST_ROOT}/artifactd.pid" 2>/dev/null || true)"
        if [ -n "${pid}" ]; then
            jexec pfapp kill "${pid}" 2>/dev/null || true
        fi
    fi

    jexec pfdb su -m postgres -c \
        "/usr/local/bin/dropdb --if-exists ${TEST_DB}" \
        >/dev/null 2>&1 || true

    rm -rf "${JAIL_ROOT}${TEST_ROOT}"

    if [ "${status}" -ne 0 ]; then
        echo
        echo "===== LIFECYCLE GATE FAILED ====="
    fi

    exit "${status}"
}

trap cleanup EXIT INT TERM HUP

echo "===== PHASE 1.4 LIVE LIFECYCLE GATE ====="

echo
echo "===== SAFETY: PRODUCTION BEFORE ====="

PROD_BEFORE="$(production_counts)"
echo "${PROD_BEFORE}"

if [ "${PROD_BEFORE}" != "0|0|0|0|0|0|0" ]; then
    echo "FAIL: production Phase 1.4 semantic tables are not pristine."
    exit 1
fi

echo
echo "===== BUILD DISPOSABLE TEST BINARIES ====="

mkdir -p "${BUILD_ROOT}"

cd "${GO_ROOT}"

go test -c \
    -o "${TEST_BINARY}" \
    ./cmd/pathfinder

go build \
    -o "${ARTIFACTD_BINARY}" \
    ./cmd/pathfinder-artifactd

sha256 -q "${TEST_BINARY}"
sha256 -q "${ARTIFACTD_BINARY}"

echo
echo "===== CREATE DISPOSABLE DATABASE ====="

jexec pfdb su -m postgres -c \
    "/usr/local/bin/dropdb --if-exists ${TEST_DB}"

jexec pfdb su -m postgres -c \
    "/usr/local/bin/createdb -T template0 ${TEST_DB}"

jexec pfdb su -m postgres -c \
    "/usr/local/bin/pg_dump --schema-only --no-owner pathfinder | \
     /usr/local/bin/psql -X -v ON_ERROR_STOP=1 -d ${TEST_DB}" \
    >/dev/null

echo "PASS: disposable database schema created."

echo
echo "===== TEMPORARY DISPOSABLE DATABASE ACCESS ====="

HBA_JAIL_PATH="$(
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d postgres -c 'SHOW hba_file;'"
)"

case "${HBA_JAIL_PATH}" in
    /*)
        ;;
    *)
        echo "FAIL: PostgreSQL returned non-absolute hba_file path."
        exit 1
        ;;
esac

HBA_FILE="${DB_JAIL_ROOT}${HBA_JAIL_PATH}"
HBA_BACKUP="${HBA_FILE}.phase14-lifecycle.$$"

if [ ! -f "${HBA_FILE}" ]; then
    echo "FAIL: host-visible pg_hba.conf was not found at ${HBA_FILE}."
    exit 1
fi

cp -p "${HBA_FILE}" "${HBA_BACKUP}"

HBA_TMP="${HBA_FILE}.phase14-lifecycle.$$.tmp"

{
    echo "# Pathfinder Phase 1.4 disposable lifecycle gate - temporary"
    echo "host    pathfinder_phase14_lifecycle    pathfinder_app    10.77.0.10/32    scram-sha-256"
    cat "${HBA_BACKUP}"
} > "${HBA_TMP}"

HBA_UID="$(stat -f '%u' "${HBA_FILE}")"
HBA_GID="$(stat -f '%g' "${HBA_FILE}")"
HBA_MODE="$(stat -f '%Lp' "${HBA_FILE}")"

chown "${HBA_UID}:${HBA_GID}" "${HBA_TMP}"
chmod "${HBA_MODE}" "${HBA_TMP}"
mv "${HBA_TMP}" "${HBA_FILE}"

jexec pfdb su -m postgres -c \
    "/usr/local/bin/psql -X -A -t -d postgres -c 'SELECT pg_reload_conf();'" \
    >/dev/null

HBA_MATCHES="$(
    jexec pfdb su -m postgres -c \
        "/usr/local/bin/psql -X -A -t -d postgres -c \"
        SELECT count(*)
        FROM pg_hba_file_rules
        WHERE database = ARRAY['pathfinder_phase14_lifecycle']
          AND user_name = ARRAY['pathfinder_app']
          AND address = '10.77.0.10'
          AND netmask = '255.255.255.255'
          AND auth_method = 'scram-sha-256'
          AND error IS NULL;\""
)"

if [ "${HBA_MATCHES}" != "1" ]; then
    echo "FAIL: temporary lifecycle pg_hba rule was not proven."
    exit 1
fi

echo "PASS: temporary pg_hba rule installed and proven."

echo
echo "===== CREATE DISPOSABLE JAIL RUNTIME ====="

rm -rf "${JAIL_ROOT}${TEST_ROOT}"

jexec pfapp install -d \
    -o root \
    -g wheel \
    -m 0755 \
    "${TEST_ROOT}"

jexec pfapp install -d \
    -o pfartifact \
    -g pfartifact \
    -m 0750 \
    "${TEST_ROOT}/artifacts" \
    "${TEST_ROOT}/artifacts/objects" \
    "${TEST_ROOT}/run"

jexec pfapp install -d \
    -o pathfinder \
    -g pathfinder \
    -m 0750 \
    "${TEST_ROOT}/state" \
    "${TEST_ROOT}/log"

install -o root -g wheel -m 0555 \
    "${TEST_BINARY}" \
    "${JAIL_TEST_BINARY}"

install -o root -g wheel -m 0555 \
    "${ARTIFACTD_BINARY}" \
    "${JAIL_ARTIFACTD_BINARY}"

awk -F= '
BEGIN {
    OFS = "="
}
$1 == "artifact_socket" {
    print "artifact_socket", "/var/tmp/pathfinder-phase14-lifecycle/run/preserve.sock"
    next
}
$1 == "artifacts_dir" {
    print "artifacts_dir", "/var/tmp/pathfinder-phase14-lifecycle/artifacts"
    next
}
$1 == "database_name" {
    print "database_name", "pathfinder_phase14_lifecycle"
    next
}
$1 == "log_dir" {
    print "log_dir", "/var/tmp/pathfinder-phase14-lifecycle/log"
    next
}
$1 == "readiness_listen" {
    print "readiness_listen", "127.0.0.1:18081"
    next
}
$1 == "state_dir" {
    print "state_dir", "/var/tmp/pathfinder-phase14-lifecycle/state"
    next
}
{
    print
}
' "${PROD_CONFIG}" > "${TEST_CONFIG}"

chown root:wheel "${TEST_CONFIG}"
chmod 0644 "${TEST_CONFIG}"

echo
echo "===== PROVE TEST CONFIG IS DISPOSABLE ====="

grep -E \
    '^(artifact_socket|artifacts_dir|database_name|log_dir|readiness_listen|state_dir)=' \
    "${TEST_CONFIG}"

if grep -q '^database_name=pathfinder$' "${TEST_CONFIG}"; then
    echo "FAIL: test config still points at production database."
    exit 1
fi

if grep -q '^artifact_socket=/var/run/pathfinder-artifact/preserve.sock$' \
    "${TEST_CONFIG}"; then
    echo "FAIL: test config still points at production artifact socket."
    exit 1
fi

echo
echo "===== START DISPOSABLE ARTIFACTD ====="

jexec pfapp /usr/sbin/daemon \
    -p "${TEST_ROOT}/artifactd.pid" \
    -o "${TEST_ROOT}/artifactd.log" \
    -u pfartifact \
    "${TEST_ROOT}/pathfinder-artifactd-phase14-lifecycle" \
    -objects "${TEST_ROOT}/artifacts/objects" \
    -socket "${TEST_ROOT}/run/preserve.sock"

ready=0
attempt=0
while [ "${attempt}" -lt 50 ]; do
    if jexec pfapp test -S "${TEST_ROOT}/run/preserve.sock"; then
        ready=1
        break
    fi

    attempt=$((attempt + 1))
    sleep 0.1
done

if [ "${ready}" -ne 1 ]; then
    echo "FAIL: disposable artifactd socket did not appear."
    jexec pfapp cat "${TEST_ROOT}/artifactd.log" || true
    exit 1
fi

echo "PASS: disposable artifactd ready."

echo
echo "===== RUN REAL LIFECYCLE TEST ====="

if ! jexec -U pathfinder pfapp env \
    PATHFINDER_PHASE14_LIFECYCLE=YES \
    PATHFINDER_PHASE14_LIFECYCLE_CONFIG="${TEST_ROOT}/pathfinder.conf" \
    "${TEST_ROOT}/pathfinder-phase14-lifecycle.test" \
    -test.run '^TestPhase14LiveLifecycle$' \
    -test.v; then

    echo
    echo "===== DISPOSABLE ARTIFACTD LOG ====="
    jexec pfapp cat "${TEST_ROOT}/artifactd.log" || true
    exit 1
fi

echo
echo "===== DISPOSABLE DATABASE PROOF ====="

jexec pfdb su -m postgres -c \
    "/usr/local/bin/psql -X -A -t -d ${TEST_DB} \
    -c \"SELECT
        (SELECT count(*) FROM pathfinder.processing_run) || '|' ||
        (SELECT count(*) FROM pathfinder.processing_event) || '|' ||
        (SELECT count(*) FROM pathfinder.source_record_processing) || '|' ||
        (SELECT count(*) FROM pathfinder.source_record) || '|' ||
        (SELECT count(*) FROM pathfinder.vulnerability) || '|' ||
        (SELECT count(*) FROM pathfinder.assertion) || '|' ||
        (SELECT count(*) FROM pathfinder.collector_checkpoint);\""

echo
echo "===== PRODUCTION MUST BE UNCHANGED ====="

PROD_AFTER="$(production_counts)"
echo "${PROD_AFTER}"

if [ "${PROD_AFTER}" != "${PROD_BEFORE}" ]; then
    echo "FAIL: production Phase 1.4 semantic tables changed."
    exit 1
fi

echo
echo "===== PRODUCTION SOURCEARTIFACT RECONCILIATION ====="

jexec -U pathfinder pfapp \
    /usr/local/sbin/pathfinder \
    source-artifact reconcile \
    -config /usr/local/etc/pathfinder/pathfinder.conf

echo
echo "===== REPOSITORY CHECK ====="

cd "${REPO_ROOT}"

git diff --check
git status --short

echo
echo "PASS: production semantic tables unchanged."
echo "PASS: production SourceArtifact reconciliation clean."
echo "PASS: disposable database and temporary pg_hba rule will be removed by cleanup."

echo
echo "===== PHASE 1.4 LIVE LIFECYCLE GATE: PASS ====="
