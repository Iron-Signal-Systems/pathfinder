#!/bin/sh
# Pathfinder Phase 1.2 verification wrapper.
# Runs the Phase 1.1 platform verifier, then validates migration 0001 and
# Source/SourceCollection/RetrievalEvent behavior.

set -u

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEFAULT_CONFIG="${SCRIPT_DIR}/../pathfinder.conf.example"
CONFIG=${1:-$DEFAULT_CONFIG}
EXPECTED_MIGRATION_SHA="8b8ca7487268ccc37c67bb7b7b85e01e7603963ea9f3b805779b47dc5bef1c5f"

if [ ! -r "$CONFIG" ]; then
    printf 'PATHFINDER PHASE 1.2 VERIFY: FAIL: deployment config not readable: %s\n' "$CONFIG" >&2
    exit 2
fi

# shellcheck disable=SC1090
. "$CONFIG"

if [ "$(id -u)" -ne 0 ]; then
    echo "PATHFINDER PHASE 1.2 VERIFY: FAIL: run as root" >&2
    exit 2
fi

BASE_VERIFY="${SCRIPT_DIR}/pathfinder-verify.sh"
BEHAVIOR_VERIFY="${SCRIPT_DIR}/phase-1.2/run-behavior-tests.sh"

[ -r "$BASE_VERIFY" ] || {
    echo "PATHFINDER PHASE 1.2 VERIFY: FAIL: Phase 1.1 verifier missing" >&2
    exit 2
}

[ -r "$BEHAVIOR_VERIFY" ] || {
    echo "PATHFINDER PHASE 1.2 VERIFY: FAIL: behavior verifier missing" >&2
    exit 2
}

echo "===== PHASE 1.1 FOUNDATION ====="
/bin/sh "$BASE_VERIFY" "$CONFIG" || exit 1

echo
echo "===== PHASE 1.2 MIGRATION RUNNER ====="
MIGRATION_OUTPUT=$(jexec "$APP_JAIL" \
    /usr/local/sbin/pathfinder migrate \
    -config /usr/local/etc/pathfinder/pathfinder.conf 2>&1)
MIGRATION_RC=$?
printf '%s\n' "$MIGRATION_OUTPUT"

if [ "$MIGRATION_RC" -ne 0 ]; then
    echo "PATHFINDER PHASE 1.2 VERIFY: FAIL: migration runner failed"
    exit 1
fi

if ! printf '%s\n' "$MIGRATION_OUTPUT" | grep -Fq \
    'Migration 0001 source-foundation: ALREADY_APPLIED'; then
    echo "PATHFINDER PHASE 1.2 VERIFY: FAIL: migration 0001 not proven applied"
    exit 1
fi

MIGRATION_ROW=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT version || '|' || name || '|' || sha256 || '|' || applied_by
     FROM pathfinder.schema_migration
     WHERE version = 1;
     \"" 2>/dev/null || true)

EXPECTED_ROW="1|source-foundation|${EXPECTED_MIGRATION_SHA}|pathfinder_migrator"

if [ "$MIGRATION_ROW" = "$EXPECTED_ROW" ]; then
    echo "PASS: migration 0001 ledger record and checksum match."
else
    echo "FAIL: migration 0001 ledger mismatch."
    echo "expected: $EXPECTED_ROW"
    echo "actual:   ${MIGRATION_ROW:-NOT_KNOWN}"
    exit 1
fi

echo
echo "===== PHASE 1.2 OWNERSHIP / RUNTIME PRIVILEGES ====="
OWNER_RESULT=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT count(*)
     FROM pg_tables
     WHERE schemaname = 'pathfinder'
       AND tablename IN ('schema_migration','source','source_collection','retrieval_event')
       AND tableowner = 'pathfinder_owner';
     \"" 2>/dev/null || true)

if [ "$OWNER_RESULT" = "4" ]; then
    echo "PASS: Phase 1.2 tables owned by pathfinder_owner."
else
    echo "FAIL: Phase 1.2 table ownership mismatch (${OWNER_RESULT:-NOT_KNOWN})."
    exit 1
fi

PRIV_RESULT=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT
       has_table_privilege('pathfinder_app','pathfinder.source','SELECT') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.source','INSERT') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.source','UPDATE') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.source','DELETE') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.schema_migration','SELECT');
     \"" 2>/dev/null || true)

if [ "$PRIV_RESULT" = "true|true|true|false|false" ]; then
    echo "PASS: runtime DML and migration-ledger privilege boundary."
else
    echo "FAIL: runtime privilege boundary mismatch (${PRIV_RESULT:-NOT_KNOWN})."
    exit 1
fi

echo
echo "===== PHASE 1.2 BEHAVIOR ====="
/bin/sh "$BEHAVIOR_VERIFY" "$APP_JAIL" "$DB_JAIL" || exit 1

echo
echo "PATHFINDER PHASE 1.2: PASS"
exit 0
