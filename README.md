# Pathfinder

**Pathfinder by Iron Signal Systems**

Pathfinder is a threat-intelligence system focused on preserving source truth, separating observation from assertion and assessment, and turning external and internal threat information into operationally useful intelligence without manufacturing certainty.

Pathfinder is pre-release and under active development.

## Mission

Pathfinder is intended to answer four practical questions:

1. **What do we know about a threat?**
2. **Where did that knowledge come from?**
3. **How confident are we in the interpretation?**
4. **Does it matter to systems we actually operate?**

Pathfinder is not intended to become a large undifferentiated IOC bucket.

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

That means Pathfinder is authoritative for the records and interpretations it creates and maintains. It is not automatically authoritative for the real-world truth of every external assertion.

## Core Invariants

> **Preserve what was received before deciding what it means.**

> **Record the claim before judging the claim.**

> **A sighting records an observation. It does not silently become a conclusion.**

> **Correlation is not identity.**

> **Confidence describes a judgment. Reliability describes a source. Corroboration describes support. They are not interchangeable.**

> **The original record is maintained no matter what.**

> **High confidence is still not authorization.**

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

## Three Categories of Truth

Pathfinder preserves three distinct categories:

```text
WHAT THE SOURCE SAID
    preserved source material / attributable Assertion

WHAT WAS OBSERVED
    Sightings and observations from an identified authority

WHAT PATHFINDER UNDERSTOOD
    normalized, correlated, enriched, or assessed intelligence
```

These categories must remain distinguishable.

Current knowledge must not rewrite historical knowledge.

## Canonical Object Families

Phase 0 defines four major record families.

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

The complete reconciled catalog and precedence rules are defined in [`docs/PHASE-0-RECONCILIATION-EXIT.md`](docs/PHASE-0-RECONCILIATION-EXIT.md).

## Source Preservation

The canonical source path is:

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

A `SourceArtifact` preserves the exact immutable acquired payload at the defined application-content boundary.

A `SourceRecord` is a logical item represented within that artifact.

For example:

```text
SourceArtifact
    one TAXII response containing 200 STIX objects

SourceRecords
    logical STIX object 1
    logical STIX object 2
    ...
    logical STIX object 200
```

Therefore:

```text
SourceArtifact != SourceRecord
SourceRecord   != Assertion
Assertion      != Assessment
```

## Intelligence Model

### Assertion

An `Assertion` is an attributable claim.

```text
Vendor A reports that 203.0.113.17 is C2 infrastructure.
```

Pathfinder can authoritatively record that Vendor A made the claim without pretending the claim is objectively proven.

### Observable

An `Observable` is a neutral canonical value that can be referenced, matched, or observed.

Initial types include:

```text
IPv4
IPv6
domain
URL
SHA-256
email
X.509 certificate SHA-256 fingerprint
```

An Observable is not inherently malicious.

### Indicator

An `Indicator` represents threat significance supported by attributable Assertions, Assessments, and provenance.

It is not an Observable with `malicious=true`.

### Sighting

A `Sighting` records an observation by an identified authority.

```text
sighting != compromise
sighting != malicious intent
sighting != attribution
```

### Assessment

An `Assessment` is a Pathfinder-recorded judgment authored by either:

```text
HUMAN_ANALYST
PATHFINDER_PROCESS
```

External-source judgments remain Assertions.

Assessment confidence belongs to the Assessment that expressed it.

### Relationship

A `Relationship` is a first-class intelligence connection governed by a versioned relationship registry.

Relationship confidence is expressed through attributable Assertions and Assessments rather than one mutable universal confidence field.

```text
relationship != identity
relationship != ownership
association  != attribution
```

## Permanent History and Git-Style Change History

Pathfinder uses append-only historical semantics.

> **The original record is maintained no matter what.**

An incorrect, revoked, superseded, disputed, conflicted, or later-invalidated record remains historical.

Corrections move forward through new records.

Material analyst and governed configuration changes use Git-style concepts:

```text
ChangeRecord
ChangeSet
log
show
diff
revert by new forward-moving change
```

A committed `ChangeSet` is atomic:

```text
all semantic changes commit
    or
none commit
```

There is no normal `reset --hard` or force-push equivalent for authoritative intelligence history.

Current-state views are rebuildable. Original historical records are not.

## Current Interpretation

Pathfinder does not use hidden last-write-wins behavior.

No Assessment becomes the singular current interpretation merely because it is newest, highest confidence, human-authored, machine-authored, or supported by the greatest raw record count.

A singular current interpretation requires an explicit semantic mechanism such as valid supersession, an authorized conflict resolution, or another versioned current-view selection policy.

If incompatible applicable Assessments remain unresolved, the current view remains explicitly conflicted.

## Confidence, Reliability, and Corroboration

Pathfinder does not reduce intelligence quality to one unexplained score.

It keeps separate:

```text
source reliability
source-reported confidence
Pathfinder Assessment confidence
analyst confidence
corroboration
age / recency
operational relevance
```

Source reliability is itself a historical Pathfinder Assessment of a Source or SourceCollection, not one mutable truth field on the source.

Repeated delivery of one upstream report does not become independent corroboration.

## Conflicting Intelligence

Conflict is first-class through `IntelligenceConflict`.

Pathfinder does not resolve disagreement using universal rules such as:

```text
majority wins
highest confidence wins
highest reliability wins
newest report wins
```

Conflicting intelligence remains visible and attributable until Pathfinder has a defensible, attributable basis to resolve it.

Resolving a conflict does not delete the opposing records.

## Lifecycle

Initial lifecycle states are:

```text
ACTIVE
AGING
EXPIRED
REVOKED
SUPERSEDED
DISPUTED
CONFLICTED
```

Lifecycle describes current operational applicability, not objective truth.

```text
expired    != benign
revoked    != never existed
superseded != deleted
```

Retention and destruction authority are separate.

## STIX and TAXII

Pathfinder supports STIX 2.1 and TAXII 2.1 at defined interoperability boundaries.

```text
STIX / TAXII
      ↓
boundary adapters
      ↓
Pathfinder-native model
```

STIX does not define Pathfinder's internal schema or truth model.

```text
valid STIX != trusted intelligence
successful TAXII transport != intelligence accepted
```

Translation loss, partial processing, unsupported semantics, pagination state, source preservation, and checkpoint safety remain explicit.

## MITRE ATT&CK

ATT&CK is optional classification/reference metadata.

> **ATT&CK is a classification aid, not a prerequisite for understanding or proving a compromise.**

Pathfinder never requires activity to fit an ATT&CK technique, tactic, or attack sequence before preserving, assessing, prioritizing, or escalating it.

A valid Pathfinder assessment may be:

```text
Threat assessment: HIGH confidence
Operational relevance: HIGH
ATT&CK: NOT_MAPPED
```

with no downgrade.

Technique overlap is not attribution.

## ISS Product Boundaries

```text
Atlas
    authoritative asset/environment context

FI
    authoritative file and file-system observations

Stronghold
    authoritative network observations and network enforcement decisions

Pathfinder
    organizational threat-intelligence record and interpretation

Guidon
    authoritative backup/recovery state
```

> **Integration shares information. It does not transfer domain authority.**

ISS products integrate through defined interfaces/contracts rather than direct cross-product database writes.

```text
Stronghold observation != Pathfinder conclusion
FI hash match          != compromise
Atlas context          != Pathfinder asset authority
Pathfinder candidate   != downstream command
```

## Security and API Boundaries

Authentication, authorization, product authority, analyst authority, administration, export, and enforcement remain distinct.

Machine-to-machine integration direction prefers mTLS with narrow identities and least privilege.

A valid certificate establishes authenticated identity under the configured trust contract. It does not establish intelligence truth.

Raw-source access is separately authorizable from normalized-intelligence access.

Sensitive writes, exports, and administrative actions fail closed when required authority cannot be established.

## Failure and Completeness

Pathfinder never reports more completeness than it can establish.

```text
PARTIAL != SUCCESS
no result != no record exists
incomplete index != complete intelligence history
integration unavailable != no observations occurred
```

State dimensions are typed rather than collapsed into one generic status. Phase 0 distinguishes operation result, preservation, integrity, availability, processing, health, coverage, index, lifecycle, review, and conflict state.

## Storage Direction

Initial storage direction is PostgreSQL and a relational model.

Relationships remain first-class even when represented relationally.

Graph infrastructure is deferred until measured Pathfinder workloads demonstrate a concrete need.

## Phase 0 Status

Phase 0 design and reconciliation are complete.

```text
PHASE 0
    COMPLETE

EXIT GATE
    PASS

NEXT
    Phase 1.1 — Runtime and Repository Foundation
```

The Phase 0 exit review is authoritative for reconciled semantics:

[`docs/PHASE-0-RECONCILIATION-EXIT.md`](docs/PHASE-0-RECONCILIATION-EXIT.md)

## Phase 0 Contract Index

```text
0.1  docs/INTELLIGENCE-MISSION.md
0.2  docs/CORE-INTELLIGENCE-MODEL.md
0.3  docs/OBSERVABLE-MODEL.md
0.4  docs/ASSERTION-ASSESSMENT-MODEL.md
0.5  docs/SOURCE-PROVENANCE-MODEL.md
0.6  docs/CONFIDENCE-RELIABILITY-CORROBORATION.md
0.7  docs/RELATIONSHIP-MODEL.md
0.8  docs/SIGHTING-MODEL.md
0.9  docs/INTELLIGENCE-LIFECYCLE-MODEL.md
0.10 docs/CONFLICTING-INTELLIGENCE.md
0.11 docs/RAW-SOURCE-PRESERVATION.md
0.12 docs/STIX-2.1-INTEROPERABILITY.md
0.13 docs/TAXII-2.1-INTEROPERABILITY.md
0.14 docs/MITRE-ATTACK-RELATIONSHIP-MODEL.md
0.15 docs/API-TRUST-SECURITY-BOUNDARIES.md
0.16 docs/ISS-PRODUCT-INTEGRATION-BOUNDARIES.md
0.17 docs/ANALYST-OVERRIDE-REVIEW-CHANGE-HISTORY.md
0.18 docs/AUDIT-ENGINEERING-COMPLETENESS.md
EXIT docs/PHASE-0-RECONCILIATION-EXIT.md
```

## Phase 1 Direction

Phase 1 begins with infrastructure and runtime groundwork before broad feature implementation.

```text
1.1 Runtime and Repository Foundation
1.2 Minimal Relational Schema for the First Vertical Slice
1.3 Source Preservation
1.4 First External Collector
1.5 Manual Analyst Entry
1.6 Observable Normalization
1.7 Relationships
1.8 Sightings
1.9 Assessments and Conflict Preservation
1.10 Lifecycle Processing
1.11 Basic Query API
1.12 Initial Validation
```

Phase 1.2 does not require implementing tables for every conceptual object immediately. Concrete object schemas are added as the first vertical slice requires them.

## Early Deferrals

Pathfinder intentionally defers:

```text
dozen-plus feed ingestion
generic plugin ecosystems
full TIP feature parity
complex graph infrastructure
AI analyst replacement
autonomous enforcement
SOAR orchestration
large-scale enrichment farms
polished UI
broad multi-product control
```

## Engineering Direction

Pathfinder follows the Iron Signal Systems engineering philosophy:

```text
explicit boundaries
truthful state reporting
minimal justified dependencies
narrow inspectable implementations
failure behavior designed with the success path
no manufactured certainty
no hidden authority transfer
permanent original history
```

See [`AGENTS.md`](AGENTS.md) for repository-wide contributor rules and [`ROADMAP.md`](ROADMAP.md) for the implementation sequence.

## Security

See [`SECURITY.md`](SECURITY.md) for vulnerability reporting and project security scope.

## License

Pathfinder is proprietary source-available software, not open-source software.

See [`LICENSE`](LICENSE) for permitted evaluation use and restrictions.

---

> **Preserve the source, preserve the original record, preserve uncertainty, preserve provenance, and never turn intelligence into authority by accident.**
