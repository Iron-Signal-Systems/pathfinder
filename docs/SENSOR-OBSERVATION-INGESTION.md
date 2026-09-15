# Pathfinder Sensor Observation Ingestion

## Purpose

This document defines the intended boundary between Pathfinder sensors and the Pathfinder server.

The first validated sensor is the UniFi UDM-Pro sensor, but the contract is intended to support additional Pathfinder observation sources without coupling the server to one appliance.

The governing rule is:

> **A sensor reports what it observed. Pathfinder decides what that observation means in the context of threat intelligence.**

## Responsibilities

### Sensor Responsibilities

A Pathfinder sensor is responsible for:

```text
collecting supported observations
recording observation time
identifying the observation vantage
preserving sensor-local health/coverage state
maintaining bounded local resilience storage
forming versioned delivery records/batches
retrying safely
reporting failure honestly
```

A sensor must not manufacture threat conclusions.

### Server Responsibilities

The Pathfinder server is responsible for:

```text
authenticating the sensor identity
authorizing the sensor's permitted contract
validating schema and limits
receiving idempotently
recording receipt/processing history
preserving required original observation records
indexing observations for historical query
deriving Sightings under versioned rules
correlating Sightings with threat intelligence
assessing applicability
historical reprocessing
```

## Local Ring Purpose

The sensor-local compressed ring is not the primary organization-wide historical query database.

Its purposes are:

```text
delivery resilience
short-term source record
sensor diagnostics
validation
troubleshooting
reconciliation/replay after delivery interruption
```

Local diagnostic commands such as connection/path reconstruction remain useful for proving what the sensor recorded and investigating collector behavior.

Broad operator historical queries should run against the Pathfinder server's indexed sensor observation store.

## Delivery Contract Direction

The wire contract is not frozen yet, but implementation must support the following semantics.

### Sensor Identity

Every delivery is attributable to an authenticated sensor identity.

The preferred machine-to-machine direction is mTLS with narrowly scoped identities.

A sensor identity must not obtain:

```text
general Pathfinder administrative authority
threat-intelligence authoring authority
other-sensor write authority
direct database authority
enforcement authority
```

### Versioning

Every batch/record must identify the applicable schema/profile version.

The server must be able to distinguish:

```text
supported
unsupported
malformed
partially processable
```

Unsupported input is not malformed input.

### Batch Identity

A delivery batch requires stable identity sufficient for safe retry and duplicate recognition.

Conceptual fields include:

```text
sensor_id
sensor_schema_version
batch_id
batch_created_at
first_event_at
last_event_at
record_count
sequence/checkpoint information where required
integrity metadata
```

Exact fields are implementation work, not frozen by this document.

### Record Identity

Sensor observations require stable identity or a deterministic duplicate-detection contract.

Retries must not create false repeated Sightings merely because the same observation was delivered twice.

```text
duplicate delivery != repeated observation
```

### Time

Preserve separately where applicable:

```text
observation/event time
sensor batch creation time
server receipt time
server preservation time
processing time
Sighting creation time
Assessment time
```

Receipt time must not replace unknown observation time.

### Acknowledgement

The sender must only advance its delivery/checkpoint state after the server reaches the defined durable acceptance boundary.

The acknowledgement contract must distinguish:

```text
accepted
already accepted
partially accepted
rejected malformed
rejected unsupported
unauthorized
temporarily unavailable
```

Do not infer success from connection close or absence of an error.

### Replay

Replay is expected operational behavior.

The server must tolerate:

```text
retry after timeout
retry after uncertain acknowledgement
sensor restart
server restart
network interruption
delivery of already accepted batch
```

Replayed source records must remain distinguishable from genuinely new observations.

## Observation Store

The server-side sensor observation store is optimized for historical observation lookup and correlation.

Initial classes include:

```text
sensor identity
endpoint identity
network addresses
protocol
ports
interface/vantage
direction
DNS
TLS SNI
NAT/path
conntrack
timestamps
sensor health
coverage
```

The observation store is not the threat-intelligence source store.

Threat-source provenance and sensor-observation provenance remain different domains.

## UDM-Pro Observation Semantics

The UDM-Pro sensor currently distinguishes:

```text
packet observation
    directly observed packet metadata

DNS observation
    DNS question/answer directly observed in traffic

TLS observation
    plaintext SNI observed in a complete ClientHello

identity observation
    endpoint identity available at the recorded time

conntrack observation
    state reported by Linux conntrack

NAT/path correlation
    derived relationship between independent observations

sensor health
    collector/kernel/storage/coverage information
```

A DNS or SNI value must not replace the underlying packet observation.

A path relationship must not be presented as a forwarding-latency measurement.

A missing WAN observation must not silently become proof that a firewall blocked the packet.

## Observation Integrity and Provenance

At minimum, the server must be able to answer:

```text
Which sensor submitted this?

Which sensor version/schema produced it?

When was it observed?

When was it received?

Was the record accepted completely?

Was it replayed?

Was it transformed?

Which processing version derived the Sighting?

Was sensor coverage healthy at the time?
```

Hashing/signing requirements for sensor batches may be added when the transport contract is implemented. They should be driven by the actual threat model rather than added decoratively.

## Coverage

Sensor coverage is first-class context.

Relevant states may include:

```text
healthy
degraded
unknown
interrupted
backlogged
partial
```

The exact enum is implementation work and must remain a typed state namespace.

Pathfinder must not conclude:

```text
no matching Sighting exists
```

as equivalent to:

```text
the activity did not occur
```

unless the required coverage can actually be established.

## Security

Sensor ingestion is untrusted input even when the sensor is authenticated.

The server must protect against:

```text
oversized batches
malformed records
unexpected schema versions
duplicate floods
replay abuse
resource exhaustion
invalid timestamps
invalid sensor identity claims
cross-sensor impersonation
path traversal in any source-supplied identifier
unexpected compression/encoding behavior
```

A compromised sensor remains bounded to its permitted observation authority.

## No Direct Database Writes

Sensors do not write directly into Pathfinder database tables.

```text
sensor
  |
  v
versioned authenticated ingestion boundary
  |
  v
validation / preservation / commit
  |
  v
sensor observation store
```

This preserves schema control, provenance, authentication, validation, failure handling, and future compatibility.

## Relationship to Sightings

The observation store may contain more records than Pathfinder needs to expose as individual Sightings.

A versioned processing step determines when an observation or bounded set of observations creates a Sighting.

That processing must be:

```text
deterministic where possible
versioned
traceable
re-runnable
non-destructive
```

The original observation remains available even if Sighting derivation changes later.

## Acceptance Requirements

The first server ingestion implementation is not complete until it proves:

```text
authenticated sensor identity
valid batch accepted
duplicate batch handled idempotently
malformed batch rejected visibly
unsupported version rejected visibly
server failure does not create false success
retry after uncertain acknowledgement is safe
accepted observations are historically queryable
coverage/health context is retained
Sighting derivation is traceable
original observation is not rewritten
```
