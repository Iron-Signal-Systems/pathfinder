# Pathfinder Phase 0.11 — Raw-Source Preservation Contract

## Purpose

Pathfinder must preserve the source material necessary to prove what it actually received before interpretation changed that material.

The governing principle is:

> **Preserve what was received before deciding what it means.**

Raw-source preservation exists so Pathfinder can later answer:

```text
What exactly arrived?
Where did it come from?
When did it arrive?
Was it intact?
Which parser processed it?
Can we process it again with a newer parser?
Did later interpretation change while the original source remained the same?
```

## Preservation Boundary

Pathfinder preserves source content at the application-content boundary before semantic parsing, normalization, correlation, or assessment.

Conceptually:

```text
TRANSPORT
    |
    v
acquired source payload
    |
    +---- PRESERVE EXACT BYTES
    |
    v
decode / parse
    |
    v
SourceRecords / Assertions
    |
    v
normalize / correlate / assess
```

Pathfinder does not need to preserve:

```text
TLS records
TCP segments
Ethernet frames
HTTP packetization
```

merely to preserve threat-intelligence source truth.

Those belong to other observation systems where applicable.

## SourceArtifact

Phase 0.11 introduces a first-class concept:

```text
SourceArtifact
```

A `SourceArtifact` is an immutable acquired payload preserved before semantic interpretation.

Examples:

```text
HTTP response entity body
TAXII response payload
downloaded JSON document
CSV feed file
XML document
vendor export
advisory PDF
analyst-imported source file
```

Identity:

```text
source_artifact_id = UUIDv7
```

## SourceArtifact Conceptual Fields

```text
source_artifact_id
source_id
source_collection_id / not_applicable
retrieval_event_id / not_applicable
received_at
source_location
media_type / not_known
content_encoding / not_known
byte_length
sha256
preservation_state
storage_reference
handling_profile
created_at
```

Transport/authentication metadata belongs to the associated retrieval/provenance records rather than being embedded into the bytes.

## Exact Bytes

The preserved artifact must contain the exact byte sequence accepted at the defined preservation boundary.

Pathfinder must not preserve only a reconstructed version such as:

```text
parsed JSON
then serialized back to JSON
```

and call that the original.

These may differ in:

```text
whitespace
ordering
escaping
encoding
numeric representation
line endings
duplicate keys
formatting
```

Therefore:

```text
re-serialized representation
    != original source bytes
```

## HTTP Preservation

For HTTP-based acquisition, Pathfinder should preserve the response entity/body representation before semantic content decoding where technically applicable.

Pathfinder does not need to preserve HTTP transfer framing such as chunk boundaries.

Conceptually:

```text
HTTP/TLS transport
      |
      v
response entity bytes
      |
      +---- preserve
      |
      v
content decoding if required
      |
      v
semantic parser
```

Important transport metadata such as:

```text
status code
content type
content encoding
selected relevant headers
request/retrieval identity
```

must remain available through provenance.

## Compression

If a source delivers a compressed artifact, Pathfinder should preserve the acquired representation and treat decompression as processing.

Example:

```text
source.gzip
    |
    +---- preserved acquired artifact
    |
    v
decompression
    |
    v
decoded artifact / parsing input
```

Where Pathfinder chooses to preserve both compressed and decoded representations, their relationship and hashes must remain explicit.

The decoded representation must never replace the acquired artifact.

## Containers and Archives

Sources may provide:

```text
ZIP
TAR
GZIP
other approved containers
```

Container processing must be bounded and treated as untrusted input.

The original container remains preserved.

Extracted members may become additional artifacts where required.

Conceptually:

```text
SourceArtifact A
    vendor-feed.zip

        |
        +-- extraction --> SourceArtifact B
        |                 feed.json
        |
        +-- extraction --> SourceArtifact C
                          metadata.json
```

Derived artifacts must reference their parent artifact and extraction method/version.

## SourceArtifact Versus SourceRecord

A SourceArtifact and SourceRecord are different.

```text
SourceArtifact
    exact preserved acquired content

SourceRecord
    logical source item represented within that content
```

Example:

```text
SourceArtifact:
    TAXII bundle containing 200 STIX objects

SourceRecords:
    STIX object 1
    STIX object 2
    ...
    STIX object 200
```

All 200 SourceRecords may point back to the same preserved SourceArtifact.

Therefore:

```text
SourceArtifact != SourceRecord
```

## SourceRecord Locator

A SourceRecord derived from a larger SourceArtifact must preserve a deterministic locator where possible.

Examples:

```text
JSON Pointer
STIX object ID plus bundle context
XML path
CSV record number
archive-member path
byte range when safely established
document section
```

The locator must allow Pathfinder to work backward from the logical record to the source artifact.

## Exact Per-Record Bytes

Pathfinder does not require every logical SourceRecord to have separately reconstructed raw bytes if the exact parent SourceArtifact is preserved and the record can be deterministically located within it.

This avoids pretending that:

```text
JSON parser output
```

is equivalent to:

```text
original JSON bytes for that object
```

Where exact per-record byte ranges can be established safely, Pathfinder may preserve them.

Where they cannot:

```text
exact_record_bytes = NOT_ESTABLISHED
```

The parent artifact remains authoritative.

## Integrity Digest

Every preserved SourceArtifact receives a Pathfinder-calculated SHA-256 digest.

```text
artifact_sha256
```

The digest is calculated over the exact preserved bytes.

The digest permits later verification that preserved content has not changed.

```text
recomputed SHA-256 == recorded SHA-256
```

supports content-integrity verification.

It does not prove:

```text
who created the artifact
whether the source was truthful
whether the transport identity was authentic
whether the assertions are correct
```

## External Hashes and Signatures

If the source provides:

```text
publisher hash
digital signature
detached signature
signed metadata
```

Pathfinder preserves those separately.

Example:

```text
Pathfinder preservation hash:
    SHA-256 A

Publisher-provided hash:
    SHA-256 B

Publisher signature:
    signature object C
```

These must not be collapsed into one generic integrity field.

## Immutability

A preserved SourceArtifact is immutable.

Once committed:

```text
bytes
hash
artifact identity
```

must not be modified in place.

If content must be reacquired or a source publishes a corrected version, Pathfinder creates a new SourceArtifact.

```text
new source version
    != mutate old artifact
```

## Commit State

Artifact preservation must have explicit states.

Initial states:

```text
RECEIVING
PRESERVED
VERIFIED
FAILED
QUARANTINED
```

### RECEIVING

Pathfinder has begun acquisition but has not established a complete durable artifact.

### PRESERVED

The complete artifact was durably stored and its required metadata committed.

### VERIFIED

The stored artifact has successfully passed Pathfinder integrity verification.

### FAILED

Artifact preservation did not complete successfully.

### QUARANTINED

The artifact was preserved but processing has been intentionally restricted because of a security, validation, handling, or policy condition.

```text
QUARANTINED != deleted
```

## No Parsing Before Required Preservation

For source classes governed by this contract, semantic ingestion must not commit authoritative derived intelligence unless required raw preservation succeeded.

Conceptually:

```text
source received
     |
     v
preservation successful?
     |
     +-- NO --> fail / quarantine processing
     |
     +-- YES --> parse
```

This prevents:

```text
derived intelligence exists
but original source was lost
```

for sources requiring preservation.

## Streaming Sources

Some sources may be too large or continuous to treat as one indefinitely growing artifact.

Such collectors must define an explicit segmentation contract.

Possible segmentation boundaries include:

```text
response
page
batch
time window
provider object set
bounded file
```

Each committed segment becomes an immutable SourceArtifact.

Pathfinder must never use:

```text
whatever happened to be in memory
```

as an undocumented preservation boundary.

## Size Limits

Raw-source acquisition operates on untrusted input.

Every source contract must define limits such as:

```text
maximum response size
maximum object size
maximum archive size
maximum extracted size
maximum member count
maximum nesting depth
maximum compression expansion
```

A source exceeding an applicable bound produces an explicit processing state.

It must not cause uncontrolled resource consumption.

## Oversized Source Material

An oversized payload may produce:

```text
SOURCE_TOO_LARGE
```

or another explicitly frozen failure state.

Pathfinder must preserve enough metadata to establish:

```text
source
retrieval event
failure
time
reported/observed size where available
configured limit
```

If full content could not be safely acquired:

```text
full_source_preserved = false
```

must remain explicit.

Pathfinder must not pretend partial content is complete.

## Partial Acquisition

Interrupted acquisition must remain distinguishable from a complete SourceArtifact.

```text
PARTIAL
    != PRESERVED
```

Partial bytes may be retained for diagnostics where policy permits, but they must never receive the semantic status of a complete artifact.

## Parser Failure

If preservation succeeds but parsing fails:

```text
SourceArtifact:
    VERIFIED

Parser:
    FAILED
```

The artifact remains available for:

```text
analysis
parser repair
future reprocessing
forensic review
```

This is one of the primary reasons for raw preservation.

## Unsupported Format

Likewise:

```text
artifact preserved
format unsupported
```

is valid.

```text
unsupported
    != discard source
```

A future Pathfinder version may gain support and reprocess the artifact.

## Reprocessing

Reprocessing always operates from preserved authoritative input where available.

Example:

```text
SourceArtifact A
    |
    +-- parser v1 --> Result Set A
    |
    +-- parser v2 --> Result Set B
```

The original SourceArtifact remains unchanged.

Each processing result retains:

```text
artifact identity
parser identity
parser version
normalizer identity/version
processing time
result state
```

## Malicious Source Content

Threat-intelligence feeds may contain deliberately hostile content.

Examples include:

```text
malformed documents
parser bombs
decompression bombs
path traversal entries
oversized strings
deep nesting
unexpected binary data
crafted URLs
hostile Unicode
embedded scripts
malicious PDFs
```

Preservation does not imply execution or rendering.

Pathfinder must treat source content as data.

## No Active Content Execution

Preservation and parsing must not automatically:

```text
execute scripts
run macros
open embedded programs
resolve URLs
fetch referenced resources
load remote images
execute document actions
invoke shell commands
```

Any future enrichment involving external retrieval is a separate controlled operation.

```text
source contains URL
    != Pathfinder visits URL
```

## Archive Extraction Safety

Archive processing must protect against:

```text
path traversal
absolute paths
device files
symlink escape
hard-link escape
resource exhaustion
recursive archive expansion
```

Extraction destinations must remain within explicitly controlled storage boundaries.

## Content-Type Trust

Pathfinder must not trust declared content type blindly.

Example:

```text
Content-Type: application/json
```

does not prove the content is valid JSON.

Likewise, a `.json` file extension does not establish format.

Declared type and validated type remain separate.

## Character Encoding

Text decoding occurs after byte preservation.

Pathfinder should preserve:

```text
declared charset
detected/validated charset where applicable
decoder used
decoder version/profile
```

A decoding failure does not alter the preserved source bytes.

## Source Markings

Handling restrictions apply to preserved artifacts.

A source marked:

```text
restricted
customer-private
provider-proprietary
TLP-controlled
```

must not become broadly accessible merely because it resides in the raw-source store.

Raw preservation must obey authorization boundaries.

## Encryption at Rest

The implementation must support protection of preserved source content appropriate to its sensitivity and deployment threat model.

Exact cryptographic implementation is deferred to runtime/storage architecture.

However, Phase 0 freezes the requirement that:

> **Raw-source preservation must not create an unprotected archive of sensitive intelligence.**

## Access Control

Access to normalized intelligence does not automatically grant access to raw source material.

These may have different handling requirements.

For example:

```text
Analyst can view:
    normalized Indicator

Analyst cannot view:
    restricted original vendor report
```

may be valid.

Therefore:

```text
normalized-object access
    != raw-artifact access
```

## Secrets

Raw source material may accidentally contain:

```text
API keys
credentials
tokens
private customer data
restricted identifiers
```

Pathfinder must not automatically copy raw source payloads into:

```text
logs
errors
support bundles
metrics
traces
panic output
```

Error handling should reference artifact identity rather than dump source content.

## Logging

Preferred diagnostic pattern:

```text
source_artifact_id = ...
sha256 = ...
byte_length = ...
processing_state = PARSE_FAILED
parser = ...
```

not:

```text
parser failed on payload:
<entire source document>
```

## Storage Corruption

Pathfinder must support verification of preserved artifacts against their recorded integrity digest.

If verification fails:

```text
integrity_state = MISMATCH
```

or equivalent.

The artifact must not silently continue as trusted source input.

Existing derived intelligence must expose that its preservation basis has an integrity problem where relevant.

## Missing Artifact

If metadata references an artifact that is unavailable:

```text
artifact_state = UNAVAILABLE
```

Pathfinder must not represent provenance as complete.

```text
metadata exists
    != raw source available
```

## Backup Is Not Primary Preservation

A backup copy is not the authoritative preservation commit.

Likewise:

```text
database replica
object-store replica
backup
```

does not substitute for successful initial artifact preservation.

Durability requirements belong to Phase 1 storage architecture, but primary preservation success must be explicitly established.

## Retention

Raw SourceArtifacts may have retention policies based on:

```text
source contract
license
handling restrictions
customer policy
operational need
legal hold
storage policy
```

Expiration of derived intelligence does not automatically authorize removal of the SourceArtifact.

```text
Indicator expired
    != raw artifact deletable
```

## Legal or Source Restrictions

A provider may prohibit indefinite retention or redistribution.

Pathfinder must be capable of honoring source-specific handling/retention rules.

If raw content must be destroyed while derived records are permitted to remain, Pathfinder must preserve that historical condition explicitly.

For example:

```text
raw_artifact_state:
    DESTROYED_BY_POLICY

destroyed_at:
    ...

destroy_authority:
    ...

derived provenance:
    retained
```

Pathfinder must not claim the raw source is still available.

## Destruction

Raw-source destruction is a distinct authorized operation.

Destruction requires:

```text
applicable authority
policy basis
artifact identity
destruction time
result
```

Deletion must not occur merely because:

```text
disk is filling
Indicator expired
source was removed
parser no longer uses artifact
artifact is old
```

unless an explicit retention/destruction policy authorizes it.

## Holds

An applicable hold overrides routine destruction eligibility.

```text
retention expired
    +
active hold
    =
do not destroy
```

Exact hold implementation is deferred, but the authority model must accommodate it.

## Source Deletion by Provider

If a provider removes a report after Pathfinder preserved it:

```text
provider no longer hosts source
    != Pathfinder never received source
```

Pathfinder preserves historical provenance subject to its own legal/handling requirements.

## Raw Source and Search

Raw artifacts do not need to become full-text searchable in Phase 1.

Search/indexing is derived functionality.

The authoritative responsibility is preservation and traceability.

```text
artifact preserved
    != artifact indexed
```

## Common Truth Separations

```text
SourceArtifact                    != SourceRecord
SourceArtifact                    != Assertion
preserved bytes                   != parsed representation
re-serialized JSON                != original JSON bytes
transport metadata                != source payload bytes
HTTP response body                != TLS packet history
content hash                      != source authenticity
content hash                      != assertion truth
publisher signature               != Pathfinder preservation hash
PRESERVED                         != VERIFIED
QUARANTINED                       != deleted
partial acquisition               != complete artifact
unsupported format                != disposable source
parser failure                    != source loss
new parser result                 != original source modified
archive extraction                != original archive replaced
declared content type             != validated content type
text decoding                     != byte preservation
source contains URL               != Pathfinder retrieved URL
normalized access                 != raw-source access
Indicator expired                 != artifact destruction authorized
backup exists                     != primary preservation succeeded
metadata exists                   != raw artifact available
provider deleted source           != Pathfinder never received it
artifact preserved                != artifact indexed
```

## Object-Model Refinement

Phase 0.11 adds:

```text
SourceArtifact
```

as a first-class Pathfinder provenance object.

The relevant acquisition chain becomes:

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
normalized intelligence
```

A single SourceArtifact may contain many SourceRecords.

A SourceRecord always remains traceable to the artifact or other explicitly defined preserved source basis from which it was derived.

## Phase 0.11 Exit Decision

Phase 0.11 is satisfied when Pathfinder accepts that:

1. `SourceArtifact` is a first-class UUIDv7 provenance object.
2. Pathfinder preserves exact acquired source bytes before semantic parsing where preservation is required.
3. Pathfinder does not claim transport packet/frame preservation as part of this contract.
4. Reconstructed or re-serialized content never substitutes for original source bytes.
5. Each artifact receives a Pathfinder-calculated SHA-256 integrity digest.
6. External signatures and hashes remain separate from Pathfinder's preservation digest.
7. SourceArtifacts are immutable after commit.
8. New or corrected source content creates a new artifact rather than modifying an old one.
9. `SourceArtifact` and `SourceRecord` remain distinct.
10. Logical SourceRecords retain deterministic locators into parent artifacts where practical.
11. Exact per-record reconstructed bytes are not falsely represented as original source bytes.
12. Parsing required for authoritative derived intelligence occurs only after required source preservation succeeds.
13. Streaming or large sources use explicit bounded segmentation contracts.
14. Oversized, partial, malformed, unsupported, and quarantined inputs remain explicit states.
15. Successful preservation survives later parser failure.
16. Reprocessing creates new processing lineage against unchanged preserved artifacts.
17. Source material is treated as untrusted data and is not automatically executed, rendered, or externally dereferenced.
18. Archive extraction and decoding are bounded processing steps.
19. Source handling restrictions survive raw preservation.
20. Raw-source access may be more restrictive than normalized-intelligence access.
21. Raw source content is not copied into diagnostic output by default.
22. Integrity verification failure is explicit and affects provenance completeness.
23. Raw-source retention and destruction remain separate from intelligence lifecycle.
24. Destruction requires explicit authority and policy basis.
25. Holds override normal destruction eligibility.
26. Pathfinder accurately reports when raw source has been destroyed, is unavailable, or was never fully preserved.
