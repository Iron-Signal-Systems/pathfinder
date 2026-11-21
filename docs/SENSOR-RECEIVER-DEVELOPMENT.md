# Pathfinder development sensor receiver: TLS custody boundary

Status: **development only**. This receiver preserves validated batches in local
immutable custody. It does **not** provide PostgreSQL observation indexing,
historical queries, Sighting derivation, full ingestion acceptance, or a sensor
delivery checkpoint. Do not point the running UDM-Pro sensor at this endpoint.

## Build / tests

From the repository's go directory:

```sh
go test ./...
go vet ./...
go test -race ./internal/sensorreceiver ./internal/sensoringest
go build -trimpath -o /home/jwood/bin/pathfinder-sensor-receiver ./cmd/sensor-receiver
```

The build does not install or start the receiver in any runtime jail.

## TLS and authorization

The standalone receiver requires:

- TLS 1.3
- a dedicated sensor-client CA (not a general enterprise CA)
- a valid client certificate with client-auth EKU
- an explicitly registered client **URI SAN** in a local identity map
- a stable, authorized logical sensor ID bound to that URI SAN
- an exact match between the request's claimed sensor ID and the certificate
  authorization

Identity map example (not a credential, not a real deployment identity):

```json
{
  "spiffe://pathfinder.example/sensor/sensor-a": "sensor-a"
}
```

The server's private TLS key, certificate, dedicated client trust CA, and
identity map are supplied by file paths. No certificate provisioning, revocation
service, or runtime installation automation is provided by this development
stage. Configure these only in a disposable isolated test environment.

Example invocation with explicitly provisioned test credentials:

```sh
pathfinder-sensor-receiver \
    -listen 127.0.0.1:9443 \
    -server-cert /private/test-server.crt \
    -server-key /private/test-server.key \
    -client-ca /private/test-sensor-ca.crt \
    -identities /private/test-sensors.json \
    -custody-dir /private/test-custody
```

The listener refuses wildcard addresses; it defaults to the loopback interface
and does not create PF rules, publish an address, or update the UDM-Pro sensor.

## API

Endpoint: POST /v1/sensor/batches

Required Content-Type: application/x-ndjson

Required headers:

- X-Pathfinder-Sensor-ID: claimed identity; must equal mTLS-authorized identity
- X-Pathfinder-Stream: observations, conntrack, identity, or health
- X-Pathfinder-Schema-Version: current stream version (5/1/1/2 respectively)
- X-Pathfinder-Batch-ID: stable sender-generated retry identity

Body: exact newline-terminated, uncompressed UTF-8 JSONL records;
the body may contain multiple records **from one stream and schema only**.

Bounds:

- maximum 8 MiB raw JSONL per batch
- maximum 25,000 records per batch
- maximum individual record 4 MiB
- maximum 8 concurrent in-handler requests
- bounded headers/read/write timeouts

Responses:

- 201 with status=preserved: verified and durably placed in file custody
- 200 with status=already_preserved: identical batch is already in custody
- 401/403 for unauthenticated or unauthorized sensor claims
- 409 for batch ID reuse with conflicting content
- 413 for oversized HTTP payload
- 415 for unsupported body content type
- 422 for malformed batch, invalid envelope, or unsupported schema
- 503 for unavailable custody or receiver concurrency saturation

Every successful receipt explicitly includes `"indexed": false`. This is
intentional: custody preservation is not completed ingestion. The sensor must
**not** advance a final indexed-delivery checkpoint based on these responses.
Both initial and duplicate custody results can be replayed safely.

## Acceptance still outstanding

- Complete record-field validation beyond current structural guards
- Transactional PostgreSQL sensor observation schema and reliable indexing
- Source/receipt/processing history and query APIs for all four streams
- A documented durable ingest/acknowledgement boundary, including DB outages
- Physical receiver restart/crash injection and full FreeBSD jail validation
- Revocation policy and certificate lifecycle/rotation handling
- Sender-side delivery queues, batching, retries, checkpoints, and monitoring
- Tested end-to-end historical query and versioned Sighting derivation

No authority is given to the sensor to write the database directly.
