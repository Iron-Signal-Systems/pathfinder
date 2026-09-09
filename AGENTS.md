# Pathfinder Agent and Contributor Rules

## Purpose

This file defines how contributors, coding agents, and automation work within the Pathfinder repository.

It is a behavioral contract and does not replace governing architecture, roadmap, schema, protocol, data-model, or implementation-contract documents.

Where a more specific committed Pathfinder contract exists, that contract governs its defined implementation boundary.

## Governing Principles

> **Preserve the source before interpreting the source.**

> **An observable is not inherently malicious.**

> **A source assertion is not automatically a fact.**

> **A sighting is an observation, not a conclusion.**

> **Correlation is not identity.**

> **Confidence is not authorization.**

> **Absence from intelligence is not proof of benign status.**

> **Shared infrastructure is not proof of common ownership or operation.**

> **Current knowledge must not rewrite historical knowledge.**

> **Derived intelligence remains traceable to the source material from which it was produced.**

> **Conflicting intelligence remains visible rather than being silently averaged into false certainty.**

> **An incomplete index is not complete intelligence history.**

> **Expiration eligibility is not permission to destroy historical intelligence.**

> **Pathfinder records what it can establish, preserves the source of that truth, and never manufactures certainty beyond it.**

> **Pathfinder intelligence may inform enforcement, but intelligence alone does not create enforcement authority.**

## Product Model

Pathfinder is a threat-intelligence system.

Its responsibilities may include:

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
an autonomous remediation engine
```

Other Iron Signal Systems products may provide observations or consume Pathfinder intelligence through explicitly defined integration boundaries.

Those integrations do not transfer product authority.

For example:

```text
Atlas
    owns its authoritative asset/environment state

FI
    owns its authoritative file observations

Stronghold
    owns its authoritative network observations and decisions

Guidon
    owns its authoritative backup/recovery state

Pathfinder
    owns its threat-intelligence records, relationships,
    assessments, provenance, and intelligence lifecycle
```

Correlation across ISS systems creates derived intelligence unless a governing contract explicitly defines otherwise.

## Three Categories of Truth

Preserve the distinction between:

```text
WHAT THE SOURCE SAID
    preserved source material / source assertion

WHAT WAS OBSERVED
    sightings and observations from an identified source or system

WHAT PATHFINDER UNDERSTOOD
    normalized, correlated, enriched, or assessed intelligence
```

These categories must not be silently collapsed.

Derived intelligence remains traceable to its applicable source material and processing lineage.

Reprocessing may create new or superseding interpretation but MUST NOT rewrite the original source record to make it appear that later understanding existed when the source was originally received.

A source saying that an IP address is malicious is a source assertion.

A Pathfinder-connected system observing communication with that IP address is a sighting.

Pathfinder associating that sighting with infrastructure, malware, a campaign, or a threat actor is derived interpretation.

Those are separate facts.

## Core Intelligence Objects

Pathfinder should preserve explicit distinctions among intelligence concepts such as:

```text
Source
Source Record
Observable
Indicator
Sighting
Assessment
Relationship
Threat Actor
Campaign
Malware
Tool
Vulnerability
Technique
Infrastructure
Report
```

The existence of one object type must not silently imply another.

For example:

```text
observable present              != indicator established
indicator present               != maliciousness proven
sighting present                != compromise established
malware hash match              != malware execution established
campaign association            != actor attribution established
actor attribution               != attribution proven
ATT&CK mapping                  != technique observed
infrastructure relationship     != infrastructure ownership
```

More specific schemas and contracts may refine these object classes.

## Source Intake Rules

External intelligence is untrusted input.

Treat all source material as potentially:

```text
malformed
incomplete
stale
duplicated
contradictory
unexpectedly large
unexpectedly encoded
schema-invalid
semantically invalid
hostile
```

Input validation is not optional merely because the source is reputable.

Do not allow malformed source material to silently become valid normalized intelligence.

Parsing failure must remain visible.

Unsupported source content must remain distinguishable from invalid content.

Normalization must not manufacture information that was absent from the source.

Where source preservation is required, preserve the source representation before transformation.

Preserve applicable provenance such as:

```text
provider
feed / collection
source record identifier
source publication time
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

Source-specific parsing belongs at the source boundary. Do not contaminate the core intelligence model with arbitrary feed-specific semantics when a clean boundary can preserve them.

## Source Reliability and Confidence Rules

Do not collapse all intelligence quality into one unexplained numeric score.

Preserve separate concepts where applicable, including:

```text
source reliability
information confidence
analyst confidence
corroboration
age
recency
independent-source count
assessment status
```

A reliable source can publish uncertain information.

Highly confident historical intelligence can still be operationally stale.

Multiple reports are not independent corroboration merely because Pathfinder received multiple copies of the same upstream source.

Do not inflate confidence because the same originating intelligence was redistributed through several feeds.

Confidence must not silently create enforcement authority.

## Correlation Rules

Correlation is derived intelligence.

Do not silently convert:

```text
possibly_related
shares_infrastructure_with
reported_with
observed_with
resolves_to
communicates_with
uses_certificate
uses_domain
uses_ip
shares_artifact_with
```

into:

```text
is
owns
operates
controls
same_entity
malicious
compromised
```

without an explicit supported basis.

Relationship candidates and confirmed relationships are different states.

Historical relationships are time-bounded where the underlying facts are time-dependent.

For example:

```text
IP -> hostname
domain -> IP
certificate -> service
infrastructure -> provider
account -> organization
malware -> infrastructure
```

must not automatically be treated as timeless relationships.

Current DNS resolution does not rewrite historical DNS state.

Current WHOIS/registration data does not rewrite historical registration state.

Current infrastructure ownership does not automatically establish historical infrastructure ownership.

## Common Truth Separations

Never collapse these distinctions:

```text
source received                    != source trusted
source authenticated               != information true
source available                   != source reliable
source record preserved            != source record understood
record parsed                      != record semantically valid
record normalized                  != intelligence confirmed
observable                         != malicious indicator
indicator                          != compromise
indicator match                    != malicious activity established
sighting                           != compromise
sighting                           != attribution
source assertion                   != Pathfinder conclusion
source confidence                  != Pathfinder confidence
high confidence                    != enforcement authorized
multiple feeds                     != independent corroboration
duplicate reports                  != corroborating reports
shared IP                          != same infrastructure operator
shared certificate                 != same threat actor
shared domain                      != same campaign
shared malware                     != same actor
relationship candidate             != confirmed relationship
association                        != identity
correlation                        != causation
current relationship               != historical relationship
current DNS resolution             != historical DNS resolution
current IP owner                   != historical IP owner
current domain owner               != historical domain owner
current assessment                 != historical assessment
new interpretation                 != new historical observation
reprocessing result                != original source knowledge
source corrected                   != old source never existed
indicator expired                  != indicator historically absent
indicator expired                  != destruction authorized
indicator revoked                  != historical record erased
assessment superseded              != previous assessment deleted
assessment disputed                != assessment disproven
conflicting intelligence           != intelligence absent
no indexed result                  != no intelligence exists
index unavailable                  != source history unavailable
processing failed                  != source record lost
parser failure                     != source invalid
unsupported record                 != malformed record
enrichment unavailable             != observable invalid
ATT&CK technique associated        != technique observed
CVE associated                     != asset vulnerable
asset vulnerable                   != exploit observed
exploit observed                   != compromise established
malware hash matched               != malware executed
malware executed                   != successful compromise
threat actor associated            != attribution proven
campaign associated                != actor identified
IP reputation                      != endpoint identity
domain reputation                  != destination intent
certificate match                  != service ownership
STIX valid                         != intelligence trusted
TAXII transport succeeded          != source content accepted
API access                         != analyst authority
analyst authority                  != enforcement authority
Pathfinder recommendation          != downstream action approved
```

## Failure-State Discipline

Preserve meaningful states such as:

```text
unknown
not_known
not_observed
not_verified
not_performed
not_present
not_applicable
unsupported
degraded
unavailable
mismatch
failed
partial
processing_pending
processing_failed
index_incomplete
source_unavailable
source_authentication_failed
source_rate_limited
source_schema_mismatch
parse_failed
normalization_failed
enrichment_unavailable
correlation_pending
conflicted
disputed
expired
revoked
superseded
```

Do not infer success from the absence of an error.

Do not represent partial processing as complete processing.

A later successful retry does not erase an earlier failed operation.

If Pathfinder could not determine something, report that state rather than inventing a value.

## Raw Source and Derived Data Rules

Original source information and derived Pathfinder information are separate data classes.

Where the source-preservation contract requires retention of original material:

```text
SOURCE
    ↓
preserve
    ↓
validate
    ↓
parse
    ↓
normalize
    ↓
correlate
    ↓
enrich
    ↓
assess
    ↓
publish
```

Derived processing MUST NOT modify the preserved original merely because the interpretation changes.

Parser upgrades may result in new derived representations.

They must not make it appear that those representations were known during the original processing event.

Where practical, derived records should retain sufficient lineage to identify:

```text
source record
parser
parser version
normalizer
normalizer version
correlation process
assessment authority
processing time
```

## Deduplication Rules

Deduplication must not destroy provenance.

Two identical observables from two genuinely independent intelligence sources may constitute two source relationships.

Two copies of one upstream report received through different redistributors do not automatically constitute two independent sources.

Distinguish:

```text
same observable
same source record
duplicate delivery
redistributed source
independent corroboration
```

Do not use deduplication to erase useful source history.

## Assessment Rules

An assessment must preserve who or what made the assessment and what information supported it where applicable.

Machine-generated, source-provided, and human-analyst assessments must remain distinguishable.

Pathfinder must not silently represent automated inference as analyst judgment.

Changing an assessment does not rewrite the prior assessment as though it never existed.

Conflicting assessments may coexist.

Where one assessment supersedes another, the relationship must remain explicit.

## Intelligence Lifecycle Rules

Intelligence may have lifecycle states such as:

```text
active
aging
expired
revoked
superseded
disputed
conflicted
```

Exact lifecycle vocabulary is governed by committed schemas/contracts once frozen.

Expiration indicates that an intelligence assertion should no longer be treated as currently valid under the applicable rule.

Expiration does not mean:

```text
historically false
historically absent
safe
benign
authorized for deletion
```

Revocation similarly does not authorize destruction of historical records.

Historical intelligence may remain valuable for incident reconstruction, hunting, forensics, source evaluation, and retrospective analysis.

## STIX / TAXII Rules

STIX and TAXII interoperability must not silently redefine Pathfinder's internal truth model.

Valid STIX syntax does not prove that its assertions are true.

Successful TAXII transport does not prove that received intelligence is valid, trusted, relevant, or accepted.

Imported external identifiers should be preserved where required for interoperability.

Pathfinder-native identity and external-source identity must not be silently conflated.

Translation between Pathfinder's internal model and STIX must preserve meaningful distinctions and expose any translation loss.

Do not discard Pathfinder provenance or uncertainty merely because an external format cannot represent it exactly.

## ATT&CK Rules

MITRE ATT&CK relationships are intelligence classifications and references.

An ATT&CK technique association does not by itself establish that the technique actually occurred in a local environment.

Keep distinctions such as:

```text
source reports technique
Pathfinder associates technique
local telemetry suggests technique
local telemetry confirms defined observation
```

according to the applicable future contracts.

Do not manufacture ATT&CK mappings merely to increase apparent coverage.

## Integration Rules

Pathfinder integrations consume or provide explicitly scoped information.

An integration does not silently transfer authority between products.

Examples:

```text
FI file observation
    != Pathfinder malware conclusion

Stronghold network observation
    != Pathfinder threat conclusion

Pathfinder threat conclusion
    != Stronghold enforcement authorization

Atlas asset relationship
    != Pathfinder asset authority

Pathfinder vulnerability relationship
    != Atlas vulnerability fact unless accepted under Atlas rules
```

Pathfinder may produce recommendations or candidate actions.

Downstream systems retain their own authorization and enforcement boundaries.

## Enforcement Boundary

Pathfinder is not an autonomous enforcement authority.

Do not silently turn:

```text
indicator
assessment
IOC match
high confidence
source consensus
threat actor association
campaign association
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

A future integration may explicitly support candidate enforcement actions, but authorization, validation, simulation, approval, commit, and audit boundaries must be defined by the applicable downstream contract.

> **High confidence is still not authorization.**

## Security and Trust Rules

External feeds, APIs, analysts, service identities, automation identities, and ISS product integrations are distinct trust domains unless an explicit contract establishes otherwise.

No credential, source identity, analyst identity, service identity, API token, certificate, signing key, or integration identity silently becomes universal authority.

Keep separate, where applicable:

```text
source authentication
service authentication
human authentication
analyst authority
administrative authority
API authority
publication authority
integration authority
enforcement authority
```

Authentication is not authorization.

Access to intelligence is not authority to change intelligence.

Authority to change an assessment is not authority to destroy history.

Authority to publish intelligence is not authority to enforce it.

Fail closed where a protected operation requires authorization or trust that could not be established.

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

Diagnostics, parser failures, source preservation, support bundles, traces, crash artifacts, and exports must not become accidental secret-storage paths.

Source content may itself contain sensitive or restricted information and must be handled according to its applicable marking and handling rules.

## Data Marking and Handling Rules

Do not discard source handling restrictions, distribution markings, or classification metadata merely because the underlying observable is technically simple.

For example, the fact that an IP address itself is public does not necessarily mean the source assertion, context, attribution, or report containing it is unrestricted.

Any future support for TLP, proprietary-source markings, government handling markings, customer-private intelligence, or other distribution controls must preserve those boundaries explicitly.

Do not broaden access during normalization, correlation, export, or integration.

## Retention / Destruction Rules

Source records, normalized intelligence, derived relationships, assessments, sightings, and processing history may require distinct retention policies.

Age or expiration establishes eligibility under a retention policy; it does not itself create destruction authority.

A hold overrides ordinary expiration where applicable.

Archive is not destruction.

Storage pressure must not silently create permission to rewrite or destroy intelligence history.

Manual destructive operations require explicit authority where defined.

## Search and Index Rules

Indexes are derived structures.

The authoritative source/intelligence records and the indexes used to find them are not the same thing.

> **Pathfinder must never present an incomplete index as complete intelligence history.**

A query over incomplete, rebuilding, unavailable, or partially processed indexes must expose that condition rather than silently returning an unqualified:

```text
no results
```

Rebuilding an index must not rewrite authoritative source or intelligence history.

Search optimization must not silently change intelligence meaning.

## Time Rules

Preserve distinctions between:

```text
source publication time
source observation time
Pathfinder receipt time
Pathfinder retrieval time
Pathfinder processing time
local sighting time
assessment time
supersession time
expiration time
```

These timestamps are not interchangeable.

Timestamp precision is not timestamp accuracy.

Retrieval time is not publication time.

Publication time is not necessarily observation time.

Later receipt of old intelligence must not make that intelligence appear newly observed.

## Resource Priority

Pathfinder should prioritize preserving incoming source material and authoritative intelligence state over secondary processing.

General priority intent:

```text
1. safely receive source material
2. preserve required source records
3. validate / establish source-processing state
4. commit authoritative Pathfinder records
5. maintain required processing/audit history
6. normalize
7. correlate
8. enrich
9. index
10. report / export / presentation
11. secondary analytics and optional convenience work
```

Exact implementation ordering must be established by architecture and measurement.

Secondary enrichment, visualization, reporting, support, or analytics work must not cause Pathfinder to silently lose authoritative source material.

## Scope Discipline

Work only within the current roadmap phase and currently approved engineering slice.

Do not pull future systems into the current phase merely because their architecture has been discussed.

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

unless they are explicitly part of the current approved roadmap phase.

Build the requirement that exists now.

Preserve future compatibility where necessary without implementing future functionality early.

## Repository Operations

Do not perform repository writes unless explicitly authorized for the specific action.

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
What source supplied this information?
What exactly did the source say?
When did the source say it?
When did Pathfinder receive it?
Was the original source preserved?
Did parsing succeed?
Did normalization succeed?
What information was derived?
What relationships were inferred?
What relationships were directly reported?
What corroborated the assessment?
Were those sources actually independent?
What confidence was assigned?
Who or what assigned it?
Is conflicting intelligence present?
Has the intelligence expired, been revoked, or been superseded?
What did Pathfinder fail to establish?
Is search/index coverage complete?
Was a downstream recommendation produced?
Was any downstream action separately authorized?
```

A useful Pathfinder result should allow an operator to work backward from a conclusion to the source records and processing decisions that produced it.

## Review Expectations

Before proposing a change as complete, verify that applicable:

```text
source-preservation
provenance
truth-separation
normalization
correlation
confidence
assessment
lifecycle
security
trust
authorization
retention
search/index
time
integration
enforcement-boundary
failure-state
scope
repository-write
```

requirements remain intact.

Also verify that:

- observations have not silently become conclusions;
- correlation has not silently become identity;
- duplicated intelligence has not silently become corroboration;
- current knowledge has not rewritten historical knowledge;
- confidence has not silently become authorization;
- incomplete search/index coverage is not presented as complete;
- failures and uncertainty remain visible;
- future roadmap work has not been pulled forward without approval; and
- applicable repository-owned validation and tests have been run.

Clearly report any validation or test that could not be executed.

## Nested AGENTS.md Files

Nested `AGENTS.md` files may refine subtree-specific requirements but must not silently weaken repository-wide:

```text
source preservation
provenance
truthfulness
uncertainty
correlation boundaries
assessment boundaries
security
authorization
retention
history
search completeness
integration authority
enforcement boundaries
scope discipline
repository-write restrictions
```

requirements.
