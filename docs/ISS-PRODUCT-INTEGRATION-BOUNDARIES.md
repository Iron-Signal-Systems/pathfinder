# Pathfinder Phase 0.16 — ISS Product Integration Boundaries

## Purpose

Pathfinder is part of the Iron Signal Systems product family, but it remains an independent authority boundary.

> **Integration shares information. It does not transfer domain authority.**

Each ISS product remains authoritative only for the facts and decisions within its defined domain. Pathfinder may consume, correlate, interpret, and assess information from another ISS product, but it must not rewrite that product's observations as Pathfinder-owned facts. Likewise, another ISS product must not become authoritative for Pathfinder intelligence merely because it consumes Pathfinder output.

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

Observation authority:
    Stronghold

Pathfinder authority:
    preservation, correlation, and threat-intelligence interpretation
```

Therefore:

```text
stored in Pathfinder != originated from Pathfinder
```

## Integration Record

Every imported ISS integration record must preserve:

```text
originating product
originating system identity
external record identity where available
observation/event time
Pathfinder receipt time
integration contract version
processing result
provenance
```

A cross-product record must always be traceable back to the product that supplied it.

## Integration Contract

Every ISS product integration requires a versioned contract, for example:

```text
pathfinder-stronghold-integration-v1
pathfinder-fi-integration-v1
pathfinder-atlas-integration-v1
```

Each contract defines:

```text
accepted record types
origin authority
required fields
time semantics
identity semantics
allowed Pathfinder objects produced
failure behavior
deduplication/idempotency behavior
security requirements
handling restrictions
```

No integration should rely on undocumented assumptions.

## Stronghold Integration

Stronghold is authoritative for the network traffic and enforcement events it directly observes or produces under its own contracts.

Potential Pathfinder inputs include:

```text
network communications
source/destination observations
DNS observations
TLS/certificate observations
application/network classification
firewall decisions
policy decision history
network-path context
interface observation context
```

Pathfinder may transform these into Sightings, relationship support, operational relevance Assessments, and correlation inputs where the applicable Pathfinder contract permits.

A Stronghold observation does not become a Pathfinder conclusion. A Stronghold deny does not mean malicious; a Stronghold allow does not mean benign.

## Pathfinder to Stronghold

Pathfinder may eventually publish intelligence or recommendations to Stronghold, including Indicators, threat classification, operational relevance, block candidates, and monitoring candidates.

These are inputs to Stronghold. They are not Stronghold policy.

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

Stronghold retains its own validation, policy evaluation, simulation, authorization, commit, audit, and rollback behavior.

There is no automatic rule:

```text
HIGH-confidence Indicator -> Stronghold block
```

Even high confidence, multiple independent sources, and a local Sighting do not themselves create network enforcement authority.

> **High confidence is still not authorization.**

## FI Integration

FI is authoritative for file and file-system observations within its collection contract.

Potential Pathfinder inputs include file hash, file path, system identity, signer/certificate data, file metadata, creation/modification observations, security-descriptor context, and process/file relationships where FI defines them.

Pathfinder may correlate those observations against malware, Indicators, campaign intelligence, certificate intelligence, known tooling, and threat relationships.

A file hash observed by FI does not prove execution, malicious intent, successful compromise, or actor identity.

```text
file present != malware executed
hash match != compromise
signed file != benign
unsigned file != malicious
```

Pathfinder may eventually provide FI with hash-watch candidates, certificate-watch candidates, file-investigation candidates, and malware-context enrichment. These remain intelligence or recommendations, not deletion/quarantine authority.

## Atlas Integration

Atlas is authoritative for the modeled asset/environment context it maintains under its own contracts.

Potential Pathfinder inputs include asset identity, hostname, IP assignment, system role, business role, operating system, software inventory, service exposure, network placement, asset criticality, and ownership context.

Pathfinder may use Atlas context to answer:

> **Does this threat matter to systems we actually operate?**

Pathfinder does not become the asset-inventory authority.

Where practical, Pathfinder should preserve references to Atlas-owned asset identity rather than creating independent drifting copies. Cached context needed for historical reconstruction must be identified as an Atlas-derived historical snapshot, not current Atlas truth.

## Historical Asset Context

Current Atlas state may differ from the state when an event occurred. Historical Pathfinder records must not be rewritten using current Atlas state.

Pathfinder must distinguish:

```text
current Atlas state
Atlas state known at event time
Pathfinder-cached historical context
```

## Correlation Is Not Identity

Cross-product correlation must remain explicit.

Matching hostname, IP address, certificate, MAC address, file path, or account name does not automatically establish identity.

> **Correlation is not identity.**

Where identity is uncertain, Pathfinder preserves candidate correlation such as:

```text
Stronghold observation X
    possibly corresponds to
Atlas Asset Y
```

rather than manufacturing equality.

## Guidon Integration

Guidon is not an initial Pathfinder runtime dependency.

Potential future integrations may include recovery-point threat context, incident recovery context, known-compromised-time references, and recovery prioritization intelligence.

Any Guidon integration requires its own explicit contract. Pathfinder must not require Guidon for normal threat-intelligence processing, and Guidon must not require Pathfinder for core backup or recovery operations.

## No Circular Runtime Dependency

ISS integrations must avoid circular dependency.

Preferred behavior:

```text
products remain independently operable
integrations enhance capability
integration outage becomes explicit degraded state
```

## Degraded Integration

If an integration is unavailable, Pathfinder reports the loss of coverage rather than manufacturing a negative conclusion.

Examples:

```text
Atlas unavailable
    -> asset enrichment UNAVAILABLE
    != asset does not exist

Stronghold unavailable
    -> network observation coverage DEGRADED
    != no suspicious communications occurred

FI unavailable
    -> file observation integration UNAVAILABLE
    != no matching files exist
```

## Missing Integration Is Not Negative Evidence

```text
no Stronghold Sighting != traffic did not occur
no FI Sighting != file did not exist
no Atlas match != asset does not exist
```

unless the applicable product can establish complete coverage for the relevant scope and time.

## Integration Health

Potential initial integration states include:

```text
HEALTHY
DEGRADED
UNAVAILABLE
AUTHENTICATION_FAILED
AUTHORIZATION_FAILED
SCHEMA_MISMATCH
VERSION_UNSUPPORTED
```

These states describe the integration path, not the underlying environment.

## Version Compatibility

Every integration contract defines supported producer/schema versions. Unsupported versions fail explicitly; Pathfinder must not silently parse new schemas using old assumptions.

Unknown additive fields may be preserved without interpretation where safe. Unknown fields do not gain semantics automatically.

## Integration Authentication

Phase 0.15 governs authentication. Each integration should have a separate principal, narrow permissions, a versioned contract, mTLS-preferred authentication, and auditable identity.

Payload-supplied product names never establish product identity.

An FI principal must not be able to submit an observation as Stronghold. Origin authority must be derived from authenticated integration identity.

## Multi-Hop Integration

If information travels through another ISS system, Pathfinder preserves origin and delivery separately.

Example:

```text
FI observation
    delivered through Atlas
    received by Pathfinder

origin: FI
delivery: Atlas
```

This mirrors Pathfinder's external source origin/delivery distinction.

## Cross-Product Provenance

Pathfinder must be able to answer:

```text
Which ISS product observed this?
Which system instance produced it?
How did Pathfinder receive it?
Was it transformed in transit?
Which schema version was used?
When was it observed?
When was it received?
How did Pathfinder interpret it?
```

## Cross-Product Time

Source-system event time and Pathfinder receipt time remain separate. Clock disagreement must not silently rewrite originating timestamps. Where source systems provide time-quality or clock-state metadata, Pathfinder should preserve it where relevant.

## Replay and Idempotency

Repeated delivery of the same authoritative observation must not create false new observations when stable external identity exists.

```text
one source observation
multiple deliveries/retries
```

should remain one Sighting plus delivery history where the integration contract permits that determination.

## Aggregate Observations

Aggregate observations remain aggregate observations. Pathfinder must not fabricate individual events from counts.

## Integration Enrichment

Pathfinder may enrich incoming observations, but the derived intelligence remains Pathfinder-owned interpretation and never rewrites the original peer observation.

## No Reverse Contamination

Later Pathfinder conclusions must not flow backward and overwrite peer history. If Stronghold classified an application as unknown and Pathfinder later associates the destination with malware, Stronghold history remains what Stronghold recorded.

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

Any future execution workflow requires a separate authorization and action contract.

## Feedback From Peer Products

Stronghold, FI, and Atlas may return outcomes or contextual feedback. These may become new Pathfinder context but do not modify the original threat intelligence.

Negative results must carry the originating product's coverage state.

## Data Handling and Tenant Boundaries

Cross-product information may carry customer, incident, topology, identity, or other handling restrictions. Pathfinder must preserve those restrictions.

If products support multiple customers/tenants, tenant identity is part of correlation scope. Pathfinder must never correlate across tenants merely because identifiers match.

Identifiers such as hostname, username, private IP, file path, and asset name are not globally unique.

## Data Minimization

Integrations should send what Pathfinder needs under the contract, not entire unrelated product state. This reduces coupling, attack surface, data exposure, and storage duplication.

## API Versus Direct Database Access

ISS products integrate through defined interfaces.

Forbidden architecture:

```text
Pathfinder writes Atlas tables
Stronghold edits Pathfinder tables
FI inserts directly into Pathfinder PostgreSQL
```

Preferred architecture:

```text
product API/message contract
        ↓
validation boundary
        ↓
owning product
```

> **Same database technology does not imply shared schema authority.**

## Failure Isolation and Compromised Peers

A malformed, defective, or compromised peer must not corrupt Pathfinder outside the integration's permitted scope.

Valid mTLS proves identity under the configured trust model, not that the peer is behaving correctly. Pathfinder still enforces schema validation, semantic validation, bounds, rate limits, authority scope, idempotency, and audit.

## Integration Disablement and Replacement

Administrators must be able to disable or replace one integration without deleting historical records.

```text
integration disabled != old observations erased
replacement system != original system identity
```

Historical records preserve the originating system identity and integration contract version.

## Reprocessing Integration Data

Where preserved material permits, Pathfinder may reprocess integration records under a newer mapping version. Original processing remains historical.

```text
integration mapper v1 -> Result A
integration mapper v2 -> Result B
```

New processing does not rewrite what Pathfinder originally understood.

## Common Truth Separations

```text
integration                     != authority transfer
stored in Pathfinder            != originated from Pathfinder
authenticated peer              != unlimited peer authority
Stronghold observation          != Pathfinder conclusion
Stronghold allow                != benign
Stronghold deny                 != malicious
Pathfinder Indicator            != Stronghold policy
Pathfinder candidate            != Stronghold command
FI file observation             != execution
FI hash match                   != compromise
Atlas asset context             != Pathfinder asset authority
Atlas no match                  != asset definitely absent
correlation                     != identity
matching hostname               != same asset automatically
matching private IP             != same asset automatically
current Atlas state             != historical asset state
integration unavailable         != no events occurred
no Sighting                     != negative observation
integration health              != environment health
schema mismatch                 != no data exists
duplicate delivery              != repeated observation
aggregate                       != individual event history
Pathfinder enrichment           != original peer observation
later Pathfinder conclusion     != originating record rewritten
candidate action                != enforcement authority
same database technology        != shared data ownership
valid mTLS                      != valid semantics
integration disabled            != history deleted
replacement system              != original system identity
new mapper                      != historical interpretation rewritten
```

## Phase 0.16 Exit Decision

Phase 0.16 is satisfied when Pathfinder accepts that:

1. Integration shares information without transferring domain authority.
2. Atlas, FI, Stronghold, Pathfinder, and Guidon retain separate product authority.
3. Pathfinder remains authoritative for the organization's record and interpretation of threat intelligence.
4. Imported ISS records preserve their originating product and system identity.
5. Every ISS integration is governed by a versioned contract.
6. Stronghold remains authoritative for defined network observations and enforcement decisions.
7. FI remains authoritative for defined file and file-system observations.
8. Atlas remains authoritative for defined asset/environment context.
9. Guidon remains independent and is not an initial Pathfinder runtime dependency.
10. Pathfinder may create Sightings and Assessments from peer data without rewriting peer observations.
11. Stronghold allow/deny decisions do not become threat classifications.
12. FI hash/file observations do not become execution or compromise conclusions.
13. Atlas relevance context does not become Pathfinder-owned asset truth.
14. Historical asset/environment context must not be rewritten using current Atlas state.
15. Cross-product correlation does not automatically establish identity.
16. Candidate correlations preserve ambiguity where identity cannot be established.
17. Integration outages become explicit degraded coverage states rather than negative evidence.
18. Missing integration data never silently means the observed condition did not occur.
19. Integration schema/version mismatches fail explicitly.
20. Authenticated integration identity determines allowed origin authority.
21. One ISS product cannot impersonate another through payload fields.
22. Multi-hop delivery preserves origin and delivery identities separately.
23. Event time and Pathfinder receipt time remain separate.
24. Retry delivery remains distinguishable from repeated observation.
25. Aggregate observations are not expanded into fabricated individual events.
26. Pathfinder enrichment never rewrites original peer observations.
27. Pathfinder candidate actions remain structurally distinct from execution commands.
28. Downstream products retain validation, authorization, and enforcement authority.
29. Customer/tenant boundaries remain part of correlation scope.
30. Non-global identifiers are never assumed globally unique.
31. Integrations follow data minimization.
32. Products integrate through controlled interfaces rather than direct cross-product database writes.
33. Shared database technology never implies shared schema authority.
34. Authenticated peers remain subject to semantic validation, bounds, least privilege, and audit.
35. One integration can be disabled or replaced without destroying historical records.
36. Historical records preserve the integration contract version under which they were processed.
37. Reprocessing produces new lineage rather than rewriting historical interpretation.
