# Pathfinder Phase 0.7 — Relationship Model

## Purpose

Pathfinder requires a strict relationship model for connecting intelligence objects without turning correlation into certainty.

Relationships are first-class Pathfinder objects.

They preserve:

```text
identity
relationship type
endpoints
directionality
time context
origin
supporting Assertions
conflicting Assertions
derivation lineage
processing history
```

A Relationship does not inherently mean Pathfinder has proven the connection.

> **A relationship records a connection Pathfinder can describe. It does not automatically establish identity, causation, ownership, or attribution.**

## Relationship Identity

Every Relationship receives a Pathfinder-controlled identity.

```text
relationship_id = UUIDv7
```

Relationship identity is not derived solely from source object, relationship type, and target object because materially different relationships may exist across time, source, context, or derivation.

## Conceptual Fields

A Relationship should preserve at least:

```text
relationship_id
relationship_type
source_object_id
target_object_id
directionality
relationship_origin
valid_from_state
valid_from / NOT_KNOWN
valid_until_state
valid_until / NOT_KNOWN
created_at
created_by
derivation_method / NOT_APPLICABLE
derivation_version / NOT_APPLICABLE
```

Supporting Assertions, conflicting Assertions, Assessments, ATT&CK mapping metadata, and processing lineage are linked separately rather than collapsed into one field.

## Relationship Does Not Own One Confidence Value

A Relationship does not contain one mutable universal confidence field representing all opinions about the relationship.

Confidence belongs to the Assertion or Assessment expressing that confidence.

Example:

```text
Relationship:
    Actor Alpha uses ExampleRAT

Assertion A:
    source confidence = HIGH

Assertion B:
    source confidence = MODERATE

Assessment C:
    relationship validity = strongly supported
    confidence = HIGH
```

These remain separate records.

## Relationship Origin

Pathfinder distinguishes how a Relationship entered the intelligence model.

Initial Relationship origins are:

```text
SOURCE_NORMALIZED
PATHFINDER_DERIVED
ANALYST_RECORDED
```

### SOURCE_NORMALIZED

One or more attributable source Assertions directly describe the relationship, even if Pathfinder normalized source wording into Pathfinder vocabulary.

### PATHFINDER_DERIVED

Pathfinder created the Relationship from other intelligence under an explicitly defined derivation rule.

The Relationship preserves derivation method, version, basis records, and derivation time.

### ANALYST_RECORDED

An authorized analyst intentionally recorded the Relationship.

The analyst principal and basis remain attributable.

## ATT&CK Mapping Origin Is Separate

ATT&CK classification/mapping origin is a separate typed dimension and must not be collapsed into `relationship_origin`.

ATT&CK mapping origins are governed by Phase 0.14 and include:

```text
ATTACK_NATIVE
SOURCE_REPORTED
PATHFINDER_ASSOCIATED
LOCALLY_SUGGESTED
LOCALLY_CONFIRMED
ANALYST_RECORDED
```

A Relationship may therefore carry both facts where applicable.

Example:

```text
relationship_origin = SOURCE_NORMALIZED
attack_mapping_origin = ATTACK_NATIVE
```

or:

```text
relationship_origin = PATHFINDER_DERIVED
attack_mapping_origin = PATHFINDER_ASSOCIATED
```

These dimensions answer different questions.

## Relationship Support

Relationships may have many supporting or conflicting Assertions.

Support links such as:

```text
SUPPORTS
CONTRADICTS
QUALIFIES
```

are provenance/support structures. They are not themselves general threat-intelligence Relationships.

## Intelligence Relationships Versus Provenance Links

Pathfinder does not turn every database connection into a threat Relationship.

The following belong to provenance, lifecycle, or processing structures:

```text
derived_from
supported_by
contradicted_by
supersedes
withdraws
produced_by
parsed_by
normalized_by
received_from
```

They remain separate from the threat-intelligence relationship vocabulary.

## Directionality

Most Pathfinder Relationships are directional.

```text
ThreatActor
    uses
Malware
```

means something materially different from the inverse.

Pathfinder should not persist artificial inverse Relationship objects solely for query convenience. A UI/API may render an inverse presentation without creating a second authoritative Relationship.

## Symmetric Relationships

Initial symmetric relationship candidates include:

```text
possibly_related
shares_infrastructure_with
```

Symmetric Relationship implementations should use deterministic endpoint ordering for identity/deduplication.

Relationships are not symmetric unless their registry definition says so.

## No Implied Transitivity

No Pathfinder Relationship is transitive unless a future explicit contract defines it as transitive.

```text
A associated_with B
B associated_with C
    !=
A associated_with C
```

Graph reachability is not sufficient to create a new intelligence Relationship.

## Initial Relationship Vocabulary

Pathfinder begins with a deliberately small vocabulary.

### `uses`

Meaning: the source object is reported or assessed as using the target object.

Initial valid directions are:

```text
ThreatActor -> Malware
ThreatActor -> Tool
ThreatActor -> Infrastructure
ThreatActor -> Technique

Campaign -> Malware
Campaign -> Tool
Campaign -> Infrastructure
Campaign -> Technique
```

The Technique endpoints are explicitly included to reconcile the ATT&CK model.

`uses` does not imply ownership, exclusive control, actor identity, or that every intrusion involving the source object uses the target.

### `communicates_with`

Meaning: the source object is reported, observed, or assessed as communicating with the target object.

Initial candidate directions include:

```text
Malware -> Infrastructure
Malware -> network Observable
```

```text
communicates_with != controlled_by
```

### `resolves_to`

Meaning: a Domain Observable is reported or observed as resolving to an IPv4 or IPv6 Observable.

Valid direction:

```text
Domain -> IPv4
Domain -> IPv6
```

This Relationship is time-sensitive. Current DNS must not rewrite historical DNS state.

### `implements`

Meaning: Malware or Tool is reported or assessed as implementing behavior represented by a Technique.

Initial directions:

```text
Malware -> Technique
Tool -> Technique
```

```text
implements Technique != Technique observed locally
```

### `exploits`

Meaning: the source object is reported or assessed as exploiting a Vulnerability.

Initial directions:

```text
ThreatActor -> Vulnerability
Campaign -> Vulnerability
Malware -> Vulnerability
Tool -> Vulnerability
```

```text
Actor exploits Vulnerability != local asset exploited
```

### `associated_with`

Meaning: a source or Pathfinder Assessment establishes an association but does not justify a more precise relationship.

This type must be used conservatively and must not become a generic escape hatch for poorly modeled semantics.

### `possibly_related`

Meaning: Pathfinder or a source has a basis to consider two objects potentially related but cannot establish a stronger relationship.

This Relationship is symmetric and explicitly uncertain.

### `shares_infrastructure_with`

Meaning: two applicable intelligence objects are reported or derived as sharing infrastructure.

This Relationship is symmetric.

It does not establish same operator, same campaign, same actor, coordination, or ownership.

## Deferred Relationship Types

The following remain deferred until semantics and endpoint rules are explicitly defined:

```text
owns
controls
same_as
targets
hosts
attributed_to
originates_from
compromised_by
```

> **Pathfinder v1 does not have a generic `same_as` Relationship.**

Identity resolution deserves a separate contract.

## Relationship Type Registry

Relationship types come from a versioned Pathfinder-controlled registry.

Each registry entry defines:

```text
name
meaning
directionality
allowed source object types
allowed target object types
time semantics
symmetry
inverse presentation behavior
automated derivation allowed / denied
analyst creation allowed / denied
```

Arbitrary caller-defined relationship strings do not become authoritative Pathfinder semantics.

## Endpoint Validation

Relationship creation validates endpoints against the registry.

Valid example:

```text
Domain resolves_to IPv4
```

Invalid example:

```text
ThreatActor resolves_to Malware
```

ATT&CK mappings do not bypass endpoint validation.

## Time Semantics

Threat Relationships may change over time.

Pathfinder preserves meaningful temporal bounds without inventing them.

Conceptually:

```text
valid_from_state:
    KNOWN
    NOT_KNOWN
    NOT_APPLICABLE

valid_until_state:
    KNOWN
    OPEN
    NOT_KNOWN
    NOT_APPLICABLE
```

Pathfinder must not use `received_at` as `valid_from` merely because a more meaningful time is unavailable.

```text
receipt time != relationship start time
```

## Point-in-Time Observations

A point Sighting may support a Relationship without inflating that point observation into an indefinite time range.

Temporal inference requires an explicit rule and preserved basis.

## Relationship Deduplication

Pathfinder does not deduplicate Relationships solely because source object, type, and target object match.

Time and semantic scope may make apparently identical edges materially distinct.

Conversely, several sources describing the same semantic Relationship should normally attach additional Assertions/support rather than blindly creating one Relationship per feed record.

The objective is:

```text
deduplicate semantic relationship
while
preserving every attributable source claim
```

## Derived Relationships

Pathfinder-derived Relationships require explicit, versioned derivation rules.

A derivation preserves:

```text
method
version
basis
time
result
```

Pathfinder must not create a Relationship merely because a graph path, report co-occurrence, temporal proximity, or shared infrastructure exists.

## Derivation Bounds

Automated derivation is bounded.

Each derivation method defines allowed input relationship types, maximum depth, required basis, output type, and failure behavior.

## Relationship Candidates

Where a possible connection does not meet the standard for a stronger Relationship, Pathfinder should use an explicitly uncertain type such as `possibly_related` or retain the result as a candidate processing record until a later workflow promotes it.

Candidate correlation remains distinct from identity.

## Relationship Assessments

Relationships may be Assessment subjects.

A Relationship may simultaneously have:

```text
supporting Assertions
contradicting Assertions
qualifying Assertions
machine Assessments
analyst Assessments
open/resolved IntelligenceConflict
```

The Relationship itself is not deleted because later analysis disputes it.

## Source Wording Versus Pathfinder Vocabulary

The original source may use wording different from Pathfinder's normalized relationship vocabulary.

Pathfinder preserves the source Assertion and normalized Relationship separately.

Normalization never replaces the original source representation.

## Relationship and Attribution

Actor attribution is especially sensitive.

```text
Actor A uses Malware X
Campaign B uses Malware X
Campaign B associated_with Actor A
    !=
Campaign B attributed_to Actor A
```

`attributed_to` remains deferred until explicit attribution semantics exist.

Pathfinder does not manufacture attribution by composing weaker Relationships.

## Relationship and Infrastructure

Shared infrastructure may reflect shared hosting, cloud/CDN use, VPN exit nodes, compromised infrastructure, commodity services, or reassigned addresses.

```text
shared infrastructure != common operator
```

## ISS Observations

Stronghold, FI, Atlas, and other approved ISS products retain their own domain authority.

Their observations may create Sightings and later support Relationships or Assessments.

They do not silently create compromise, attribution, or identity conclusions.

## ATT&CK Relationship Rules

ATT&CK is optional classification/reference metadata.

Relationships such as:

```text
ThreatActor -> uses -> Technique
Campaign -> uses -> Technique
Malware -> implements -> Technique
Tool -> implements -> Technique
```

remain attributable intelligence claims/mappings.

They do not establish:

```text
Technique occurred locally
actor identity proven
malware identity proven
compromise proven
response authorized
```

`ATT&CK NOT_MAPPED` is a valid state and causes no downgrade to threat significance, confidence, operational relevance, or escalation priority.

## No Relationship-by-Proximity

Pathfinder does not infer Relationships merely because objects occur near one another in the same report, response, paragraph, incident bundle, graph neighborhood, or timestamp window unless an explicit parser/derivation contract defines why that context establishes the Relationship.

```text
co-occurrence != relationship
```

## Common Truth Separations

```text
Relationship != identity
Relationship != attribution
Relationship != ownership
Relationship != causation
Relationship != compromise
Relationship != Assessment
Relationship != Assertion
relationship_origin != attack_mapping_origin
supporting Assertion != proven Relationship
multiple supporting Assertions != independent corroboration
inverse presentation != second authoritative Relationship
shared infrastructure != same actor
co-occurrence != Relationship
graph path != established Relationship
possibly_related != confirmed identity
uses != owns
communicates_with != controlled_by
implements Technique != Technique observed locally
ThreatActor uses Technique != actor observed locally
Campaign uses Technique != attack sequence required
exploits Vulnerability != local exploitation observed
current Relationship != historical Relationship
receipt time != Relationship start time
point observation != indefinite Relationship
new interpretation != historical source rewritten
```

## Phase 0.7 Exit Decision

Phase 0.7 is satisfied when Pathfinder accepts that:

1. Relationship is a first-class UUIDv7 record.
2. Relationship endpoints/types are structurally validated by a versioned registry.
3. `SOURCE_NORMALIZED`, `PATHFINDER_DERIVED`, and `ANALYST_RECORDED` are Relationship origins.
4. ATT&CK mapping origin remains a separate typed dimension.
5. `uses` explicitly supports ThreatActor/Campaign -> Technique as well as the other frozen endpoints.
6. Relationship confidence belongs to Assertions/Assessments rather than one mutable field.
7. Directed/symmetric behavior is explicit; inverse presentation does not create duplicate records.
8. No Relationship is transitive by default.
9. Provenance/lifecycle links remain outside the threat-intelligence relationship vocabulary.
10. Time bounds are explicit and are never manufactured from receipt time.
11. Semantic deduplication preserves all underlying source Assertions.
12. Derived Relationships preserve method, version, basis, time, and result.
13. Graph proximity, co-occurrence, or shared infrastructure do not create unqualified Relationships.
14. `possibly_related` remains explicitly uncertain.
15. `same_as` and other identity-merging semantics remain deferred.
16. ATT&CK mappings never become compromise, identity, or enforcement authority.
17. **The original record is maintained no matter what.**
