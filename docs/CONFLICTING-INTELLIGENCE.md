# Pathfinder Phase 0.10 — Conflicting Intelligence Behavior

## Purpose

Threat intelligence will disagree.

Pathfinder must treat disagreement as meaningful intelligence rather than as a data-quality problem that must always be collapsed into one answer.

The governing principle is:

> **Conflicting intelligence remains visible, attributable, and unresolved until Pathfinder has a defensible basis to resolve it.**

Pathfinder must not manufacture consensus.

```text
conflict
    != failure

disagreement
    != missing data

majority
    != truth

higher confidence
    != automatic winner
```

## What Constitutes Conflict

A conflict exists when two or more relevant Assertions or Assessments make materially incompatible claims about the same subject and applicable context.

Example:

```text
Assertion A
    Source A
    IP X is malicious

Assertion B
    Source B
    IP X is benign
```

This is a direct classification conflict.

Another example:

```text
Assertion A
    Actor Alpha operates Infrastructure X

Assertion B
    Actor Beta operates Infrastructure X
```

These claims may conflict if they refer to the same applicable time and operational context.

However:

```text
Actor Alpha used Infrastructure X in January

Actor Beta used Infrastructure X in June
```

may both be true.

Pathfinder must examine time and scope before declaring conflict.

## Conflict Is Contextual

Two statements are not conflicting merely because their values differ.

Conflict evaluation must consider:

```text
subject
assertion type
scope
effective time
source context
relationship semantics
assessment domain
```

Example:

```text
Source A:
    IP X was malicious on March 1

Source B:
    IP X was benign on September 1
```

These are not necessarily contradictory.

Infrastructure changes ownership and use.

Likewise:

```text
Source A:
    Domain X associated with Campaign A

Source B:
    Domain X associated with Campaign B
```

may both be valid.

Association is not exclusive unless the relationship contract says otherwise.

## Conflict Object

Conflict itself is a first-class Pathfinder object.

Conceptual object:

```text
IntelligenceConflict
```

Identity:

```text
conflict_id = UUIDv7
```

This avoids hiding conflict as a boolean flag scattered across records.

## Conceptual Fields

An IntelligenceConflict should preserve:

```text
conflict_id

subject_id

conflict_type

scope

status

detected_at

detection_method

detection_version

participating_assertion_ids

participating_assessment_ids

applicable_time_range

resolution_state

resolved_at / not_applicable

resolved_by / not_applicable

resolution_basis / not_applicable
```

The exact persistence structure is deferred until schema design.

## Initial Conflict Types

Pathfinder initially recognizes broad conflict classes such as:

```text
CLASSIFICATION
ATTRIBUTION
RELATIONSHIP
TEMPORAL
IDENTITY
LIFECYCLE
CONFIDENCE
SOURCE_PROVENANCE
OTHER
```

These describe what is in disagreement.

They do not determine how the conflict is resolved.

### CLASSIFICATION

Example:

```text
malicious
vs
benign
```

### ATTRIBUTION

Example:

```text
Campaign X attributed to Actor A
vs
Campaign X attributed to Actor B
```

### RELATIONSHIP

Example:

```text
Actor A uses Malware X
vs
Source explicitly denies Actor A uses Malware X
```

### TEMPORAL

Example:

```text
Source A:
    infrastructure active through September 5

Source B:
    infrastructure active through September 9
```

### IDENTITY

Example:

```text
Source A:
    Group X and Group Y are the same actor

Source B:
    Group X and Group Y are distinct
```

Identity conflict does not authorize Pathfinder to merge or separate objects automatically.

### LIFECYCLE

Example:

```text
Source A:
    indicator revoked

Source B:
    indicator still active
```

### CONFIDENCE

Different confidence alone is not necessarily substantive conflict.

For example:

```text
Source A:
    malicious, HIGH confidence

Source B:
    malicious, LOW confidence
```

Both agree on the substantive classification.

Pathfinder may record confidence disagreement without marking the underlying classification conflicted.

## Conflict Detection

Conflict may be identified by:

```text
PATHFINDER_RULE
ANALYST
SOURCE_EXPLICIT
```

### PATHFINDER_RULE

A versioned Pathfinder rule detects incompatible structured claims.

The rule must preserve:

```text
rule identity
version
input records
detection time
```

### ANALYST

An authorized analyst identifies a conflict not automatically detected.

The analyst's identity and basis remain recorded.

### SOURCE_EXPLICIT

A source explicitly disputes or contradicts another source or previously published claim.

Pathfinder preserves that as an Assertion and may create an IntelligenceConflict around the incompatible claims.

## No Automatic Majority Vote

Pathfinder must not implement:

```text
3 malicious
2 benign

therefore:
    malicious
```

as a universal rule.

Three feeds may all redistribute one upstream report.

The two opposing sources may be independent and technically stronger.

Therefore:

> **Record count is not truth weight.**

## No Automatic Reliability Winner

Likewise:

```text
Source A reliability = HIGH
Source B reliability = MODERATE
```

does not automatically mean every Source A claim defeats every Source B claim.

Source reliability is one input into an Assessment.

It is not an override mechanism.

## No Automatic Confidence Winner

Similarly:

```text
Source A:
    malicious HIGH

Source B:
    benign MODERATE
```

must not automatically become:

```text
malicious
```

Source confidence describes the source's own certainty.

It does not make the assertion objectively true.

## Corroboration and Conflict

Pathfinder may expose independent support on each side.

Example:

```text
Claim:
    IP X malicious

Independent supporting origins:
    3

Conflicting claim:
    IP X benign

Independent supporting origins:
    1
```

This is useful context.

It still does not automatically resolve the conflict.

A defined Assessment method or analyst may conclude that one side is better supported.

That conclusion becomes an Assessment.

The conflicting source records remain intact.

## Conflict Status

Initial conflict status:

```text
OPEN
UNDER_REVIEW
RESOLVED
SUPERSEDED
```

### OPEN

Pathfinder has identified materially incompatible intelligence and no resolution has been established.

### UNDER_REVIEW

An authorized review process is actively evaluating the conflict.

This state does not imply that resolution will favor either side.

### RESOLVED

An authorized Assessment has established the currently accepted interpretation.

The original conflicting records remain available.

### SUPERSEDED

The conflict record itself has been replaced by a newer conflict representation because the scope or participating claims materially changed.

Supersession does not erase the old conflict.

## Resolved Does Not Mean Deleted

Consider:

```text
Source A:
    malicious

Source B:
    benign

Analyst Assessment:
    malicious
    confidence HIGH
```

The conflict may be marked:

```text
RESOLVED
```

for current Pathfinder interpretation.

But the original benign Assertion remains part of history.

```text
resolved conflict
    != opposing assertion deleted
```

## Resolution Requires Authority

Conflict resolution must identify:

```text
resolving authority
resolution Assessment
basis
time
confidence
```

A parser cannot resolve an intelligence conflict simply because it processed one source later than another.

```text
last record received
    != winner
```

Likewise:

```text
newest assertion
    != automatically correct assertion
```

## Source Correction

A source may correct its own prior claim.

Example:

```text
Assertion A
    Source X says malicious

later:

Assertion B
    Source X says prior classification was incorrect
```

This is different from disagreement between independent sources.

Pathfinder should preserve:

```text
Assertion A
Assertion B
source correction relationship
applicable lifecycle events
```

Depending on semantics, the conflict may be resolved through source correction.

But Assertion A remains historical.

## Withdrawal Versus Contradiction

These are separate.

```text
Source withdraws prior assertion
```

means the source no longer stands behind the claim.

```text
Source asserts the opposite
```

creates a new substantive claim.

Pathfinder must not treat them identically.

## Time Can Resolve Apparent Conflict

Before creating or escalating a conflict, Pathfinder should evaluate whether claims apply to different periods.

Example:

```text
March 1:
    Domain X resolves_to IP A

March 2:
    Domain X resolves_to IP B
```

This is normal historical change.

It is not a conflict.

But:

```text
Same observation context and time:

Source A:
    Domain X resolves_to IP A

Source B:
    Domain X resolves_to IP B
```

may represent either real multi-answer DNS behavior or conflicting observation.

Pathfinder should preserve both and avoid premature classification.

## Scope Can Resolve Apparent Conflict

Example:

```text
Source A:
    Malware X targets Windows

Source B:
    Malware X targets Linux
```

These statements can coexist.

Different scope does not equal disagreement.

Likewise:

```text
Actor X uses Tool Y

Campaign Z does not use Tool Y
```

does not necessarily conflict.

The subjects are different.

## Unknown Is Not Conflict

These are not conflicting:

```text
Source A:
    IP X malicious

Source B:
    no classification supplied
```

Missing information is not opposition.

Likewise:

```text
NOT_KNOWN
```

does not contradict:

```text
HIGH
```

unless the applicable semantic contract explicitly says otherwise.

## Absence Is Not Contradiction

Pathfinder must never infer:

```text
Source A reports IP X malicious

Source B has no record for IP X

therefore:
    Source B disagrees
```

No record is not a benign assertion.

```text
absence
    != contradiction
```

## Explicit Negative Assertions

An explicit negative Assertion can create conflict.

Example:

```text
Source A:
    Domain X is malicious

Source B:
    Domain X is not malicious
```

That is materially different from Source B simply not mentioning Domain X.

## Partial Agreement

Sources may agree on part of an intelligence claim and disagree on another part.

Example:

```text
Source A:
    IP X is malicious
    operated by Actor Alpha

Source B:
    IP X is malicious
    attribution unknown
```

They agree on classification.

They do not necessarily conflict on attribution because Source B does not make an opposing attribution.

Another example:

```text
Source A:
    IP X malicious
    Actor Alpha

Source B:
    IP X malicious
    Actor Beta
```

Classification agrees.

Attribution conflicts.

Pathfinder should represent conflict at the narrowest meaningful semantic scope.

## Conflict Granularity

The governing rule is:

> **Conflict attaches to the smallest claim that actually disagrees.**

Do not mark an entire report, indicator, or actor record `CONFLICTED` merely because one relationship inside it is disputed.

Example:

```text
Malware:
    family = ExampleRAT

Sources agree:
    C2 protocol = HTTPS

Sources disagree:
    actor attribution
```

Only the attribution relationship needs conflict treatment.

## Object Lifecycle Interaction

The `CONFLICTED` lifecycle state from Phase 0.9 should be used carefully.

A record should become `CONFLICTED` only when the conflict materially affects the record's current interpretation.

The existence of any associated conflict does not automatically propagate `CONFLICTED` everywhere.

```text
related conflict
    != entire object conflicted
```

## Conflict and Indicators

An Indicator may remain usable while some associated intelligence is disputed.

Example:

```text
Sources agree:
    IP X is malicious

Sources disagree:
    Actor responsible
```

The Indicator's malicious classification may remain current.

The attribution Relationship may be conflicted.

Pathfinder should not disable useful intelligence simply because adjacent context is disputed.

## Conflict and Enforcement Candidates

If the precise intelligence basis required for a downstream recommendation is conflicted, Pathfinder must expose that conflict.

Example:

```text
Indicator classification:
    CONFLICTED

Candidate action:
    block IP
```

Pathfinder must not quietly present this as an ordinary high-confidence recommendation.

Downstream systems must receive enough lifecycle/conflict information to apply their own policy.

Pathfinder still does not authorize enforcement.

## Human Review

Conflicts may be routed for analyst review.

A review should expose:

```text
the exact competing claims

origin and delivery sources

source reliability

source-provided confidence

Pathfinder Assessments

independent corroboration

time applicability

local Sightings

supporting source records

processing history
```

The analyst should not have to reconstruct the conflict manually from unrelated screens.

## Analyst Resolution

An authorized analyst may create an Assessment resolving a conflict for Pathfinder's current interpretation.

Example:

```text
Conflict:
    OPEN

Analyst Assessment:
    malicious
    confidence HIGH

Basis:
    technical artifacts
    two independent sources
    conflicting benign source appears stale

Conflict:
    RESOLVED
```

The analyst Assessment does not erase the opposing source.

## Machine Resolution

A Pathfinder process may resolve narrowly defined conflict types only where a versioned rule explicitly permits it.

Example:

```text
same source record corrected by later signed version
```

may be mechanically resolvable under a source-specific contract.

General threat-attribution conflicts should not be silently machine-resolved merely because an algorithm produces a higher score.

## Reopening Conflict

A resolved conflict may be reopened when materially new intelligence arrives.

Example:

```text
Conflict resolved:
    September 5

New contradictory independent source:
    September 9
```

Pathfinder may create an explicit reopen event or a new superseding conflict according to the eventual schema.

It must not silently mutate the prior review history.

## Conflict Provenance

Conflict detection and resolution are derived intelligence operations.

They require provenance.

Pathfinder should be able to answer:

```text
Which records conflict?

Why are they considered incompatible?

Which rule or analyst identified the conflict?

What information was available at that time?

Was the conflict reviewed?

Who resolved it?

What Assessment resolved it?

What changed later?
```

## Search and Presentation

Search results should expose material conflict.

A query for an Observable should not show:

```text
Status: malicious
```

when Pathfinder actually has:

```text
2 independent malicious assertions
1 independent benign assertion
current analyst assessment: malicious / moderate confidence
conflict state: RESOLVED
```

The concise UI may summarize.

The underlying disagreement must remain accessible.

## No Synthetic Consensus Record

Pathfinder should not create a fake source-like assertion such as:

```text
"Pathfinder Sources say IP X is malicious"
```

unless that statement is represented as an explicit Pathfinder Assessment with attributable method and basis.

Pathfinder must never make several source Assertions appear to be one original Assertion.

## Conflict Is Useful Intelligence

Conflict itself may tell the analyst something important.

Examples include:

```text
rapidly changing infrastructure

source lag

provider false-positive behavior

shared hosting

actor-name disagreement

campaign overlap

intelligence laundering

redistributed stale intelligence

source compromise or poisoning

different collection visibility
```

Pathfinder should preserve the condition so analysts can investigate it.

## Poisoning and Manipulation

Conflicting intelligence may result from intentional manipulation.

Pathfinder should not assume disagreement is malicious manipulation.

However, the provenance model should allow analysts to examine:

```text
sudden source divergence

unusual upstream origin

mass redistribution

newly created source identity

unexpected changes in confidence

signed versus unsigned source material

historical source behavior
```

Any conclusion that a source is compromised or intentionally poisoning intelligence requires its own Assessment.

## Common Truth Separations

```text
conflict                         != ingestion failure

different values                 != necessarily conflict

absence                          != contradiction

unknown                          != disagreement

majority                         != truth

record count                     != independent support

higher source reliability        != automatic winner

higher confidence                != automatic winner

newest assertion                 != automatic winner

last received                    != automatic winner

source correction                != historical assertion erased

withdrawal                       != opposite assertion

different time                   != necessarily conflict

different scope                  != necessarily conflict

partial disagreement             != total disagreement

attribution conflict             != classification conflict

related conflict                 != whole object conflicted

resolved conflict                != conflicting source deleted

resolved                         != objectively proven

analyst resolution               != source history rewritten

machine resolution               != analyst review

conflict reopened                != prior resolution never occurred

conflicted intelligence          != enforcement authorization

no record from source            != benign assertion

synthetic consensus              != source assertion
```

## Phase 0.10 Exit Decision

Phase 0.10 is satisfied when Pathfinder accepts that:

1. Conflicting intelligence is a normal first-class condition.
2. `IntelligenceConflict` is a first-class Pathfinder object with UUIDv7 identity.
3. Conflict is evaluated against subject, semantics, scope, and time rather than merely differing values.
4. Conflict attaches to the narrowest meaningful incompatible claim.
5. Missing or unknown information does not count as disagreement.
6. Absence of a source record does not count as a negative assertion.
7. Explicit negative assertions may conflict with positive assertions.
8. Pathfinder does not resolve conflicts through majority vote.
9. Pathfinder does not automatically choose the source with the highest reliability.
10. Pathfinder does not automatically choose the assertion with the highest source-reported confidence.
11. Independent corroboration may inform resolution but does not mechanically determine it.
12. Conflict detection preserves method, version, basis, and time.
13. Conflict may be identified by Pathfinder rules, analysts, or explicit source disagreement.
14. Initial conflict states are `OPEN`, `UNDER_REVIEW`, `RESOLVED`, and `SUPERSEDED`.
15. Resolution requires attributable authority and basis.
16. Resolved conflicts retain every original Assertion and Assessment.
17. Source corrections and withdrawals remain distinct from independent-source contradiction.
18. Time and scope differences are evaluated before declaring conflict.
19. `CONFLICTED` lifecycle state does not automatically propagate to every related object.
20. Material conflict affecting a downstream recommendation must remain visible.
21. Resolved conflicts may be reopened or superseded when materially new intelligence arrives.
22. Conflict itself may be analytically useful and is therefore preserved.
23. Pathfinder never manufactures a synthetic source consensus from multiple underlying Assertions.
