# Pathfinder Phase 0.9 — Intelligence Aging, Expiration, Revocation, and Supersession

## Purpose

Threat intelligence changes over time.

Pathfinder must represent those changes without rewriting history or confusing:

```text
old
invalid
withdrawn
replaced
disputed
conflicting
deleted
```

The governing principle is:

> **Lifecycle changes what Pathfinder should currently do with intelligence. It does not change what historically existed.**

## Lifecycle Is Not Truth

Lifecycle state describes the current operational status of a Pathfinder record.

It does not establish whether the underlying intelligence was objectively true or false.

```text
expired     != false
active      != true
revoked     != never existed
superseded  != deleted
disputed    != disproven
conflicted  != invalid
```

## Historical Preservation

Lifecycle transitions must remain historical events.

Example:

```text
September 1
Indicator: ACTIVE

September 7
Indicator: EXPIRED
```

Historical reconstruction must still establish that the Indicator was considered active between those times.

Pathfinder must not rewrite the September 1 record so that it appears always to have been expired.

## Initial Lifecycle States

Pathfinder v1 uses:

```text
ACTIVE
AGING
EXPIRED
REVOKED
SUPERSEDED
DISPUTED
CONFLICTED
```

These values describe materially different conditions.

### ACTIVE

`ACTIVE` means the record remains currently usable under its applicable intelligence contract.

It does not mean confirmed true, currently malicious, currently observed, locally relevant, or authorized for enforcement.

### AGING

`AGING` means the intelligence remains active but has crossed a defined age or recency threshold indicating that its operational value may be declining.

Aging provides warning before expiration.

Not all object types require aging. The applicable lifecycle profile determines whether `AGING` is used.

### EXPIRED

`EXPIRED` means the intelligence has passed the operational validity window established by its lifecycle contract.

Expiration means Pathfinder should no longer present the record as currently valid without exposing that it is expired.

Expiration does not mean benign, false, safe, deleted, historically irrelevant, or never malicious.

### REVOKED

`REVOKED` means an authority with applicable standing has explicitly withdrawn the record or its prior assertion.

Revocation requires, where applicable:

```text
revoking authority
revocation time
reason / basis
target record
```

Revocation does not erase the original record.

### SUPERSEDED

`SUPERSEDED` means a newer record has explicitly replaced an older record for current interpretation.

The superseding record must be explicitly linked.

```text
newer record exists != older record automatically superseded
```

### DISPUTED

`DISPUTED` means an authorized source, analyst, or Pathfinder process has challenged the validity or interpretation of a record without establishing a replacement conclusion.

A dispute does not mean the original assessment was disproven.

### CONFLICTED

`CONFLICTED` means Pathfinder has materially incompatible intelligence that cannot currently be reconciled under the applicable rules.

Pathfinder preserves the conflict without inventing a synthetic answer.

## Lifecycle Belongs to the Record

Lifecycle state applies to the applicable Pathfinder record.

It must not automatically propagate to everything related to it.

```text
Indicator expires
    != Observable expires
    != Sightings expire
    != SourceRecords expire
    != Assertions disappear
    != Relationships become false
```

## Object-Specific Behavior

Observables themselves should normally not expire merely because threat intelligence associated with them expires.

SourceRecords are historical received material and are governed by retention, not indicator-style expiration.

Assertions may become revoked, superseded, disputed, or conflicted where applicable.

Indicators are the primary lifecycle-sensitive objects and may use `valid_from`, `valid_until`, aging thresholds, and expiration behavior.

Assessments may be superseded, disputed, qualified, or confirmed without rewriting the original assessment.

Relationships may cease to be current while remaining historically true for a prior time period.

Sightings are historical observations and do not normally become expired merely because time passes.

## Lifecycle Profiles

Pathfinder must not define one global aging policy.

Lifecycle behavior is controlled by a versioned profile appropriate to the intelligence type.

Example:

```text
lifecycle_profile: ipv4-c2-v1
aging_after: 48h
expire_after: 7d
```

Different intelligence classes may have substantially different useful lifetimes.

## Expiration Origins

Expiration can originate from:

```text
SOURCE_DEFINED
PATHFINDER_POLICY
ANALYST_DEFINED
```

These remain distinguishable.

Pathfinder must not rewrite source-provided validity periods when applying local policy.

## No Silent Extension or Reactivation

Pathfinder must not automatically extend validity merely because an indicator was recently queried, remains popular, the same Observable was sighted, or related threat activity remains active.

A new Sighting may justify a new Assessment or lifecycle decision. It does not silently change the original Indicator's expiration.

Likewise, an expired Indicator must not silently become `ACTIVE` merely because the same Observable is later observed.

Reactivation, where allowed, requires an explicit lifecycle event and attributable authority.

## Reissue

A source may reissue materially equivalent intelligence.

Pathfinder must determine whether that is a new Assertion, new Indicator, extension, replacement, or duplicate delivery according to source-specific semantics.

It must not assume those are equivalent.

## Supersession Graph

Supersession is explicit and directional.

```text
NEW
 |
 | supersedes
 v
OLD
```

Supersession chains must not contain cycles.

## Revocation Authority

Not every actor may revoke every record.

External sources may revoke their own source assertions. Analysts may revoke records within their authority. Pathfinder processes may revoke defective derived results they own. Ordinary readers may not revoke intelligence.

Exact authorization is finalized in Phase 0.15 and Phase 0.17.

## Lifecycle Event History

Pathfinder should preserve lifecycle transitions as explicit history.

Conceptually:

```text
LifecycleEvent

event_id
subject_id
previous_state
new_state
authority
reason
effective_at
recorded_at
basis
```

This allows reconstruction of what changed, when, by whom, and why.

## Effective Time Versus Recorded Time

Lifecycle events must distinguish `effective_at` from `recorded_at`.

A source may state that an indicator expired or was revoked earlier than the time Pathfinder learned about it.

Pathfinder records both facts.

## Current State Versus Knowledge-at-Time State

Pathfinder must be able to answer two different questions:

```text
What do we now know the lifecycle state was at time T?

What lifecycle state did Pathfinder actually know at time T?
```

These are not always the same.

For example:

```text
retrospective truth:
    revoked September 5

Pathfinder knowledge on September 6:
    still ACTIVE

Pathfinder learned revocation:
    September 9
```

Both views matter during forensic reconstruction.

## Expiration and Search

Expired intelligence remains searchable according to authorization and retention policy.

Current-intelligence queries may default to `ACTIVE` and `AGING`, but authorized historical queries must be able to include expired intelligence.

A match against expired intelligence must remain distinguishable from a match against active intelligence.

## Expiration and Enforcement

Expired, revoked, or superseded intelligence must not silently remain active for future candidate enforcement output.

A downstream policy may explicitly permit historical matching, but that is separate authority.

## Retention Is Separate

Lifecycle status must not control physical destruction directly.

```text
EXPIRED    != delete
REVOKED    != delete
SUPERSEDED != delete
```

Retention and destruction authority are separate policy domains.

## Current-State Derivation

Pathfinder may derive the current lifecycle state from lifecycle events.

The current state is a convenience/current-view representation. The authoritative historical record remains the event sequence and underlying intelligence records.

## Invalid Transitions

Lifecycle transitions must be validated.

Transitions such as:

```text
EXPIRED -> ACTIVE
REVOKED -> ACTIVE
SUPERSEDED -> ACTIVE
```

must never occur silently.

Where reactivation is allowed, it must create an explicit lifecycle event with attributable authority and reason.

## Failure Behavior

If lifecycle processing fails, Pathfinder must not silently present stale records as confidently current.

Lifecycle-processing failure must be operationally visible under the later Phase 0.18 completeness model.

## Clock Dependence

Aging and expiration depend on time.

Pathfinder must use an explicitly defined trusted system-time source for lifecycle evaluation.

Clock disagreement or invalid system time must not silently drive trust-sensitive or destructive transitions.

## Common Truth Separations

```text
ACTIVE                         != true
AGING                          != false
EXPIRED                        != false
EXPIRED                        != benign
EXPIRED                        != delete
REVOKED                        != never existed
REVOKED                        != automatically false historically
SUPERSEDED                     != deleted
SUPERSEDED                     != never valid
DISPUTED                       != disproven
CONFLICTED                     != invalid
current state                  != historical state
current knowledge              != knowledge at the time
effective_at                   != recorded_at
source expiration              != Pathfinder expiration
new Sighting                   != automatic reactivation
reissued intelligence          != duplicate delivery
Observable                     != expiring Indicator
old Sighting                   != expired observation
relationship no longer current != relationship never existed
expiration eligibility         != destruction authority
lifecycle transition           != enforcement authorization
lifecycle processing failure   != successful expiration
```

## Phase 0.9 Exit Decision

Phase 0.9 is satisfied when Pathfinder accepts:

1. `ACTIVE`, `AGING`, `EXPIRED`, `REVOKED`, `SUPERSEDED`, `DISPUTED`, and `CONFLICTED` as the initial lifecycle vocabulary.
2. Lifecycle state describes operational status, not objective truth.
3. Historical records survive every lifecycle transition.
4. Observables and Sightings do not automatically expire when associated Indicators expire.
5. Lifecycle behavior is object-appropriate rather than governed by one universal age.
6. Versioned lifecycle profiles control policy-driven aging and expiration.
7. Source-defined, Pathfinder-policy, and analyst-defined lifecycle decisions remain distinguishable.
8. Source validity periods are never rewritten by Pathfinder policy.
9. New sightings do not silently extend or reactivate expired intelligence.
10. Supersession is explicit, directional, and cycle-free.
11. Revocation requires attributable authority.
12. Lifecycle transitions preserve event history.
13. Effective time and Pathfinder-recorded time remain separate.
14. Late-arriving revocation or expiration information does not rewrite what Pathfinder actually knew at the earlier time.
15. Pathfinder can distinguish retrospective lifecycle truth from knowledge-at-time state.
16. Expired intelligence remains historically searchable according to retention and authorization.
17. Historical matches remain distinguishable from active intelligence matches.
18. Expired, revoked, or superseded intelligence does not silently remain active for candidate enforcement.
19. Lifecycle status never directly authorizes deletion.
20. Lifecycle processing failures remain visible.
