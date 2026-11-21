# Pathfinder sensor ingestion: frozen-capture validation

Status: **development only**. This is a read-only validation stage, **not** a deployed receiver or durable ingestion service.

## Source contracts

These are grounded in the UDM-Pro sensor implementation and the November 20, 2026 schema inventory:

| Stream | Record type | Schema version | Sensor identifier carried in record |
| --- | --- | --- | --- |
| observations | packet/DNS/TLS metadata, no record_type | 5 | sensor_id |
| conntrack | conntrack_event | 1 | no |
| identity | endpoint_identity_snapshot | 1 | no |
| health | sensor_health | 2 | no |

These four streams remain independent evidence. Identity snapshots, conntrack state, and health are not packet observations. A conntrack ID is not a stable globally unique event identity. Sensor observations remain separate from threat intelligence assertions and from derived Sightings.

## Checks

From the go directory:

```sh
go test ./...
go vet ./...
```

For a *frozen* sensor archive copied to the development jail, with an account that has read permissions:

```sh
go run ./cmd/sensor-audit \
  -root /mnt/udm-pro-20261120-0500EST \
  -since 2026-11-20T10:00:00Z \
  -until 2026-11-20T10:55:00Z
```

The command validates all JSONL records, including those outside the time window, then reports per-file counts and original stored-file SHA-256 digests. It accepts .jsonl.gz and captured .jsonl.active snapshots, uses bounded scan buffers, and fails on malformed records, unsupported schema versions, and an incomplete active trailing line. The report intentionally omits actual IP addresses, endpoint names, and packet details. Optional -expected-sensor-id only compares packet-observation sensor_id values; it does not authenticate the sender.

## Unfinished acceptance requirements

- Authenticated sensor identity and authorization, especially for the three streams that lack an embedded sensor_id
- Transport delivery with strict versioning, batch identity, resource limits, identity-bound source provenance, and specific rejection categories
- Durable preservation and a commit/acknowledgement checkpoint that survives an uncertain response and retry
- Exact duplicate handling without suppressing distinct observations
- Historical indexed sensor observation store, separate from the threat intelligence source store
- Retention of capture-health/coverage and traceable, versioned Sighting derivation
- Database and transport outage, replay, schema version, and malicious input acceptance tests

The audit never writes to PostgreSQL, never acknowledges batches, never records a checkpoint, and never changes the UDM-Pro sensor. The copied source files are read-only; several are root:wheel mode 0640. Do not weaken source-file permissions or change the source data.

The previously reported 599,984 records were for a filtered period. File totals include events outside that period and must not be confused with the filtered count.


## November 20 frozen-capture audit result (2026-11-21)

The C220 read-only audit built successfully with FreeBSD Go 1.26 and passed
go test ./..., go vet ./..., and checks of all nine captured files.

Window: [2026-11-20T10:00:00Z, 2026-11-20T10:55:00Z).

- observations: 554,863
- conntrack: 45,009
- identity: 56
- health: 56
- total in window: 599,984
- total in the nine unfiltered files: 685,218

These counts reconcile with the earlier frozen-data inspection. This is
structural validation only, not evidence of network delivery or SoR indexing.

## Experimental immutable batch custody (2026-11-21)

The dev branch now includes go/internal/sensoringest/custody.go:

- bounded JSONL batch <= 8 MiB and <= 25,000 records
- stable claimed sensor ID, stream, batch ID, schema version, exact payload bytes
- validation before storage; unknown schema versions and malformed input rejected
- SHA-256 of original JSONL bytes
- fsync of temporary file and directory; atomic no-overwrite link into a private directory
- duplicate delivery with identical payload returns already_preserved
- reuse of a batch identity with different payload returns a conflict
- concurrent retries and collision/error paths covered by unit tests

**Not a production server ingestion implementation.** The API's SensorID is
caller-supplied, so an authenticated transport boundary must bind the caller to
that identity before it can be trusted. This custody package does not create
network listeners, acknowledgements, DB indexes, processing checkpoints,
Sightings, or Assessments; it is a foundation for their implementation.
A custody receipt must never be represented as full sensor ingestion acceptance.

The development tests have also been executed locally with go test -race,
but FreeBSD/jail acceptance and restart/crash fault injection remain pending.
