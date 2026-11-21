-- Pathfinder sensor SoR foundation. All evidence is append-only in this schema.
-- Schema v8 is distinct from threat-intelligence source/SourceArtifact structures.
-- Record byte fidelity resides in immutable custody; raw_record is queryable JSONB.
-- Daily partitions are provisioned with an explicit DEFAULT fallback.
CREATE TABLE pathfinder.sensor (
    sensor_id uuid PRIMARY KEY DEFAULT uuidv7(),
    sensor_key text NOT NULL UNIQUE CHECK (sensor_key ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    sensor_type text NOT NULL CHECK (btrim(sensor_type) <> ''),
    display_name text NOT NULL CHECK (btrim(display_name) <> ''),
    registered_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE pathfinder.sensor_segment (
    segment_id uuid PRIMARY KEY DEFAULT uuidv7(),
    sensor_id uuid NOT NULL REFERENCES pathfinder.sensor(sensor_id) ON DELETE RESTRICT,
    stream text NOT NULL CHECK (stream IN ('observations','conntrack','identity','health')),
    source_filename text NOT NULL CHECK (btrim(source_filename) <> ''),
    stored_sha256 character(64) NOT NULL CHECK (stored_sha256 ~ '^[0-9a-f]{64}$'),
    stored_bytes bigint NOT NULL CHECK (stored_bytes >= 0),
    record_count bigint NOT NULL CHECK (record_count > 0),
    custody_reference text NOT NULL CHECK (btrim(custody_reference) <> ''),
    registered_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (sensor_id, stream, stored_sha256),
    UNIQUE (segment_id, sensor_id, stream)
);

CREATE TABLE pathfinder.sensor_batch (
    batch_id uuid PRIMARY KEY DEFAULT uuidv7(),
    sensor_id uuid NOT NULL REFERENCES pathfinder.sensor(sensor_id) ON DELETE RESTRICT,
    stream text NOT NULL CHECK (stream IN ('observations','conntrack','identity','health')),
    wire_batch_id text NOT NULL CHECK (btrim(wire_batch_id) <> ''),
    schema_version text NOT NULL CHECK (
        (stream = 'observations' AND schema_version = '5') OR
        (stream = 'conntrack' AND schema_version = '1') OR
        (stream = 'identity' AND schema_version = '1') OR
        (stream = 'health' AND schema_version = '2')
    ),
    payload_sha256 character(64) NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    payload_bytes bigint NOT NULL CHECK (payload_bytes > 0),
    record_count bigint NOT NULL CHECK (record_count BETWEEN 1 AND 25000),
    preserved_at timestamptz NOT NULL,
    custody_reference text NOT NULL CHECK (btrim(custody_reference) <> ''),
    segment_id uuid,
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (first_observed_at <= last_observed_at),
    UNIQUE (sensor_id, stream, wire_batch_id),
    UNIQUE (batch_id, sensor_id, stream),
    FOREIGN KEY (segment_id, sensor_id, stream)
        REFERENCES pathfinder.sensor_segment(segment_id, sensor_id, stream)
        ON DELETE RESTRICT
);

-- The completion marker and all records are committed in the SAME transaction.
-- A custody receipt alone does not create one of these rows.
CREATE TABLE pathfinder.sensor_batch_index_completion (
    batch_id uuid PRIMARY KEY REFERENCES pathfinder.sensor_batch(batch_id) ON DELETE RESTRICT,
    indexed_record_count bigint NOT NULL CHECK (indexed_record_count > 0),
    indexer_version text NOT NULL CHECK (btrim(indexer_version) <> ''),
    indexed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE pathfinder.sensor_observation (
    observed_at timestamptz NOT NULL,
    observation_id uuid NOT NULL DEFAULT uuidv7(),
    sensor_id uuid NOT NULL,
    stream text NOT NULL DEFAULT 'observations' CHECK (stream = 'observations'),
    batch_id uuid NOT NULL,
    record_ordinal integer NOT NULL CHECK (record_ordinal > 0),
    segment_record_ordinal bigint CHECK (segment_record_ordinal > 0),
    observed_at_ns bigint NOT NULL,
    timestamp_source text NOT NULL,
    interface_name text NOT NULL,
    interface_index integer,
    direction text,
    vlan_id integer,
    zone text,
    network_name text,
    svi text,
    network_family text,
    protocol text,
    source_ip inet,
    destination_ip inet,
    source_port integer CHECK (source_port BETWEEN 0 AND 65535),
    destination_port integer CHECK (destination_port BETWEEN 0 AND 65535),
    source_mac macaddr,
    destination_mac macaddr,
    ethertype text,
    captured_length integer CHECK (captured_length >= 0),
    dns_message_type text,
    dns_query_name text,
    dns_query_type text,
    tls_client_hello_observed boolean NOT NULL DEFAULT false,
    tls_sni text,
    arp_operation text,
    kernel_offload_suspected boolean NOT NULL DEFAULT false,
    raw_record jsonb NOT NULL CHECK (jsonb_typeof(raw_record) = 'object'),
    PRIMARY KEY (observed_at, observation_id),
    UNIQUE (observed_at, batch_id, record_ordinal),
    FOREIGN KEY (batch_id, sensor_id, stream)
        REFERENCES pathfinder.sensor_batch(batch_id, sensor_id, stream)
        ON DELETE RESTRICT
) PARTITION BY RANGE (observed_at);

CREATE TABLE pathfinder.sensor_dns_question (
    observation_at timestamptz NOT NULL,
    observation_id uuid NOT NULL,
    question_ordinal integer NOT NULL CHECK (question_ordinal >= 0),
    question_name text NOT NULL,
    question_type text NOT NULL,
    PRIMARY KEY (observation_at, observation_id, question_ordinal),
    FOREIGN KEY (observation_at, observation_id)
        REFERENCES pathfinder.sensor_observation(observed_at, observation_id)
        ON DELETE RESTRICT
);

CREATE TABLE pathfinder.sensor_dns_record (
    observation_at timestamptz NOT NULL,
    observation_id uuid NOT NULL,
    section text NOT NULL CHECK (section IN ('answer', 'authority', 'additional')),
    record_ordinal integer NOT NULL CHECK (record_ordinal >= 0),
    name text NOT NULL,
    record_type text NOT NULL,
    record_class text,
    ttl_seconds bigint CHECK (ttl_seconds >= 0),
    value text,
    raw_record jsonb NOT NULL CHECK (jsonb_typeof(raw_record) = 'object'),
    PRIMARY KEY (observation_at, observation_id, section, record_ordinal),
    FOREIGN KEY (observation_at, observation_id)
        REFERENCES pathfinder.sensor_observation(observed_at, observation_id)
        ON DELETE RESTRICT
);

CREATE TABLE pathfinder.sensor_conntrack_event (
    observed_at timestamptz NOT NULL,
    event_id uuid NOT NULL DEFAULT uuidv7(),
    sensor_id uuid NOT NULL,
    stream text NOT NULL DEFAULT 'conntrack' CHECK (stream = 'conntrack'),
    batch_id uuid NOT NULL,
    record_ordinal integer NOT NULL CHECK (record_ordinal > 0),
    segment_record_ordinal bigint CHECK (segment_record_ordinal > 0),
    observed_at_ns bigint NOT NULL,
    timestamp_source text NOT NULL,
    conntrack_id bigint NOT NULL CHECK (conntrack_id BETWEEN 0 AND 4294967295),
    event_type text NOT NULL CHECK (event_type IN ('new','update','destroy')),
    original_family text,
    original_protocol text,
    original_source_ip inet,
    original_destination_ip inet,
    original_source_port integer CHECK (original_source_port BETWEEN 0 AND 65535),
    original_destination_port integer CHECK (original_destination_port BETWEEN 0 AND 65535),
    reply_family text,
    reply_protocol text,
    reply_source_ip inet,
    reply_destination_ip inet,
    reply_source_port integer CHECK (reply_source_port BETWEEN 0 AND 65535),
    reply_destination_port integer CHECK (reply_destination_port BETWEEN 0 AND 65535),
    source_nat boolean NOT NULL,
    destination_nat boolean NOT NULL,
    tcp_state text,
    assured boolean NOT NULL,
    confirmed boolean NOT NULL,
    seen_reply boolean NOT NULL,
    dying boolean NOT NULL,
    offload boolean NOT NULL,
    hardware_offload boolean NOT NULL,
    timeout_seconds bigint CHECK (timeout_seconds >= 0),
    conntrack_status bigint CHECK (conntrack_status >= 0),
    mark bigint CHECK (mark >= 0),
    original_counters_present boolean NOT NULL,
    original_packets bigint CHECK (original_packets >= 0),
    original_bytes bigint CHECK (original_bytes >= 0),
    reply_counters_present boolean NOT NULL,
    reply_packets bigint CHECK (reply_packets >= 0),
    reply_bytes bigint CHECK (reply_bytes >= 0),
    raw_record jsonb NOT NULL CHECK (jsonb_typeof(raw_record) = 'object'),
    PRIMARY KEY (observed_at, event_id),
    UNIQUE (observed_at, batch_id, record_ordinal),
    FOREIGN KEY (batch_id, sensor_id, stream)
        REFERENCES pathfinder.sensor_batch(batch_id, sensor_id, stream)
        ON DELETE RESTRICT
) PARTITION BY RANGE (observed_at);

CREATE TABLE pathfinder.sensor_identity_snapshot (
    snapshot_id uuid PRIMARY KEY DEFAULT uuidv7(),
    sensor_id uuid NOT NULL,
    stream text NOT NULL DEFAULT 'identity' CHECK (stream = 'identity'),
    batch_id uuid NOT NULL,
    record_ordinal integer NOT NULL CHECK (record_ordinal > 0),
    segment_record_ordinal bigint CHECK (segment_record_ordinal > 0),
    observed_at timestamptz NOT NULL,
    observed_at_ns bigint NOT NULL,
    expected_interval_seconds bigint NOT NULL CHECK (expected_interval_seconds > 0),
    identity_source text NOT NULL,
    identity_source_path text NOT NULL,
    raw_record jsonb NOT NULL CHECK (jsonb_typeof(raw_record) = 'object'),
    UNIQUE (batch_id, record_ordinal),
    FOREIGN KEY (batch_id, sensor_id, stream)
        REFERENCES pathfinder.sensor_batch(batch_id, sensor_id, stream)
        ON DELETE RESTRICT
);

CREATE TABLE pathfinder.sensor_identity_endpoint (
    snapshot_id uuid NOT NULL REFERENCES pathfinder.sensor_identity_snapshot(snapshot_id) ON DELETE RESTRICT,
    endpoint_ordinal integer NOT NULL CHECK (endpoint_ordinal >= 0),
    ip inet,
    mac macaddr,
    hostname text,
    fqdn text,
    lease_expires_at timestamptz,
    raw_endpoint jsonb NOT NULL CHECK (jsonb_typeof(raw_endpoint) = 'object'),
    PRIMARY KEY (snapshot_id, endpoint_ordinal)
);

CREATE TABLE pathfinder.sensor_health (
    health_id uuid PRIMARY KEY DEFAULT uuidv7(),
    sensor_id uuid NOT NULL,
    stream text NOT NULL DEFAULT 'health' CHECK (stream = 'health'),
    batch_id uuid NOT NULL,
    record_ordinal integer NOT NULL CHECK (record_ordinal > 0),
    segment_record_ordinal bigint CHECK (segment_record_ordinal > 0),
    observed_at timestamptz NOT NULL,
    observed_at_ns bigint NOT NULL,
    capture_started_at timestamptz NOT NULL,
    expected_interval_seconds bigint NOT NULL CHECK (expected_interval_seconds > 0),
    final boolean NOT NULL,
    observations_written bigint NOT NULL CHECK (observations_written >= 0),
    observations_discarded bigint NOT NULL CHECK (observations_discarded >= 0),
    capture_write_failures bigint NOT NULL CHECK (capture_write_failures >= 0),
    conntrack_enabled boolean NOT NULL,
    conntrack_events_received bigint CHECK (conntrack_events_received >= 0),
    conntrack_receive_overruns bigint CHECK (conntrack_receive_overruns >= 0),
    raw_record jsonb NOT NULL CHECK (jsonb_typeof(raw_record) = 'object'),
    UNIQUE (batch_id, record_ordinal),
    FOREIGN KEY (batch_id, sensor_id, stream)
        REFERENCES pathfinder.sensor_batch(batch_id, sensor_id, stream)
        ON DELETE RESTRICT
);

-- Connections are derived. They are NOT immutable sensor evidence.
CREATE TABLE pathfinder.sensor_connection (
    connection_id uuid PRIMARY KEY DEFAULT uuidv7(),
    sensor_id uuid NOT NULL REFERENCES pathfinder.sensor(sensor_id) ON DELETE RESTRICT,
    correlation_version text NOT NULL CHECK (btrim(correlation_version) <> ''),
    connection_fingerprint character(64) NOT NULL CHECK (connection_fingerprint ~ '^[0-9a-f]{64}$'),
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    protocol text NOT NULL,
    original_source_ip inet,
    original_destination_ip inet,
    original_source_port integer CHECK (original_source_port BETWEEN 0 AND 65535),
    original_destination_port integer CHECK (original_destination_port BETWEEN 0 AND 65535),
    translated_source_ip inet,
    translated_destination_ip inet,
    translated_source_port integer CHECK (translated_source_port BETWEEN 0 AND 65535),
    translated_destination_port integer CHECK (translated_destination_port BETWEEN 0 AND 65535),
    correlation_status text NOT NULL CHECK (correlation_status IN ('CONFIRMED', 'CANDIDATE', 'AMBIGUOUS', 'UNMATCHED')),
    derived_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (first_observed_at <= last_observed_at),
    UNIQUE (sensor_id, correlation_version, connection_fingerprint)
);

CREATE TABLE pathfinder.sensor_connection_observation_evidence (
    connection_id uuid NOT NULL REFERENCES pathfinder.sensor_connection(connection_id) ON DELETE RESTRICT,
    observation_at timestamptz NOT NULL,
    observation_id uuid NOT NULL,
    evidence_role text NOT NULL CHECK (btrim(evidence_role) <> ''),
    PRIMARY KEY (connection_id, observation_at, observation_id, evidence_role),
    FOREIGN KEY (observation_at, observation_id)
        REFERENCES pathfinder.sensor_observation(observed_at, observation_id)
        ON DELETE RESTRICT
);

CREATE TABLE pathfinder.sensor_connection_conntrack_evidence (
    connection_id uuid NOT NULL REFERENCES pathfinder.sensor_connection(connection_id) ON DELETE RESTRICT,
    conntrack_at timestamptz NOT NULL,
    event_id uuid NOT NULL,
    evidence_role text NOT NULL CHECK (btrim(evidence_role) <> ''),
    PRIMARY KEY (connection_id, conntrack_at, event_id, evidence_role),
    FOREIGN KEY (conntrack_at, event_id)
        REFERENCES pathfinder.sensor_conntrack_event(observed_at, event_id)
        ON DELETE RESTRICT
);

-- Practical historical query paths. Benchmark index/write cost on frozen
-- UDM-Pro sample before scaling retention or indexes to production.
CREATE INDEX sensor_segment_sensor_idx ON pathfinder.sensor_segment(sensor_id, registered_at DESC);
CREATE INDEX sensor_batch_sensor_time_idx ON pathfinder.sensor_batch(sensor_id, preserved_at DESC);
CREATE INDEX sensor_observation_time_brin ON pathfinder.sensor_observation USING brin(observed_at);
CREATE INDEX sensor_observation_sensor_time_idx ON pathfinder.sensor_observation(sensor_id, observed_at DESC);
CREATE INDEX sensor_observation_source_time_idx ON pathfinder.sensor_observation(source_ip, observed_at DESC) WHERE source_ip IS NOT NULL;
CREATE INDEX sensor_observation_dest_time_idx ON pathfinder.sensor_observation(destination_ip, observed_at DESC) WHERE destination_ip IS NOT NULL;
CREATE INDEX sensor_observation_tuple_idx ON pathfinder.sensor_observation(sensor_id, protocol, source_ip, destination_ip, source_port, destination_port, observed_at DESC);
CREATE INDEX sensor_observation_dns_time_idx ON pathfinder.sensor_observation(dns_query_name, observed_at DESC) WHERE dns_query_name IS NOT NULL;
CREATE INDEX sensor_observation_sni_time_idx ON pathfinder.sensor_observation(tls_sni, observed_at DESC) WHERE tls_sni IS NOT NULL;
CREATE INDEX sensor_conntrack_time_brin ON pathfinder.sensor_conntrack_event USING brin(observed_at);
CREATE INDEX sensor_conntrack_sensor_time_idx ON pathfinder.sensor_conntrack_event(sensor_id, observed_at DESC);
CREATE INDEX sensor_conntrack_id_time_idx ON pathfinder.sensor_conntrack_event(sensor_id, conntrack_id, observed_at DESC);
CREATE INDEX sensor_conntrack_orig_time_idx ON pathfinder.sensor_conntrack_event(original_source_ip, observed_at DESC) WHERE original_source_ip IS NOT NULL;
CREATE INDEX sensor_conntrack_reply_time_idx ON pathfinder.sensor_conntrack_event(reply_destination_ip, observed_at DESC) WHERE reply_destination_ip IS NOT NULL;
CREATE INDEX sensor_identity_time_idx ON pathfinder.sensor_identity_snapshot(sensor_id, observed_at DESC);
CREATE INDEX sensor_identity_endpoint_ip_idx ON pathfinder.sensor_identity_endpoint(ip) WHERE ip IS NOT NULL;
CREATE INDEX sensor_identity_endpoint_mac_idx ON pathfinder.sensor_identity_endpoint(mac) WHERE mac IS NOT NULL;
CREATE INDEX sensor_health_time_idx ON pathfinder.sensor_health(sensor_id, observed_at DESC);
CREATE INDEX sensor_connection_time_idx ON pathfinder.sensor_connection(sensor_id, first_observed_at DESC);
CREATE INDEX sensor_connection_orig_ip_idx ON pathfinder.sensor_connection(original_source_ip, first_observed_at DESC);
CREATE INDEX sensor_connection_dest_ip_idx ON pathfinder.sensor_connection(original_destination_ip, first_observed_at DESC);

-- Make the sample date and a short operational horizon fast; other historical
-- dates land in the default partition rather than failing unaccountably.
DO $pathfinder_sensor_partition_bootstrap$
DECLARE
    day_start date;
    observation_name text;
    conntrack_name text;
BEGIN
    FOR day_start IN
        SELECT generate_series(
            (current_date - interval '7 days')::date,
            (current_date + interval '14 days')::date,
            interval '1 day'
        )::date
    LOOP
        observation_name := format('sensor_observation_%s', to_char(day_start, 'YYYYMMDD'));
        conntrack_name := format('sensor_conntrack_event_%s', to_char(day_start, 'YYYYMMDD'));
        EXECUTE format(
            'CREATE TABLE pathfinder.%I PARTITION OF pathfinder.sensor_observation FOR VALUES FROM (%L) TO (%L)',
            observation_name, day_start, day_start + 1
        );
        EXECUTE format(
            'CREATE TABLE pathfinder.%I PARTITION OF pathfinder.sensor_conntrack_event FOR VALUES FROM (%L) TO (%L)',
            conntrack_name, day_start, day_start + 1
        );
    END LOOP;
END;
$pathfinder_sensor_partition_bootstrap$;

CREATE TABLE pathfinder.sensor_observation_default PARTITION OF pathfinder.sensor_observation DEFAULT;
CREATE TABLE pathfinder.sensor_conntrack_event_default PARTITION OF pathfinder.sensor_conntrack_event DEFAULT;

-- Security boundaries: normal runtime can read sensor registration, but not
-- self-register a new trusted sensor. Evidence only receives append authority.
GRANT SELECT ON pathfinder.sensor TO pathfinder_app;
GRANT SELECT, INSERT ON
    pathfinder.sensor_segment,
    pathfinder.sensor_batch,
    pathfinder.sensor_batch_index_completion,
    pathfinder.sensor_observation,
    pathfinder.sensor_dns_question,
    pathfinder.sensor_dns_record,
    pathfinder.sensor_conntrack_event,
    pathfinder.sensor_identity_snapshot,
    pathfinder.sensor_identity_endpoint,
    pathfinder.sensor_health,
    pathfinder.sensor_connection,
    pathfinder.sensor_connection_observation_evidence,
    pathfinder.sensor_connection_conntrack_evidence
TO pathfinder_app;

COMMENT ON TABLE pathfinder.sensor_observation IS 'Immutable network metadata observation; original JSONL bytes remain in immutable batch/segment custody.';
COMMENT ON TABLE pathfinder.sensor_conntrack_event IS 'Immutable Linux conntrack event: original and reply tuples, NAT flags, and original metadata.';
COMMENT ON TABLE pathfinder.sensor_connection IS 'Rebuildable versioned correlation, never an authoritative replacement for observations.';
COMMENT ON TABLE pathfinder.sensor_batch_index_completion IS 'Inserted in the same transaction as all indexed records; custody alone is not indexed acceptance.';
