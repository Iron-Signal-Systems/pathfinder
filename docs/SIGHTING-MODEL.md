# Pathfinder Phase 0.8 — Sighting Model

## Purpose

A `Sighting` records that an identified observation authority observed a specific Pathfinder subject within explicit time and context.

A Sighting answers:

> **What was observed, where applicable, by whom or what, and when?**

A Sighting does not answer:

> **What does the observation prove?**

That belongs to Assertions, Relationships, and Assessments.

> **A sighting records an observation. It does not silently become a conclusion.**

```text
Sighting != maliciousness
Sighting != compromise
Sighting != attribution
Sighting != successful exploitation
Sighting != intent
```

The governing historical invariant is:

> **The original record is maintained no matter what.**

## Sighting Identity

Every Sighting receives a Pathfinder-controlled identity.

```text
sighting_id = UUIDv7
```

The observation identity is separate from the observed subject identity.

Repeated observations of one Observable produce separate Sightings unless an explicit source/integration aggregation contract says otherwise.

## Conceptual Fields

A Sighting should preserve at least:

```text
sighting_id
subject_id
observer_type
observer_identity
origin
sighting_form
observed_at_state
observed_at / NOT_KNOWN
first_seen / NOT_APPLICABLE
last_seen / NOT_APPLICABLE
observation_count / NOT_APPLICABLE
observation_context
source_record_id / NOT_APPLICABLE
assertion_id / NOT_APPLICABLE
external_observation_id / NOT_KNOWN
received_at
created_at
```

Source/integration-specific context remains governed by explicit contracts rather than an unrestricted semantic key/value bag.

## Sighting Subject

A Sighting identifies exactly what Pathfinder considers to have been observed.

Initial supported subjects include:

```text
Observable
Indicator
```

The primary case should normally be an Observable.

Example:

```text
Sighting subject:
    IPv4 203.0.113.17
```

rather than a conclusion such as:

```text
Sighting subject:
    malicious C2
```

An Observable sighted does not mean an associated Indicator was proven.

```text
Observable sighted != Indicator proven
```

Future subject classes require explicit observation semantics.

## Sighting Origin

Initial Sighting origins are:

```text
INTEGRATION_OBSERVED
SOURCE_REPORTED
ANALYST_RECORDED
```

### INTEGRATION_OBSERVED

A trusted integration supplies an observation produced within its defined domain authority.

Examples include Stronghold network observations and FI file observations.

Pathfinder records the Sighting while preserving the originating product/system authority.

### SOURCE_REPORTED

An external source reports that an observation occurred.

Pathfinder preserves the external report as an Assertion/SourceRecord path and may represent the reported observation as a Sighting while retaining external observer/source authority.

```text
Pathfinder received report != Pathfinder directly observed subject
```

### ANALYST_RECORDED

An authorized analyst records an observation established through material/investigation available to the analyst.

The analyst principal and basis remain attributable.

The analyst does not impersonate Stronghold, FI, or an external source as the observer.

## Observation Authority

A Sighting identifies the authority responsible for the observation.

Examples include:

```text
Stronghold appliance
FI collector
external provider
human analyst
approved future sensor/integration
```

Pathfinder receiving information does not make Pathfinder the observation authority.

## Stronghold Sightings

Stronghold may supply authoritative network observations.

Example:

```text
Stronghold observed:
    source asset = FIN-PC-17
    destination = 203.0.113.17
    destination port = 443
    protocol = TCP
    observed_at = T
```

Pathfinder may record a Sighting of the destination/contact and separately correlate it with threat intelligence.

A later Assessment may conclude high operational relevance, but the Stronghold Sighting itself remains the network observation.

```text
Stronghold contact observed != host compromised
```

## FI Sightings

FI may supply authoritative file/file-system observations.

Example:

```text
FI observed:
    system = FS-03
    path = C:\Data\example.exe
    sha256 = abc...
    observed_at = T
```

Pathfinder may record a Sighting of the SHA-256 with FI observation context.

```text
hash sighted on host != malware executed
hash sighted on host != host compromised
```

## Atlas Context

Atlas may provide asset/environment context associated with a Sighting.

That context does not become part of the original Stronghold/FI observation and does not transfer Atlas asset authority to Pathfinder.

Historical asset context must remain distinguishable from current Atlas state.

## Point Sightings

A point Sighting represents an observation at one known observation time.

Pathfinder preserves timestamp precision and does not invent precision.

A source supplying only a date must not be silently expanded into a precise midnight timestamp as though the time were known.

## Bounded Aggregate Sightings

Some observation systems provide bounded summaries.

Initial forms are:

```text
POINT
BOUNDED_AGGREGATE
```

A bounded aggregate should preserve where supplied:

```text
first_seen
last_seen
observation_count
aggregation_method
aggregation_version
```

An aggregate is not retained individual event history.

```text
aggregate Sighting != raw event history
```

Pathfinder does not fabricate individual events from an aggregate count.

## No Invented Observation Count

If a source says “frequently observed,” Pathfinder does not convert that phrase into an invented numeric count.

A missing count remains `NOT_KNOWN` or `NOT_APPLICABLE` according to the source contract.

## Repeated Sightings

Repeated observations may matter for recency, frequency, and operational relevance.

They do not become independent corroboration of maliciousness.

```text
500 Sightings != 500 independent sources
```

## Observation Context

Potential structured Sighting context includes:

```text
asset reference
network direction
source/destination role
port
protocol
file path
process context
sensor identity
interface
collection point
tenant/customer boundary
external observation context
```

Context belongs to the observation and does not become identity of the observed Observable.

```text
IP observed from FIN-PC-17 != FIN-PC-17 is part of IP identity
file hash observed at path X != path is part of hash identity
```

## Observation Time Versus Receipt Time

Pathfinder preserves:

```text
observed_at
received_at
```

separately.

Late delivery does not rewrite observation time.

If observation time is unknown:

```text
observed_at_state = NOT_KNOWN
```

Pathfinder does not substitute publication, retrieval, receipt, or current time.

Uncertain time windows remain uncertain rather than being collapsed into a fabricated instant.

## Sighting Provenance

Every Sighting is traceable to origin.

ISS integration example:

```text
Sighting
    ↓
integration record identity
    ↓
Stronghold/FI authoritative observation
```

External-source example:

```text
Sighting
    ↓
Assertion
    ↓
SourceRecord
    ↓
SourceArtifact
```

Analyst example:

```text
Sighting
    ↓
analyst principal
    ↓
recorded basis
```

## External Observation Identifier and Idempotency

Where an observation authority supplies a stable event identifier, Pathfinder preserves it separately from `sighting_id`.

A retry delivering the same external event twice may resolve to one Sighting where the integration contract establishes stable identity.

Delivery/retry processing remains historical.

```text
duplicate delivery != repeated observation
```

Pathfinder does not deduplicate merely because subject, observer, and timestamp happen to match.

## Conflicting Sightings

Apparently inconsistent Sightings are not automatically invalid.

Different observation points may legitimately see different results because of DNS behavior, geography, cache state, load balancing, timing, or other context.

Pathfinder preserves both observations and uses Assessment/IntelligenceConflict where the difference is materially incompatible.

## Negative Observation

`not observed` is meaningful only relative to defined, sufficiently complete observation coverage.

```text
no Sighting found != activity did not occur
```

Pathfinder v1 does not create ordinary negative Sightings unless a future coverage contract can establish the required observation scope/time/completeness.

## Coverage Is Separate From Sighting

Coverage is not a property of one Sighting.

It belongs to the observing integration/source and applicable scope/time.

The canonical `CoverageState` values are:

```text
COMPLETE
PARTIAL
INCOMPLETE
NOT_KNOWN
```

`DEGRADED` and `UNAVAILABLE` are `HealthState` values describing a subsystem/integration, not CoverageState values.

Example:

```text
Stronghold integration health = DEGRADED
Stronghold coverage = INCOMPLETE
Sightings found = 0
```

The correct conclusion is:

```text
No matching Sighting was found within the available/incomplete observation coverage.
```

not:

```text
The communication never occurred.
```

## No Result Semantics

A Sighting query must account for:

```text
coverage state
processing backlog
index state
authorization scope
integration/source health
```

Zero visible results do not automatically establish universal absence.

## Sighting and Relationship

A Sighting may support a Relationship.

Example:

```text
DNS Sighting:
    Domain X observed resolving to IP Y

Relationship:
    Domain X resolves_to IP Y
```

The point observation remains distinct from the Relationship and does not silently become an indefinite relationship.

## Sighting and Assessment

Sighting is an allowed Assessment subject where the Assessment type permits it.

A Sighting may also form the basis for an Assessment of another subject.

Example:

```text
Indicator:
    IP X reported C2

Sighting:
    Stronghold observed local contact with IP X

Assessment:
    operational relevance = HIGH
```

The Sighting remains unchanged.

## Observation Validity

If later investigation establishes that a sensor generated an invalid observation, Pathfinder preserves the Sighting and creates an Assessment such as:

```text
subject = Sighting A
assessment_type = OBSERVATION_VALIDITY
assessment_value = INVALID
```

```text
invalid observation != historical Sighting deleted
```

## Sighting and Compromise

Pathfinder must never implement an implicit rule equivalent to:

```text
if Sighting matches malicious Indicator:
    compromised = true
```

A match may justify investigation, hunting, high operational relevance, or candidate downstream action, but compromise is a separate Assessment.

## Sighting and Enforcement

A Sighting never directly authorizes firewall blocking, endpoint isolation, account disablement, file deletion, or other remediation.

> **High confidence is still not authorization.**

## Historical Preservation

Later intelligence, analyst review, sensor defect discovery, source correction, or lifecycle change never rewrites the original Sighting.

Corrections move forward through Assessments, conflicts, or other attributable records.

## Common Truth Separations

```text
Sighting != maliciousness
Sighting != compromise
Sighting != attribution
Sighting != successful exploitation
Observable sighted != Indicator proven
Pathfinder receipt != Pathfinder observation
Source-reported Sighting != local direct observation
analyst-recorded Sighting != integration Sighting
point observation != indefinite Relationship
aggregate Sighting != individual event history
observation count missing != 1
repeated Sightings != independent corroboration
duplicate delivery != repeated observation
context != Observable identity
observed_at != received_at
unknown observed_at != receipt time
no Sighting != negative observation
CoverageState != HealthState
DEGRADED health != INCOMPLETE coverage
zero query results != activity never occurred
sensor defect != Sighting deleted
Sighting match != compromise
Sighting != enforcement authority
```

## Phase 0.8 Exit Decision

Phase 0.8 is satisfied when Pathfinder accepts that:

1. Sighting is a first-class UUIDv7 observation record.
2. A Sighting records observation and never silently becomes a conclusion.
3. Observation authority remains attributable to the originating system/source/analyst.
4. Point and bounded-aggregate Sightings remain distinct.
5. Counts and timestamps are never invented.
6. Repeated Sightings do not become independent corroboration.
7. Duplicate delivery remains distinct from repeated observation.
8. Observation context does not become Observable identity.
9. Observation time remains separate from receipt time.
10. Sighting provenance traces back through integration or SourceRecord/SourceArtifact history.
11. Coverage is separate from Sighting and uses typed `CoverageState`.
12. `DEGRADED`/`UNAVAILABLE` are health/capability state rather than coverage state.
13. Negative-observation claims require established coverage and are not created from missing Sightings.
14. Sighting may support Relationships and Assessments without becoming either.
15. Sighting itself may be assessed for observation validity without being rewritten.
16. Sighting never directly establishes compromise or enforcement authority.
17. **The original record is maintained no matter what.**
