package sensorsor

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BatchSQL is already validated/preserved source material, not an unauthenticated
// public ingestion endpoint. The generated script commits all child rows and the
// completion marker in one serializable-by-key PostgreSQL transaction.
type BatchSQL struct {
	SensorUUID, SegmentUUID, BatchUUID            string
	Stream, WireBatchID, SchemaVersion            string
	SegmentFilename, SegmentSHA, SegmentReference string
	SegmentBytes, SegmentRecords                  int64
	SHA, Reference                                string
	Bytes                                         int
	PreservedAt                                   time.Time
	Evidence                                      *Evidence
}

func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (b BatchSQL) Validate() error {
	if b.Evidence == nil || b.Evidence.Count == 0 || b.Evidence.Count > 25000 || b.Bytes < 1 || b.Bytes > 8<<20 {
		return fmt.Errorf("invalid batch limits")
	}
	for _, uuid := range []string{b.SensorUUID, b.SegmentUUID, b.BatchUUID} {
		if len(uuid) != 36 {
			return fmt.Errorf("invalid UUID")
		}
	}
	if b.Evidence.First.IsZero() || b.Evidence.Last.IsZero() {
		return fmt.Errorf("missing observed times")
	}
	if b.SegmentRecords <= 0 || b.SegmentBytes <= 0 {
		return fmt.Errorf("missing segment provenance")
	}
	if len(b.SHA) != 64 || len(b.SegmentSHA) != 64 {
		return fmt.Errorf("invalid source hash")
	}
	if b.Reference == "" || b.SegmentReference == "" || b.WireBatchID == "" {
		return fmt.Errorf("missing custody reference")
	}
	switch b.Stream {
	case "observations":
		if b.SchemaVersion != "5" {
			return fmt.Errorf("bad schema")
		}
	case "conntrack", "identity":
		if b.SchemaVersion != "1" {
			return fmt.Errorf("bad schema")
		}
	case "health":
		if b.SchemaVersion != "2" {
			return fmt.Errorf("bad schema")
		}
	default:
		return fmt.Errorf("unknown stream")
	}
	return nil
}

// SQLScript renders the SQL + PostgreSQL COPY stream for a single batch.
// The first guard fails closed on any existing batch identity with different
// payload hash/count/size, and on any conflicting source segment metadata.
// An already complete, matching batch skips COPY entirely (replay = no change).
func (b BatchSQL) SQLScript() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	write := func(f string, args ...any) { fmt.Fprintf(&out, f, args...) }
	ts := func(t time.Time) string { return Quote(t.UTC().Format(time.RFC3339Nano)) }
	write("\\set ON_ERROR_STOP on\nBEGIN;\nSELECT pg_advisory_xact_lock(hashtextextended(%s,0));\n", Quote("sensor-index-v1:"+b.SensorUUID+":"+b.Stream+":"+b.WireBatchID))
	write("INSERT INTO pathfinder.sensor_segment (segment_id,sensor_id,stream,source_filename,stored_sha256,stored_bytes,record_count,custody_reference) VALUES (%s,%s,%s,%s,%s,%d,%d,%s) ON CONFLICT (sensor_id,stream,stored_sha256) DO NOTHING;\n",
		Quote(b.SegmentUUID), Quote(b.SensorUUID), Quote(b.Stream), Quote(b.SegmentFilename), Quote(b.SegmentSHA), b.SegmentBytes, b.SegmentRecords, Quote(b.SegmentReference))
	write(`DO $sensor_guard$
BEGIN
 IF NOT EXISTS (
     SELECT 1 FROM pathfinder.sensor_segment
      WHERE segment_id=%s AND sensor_id=%s AND stream=%s
        AND stored_sha256=%s AND source_filename=%s
        AND stored_bytes=%d AND record_count=%d AND custody_reference=%s
 ) THEN RAISE EXCEPTION 'sensor source segment identity/metadata conflict'; END IF;
 IF EXISTS (
     SELECT 1 FROM pathfinder.sensor_batch
      WHERE sensor_id=%s AND stream=%s AND wire_batch_id=%s
        AND (batch_id<>%s OR payload_sha256<>%s
         OR payload_bytes<>%d OR record_count<>%d
         OR segment_id IS DISTINCT FROM %s
         OR custody_reference<>%s OR schema_version<>%s
         OR first_observed_at<>%s OR last_observed_at<>%s)
 ) THEN RAISE EXCEPTION 'sensor batch replay identity/metadata conflict'; END IF;
END;
$sensor_guard$;
`,
		Quote(b.SegmentUUID), Quote(b.SensorUUID), Quote(b.Stream), Quote(b.SegmentSHA), Quote(b.SegmentFilename), b.SegmentBytes, b.SegmentRecords, Quote(b.SegmentReference),
		Quote(b.SensorUUID), Quote(b.Stream), Quote(b.WireBatchID), Quote(b.BatchUUID), Quote(b.SHA), b.Bytes, b.Evidence.Count, Quote(b.SegmentUUID), Quote(b.Reference), Quote(b.SchemaVersion), ts(b.Evidence.First), ts(b.Evidence.Last))
	write(`DO $replay_guard$
BEGIN
 IF EXISTS (SELECT 1 FROM pathfinder.sensor_batch_index_completion WHERE batch_id=%s) THEN
    RAISE EXCEPTION 'batch completed concurrently; retry import to verify replay';
 END IF;
 IF EXISTS (SELECT 1 FROM pathfinder.sensor_batch WHERE batch_id=%s) THEN
    RAISE EXCEPTION 'batch metadata exists without completion; manual recovery required';
 END IF;
END;
$replay_guard$;
`, Quote(b.BatchUUID), Quote(b.BatchUUID))
	write("INSERT INTO pathfinder.sensor_batch (batch_id,sensor_id,stream,wire_batch_id,schema_version,payload_sha256,payload_bytes,record_count,preserved_at,custody_reference,segment_id,first_observed_at,last_observed_at) VALUES (%s,%s,%s,%s,%s,%s,%d,%d,%s,%s,%s,%s,%s) ON CONFLICT (sensor_id,stream,wire_batch_id) DO NOTHING;\n",
		Quote(b.BatchUUID), Quote(b.SensorUUID), Quote(b.Stream), Quote(b.WireBatchID), Quote(b.SchemaVersion), Quote(b.SHA), b.Bytes, b.Evidence.Count, ts(b.PreservedAt), Quote(b.Reference), Quote(b.SegmentUUID), ts(b.Evidence.First), ts(b.Evidence.Last))
	// Parent first, then children. No dependency-crossing COPY ordering.
	for _, name := range []string{"sensor_observation", "sensor_dns_question", "sensor_dns_record", "sensor_conntrack_event", "sensor_identity_snapshot", "sensor_identity_endpoint", "sensor_health"} {
		table := b.Evidence.Tables[name]
		if table == nil || len(table.Rows) == 0 {
			continue
		}
		write("COPY pathfinder.%s (%s) FROM STDIN WITH (FORMAT csv, NULL '\\N');\n", name, strings.Join(table.Names, ","))
		writer := csv.NewWriter(&out)
		for _, row := range table.Rows {
			if err := writer.Write(row); err != nil {
				return nil, err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return nil, err
		}
		write("\\.\n")
	}
	write(`DO $completion_guard$
BEGIN
 IF (SELECT count(*) FROM pathfinder.%s WHERE batch_id=%s)<>%d THEN
    RAISE EXCEPTION 'sensor batch incomplete: evidence row count mismatch';
 END IF;
END;
$completion_guard$;
`, b.baseTable(), Quote(b.BatchUUID), b.Evidence.Count)
	// Cross-check child rows, not only the 1:1 source evidence table.
	for _, child := range []struct{ name, parent, clause string }{
		{"sensor_dns_question", "sensor_observation", "d.observation_at=o.observed_at AND d.observation_id=o.observation_id"},
		{"sensor_dns_record", "sensor_observation", "d.observation_at=o.observed_at AND d.observation_id=o.observation_id"},
		{"sensor_identity_endpoint", "sensor_identity_snapshot", "d.snapshot_id=o.snapshot_id"},
	} {
		rows := 0
		if table := b.Evidence.Tables[child.name]; table != nil {
			rows = len(table.Rows)
		}
		if (b.Stream != "observations" && child.parent == "sensor_observation") ||
			(b.Stream != "identity" && child.parent == "sensor_identity_snapshot") {
			continue
		}
		write(`DO $child_guard$
BEGIN
 IF (SELECT count(*) FROM pathfinder.%s d JOIN pathfinder.%s o
      ON %s WHERE o.batch_id=%s)<>%d THEN
    RAISE EXCEPTION 'sensor batch child record count mismatch: %s';
 END IF;
END;
$child_guard$;
`, child.name, child.parent, child.clause, Quote(b.BatchUUID), rows, child.name)
	}
	write("INSERT INTO pathfinder.sensor_batch_index_completion (batch_id,indexed_record_count,indexer_version) VALUES (%s,%d,%s);\n",
		Quote(b.BatchUUID), b.Evidence.Count, Quote("sensorsor-copy-v1"))
	write("COMMIT;\n\\echo SENSOR_SOR_INDEXED %s count=%s\n", b.WireBatchID, strconv.Itoa(b.Evidence.Count))
	return out.Bytes(), nil
}
func (b BatchSQL) baseTable() string {
	switch b.Stream {
	case "observations":
		return "sensor_observation"
	case "conntrack":
		return "sensor_conntrack_event"
	case "identity":
		return "sensor_identity_snapshot"
	default:
		return "sensor_health"
	}
}
