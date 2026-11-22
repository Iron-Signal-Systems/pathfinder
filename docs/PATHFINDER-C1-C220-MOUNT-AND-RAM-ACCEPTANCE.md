# Pathfinder C1 — C220 native shared-RAM and two-jail acceptance

**Status:** STAGED DEVELOPMENT PROCEDURE. Nothing in this document has
been performed on C220. The populated Pathfinder SoR, UDM-Pro, FI, and
planned receiver remain out of scope.

**Branch:** `dev/c1-ingest-sor-20261122`
**Gate:** Host/jail mount acceptance must pass before starting either C1
development SQL or verifier service. This procedure does **not** create
mounts, service accounts, SQL clones, certificates, or startup services.

## Goal

Prove the same volatile host tmpfs directory is visible as an ingest-writable
nullfs inside `pathfinder-ingest` and as SoR-read-only nullfs inside
`pathfinder-sor`. Prove **two distinct unprivileged service accounts**
can exchange a real `0640` object through an operator-defined private
non-root shared group. No world-readable RAM and no writable SoR view.

The receiver is **not** involved. The future receiver owns durable signed
custody on `cold`, but there is no receiver implementation in this stage.

## Inputs to record before any command

All values are operator-verified and must **not** be guessed from FI paths
or reused from a different jail:

| Input | Source / requirement |
| --- | --- |
| `PF_C1_HOST_RAM` | Actual dedicated **mounted host tmpfs** absolute path |
| `PF_C1_INGEST_RAM` | Absolute host-side path of ingest-jail nullfs mount |
| `PF_C1_SOR_RAM` | Absolute host-side path of SoR-jail nullfs mount |
| `PF_C1_JAIL_RAM_PATH` | Absolute mounted path inside both named jails |
| `PF_C1_SHARED_GID` | Dedicated non-zero group number, shared by both service users |
| `PF_C1_INGEST_USER` | Distinct non-root user inside `pathfinder-ingest` |
| `PF_C1_SOR_USER` | Distinct non-root user inside `pathfinder-sor` |
| staged binary paths | Explicit paths to an operator-built `pathfinder-c1-ram-probe-dev` in each jail |
| source archive + sensor identity | Frozen archive, actual original `sensor_id`, registered UUID; required only for later SQL test |
| `PF_C1_PINNED_MANIFEST_SHA256` | Exact 64-character lowercase aggregate fingerprint emitted by the read-only baseline below; required for native ingest and independent verifier B |
| clone + credentials | Separately isolated PostgreSQL 18 `pathfinder_c1_clone`, local socket, SoR SQL role, B SELECT-only role; required only for later SQL test |

The host tmpfs must be **bounded** in size and host memory use. Its
directory must be traversable by the SoR reader, writable by the ingest
writer, and not accessible to unrelated users. The published test and
bundle files are exactly `0640`; if the cross-jail group cannot be
assigned, the publisher must fail, not fall back to `0644`.
ZFS is NOT the RAM handoff. A tmpfs loss is intentional and recoverable
from original source custody plus PostgreSQL commit state.

## Gate 1 — read-only host inspection

The operator may run this from the **FreeBSD host**, after setting the
exact verified variables above:

```
sh tools/pathfinder_c1_c220_preflight.sh
```

It must report `PATHFINDER_C1_C220_RAM_PREFLIGHT_PASS` only when:

- the host source is actually mounted as `tmpfs`;
- both named jails are in `jail_list` and already running;
- the two mounted `nullfs` views point to exactly that source;
- ingest is RW, SoR is RO;
- the host source directory has `PF_C1_SHARED_GID`;
- the two unprivileged service accounts are distinct, both carry the
  shared group, ingest can write and SoR can read its jail-visible RAM path.

The script performs no writes or mount actions. **STOP** if it does not
pass, if any paths differ, or if the original FI jails are accidentally
selected. Do not create/adjust mounts as a side effect of this preflight.

## Gate 2 — actual 64-byte cross-jail proof

Compile the separately tagged FreeBSD development probe from the current
branch, then stage the binary in **each** named jail by an independently
reviewed procedure:

```
cd go
go build -tags pathfinder_c1_dev \
  -o /EXPLICIT/DEV/BUILD/DIRECTORY/pathfinder-c1-ram-probe-dev \
  ./cmd/pathfinder-c1-ram-probe-dev
```

Do not run as root. After Gate 1 passes, publish from the actual ingest
service identity (replace the absolute binary path with the verified
jail-visible location):

```
jexec -U "$PF_C1_INGEST_USER" pathfinder-ingest \
  /ABSOLUTE/STAGED/DEV/BINARY/pathfinder-c1-ram-probe-dev \
  -operation publish -ram-dir "$PF_C1_JAIL_RAM_PATH" \
  -shared-gid "$PF_C1_SHARED_GID"
```

Record exactly the printed `name` and `sha256`. Then verify with the
distinct non-root SoR service identity:

```
jexec -U "$PF_C1_SOR_USER" pathfinder-sor \
  /ABSOLUTE/STAGED/DEV/BINARY/pathfinder-c1-ram-probe-dev \
  -operation read -ram-dir "$PF_C1_JAIL_RAM_PATH" \
  -shared-gid "$PF_C1_SHARED_GID" \
  -name "NAME_FROM_PUBLISH" -sha256 "SHA256_FROM_PUBLISH"
```

Only an exact `PATHFINDER_C1_RAM_PROBE_READ_PASS` establishes practical
cross-jail group readability. SoR must be unable to modify or remove the
probe because its entire mount is read-only, independently of file mode.
Return to ingest to clean up **only the verified disposable probe**:

```
jexec -U "$PF_C1_INGEST_USER" pathfinder-ingest \
  /ABSOLUTE/STAGED/DEV/BINARY/pathfinder-c1-ram-probe-dev \
  -operation remove -ram-dir "$PF_C1_JAIL_RAM_PATH" \
  -shared-gid "$PF_C1_SHARED_GID" \
  -name "NAME_FROM_PUBLISH" -sha256 "SHA256_FROM_PUBLISH"
```

Expected: `PATHFINDER_C1_RAM_PROBE_REMOVED`. The read operation never
modifies the probe. No PostgreSQL, receiver, or sensor evidence is read
or written during this gate.

**STOP** on any group, nullfs, permission, digest, size, cleanup or
process-identity failure. A CI pass is not a substitute for Gate 2.

## Gate 3 — isolated SQL and native FreeBSD (not yet performed)

Before starting `pathfinder-c1-sor-dev`, independently verify:

1. Source JSONL is mounted read-only and is independently readable to
   the SoR and verifier identities; its original content hashes match.
2. The local Unix socket targets a proven **isolated**
   `pathfinder_c1_clone` database, never the populated `pathfinder`
   database. Clone creation is a separate reviewed activity.
3. The SoR SQL role is limited to the clone; verifier B uses a
   **different SELECT-only** PostgreSQL role.
4. The mTLS certificates and private CA are provisioned for the intended
   pathfinder-ingest and pathfinder-sor identities and names only.
5. The dev ingest program is configured with the verified
   `-ram-shared-gid` value and exact source counts.
6. Native startup and service shutdown order, cancellation, backpressure,
   replay after lost SQL ACK, and clean restart have been proven. No
   existing FreeBSD `rc.conf`, `jail.conf` or jail mount plan is changed
   by this branch.

**Only after these checks:** run development-only replay through both
FreeBSD processes, then independent verifier B in parallel on the same
SQL clone. A four-record CI fixture is not full-history acceptance.

## Gate 4 — full historical source reconciliation (not yet performed)

The previously imported frozen archive includes **9 files and 685,218
records**. For acceptance of *that original archive only*, require:
- file SHA-256, registered sensor UUID, exact source total and each of
  four stream counts match the frozen evidence;
- deterministic batch IDs, all source ordinals, typed parent and
  child rows match independent B;
- an identical replay creates no duplicate relational records;
- interruption tests include SQL rollback, lost acknowledgment and
  volatile RAM loss with reprocessing from original source;
- record rate, CPU, RSS, WAL, filesystem I/O and SQL overhead are measured;
- original populated SoR is unchanged before, during and after.

The development SoR and verifier original-source readers each cache raw
batch payloads in process memory, with an independent **4 GiB maximum
cached-payload budget** (default). This is not an RSS limit: runtime maps,
temporary typed projectors, and decompression buffers consume additional
memory. The FreeBSD SoR and verifier binaries accept
`-max-original-cache-mib` (1–4096) and fail without starting if the audited
archive exceeds that payload allowance. The executables print their
cached original batch count and payload bytes at startup.

Before any SQL handoff, the native ingest program now runs
`AuditFrozen`, rejecting missing files, incorrect record counts, an
absent one of the four stream families, malformed original JSON, or
unsuccessful file SHA-256 validation. It then re-audits the original files
during `ReplayFrozen`; no incomplete capture is silently accepted as a
complete historical replay.

For baseline **read-only** source measurements, run the existing
`pathfinder-c1-frozen-check` binary (no SQL access) with the operator's
verified original sensor UUID and archive path:

```sh
go run ./cmd/pathfinder-c1-frozen-check \
  -root /VERIFIED/READONLY/FROZEN/UDM/ARCHIVE \
  -sensor-id REGISTERED_SENSOR_ID \
  -sensor-uuid REGISTERED_SENSOR_UUID \
  -expect-files 9 \
  -expect-records 685218
```

It reports per-stream counts, exact stored-file SHA-256 digests, a stable
whole-archive manifest fingerprint, elapsed time, throughput, CPU time,
Go heap allocation snapshot and cumulative Go allocation volume. These
are **not** PostgreSQL commit-rate or total peak-RSS measurements.
The line `FROZEN_SOURCE_MANIFEST_SHA256 value=...` establishes the **exact
original capture identity**. Record that output as the operator-approved
`PF_C1_PINNED_MANIFEST_SHA256`. Both native `pathfinder-c1-ingest-dev` and
`pathfinder-c1-verifier-dev` now require
`-expect-manifest-sha256 "$PF_C1_PINNED_MANIFEST_SHA256"` along with the
verified source path, sensor identity, and file/record totals. They must
abort before starting SQL work if the hash differs, even when the archive
contains the same number of files and records.

The original 685,218-record archive must be present to record real values;
none are inferred from CI test fixtures.

Record per-process peak RSS externally during the native full run,
including SQL writers and both independent source readers. Do **not**
assume the four-stream CI fixture proves acceptable C220 memory usage.

## Completion status

As of the branch stage: GitHub CI covers contract/race/vet tests,
FreeBSD cross-builds, real PostgreSQL 18 clone faults and a loopback
mTLS mini-end-to-end flow. **Gate 1–4 have not been executed on C220.**

This checklist is not a release approval or permission to enable live
sensor ingestion, receiver acknowledgments, autonomous deletion, or
canonical SoR promotion.
