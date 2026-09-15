# Pathfinder Documentation

Pathfinder documentation is divided into two categories:

```text
CURRENT IMPLEMENTATION DIRECTION
    what the project is building now

SEMANTIC / HISTORICAL CONTRACTS
    the preserved design contracts that define meaning,
    provenance, authority, history, and failure behavior
```

A roadmap reset does not erase the semantic contracts that have already been completed.

## Current Implementation Direction

Start here:

1. [`../README.md`](../README.md) — product overview and current operating model
2. [`../ROADMAP.md`](../ROADMAP.md) — current implementation sequence
3. [`OPERATIONAL-INTELLIGENCE-ARCHITECTURE.md`](OPERATIONAL-INTELLIGENCE-ARCHITECTURE.md) — threat intelligence + observation + interpretation architecture
4. [`SENSOR-OBSERVATION-INGESTION.md`](SENSOR-OBSERVATION-INGESTION.md) — intended sensor-to-server boundary
5. [`CORRELATION-HISTORICAL-REPROCESSING.md`](CORRELATION-HISTORICAL-REPROCESSING.md) — correlation, applicability, Assessment, and historical reprocessing

The current implementation goal is:

```text
threat source
  -> preserved Assertion
  -> normalized Observable
  -> sensor observation
  -> Sighting
  -> correlation
  -> applicability
  -> Assessment
  -> historical reprocessing
  -> explainable query result
```

## Semantic Foundation

The Phase 0 contracts remain the semantic foundation unless a later committed reconciliation explicitly refines them.

```text
INTELLIGENCE-MISSION.md
CORE-INTELLIGENCE-MODEL.md
OBSERVABLE-MODEL.md
ASSERTION-ASSESSMENT-MODEL.md
SOURCE-PROVENANCE-MODEL.md
CONFIDENCE-RELIABILITY-CORROBORATION.md
RELATIONSHIP-MODEL.md
SIGHTING-MODEL.md
INTELLIGENCE-LIFECYCLE-MODEL.md
CONFLICTING-INTELLIGENCE.md
RAW-SOURCE-PRESERVATION.md
STIX-2.1-INTEROPERABILITY.md
TAXII-2.1-INTEROPERABILITY.md
MITRE-ATTACK-RELATIONSHIP-MODEL.md
API-TRUST-SECURITY-BOUNDARIES.md
ISS-PRODUCT-INTEGRATION-BOUNDARIES.md
ANALYST-OVERRIDE-REVIEW-CHANGE-HISTORY.md
AUDIT-ENGINEERING-COMPLETENESS.md
PHASE-0-RECONCILIATION-EXIT.md
```

The governing semantic principles include:

```text
preserve what was received before deciding what it means
record the claim before judging the claim
Observable is neutral
Sighting is observation, not conclusion
Assertion is attributable claim
Assessment is Pathfinder judgment
correlation is not identity
association is not attribution
current knowledge does not rewrite historical knowledge
confidence is not authorization
```

## Completed Implementation Records

Completed Phase 1 implementation records document what was actually built and accepted.

Examples include:

```text
PHASE-1.1-PLATFORM-FOUNDATION.md
PHASE-1.2-SOURCE-FOUNDATION.md
PHASE-1.3-SOURCE-PRESERVATION.md
PHASE-1.4-FIRST-EXTERNAL-COLLECTOR.md
PHASE-1-FRESH-INSTALL-ACCEPTANCE.md
```

These remain historical implementation records even when later roadmap priorities change.

## Sensor Documentation

Sensor-specific documentation lives with each sensor.

Current sensor:

[`../sensors/unifi/udm-pro/README.md`](../sensors/unifi/udm-pro/README.md)

The sensor's local ring and diagnostic query tools are not the intended primary organization-wide historical query path. Server-side sensor ingestion and indexing are defined by the current implementation architecture.

## Documentation Precedence

When documents appear to conflict:

1. a narrowly scoped later committed reconciliation governs the scope it explicitly refines;
2. current `ROADMAP.md` governs implementation sequence;
3. current operational architecture documents govern the implementation boundaries they define;
4. Phase 0 reconciliation governs conflicts among earlier Phase 0 semantic wording;
5. historical phase-completion documents remain authoritative for what was built and accepted at that time.

Do not silently rewrite historical completion records to make them appear to have anticipated later design decisions.

## Product Boundary

Pathfinder remains a threat-intelligence and operational-correlation system.

Its sensor observation plane exists to make threat intelligence operational.

It is not intended to become a general-purpose SIEM, packet-capture authority, EDR, SOAR platform, asset authority, or enforcement system.
