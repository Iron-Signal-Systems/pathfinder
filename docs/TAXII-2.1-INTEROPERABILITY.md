# Pathfinder Phase 0.13 — TAXII 2.1 Interoperability Boundary

## Purpose

Pathfinder supports TAXII 2.1 as an external transport/exchange boundary for threat intelligence.

> **Successful transport proves transport succeeded. It does not prove the intelligence was accepted, trusted, complete, or understood.**

TAXII determines how intelligence moves. STIX determines how much of that intelligence is represented. Pathfinder remains authoritative for its own internal semantics.

```text
TAXII server
   -> transport/authentication
   -> TAXII response
   -> SourceArtifact preservation
   -> STIX processing
   -> Pathfinder-native intelligence
```

## Source and Collection Model

A configured TAXII relationship maps into Pathfinder provenance as:

```text
Source
  -> SourceCollection
  -> RetrievalEvent
  -> SourceArtifact
  -> SourceRecord
```

A TAXII Collection is an external collection identity and does not replace Pathfinder's own SourceCollection identity.

Discovery establishes what a server advertises, not what Pathfinder is authorized to ingest. Accessible does not mean trusted, and advertised write capability does not create Pathfinder publication authority.

## Read and Write Separation

TAXII READ and TAXII WRITE are separate capabilities. Initial implementation should prefer read-only TAXII collection unless a later requirement explicitly authorizes publication.

```text
can_write advertised != Pathfinder publish authority
```

## Authentication

Authentication establishes transport/access identity, not intelligence truth. Credentials, tokens, authorization headers, private keys, and equivalent secrets must not enter SourceArtifacts, ordinary provenance, logs, traces, or support output.

## Retrieval Events

Each materially significant TAXII poll/request is associated with a RetrievalEvent recording source, API Root, Collection, request timing, filters, pagination/continuation state, transport result, TAXII result, object count, artifact preservation, and processing outcome.

## Layered Status

Pathfinder must keep separate:

```text
TRANSPORT STATUS
TAXII STATUS
SOURCE PRESERVATION STATUS
STIX VALIDATION STATUS
PATHFINDER MAPPING STATUS
COMMIT STATUS
```

A 200 response may still end with partial or unsupported native processing.

```text
HTTP 200 != full ingestion success
```

## Pagination and Completeness

A successful page is not a completed retrieval. Pagination state, continuation values, pages received, advertised `more`, and final completion state remain explicit.

Initial retrieval states include:

```text
COMPLETE
PARTIAL
INTERRUPTED
FAILED
NOT_KNOWN
```

`more=true` means retrieval is not yet complete. If one page succeeds and a later page fails, preserved successful pages remain available and the overall retrieval becomes PARTIAL rather than being discarded or misrepresented as complete.

## Checkpointing

Checkpoint state is operational collector state, not intelligence. A checkpoint may advance only after the required retrieval/preservation/commit boundary succeeds.

```text
retrieve
  -> preserve
  -> commit
  -> checkpoint
```

Never:

```text
retrieve
  -> checkpoint
  -> hope persistence succeeds
```

This prevents silent data loss after crashes or interrupted processing.

## Retries and Idempotency

Retries remain distinguishable from new retrievals. Duplicate delivery does not automatically create a new Assertion or independent corroboration. Where stable external identifiers exist, ingestion should be idempotent while still preserving delivery history.

## Time Semantics

TAXII Collection-added time is transport/collection metadata, not source publication time, source observation time, Pathfinder receipt time, or local sighting time.

```text
TAXII added time != intelligence observation time
```

Incremental synchronization through `added_after` does not redefine the semantic age of the intelligence.

## Filters

Filter profiles are versioned configuration. A successful filtered retrieval establishes coverage only for the requested subset and never implies full Collection coverage.

```text
filtered retrieval complete != Collection fully ingested
```

## Manifests and Object Retrieval

A manifest entry means an object was advertised/listed, not necessarily retrieved, preserved, understood, or committed. Every retrieved object still passes through source preservation, STIX validation, native mapping, validation, and commit.

## Failures

Authentication failures, authorization failures, TLS failures, 404-like responses, server errors, rate limiting, timeouts, media-type errors, malformed responses, and source unavailability remain explicit states. None may be translated into “no intelligence exists.”

```text
timeout != zero results
404 != resource objectively absent
rate limited != source empty
```

## Redirects and Identity Changes

Redirects or server identity changes that alter scheme, host, port, or trust context are security-sensitive. Pathfinder must not blindly follow redirects or leak credentials to unexpected destinations.

## SourceArtifact Boundaries

Each materially independent TAXII response/page has an explicit preservation boundary. Paginated pages remain separate SourceArtifacts and are not concatenated and falsely represented as one exact original response.

## Completeness and Gaps

Collection completeness may depend on configured start point, filters, page completion, checkpoint continuity, source availability, authentication continuity, and retention horizon. A successful current poll does not prove historical completeness.

Potential gaps must be visible and never silently treated as continuous collection.

## Publication Boundary

Future outbound TAXII publication requires separate export authorization, handling/marking evaluation, STIX export mapping, and TAXII publication authority. A successful remote POST does not prove semantic agreement, redistribution, or trust by the remote organization.

## Resource Controls

Collectors must bound response size, page count, object count, total retrieval volume, retries, concurrency, request duration, and redirect behavior. Authenticated servers remain untrusted input sources.

## Operational Reporting

Operators should be able to distinguish, for example:

```text
retrieval: PARTIAL
pages completed: 12
next page: FAILED
failure: timeout
last safe checkpoint: ...
artifacts preserved: 12
STIX objects received: 1184
native mappings: 1171 complete / 9 partial / 4 unsupported
```

rather than receiving only “TAXII sync failed.”

## Truth Separations

```text
TAXII transport succeeded    != intelligence accepted
HTTP 200                     != full ingestion success
authenticated server         != truthful intelligence
Collection discovered        != Collection configured
Collection accessible        != Collection trusted
page succeeded               != retrieval complete
more=true                    != retrieval complete
filtered retrieval complete  != Collection fully covered
manifest entry               != object ingested
checkpoint                   != intelligence record
retry delivery               != independent intelligence
rate limited                 != source empty
timeout                      != zero results
remote deletion              != local historical deletion
TAXII POST accepted          != remote organization agrees
current successful poll      != historical completeness
```

## Phase 0.13 Exit Decision

Phase 0.13 is satisfied when Pathfinder treats TAXII 2.1 as a transport boundary; separates transport, preservation, STIX validation, native mapping, and commit state; explicitly models pagination and completeness; advances checkpoints only after safe persistence; preserves partial retrievals and retry history; keeps transport timestamps distinct from intelligence time; fails safely on authentication, network, rate-limit, and protocol errors; and never translates TAXII success into intelligence trust, confidence, corroboration, or enforcement authority.
