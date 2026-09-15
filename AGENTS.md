# Pathfinder Agent and Contributor Rules

## Purpose

This file defines how contributors, coding agents, and automation work within the Pathfinder repository.

It is a behavioral contract. It does not replace a narrower committed architecture, protocol, schema, data-model, or implementation contract.

For current implementation sequence, [`ROADMAP.md`](ROADMAP.md) governs.

For current operational architecture, these documents govern their defined boundaries:

- [`docs/OPERATIONAL-INTELLIGENCE-ARCHITECTURE.md`](docs/OPERATIONAL-INTELLIGENCE-ARCHITECTURE.md)
- [`docs/SENSOR-OBSERVATION-INGESTION.md`](docs/SENSOR-OBSERVATION-INGESTION.md)
- [`docs/CORRELATION-HISTORICAL-REPROCESSING.md`](docs/CORRELATION-HISTORICAL-REPROCESSING.md)

The Phase 0 reconciliation remains the authority for conflicts among earlier Phase 0 semantic wording:

[`docs/PHASE-0-RECONCILIATION-EXIT.md`](docs/PHASE-0-RECONCILIATION-EXIT.md)

A later narrowly scoped reconciliation may refine implementation direction without erasing historical completion records.

## Governing Principles

> **Preserve what was received before deciding what it means.**

> **Record the claim before judging the claim.**

> **Preserve what was actually observed before deciding what the observation means.**

> **An Observable is not inherently malicious.**

> **A source Assertion is not automatically a fact.**

> **A Sighting is an observation, not a conclusion.**

> **Correlation is not identity.**

> **Applicability is not guaranteed by a match.**

> **Association is not attribution.**

> **Confidence describes a judgment. Reliability describes a source. Corroboration describes support. They are not interchangeable.**

> **The original record is maintained no matter what.**

> **Current knowledge must not rewrite historical knowledge.**

> **Derived intelligence remains traceable to the source material, observations, and processing lineage from which it was produced.**

> **Conflicting intelligence remains visible rather than being silently averaged into false certainty.**

> **An incomplete index or incomplete sensor coverage is not complete history.**

> **ATT&CK is a classification aid, not a prerequisite for understanding or proving a compromise.**

> **High confidence is still not authorization.**

> **Pathfinder can inform a decision. Pathfinder does not silently become the authority to execute that decision.**

## Product Authority

> **Pathfinder owns the organization's record and interpretation of threat intelligence.**

Pathfinder is authoritative for its threat-intelligence records, provenance, processing history, correlations, applicability results, Assessments, conflicts, lifecycle, and governed change history.

It is not automatically authoritative for the real-world truth of an external Assertion.

A Pathfinder sensor is authoritative only for the record of what that identified sensor observed at its documented vantage and under its documented collection semantics.

```text
sensor observation
    != authoritative full network truth

sensor match
    != maliciousness

sensor absence under incomplete coverage
    != proof of absence
```

ISS product authority remains separated:

```text
Atlas
    asset/environment context

FI
    file and file-system observations

Stronghold
    Stronghold network observations and enforcement decisions

Pathfinder Sensors
    observations made by each identified Pathfinder sensor

Pathfinder
    organizational threat-intelligence record,
    correlation, applicability, and interpretation

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
record observations
derive Sightings
correlate
evaluate applicability
enrich
assess
relate
reprocess history
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
a general-purpose SIEM
a universal event lake
a firewall
an EDR
an endpoint enforcement system
an identity authority
an asset authority
a vulnerability scanner
an authoritative full-packet-capture system
a backup/recovery authority
a SOAR platform
an autonomous remediation engine
```

The sensor observation plane exists to make threat intelligence operational, not to ingest data merely because data exists.

## Three Categories of Truth

Preserve the distinction between:

```text
WHAT THE SOURCE SAID
    preserved source material
    attributable Assertion

WHAT WAS OBSERVED
    sensor observations
    Sightings from identified observation authorities

WHAT PATHFINDER UNDERSTOOD
    normalized intelligence
    correlation
    applicability
    Relationships
    Assessments
```

These categories must not be silently collapsed.

Examples:

```text
Source A says 203.0.113.17 is C2
    -> Assertion

UDM-A observes endpoint X contacting 203.0.113.17
    -> observation / Sighting

Pathfinder determines Source A's C2 assertion applies to that Sighting
    -> Assessment
```

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

Sensor-observation storage may introduce observation-specific records required by the sensor contract. Do not collapse those records into Intelligence objects merely because both are stored in Pathfinder.

## SourceArtifact and SourceRecord

The canonical intelligence-source path remains:

```text
Source
  |
  v
SourceCollection
  |
  v
RetrievalEvent
  |
  v
SourceArtifact
  |
  v
SourceRecord
  |
  v
Assertion
  |
  v
normalized intelligence
```

`SourceArtifact` preserves the exact immutable acquired payload at the preservation boundary.

`SourceRecord` is a logical item represented within that artifact.

```text
SourceArtifact != SourceRecord
SourceRecord   != Assertion
```

Parser output or reserialized JSON is not the original source bytes.

## Sensor Observation and Sighting

A high-volume sensor observation and a Pathfinder Sighting are related but not identical.

```text
sensor observation
    raw/direct observation context

Sighting
    Pathfinder intelligence record that an identified authority
    observed a relevant Observable
```

Sighting derivation must be versioned, attributable, traceable, and non-destructive.

```text
duplicate delivery != repeated observation
reprocessed observation != new historical event
Sighting != maliciousness
Sighting != compromise
Sighting != attribution
```

## Original-History Rule

> **The original record is maintained no matter what.**

Do not destructively correct historical records.

If a record is later found to be wrong, stale, revoked, superseded, disputed, conflicted, sensor-generated in error, parser-generated in error, analyst-entered in error, or produced by a compromised account, preserve the original and create the proper forward-moving corrective record.

```text
record incorrect  != record deleted
revoked           != deleted
superseded        != deleted
disputed          != deleted
conflict resolved != opposing record deleted
reprocess         != rewrite
revert            != erase
```

If mandatory policy requires destruction of preserved raw bytes, retain the metadata and destruction history required by the applicable preservation contract.

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

Input validation is mandatory.

Parsing failure remains visible.

Unsupported input remains distinguishable from malformed input.

Normalization must not manufacture information absent from the source.

Preserve required source bytes before semantic parsing or destructive transformation.

Do not automatically execute, render, dereference, resolve, fetch, or activate source-supplied content merely because it was received.

## Sensor Intake Rules

Authenticated sensors are still untrusted input outside their permitted observation authority.

Treat sensor deliveries as potentially:

```text
malformed
duplicated
replayed
out of order
partial
oversized
schema-unsupported
timestamp-invalid
coverage-degraded
produced by a compromised sensor
```

Sensor ingestion must use a versioned contract.

Sensors do not write directly to Pathfinder database tables.

Acknowledgement/checkpoint behavior must be idempotent and durable.

A valid sensor certificate or identity proves the configured sender identity. It does not prove the observation's interpretation.

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

Sighting may be an Assessment subject where the assessment type permits it.

Do not implement judgment as a mutable universal truth field such as:

```text
indicator.malicious = true
```

when the value actually belongs to an Assessment.

## Correlation Rules

Correlation establishes that records intersect under a defined algorithm.

Correlation does not establish applicability or maliciousness.

Do not silently convert:

```text
Observable match
IP match
domain match
hash match
certificate match
periodic traffic
```

into:

```text
malicious
C2 session
compromise
campaign participation
attribution
```

Correlation output must preserve the records used, processing/profile version, time, and applicable coverage/index state.

## Applicability Rules

After correlation, evaluate only dimensions supported by available information.

Potential dimensions include:

```text
time
platform
product/software
version
campaign
malware family
infrastructure role
geography
service/technology
observed behavior
source lifecycle
conflicting intelligence
```

Unknown is first-class.

```text
unknown platform       != match
unknown time window    != match
shared IP              != C2 session
source says Roku-only  != applies to tvOS
```

Applicability must not silently infer scope absent from the source or authorized local context.

## Infrastructure Role Rules

Roles such as:

```text
command_and_control
payload_delivery
redirector
phishing
shared_hosting
compromised_infrastructure
legitimate_service
```

must be attributable to a source Assertion or an explicitly governed Pathfinder Assessment/process.

An IP appearing in a report is not sufficient to classify every connection to that IP as C2.

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

A singular current interpretation requires explicit supersession, conflict resolution, or another versioned approved current-view rule.

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
applicability
```

into one unexplained score.

Source reliability is a historical Assessment of Source or SourceCollection, not one mutable field.

Multiple deliveries of one upstream report are not independent corroboration.

Repeated Sightings are not repeated independent corroboration of maliciousness.

`NOT_KNOWN` source independence never counts as `INDEPENDENT` by default.

## Relationship Rules

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

No relationship is assumed symmetric or transitive unless its registry definition explicitly says so.

Do not infer a relationship merely because objects co-occur in a report, bundle, graph neighborhood, or time window.

## ATT&CK Rules

ATT&CK is optional classification/reference metadata.

Pathfinder must never require an ATT&CK technique, tactic, mapping, or sequence before it can preserve, assess, prioritize, or escalate activity.

```text
ATT&CK NOT_MAPPED != low significance
Technique overlap != attribution
Technique matched != actor identity
Technique matched != malware identity
```

Do not manufacture mappings to improve apparent coverage.

## Sighting Rules

A Sighting is an observation, not a conclusion.

Preserve:

```text
observer authority
event time
receipt time
Observable
context
provenance
coverage context where applicable
```

Duplicate delivery is not repeated observation.

Aggregate observation is not fabricated individual event history.

No-Sighting results require coverage context.

## Historical Reprocessing Rules

New intelligence may cause Pathfinder to re-evaluate old Sightings.

Historical reprocessing must preserve:

```text
original SourceArtifact
original SourceRecord
original Assertion
original sensor observation
original Sighting
prior Assessment
prior ProcessingRecord
```

Reprocessing creates new derived state and ProcessingRecords and may create new Assessments.

It must never make later knowledge appear to have existed earlier.

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

Conflict resolution never deletes the opposing Assertion or Assessment.

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

Sensor delivery/coverage states must also remain typed rather than being collapsed into an unrelated generic status.

## Raw and Derived Data

Original source information, original sensor observations, and derived Pathfinder information are separate data classes.

Derived processing MUST NOT modify preserved original records because later interpretation changes.

Parser, mapper, normalizer, correlation, applicability, or reprocessing upgrades create new processing lineage and derived results.

They must not make later understanding appear to have existed during the original event.

## ChangeRecord and ChangeSet Rules

`ChangeRecord` records an intentional material change to governed current interpretation, workflow, or configuration.

`ChangeSet` groups one logical set of ChangeRecords.

A committed ChangeSet is atomic:

```text
all semantic changes commit
    or
none commit
```

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
    parser / normalizer / mapper / correlation /
    applicability / lifecycle / conflict / reprocessing lineage
```

These histories may reference one another but must not be collapsed into one ambiguous universal event table.

## Deduplication Rules

Deduplication must not destroy provenance.

Distinguish:

```text
same Observable
same SourceArtifact
same SourceRecord
duplicate source delivery
duplicate sensor delivery
replayed batch
repeated real observation
redistributed source
independent corroboration
```

Identical canonical Observables may resolve to one Observable while every source record, Assertion, delivery path, and observation remains traceable.

## STIX / TAXII Rules

STIX and TAXII are interoperability boundaries, not Pathfinder's internal truth model.

```text
valid STIX != intelligence trusted
TAXII success != content accepted
same STIX object from many feeds != independent corroboration
```

STIX translation loss and unsupported semantics remain explicit.

TAXII transport, pagination, source preservation, STIX validation, native mapping, commit, retry, and checkpoint state remain separate.

## Integration Rules

ISS integrations use explicitly versioned contracts and narrow authenticated identities.

No direct cross-product database writes.

```text
Pathfinder writes Atlas tables       -> forbidden
Stronghold edits Pathfinder tables   -> forbidden
FI inserts directly into Pathfinder  -> forbidden
sensor writes Pathfinder tables      -> forbidden
```

Use controlled API/message boundaries with schema and semantic validation.

A compromised authenticated peer remains untrusted outside its permitted authority.

Integration outage or authentication failure means coverage may be degraded. It does not mean no observation occurred.

## SIEM Boundary

Pathfinder is not a general-purpose SIEM.

Do not add broad event collection merely because it may someday be useful.

In-scope sensor data should have a clear relationship to:

```text
Observable
Sighting
threat relevance
applicability
coverage
historical correlation
```

Out of scope by default:

```text
all Windows event logs
all syslog
generic application logs
general infrastructure metrics
generic authentication dashboards
SOC case-management queues
unbounded event retention
```

A future SIEM may exchange selected data with Pathfinder through a defined contract.

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

No credential, token, certificate, key, service identity, analyst identity, or integration identity becomes universal authority.

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

Raw source content and sensor context may itself contain restricted data.

Diagnostics, errors, traces, crash output, support bundles, and metrics must not become accidental raw-source, observation, or secret stores.

## Marking and Handling Rules

Do not discard source handling restrictions during normalization, correlation, reprocessing, export, or integration.

A technically public Observable extracted from a restricted report does not automatically make the report, Assertion, context, or attribution unrestricted.

Raw SourceArtifact access may require stronger authorization than normalized-intelligence access.

Sensor observation access may require controls distinct from public threat-intelligence access.

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

## Search, Index, and Coverage Rules

Indexes, caches, and current views are derived structures.

> **Pathfinder must never present an incomplete index or incomplete observation coverage as complete history.**

A query over incomplete, stale, rebuilding, failed, unavailable, or partially processed data must expose the applicable condition.

```text
no indexed result != no intelligence exists
no Sighting under degraded coverage != activity did not occur
no visible result != no restricted matching record exists
```

## Time Rules

Preserve distinct time semantics, including where applicable:

```text
source observation time
source publication time
source modified time
retrieval start/completion
Pathfinder receipt time
preservation time
sensor observation/event time
sensor batch time
sensor receipt time
Sighting time
processing time
Assessment time
lifecycle effective time
lifecycle recorded time
supersession time
reprocessing time
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

Collectors and sensors advance checkpoints only after the applicable safe preservation/commit boundary succeeds.

## Resource Priority

When resources are constrained, authoritative history and required original records take priority over rebuildable convenience work where architecture permits.

General intent:

```text
1. safely receive/preserve required source and observation records
2. protect authoritative historical records
3. commit required processing/change/audit history
4. maintain correctness/readiness/coverage truth
5. normalize and derive Sightings
6. correlate and evaluate applicability
7. assess/reprocess
8. index and build current views
9. enrich/report/export
10. optional analytics and presentation
```

Exact runtime priorities must be measured and implemented deliberately.

## Scope Discipline

Work within the current roadmap phase and approved engineering slice.

Current implementation sequence begins at:

```text
Phase 1.5 — Observable Normalization
```

The immediate core path is:

```text
Observable normalization
Relationships
Sightings
sensor ingestion
sensor observation store
one high-context network threat source
correlation
applicability / Assessment
historical reprocessing
query
validation
```

Do not prematurely implement:

```text
dozen-plus feed catalogs
generic plugin ecosystems
general-purpose SIEM ingestion
automated enforcement
AI analyst replacement
complex graph infrastructure
broad SOAR functionality
large-scale enrichment farms
polished UI
unbounded analytics
```

unless explicitly added to the current approved roadmap slice.

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

Permission for one repository write applies only to the specifically approved action/change set and does not carry forward automatically.

## Engineering Completeness Test

> **When this fails at 2:00 AM, will Pathfinder tell the operator exactly what it received, what it observed, where each record came from, what it could validate, what it correlated, why the intelligence did or did not apply, what it concluded, what remains uncertain, and what failed?**

Important paths should preserve enough state and history to answer:

```text
What Source supplied this information?
What SourceArtifact was preserved?
What logical SourceRecord was processed?
What exactly did the source assert?
Which sensor observed the local activity?
What exactly did the sensor observe?
What was sensor coverage at the time?
Which Observable was normalized?
Which Sighting was created?
What correlation matched?
Which processing/version produced it?
Which applicability dimensions matched?
Which remained unknown?
What Assessment was created and why?
What confidence was assigned and by whom?
Is conflicting intelligence present?
What was the interpretation earlier?
What changed during reprocessing?
Has intelligence expired, been revoked, disputed, or superseded?
Is source/sensor/search/index coverage complete?
Was any downstream recommendation produced?
Was downstream action separately authorized?
```

## Review Expectations

Before proposing a change as complete, verify applicable:

```text
source preservation
observation preservation
original history
provenance
truth separation
normalization
Sighting semantics
correlation
applicability
confidence
Assessment
current view
historical reprocessing
lifecycle
conflict
security
trust
authorization
retention
search/index/coverage
time
integration
SIEM boundary
enforcement boundary
failure state
typed states
scope
repository-write permission
```

Also verify that:

- SourceArtifact and SourceRecord were not collapsed;
- sensor observations and Sightings were not silently collapsed where the distinction matters;
- observations have not silently become conclusions;
- correlation has not silently become identity, applicability, maliciousness, or C2;
- duplicated delivery has not become repeated observation or corroboration;
- current knowledge has not rewritten historical knowledge;
- current-view selection does not use hidden last-write-wins;
- confidence has not silently become authorization;
- incomplete search/index/coverage is not presented as complete;
- failures and uncertainty remain visible;
- Pathfinder has not drifted into general-purpose SIEM scope without an explicit roadmap decision; and
- applicable repository-owned validation has been run.

Clearly report any validation or test that could not be executed.

## Nested AGENTS.md Files

Nested `AGENTS.md` files may refine subtree-specific requirements but must not silently weaken repository-wide:

```text
source preservation
observation preservation
original history
provenance
truthfulness
uncertainty
correlation boundaries
applicability boundaries
Assessment boundaries
historical reprocessing rules
current-view rules
security
authorization
retention
search/index/coverage truth
integration authority
SIEM boundary
enforcement boundary
typed state namespaces
ChangeSet atomicity
scope discipline
repository-write restrictions
```
