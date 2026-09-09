# Pathfinder Phase 0.2 — Core Intelligence Object Model

## Purpose

Pathfinder requires an explicit internal intelligence model before schemas, APIs, collectors, or STIX translation are implemented.

The model must preserve the distinction between:

```text
what was reported
what was observed
what Pathfinder inferred
what Pathfinder assessed
```

Pathfinder uses its own internal object model. External formats such as STIX may later map into or out of this model but do not define Pathfinder's internal truth.

## Common Object Identity

Every Pathfinder first-class object receives a Pathfinder-controlled identifier.

Initial direction:

```text
Pathfinder ID: UUIDv7
```

External identifiers are retained separately and never replace Pathfinder identity.

Where applicable, objects also preserve creation time, creating authority, source/provenance references, processing lineage, and lifecycle state.

Pathfinder avoids ambiguous null values. Where a field requires a state, explicit values such as `not_known`, `not_observed`, `not_verified`, and `not_applicable` are preferred.

## Source

A `Source` identifies the origin from which threat intelligence is obtained.

A Source describes who or what originated or supplied intelligence. It does not mean the information supplied by that source is true.

Conceptual fields include:

```text
source_id
name
source_type
ownership / provider identity
trust configuration
handling / marking metadata
status
```

## SourceRecord

A `SourceRecord` represents a specific item received from a Source.

Examples include one API response object, TAXII object, advisory, feed entry, report, or analyst-supplied source artifact.

A SourceRecord answers:

> **What exactly did this source provide to Pathfinder?**

A SourceRecord is not the normalized intelligence object.

Conceptually:

```text
Source
   |
   v
SourceRecord
   |
   v
parsing / normalization
   |
   v
Pathfinder objects
```

SourceRecord history must survive later reinterpretation.

## Assertion

An `Assertion` is a first-class attributable claim extracted from or recorded against a SourceRecord.

It answers:

> **What did this source claim?**

Assertions are distinct from Pathfinder Assessments. The detailed assertion/assessment contract is defined in `ASSERTION-ASSESSMENT-MODEL.md`.

## Observable

An `Observable` is something that can be observed, referenced, or matched.

An Observable is semantically neutral.

Initial observable classes are defined in `OBSERVABLE-MODEL.md` and include:

```text
IPv4 address
IPv6 address
domain name
URL
SHA-256
email address
X.509 certificate SHA-256 fingerprint
```

The presence of an Observable in Pathfinder does not mean malicious, suspicious, benign, compromised, actor-owned, or campaign-related.

## Indicator

An `Indicator` represents an intelligence assertion that one or more observables or observable patterns have operational threat significance.

An Indicator is not simply an Observable with `malicious=true`.

An Indicator may refer to one observable, multiple observables, or a defined observable pattern.

## Sighting

A `Sighting` records that an observable or relevant intelligence object was actually observed by an identified source or system.

A Sighting answers:

> **Was this actually observed somewhere, by whom, and when?**

A Sighting does not prove malicious activity, successful exploitation, compromise, attribution, or intent.

Where the sighting originates in another ISS product, that product remains authoritative for the underlying observation.

## Assessment

An `Assessment` records a judgment about another Pathfinder object, relationship, assertion, source, or source collection under a defined assessment type.

Assessment authorities may include an external source, Pathfinder processing, or human analyst, but external-source judgments remain Assertions unless explicitly transformed into a Pathfinder Assessment through an attributable process.

Changing an Assessment creates new assessment history or explicit supersession. It does not rewrite the old Assessment.

## Relationship

A `Relationship` states that Pathfinder has information connecting two objects.

General form:

```text
SOURCE OBJECT
      |
 relationship
      |
      v
TARGET OBJECT
```

Relationships are first-class because they require their own identity, provenance, confidence, time bounds, assessment state, and processing lineage.

Directly reported and Pathfinder-derived Relationships remain distinguishable.

## ThreatActor

A `ThreatActor` represents an actor identity or actor construct reported or assessed within threat intelligence.

It may represent a named actor, tracked group, cluster, unknown actor grouping, or source-specific actor identity.

Pathfinder must not assume that two vendor names refer to the same actor merely because reporting suggests similarities. Actor attribution remains an assessment.

## Campaign

A `Campaign` represents a bounded or identified collection of related threat activity.

A Campaign may be source-reported, analyst-defined, or Pathfinder-derived. These origins remain explicit.

Campaign association does not itself prove actor attribution.

## Malware

A `Malware` object represents a malware family, strain, variant, or identified malicious software construct.

The Malware object is not the same thing as an individual file hash.

Multiple hashes may refer to the same malware family. A hash match does not itself prove malware execution.

## Tool

A `Tool` represents software used during threat activity that is not inherently represented as malware.

Examples may include legitimate administration tools, dual-use utilities, offensive-security tools, remote management utilities, and credential tooling.

The presence of a Tool does not imply malicious use.

```text
tool observed != malicious use established
```

## Vulnerability

A `Vulnerability` represents a known or tracked vulnerability identity.

Initial external identifiers may include CVE and vendor advisory identifiers.

Pathfinder may own intelligence relationships involving a Vulnerability, but it does not own whether an Atlas asset is actually vulnerable.

## Technique

A `Technique` represents a behavioral classification such as a MITRE ATT&CK technique or sub-technique.

Pathfinder must distinguish source-reported technique, Pathfinder-mapped technique, locally observed behavior consistent with a technique, and any future confirmed observation state.

Technique classification is not proof that behavior occurred locally.

## Infrastructure

An `Infrastructure` object represents a logical infrastructure construct used to group infrastructure-related observables and relationships.

Examples include command-and-control, phishing, payload-delivery, redirect, hosting, or malware-distribution infrastructure.

Infrastructure is not synonymous with one IP address or domain.

Infrastructure ownership and operation are assessments unless directly established through an applicable source.

## Report

A `Report` represents a coherent intelligence publication or analytical product.

Examples include government advisories, vendor threat reports, incident intelligence reports, analyst reports, and Pathfinder-produced intelligence reports.

A Report may reference many Pathfinder objects. A Report is not itself proof of every assertion it contains.

## Object Ownership Rules

Pathfinder owns:

```text
Pathfinder object identity
Pathfinder normalization
Pathfinder relationships
Pathfinder assessments
Pathfinder provenance records
Pathfinder processing history
Pathfinder lifecycle state
```

Pathfinder does not silently claim ownership of external truth.

External assertions remain attributable to their source. Stronghold observations remain Stronghold-origin observations. FI observations remain FI-origin observations. Atlas asset facts remain Atlas authority.

Pathfinder may preserve, reference, correlate, and interpret those facts without rewriting their originating authority.

## Core Truth Separations

```text
Source != SourceRecord
SourceRecord != Assertion
Assertion != Assessment
SourceRecord != Observable
Observable != Indicator
Observable != Assessment
Indicator != Sighting
Sighting != Assessment
Relationship != Identity
Infrastructure != IP address
Malware != file hash
Tool != malicious activity
Technique != observed behavior
Vulnerability != vulnerable asset
Report != proof
ThreatActor association != attribution
Campaign association != attribution
external identifier != Pathfinder identity
```

## Phase 0.2 Exit Decision

Phase 0.2 is satisfied when Pathfinder accepts the following first-class object model:

```text
Source
SourceRecord
Assertion
Observable
Indicator
Sighting
Assessment
Relationship
ThreatActor
Campaign
Malware
Tool
Vulnerability
Technique
Infrastructure
Report
```

with these governing rules:

1. Every Pathfinder object has Pathfinder-controlled identity.
2. External identifiers are preserved but never substitute for Pathfinder identity.
3. Observable remains semantically neutral.
4. SourceRecord preserves what a source supplied and remains separate from normalized intelligence.
5. Assertion preserves what a source claimed and remains separate from Pathfinder Assessment.
6. Sighting records observation and does not imply compromise.
7. Assessment records judgment and preserves its authority and provenance.
8. Relationship is first-class and preserves provenance, direction, time, and derivation.
9. ThreatActor, Campaign, Malware, Tool, Vulnerability, Technique, and Infrastructure represent intelligence concepts rather than overloaded observable records.
10. Historical source and assessment information survives later reinterpretation.
11. No object type silently implies another object type.
