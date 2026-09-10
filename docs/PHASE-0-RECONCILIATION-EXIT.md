# Pathfinder Phase 0 — Reconciliation and Exit Review

## Purpose

This document reconciles the Phase 0 contracts after completion of Phases 0.1 through 0.18 and freezes the decisions required to begin Phase 1 without guessing at Pathfinder semantics.

Where an earlier Phase 0 document conflicts with a refinement explicitly frozen here, this reconciliation document governs. Earlier documents remain historical design records and are not rewritten merely to make the design process appear linear.

The governing historical invariant is:

> **The original record is maintained no matter what.**

The governing product authority statement remains:

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

The governing operational rule remains:

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

## Phase 0 Exit Status

After cross-document review, the Phase 0 architecture is internally coherent once the refinements in this document are applied.

```text
PHASE 0
    COMPLETE

EXIT GATE
    PASS

NEXT
    Phase 1.1 — Runtime and Repository Foundation
```

Phase 1 implementation must follow the reconciled contracts rather than reintroducing superseded early assumptions.

---

# 1. Canonical Object Families

Pathfinder does not place every record into one undifferentiated intelligence-object category.

The canonical Phase 0 object families are:

```text
SOURCE / PROVENANCE
    Source
    SourceCollection
    RetrievalEvent
    SourceArtifact
    SourceRecord

INTELLIGENCE
    Assertion
    Observable
    Indicator
    Sighting
    Assessment
    Relationship
    ThreatActor
    Campaign
    Malware
    Tool
    Vulnerability
    Technique
    Infrastructure
    Report

INTELLIGENCE STATE
    LifecycleEvent
    IntelligenceConflict

SYSTEM HISTORY
    ChangeRecord
    ChangeSet
    AuditEvent
    ProcessingRecord
```

These families may reference one another, but their meanings remain distinct.

All first-class Pathfinder records use Pathfinder-controlled identity. The initial identity direction remains UUIDv7 unless a more specific contract explicitly states otherwise.

External identifiers never replace Pathfinder identity.

## Object-Family Truth Separation

```text
SourceArtifact      != SourceRecord
SourceRecord        != Assertion
Assertion           != Assessment
Sighting            != Assessment
Relationship        != provenance link
LifecycleEvent      != ChangeRecord
ChangeRecord        != AuditEvent
AuditEvent          != ProcessingRecord
current view        != historical record
```

---

# 2. Canonical Source Preservation Chain

Phase 0.11 refines the earlier SourceRecord wording from Phases 0.2 and 0.5.

The canonical acquisition and interpretation chain is:

```text
Source
  ↓
SourceCollection
  ↓
RetrievalEvent
  ↓
SourceArtifact
  ↓
SourceRecord
  ↓
Assertion
  ↓
normalized Pathfinder intelligence
```

## SourceArtifact

`SourceArtifact` owns the exact immutable acquired payload at the preservation boundary.

It is responsible for properties such as:

```text
source_artifact_id
source_id
source_collection_id / not_applicable
retrieval_event_id / not_applicable
received_at
source_location
media_type / not_known
content_encoding / not_known
byte_length
sha256
preservation_state
integrity_state
availability_state
storage_reference
handling_profile
created_at
```

The exact preserved bytes and Pathfinder preservation digest belong to SourceArtifact, not SourceRecord.

## SourceRecord

`SourceRecord` is a logical source item represented within a SourceArtifact.

Examples:

```text
SourceArtifact:
    one TAXII response containing 200 STIX objects

SourceRecords:
    logical STIX object 1
    logical STIX object 2
    ...
    logical STIX object 200
```

A SourceRecord references its parent SourceArtifact and preserves a deterministic locator where practical.

Potential SourceRecord fields include:

```text
source_record_id
source_artifact_id
source_id
source_collection_id / not_applicable
external_record_id / not_known
source_publication_time / not_known
source_modified_time / not_known
source_observation_time / not_known
source_locator
source_marking
validation_state
processing_state
created_at
```

A SourceRecord does not claim reconstructed parser output is the original source bytes.

Where exact per-record bytes cannot be safely established:

```text
exact_record_bytes = NOT_ESTABLISHED
```

The exact parent SourceArtifact remains authoritative for what Pathfinder acquired.

## Refinement of Earlier Wording

Earlier statements that a SourceRecord itself stores or owns `raw_content_hash`, exact preserved source bytes, `preserved_content_reference`, or artifact-level preservation state are superseded by Phase 0.11 and this reconciliation.

The original source representation referenced by normalization therefore means the applicable SourceArtifact plus SourceRecord locator/provenance, not a reserialized SourceRecord.

---

# 3. Permanent Historical Record

The Phase 0.17 rule is global:

> **The original record is maintained no matter what.**

An original record is not rewritten or removed merely because it was later determined to be:

```text
incorrect
superseded
revoked
disputed
conflicted
misinterpreted
produced by a parser defect
produced by a sensor defect
created by analyst error
created by a compromised account
based on poisoned intelligence
```

Corrections move history forward.

Later records may:

```text
supersede
revoke
dispute
qualify
correct
reinterpret
resolve
invalidate
```

but do not cause the original record to cease having existed.

## Mandatory Raw-Byte Destruction

If law, contract, handling policy, or another authorized retention requirement mandates destruction of SourceArtifact bytes, Pathfinder retains immutable historical metadata describing that artifact and the destruction action.

Therefore:

```text
raw bytes destroyed by authorized policy
    != historical record removed
```

---

# 4. Assessment Authority and Subjects

## Assessment Authority

Pathfinder Assessment authorities are:

```text
HUMAN_ANALYST
PATHFINDER_PROCESS
```

External-source judgments remain Assertions attributable to the external source.

An external source is not a Pathfinder Assessment authority merely because Pathfinder trusts or authenticates that source.

```text
external source judgment
    = Assertion
    != Pathfinder Assessment
```

An analyst may record an Assertion under an explicit manual-entry/source-authority contract, but that remains distinguishable from an analyst Assessment.

## Assessment Subjects

Phase 0.17 requires Sighting to be an allowed Assessment subject so Pathfinder can preserve observations while separately assessing their validity or interpretation.

Initial Assessment subjects therefore include, where the assessment type permits:

```text
Source
SourceCollection
Observable
Indicator
Assertion
Sighting
Relationship
ThreatActor
Campaign
Malware
Tool
Vulnerability
Technique
Infrastructure
Report
```

Assessment types still control which subject classes and values are valid.

Free-form narrative does not become structured Assessment semantics merely because an analyst entered it.

---

# 5. Current-View Resolution Semantics

Phase 0.17 establishes that current-state views are derived and rebuildable. This reconciliation freezes how Pathfinder avoids hidden last-write-wins behavior.

For an assessment domain identified by at least:

```text
subject
assessment_type
applicable scope
applicable time
```

Pathfinder derives the set of applicable, non-invalidated, non-superseded Assessment candidates.

No Assessment becomes the singular current interpretation merely because it is:

```text
newest
highest confidence
human-authored
machine-authored
supported by the greatest raw record count
```

## Singular Current Interpretation

Pathfinder may expose one singular current interpretation only when an explicit semantic mechanism establishes it, such as:

```text
valid supersession lineage
resolved IntelligenceConflict with resolution Assessment
versioned approved current-view selection policy
```

The selection mechanism and its basis must remain attributable.

## Unresolved Competing Assessments

When multiple materially incompatible applicable Assessments remain and no authorized resolution establishes one current interpretation, Pathfinder does not choose a hidden winner.

The current view reports an explicit conflicted state and exposes the competing Assessments subject to authorization.

Conceptually:

```text
current_interpretation_state:
    CONFLICTED

current_assessments:
    Assessment A
    Assessment B
```

This is preferable to database last-write-wins.

## Historical Reconstruction

Current-view derivation must never alter what Pathfinder knew at an earlier time.

```text
current interpretation
    != knowledge-at-time interpretation
```

---

# 6. Relationship Registry and ATT&CK

Phase 0.14 expands the Phase 0.7 `uses` endpoint registry to include ATT&CK behavioral relationships.

The initial valid `uses` directions therefore include:

```text
ThreatActor -> Malware
ThreatActor -> Tool
ThreatActor -> Infrastructure
ThreatActor -> Technique

Campaign -> Malware
Campaign -> Tool
Campaign -> Infrastructure
Campaign -> Technique
```

`uses` remains distinct from ownership, identity, attribution certainty, or local observation.

## Relationship Origin Versus ATT&CK Mapping Origin

These are separate dimensions and MUST NOT be collapsed into one enum.

Pathfinder Relationship origin remains:

```text
SOURCE_NORMALIZED
PATHFINDER_DERIVED
ANALYST_RECORDED
```

ATT&CK mapping origin remains:

```text
ATTACK_NATIVE
SOURCE_REPORTED
PATHFINDER_ASSOCIATED
LOCALLY_SUGGESTED
LOCALLY_CONFIRMED
ANALYST_RECORDED
```

Example mappings include:

```text
ATTACK_NATIVE
    relationship_origin = SOURCE_NORMALIZED

SOURCE_REPORTED
    relationship_origin = SOURCE_NORMALIZED

PATHFINDER_ASSOCIATED
    relationship_origin = PATHFINDER_DERIVED
```

For locally suggested/confirmed or analyst mappings, the applicable processing/analyst contract determines the Relationship origin while the ATT&CK mapping origin remains separately recorded.

ATT&CK continues to be optional classification metadata.

```text
ATT&CK NOT_MAPPED
    != lower confidence
    != lower severity
    != lower operational relevance
    != lower response priority
```

---

# 7. Typed State Namespaces

Pathfinder does not define one universal `status` or `state` enum.

Similar words may appear in different domains, but every persisted state belongs to an explicit namespace/type.

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

## OperationResult

General operation completion semantics:

```text
SUCCESS
PARTIAL
FAILED
PENDING
NOT_PERFORMED
NOT_APPLICABLE
```

`DEGRADED` normally belongs to health/capability state rather than pretending a completed mutation partially succeeded.

## PreservationState

For SourceArtifact acquisition/preservation:

```text
RECEIVING
PRESERVED
PARTIAL
FAILED
```

## IntegrityState

For content/record integrity verification:

```text
NOT_VERIFIED
VERIFIED
MISMATCH
```

## AvailabilityState

For whether preserved material can currently be accessed/processed:

```text
AVAILABLE
QUARANTINED
UNAVAILABLE
DESTROYED_BY_POLICY
```

This refines Phase 0.11's earlier combined preservation-state vocabulary. `QUARANTINED` and `DESTROYED_BY_POLICY` describe availability/handling, not whether original preservation completed.

## ProcessingState

Processing-specific state may include:

```text
PENDING
SUCCESS
PARTIAL
FAILED
UNSUPPORTED
NOT_PERFORMED
```

Detailed failure codes remain separate from the high-level state.

## HealthState

Subsystem/service capability state may include:

```text
HEALTHY
DEGRADED
UNAVAILABLE
NOT_READY
FAILED
```

## CoverageState

Coverage/completeness for a defined scope may include:

```text
COMPLETE
PARTIAL
INCOMPLETE
NOT_KNOWN
```

Known gaps are separate records/state and may later be marked recovered without rewriting the fact that a gap existed.

## IndexState

Derived indexes/current query structures may use:

```text
COMPLETE
INCOMPLETE
REBUILDING
STALE
FAILED
NOT_KNOWN
```

## Existing Domain States

The Phase 0 lifecycle, review, and conflict vocabularies remain independent:

```text
LifecycleState
    ACTIVE
    AGING
    EXPIRED
    REVOKED
    SUPERSEDED
    DISPUTED
    CONFLICTED

ReviewState
    NOT_REQUIRED
    PENDING
    IN_REVIEW
    COMPLETED
    DEFERRED

ConflictStatus
    OPEN
    UNDER_REVIEW
    RESOLVED
    SUPERSEDED
```

Typed state dimensions must be reflected in Go types and database constraints rather than being collapsed into one generic text status.

---

# 8. ChangeRecord, ChangeSet, AuditEvent, and ProcessingRecord

These histories remain separate.

## ChangeRecord

Records an intentional material change to Pathfinder interpretation, workflow state, or governed configuration.

It answers:

> **How did the governed current interpretation/configuration change?**

## ChangeSet

Groups one logical set of ChangeRecords.

A **committed ChangeSet is atomic**.

```text
all semantic changes commit
    or
none of the ChangeSet commits
```

The earlier phrase `atomic where practical` is superseded for committed ChangeSets.

Preflight may identify invalid or ineligible members before commit. A large bulk job may intentionally produce multiple independent ChangeSets. The overall bulk job may therefore finish `PARTIAL`, while every committed ChangeSet remains individually atomic.

```text
bulk operation PARTIAL
    != partially committed ChangeSet
```

## AuditEvent

Records a security-, authorization-, access-, administration-, or integrity-relevant operation and its result.

It answers:

> **Who or what attempted/performed a material operation, under what authority, and what happened?**

## ProcessingRecord

Records machine processing lineage such as parser, normalizer, mapper, correlation, conflict detection, lifecycle evaluation, reprocessing, and similar deterministic/analytical processing.

It answers:

> **Which process/version transformed or evaluated which inputs, and what result did it produce?**

## No Universal Event Table Semantics

These records may share storage infrastructure, but Pathfinder must not collapse them into one ambiguous semantic object merely because all can be represented as events.

The conceptual `log/show/diff` operator experience may project across multiple history families without duplicating every machine operation into ChangeRecord.

---

# 9. Source Reliability

Source reliability is a historical Pathfinder Assessment, not one mutable authoritative field on Source.

```text
Assessment
    subject = Source or SourceCollection
    assessment_type = source_reliability
    value = HIGH | MODERATE | LOW | NOT_ASSESSED
```

A Source or SourceCollection may contain configuration references that control how reliability is evaluated, but an authoritative current reliability judgment is derived from Assessment history using current-view semantics.

Earlier `Source.reliability_profile` wording must not be implemented as a mutable truth field equivalent to `Source.reliability = HIGH`.

Source-reported confidence remains separate from Pathfinder source-reliability Assessment.

---

# 10. Phase 1 Schema Scope

Phase 1.2 does **not** require Pathfinder to implement a complete relational schema for every conceptual Phase 0 intelligence family before proving the first vertical slice.

The canonical Phase 1.2 objective is:

> **Implement the relational schema required for the first complete Pathfinder vertical slice, using the Phase 0 contracts and adding object-specific schema contracts before additional object families are implemented.**

The first vertical slice should prove the minimum set necessary for end-to-end source truth and analyst reconstruction.

Expected early records include, where required by the chosen source:

```text
Source
SourceCollection
RetrievalEvent
SourceArtifact
SourceRecord
Assertion
Observable
Indicator
Sighting
Assessment
Relationship
LifecycleEvent
IntelligenceConflict
ProcessingRecord
AuditEvent
ChangeRecord / ChangeSet where an intentional change requires them
```

Objects such as ThreatActor, Campaign, Malware, Tool, Infrastructure, Technique, Vulnerability, and Report are added only as the first source/workload requires them and after their concrete schema contract is defined.

Pathfinder must not invent broad object fields merely because the conceptual object exists in Phase 0.

---

# 11. Phase 1.1 Boundary

Phase 1 begins with runtime/repository groundwork, not feature implementation.

Phase 1.1 should establish at least:

```text
supported operating environment
service/runtime identity
repository layout
configuration contract
filesystem/state paths
PostgreSQL dependency/version direction
startup/readiness behavior
logging boundary
secret-handling boundary
migration entry point
validation/test entry point
build/release expectations
```

Only after that groundwork is frozen should Phase 1.2 implement the first relational schema.

---

# 12. Reconciled Invariants

The following invariants are considered Phase 0 exit-gate requirements:

```text
Observable != malicious
SourceArtifact != SourceRecord
SourceRecord != Assertion
Assertion != Assessment
Sighting != conclusion
Correlation != identity
Association != attribution
Relationship != ownership
Confidence != authorization
ATT&CK mapping != compromise
ATT&CK NOT_MAPPED != insignificant threat
Stronghold observation != Pathfinder conclusion
FI observation != compromise
Atlas context != Pathfinder asset authority
Pathfinder candidate != downstream command
Current view != historical record
Newest Assessment != automatic winner
Highest confidence != automatic winner
Human Assessment != automatic winner solely because human
Machine Assessment != automatic winner solely because machine
ChangeRecord != AuditEvent
AuditEvent != ProcessingRecord
Bulk operation PARTIAL != partially committed ChangeSet
Expired != deleted
Revoked != deleted
Superseded != deleted
Conflict resolved != opposing records deleted
Raw bytes destroyed by authorized policy != historical record removed
Incomplete index != complete intelligence history
No result != no record exists
Successful transport != accepted intelligence
Successful retry != earlier failure never happened
```

---

# 13. Phase 0 Exit Gate

The Phase 0 exit gate passes with this reconciliation because Pathfinder now has a deterministic answer for:

```text
what objects exist
which families they belong to
what exact source bytes belong to
what a logical SourceRecord means
who may author an Assessment
what may be assessed
how current interpretation is selected
how unresolved competing assessments remain visible
how ATT&CK mappings coexist with native Relationships
how state vocabularies are typed
what a committed ChangeSet guarantees
how change/audit/processing histories differ
how source reliability is represented
how historical records survive every correction
what Phase 1.2 is actually required to implement
```

## Exit Decision

```text
Phase 0 semantic contracts:
    COMPLETE

Phase 0 reconciliation:
    COMPLETE

Unresolved schema-blocking semantic contradictions:
    NONE KNOWN

Phase 0 exit gate:
    PASS

Next authorized design step:
    Phase 1.1 Runtime and Repository Foundation
```

This does not mean every future object field or feature is frozen. It means Phase 1 can begin without guessing at the fundamental meaning, authority, provenance, history, and failure behavior of Pathfinder's records.

---

# Final Rule

> **Preserve the source, preserve the original record, preserve uncertainty, preserve provenance, and never turn intelligence into authority by accident.**
