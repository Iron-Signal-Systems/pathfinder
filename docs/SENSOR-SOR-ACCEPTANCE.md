# Pathfinder Sensor SoR — query and acceptance contracts

See [the frozen-archive backfill runbook](SENSOR-SOR-BACKFILL-RUNBOOK.md) for the operator-executable process, custody configuration, database role boundaries, exact full-file record counts, and PostgreSQL SNI output.

**Development-only design and query examples.** These SQL statements are
repo-owned acceptance targets; no live PostgreSQL execution is claimed.

Reference: [DB Data Dictionary](DB-DATA-DICTIONARY.md), migration
`go/cmd/pathfinder/migrations/0008-sensor-observation-sor.sql`.

The example range covers the UDM-Pro sample:
2026-11-20 10:00:00Z inclusive through 10:55:00Z exclusive.

## Mandatory full-archive ingest and PostgreSQL SNI acceptance

**Operator requirement (2026-11-21):** Once sensor SoR indexing is ready,
ingest **everything in the existing frozen UDM-Pro archive on C220**, then
return **every distinct observed TLS SNI hostname by querying PostgreSQL**,
not by reporting names read from JSONL or a sidecar file.

Full existing nine-file archive (not merely the previously analyzed 55-minute
slice):

| Evidence stream | Full source-file rows |
|---|---:|
| observations | 569,671 |
| conntrack | 111,842 |
| identity snapshots | 2,446 |
| health reports | 1,259 |
| **TOTAL** | **685,218** |

These are archive-source counts. The November 20 10:00Z–10:55Z window
accounts for 599,984 of them. Preserve all nine original capture files
and ingest every source record, including earlier identity/health history.
Never silently apply the analysis-window filter to the actual import.

Acceptance must prove:

1. Each frozen source segment has the verified stored-file SHA-256 from the
   original read-only audit. The import reads source bytes without modifying
   the archive.
2. Every source line is accounted for in exactly one SoR evidence row, with
   its original stream, authenticated sensor association, batch identity,
   ordinal and timestamp.
3. The four total relational evidence-row counts equal the full archive
   counts above; the earlier 55-minute filtered counts also reconcile.
4. The same frozen archive is replayed. Row counts stay **identical**,
   individual original records stay unchanged, and batch-index completion
   markers remain stable. Conflicting batch identities fail closed.
5. The distinct TLS SNI result is produced by executing
   `go/queries/sensor_unique_sni.sql` against the **committed PostgreSQL
   `pathfinder.sensor_observation.tls_sni` column**. No JSONL, JSONB-only,
   Python aggregation or archive scan may substitute for this final query.
6. The query returns all indexed names, not a top-N sample, and reports
   the unique count along with the complete ordered values; the output is
   captured as a test artifact. No claim of completion is made until an
   actual SQL query against the imported database succeeds.

Example after the database and migration are installed, using an
appropriately authenticated database account (credentials supplied by
the installation, not embedded in commands):

```sh
psql -X -v ON_ERROR_STOP=1 -d pathfinder \
  -f go/queries/sensor_unique_sni.sql
```

**Current state:** This query is committed and the SoR schema exists in
source control. The import/indexer, PostgreSQL execution and full-archive
reconciliation have not yet passed acceptance.

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
