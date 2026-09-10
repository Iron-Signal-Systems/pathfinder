# Pathfinder Phase 0.5 — Source and Provenance Model

## Purpose

Pathfinder must preserve enough provenance to answer:

> **Where did this intelligence come from, what exactly was acquired, how was it processed, and what later conclusions depend on it?**

Provenance is part of intelligence meaning. It is not optional decoration.

The governing principles are:

> **Preserve origin before interpretation.**

> **Every material derived fact must be traceable toward its source.**

> **The source of a claim and the source from which Pathfinder received that claim are not always the same thing.**

> **Redistribution is not independent corroboration.**

> **Parser and normalization versions are part of provenance.**

> **Later processing must not make earlier knowledge appear more complete than it actually was.**

> **The original record is maintained no matter what.**

## Canonical Acquisition Chain

The authoritative acquisition/provenance chain is:

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
normalized / derived Pathfinder intelligence
```

`SourceArtifact` and `SourceRecord` are distinct.

```text
SourceArtifact = exact acquired payload
SourceRecord   = logical item within that payload
```

## Source

A `Source` represents an intelligence provider or origin known to Pathfinder.

Examples include government sources, ISACs, vendors, internal SOC sources, customer-private sources, partner organizations, and research sources.

Every Source receives a Pathfinder-controlled UUIDv7 identity. Display names, URLs, hostnames, and endpoints are attributes, not object identity.

Conceptual fields may include:

```text
source_id
name
source_type
provider_identity
description / NOT_RECORDED
status
created_at
retired_at / NOT_APPLICABLE
handling_profile
authentication_profile_reference / NOT_APPLICABLE
reliability_policy_reference / NOT_APPLICABLE
```

Credentials do not belong in Source records.

Authoritative source reliability does not live as a mutable field on Source. Reliability is represented through historical `Assessment` records against Source or SourceCollection.

```text
Source.reliability = HIGH
```

is not an authoritative Pathfinder storage model.

Initial source classes may include:

```text
GOVERNMENT
INDUSTRY
ISAC
VENDOR
COMMUNITY
INTERNAL
CUSTOMER
PARTNER
RESEARCH
OTHER
```

Source class does not establish reliability or truth.

## SourceCollection

A Source may expose multiple distinct collections with different semantics, update cadence, handling rules, and quality characteristics.

Examples include a vendor malware feed, infrastructure feed, phishing feed, KEV-style catalog, advisory publication stream, or TAXII Collection.

Conceptual fields may include:

```text
source_collection_id
source_id
name
collection_type
external_collection_identifier / NOT_KNOWN
transport_type
endpoint_reference / NOT_APPLICABLE
expected_format
status
created_at
```

Reliability may be assessed specifically at SourceCollection scope where that is more meaningful than Source-wide reliability.

## RetrievalEvent

A `RetrievalEvent` is an acquisition attempt and operational record distinct from both SourceArtifact and SourceRecord.

A retrieval may produce:

```text
zero artifacts
one artifact
many artifacts
partial artifacts
failure
```

Conceptual fields may include:

```text
retrieval_event_id
source_id
source_collection_id / NOT_APPLICABLE
started_at
completed_at / NOT_COMPLETED
operation_result
transport_status
authentication_state
rate_limit_state
continuation_state
artifacts_received
records_reported / NOT_KNOWN
error_class / NOT_APPLICABLE
```

Truth separations include:

```text
retrieval succeeded != intelligence accepted
retrieval returned zero records != source contains no intelligence
retrieval failed != source content invalid
```

## SourceArtifact

A `SourceArtifact` is an immutable acquired payload preserved at the application-content boundary before semantic parsing, normalization, correlation, or assessment.

Examples include:

```text
HTTP response entity body
TAXII response page
downloaded JSON/CSV/XML document
compressed feed artifact
vendor export
advisory PDF
analyst-imported source file
```

Every SourceArtifact receives a Pathfinder-controlled UUIDv7 identity.

Conceptual fields include:

```text
source_artifact_id
source_id
source_collection_id / NOT_APPLICABLE
retrieval_event_id / NOT_APPLICABLE
received_at
source_location
media_type / NOT_KNOWN
content_encoding / NOT_KNOWN
byte_length
sha256
preservation_state
integrity_state
availability_state
storage_reference
handling_profile
created_at
```

The exact preserved bytes and Pathfinder-calculated preservation digest belong here.

```text
SourceArtifact owns exact bytes
SourceRecord does not own exact bytes
```

## SourceRecord

A `SourceRecord` represents one logical item within a SourceArtifact.

Examples include:

```text
one STIX object within a TAXII response
one JSON object within a feed document
one CSV row
one logical advisory entry
one report item
```

A SourceRecord receives a Pathfinder-controlled UUIDv7 identity and references its parent SourceArtifact.

Conceptual fields include:

```text
source_record_id
source_artifact_id
source_id
source_collection_id / NOT_APPLICABLE
external_record_id / NOT_KNOWN
source_publication_time / NOT_KNOWN
source_modified_time / NOT_KNOWN
source_observation_time / NOT_KNOWN
source_locator
source_marking
validation_state
processing_state
created_at
```

The following artifact-level fields do **not** belong on SourceRecord as authoritative storage:

```text
raw_content_hash
preserved_content_reference
artifact media type
artifact byte length
artifact preservation state
artifact integrity state
artifact availability state
```

Those belong to SourceArtifact.

Where exact per-record bytes cannot be established safely, the parent SourceArtifact plus deterministic SourceRecord locator remains the authoritative path back to what Pathfinder acquired.

## SourceRecord Locator

A SourceRecord should preserve a deterministic locator where practical.

Examples include:

```text
JSON Pointer
STIX object ID + bundle/page context
XML path
CSV row number
archive-member path
byte range where safely established
document section
```

A parser-generated reserialization must never be called the original source bytes.

## Origin Source Versus Delivery Source

Pathfinder distinguishes:

```text
ORIGIN SOURCE
    who originally made the intelligence claim

DELIVERY SOURCE
    who delivered that material to Pathfinder
```

If one upstream report is redistributed by several vendors, those redistributors do not automatically become independent origins.

Where known, provenance may include:

```text
origin_source_id / NOT_KNOWN
delivery_source_id
upstream_source_record_id / NOT_KNOWN
redistribution_chain / NOT_KNOWN
```

Unknown origin remains unknown.

## Preservation and Integrity

Pathfinder preserves SourceArtifact bytes before destructive transformation where the source contract requires it.

Conceptually:

```text
RECEIVE
   ↓
BOUND TRANSPORT
   ↓
PRESERVE SOURCEARTIFACT
   ↓
HASH / VERIFY
   ↓
PARSE TO SOURCERECORDS
   ↓
ASSERTIONS
   ↓
NORMALIZE / DERIVE / ASSESS
```

A Pathfinder SHA-256 digest over the exact preserved SourceArtifact supports later integrity verification.

It does not prove source identity, source reliability, or assertion truth.

```text
hash matches != source authenticated
source authenticated != assertion true
```

External source-provided hashes/signatures remain separate from Pathfinder's preservation digest.

## Source Authenticity

Pathfinder keeps separate facts such as:

```text
transport authenticated
source identity authenticated
content cryptographically signed
signature verified
content unsigned
authenticity NOT_VERIFIED
```

```text
HTTPS connection valid != individual object signed
signature valid != assertion true
```

## Source Location

Pathfinder preserves acquisition location/context such as TAXII Collection, API endpoint, document URL, feed path, manual import reference, or partner exchange reference.

Source location is provenance, not identity.

## Source Markings

Handling restrictions survive ingestion and normalization.

A technically public Observable extracted from a restricted report does not automatically make the surrounding Assertion, context, or source material unrestricted.

Marking and handling rules remain server-enforced through later read/export paths.

## Parser Provenance

Any source-derived structured information identifies the parser/process responsible.

Processing lineage should preserve, where applicable:

```text
process identity
parser name
parser version
parser contract version
input SourceArtifact / SourceRecord
output records
processed_at
result
```

A parser upgrade creates new processing lineage.

```text
new parser result != old parser result rewritten
```

## Normalization Provenance

Normalization also requires versioning.

Conceptual lineage includes:

```text
normalizer_name
normalizer_version
canonicalization_profile
input SourceRecord / Assertion
output object IDs
processed_at
result
```

If canonicalization behavior changes, historical outputs remain attributable to the version that produced them.

## Correlation Provenance

Pathfinder-derived Relationships preserve:

```text
derivation_method
derivation_version
basis_ids
derived_at
result
```

`related = true` without a reconstructable basis is insufficient.

## Assessment Provenance

Machine Assessments preserve process/rule identity, version, basis, inputs, and time.

Human Assessments preserve analyst principal, basis, time, and rationale where recorded.

Machine and human authority remain distinct.

## Source Reliability Provenance

Source reliability is historical Assessment state.

Example:

```text
Assessment
    subject = SourceCollection X
    assessment_type = SOURCE_RELIABILITY
    assessment_value = MODERATE
    authority = HUMAN_ANALYST
```

A later reliability Assessment does not rewrite what Pathfinder's reliability judgment was when an earlier intelligence Assessment was made.

```text
current source reliability != historical source reliability
```

## Processing Lineage

Canonical lineage may look like:

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
Observable / Indicator / Relationship / other intelligence
   ↓
Assessment
   ↓
current view / export where authorized
```

Not every path requires every stage, but every material derived record must preserve enough lineage to explain its origin.

## Provenance Is Many-to-Many

One SourceArtifact may contain many SourceRecords.

One SourceRecord may produce many Assertions and normalized objects.

One normalized object or Relationship may be supported by many Assertions from many SourceRecords and Sources.

The implementation must not put one simplistic `source_id` on every intelligence object and call provenance complete.

## Independent Corroboration

Provenance must allow Pathfinder to distinguish:

```text
record count
delivery source count
origin source count
independent origin count
```

If independence cannot be established:

```text
independence = NOT_KNOWN
```

Pathfinder does not assume independence.

## Provenance Completeness

Provenance itself has completeness/coverage state.

Unknown upstream provenance is legitimate. Missing origin information must not be guessed merely to fill a schema field.

## Historical Preservation

If source names, endpoints, ownership, collection identifiers, trust configuration, or authentication methods change, historical records retain enough context to reconstruct how older material entered Pathfinder.

```text
current source endpoint != historical retrieval endpoint
```

## Time Semantics

Pathfinder preserves separate times where available:

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

These are not interchangeable.

## Failure Provenance

Failure is also provenance.

Pathfinder preserves stage-level states such as:

```text
SourceArtifact preserved
integrity verified
SourceRecord created
validation succeeded
parser failed
normalization NOT_PERFORMED
assessment NOT_PERFORMED
```

A later successful retry does not erase an earlier failed attempt.

## Reprocessing

Reprocessing starts from preserved SourceArtifact material or other authoritative historical Pathfinder records.

Example:

```text
SourceArtifact A
    ├─ parser v1 -> Result Set A
    └─ parser v2 -> Result Set B
```

Both processing histories remain.

## Retention and Destruction

Lifecycle status does not directly authorize destruction of source material or provenance.

```text
Indicator EXPIRED != SourceArtifact deletable
Source withdrawn != SourceArtifact deletable
```

If law, contract, handling policy, or another authorized requirement mandates destruction of raw SourceArtifact bytes, Pathfinder preserves immutable historical metadata describing the artifact and destruction action.

```text
raw bytes destroyed by policy != historical record removed
```

## Common Truth Separations

```text
Source != SourceCollection
SourceCollection != RetrievalEvent
RetrievalEvent != SourceArtifact
SourceArtifact != SourceRecord
SourceRecord != Assertion
origin source != delivery source
delivery count != independent origin count
source authenticated != assertion true
content signed != assertion true
SourceArtifact hash != source authenticity
same content != independent corroboration
current source endpoint != historical endpoint
retrieval time != publication time
publication time != observation time
receipt time != observation time
parser output != original bytes
normalization != source assertion
derived Relationship != directly reported Relationship
new parser interpretation != old interpretation erased
processing failure != source history lost
provenance incomplete != provenance fabricated
source marking != removable during normalization
one normalized object != one source
Source reliability != mutable Source field
```

## Phase 0.5 Exit Decision

Phase 0.5 is satisfied when Pathfinder accepts that:

1. `Source`, `SourceCollection`, `RetrievalEvent`, `SourceArtifact`, and `SourceRecord` are distinct provenance concepts.
2. SourceArtifact owns exact acquired bytes and Pathfinder's preservation digest.
3. SourceRecord is a logical item within a SourceArtifact and preserves a locator/provenance path.
4. Origin and delivery source remain separate.
5. Source provenance survives normalization, correlation, Assessment, export, and reprocessing.
6. SourceArtifact hashes support integrity verification but do not establish truth/authenticity.
7. Markings and handling restrictions survive processing.
8. Parser and normalizer identities/versions are part of provenance.
9. Derived Relationships preserve their derivation basis.
10. Human and machine Assessments preserve distinct authority/provenance.
11. Source reliability is represented through historical Assessments, not a mutable Source truth field.
12. Provenance supports many-to-many lineage.
13. Redistributed intelligence does not automatically count as independent corroboration.
14. Unknown provenance remains explicitly unknown.
15. Observation, publication, retrieval, receipt, preservation, processing, and Assessment times remain distinct.
16. Failed and successful processing attempts both remain reconstructable.
17. Reprocessing creates new lineage rather than rewriting old lineage.
18. Retention/destruction never silently removes the historical record needed to explain retained intelligence.
19. **The original record is maintained no matter what.**
