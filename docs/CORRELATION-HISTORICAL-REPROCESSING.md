# Pathfinder Correlation and Historical Reprocessing

## Purpose

This document defines how Pathfinder should connect threat intelligence with historical observations without turning a match into an unsupported conclusion.

The central rule is:

> **A match tells Pathfinder that records intersect. Applicability and Assessment determine what that intersection means.**

## Correlation Is Not Judgment

Suppose a sensor observed:

```text
endpoint:      Living-Room-2
platform:      Apple TV / tvOS context
time:          T1
TLS SNI:       example.net
destination:   203.0.113.20:443
```

Later a source reports:

```text
observable:          example.net
campaign:            Campaign X
role:                command_and_control
affected platform:   Roku
valid from:          T2
```

Pathfinder can establish:

```text
Observable match: YES
```

It cannot immediately establish:

```text
the Apple TV session was C2
```

The source assertion, Sighting, correlation, applicability result, and Pathfinder Assessment are separate records.

## Infrastructure Role Must Be Attributable

Pathfinder does not infer that infrastructure was C2 merely because:

```text
an IP appears on a bad list
a domain is unusual
traffic is periodic
an address appeared in a campaign report
a connection used TLS
a source used the word malicious somewhere nearby
```

A role such as:

```text
command_and_control
payload_delivery
redirector
phishing
shared_hosting
compromised_infrastructure
legitimate_service
```

must come from an attributable source assertion, an explicitly governed Pathfinder Assessment, or another versioned analytical process that records its basis.

If Source A says:

```text
203.0.113.20 was C2 for Campaign X
```

Pathfinder records that Source A made that assertion.

It still does not mean every connection to the IP was a C2 session.

## Correlation Types

Initial correlation should favor conservative, explainable matches.

Examples:

```text
normalized domain == normalized domain
IPv4 == IPv4
IPv6 == IPv6
SHA-256 == SHA-256
certificate fingerprint == certificate fingerprint
URL == URL under the applicable canonicalization profile
```

More complex relationships may be introduced only with explicit semantics.

Do not silently equate:

```text
domain resolves to IP
    with
domain is identical to IP

shared IP
    with
same service

same ASN
    with
same operator

same certificate
    with
same campaign
```

## Applicability Dimensions

After correlation, Pathfinder evaluates the dimensions actually supported by available information.

Potential dimensions include:

```text
observable match
time
platform
product/software
version
campaign
malware family
infrastructure role
geography
service/technology
observed behavior
source validity/lifecycle
conflicting intelligence
```

Each dimension must support uncertainty.

Recommended conceptual results:

```text
MATCH
NO_MATCH
UNKNOWN
NOT_APPLICABLE
CONFLICTED
```

A final implementation may use more precise typed states, but it must not collapse unknown into match/no-match.

### Time

If a source reports that infrastructure served a campaign from September 20 onward, an observation on September 14 is not automatically applicable.

If the source does not provide a reliable time window:

```text
time applicability = UNKNOWN
```

not `MATCH`.

### Platform

If a campaign is reported as Roku-specific and the observed endpoint is an Apple TV:

```text
platform applicability = NO_MATCH
```

unless other intelligence expands the affected scope.

Unknown endpoint platform remains `UNKNOWN`.

### Infrastructure Role

A source may report an IP as:

```text
shared CDN
C2
payload delivery
redirector
compromised host
```

The role matters.

A shared service can appear in both legitimate and malicious activity.

Pathfinder should preserve the role, campaign, time, and source context rather than reducing the IP to `bad=true`.

### Behavior

Observed behavior may support an Assessment, but behavior alone should be described accurately.

Example:

```text
periodic outbound traffic at a stable interval
```

may support:

```text
behavior consistent with beaconing
```

It does not automatically establish:

```text
confirmed C2
```

## Assessment

An external source judgment remains an `Assertion`.

A Pathfinder judgment is an `Assessment`.

Example:

```text
Assessment subject:
    Sighting S-123

Assessment:
    historical threat-intelligence match is operationally relevant

Basis:
    domain exact match
    campaign time window match
    affected platform match
    source asserts C2 role
    no unresolved contradictory source at higher applicable scope

Authority:
    PATHFINDER_PROCESS

Confidence:
    recorded on this Assessment
```

The Assessment must reference the records and processing version that support it.

## Historical Reprocessing

Historical reprocessing applies current or newly received intelligence to previously preserved Sightings.

Typical triggers:

```text
new source acquired
new Assertion created
source correction
source revocation
new campaign relationship
new affected platform
changed campaign window
new infrastructure role
new conflict
new Observable normalization version
approved correlation/applicability algorithm change
```

The flow is:

```text
new/changed intelligence
        |
        v
identify candidate historical Sightings
        |
        v
run versioned correlation
        |
        v
run versioned applicability
        |
        v
record ProcessingRecord
        |
        +--> no Assessment change
        |
        +--> new Assessment
        |
        +--> conflict created/updated
```

## Never Rewrite the Past

Historical reprocessing must not mutate:

```text
original SourceArtifact
original SourceRecord
original Assertion
original sensor observation
original Sighting
prior Assessment
prior ProcessingRecord
```

Instead:

```text
old understanding remains historical
new understanding is added
current view is derived
```

This allows Pathfinder to answer:

```text
What did we know on September 14?

What new intelligence arrived on September 28?

Why did the September 14 Sighting become interesting on September 28?

What changed again on October 3?
```

## Synthetic Acceptance Scenario

The development acceptance scenario intentionally uses synthetic intelligence.

Historical observation:

```text
Sighting:
    endpoint:          Living-Room-Test
    platform:          tvOS
    time:              2026-09-14T22:25:43Z
    Observable:        test-stream.example
```

First source assertion:

```text
Source A reports:
    Observable:        test-stream.example
    role:              command_and_control
    campaign:          Pathfinder-Test-Campaign
    affected platform: Roku
    valid from:        2026-09-20
```

Expected:

```text
observable match        = MATCH
platform applicability  = NO_MATCH
time applicability      = NO_MATCH

Assessment:
    source intelligence intersects the Observable,
    but it does not apply to this historical Sighting
```

Later source assertion:

```text
Source A update:
    affected platforms:
        Roku
        tvOS

    valid from:
        2026-09-01
```

Expected reprocessing:

```text
observable match        = MATCH
platform applicability  = MATCH
time applicability      = MATCH

Assessment:
    historical Sighting requires review
```

Every original record remains.

## Real-World Development Motivation

The UDM-Pro sensor demonstrated that Pathfinder can preserve enough network context to reconstruct a communication without full payload capture, including:

```text
historical endpoint identity
destination IP/port
TLS SNI when visible
DNS context when observed
NAT translation
LAN/WAN observation points
return traffic
connection timing/state
sensor health
```

That is sufficient to make future threat-intelligence correlation operationally meaningful while remaining honest about what was not captured.

Full PCAP remains a different authority and can provide payload-level truth Pathfinder sensors do not possess.

## Query Requirements

Pathfinder must eventually be able to answer:

```text
Show historical Sightings for this Observable.

Show the source Assertions that caused this Sighting to be re-evaluated.

Show which applicability dimensions matched, did not match, or remained unknown.

Show the Assessment before and after reprocessing.

Show the processing version that produced each result.

Show unresolved conflicts.

Show whether source/sensor/index coverage was complete.
```

## Current View

The current interpretation is derived.

It must not be selected using a hidden universal rule such as:

```text
newest wins
highest confidence wins
most sources wins
human always wins
machine always wins
```

Where incompatible applicable Assessments remain unresolved, expose conflict rather than inventing certainty.

## Final Rule

> **Pathfinder may change what it currently thinks about an old observation. It may not change what was originally observed or what a source originally said.**
