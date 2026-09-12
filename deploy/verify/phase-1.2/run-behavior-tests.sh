#!/bin/sh
# Pathfinder Phase 1.2 Source/SourceCollection/RetrievalEvent behavior tests.
# All successful DML is rolled back. Expected failures must fail for the named boundary.

set -u

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP_JAIL=${1:-pfapp}
DB_JAIL=${2:-pfdb}

run_runtime_sql()
{
    sql_file=$1

    cat "$sql_file" | \
    jexec -U pathfinder "$APP_JAIL" /bin/sh -c '
    cfg=/usr/local/etc/pathfinder/pathfinder.conf
    db_host=$(awk -F= '\''$1 == "database_host" {print $2}'\'' "$cfg")
    db_port=$(awk -F= '\''$1 == "database_port" {print $2}'\'' "$cfg")
    db_name=$(awk -F= '\''$1 == "database_name" {print $2}'\'' "$cfg")
    db_user=$(awk -F= '\''$1 == "database_user" {print $2}'\'' "$cfg")
    password_file=$(awk -F= '\''$1 == "database_password_file" {print $2}'\'' "$cfg")

    password=$(cat "$password_file")
    pgpass="/tmp/pathfinder-behavior-pgpass.$$"
    umask 077

    printf "%s:%s:%s:%s:%s\n" \
        "$db_host" "$db_port" "$db_name" "$db_user" "$password" > "$pgpass"

    PGPASSFILE="$pgpass" /usr/local/bin/psql \
        -X -A -t -v ON_ERROR_STOP=1 \
        -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" -f -

    rc=$?
    rm -f "$pgpass"
    exit "$rc"
    '
}

expect_failure()
{
    name=$1
    sql_file=$2
    expected=$3

    printf '\n===== %s =====\n' "$name"

    output=$(run_runtime_sql "$sql_file" 2>&1)
    rc=$?
    printf '%s\n' "$output"

    if [ "$rc" -eq 0 ]; then
        printf 'FAIL: %s unexpectedly succeeded.\n' "$name"
        return 1
    fi

    if printf '%s\n' "$output" | grep -q "$expected"; then
        printf 'PASS: %s was rejected by the expected boundary.\n' "$name"
        return 0
    fi

    printf 'FAIL: %s failed, but not for the expected reason.\n' "$name"
    return 1
}

baseline_rows=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT
       (SELECT count(*) FROM pathfinder.source) || '|' ||
       (SELECT count(*) FROM pathfinder.source_collection) || '|' ||
       (SELECT count(*) FROM pathfinder.retrieval_event);
     \"" 2>/dev/null || true)

echo "baseline_runtime_rows=$baseline_rows"

echo "===== VALID RUNTIME DML ====="
positive_output=$(run_runtime_sql "$ROOT/positive-runtime.sql" 2>&1)
positive_rc=$?
printf '%s\n' "$positive_output"

if [ "$positive_rc" -ne 0 ]; then
    echo "FAIL: valid runtime DML test failed."
    exit 1
fi

if printf '%s\n' "$positive_output" | grep -q '^FAIL:'; then
    echo "FAIL: positive runtime test reported a semantic failure."
    exit 1
fi

for expected in \
    "PASS: Source uses UUIDv7." \
    "PASS: SourceCollection uses UUIDv7." \
    "PASS: RetrievalEvent uses UUIDv7." \
    "PASS: RetrievalEvent defaults are explicit and consistent." \
    "PASS: RetrievalEvent completed-state transition is valid." \
    "PASS: Runtime UPDATE permission works on Source."
do
    if ! printf '%s\n' "$positive_output" | grep -Fq "$expected"; then
        printf 'FAIL: missing positive assertion: %s\n' "$expected"
        exit 1
    fi
done

echo "PASS: valid runtime DML and state transitions."

expect_failure \
    "MISMATCHED SOURCE/COLLECTION FK" \
    "$ROOT/reject-mismatched-source.sql" \
    "retrieval_event_collection_fk" || exit 1

expect_failure \
    "INVALID COMPLETION STATE" \
    "$ROOT/reject-invalid-completion.sql" \
    "retrieval_event_completion_check" || exit 1

expect_failure \
    "UNKNOWN REPORTED COUNT WITH NONZERO VALUE" \
    "$ROOT/reject-invalid-reported-count.sql" \
    "retrieval_event_reported_count_consistency_check" || exit 1

expect_failure \
    "RUNTIME DELETE" \
    "$ROOT/reject-runtime-delete.sql" \
    "permission denied for table source" || exit 1

expect_failure \
    "RUNTIME MIGRATION HISTORY READ" \
    "$ROOT/reject-runtime-migration-read.sql" \
    "permission denied for table schema_migration" || exit 1

echo
echo "===== VERIFY TESTS LEFT RUNTIME ROW COUNTS UNCHANGED ====="
rows=$(jexec "$DB_JAIL" su - postgres -c \
    "/usr/local/bin/psql -d pathfinder -Atc \"
     SELECT
       (SELECT count(*) FROM pathfinder.source) || '|' ||
       (SELECT count(*) FROM pathfinder.source_collection) || '|' ||
       (SELECT count(*) FROM pathfinder.retrieval_event);
     \"" 2>/dev/null || true)

echo "$rows"

if [ "$rows" = "$baseline_rows" ]; then
    echo "PASS: behavioral tests left runtime row counts unchanged."
else
    echo "FAIL: behavioral tests changed persistent runtime row counts."
    echo "before=$baseline_rows"
    echo "after=$rows"
    exit 1
fi

echo
echo "PATHFINDER PHASE 1.2 SOURCE BEHAVIOR TESTS: PASS"
