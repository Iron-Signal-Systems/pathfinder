# Pathfinder Phase 0.18 — Audit and Engineering Completeness Requirements

## Purpose

Pathfinder must never report more certainty, completeness, or success than its recorded state can support.

> **If Pathfinder cannot prove that an operation completed, it must not report the operation as complete.**

> **If Pathfinder cannot establish coverage, absence must remain unknown rather than becoming a negative conclusion.**

The system must allow an operator, analyst, auditor, or future developer to reconstruct:

```text
what was attempted
what was received
what was preserved
what was validated
what was interpreted
what was committed
what failed
what was skipped
what remained uncertain
what the system knew at the time
```

## Absolute Historical Invariant

Phase 0.17 remains authoritative:

> **The original record is maintained no matter what.**

Audit history follows the same rule.

An audit or processing record may later be corrected, qualified, superseded, associated with a failure, or associated with a compromised identity, but the original record remains.

No maintenance operation may silently erase history merely to make Pathfinder appear clean or internally consistent.

## Success Must Be Earned

A processing operation may be called `SUCCESS` only when every requirement of that operation's success contract has completed.

```text
source retrieved != ingestion succeeded
artifact preserved != source processed
source parsed != native intelligence committed
```

A complete path may include:

```text
retrieval
   ↓
artifact preservation
   ↓
integrity verification
   ↓
format validation
   ↓
parsing
   ↓
SourceRecord creation
   ↓
Assertion extraction
   ↓
normalization
   ↓
relationship creation
   ↓
native validation
   ↓
durable commit
   ↓
index/update processing
```

Each stage has its own state.

## Completion Vocabulary

Pathfinder should use explicit completion states such as:

```text
SUCCESS
PARTIAL
FAILED
DEGRADED
PENDING
NOT_PERFORMED
NOT_APPLICABLE
UNAVAILABLE
```

Existing detailed failure vocabulary remains valid, including:

```text
processing_pending
processing_failed
index_incomplete
source_unavailable
source_authentication_failed
source_rate_limited
source_schema_mismatch
parse_failed
normalization_failed
enrichment_unavailable
correlation_pending
```

A generic success/failure flag is insufficient.

## PARTIAL Is First-Class

`PARTIAL` is a legitimate result.

If 100 objects are received, 94 map completely, 4 map partially, and 2 are unsupported, the overall processing result is `PARTIAL`, not `SUCCESS` merely because most records worked.

Likewise, a paged retrieval that fails before completion is partial even if earlier pages were preserved successfully.

## DEGRADED Is Not SUCCESS

`DEGRADED` means the system remains operational but meaningful capability, coverage, or dependency is impaired.

A service may remain available while a Stronghold integration is unavailable. Any query depending on Stronghold observation coverage must expose that degradation.

## NOT_PERFORMED and NOT_KNOWN

Pathfinder must distinguish an operation that was attempted and failed from one that was never attempted.

Unknown remains explicit. The system must not replace uncertainty with guessed values merely for schema convenience.

## AuditEvent

Pathfinder should maintain a first-class `AuditEvent` for security- and integrity-relevant operations.

Conceptually:

```text
audit_event_id
principal_id
principal_type
action
target_type
target_id
request_id / not_applicable
changeset_id / not_applicable
occurred_at
result
authorization_context
source_system / not_applicable
reason / not_recorded
failure_code / not_applicable
```

`audit_event_id` uses UUIDv7.

AuditEvent is not the same as ChangeRecord.

## ChangeRecord Versus AuditEvent

`ChangeRecord` answers:

> **How did Pathfinder's interpretation or configured state change?**

`AuditEvent` answers:

> **Who or what performed a material action, and what happened operationally?**

Both may reference the same logical operation. Neither substitutes for the other.

## ProcessingRecord

Pathfinder should preserve processing lineage separately in a conceptual `ProcessingRecord` for parser execution, normalization, mapping, correlation, ATT&CK classification, lifecycle evaluation, conflict detection, and reprocessing.

Potential fields:

```text
processing_id
process_type
process_name
process_version
input_ids
output_ids
started_at
completed_at / not_applicable
result
failure_code / not_applicable
resource_state / not_known
```

This provides reproducibility without turning AuditEvent into a generic trace table.

## Three Histories

Pathfinder maintains three related but distinct histories:

```text
INTELLIGENCE HISTORY
    SourceRecords
    Assertions
    Sightings
    Assessments
    Relationships
    LifecycleEvents
    Conflicts

CHANGE HISTORY
    ChangeRecords
    ChangeSets

OPERATIONAL/AUDIT HISTORY
    AuditEvents
    ProcessingRecords
```

They may reference one another but must not collapse into one ambiguous universal events table.

## Request Identity

Material API requests should receive a stable `request_id` so operators can correlate API request, authentication, authorization, ChangeSet, AuditEvent, ProcessingRecord, and failure without dumping sensitive content into logs.

## Material Actions

At minimum, the following should be auditable where applicable:

```text
authentication failures
authorization failures
raw SourceArtifact access
Source creation/change
SourceCollection change
integration configuration change
integration enable/disable
credential/trust configuration change
Assessment creation
Relationship creation
conflict resolution
lifecycle transition
analyst override
ChangeSet commit
export
bulk query/export where policy requires
retention/destruction action
security administration
system startup/shutdown
database migration
integrity verification failure
```

Routine low-risk reads may use a lower audit level depending on policy.

## Audit Failure

If a security-sensitive mutation requires audit and the required AuditEvent cannot be committed, the preferred behavior is to fail closed unless a later contract explicitly defines another safe mode.

Pathfinder must not knowingly perform an authoritative high-risk mutation while being unable to record that it happened.

Some read-only degraded operations may be permitted later if identity and authorization remain safely established and the read does not itself require a guaranteed audit record.

## Indexes and Current Views Are Derived

Search indexes, caches, materialized views, current views, and correlation indexes are derived structures.

They are not original intelligence history.

They may be rebuilt. Original historical records may not.

## Index Completeness

Every index used for authoritative query interpretation must expose completeness state.

Initial states may include:

```text
COMPLETE
INCOMPLETE
REBUILDING
STALE
FAILED
NOT_KNOWN
```

If an index is incomplete and a query returns no result, Pathfinder must not claim that no matching intelligence exists.

The accurate statement is that no match was found within the currently indexed data.

## Index Watermarks

Where practical, indexes should expose an advancement point or watermark.

Example:

```text
intelligence commit sequence:
    812347

index processed through:
    812300
```

Concrete lag is better than an assertion that the index is "probably current."

## Current View Completeness

Current-state views must identify whether they reflect all required historical changes.

A stale current view must not be presented as authoritative current interpretation without qualification.

## Rebuildability

Derived state should be reproducible from authoritative historical records.

```text
original history
    ↓
processing contracts
    ↓
rebuild derived state
```

If a supposedly derived structure contains facts that cannot be reconstructed from history, that structure may actually contain authoritative data and must be modeled accordingly.

## Database Commit Boundary

Application success must not be returned before the required database transaction or durable storage operation has successfully completed.

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

Bytes received do not equal a committed SourceArtifact. Artifact success requires the preservation contract from Phase 0.11.

If the system crashes after network receipt but before durable preservation, the artifact was not successfully preserved.

## Checkpoint Completeness

Collector checkpoints advance only after corresponding durability requirements succeed.

This applies to TAXII, future collectors, integration replay positions, pagination state, and source cursors.

A checkpoint is an assertion that Pathfinder safely processed through a boundary. It must be earned.

## Crash Recovery

After unexpected termination, Pathfinder must not assume in-flight work succeeded.

On restart, the system should distinguish:

```text
committed work
uncommitted work
processing pending
processing interrupted
state requiring verification
```

Where ambiguity exists, `NOT_VERIFIED` is preferable to assuming success.

## Startup Verification

Before advertising full health after startup, Pathfinder should verify critical state required for authoritative work, including database schema compatibility, repository/storage availability, migration state, artifact storage accessibility, security configuration, required key/certificate availability, change-history readability, and audit-path functionality where applicable.

Exact checks belong to implementation. Readiness must be established, not assumed.

## Health Versus Readiness

Pathfinder must distinguish process liveness from readiness for authoritative work.

Potential conceptual states:

```text
HEALTHY
DEGRADED
NOT_READY
FAILED
```

A running process whose database is unavailable is alive but not ready.

## Dependency Health

External dependency state remains visible. Pathfinder should avoid one global green/red indicator that hides meaningful subsystem failure.

## Source Coverage

For each source or integration, Pathfinder should be capable of representing coverage state, such as:

```text
coverage_from
coverage_through
coverage_state
known_gap_count
last_successful_retrieval
last_failure
```

Not every source can establish full historical coverage. `NOT_KNOWN` is valid where appropriate.

## Known Gaps

Known gaps are first-class operational facts.

A later successful synchronization does not erase knowledge that a gap occurred unless Pathfinder can prove the missing interval was recovered.

If recovered, the gap may become `RECOVERED` with recovery time, method, records recovered, and basis for declaring completeness. Historical audit still shows that Pathfinder previously operated with incomplete coverage.

## Absence Queries

Queries equivalent to "Did we ever see X?" must account for coverage.

Possible outcomes include:

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

The latter may result from incomplete index, limited query scope, authorization filtering, source coverage gap, or processing backlog.

## Authorization-Filtered Results

A caller may lack authority to see some matching records. Zero visible results do not necessarily mean Pathfinder contains zero matching records.

The API must avoid leaking restricted-object existence while also avoiding false completeness claims.

## Processing Backlog

Pathfinder must expose when acquisition/preservation is current but downstream processing is behind.

Example:

```text
SourceArtifacts preserved:
    current

normalization:
    12 minutes behind
```

Raw collection state and queryable-intelligence state remain separate.

Where practical, each processing stage should expose a committed input point, completed processing point, pending count, and oldest pending age.

## Reprocessing and Mixed Versions

Historical reprocessing must not masquerade as live ingestion.

During upgrades, mixed parser or mapping versions may coexist if visible. Pathfinder must identify which version produced each result.

New processing never rewrites the original processing history.

## Schema Migration

Database migrations must be versioned, ordered, auditable, and fail-safe.

Pathfinder must not start authoritative processing against an incompatible schema.

A failed migration should result in `NOT_READY` rather than operation against uncertain data structures.

Storage migrations may change physical representation but must not silently change historical semantic meaning. Semantic reinterpretation requires explicit reprocessing/versioning.

## Integrity Verification

Pathfinder should support verification of critical immutable structures, including SourceArtifact hashes, ChangeRecord lineage, critical record references, supersession cycles, relationship endpoint validity, and provenance links.

Integrity verification failure remains explicit.

## Referential Integrity and Orphan Detection

Pathfinder should reject or flag invalid states such as missing Assessment subjects, missing Relationship endpoints, unexpectedly missing SourceArtifacts, ChangeRecords referencing nonexistent records, and supersession cycles unless an explicit destroyed/unavailable state permits the absence.

Periodic integrity checks should identify structural orphaning caused by defects, failed migration, direct database tampering, or storage corruption.

## Direct Database Modification

Direct database modification outside supported Pathfinder interfaces is unsupported and may invalidate integrity guarantees.

If detected or suspected, Pathfinder should be capable of reporting:

```text
integrity:
    NOT_VERIFIED
```

rather than pretending audit history proves all changes.

## Backup and Restore

A restored Pathfinder instance must preserve historical records, ChangeRecords, AuditEvents, ProcessingRecords, SourceArtifact metadata, identity continuity, and version information.

After restore, critical integrity should be verified before full readiness is declared. Derived indexes may be rebuilt instead of trusted blindly.

## Time

Audit and processing depend on trustworthy time.

Pathfinder preserves originating timestamps separately from local receipt/processing time.

If local system time becomes materially invalid or jumps unexpectedly, a degraded/not-verified time state should be available.

The system must not silently reorder historical events merely to make timestamps appear monotonic.

UUIDv7 is useful for time-oriented identity but does not replace explicit timestamps or committed ordering metadata.

## Security Event Audit

Security-relevant events include authentication failure, authorization denial, certificate validation failure, unexpected identity, replay rejection, rate-limit enforcement, raw-source access, bulk export, administrative security changes, and audit subsystem failure.

Audit output must not expose secrets.

## Audit Retention

Audit retention is a distinct policy.

Expiration of intelligence does not imply expiration of audit history.

SourceArtifact destruction by policy does not imply the destruction AuditEvent disappears.

## Export and Bulk Operation Audit

Material exports should record principal, time, export profile, scope, object count where safe, handling evaluation, destination context where applicable, and result.

Bulk operations should preserve intended and actual scope, including partial results.

Do not reduce a partial bulk operation to "bulk update complete."

## Configuration History

Material configuration should use Git-style ChangeRecords or equivalent reconstructable history.

Examples include source enable/disable, lifecycle profile changes, STIX mapping upgrades, ATT&CK dataset version changes, integration trust changes, and API policy changes.

Pathfinder should eventually be able to answer which relevant configuration was in effect when a record was processed.

Current configuration must not rewrite historical processing context.

## 2:00 AM Test

Every important failure path should pass the ISS operational test:

> **When this fails at 2:00 AM, will Pathfinder tell the operator exactly what it received, where it came from, what it could validate, what it understood, how it reached that interpretation, what remains uncertain, and what failed?**

A useful result should expose concrete stage-level state rather than a generic `processing error`.

## Operator-Facing Truth

Operational status should favor explicit facts, named states, concrete counts, timestamps, and watermarks over unexplained health percentages.

Pathfinder should not expose values such as `system confidence = 93%`, `health = 87%`, or `completeness = 98.4%` unless an explicit contract defines exactly how those values are calculated and what they mean.

## Engineering Completeness

A feature is not complete merely because its happy path works.

A Phase 1 component should not be considered engineering-complete until applicable cases are defined and tested for:

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

Not every component requires every category, but applicable cases must be deliberate.

## Semantic Tests

Tests should validate product invariants, not merely code paths.

Examples include:

```text
Observable normalization never changes meaning
Assertion cannot become Assessment silently
Sighting cannot imply compromise
Relationship cannot bypass endpoint rules
expired Indicator does not delete history
source correction preserves original Assertion
analyst override preserves original Assessment
TAXII partial retrieval remains PARTIAL
incomplete index cannot claim universal absence
ATT&CK NOT_MAPPED does not reduce threat significance
Stronghold observation cannot become Stronghold-authored threat attribution
```

## Negative Tests

Pathfinder should deliberately test prohibited transitions, including attempts to modify SourceArtifact bytes, overwrite historical Assertions, create unsupported Relationships, impersonate integration origins, export without authority, advance checkpoints before commit, report complete absence from incomplete indexes, or delete superseded Assessments through ordinary analyst paths.

Expected behavior is explicit failure.

## Restart and Fault-Injection Tests

Critical paths should be tested across interruption points such as after receive, after artifact write, after hash calculation, after SourceRecord creation, before transaction commit, and after commit but before checkpoint advancement.

Recovery must avoid silent duplicate, silent skip, history rewrite, and false SUCCESS.

Where practical, fault injection should cover disk full, database unavailable, permission loss, corrupted artifacts, network timeout, TAXII partial pagination, invalid certificate, schema mismatch, index failure, and clock anomaly.

## UNSUPPORTED Is Valid

Pathfinder engineering must be comfortable returning `UNSUPPORTED` rather than implementing unsafe guesses.

Explicit unsupported behavior is preferable to semantic corruption.

## Degraded Dependency Does Not Rewrite Data

If an enrichment or external dependency is unavailable, existing intelligence remains intact and the enrichment/dependency state becomes unavailable or degraded. Pathfinder does not remove or downgrade original records merely because optional processing cannot currently run.

## Monitoring and Alerts

Operational metrics may summarize retrieval success/failure, artifact preservation, processing backlog, index lag, integration health, audit failures, query latency, and storage consumption.

Metrics are operational summaries, not authoritative historical records unless explicitly modeled as such.

Alerts should focus on actionable conditions such as artifact integrity mismatch, audit write failure, database unavailable, persistent indexing lag, source coverage gap, integration authentication failure, checkpoint failure, and storage safety thresholds.

Alert absence is not proof of system health.

## Storage Pressure

If storage pressure threatens historical preservation, Pathfinder should protect authoritative records over expendable derived state where architecture permits.

Potential priority:

```text
1. original historical records
2. SourceArtifacts required by policy
3. Change/Audit/Processing history
4. current authoritative commits
5. rebuildable indexes/caches
6. optional derived/enrichment caches
```

> **Do not sacrifice original history to preserve convenience data.**

## Readiness for Phase 1

Phase 0 is complete only when the contracts collectively answer:

```text
What objects exist?
What do they mean?
What do they not mean?
Where did they come from?
How are they related?
How does time affect them?
How are observations represented?
How is disagreement preserved?
How are raw sources preserved?
How do STIX/TAXII/ATT&CK fit?
Who may perform which actions?
How do ISS products integrate?
How do analysts change interpretation?
How does Pathfinder prove completeness and preserve history?
```

If Phase 1 implementation must guess at fundamental semantics, Phase 0 is not actually complete.

## Common Truth Separations

```text
process alive                    != ready
HTTP success                     != ingestion success
artifact preserved               != artifact processed
parser success                   != mapping success
mapping success                  != commit success
commit success                   != index current
PARTIAL                          != SUCCESS
DEGRADED                         != SUCCESS
NOT_PERFORMED                    != FAILED
no result                        != no record exists
index incomplete                 != intelligence absent
source unavailable               != source has no intelligence
integration unavailable          != no observation occurred
checkpoint exists                != checkpoint safe
bytes received                   != artifact committed
current view                     != historical source
derived index                    != original record
audit event                      != ChangeRecord
ChangeRecord                     != intelligence record
processing history               != source Assertion
successful retry                 != original failure never happened
recovered gap                    != gap never existed
current configuration            != configuration at processing time
UUID ordering                    != authoritative event ordering
health score                     != operational truth
unsupported                      != failed interpretation
expired intelligence             != audit deletable
restored system                  != automatically verified system
feature happy path works         != engineering complete
```

## Phase 0.18 Exit Decision

Phase 0.18 is satisfied when Pathfinder accepts that:

1. Success is reported only when the defined success contract is actually satisfied.
2. `PARTIAL`, `FAILED`, `DEGRADED`, `PENDING`, `NOT_PERFORMED`, `NOT_APPLICABLE`, and `UNAVAILABLE` remain explicit.
3. The original historical record is maintained no matter what.
4. AuditEvent, ChangeRecord, and ProcessingRecord have distinct semantics.
5. Security-sensitive and authoritative mutations are attributable and auditable.
6. Required audit failure causes sensitive mutations to fail closed.
7. Derived indexes, caches, and current views never become substitutes for original history.
8. Index and view completeness remain explicitly visible.
9. Derived state can be rebuilt from authoritative history.
10. Durable commit occurs before SUCCESS is returned.
11. Collector checkpoints advance only after safe commit boundaries.
12. Crash recovery never assumes ambiguous in-flight work succeeded.
13. Startup readiness requires verification of critical dependencies and state.
14. Process health and authoritative readiness remain separate.
15. External dependency degradation remains visible.
16. Source/integration coverage and known gaps remain first-class operational state.
17. Recovered gaps remain historically visible.
18. Absence queries incorporate coverage and index completeness.
19. Authorization filtering does not become false universal absence.
20. Processing backlog and stage lag remain visible.
21. Reprocessing and mixed processing versions remain identifiable.
22. Schema migrations cannot silently alter historical meaning.
23. Integrity verification failures remain explicit.
24. Referential-integrity and orphan problems are detectable.
25. Unsupported direct database mutation can invalidate integrity guarantees.
26. Restore requires verification before full readiness.
27. Time anomalies do not silently reorder or rewrite history.
28. Security-sensitive events are auditable without leaking secrets.
29. Intelligence expiration does not automatically remove audit history.
30. Export and bulk operations preserve attributable scope and result.
31. Historical processing can identify the configuration and versions used at the time.
32. Every major failure path must satisfy the Pathfinder 2:00 AM operational test.
33. Operator-facing state favors concrete facts and named states over unexplained scores.
34. Engineering completeness includes relevant failure, restart, idempotency, audit, and historical-preservation behavior.
35. Tests validate semantic invariants in addition to implementation behavior.
36. Forbidden operations receive explicit negative tests.
37. Critical paths receive interruption/recovery testing.
38. Fault injection is used where failure can threaten historical integrity.
39. `UNSUPPORTED` remains a valid and preferred outcome over unsafe guessing.
40. Original historical data is prioritized over rebuildable convenience state under resource pressure.
41. Phase 1 does not begin until the Phase 0 contracts can be implemented without guessing fundamental semantics.
