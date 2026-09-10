# Pathfinder Phase 0.17 — Analyst Override, Review, and Change History

## Core Invariant

> **The original record is maintained no matter what.**

Pathfinder is append-only with respect to historical intelligence meaning.

An original record is never rewritten merely because later information changes Pathfinder's understanding of it.

This applies to historical SourceRecords, Assertions, Sightings, Assessments, Relationships, LifecycleEvents, IntelligenceConflicts, ChangeRecords, ChangeSets, ProcessingRecords, integration history, and applicable AuditEvents.

Later records may supersede, revoke, dispute, qualify, correct, reinterpret, resolve, or invalidate earlier records. They never cause the earlier record to cease having existed.

## Purpose

Human analysts must be able to interpret, challenge, correct, and refine Pathfinder intelligence without rewriting source or observation history.

Every material analyst change creates reconstructable, Git-style history showing:

```text
who changed the interpretation
what changed
when
why
what prior state existed
what records formed the basis
what resulting current view was produced
```

The governing principles are:

> **An analyst may change Pathfinder's interpretation. An analyst may not rewrite what a source said or what a system observed.**

> **Every material intelligence change should be reconstructable like a commit history.**

> **Corrections move history forward. They never rewrite history backward.**

## Git-Like, Not Git

Pathfinder's database remains authoritative. Git is not the intelligence datastore.

The intended semantics are Git-like:

```text
append-only history
stable record identity
attributable changes
parent/base lineage where applicable
human-readable summaries
structured semantic diffs
reconstructable prior state
forward-only correction
stale-base detection
```

## ChangeRecord

A first-class `ChangeRecord` records one material intentional change to governed interpretation, review/workflow state, or governed configuration.

Conceptual fields include:

```text
change_id
changeset_id / NOT_APPLICABLE
base_change_id / NOT_APPLICABLE
principal_id
principal_type
action_type
subject_type
subject_id
created_at
summary
reason / NOT_RECORDED
basis_ids
created_object_ids
superseded_object_ids
previous_view_digest / NOT_APPLICABLE
resulting_view_digest / NOT_APPLICABLE
change_status
```

Identity:

```text
change_id = UUIDv7
```

The ChangeRecord describes the change. It does not replace the intelligence records involved.

## ChangeSet

A `ChangeSet` groups one logical set of ChangeRecords.

Conceptual fields include:

```text
changeset_id
principal_id
created_at
summary
reason / NOT_RECORDED
base_revision / NOT_APPLICABLE
change_ids
commit_result
```

Identity:

```text
changeset_id = UUIDv7
```

### Atomic Commit Rule

> **A committed ChangeSet is atomic.**

Either all semantic changes in a ChangeSet commit durably, or none of that ChangeSet commits.

A committed ChangeSet may not mean:

```text
8 changes requested
5 committed
3 failed
```

A large bulk job may preflight 10,000 candidate mutations, reject ineligible members, then intentionally create one or more independent atomic ChangeSets for the approved scope. The overall bulk operation may therefore be `PARTIAL` while every committed ChangeSet remains individually atomic.

```text
bulk operation PARTIAL
    != partially committed ChangeSet
```

The earlier phrase `atomic where practical` is superseded by this rule.

## Original Record Preservation

If Pathfinder originally contains:

```text
Assessment A
    classification = malicious
    confidence = HIGH
```

and later analysis determines that conclusion was wrong, Pathfinder does not modify Assessment A.

Instead:

```text
Assessment A
    retained historical record

Assessment B
    classification = benign
    confidence = HIGH
    supersedes Assessment A
```

The current view may change to benign, while the historical record still shows what Pathfinder believed before the correction and why the correction happened.

## Wrong Records Are Still Records

A later determination that an original record was incorrect, misinterpreted, based on faulty telemetry, produced by parser defect, created by analyst error, created by a compromised identity, or based on poisoned intelligence does not authorize historical deletion.

The correction is another record.

This allows an investigator to reconstruct both what happened and why Pathfinder believed what it believed at the time.

## Analyst Assessment

The primary mechanism for analyst judgment is `Assessment`.

Analyst authority is `HUMAN_ANALYST`.

An analyst may create an Assessment, supersede a prior Assessment, dispute a Relationship, resolve an IntelligenceConflict, record permitted analyst-origin Assertions/Sightings, apply optional ATT&CK mappings, or make other explicitly authorized changes.

An analyst may not silently mutate SourceArtifact bytes, SourceRecords, source Assertions, or historical Sightings.

## Analyst Override

An analyst override means:

```text
new analyst Assessment
    +
explicit supersession/dispute/resolution where applicable
    +
ChangeRecord
    +
atomic ChangeSet where the logical operation spans multiple records
```

It never means overwriting the old authoritative row.

## Source Assertions

Analysts cannot rewrite what an external source originally asserted merely because they disagree with it.

Example:

```text
Assertion A:
    Vendor A says IP X malicious

Assessment B:
    Analyst assesses Vendor A claim incorrect
```

Both remain historical.

If Vendor A later issues a correction, that correction becomes new source-derived history; the analyst does not manufacture it by editing Assertion A.

## Sightings

Sightings remain historical observations.

If the underlying activity occurred but later proves benign, the Sighting remains and a later Assessment records the benign interpretation.

If a sensor defect produced an invalid observation, the Sighting still remains and may receive an Assessment such as:

```text
assessment_type = OBSERVATION_VALIDITY
assessment_value = INVALID
```

The observation record is valuable precisely because it documents what influenced Pathfinder.

## False Positive Semantics

Pathfinder distinguishes:

```text
observation invalid
interpretation incorrect
```

These are different conditions and neither is corrected by deleting history.

A valid observation that led to a bad conclusion requires a new interpretation. A defective sensor event requires a separate validity Assessment.

## Analyst Assertions and Sightings

Where permitted, analysts may record Assertions or Sightings under explicit analyst authority.

Such records preserve analyst principal, basis, observation time where known, recorded time, and applicable source/investigation context.

An analyst must not impersonate Stronghold, FI, Atlas, or an external source as the origin.

## Analyst Relationships

Analyst-created Relationships use `ANALYST_RECORDED` relationship origin and remain subject to:

```text
relationship registry
endpoint validation
time semantics
identity safeguards
attribution safeguards
```

Human authority does not bypass semantic validation.

## ATT&CK

Analysts may add ATT&CK mappings, but ATT&CK remains optional classification/reference metadata.

A legitimate Pathfinder state may be:

```text
Threat significance = HIGH
Operational relevance = HIGH
ATT&CK = NOT_MAPPED
```

with no downgrade.

Inability to map activity to ATT&CK must never prevent escalation, investigation, compromise Assessment, or candidate action recommendation.

## Conflict Review

Analysts may review `IntelligenceConflict` records.

Review should expose the exact competing Assertions/Assessments, source/delivery provenance, source reliability, source confidence, independent corroboration, Sightings, time applicability, prior review, and ProcessingRecords.

A conflict is resolved through an attributable resolution Assessment or other explicitly defined resolution record.

Opposing source/Assessment records remain intact.

## Unresolved Is Valid

An analyst may conclude:

```text
INSUFFICIENT_INFORMATION
```

and leave a conflict unresolved.

> **Uncertainty is an acceptable analytical result.**

Pathfinder does not force a winner merely for workflow convenience.

## Review Workflow

Review workflow state remains separate from intelligence lifecycle state.

Initial review states may include:

```text
NOT_REQUIRED
PENDING
IN_REVIEW
COMPLETED
DEFERRED
```

Assignment, priority, review reason, and reviewer identity are workflow metadata, not threat-intelligence meaning.

## Machine Versus Analyst

Machine Assessments remain `PATHFINDER_PROCESS` authored.

Human Assessments remain `HUMAN_ANALYST` authored.

Accepting a machine suggestion does not mutate the machine Assessment into a human Assessment.

Pathfinder may instead record:

```text
analyst workflow approval
```

or:

```text
new HUMAN_ANALYST Assessment based on machine Assessment
```

according to the operation's semantic contract.

Rejecting a machine suggestion does not delete it.

## Current-View Resolution

For an Assessment domain identified by at least:

```text
subject
assessment_type
applicable scope
applicable time
```

Pathfinder derives the applicable, non-invalidated, non-superseded Assessment candidates.

No Assessment automatically wins because it is:

```text
newest
highest confidence
human-authored
machine-authored
supported by the most raw records
```

A singular current interpretation exists only when an explicit mechanism establishes it, such as:

```text
valid supersession lineage
resolved IntelligenceConflict with resolution Assessment
versioned approved current-view selection policy
```

If materially incompatible applicable Assessments remain without an authorized resolution:

```text
current_interpretation_state = CONFLICTED
```

Pathfinder exposes the competing Assessments subject to authorization.

Database last-write-wins is not an intelligence resolution policy.

## No Permanent Human Lock

A prior analyst decision does not suppress materially new intelligence forever.

If new Assertions, Sightings, or Assessments materially conflict with the existing human conclusion, Pathfinder preserves the new information and may reopen review/conflict.

Earlier analyst history remains intact.

## Override Scope

An override applies only to the explicit subject and Assessment domain.

Changing operational relevance must not silently change source reliability, threat classification, attribution, or historical Sightings.

Pathfinder has no generic `analyst_override = true` field that suppresses unrelated intelligence.

## Basis and Rationale

Material analyst decisions preserve structured basis where available.

Possible basis objects include:

```text
Assertions
Sightings
Relationships
SourceRecords
SourceArtifacts where authorized
other Assessments
IntelligenceConflicts
external references
investigation records
```

Narrative rationale may supplement structured basis but does not replace provenance when structured references exist.

## Four-Eyes / Approval Workflow

The model may support single-review, dual-review, or required approval for high-impact operations without changing the underlying intelligence records.

Workflow approval and analytical Assessment are different records.

```text
second reviewer approved export != second identical threat Assessment
```

## High-Impact and Bulk Changes

High-impact or bulk changes require explicit scope, authorization, reason, and audit. Preview/preflight should be supported where practical.

A typo or overly broad query must not silently alter an entire corpus.

Preflight is not commit.

## Concurrency and Stale Base

Change lineage provides a clean way to detect stale analyst state.

Example:

```text
Analyst A begins from revision 100
Analyst B commits revision 101
Analyst A attempts overlapping change based on 100
```

Pathfinder may return:

```text
STALE_BASE
```

and require the analyst to review the new state.

Semantic collisions are not resolved by automatic last-write-wins merge.

## Per-Subject and Global History

Pathfinder should support operator concepts equivalent to:

```text
log <subject>
show <change_id>
diff <change_a> <change_b>
```

These are projections over historical intelligence/change/audit data. They do not require duplicating every machine processing event into ChangeRecord.

## Structured Diff

Pathfinder may render a semantic diff such as:

```text
threat_classification:
    - suspicious
    + malicious

confidence:
    - MODERATE
    + HIGH
```

This represents a change in the derived current interpretation.

Internally, the actual historical records remain append-only.

## Revert

A conceptual `revert <change_id>` creates a new forward-moving ChangeSet that restores/supersedes current interpretation as permitted.

> **Revert creates history. Revert never removes history.**

## No Reset / No Force Push

Pathfinder has no ordinary semantic equivalent of:

```text
git reset --hard
git push --force
```

for authoritative intelligence history.

Corrections move forward.

History is not rewritten to make mistakes disappear.

## Changelog Integrity

Phase 0 requires append-only semantics, stable identities, explicit base/parent lineage where used, atomic ChangeSets, and audit linkage.

Future implementation may strengthen this with record hashes, hash chaining, signatures, or an external witness if warranted.

Phase 1 should prove the basic model before overbuilding integrity machinery.

## Administrative Changes

The same reconstructable change-history approach may cover material governed configuration such as source enable/disable state, lifecycle profiles, mapping profiles, integration trust, ATT&CK dataset version, export policy, and security-sensitive configuration.

These are configuration changes, not intelligence Assessments.

## Physical Destruction and Raw Artifact Exception

The historical record remains.

If law, contract, handling policy, or another authorized retention requirement mandates destruction of SourceArtifact bytes, Pathfinder retains immutable metadata describing the artifact and destruction action.

```text
raw bytes destroyed by policy != historical record removed
```

## Analyst Identity and Compromise

Historical actions remain attributable if the analyst identity is later renamed, disabled, removed from the identity provider, or determined to have been compromised.

Actions performed by a compromised identity are remediated through forward-moving corrective records, not erasure.

## Common Truth Separations

```text
record incorrect != record deleted
record revoked != record deleted
record superseded != record deleted
record disputed != record deleted
source withdrawal != original Assertion deleted
sensor malfunction != original Sighting deleted
analyst mistake != original Assessment deleted
new interpretation != previous interpretation rewritten
current view != historical record
ChangeRecord != intelligence object
ChangeSet != bulk job
bulk job PARTIAL != partial ChangeSet commit
revert != erase
correction != reset
machine Assessment != analyst Assessment
workflow approval != analytical conclusion
ATT&CK unmapped != threat unimportant
stale base != last-write-wins permission
administrator != intelligence author
raw-byte destruction != historical-record destruction
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

The original state must be reconstructed from original historical records, not inferred backward from the current view.

## Phase 0.17 Exit Decision

Phase 0.17 is satisfied when Pathfinder accepts that:

1. **The original record is maintained no matter what.**
2. Analyst correction creates new attributable records instead of destructive mutation.
3. Source Assertions and Sightings are not rewritten because interpretation changes.
4. Material analyst changes create ChangeRecords.
5. Multi-record logical changes use ChangeSets where appropriate.
6. **A committed ChangeSet is atomic.**
7. A bulk operation may be PARTIAL without any committed ChangeSet being partial.
8. Machine and human Assessment authority remain distinct.
9. Conflict resolution preserves opposing records and uncertainty may remain unresolved.
10. Current-view resolution never silently uses newest, highest-confidence, human-wins, machine-wins, majority, or last-write-wins.
11. Unresolved materially incompatible Assessments produce an explicit conflicted current interpretation.
12. High-impact operations support scope validation, audit, and preflight/preview where practical.
13. Stale overlapping analyst changes may be rejected with explicit stale-base semantics.
14. Revert creates new history; reset-hard/force-push semantics do not exist for authoritative history.
15. ATT&CK remains optional classification/reference metadata.
16. Mandatory raw-byte destruction does not destroy the historical metadata proving what occurred.
