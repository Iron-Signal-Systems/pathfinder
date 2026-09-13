package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestMigrationDryRunSQLUsesOneTransactionForPendingSequence(t *testing.T) {
	t.Parallel()

	items := []migration{
		{
			Name:    "first",
			SHA256:  strings.Repeat("a", 64),
			SQL:     "CREATE TABLE pathfinder.test_first (id bigint PRIMARY KEY);",
			Version: 3,
		},
		{
			Name:   "second",
			SHA256: strings.Repeat("b", 64),
			SQL: `CREATE TABLE pathfinder.test_second (
    id bigint PRIMARY KEY,
    first_id bigint NOT NULL REFERENCES pathfinder.test_first(id)
);`,
			Version: 4,
		},
	}

	sql := migrationDryRunSQL(items)

	if got := strings.Count(sql, "BEGIN;"); got != 1 {
		t.Fatalf("BEGIN count=%d want=1", got)
	}

	if got := strings.Count(sql, "ROLLBACK;"); got != 1 {
		t.Fatalf("ROLLBACK count=%d want=1", got)
	}

	if strings.Contains(sql, "COMMIT;") {
		t.Fatal("dry-run SQL unexpectedly contains COMMIT")
	}

	firstIndex := strings.Index(sql, items[0].SQL)
	secondIndex := strings.Index(sql, items[1].SQL)

	if firstIndex == -1 {
		t.Fatal("first pending migration SQL missing")
	}

	if secondIndex == -1 {
		t.Fatal("second pending migration SQL missing")
	}

	if firstIndex >= secondIndex {
		t.Fatal("pending migrations are not emitted in order")
	}

	for _, item := range items {
		if !strings.Contains(
			sql,
			fmt.Sprintf("Pathfinder migration %04d %s", item.Version, item.Name),
		) {
			t.Fatalf("migration marker missing for version %d", item.Version)
		}
	}

	if got := strings.Count(
		sql,
		"INSERT INTO pathfinder.schema_migration",
	); got != len(items) {
		t.Fatalf(
			"ledger insert count=%d want=%d",
			got,
			len(items),
		)
	}
}
