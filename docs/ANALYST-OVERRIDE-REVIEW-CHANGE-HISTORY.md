# Pathfinder Phase 0.17 — Analyst Override, Review, and Change History

## Core Invariant

> **The original record is maintained no matter what.**

Pathfinder is append-only with respect to historical intelligence meaning.

An original record is never rewritten merely because later information changes Pathfinder's understanding of it.

This applies to:

```text
SourceRecords
Assertions
Sightings
Assessments
Relationships
LifecycleEvents
IntelligenceConflicts
ChangeRecords
ChangeSets
processing history
integration history
```

Later records may supersede, revoke, dispute, qualify, correct, reinterpret, resolve, or invalidate an earlier record. They never cause the earlier record to cease having existed.

## Purpose

Human analysts must be able to interpret, challenge, correct, and refine Pathfinder intelligence without rewriting historical records.

Every material analyst change creates append-only history showing who changed the interpretation, what changed, when, why, what prior state existed, what information formed the basis, and what resulting current view was created.

The governing principles are:

> **An analyst may change Pathfinder's interpretation. An analyst may not rewrite what a source said or what a system observed.**

> **Every material intelligence change should be reconstructable like a commit history.**

> **Corrections move history forward. They never rewrite history backward.**

## Git-Style Change Model

Pathfinder maintains a first-class `ChangeRecord`.

Conceptually:

```text
change_id
parent_change_id / not_applicable
principal_id
principal_type
action_type
subject_id
created_at
summary
reason
basis_ids
created_object_ids
superseded_object_ids
previous_view_digest / not_applicable
resulting_view_digest / not_applicable
change_status
```

`change_id` uses UUIDv7.

The ChangeRecord records what happened. It does not replace the intelligence records involved.

## Original Record Preservation

If Pathfinder originally contains an Assessment marking an Indicator malicious with HIGH confidence and later investigation determines that conclusion was wrong, Pathfinder does not modify the original Assessment.

Instead:

```text
Assessment A
    malicious
    HIGH
    retained permanently as historical record

Assessment B
    benign
    HIGH
    supersedes Assessment A
```

The current view may now show `benign`, while historical reconstruction still shows what Pathfinder believed at T1, what changed at T2, and which Assessment superseded the earlier one.

The incorrect record remains because the fact that Pathfinder once reached that conclusion is itself historically important.

## Wrong Records Are Still Records

A later determination that an original record was incorrect, misinterpreted, based on faulty telemetry, produced by a parser defect, created by analyst error, created by a compromised account, or based on poisoned intelligence does not authorize its removal.

Pathfinder attaches later state or interpretation instead.

This permits an investigator to understand both what happened and why Pathfinder believed what it believed at the time.

## Git-Like, Not Git

The database remains authoritative. Pathfinder does not literally use Git as its intelligence datastore.

The intended semantics are Git-like:

```text
append-only history
stable record identity
attributable changes
parent lineage
human-readable summaries
structured diffs
reconstructable prior state
forward-only correction
```

A material change should resemble:

```text
commit 0199...
Author: analyst:jwood
Date:   ...

    Reclassify IP indicator after incident review

Subject:
    indicator 0198...

Changes:
    operational_relevance:
        HIGH -> LOW

    threat_classification:
        malicious -> benign

Basis:
    assessment 0197...
    sighting 0196...
    source_record 0195...

Supersedes:
    assessment 0194...
```

This is more useful than a generic `indicator updated successfully` message.

## No In-Place Historical Mutation

Historical intelligence objects remain immutable where their semantics require immutability.

A correction creates a new Assessment, Relationship, lifecycle event, workflow decision, or other proper object and a ChangeRecord links the operation together.

Pathfinder should not implement historical correction as a direct field overwrite of an existing authoritative record.

## Parent Change

A material change may reference the immediately preceding applicable change to form an ordered history.

Parent references may help detect concurrent changes, stale analyst views, unexpected history gaps, and attempted rewrites.

Different subjects can have independent change streams; a parent reference does not imply one universal linear intelligence history.

## Per-Subject and Global History

Pathfinder should support conceptual operations equivalent to:

```text
log <subject>
show <change_id>
diff <change_a> <change_b>
```

A per-subject log answers how the current interpretation developed. A global log may show material Assessments, Relationships, conflict resolutions, lifecycle changes, source reliability changes, integration configuration changes, and export approvals subject to authorization.

## Change Summary and Rationale

Every material analyst change should have a concise human-readable summary analogous to a Git commit message. Significant changes should also preserve a longer rationale.

A commit message never substitutes for analytical basis.

## Structured Diff

Pathfinder may generate structured semantic diffs for operator convenience:

```text
threat_classification:
    - suspicious
    + malicious

confidence:
    - MODERATE
    + HIGH
```

The diff represents change in Pathfinder's current interpretation. Internally, the system preserves the actual records created, superseded, disputed, revoked, or otherwise affected.

## Current View

The current state of intelligence is derived from historical records and their lifecycle, supersession, conflict, and review relationships.

```text
history
   ↓
resolution rules
   ↓
current view
```

Current-state tables, indexes, caches, or materialized views may be rebuilt.

> **Current-state views can be rebuilt. Original historical records cannot.**

## Change History Is Append-Only

Material ChangeRecords are append-only. They cannot normally be edited, reordered, or silently deleted.

If a ChangeRecord itself contains an error, Pathfinder records a corrective ChangeRecord.

## Revert

Pathfinder may support a conceptual `revert <change_id>` operation.

A revert creates another forward-moving change. It does not erase the original change.

> **Revert creates history. Revert never removes history.**

## No Reset and No Force Push

Pathfinder has no normal equivalent of `git reset --hard` against authoritative intelligence history.

There is no ordinary analyst operation that rewinds the database and makes intervening records disappear.

Pathfinder also has no semantic equivalent of `git push --force` for intelligence history.

> **Corrections move forward. History is not force-pushed.**

## Analyst Assessment

The primary mechanism for human judgment remains the Pathfinder `Assessment`.

Creating or superseding a material Assessment also creates a ChangeRecord describing the operation.

Analyst confidence belongs to the analyst Assessment and remains separate from source-reported confidence.

## Analyst Override

An analyst override means:

```text
new analyst interpretation
    +
explicit supersession where applicable
    +
ChangeRecord
```

It does not mean overwriting the old row.

## Source Assertions

Analysts cannot change what a source originally asserted because they disagree with it or later evidence proves it wrong.

Instead, Pathfinder records a new analyst Assessment, source correction, withdrawal, dispute, or other appropriate forward-moving record.

## Source Withdrawal

A source withdrawal becomes new source-derived history. The original Assertion remains.

```text
Assertion A:
    Source says IP X malicious

Assertion B:
    Source withdraws Assertion A
```

Both remain.

## Revocation and Supersession

Revocation and supersession change current applicability or interpretation. They do not delete historical records.

```text
REVOKED != deleted
SUPERSEDED != deleted
```

## Sightings

Sightings remain historical observations even when later interpretation changes.

If the underlying activity occurred but was benign, the Sighting remains and a later Assessment records the benign interpretation.

If a sensor defect generated an invalid observation, the original Sighting remains and a later Assessment may establish that the observation was invalid.

The record is valuable precisely because it documents what influenced the system.

## False Positive

Pathfinder distinguishes:

```text
observation invalid
interpretation incorrect
```

These are not the same condition and neither is corrected by deleting history.

## Analyst Assertions and Sightings

Where permitted, analysts may record Assertions or Sightings under explicit analyst authority. Such records preserve analyst principal, basis, observation time where known, and recorded time.

An analyst must not impersonate Stronghold, FI, Atlas, or an external source as the origin.

## Analyst Relationships

Analyst-created Relationships use `ANALYST_RECORDED` origin and remain subject to the Relationship registry, endpoint rules, time semantics, identity safeguards, and attribution safeguards.

Human authority does not bypass semantic validation.

## ATT&CK

Analysts may add ATT&CK mappings, but ATT&CK remains optional classification metadata.

A legitimate Pathfinder state may be:

```text
Threat:
    HIGH confidence

Operational relevance:
    HIGH

ATT&CK:
    NOT_MAPPED
```

with no downgrade.

An inability to map activity to ATT&CK must never prevent escalation, compromise assessment, investigation, or recommended action.

## Conflict Review

Analysts may review IntelligenceConflict objects. Review should expose competing Assertions and Assessments, source/delivery provenance, reliability, confidence, corroboration, Sightings, time applicability, prior reviews, and processing lineage.

A conflict is resolved by an attributable Assessment or other explicit resolution record. Opposing intelligence remains intact.

## Unresolved Is Valid

An analyst may legitimately conclude `insufficient information` and leave a conflict unresolved.

> **Uncertainty is an acceptable analytical result.**

Pathfinder must never force a winner merely for workflow convenience.

## Review Workflow

Review state remains separate from intelligence lifecycle state.

Initial review states may include:

```text
NOT_REQUIRED
PENDING
IN_REVIEW
COMPLETED
DEFERRED
```

Review assignment, priority, reason, and analyst identity are workflow metadata, not intelligence meaning.

## Machine Versus Analyst

Machine Assessments remain machine authored. Analyst Assessments remain human authored.

Accepting a machine suggestion must not mutate the machine record into a human-authored record. The system may record a separate analyst approval or a new analyst Assessment based on the machine result.

Rejecting a machine suggestion does not delete it.

A prior analyst decision also must not permanently suppress materially new machine or source intelligence. New contradictory information may reopen review or conflict.

## Override Scope

An override applies only to the explicit subject and assessment domain.

Changing operational relevance must not silently change source reliability, threat classification, attribution, or historical Sightings.

Pathfinder should have no generic `analyst_override = true` field that suppresses unrelated intelligence.

## Basis and Rationale

Material analyst decisions preserve analytical basis. Possible basis objects include Assertions, Sightings, Relationships, SourceRecords, SourceArtifacts where permitted, other Assessments, external references, and local investigation records.

Narrative reasoning may supplement structured basis but should not replace it when structured provenance exists.

## Four-Eyes Review

The model should support optional single-review, dual-review, or required approval workflows for high-impact actions without altering underlying intelligence history.

Workflow approval and analytical Assessment remain separate records.

## High-Impact and Bulk Changes

High-impact or bulk actions should require explicit scope, preview where practical, authorization, reason, and audit.

A typo should not silently alter the current interpretation of an entire corpus.

## ChangeSet

A single logical operation affecting multiple records may be grouped into a first-class `ChangeSet`.

Conceptually:

```text
changeset_id = UUIDv7
principal
created_at
summary
reason
individual_change_ids
```

A ChangeSet is analogous to one Git commit touching several files. It groups changes; it does not mutate original records.

Where practical, one logical ChangeSet should commit atomically or return an explicit failure/partial state.

## Concurrency and Stale Base

Parent/change lineage provides a clean way to detect stale writes.

If an analyst attempts to commit against an outdated parent state after another analyst has committed a conflicting change, Pathfinder may return:

```text
STALE_BASE
```

rather than silently applying database last-write-wins behavior.

Semantic collisions require explicit review; they are not automatically merged merely because both writers were authorized.

## Changelog Integrity

Phase 0 requires append-only semantics, stable identities, ordered parent relationships where used, and audit linkage.

Future implementation may strengthen this with record hashes, hash chaining, signatures, or an external journal/witness if warranted. Phase 1 should not overbuild those mechanisms before the basic model is proven.

## Administrative Changes

The same reconstructable change-history approach should eventually cover material configuration changes such as source reliability profiles, integration enable/disable state, lifecycle profiles, mapping profiles, ATT&CK dataset versions, export policy, and security-sensitive configuration.

These are not intelligence Assessments, but they can materially affect Pathfinder behavior.

## Physical Destruction and Raw Artifact Exception

The historical record must remain.

If an external legal, contractual, or handling requirement requires destruction of raw source bytes, Pathfinder still retains immutable historical metadata describing the artifact and its destruction, including identity, hash, original byte length, received time, source, destruction state, destruction time, authority, policy/legal basis, associated SourceRecords, and prior processing lineage.

```text
raw bytes destroyed by mandatory policy
    != historical record removed
```

The bytes may become unavailable. The history proving that they existed and explaining what happened to them remains.

## Analyst Identity and Compromise

Historical actions remain attributable even if the analyst account is later renamed, disabled, removed from the identity provider, or determined to have been compromised.

Actions from a compromised identity are corrected through forward-moving records, not historical erasure.

## Common Truth Separations

```text
record incorrect                  != record deleted
record revoked                    != record deleted
record superseded                 != record deleted
record disputed                   != record deleted
record conflicted                 != record deleted
source withdrew claim             != original Assertion deleted
sensor malfunction                != original Sighting deleted
analyst mistake                   != original Assessment deleted
parser defect                     != original processing deleted
new interpretation                != previous interpretation rewritten
current view                      != historical record
analyst change                    != historical rewrite
ChangeRecord                      != intelligence object
current-view diff                 != database row mutation
revert                            != erase
correction                        != reset
supersession                      != deletion
analyst override                  != magic truth flag
machine Assessment                != analyst Assessment
review approval                   != source Assertion
ATT&CK unmapped                   != threat unimportant
commit message                    != analytical basis
ChangeSet                         != arbitrary bulk mutation
stale base                        != permission for last-write-wins
administrator                     != intelligence author
legal raw-byte destruction        != historical record destruction
```

## Absolute Historical Rule

Pathfinder must always be able to answer:

```text
What was originally recorded?
What did Pathfinder originally understand?
What changed afterward?
Who changed it?
Why?
What is the current interpretation now?
```

The first answer must never be reconstructed from the last answer.

The original record itself remains the source for the original state.

## Phase 0.17 Exit Decision

Phase 0.17 is satisfied when Pathfinder accepts that:

1. **The original record is maintained no matter what.**
2. Historical intelligence records are never corrected through destructive mutation.
3. Incorrect records remain historical records and receive later corrective interpretation.
4. Analysts change Pathfinder interpretation through new attributable records.
5. Every material analyst change creates an append-only ChangeRecord.
6. Multi-record operations may be grouped into ChangeSets.
7. ChangeRecord and ChangeSet identities use UUIDv7.
8. Pathfinder supports Git-like per-subject historical reconstruction and conceptual log/show/diff views.
9. Structured diffs never replace immutable intelligence history.
10. Source withdrawals never erase original source Assertions.
11. Revocation never means historical deletion.
12. Supersession never means historical deletion.
13. Conflict resolution never removes opposing intelligence.
14. Parser and mapping reprocessing never overwrites earlier processing history.
15. Sightings remain even when later shown to have resulted from sensor error.
16. Analyst mistakes are corrected by new records.
17. Compromised-account actions remain identifiable historical actions.
18. Revert creates another forward-moving change.
19. Pathfinder provides no normal reset-hard equivalent for intelligence history.
20. Pathfinder provides no force-push equivalent for authoritative history.
21. Current views are derived from immutable history rather than serving as the historical source themselves.
22. Parent/change lineage may protect against stale concurrent modifications.
23. Pathfinder does not use last-write-wins for competing analytical judgments.
24. ATT&CK remains optional classification metadata and NOT_MAPPED causes no downgrade.
25. Analysts may legitimately leave uncertainty unresolved.
26. Machine and human authorship remain distinguishable.
27. High-impact operations support explicit scope, audit, and preview where practical.
28. Material administrative changes can use the same reconstructable change-history approach.
29. If mandatory policy requires destruction of underlying raw bytes, immutable historical metadata and destruction history remain.
30. Pathfinder can always reconstruct the original record independently of its current interpretation.
