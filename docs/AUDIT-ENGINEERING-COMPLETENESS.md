# Pathfinder Phase 0.18 — Audit and Engineering Completeness Requirements

## Purpose

Pathfinder must never report more certainty, completeness, or success than its recorded state can support.

> **If Pathfinder cannot prove that an operation completed, it must not report the operation as complete.**

> **If Pathfinder cannot establish coverage, absence remains unknown rather than becoming a negative conclusion.**

The system must allow an operator, analyst, auditor, or future developer to reconstruct:

```text
what was attempted
what was acquired
what was preserved
what was validated
what was interpreted
what was committed
what failed
what was skipped
what remains uncertain
what the system knew at the time
```

The governing historical invariant is:

> **The original record is maintained no matter what.**

## Typed State Discipline

Pathfinder does not use one universal `status` enum.

Every persisted state belongs to an explicit semantic dimension.

Initial state dimensions include:

```text
OperationResult
PreservationState
IntegrityState
AvailabilityState
ProcessingState
HealthState
CoverageState
IndexState
LifecycleState
ReviewState
ConflictStatus
```

Similar words may appear in different dimensions only where their meanings remain explicit.

## OperationResult

General operation-completion semantics are:

```text
SUCCESS
PARTIAL
FAILED
PENDING
NOT_PERFORMED
NOT_APPLICABLE
```

`DEGRADED` is not a generic successful mutation result. It belongs primarily to service/subsystem capability state.

`PARTIAL` is a valid operation result where the operation itself can legitimately complete with a subset of its intended scope, such as a paginated retrieval or larger bulk job.

A committed `ChangeSet`, however, is atomic and cannot itself be partially committed.

```text
bulk operation PARTIAL != partial ChangeSet commit
```

## PreservationState

SourceArtifact preservation uses:

```text
RECEIVING
PRESERVED
PARTIAL
FAILED
```

Preservation state answers whether the exact artifact acquisition/preservation contract completed.

## IntegrityState

Integrity verification uses:

```text
NOT_VERIFIED
VERIFIED
MISMATCH
```

Integrity state answers whether the preserved content passed the defined verification basis.

## AvailabilityState

SourceArtifact availability uses:

```text
AVAILABLE
QUARANTINED
UNAVAILABLE
DESTROYED_BY_POLICY
```

Availability is distinct from both preservation and integrity.

## ProcessingState

Processing stages may use:

```text
PENDING
SUCCESS
PARTIAL
FAILED
UNSUPPORTED
NOT_PERFORMED
```

Detailed failure codes remain separate from the high-level processing state.

Examples include:

```text
PARSE_FAILED
NORMALIZATION_FAILED
SOURCE_SCHEMA_MISMATCH
ENRICHMENT_UNAVAILABLE
CORRELATION_PENDING
```

## HealthState

Service/subsystem health may use:

```text
HEALTHY
DEGRADED
UNAVAILABLE
NOT_READY
FAILED
```

A process can be alive without being ready for authoritative work.

## CoverageState

Coverage for a defined source/integration/time/scope may use:

```text
COMPLETE
PARTIAL
INCOMPLETE
NOT_KNOWN
```

Known gaps are separate historical facts and may later be marked recovered without erasing that the gap occurred.

## IndexState

Derived search/index/current-query structures may use:

```text
COMPLETE
INCOMPLETE
REBUILDING
STALE
FAILED
NOT_KNOWN
```

Indexes are derived; they do not become authoritative history.

## Success Must Be Earned

A stage may be reported `SUCCESS` only when that stage's defined success contract completed.

```text
transport success != retrieval complete
retrieval complete != SourceArtifact preserved
SourceArtifact preserved != integrity verified
integrity verified != parsing succeeded
parsing succeeded != native mapping succeeded
mapping succeeded != durable commit succeeded
durable commit succeeded != derived index current
```

A complete pipeline may include:

```text
RetrievalEvent
    ↓
SourceArtifact preservation
    ↓
integrity verification
    ↓
format validation
    ↓
SourceRecord creation
    ↓
Assertion extraction
    ↓
normalization / relationship processing
    ↓
Assessment / conflict / lifecycle processing
    ↓
durable commit
    ↓
derived index/current-view update
```

Each stage retains its own result.

## PARTIAL Is First-Class

If 100 source objects are acquired, 94 map completely, 4 map partially, and 2 are unsupported, Pathfinder does not flatten that history into one generic success.

The applicable RetrievalEvent/ProcessingRecord exposes the actual result distribution and overall `PARTIAL` state.

Likewise, if 12 TAXII pages succeed and page 13 fails before collection completion, preserved pages remain valid historical artifacts and the retrieval remains `PARTIAL` or failed according to the source contract. It is not complete.

## NOT_PERFORMED Is Not FAILED

Pathfinder distinguishes:

```text
attempted and failed
```

from:

```text
never attempted
```

For example:

```text
enrichment_state = NOT_PERFORMED
```

is not equivalent to `FAILED` and does not mean no enrichment exists anywhere.

## AuditEvent

Pathfinder maintains a first-class `AuditEvent` for security-, authorization-, access-, administration-, and integrity-relevant operations.

Conceptual fields include:

```text
audit_event_id
principal_id
principal_type
action
target_type
target_id
request_id / NOT_APPLICABLE
changeset_id / NOT_APPLICABLE
occurred_at
result
authorization_context
source_system / NOT_APPLICABLE
reason / NOT_RECORDED
failure_code / NOT_APPLICABLE
```

Identity:

```text
audit_event_id = UUIDv7
```

## ChangeRecord Versus AuditEvent

`ChangeRecord` answers:

> **How did governed interpretation, workflow state, or configuration change?**

`AuditEvent` answers:

> **Who or what attempted/performed a material operation, under what authority, and what happened?**

One logical operation may produce both. Neither substitutes for the other.

## ProcessingRecord

`ProcessingRecord` captures machine-processing lineage such as parser execution, normalization, mapping, correlation, ATT&CK classification, lifecycle evaluation, conflict detection, and reprocessing.

Conceptual fields include:

```text
processing_id
process_type
process_name
process_version
input_ids
output_ids
started_at
completed_at / NOT_APPLICABLE
processing_state
failure_code / NOT_APPLICABLE
resource_state / NOT_KNOWN
```

## Three Distinct Histories

Pathfinder maintains related but distinct histories:

```text
INTELLIGENCE HISTORY
    SourceRecords
    Assertions
    Sightings
    Assessments
    Relationships
    LifecycleEvents
    IntelligenceConflicts

CHANGE HISTORY
    ChangeRecords
    ChangeSets

OPERATIONAL / AUDIT HISTORY
    RetrievalEvents
    AuditEvents
    ProcessingRecords
```

These may reference one another but must not collapse into one ambiguous universal event table.

A future `log/show/diff` operator view may project across several histories without duplicating every machine event into ChangeRecord.

## Request Identity

Material API operations receive a stable request identity where appropriate so operators can correlate:

```text
request
authentication
authorization
ChangeSet
AuditEvent
ProcessingRecord
failure
```

without dumping sensitive content into logs.

## Material Audit Actions

At minimum, audit should cover applicable:

```text
authentication failures
authorization failures
raw SourceArtifact access
source/source-collection configuration
integration configuration and enable/disable
credential/trust configuration
Assessment creation
Relationship creation
conflict resolution
lifecycle transition
analyst ChangeSet commit
export
bulk operation
retention/destruction action
security administration
startup/shutdown
schema migration
integrity verification failure
```

Routine low-risk reads may use a lower audit level according to policy.

## Audit Failure

If a security-sensitive or authoritative mutation requires guaranteed audit and the required AuditEvent cannot be committed, the mutation fails closed unless a later explicit contract defines another safe mode.

Pathfinder must not knowingly perform a high-risk authoritative mutation while unable to record that it happened.

Read-only degraded behavior may be permitted later where identity/authorization remain established and the read itself does not require guaranteed audit.

## Committed ChangeSet Atomicity

Phase 0.17 governs the change-history commit boundary:

> **A committed ChangeSet is atomic.**

Either all semantic changes in the ChangeSet commit durably or none commit.

A larger bulk job may be PARTIAL and may create several independent committed ChangeSets, but no one ChangeSet is a partial commit.

## Database Commit Boundary

Pathfinder does not return operation success before required durable commit succeeds.

```text
validate
    ↓
commit
    ↓
commit confirmed
    ↓
return SUCCESS
```

not:

```text
validate
    ↓
return SUCCESS
    ↓
hope commit succeeds
```

## SourceArtifact Commit Boundary

Bytes received do not equal a preserved SourceArtifact.

If the process terminates after network receipt but before required durable preservation:

```text
preservation_state != PRESERVED
```

The exact result reflects what was actually committed.

## Checkpoint Completeness

Collector checkpoints advance only after the durability boundary required by the source contract succeeds.

This applies to TAXII continuation, future collectors, source cursors, pagination state, and integration replay positions.

A checkpoint asserts that Pathfinder safely processed through a boundary. It must be earned.

## Crash Recovery

After unexpected termination, Pathfinder does not assume in-flight work succeeded.

On restart, it distinguishes:

```text
committed work
uncommitted work
pending work
interrupted work
state requiring verification
```

Where state cannot be established safely, `NOT_VERIFIED` or another explicit uncertainty state is preferable to assuming success.

## Startup Readiness

Before advertising readiness for authoritative work, Pathfinder verifies critical state such as:

```text
database reachable and compatible
schema migration state valid
artifact storage accessible
security configuration valid
required credentials/keys/certificates accessible
change history readable
audit path functioning where required
```

Exact checks belong to implementation. Readiness must be established, not assumed.

## Dependency Health

Pathfinder exposes meaningful subsystem state rather than one misleading green/red indicator.

Example:

```text
PostgreSQL = HEALTHY
SourceArtifact storage = HEALTHY
Stronghold integration = DEGRADED
TAXII Source A = UNAVAILABLE
```

A degraded integration does not rewrite the underlying environment into a negative conclusion.

## Coverage and Known Gaps

For each relevant source/integration, Pathfinder should be able to represent coverage facts such as:

```text
coverage_from
coverage_through
coverage_state
known_gap_count
last_complete_retrieval
last_failure
```

A later successful retrieval does not erase a prior gap unless Pathfinder can establish that the missing interval was actually recovered.

A recovered gap remains historical as a recovered gap.

## Absence Queries

Queries equivalent to “Did we ever see X?” must account for coverage and index state.

Potential outcomes include:

```text
MATCH_FOUND
NO_MATCH_WITH_COMPLETE_COVERAGE
NO_MATCH_COVERAGE_INCOMPLETE
NO_MATCH_COVERAGE_UNKNOWN
```

This is safer than a simple true/false result.

## No Result Is Not No Record

The following remain distinct:

```text
no matching record exists
no matching record was found
```

The latter may result from incomplete index, limited query scope, authorization filtering, source coverage gaps, or processing backlog.

Pathfinder APIs must not silently present the latter as universal absence.

## Authorization-Filtered Results

Zero records visible to a caller does not necessarily mean Pathfinder contains zero matching records.

The API must avoid both leaking restricted-object existence and making false completeness claims.

## Indexes and Current Views Are Derived

Search indexes, caches, materialized views, current views, and correlation indexes are derived structures.

They can be rebuilt. Original historical records cannot.

> **Current-state views can be rebuilt. Original historical records cannot.**

Every authoritative query path must expose whether the required derived structures are complete/current enough for the claim being made.

## Index Watermarks

Where practical, indexes expose an advancement point.

Example:

```text
intelligence commit sequence = 812347
index processed through       = 812300
```

Concrete lag is better than “probably current.”

## Current-View Resolution

Current interpretation follows the Phase 0.4/0.17 rules.

For an Assessment domain identified by subject, Assessment type, scope, and applicable time, Pathfinder derives applicable non-invalidated/non-superseded candidates.

No candidate automatically wins because it is newest, highest confidence, human-authored, machine-authored, or supported by the greatest raw record count.

A singular current interpretation requires an explicit semantic mechanism such as:

```text
valid supersession lineage
resolved IntelligenceConflict with resolution Assessment
versioned approved current-view selection policy
```

If materially incompatible applicable Assessments remain unresolved:

```text
current_interpretation_state = CONFLICTED
```

The current view never silently becomes database last-write-wins.

## Rebuildability

Derived state should be reproducible from authoritative historical records and versioned processing contracts.

If a supposedly derived structure contains facts that cannot be reconstructed, that state may actually be authoritative and must be modeled accordingly.

## Processing Backlog

Pathfinder exposes when acquisition/preservation is current but downstream processing is behind.

Example:

```text
SourceArtifacts preserved = current
normalization backlog = 1,418 records
oldest pending = 12 minutes
```

Raw collection state and queryable-intelligence state remain separate.

## Reprocessing and Mixed Versions

Historical reprocessing does not masquerade as live ingestion.

Mixed parser/mapping versions may coexist during migration/reprocessing if each output remains attributable to its process/version.

A newer result never rewrites the earlier ProcessingRecord or historical interpretation.

## Schema Migration

Database migrations are versioned, ordered, auditable, and fail-safe.

Pathfinder must not perform authoritative work against an incompatible schema.

A failed migration produces `NOT_READY` or `FAILED` health/readiness state rather than uncertain normal operation.

Physical storage migrations may change representation but must not silently change historical meaning. Semantic reinterpretation requires explicit new processing lineage.

## Integrity Verification

Pathfinder supports verification of critical structures such as:

```text
SourceArtifact digests
ChangeRecord/ChangeSet references
supersession cycles
Relationship endpoint validity
critical provenance links
```

Integrity failure remains explicit.

## Referential Integrity and Orphans

Pathfinder should reject or flag invalid conditions such as:

```text
Assessment references missing subject
Relationship references invalid/missing endpoint
SourceRecord references missing SourceArtifact unexpectedly
ChangeRecord references nonexistent created record
supersession cycle
```

unless an explicit unavailable/destroyed state permits the missing source material.

Periodic integrity checks should detect structural orphaning caused by defects, failed migrations, corruption, or unsupported direct database modification.

## Direct Database Modification

Direct modification outside supported Pathfinder interfaces is unsupported and may invalidate audit/change-history guarantees.

If suspected or detected:

```text
integrity_state = NOT_VERIFIED
```

or an equivalent explicit system-integrity condition must be available.

## Backup and Restore

A restored Pathfinder instance preserves historical intelligence, ChangeRecords/ChangeSets, AuditEvents, ProcessingRecords, SourceArtifact metadata, principal identity continuity, and version information.

After restore, critical integrity/readiness checks run before full readiness is claimed.

Derived indexes may be rebuilt rather than blindly trusted.

## Time

Originating times, receipt times, processing times, Assessment times, and system/audit times remain separate.

If Pathfinder's local clock becomes materially invalid or jumps unexpectedly, time quality becomes explicit rather than silently reordering history.

UUIDv7 provides useful time-oriented identity but does not replace explicit timestamps or committed ordering metadata.

## Security Event Audit

Security-relevant audit includes authentication/authorization failures, certificate validation failures, replay rejection, rate-limit enforcement, raw-source access, bulk export, security configuration change, and audit-subsystem failure.

Audit output must not expose credentials or raw restricted source content.

## Audit Retention

Intelligence lifecycle does not automatically control audit retention.

```text
Indicator EXPIRED != AuditEvent deletable
SourceArtifact DESTROYED_BY_POLICY != destruction AuditEvent removed
```

The record that destruction occurred remains.

## Export and Bulk Operation Audit

Material exports should preserve principal, time, export profile, scope, handling evaluation, destination context where applicable, object counts where safe, and result.

Bulk operations preserve intended and actual scope.

A bulk operation with partial success is explicitly `PARTIAL`; it is not summarized as complete.

## Configuration History

Material governed configuration should use ChangeRecord/ChangeSet or equivalent reconstructable change history.

Examples include:

```text
Source enable/disable
integration trust changes
lifecycle profile changes
STIX mapping changes
ATT&CK dataset version changes
export policy changes
security-sensitive configuration
```

Pathfinder should be able to determine which relevant configuration/version was in effect when a historical record was processed.

Current configuration does not rewrite historical processing context.

## Operator-Facing Truth

Pathfinder favors explicit states, counts, times, watermarks, and causes over unexplained percentages.

Prefer:

```text
Stronghold integration = AUTHENTICATION_FAILED since T
normalization backlog = 1,418
index lag = 47 commits
last complete TAXII retrieval = T
```

rather than:

```text
Pathfinder health = 87%
```

unless a future explicit contract defines exactly what that number means.

## 2:00 AM Test

Every important failure path should satisfy:

> **When this fails at 2:00 AM, will Pathfinder tell the operator exactly what it received, where it came from, what it could validate, what it understood, how it reached that interpretation, what remains uncertain, and what failed?**

A generic `processing error` is insufficient.

## Engineering Completeness

A feature is not complete merely because its happy path works.

Applicable cases should be defined/tested for:

```text
normal success
invalid input
malformed input
unsupported input
duplicate input
partial input
dependency unavailable
permission denied
resource exhaustion
restart/recovery
idempotency
time behavior
audit behavior
historical preservation
operator-visible failure state
```

Not every component requires every case, but applicable cases must be deliberate.

## Semantic and Negative Tests

Tests should validate Pathfinder invariants, including:

```text
Observable normalization never changes meaning
Assertion never silently becomes Assessment
Sighting never silently becomes compromise
Relationship endpoint rules are enforced
Source correction preserves original Assertion
analyst override preserves original Assessment
TAXII partial retrieval remains partial
incomplete index cannot claim universal absence
ATT&CK NOT_MAPPED causes no significance downgrade
Stronghold observation cannot become Pathfinder/Stronghold threat attribution by accident
committed ChangeSet cannot be partial
```

Negative tests should explicitly exercise prohibited transitions such as overwriting historical records, impersonating integration origins, exporting without authority, advancing checkpoints before durable commit, or claiming complete absence from incomplete coverage.

## Restart and Fault Injection

Critical paths should be tested across interruption points including receive, artifact write, hash calculation, SourceRecord creation, transaction commit, and checkpoint advancement.

Recovery must avoid silent duplicate, silent skip, history rewrite, and false SUCCESS.

Where practical, fault injection should cover disk full, database unavailable, permission loss, corrupt artifact, timeout, partial TAXII pagination, invalid certificate, schema mismatch, index failure, and time anomaly.

## Resource Priority

Under resource pressure, Pathfinder prioritizes authoritative history over rebuildable convenience structures where architecture permits.

General priority intent:

```text
1. original historical records and required SourceArtifact preservation
2. Change/Audit/Processing history
3. durable current authoritative commits
4. rebuildable indexes/materialized views/caches
5. optional enrichment/convenience data
```

> **Do not sacrifice original history to preserve convenience data.**

## Common Truth Separations

```text
process alive != ready
transport success != ingestion success
SourceArtifact PRESERVED != integrity VERIFIED
integrity VERIFIED != parser SUCCESS
parser SUCCESS != durable commit SUCCESS
commit SUCCESS != index current
PARTIAL != SUCCESS
DEGRADED health != successful mutation
NOT_PERFORMED != FAILED
no result != no record exists
incomplete index != intelligence absent
source unavailable != source empty
checkpoint exists != checkpoint safe
current view != historical record
ChangeRecord != AuditEvent
AuditEvent != ProcessingRecord
bulk PARTIAL != partial ChangeSet commit
recovered gap != gap never existed
current configuration != configuration at processing time
UUID order != authoritative event order
unsupported != guessed interpretation
expired intelligence != audit history deletable
restored system != automatically verified system
```

## Phase 0.18 Exit Decision

Phase 0.18 is satisfied when Pathfinder accepts that:

1. Success is reported only when the defined stage contract actually completed.
2. Operation, preservation, integrity, availability, processing, health, coverage, and index state remain typed dimensions.
3. `DEGRADED` is not used as a generic mutation-success result.
4. SourceArtifact preservation/integrity/availability are independent.
5. AuditEvent, ChangeRecord/ChangeSet, RetrievalEvent, and ProcessingRecord remain semantically distinct.
6. Required audit failure causes protected authoritative mutations to fail closed.
7. **A committed ChangeSet is atomic.**
8. A larger bulk operation may be PARTIAL while every committed ChangeSet remains atomic.
9. Durable commit occurs before SUCCESS is returned.
10. Checkpoints advance only after the required safe commit boundary.
11. Crash recovery never assumes ambiguous in-flight work succeeded.
12. Readiness is verified rather than inferred from process liveness.
13. Coverage and known gaps remain explicit historical state.
14. Absence queries account for coverage, authorization, and index completeness.
15. Derived indexes/current views remain rebuildable and never substitute for original history.
16. Current-view resolution never silently uses newest/highest-confidence/human-wins/machine-wins/majority/last-write-wins.
17. Unresolved incompatible Assessments produce explicit conflicted current interpretation.
18. Reprocessing/mixed versions remain attributable and do not rewrite prior processing.
19. Schema migration and restore require integrity/readiness checks before authoritative work.
20. Operator-facing status uses explicit concrete facts rather than unexplained scores.
21. Engineering completeness includes failure, restart, idempotency, audit, and historical-preservation behavior.
22. Semantic and negative tests protect prohibited transitions.
23. Original historical state is prioritized over rebuildable convenience data.
24. **The original record is maintained no matter what.**
