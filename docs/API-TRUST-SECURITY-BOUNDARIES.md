# Pathfinder Phase 0.15 — API Trust and Security Boundaries

## Purpose

Pathfinder exposes intelligence through APIs and receives intelligence from integrations. Those interfaces are security boundaries.

> **Authentication establishes identity. Authorization establishes permitted action. Neither establishes intelligence truth.**

Pathfinder keeps separate:

```text
identity
authentication
authorization
data authority
intelligence authority
administrative authority
export authority
enforcement authority
```

## API Boundary

Every API remains an untrusted-input boundary even when the caller is authenticated. Authenticated clients can still submit malformed, oversized, stale, replayed, conflicting, unsupported, or semantically invalid data.

```text
authenticated != valid
authorized != correct
trusted integration != infallible integration
```

## Initial Authority Classes

Pathfinder distinguishes at least:

```text
READ
INGEST
ANALYST_WRITE
EXPORT
ADMINISTRATION
INTEGRATION
```

READ does not automatically include raw-source access, export, analyst authority, or administration. INGEST does not bypass provenance, preservation, validation, Assertion semantics, or conflict handling. EXPORT remains separate from READ. ADMINISTRATION remains distinct from analyst authorship. INTEGRATION authority is narrow and domain-specific.

## Principal Identity

Every material authenticated action resolves to an attributable principal.

Initial principal classes:

```text
HUMAN
SERVICE
INTEGRATION
SYSTEM_PROCESS
```

Pathfinder preserves the logical principal separately from individual credentials so credential rotation does not erase identity continuity.

```text
credential != logical principal
```

## Human and Service Identity

Human users should use centrally governed identity where practical, such as enterprise SSO/OIDC/AD-backed identity. Service principals remain non-human and must not masquerade as human analysts.

Machine-created Assessments remain machine-created Assessments.

## Integration Identity

Each integration receives its own identity and can be revoked independently.

```text
Stronghold -> integration principal A
FI         -> integration principal B
Atlas      -> integration principal C
```

A single universal ISS service identity should be avoided.

## Least Privilege and Product Authority

> **A trusted integration may speak authoritatively only within its defined domain.**

Stronghold may submit defined network observations but cannot silently create Pathfinder attribution or malware conclusions. FI may submit defined file/process observations but cannot silently assert compromise. Atlas may provide asset/environment context but does not become threat-intelligence authority.

Authentication does not transfer domain authority.

## Machine-to-Machine Authentication

Preferred direction for machine integrations is mTLS with certificate-based identity. Pathfinder validates chain, trust root, validity, intended usage, configured identity binding, and applicable revocation/status policy.

```text
valid client certificate != valid submitted intelligence
```

Transport trust should be narrow enough that possession of an unrelated enterprise certificate does not automatically grant Pathfinder integration access.

## Credential Protection

Secrets, passwords, bearer tokens, API keys, private keys, authorization headers, session cookies, and OTP secrets must not enter ordinary provenance, logs, traces, or support output. Configuration should reference protected credential material rather than embed plaintext where avoidable.

## Authorization

Authorization should be action-oriented and resource-scoped where appropriate.

Potential permissions include:

```text
read_normalized_intelligence
read_raw_source
submit_source_material
submit_sighting
create_assessment
review_conflict
change_lifecycle
export_intelligence
configure_source
configure_integration
administer_security
```

Broad `write_everything` style authority should be avoided.

## Raw Source Access

Raw SourceArtifact access remains separately authorizable from normalized intelligence access because raw material may contain provider-restricted, customer-private, partner-restricted, or otherwise sensitive information.

```text
read normalized != read raw source
```

## Marking and Handling Enforcement

Handling restrictions are enforced server-side and cannot depend only on UI filtering. Export/read decisions must respect source markings, customer boundaries, and redistribution restrictions.

## Write Validation

Authorization answers whether a principal may attempt an action. Validation answers whether the proposed data is valid under Pathfinder's contracts. Both are required.

An authorized relationship writer still cannot create structurally invalid semantics such as `ThreatActor resolves_to Malware`.

## Immutable History

API write authority does not permit editing immutable historical truth in place.

The API must not mutate SourceArtifact bytes, historical SourceRecords, original Assertions, or historical Sightings. Corrections use new records, supersession, revocation, disputes, or Assessments as defined by the governing contract.

## No Generic PATCH of Truth

Pathfinder should avoid generic mutation such as:

```text
PATCH /object
{
  "malicious": true,
  "confidence": 95,
  "actor": "APT-X"
}
```

when those fields represent different authorities and historical claims.

Prefer domain-specific operations:

```text
create Assessment
record Assertion
record Relationship
record Sighting
supersede Assessment
```

## API Versioning

API semantics are versioned. Breaking semantic changes require an explicit version boundary so existing clients are not silently given new meanings for confidence, lifecycle, relationships, sightings, or source semantics.

## Idempotency and Replay

Mutation paths support idempotency where duplicate delivery is expected. Sensitive machine-to-machine writes must explicitly address replay risk through bounded mechanisms such as request identity, timestamp, nonce, signatures, or idempotency identity.

## Bounds and Resource Controls

Every endpoint bounds untrusted input: request size, object count, strings, arrays, nesting, relationship count, batch size, query complexity, time ranges, pagination, and equivalent resource controls.

Read APIs must also be bounded so expensive queries or exports cannot starve source preservation, durable commits, or security-critical processing.

## Rate Limits and Errors

Rate limiting, authorization denial, unavailability, validation failure, and zero-result queries remain distinct conditions. Known validation failures should return explicit structured errors rather than generic internal-server errors.

Errors must not echo entire SourceArtifacts, secrets, authorization headers, restricted reports, or private customer context. Prefer request/artifact/source record identifiers for correlation.

## Audit

Material actions are auditable, including authentication failures, authorization denials, source/integration configuration, raw-source access, Assessment creation, conflict resolution, lifecycle changes, export, and security administration.

Material mutation audit should preserve who, what, target, when, result, request identity, and applicable authorization context.

## Export Boundary

Export is a separate security-sensitive operation. Authorization evaluates caller authority, source restrictions, customer boundaries, markings, export profile, and requested format.

Read authority does not imply export authority, and bulk export may require stronger authorization than routine reads.

```text
can read != can export
```

## Source and Analyst Impersonation Prevention

A source collector may write only within its configured Source/SourceCollection context. A Vendor-A collector cannot submit intelligence as CISA unless explicitly authorized.

Likewise, analyst-created judgments must be represented as analyst Assertions/Assessments and cannot masquerade as external source claims.

## Internal Process Authority

Pathfinder internal processes may create derived Relationships, machine Assessments, conflict detections, and lifecycle events only under versioned processing contracts. Running inside Pathfinder does not create unlimited authority.

## Administrative Separation

Security administration, intelligence analysis, and system operation should remain logically separable. A TLS administrator does not automatically become an intelligence author, and an analyst does not automatically control PKI trust.

## Break-Glass

If emergency administrative access is implemented, it must be explicit, narrow, strongly authenticated, time-bounded where practical, and fully audited. Emergency access must not erase attribution.

## Integration Failure and Coverage

Loss of integration authentication/authorization must not be interpreted as an absence of observations.

```text
Stronghold authentication failed != no Stronghold observations occurred
```

Instead, Pathfinder records a collection/coverage gap.

## Revocation and Rotation

Human, service, integration, source, and export authority can be revoked without deleting historical actions. Credential rotation preserves logical principal continuity unless the logical identity itself changes.

## No Trust by Network Location or Product Name

Pathfinder never grants authority merely because a request originates from localhost, an internal subnet, a management VLAN, a VPN, or RFC1918 space. Nor does caller-supplied metadata such as `product=FI` or `User-Agent: Stronghold` establish identity.

```text
inside network != trusted principal
caller says Stronghold != authenticated Stronghold identity
```

## No Hidden Backdoor

Production must not contain secret query parameters, debug bypasses, hard-coded master tokens, localhost authorization bypasses, or development passwords that override normal security policy.

## Secure Defaults

Initial defaults should be deny-by-default with no anonymous write, no anonymous raw-source access, no automatic TAXII publication, no wildcard integration permission, no source impersonation, and no arbitrary relationship write.

If authorization cannot be established, security-sensitive write/export/admin operations fail closed.

## Enforcement Boundary

No Pathfinder API permission turns intelligence into direct Stronghold blocking, endpoint isolation, account disablement, file deletion, or equivalent remediation without a separate downstream action contract.

> **High confidence is still not authorization.**

## Truth Separations

```text
authentication          != authorization
authorization           != semantic validity
authenticated caller    != trustworthy intelligence
trusted integration     != unlimited authority
integration identity    != analyst identity
administrator           != analyst
read access             != raw-source access
read access             != export access
network location        != authentication
product name            != product identity
valid certificate       != valid intelligence
source ingestion        != analyst assessment
integration observation != Pathfinder conclusion
credential              != logical principal
API write               != mutable history
bulk export             != ordinary read
high confidence         != enforcement authority
```

## Phase 0.15 Exit Decision

Phase 0.15 is satisfied when Pathfinder treats APIs and integrations as explicit security boundaries; keeps authentication, authorization, product authority, analyst authority, export, administration, and enforcement separate; assigns attributable human/service/integration/system principals; uses narrow least-privilege integration contracts; protects credentials; validates every write; preserves immutable history; versions API semantics; bounds requests and queries; audits material security actions; fails closed for sensitive operations when authority cannot be established; and prevents any API role from silently becoming downstream enforcement authority.
