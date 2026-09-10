# Pathfinder Phase 0.16 — ISS Product Integration Boundaries

## Purpose

Pathfinder is part of the Iron Signal Systems product family, but it remains an independent authority boundary.

> **Integration shares information. It does not transfer domain authority.**

Each ISS product remains authoritative only for the facts and decisions within its defined domain. Pathfinder may consume, correlate, interpret, and assess information from another ISS product, but it must not rewrite that product's observations as Pathfinder-owned facts.

Likewise, another ISS product does not become authoritative for Pathfinder intelligence merely because it consumes Pathfinder output.

The governing historical invariant is:

> **The original record is maintained no matter what.**

## Product Authority Map

```text
Atlas
    asset and environment context

FI
    file and file-system observations

Stronghold
    network observations and network enforcement decisions

Pathfinder
    organizational threat-intelligence record and interpretation

Guidon
    backup, recovery, and recovery-state authority
```

Pathfinder's authority statement remains:

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

That does not mean Pathfinder owns the underlying observations supplied by other systems.

## Authority Versus Storage

A fact being stored inside Pathfinder does not make Pathfinder the original authority for that fact.

Example:

```text
Stronghold observed:
    FIN-PC-17 connected to 203.0.113.17

Pathfinder stores:
    Sighting of that observation

observation authority:
    Stronghold

Pathfinder authority:
    preservation, correlation, and threat-intelligence interpretation
```

```text
stored in Pathfinder != originated from Pathfinder
```

## Integration Contract

Every ISS product integration requires a versioned contract.

Examples:

```text
pathfinder-stronghold-integration-v1
pathfinder-fi-integration-v1
pathfinder-atlas-integration-v1
```

Each contract defines:

```text
producer product/system identity
accepted record types
origin authority
required fields
time semantics
identity/correlation semantics
allowed Pathfinder objects produced
failure behavior
idempotency behavior
security requirements
handling restrictions
coverage semantics
contract/schema version
```

No integration relies on undocumented assumptions.

## Integration Record / Envelope

Every imported ISS integration event or record preserves enough information to identify:

```text
originating product
originating system identity
external record/event identity where available
integration contract version
observation/event time
Pathfinder receipt time
trigger/origin context where relevant
handling/tenant scope
processing result
provenance
```

The integration envelope is transport/provenance context. It does not automatically become a threat-intelligence object.

## Stronghold to Pathfinder

Stronghold is authoritative for the network traffic observations and enforcement decisions it produces under its own contracts.

Potential Pathfinder inputs include:

```text
network communications
source/destination observations
DNS observations
TLS/certificate observations
application/network classification
firewall decisions
policy decision history
network-path/interface context
```

Pathfinder may derive or create permitted:

```text
Sightings
relationship support
operational-relevance Assessments
correlation candidates
```

while preserving Stronghold as the observation authority.

```text
Stronghold allow != benign
Stronghold deny != malicious
Stronghold Sighting != compromise
```

## Pathfinder to Stronghold

Pathfinder may publish threat context or candidate recommendations such as:

```text
Indicator context
threat classification
operational relevance
WATCH candidate
HUNT candidate
BLOCK_CANDIDATE
```

These are inputs to Stronghold. They are not Stronghold policy or commands.

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

Stronghold retains its own validation, policy evaluation, simulation, authorization, commit, audit, and rollback behavior.

> **High confidence is still not authorization.**

## FI to Pathfinder

FI remains authoritative for file and file-system observations within its collection contract.

Potential inputs include:

```text
file hash
file path
system identity
signer/certificate information
file metadata
creation/modification observations
security-descriptor context
process/file relationships where FI defines them
```

Pathfinder may correlate these observations with malware, Indicators, campaigns, certificates, tools, infrastructure, and other threat intelligence.

```text
file present != malware executed
hash match != compromise
signed file != benign
unsigned file != malicious
```

## Pathfinder to FI

Pathfinder may provide FI with permitted investigation/watch candidates such as hashes, certificates, or file-review context.

These are intelligence/recommendations, not deletion, quarantine, ACL-change, or process-termination authority.

## Atlas to Pathfinder

Atlas remains authoritative for modeled asset/environment context.

Potential context includes:

```text
asset identity
hostname/IP assignment
system/business role
operating system
software inventory
service exposure
network placement
asset criticality
ownership context
```

Pathfinder may use Atlas context to assess operational relevance.

Pathfinder does not become the asset-inventory authority.

## Historical Atlas Context

Current Atlas state may differ from historical event-time state.

Pathfinder must distinguish:

```text
current Atlas state
Atlas state known at event time
Pathfinder-cached historical Atlas context
```

Historical Pathfinder records are not rewritten using current Atlas state.

## Pathfinder to Atlas

Pathfinder may provide threat associations, Assessments, and relevance context for Atlas presentation/correlation.

Atlas must not silently transform Pathfinder threat interpretation into Atlas-owned asset truth.

## Guidon

Guidon is not an initial Pathfinder runtime dependency.

Potential future integrations may include recovery-point threat context, known-compromised-time references, incident recovery context, and recovery prioritization intelligence.

Any Guidon integration requires its own explicit contract.

Pathfinder must not require Guidon for normal threat-intelligence processing, and Guidon must not require Pathfinder for core backup/recovery operation.

## Correlation Is Not Identity

Cross-product correlation remains explicit.

Matching hostname, IP address, certificate, MAC address, file path, or account name does not automatically establish identity.

> **Correlation is not identity.**

Where identity is uncertain, preserve a candidate such as:

```text
Stronghold observation X
    possibly corresponds to
Atlas Asset Y
```

rather than manufacturing equality.

## Tenant / Scope Identity

Identifiers such as hostnames, usernames, private IPs, and file paths are not globally unique.

Customer/tenant and other necessary scope are part of correlation semantics.

Pathfinder must not correlate across customer/tenant boundaries merely because text values match.

## No Circular Runtime Dependency

ISS products should remain independently operable within their core missions.

Preferred behavior:

```text
products remain independently operable
integrations enhance capability
integration outage becomes explicit health/coverage state
```

Pathfinder unavailability should not cause Stronghold to stop enforcing existing network policy. Stronghold/FI/Atlas integration failure should not cause Pathfinder to manufacture negative observations.

## Typed Integration State

Integration state is not one overloaded status enum.

### HealthState

Overall integration capability uses the Phase 0.18 health dimension:

```text
HEALTHY
DEGRADED
UNAVAILABLE
NOT_READY
FAILED
```

### CoverageState

Observation/context coverage for a defined scope/time uses:

```text
COMPLETE
PARTIAL
INCOMPLETE
NOT_KNOWN
```

### Operation / Processing Result

Individual delivery/processing attempts use their applicable `OperationResult` / `ProcessingState`.

### Failure Code

Specific causes such as the following are failure codes, not HealthState values:

```text
AUTHENTICATION_FAILED
AUTHORIZATION_FAILED
SCHEMA_MISMATCH
VERSION_UNSUPPORTED
REPLAY_REJECTED
RATE_LIMITED
TRANSPORT_FAILED
VALIDATION_FAILED
```

Example:

```text
integration_health = DEGRADED
coverage_state = INCOMPLETE
last_failure_code = AUTHENTICATION_FAILED
```

This is preferable to putting `DEGRADED` and `AUTHENTICATION_FAILED` into the same enum.

## Missing Integration Is Not Negative Observation

```text
no Stronghold Sighting != traffic did not occur
no FI Sighting != file did not exist
no Atlas match != asset does not exist
```

unless the applicable producer can establish sufficient coverage for the relevant scope/time.

If an integration is unavailable:

```text
integration_health = UNAVAILABLE or DEGRADED
coverage_state = INCOMPLETE / NOT_KNOWN
```

not a fabricated negative environmental conclusion.

## Version Compatibility

Every integration contract defines supported producer/schema versions.

Unsupported versions produce explicit processing failure, for example:

```text
processing_state = FAILED
failure_code = VERSION_UNSUPPORTED
```

Pathfinder does not silently parse v2 semantics using v1 assumptions.

Unknown additive fields may be preserved without interpretation where the contract allows it.

```text
unknown field != trusted new semantic field
```

## Authentication and Product Identity

Phase 0.15 governs authentication/authorization.

Each integration uses a separate principal with narrow permissions and auditable identity. mTLS is the preferred initial machine-to-machine direction.

Payload text such as `product=FI` or a User-Agent string does not establish product identity.

An FI principal may not submit data as Stronghold unless a future explicit mediation contract permits that path and preserves origin/delivery identities.

## Multi-Hop Delivery

If one ISS product relays another product's data, Pathfinder preserves origin and delivery separately.

Example:

```text
origin = FI
delivery = Atlas
consumer = Pathfinder
```

Atlas relaying the record does not become the original observation authority.

## Cross-Product Time

Producer event/observation time and Pathfinder receipt time remain separate.

Where supplied, clock-quality/offset state may also be preserved.

Pathfinder does not silently rewrite producer timestamps to fit local clock assumptions.

## Replay and Idempotency

Repeated delivery of the same authoritative producer event does not create false new Sightings when stable external identity permits reliable idempotency.

```text
one producer event
multiple retries
    -> one Sighting + delivery history
```

Duplicate delivery remains distinct from repeated real observation.

## Aggregate Observations

Aggregate producer records remain aggregates.

Pathfinder does not fabricate individual events from counts unless the producer supplied those underlying events.

## Cross-Product Provenance

Pathfinder must be able to answer:

```text
Which ISS product observed/provided this?
Which system instance produced it?
How was it delivered?
Was it transformed in transit?
Which integration/schema version applied?
When was it observed?
When was it received?
How did Pathfinder interpret it?
Was processing complete?
Was coverage complete?
```

## Integration Enrichment

Pathfinder may enrich an incoming peer observation.

The enrichment is Pathfinder-derived intelligence and must not overwrite the original producer record.

```text
Pathfinder enrichment != original Stronghold/FI/Atlas observation
```

## No Reverse Contamination

Later Pathfinder interpretation must not flow backward and rewrite peer history.

If Stronghold originally recorded `application = unknown` and Pathfinder later associates the destination with malware, Stronghold's original observation remains `unknown` unless Stronghold itself emits a new authoritative classification.

## Circular Corroboration Prevention

Cross-product feedback loops must not manufacture independent corroboration.

Example:

```text
1. Pathfinder rates Domain X high-risk.
2. An authorized downstream workflow causes Stronghold to watch/block X.
3. Stronghold reports blocked attempts to X.
4. Pathfinder records local Sightings of attempted contact.
```

The Stronghold observations corroborate local attempted contact. They do **not** independently corroborate Pathfinder's original maliciousness Assessment merely because the monitoring/block action was prompted by Pathfinder.

Likewise:

```text
Pathfinder requests FI hunt for hash H
FI finds hash H
```

corroborates local presence, not the maliciousness semantics that caused Pathfinder to request the hunt.

Trigger/origin lineage must remain visible so derived feedback is not miscounted as independent upstream support.

## Candidate Actions

Pathfinder may generate candidate actions such as:

```text
INVESTIGATE
WATCH
HUNT
BLOCK_CANDIDATE
FILE_REVIEW_CANDIDATE
PATCH_PRIORITY_CANDIDATE
```

These are recommendations.

```text
Pathfinder CandidateAction != command
```

Any future execution workflow requires separate downstream authorization/action contracts.

## Feedback From Peer Products

Stronghold, FI, Atlas, and future approved peers may return outcome/context records.

These may become new Pathfinder context or Sightings without modifying the original threat intelligence.

Negative outcomes must carry producer coverage/health state where needed for interpretation.

## Data Minimization

Integrations should send only what Pathfinder needs under the versioned contract rather than copying entire product state by default.

Examples:

```text
Pathfinder may need a Stronghold network observation
    != Pathfinder needs the entire firewall configuration

Pathfinder may need FI hash/path observation
    != Pathfinder needs the entire FI repository
```

Raw PCAP remains Stronghold-owned source material unless a separately authorized contract says otherwise. Raw file contents remain FI-owned unless specifically required and authorized.

## API / Message Contract, Not Direct Database Access

ISS products integrate through explicit APIs/message contracts/durable exchange artifacts.

Forbidden architecture:

```text
Pathfinder writes Atlas tables
Stronghold edits Pathfinder tables
FI inserts directly into Pathfinder PostgreSQL
```

> **Same database technology does not imply shared schema authority.**

Direct cross-product table access bypasses product validation, authority, audit, and version contracts and is not an approved integration mechanism.

## Failure Isolation and Compromised Peers

A malformed, defective, or compromised authenticated peer must not corrupt Pathfinder outside its permitted integration scope.

Valid mTLS proves configured identity, not correct behavior.

Pathfinder still enforces:

```text
schema validation
semantic validation
bounds
rate limits
authority scope
idempotency/replay rules
audit
```

## Integration Disablement and Replacement

Administrators may disable or replace one integration without deleting historical records.

```text
integration disabled != old observations erased
replacement system != original system identity
```

Historical records preserve producer system identity and contract version.

## Reprocessing Integration Data

Where preserved integration material permits, Pathfinder may process old records under a newer mapping version.

Original processing remains historical.

```text
mapper v1 -> Result A
mapper v2 -> Result B
```

New processing does not rewrite what Pathfinder understood originally.

## Common Truth Separations

```text
integration != authority transfer
stored in Pathfinder != originated from Pathfinder
authenticated peer != unlimited authority
HealthState != CoverageState
HealthState != failure code
DEGRADED != AUTHENTICATION_FAILED
Stronghold observation != Pathfinder conclusion
Stronghold allow != benign
Stronghold deny != malicious
Pathfinder Indicator != Stronghold policy
Pathfinder candidate != Stronghold command
FI file observation != execution
FI hash match != compromise
Atlas context != Pathfinder asset authority
Atlas no match != asset definitely absent
correlation != identity
same hostname/private IP != same asset automatically
current Atlas state != historical Atlas state
integration unavailable != no events occurred
no Sighting != negative observation
schema mismatch != no data exists
duplicate delivery != repeated observation
aggregate != fabricated individual events
Pathfinder enrichment != original peer observation
feedback Sighting != independent maliciousness corroboration automatically
valid mTLS != valid semantics
integration disabled != history deleted
new mapper != historical interpretation rewritten
```

## Phase 0.16 Exit Decision

Phase 0.16 is satisfied when Pathfinder accepts that:

1. Integration shares information without transferring domain authority.
2. Atlas, FI, Stronghold, Pathfinder, and Guidon retain separate product authority.
3. Imported ISS records preserve producer product/system identity and contract version.
4. Stronghold remains network-observation/enforcement authority; FI file-observation authority; Atlas asset/environment authority; Guidon recovery authority; Pathfinder threat-intelligence interpretation authority.
5. Cross-product correlation never silently establishes identity.
6. Missing integration data never becomes a negative observation without sufficient coverage.
7. Integration health, coverage, operation/processing result, and detailed failure code are separate typed dimensions.
8. `AUTHENTICATION_FAILED`, `SCHEMA_MISMATCH`, and `VERSION_UNSUPPORTED` are failure codes rather than HealthState values.
9. Multi-hop delivery preserves origin and delivery separately.
10. Retry delivery remains distinct from repeated real observation.
11. Aggregates are not expanded into fabricated event histories.
12. Pathfinder enrichment never rewrites peer observations.
13. Cross-product feedback loops do not manufacture independent corroboration.
14. Candidate actions remain recommendations rather than commands.
15. ISS products integrate through controlled interfaces rather than direct cross-product database writes.
16. Authenticated peers remain subject to semantic validation, bounds, least privilege, and audit.
17. Integration disablement/replacement/reprocessing never erases historical records.
18. **The original record is maintained no matter what.**
