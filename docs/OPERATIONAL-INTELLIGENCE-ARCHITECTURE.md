# Pathfinder Operational Intelligence Architecture

## Purpose

This document defines Pathfinder's current implementation architecture after validation of the first Pathfinder network sensor.

It reconciles the original threat-intelligence design with a practical requirement that became concrete during sensor development:

> Threat intelligence becomes substantially more useful when Pathfinder can determine whether the organization actually observed the infrastructure, communication, or artifact described by that intelligence.

This document refines implementation direction. It does not erase or replace completed Phase 0 semantic history.

The core semantic rules remain:

```text
What the source said
What was actually observed
What Pathfinder concluded
```

## Product Statement

Pathfinder is a threat-intelligence and operational-correlation system.

It preserves intelligence, preserves attributable observations, correlates the two, evaluates applicability, records assessments, and can re-evaluate historical observations when intelligence changes.

Pathfinder is not a general-purpose SIEM, packet-capture authority, EDR, SOAR platform, or autonomous enforcement system.

## Primary Operational Question

> **Given what we know now, what have we actually observed, does the intelligence apply to those observations, and why?**

Supporting questions include:

```text
What exactly did the source report?

What Observable did Pathfinder normalize from that report?

Has an identified sensor observed that Observable?

Which endpoint was involved?

When did the observation occur?

What network context existed at that time?

What role did the source claim for the infrastructure?

What campaign or malware relationships were reported?

What platform/product scope was reported?

Does the reported time window include the Sighting?

What is known?

What is inferred?

What remains unknown?

Did new intelligence change the current Assessment of old activity?
```

## Three Data Planes

Pathfinder separates three semantic planes.

### Threat Intelligence Plane

This plane preserves and interprets threat-intelligence sources.

```text
Source
SourceCollection
RetrievalEvent
SourceArtifact
SourceRecord
Assertion
Observable
Indicator
ThreatActor
Campaign
Malware
Tool
Vulnerability
Technique
Infrastructure
Report
Relationship
```

The intelligence plane can say:

```text
Source A reports that example.net served as C2 for Campaign X.
```

It cannot, by itself, say that a particular organizational connection to `example.net` was a C2 session.

### Sensor Observation Plane

This plane preserves what identified sensors observed.

Initial network observation context includes:

```text
sensor identity
sensor schema/version
event time
receipt time
endpoint identity
source/destination IP
source/destination port
protocol
interface/vantage
DNS observations
TLS SNI observations
NAT/path relationships
conntrack context
sensor health
coverage state
```

The observation plane can say:

```text
Sensor UDM-A observed endpoint 192.0.2.10
communicating with 203.0.113.20:443
with plaintext TLS SNI example.net
at time T.
```

It cannot, by itself, say:

```text
this was C2
this endpoint was compromised
this campaign succeeded
```

### Interpretation Plane

The interpretation plane connects intelligence and observations without collapsing them.

```text
Sighting
Assessment
IntelligenceConflict
applicability results
correlation results
lifecycle/current interpretation
ProcessingRecord
ChangeRecord / ChangeSet
AuditEvent
```

This plane can say:

```text
The Sighting matched an Observable referenced by Source A.

Source A reported that Observable as C2 for Campaign X.

The reported platform and time window apply to this Sighting.

Pathfinder therefore created Assessment Y with confidence Z.
```

That Assessment remains a Pathfinder judgment.

## Storage Direction

The three planes are semantically separate even if early implementation uses one PostgreSQL deployment.

Conceptually:

```text
threat_intelligence_store
    source/provenance
    Assertions
    Observables
    Indicators
    threat entities
    Relationships

sensor_observation_store
    high-volume attributable observations
    endpoint/time/vantage context
    sensor health and coverage

interpretation_store
    Sightings
    correlations
    applicability
    Assessments
    conflicts
    reprocessing lineage
```

Direct database coupling is not an integration contract.

Derived indexes and current views must be rebuildable.

Original source and observation records are not replaced by derived views.

## Sensor Authority

A Pathfinder sensor is authoritative only for the record of what that identified sensor observed at its defined vantage and under its documented collection semantics.

```text
sensor observed packet metadata
    != authoritative full network truth

sensor observed SNI
    != application identity in all cases

sensor did not observe event
    != event did not occur
```

Sensor health and coverage therefore matter to interpretation.

Stronghold remains authoritative for Stronghold's own network observation and enforcement records.

A Pathfinder network sensor does not inherit Stronghold authority merely because both products observe network activity.

## Observation to Sighting

Raw/high-volume sensor observations and Pathfinder `Sighting` are related but not identical.

The sensor observation store may contain many packet- or flow-level records.

A Sighting is the Pathfinder intelligence object that records the relevant observation of an Observable by an identified observation authority.

Example:

```text
sensor observations
    packet metadata
    TLS ClientHello SNI = example.net
    endpoint identity
    timestamp
        |
        v
Sighting
    Observable: domain example.net
    observer:   UDM-A
    observed:   time T
    context:    endpoint/vantage references
```

Promotion or derivation from sensor observation to Sighting must be versioned, attributable, and repeatable.

A parser/correlation upgrade must not mutate the original observation.

## Threat Source to Assertion

The existing source-preservation chain remains:

```text
Source
  |
  v
SourceCollection
  |
  v
RetrievalEvent
  |
  v
SourceArtifact
  |
  v
SourceRecord
  |
  v
Assertion
```

If an external report states that `example.net` was command-and-control infrastructure, Pathfinder records the claim as an Assertion attributable to the source.

Pathfinder does not convert the claim into an unquestionable fact.

## Correlation

Correlation establishes that records intersect under a defined algorithm.

Examples:

```text
exact normalized domain match
exact IP match
certificate fingerprint match
hash match
explicit source relationship match
```

Correlation must record:

```text
input records
algorithm/profile version
processing time
match type
coverage/index state
uncertainty
```

Correlation does not establish applicability or maliciousness.

## Applicability

Applicability asks whether the intelligence claim can reasonably apply to the specific Sighting.

Potential dimensions, when provided by the source or established by authorized local context, include:

```text
time
platform
product/software
version
campaign
malware family
infrastructure role
geography
service/technology
observed behavior
```

Each dimension must preserve unknowns.

```text
unknown platform
    != platform match

unknown campaign window
    != time match

shared IP
    != C2 connection
```

Applicability is detailed in [`CORRELATION-HISTORICAL-REPROCESSING.md`](CORRELATION-HISTORICAL-REPROCESSING.md).

## Historical Reprocessing

Historical reprocessing is a first-class Pathfinder capability.

When new intelligence arrives:

```text
new source record
    |
    v
new Assertion / Relationship / scope
    |
    v
find relevant historical Sightings
    |
    v
re-evaluate applicability
    |
    v
create new ProcessingRecord
    |
    v
create new Assessment if justified
```

The old Sighting remains unchanged.

The old Assessment remains historical.

The new Assessment must be traceable to the new intelligence and the reprocessing version.

## Why This Is Not a SIEM

Pathfinder's sensor observation store exists to answer intelligence questions, not to become a universal event lake.

In scope:

```text
observations that can create or contextualize Sightings
sensor coverage required to understand those Sightings
network metadata required to evaluate threat relevance
```

Out of scope by default:

```text
generic Windows event collection
generic syslog aggregation
application log warehousing
infrastructure performance monitoring
VPN dashboarding
general SOC alert queues
case-management replacement
```

A future SIEM integration may provide selected observations to Pathfinder or consume Pathfinder intelligence. That does not make Pathfinder a SIEM.

## Failure and Coverage

Pathfinder must never infer absence from missing coverage.

```text
sensor offline
    -> coverage degraded

sensor ingestion delayed
    -> current observation coverage incomplete

index rebuilding
    -> query coverage incomplete

no Sighting found under incomplete coverage
    -> unknown, not negative proof
```

Sensor health and ingestion state are therefore part of the operational intelligence model.

## Security Boundary

Sensor and source ingestion are both untrusted input boundaries.

Requirements include:

```text
narrow authenticated identities
schema validation
size limits
rate controls where appropriate
idempotency/replay handling
durable commit semantics
explicit partial/failure state
no direct cross-product database writes
```

Authentication proves identity under the configured trust contract.

It does not prove semantic truth.

## Implementation Priority

The implementation priority is now:

```text
Observable normalization
Relationships
Sightings
sensor ingestion
sensor observation store
one high-context network threat source
correlation
applicability
Assessment
historical reprocessing
query
validation
```

Broad feed count, generic enrichment, polished UI, and general log ingestion are intentionally secondary.

## Acceptance Principle

Pathfinder's operational intelligence core is proven when it can:

```text
preserve a threat source
preserve an attributable sensor observation
normalize the common Observable
create/locate the Sighting
correlate the records
evaluate applicability
create an explainable Assessment
ingest changed intelligence later
reprocess the old Sighting
create a new explainable Assessment
preserve every original record
```

That is the product loop.
