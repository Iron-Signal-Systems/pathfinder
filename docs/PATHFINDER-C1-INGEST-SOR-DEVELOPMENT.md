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


## Stage 2: continuous buffering, bounded handoff and SQL ownership contract

Implemented in this branch (Go-only development):

- `scheduler.go` is the single-owner, four-family, continuous buffering scheduler.
  Only its **own** capacity, idle timer, or true end-of-input flushes a
  family. It defaults to 16 MiB source bytes, 8,192 rows, sixteen source
  batches, and 30 seconds inactivity per family. Large valid source batches
  flush intact, never split parents from their DNS/identity children.
- `bundle.go` packages multiple complete prepared batches from one stream and
  uses the same checksummed volatile 128 MiB object boundary. RAM names are
  unpredictable, and complete files are published with no-overwrite link.
- `dispatcher.go` caps outstanding RAM handoffs at sixteen. Sending a RAM
  reference is **not** a SQL commit. Only a matching SQL COMMIT ACK (stream,
  exact original batch UUIDs and order, digest, byte object name, row count)
  can trigger producer-side RAM reclamation. Missing/rejected ACK leaves
  RAM unreclaimed and blocks the session; on restart original custody and
  durable PostgreSQL progress determine what to replay.
- `sor_plan.go` reopens an independently supplied original custody batch and
  compares all source and child projections before generating the existing
  transactional SQL batch plan, including indexed-completion marker.
- `sor_worker.go` provides the SoR-only processing controller, requiring
  independent original source resolution, PostgreSQL batch-state inspection,
  exact transactional execution, and postcommit SQL readback. Only then does
  it produce the per-bundle SQL COMMIT ACK. A partial, conflicted or uncertain
  SQL outcome produces **no** ACK.
- Tests cover family independence, inactivity/capacity flush, failure retention,
  tampered child evidence, bounded RAM, forged/uncertain commit acknowledgments,
  partial SQL commits, readback rejection and an in-memory end-to-end pipeline.

Important: `SoRSQLExecutor` and `OriginalSourceReader` are currently **interfaces
with tested fakes**. No actual PostgreSQL connection, independently verified
live custody resolver, FreeBSD service, mTLS control listener, or cross-jail
runtime is implemented by this slice. The in-memory test is **not** evidence
of C220, PostgreSQL, crash-recovery, or performance acceptance.


## Stage 3: real isolated PostgreSQL, mTLS control, FreeBSD service candidates

The development branch now also includes:

- `pg_clone.go`, `pg_inspect.go` and `pg_readback.go`: PostgreSQL
  executor over the explicitly configured **Unix-domain socket**. It refuses
  any database name other than `pathfinder_c1_clone` and requires
  `I_VERIFIED_AN_ISOLATED_PATHFINDER_C1_CLONE` to execute SQL. Every SQL
  operation rechecks the target database and expected existing sensor tables.
  SQL batch status is `new`, `complete`, `partial` or `conflict` based on
  durable source metadata, custody references and completion markers.
- SQL readback uses `EXCEPT ALL` in both directions to compare source
  identity, original JSON, observation time and ordinals, plus **all** DNS
  question/resource-record and identity endpoint fields. Separate B verification
  of every other typed parent column is still required before production.
- `frozen_original.go`: development-only separate audit and original source
  reader for the **existing frozen UDM-Pro archive**. It re-audits stored-file
  hashes and reconstructs original batches independently of RAM references.
  This deliberately uses a RAM-resident cache in the test process; it is **not**
  the eventual receiver source or a durable source authority.
- `control_mtls.go`: TLS 1.3 mutual authentication with dedicated trust roots,
  explicit expected client and server certificate DNS names, bounded control
  messages and concurrency. Only checksummed RAM object references and exact
  SQL commit acknowledgments travel over the private control connection.
  A failed/uncertain SQL result returns NO acknowledgment. A nonce binds each
  HTTP request/response; durable cross-restart request sequence fencing is
  still required before live service acceptance.
- `go/cmd/pathfinder-c1-ingest-dev` and `go/cmd/pathfinder-c1-sor-dev`:
  **FreeBSD-only, explicitly tagged development service candidates**.
  Neither starts without explicit operator-supplied source, RAM mount and
  mTLS configuration. The ingest process does not import a SQL driver.
  The SoR process rejects non-clone PostgreSQL databases and requires
  explicit isolated clone authorization.
- `native_mount_freebsd.go`: the jail process requires the shared RAM
  directory to be an actual FreeBSD `nullfs` mount with the expected RW
  (ingest) or RO (SoR) mode. The host must separately prove its source is
  **tmpfs**; no mount installer or boot persistence is implied.

### Evidence from automated tests

The Go development workflow runs unit tests, the race detector, vet, and
FreeBSD cross-builds. A separate CI job installs the **existing** PostgreSQL
18 sensor migrations in an ephemeral Docker database named
`pathfinder_c1_clone`, runs transactional SQL/COPY, duplicate replay and
tampered-child readback tests, then destroys the container. The full CI
pipeline also runs four frozen streams through preparation, buffering,
TLS control, SoR and actual PostgreSQL; those are miniature captured-stream
fixtures, not a benchmark of the physical C220.

Every test runs on GitHub infrastructure and must **not** be confused with
native two-jail FreeBSD execution, a restart fault test, or successful
685,218-record ingestion on the actual C220. The authoritative populated
Pathfinder database, FI, and the UDM-Pro sensor remain unchanged.

### Cross-build-only development binaries

From `go` in a FreeBSD or Go cross-build environment:

```
GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
  go build -tags pathfinder_c1_dev ./cmd/pathfinder-c1-ingest-dev ./cmd/pathfinder-c1-sor-dev ./cmd/pathfinder-c1-verifier-dev
```

They MUST NOT be installed or started on the populated SoR host until host
mounts, dedicated service identities, real PostgreSQL clone isolation,
trust/certificate deployment, startup order, replay, rollback, and
independent verifier B have been accepted. In particular, adding a
`nullfs` mount in only one jail cannot establish a working shared RAM system.


## Stage 4: independent concurrent verifier B (development)

`pathfinder-c1-sor-dev` now issues a prepared-bundle SQL COMMIT ACK after it
has independently checked original source identity and confirmed every
PostgreSQL completion marker. **Full readback verification is NOT in this
critical path.** An SQL ACK never means independent B acceptance.

The separate `pathfinder-c1-verifier-dev` FreeBSD-only OS process reads
original frozen source evidence independently, polls durable PostgreSQL batch
state while ingest continues, and verifies every completed batch. It uses a
**separate SELECT-only SQL role** and refuses to start if that role can insert
into any of the sensor evidence tables or create schema objects. No INSERT,
COPY, UPDATE or DELETE is performed by B.

`IndependentVerifierB` tests cover:
- source still uncommitted: independently poll with a bounded deadline;
- complete: run SQL evidence readback and record the verified batch;
- partial, conflicting or tampered source: fail closed and report NO B PASS;
- all source files/batches: reconcile exact file/row count totals;
- typed parent mismatch (such as a modified TLS SNI with intact original
  JSON) is checked via full-column `EXCEPT ALL` proofs using the
  **actual PostgreSQL catalog types**;
- DNS/identity child records are compared in both directions for missing,
  duplicate, extra and modified rows.

The disposable PostgreSQL 18 CI role `pathfinder_c1_verifier` is
explicitly created with SELECT only, distinct from `pathfinder_app`.
The native verifier and the ingest/SoR development binaries are all
**cross-built but not installed, started or accepted on C220**.

The original JSON archive remains the test source; no receiver development or
UDM-Pro configuration changes are part of this stage. The independent
verifier has no authority to promote records into the populated SoR or
change the receiver's acknowledgment state.


## Stage 5: restart recovery and authenticated control replay tests

Actual PostgreSQL 18 fault injection (disposable GitHub Actions clone):

- **SQL COMMIT succeeds; acknowledgment is lost:** no acknowledgment is
  delivered to ingest, the entire volatile RAM directory is discarded, and a
  fresh ingest/SoR session reconstructs the original source from the frozen
  archive. PostgreSQL reports `complete` and retains exactly one completion
  marker, while independent full typed readback passes.
- **Transaction abort before COMMIT:** an injected SQL error aborts the
  transaction. PostgreSQL reports `new`, with no durable completion; the
  exact original batch is then committed successfully on replay.
- **Damaged/missing SQL completion:** removing a completion marker in the
  disposable test database yields `partial`. The SoR refuses an ACK and
  retains the RAM object. This is fault injection, not an ordinary state
  created by the atomic SQL writer.
- **Independent verifier privilege abuse:** even a column-level UPDATE grant
  causes `VerifyReadOnlyRole` to fail, in addition to table-wide writes and
  CREATE privileges. SELECT access is required on each evidence table.

The authenticated TLS control endpoint also maintains a **bounded,
certificate-scoped 30-minute nonce replay registry**. An exact duplicate
control message is rejected before SQL work; an independently authenticated
new-nonce retry can reconcile its original batch using PostgreSQL's durable
identities. The registry is deliberately volatile; it is NOT a durable
cross-restart request ledger. The immutable batch identities and database
transactions are the durable idempotency mechanism.

### Safe native C220 host inspection — no configuration changes

`tools/pathfinder_c1_c220_preflight.sh` is a read-only FreeBSD host
preflight that requires the operator to supply three **explicit absolute**
paths:

- `PF_C1_HOST_RAM`: actual mounted host `tmpfs` source directory
- `PF_C1_INGEST_RAM`: jail-root view of that source via `nullfs rw`
- `PF_C1_SOR_RAM`: jail-root view of that source via `nullfs ro`

It checks both named Pathfinder jails are running and included in enabled
host jail startup. It verifies that the two nullfs mounts share the same
original tmpfs source and enforce their intended permissions. It never
creates, mounts, writes, restarts, or changes service settings and **cannot**
return PASS until the actual mounts exist. A passing mount preflight still
does NOT establish correct source archive read-only mounts, SQL clone safety,
TLS secret ownership, cross-jail execution, reboot persistence, or restart
acceptance. Those require additional native testing.

**Status:** Dev fault injection passed on isolated PostgreSQL 18; no
physical power-cycle, native C220 jail test, or full historical replay has
been performed. The current populated Pathfinder SoR was not touched.

## Stage 6: explicit FreeBSD shared group and real RAM access probe

Native integration safety has been strengthened without changing any C220
service configuration. The ingest service now **requires**
`-ram-shared-gid`, backed by `NewDispatcherForJails`. All bundles are
published with mode `0640` and the explicitly assigned shared non-root group
**before** publication. Group-assignment errors fail closed; there is no
`0644` fallback. Separate user identities remain mandatory.

The host read-only script `tools/pathfinder_c1_c220_preflight.sh` now
requires explicit:

- mounted host tmpfs and ingest RW/SoR RO jail-root nullfs paths;
- common jail-visible RAM path and dedicated positive shared GID;
- distinct unprivileged ingest and SoR service accounts in that group;
- actual ability for ingest to traverse/write its RAM view, and SoR to
  traverse/read its view, without creating or deleting anything.

For real end-to-end mount permission proof, the **FreeBSD-only**
`pathfinder-c1-ram-probe-dev` executable provides three operator-driven
operations: ingest **publish** (a random 64-byte `0640` file), SoR **read**
(verifies actual bytes, digest, file type, mode and GID from its RO mount),
and ingest **remove** (only the exact matching temporary probe).
It carries no records, custody, PostgreSQL state, credentials, or
payload from the sensor.

Full instructions and failure gates are in
[`PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md`](PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md).
CI exercises the group ownership and probe refusal paths, and
cross-compiles the actual FreeBSD executable. **The probe has not been
executed on C220.** No jail mount installation or service startup is
performed by these source changes.

## Stage 7: bounded frozen original memory and complete-source preflight

For full historical C1 acceptance the archive is not trusted merely because
an individual source batch validates. The native ingest now **audits all
original frozen files before opening an SQL/RAM handoff session**, requiring
exact file/record totals and presence of all four stream families.
The subsequent replay rechecks the stored-file SHA-256 for every segment.

Independent verifier B also performs its own original-archive audit and
checks exact file SHA-256 correspondence to its independent replay before
reporting PASS. A truncated source is rejected **before any verifier SQL
evidence checks**.

The SoR and verifier B development source readers cache the reconstructed
original batch bytes in process memory; these caches are intentionally
volatile and are not custody. Each process now has a configurable **payload
cache budget**, default 4 GiB, with the FreeBSD
`-max-original-cache-mib` flag (1–4096). An archive larger than the
budget fails without silently dropping source batches. This limit bounds
retained raw payload bytes, **not** total Go heap or process RSS.
The actual frozen archive must be measured to determine whether
a streaming, bounded-memory source index is required before native
full-record acceptance.

The read-only `pathfinder-c1-frozen-check` tool now reports:
source-file SHA-256 digests, a deterministic aggregate archive fingerprint,
per-stream row totals, preparation throughput, process CPU times,
Go heap snapshot and cumulative allocations. These are not SQL/WAL
performance results and no actual 685,218-record source has been
processed on C220 yet.

A new 2,501-record fixture verifies the historical importer's
2,500-record chunk boundary, source ordinal 2,501, and unique
deterministic batch IDs. The complete native C220 run still must
measure wall time, per-process peak RSS, WAL and I/O independently.

## Stage 8: duplicate natural-source concurrency without global SQL serialization

The SoR SQL controller now uses **256 bounded in-process natural-key gates**
based on `sensor_uuid + stream + wire_batch_id`. Each gate protects the
exact `Inspect → Execute → Inspect` sequence for one SQL batch while
independent verifier B remains outside the SQL ACK critical path. This
prevents two simultaneous authenticated requests carrying the same original
source batch under different RAM object names from both observing `new`
and attempting duplicate SQL writes.

This is **not** a global SQL mutex: distinct source keys use independent
gates, preserving family concurrency and the 16-credit RAM/SQL handoff
design. The gates allocate no source-sized or unbounded per-batch map.

Additional acceptance tests include:

- Twelve concurrent requests for the same original source with twelve
  distinct RAM object references. Exactly one new commit and eleven
  exact `already_committed` acknowledgments are required.
- Eight concurrent **mTLS** requests against an **actual isolated
  PostgreSQL 18** database. Exactly one durable completion row, one
  new-commit ACK, seven matching duplicate ACKs, and complete B-style
  typed evidence readback are required.
- Two unrelated sensor stream keys must actually **enter SQL execution
  simultaneously**; the test coordinates both writers before allowing
  either to complete, so a global single-worker gate fails the test.

These in-process locks do **not** replace PostgreSQL transaction-level
advisory locking and database uniqueness checks between separate SoR
processes. The native service remains single-instanced per jail/listener
until the C220 restart and process-overlap acceptance proves otherwise.
A lost network response is handled through durable SQL state and replay,
not a volatile RAM acknowledgment or a claimed source-custody copy.

## Stage 9: cross-controller PostgreSQL transaction collision recovery

The existing `sensorsor.BatchSQL.SQLScript` already holds PostgreSQL
`pg_advisory_xact_lock` keyed on the original sensor, stream and wire
batch identity inside its source/children/completion transaction. Two
**separate** SoR process instances can each see `new` before competing
for that PostgreSQL transaction lock. After the first commits, the
second transaction may deliberately raise the immutable replay guard,
even though the original batch now has a durable completion marker.

The C1 `SoRProcessor.commitOne` now handles an uncertain/failed
`Execute` response by issuing a **fresh exact SQL provenance inspection**:

- If the requested original batch is `complete` with exact same
  immutable source metadata, report it as `already_committed`, not a
  second new SQL commit.
- If the state is `new`, `partial`, `conflict`, invalid or cannot
  be queried, return an error and **NO SQL ACK**, preserving replay
  responsibility. Do not erase any custody or RAM evidence.
- This reinspection remains local to the SoR SQL ACK path. Independent
  typed verifier B still performs its read-only evidence check in
  parallel and does not delay commit confirmation.

Disposable PostgreSQL 18 tests include two **independent SoR controller
objects**, each with its own Go synchronization gates and mTLS listener,
sharing only PostgreSQL and the disposable original source/RAM fixture.
A barrier forces both to complete `Inspect(new)` before either
`Execute` runs. The exact-source test requires one durable SQL
completion, one `committed` ACK and one `already_committed` ACK.
The conflicting-source test instead gives the same batch identity
different original bytes and requires one accepted commit, one refused
ACK, a `conflict` state for the rejected source and one completion
marker.

This is proof of **independent SQL connections and controller objects**
on a shared CI host, **not** proof of distinct FreeBSD jail processes,
host restart, or C220 deployment. The planned C220 test remains a
separate acceptance requirement.

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
