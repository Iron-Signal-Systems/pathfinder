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
