# Pathfinder Phase 0.4 — Assertion and Assessment Model

## Purpose

Pathfinder must distinguish an attributable claim from Pathfinder's interpretation or judgment of that claim.

> **Record the claim before judging the claim.**

Pathfinder preserves:

```text
SOURCE RECORD
    what Pathfinder received

ASSERTION
    what the source claimed

NORMALIZED INTELLIGENCE
    what Pathfinder understood the claim to refer to

ASSESSMENT
    what Pathfinder or an authorized analyst concluded about it
```

These are separate states of knowledge.

## Core Truth Separation

```text
source said X != Pathfinder believes X
Pathfinder believes X != X is objectively proven
source confidence != Pathfinder confidence
assertion != assessment
assessment != authorization
```

## Assertion

An `Assertion` is an attributable claim extracted from or recorded against a specific SourceRecord.

An Assertion answers:

> **What did this source claim?**

Examples include a vendor reporting an IP as command-and-control infrastructure, a government advisory reporting active exploitation of a vulnerability, or a threat report stating that an actor uses a malware family.

Pathfinder can establish with certainty that the source made the recorded assertion even when Pathfinder cannot establish that the assertion itself is true.

### Assertion Authority

The authority for an Assertion is the source that made the claim.

Pathfinder is authoritative for its record that the assertion was received and interpreted. Pathfinder does not silently assume authority for the truth of the assertion.

### Assertion Identity

Every Assertion receives a Pathfinder-controlled identity.

```text
assertion_id = UUIDv7
```

External claim identifiers remain separate.

### Assertion Conceptual Fields

```text
assertion_id
source_id
source_record_id
subject
predicate / assertion_type
object or asserted_value
asserted_at / not_known
effective_from / not_known
effective_until / not_known
source_confidence / not_known
source_locator
extraction_method
extraction_version
created_at
lifecycle_state
```

### Source Locator

An Assertion should retain enough information to locate the basis of the claim within the SourceRecord where practical.

Examples include JSON pointer, XML path, STIX object identifier, feed-record field, document section, record offset, or structured source-object reference.

### Assertion Extraction Method

Pathfinder distinguishes how an Assertion was obtained.

Initial conceptual values:

```text
source_structured
parser_extracted
analyst_recorded
```

Machine extraction must not be represented as though the source itself supplied Pathfinder's normalized vocabulary.

### Historical Assertions

An Assertion is not silently edited because later information changes.

If a source later corrects, withdraws, or changes a claim, Pathfinder records that later event explicitly and preserves the original historical assertion.

```text
source corrected assertion != source never made original assertion
```

## Assertion Versus Normalized Intelligence

Assertions preserve attributable claims. Pathfinder objects preserve normalized intelligence concepts.

Example:

```text
SourceRecord
    |
    v
Assertion
    "203.0.113.17 is command-and-control infrastructure"
    |
    v
normalized objects
    |
    +-- Observable: 203.0.113.17
    +-- Infrastructure: C2
    +-- Relationship: associated_with
```

The normalized relationship does not replace the Assertion. The Assertion explains why that relationship exists.

Multiple Assertions may support the same normalized intelligence relationship without losing provenance.

## Assessment

An `Assessment` is a Pathfinder-recorded judgment about an intelligence object, relationship, assertion, source, source collection, or other explicitly supported subject.

An Assessment answers:

> **What conclusion has Pathfinder or an authorized analyst reached about this information?**

Examples include likely malicious, strongly supported relationship, stale assertion, possible attribution, locally relevant threat intelligence, or likely false positive.

### Assessment Authority

Every Assessment identifies who or what made the judgment.

Initial authority classes:

```text
human_analyst
pathfinder_process
```

External-source judgments remain Assertions attributed to the external source. They do not automatically become Pathfinder Assessments.

```text
Vendor says malicious = Assertion
Pathfinder assesses malicious = Assessment
```

### Assessment Subject

An Assessment may target an explicitly supported Pathfinder entity such as:

```text
Source
SourceCollection
Observable
Indicator
Assertion
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

### Assessment Identity

Every Assessment receives a Pathfinder-controlled identity.

```text
assessment_id = UUIDv7
```

### Assessment Conceptual Fields

```text
assessment_id
subject_id
assessment_type
assessment_value
assessment_authority
assessment_method
method_version / not_applicable
confidence
basis
assessed_at
rationale / not_recorded
supersedes / not_applicable
lifecycle_state
```

Exact confidence representation is defined in Phase 0.6.

## Typed Assessments

Pathfinder must not create an unrestricted authoritative `assessment = arbitrary text` field.

Assessment types and allowed structured values must be explicitly defined. Free-form analyst explanation may accompany an Assessment but does not replace structured assessment semantics.

Potential assessment domains include:

```text
threat_classification
relationship_validity
attribution
operational_relevance
false_positive_likelihood
source_claim_quality
source_reliability
```

## Assessment Basis

An Assessment should identify what information supported the judgment.

Potential basis references include Assertions, Sightings, Relationships, Indicators, SourceRecords, other Assessments, local observations, and analyst reasoning.

Pathfinder must preserve the difference between several basis records and several independent corroborating sources.

## Machine Assessments

Automated Pathfinder assessments must identify the process that produced them, including process identity, algorithm/rule identity, version, basis, inputs, and processing time.

Pathfinder must not store unexplained machine scores or confidence values.

A future statistical or machine-learning system remains subject to the same provenance requirements.

```text
AI said so
```

is not sufficient provenance.

## Human Analyst Assessments

Human assessments preserve the authorized analyst identity or attributable analyst principal, what was assessed, when it was assessed, the conclusion, confidence where applicable, and basis where applicable.

An analyst may confirm, dispute, supersede, reject, qualify, or annotate according to the later authority model.

Human authority does not rewrite preserved source material.

## Confidence

Confidence belongs to the assertion or assessment whose certainty is being described.

Pathfinder keeps distinct:

```text
source-reported confidence
Pathfinder assessment confidence
analyst confidence
source reliability
corroboration
```

Source confidence is never silently copied into Pathfinder confidence.

## Negative Assertions

An explicit negative claim is materially different from absence of information.

```text
Source asserts Domain X is benign
    !=
Source has no record for Domain X
```

Therefore:

```text
no malicious assertion != benign assertion
no indicator != benign
no sighting != not observed anywhere
no assessment != safe
```

## Conflicting Assertions

Pathfinder allows contradictory Assertions to coexist.

It does not resolve disagreement by silently averaging opposing claims into a numeric score.

A later Assessment may evaluate the conflict, but the original conflicting Assertions remain visible and attributable.

## Supersession

A new Assessment may supersede an earlier Assessment. The earlier Assessment remains historical.

```text
current assessment changed != historical assessment rewritten
```

## Withdrawal and Revocation

Assertions and Assessments may later acquire lifecycle states such as withdrawn, revoked, superseded, disputed, or expired. Exact lifecycle behavior belongs to Phase 0.9.

Regardless of later lifecycle state, historical intelligence remains historical intelligence.

## Relationship to Indicators

An external source report that an Observable has threat significance first creates an attributable Assertion. Pathfinder may then create or associate an Indicator according to the applicable Indicator contract.

The Indicator remains traceable to the Assertion that supplied its basis.

## Relationship to Sightings

A Sighting may contribute to an Assessment but does not automatically determine that Assessment.

For example, a Stronghold sighting of a destination already present in Pathfinder intelligence may support an assessment of local relevance while remaining only an observation with respect to maliciousness.

## No Silent Promotion

Pathfinder must not silently promote information through levels of certainty.

Forbidden implicit transitions include:

```text
SourceRecord -> established fact
Assertion -> Assessment
Observable -> Indicator
Indicator -> malicious
Sighting -> compromise
Relationship -> identity
Association -> attribution
high confidence -> enforcement action
```

Every transition that adds meaning must have an attributable processing step or authorized assessment.

## Current Versus Historical Understanding

Pathfinder may know more today than it knew yesterday. That must not rewrite what Pathfinder knew yesterday.

Later information must not be projected backward into earlier assessments or processing state.

```text
current understanding != historical understanding
```

## Processing History

Material transitions should preserve enough history to answer which SourceRecord produced an Assertion, which parser extracted it, which normalization rules were applied, which objects were created or linked, which Assertions supported an Assessment, whether the Assessment was human or machine generated, what method/version produced it, and whether it was later superseded.

## Common Truth Separations

```text
SourceRecord != Assertion
Assertion != fact
Assertion != Assessment
external-source judgment != Pathfinder Assessment
source confidence != Pathfinder confidence
source reliability != assertion truth
machine Assessment != human Assessment
machine Assessment != analyst approval
analyst note != structured Assessment
Assessment != objective proof
Assessment != authorization
Assessment != enforcement
supporting records != independent sources
duplicate assertion != corroboration
negative assertion != absence of assertion
no malicious assertion != benign
no Assessment != safe
conflicting Assertions != no intelligence
superseded Assessment != deleted Assessment
withdrawn Assertion != assertion never existed
new understanding != historical knowledge rewritten
Relationship supported != Relationship proven
actor association != attribution
Sighting != Assessment
Sighting != compromise
high confidence != enforcement authority
```

## Phase 0.4 Exit Decision

Phase 0.4 is satisfied when Pathfinder accepts:

1. `Assertion` as a first-class object.
2. Assertions represent attributable source claims.
3. Assessments represent Pathfinder-recorded judgments.
4. External-source judgments remain Assertions and do not silently become Pathfinder Assessments.
5. Human and machine Assessments remain distinguishable.
6. Assertions and Assessments have independent Pathfinder identities.
7. Assertions remain traceable to SourceRecords and, where practical, exact source locations.
8. Normalized objects and Relationships remain traceable to supporting Assertions.
9. Assessment basis remains explicit.
10. Confidence remains attached to the authority that expressed it.
11. Source confidence, Pathfinder confidence, source reliability, and corroboration remain separate.
12. Conflicting Assertions may coexist.
13. Negative Assertions remain distinct from absence of information.
14. Superseded, withdrawn, disputed, or changed understanding does not rewrite historical records.
15. No information is silently promoted from source material to conclusion or from conclusion to enforcement.
