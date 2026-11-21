# Pathfinder frozen UDM-Pro archive → PostgreSQL SoR

**Development status:** The importer must pass FreeBSD tests and an isolated PostgreSQL 18 database replay before the SoR is considered ingest-ready. No live C220 database or imported SNI results are claimed here.

## Scope

The frozen read-only archive is mounted in `dev-pathfinder` at
`/mnt/udm-pro-20261120-0500EST`. All **nine** files must be imported, without
time filtering:

| Stream | Expected records |
|---|---:|
| observations | 569,671 |
| conntrack | 111,842 |
| identity | 2,446 |
| health | 1,259 |
| **Total** | **685,218** |

The snapshot root may also contain the packaging file `.pathfinder-transfer.tar`. The importer explicitly skips **only that root-level transfer artifact**; it is not a sensor JSONL file and is excluded from the nine-file and 685,218-record counts. Unexpected files, including other `.tar` files, still fail closed. Do not delete or alter the transfer artifact to make validation work.

The importer has three modes: `dry-run`, `import` and `sni`. Source files remain read-only. In `import` mode the files and validated JSONL batches are copied into separate private, fsync-backed immutable custody before any batch is indexed. The importer does **not** replace the authenticated live sensor receiver; it is a special operator-controlled historical backfill.

## Prerequisites and authority

- PostgreSQL 18 database with Pathfinder roles, schema and **migrations 0001–0009** installed. The current C220 `pathfinder-sor` jail was observed without PostgreSQL installed, so **it cannot accept this import yet**.
- The development jail has Go 1.26 and Python 3.12. The `psql` client must be accessible where the importer runs, and DB auth supplied via protected `.pgpass` / `PGPASSFILE` or an equivalent approved mechanism. Do not put passwords in CLI arguments or Git.
- `pathfinder_app` can INSERT and SELECT sensor evidence but **cannot register sensors or run DDL**. Sensor registration is explicitly performed by the migrator with `SET ROLE pathfinder_owner`.
- The operator knows the sensor key. It must **match exactly** the `sensor_id` from the packet observations. The other three streams have no embedded sensor ID; provenance is attributed through this trusted, operator-scoped frozen archive.
- A private absolute custody directory writable by the importer account; not on the nine read-only files. The importer creates `segments/` and `batches/` with permissions 0700, files 0600.
- Do not run `install.sh` expecting an all-in-one deployment: the current repository installer intentionally refuses incomplete construction. Do not confuse this frozen-capture import with full product installation acceptance.

## C220 read-only acceptance preflight (first required operator action)

The repository includes `tools/verify_sensor_sor_frozen.sh` for the frozen archive. It reads the first observation sensor ID as a consistency key for **dry-run only**, then audits and projects all nine source files with the exact expected row count. This does not register a trusted sensor, contact PostgreSQL, write custody, or modify the archived source. Run in `dev-pathfinder` as jail root because some frozen source files are root:wheel 0640.

```sh
sh /home/jwood/pathfinder/tools/verify_sensor_sor_frozen.sh
```

## Read-only dry-run first

From `dev-pathfinder`'s checked-out `pathfinder/go` directory:

```sh
go test ./...
go vet ./...
go run ./cmd/sensor-sor-import \
  -mode dry-run \
  -root /mnt/udm-pro-20261120-0500EST \
  -sensor-key 'ACTUAL_UDM_SENSOR_ID' \
  -expect-files 9 -expect-rows 685218
```

This validates and projects all records without writing PostgreSQL or custody; the explicit sensor key must match the archive's observations. Any missing file, malformed source, changed file, or projection error stops with a nonzero exit code. Do not accept partial output as PASS.

## Migration and sensor registration

Migrate with the existing Pathfinder role-aware `pathfinder migrate` tooling and its private migrator credential. The importer does not create roles, initialize PostgreSQL, or mutate schema. Once migrations 0001–0009 have been applied, register this source identity *once*, using the existing migration login:

```sh
psql -X -v ON_ERROR_STOP=1 -U pathfinder_migrator \
  -d pathfinder \
  -v sensor_key='ACTUAL_UDM_SENSOR_ID' \
  -v sensor_name='UDM-Pro historical capture' \
  -f queries/sensor_register.sql
```

The example is intentionally not hard-coded to a DB host or secret. Use the approved Pathfinder DB endpoint and authentication mechanism for the actual installation.

## Import complete archive and emit DB SNI

After PostgreSQL is running and the sensor is registered, from the same
`pathfinder/go` directory:

```sh
go run ./cmd/sensor-sor-import \
  -mode import \
  -root /mnt/udm-pro-20261120-0500EST \
  -sensor-key 'ACTUAL_UDM_SENSOR_ID' \
  -custody /PATH/TO/PRIVATE/SENSOR-CUSTODY \
  -db-host DB_HOST -db-port 5432 -db-name pathfinder \
  -db-user pathfinder_app \
  -expect-files 9 -expect-rows 685218 \
  -sni-query queries/sensor_unique_sni.sql
```

The importer:

1. Audits all nine original files (SHA-256, schema, exact unfiltered record counts).
2. Preserves the original stored segment bytes in private custody and verifies the digest again.
3. Creates bounded verified JSONL batches (≤4 MiB and ≤2,500 records, below the existing custody limits).
4. Uses deterministic batch identities, custody receipts, replay checks and an advisory lock per batch.
5. Executes one PostgreSQL transaction per batch, with COPY of original evidence and DNS/identity children, verified base/child row counts and a completion marker in the **same commit**.
6. Reconciles **actual relational evidence rows by captured-file SHA**, per stream, against all 685,218 source records.
7. Runs the **committed SQL file against PostgreSQL** and prints every distinct lowercase TLS SNI hostname plus the unique count. That result is not computed from source JSON or from JSONB scanning.

Re-run the same command to verify the backfill is repeatable: completed matching batches are checked and skipped; changed hashes or conflicting identities fail closed. The final SNI query is also independently available:

```sh
go run ./cmd/sensor-sor-import -mode sni \
  -db-host DB_HOST -db-name pathfinder -db-user pathfinder_app \
  -sni-query queries/sensor_unique_sni.sql
```

**No names or record counts should be reported as imported until this query has actually succeeded on the database.**

## Failure and limitations

- **Crash / DB error:** PostgreSQL rolls back the current batch; the already durable custody receipt survives for retry. Earlier completed batches stay intact.
- **Conflicting replay:** one sensor+stream+batch identity with different payload metadata is rejected; no silent overwrite.
- **Interrupted or concurrent attempt:** a second writer may receive a recoverable race error; rerun and it verifies the committed batch.
- **Original evidence:** preserve both exact source bytes and source JSON representation; JSONB is not byte-exact custody.
- **Coverage:** health reports are source observations; absence of a network record is not proof of blocked or nonexistent traffic.
- **Conntrack:** a Linux conntrack ID or five-tuple alone is not a unique connection lifetime. This importer does not implement derived network-session correlation or threat-intelligence Sightings.
- **Partition maintenance:** daily partitions outside the creation horizon need a forward maintenance process; the migration includes default partitions as a fallback.
- **Testing limit:** compiling and unit-testing the importer is not PostgreSQL execution acceptance. An isolated database integration run, outage/retry tests and FreeBSD in-jail acceptance remain required.
