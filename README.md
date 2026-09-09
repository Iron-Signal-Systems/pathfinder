# Pathfinder

**Pathfinder by Iron Signal Systems**

Pathfinder is a threat-intelligence project focused on preserving source truth, separating observation from assertion and assessment, and turning external and internal threat information into operationally useful intelligence without manufacturing certainty.

Pathfinder is pre-release and under active development.

## Mission

Pathfinder is intended to answer four practical questions:

1. **What do we know about a threat?**
2. **Where did that knowledge come from?**
3. **How confident are we in it?**
4. **Does it matter to systems we actually operate?**

The project is not intended to become a large undifferentiated IOC bucket.

The central design principles include:

> **Record the claim before judging the claim.**

> **A sighting is an observation, not a conclusion.**

> **Correlation is not identity.**

> **High confidence is still not authorization.**

Pathfinder should allow an operator to work backward from an intelligence conclusion to the source records, assertions, observations, relationships, and processing decisions that produced it.

## Product Boundary

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

Pathfinder is authoritative for its threat-intelligence records, assertions, relationships, provenance, assessments, processing history, and intelligence lifecycle. It is not automatically authoritative for the real-world truth of every external assertion.

It does not silently become:

```text
a firewall
an EDR
an endpoint enforcement product
an asset authority
a vulnerability scanner
a packet-capture authority
an autonomous remediation engine
```

Pathfinder intelligence may inform downstream action, but intelligence alone does not create enforcement authority.

## Categories of Truth

Pathfinder preserves three distinct categories:

```text
WHAT THE SOURCE SAID
    preserved source material / attributable assertion

WHAT WAS OBSERVED
    sightings and observations from an identified source or system

WHAT PATHFINDER UNDERSTOOD
    normalized, correlated, enriched, or assessed intelligence
```

These must remain distinguishable.

Current knowledge must not rewrite historical knowledge.

## Core Intelligence Model

The initial conceptual model includes:

```text
Source
SourceCollection
RetrievalEvent
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

### SourceRecord

A SourceRecord preserves what Pathfinder actually received from a source or source collection.

It is separate from the normalized intelligence derived from it.

### Assertion

An Assertion is an attributable claim made by a source.

For example:

```text
Vendor A reports that 203.0.113.17 is C2 infrastructure.
```

Pathfinder can authoritatively record that Vendor A made that assertion without silently claiming the assertion itself is proven true.

### Observable

An Observable is something that can be observed, referenced, or matched.

Examples include:

```text
IP address
domain
URL
SHA-256
email address
certificate fingerprint
```

An Observable is semantically neutral.

### Indicator

An Indicator represents operational threat significance associated with one or more observables or observable patterns and remains traceable to the assertions, assessments, and provenance supporting that meaning.

An Indicator is not simply an Observable with `malicious=true`.

### Sighting

A Sighting records that an observable or intelligence object was actually observed by an identified source or system at a particular time.

A Sighting does not itself establish compromise, malicious intent, or attribution.

### Assessment

An Assessment records a Pathfinder or authorized analyst judgment and preserves who or what made it, the basis for it, and confidence where applicable.

External-source judgments remain Assertions unless an attributable Pathfinder process or analyst produces a separate Assessment.

### Relationship

Relationships are first-class intelligence objects.

Examples may include:

```text
threat actor -> uses -> malware
threat actor -> associated with -> campaign
malware -> communicates with -> infrastructure
malware -> implements -> technique
campaign -> targets -> sector
observable -> sighted on -> external system context
```

Correlation does not automatically establish identity, ownership, control, or maliciousness.

## Provenance

Provenance is a first-class requirement.

Pathfinder preserves distinctions among Source, SourceCollection, RetrievalEvent, SourceRecord, origin source, and delivery source.

Where applicable, Pathfinder preserves:

```text
provider
feed or collection
origin source
delivery source
source record identifier
source publication time
source observation time
retrieval time
receipt time
source location
source marking
source format
source hash
parser version
normalizer version
processing time
```

Where the governing source-preservation contract requires it, the original source representation is retained before transformation.

A later parser or interpretation may produce new derived intelligence, but it must not rewrite the original source to make it appear that later understanding existed at the time of ingestion.

Redistribution is not automatically independent corroboration.

## Intelligence Processing Direction

The intended high-level processing path is:

```text
SOURCE
  |
  v
receive
  |
  v
preserve source
  |
  v
validate
  |
  v
parse
  |
  v
record Assertion
  |
  v
normalize
  |
  v
deduplicate
  |
  v
correlate
  |
  v
enrich
  |
  v
assess
  |
  v
publish / query / export
```

Failure or partial processing must remain visible at each meaningful boundary.

## Confidence, Reliability, and Corroboration

Pathfinder does not reduce intelligence quality to one unexplained risk score.

Pathfinder keeps separate:

```text
source reliability
source-reported confidence
Pathfinder assessment confidence
analyst confidence
corroboration
age / recency
operational relevance
```

Initial qualitative reliability and Pathfinder confidence values are:

```text
HIGH
MODERATE
LOW
NOT_ASSESSED
```

For source-reported confidence normalization, `NOT_REPORTED` and `UNMAPPED` remain distinct.

Several feeds repeating one original report do not automatically constitute independent corroboration. Pathfinder distinguishes delivery sources, origin sources, and independently originated support.

Local Sightings may increase operational relevance without independently proving maliciousness.

## Conflicting Intelligence

Pathfinder preserves disagreement rather than hiding it behind an averaged score.

Two high-confidence sources may disagree about the same observable or relationship. That conflict remains visible and attributable to the contributing sources.

Potential lifecycle/assessment states include:

```text
active
aging
expired
revoked
superseded
disputed
conflicted
```

Exact lifecycle vocabulary is frozen through later contracts.

## Time and History

Pathfinder preserves distinctions among:

```text
source publication time
source observation time
Pathfinder retrieval time
Pathfinder receipt time
Pathfinder processing time
local sighting time
assessment time
supersession time
expiration time
```

Receipt time is not publication time. Publication time is not necessarily observation time. Later receipt of historical intelligence must not make that intelligence appear newly observed.

## STIX and TAXII

Pathfinder is intended to support STIX 2.1 and TAXII 2.1 interoperability at defined system boundaries.

The initial architectural direction is:

```text
STIX / TAXII
      |
      v
boundary adapters
      |
      v
Pathfinder-native intelligence model
      |
      v
boundary adapters
      |
      v
STIX / TAXII
```

STIX is not intended to dictate Pathfinder's internal storage or truth model.

Valid STIX syntax does not prove that intelligence is true, trusted, or relevant. Successful TAXII transport does not prove that received intelligence should be accepted.

Translation loss or unsupported semantics must remain visible rather than being silently discarded.

## MITRE ATT&CK

Pathfinder is intended to support ATT&CK relationships and classifications where useful.

An ATT&CK technique association is not automatically proof that the technique occurred in a local environment.

Source-reported mappings, Pathfinder-derived mappings, and locally observed behavior remain distinguishable.

## Storage Direction

The initial direction is a relational data model rather than introducing graph infrastructure before measurement and requirements justify it.

PostgreSQL is the initial database direction.

Relationships remain first-class even when implemented relationally.

A graph database may be evaluated later if real Pathfinder workloads demonstrate a need.

## Implementation Direction

The initial implementation direction remains intentionally small:

```text
Go service
PostgreSQL
one external intelligence collector
manual analyst entry
source preservation
observable normalization
relationships
sightings
assessments
basic query API
```

The project should not begin by ingesting dozens of feeds, building a polished UI, adding AI analysis, or implementing automatic enforcement.

## ISS Integration Direction

Pathfinder is an independent ISS product boundary.

Potential relationships include:

```text
                 Pathfinder
                     |
        +------------+-------------+
        |            |             |
        v            v             v
      Atlas       Stronghold       FI
   asset context    network       file
                  observations   observations
```

### Atlas

Atlas can provide environmental and asset context. Pathfinder can add threat context without becoming Atlas's asset authority.

### Stronghold

Stronghold can provide authoritative network observations and decision history. Pathfinder can correlate those observations with external intelligence.

A Pathfinder threat conclusion does not itself authorize Stronghold enforcement.

### FI

FI can provide authoritative file observations such as hashes, signer/certificate information, paths, hosts, and file activity.

Pathfinder can correlate those observations with malware and threat intelligence while preserving the distinction between a hash match and proof of execution or compromise.

### Guidon

Guidon may eventually consume or contribute narrowly defined intelligence where recovery or incident context benefits from it, but Pathfinder must not introduce Guidon as a runtime dependency without an explicit future contract.

## Enforcement Boundary

Pathfinder may eventually produce candidate recommendations such as:

```text
block candidate
investigation candidate
detection candidate
priority patch candidate
hunting candidate
```

A recommendation is not an enforcement action.

A future downstream enforcement workflow preserves its own authorization, validation, simulation, approval, commit, audit, and rollback boundaries.

> **High confidence is still not authorization.**

## Initial Source Strategy

Pathfinder should begin with a small number of meaningfully different sources rather than a large feed count.

Useful initial source classes include:

```text
government / authoritative intelligence
vendor intelligence
community intelligence
internal ISS observations
```

The objective is to prove the intelligence model, provenance, conflict handling, lifecycle, and correlation behavior before expanding collection breadth.

## Phase 0 Contracts

Current governing design documents include:

```text
docs/INTELLIGENCE-MISSION.md
    Phase 0.1 — mission, consumers, and product authority

docs/CORE-INTELLIGENCE-MODEL.md
    Phase 0.2 — first-class intelligence objects

docs/OBSERVABLE-MODEL.md
    Phase 0.3 — observable types and canonicalization

docs/ASSERTION-ASSESSMENT-MODEL.md
    Phase 0.4 — attributable claims and Pathfinder judgments

docs/SOURCE-PROVENANCE-MODEL.md
    Phase 0.5 — source identity, provenance, and lineage

docs/CONFIDENCE-RELIABILITY-CORROBORATION.md
    Phase 0.6 — reliability, confidence, independence, and corroboration
```

## Engineering Direction

Pathfinder follows the Iron Signal Systems engineering philosophy:

- explicit boundaries;
- truthful state reporting;
- minimal unjustified dependencies;
- narrow inspectable implementations;
- failure behavior designed with the success path;
- no manufactured certainty;
- no hidden transfer of authority between systems; and
- no repository writes without explicit authorization for the specific action.

See [`AGENTS.md`](AGENTS.md) for repository-wide contributor and coding-agent rules.

See [`ROADMAP.md`](ROADMAP.md) for the current implementation sequence.

## Security

Please see [`SECURITY.md`](SECURITY.md) for vulnerability reporting and project security scope.

## License

Pathfinder is proprietary source-available software, not open-source software.

See [`LICENSE`](LICENSE) for permitted evaluation use and restrictions.
