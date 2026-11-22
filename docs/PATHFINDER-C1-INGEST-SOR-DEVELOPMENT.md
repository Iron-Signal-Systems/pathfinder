# Pathfinder C1 backend — ingest/SoR development contract

**Status:** development-only; NOT installed, NOT running on C220, NOT permitted to
write to the populated Pathfinder PostgreSQL SoR.

**Branch:** `dev/c1-ingest-sor-20261122`, based on
`dev/sensor-ingest-20261121`. First implementation slice adds
`go/internal/sensorc1` only. No receiver, UDM-Pro, jail configuration,
database schema, or production service has been changed.

## Agreed target

```
UDM-Pro and other sensors                    (out of scope for this phase)
             |
             v
pathfinder-receiver                        (future; owns durable custody)
   COLD ZFS pool                           (future agreed receiver storage)
             |
             | read-only verified source and exact original identity
             v
pathfinder-ingest                          (existing jail; new C1 processor)
   continuous independent families:
   observations + DNS children / conntrack / identity / health
   bounded preparation and dependency-aware scheduling
   NO PostgreSQL access
             |
             v
host tmpfs prepared-work handoff          (volatile; NOT custody)
   ingest RW nullfs / SoR RO nullfs
   bounded checksummed immutable ready objects
             |
             v
pathfinder-sor                             (existing jail; PostgreSQL owner)
   SQL work admission and commit ACK
   original sensor_batch and four stream tables
   DNS children / identity endpoints in parent transactions
   per-batch completion marker in SAME PostgreSQL transaction
   independently scheduled read-only verifier
             |
             v
FAST pool: existing PostgreSQL 18.6      (preserve populated SoR)
             |
             v
Sightings / correlation / Assessments    (later processing; distinct authority)
```

The temporary replay source for development will be the already-frozen UDM-Pro
JSONL files. No live sender or new receiver is necessary for the next milestone.

## Custody and persistence

* A device retains a batch until receiver durable custody is acknowledged.
* The future receiver uses a dedicated **cold** ZFS dataset for exact original
  batches and receipts; no additional replication or snapshot requirement now.
* Ingest and SoR derive work from original custody but never own it.
* tmpfs is disposable. It is NOT a backup, journal, or acknowledgment boundary.
* The source of durable SQL progress is PostgreSQL commit evidence.
* A missing or uncertain SQL COMMIT ACK MUST NOT trigger RAM-object reclamation;
  reconcile SQL state against original custody before replay.
* Existing 685,218 committed frozen-archive evidence rows are left untouched.
* ZFS snapshots/cold-storage archival remain a future option, not this project.

## Stage 1: committed code contract

`go/internal/sensorc1/prepared.go`:

* accepts an explicitly scoped, bounded original JSONL batch;
* validates all records through existing `sensoringest.Validate`;
* retains exact source bytes in existing `sensorsor.Evidence` projections;
* preserves the authority-supplied batch UUID (including existing frozen-import identities), source SHA-256, exact byte and row counts,
  source ordinals, and original stream/schema identity;
* keeps all DNS/identity child rows attached to their parent evidence;
* requires 8 MiB / 25,000-record input limits and LF-only JSONL;
* cannot connect to PostgreSQL or acknowledge sensor delivery.

`go/internal/sensorc1/handoff.go`:

* publishes a complete encoded prepared object under an unpredictable name
  through temp-write + atomic rename;
* bounds each object to 128 MiB;
* refuses world-accessible, symlinked, or relative handoff roots;
* validates file type, exact bytes, SHA-256 and batch identity on read;
* does not export a general-purpose deletion operation;
* has no fsync or filesystem persistence step.

**Limitations:** The current handoff abstraction does NOT yet prove a FreeBSD
tmpfs filesystem, enforce the FI-style sixteen-credit outstanding limit, bind
a reference to an authenticated mTLS control channel, provide real cross-jail
processes, or perform any PostgreSQL transaction. It must not be deployed or
presented as an accepted C1 backend.

## Frozen-source preparation tool (no writes)

The current branch contains `go/cmd/pathfinder-c1-frozen-check/main.go`
and `go/internal/sensorc1/frozen.go`. The reader:

- audits **all** registered `.jsonl.gz` and `.jsonl.active` sensor files;
- rejects unsupported sensor paths but ignores unrelated archive metadata;
- verifies original stored-file SHA-256 again during the projection pass;
- uses the **same** 4 MiB / 2,500-row chunks as `sensor-sor-import`;
- reproduces exact historical `frozen-v1-<stored-hash>-<chunk-index>`
  batch names and `frozen-import-batch-v1` UUIDs;
- validates original source bytes, ordinals and four stream totals;
- produces a PASS and measured preparation rate only after all files pass;
- never publishes RAM objects, opens PostgreSQL, changes the receiver,
  changes the source archive, or advances a delivery checkpoint.

From `dev-pathfinder`, after fetching this development branch and using the
**actual existing archive path, registered sensor key and database sensor UUID**:

```
cd /home/jwood/pathfinder/go
go test ./internal/sensorc1 ./cmd/pathfinder-c1-frozen-check
go vet ./internal/sensorc1 ./cmd/pathfinder-c1-frozen-check

go run ./cmd/pathfinder-c1-frozen-check \
  -root /PATH/TO/EXISTING/FROZEN/UDM/ARCHIVE \
  -sensor-id 'REGISTERED_SENSOR_KEY' \
  -sensor-uuid 'EXISTING_POSTGRESQL_SENSOR_UUID' \
  -expect-files 9 -expect-records 685218
```

The expected file/record counts apply to the original November 20 frozen
archive. Change expectations when processing a new, larger capture.
Do NOT generate a new sensor UUID to satisfy this command.

**This is a read-only development correctness/throughput test.** It is not
SQL ingestion, and does not complete cross-jail, replay, or live acceptance.


## Subsequent implementation gates

1. **Frozen-source driver:** stream all nine archive files with exact source
   SHA-256/ordinal mapping, no filtered analysis window; deterministic bounded
   batch identity, and explicit verification against the original 685,218 rows.
2. **Continuous family buffering:** keep observations+DNS, conntrack, identity,
   health separate and bounded. A batch boundary does NOT flush other families.
   Require independent capacity, dependency, inactivity, and true-end flushes.
3. **Real shared-RAM isolation:** prove FreeBSD host tmpfs, C220 jail identities,
   read-only original custody paths, ingest RW / SoR RO nullfs, permissions,
   limits, and reboot ordering on a disposable staged test setup.
4. **SoR SQL owner:** use the existing `sensorsor` projectors and existing
   PostgreSQL 18 migration. Only `pathfinder-sor` gets a database connection.
   Commit original source evidence, child records, batch metadata, and a
   completion marker atomically, with duplicate and conflict checks.
5. **Ack and replay:** exact SQL commit receipts; backpressure/credits;
   crash recovery reconstructs from original custody. Partial SQL state
   reconciled, never silently deleted or blindly replayed.
6. **Independent verification:** read-only process checks exact originals
   against durable SQL counts, hash/identity, child tables and completion
   markers, running in parallel rather than serially blocking writers.
7. **Acceptance:** Go tests/race/vet, FreeBSD build and two-jail integration,
   disposable PostgreSQL clone with full 685,218-source-row replay,
   duplicate/tamper/power-loss/incomplete-batch fault tests and measured CPU,
   WAL, I/O and rec/s. No live SoR changes without all gates and approval.

The existing historical backfill importer is **not** replaced or deleted.
It remains an operator-controlled input and SQL correctness reference until
the separate C1 processing path passes acceptance.
