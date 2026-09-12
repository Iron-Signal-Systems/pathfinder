# Pathfinder Roadmap

Pathfinder is pre-release and under active development.

The roadmap intentionally begins with intelligence semantics, provenance, source preservation, trust boundaries, failure behavior, and historical integrity before implementation breadth.

## Current Status

```text
Phase 0 — Threat Intelligence Foundation
    COMPLETE

Phase 0 Reconciliation / Exit Review
    COMPLETE

Phase 0 Exit Gate
    PASS

Phase 1.1 — Runtime and Repository Foundation
    COMPLETE

Phase 1.2 — Source Foundation / Minimal Relational Schema
    COMPLETE

Phase 1.3 — Source Preservation
    COMPLETE

Current work
    Phase 1.4 — First External Collector
```

The governing Phase 0 reconciliation document is:

[`docs/PHASE-0-RECONCILIATION-EXIT.md`](docs/PHASE-0-RECONCILIATION-EXIT.md)

The completed Phase 1.1 runtime foundation is recorded in:

[`docs/PHASE-1.1-PLATFORM-FOUNDATION.md`](docs/PHASE-1.1-PLATFORM-FOUNDATION.md)

The completed Phase 1.2 source foundation is recorded in:

[`docs/PHASE-1.2-SOURCE-FOUNDATION.md`](docs/PHASE-1.2-SOURCE-FOUNDATION.md)

Phase 1 fresh-install acceptance is governed by:

[`docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md`](docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md)

Where an earlier Phase 0 description conflicts with a refinement frozen by the reconciliation document, the reconciliation document governs.

---

# Phase 0 — Threat Intelligence Foundation — COMPLETE

Phase 0 froze Pathfinder's core meaning before production implementation.

## 0.1 Intelligence Mission and Consumers

Frozen in [`docs/INTELLIGENCE-MISSION.md`](docs/INTELLIGENCE-MISSION.md).

Key result:

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

Product integration does not transfer authority, and intelligence does not silently become enforcement authority.

## 0.2 Core Intelligence Object Model

Frozen in [`docs/CORE-INTELLIGENCE-MODEL.md`](docs/CORE-INTELLIGENCE-MODEL.md) and refined by later contracts plus the Phase 0 reconciliation.

The reconciled object families are:

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

## 0.3 Observable Model

Frozen in [`docs/OBSERVABLE-MODEL.md`](docs/OBSERVABLE-MODEL.md).

Initial observable classes:

```text
IPv4
IPv6
domain
URL
SHA-256
email
X.509 certificate SHA-256 fingerprint
```

Governing rule:

> **Normalize representation, not meaning.**

## 0.4 Assertion and Assessment Model

Frozen in [`docs/ASSERTION-ASSESSMENT-MODEL.md`](docs/ASSERTION-ASSESSMENT-MODEL.md) and reconciled at the Phase 0 exit gate.

```text
Assertion
    attributable claim

Assessment
    Pathfinder-recorded human or machine judgment
```

External-source judgments remain Assertions. Assessment authorities are `HUMAN_ANALYST` and `PATHFINDER_PROCESS`. Sighting is an allowed Assessment subject where the assessment type permits it.

## 0.5 Source and Provenance Model

Frozen in [`docs/SOURCE-PROVENANCE-MODEL.md`](docs/SOURCE-PROVENANCE-MODEL.md) and refined by Phase 0.11.

Canonical chain:

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
```

`SourceArtifact` owns exact preserved acquired bytes. `SourceRecord` is the logical source item inside or represented by that artifact.

## 0.6 Confidence, Reliability, and Corroboration

Frozen in [`docs/CONFIDENCE-RELIABILITY-CORROBORATION.md`](docs/CONFIDENCE-RELIABILITY-CORROBORATION.md).

> **Confidence describes a judgment. Reliability describes a source. Corroboration describes support. They are not interchangeable.**

Source reliability is represented as historical Assessment rather than one mutable source truth field.

## 0.7 Relationship Model

Frozen in [`docs/RELATIONSHIP-MODEL.md`](docs/RELATIONSHIP-MODEL.md) and refined by ATT&CK/reconciliation.

Initial relationship vocabulary includes:

```text
uses
communicates_with
resolves_to
implements
exploits
associated_with
possibly_related
shares_infrastructure_with
```

`uses` additionally permits the reconciled ATT&CK directions:

```text
ThreatActor -> Technique
Campaign    -> Technique
```

The following remain deferred until their semantics are explicitly defined:

```text
owns
controls
same_as
targets
hosts
attributed_to
originates_from
compromised_by
```

## 0.8 Sighting Model

Frozen in [`docs/SIGHTING-MODEL.md`](docs/SIGHTING-MODEL.md).

> **A sighting records an observation. It does not silently become a conclusion.**

Negative observation semantics require known coverage and are not manufactured from missing Sightings.

## 0.9 Intelligence Lifecycle

Frozen in [`docs/INTELLIGENCE-LIFECYCLE-MODEL.md`](docs/INTELLIGENCE-LIFECYCLE-MODEL.md).

```text
ACTIVE
AGING
EXPIRED
REVOKED
SUPERSEDED
DISPUTED
CONFLICTED
```

Lifecycle changes current applicability. It does not rewrite what historically existed.

## 0.10 Conflicting Intelligence

Frozen in [`docs/CONFLICTING-INTELLIGENCE.md`](docs/CONFLICTING-INTELLIGENCE.md).

`IntelligenceConflict` is first-class. Pathfinder does not use universal majority-wins, highest-confidence-wins, or newest-record-wins conflict resolution.

## 0.11 Raw-Source Preservation

Frozen in [`docs/RAW-SOURCE-PRESERVATION.md`](docs/RAW-SOURCE-PRESERVATION.md).

> **Preserve what was received before deciding what it means.**

`SourceArtifact` preserves exact acquired bytes before semantic interpretation. Parser or mapper upgrades never replace the original artifact.

## 0.12 STIX 2.1 Interoperability

Frozen in [`docs/STIX-2.1-INTEROPERABILITY.md`](docs/STIX-2.1-INTEROPERABILITY.md).

STIX is an interchange boundary, not Pathfinder's internal truth or storage model.

## 0.13 TAXII 2.1 Interoperability

Frozen in [`docs/TAXII-2.1-INTEROPERABILITY.md`](docs/TAXII-2.1-INTEROPERABILITY.md).

Transport, preservation, STIX validation, Pathfinder mapping, commit, pagination, retry, and checkpoint state remain distinct.

## 0.14 MITRE ATT&CK Relationship Model

Frozen in [`docs/MITRE-ATTACK-RELATIONSHIP-MODEL.md`](docs/MITRE-ATTACK-RELATIONSHIP-MODEL.md).

> **ATT&CK is a classification aid, not a prerequisite for understanding or proving a compromise.**

`NOT_MAPPED` never reduces threat significance, confidence, relevance, or escalation priority.

## 0.15 API Trust and Security Boundaries

Frozen in [`docs/API-TRUST-SECURITY-BOUNDARIES.md`](docs/API-TRUST-SECURITY-BOUNDARIES.md).

Authentication, authorization, analyst authority, product authority, administration, export, and enforcement remain separate.

## 0.16 ISS Product Integration Boundaries

Frozen in [`docs/ISS-PRODUCT-INTEGRATION-BOUNDARIES.md`](docs/ISS-PRODUCT-INTEGRATION-BOUNDARIES.md).

```text
Atlas       -> asset/environment authority
FI          -> file observation authority
Stronghold  -> network observation/enforcement authority
Pathfinder  -> threat-intelligence record/interpretation authority
Guidon      -> backup/recovery authority
```

ISS products integrate through explicit interfaces rather than direct cross-product database writes.

## 0.17 Analyst Override, Review, and Change History

Frozen in [`docs/ANALYST-OVERRIDE-REVIEW-CHANGE-HISTORY.md`](docs/ANALYST-OVERRIDE-REVIEW-CHANGE-HISTORY.md) and tightened by the Phase 0 reconciliation.

> **The original record is maintained no matter what.**

Analyst changes move forward through new attributable records. Git-style `ChangeRecord` and `ChangeSet` history supports reconstructable `log` / `show` / `diff` behavior.

A committed `ChangeSet` is atomic.

There is no normal reset-hard or force-push equivalent for authoritative intelligence history.

## 0.18 Audit and Engineering Completeness

Frozen in [`docs/AUDIT-ENGINEERING-COMPLETENESS.md`](docs/AUDIT-ENGINEERING-COMPLETENESS.md).

> **If Pathfinder cannot prove that an operation completed, it must not report the operation as complete.**

> **If Pathfinder cannot establish coverage, absence must remain unknown rather than becoming a negative conclusion.**

Derived indexes and current views are rebuildable. Original historical records are not.

## Phase 0 Exit Reconciliation

Frozen in [`docs/PHASE-0-RECONCILIATION-EXIT.md`](docs/PHASE-0-RECONCILIATION-EXIT.md).

The reconciliation resolves late-stage refinements without erasing the earlier design history. It freezes:

```text
canonical object families
SourceArtifact / SourceRecord ownership
permanent historical-record behavior
Assessment authorities and Sighting assessment support
current-view selection semantics
ATT&CK / Relationship registry interaction
typed state namespaces
atomic committed ChangeSets
ChangeRecord / AuditEvent / ProcessingRecord separation
Source reliability representation
Phase 1 schema scope
```

Phase 0 exit gate result:

```text
PASS
```

---

# Phase 1 — Minimal Intelligence Core

Phase 1 implements the smallest complete Pathfinder system capable of proving the Phase 0 contracts.

The objective is not feature count. The objective is one trustworthy vertical slice.

## 1.1 Runtime and Repository Foundation — COMPLETE

Phase 1.1 established and validated the environment in which Pathfinder lives.

Completed groundwork includes:

```text
supported FreeBSD environment
native VNET jail boundary
host-owned ZFS and PF authority
PostgreSQL 18 runtime
separate runtime and migration DB authority
service/runtime identity
configuration and secret boundaries
filesystem/state paths
startup/readiness behavior
logging boundary
migration entry point
validation entry point
build/install boundary
rc.d service lifecycle
graceful shutdown
full reboot persistence
runtime build-tool cleanup
```

The closing record is [`docs/PHASE-1.1-PLATFORM-FOUNDATION.md`](docs/PHASE-1.1-PLATFORM-FOUNDATION.md).

### 1.1 Exit Gate

Phase 1.1 result:

```text
PASS — COMPLETE
```

A developer/operator can determine without guessing:

```text
where Pathfinder runs
which dependencies are required
how it is configured
which identity it runs as
where durable/local state belongs
how secrets are referenced
how startup/readiness is determined
how migrations are invoked
how validation is invoked
how the service is built, installed, started, stopped, and restored after reboot
```

## 1.2 Source Foundation / Minimal Relational Schema — COMPLETE

Phase 1.2 implemented the minimum relational source/provenance slice required to move into raw-source preservation:

```text
schema_migration
Source
SourceCollection
RetrievalEvent
```

It also replaced the Phase 1.1 migration preflight stub with the real embedded migration runner, including ordered versions, SHA-256 tracking, advisory locking, dry-run rollback, idempotency, and fail-closed checksum mismatch handling.

The runtime role receives only the DML required by this slice and cannot delete the source records, perform DDL, read migration history, or invoke migrations as the non-root service identity.

The closing record is [`docs/PHASE-1.2-SOURCE-FOUNDATION.md`](docs/PHASE-1.2-SOURCE-FOUNDATION.md).

### 1.2 Exit Gate

Phase 1.2 result:

```text
PASS — COMPLETE
```

Repository-owned validation covers UUIDv7 identity, explicit RetrievalEvent state, valid completion transition, Source/SourceCollection consistency, runtime privileges, migration tamper rejection, and zero test residue.

`SourceArtifact` was intentionally not pulled forward into Phase 1.2. Exact-byte preservation remains the Phase 1.3 responsibility.

## 1.3 Source Preservation — COMPLETE

Implement the Phase 0.11 SourceArtifact preservation contract for the first source.

The exact artifact must be committed before authoritative semantic processing proceeds where the source contract requires preservation.

Phase 1.3 must establish at least:

```text
SourceArtifact identity and retrieval-event ownership
exact acquired byte preservation
SHA-256 integrity over exact bytes
immutable artifact storage semantics
metadata sufficient to locate and verify preserved bytes
safe duplicate-content handling without rewriting provenance
storage-write / database-commit failure behavior
restart/interruption behavior
runtime authority that cannot silently replace preserved bytes
```

The design must continue to distinguish preservation from parsing, validation, normalization, interpretation, and acceptance.

The closing record is [`docs/PHASE-1.3-SOURCE-PRESERVATION.md`](docs/PHASE-1.3-SOURCE-PRESERVATION.md).

### 1.3 Exit Gate

Phase 1.3 result:

```text
PASS — COMPLETE
```

Validated behavior includes exact-byte preservation, separated committed-object authority,
preservation-aware readiness, explicit preserved-but-uncommitted failure semantics,
read-only orphan reconciliation, successful application-owned SourceArtifact commit,
retry-safe `ALREADY_CONFIRMED` behavior, and full post-reboot acceptance.

## 1.4 First External Collector

Implement one external intelligence source end to end.

The proof path is:

```text
retrieve
  ↓
preserve SourceArtifact
  ↓
create SourceRecord
  ↓
record Assertion
  ↓
normalize
  ↓
commit native records
  ↓
query
```

Failure, partial processing, retry, provenance, and checkpoint behavior are part of the implementation—not later polish.

## 1.5 Manual Analyst Entry

Implement a narrow authorized manual-entry path without allowing analyst input to impersonate an external source or rewrite existing history.

## 1.6 Observable Normalization

Implement the first frozen observable classes and `pathfinder-observable-v1` canonicalization behavior.

Normalization must be deterministic and pure.

## 1.7 Relationships

Implement the versioned relationship registry, endpoint validation, time behavior, provenance, and origin separation required by Phase 0.7 and the exit reconciliation.

## 1.8 Sightings

Implement point and permitted bounded-aggregate Sightings independently from Indicators and Assessments.

## 1.9 Assessments and Conflict Preservation

Implement human/machine Assessment separation, source reliability Assessment, Sighting assessment support, `IntelligenceConflict`, and explicit current-view resolution.

No last-write-wins current interpretation.

## 1.10 Lifecycle Processing

Implement initial lifecycle events and current-state derivation without rewriting historical records.

## 1.11 Basic Query API

Implement a narrow query API sufficient to inspect the vertical slice and its history.

Initial query surfaces should support the applicable subset of:

```text
sources
source artifacts
source records
assertions
observables
indicators
sightings
assessments
relationships
conflicts
provenance
processing history
change history
lifecycle
coverage/index state
```

A `no results` response must not imply complete coverage when indexing, processing, authorization, or source coverage is incomplete.

## 1.12 Initial Validation

Repository-owned tests must cover semantic invariants as well as implementation behavior.

Initial applicable failure/recovery cases include:

```text
source unavailable
authentication failure
rate limiting
partial response
malformed input
unsupported input
oversized input
duplicate delivery
parser failure
normalization failure
database unavailable
transaction failure
conflicting intelligence
expired intelligence
interrupted processing
checkpoint failure
index incomplete
artifact integrity mismatch
stale analyst base
```

Critical paths should include restart/interruption testing and selected fault injection.

## Phase 1 Fresh-Install Requirement

Phase 1 completion also requires the system to be reproducible from the clean-host contract in [`docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md`](docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md).

The supported operator entry point is:

```sh
sh install.sh --config /path/to/pathfinder.conf
```

The acceptance starting point is a supported FreeBSD amd64 ZFS host with working networking/DNS, a root account, and one non-root wheel user. Git, Go, PostgreSQL, Pathfinder identities, jail templates, jails, datasets, PF policy, secrets, and Pathfinder services must not be pre-required.

The installer must own required dependency installation, secret generation, construction, validation, and reboot-persistence proof. Until that path is complete and clean-host tested, `install.sh` must fail closed rather than claim a partial installation succeeded.

## Phase 1 Exit Gate

Phase 1 must demonstrate one complete narrow path:

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
normalized Pathfinder object(s)
  ↓
Relationships / Assessments / Sightings
  ↓
provenance + processing/change/audit history
  ↓
queryable result
```

The operator must be able to trace a result backward and distinguish:

```text
source-provided
directly observed
derived
analyst-assessed
machine-assessed
uncertain
conflicted
expired
revoked
superseded
failed
partial
unsupported
not established
```

Phase 1 additionally cannot close until:

```text
install.sh reconstructs the Phase 1 system from the defined clean-host state
repository-owned verification passes on that fresh system
full reboot persistence passes
no undocumented manual construction step is required
```

---

# Explicit Early Deferrals

The following remain intentionally deferred:

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

These may be evaluated only when the intelligence core is proven and a concrete requirement justifies them.

---

# Longer-Term Direction

Potential later capabilities include additional high-quality source classes, STIX import/export, TAXII client/server interoperability, optional ATT&CK mappings, controlled enrichment, richer analyst workflow, historical reprocessing, ISS-product correlation, candidate detections, candidate enforcement recommendations, and operational relevance analysis.

Later-phase ordering remains intentionally unfrozen.

---

> **Preserve the source, preserve the original record, preserve uncertainty, preserve provenance, and never turn intelligence into authority by accident.**
