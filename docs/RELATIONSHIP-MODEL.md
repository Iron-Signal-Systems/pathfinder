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

The governing principle is:

> **A relationship records a connection Pathfinder can describe. It does not automatically establish identity, causation, ownership, or attribution.**

## Relationship Identity

Every Relationship receives a Pathfinder-controlled identity.

```text
relationship_id = UUIDv7
```

Relationship identity must not be derived solely from:

```text
source object
relationship type
target object
```

because the same objects may have materially different relationships across different times, sources, or contexts.

## Conceptual Fields

A Relationship should preserve at least:

```text
relationship_id
relationship_type
source_object_id
target_object_id
directionality
origin
valid_from_state
valid_from / not_known
valid_until_state
valid_until / not_known
created_at
created_by
derivation_method / not_applicable
derivation_version / not_applicable
```

Supporting Assertions, conflicting Assertions, Assessments, and processing lineage are linked separately rather than collapsed into one field.

## Relationship Does Not Own One Confidence Value

A Relationship does not contain one mutable universal confidence field representing all opinions about the relationship.

Confidence belongs to the Assertion or Assessment expressing that confidence.

For example:

```text
Relationship:
    Actor Alpha
        uses
    ExampleRAT

Assertion A:
    Vendor A reports relationship
    source confidence = HIGH

Assertion B:
    Vendor B reports relationship
    source confidence = MODERATE

Assessment C:
    Pathfinder assesses relationship strongly supported
    confidence = HIGH
```

These remain separate.

## Relationship Origin

Pathfinder distinguishes how a Relationship entered the intelligence model.

Initial origin classes:

```text
SOURCE_NORMALIZED
PATHFINDER_DERIVED
ANALYST_RECORDED
```

### SOURCE_NORMALIZED

One or more attributable source Assertions directly describe the relationship, even if Pathfinder normalized the source wording.

### PATHFINDER_DERIVED

Pathfinder created the Relationship from other intelligence through an explicitly defined derivation rule.

The Relationship must preserve derivation method, version, basis records, and derivation time.

### ANALYST_RECORDED

An authorized analyst intentionally recorded the Relationship.

The analyst principal and basis remain attributable.

## Relationship Support

Relationships may have many supporting or conflicting Assertions.

Pathfinder should preserve explicit support links such as:

```text
SUPPORTS
CONTRADICTS
QUALIFIES
```

These links are provenance/support structures. They are not themselves general threat-intelligence Relationships.

## Intelligence Relationships Versus Provenance Links

Pathfinder must not turn every database connection into a threat-intelligence Relationship.

The following concepts belong to provenance, lifecycle, or processing structures:

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

They must not be mixed into the general intelligence relationship vocabulary merely because a graph representation could express them as edges.

## Directed Relationships

Most Pathfinder relationships are directional.

For example:

```text
ThreatActor
    uses
Malware
```

Directional relationships preserve:

```text
source_object_id
relationship_type
target_object_id
```

The order is semantically meaningful.

## Inverse Presentation

Pathfinder should not persist artificial inverse Relationship objects solely for query convenience.

For example, Pathfinder may store:

```text
ThreatActor
    uses
Malware
```

A UI or API may present the inverse view as:

```text
Malware
    used_by
ThreatActor
```

without creating a second independent Relationship.

## Symmetric Relationships

Some relationships are inherently symmetric.

Initial examples may include:

```text
possibly_related
shares_infrastructure_with
```

For a symmetric Relationship, endpoint order does not change meaning.

The implementation should use deterministic endpoint ordering for symmetric relationship identity/deduplication.

## No Implied Symmetry

Relationships are not symmetric unless their type explicitly defines symmetry.

```text
A uses B
    !=
B uses A
```

Likewise:

```text
Domain resolves_to IP
```

does not create a second stored inverse Relationship.

## No Implied Transitivity

No Pathfinder Relationship is transitive unless a future contract explicitly defines it as transitive.

```text
A associated_with B
B associated_with C
    !=
A associated_with C
```

Derived relationships require explicit derivation rules and provenance.

## Initial Relationship Vocabulary

Pathfinder v1 should begin with a deliberately small relationship vocabulary.

### `uses`

Meaning: the source object is reported or assessed as using the target object.

Initial valid directions include:

```text
ThreatActor -> Malware
ThreatActor -> Tool
ThreatActor -> Infrastructure
Campaign -> Malware
Campaign -> Tool
Campaign -> Infrastructure
```

`uses` does not imply ownership.

### `communicates_with`

Meaning: the source object is reported, observed, or assessed as communicating with the target object.

Initial candidate directions include:

```text
Malware -> Infrastructure
Malware -> network Observable
```

`communicates_with != controlled_by`.

### `resolves_to`

Meaning: a Domain Observable is reported or observed as resolving to an IPv4 or IPv6 Observable.

Valid direction:

```text
Domain -> IPv4
Domain -> IPv6
```

This relationship is inherently time-sensitive.

Current DNS resolution must never rewrite historical resolution.

### `implements`

Meaning: Malware or a Tool is reported or assessed as implementing behavior represented by a Technique.

Initial directions:

```text
Malware -> Technique
Tool -> Technique
```

`implements Technique != Technique observed locally`.

### `exploits`

Meaning: the source object is reported or assessed as exploiting a Vulnerability.

Initial directions:

```text
ThreatActor -> Vulnerability
Campaign -> Vulnerability
Malware -> Vulnerability
Tool -> Vulnerability
```

`Actor exploits vulnerability != local asset exploited`.

### `associated_with`

Meaning: a source establishes some association but does not justify a more precise relationship.

This type must be used conservatively and must not become a generic escape hatch for poorly modeled data.

### `possibly_related`

Meaning: Pathfinder or a source has a basis to consider two intelligence objects potentially related but cannot establish a stronger relationship.

This relationship is symmetric and explicitly uncertain.

### `shares_infrastructure_with`

Meaning: two applicable intelligence objects are reported or derived as sharing infrastructure.

This Relationship is symmetric.

It does not establish same operator, same campaign, same actor, coordination, or ownership.

## Deferred Relationship Types

The following should remain deferred until their semantics and valid endpoints are explicitly defined:

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

In particular:

> **Pathfinder v1 should not have a generic `same_as` Relationship.**

Identity resolution deserves its own future contract.

## Relationship Type Registry

Relationship types must come from a versioned Pathfinder-controlled registry.

Each Relationship type definition should establish:

```text
name
meaning
directionality
allowed source object types
allowed target object types
time semantics
whether symmetry applies
whether inverse presentation exists
whether automated derivation is allowed
whether analyst creation is allowed
```

Arbitrary caller-defined Relationship strings must not become authoritative Pathfinder semantics.

## Endpoint Validation

Relationship creation must validate that its endpoints are permitted for the Relationship type.

For example:

```text
Domain
    resolves_to
IPv4
```

may be valid.

But:

```text
ThreatActor
    resolves_to
Malware
```

must fail structural validation.

## Time Semantics

Threat relationships may change over time.

Pathfinder must preserve meaningful temporal bounds without inventing them.

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

Pathfinder must not use `received_at` as `valid_from` merely because no better date exists.

`receipt time != relationship start time`.

## Point-in-Time Relationships

Some Relationships may be based on a specific observation rather than a broad interval.

Pathfinder must not inflate one point observation into an indefinite relationship.

## Relationship Deduplication

Pathfinder must not deduplicate Relationships solely because source object, relationship type, and target object match.

Time context and semantics may make apparently identical relationships materially distinct.

At the same time, repeated sources describing the same semantic Relationship should normally attach additional Assertions/support rather than blindly create one Relationship per feed record.

The governing objective is:

```text
deduplicate semantic relationship
    while
preserving every attributable source claim
```

## Derived Relationships

Pathfinder-derived Relationships require explicit derivation rules.

A derivation must preserve:

```text
method
version
basis
time
result
```

Pathfinder must not silently create relationships merely because a graph path exists.

`graph path exists != intelligence relationship established`.

## Derivation Depth

Pathfinder must not perform unbounded relationship inference.

Every automated derivation method defines allowed input relationship types, maximum derivation depth, required basis, output relationship type, and failure behavior.

## Relationship Candidates

Where Pathfinder derives a possible connection that does not meet the standard for a normal Relationship, it should prefer an explicitly uncertain relationship such as `possibly_related` or retain the result as a candidate processing artifact until a later workflow contract defines promotion.

## Relationship Assessments

Relationships may be assessed just like other Pathfinder objects.

A conflicting Assessment may also exist.

The Relationship itself does not need to be deleted because an Assessment later disputes it.

## Relationship Conflict

A Relationship may simultaneously have supporting Assertions, contradicting Assertions, qualifying Assertions, Pathfinder Assessments, and analyst Assessments.

Pathfinder preserves all of them.

## Source Wording Versus Pathfinder Vocabulary

The original source may use wording different from Pathfinder's relationship vocabulary.

Pathfinder preserves both the original source assertion and normalized Relationship.

The normalized vocabulary must never replace the original source representation.

## Relationship and Attribution

Actor attribution is particularly sensitive.

```text
Actor A uses Malware X
Campaign B uses Malware X
Campaign B associated_with Actor A
    !=
Campaign B attributed_to Actor A
```

`attributed_to` remains deferred until Pathfinder defines explicit attribution semantics.

Pathfinder must not manufacture attribution by combining weaker relationships.

## Relationship and Infrastructure

Shared infrastructure does not automatically prove actor identity, collaboration, coordination, or ownership.

The infrastructure may represent shared hosting, public cloud, VPN exit, CDN, compromised infrastructure, commodity services, or reassigned addresses.

Pathfinder preserves what it can establish and stops there.

## Relationship and ISS Observations

ISS-product observations may contribute relationship context.

Stronghold observations or FI observations may create Sightings under the Sighting Model and may later support relationships or assessments.

They do not automatically create compromise or attribution conclusions.

## No Relationship-by-Proximity

Pathfinder must never infer relationships merely because objects appear near one another in the same report, API response, paragraph, incident bundle, graph neighborhood, or timestamp window unless an explicit parser or derivation contract defines why that proximity establishes a relationship.

`co-occurrence != relationship`.

## Common Truth Separations

```text
Relationship != identity
Relationship != attribution
Relationship != ownership
Relationship != causation
Relationship != compromise
Relationship != Assessment
Relationship != Assertion
supporting Assertion != proven Relationship
multiple supporting Assertions != independent corroboration
inverse query view != second Relationship
shared infrastructure != same actor
shared malware != same actor
same campaign != same organization
co-occurrence != relationship
graph path != established relationship
association != attribution
possibly_related != confirmed relationship
uses != owns
communicates_with != controlled_by
implements Technique != Technique observed locally
exploits Vulnerability != local exploitation observed
current relationship != historical relationship
receipt time != relationship start time
point observation != indefinite relationship
relationship endpoint match != semantic duplicate
new relationship interpretation != historical source rewritten
derived relationship != directly reported relationship
analyst-recorded relationship != source-reported relationship
```

## Object-Model Refinements

Phase 0.7 refines earlier drafts in three ways.

First, Relationship confidence is represented through attributable Assertions and Assessments rather than one mutable confidence field on the Relationship.

Second, provenance/lifecycle edges such as `derived_from`, `supported_by`, `supersedes`, and `withdraws` remain separate from the threat-intelligence Relationship vocabulary.

Third, generic identity relationships such as `same_as` are deferred until an explicit identity-resolution contract exists.

## Phase 0.7 Exit Decision

Phase 0.7 is satisfied when Pathfinder accepts:

1. Relationship as a first-class UUIDv7 Pathfinder object.
2. Relationship endpoints and type are structurally validated.
3. Relationship types come from a versioned Pathfinder-controlled registry.
4. Directed and symmetric Relationships have explicit different semantics.
5. Inverse query presentation does not create duplicate authoritative Relationships.
6. No Relationship is assumed transitive by default.
7. `SOURCE_NORMALIZED`, `PATHFINDER_DERIVED`, and `ANALYST_RECORDED` remain distinguishable origins.
8. Supporting, contradicting, and qualifying Assertions remain attributable.
9. Provenance/lifecycle links remain separate from threat-intelligence Relationships.
10. Relationship confidence belongs to Assertions/Assessments rather than one mutable universal field.
11. Temporal bounds remain explicit and unknown time is never manufactured.
12. Receipt time is never silently substituted for relationship start time.
13. Point observations are not silently expanded into indefinite relationships.
14. Semantic deduplication preserves every underlying source Assertion.
15. Derived Relationships preserve method, version, basis, and time.
16. Graph proximity or graph reachability does not itself establish a Relationship.
17. Automated derivation is bounded and explicitly defined.
18. `possibly_related` remains explicitly uncertain.
19. Shared infrastructure does not establish common ownership, actor identity, or coordination.
20. Attribution is not manufactured from weaker relationships.
21. `same_as` and other identity-merging semantics remain deferred.
22. ISS-product observations may inform Pathfinder relationships but do not silently create compromise or attribution conclusions.
