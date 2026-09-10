# Pathfinder Phase 0.13 — TAXII 2.1 Interoperability Boundary

## Purpose

Pathfinder supports TAXII 2.1 as an external transport/exchange boundary for threat intelligence.

> **Successful transport proves transport succeeded. It does not prove the intelligence was accepted, trusted, complete, or understood.**

TAXII determines how intelligence moves. STIX determines how much of that intelligence is represented. Pathfinder remains authoritative for its own internal semantics.

```text
TAXII server
   ↓
transport/authentication
   ↓
TAXII response
   ↓
SourceArtifact preservation
   ↓
STIX processing
   ↓
Pathfinder-native intelligence
```

## Source and Collection Model

A configured TAXII relationship maps into Pathfinder provenance as:

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
```

A TAXII Collection is an external collection identity and does not replace Pathfinder's SourceCollection identity.

Discovery establishes what a server advertises. It does not establish trust, authorization, or Pathfinder publication authority.

## Read and Write Separation

TAXII READ and TAXII WRITE are separate capabilities.

Initial implementation should prefer read-only collection unless a later explicit requirement authorizes publication.

```text
remote can_write advertised != Pathfinder publish authority
```

## Authentication

Authentication establishes transport/access identity, not intelligence truth.

Credentials, authorization headers, bearer tokens, private keys, and equivalent secrets must not enter SourceArtifacts, ordinary provenance, logs, traces, or support output.

## RetrievalEvent

Each materially significant TAXII request/poll produces or associates with a `RetrievalEvent` recording the configured Source/SourceCollection, API Root, Collection, request timing, filters, pagination state, transport result, artifact preservation, object counts, processing result, and checkpoint behavior.

## Typed Retrieval State

TAXII does not use one overloaded retrieval `status` field.

The high-level attempt uses `OperationResult`:

```text
SUCCESS
PARTIAL
FAILED
PENDING
NOT_PERFORMED
NOT_APPLICABLE
```

Detailed interruption/failure cause remains separate, for example:

```text
TIMEOUT
CONNECTION_FAILED
AUTHENTICATION_FAILED
AUTHORIZATION_FAILED
RATE_LIMITED
PROTOCOL_ERROR
MEDIA_TYPE_MISMATCH
PAGINATION_INTERRUPTED
```

Collection/time-range completeness uses `CoverageState`:

```text
COMPLETE
PARTIAL
INCOMPLETE
NOT_KNOWN
```

Transport/service capability uses `HealthState` where applicable:

```text
HEALTHY
DEGRADED
UNAVAILABLE
NOT_READY
FAILED
```

These are different dimensions.

```text
OperationResult != CoverageState
CoverageState != HealthState
```

## Layered Status

Pathfinder keeps separate:

```text
transport result
TAXII protocol result
SourceArtifact preservation state
SourceArtifact integrity state
STIX validation/mapping state
Pathfinder commit state
coverage state
checkpoint state
```

A successful HTTP/TAXII request may still end with partial or unsupported native processing.

```text
HTTP 200 != full ingestion success
```

## Pagination

One successful page is not a completed multi-page retrieval.

Pathfinder preserves continuation state, pages received, `more` semantics, and per-page SourceArtifacts.

If an earlier page succeeds and a later page fails:

```text
operation_result = PARTIAL
coverage_state   = PARTIAL or INCOMPLETE
failure_code     = PAGINATION_INTERRUPTED / applicable cause
```

The successful page artifacts remain historical.

`more=true` means retrieval is not complete.

## Checkpointing

Checkpoint state is operational collector state, not intelligence.

The safe order is:

```text
retrieve
  ↓
preserve required SourceArtifact(s)
  ↓
process/commit required state
  ↓
advance checkpoint
```

Never:

```text
retrieve
  ↓
advance checkpoint
  ↓
hope persistence succeeds
```

This prevents silent data loss after crash or interrupted processing.

## Retries and Idempotency

Retries remain distinguishable from new source events.

Duplicate delivery does not automatically create a new Assertion or independent corroboration.

Where stable external identity exists, ingestion should be idempotent while delivery/retry history remains preserved.

## Time Semantics

TAXII Collection-added time is collection/transport metadata.

It is not automatically:

```text
source publication time
source observation time
Pathfinder receipt time
local Sighting time
```

Incremental synchronization through `added_after` does not redefine the semantic age of the intelligence.

## Filters and Coverage

Filter configuration is versioned.

A successful filtered retrieval establishes completeness only for the requested subset and applicable time/scope when Pathfinder can establish that coverage.

```text
filtered retrieval SUCCESS != Collection fully covered
```

Coverage claims must include the filter/start/checkpoint context that supports them.

## Manifests and Object Retrieval

A manifest entry means an object was advertised/listed.

It does not mean the object was retrieved, preserved, understood, mapped, or committed.

Every retrieved object still passes the source-preservation, STIX, Pathfinder semantic, and commit boundaries.

## SourceArtifact Boundaries

Each materially independent TAXII response/page has an explicit SourceArtifact preservation boundary.

Paginated responses are not concatenated and then falsely represented as one exact original payload.

```text
page 1 -> SourceArtifact A
page 2 -> SourceArtifact B
page 3 -> SourceArtifact C
```

## Completeness and Gaps

TAXII coverage may depend on:

```text
configured start point
filter set
pagination completion
checkpoint continuity
source retention horizon
server availability
authentication continuity
Pathfinder processing completeness
```

A successful current poll does not prove historical completeness.

Known gaps remain visible until Pathfinder can establish recovery.

## Failures

Authentication failure, authorization failure, TLS failure, server errors, rate limiting, timeout, media-type mismatch, malformed response, and source unavailability remain explicit.

None may be translated into “no intelligence exists.”

```text
timeout != zero results
rate limited != source empty
404-like response != objective absence in all contexts
```

Access-controlled hiding may make some remote errors ambiguous; Pathfinder records what the protocol interaction established without manufacturing certainty.

## Redirects and Identity Changes

Redirects or identity changes affecting scheme, host, port, trust context, or credential destination are security-sensitive.

Pathfinder must not blindly follow unexpected redirects or leak credentials.

## Publication Boundary

Future outbound TAXII publication requires separate:

```text
export authorization
handling/marking evaluation
STIX export mapping
TAXII write authority
```

A successful remote POST means transport/protocol acceptance under the remote server contract. It does not prove semantic agreement, independent corroboration, or trust by the remote organization.

## Resource Controls

Collectors must bound response size, page count, object count, total retrieval volume, retries, concurrency, request duration, redirect behavior, and other resource-sensitive processing.

Authenticated servers remain untrusted input sources.

## Operational Reporting

A useful operator view should distinguish facts such as:

```text
operation_result = PARTIAL
pages_completed = 12
failure_code = TIMEOUT
coverage_state = INCOMPLETE
last_safe_checkpoint = ...
artifacts_preserved = 12
STIX objects received = 1184
native mappings = 1171 success / 9 partial / 4 unsupported
```

rather than only `TAXII sync failed`.

## Historical Preservation

Later retry, recovered coverage, source correction, mapping upgrade, or server reconfiguration does not erase earlier RetrievalEvents, SourceArtifacts, failures, checkpoints, or processing history.

> **The original record is maintained no matter what.**

## Common Truth Separations

```text
TAXII transport success != intelligence accepted
HTTP 200 != full ingestion success
OperationResult != CoverageState
CoverageState != HealthState
authenticated server != truthful intelligence
Collection discovered != Collection configured
Collection accessible != Collection trusted
page success != retrieval complete
more=true != retrieval complete
filtered retrieval success != Collection fully covered
manifest entry != object ingested
checkpoint != intelligence record
retry delivery != independent intelligence
rate limited != source empty
timeout != zero results
remote deletion != local historical deletion
TAXII POST accepted != remote agreement
current successful poll != historical completeness
```

## Phase 0.13 Exit Decision

Phase 0.13 is satisfied when Pathfinder:

1. Treats TAXII 2.1 as a transport boundary rather than an internal truth model.
2. Separates `OperationResult`, `CoverageState`, `HealthState`, preservation, STIX mapping, commit, and checkpoint state.
3. Uses `SUCCESS/PARTIAL/FAILED/...` for retrieval-operation result rather than mixing `COMPLETE/DEGRADED/UNAVAILABLE` into one state.
4. Preserves every completed page as its own SourceArtifact.
5. Advances checkpoints only after the required durable boundary.
6. Preserves partial retrievals and retry history.
7. Keeps TAXII-added time separate from intelligence observation/publication time.
8. Makes filters and source retention part of coverage semantics.
9. Fails explicitly on authentication, protocol, timeout, rate-limit, and pagination problems.
10. Never translates transport success into intelligence trust or enforcement authority.
11. **Maintains the original record no matter what.**
