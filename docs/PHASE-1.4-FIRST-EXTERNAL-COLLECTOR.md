# Phase 1.4 — First External Collector

Status: **CURRENT / SCHEMA CANDIDATE UNDER VALIDATION**

Phase 1.4 implements Pathfinder's first complete external intelligence collection path.

The initial external source is the CISA Known Exploited Vulnerabilities (KEV) catalog.

The purpose of this phase is not to make CISA's data model Pathfinder's data model. The purpose is to prove a complete acquisition, exact-byte preservation, logical-source-record, processing-history, assertion, native-vulnerability, checkpoint, retry, and query path against a real external source.

The governing acquisition invariant remains:

> **Preserve what was received before deciding what it means.**

The governing semantic invariant is:

> **Vulnerability intelligence and threat-indicator intelligence may share acquisition and provenance infrastructure, but they remain distinct semantic object families and are never collapsed for ingestion convenience.**

## Initial Source Boundary

```text
Source
    CISA

SourceCollection
    Known Exploited Vulnerabilities Catalog
    collection_type = VULNERABILITY_CATALOG
```

CISA KEV is vulnerability intelligence.

```text
CVE != threat
Vulnerability != Observable
Vulnerability != Indicator
KEV membership != compromise
known exploited != exploitation observed in our environment
```

## Authoritative Phase 1.4 Path

```text
create RetrievalEvent
  ↓
retrieve
  ↓
preserve exact SourceArtifact bytes
  ↓
complete RetrievalEvent according to acquisition outcome
  ↓
create ProcessingRun
  ↓
parse preserved artifact
  ↓
resolve stable SourceRecords by artifact + locator
  ↓
record per-run SourceRecordProcessing outcomes
  ↓
resolve / create native Vulnerabilities
  ↓
record source-attributable Assertions
  ↓
atomically commit semantic rows
         + COMPLETED ProcessingEvent
         + CollectorCheckpoint
  ↓
query
```

Retrieval and semantic processing are distinct truths.

```text
retrieval succeeded != semantic processing succeeded
```

## SourceArtifact Boundary

The exact acquired HTTP response entity body is the authoritative SourceArtifact.

Phase 1.4 does not reserialize parsed JSON and call it the original source.

The existing Phase 1.3 preservation authority remains unchanged:

```text
Pathfinder runtime
    may stage acquisition bytes
    may request preservation
    may read committed objects
    may not mutate committed objects

pathfinder-artifactd
    owns committed-object mutation authority
    has no Pathfinder database credentials
```

## SourceRecord

A SourceRecord is one stable logical item within one SourceArtifact.

For KEV, the locator is one deterministic entry location in `vulnerabilities[]`.

```text
SourceArtifact
    ↓
SourceRecord
    source_locator = /vulnerabilities/417
```

SourceRecord identity does not change merely because a parser version changes.

Initial SourceRecord state includes:

```text
source_record_id
source_artifact_id
source_id
source_collection_id
external_record_id
source_locator
source_marking
created_at
```

`external_record_id` preserves the source-supplied identifier when safely established. For KEV this is normally the source-supplied CVE identifier.

A SourceRecord does not own raw artifact bytes, preservation hashes, or parser outcome state.

## ProcessingRun and ProcessingEvent

Every semantic attempt against a preserved SourceArtifact receives a new immutable ProcessingRun.

```text
processing_run_id
Source / SourceCollection / RetrievalEvent / SourceArtifact context
process_name
process_version
started_at
```

The ProcessingRun row itself is the durable STARTED state.

A terminal ProcessingEvent is appended later:

```text
COMPLETED
FAILED
INTERRUPTED
```

Exactly one terminal ProcessingEvent is permitted per run.

```text
ProcessingRun with no ProcessingEvent
    = unresolved / non-terminal processing attempt
```

A caught semantic failure appends `FAILED`.

After restart, a sufficiently old unresolved ProcessingRun is reconciled to `INTERRUPTED` before a retry is started.

A retry receives a new ProcessingRun identity.

## SourceRecordProcessing

SourceRecord processing outcome is separate from SourceRecord identity.

Each ProcessingRun may record one outcome for each SourceRecord:

```text
SourceRecordProcessing
    processing_run_id
    source_record_id
    exact artifact/source context
    validation_state
    processing_state
    processing_error_class
```

Initial validation states:

```text
NOT_VALIDATED
VALID
INVALID
```

Initial processing states:

```text
PROCESSED
FAILED
UNSUPPORTED
```

`PROCESSED` requires `VALID` and no processing error.

`FAILED` and `UNSUPPORTED` require an explicit error class.

The database enforces that a SourceRecordProcessing row cannot connect a SourceRecord from one artifact/source context to a ProcessingRun for another.

This preserves reprocessing history without rewriting SourceRecord:

```text
SourceRecord R
    ├─ ProcessingRun parser-v1 -> FAILED
    └─ ProcessingRun parser-v2 -> PROCESSED
```

## Vulnerability

A Pathfinder Vulnerability is a native intelligence object.

Phase 1.4 initially supports CVE identity only.

```text
identifier_namespace = CVE
identifier_value     = CVE-YYYY-NNNN
```

One Pathfinder Vulnerability represents one CVE identity.

Multiple source-derived Assertions may refer to the same Vulnerability.

Vulnerability is not an Observable or Indicator.

## Assertion

An Assertion records what the source claimed.

The initial KEV assertion is:

```text
subject             Vulnerability
assertion_type      KNOWN_EXPLOITED
asserted_value      TRUE
extraction_method   SOURCE_STRUCTURED
```

The Assertion references the SourceRecordProcessing record that produced it, preserving the exact processing lineage back to SourceRecord, SourceArtifact, RetrievalEvent, SourceCollection, and Source.

The assertion means:

> CISA included this CVE in the Known Exploited Vulnerabilities catalog.

It does not mean Pathfinder independently proved exploitation, that the CVE exists in the local environment, or that compromise occurred.

## CISA-Specific Source Fields

KEV includes source-specific fields such as:

```text
vendor / project
product
vulnerability name
date added
short description
required action
due date
known ransomware campaign use
notes
CWE references
```

Phase 1.4 does not automatically promote those fields into mutable authoritative properties on Vulnerability.

They remain source-attributable until Pathfinder defines an explicit native semantic contract for them.

## CollectorCheckpoint

Checkpoint history is append-only.

Phase 1.4 does not use one mutable "last cursor" row.

A successful full-snapshot processing run creates one CollectorCheckpoint only after that ProcessingRun has a `COMPLETED` ProcessingEvent.

For CISA KEV:

```text
checkpoint_kind = FULL_SNAPSHOT
source_checkpoint_value = catalogVersion when safely established
                          otherwise NOT_REPORTED
```

The successful semantic transaction contains:

```text
SourceRecords as needed
SourceRecordProcessing rows
Vulnerabilities as needed
Assertions
COMPLETED ProcessingEvent
CollectorCheckpoint
```

A rollback exposes none of those transaction-owned rows.

The checkpoint repeats the exact artifact/process contract and the database verifies that it matches the referenced ProcessingRun.

The database also permits at most one successful checkpoint for:

```text
SourceArtifact
+ process_name
+ process_version
```

This is the retry/idempotency guard for an already-completed processing contract.

If a client loses the response after commit, a retry can prove the existing completed checkpoint instead of manufacturing another successful interpretation of the same artifact under the same process contract.

## Per-Record Failure Preservation

A malformed or unsupported logical entry is not silently skipped.

If the entry can be located, it receives a stable SourceRecord and a SourceRecordProcessing row such as:

```text
validation_state        = INVALID
processing_state        = FAILED
processing_error_class  = INVALID_CVE_IDENTIFIER
```

or:

```text
validation_state        = NOT_VALIDATED
processing_state        = UNSUPPORTED
processing_error_class  = <explicit class>
```

The exact SourceArtifact remains authoritative regardless of processing outcome.

## Artifact-Level Failure Preservation

If the preserved artifact cannot be parsed far enough to identify SourceRecords, the ProcessingRun still exists.

A terminal `FAILED` ProcessingEvent records the artifact-level processing failure.

A later parser version or retry does not erase the failed attempt.

## Security Boundary

The network-facing collector does not receive committed-object mutation authority.

```text
collector / Pathfinder runtime
    |
    | outbound HTTPS
    v
CISA

collector / Pathfinder runtime
    |
    | local preservation protocol
    v
pathfinder-artifactd

collector / Pathfinder runtime
    |
    | pathfinder_app
    v
PostgreSQL
```

New Phase 1.4 tables grant runtime only the minimum privileges required by their append-oriented contracts.

## Query Proof

Phase 1.4 is not complete merely because rows exist.

The first useful query must be able to begin with a CVE and walk back through the complete acquisition and processing lineage.

Conceptually:

```text
pathfinder vulnerability show CVE-YYYY-NNNN
```

must expose at least:

```text
Pathfinder Vulnerability identity
CVE identifier
CISA KEV KNOWN_EXPLOITED Assertion
SourceRecord identity and locator
SourceRecordProcessing result
ProcessingRun name/version
SourceArtifact identity and SHA-256
RetrievalEvent identity
SourceCollection
Source
CollectorCheckpoint
```

## Explicitly Out of Scope

Phase 1.4 does not implement:

```text
general threat-feed normalization
full Observable normalization
Indicator lifecycle
manual analyst entry
relationship engine
sightings
confidence scoring
source reliability scoring
automatic enforcement
UI
general TAXII support
multiple external collectors
```

## Phase 1.4 Exit Gate

Phase 1.4 remains incomplete until the reference FreeBSD appliance proves:

```text
real external HTTPS retrieval                              PASS
RetrievalEvent acquisition lifecycle                       PASS
exact SourceArtifact preserved before interpretation       PASS
stable SourceRecord identity                               PASS
ProcessingRun / ProcessingEvent lifecycle                  PASS
per-run SourceRecordProcessing lineage                     PASS
cross-artifact processing linkage rejected                 PASS
CVE identity resolution / creation                         PASS
source-derived KNOWN_EXPLOITED Assertion                   PASS
full provenance back to exact SourceArtifact               PASS
per-record failure preservation                            PASS
artifact-level processing failure preservation             PASS
retry / idempotency behavior                               PASS
completed artifact/process contract not duplicated         PASS
checkpoint requires COMPLETED ProcessingEvent              PASS
checkpoint contract matches ProcessingRun                  PASS
failure / interruption recovery                            PASS
reboot / restart recovery                                  PASS
useful CVE query with provenance                           PASS
repository-owned verifier                                  PASS
```

The phase remains incomplete until those behaviors are reproducible and repository-owned.
