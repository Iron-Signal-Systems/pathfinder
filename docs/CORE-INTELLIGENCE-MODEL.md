# Pathfinder Phase 0.2 — Core Intelligence Object Model

## Purpose

Pathfinder requires an explicit internal model before schemas, APIs, collectors, or interchange mappings are implemented.

The model preserves the distinction between:

```text
what was acquired
what the source claimed
what was observed
what Pathfinder normalized or derived
what Pathfinder or an authorized analyst assessed
what changed later
```

Pathfinder uses its own internal model. External formats such as STIX may map into or out of this model but do not define Pathfinder's internal semantics.

The governing historical invariant is:

> **The original record is maintained no matter what.**

## Common Identity

Every first-class Pathfinder record receives a Pathfinder-controlled identity.

Initial direction:

```text
Pathfinder ID = UUIDv7
```

External identifiers remain separate and never replace Pathfinder identity.

Where applicable, records preserve creation time, authority, provenance references, processing lineage, lifecycle information, and later change history.

Pathfinder avoids ambiguous nulls. Explicit states such as `NOT_KNOWN`, `NOT_OBSERVED`, `NOT_VERIFIED`, and `NOT_APPLICABLE` are preferred when a state is required.

## Canonical Object Families

Pathfinder does not place every record into one generic intelligence-object bucket.

The canonical Phase 0 families are:

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

These families may reference each other. They do not become semantically interchangeable merely because they share storage or identifiers.

## Source

A `Source` identifies an intelligence provider or origin known to Pathfinder.

A Source answers:

> **Who or what is the intelligence provider/origin?**

A Source does not mean information from that Source is true, reliable, current, or independently corroborated.

Conceptual fields may include:

```text
source_id
name
source_type
provider_identity
status
handling_profile
trust/authentication configuration references
created_at
retired_at / NOT_APPLICABLE
```

Source reliability is not one mutable truth field on Source. Pathfinder reliability judgments are historical `Assessment` records about a Source or SourceCollection.

## SourceCollection

A `SourceCollection` represents a distinct feed, publication stream, TAXII Collection, API collection, or similar source subdivision with its own transport, handling, and collection semantics.

One Source may expose many SourceCollections.

```text
Source != SourceCollection
```

## RetrievalEvent

A `RetrievalEvent` records an acquisition attempt.

It answers operational questions such as:

```text
what was requested
when it was attempted
which Source/SourceCollection was involved
whether transport succeeded
whether results were complete or partial
whether retry/checkpoint state changed
```

A RetrievalEvent may produce zero, one, or many SourceArtifacts.

```text
retrieval succeeded != intelligence accepted
```

## SourceArtifact

A `SourceArtifact` is the immutable acquired payload preserved at Pathfinder's source-preservation boundary before semantic interpretation.

Examples include:

```text
HTTP response entity body
TAXII response page
downloaded JSON/CSV/XML document
vendor export
advisory document
analyst-imported source file
```

A SourceArtifact owns properties such as:

```text
source_artifact_id
source_id
source_collection_id / NOT_APPLICABLE
retrieval_event_id / NOT_APPLICABLE
received_at
source_location
media_type / NOT_KNOWN
content_encoding / NOT_KNOWN
byte_length
sha256
preservation_state
integrity_state
availability_state
storage_reference
handling_profile
created_at
```

The exact preserved bytes and Pathfinder preservation digest belong to `SourceArtifact`, not `SourceRecord`.

```text
SourceArtifact != SourceRecord
```

## SourceRecord

A `SourceRecord` is one logical source item represented within a SourceArtifact.

Examples include:

```text
one STIX object within a TAXII response
one JSON feed object
one CSV row
one logical advisory section
one report item
```

A SourceRecord answers:

> **Which logical item within the acquired source material are we referring to?**

Conceptual lineage:

```text
Source
  ↓
SourceCollection
  ↓
RetrievalEvent
  ↓
SourceArtifact        exact acquired bytes
  ↓
SourceRecord          logical item + locator
  ↓
Assertion
```

A SourceRecord references its SourceArtifact and preserves a deterministic locator where practical. It does not claim reconstructed parser output is the original received byte sequence.

## Assertion

An `Assertion` is an attributable claim extracted from or intentionally recorded against a SourceRecord.

It answers:

> **What did the source claim?**

The source is the authority for the claim. Pathfinder is authoritative for its record that the claim was received and interpreted, not automatically for the real-world truth of the claim.

External-source judgments remain Assertions.

```text
external source judgment = Assertion
external source judgment != Pathfinder Assessment
```

## Observable

An `Observable` is something that can be observed, referenced, or matched.

An Observable is semantically neutral.

Initial classes are defined by the Observable contract and include IPv4, IPv6, domain, URL, SHA-256, email, and X.509 certificate SHA-256 fingerprint.

```text
Observable != malicious
Observable != suspicious
Observable != benign
Observable != Indicator
```

## Indicator

An `Indicator` represents intelligence that one or more Observables or observable patterns have operational threat significance.

An Indicator is not an Observable with `malicious=true`.

Indicator significance remains supported by attributable Assertions, Assessments, provenance, lifecycle, and conflict state.

## Sighting

A `Sighting` records an observation made by an identified observation authority.

A Sighting answers:

> **What was observed, by whom or what, where applicable, and when?**

It does not by itself prove malicious activity, compromise, successful exploitation, attribution, or intent.

Where a Sighting originates from another ISS product, that product remains authoritative for the underlying observation.

## Assessment

An `Assessment` records a Pathfinder or authorized human-analyst judgment about an explicitly supported subject.

Initial Assessment authority classes are:

```text
HUMAN_ANALYST
PATHFINDER_PROCESS
```

External sources are not Pathfinder Assessment authorities. Their judgments remain Assertions.

Initial Assessment subjects may include, where the Assessment type permits:

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

An Assessment may be superseded, disputed, or otherwise reinterpreted through later records. The original Assessment remains historical.

## Relationship

A `Relationship` records a connection Pathfinder can describe between two permitted endpoints under a versioned relationship type.

Relationships preserve identity, relationship type, endpoints, directionality, time context, origin, support, conflict, and derivation lineage.

A Relationship does not own one universal mutable confidence value. Confidence belongs to the Assertion or Assessment expressing that judgment.

```text
Relationship != identity
Relationship != causation
Relationship != ownership
Relationship != attribution
```

## ThreatActor

A `ThreatActor` represents an actor identity or actor construct reported or assessed within threat intelligence.

Vendor names, aliases, clusters, and ATT&CK Groups are not silently merged merely because they appear similar.

Actor attribution remains attributable interpretation.

## Campaign

A `Campaign` represents a bounded or identified collection of related threat activity.

A Campaign may be source-reported, analyst-defined, or Pathfinder-derived under explicit provenance.

Campaign association does not itself prove actor attribution.

## Malware

A `Malware` object represents a malware family, strain, variant, or identified malicious-software construct.

Malware is not an individual file hash.

```text
hash match != malware execution
```

## Tool

A `Tool` represents software used in threat activity that is not inherently represented as malware.

Legitimate administration tools, dual-use utilities, offensive-security tools, and remote-management software may all be Tools.

```text
Tool observed != malicious use established
```

## Vulnerability

A `Vulnerability` represents a known or tracked vulnerability identity such as a CVE or vendor advisory identity.

Pathfinder may own intelligence concerning a Vulnerability but does not own whether a specific Atlas asset is actually vulnerable.

## Technique

A `Technique` represents an external or Pathfinder-recognized behavioral classification, including MITRE ATT&CK Technique/Sub-Technique mappings where useful.

ATT&CK is optional classification/reference metadata. Lack of an ATT&CK mapping does not reduce threat significance, confidence, operational relevance, or escalation priority.

```text
Technique classification != behavior observed locally
```

## Infrastructure

An `Infrastructure` object represents a logical infrastructure construct used to group infrastructure-related intelligence.

Infrastructure is not synonymous with one IP address or domain. Shared infrastructure does not establish common ownership or actor identity.

## Report

A `Report` represents a coherent intelligence publication or analytical product.

A Report may reference many Pathfinder objects. It is not proof that every claim within it is true.

## LifecycleEvent

A `LifecycleEvent` records a lifecycle transition such as aging, expiration, revocation, supersession, dispute, or conflict-related state where the applicable contract permits it.

Lifecycle history is append-only.

```text
lifecycle transition != original record rewritten
```

## IntelligenceConflict

An `IntelligenceConflict` is a first-class record representing materially incompatible Assertions or Assessments within a defined subject, scope, and applicable time.

Conflict is not a data-quality failure and is not resolved by majority vote, highest confidence, or newest-record-wins.

Opposing records remain historical after resolution.

## ChangeRecord

A `ChangeRecord` records an intentional material change to governed interpretation, workflow state, or governed configuration.

It is part of Pathfinder's Git-style change history.

A ChangeRecord does not replace the intelligence records it describes.

## ChangeSet

A `ChangeSet` groups one logical set of ChangeRecords.

> **A committed ChangeSet is atomic.**

Either all semantic changes in that ChangeSet commit or none do. A larger bulk job may produce multiple ChangeSets and may finish `PARTIAL`, but no committed ChangeSet is partially committed.

## AuditEvent

An `AuditEvent` records a security-, authorization-, access-, administration-, or integrity-relevant operation and its result.

It answers who or what acted, under what authority, against what target, and what happened operationally.

```text
AuditEvent != ChangeRecord
```

## ProcessingRecord

A `ProcessingRecord` records machine processing lineage such as parsing, normalization, mapping, correlation, conflict detection, lifecycle evaluation, or reprocessing.

It identifies process/method/version, input, output, time, and result.

```text
ProcessingRecord != AuditEvent
ProcessingRecord != ChangeRecord
```

## Current Interpretation

Current views are derived from historical records. They are rebuildable; historical records are not.

No Assessment automatically becomes the singular current interpretation merely because it is newest, highest confidence, human-authored, machine-authored, or supported by the largest raw record count.

A singular current interpretation requires an explicit semantic mechanism such as valid supersession lineage, resolved IntelligenceConflict with a resolution Assessment, or a versioned approved current-view selection policy.

Where materially incompatible applicable Assessments remain unresolved:

```text
current_interpretation_state = CONFLICTED
```

Pathfinder does not silently use database last-write-wins.

## Object Ownership Rules

Pathfinder owns its records and interpretation of threat intelligence, including its identities, normalization, Assessments, Relationships, provenance, lifecycle, processing history, conflict history, and current-view derivation.

It does not silently claim authority over external real-world facts.

Stronghold observations remain Stronghold-origin observations. FI observations remain FI-origin observations. Atlas asset/environment facts remain Atlas authority. Guidon remains backup/recovery authority.

## Core Truth Separations

```text
Source != SourceCollection
RetrievalEvent != SourceArtifact
SourceArtifact != SourceRecord
SourceRecord != Assertion
Assertion != Assessment
Observable != Indicator
Indicator != Sighting
Sighting != Assessment
Relationship != identity
LifecycleEvent != ChangeRecord
IntelligenceConflict != Assessment
ChangeRecord != AuditEvent
AuditEvent != ProcessingRecord
current view != historical record
Infrastructure != IP address
Malware != file hash
Tool != malicious activity
Technique != observed behavior
Vulnerability != vulnerable asset
Report != proof
external identifier != Pathfinder identity
```

## Phase 0.2 Exit Decision

Phase 0.2 is satisfied when Pathfinder accepts the canonical object families above and these governing rules:

1. Every first-class Pathfinder record has Pathfinder-controlled identity.
2. External identifiers remain separate.
3. SourceArtifact owns exact acquired source bytes and preservation digest.
4. SourceRecord identifies a logical item within a SourceArtifact and retains a locator/provenance path.
5. Assertion preserves what a source claimed.
6. External-source judgments remain Assertions.
7. Assessment authority is HUMAN_ANALYST or PATHFINDER_PROCESS.
8. Sighting records observation and does not imply compromise.
9. Relationship is first-class and does not own one universal confidence value.
10. LifecycleEvent and IntelligenceConflict preserve intelligence-state history without rewriting original records.
11. ChangeRecord/ChangeSet, AuditEvent, and ProcessingRecord remain separate history families.
12. A committed ChangeSet is atomic.
13. Current views are derived and do not use hidden newest/highest-confidence/last-write-wins semantics.
14. ThreatActor, Campaign, Malware, Tool, Vulnerability, Technique, Infrastructure, and Report remain explicit concepts rather than overloaded Observables.
15. **The original record is maintained no matter what.**
