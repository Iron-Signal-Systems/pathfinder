# Pathfinder Phase 0.14 — MITRE ATT&CK Relationship Model

## Purpose

Pathfinder may use MITRE ATT&CK as a controlled external behavioral classification/reference framework.

ATT&CK can help describe and communicate behavior, organize intelligence, and support interoperability. It is not Pathfinder's truth model and is not required for Pathfinder to recognize, assess, prioritize, or escalate compromise.

> **ATT&CK is a classification aid, not a prerequisite for understanding or proving a compromise.**

> **Pathfinder must never require activity to fit an ATT&CK technique, tactic, or attack sequence before preserving, assessing, or escalating the activity.**

A threat may be highly significant while remaining:

```text
ATT&CK mapping: NOT_MAPPED
```

No confidence, severity, operational relevance, or response priority is reduced merely because no ATT&CK mapping exists.

The native analytical path remains:

```text
Sources
  -> Assertions
  -> Observables
  -> Relationships
  -> Sightings
  -> Assessments
```

ATT&CK is optional classification metadata attached to that intelligence where useful. Pathfinder is never designed as:

```text
Everything
  -> must become ATT&CK
  -> then Pathfinder understands it
```

## External Reference Authority

MITRE ATT&CK owns its external identifiers, names, domains, versions, and framework semantics. Pathfinder retains its own UUIDv7 identities.

```text
ATT&CK ID != Pathfinder ID
```

ATT&CK domain and release/version are preserved for every mapping so historical interpretation is reproducible.

## Version Pinning

ATT&CK-dependent processing uses an explicit dataset version. Upgrading the local ATT&CK dataset does not silently rewrite historical mappings.

```text
ATT&CK dataset upgraded != historical mapping rewritten
```

Pathfinder should be able to distinguish:

```text
classification under ATT&CK version used at the time
```

from:

```text
classification under the currently configured ATT&CK version
```

## Techniques and Sub-Techniques

Techniques and Sub-Techniques remain distinct external concepts. Pathfinder must not manufacture Sub-Technique precision when source or local observation only supports a broader Technique.

> **Do not manufacture precision.**

A defensible parent Technique is preferable to an unsupported specific Sub-Technique.

## Tactics

ATT&CK Tactics are framework classification context and do not prove adversary intent in a particular local event.

```text
Technique belongs to Tactic != local adversary intent established
```

## Taxonomy Versus Intelligence Relationships

ATT&CK taxonomy/reference relationships remain separate from Pathfinder threat-intelligence Relationships. Framework structure such as technique/tactic and sub-technique/parent relationships must not be confused with claims such as:

```text
ThreatActor -> uses -> Technique
Malware -> implements -> Technique
Campaign -> uses -> Technique
```

## Mapping Origins

Every ATT&CK association identifies how it was established.

Initial origins:

```text
ATTACK_NATIVE
SOURCE_REPORTED
PATHFINDER_ASSOCIATED
LOCALLY_SUGGESTED
LOCALLY_CONFIRMED
ANALYST_RECORDED
```

### ATTACK_NATIVE

The relationship is present in the imported official ATT&CK dataset. It remains attributable to ATT&CK and does not become a Pathfinder independent conclusion.

### SOURCE_REPORTED

An external source explicitly supplied the ATT&CK mapping. Pathfinder preserves that mapping as the source's assertion.

### PATHFINDER_ASSOCIATED

The source described behavior but Pathfinder applied the ATT&CK classification. The mapping must preserve method, version, basis, ATT&CK dataset version, and mapping time.

### LOCALLY_SUGGESTED

Local observations are consistent with a Technique but the defined criteria for confirmation are not met.

### LOCALLY_CONFIRMED

The required local observation criteria defined by a versioned Pathfinder mapping contract have been satisfied for the behavior represented by the Technique.

This means only:

> **The observed behavior satisfies our mapping criteria for this ATT&CK technique.**

It does not mean:

```text
the attack is confirmed
compromise is proven
malicious intent is proven
actor identity is proven
campaign identity is proven
response is authorized
```

### ANALYST_RECORDED

An authorized analyst intentionally records an ATT&CK mapping with attributable analyst identity, basis, time, applicable ATT&CK version, and confidence where relevant.

## Classification Is Optional

Pathfinder may represent intelligence such as:

```text
Threat assessment: HIGH confidence
Operational relevance: HIGH
Local observations: confirmed
ATT&CK: NOT_MAPPED
```

This is valid and complete Pathfinder intelligence.

ATT&CK mapping is never a mandatory gate for:

```text
ingestion
preservation
correlation
assessment
conflict detection
operational relevance
incident escalation
candidate response generation
```

## ATT&CK Is Not an Attack Sequence Requirement

Pathfinder must not assume a compromise follows a complete or orderly ATT&CK progression.

Real compromise may occur through short, direct paths such as:

```text
stolen valid credentials
  -> remote access
  -> privileged access
  -> data access
```

or:

```text
vulnerable exposed service
  -> remote code execution
  -> SYSTEM/root
```

The absence of a long or recognizable ATT&CK chain does not reduce the reality or severity of the compromise.

```text
incomplete ATT&CK sequence != incomplete compromise
```

## Groups and Actor Identity

ATT&CK Group constructs remain external threat-actor references. Pathfinder must not automatically merge vendor actors, ATT&CK Groups, aliases, or other actor records because names or mappings overlap.

```text
alias similarity != identity
ATT&CK association != universal attribution certainty
```

Identity resolution remains separate.

## Campaigns

ATT&CK Campaigns may map to Pathfinder Campaigns through explicit external-reference mappings. Group/Campaign relationships remain attributable to ATT&CK rather than becoming unqualified Pathfinder truth.

## Malware and Tools

ATT&CK Software classifications may map to Pathfinder Malware or Tool objects as appropriate. ATT&CK Tool membership does not mean the software is inherently malicious.

```text
ATT&CK Tool != malicious software
```

Pathfinder must distinguish tool existence, tool observation, execution, behavior, malicious use, and actor use.

## Behavioral Relationships

Relationships such as:

```text
Malware -> implements -> Technique
ThreatActor -> uses -> Technique
Campaign -> uses -> Technique
```

remain intelligence claims with provenance.

They do not mean every execution uses the Technique, every incident involving the actor includes it, or observing the Technique identifies the actor or malware.

## Technique Overlap Is Not Attribution

This is a governing rule:

> **Technique overlap is not attribution.**

Pathfinder must prohibit reasoning equivalent to:

```text
Observed Technique X
Actor Alpha is known to use Technique X
therefore Actor Alpha performed the activity
```

The same applies to multiple overlapping Techniques unless an explicit analytical method with independent supporting basis creates an attributable Assessment.

```text
Technique match != actor fingerprint
Technique overlap != actor identity
Technique overlap != malware identity
```

## Procedure Examples and References

ATT&CK procedure examples and references remain provenance/context for why ATT&CK represents a relationship. An ATT&CK citation does not mean Pathfinder independently acquired the cited original source.

Corroboration logic must account for shared upstream reporting so ATT&CK plus a vendor report cited by ATT&CK is not automatically counted as two independent origins.

## Local ISS Observations

Stronghold, FI, Atlas, and future ISS systems retain their own domain authority. Pathfinder may apply ATT&CK classifications to their data, but the classification never rewrites the originating observation.

```text
Stronghold/FI observation != ATT&CK conclusion
ATT&CK conclusion != originating observation rewritten
```

Stronghold remains authoritative for network observations. FI remains authoritative for file/process observations. Atlas remains authoritative for asset/environment context. Pathfinder owns its behavioral interpretation.

## Applicability Versus Observation

A Technique may be applicable to an asset or environment without being observed.

```text
Technique applicable != Technique observed
```

Atlas context may increase relevance without proving behavior occurred.

## Ambiguous Mapping

Behavior may reasonably map to multiple Techniques. Pathfinder allows ambiguity and does not choose one solely because a schema or UI prefers a singular result.

Confidence belongs to an attributable Assessment, not to the Technique itself.

## ATT&CK Lifecycle

Imported ATT&CK objects may be added, modified, deprecated, revoked, renamed, split, or replaced. Those external lifecycle changes do not erase historical Pathfinder mappings.

Reprocessing under a newer ATT&CK release creates new lineage instead of modifying earlier mappings.

## Detection Content

ATT&CK Detection Strategies, Analytics, Data Components, and similar defensive references are useful classification/reference material. They are not automatically Pathfinder detections or proof that local telemetry exists.

```text
ATT&CK Detection Strategy != Pathfinder detector
ATT&CK Analytics != local detection result
Data Component != telemetry available
```

Future detection or hunting recommendations require separate contracts.

## No ATT&CK-Driven Enforcement

ATT&CK classification never creates direct enforcement authority. A Technique mapping may influence an investigation or candidate action, but downstream authorization remains separate.

```text
ATT&CK mapping != enforcement authority
```

## Truth Separations

```text
ATT&CK Technique              != local observation
ATT&CK mapping                != Technique occurred
ATT&CK mapping                != compromise required/proven
ATT&CK sequence               != required compromise path
NOT_MAPPED                    != low significance
SOURCE_REPORTED               != Pathfinder-created mapping
PATHFINDER_ASSOCIATED         != source-reported ATT&CK ID
LOCALLY_SUGGESTED             != locally confirmed
LOCALLY_CONFIRMED             != attack confirmed
LOCALLY_CONFIRMED             != compromise
LOCALLY_CONFIRMED             != malicious intent
Technique confirmed           != actor attribution
Technique overlap             != actor identity
Technique overlap             != malware identity
ATT&CK Group                  != universal actor identity
ATT&CK Tool                   != malicious software
ATT&CK citation               != independently acquired source
ATT&CK current version        != historical ATT&CK version
ATT&CK Detection Strategy     != local detector
Technique applicable          != Technique observed
ATT&CK mapping                != enforcement authority
```

## Phase 0.14 Exit Decision

Phase 0.14 is satisfied when Pathfinder accepts ATT&CK as optional external classification/reference metadata; never requires ATT&CK coverage or attack-sequence conformance before recognizing or escalating compromise; keeps ATT&CK identities, versions, taxonomy, and source relationships distinct from Pathfinder authority; preserves mapping origin and historical version; separates suggested and confirmed local behavioral mappings; prohibits technique-overlap attribution; allows NOT_MAPPED intelligence without downgrade; and never turns ATT&CK classification into compromise, intent, identity, or enforcement authority.
