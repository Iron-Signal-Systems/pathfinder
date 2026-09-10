# Pathfinder Agent and Contributor Rules

## Purpose

This file defines how contributors, coding agents, and automation work within the Pathfinder repository.

It is a behavioral contract and does not replace governing architecture, roadmap, schema, protocol, data-model, or implementation-contract documents.

Where a more specific committed Pathfinder contract exists, that contract governs its defined implementation boundary.

The Phase 0 reconciliation and exit document is the authority for conflicts between early Phase 0 wording and later refinements:

[`docs/PHASE-0-RECONCILIATION-EXIT.md`](docs/PHASE-0-RECONCILIATION-EXIT.md)

## Governing Principles

> **Preserve what was received before deciding what it means.**

> **Record the claim before judging the claim.**

> **An observable is not inherently malicious.**

> **A source assertion is not automatically a fact.**

> **A sighting is an observation, not a conclusion.**

> **Correlation is not identity.**

> **Association is not attribution.**

> **Confidence describes a judgment. Reliability describes a source. Corroboration describes support. They are not interchangeable.**

> **The original record is maintained no matter what.**

> **Current knowledge must not rewrite historical knowledge.**

> **Derived intelligence remains traceable to the source material and processing lineage from which it was produced.**

> **Conflicting intelligence remains visible rather than being silently averaged into false certainty.**

> **An incomplete index is not complete intelligence history.**

> **Expiration eligibility is not permission to destroy historical intelligence.**

> **ATT&CK is a classification aid, not a prerequisite for understanding or proving a compromise.**

> **High confidence is still not authorization.**

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

## Product Authority

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

Pathfinder is authoritative for its threat-intelligence records, interpretations, provenance, processing history, lifecycle, conflicts, and governed change history.

It is not automatically authoritative for the real-world truth of an external assertion.

ISS product authority remains separated:

```text
Atlas
    asset/environment context

FI
    file and file-system observations

Stronghold
    network observations and network enforcement decisions

Pathfinder
    organizational threat-intelligence record and interpretation

Guidon
    backup/recovery state
```

Integration shares information. It does not transfer domain authority.

## Product Boundary

Pathfinder may:

```text
receive
preserve
validate
parse
normalize
deduplicate
correlate
enrich
assess
relate
age
expire
supersede
revoke
dispute
query
report
export
publish
```

Pathfinder does not silently become:

```text
a firewall
an EDR
an endpoint enforcement system
an identity authority
an asset authority
a vulnerability scanner
a packet-capture authority
a backup/recovery authority
an autonomous remediation engine
```

## Three Categories of Truth

Preserve the distinction between:

```text
WHAT THE SOURCE SAID
    preserved source material / attributable Assertion

WHAT WAS OBSERVED
    Sightings and observations from an identified authority

WHAT PATHFINDER UNDERSTOOD
    normalized, correlated, enriched, or assessed intelligence
```

These categories must not be silently collapsed.

A source saying an IP is malicious is an Assertion.

Stronghold observing communication with that IP is a Sighting.

Pathfinder associating the Sighting with infrastructure, malware, a campaign, or a threat actor is interpretation.

Those are different records.

## Canonical Record Families

Use the reconciled object families:

```text
SOURCE / PROVENANCE
    Source
    SourceCollection
    RetrievalEvent
    SourceArtifact
    SourceRecord

INTELLIGENCE
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

INTELLIGENCE STATE
    LifecycleEvent
    IntelligenceConflict

SYSTEM HISTORY
    ChangeRecord
    ChangeSet
    AuditEvent
    ProcessingRecord
```

Do not introduce a generic object type that erases these distinctions merely because the database or API could represent them similarly.

## SourceArtifact and SourceRecord

The canonical source path is:

```text
Source
  ↓
SourceCollection
  ↓
RetrievalEvent
  ↓
SourceArtifact
  ↓
SourceRecord
  ↓
Assertion
  ↓
normalized intelligence
```

`SourceArtifact` preserves the exact immutable acquired payload at the preservation boundary.

`SourceRecord` is a logical item represented within that artifact.

Do not put artifact-owned exact-byte semantics back onto SourceRecord.

```text
SourceArtifact != SourceRecord
SourceRecord   != Assertion
```

A SourceRecord may reference its parent artifact and deterministic locator. Parser output or reserialized JSON is not the original source bytes.

## Original-History Rule

> **The original record is maintained no matter what.**

Do not destructively correct historical records.

If a record is later found to be wrong, stale, revoked, superseded, disputed, conflicted, sensor-generated in error, parser-generated in error, created by analyst mistake, or produced by a compromised account, preserve the original and create the proper forward-moving corrective record.

```text
record incorrect  != record deleted
revoked           != deleted
superseded        != deleted
disputed          != deleted
conflict resolved != opposing record deleted
revert            != erase
```

If mandatory policy requires destruction of SourceArtifact bytes, retain the immutable artifact metadata and destruction history required by the preservation contract.

## Source Intake Rules

External intelligence is untrusted input even when the source is reputable or authenticated.

Treat source material as potentially:

```text
malformed
incomplete
stale
duplicated
contradictory
oversized
unexpectedly encoded
schema-invalid
semantically invalid
hostile
```

Input validation is mandatory at applicable boundaries.

Parsing failure remains visible.

Unsupported input remains distinguishable from malformed input.

Normalization must not manufacture information absent from the source.

Preserve required source bytes before semantic parsing or destructive transformation.

Do not automatically execute, render, dereference, resolve, fetch, or otherwise activate source-supplied content merely because it was received.

## Observable Rules

Observables are neutral.

Canonicalization follows the applicable versioned Observable profile and must be deterministic and pure.

Do not silently turn:

```text
Observable -> Indicator
Observable -> malicious
URL host -> Domain Observable
email domain -> Domain Observable
certificate hash -> generic hash
```

without explicit versioned processing and provenance.

When safe canonicalization cannot be established, return the appropriate malformed/unsupported state rather than a best guess.

## Assertion and Assessment Rules

`Assertion` records an attributable claim.

`Assessment` records a Pathfinder judgment.

Assessment authorities are:

```text
HUMAN_ANALYST
PATHFINDER_PROCESS
```

External-source judgments remain Assertions.

Machine and human Assessments remain distinguishable.

Sighting may be an Assessment subject where the assessment type permits it, allowing Pathfinder to assess observation validity without rewriting the Sighting.

Do not implement judgment as a mutable truth field such as:

```text
indicator.malicious = true
```

when the value actually belongs to an Assessment.

## Current-View Rules

Current-state views are derived and rebuildable.

Original historical records are not.

Do not use implicit:

```text
newest wins
highest confidence wins
human always wins
machine always wins
largest record count wins
last database row wins
```

semantics.

For an applicable assessment domain, a singular current interpretation requires explicit supersession, conflict resolution, or another versioned approved current-view rule.

If materially incompatible applicable Assessments remain unresolved, expose the current interpretation as conflicted rather than choosing a hidden winner.

## Confidence, Reliability, and Corroboration

Never collapse:

```text
source reliability
source-reported confidence
Pathfinder Assessment confidence
analyst confidence
corroboration
age / recency
operational relevance
```

into one unexplained score.

Source reliability is a historical Assessment of Source or SourceCollection, not one mutable authoritative field on Source.

A reliable source can publish uncertain information.

A less reliable source can publish something correct.

Multiple deliveries of one upstream report are not independent corroboration.

`NOT_KNOWN` source independence never counts as `INDEPENDENT` by default.

Repeated Sightings are not repeated independent corroboration of maliciousness.

## Correlation and Relationship Rules

Relationships come from a versioned Pathfinder-controlled registry.

Do not allow arbitrary caller-supplied relationship strings to become authoritative semantics.

Do not silently convert:

```text
possibly_related
shares_infrastructure_with
resolves_to
communicates_with
associated_with
```

into:

```text
is
same_entity
owns
operates
controls
attributed_to
malicious
compromised
```

Relationship origin and ATT&CK mapping origin are separate dimensions.

Relationship origin:

```text
SOURCE_NORMALIZED
PATHFINDER_DERIVED
ANALYST_RECORDED
```

ATT&CK mapping origin:

```text
ATTACK_NATIVE
SOURCE_REPORTED
PATHFINDER_ASSOCIATED
LOCALLY_SUGGESTED
LOCALLY_CONFIRMED
ANALYST_RECORDED
```

Do not merge these enums.

No relationship is assumed symmetric or transitive unless its registry definition explicitly says so.

Do not infer relationships merely because objects co-occur in a report, bundle, graph neighborhood, or time window.

## ATT&CK Rules

ATT&CK is optional classification/reference metadata.

Pathfinder must never require an ATT&CK technique, tactic, mapping, or sequence before it can preserve, assess, prioritize, or escalate activity.

```text
ATT&CK NOT_MAPPED != low significance
Technique overlap != attribution
Technique matched != actor identity
Technique matched != malware identity
LOCALLY_CONFIRMED != compromise proven
```

Do not manufacture mappings to improve apparent coverage.

## Sighting Rules

A Sighting is an observation, not a conclusion.

```text
Sighting != maliciousness
Sighting != compromise
Sighting != attribution
Sighting != successful exploitation
```

Preserve observer authority, event time, receipt time, context, and provenance.

Duplicate delivery is not repeated observation.

Aggregate observation is not fabricated individual event history.

No-Sighting results require coverage context. Do not create ordinary negative Sightings without a contract capable of proving the required observation coverage.

## Conflict Rules

`IntelligenceConflict` is first-class.

Conflict is not ingestion failure.

Do not resolve conflict through universal rules such as:

```text
majority wins
highest confidence wins
highest reliability wins
newest record wins
last received wins
```

A conflict may remain unresolved.

> **Uncertainty is an acceptable analytical result.**

Conflict resolution never deletes the losing Assertion or Assessment.

## Lifecycle Rules

Initial lifecycle vocabulary:

```text
ACTIVE
AGING
EXPIRED
REVOKED
SUPERSEDED
DISPUTED
CONFLICTED
```

Lifecycle state describes current applicability, not objective truth.

```text
ACTIVE      != true
EXPIRED     != false
EXPIRED     != benign
REVOKED     != never existed
SUPERSEDED  != deleted
DISPUTED    != disproven
CONFLICTED  != invalid
```

Lifecycle does not directly authorize physical destruction.

## Typed State Rules

Do not create one generic `status` enum for unrelated domains.

Use explicit typed dimensions such as:

```text
OperationResult
PreservationState
IntegrityState
AvailabilityState
ProcessingState
HealthState
CoverageState
IndexState
LifecycleState
ReviewState
ConflictStatus
```

The same textual word may appear in more than one domain only when the typed field preserves which semantic dimension it belongs to.

For SourceArtifact specifically, keep preservation, integrity, and availability separate.

Example direction:

```text
PreservationState
    RECEIVING
    PRESERVED
    PARTIAL
    FAILED

IntegrityState
    NOT_VERIFIED
    VERIFIED
    MISMATCH

AvailabilityState
    AVAILABLE
    QUARANTINED
    UNAVAILABLE
    DESTROYED_BY_POLICY
```

## Raw and Derived Data

Original source information and derived Pathfinder information are separate data classes.

Derived processing MUST NOT modify preserved original records merely because later interpretation changes.

Parser or mapper upgrades create new ProcessingRecords and derived results.

They must not make later understanding appear to have existed during the original processing event.

## ChangeRecord and ChangeSet Rules

`ChangeRecord` records an intentional material change to governed current interpretation, workflow, or configuration.

`ChangeSet` groups one logical set of ChangeRecords.

A committed ChangeSet is atomic:

```text
all semantic changes commit
    or
none commit
```

A bulk operation may preflight many records and may intentionally produce several independent ChangeSets. The bulk job may be `PARTIAL`, but an individual committed ChangeSet is not partially committed.

Corrections move forward.

No normal `reset --hard` or force-push equivalent exists for authoritative Pathfinder history.

## AuditEvent and ProcessingRecord Rules

Do not use ChangeRecord as a generic event log.

Use:

```text
ChangeRecord / ChangeSet
    intentional governed change

AuditEvent
    security / authorization / access / administration / integrity operation

ProcessingRecord
    parser / normalizer / mapper / correlation / lifecycle / conflict / reprocessing lineage
```

These histories may reference one another but must not be collapsed into one ambiguous universal event table.

A future `log` or `show` view may project across them without duplicating every processing event into the changelog.

## Deduplication Rules

Deduplication must not destroy provenance.

Distinguish:

```text
same Observable
same SourceArtifact
same SourceRecord
duplicate delivery
redistributed source
independent corroboration
```

Identical canonical Observables may resolve to one Observable while every source record, assertion, and delivery path remains independently traceable.

## STIX / TAXII Rules

STIX and TAXII are interoperability boundaries, not Pathfinder's internal truth model.

```text
valid STIX != intelligence trusted
TAXII success != content accepted
same STIX object from many feeds != independent corroboration
```

STIX translation loss and unsupported semantics remain explicit.

TAXII transport, pagination, source preservation, STIX validation, native mapping, commit, retry, and checkpoint state remain separate.

A checkpoint advances only after the applicable safe preservation/commit boundary succeeds.

## Integration Rules

ISS integrations use explicitly versioned contracts and narrow authenticated identities.

No direct cross-product database writes.

```text
Pathfinder writes Atlas tables        -> forbidden
Stronghold edits Pathfinder tables    -> forbidden
FI inserts directly into Pathfinder   -> forbidden
```

Use controlled API/message boundaries with schema and semantic validation.

A compromised authenticated peer is still untrusted input outside its permitted authority.

Integration outage or authentication failure means coverage is degraded. It does not mean no observation occurred.

## Enforcement Boundary

Pathfinder is not an autonomous enforcement authority.

Do not silently turn:

```text
Indicator
Assessment
IOC match
high confidence
source consensus
ThreatActor association
Campaign association
ATT&CK mapping
```

into:

```text
firewall block
endpoint isolation
account disablement
file deletion
configuration change
quarantine
destructive remediation
```

A Pathfinder candidate action is a recommendation, not a command.

Downstream systems retain their own validation, simulation, authorization, commit, audit, and rollback rules.

## Security and Trust Rules

Authentication is not authorization.

Authorization is not semantic validity.

No credential, API token, certificate, signing key, service principal, analyst identity, or integration identity becomes universal authority.

Machine-to-machine integration direction prefers mTLS and narrow identity binding.

Network location, localhost, RFC1918 addressing, management VLAN membership, VPN presence, or caller-provided product name does not establish authority.

Protected operations fail closed when required authorization/trust cannot be established.

## Secrets and Sensitive Material

Do not intentionally log or persist secrets outside their explicitly defined protected storage boundaries.

Examples include:

```text
private keys
API tokens
feed credentials
OAuth tokens
client secrets
database credentials
session credentials
protected customer information
restricted intelligence-source credentials
```

Raw source content may itself contain secrets or restricted data.

Diagnostics, errors, traces, crash output, support bundles, and metrics must not become accidental raw-source or secret stores.

Prefer stable record/request identifiers for diagnostic correlation.

## Marking and Handling Rules

Do not discard source handling restrictions during normalization, correlation, export, or integration.

A technically public Observable extracted from a restricted report does not automatically make the report, Assertion, context, or attribution unrestricted.

Raw SourceArtifact access may require stronger authorization than normalized-intelligence access.

## Retention and Destruction

Retention, lifecycle, archive, and physical destruction are separate domains.

```text
expired != destroy
revoked != destroy
superseded != destroy
archive != destroy
storage pressure != destruction authority
```

A hold overrides ordinary destruction eligibility where applicable.

Authorized destruction of raw bytes preserves the historical record of what existed and why it was destroyed.

## Search and Index Rules

Indexes, caches, and current views are derived structures.

> **Pathfinder must never present an incomplete index as complete intelligence history.**

A query over incomplete, stale, rebuilding, failed, unavailable, or partially processed data must expose the applicable condition.

```text
no indexed result != no intelligence exists
no visible result != no restricted matching record exists
```

Current views and indexes may be rebuilt from historical records. Historical records may not be replaced by those derived structures.

## Time Rules

Preserve distinct time semantics, including where applicable:

```text
source observation time
source publication time
source modified time
retrieval start/completion
Pathfinder receipt time
preservation time
processing time
local Sighting time
Assessment time
lifecycle effective time
lifecycle recorded time
supersession time
```

Do not substitute receipt time when observation time is unknown.

Timestamp precision is not timestamp accuracy.

UUIDv7 ordering does not replace explicit timestamps or committed ordering where order matters.

## Success and Failure Discipline

Do not infer success from absence of an error.

Do not represent partial processing as complete processing.

Do not erase an earlier failure because a retry later succeeds.

If Pathfinder cannot establish something, use an explicit unknown/not-known/not-verified/not-performed state rather than inventing a value.

Success is returned only after the required durable commit boundary succeeds.

Collectors advance checkpoints only after the required safe preservation/commit boundary succeeds.

## Resource Priority

When resources are constrained, authoritative history and source preservation take priority over rebuildable convenience work where architecture permits.

General intent:

```text
1. safely receive and preserve required SourceArtifacts
2. protect authoritative historical records
3. commit required processing/change/audit history
4. maintain correctness/readiness
5. normalize and correlate
6. index and build current views
7. enrich
8. report/export/presentation
9. optional analytics and convenience work
```

Exact runtime priorities must be measured and implemented deliberately.

## Scope Discipline

Work only within the current roadmap phase and approved engineering slice.

Current next phase is:

```text
Phase 1.1 — Runtime and Repository Foundation
```

Do not skip directly into broad feed implementation or a complete speculative object schema.

Do not prematurely implement:

```text
large feed catalogs
generic plugin ecosystems
automated enforcement
AI analyst replacement
complex graph infrastructure
multi-product orchestration
broad SOAR functionality
full TIP feature parity
large-scale enrichment farms
unbounded analytics
polished UI
```

unless explicitly added to the current approved roadmap slice.

## Phase 1 Schema Discipline

Phase 1.2 implements the relational schema required for the first complete vertical slice.

It does not require tables for every conceptual object immediately.

Before implementing additional object families, define the concrete schema contract required by the actual workload.

Do not invent fields merely because Phase 0 named a conceptual object.

## Repository Operations

Do not perform repository writes unless explicitly authorized for the specific action or change set.

This includes:

```text
commit
push
merge
pull-request creation
branch changes
tag creation
release publication
issue creation
ruleset changes
repository settings changes
repository-content deletion
```

Read-only repository inspection and local/offline work are permitted unless explicitly restricted.

Permission to create or modify local working files is not permission to commit or push them.

Permission for one repository write applies only to the specifically approved action or change set and does not carry forward automatically.

## Engineering Completeness Test

> **When this fails at 2:00 AM, will Pathfinder tell the operator exactly what it received, where it came from, what it could validate, what it understood, how it reached that interpretation, what remains uncertain, and what failed?**

Important paths should preserve enough state and history to answer:

```text
What Source supplied this information?
What SourceArtifact was preserved?
What logical SourceRecord was processed?
What exactly did the source assert?
When did Pathfinder receive it?
Was the original artifact preserved and verified?
Did parsing succeed?
Did normalization succeed?
Which process/version produced the interpretation?
What relationships were source-normalized?
What relationships were derived?
What corroborated the Assessment?
Were those sources actually independent?
What confidence was assigned and by whom?
Is conflicting intelligence present?
What is the current interpretation and why?
What was the interpretation at an earlier time?
Has the intelligence expired, been revoked, disputed, or superseded?
What did Pathfinder fail to establish?
Is source/search/index coverage complete?
Was a downstream recommendation produced?
Was any downstream action separately authorized?
```

## Review Expectations

Before proposing a change as complete, verify applicable:

```text
source-preservation
original-history
provenance
truth-separation
normalization
correlation
confidence
assessment
current-view
lifecycle
conflict
security
trust
authorization
retention
search/index
time
integration
enforcement-boundary
failure-state
state-namespace
scope
repository-write
```

requirements remain intact.

Also verify that:

- SourceArtifact and SourceRecord were not collapsed;
- observations have not silently become conclusions;
- correlation has not silently become identity;
- duplicated intelligence has not silently become corroboration;
- current knowledge has not rewritten historical knowledge;
- current-view selection does not use hidden last-write-wins;
- confidence has not silently become authorization;
- ATT&CK mapping has not become a prerequisite for threat significance;
- incomplete search/index/coverage state is not presented as complete;
- ChangeSet atomicity has not been weakened;
- failures and uncertainty remain visible;
- future roadmap work has not been pulled forward without approval; and
- applicable repository-owned validation and tests have been run.

Clearly report any validation or test that could not be executed.

## Nested AGENTS.md Files

Nested `AGENTS.md` files may refine subtree-specific requirements but must not silently weaken repository-wide:

```text
source preservation
original historical record preservation
provenance
truthfulness
uncertainty
correlation boundaries
assessment boundaries
current-view rules
security
authorization
retention
history
search completeness
integration authority
enforcement boundaries
state namespaces
ChangeSet atomicity
scope discipline
repository-write restrictions
```

requirements.
