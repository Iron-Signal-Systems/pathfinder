# Pathfinder Phase 0.5 — Source and Provenance Model

## Purpose

Pathfinder must preserve enough provenance to answer:

> **Where did this intelligence come from, what exactly was received, how was it processed, and what later conclusions depend on it?**

Provenance is not optional metadata. It is part of the meaning of the intelligence.

```text
intelligence without provenance != trustworthy intelligence
```

Pathfinder preserves provenance across receipt, preservation, validation, parsing, normalization, correlation, assessment, publication, and reprocessing.

## Governing Principles

> **Preserve origin before interpretation.**

> **Every material derived fact must be traceable toward its source.**

> **The source of a claim and the source from which Pathfinder received that claim are not always the same thing.**

> **Redistribution is not independent corroboration.**

> **Parser and normalization versions are part of provenance.**

> **Later processing must not make earlier knowledge appear more complete than it actually was.**

## Source

A `Source` represents an intelligence provider or origin known to Pathfinder.

Examples include government sources, ISACs, commercial vendors, internal SOC sources, customer-private intelligence, partner organizations, and research sources.

Every Source receives a Pathfinder-controlled UUIDv7 identity. Display names, URLs, hostnames, and API endpoints are attributes and must not become object identity because they may change.

Conceptual fields include:

```text
source_id
name
source_type
provider_identity
description / not_recorded
status
created_at
retired_at / not_applicable
reliability_profile / not_assessed
handling_profile
authentication_profile_reference / not_applicable
```

Credentials themselves do not belong in Source records.

Initial source classes may include:

```text
government
industry
isac
vendor
community
internal
customer
partner
research
other
```

Source class does not establish trustworthiness.

## SourceCollection

A Source may expose multiple distinct collections with different semantics, update cadence, handling rules, and quality characteristics.

Examples include a vendor malware feed, infrastructure feed, phishing feed, or a government KEV catalog and advisory publication stream.

Pathfinder therefore represents `SourceCollection` separately.

Conceptual fields:

```text
source_collection_id
source_id
name
collection_type
external_collection_identifier / not_known
transport_type
endpoint_reference / not_applicable
expected_format
status
```

This allows Pathfinder to record not just who supplied information, but through which collection it arrived.

## SourceRecord

A `SourceRecord` represents one specific received unit of source material.

Examples include one TAXII object, JSON feed item, API result object, advisory document, CSV row, structured report, or analyst-submitted source artifact.

A SourceRecord answers:

> **What exactly did Pathfinder receive?**

Every SourceRecord receives a Pathfinder-controlled UUIDv7 identity. Provider-supplied record identifiers remain separate.

Conceptual fields include:

```text
source_record_id
source_id
source_collection_id / not_applicable
external_record_id / not_known
source_publication_time / not_known
source_modified_time / not_known
source_observation_time / not_known
retrieved_at / not_applicable
received_at
transport_type
media_type / not_known
source_format
source_location
source_marking
raw_content_hash
preserved_content_reference
preservation_state
validation_state
processing_state
created_at
```

## Origin Source Versus Delivery Source

Pathfinder must distinguish:

```text
ORIGIN SOURCE
    who originally made the intelligence claim

DELIVERY SOURCE
    who delivered that material to Pathfinder
```

If CISA publishes an advisory and multiple vendors redistribute it, Pathfinder must not count the redistributors as independent origins.

Where this information can be established, provenance should support:

```text
origin_source_id / not_known
delivery_source_id
upstream_source_record_id / not_known
redistribution_chain / not_known
```

Unknown origin remains unknown. Pathfinder does not guess.

## RetrievalEvent

A retrieval attempt is operational history distinct from the SourceRecord.

Pathfinder should preserve a `RetrievalEvent` or equivalent processing record for activity such as API polling, TAXII retrieval, HTTPS document retrieval, and manual import.

A retrieval event may produce zero records, one record, many records, partial results, or failure.

Conceptual fields:

```text
retrieval_event_id
source_id
source_collection_id
started_at
completed_at / not_completed
result_state
records_received
continuation_state
transport_status
rate_limit_state
authentication_state
error_class / not_applicable
```

Truth separations include:

```text
retrieval succeeded != intelligence accepted
retrieval returned zero records != source contains no intelligence
retrieval failed != source record invalid
```

## Raw Source Preservation

Where the governing preservation contract requires it, Pathfinder preserves original source bytes before destructive transformation.

Conceptually:

```text
RECEIVE
   ↓
BOUND / VALIDATE TRANSPORT
   ↓
PRESERVE ORIGINAL
   ↓
HASH
   ↓
PARSE
   ↓
NORMALIZE
```

Preserved source material should retain original bytes, content hash, size, received time, source identity, collection identity, transport context, media type, and preservation state.

The detailed storage and retention rules are frozen in Phase 0.11.

## Source Hash

Pathfinder calculates a cryptographic digest of preserved source content. Initial direction is SHA-256.

The digest establishes content identity/integrity within Pathfinder's processing chain. It does not establish source authenticity by itself.

```text
hash matches != source authenticated
source authenticated != content true
```

External source-provided signatures or hashes are preserved separately from Pathfinder's own preservation digest.

## Source Authenticity

Pathfinder distinguishes:

```text
transport authenticated
source identity authenticated
content cryptographically signed
signature verified
content unsigned
source authenticity not_verified
```

These are separate facts.

```text
HTTPS connection valid != individual record signed
record signature valid != intelligence assertion true
```

The broader trust/security model is finalized in Phase 0.15.

## Source Location

Pathfinder preserves where a SourceRecord was obtained, such as TAXII collection identifier, API endpoint, document URL, feed path, manual import reference, or partner exchange reference.

Source location is provenance, not object identity.

## Source Markings

Source handling restrictions survive ingestion and normalization.

Potential marking classes may later include TLP, provider proprietary markings, customer-private, partner-restricted, government handling markings, and redistribution restrictions.

Normalizing a technically public observable out of a restricted report does not automatically remove the handling restrictions associated with the report, source assertion, or surrounding context.

## Parser Provenance

Any source-derived structured information identifies the parser responsible.

Conceptual fields include:

```text
parser_name
parser_version
parser_contract_version
processed_at
```

A parser upgrade may interpret the same SourceRecord differently. Pathfinder preserves which parser version produced which result.

```text
new parser output != original parser output rewritten
```

## Normalization Provenance

Normalization also requires versioning.

Conceptual fields include:

```text
normalizer_name
normalizer_version
canonicalization_profile
processed_at
```

If normalization behavior changes later, Pathfinder remains able to establish which rule set generated historical results.

## Correlation Provenance

Derived Relationships preserve their derivation basis.

Conceptual fields include:

```text
derivation_method
derivation_version
basis_ids
derived_at
```

Pathfinder must not merely record `related = true` without retaining why the relationship exists.

## Assessment Provenance

Machine Assessments preserve process identity, algorithm/rule identity, version, basis, inputs, and processing time.

Human Assessments preserve analyst principal, basis, assessment time, and rationale where recorded.

Machine and human provenance remain distinct.

## Processing Lineage

Pathfinder supports directional lineage such as:

```text
Source
   ↓
SourceCollection
   ↓
RetrievalEvent
   ↓
SourceRecord
   ↓
Assertion
   ↓
Observable / Relationship
   ↓
Assessment
   ↓
Published Intelligence
```

Not every path requires every stage, but every material derived object must preserve enough lineage to explain its origin.

## Provenance Is Many-to-Many

One SourceRecord may produce many Assertions, Observables, and Relationships. One normalized object may be supported by many SourceRecords, Assertions, and Sources.

The implementation must preserve this rather than placing one simplistic `source_id` field on every object and calling provenance complete.

## Independent Corroboration

Provenance must allow Pathfinder to determine whether apparently separate reports actually share an origin.

For example, three delivery sources may represent only two independent origin sources if two merely redistribute the same upstream report.

If independence cannot be established:

```text
independence = not_known
```

Pathfinder does not assume independence.

## Provenance Completeness

Pathfinder represents whether provenance is complete, partial, or not known.

Unknown upstream provenance is legitimate. Pathfinder must not invent missing provenance to fill a field.

## Historical Preservation

Provenance is historical.

If a source changes name, URL, provider ownership, collection identifier, or authentication method, Pathfinder retains enough historical context to reconstruct how past records entered the system.

```text
current source endpoint != historical retrieval endpoint
```

## Time Semantics

Pathfinder preserves distinct times where available:

```text
source_observation_time
source_publication_time
source_modified_time
retrieval_started_at
retrieval_completed_at
received_at
preserved_at
parsed_at
normalized_at
correlated_at
assessed_at
```

These are not collapsed into one generic timestamp.

## Failure Provenance

Failed processing is also provenance.

Pathfinder preserves enough history to establish states such as:

```text
SourceRecord received
preservation succeeded
validation succeeded
parser failed
normalization not_performed
assessment not_performed
```

A later successful retry does not erase an earlier failed attempt.

## Reprocessing

Reprocessing occurs against preserved source material or other authoritative Pathfinder records.

Reprocessing creates new lineage rather than rewriting old processing history.

Conceptually:

```text
SourceRecord
   |
   +--> Parser v1 --> Result A
   |
   +--> Parser v2 --> Result B
```

A newer result may become current while earlier processing history remains available.

## Deletion and Provenance

Deleting or expiring derived intelligence must not destroy provenance required by retained historical records.

```text
object expired != provenance disposable
```

The later retention contract must account for dependency chains such as Assessment -> Assertion -> SourceRecord -> preserved source content.

## Common Truth Separations

```text
Source != SourceRecord
Source != SourceCollection
SourceRecord != RetrievalEvent
origin source != delivery source
delivery source count != independent source count
source authenticated != assertion true
TLS validated != source content signed
content signed != assertion true
source hash != source authenticity
same source content != independent corroboration
current source endpoint != historical source endpoint
retrieval time != publication time
publication time != observation time
receipt time != observation time
parser version != source version
parser output != source text
normalization != source assertion
derived relationship != directly reported relationship
new parser interpretation != old interpretation erased
processing failure != source-record loss
provenance incomplete != provenance fabricated
raw source != normalized intelligence
source marking != automatically removable during normalization
one normalized object != one source
one source record != one assertion
```

## Phase 0.5 Exit Decision

Phase 0.5 is satisfied when Pathfinder accepts:

1. `Source`, `SourceCollection`, `RetrievalEvent`, and `SourceRecord` as distinct provenance concepts.
2. Pathfinder-controlled UUIDv7 identities for provenance records.
3. Explicit separation between origin source and delivery source.
4. Source provenance survives normalization, correlation, assessment, and reprocessing.
5. SourceRecord preserves what Pathfinder actually received.
6. Source hashes protect preserved-content identity but do not establish truth or source authenticity.
7. Source markings and handling restrictions survive processing.
8. Parser and normalizer identities and versions are part of provenance.
9. Derived Relationships preserve their derivation basis.
10. Human and machine Assessments preserve different authority/provenance.
11. Provenance supports many-to-many relationships.
12. Redistributed intelligence does not automatically count as independent corroboration.
13. Unknown provenance remains explicitly unknown.
14. Historical source configuration and processing history are not overwritten by current configuration.
15. Observation, publication, retrieval, receipt, processing, and assessment times remain distinct.
16. Failed and successful processing attempts both remain reconstructable.
17. Reprocessing produces new lineage rather than rewriting old lineage.
18. Retention must not silently destroy provenance required to understand retained intelligence.
