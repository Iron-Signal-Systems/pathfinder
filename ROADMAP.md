# Pathfinder Roadmap

Pathfinder is currently pre-release and under active design.

The roadmap intentionally begins with intelligence semantics, provenance, source preservation, trust boundaries, and failure behavior before implementation breadth.

## Phase 0 — Threat Intelligence Foundation

Phase 0 freezes the core meaning of Pathfinder before production implementation begins.

### 0.1 Define the Intelligence Mission and Consumers

Define what Pathfinder is responsible for and who consumes its output.

Initial consumer classes may include:

```text
human analysts
Pathfinder query/API clients
Atlas
Stronghold
FI
future approved ISS integrations
```

Exit criteria:

- Pathfinder product authority is explicit;
- consumer boundaries are explicit; and
- intelligence does not silently become enforcement authority.

### 0.2 Define the Core Intelligence Object Model

Freeze the initial meaning and ownership of objects such as:

```text
Source
Source Record
Observable
Indicator
Sighting
Assessment
Relationship
Threat Actor
Campaign
Malware
Tool
Vulnerability
Technique
Infrastructure
Report
```

Exit criteria:

- object identity is defined;
- required fields are defined;
- lifecycle ownership is defined; and
- object types do not silently imply one another.

### 0.3 Define the Observable Model

Define supported observable classes and normalization rules.

Initial candidates include:

```text
IPv4
IPv6
domain
URL
SHA-256
email address
certificate fingerprint
```

Exit criteria:

- canonical representation rules are frozen;
- malformed and unsupported inputs remain distinguishable; and
- an observable remains semantically neutral unless separately assessed.

### 0.4 Define the Assertion and Assessment Model

Define how Pathfinder records assertions and conclusions.

Preserve distinctions among:

```text
source assertion
machine-derived assessment
human analyst assessment
corroborated assessment
conflicting assessment
superseding assessment
```

Exit criteria:

- assessment authority is attributable;
- historical assessments are preserved; and
- changed understanding does not rewrite prior understanding.

### 0.5 Define the Source and Provenance Model

Freeze the provenance fields needed to work backward from a Pathfinder conclusion to its source.

Candidate provenance includes:

```text
provider
feed / collection
source record identifier
source publication time
source observation time
retrieval time
receipt time
source location
source marking
source format
source hash
parser version
normalizer version
processing time
```

Exit criteria:

- source identity is defined;
- processing lineage is defined; and
- provenance survives normalization and deduplication.

### 0.6 Define Confidence, Reliability, and Corroboration

Define separate semantics for:

```text
source reliability
information confidence
analyst confidence
corroboration
age
recency
independent-source count
```

Exit criteria:

- Pathfinder does not rely on one unexplained risk score;
- duplicated upstream intelligence cannot inflate independent corroboration; and
- confidence never silently becomes authorization.

### 0.7 Define the Relationship Model

Freeze relationship identity, directionality, time bounds, provenance, and confidence semantics.

Examples include:

```text
uses
associated_with
communicates_with
resolves_to
hosts
targets
implements
possibly_related
shares_infrastructure_with
```

Exit criteria:

- correlation remains distinguishable from identity;
- direct source relationships remain distinguishable from derived relationships; and
- time-dependent relationships do not become timeless facts.

### 0.8 Define the Sighting Model

Define what constitutes a sighting and what a sighting does not prove.

Exit criteria:

- source system is attributable;
- observation time is explicit;
- sighting provenance is preserved; and
- a sighting does not automatically establish compromise, malicious intent, or attribution.

### 0.9 Define Intelligence Aging, Expiration, Revocation, and Supersession

Freeze lifecycle rules for current and historical intelligence.

Candidate states include:

```text
active
aging
expired
revoked
superseded
disputed
conflicted
```

Exit criteria:

- expiration meaning is explicit;
- expiration does not imply benign status;
- revocation does not erase history; and
- historical intelligence remains available according to retention policy.

### 0.10 Define Conflicting-Intelligence Behavior

Define how Pathfinder represents disagreement between sources and assessments.

Exit criteria:

- conflicting assertions may coexist;
- conflict remains attributable;
- disagreement is not hidden behind averaging; and
- human review can be requested without manufacturing a synthetic conclusion.

### 0.11 Define the Raw-Source Preservation Contract

Define which source records must be preserved, in what representation, with what integrity metadata, and for how long.

Exit criteria:

- preservation occurs before destructive transformation where required;
- raw source and derived intelligence are separate data classes;
- parser upgrades cannot rewrite original source history; and
- retention/destruction authority is explicit.

### 0.12 Define the STIX 2.1 Interoperability Boundary

Define import/export behavior without making STIX the Pathfinder internal truth model.

Exit criteria:

- external identifiers are handled explicitly;
- translation loss is visible;
- Pathfinder provenance is not silently discarded; and
- syntactically valid STIX is not equated with trusted intelligence.

### 0.13 Define the TAXII 2.1 Interoperability Boundary

Define client/server transport behavior, authentication, source identity, acceptance, retry, and failure semantics.

Exit criteria:

- transport success remains distinct from intelligence acceptance;
- partial retrieval and rate limiting are explicit states; and
- source authentication does not automatically establish information truth.

### 0.14 Define the MITRE ATT&CK Relationship Model

Define how Pathfinder records source-reported, analyst-assigned, and locally observed ATT&CK relationships.

Exit criteria:

- technique association remains distinct from local technique observation;
- mappings preserve provenance; and
- mappings are not manufactured merely to increase coverage.

### 0.15 Define API Trust and Security Boundaries

Define human, service, source, publication, integration, and administrative authority.

Exit criteria:

- authentication and authorization remain separate;
- read, assess, publish, administer, export, and future integration authorities are independently definable; and
- no credential silently becomes universal authority.

### 0.16 Define ISS Product Integration Boundaries

Freeze the authority relationship between Pathfinder and other ISS systems.

Initial direction:

```text
Atlas
    authoritative asset/environment context

Stronghold
    authoritative network observations and decisions

FI
    authoritative file observations

Pathfinder
    authoritative Pathfinder intelligence records,
    assessments, relationships, provenance, and lifecycle
```

Exit criteria:

- integrations exchange explicitly defined records;
- correlation remains derived where applicable; and
- Pathfinder cannot silently take control of another product's authority.

### 0.17 Define Analyst Override and Review Behavior

Define how analysts dispute, annotate, supersede, confirm, or reject derived assessments.

Exit criteria:

- analyst identity/authority is preserved;
- machine and human assessments remain distinguishable;
- historical assessment state remains available; and
- override does not silently alter preserved source material.

### 0.18 Define Audit and Engineering Completeness Requirements

Define the minimum processing/audit history required to answer:

> **When this fails at 2:00 AM, will Pathfinder tell the operator exactly what it received, where it came from, what it could validate, what it understood, how it reached that interpretation, what remains uncertain, and what failed?**

Exit criteria:

- source receipt and processing state are reconstructable;
- failures and retries remain visible;
- index/search completeness is knowable; and
- downstream recommendations remain distinguishable from downstream actions.

## Phase 0 Exit Gate

Phase 0 is complete only when the project has frozen enough contracts to implement a narrow first system without guessing at the meaning of its records.

At minimum, the gate should establish:

```text
product authority
object model
observable model
source/provenance model
assessment model
relationship model
sighting model
confidence/reliability model
lifecycle model
conflict model
source-preservation contract
STIX/TAXII boundaries
ATT&CK boundary
security/authorization boundary
ISS integration boundary
failure/audit requirements
```

Phase 0 should not be considered complete merely because documentation exists. The contracts must be internally consistent and reviewable against representative source and correlation examples.

## Phase 1 — Minimal Intelligence Core

Phase 1 implements the smallest complete Pathfinder system that can prove the Phase 0 model.

### 1.1 Runtime and Repository Foundation

Establish the supported runtime, repository layout, configuration model, service identity expectations, dependency policy, local state paths, logging boundaries, and validation entry point before broad feature implementation.

Initial implementation direction:

```text
Go
PostgreSQL
minimal justified dependencies
explicit configuration
repository-owned validation
```

### 1.2 PostgreSQL Schema

Implement relational storage for the frozen Phase 0 model.

Relationships remain first-class even though the initial implementation is relational.

Do not introduce graph infrastructure unless real Pathfinder workloads demonstrate a requirement.

### 1.3 Source Preservation

Implement the Phase 0 raw-source preservation contract for the first supported source.

### 1.4 First External Collector

Implement one external intelligence source end to end.

The objective is not feed count. The objective is to prove:

```text
receive
preserve
validate
parse
normalize
commit
query
failure handling
provenance
```

### 1.5 Manual Analyst Entry

Provide a narrow mechanism for authorized manual analyst entry so the model is not dependent on external feed semantics.

### 1.6 Observable Normalization

Implement the first frozen observable classes and canonicalization rules.

### 1.7 Relationships

Implement direct and derived relationship storage with provenance and time semantics.

### 1.8 Sightings

Implement sightings independently from indicators and assessments.

### 1.9 Assessments and Conflict Preservation

Implement source, machine, and analyst assessment distinctions required by the Phase 0 contract.

Conflicting assessments must remain visible.

### 1.10 Lifecycle Processing

Implement initial aging, expiration, revocation, supersession, and dispute behavior according to the frozen lifecycle contract.

### 1.11 Basic Query API

Implement a narrow query API sufficient to inspect:

```text
observables
indicators
sources
source records
sightings
assessments
relationships
provenance
processing state
lifecycle state
```

A `no results` response must not imply complete search coverage when processing or indexing is incomplete.

### 1.12 Initial Validation

Create repository-owned tests and validation covering both success and important failure paths.

Initial failure cases should include, where applicable:

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
```

## Phase 1 Exit Gate

Phase 1 should demonstrate a complete narrow path:

```text
one source
   ↓
preserved source record
   ↓
validated / parsed
   ↓
normalized Pathfinder objects
   ↓
relationships / assessments / sightings
   ↓
persisted provenance
   ↓
queryable result
```

The operator must be able to trace a result backward to its source and determine what was source-provided, directly observed, derived, uncertain, conflicted, failed, or not established.

## Explicit Early Deferrals

The following are intentionally not early implementation requirements:

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

These may be evaluated later only when the intelligence core is proven and a concrete requirement justifies them.

## Longer-Term Direction

Potential later capabilities include:

```text
additional high-quality source classes
STIX import/export
TAXII client/server interoperability
ATT&CK mappings
controlled enrichment
analyst workflow
historical reprocessing
ISS-product correlation
candidate detections
candidate enforcement recommendations
operational relevance analysis
```

Later-phase ordering is intentionally not frozen here.

The core remains:

> **Preserve the source, preserve uncertainty, preserve provenance, and do not turn intelligence into authority by accident.**
