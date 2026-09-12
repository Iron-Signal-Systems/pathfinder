# Phase 1.2 — Source Foundation

Status: **VALIDATED / COMPLETE**

Phase 1.2 establishes Pathfinder's first persistent relational source/provenance slice and the migration mechanism that owns it.

The implemented scope is intentionally narrow:

```text
schema_migration
Source
SourceCollection
RetrievalEvent
```

`SourceArtifact` remains Phase 1.3 because exact-byte preservation is a separate product invariant and storage contract.

## Migration Contract

Pathfinder migrations are embedded into the runtime binary and are invoked explicitly with:

```sh
pathfinder migrate
```

Normal service startup does not apply schema changes.

The migration command requires host/jail root authority, authenticates as `pathfinder_migrator`, explicitly performs `SET ROLE pathfinder_owner`, and uses a migration credential that is unreadable by the normal `pathfinder` service identity.

Migration execution additionally provides:

```text
transaction-scoped advisory locking
ordered version numbers
SHA-256 tracking
ALREADY_APPLIED detection
checksum mismatch rejection
-dry-run rollback mode
atomic ledger recording
```

An applied migration whose embedded bytes no longer match its recorded SHA-256 fails closed. Applied history is not silently rewritten or reapplied.

## Migration 0001

Migration:

```text
0001-source-foundation.sql
```

Validated SHA-256:

```text
8b8ca7487268ccc37c67bb7b7b85e01e7603963ea9f3b805779b47dc5bef1c5f
```

Validated ledger row:

```text
1|source-foundation|8b8ca7487268ccc37c67bb7b7b85e01e7603963ea9f3b805779b47dc5bef1c5f|pathfinder_migrator
```

The migration creates all Phase 1.2 tables as `pathfinder_owner` and grants `pathfinder_app` only the runtime DML needed for the current slice.

## Source

`Source` records who or what Pathfinder receives intelligence from.

Implemented fields include:

```text
source_id               UUIDv7
name
source_type
provider_identity
source description
created_at
```

Initial source types are:

```text
GOVERNMENT
INDUSTRY
ISAC
VENDOR
COMMUNITY
INTERNAL
CUSTOMER
PARTNER
RESEARCH
OTHER
```

Source reliability is deliberately not stored as one mutable Source field. The Phase 0 contract represents reliability through historical Assessment.

## SourceCollection

`SourceCollection` identifies a concrete collection surface belonging to a Source.

Implemented fields include:

```text
source_collection_id            UUIDv7
source_id
name
collection_type
external_collection_identifier
transport_type
expected_format
created_at
```

The `(source_collection_id, source_id)` pair is unique so downstream records can prove that the referenced collection actually belongs to the referenced Source.

## RetrievalEvent

`RetrievalEvent` records one acquisition attempt against a SourceCollection.

Implemented fields include:

```text
retrieval_event_id      UUIDv7
source_id
source_collection_id
started_at
completion_state
completed_at
operation_result
artifacts_received
records_reported_state
records_reported
error_class
created_at
```

Pathfinder uses explicit state instead of nullable ambiguity for the current slice.

An incomplete retrieval is represented as:

```text
completion_state        NOT_COMPLETED
completed_at            -infinity
operation_result        IN_PROGRESS
```

A completed retrieval must have a real completion timestamp at or after `started_at` and may not remain `IN_PROGRESS`.

`records_reported_state=NOT_KNOWN` requires the numeric placeholder `records_reported=0`; known zero remains distinguishable through `records_reported_state=KNOWN`.

The composite foreign key on `(source_collection_id, source_id)` prevents a RetrievalEvent from combining one Source with another Source's collection.

## Runtime Authority

`pathfinder_app` currently receives:

```text
SELECT
INSERT
UPDATE
```

on:

```text
source
source_collection
retrieval_event
```

It does not receive:

```text
DELETE on the Phase 1.2 source tables
DDL authority
SELECT on schema_migration
migration authority
```

The application therefore cannot silently destroy the first historical source records or alter migration history.

## Validated Failure Behavior

Repository-owned Phase 1.2 behavior tests prove:

```text
Source UUIDv7                                        PASS
SourceCollection UUIDv7                              PASS
RetrievalEvent UUIDv7                                PASS
explicit RetrievalEvent defaults                    PASS
valid completion transition                         PASS
runtime UPDATE                                      PASS
mismatched Source / SourceCollection                REJECTED
COMPLETED with -infinity completed_at                REJECTED
NOT_KNOWN records_reported with nonzero count        REJECTED
runtime DELETE                                      REJECTED
runtime migration-history read                      REJECTED
behavior-test residue                               0|0|0
```

The behavior verifier lives under:

```text
deploy/verify/phase-1.2/
```

The Phase 1.2 verification wrapper first runs the Phase 1.1 platform verifier and then validates migration 0001, ownership, runtime privileges, and the behavioral contract.

## Appliance Validation Record

Reference appliance validation proved:

```text
migration dry-run rollback                         PASS
persistent migration apply                         PASS
second migration run -> ALREADY_APPLIED            PASS
intentional migration-byte tamper                   REJECTED
migration ledger remained unchanged                PASS
all four tables owned by pathfinder_owner           PASS
runtime DML                                         PASS
runtime DDL                                         REJECTED
non-root migration                                  REJECTED
service livez                                       live
service readyz                                      ready
Go toolchain removed after build                    PASS
```

Validated installed binary SHA-256:

```text
ff30ac8c57324e960ac1becdf588b84d32dc854a957fcab18ffad2c74eba33a7
```

Reference PostgreSQL checkpoints include:

```text
zroot/pathfinder/pgdata@phase-1.2-pre-0001
zroot/pathfinder/pgdata@phase-1.2-0001-validated
```

The snapshots are appliance validation points, not substitutes for migration history or release backup strategy.

## Fresh-Install Requirement

Phase 1.2 completion does not waive the overall Phase 1 clean-install gate.

The final installer must reconstruct this state from the defined clean FreeBSD starting contract, including:

```text
migration roles and secrets
embedded migration-capable Pathfinder binary
migration 0001 application
migration checksum ledger
Phase 1.2 ownership and runtime privileges
repository-owned Phase 1.2 verification
removal of temporary Go build dependencies
```

Until the full Phase 1 construction path is automated and clean-host validated, `install.sh` continues to fail closed in install mode rather than claim a partial product installation.

## Exit Gate

Phase 1.2 result:

```text
PASS — COMPLETE
```

Current implementation work proceeds to **Phase 1.3 — Source Preservation**.
