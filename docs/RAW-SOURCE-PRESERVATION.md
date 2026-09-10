# Pathfinder Phase 0.11 — Raw-Source Preservation Contract

## Purpose

Pathfinder must preserve the source material necessary to establish what it actually acquired before semantic interpretation changes that material.

> **Preserve what was received before deciding what it means.**

Raw-source preservation exists so Pathfinder can later answer:

```text
What exactly arrived?
Where did it come from?
When did it arrive?
Was it preserved completely?
Was its integrity verified?
Is it currently available?
Which parser processed it?
Can it be processed again?
Did later interpretation change while the original source remained the same?
```

The governing historical invariant is:

> **The original record is maintained no matter what.**

## Preservation Boundary

Pathfinder preserves source content at the application-content boundary before semantic parsing, normalization, correlation, or Assessment.

Conceptually:

```text
TRANSPORT
    ↓
acquired application payload
    ├─ preserve exact bytes as SourceArtifact
    ↓
decode / extract / parse
    ↓
SourceRecord / Assertion
    ↓
normalize / correlate / assess
```

Pathfinder does not need to preserve TLS records, TCP segmentation, Ethernet frames, or HTTP transfer framing merely to preserve threat-intelligence source content.

Those belong to other observation systems where applicable.

## SourceArtifact

A `SourceArtifact` is an immutable acquired payload preserved before semantic interpretation.

Examples include:

```text
HTTP response entity body
TAXII response page
downloaded JSON document
CSV feed file
XML document
compressed feed artifact
vendor export
advisory PDF
analyst-imported source file
```

Identity:

```text
source_artifact_id = UUIDv7
```

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

Transport/authentication metadata belongs to RetrievalEvent/provenance records rather than being embedded into the preserved bytes.

## Exact Bytes

The SourceArtifact contains the exact byte sequence accepted at the defined preservation boundary.

Pathfinder must not parse data, serialize it again, and call the reconstructed representation the original.

Re-serialization may change whitespace, ordering, escaping, encoding, numeric representation, line endings, duplicate-key representation, or formatting.

```text
re-serialized representation != original SourceArtifact bytes
```

## SourceArtifact Versus SourceRecord

These are separate concepts.

```text
SourceArtifact
    exact acquired payload

SourceRecord
    logical item represented within the payload
```

Example:

```text
SourceArtifact:
    TAXII response containing 200 STIX objects

SourceRecords:
    STIX object 1
    STIX object 2
    ...
    STIX object 200
```

All 200 SourceRecords may reference the same SourceArtifact.

A SourceRecord should preserve a deterministic locator where practical, such as JSON Pointer, STIX object ID plus bundle/page context, XML path, CSV row, archive member, byte range where safely established, or document section.

Where exact per-record byte boundaries cannot be established safely:

```text
exact_record_bytes = NOT_ESTABLISHED
```

The parent SourceArtifact remains authoritative for the exact acquired bytes.

## HTTP Preservation

For HTTP-based acquisition, Pathfinder preserves the response entity/body representation at the defined application boundary before semantic parsing.

It does not need to preserve transfer framing such as chunk boundaries.

Relevant HTTP metadata such as status, content type, content encoding, selected headers, and request/retrieval identity remains in provenance.

## Compression and Containers

If a source delivers compressed or archived content, Pathfinder preserves the acquired representation and treats decompression/extraction as processing.

Example:

```text
SourceArtifact A
    vendor-feed.zip
        ├─ extraction -> SourceArtifact B (feed.json)
        └─ extraction -> SourceArtifact C (metadata.json)
```

Derived artifacts preserve parent identity, extraction method/version, and their own integrity metadata.

The decoded/extracted representation never replaces the acquired artifact.

## Integrity Digest

Every complete preserved SourceArtifact receives a Pathfinder-calculated SHA-256 digest over the exact preserved bytes.

```text
recomputed SHA-256 == recorded SHA-256
```

supports integrity verification.

It does not prove:

```text
who created the content
source reliability
source truthfulness
transport identity
assertion correctness
```

External publisher hashes or signatures are preserved separately from Pathfinder's preservation digest.

## Immutability

A committed SourceArtifact is immutable.

Once committed, its identity and preserved byte content are not modified in place.

If a source publishes a corrected payload or Pathfinder reacquires content, a new SourceArtifact is created.

```text
new source version != mutate old artifact
```

## Typed Artifact State

Pathfinder does **not** use one combined SourceArtifact status field.

Artifact state is represented through separate dimensions.

### PreservationState

Describes whether acquisition/preservation itself completed.

```text
RECEIVING
PRESERVED
PARTIAL
FAILED
```

`PRESERVED` means the complete artifact bytes and required metadata were durably committed.

`PARTIAL` means some bytes/material were acquired or retained but the complete artifact was not established.

`FAILED` means the preservation contract was not satisfied.

### IntegrityState

Describes whether the preserved content has been verified against its recorded integrity basis.

```text
NOT_VERIFIED
VERIFIED
MISMATCH
```

`VERIFIED` means the stored bytes passed the required Pathfinder integrity check.

`MISMATCH` means integrity verification failed and the artifact must not silently continue as trusted processing input.

### AvailabilityState

Describes whether the artifact is currently accessible for ordinary authorized processing.

```text
AVAILABLE
QUARANTINED
UNAVAILABLE
DESTROYED_BY_POLICY
```

`QUARANTINED` means the artifact is retained but ordinary processing/access is intentionally restricted by security, validation, handling, or policy.

`UNAVAILABLE` means Pathfinder has artifact metadata/history but cannot currently access the source content.

`DESTROYED_BY_POLICY` means authorized retention/handling policy required destruction of the raw bytes while historical metadata remains.

These state dimensions are independent.

A valid artifact state can therefore be:

```text
preservation_state = PRESERVED
integrity_state    = VERIFIED
availability_state = QUARANTINED
```

without contradiction.

The earlier combined vocabulary that treated `VERIFIED` or `QUARANTINED` as preservation states is superseded by this typed model.

## No Parsing Before Required Preservation

Where the source contract requires raw preservation, authoritative derived intelligence must not commit until required preservation succeeds.

Conceptually:

```text
receive
  ↓
preserve SourceArtifact completely?
  ├─ NO  -> fail/restrict semantic commit
  └─ YES -> validate / parse
```

This prevents authoritative derived intelligence from existing without the required original source basis.

## Streaming and Segmentation

Large or continuous sources require explicit segmentation contracts.

Possible segment boundaries include:

```text
response
page
batch
time window
provider object set
bounded file
```

Each committed segment is its own immutable SourceArtifact.

Pathfinder must never use an undocumented boundary equivalent to “whatever happened to be in memory.”

## Resource Bounds

Raw-source acquisition operates on untrusted input.

Every source contract defines appropriate bounds such as:

```text
maximum response size
maximum object size
maximum archive size
maximum extracted size
maximum member count
maximum nesting depth
maximum compression expansion
maximum processing duration
```

Exceeding a bound creates an explicit failure/processing result and must not cause uncontrolled resource use.

## Oversized or Partial Acquisition

Oversized input may produce a detailed failure such as `SOURCE_TOO_LARGE`.

Pathfinder preserves enough metadata to establish source, RetrievalEvent, failure time, configured limit, and observed/reported size where available.

If full content was not safely acquired:

```text
preservation_state = PARTIAL or FAILED
```

not `PRESERVED`.

Partial diagnostic bytes may be retained where policy permits but must never receive complete-artifact semantics.

## Parser Failure and Unsupported Format

Preservation success and parser success are independent.

Example:

```text
preservation_state = PRESERVED
integrity_state    = VERIFIED
availability_state = AVAILABLE
processing_state   = FAILED
failure_code       = PARSE_FAILED
```

The SourceArtifact remains available for parser repair, future reprocessing, analysis, and forensic review.

Likewise, an unsupported format remains preserved where policy permits.

```text
UNSUPPORTED != discard source
```

## Reprocessing

Reprocessing starts from preserved authoritative SourceArtifact bytes where available.

Example:

```text
SourceArtifact A
    ├─ parser v1 -> Result Set A
    └─ parser v2 -> Result Set B
```

The SourceArtifact remains unchanged. Both ProcessingRecords remain historical.

## Hostile Source Content

Threat-intelligence source material may be deliberately hostile.

Examples include malformed documents, parser/decompression bombs, path traversal entries, oversized strings, deep nesting, unexpected binary data, crafted URLs, hostile Unicode, embedded scripts, and malicious PDFs.

Preservation does not imply execution or rendering.

Pathfinder treats source content as data.

## No Active Content Execution

Preservation/parsing must not automatically:

```text
execute scripts
run macros
open embedded executables
resolve or visit URLs
fetch referenced resources
load remote images
execute document actions
invoke shell commands
```

Any future external enrichment is a separate controlled operation.

```text
source contains URL != Pathfinder visits URL
```

## Archive Extraction Safety

Archive processing protects against path traversal, absolute paths, device files, symlink/hard-link escape, recursive archive expansion, and resource exhaustion.

Extraction remains within explicitly controlled storage boundaries.

## Content-Type and Encoding

Declared media type or file extension does not establish actual format validity.

Character decoding occurs after byte preservation.

Where relevant, Pathfinder preserves declared charset, validated/detected charset, decoder/profile identity, and processing result.

A decoding failure never alters the preserved source bytes.

## Marking and Handling

Source handling restrictions apply to raw artifacts.

Normalized-object read authority does not automatically grant raw-artifact read authority.

```text
read normalized intelligence != read SourceArtifact
```

Raw source may contain provider-restricted, customer-private, partner-restricted, government-handled, or otherwise sensitive content.

## Encryption at Rest

The implementation must support protection of preserved source content appropriate to deployment sensitivity and threat model.

> **Raw-source preservation must not create an unprotected archive of sensitive intelligence.**

Exact implementation belongs to storage/runtime architecture.

## Secrets and Logging

Source content may accidentally contain credentials, API keys, tokens, or private customer data.

Pathfinder must not automatically copy raw payloads into logs, errors, metrics, traces, support bundles, or panic output.

Prefer diagnostic references such as:

```text
source_artifact_id
sha256
byte_length
processing_state
failure_code
parser/version
```

rather than dumping source bytes.

## Missing or Corrupt Artifact

If integrity verification fails:

```text
integrity_state = MISMATCH
```

If content cannot currently be accessed:

```text
availability_state = UNAVAILABLE
```

Neither condition rewrites the original artifact metadata or downstream processing history.

Derived intelligence must expose the preservation/integrity problem where it materially affects provenance or current interpretation.

## Backup Is Not Primary Preservation

A replica or backup does not substitute for successful initial SourceArtifact preservation.

Primary preservation success must be established at the authoritative commit boundary.

## Retention and Destruction

Raw SourceArtifact retention may depend on source contract, license, handling restrictions, customer policy, operational need, legal hold, or storage policy.

Intelligence lifecycle does not itself authorize raw destruction.

```text
Indicator EXPIRED != SourceArtifact deletable
source removed != SourceArtifact deletable
```

Raw-byte destruction is a separate authorized operation requiring artifact identity, applicable authority, policy basis, destruction time, and result.

An applicable hold overrides routine destruction eligibility.

## Mandatory Raw-Byte Destruction

If law, contract, handling policy, or another authorized requirement mandates destruction of raw bytes, Pathfinder preserves immutable historical metadata such as:

```text
source_artifact_id
sha256
original byte_length
received_at
source/source_collection
prior availability/integrity state
destroyed_at
destroy_authority
policy/legal basis
associated SourceRecords
prior processing lineage
```

Then:

```text
availability_state = DESTROYED_BY_POLICY
```

The raw bytes may be gone. The historical record proving the artifact existed and explaining what happened remains.

```text
raw bytes destroyed != historical record removed
```

## Source Deletion by Provider

A provider removing a report later does not rewrite Pathfinder history.

```text
provider no longer hosts source != Pathfinder never received source
```

Pathfinder retains its historical record subject to applicable retention/handling authority.

## Raw Source and Search

Raw SourceArtifacts do not need to be full-text indexed in the first vertical slice.

Search indexes are derived structures. Failure to index raw content does not change SourceArtifact preservation state.

## Common Truth Separations

```text
SourceArtifact != SourceRecord
PRESERVED != VERIFIED
VERIFIED != AVAILABLE
QUARANTINED != deleted
UNAVAILABLE != never preserved
DESTROYED_BY_POLICY != history removed
partial bytes != complete artifact
Pathfinder SHA-256 != source authenticity
source authenticated != assertion true
parser failed != artifact lost
unsupported format != artifact discarded
source contains URL != Pathfinder retrieves URL
normalized read != raw read
backup exists != primary preservation succeeded
Indicator expired != raw destruction authorized
new parser result != old processing erased
```

## Phase 0.11 Exit Decision

Phase 0.11 is satisfied when Pathfinder accepts that:

1. SourceArtifact is the immutable exact acquired payload.
2. SourceRecord is a logical item within SourceArtifact and uses a deterministic locator where practical.
3. Parser reserialization is never called the original source representation.
4. Pathfinder SHA-256 supports content-integrity verification but does not establish source truth/authenticity.
5. Preservation, integrity, and availability are separate typed state dimensions.
6. `PRESERVED`, `VERIFIED`, `QUARANTINED`, `UNAVAILABLE`, and `DESTROYED_BY_POLICY` therefore do not compete in one status enum.
7. Required preservation succeeds before authoritative derived intelligence commits.
8. Streaming/large sources have explicit segment boundaries.
9. Acquisition and extraction are bounded against hostile input.
10. Parser/format failure does not erase a successfully preserved SourceArtifact.
11. Reprocessing creates new ProcessingRecords and never rewrites earlier processing.
12. Raw source access/handling is separately authorized and sensitive content is not dumped into diagnostics.
13. Lifecycle expiration does not itself authorize source destruction.
14. Mandatory raw-byte destruction preserves immutable artifact metadata and destruction history.
15. **The original record is maintained no matter what.**
