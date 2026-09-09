# Pathfinder Phase 0.8 — Sighting Model

## Purpose

A `Sighting` records that an identified observation authority observed a specific Pathfinder subject within an explicit time and context.

A Sighting answers:

> **What was observed, where was it observed, by whom or what, and when?**

A Sighting does not answer:

> **What does the observation prove?**

That belongs to Assertions, Relationships, and Assessments.

The governing principle is:

> **A sighting records an observation. It does not silently become a conclusion.**

```text
sighting
    != maliciousness

sighting
    != compromise

sighting
    != attribution

sighting
    != successful exploitation

sighting
    != intent
```

## Sighting Identity

Every Sighting receives a Pathfinder-controlled identity.

```text
sighting_id = UUIDv7
```

The identity of the observation is separate from the identity of the subject being observed.

For example:

```text
Observable:
    203.0.113.17

Sighting A:
    observed by Stronghold
    at 14:03

Sighting B:
    observed by Stronghold
    at 15:17
```

The Observable is one Pathfinder object.

The observations are separate historical events.

## Conceptual Fields

A Sighting should preserve at least:

```text
sighting_id

subject_id

observer_type

observer_identity

origin

observed_at_state
observed_at / not_known

first_seen / not_applicable
last_seen / not_applicable

observation_count / not_applicable

observation_context

source_record_id / not_applicable

assertion_id / not_applicable

external_observation_id / not_known

received_at

created_at
```

Additional source-specific observation details remain in provenance or explicitly defined context structures.

## Sighting Subject

A Sighting must identify exactly what Pathfinder considers to have been observed.

Initial supported subjects should include:

```text
Observable

Indicator
```

The primary use should normally be an Observable.

Example:

```text
Sighting:
    subject:
        IPv4 203.0.113.17
```

rather than:

```text
Sighting:
    subject:
        "C2 infrastructure"
```

because the first is an observation and the second already includes interpretation.

Future object types may become valid Sighting subjects only after their observation semantics are defined.

## Observable Sighting Versus Indicator Sighting

These are materially different.

```text
Observed:
    203.0.113.17
```

means the Observable was observed.

If Pathfinder also has an Indicator relating to that address, Pathfinder may correlate the two.

It must not rewrite the observation as:

```text
Observed:
    malicious C2
```

unless the observation system actually established that separate fact under its own contract.

Therefore:

```text
Observable sighted
    != Indicator proven
```

## Sighting Origin

Pathfinder distinguishes how a Sighting entered the system.

Initial origin classes:

```text
INTEGRATION_OBSERVED

SOURCE_REPORTED

ANALYST_RECORDED
```

### INTEGRATION_OBSERVED

A trusted integration supplies an observation produced by another system operating within its defined authority.

Examples:

```text
Stronghold network observation

FI file observation

future approved ISS observation source
```

Pathfinder records the observation while preserving the originating system's authority.

### SOURCE_REPORTED

An external intelligence source reports that an observation occurred.

Example:

```text
Vendor A reports:
    IP X observed communicating with malware samples
```

In this case Pathfinder should preserve both:

```text
Assertion:
    Vendor A says the observation occurred

Sighting:
    observer = Vendor A or identified observer
    basis = Assertion
```

The external report does not become a Pathfinder-direct observation.

### ANALYST_RECORDED

An authorized analyst records an observation from material or investigation available to the analyst.

The analyst identity and basis must remain attributable.

## Observation Authority

A Sighting must identify the authority responsible for the observation.

Examples:

```text
Stronghold appliance

FI collector

external provider

human analyst

approved sensor/integration
```

Pathfinder itself must not claim to have observed something merely because it received intelligence about it.

```text
Pathfinder received report
    != Pathfinder observed subject
```

## Stronghold Sightings

Stronghold may supply authoritative network observations.

Example:

```text
Stronghold observed:

source asset:
    FIN-PC-17

destination:
    203.0.113.17

destination port:
    443

protocol:
    TCP

observed at:
    2026-09-09T14:03:17Z
```

Pathfinder may create:

```text
Sighting:
    subject:
        203.0.113.17

    observer:
        Stronghold

    observed_at:
        2026-09-09T14:03:17Z

    context:
        FIN-PC-17
        destination
        TCP/443
```

Pathfinder may separately know:

```text
Indicator:
    203.0.113.17 associated with reported C2 activity
```

Combining those records may support an Assessment such as:

```text
operational_relevance = HIGH
```

But the Sighting itself remains:

```text
FIN-PC-17 communicated with 203.0.113.17
```

It does not become:

```text
FIN-PC-17 compromised
```

## FI Sightings

FI may supply authoritative file observations.

Example:

```text
FI observed:

system:
    FS-03

file:
    C:\Data\example.exe

SHA-256:
    abc...

observed at:
    ...
```

Pathfinder may create:

```text
Sighting:
    subject:
        SHA256 abc...

    observer:
        FI

    context:
        system FS-03
        path C:\Data\example.exe
```

If Pathfinder intelligence relates that SHA-256 to malware, that relationship exists separately.

Therefore:

```text
hash sighted on host
    != malware executed

hash sighted on host
    != host compromised
```

## Atlas Context

Atlas may provide authoritative asset/environment context associated with a Sighting.

For example:

```text
Stronghold sighting
    |
    v
asset reference
    |
    v
Atlas context:
    system = FIN-PC-17
    role = finance workstation
```

The Atlas information does not become part of the original Stronghold observation.

Pathfinder may reference the Atlas asset identity or use it during assessment while preserving authority boundaries.

## Point Sightings

A point Sighting represents an observation at one known observation time.

Example:

```text
observed_at:
    2026-09-09T14:03:17Z
```

Pathfinder must preserve available timestamp precision.

A source providing only:

```text
2026-09-09
```

must not be silently expanded into:

```text
2026-09-09T00:00:00.000000Z
```

as though that precision were known.

Time precision must remain explicit.

## Aggregated Sightings

Some observation systems may provide summaries instead of one event per observation.

Example:

```text
first_seen:
    14:00

last_seen:
    15:00

observation_count:
    427
```

Pathfinder may represent this as a bounded aggregate Sighting when the integration contract explicitly permits it.

Initial Sighting forms:

```text
POINT

BOUNDED_AGGREGATE
```

A bounded aggregate must preserve:

```text
first_seen

last_seen

observation_count

aggregation_method

aggregation_version
```

where applicable.

An aggregate is not equivalent to retained individual observation events.

```text
aggregate sighting
    != raw event history
```

## No Invented Observation Count

If an external source says:

```text
"frequently observed"
```

Pathfinder must not convert that phrase into:

```text
observation_count = 50
```

Likewise, a missing count is:

```text
not_known
```

not:

```text
1
```

unless the source semantics establish that the record itself represents exactly one observation.

## Repeated Sightings

Repeated observations may matter operationally.

Pathfinder should preserve:

```text
frequency
recency
first seen
last seen
observer diversity
local-system diversity
```

where available.

But repeated Sightings do not become independent threat-intelligence corroboration.

```text
500 sightings
    != 500 independent sources
```

Repeated activity may affect an Assessment.

It does not alter the meaning of the Sightings themselves.

## Observation Context

A Sighting may include structured context necessary to understand the observation.

Potential context includes:

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

Context must be governed by the integration producing it.

Pathfinder must not create a universal arbitrary key/value bag and treat every caller-defined field as trusted semantics.

Source-specific context should remain namespaced or contract-defined.

## Context Does Not Become Identity

For example:

```text
IP X observed from FIN-PC-17
```

does not make:

```text
FIN-PC-17
```

part of the identity of IP X.

Likewise:

```text
SHA256 X observed at C:\Temp\a.exe
```

does not make the path part of the SHA-256 Observable.

Context belongs to the observation.

## Observation Time Versus Receipt Time

Pathfinder must preserve:

```text
observed_at

received_at
```

separately.

Example:

```text
Stronghold observed:
    14:03

Pathfinder received:
    14:04
```

Pathfinder must not claim the event occurred at 14:04 merely because that is when the Sighting arrived.

```text
observation time
    != receipt time
```

## Delayed Sightings

A Sighting may arrive long after the observation occurred.

Example:

```text
observed:
    September 1

received by Pathfinder:
    September 9
```

This remains a September 1 observation.

Late arrival may be relevant to processing or assessment but does not rewrite event time.

## Unknown Observation Time

If a source establishes that an observation occurred but does not establish when:

```text
observed_at_state = NOT_KNOWN
```

Pathfinder must not substitute:

```text
publication time
retrieval time
receipt time
current time
```

for the missing observation time.

## Time Range

A source may report:

```text
observed sometime between
    September 1
and
    September 3
```

A future precision model may permit explicit observation windows.

Until defined, Pathfinder must not collapse uncertain ranges into an invented precise timestamp.

## Sighting Provenance

Every Sighting must be traceable to its origin.

For an ISS integration:

```text
Sighting
   ↓
integration record identity
   ↓
Stronghold/FI authoritative observation
```

For an external source:

```text
Sighting
   ↓
Assertion
   ↓
SourceRecord
   ↓
preserved source
```

For an analyst:

```text
Sighting
   ↓
analyst principal
   ↓
recorded basis
```

## External Observation Identifier

Where the originating system supplies a stable event or observation identifier, Pathfinder preserves it separately.

Example:

```text
observer:
    Stronghold

external_observation_id:
    ...
```

The external identifier does not replace Pathfinder's `sighting_id`.

## Sighting Deduplication

Pathfinder must not deduplicate Sightings simply because:

```text
same subject
same observer
same timestamp
```

Two real observation events may share those values.

Likewise, retries may cause one external observation to be delivered more than once.

Where an authoritative external observation identifier exists, the integration may use it to identify duplicate delivery.

Therefore:

```text
duplicate delivery
    != repeated observation
```

This distinction must be preserved.

## Idempotent Integration

ISS integrations should support idempotent delivery where practical.

If Stronghold delivers observation `ABC123` twice because the first acknowledgement was lost:

```text
delivery 1
delivery 2
```

should resolve to one Pathfinder Sighting if the integration contract establishes that both deliveries represent the same authoritative Stronghold observation.

Pathfinder should preserve delivery/retry processing history separately.

## Conflicting Sightings

Two Sightings may appear inconsistent.

Example:

```text
System A:
    domain resolved to IP X at 14:00

System B:
    domain resolved to IP Y at 14:00
```

This is not automatically an error.

Possible causes include:

```text
DNS load balancing

resolver differences

geography

cache state

observation-point differences

source error
```

Pathfinder preserves both observations.

Assessment determines whether they are meaningfully conflicting.

## Negative Observation

Pathfinder must be very careful with claims that something was **not observed**.

```text
not observed
```

is meaningful only relative to known observation coverage.

For example:

```text
Stronghold observed no communication with IP X
between 14:00 and 15:00
```

can only be represented strongly if Pathfinder knows that:

```text
Stronghold collection was active

the applicable interfaces were covered

the relevant time range was available

processing was complete
```

Otherwise:

```text
no sighting found
    != activity did not occur
```

Pathfinder v1 should therefore not create ordinary negative Sightings.

Negative-observation semantics should remain deferred until coverage/completeness contracts can support them safely.

## Collection Completeness

A Sighting query must not imply complete observational history when the underlying observation source is incomplete.

Example:

```text
Sightings found:
    0

Stronghold coverage:
    incomplete
```

must not be presented as:

```text
This system never communicated with the indicator.
```

The accurate conclusion is closer to:

```text
No matching Sighting was found within the available Pathfinder observation coverage.
```

The API should expose coverage state structurally.

## Observation Coverage Is Separate

Coverage information belongs to the observing integration or processing state.

It is not part of the Sighting itself.

Possible future states include:

```text
COMPLETE

PARTIAL

DEGRADED

UNAVAILABLE

NOT_KNOWN
```

These states will be reconciled with the Phase 0.18 audit/completeness contract.

## Sighting and Relationships

A Sighting may support creation or assessment of a Relationship.

For example:

```text
DNS observation:

Domain X resolved to IP Y
```

may produce:

```text
Sighting:
    Domain X observed resolving to IP Y
```

and support:

```text
Relationship:
    Domain X resolves_to IP Y
```

The point-in-time Sighting remains distinct from the intelligence Relationship.

The Relationship must not become timeless merely because a Sighting exists.

## Sighting and Assessment

Sightings are inputs to Assessments.

Example:

```text
Indicator:
    IP X associated with reported C2

Sighting:
    Stronghold observed FIN-PC-17 contacting IP X

Assessment:
    operational relevance = HIGH
```

This is a clean progression:

```text
threat intelligence
        +
local observation
        ↓
assessment
```

The Sighting itself remains unchanged.

## Sighting and Compromise

Pathfinder must never implement:

```text
if sighting matches malicious indicator:
    compromised = true
```

as an implicit semantic rule.

A matching Sighting may be extremely important.

It may justify:

```text
investigation candidate

hunt candidate

high operational relevance

candidate downstream action
```

But compromise is a separate conclusion requiring an applicable Assessment contract.

## Sighting and Enforcement

A Sighting never directly authorizes:

```text
firewall block

endpoint isolation

account disable

file deletion

quarantine

remediation
```

A downstream action requires its own authorization boundary.

```text
sighting
    != authorization
```

## Source-Reported Sighting Versus Direct Integration

Pathfinder must preserve the difference between:

```text
Vendor A reports that IP X was observed
```

and:

```text
Stronghold directly reports its own observation of IP X
```

Both may become Sightings.

But their provenance differs:

```text
SOURCE_REPORTED
```

versus:

```text
INTEGRATION_OBSERVED
```

A UI/API must not flatten those into indistinguishable records.

## Analyst-Recorded Sighting

An analyst may record a Sighting when authorized.

Example:

```text
Analyst reviewed incident artifact
and observed SHA256 X
```

Pathfinder preserves:

```text
analyst principal

observation time if known

recorded time

basis

context
```

If the analyst cannot establish when the underlying observation occurred:

```text
observed_at = NOT_KNOWN
```

The time of analyst entry must not be substituted.

## Sighting Retention

Sightings are historical records.

Expiration of an Indicator does not delete its Sightings.

```text
Indicator expired
    != Sighting erased
```

Likewise:

```text
Assessment changed
    != Sighting rewritten
```

Retention/destruction authority is handled separately.

## Sighting Reprocessing

A Sighting may later participate in new correlations or Assessments.

The original Sighting is not rewritten simply because Pathfinder learns new intelligence.

Example:

```text
September 1:
    Stronghold observes IP X

September 9:
    Pathfinder receives intelligence linking IP X to malware
```

Pathfinder may now produce a new Assessment.

It must not make the September 1 Sighting appear to have been recognized as malicious on September 1.

```text
later intelligence
    != earlier observation interpretation
```

## Common Truth Separations

```text
Sighting                         != Assertion

Sighting                         != Indicator

Sighting                         != Assessment

Sighting                         != maliciousness

Sighting                         != compromise

Sighting                         != attribution

Sighting                         != intent

Sighting                         != successful exploitation

Observable sighted               != Indicator proven

hash sighted                     != malware executed

IP contacted                     != endpoint compromised

local Sighting                   != independent threat-source corroboration

repeated Sightings               != repeated independent corroboration

observation count                != source count

observation time                 != receipt time

receipt time                     != event time

unknown observation time         != receipt time

point observation                != indefinite Relationship

aggregate Sighting               != raw event history

duplicate delivery               != repeated observation

no Sighting found                != activity did not occur

incomplete coverage              != negative observation

source-reported Sighting         != direct local observation

analyst-recorded Sighting        != sensor observation

Sighting context                 != Observable identity

later intelligence               != earlier interpretation

Sighting                         != enforcement authority
```

## Phase 0.8 Exit Decision

Phase 0.8 is satisfied when Pathfinder accepts that:

1. A Sighting is a first-class UUIDv7 historical observation record.
2. A Sighting identifies an explicit subject, observer, time state, origin, context, and provenance.
3. Initial Sighting origins are `INTEGRATION_OBSERVED`, `SOURCE_REPORTED`, and `ANALYST_RECORDED`.
4. External reported observations remain traceable through Assertions and SourceRecords.
5. ISS integrations retain authority for their underlying observations.
6. Pathfinder receipt time never substitutes for unknown observation time.
7. Timestamp precision is preserved rather than manufactured.
8. Point and bounded-aggregate Sightings remain distinguishable.
9. Aggregate Sightings do not imply retained raw event history.
10. Missing counts are not silently converted to one.
11. Repeated Sightings do not become independent intelligence corroboration.
12. Duplicate delivery remains distinguishable from repeated observation.
13. Integrations should support idempotent delivery using authoritative external observation identities where available.
14. Observation context does not become Observable identity.
15. A Sighting may support a Relationship or Assessment but remains a separate observation.
16. A malicious-indicator match does not automatically establish compromise.
17. Pathfinder v1 does not create ordinary negative Sightings without a defined coverage/completeness contract.
18. `no sightings found` never implies `activity did not occur` when observation coverage is incomplete or unknown.
19. Later intelligence and reprocessing do not rewrite what was known when the original Sighting occurred.
20. Sightings never directly create enforcement authority.
