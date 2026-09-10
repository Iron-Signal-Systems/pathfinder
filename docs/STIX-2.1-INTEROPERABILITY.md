# Pathfinder Phase 0.12 — STIX 2.1 Interoperability Boundary

## Purpose

Pathfinder supports STIX 2.1 as an interoperability format without allowing STIX to become Pathfinder's internal truth model.

> **STIX is an interchange language, not Pathfinder's internal truth model.**

Incoming STIX follows the normal Pathfinder pipeline:

```text
receive
  -> preserve SourceArtifact
  -> validate STIX representation
  -> create SourceRecords
  -> extract Assertions
  -> map into Pathfinder-native objects
  -> correlate / assess
```

Valid STIX proves only that the representation can be interpreted under the supported contract. It does not prove source reliability, assertion truth, maliciousness, attribution, relevance, compromise, or enforcement authority.

## Identity and Versioning

STIX identifiers remain external identifiers and never replace Pathfinder UUIDv7 identities. The same STIX ID received through multiple delivery paths does not collapse delivery provenance. External STIX versions and revocation state remain historical source information; later versions never overwrite earlier Pathfinder history.

```text
STIX ID != Pathfinder ID
same STIX ID != same delivery provenance
STIX revoked != Pathfinder history deleted
```

## Bundle Semantics

A STIX Bundle is treated as a container. Co-bundled objects are not inferred to be related merely because they arrived together.

```text
same Bundle != related
```

Any native Pathfinder Relationship must come from an explicit STIX relationship/reference, an attributable Assertion, an approved Pathfinder derivation, or an authorized analyst action.

## Object Mapping

Each supported STIX object type requires an explicit, versioned semantic mapping. Similar field names do not prove equivalent meaning.

Potential mappings include:

```text
STIX Indicator      -> Pathfinder Indicator
STIX Campaign       -> Pathfinder Campaign
STIX Malware        -> Pathfinder Malware
STIX Threat Actor   -> Pathfinder ThreatActor
STIX Tool           -> Pathfinder Tool
STIX Infrastructure -> Pathfinder Infrastructure
STIX Vulnerability  -> Pathfinder Vulnerability
STIX Report         -> Pathfinder Report
```

Unsupported or partially supported STIX objects remain preserved as source material and receive explicit processing states rather than guessed mappings.

## Cyber-observable Objects

Supported STIX Cyber-observable Objects still pass through Pathfinder's own Observable Model and canonicalization rules.

```text
valid STIX observable != automatically valid Pathfinder Observable
```

Unsupported observable types remain preserved for future reprocessing.

## Indicators and Patterns

A STIX Indicator never becomes `Observable + malicious=true`. STIX patterns are untrusted input and require bounded parsing. Pathfinder must distinguish valid, invalid, unsupported, and partially supported pattern semantics. Unsupported clauses are never silently removed.

## Relationships

STIX Relationship Objects may support native Pathfinder Relationships, but Pathfinder's controlled relationship registry remains authoritative for allowed endpoints, directionality, time semantics, provenance, and conflict behavior.

Unknown STIX relationship vocabulary must not be dumped into `associated_with`.

## Sightings and Observed Data

A STIX Sighting is normally `SOURCE_REPORTED` unless a separate trusted integration contract establishes direct observation authority. STIX Observed Data does not automatically become a Pathfinder Sighting without sufficient observer, subject, time, and context semantics.

```text
STIX Sighting != Pathfinder direct observation
```

## Confidence

STIX confidence is source-provided confidence. It may be normalized only through an explicit mapping profile and never automatically becomes Pathfinder or analyst confidence.

```text
STIX confidence != Pathfinder confidence
```

## Markings, References, and Extensions

STIX markings and granular markings survive normalization. Unknown extensions remain preserved/uninterpreted or explicitly unsupported and never dynamically expand Pathfinder's native schema. External references are provenance/context and do not trigger automatic network retrieval.

## Translation Loss

Import/export mapping states must distinguish:

```text
COMPLETE
PARTIAL
UNSUPPORTED
FAILED
```

Translation loss is explicit. Pathfinder -> STIX -> Pathfinder is not assumed lossless.

## Export Boundary

Export begins from Pathfinder-native objects and requires separate export authorization plus handling/marking checks. External assertions, Pathfinder Assessments, derived Relationships, and local Sightings must remain distinguishable so Pathfinder does not misrepresent who made a claim.

Sensitive local context such as hostnames, internal addresses, paths, identities, customer information, or topology must not be exported without explicit authorization.

## Profiles and Reprocessing

Import and export behavior is governed by versioned profiles. A newer mapper may reprocess preserved SourceArtifacts and produce new lineage without rewriting prior processing history.

## Security

STIX processing remains an untrusted-input boundary. Implementations must bound artifact size, object count, nesting, references, timestamp handling, pattern complexity, and extension processing. No embedded or referenced content is automatically executed or fetched.

## Truth Separations

```text
STIX valid                       != intelligence true
STIX object                      != Pathfinder object
STIX ID                          != Pathfinder ID
same Bundle                      != related
STIX Indicator                   != malicious Observable
STIX Sighting                    != direct observation
STIX confidence                  != Pathfinder confidence
STIX relationship                != approved Pathfinder Relationship
custom STIX object               != Pathfinder object type
external reference               != automatically retrieved resource
partial translation              != complete translation
read authority                   != export authority
same STIX object from many feeds != independent corroboration
```

## Phase 0.12 Exit Decision

Phase 0.12 is satisfied when Pathfinder accepts STIX 2.1 as a versioned interoperability boundary; preserves all imported source material and provenance; keeps STIX identity, versioning, confidence, markings, relationships, sightings, and extensions distinct from native Pathfinder authority; exposes unsupported and lossy mappings explicitly; and never treats standards compliance as intelligence truth, corroboration, authorization, or enforcement authority.
