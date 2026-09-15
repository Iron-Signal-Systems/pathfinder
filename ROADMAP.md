# Pathfinder Roadmap

Pathfinder is pre-release and under active development.

This roadmap was reset on **2026-09-15** after the UniFi UDM-Pro sensor demonstrated that Pathfinder can preserve useful network observations with enough historical context to support threat-intelligence correlation.

The reset does **not** discard Phase 0 or completed Phase 1 work.

It changes the implementation priority from building the object model in isolation to proving Pathfinder's complete operational-intelligence loop:

```text
threat intelligence
        +
actual observations
        |
        v
correlation
        |
        v
applicability
        |
        v
assessment
        |
        v
historical reprocessing
```

## Product Objective

Pathfinder must be able to answer:

> **Given what we know now, what have we actually observed in our environment, does the intelligence apply to those observations, and why?**

The first major implementation exit gate is reached when Pathfinder can preserve an external intelligence assertion, normalize its observables, ingest an attributable sensor observation, create or locate the corresponding Sighting, evaluate applicability, create an explainable Assessment, and later reprocess the historical Sighting when intelligence changes.

## Retained Foundation

The following work remains valid and complete.

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

Phase 1.4 — First External Collector / CISA KEV
    COMPLETE
```

The completed foundation already establishes:

```text
source/provenance model
exact SourceArtifact preservation
SourceRecord identity
Assertions versus Assessments
neutral Observables
Sightings as observations
Relationships
conflict preservation
lifecycle semantics
change/audit/processing history
STIX/TAXII boundaries
ATT&CK boundaries
API trust boundaries
ISS product authority boundaries
PostgreSQL runtime
FreeBSD deployment foundation
one validated external collector
```

None of that is reset.

## Governing Architecture

Current implementation is governed by:

- [`docs/OPERATIONAL-INTELLIGENCE-ARCHITECTURE.md`](docs/OPERATIONAL-INTELLIGENCE-ARCHITECTURE.md)
- [`docs/SENSOR-OBSERVATION-INGESTION.md`](docs/SENSOR-OBSERVATION-INGESTION.md)
- [`docs/CORRELATION-HISTORICAL-REPROCESSING.md`](docs/CORRELATION-HISTORICAL-REPROCESSING.md)

Existing Phase 0 documents remain semantic contracts unless explicitly reconciled by a later committed contract.

## Phase 1 — Operational Intelligence Core

Phase 1 now exists to prove one complete, trustworthy, explainable correlation path.

### 1.5 Observable Normalization — CURRENT

Implement the first frozen observable classes and the versioned `pathfinder-observable-v1` canonicalization behavior.

Initial priority for the operational-correlation path:

```text
IPv4
IPv6
domain
URL
SHA-256
email
X.509 certificate SHA-256 fingerprint
```

Normalization must be deterministic and pure.

```text
normalize representation
    !=
invent meaning
```

Exit criteria include:

```text
same valid value resolves consistently
malformed input remains malformed
unsupported input remains unsupported
normalization preserves provenance
Observable remains neutral
normalization does not create an Indicator
```

### 1.6 Relationships

Implement the versioned relationship registry, endpoint validation, time behavior, provenance, and origin separation required by the existing relationship contracts.

The first slice must support relationships needed for operational correlation without creating broad speculative graph semantics.

Priority relationship behavior includes:

```text
communicates_with
resolves_to
associated_with
possibly_related
shares_infrastructure_with
uses
```

No relationship becomes ownership, identity, attribution, or maliciousness by implication.

### 1.7 Sightings

Implement point and permitted bounded-aggregate Sightings independently from Indicators and Assessments.

A Sighting must preserve:

```text
observer / observation authority
observed Observable
event time
receipt time
observation context
source observation reference
coverage context where applicable
provenance
```

A Sighting records an observation.

```text
Sighting != malicious
Sighting != compromise
Sighting != attribution
```

### 1.8 Sensor Ingestion Contract

Define and implement the authenticated sensor-to-Pathfinder delivery boundary.

The contract must establish:

```text
sensor identity
schema version
batch identity
record identity
event time
receipt time
idempotent delivery
replay behavior
acknowledgement semantics
partial/failure behavior
coverage/health metadata
original observation preservation
processing lineage
```

Machine-to-machine direction prefers mTLS with narrow sensor identities.

A valid sensor identity establishes who submitted the observation. It does not prove the observation's interpretation.

See [`docs/SENSOR-OBSERVATION-INGESTION.md`](docs/SENSOR-OBSERVATION-INGESTION.md).

### 1.9 Sensor Observation Store

Implement the server-side store optimized for high-volume sensor observations.

Initial observation classes:

```text
sensor identity
endpoint identity
IPv4 / IPv6 communication
protocol
source/destination port
interface/vantage
DNS observation
TLS SNI observation
NAT/path correlation
conntrack context
event time
sensor health
coverage state
```

The sensor observation store remains semantically separate from the threat-intelligence source store.

Normal historical operator queries should target this indexed store rather than repeatedly scanning compressed sensor-local rings.

The sensor-local ring remains valuable for:

```text
delivery resilience
short-term source record
sensor diagnostics
collector validation
troubleshooting
reconciliation after delivery failure
```

### 1.10 High-Context Network Intelligence Source

Add one additional threat source capable of contributing network-relevant context beyond vulnerability catalog data.

Selection criteria favor:

```text
clear original producer/provenance
domain/IP/URL/certificate observables
infrastructure role where supplied
campaign or malware relationships where supplied
validity/time context where supplied
affected platform/product context where supplied
source confidence where supplied
revocation/update semantics
stable acquisition behavior
```

Do **not** optimize this phase for feed count.

A large contextless bad-IP list is not the target.

The objective is to prove that Pathfinder can preserve and use enough context to distinguish:

```text
observable matched
    from
this historical Sighting is actually applicable to the reported threat context
```

### 1.11 Correlation Engine

Implement correlation between normalized threat intelligence and sensor-origin Sightings.

The first engine must support exact or explicitly versioned matching for applicable observable types.

Correlation output must preserve:

```text
what matched
which records participated
which algorithm/version produced the match
when correlation ran
what was not established
whether source/search/index coverage was complete
```

Correlation alone does not create maliciousness.

```text
Observable match != malicious Sighting
IOC match        != compromise
IP match         != C2 session
domain match     != campaign participation
```

### 1.12 Applicability and Assessment

Implement explicit applicability evaluation and machine/human Assessment behavior.

Applicable dimensions may include, where the intelligence actually supplies them:

```text
time window
platform
product/software
version
campaign
malware family
infrastructure role
geography
service/technology
source confidence
conflicting intelligence
observed behavior
```

Each dimension must distinguish:

```text
MATCH
NO_MATCH
UNKNOWN
NOT_APPLICABLE
CONFLICTED
```

Unknown information must not silently become a match or a non-match.

External-source judgments remain Assertions.

Pathfinder-generated judgments become `Assessment` records authored by `PATHFINDER_PROCESS`.

### 1.13 Historical Reprocessing

Implement forward-moving reevaluation of historical Sightings when threat intelligence changes.

Triggers may include:

```text
new Assertion
new Observable
new Relationship
changed applicability window
new affected platform
revocation
supersession
conflict
source correction
parser/normalizer upgrade where reprocessing is authorized
```

Reprocessing must:

```text
preserve the original source record
preserve the original sensor observation
preserve the original Sighting
preserve prior Assessments
record the reprocessing version/time
create new derived state or Assessment where justified
make changed interpretation explainable
```

It must never make later knowledge appear to have existed earlier.

See [`docs/CORRELATION-HISTORICAL-REPROCESSING.md`](docs/CORRELATION-HISTORICAL-REPROCESSING.md).

### 1.14 Query API

Implement narrow queries that expose the complete vertical slice.

The operator must be able to ask:

```text
What does intelligence say about this Observable?

Where did that claim come from?

Have our sensors ever observed it?

Which endpoints were involved?

What did we observe at the time?

Which threat relationships match?

Does the intelligence apply to this Sighting?

What is Pathfinder's Assessment?

Why?

What changed between the earlier and current Assessment?

Is source, sensor, processing, and index coverage complete?
```

A `no results` response must not imply complete coverage when coverage is incomplete.

### 1.15 Validation and Fresh-Install Exit Gate

Repository-owned validation must cover the entire operational-intelligence path and important failure behavior.

Required categories include:

```text
source unavailable
sensor unavailable
authentication failure
duplicate delivery
replay
partial batch
malformed source input
malformed sensor input
normalization failure
database unavailable
transaction failure
index incomplete
coverage unknown
conflicting intelligence
expired/revoked intelligence
interrupted correlation
interrupted reprocessing
stale derived view
artifact integrity mismatch
observation integrity mismatch
```

Phase 1 must still satisfy the clean-host installation and full reboot-persistence requirements already defined for the project.

## Phase 1 Acceptance Scenario

The canonical test is synthetic and must not depend on a real malicious event.

Example:

```text
Historical sensor Sighting
--------------------------
endpoint: Living-Room-Test
platform: tvOS
time:     2026-09-14T22:25:43Z
SNI:      test-stream.example
dst:      203.0.113.44:443

Initial threat Assertion
------------------------
observable:          test-stream.example
role:                command_and_control
campaign:            Pathfinder-Test-Campaign
affected platform:   Roku
valid from:          2026-09-20
```

Expected first evaluation:

```text
observable match:        MATCH
historical sighting:     MATCH
platform applicability:  NO_MATCH
time applicability:      NO_MATCH

result:
    intelligence intersects the Observable,
    but the Assertion does not apply to this historical Sighting
```

Then ingest a new source record:

```text
affected platforms:
    Roku
    tvOS

valid from:
    2026-09-01
```

Expected historical reprocessing:

```text
observable match:        MATCH
platform applicability:  MATCH
time applicability:      MATCH

result:
    create a new attributable Assessment
    flag historical activity for review
```

The original sensor observation, original Sighting, original Assertion, and first Assessment remain unchanged.

If Pathfinder can perform this end to end and explain every step, the core product loop is real.

## Phase 2 — Analyst Workflow and Source Breadth

After Phase 1 proves the operational loop, Phase 2 may expand:

```text
authorized manual analyst entry
additional high-quality threat sources
STIX import/export implementation
TAXII client/server implementation
richer analyst review
conflict resolution workflow
source reliability assessment workflow
publication/export workflow
bulk historical hunting
cross-sensor correlation
controlled enrichment
```

Manual entry is intentionally moved behind the core machine-verifiable operational loop. It remains important, but it is no longer the next implementation priority.

## Phase 3 — ISS and External Integration

Potential later work:

```text
Atlas relevance integration
FI Sighting integration
Stronghold Sighting/PCAP linkage
candidate detections
candidate enforcement recommendations
third-party SIEM integration
external API consumers
reporting/export integrations
```

Integration does not transfer authority.

## Explicit Deferrals

Pathfinder intentionally defers until a measured requirement exists:

```text
dozen-plus feed ingestion
generic plugin ecosystems
full SIEM functionality
full TIP feature parity
complex graph infrastructure
AI analyst replacement
autonomous enforcement
SOAR orchestration
large-scale enrichment farms
polished UI
broad multi-product control
general-purpose log ingestion
```

## SIEM Boundary

Pathfinder may retain observation data required to make threat intelligence operational.

It does not become the organization's general-purpose event lake.

```text
Pathfinder should ingest:
    threat-relevant observations with defined semantics and provenance

Pathfinder should not ingest merely because data exists:
    every Windows log
    every syslog message
    every application log
    generic infrastructure metrics
```

The sensor observation plane exists to answer threat-intelligence questions.

## Final Engineering Rule

> **Preserve the source, preserve the observation, preserve history, preserve uncertainty, and make every changed interpretation explainable.**
