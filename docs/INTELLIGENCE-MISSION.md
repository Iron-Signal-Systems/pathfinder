# Pathfinder Phase 0.1 — Intelligence Mission and Consumers

## Mission

Pathfinder is the Iron Signal Systems threat-intelligence platform.

Pathfinder exists to receive, preserve, normalize, correlate, assess, and distribute threat intelligence while preserving the provenance, time context, uncertainty, disagreement, and processing history necessary to understand what that intelligence actually means.

Pathfinder is intended to answer questions such as:

```text
What do we know about this observable?

Where did the information come from?

What exactly did the source report?

When was it observed or reported?

How reliable is the source?

How confident are we in the assessment?

Has it been independently corroborated?

Does conflicting intelligence exist?

What infrastructure, malware, campaigns, vulnerabilities,
techniques, or actors are associated with it?

Has it been observed within our environment?

Does it appear relevant to systems we operate?

What is known?

What is inferred?

What remains unknown?
```

Pathfinder does not treat the presence of an observable in a threat-intelligence source as proof of maliciousness.

Pathfinder does not treat correlation as proof of identity.

Pathfinder does not treat confidence as authorization.

Pathfinder does not silently convert intelligence into enforcement.

## Pathfinder Authority

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

Pathfinder is authoritative for the records Pathfinder itself creates and maintains, including:

```text
source registration and source identity
received source-record history
Pathfinder-normalized observables
Pathfinder indicators
Pathfinder assertions
Pathfinder assessments
Pathfinder relationships
Pathfinder threat-actor records
Pathfinder campaign records
Pathfinder malware/tool records
Pathfinder infrastructure records
Pathfinder vulnerability and technique relationships
Pathfinder sightings received through defined integration contracts
Pathfinder provenance and processing lineage
Pathfinder intelligence lifecycle state
Pathfinder analyst annotations and assessments
Pathfinder publication/export state
```

"Authoritative" means Pathfinder is authoritative for its record of the intelligence. It does not mean Pathfinder is the ultimate authority for the real-world truth of an external assertion.

For example, if a vendor reports that an IP address is command-and-control infrastructure, Pathfinder may authoritatively establish that the vendor made that assertion, when it was received, how it was represented, and how Pathfinder processed it. Pathfinder must not silently transform that into an unquestionable statement that the IP address is command-and-control infrastructure.

## Product Boundary

Pathfinder owns threat-intelligence knowledge and interpretation. It does not own every fact associated with that intelligence.

Initial ISS authority boundaries are:

```text
PATHFINDER
    threat intelligence
    source assertions
    threat relationships
    assessments
    provenance
    intelligence lifecycle

ATLAS
    asset and environmental context

STRONGHOLD
    network observation
    packet history
    network decisions
    network enforcement

FI
    file-system and file-related observations

GUIDON
    backup and recovery state
```

One product may provide information to another without transferring authority.

A Stronghold network observation remains a Stronghold-origin fact. Pathfinder may record a sighting and correlate it with threat intelligence, but the underlying observation remains Stronghold authority.

An FI file observation remains an FI-origin fact. Pathfinder may associate the observed hash, certificate, or other artifact with malware or threat infrastructure, but it does not rewrite FI's observation.

Atlas remains authoritative for its asset/environment facts. Pathfinder may add threat context but does not become Atlas's asset authority.

## Primary Consumers

### Human Analysts

Human analysts are the primary intelligence consumers.

An analyst must be able to:

```text
search
investigate
trace provenance
review relationships
compare sources
identify conflicts
review sightings
assess intelligence
annotate
dispute
supersede
publish where authorized
```

The interface must preserve enough detail to distinguish:

```text
source-provided
directly observed
normalized
derived
correlated
analyst-assessed
unknown
conflicted
expired
revoked
superseded
```

### Pathfinder API Clients

Authorized software may consume Pathfinder intelligence through stable APIs.

API access does not itself grant assessment, publication, administrative, or enforcement authority. Those are separate permissions.

### Atlas

Atlas may consume Pathfinder intelligence to determine organizational relevance. Pathfinder supplies threat context; Atlas supplies asset/environment context.

### Stronghold

Stronghold may provide network sightings to Pathfinder and may consume published intelligence, context, confidence, expiration, and future candidate enforcement recommendations.

Pathfinder does not directly grant network permission or denial. Any future enforcement remains subject to Stronghold's own policy, authorization, validation, simulation, approval, commit, and journal boundaries.

### FI

FI may provide file-related observations such as hashes, signer/certificate identity, file name, path, host, and observation time. Pathfinder may correlate those observations with malware, campaigns, actors, infrastructure, reports, and other observables while preserving FI's observation authority.

## Future Consumers

Potential future consumers may include TAXII clients, external APIs, authorized third-party security products, other ISS products, reporting/export systems, and controlled detection systems.

These are interoperability directions, not current implementation requirements.

## Non-Responsibilities

Pathfinder is not initially responsible for:

```text
packet capture
network forwarding
firewall enforcement
endpoint isolation
file deletion
account disablement
asset inventory ownership
vulnerability scanning
backup/recovery authority
SOAR orchestration
generic SIEM ingestion
EDR functionality
autonomous remediation
automatic firewall blocklists
AI replacement of human analysts
```

Some of these systems may consume Pathfinder output later. That does not make them Pathfinder responsibilities.

## Intelligence Versus Enforcement

Pathfinder may eventually produce recommendations such as a block candidate, investigation candidate, detection candidate, hunting candidate, or priority patch candidate.

A recommendation is intelligence. It is not authorization to execute the action.

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

## Primary Operational Question

Pathfinder should ultimately help an operator answer:

> **What do we know about this threat, why do we believe it, and does it matter to our environment?**

That question should remain recognizable as Pathfinder grows.

## Phase 0.1 Invariants

```text
observable != malicious
source assertion != established fact
sighting != compromise
correlation != identity
association != attribution
confidence != authorization
absence of intelligence != benign
current knowledge != historical knowledge
Pathfinder assessment != downstream enforcement approval
external-source authority != Pathfinder administrative authority
Pathfinder intelligence authority != Atlas asset authority
Pathfinder intelligence authority != Stronghold enforcement authority
Pathfinder intelligence authority != FI observation authority
```

## Phase 0.1 Exit Decision

Phase 0.1 is satisfied when the project accepts that:

1. Pathfinder is an intelligence system whose core responsibility is attributable, traceable, time-aware threat knowledge.
2. Pathfinder is authoritative for its intelligence records and processing history, not automatically for the real-world truth of every external assertion.
3. Human analysts are the primary consumer.
4. Atlas, Stronghold, and FI are explicitly bounded ISS consumers/producers.
5. Product integration does not transfer authority.
6. Threat intelligence may recommend downstream action but does not itself authorize enforcement.
7. Future external consumers must enter through explicitly defined interfaces and permissions.
