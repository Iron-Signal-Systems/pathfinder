# Pathfinder Phase 0.3 — Observable Model

## Purpose

Pathfinder requires deterministic rules for representing observables so that the same technical value can be reliably compared across sources without destroying meaningful distinctions in the source material.

An Observable represents something that can be observed, referenced, or matched.

An Observable remains semantically neutral.

```text
observable
    != malicious
    != suspicious
    != benign
    != indicator
    != assessment
    != compromise
```

Pathfinder normalization MUST NOT manufacture threat meaning.

## Governing Principle

> **Normalize representation, not meaning.**

Pathfinder should normalize only where the equivalence is sufficiently well-defined for the applicable observable type.

When equivalence is uncertain, Pathfinder preserves the distinction.

Aggressive convenience normalization is prohibited where it could collapse technically or operationally distinct values.

## Source Representation Versus Canonical Observable

The exact value received from a source belongs to the preserved SourceRecord and associated provenance.

The Observable contains Pathfinder's canonical representation.

Conceptually:

```text
SourceRecord

    "WWW.Example.COM."

          |
          | parse / validate
          v

Observable

    type: domain
    canonical_value: www.example.com
```

Pathfinder preserves that the source actually supplied:

```text
WWW.Example.COM.
```

It does not rewrite the SourceRecord to:

```text
www.example.com
```

This distinction permits later parser or normalization changes without rewriting history.

## Common Observable Model

Every Observable has at least:

```text
observable_id
observable_type
canonical_value
canonicalization_profile
created_at
```

Initial identity direction remains:

```text
observable_id = Pathfinder-controlled UUIDv7
```

The UUID is the object's identity.

The canonical value is not itself the Pathfinder object identifier.

This prevents future canonicalization changes from requiring historical object identities to be rewritten.

## Canonicalization Profiles

Canonicalization behavior must be explicitly versioned.

Initial direction:

```text
pathfinder-observable-v1
```

A canonicalization rule change is an explicit compatibility event.

Do not silently change normalization behavior in place.

Where a future version determines that previously distinct observables are equivalent, Pathfinder must reconcile them explicitly rather than rewriting their historical identities without record.

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

Additional types require explicit definition before implementation.

The initial model intentionally does not attempt to represent every possible STIX Cyber-observable type.

## Validation Outcomes

Source input being processed as a potential Observable may result in:

```text
VALID
MALFORMED
UNSUPPORTED_TYPE
UNSUPPORTED_REPRESENTATION
```

Only `VALID` input creates or resolves a canonical Observable.

Invalid or unsupported source material remains preserved through its SourceRecord and processing history.

Pathfinder must not discard a source record because one field could not become an Observable.

```text
normalization failed
    != source record lost
```

# IPv4 Observable

## Type

```text
ipv4
```

## Accepted Form

Initial Pathfinder IPv4 input accepts conventional dotted-decimal IPv4 addresses consisting of four decimal octets.

Example:

```text
192.0.2.47
```

Each octet must be within:

```text
0-255
```

## Canonical Form

Canonical representation is dotted decimal with no unnecessary leading zeroes.

```text
192.0.2.47
```

## Alternate Numeric Representations

Pathfinder v1 MUST NOT silently interpret alternate IPv4 representations such as:

```text
3232235777
0xC0A80101
0300.0250.0001.0001
192.168.257
```

as equivalent IPv4 observables.

If such a representation is present in source material, preserve the original SourceRecord and report that the representation could not be accepted under the IPv4 v1 observable contract.

The canonicalizer must not guess what a source intended.

## IPv4 Truth Separation

```text
valid IPv4 syntax
    != routable

routable
    != public

public
    != malicious

same IPv4
    != same host over time
```

Address ownership and use are time-dependent intelligence relationships, not properties permanently embedded in the IPv4 Observable.

# IPv6 Observable

## Type

```text
ipv6
```

## Accepted Form

Pathfinder accepts valid IPv6 textual representations.

## Canonical Form

Canonical text follows RFC 5952 representation rules.

For example:

```text
2001:0DB8:0000:0000:0000:0000:0000:0001
```

becomes:

```text
2001:db8::1
```

Canonicalization includes appropriate:

```text
lowercase hexadecimal
leading-zero suppression
zero-run compression
```

The canonical representation does not determine the internal database storage representation.

## Scoped Addresses

Interface-zone identifiers such as:

```text
fe80::1%eth0
```

are not accepted as ordinary IPv6 v1 observables.

The `%eth0` component describes local scope/context rather than globally meaningful IPv6 address identity.

Such source input remains preserved and should be reported as an unsupported representation unless a future scoped-address model is explicitly defined.

## IPv4-Mapped IPv6

An IPv4-mapped IPv6 address remains an IPv6 Observable.

Pathfinder must not silently replace its object type with IPv4 merely because an IPv4 value is embedded within it.

## IPv6 Truth Separation

```text
IPv6 textual forms equivalent
    != separate observables

IPv4-mapped IPv6
    != IPv4 observable identity

IPv6 address
    != endpoint identity

current IPv6 owner
    != historical IPv6 owner
```

# Domain Observable

## Type

```text
domain
```

## Canonical Form

DNS names are canonicalized to a comparison representation.

For ordinary ASCII DNS names:

```text
WWW.Example.COM
```

becomes:

```text
www.example.com
```

A single terminal root dot is removed from the canonical representation:

```text
www.example.com.
```

becomes:

```text
www.example.com
```

The original spelling remains available through SourceRecord provenance.

## Internationalized Domain Names

Internationalized domain names must use defined IDNA processing.

Pathfinder's canonical comparison representation uses the validated ASCII A-label representation.

For example, where valid IDNA conversion applies:

```text
Unicode source representation
       |
       v
validated IDNA processing
       |
       v
ASCII A-label canonical representation
```

The original Unicode form remains preserved through the source record.

Pathfinder MUST NOT perform visual-confusable replacement or assume visually similar Unicode names are the same domain.

```text
visually similar
    != same domain
```

## Domain Rules

Pathfinder must reject malformed label structure rather than repairing it.

Pathfinder does not automatically:

```text
remove arbitrary internal whitespace
repair repeated dots
guess omitted labels
replace Unicode lookalikes
infer a registrable domain
infer organizational ownership
```

Public-suffix or registrable-domain analysis may later create derived intelligence.

It is not domain canonicalization.

## Domain Truth Separation

```text
domain exists
    != malicious

domain resolves
    != domain owner identified

same registrable domain
    != same infrastructure

similar spelling
    != same domain

current DNS
    != historical DNS
```

# URL Observable

## Type

```text
url
```

URL canonicalization is intentionally conservative.

URLs are security-sensitive structured identifiers whose path, query, encoding, and other components may have application-specific meaning.

Pathfinder must not aggressively rewrite them merely to improve deduplication.

## Initial Scheme Scope

Pathfinder v1 initially supports absolute:

```text
http
https
```

URLs.

Additional schemes require explicit future rules because scheme semantics may materially affect comparison.

## Canonicalization

Pathfinder may safely normalize:

```text
scheme -> lowercase
DNS host -> domain canonicalization rules
IPv4 host -> IPv4 canonicalization rules
IPv6 host -> IPv6 canonicalization rules
```

Other components remain conservatively preserved.

Pathfinder v1 does NOT automatically:

```text
remove default ports
add or remove trailing slash
remove dot-segments
reorder query parameters
decode percent-encoded values
re-encode path data
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

does not automatically become equivalent to:

```text
https://example.com/a?y=2&x=1
```

Pathfinder has no basis to assume the target application treats them identically.

Likewise:

```text
https://example.com/a
```

and:

```text
https://example.com/a/
```

remain distinct unless a later explicitly defined analysis determines otherwise.

## URL Parsing

A URL may expose components useful to later Pathfinder processing:

```text
scheme
host
port
path
query
fragment
```

Parsed components do not silently create additional intelligence relationships.

For example, parsing:

```text
https://bad.example/payload
```

reveals the host:

```text
bad.example
```

but creation or association of a corresponding Domain Observable must occur through explicit Pathfinder processing with provenance.

## URL Truth Separation

```text
same host
    != same URL

same path
    != same resource

same parameters
    != same semantics

redirect target
    != original URL

URL observed
    != URL successfully retrieved

URL associated with malware
    != every request malicious
```

# SHA-256 Observable

## Type

```text
sha256
```

## Accepted Form

The value must contain exactly 256 bits represented as 64 hexadecimal characters.

Uppercase and lowercase hexadecimal input are accepted.

## Canonical Form

Canonical representation is lowercase hexadecimal without:

```text
0x prefix
sha256: prefix
spaces
colon separators
```

Example:

```text
ABCDEF...
```

becomes:

```text
abcdef...
```

A source-provided textual prefix such as:

```text
SHA256:
```

belongs to source representation/parsing and is not part of the canonical hash value.

## Hash Meaning

The SHA-256 Observable represents a digest value.

It does not itself establish what object was hashed unless provenance or a relationship supplies that context.

```text
same digest
    != execution

hash present
    != malware

malware-associated hash
    != malware executed locally
```

Pathfinder v1 uses SHA-256 for the initial cryptographic-hash Observable.

MD5 and SHA-1 may later be supported where threat-intelligence interoperability requires them, but they require their own explicit observable types and must not weaken Pathfinder's preferred integrity primitives.

# Email Observable

## Type

```text
email
```

## Initial Scope

Pathfinder v1 represents conventional mailbox addresses in:

```text
local-part@domain
```

form.

Internationalized SMTP local-part support is deferred until an explicit EAI/SMTPUTF8 contract is defined.

## Canonicalization

The domain portion follows Pathfinder Domain canonicalization.

The local-part preserves its original case and meaningful syntax.

For example:

```text
User.Name@EXAMPLE.COM
```

becomes:

```text
User.Name@example.com
```

Pathfinder MUST NOT assume:

```text
User@example.com
```

is the same mailbox as:

```text
user@example.com
```

even if many real-world mail systems happen to treat them that way.

Pathfinder also MUST NOT apply provider-specific convenience rules such as:

```text
removing dots
removing +tags
rewriting aliases
mapping provider domains
lowercasing the local-part
```

For example:

```text
john.smith+test@example.com
```

is not silently collapsed into:

```text
johnsmith@example.com
```

Provider-specific account relationships may later be derived separately.

They are not canonicalization.

## Email Truth Separation

```text
same domain
    != same sender

same display name
    != same mailbox

similar local-part
    != same identity

email address observed
    != sender authenticated

sender authenticated
    != content benign
```

# X.509 Certificate SHA-256 Observable

## Type

```text
x509_certificate_sha256
```

The initial certificate observable is explicitly a SHA-256 fingerprint of the DER-encoded X.509 certificate.

The algorithm and hashed object are part of the type definition.

This avoids ambiguous records such as:

```text
certificate_hash = abc...
```

where Pathfinder cannot determine whether the value represents:

```text
certificate SHA-256
certificate SHA-1
SPKI hash
public-key hash
other certificate-derived material
```

## Accepted Representation

Pathfinder may accept common SHA-256 fingerprint presentation forms including:

```text
AA:BB:CC:...
```

and:

```text
AABBCC...
```

after strict validation.

## Canonical Form

Canonical representation is:

```text
lowercase hexadecimal
64 hexadecimal characters
no colon separators
```

## Certificate Truth Separation

```text
same certificate fingerprint
    != same endpoint

same public key
    != same certificate

certificate valid
    != peer trusted

certificate trusted
    != service benign

certificate associated with threat infrastructure
    != every use malicious
```

A future SPKI fingerprint is a different Observable type.

It must not be represented as an X.509 certificate fingerprint.

# Values Intentionally Deferred

The first observable profile does not yet define canonicalization for:

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

This does not mean these concepts are unimportant.

It means Pathfinder does not create first-class observable semantics until their identity and normalization rules are intentionally defined.

CVE, for example, belongs initially to the Vulnerability object model rather than being treated as a generic Observable.

# Duplicate Observable Behavior

Within one canonicalization profile, Pathfinder should prevent uncontrolled duplicate creation for an exact pair of:

```text
observable_type
canonical_value
```

For example:

```text
IPv4 source representation:
    192.0.2.1

IPv4 source representation:
    192.0.2.1
```

resolve to the same Pathfinder Observable.

Different source records still retain their own provenance.

Therefore:

```text
same Observable
    != same SourceRecord
```

and:

```text
same Observable reported twice
    != two independent sources
```

Deduplication of the Observable must not deduplicate the intelligence history that references it.

# Canonicalization Failure

Canonicalization must be deterministic.

It may return either:

```text
VALID
    canonical representation produced

MALFORMED
    input cannot represent a valid value of this type

UNSUPPORTED_REPRESENTATION
    representation may be meaningful but Pathfinder v1
    does not define safe canonicalization for it

UNSUPPORTED_TYPE
    Pathfinder has no observable contract for this type
```

The canonicalizer MUST NOT return:

```text
best_guess
probably_valid
fixed
close_enough
```

as successful observable states.

If Pathfinder cannot safely determine the canonical value, it does not create one.

The source record remains available.

# No Silent Cross-Type Conversion

Pathfinder does not silently change observable types merely because representations overlap.

Examples:

```text
IPv4-mapped IPv6
    != IPv4

URL host
    != Domain Observable automatically

certificate fingerprint
    != generic SHA-256 automatically

SHA-256
    != Malware

email domain
    != Domain Observable automatically
```

Derived objects or relationships may later connect these concepts through explicit processing.

That processing must preserve its lineage.

# Canonicalization Must Be Pure

Given:

```text
observable_type
input value
canonicalization profile
```

canonicalization must not depend upon:

```text
DNS lookup
WHOIS lookup
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

Canonicalization is deterministic representation processing.

Enrichment and correlation occur later.

Therefore:

```text
canonicalization
    != enrichment
```

and:

```text
canonicalization
    != intelligence assessment
```

# Security Requirements

Observable parsers operate on untrusted input.

Implementations must:

```text
bound input size
validate syntax before transformation
avoid uncontrolled recursion
avoid ambiguous decoding
avoid repeated percent decoding
avoid implicit network access
avoid locale-dependent normalization
avoid hidden provider-specific transformations
return explicit failure state
```

Canonicalization must not invoke external services.

Malformed input must not crash ingestion or cause unrelated source records to be discarded.

# Core Truth Separations

Pathfinder Observable v1 freezes:

```text
raw source value                  != canonical observable
canonicalization                  != enrichment
normalization                     != assessment
observable                        != indicator
observable                        != malicious
observable                        != benign
same observable                   != same source record
same observable                   != independent corroboration
same IP                           != same endpoint over time
same domain                       != same organization
same certificate                  != same service
same hash                         != malware execution
same URL host                     != same URL
similar Unicode                   != same domain
URL parsing                       != URL retrieval
email similarity                  != mailbox identity
certificate SHA-256               != SPKI SHA-256
malformed                         != unsupported
unsupported                       != absent
canonicalization failure          != source-record loss
```

# Phase 0.3 Exit Decision

Phase 0.3 is satisfied when Pathfinder accepts:

```text
ipv4
ipv6
domain
url
sha256
email
x509_certificate_sha256
```

as the initial observable classes and freezes the following rules:

1. Observable values remain semantically neutral.
2. Exact source representation remains preserved independently from canonical representation.
3. Canonicalization is deterministic, versioned, local, and free of enrichment.
4. IPv4 uses strict dotted-decimal representation and does not guess alternate numeric forms.
5. IPv6 canonical text follows RFC 5952.
6. Domain names are case-normalized and use defined IDNA handling without visual-confusable guessing.
7. URL normalization is intentionally conservative.
8. SHA-256 is represented as lowercase 64-character hexadecimal.
9. Email local-part case is preserved while the domain is canonicalized.
10. X.509 certificate SHA-256 and generic SHA-256 values remain different observable types.
11. Malformed and unsupported representations remain distinguishable.
12. Failed canonicalization never causes loss of the original SourceRecord.
13. Duplicate canonical Observables do not erase independent source/provenance history.
14. No cross-type relationship is silently inferred during canonicalization.
