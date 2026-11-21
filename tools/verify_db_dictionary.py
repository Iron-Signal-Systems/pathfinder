#!/usr/bin/env python3
"""Verify documented DB columns against embedded SQL migrations.

This checks documentation consistency, not PostgreSQL SQL syntax.
"""
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
SQL = ROOT / "go" / "cmd" / "pathfinder" / "migrations"
DOC = ROOT / "docs" / "DB-DATA-DICTIONARY.md"

TABLE = re.compile(
    r"CREATE TABLE pathfinder\.([a-z_]+) \((.*?)\n\)"
    r"(?: PARTITION BY RANGE \(observed_at\))?;",
    re.S,
)
COLUMN = re.compile(
    r"^    ([a-z_][a-z_0-9]*) "
    r"(character\(64\)|timestamptz|timestamp with time zone|bigint|integer|"
    r"boolean|macaddr|jsonb|uuid|inet|text)(?=\s|,|$)"
)
HEADING = re.compile(r"^### `pathfinder\.([a-z_]+)` — migration (\d{4})$", re.M)
DOCUMENTED_COLUMN = re.compile(r"^\| `([a-z_][a-z_0-9]*)` \| `([^`]+)` \| (Required|Nullable) \|", re.M)

def main():
    expected = {}
    for migration in sorted(SQL.glob("[0-9][0-9][0-9][0-9]-*.sql")):
        for table in TABLE.finditer(migration.read_text()):
            name = table.group(1)
            columns = {}
            for line in table.group(2).splitlines():
                match = COLUMN.match(line)
                if match:
                    if match.group(1) in columns:
                        raise ValueError(f"duplicate field {name}.{match.group(1)}")
                    columns[match.group(1)] = (
                        match.group(2),
                        "Required" if "NOT NULL" in line or "PRIMARY KEY" in line else "Nullable",
                    )
            if name in expected:
                raise ValueError(f"duplicate CREATE TABLE {name}")
            expected[name] = (migration.name[:4], columns)

    doc = DOC.read_text()
    heads = list(HEADING.finditer(doc))
    actual = {}
    for i, head in enumerate(heads):
        name, migration = head.group(1), head.group(2)
        until = heads[i + 1].start() if i + 1 < len(heads) else len(doc)
        block = doc[head.end():until]
        columns = {}
        for entry in DOCUMENTED_COLUMN.finditer(block):
            field, pgtype, nullable = entry.groups()
            if pgtype == "char(64)":
                pgtype = "character(64)"
            columns[field] = (pgtype, nullable)
        if name in actual:
            raise ValueError(f"duplicate documented table {name}")
        actual[name] = (migration, columns)

    failures = []
    for name in sorted(set(expected) | set(actual)):
        want, got = expected.get(name), actual.get(name)
        if want != got:
            failures.append(f"{name}: SQL={want!r} document={got!r}")
    if failures:
        print("DB DATA DICTIONARY FAIL:", len(failures), "table differences")
        for failure in failures:
            print(failure)
        return 1
    total_columns = sum(len(columns) for _, columns in expected.values())
    print(f"DB DATA DICTIONARY PASS: {len(expected)} tables, {total_columns} columns verified")
    return 0

if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError) as exc:
        print(f"DB DATA DICTIONARY FAIL: {exc}", file=sys.stderr)
        sys.exit(1)
