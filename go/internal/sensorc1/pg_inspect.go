package sensorc1

import (
 "fmt"
 "strings"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

func pgInspectQuery(b sensorsor.BatchSQL) string {
 q:=sensorsor.Quote
 ts:=func(t time.Time)string{return q(t.UTC().Format(time.RFC3339Nano))}
 // Collect BOTH natural batch key and primary batch UUID. A stolen or
 // mismatched UUID with the same source key must be reported as CONFLICT.
 // Likewise, a reused segment UUID/hash never silently becomes a new source.
 return fmt.Sprintf(`
WITH candidates AS (
 SELECT b.*, c.indexed_record_count
 FROM pathfinder.sensor_batch b
 LEFT JOIN pathfinder.sensor_batch_index_completion c ON c.batch_id=b.batch_id
 WHERE b.batch_id=%s::uuid
    OR (b.sensor_id=%s::uuid AND b.stream=%s AND b.wire_batch_id=%s)
),
segments AS (
 SELECT s.* FROM pathfinder.sensor_segment s
 WHERE s.segment_id=%s::uuid
    OR (s.sensor_id=%s::uuid AND s.stream=%s AND s.stored_sha256=%s)
)
SELECT CASE
 WHEN (SELECT count(*) FROM candidates)>1 OR (SELECT count(*) FROM segments)>1
 THEN 'conflict'
 WHEN EXISTS (
  SELECT 1 FROM candidates b WHERE
   b.batch_id IS DISTINCT FROM %s::uuid
   OR b.sensor_id IS DISTINCT FROM %s::uuid
   OR b.stream IS DISTINCT FROM %s
   OR b.wire_batch_id IS DISTINCT FROM %s
   OR b.schema_version IS DISTINCT FROM %s
   OR b.payload_sha256 IS DISTINCT FROM %s
   OR b.payload_bytes IS DISTINCT FROM %d
   OR b.record_count IS DISTINCT FROM %d
   OR b.segment_id IS DISTINCT FROM %s::uuid
   OR b.custody_reference IS DISTINCT FROM %s
   OR b.first_observed_at IS DISTINCT FROM %s::timestamptz
   OR b.last_observed_at IS DISTINCT FROM %s::timestamptz
   OR (b.indexed_record_count IS NOT NULL AND b.indexed_record_count<>%d)
 ) THEN 'conflict'
 WHEN EXISTS (
  SELECT 1 FROM segments s WHERE
   s.segment_id IS DISTINCT FROM %s::uuid
   OR s.sensor_id IS DISTINCT FROM %s::uuid
   OR s.stream IS DISTINCT FROM %s
   OR s.source_filename IS DISTINCT FROM %s
   OR s.stored_sha256 IS DISTINCT FROM %s
   OR s.stored_bytes IS DISTINCT FROM %d
   OR s.record_count IS DISTINCT FROM %d
   OR s.custody_reference IS DISTINCT FROM %s
 ) THEN 'conflict'
 WHEN EXISTS (SELECT 1 FROM candidates WHERE indexed_record_count IS NULL)
 THEN 'partial'
 WHEN EXISTS (SELECT 1 FROM candidates WHERE indexed_record_count=%d)
 THEN 'complete'
 ELSE 'new'
END;
`,
  q(b.BatchUUID),q(b.SensorUUID),q(b.Stream),q(b.WireBatchID),
  q(b.SegmentUUID),q(b.SensorUUID),q(b.Stream),q(b.SegmentSHA),
  q(b.BatchUUID),q(b.SensorUUID),q(b.Stream),q(b.WireBatchID),
  q(b.SchemaVersion),q(b.SHA),b.Bytes,b.Evidence.Count,
  q(b.SegmentUUID),q(b.Reference),ts(b.Evidence.First),ts(b.Evidence.Last),b.Evidence.Count,
  q(b.SegmentUUID),q(b.SensorUUID),q(b.Stream),q(b.SegmentFilename),
  q(b.SegmentSHA),b.SegmentBytes,b.SegmentRecords,q(b.SegmentReference),
  b.Evidence.Count,
 )
}

func stateQueryIsReadOnly(query string)bool{
 upper:=strings.ToUpper(query)
 return strings.Contains(upper,"SELECT CASE")&&
  !strings.Contains(upper,"INSERT INTO")&&
  !strings.Contains(upper,"DELETE FROM")&&
  !strings.Contains(upper,"UPDATE PATHFINDER")
}
