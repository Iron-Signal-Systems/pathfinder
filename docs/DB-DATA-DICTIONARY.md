# Pathfinder SoR Database Data Dictionary

**Scope:** All database tables in embedded SQL migrations 0001–0009, with sensor SoR tables created by migration 0008. **Status: DEVELOPMENT SCHEMA ONLY**; no live Pathfinder PostgreSQL instance has been created or validated on the C220.

**Authoritative source:** `go/cmd/pathfinder/migrations/*.sql`, registered by `go/cmd/pathfinder/migrations.go`. This dictionary documents intended types, nullability, purpose and sensor field mappings; physical DDL is the final authority.

## Data model boundaries

The existing threat-intelligence tables (migrations 0001–0007) are NOT the sensor evidence store. The sensor area has independent identities, captured-file and batch provenance, immutable event/snapshot history and rebuildable connection/NAT correlation. Source custody has exact raw bytes; JSONB retains parsed source values but is not byte-identical to the JSONL files. Only packet observations v5 contain an inline sensor_id; conntrack v1, endpoint identity v1 and health v2 require a batch identity bound to an authenticated sensor.

**Conventions:** Required means SQL NOT NULL or PRIMARY KEY; nullable means absence / unknown may remain SQL NULL. Numeric zero, boolean false and VLAN 0 are valid values, not missing. JSON source strings no_record/not_known must map to NULL when the target type is inet/macaddr/timestamptz, without altering custody. PostgreSQL timestamptz preserves microseconds, so the *observed_at_ns* bigint field separately preserves original nanoseconds. Batch/segment ordinals are one-based; nested array ordinals zero-based. Parent partition PKs include observed_at. All new evidence tables have runtime SELECT/INSERT but not UPDATE/DELETE privileges.

## Logical relationships

```text
sensor --< sensor_segment
sensor --< sensor_batch --1:0..1--> sensor_batch_index_completion
sensor_batch --< sensor_observation --< sensor_dns_question / sensor_dns_record
sensor_batch --< sensor_conntrack_event
sensor_batch --< sensor_identity_snapshot --< sensor_identity_endpoint
sensor_batch --< sensor_health
sensor --< sensor_connection
sensor_connection --< sensor_connection_observation_evidence --> sensor_observation
sensor_connection --< sensor_connection_conntrack_evidence --> sensor_conntrack_event
```

## Column dictionaries

### `pathfinder.schema_migration` — migration 0001

Foundation table defined in migration 1.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `version` | `bigint` | Required | version |
| `name` | `text` | Required | name |
| `sha256` | `character(64)` | Required | sha256 |
| `applied_at` | `timestamp with time zone` | Required | applied at |
| `applied_by` | `text` | Required | applied by |

### `pathfinder.source` — migration 0001

Threat-intelligence provider identity, not a network sensor.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `source_id` | `uuid` | Required | source id |
| `name` | `text` | Required | name |
| `source_type` | `text` | Required | source type |
| `provider_identity` | `text` | Required | provider identity |
| `description` | `text` | Required | description |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.source_collection` — migration 0001

Collection configured under an intelligence source.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `source_collection_id` | `uuid` | Required | source collection id |
| `source_id` | `uuid` | Required | source id |
| `name` | `text` | Required | name |
| `collection_type` | `text` | Required | collection type |
| `external_collection_identifier` | `text` | Required | external collection identifier |
| `transport_type` | `text` | Required | transport type |
| `expected_format` | `text` | Required | expected format |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.retrieval_event` — migration 0001

Source retrieval attempt and result.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `retrieval_event_id` | `uuid` | Required | retrieval event id |
| `source_id` | `uuid` | Required | source id |
| `source_collection_id` | `uuid` | Required | source collection id |
| `started_at` | `timestamp with time zone` | Required | started at |
| `completion_state` | `text` | Required | completion state |
| `completed_at` | `timestamp with time zone` | Required | completed at |
| `operation_result` | `text` | Required | operation result |
| `artifacts_received` | `bigint` | Required | artifacts received |
| `records_reported_state` | `text` | Required | records reported state |
| `records_reported` | `bigint` | Required | records reported |
| `error_class` | `text` | Required | error class |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.source_artifact` — migration 0002

Preserved source evidence metadata for intelligence feeds.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `source_artifact_id` | `uuid` | Required | source artifact id |
| `source_id` | `uuid` | Required | source id |
| `source_collection_id` | `uuid` | Required | source collection id |
| `retrieval_event_id` | `uuid` | Required | retrieval event id |
| `received_at` | `timestamp with time zone` | Required | received at |
| `source_location` | `text` | Required | source location |
| `media_type` | `text` | Required | media type |
| `content_encoding` | `text` | Required | content encoding |
| `byte_length` | `bigint` | Required | byte length |
| `sha256` | `text` | Required | sha256 |
| `preservation_state` | `text` | Required | preservation state |
| `integrity_state` | `text` | Required | integrity state |
| `availability_state` | `text` | Required | availability state |
| `storage_reference` | `text` | Required | storage reference |
| `handling_profile` | `text` | Required | handling profile |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.source_record` — migration 0003

Source-record locator within a preserved intelligence artifact.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `source_record_id` | `uuid` | Required | source record id |
| `source_artifact_id` | `uuid` | Required | source artifact id |
| `source_id` | `uuid` | Required | source id |
| `source_collection_id` | `uuid` | Required | source collection id |
| `external_record_id` | `text` | Required | external record id |
| `source_locator` | `text` | Required | source locator |
| `source_marking` | `text` | Required | source marking |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.processing_run` — migration 0004

Versioned intelligence source-processing run.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `processing_run_id` | `uuid` | Required | processing run id |
| `source_id` | `uuid` | Required | source id |
| `source_collection_id` | `uuid` | Required | source collection id |
| `retrieval_event_id` | `uuid` | Required | retrieval event id |
| `source_artifact_id` | `uuid` | Required | source artifact id |
| `process_name` | `text` | Required | process name |
| `process_version` | `text` | Required | process version |
| `started_at` | `timestamp with time zone` | Required | started at |

### `pathfinder.processing_event` — migration 0004

Outcome/state event for an intelligence processing run.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `processing_event_id` | `uuid` | Required | processing event id |
| `processing_run_id` | `uuid` | Required | processing run id |
| `event_type` | `text` | Required | event type |
| `error_class` | `text` | Required | error class |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.source_record_processing` — migration 0004

Per-source-record processing result.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `source_record_processing_id` | `uuid` | Required | source record processing id |
| `processing_run_id` | `uuid` | Required | processing run id |
| `source_record_id` | `uuid` | Required | source record id |
| `source_artifact_id` | `uuid` | Required | source artifact id |
| `source_collection_id` | `uuid` | Required | source collection id |
| `source_id` | `uuid` | Required | source id |
| `validation_state` | `text` | Required | validation state |
| `processing_state` | `text` | Required | processing state |
| `processing_error_class` | `text` | Required | processing error class |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.vulnerability` — migration 0005

Canonical CVE identity.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `vulnerability_id` | `uuid` | Required | vulnerability id |
| `identifier_namespace` | `text` | Required | identifier namespace |
| `identifier_value` | `text` | Required | identifier value |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.assertion` — migration 0006

Versioned original vulnerability assertion.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `assertion_id` | `uuid` | Required | assertion id |
| `source_record_processing_id` | `uuid` | Required | source record processing id |
| `vulnerability_id` | `uuid` | Required | vulnerability id |
| `subject_type` | `text` | Required | subject type |
| `assertion_type` | `text` | Required | assertion type |
| `asserted_value` | `text` | Required | asserted value |
| `extraction_method` | `text` | Required | extraction method |
| `extraction_version` | `text` | Required | extraction version |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.collector_checkpoint` — migration 0007

Source feed collector checkpoint, NOT sensor checkpoint.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `collector_checkpoint_id` | `uuid` | Required | collector checkpoint id |
| `processing_run_id` | `uuid` | Required | processing run id |
| `processing_event_type` | `text` | Required | processing event type |
| `source_artifact_id` | `uuid` | Required | source artifact id |
| `process_name` | `text` | Required | process name |
| `process_version` | `text` | Required | process version |
| `checkpoint_kind` | `text` | Required | checkpoint kind |
| `source_checkpoint_value` | `text` | Required | source checkpoint value |
| `created_at` | `timestamp with time zone` | Required | Server-side creation time |

### `pathfinder.sensor` — migration 0008

Authenticated, registered sensor identity (administrative registration).

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `sensor_key` | `text` | Required | sensor key |
| `sensor_type` | `text` | Required | sensor type |
| `display_name` | `text` | Required | display name |
| `registered_at` | `timestamptz` | Required | Server-side registration time |

### `pathfinder.sensor_segment` — migration 0008

Exact captured-file provenance and its immutable external custody reference.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `segment_id` | `uuid` | Required | Optional original segment UUID |
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `stream` | `text` | Required | One of four source streams |
| `source_filename` | `text` | Required | Original captured segment basename |
| `stored_sha256` | `character(64)` | Required | SHA-256 of exact stored frozen file bytes |
| `stored_bytes` | `bigint` | Required | Frozen stored-file length |
| `record_count` | `bigint` | Required | Record count claimed / verified by ingest transaction |
| `custody_reference` | `text` | Required | Server-controlled immutable external file locator |
| `registered_at` | `timestamptz` | Required | Server-side registration time |

### `pathfinder.sensor_batch` — migration 0008

Immutable source batch identity, digest, custody reference, and declared record range.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `batch_id` | `uuid` | Required | Internal batch UUID / foreign key |
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `stream` | `text` | Required | One of four source streams |
| `wire_batch_id` | `text` | Required | Stable sender retry identity scoped by sensor and stream |
| `schema_version` | `text` | Required | Version of supported stream schema |
| `payload_sha256` | `character(64)` | Required | SHA-256 of original JSONL batch bytes |
| `payload_bytes` | `bigint` | Required | Batch uncompressed JSONL length |
| `record_count` | `bigint` | Required | Record count claimed / verified by ingest transaction |
| `preserved_at` | `timestamptz` | Required | Server durable custody timestamp |
| `custody_reference` | `text` | Required | Server-controlled immutable external file locator |
| `segment_id` | `uuid` | Nullable | Optional original segment UUID |
| `first_observed_at` | `timestamptz` | Required | Earliest associated observation time |
| `last_observed_at` | `timestamptz` | Required | Latest associated observation time |
| `created_at` | `timestamptz` | Required | Server-side creation time |

### `pathfinder.sensor_batch_index_completion` — migration 0008

Indexing completion marker committed atomically with all records.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `batch_id` | `uuid` | Required | Internal batch UUID / foreign key |
| `indexed_record_count` | `bigint` | Required | Rows indexed, verified against batch record_count |
| `indexer_version` | `text` | Required | Indexed projection version |
| `indexed_at` | `timestamptz` | Required | Server database indexing time |

### `pathfinder.sensor_observation` — migration 0008

Immutable packet/ARP/DNS/TLS/network metadata, partitioned by observation timestamp.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `observed_at` | `timestamptz` | Required | observed_at (timestamp; PostgreSQL µs) |
| `observation_id` | `uuid` | Required | observation id |
| `sensor_id` | `uuid` | Required | authenticated batch sensor; body sensor_id must match |
| `stream` | `text` | Required | One of four source streams |
| `batch_id` | `uuid` | Required | Internal batch UUID / foreign key |
| `record_ordinal` | `integer` | Required | One-based row position within batch |
| `segment_record_ordinal` | `bigint` | Nullable | Optional one-based position in frozen capture file |
| `observed_at_ns` | `bigint` | Required | observed_at (Unix nanoseconds) |
| `timestamp_source` | `text` | Required | timestamp_source |
| `interface_name` | `text` | Required | observed.interface |
| `interface_index` | `integer` | Nullable | observed.interface_index |
| `direction` | `text` | Nullable | context.direction |
| `vlan_id` | `integer` | Nullable | context.vlan_id |
| `zone` | `text` | Nullable | context.zone |
| `network_name` | `text` | Nullable | context.network_name |
| `svi` | `text` | Nullable | context.svi |
| `network_family` | `text` | Nullable | observed.network.family |
| `protocol` | `text` | Nullable | observed.network.protocol |
| `source_ip` | `inet` | Nullable | observed.network.source_ip |
| `destination_ip` | `inet` | Nullable | observed.network.destination_ip |
| `source_port` | `integer` | Nullable | observed.transport.source_port |
| `destination_port` | `integer` | Nullable | observed.transport.destination_port |
| `source_mac` | `macaddr` | Nullable | observed.ethernet.source_mac |
| `destination_mac` | `macaddr` | Nullable | observed.ethernet.destination_mac |
| `ethertype` | `text` | Nullable | observed.ethernet.ethertype |
| `captured_length` | `integer` | Nullable | observed.captured_length |
| `dns_message_type` | `text` | Nullable | observed.dns.message_type |
| `dns_query_name` | `text` | Nullable | observed.dns.query_name |
| `dns_query_type` | `text` | Nullable | observed.dns.query_type |
| `tls_client_hello_observed` | `boolean` | Required | observed.tls.client_hello_observed |
| `tls_sni` | `text` | Nullable | observed.tls.sni |
| `arp_operation` | `text` | Nullable | observed.arp.operation |
| `kernel_offload_suspected` | `boolean` | Required | context.kernel_offload_suspected |
| `raw_record` | `jsonb` | Required | Queryable JSONB document; byte-exact original remains in custody |

### `pathfinder.sensor_dns_question` — migration 0008

Ordered DNS question child linked to its original packet observation.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `observation_at` | `timestamptz` | Required | observation at |
| `observation_id` | `uuid` | Required | observation id |
| `question_ordinal` | `integer` | Required | question ordinal |
| `question_name` | `text` | Required | observed.dns.questions[].name |
| `question_type` | `text` | Required | observed.dns.questions[].type |

### `pathfinder.sensor_dns_record` — migration 0008

Ordered DNS answer/authority/additional child with preserved source record.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `observation_at` | `timestamptz` | Required | observation at |
| `observation_id` | `uuid` | Required | observation id |
| `section` | `text` | Required | section |
| `record_ordinal` | `integer` | Required | One-based row position within batch |
| `name` | `text` | Required | observed.dns.{section}[].name |
| `record_type` | `text` | Required | observed.dns.{section}[].type |
| `record_class` | `text` | Nullable | observed.dns.{section}[].class |
| `ttl_seconds` | `bigint` | Nullable | observed.dns.{section}[].ttl |
| `value` | `text` | Nullable | observed.dns.{section}[].value |
| `raw_record` | `jsonb` | Required | Queryable JSONB document; byte-exact original remains in custody |

### `pathfinder.sensor_conntrack_event` — migration 0008

Immutable conntrack original/reply tuples, lifecycle, flags, counters and NAT, time-partitioned.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `observed_at` | `timestamptz` | Required | observed_at |
| `event_id` | `uuid` | Required | event id |
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `stream` | `text` | Required | One of four source streams |
| `batch_id` | `uuid` | Required | Internal batch UUID / foreign key |
| `record_ordinal` | `integer` | Required | One-based row position within batch |
| `segment_record_ordinal` | `bigint` | Nullable | Optional one-based position in frozen capture file |
| `observed_at_ns` | `bigint` | Required | observed_at (Unix nanoseconds) |
| `timestamp_source` | `text` | Required | timestamp_source |
| `conntrack_id` | `bigint` | Required | id; NOT globally unique |
| `event_type` | `text` | Required | event_type |
| `original_family` | `text` | Nullable | original.family |
| `original_protocol` | `text` | Nullable | original.protocol |
| `original_source_ip` | `inet` | Nullable | original.source_ip |
| `original_destination_ip` | `inet` | Nullable | original.destination_ip |
| `original_source_port` | `integer` | Nullable | original.source_port |
| `original_destination_port` | `integer` | Nullable | original.destination_port |
| `reply_family` | `text` | Nullable | reply.family |
| `reply_protocol` | `text` | Nullable | reply.protocol |
| `reply_source_ip` | `inet` | Nullable | reply.source_ip |
| `reply_destination_ip` | `inet` | Nullable | reply.destination_ip |
| `reply_source_port` | `integer` | Nullable | reply.source_port |
| `reply_destination_port` | `integer` | Nullable | reply.destination_port |
| `source_nat` | `boolean` | Required | source_nat flag |
| `destination_nat` | `boolean` | Required | destination_nat flag |
| `tcp_state` | `text` | Nullable | tcp_state |
| `assured` | `boolean` | Required | assured flag |
| `confirmed` | `boolean` | Required | confirmed flag |
| `seen_reply` | `boolean` | Required | seen_reply flag |
| `dying` | `boolean` | Required | dying flag |
| `offload` | `boolean` | Required | offload flag |
| `hardware_offload` | `boolean` | Required | hardware_offload flag |
| `timeout_seconds` | `bigint` | Nullable | timeout_seconds |
| `conntrack_status` | `bigint` | Nullable | status |
| `mark` | `bigint` | Nullable | mark |
| `original_counters_present` | `boolean` | Required | original_counters.present |
| `original_packets` | `bigint` | Nullable | original_counters.packets |
| `original_bytes` | `bigint` | Nullable | original_counters.bytes |
| `reply_counters_present` | `boolean` | Required | reply_counters.present |
| `reply_packets` | `bigint` | Nullable | reply_counters.packets |
| `reply_bytes` | `bigint` | Nullable | reply_counters.bytes |
| `raw_record` | `jsonb` | Required | Queryable JSONB document; byte-exact original remains in custody |

### `pathfinder.sensor_identity_snapshot` — migration 0008

Historical point-in-time identity snapshot, not current endpoint state.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `snapshot_id` | `uuid` | Required | snapshot id |
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `stream` | `text` | Required | One of four source streams |
| `batch_id` | `uuid` | Required | Internal batch UUID / foreign key |
| `record_ordinal` | `integer` | Required | One-based row position within batch |
| `segment_record_ordinal` | `bigint` | Nullable | Optional one-based position in frozen capture file |
| `observed_at` | `timestamptz` | Required | observed_at |
| `observed_at_ns` | `bigint` | Required | observed_at (Unix nanoseconds) |
| `expected_interval_seconds` | `bigint` | Required | expected_interval_seconds |
| `identity_source` | `text` | Required | source |
| `identity_source_path` | `text` | Required | source_path |
| `raw_record` | `jsonb` | Required | Queryable JSONB document; byte-exact original remains in custody |

### `pathfinder.sensor_identity_endpoint` — migration 0008

Ordered endpoint entry of an identity snapshot.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `snapshot_id` | `uuid` | Required | snapshot id |
| `endpoint_ordinal` | `integer` | Required | endpoint ordinal |
| `ip` | `inet` | Nullable | endpoints[].ip |
| `mac` | `macaddr` | Nullable | endpoints[].mac |
| `hostname` | `text` | Nullable | endpoints[].hostname |
| `fqdn` | `text` | Nullable | endpoints[].fqdn |
| `lease_expires_at` | `timestamptz` | Nullable | endpoints[].lease_expires_at |
| `raw_endpoint` | `jsonb` | Required | Entire source endpoint JSONB |

### `pathfinder.sensor_health` — migration 0008

Historical sensor, kernel, conntrack, storage and capture health snapshot.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `health_id` | `uuid` | Required | health id |
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `stream` | `text` | Required | One of four source streams |
| `batch_id` | `uuid` | Required | Internal batch UUID / foreign key |
| `record_ordinal` | `integer` | Required | One-based row position within batch |
| `segment_record_ordinal` | `bigint` | Nullable | Optional one-based position in frozen capture file |
| `observed_at` | `timestamptz` | Required | observed_at |
| `observed_at_ns` | `bigint` | Required | observed_at (Unix nanoseconds) |
| `capture_started_at` | `timestamptz` | Required | capture.started_at |
| `expected_interval_seconds` | `bigint` | Required | expected_interval_seconds |
| `final` | `boolean` | Required | final |
| `observations_written` | `bigint` | Required | capture.observations_written |
| `observations_discarded` | `bigint` | Required | capture.observations_discarded |
| `capture_write_failures` | `bigint` | Required | capture.write_failures |
| `conntrack_enabled` | `boolean` | Required | conntrack.enabled |
| `conntrack_events_received` | `bigint` | Nullable | conntrack.events_received |
| `conntrack_receive_overruns` | `bigint` | Nullable | conntrack.receive_overruns |
| `raw_record` | `jsonb` | Required | Queryable JSONB document; byte-exact original remains in custody |

### `pathfinder.sensor_connection` — migration 0008

Versioned, rebuildable connection and NAT correlation, not original evidence.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `connection_id` | `uuid` | Required | connection id |
| `sensor_id` | `uuid` | Required | Registered sensor UUID; must match batch |
| `correlation_version` | `text` | Required | Versioned deterministic correlation logic |
| `connection_fingerprint` | `character(64)` | Required | SHA-256 of correlation's canonical identity/time inputs |
| `first_observed_at` | `timestamptz` | Required | Earliest associated observation time |
| `last_observed_at` | `timestamptz` | Required | Latest associated observation time |
| `protocol` | `text` | Required | protocol |
| `original_source_ip` | `inet` | Nullable | original source ip |
| `original_destination_ip` | `inet` | Nullable | original destination ip |
| `original_source_port` | `integer` | Nullable | original source port |
| `original_destination_port` | `integer` | Nullable | original destination port |
| `translated_source_ip` | `inet` | Nullable | translated source ip |
| `translated_destination_ip` | `inet` | Nullable | translated destination ip |
| `translated_source_port` | `integer` | Nullable | translated source port |
| `translated_destination_port` | `integer` | Nullable | translated destination port |
| `correlation_status` | `text` | Required | CONFIRMED/CANDIDATE/AMBIGUOUS/UNMATCHED |
| `derived_at` | `timestamptz` | Required | Versioned derived result creation time |

### `pathfinder.sensor_connection_observation_evidence` — migration 0008

FK from derived connection to the exact original packet observation.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `connection_id` | `uuid` | Required | connection id |
| `observation_at` | `timestamptz` | Required | observation at |
| `observation_id` | `uuid` | Required | observation id |
| `evidence_role` | `text` | Required | How referenced source evidence supports correlation |

### `pathfinder.sensor_connection_conntrack_evidence` — migration 0008

FK from derived connection to the exact original conntrack event.

| Column | PostgreSQL type | Null | Meaning / source mapping |
|---|---|---|---|
| `connection_id` | `uuid` | Required | connection id |
| `conntrack_at` | `timestamptz` | Required | conntrack at |
| `event_id` | `uuid` | Required | event id |
| `evidence_role` | `text` | Required | How referenced source evidence supports correlation |

## Evidence and replay invariants

- The `sensor_batch(sensor_id,stream,wire_batch_id)` unique constraint scopes sender retries. The indexing code must compare stored hashes and counts; SQL uniqueness alone cannot detect a conflicting payload silently ignored by an unsafe `ON CONFLICT DO NOTHING`.
- An ingestion transaction must insert **all** evidence rows and DNS/identity children, check complete counts and source checksums, then insert `sensor_batch_index_completion` before committing. A custody receipt alone must not be interpreted as indexed SoR acceptance.
- Source ordinal and (for time partitions) observation timestamp help preserve identity; do **not** merge repeated five-tuples or two interfaces into one source observation.
- Connection rows are *derived* and versioned. Evidence tables hold the authoritative sensor records, including unmatched DNAT, disconnected conntrack lifecycle events and health loss data.
- Conntrack id is not globally unique; even a complete tuple does not prove a connection lifetime without event timing, NAT and vantage evidence.
- Identity histories use the snapshot observed_at and expected_interval_seconds, and honor lease expiry. A current IP assignment is not retroactively authoritative.
- Health counters are cumulative within a capture session. Capture restart resets their comparison baseline. Kernel drops and decode-reject counters are preserved inside raw_record even where not separately projected.
- No attempt has been made to benchmark this DDL on PostgreSQL, run an importer, provision a runtime SoR, or prove loss/replay recovery in an isolated database yet.

## Partitioning and index policy

The large `sensor_observation` and `sensor_conntrack_event` tables use daily range partitions by observed_at; migration 0008 provisions migration date -7 through +14 days and fallback default partitions. Maintaining future partitions and draining default partitions is an **outstanding operational requirement**. BRIN on time; B-trees on source/destination IP, sensor/time, tuple candidates, DNS names, SNI and conntrack identity. Measure sustained ingestion costs against the complete 599,984-record frozen capture before selecting final index, retention, vacuum and WAL settings.

**Captured validation sample:** 554,863 packet observations, 45,009 conntrack events, 56 endpoint identity snapshots, 56 health reports, 846.03 MiB uncompressed JSONL in 55 minutes (2026-11-20 10:00:00–10:55:00Z). SNAT events: 8,355/8,391 had LAN/WAN tuple evidence somewhere in the window, 7,923/8,391 within two minutes. These are evidence counts, not guaranteed distinct connection lifetimes.

## Forward-only migration 0009 — Replay lookup performance

Migration `0009-sensor-batch-replay-indexes.sql` adds time-partition-compatible `(batch_id, observed_at)` indexes on `sensor_observation` and `sensor_conntrack_event`. No table or column definitions change; the dictionary remains 26 tables and 286 columns. The indexes support transactional batch completeness checks and exact frozen-archive reconciliation.
