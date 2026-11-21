package sensorsor

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var ctx = Context{SensorUUID: "11111111-2222-4333-8444-555555555555", BatchUUID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", StartOrdinal: 1}
var records = map[string]string{
	"observations": `{"sensor_id":"udm-1","schema_version":"5","observed_at":"2026-11-20T10:00:00.123456789Z","timestamp_source":"kernel_software_ns","context":{"direction":"ingress","vlan_id":0,"zone":"lan","kernel_offload_suspected":false},"observed":{"interface":"br0","interface_index":2,"captured_length":128,"network":{"family":"ipv4","protocol":"tcp","source_ip":"192.0.2.10","destination_ip":"198.51.100.23"},"transport":{"source_port":443,"destination_port":40000},"ethernet":{"source_mac":"aa:bb:cc:dd:ee:ff","destination_mac":"11:22:33:44:55:66","ethertype":"0x0800"},"tls":{"client_hello_observed":true,"sni":"example.org"},"dns":{"message_type":"response","query_name":"example.org","query_type":"A","questions":[{"name":"example.org","type":"A"}],"answers":[{"name":"example.org","type":"A","class":"IN","ttl":120,"value":"198.51.100.23"}]},"arp":{"operation":"no_record"}},"enrichment":{}}`,
	"conntrack":    `{"record_type":"conntrack_event","schema_version":"1","observed_at":"2026-11-20T10:00:00Z","timestamp_source":"userspace_receive","id":42,"event_type":"new","source_nat":true,"destination_nat":false,"assured":false,"confirmed":true,"seen_reply":true,"dying":false,"offload":false,"hardware_offload":false,"timeout_seconds":60,"status":13,"mark":0,"tcp_state":"ESTABLISHED","original":{"family":"ipv4","protocol":"tcp","source_ip":"192.0.2.1","destination_ip":"198.51.100.2","source_port":50000,"destination_port":443},"reply":{"family":"ipv4","protocol":"tcp","source_ip":"198.51.100.2","destination_ip":"203.0.113.1","source_port":443,"destination_port":65000},"original_counters":{"present":true,"packets":10,"bytes":500},"reply_counters":{"present":false,"packets":0,"bytes":0}}`,
	"identity":     `{"record_type":"endpoint_identity_snapshot","schema_version":"1","observed_at":"2026-11-20T10:00:00Z","expected_interval_seconds":60,"source":"dnsmasq","source_path":"/run/dnsmasq","endpoints":[{"ip":"192.0.2.12","mac":"aa:bb:cc:dd:ee:ff","hostname":"test-1","fqdn":"test-1.example","lease_expires_at":"2026-11-20T12:00:00Z"}]}`,
	"health":       `{"record_type":"sensor_health","schema_version":"2","observed_at":"2026-11-20T10:00:00Z","expected_interval_seconds":60,"final":false,"capture":{"started_at":"2026-11-20T09:00:00Z","observations_written":5000,"observations_discarded":0,"write_failures":0},"conntrack":{"enabled":true,"events_received":30,"receive_overruns":0},"identity":{},"storage":{}}`,
}

func TestProjectFourStreams(t *testing.T) {
	for stream, raw := range records {
		t.Run(stream, func(t *testing.T) {
			e := New()
			if err := e.Append(stream, ctx, 12, []byte(raw)); err != nil {
				t.Fatal(err)
			}
			if e.Count != 1 {
				t.Fatal(e.Count)
			}
			if e.First.UTC().Format(time.RFC3339) != "2026-11-20T10:00:00Z" {
				t.Fatalf("timestamp unexpected %s", e.First)
			}
			var table string
			switch stream {
			case "observations":
				table = "sensor_observation"
			case "conntrack":
				table = "sensor_conntrack_event"
			case "identity":
				table = "sensor_identity_snapshot"
			default:
				table = "sensor_health"
			}
			if len(e.Tables[table].Rows) != 1 {
				t.Fatalf("base table missing")
			}
			switch stream {
			case "observations":
				row := e.Tables[table].Rows[0]
				if row[6] != "1795168800123456789" {
					t.Fatalf("precision lost: %s", row[6])
				}
				if row[24] != "128" || row[29] != "example.org" {
					t.Fatalf("bad packet projection: %v", row)
				}
				if len(e.Tables["sensor_dns_question"].Rows) != 1 || len(e.Tables["sensor_dns_record"].Rows) != 1 {
					t.Fatal("missing DNS child")
				}
			case "conntrack":
				row := e.Tables[table].Rows[0]
				if row[22] != "t" {
					t.Fatalf("bad SNAT boolean %v", row)
				}
			case "identity":
				if len(e.Tables["sensor_identity_endpoint"].Rows) != 1 {
					t.Fatal("missing endpoint child")
				}
			}
		})
	}
}
func TestProjectRejectsInvalidValues(t *testing.T) {
	e := New()
	raw := strings.Replace(records["observations"], "192.0.2.10", "this is not an IP", 1)
	if err := e.Append("observations", ctx, 1, []byte(raw)); err == nil {
		t.Fatal("invalid source IP accepted")
	}
	e = New()
	raw = strings.Replace(records["health"], "2026-11-20T09:00:00Z", "no_record", 1)
	if err := e.Append("health", ctx, 1, []byte(raw)); err == nil {
		t.Fatal("missing required capture session accepted")
	}
}
func TestCSVAndAtomicMarker(t *testing.T) {
	e := New()
	if err := e.Append("observations", ctx, 1, []byte(records["observations"])); err != nil {
		t.Fatal(err)
	}
	meta := BatchSQL{SensorUUID: ctx.SensorUUID, SegmentUUID: "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee", BatchUUID: ctx.BatchUUID, Stream: "observations", WireBatchID: "test-batch-0001", SchemaVersion: "5", SegmentFilename: "test.jsonl.active", SegmentSHA: strings.Repeat("a", 64), SegmentReference: "/private/frozen/test.jsonl.active", SegmentBytes: 100, SegmentRecords: 1, SHA: strings.Repeat("b", 64), Reference: "/private/custody/batch.json", Bytes: 128, PreservedAt: time.Now(), Evidence: e}
	script, err := meta.SQLScript()
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"BEGIN;", "pg_advisory_xact_lock", "source segment identity/metadata conflict", "sensor batch replay identity/metadata conflict", "COPY pathfinder.sensor_observation", "COPY pathfinder.sensor_dns_question", "COPY pathfinder.sensor_dns_record", "sensor_batch_index_completion", "COMMIT;"} {
		if !bytes.Contains(script, []byte(part)) {
			t.Errorf("missing %q", part)
		}
	}
	// A real PostgreSQL COPY record must be well-formed CSV, not raw interpolated SQL.
	lines := bytes.Split(script, []byte("\n"))
	var csvRow []byte
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("COPY pathfinder.sensor_observation ")) {
			csvRow = lines[i+1]
			break
		}
	}
	rdr := csv.NewReader(bytes.NewReader(csvRow))
	row, err := rdr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != len(Schema["sensor_observation"]) {
		t.Fatalf("CSV fields=%d want %d", len(row), len(Schema["sensor_observation"]))
	}
}

func TestProjectionColumnsExistInMigration0008(t *testing.T) {
	migration := filepath.Join("..", "..", "cmd", "pathfinder", "migrations", "0008-sensor-observation-sor.sql")
	body, err := os.ReadFile(migration)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for table, columns := range Schema {
		start := strings.Index(sql, "CREATE TABLE pathfinder."+table+" (")
		if start < 0 {
			t.Errorf("migration 0008 missing projection table %s", table)
			continue
		}
		block := sql[start:]
		end := strings.Index(block, "\n);")
		if end < 0 {
			end = strings.Index(block, "\n) PARTITION BY RANGE (observed_at);")
		}
		if end < 0 {
			t.Errorf("migration 0008 malformed projection table %s", table)
			continue
		}
		block = block[:end]
		for _, name := range columns {
			if !strings.Contains(block, "\n    "+name+" ") {
				t.Errorf("migration 0008 missing projected column %s.%s", table, name)
			}
		}
	}
}
