#!/bin/sh
# Pathfinder Phase 1.3 verification wrapper.
# Runs Phase 1.2 verification, then validates SourceArtifact persistence,
# artifact authority separation, preservation-aware readiness, and
# reconciliation without manufacturing persistent artifact data.

set -u

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEFAULT_CONFIG="${SCRIPT_DIR}/../pathfinder.conf.example"
CONFIG=${1:-$DEFAULT_CONFIG}

EXPECTED_MIGRATION_0002_SHA="3eeab1f592c263ca7d6fc6bb34f4b5e58ea0989759374375196a59c71dc9897c"

fatal()
{
    printf 'PATHFINDER PHASE 1.3 VERIFY: FAIL: %s\n' "$1" >&2
    exit 1
}

pass()
{
    printf 'PASS: %s\n' "$1"
}

[ -r "$CONFIG" ] ||
    fatal "deployment config not readable: $CONFIG"

# shellcheck disable=SC1090
. "$CONFIG"

[ "$(id -u)" -eq 0 ] ||
    fatal "run as root"

PHASE12_VERIFY="${SCRIPT_DIR}/pathfinder-phase-1.2-verify.sh"

[ -r "$PHASE12_VERIFY" ] ||
    fatal "Phase 1.2 verifier missing"

echo "===== PHASE 1.2 FOUNDATION ====="
/bin/sh "$PHASE12_VERIFY" "$CONFIG" || exit 1

echo
echo "===== PHASE 1.3 MIGRATION ====="

MIGRATION_OUTPUT=$(jexec "$APP_JAIL" \
    /usr/local/sbin/pathfinder migrate \
    -config /usr/local/etc/pathfinder/pathfinder.conf 2>&1)
MIGRATION_RC=$?

printf '%s\n' "$MIGRATION_OUTPUT"

[ "$MIGRATION_RC" -eq 0 ] ||
    fatal "migration runner failed"

printf '%s\n' "$MIGRATION_OUTPUT" |
    grep -Fq 'Migration 0002 source-artifact-preservation: ALREADY_APPLIED' ||
    fatal "migration 0002 not proven applied"

MIGRATION_ROW=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT version || '|' || name || '|' || sha256 || '|' || applied_by
     FROM pathfinder.schema_migration
     WHERE version = 2;
     \"" 2>/dev/null || true)

EXPECTED_ROW="2|source-artifact-preservation|${EXPECTED_MIGRATION_0002_SHA}|pathfinder_migrator"

[ "$MIGRATION_ROW" = "$EXPECTED_ROW" ] ||
    fatal "migration 0002 ledger mismatch (actual: ${MIGRATION_ROW:-NOT_KNOWN})"

pass "migration 0002 ledger record and checksum match"

echo
echo "===== SOURCEARTIFACT OWNERSHIP / PRIVILEGES ====="

OWNER_RESULT=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT tableowner
     FROM pg_tables
     WHERE schemaname = 'pathfinder'
       AND tablename = 'source_artifact';
     \"" 2>/dev/null || true)

[ "$OWNER_RESULT" = "pathfinder_owner" ] ||
    fatal "source_artifact ownership mismatch (${OWNER_RESULT:-NOT_KNOWN})"

pass "source_artifact owned by pathfinder_owner"

PRIV_RESULT=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT
       has_table_privilege('pathfinder_app','pathfinder.source_artifact','SELECT') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.source_artifact','INSERT') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.source_artifact','UPDATE') || '|' ||
       has_table_privilege('pathfinder_app','pathfinder.source_artifact','DELETE');
     \"" 2>/dev/null || true)

[ "$PRIV_RESULT" = "true|true|false|false" ] ||
    fatal "SourceArtifact runtime privilege boundary mismatch (${PRIV_RESULT:-NOT_KNOWN})"

pass "runtime SourceArtifact authority is SELECT/INSERT only"

echo
echo "===== ARTIFACT STORAGE AUTHORITY ====="

STAGING_META=$(jexec "$APP_JAIL" stat -f '%Su|%Sg|%Lp' \
    /var/db/pathfinder/artifacts/staging 2>/dev/null || true)

OBJECTS_META=$(jexec "$APP_JAIL" stat -f '%Su|%Sg|%Lp' \
    /var/db/pathfinder/artifacts/objects 2>/dev/null || true)

[ "$STAGING_META" = "pathfinder|pathfinder|700" ] ||
    fatal "staging metadata mismatch (${STAGING_META:-NOT_KNOWN})"

[ "$OBJECTS_META" = "pfartifact|pfartifact|750" ] ||
    fatal "objects metadata mismatch (${OBJECTS_META:-NOT_KNOWN})"

pass "staging and committed-object metadata match Phase 1.3 authority split"

if jexec -U pathfinder "$APP_JAIL" \
    test -w /var/db/pathfinder/artifacts/staging; then
    pass "pathfinder can write staging"
else
    fatal "pathfinder cannot write staging"
fi

if jexec -U pathfinder "$APP_JAIL" \
    test -w /var/db/pathfinder/artifacts/objects; then
    fatal "pathfinder can write committed object directory"
else
    pass "pathfinder cannot write committed object directory"
fi

echo
echo "===== ARTIFACT PRESERVATION SERVICE ====="

ARTIFACTD_ENABLE=$(jexec "$APP_JAIL" \
    sysrc -n pathfinder_artifactd_enable 2>/dev/null || true)

[ "$ARTIFACTD_ENABLE" = "YES" ] ||
    fatal "pathfinder_artifactd boot enable mismatch (${ARTIFACTD_ENABLE:-NOT_KNOWN})"

jexec "$APP_JAIL" service pathfinder_artifactd status >/dev/null 2>&1 ||
    fatal "pathfinder_artifactd is not running"

pass "pathfinder_artifactd enabled and running"

ARTIFACT_PID=$(jexec "$APP_JAIL" \
    cat /var/run/pathfinder-artifact/pathfinder-artifactd.pid 2>/dev/null || true)

ARTIFACT_USER=$(jexec "$APP_JAIL" \
    ps -p "$ARTIFACT_PID" -o user= 2>/dev/null | awk 'NF {print $1; exit}')

[ "$ARTIFACT_USER" = "pfartifact" ] ||
    fatal "artifactd process identity mismatch (${ARTIFACT_USER:-NOT_KNOWN})"

pass "artifactd runs as pfartifact"

SOCKET_META=$(jexec "$APP_JAIL" \
    stat -f '%Su|%Sg|%Lp' \
    /var/run/pathfinder-artifact/preserve.sock 2>/dev/null || true)

[ "$SOCKET_META" = "pfartifact|pfartifact|660" ] ||
    fatal "preservation socket metadata mismatch (${SOCKET_META:-NOT_KNOWN})"

pass "preservation socket authority matches contract"

if jexec -U pfartifact "$APP_JAIL" \
    test -r /usr/local/etc/pathfinder/secrets/postgresql-password; then
    fatal "pfartifact can read runtime DB credential"
else
    pass "pfartifact cannot read runtime DB credential"
fi

if jexec -U pfartifact "$APP_JAIL" \
    test -r /usr/local/etc/pathfinder/secrets/postgresql-migration-password; then
    fatal "pfartifact can read migration DB credential"
else
    pass "pfartifact cannot read migration DB credential"
fi

echo
echo "===== PRESERVATION-AWARE VALIDATION WITHOUT OBJECT CREATION ====="

OBJECT_COUNT_BEFORE=$(jexec "$APP_JAIL" sh -c \
    "find /var/db/pathfinder/artifacts/objects/sha256 -type f 2>/dev/null | wc -l | awk '{print \$1}'")

VALIDATE_OUTPUT=$(jexec -U pathfinder "$APP_JAIL" \
    /usr/local/sbin/pathfinder validate \
    -config /usr/local/etc/pathfinder/pathfinder.conf 2>&1)
VALIDATE_RC=$?

printf '%s\n' "$VALIDATE_OUTPUT"

[ "$VALIDATE_RC" -eq 0 ] ||
    fatal "Pathfinder validation failed"

printf '%s\n' "$VALIDATE_OUTPUT" |
    grep -Fq 'Pathfinder validation: PASS' ||
    fatal "Pathfinder validation did not report PASS"

OBJECT_COUNT_AFTER=$(jexec "$APP_JAIL" sh -c \
    "find /var/db/pathfinder/artifacts/objects/sha256 -type f 2>/dev/null | wc -l | awk '{print \$1}'")

[ "$OBJECT_COUNT_AFTER" = "$OBJECT_COUNT_BEFORE" ] ||
    fatal "validation probe created artifact data"

pass "preservation PROBE leaves committed object count unchanged"

LIVE=$(jexec "$APP_JAIL" fetch -T 2 -qo - \
    http://127.0.0.1:8080/livez 2>/dev/null || true)

READY=$(jexec "$APP_JAIL" fetch -T 2 -qo - \
    http://127.0.0.1:8080/readyz 2>/dev/null || true)

[ "$LIVE" = "live" ] ||
    fatal "/livez is not live"

[ "$READY" = "ready" ] ||
    fatal "/readyz is not ready"

pass "Pathfinder livez/readyz healthy with preservation authority"

echo
echo "===== SOURCEARTIFACT RECONCILIATION ====="

RECON_OUTPUT=$(jexec -U pathfinder "$APP_JAIL" \
    /usr/local/sbin/pathfinder source-artifact reconcile \
    -config /usr/local/etc/pathfinder/pathfinder.conf 2>&1)
RECON_RC=$?

printf '%s\n' "$RECON_OUTPUT"

[ "$RECON_RC" -eq 0 ] ||
    fatal "SourceArtifact reconciliation returned findings or failed"

printf '%s\n' "$RECON_OUTPUT" |
    grep -Fq 'Pathfinder SourceArtifact reconciliation: PASS' ||
    fatal "SourceArtifact reconciliation did not report PASS"

printf '%s\n' "$RECON_OUTPUT" |
    grep -Fq 'findings=0' ||
    fatal "SourceArtifact reconciliation did not prove zero findings"

pass "SourceArtifact object/database state reconciles cleanly"

echo
echo "PATHFINDER PHASE 1.3: PASS"
exit 0
