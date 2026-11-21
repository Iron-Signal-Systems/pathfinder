-- Sensor SoR ingest/replay lookup indexes. Forward-only; migration 0008 unchanged.
-- Source records remain append-only; these indexes support exact batch replay
-- verification and full frozen-archive reconciliation without full table scans.
CREATE INDEX sensor_observation_batch_lookup_idx
    ON pathfinder.sensor_observation (batch_id, observed_at);

CREATE INDEX sensor_conntrack_batch_lookup_idx
    ON pathfinder.sensor_conntrack_event (batch_id, observed_at);

COMMENT ON INDEX pathfinder.sensor_observation_batch_lookup_idx IS
    'Batch/record reconciliation and replay integrity lookup, across daily partitions';

COMMENT ON INDEX pathfinder.sensor_conntrack_batch_lookup_idx IS
    'Batch/record reconciliation and replay integrity lookup, across daily partitions';
