# Pathfinder Phase 0.6 — Confidence, Reliability, and Corroboration

## Purpose

Pathfinder must express how strongly intelligence is supported without collapsing different kinds of uncertainty into one score.

> **Confidence describes a judgment. Reliability describes a source. Corroboration describes support. They are not interchangeable.**

Pathfinder preserves these as separate dimensions.

```text
source reliability
        !=
assertion confidence
        !=
Pathfinder assessment confidence
        !=
corroboration
        !=
age / recency
        !=
local observation
```

## No Universal Risk Score

Pathfinder v1 does not define a universal numeric intelligence score such as `risk_score = 87`, `threat_score = 72`, or `confidence = 91` unless a later explicit contract defines exactly what the number means, how it is calculated, which inputs contribute, how missing inputs behave, how conflicts affect it, and how the calculation is versioned.

A number without those semantics creates false precision.

Pathfinder instead exposes underlying dimensions directly, such as source reliability, source-reported confidence, Pathfinder confidence, independent origin sources, conflicting sources, local sightings, and relevant observation times.

## Source Reliability

`Source Reliability` describes Pathfinder's assessment of the source's demonstrated ability to provide accurate and useful intelligence.

It does not describe whether one particular assertion is true.

A highly reliable source may publish an early or uncertain assertion. A historically weak source may still publish an assertion that later proves correct.

### Initial Reliability Levels

```text
HIGH
MODERATE
LOW
NOT_ASSESSED
```

These values are deliberately coarse.

`NOT_ASSESSED` is not the same as `LOW`. Unknown reliability is not demonstrated poor reliability.

### Reliability Scope

Reliability may apply at Source or SourceCollection scope.

A source may have one collection with HIGH reliability and another collection with MODERATE reliability. A more specific valid SourceCollection reliability assessment may therefore be more relevant than a general Source assessment.

This refines the Assessment model so that defined Assessments may target both `Source` and `SourceCollection`.

### Reliability Is Historical

Source reliability may change over time. Pathfinder preserves those changes as historical Assessments.

```text
current source reliability != historical source reliability
```

When reconstructing an earlier Assessment, Pathfinder should be able to determine the reliability information available at that time.

### Reliability Basis

Potential reliability factors include historical accuracy, quality of sourcing, methodology transparency, correction behavior, consistency, timeliness, provenance quality, independence of collection, known false-positive history, and domain-specific expertise.

Pathfinder v1 does not prescribe a mathematical weighting formula.

## Source-Reported Confidence

An external source may express confidence in its own Assertion using labels, prose, or numbers.

Pathfinder preserves what the source actually reported.

Conceptually:

```text
source_confidence_original
source_confidence_normalized
confidence_mapping_profile
```

The source-provided representation remains preserved even when Pathfinder maps it to a normalized field.

### Normalized Source Confidence

Where a source-specific mapping contract exists, Pathfinder may map external confidence into:

```text
HIGH
MODERATE
LOW
NOT_REPORTED
UNMAPPED
```

`NOT_REPORTED` means the source did not provide a confidence expression.

`UNMAPPED` means the source did provide one, but Pathfinder does not have a valid mapping rule for it.

```text
NOT_REPORTED != UNMAPPED
```

Pathfinder does not guess.

### Confidence Mapping

Confidence normalization is source-specific when source semantics differ.

The label `High` from two different providers does not automatically mean the same thing.

Where mapping exists, Pathfinder preserves source confidence value, mapping profile, mapping version, and normalized value.

A later mapping change creates new processing lineage rather than rewriting the historical interpretation.

## Pathfinder Assessment Confidence

A Pathfinder Assessment may carry Pathfinder confidence.

Initial levels:

```text
HIGH
MODERATE
LOW
NOT_ASSESSED
```

Confidence belongs to the specific Assessment, not permanently to the subject object.

The same Observable, Relationship, or other subject may have multiple Assessments with different confidence because they address different claims.

## Confidence Is Not Probability

Pathfinder confidence levels are qualitative unless a future explicit contract defines probabilistic semantics.

`HIGH` does not automatically mean 90%, 95%, or greater than 80%. `MODERATE` does not automatically mean 50%.

Do not silently translate qualitative confidence into numerical probability.

## Analyst Confidence

A human analyst may assign confidence to an Assessment. That confidence belongs to the analyst Assessment and remains attributable to the analyst principal.

Different analysts may reach different confidence judgments. Both remain historical records.

## Machine Confidence

A Pathfinder process may produce an Assessment with confidence.

Machine-generated confidence must preserve process identity, algorithm/rule identity, version, basis, inputs, and processing time.

Unexplained machine confidence is prohibited.

```text
machine HIGH != analyst confirmed
```

## Corroboration

Corroboration describes independent support for the same or materially compatible claim.

It is not simply a count of records.

The key question is:

> **How many meaningfully independent origins support this claim?**

Pathfinder distinguishes:

```text
records
delivery sources
origin sources
independent origin sources
```

A government report redistributed by three vendors is not four independent sources.

## Independence Classification

When evaluating potential corroboration, Pathfinder may use:

```text
INDEPENDENT
SHARED_ORIGIN
POSSIBLY_SHARED_ORIGIN
NOT_KNOWN
```

`NOT_KNOWN` must never silently count as `INDEPENDENT`.

## Corroboration Must Match the Claim

Two intelligence records are corroborating only with respect to what they actually support.

A source stating that an IP belongs to a hosting provider does not directly corroborate another source's claim that the IP is command-and-control infrastructure.

Pathfinder distinguishes:

```text
direct corroboration
supporting context
derived support
```

Related context is not automatically direct corroboration.

## Local Sightings and Corroboration

Local observation is operationally important, but its meaning remains precise.

If external intelligence reports an IP as C2 infrastructure and Stronghold observes a local system communicating with that IP, Stronghold corroborates that the IP was contacted locally. It does not independently prove that the IP is C2 infrastructure unless the observation contains additional evidence directly supporting that claim.

```text
local sighting != corroboration of maliciousness
```

A local sighting may substantially increase operational relevance while leaving threat classification dependent on external intelligence.

## Repeated Sightings

Repeated sightings do not become independent corroboration.

Five hundred observations of the same IP are five hundred observations, not five hundred independent confirmations that the IP is malicious.

Frequency and corroboration are separate dimensions.

## Conflicting Intelligence

Corroboration analysis preserves contradictory information.

Pathfinder may report independent supporting origins and independent conflicting origins separately.

It must not silently subtract one from the other, average them into a percentage, or manufacture a synthetic conclusion unless a future explicitly defined analytical model supports that calculation.

## Corroboration Does Not Automatically Set Confidence

More independent support may influence an Assessment but does not mechanically dictate its confidence level.

Three independent low-quality sources do not necessarily justify the same confidence as one highly reliable source with direct technical artifacts.

```text
corroboration count != confidence
```

The relationship between corroboration and confidence belongs to a defined assessment method.

## Age and Recency

Age is not confidence.

An old assertion may remain historically correct. A recent assertion may still be wrong.

```text
old != false
recent != true
```

Age may affect operational usefulness or lifecycle processing, but it does not automatically rewrite confidence.

Different intelligence types age differently. Pathfinder must not apply one universal aging formula to residential IP reputation, domain registration, malware hashes, threat actor technique use, and vulnerability exploitation reporting without an explicit contract.

## Confidence at Assessment Time

An Assessment's confidence reflects what was known when the Assessment was made.

If later independent corroboration or local sightings lead to a new higher-confidence Assessment, the earlier lower-confidence Assessment remains historical.

```text
current confidence != historical confidence
```

## Confidence Basis

Every material Pathfinder confidence judgment should be able to identify its basis, which may include source Assertions, Source reliability Assessments, independent corroboration, conflicting Assertions, technical artifacts, Sightings, relationship support, or analyst reasoning.

A confidence judgment must not exist merely because the number of records was large.

## Missing Information

Pathfinder preserves the difference between `LOW` and `NOT_ASSESSED` confidence.

Likewise, `no corroboration found` and `corroboration search incomplete` are different.

```text
no corroboration located != no corroboration exists
```

## Search Coverage and Corroboration

Pathfinder may make strong statements about corroboration only when relevant source/search coverage is known.

If coverage is incomplete, the system should state that no corroborating source was identified within the currently processed coverage rather than claiming no corroborating source exists.

This distinction should be structural in the data model and API, not merely prose in the UI.

## Corroboration Analysis Provenance

Any derived corroboration result should preserve:

```text
subject / claim evaluated
supporting assertion IDs
conflicting assertion IDs
origin source IDs
delivery source IDs
independence classifications
analysis method
analysis version
analysis time
```

## Reliability Versus Authenticity

Source reliability and source authenticity are separate.

```text
authenticated API != reliable intelligence
cryptographically signed report != accurate report
reliable source != every assertion true
```

Authentication asks whether information came through the expected identity/path.

Reliability asks how much basis Pathfinder has for trusting the source's historical reporting quality.

Confidence asks how strongly a particular authority supports a particular judgment.

## Confidence Versus Severity

Threat severity is not confidence.

A potential impact may be CRITICAL with LOW confidence, or LOW with HIGH confidence.

Pathfinder v1 does not silently combine severity and confidence into one number.

## Confidence Versus Relevance

Operational relevance is also separate.

Threat classification confidence may be HIGH while local relevance is LOW because no known local asset uses the affected technology.

Threat classification confidence may be MODERATE while local relevance is HIGH because local telemetry observed the infrastructure.

## Common Truth Separations

```text
source reliability != assertion truth
source reliability != source authenticity
source-reported confidence != Pathfinder confidence
Pathfinder confidence != analyst confidence
confidence != probability
confidence != severity
confidence != relevance
confidence != authorization
confidence != enforcement
HIGH confidence != proven
LOW confidence != false
NOT_ASSESSED != LOW
NOT_REPORTED != UNMAPPED
record count != source count
delivery source count != origin source count
origin source count != independent source count
duplicate reporting != corroboration
redistribution != independent corroboration
many sightings != many independent sources
local sighting != maliciousness corroborated
same observable != same claim
supporting context != direct corroboration
corroboration count != confidence
recent != true
old != false
current confidence != historical confidence
no corroboration found != no corroboration exists
incomplete coverage != complete corroboration search
signed source content != accurate source content
authenticated source != reliable source
threat severity != intelligence confidence
operational relevance != threat classification
```

## Phase 0.6 Exit Decision

Phase 0.6 is satisfied when Pathfinder accepts:

1. Source reliability, assertion confidence, Pathfinder confidence, analyst confidence, corroboration, age, and relevance as separate concepts.
2. No universal Pathfinder threat/risk score in the initial model.
3. Initial qualitative reliability values of `HIGH`, `MODERATE`, `LOW`, and `NOT_ASSESSED`.
4. Initial Pathfinder confidence values of `HIGH`, `MODERATE`, `LOW`, and `NOT_ASSESSED`.
5. External source confidence is preserved in its original form before any normalization.
6. Source-confidence mappings are source-specific, versioned, and never guessed.
7. `NOT_REPORTED` and `UNMAPPED` remain distinct.
8. Reliability may be assessed at Source or SourceCollection scope.
9. Reliability Assessments are historical and attributable.
10. Corroboration counts independent origins rather than merely records or delivery sources.
11. Independence may be `INDEPENDENT`, `SHARED_ORIGIN`, `POSSIBLY_SHARED_ORIGIN`, or `NOT_KNOWN`.
12. Unknown source independence never silently counts as independent corroboration.
13. Corroboration applies to the specific claim being supported.
14. Supporting context remains distinguishable from direct corroboration.
15. Local Sightings establish local observation/relevance but do not automatically corroborate maliciousness.
16. Repeated observations do not become repeated independent corroboration.
17. Conflicting support remains visible.
18. Corroboration does not mechanically determine confidence.
19. Confidence reflects information available at assessment time and historical confidence is not rewritten later.
20. Age and recency remain separate from confidence.
21. Search/index incompleteness remains visible when evaluating corroboration.
22. Derived corroboration analysis preserves its supporting records, source origins, independence judgments, method, and version.
23. Confidence never silently becomes authorization or enforcement.
