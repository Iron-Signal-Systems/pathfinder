# Pathfinder Phase 0.3 — Observable Model

## Purpose

Pathfinder requires deterministic rules for representing Observables so the same technical value can be compared across sources without destroying meaningful distinctions in the original source material.

An Observable represents something that can be observed, referenced, or matched.

An Observable remains semantically neutral.

```text
Observable
    != malicious
    != suspicious
    != benign
    != Indicator
    != Assessment
    != compromise
```

Pathfinder normalization MUST NOT manufacture threat meaning.

## Governing Principle

> **Normalize representation, not meaning.**

Pathfinder normalizes only where equivalence is sufficiently defined for the applicable Observable type.

When equivalence is uncertain, Pathfinder preserves the distinction.

Aggressive convenience normalization is prohibited where it could collapse technically or operationally different values.

## Source Representation Versus Canonical Observable

The exact acquired byte representation belongs to the preserved `SourceArtifact`.

The logical source item and deterministic location of the value belong to the associated `SourceRecord` and its provenance.

The Observable contains Pathfinder's canonical representation.

Conceptually:

```text
SourceArtifact
    exact acquired bytes
        ↓
SourceRecord
    logical item + locator
        ↓
source representation
    "WWW.Example.COM."
        ↓
parse / validate / canonicalize
        ↓
Observable
    type: domain
    canonical_value: www.example.com
```

Pathfinder preserves that the source supplied `WWW.Example.COM.` by retaining the immutable SourceArtifact plus SourceRecord locator/provenance.

It does not rewrite either historical source record or artifact to `www.example.com`.

This permits later parser or normalization changes without rewriting history.

## Common Observable Model

Every Observable has at least:

```text
observable_id
observable_type
canonical_value
canonicalization_profile
created_at
```

Initial identity direction:

```text
observable_id = Pathfinder-controlled UUIDv7
```

The UUID is the object identity. The canonical value is not the Pathfinder object identifier.

This prevents later canonicalization changes from requiring historical identities to be rewritten.

## Canonicalization Profiles

Canonicalization behavior is explicitly versioned.

Initial direction:

```text
pathfinder-observable-v1
```

A canonicalization rule change is a compatibility event.

Where a future profile determines that previously distinct Observables are equivalent, Pathfinder reconciles them explicitly and preserves historical identities and processing lineage.

## Initial Observable Types

Pathfinder v1 begins with:

```text
ipv4
ipv6
domain
url
sha256
email
x509_certificate_sha256
```

Additional types require explicit identity, validation, and canonicalization rules before implementation.

The initial model intentionally does not attempt to support every STIX Cyber-observable type.

## Validation Outcomes

Potential Observable input processing returns an explicit outcome such as:

```text
VALID
MALFORMED
UNSUPPORTED_TYPE
UNSUPPORTED_REPRESENTATION
```

Only `VALID` produces or resolves a canonical Observable.

Invalid or unsupported source material remains preserved through SourceArtifact, SourceRecord, and ProcessingRecord history.

```text
normalization failed != source history lost
```

# IPv4 Observable

## Type

```text
ipv4
```

Pathfinder v1 accepts conventional dotted-decimal IPv4 consisting of four decimal octets, each 0 through 255.

Example:

```text
192.0.2.47
```

Canonical form is dotted decimal with no unnecessary leading zeroes.

Pathfinder v1 MUST NOT silently interpret alternate numeric representations such as:

```text
3232235777
0xC0A80101
0300.0250.0001.0001
192.168.257
```

as equivalent IPv4 Observables.

Such source representations remain preserved and return an explicit unsupported/malformed outcome under the v1 contract.

```text
valid IPv4 syntax != routable
routable != public
public != malicious
same IPv4 != same host over time
```

# IPv6 Observable

## Type

```text
ipv6
```

Pathfinder accepts valid IPv6 textual representations and canonicalizes them according to RFC 5952-style representation rules, including lowercase hexadecimal, leading-zero suppression, and zero-run compression.

Example:

```text
2001:0DB8:0000:0000:0000:0000:0000:0001
```

becomes:

```text
2001:db8::1
```

Scoped zone identifiers such as:

```text
fe80::1%eth0
```

are not ordinary IPv6 v1 Observables because the zone identifier is local context rather than global address identity.

IPv4-mapped IPv6 remains an IPv6 Observable; Pathfinder does not silently replace the type with IPv4.

```text
IPv6 textual equivalents != separate Observables
IPv4-mapped IPv6 != IPv4 Observable identity
IPv6 address != endpoint identity
current IPv6 owner != historical IPv6 owner
```

# Domain Observable

## Type

```text
domain
```

Pathfinder canonicalizes ordinary DNS names to lowercase ASCII comparison form and removes one terminal root dot.

Example:

```text
WWW.Example.COM.
```

becomes:

```text
www.example.com
```

Internationalized names use defined IDNA processing and a validated ASCII A-label canonical representation.

The original Unicode/source form remains recoverable through SourceArtifact + SourceRecord provenance.

Pathfinder does not perform visual-confusable replacement or assume visually similar names are the same domain.

Pathfinder does not automatically:

```text
repair malformed label structure
remove arbitrary internal whitespace
repair repeated dots
guess omitted labels
replace Unicode lookalikes
infer registrable domain
infer organizational ownership
```

Public-suffix or ownership analysis is derived intelligence, not canonicalization.

```text
domain exists != malicious
domain resolves != owner identified
same registrable domain != same infrastructure
similar spelling != same domain
current DNS != historical DNS
```

# URL Observable

## Type

```text
url
```

URL canonicalization is intentionally conservative.

Pathfinder v1 initially supports absolute `http` and `https` URLs.

It may safely normalize:

```text
scheme -> lowercase
DNS host -> Domain canonicalization rules
IPv4 host -> IPv4 canonicalization rules
IPv6 host -> IPv6 canonicalization rules
```

Pathfinder v1 does NOT automatically:

```text
remove default ports
add/remove trailing slash
remove dot segments
reorder query parameters
decode or re-encode path/query values
remove fragments
sort parameters
remove duplicate parameters
collapse repeated slashes
convert HTTP to HTTPS
resolve redirects
```

Example:

```text
https://example.com/a?x=1&y=2
```

is not automatically equivalent to:

```text
https://example.com/a?y=2&x=1
```

Parsing a URL may expose scheme, host, port, path, query, and fragment, but component parsing does not silently create additional Observables or Relationships.

```text
same host != same URL
redirect target != original URL
URL observed != URL successfully retrieved
URL associated with malware != every request malicious
```

# SHA-256 Observable

## Type

```text
sha256
```

The value must contain exactly 64 hexadecimal characters representing 256 bits.

Uppercase and lowercase input may be accepted after validation.

Canonical representation is lowercase hexadecimal with no prefix, spaces, or separators.

The SHA-256 Observable represents a digest value; it does not itself establish what object was hashed.

```text
same digest != execution
hash present != malware
malware-associated hash != malware executed locally
```

MD5, SHA-1, SHA-512, and other hash types require their own explicit future Observable contracts.

# Email Observable

## Type

```text
email
```

Pathfinder v1 represents conventional `local-part@domain` mailbox addresses. EAI/SMTPUTF8 local-part semantics are deferred.

The domain follows Domain canonicalization. The local-part preserves case and meaningful syntax.

Example:

```text
User.Name@EXAMPLE.COM
```

becomes:

```text
User.Name@example.com
```

Pathfinder does not apply provider-specific convenience rules such as removing dots, removing `+tags`, rewriting aliases, mapping provider domains, or lowercasing the local-part.

```text
same domain != same sender
same display name != same mailbox
similar local-part != same identity
email observed != sender authenticated
sender authenticated != content benign
```

# X.509 Certificate SHA-256 Observable

## Type

```text
x509_certificate_sha256
```

This type is specifically the SHA-256 fingerprint of DER-encoded X.509 certificate bytes.

It is not an SPKI hash, public-key hash, SHA-1 certificate fingerprint, or generic certificate hash.

Pathfinder may accept conventional presentation forms such as colon-separated uppercase hex after strict validation.

Canonical form is:

```text
lowercase hexadecimal
64 characters
no separators
```

```text
same certificate fingerprint != same endpoint
same public key != same certificate
certificate valid != peer trusted
certificate trusted != service benign
certificate threat-associated != every use malicious
```

A future SPKI fingerprint is a distinct Observable type.

# Values Intentionally Deferred

The initial profile does not yet define canonicalization for:

```text
MD5
SHA-1
SHA-512
file name
file path
process name
registry path
MAC address
Autonomous System Number
CIDR/network prefix
telephone number
user account
cryptocurrency address
JA3/JA4-style fingerprint
mutex
Windows service name
registry key
software package
CPE
CVE
```

This means only that their identity/canonicalization contract is not yet frozen.

CVE belongs initially to the `Vulnerability` model, not a generic Observable.

# Duplicate Observable Behavior

Within one canonicalization profile, Pathfinder should prevent uncontrolled duplicate creation for the exact pair:

```text
observable_type
canonical_value
```

Different SourceRecords may still reference the same Observable with independent provenance.

```text
same Observable != same SourceRecord
same Observable reported twice != two independent sources
```

Deduplication of Observable identity must never deduplicate intelligence history.

# Canonicalization Failure

Canonicalization is deterministic.

It returns an explicit result such as:

```text
VALID
MALFORMED
UNSUPPORTED_REPRESENTATION
UNSUPPORTED_TYPE
```

The canonicalizer must not return successful states equivalent to:

```text
best_guess
probably_valid
fixed
close_enough
```

If Pathfinder cannot safely determine a canonical representation, it does not create one.

# No Silent Cross-Type Conversion

Pathfinder does not silently change Observable types merely because representations overlap.

Examples:

```text
IPv4-mapped IPv6 != IPv4
URL host != Domain Observable automatically
certificate fingerprint != generic SHA-256 automatically
SHA-256 != Malware
email domain != Domain Observable automatically
```

Explicit processing may create additional objects or relationships, but it must preserve lineage.

# Canonicalization Must Be Pure

Given:

```text
observable_type
input value
canonicalization profile
```

canonicalization does not depend on:

```text
DNS lookup
WHOIS/RDAP lookup
network access
current time
threat feed
reputation service
analyst opinion
local asset inventory
Stronghold
FI
Atlas
```

Those are enrichment/correlation activities and occur separately.

# Reprocessing

A later canonicalization profile may process the same historical SourceRecord differently.

The original SourceArtifact, SourceRecord, prior ProcessingRecord, and prior Observable identities remain historical.

New processing creates new lineage rather than rewriting what Pathfinder understood originally.

# Common Truth Separations

```text
SourceArtifact bytes != canonical Observable
SourceRecord representation != canonical Observable
canonicalization != enrichment
canonicalization != threat classification
same Observable != same source
same value != same host/entity over time
valid syntax != maliciousness
normalization failure != source history lost
new canonicalization result != historical interpretation rewritten
```

## Phase 0.3 Exit Decision

Phase 0.3 is satisfied when Pathfinder accepts that:

1. Observable remains semantically neutral.
2. Exact source bytes belong to SourceArtifact; SourceRecord supplies the logical item and locator/provenance path.
3. Observable identity is Pathfinder-controlled UUIDv7 and is separate from canonical value.
4. Canonicalization behavior is deterministic and versioned.
5. Initial v1 Observable types are IPv4, IPv6, Domain, URL, SHA-256, email, and X.509 certificate SHA-256 fingerprint.
6. Unsupported or malformed source representation remains preserved and does not produce guessed canonical data.
7. Canonicalization is pure and performs no network/enrichment lookup.
8. Cross-type conversion is explicit rather than silent.
9. Observable deduplication does not erase source/provenance history.
10. Reprocessing creates new lineage and never rewrites the original source or prior interpretation.
11. **The original record is maintained no matter what.**
