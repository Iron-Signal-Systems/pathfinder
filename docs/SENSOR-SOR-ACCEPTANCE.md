# Pathfinder Sensor SoR — query and acceptance contracts

**Development-only design and query examples.** These SQL statements are
repo-owned acceptance targets; no live PostgreSQL execution is claimed.

Reference: [DB Data Dictionary](DB-DATA-DICTIONARY.md), migration
`go/cmd/pathfinder/migrations/0008-sensor-observation-sor.sql`.

The example range covers the UDM-Pro sample:
2026-11-20 10:00:00Z inclusive through 10:55:00Z exclusive.

## Baseline source record counts

The original source rows must be queryable in the SoR with counts exactly:

| Stream table | Expected rows in sample |
|---|---:|
| `sensor_observation` | 554,863 |
| `sensor_conntrack_event` | 45,009 |
| `sensor_identity_snapshot` | 56 |
| `sensor_health` | 56 |
| **Total** | **599,984** |

These are counts of source evidence, not DNS child rows, endpoint child rows,
derived connections or distinct five-tuples. A replay of the same original
batches must not increase any count, alter a stored original record, or create
new completion markers.

```sql
SELECT 'observations' AS stream, count(*) FROM pathfinder.sensor_observation
 WHERE observed_at >= '2026-11-20 10:00:00+00'::timestamptz
   AND observed_at < '2026-11-20 10:55:00+00'::timestamptz
UNION ALL
SELECT 'conntrack', count(*) FROM pathfinder.sensor_conntrack_event
 WHERE observed_at >= '2026-11-20 10:00:00+00'::timestamptz
   AND observed_at < '2026-11-20 10:55:00+00'::timestamptz
UNION ALL
SELECT 'identity', count(*) FROM pathfinder.sensor_identity_snapshot
 WHERE observed_at >= '2026-11-20 10:00:00+00'::timestamptz
   AND observed_at < '2026-11-20 10:55:00+00'::timestamptz
UNION ALL
SELECT 'health', count(*) FROM pathfinder.sensor_health
 WHERE observed_at >= '2026-11-20 10:00:00+00'::timestamptz
   AND observed_at < '2026-11-20 10:55:00+00'::timestamptz;
```

## Historical network and DNS/TLS queries

Search time-bounded source and destination IP separately so both dedicated
indexes can be used; parameterize the address and time in the application.

```sql
SELECT observed_at, interface_name, direction, protocol,
       source_ip, source_port, destination_ip, destination_port,
       dns_query_name, tls_sni, kernel_offload_suspected
FROM pathfinder.sensor_observation
WHERE observed_at >= '2026-11-20 10:00:00+00'
  AND observed_at < '2026-11-20 10:55:00+00'
  AND source_ip = inet '192.0.2.10'
ORDER BY observed_at
LIMIT 500;
```

```sql
SELECT observed_at, source_ip, destination_ip, dns_query_name,
       dns_query_type, dns_message_type
FROM pathfinder.sensor_observation
WHERE observed_at >= '2026-11-20 10:00:00+00'
  AND observed_at < '2026-11-20 10:55:00+00'
  AND dns_query_name = 'dns.google'
ORDER BY observed_at
LIMIT 500;

SELECT observed_at, source_ip, destination_ip, tls_sni
FROM pathfinder.sensor_observation
WHERE observed_at >= '2026-11-20 10:00:00+00'
  AND observed_at < '2026-11-20 10:55:00+00'
  AND tls_sni = 'dns.google'
ORDER BY observed_at
LIMIT 500;
```

DNS question/resource-record arrays are preserved in the ordered child tables,
linked by both observation ID **and its timestamp**.

```sql
SELECT o.observed_at, q.question_ordinal, q.question_name, q.question_type
FROM pathfinder.sensor_observation o
JOIN pathfinder.sensor_dns_question q
  ON q.observation_at = o.observed_at
 AND q.observation_id = o.observation_id
WHERE o.observed_at >= '2026-11-20 10:00:00+00'
  AND o.observed_at < '2026-11-20 10:55:00+00'
ORDER BY o.observed_at, q.question_ordinal
LIMIT 500;
```

## Conntrack and NAT history

Source and reply tuples remain distinct; one conntrack ID or tuple may be reused.
Do not equate an event count with connection count.

```sql
SELECT observed_at, conntrack_id, event_type, tcp_state,
       source_nat, destination_nat,
       original_source_ip, original_source_port,
       original_destination_ip, original_destination_port,
       reply_source_ip, reply_source_port,
       reply_destination_ip, reply_destination_port
FROM pathfinder.sensor_conntrack_event
WHERE observed_at >= '2026-11-20 10:00:00+00'
  AND observed_at < '2026-11-20 10:55:00+00'
  AND source_nat
ORDER BY observed_at
LIMIT 500;
```

## Time-bounded endpoint identity

A matching snapshot is a historical *candidate* only if it precedes the event
and is within twice the expected snapshot interval plus five seconds, with a
valid unexpired lease when one is supplied. IP reuse must be handled explicitly.

```sql
WITH target AS (
  SELECT '2026-11-20 10:31:00+00'::timestamptz AS at,
         inet '192.0.2.10' AS ip
)
SELECT s.observed_at AS snapshot_at, e.ip, e.mac, e.hostname,
       e.fqdn, e.lease_expires_at
FROM target AS t
JOIN pathfinder.sensor_identity_endpoint e ON e.ip = t.ip
JOIN pathfinder.sensor_identity_snapshot s ON s.snapshot_id = e.snapshot_id
WHERE s.observed_at <= t.at
  AND t.at - s.observed_at <=
      make_interval(secs => (2 * s.expected_interval_seconds + 5)::int)
  AND (e.lease_expires_at IS NULL OR e.lease_expires_at >= t.at)
ORDER BY s.observed_at DESC
LIMIT 20;
```

## Sensor coverage and restarts

Only compare cumulative counters within the **same capture session**. The
Nov 20 capture contains a restart near 10:45:25Z.

```sql
SELECT observed_at, capture_started_at,
       observations_written, observations_discarded, capture_write_failures,
       observations_discarded -
          lag(observations_discarded) OVER (
            PARTITION BY sensor_id, capture_started_at ORDER BY observed_at
          ) AS discarded_since_prior_health_sample
FROM pathfinder.sensor_health
WHERE observed_at >= '2026-11-20 10:00:00+00'
  AND observed_at < '2026-11-20 10:55:00+00'
ORDER BY observed_at;
```

Zero user-space discards alone do not establish perfect packet coverage: inspect
the complete raw_record kernel drop/decode counters and offload qualification.

## Rebuildable connection evidence

Every derived connection must disclose its correlation_version, status and
supporting observation/conntrack source IDs. Source rows are unchanged when a
new correlation algorithm is run.

```sql
SELECT c.connection_id, c.correlation_version, c.correlation_status,
       count(DISTINCT oe.observation_id) AS supporting_packet_records,
       count(DISTINCT ce.event_id) AS supporting_conntrack_events
FROM pathfinder.sensor_connection c
LEFT JOIN pathfinder.sensor_connection_observation_evidence oe
    ON oe.connection_id = c.connection_id
LEFT JOIN pathfinder.sensor_connection_conntrack_evidence ce
    ON ce.connection_id = c.connection_id
WHERE c.first_observed_at < '2026-11-20 10:55:00+00'
  AND c.last_observed_at >= '2026-11-20 10:00:00+00'
GROUP BY c.connection_id, c.correlation_version, c.correlation_status
ORDER BY c.connection_id
LIMIT 500;
```

*The example above is a small-scale contract check; its two evidence joins can
multiply intermediate rows. For large result sets, use separately aggregated
evidence counts or lateral subqueries.*

## Acceptance gates (not yet satisfied)

1. PostgreSQL 18 migration 0008 dry-run/rollback and fresh-install apply.
2. FreeBSD jail validation of full migration chain and role permissions.
3. Immutable source-custody verification for every input batch.
4. Atomic per-batch source record and child-row writes with completion marker.
5. Ingest all 599,984 frozen records, reconcile exact stream and source hashes.
6. Replay identical batches without changing indexed rows; reject a batch ID
   carrying different source bytes.
7. Verify interruption, crash, and network/DB outage recovery cannot produce a
   false completion marker.
8. Benchmark table/index/WAL growth, partition pruning, historical query latency,
   2,000 candidate SNAT tuple mappings, time-bounded identity, and coverage.
9. Implement and validate derived connection/Sighting algorithms separately,
   including ambiguous and unmatched events.

This document is a test plan, not a claim that the database is running.
