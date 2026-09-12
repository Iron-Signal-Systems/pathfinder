# Phase 1.3 — Source Preservation

Status: **VALIDATED / COMPLETE**

Phase 1.3 implements Pathfinder's first authoritative exact-byte source
preservation boundary and the `SourceArtifact` record that owns preserved
acquisition provenance.

The governing invariant remains:

> **Preserve what was received before deciding what it means.**

Phase 1.3 is intentionally about preservation, provenance, and failure
semantics. Parsing, source-record extraction, normalization, interpretation,
and external collection remain later work.

## Implemented SourceArtifact Contract

The persistent chain is:

```text
Source
  ↓
SourceCollection
  ↓
RetrievalEvent
  ↓
SourceArtifact
```

A committed `SourceArtifact` records:

```text
source_artifact_id       UUIDv7
source_id
source_collection_id
retrieval_event_id
source_location
media_type
content_encoding
handling_profile
byte_length
sha256
preservation_state
integrity_state
availability_state
storage_reference
created_at
```

The database enforces Source / SourceCollection / RetrievalEvent identity
consistency so a SourceArtifact cannot silently combine unrelated provenance.

Duplicate byte content does not collapse provenance. Distinct retrieval
contexts may produce distinct SourceArtifact records referencing the same
content-addressed object.

## Migration 0002

Migration:

```text
0002-source-artifact.sql
```

Validated SHA-256:

```text
3eeab1f592c263ca7d6fc6bb34f4b5e58ea0989759374375196a59c71dc9897c
```

Validated ledger identity:

```text
2|source-artifact-preservation|3eeab1f592c263ca7d6fc6bb34f4b5e58ea0989759374375196a59c71dc9897c|pathfinder_migrator
```

Normal service startup does not apply migrations.

## Storage Authority Split

Pathfinder uses separate staging and committed-object authority:

```text
/var/db/pathfinder/artifacts/
    staging/    pathfinder:pathfinder 0700
    objects/    pfartifact:pfartifact 0750
```

`pathfinder` may create acquisition staging data and read committed objects.

`pathfinder` may not create, replace, modify, or delete committed objects.

`pfartifact` owns committed-object mutation authority and does not receive
Pathfinder database credentials.

Committed object files use content-addressed storage:

```text
objects/sha256/<first-two-hex>/<sha256>
```

The artifact preservation daemon computes the SHA-256 digest and byte length
itself. The caller does not choose the committed path or supply the authoritative
digest.

## Preservation Service

`pathfinder-artifactd` runs as the dedicated `pfartifact` identity and refuses
root execution.

The local preservation protocol is framed:

```text
PATHFINDER/1 PROBE
PATHFINDER/1 PRESERVE
```

`PROBE` verifies that the preservation service is responsive and that its
committed-object directory remains present and writable by the preservation
authority, without creating artifact object data.

`PRESERVE` accepts raw bytes and returns only the preservation receipt:

```text
sha256
byte_length
storage_reference
reused
```

The daemon:

```text
enforces an artifact byte bound
writes through temporary .incoming-* objects
fsyncs committed data
uses create-if-absent publication
does not replace an existing object
verifies an existing object's digest and length before reuse
cleans controlled stale top-level incoming residue at startup
fails closed on unexpected incoming-state structure
```

## Preservation-Aware Readiness

Pathfinder readiness depends on both PostgreSQL and the artifact preservation
authority.

```text
/livez
    process liveness

/readyz
    PostgreSQL available
    artifact preservation PROBE succeeds
```

If artifactd is unavailable:

```text
/livez       remains live
/readyz      fails closed
validate     fails closed
preserve     fails closed
```

Readiness probing does not manufacture preserved objects.

## Application Commit Boundary

Phase 1.3 also implements the application-owned boundary:

```text
raw bytes
  ↓
artifact preservation
  ↓
preservation receipt
  ↓
SourceArtifact database commit
```

The command surface is:

```text
pathfinder source-artifact commit
pathfinder source-artifact reconcile
```

Pathfinder never reports database completion merely because byte preservation
succeeded.

If preservation succeeds and the database outcome cannot be proven, the
operation reports a preserved-but-unproven result containing the retry-safe
SourceArtifact UUID and preservation receipt.

A retry with the same SourceArtifact UUID can prove an already committed record
and returns:

```text
database_commit=ALREADY_CONFIRMED
```

without duplicating provenance.

## Reconciliation

`pathfinder source-artifact reconcile` is read-only.

It compares committed artifact objects to SourceArtifact records and can report:

```text
ORPHAN_OBJECT
MISSING_OBJECT
INVALID_STORAGE_REFERENCE
OBJECT_INTEGRITY_MISMATCH
UNEXPECTED_OBJECT_PATH
```

Reconciliation does not delete objects and does not invent Source,
SourceCollection, RetrievalEvent, or SourceArtifact provenance.

Multiple SourceArtifact records may legitimately reference the same exact-byte
content object.

## Validated Failure and Recovery Behavior

Reference-appliance validation proved:

```text
exact-byte preservation                                  PASS
content-addressed object reuse                           PASS
duplicate bytes retain distinct provenance semantics     PASS
runtime object create/modify/delete                      REJECTED
runtime SourceArtifact UPDATE/DELETE                     REJECTED
artifactd database-secret read                           REJECTED

artifactd unavailable:
  Pathfinder process remains live                        PASS
  /livez remains live                                    PASS
  /readyz fails closed                                   PASS
  validate fails closed                                  PASS
  preserve fails closed                                  PASS

artifactd restart:
  controlled stale .incoming-* residue removed           PASS
  preservation authority recovered                       PASS
  authoritative object unchanged                         PASS
  authoritative SourceArtifact unchanged                 PASS

preservation succeeds / DB commit rejected:
  completion not falsely reported                        PASS
  exact object retained                                  PASS
  SourceArtifact row absent                              PASS
  reconciliation reports ORPHAN_OBJECT                   PASS

successful application-owned commit:
  preservation occurs before DB confirmation             PASS
  database_commit=CONFIRMED                              PASS

same UUID retry:
  object reused                                          PASS
  database_commit=ALREADY_CONFIRMED                      PASS
  duplicate provenance row not created                   PASS

post-test production reconciliation:
  database_records=1
  findings=0
```

## Reference Appliance Checkpoint

The quiesced recursive pre-application-integration validation checkpoint is:

```text
zroot/pathfinder@phase-1.3-validated
zroot/pathfinder/artifacts@phase-1.3-validated
zroot/pathfinder/artifacts/objects@phase-1.3-validated
zroot/pathfinder/artifacts/staging@phase-1.3-validated
zroot/pathfinder/pgdata@phase-1.3-validated
zroot/pathfinder/state@phase-1.3-validated
```

The checkpoint was created with Pathfinder, artifactd, and PostgreSQL stopped.
It predates final SourceArtifact application-binary promotion; that later runtime
state was validated independently through promotion checks and final reboot
acceptance.

The snapshot is an appliance validation point, not a substitute for migration
history or release backup strategy.

## Final Reboot Acceptance

The final Phase 1.3 runtime survived a full host reboot with PostgreSQL,
artifactd, and Pathfinder auto-starting in the intended order.

Post-reboot acceptance proved:

```text
jails restored                                           PASS
PostgreSQL auto-start                                    PASS
artifactd auto-start                                     PASS
Pathfinder auto-start                                    PASS
preservation socket recreated                            PASS
/livez                                                   live
/readyz                                                  ready
Pathfinder validation                                    PASS
SourceArtifact reconciliation                            PASS
  database_records=1
  findings=0
known authoritative object reuse                         PASS
migration 0001                                           ALREADY_APPLIED
migration 0002                                           ALREADY_APPLIED
Go toolchain absent                                      PASS
```

Validated final FreeBSD binary SHA-256 values:

```text
pathfinder
f736026147d3035778285034be53f05413bd7d107dddbcddf5e3d193bfc8cfb6

pathfinder-artifactd
84f29b330a4fab0e8e44eb6cb021768e4ad5ea71544dc19cfee4b16e51142429
```

## Phase 1.3 Exit Gate

Phase 1.3 result:

```text
PASS — COMPLETE
```

The next implementation phase is:

```text
Phase 1.4 — First External Collector
```

Phase 1.4 may now rely on the invariant that required source bytes are preserved
before authoritative semantic processing proceeds.

## Fresh-Install Requirement

Phase 1.3 completion does not waive the overall Phase 1 clean-install gate.

The final Phase 1 installer must reproduce the preservation user/group,
datasets, permissions, artifact daemon, service ordering, migration 0002,
runtime configuration, SourceArtifact authority boundary, and repository-owned
verification from the documented clean FreeBSD starting contract.

Until the complete Phase 1 construction path is repository-owned and
clean-host validated, `install.sh` continues to fail closed in install mode
rather than report a partial product installation as complete.
