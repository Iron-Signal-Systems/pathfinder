# Pathfinder Phase 0.4 — Assertion and Assessment Model

## Purpose

Pathfinder must distinguish an attributable claim from Pathfinder's or an authorized analyst's judgment of that claim.

> **Record the claim before judging the claim.**

The canonical source-to-interpretation chain is:

```text
SourceArtifact
    exact acquired bytes
        ↓
SourceRecord
    logical source item + locator
        ↓
Assertion
    what the source claimed
        ↓
Normalized Pathfinder intelligence
    what Pathfinder understood the claim to refer to
        ↓
Assessment
    what Pathfinder or an authorized analyst concluded
```

These are separate records and separate states of knowledge.

The governing historical invariant is:

> **The original record is maintained no matter what.**

## Core Truth Separation

```text
source said X != Pathfinder believes X
Pathfinder believes X != X objectively proven
source confidence != Pathfinder confidence
Assertion != Assessment
Assessment != authorization
current interpretation != historical interpretation
```

## Assertion

An `Assertion` is an attributable claim extracted from or intentionally recorded against a specific SourceRecord.

An Assertion answers:

> **What did this source claim?**

Examples include a vendor reporting an IP as command-and-control infrastructure, a government advisory reporting active exploitation, or a report stating that an actor uses a malware family.

Pathfinder can establish that the source made the recorded claim without establishing that the claim is objectively true.

### Assertion Authority

The authority for an Assertion is the source that made the claim.

Pathfinder is authoritative for its record of receipt, extraction, and interpretation. It does not silently become authoritative for the real-world truth of the assertion.

### Assertion Identity

Every Assertion receives a Pathfinder-controlled identity.

```text
assertion_id = UUIDv7
```

External identifiers remain separate.

### Assertion Conceptual Fields

```text
assertion_id
source_id
source_record_id
subject
predicate / assertion_type
object / asserted_value
asserted_at / NOT_KNOWN
effective_from / NOT_KNOWN
effective_until / NOT_KNOWN
source_confidence_original / NOT_REPORTED
source_confidence_normalized / UNMAPPED / NOT_REPORTED
source_locator
extraction_method
extraction_version
created_at
lifecycle_state
```

Exact field names are finalized in schema work. The semantic distinctions are not optional.

### Source Locator

An Assertion should retain enough information to trace the claim through its SourceRecord to the parent SourceArtifact where practical.

Examples include:

```text
JSON Pointer
XML path
STIX object identifier + bundle/page context
CSV record number
document section
byte range where safely established
structured record locator
```

### Assertion Extraction Method

Initial conceptual extraction origins include:

```text
SOURCE_STRUCTURED
PARSER_EXTRACTED
ANALYST_RECORDED
```

Machine extraction must not be represented as though the source itself supplied Pathfinder's normalized vocabulary.

### Historical Assertions

Assertions are historical records.

If a source later corrects, withdraws, or changes a claim, Pathfinder records a new source-derived record and preserves the original Assertion.

```text
source corrected assertion != source never made original assertion
```

## Assertion Versus Normalized Intelligence

Assertions preserve attributable claims. Pathfinder objects preserve normalized intelligence concepts.

Example:

```text
SourceArtifact
    ↓
SourceRecord
    ↓
Assertion
    "203.0.113.17 is command-and-control infrastructure"
    ↓
normalized objects
    ├─ Observable: 203.0.113.17
    ├─ Infrastructure: C2 construct
    └─ Relationship: associated_with
```

The normalized objects and Relationship do not replace the Assertion.

Multiple Assertions may support the same normalized object or Relationship while retaining separate provenance and source independence state.

## Assessment

An `Assessment` is a Pathfinder-recorded judgment about an explicitly supported subject.

An Assessment answers:

> **What conclusion has Pathfinder or an authorized human analyst reached about this subject?**

Examples include:

```text
threat classification
relationship validity
source reliability
operational relevance
attribution judgment
observation validity
false-positive interpretation
```

### Assessment Authority

Every Assessment identifies who or what made the judgment.

Initial authority classes are exactly:

```text
HUMAN_ANALYST
PATHFINDER_PROCESS
```

External sources are not Pathfinder Assessment authorities.

```text
Vendor says malicious = Assertion
Pathfinder assesses malicious = Assessment
Analyst assesses malicious = Assessment
```

Authentication, source trust, source reliability, or signed source material does not convert an external judgment into a Pathfinder Assessment.

### Assessment Subject

An Assessment may target an explicitly supported Pathfinder subject when the Assessment type permits it.

Initial subject classes include:

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

`Sighting` is explicitly supported so Pathfinder may assess an observation's validity, interpretation, or operational significance without rewriting the original observation.

Example:

```text
Sighting A:
    sensor reported communication

Assessment B:
    subject = Sighting A
    assessment_type = observation_validity
    assessment_value = INVALID
    basis = confirmed sensor defect
```

Sighting A remains historical.

### Assessment Identity

```text
assessment_id = UUIDv7
```

### Assessment Conceptual Fields

```text
assessment_id
subject_id
assessment_type
assessment_value
assessment_authority
assessment_method
method_version / NOT_APPLICABLE
confidence
basis
assessed_at
rationale / NOT_RECORDED
supersedes / NOT_APPLICABLE
lifecycle_state
created_at
```

Confidence semantics are governed by Phase 0.6.

## Typed Assessments

Pathfinder must not create an unrestricted authoritative `assessment = arbitrary text` field.

Assessment types, valid subject classes, and allowed structured values must be explicitly defined.

Potential initial domains include:

```text
THREAT_CLASSIFICATION
RELATIONSHIP_VALIDITY
ATTRIBUTION
OPERATIONAL_RELEVANCE
OBSERVATION_VALIDITY
FALSE_POSITIVE_LIKELIHOOD
SOURCE_CLAIM_QUALITY
SOURCE_RELIABILITY
```

Free-form analyst narrative may accompany a structured Assessment but does not replace the structured semantics.

## Assessment Basis

An Assessment identifies what supported the judgment.

Potential basis references include:

```text
Assertions
Sightings
Relationships
Indicators
SourceRecords
SourceArtifacts where access/handling permits
other Assessments
IntelligenceConflicts
local investigation records
analyst reasoning
```

Pathfinder must preserve the difference between several basis records and several genuinely independent origins.

## Machine Assessments

Automated Assessments preserve:

```text
process identity
rule/algorithm identity
version
input records
basis
processing time
result
```

Pathfinder must not store unexplained machine scores or confidence values.

A future statistical or machine-learning process remains subject to the same provenance requirements.

## Human Analyst Assessments

Human Assessments preserve the authenticated analyst principal, subject, conclusion, confidence where applicable, basis, assessed time, and rationale where recorded.

An analyst may confirm, dispute, supersede, reject, qualify, or reinterpret according to the analyst/change-history contract.

Human authority never permits rewriting preserved source or observation history.

## Source Reliability Is an Assessment

Pathfinder source reliability is represented through historical Assessments against `Source` or `SourceCollection`.

Example:

```text
Assessment
    subject = SourceCollection A
    assessment_type = SOURCE_RELIABILITY
    assessment_value = HIGH
    authority = HUMAN_ANALYST or PATHFINDER_PROCESS
```

Pathfinder must not implement authoritative reliability as a mutable field equivalent to:

```text
Source.reliability = HIGH
```

The current reliability view is derived from Assessment history under current-view resolution rules.

## Confidence

Pathfinder keeps distinct:

```text
source-reported confidence
Pathfinder process Assessment confidence
human analyst Assessment confidence
source reliability
corroboration
operational relevance
```

Source confidence is never silently copied into Pathfinder confidence.

Confidence never creates authorization.

## Negative Assertions

An explicit negative claim is materially different from absence.

```text
Source asserts Domain X is benign
    !=
Source has no record for Domain X
```

Therefore:

```text
no malicious Assertion != benign Assertion
no Indicator != benign
no Sighting != not observed everywhere
no Assessment != safe
```

## Conflicting Assertions and Assessments

Pathfinder allows contradictory Assertions and Assessments to coexist.

It does not resolve them through majority vote, highest confidence, newest record, or database last-write-wins.

`IntelligenceConflict` records materially incompatible claims or judgments when the conflict contract applies.

The original participating records remain intact after conflict resolution.

## Supersession

A new Assessment may explicitly supersede an earlier Assessment.

The earlier Assessment remains historical.

```text
current Assessment changed != historical Assessment rewritten
```

Supersession must be explicit, attributable, and cycle-free.

## Current-View Resolution

For an assessment domain identified by at least:

```text
subject
assessment_type
applicable scope
applicable time
```

Pathfinder derives the applicable, non-invalidated, non-superseded Assessment candidates.

No Assessment becomes the singular current interpretation merely because it is:

```text
newest
highest confidence
human-authored
machine-authored
supported by the largest raw record count
```

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

The competing Assessments remain available subject to authorization.

## Knowledge at Time

Pathfinder must be able to distinguish:

```text
what the current interpretation is now
what Pathfinder actually knew at historical time T
what later information says about historical time T
```

Later information must not be projected backward to make an earlier Assessment appear to have known facts that arrived later.

## Relationship to Indicators

An external source report that an Observable has threat significance first becomes an attributable Assertion.

Pathfinder may then create or associate an Indicator under the Indicator contract.

The Indicator remains traceable to its supporting Assertions and Assessments.

## Relationship to Sightings

A Sighting may support an Assessment without becoming the Assessment.

Example:

```text
external intelligence:
    IP X associated with C2

Stronghold Sighting:
    FIN-PC-17 contacted IP X

Assessment:
    operational relevance = HIGH
```

The Sighting establishes local observation. It does not independently prove maliciousness.

## No Silent Promotion

Forbidden implicit transitions include:

```text
SourceArtifact -> trusted source content
SourceRecord -> established fact
Assertion -> Assessment
Observable -> Indicator
Indicator -> malicious
Sighting -> compromise
Relationship -> identity
Association -> attribution
HIGH confidence -> enforcement
```

Every transition that adds meaning requires an attributable processing step or authorized Assessment.

## Change History

Material analyst changes to Assessments use the Phase 0.17 Git-style `ChangeRecord` / `ChangeSet` model.

A correction moves forward by creating new records and explicit supersession/dispute/resolution history.

> **The original record is maintained no matter what.**

## Common Truth Separations

```text
SourceArtifact != SourceRecord
SourceRecord != Assertion
Assertion != fact
Assertion != Assessment
external-source judgment != Pathfinder Assessment
source confidence != Pathfinder confidence
source reliability != assertion truth
machine Assessment != human Assessment
machine Assessment != analyst approval
Sighting != Assessment
Sighting invalidated != Sighting deleted
Assessment != objective proof
Assessment != authorization
Assessment != enforcement
supporting records != independent sources
negative Assertion != absence
conflicting Assessments != last-write-wins
superseded Assessment != deleted Assessment
withdrawn Assertion != Assertion never existed
current understanding != historical understanding
HIGH confidence != enforcement authority
```

## Phase 0.4 Exit Decision

Phase 0.4 is satisfied when Pathfinder accepts that:

1. Assertion is a first-class attributable source claim.
2. SourceArtifact owns exact acquired bytes; SourceRecord identifies the logical source item supporting the Assertion.
3. Assessment is a Pathfinder or human-analyst judgment.
4. Assessment authority is `HUMAN_ANALYST` or `PATHFINDER_PROCESS`.
5. External-source judgments remain Assertions.
6. Sighting is an explicitly supported Assessment subject.
7. Source reliability is represented as historical Assessment state, not a mutable Source truth field.
8. Assertions and Assessments retain independent Pathfinder identities and provenance.
9. Conflicting Assertions and Assessments may coexist.
10. Current-view resolution never silently uses newest/highest-confidence/human-wins/machine-wins/majority/last-write-wins.
11. Unresolved incompatible Assessments produce an explicit conflicted current state.
12. Historical records survive correction, withdrawal, dispute, supersession, reprocessing, and analyst error.
13. No intelligence meaning is silently promoted into greater certainty or enforcement authority.
14. **The original record is maintained no matter what.**
