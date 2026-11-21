package main

import (
	"strings"
	"testing"
)

// These checks protect the committed sensor SoR contract from accidental
// omission. They do not replace running the DDL in PostgreSQL 18.
func TestSensorSORMigrationContract(t *testing.T) {
	item, err := loadMigration(
		"migrations/0008-sensor-observation-sor.sql",
		"sensor-observation-sor", 8,
	)
	if err != nil { t.Fatal(err) }
	if item.SHA256 != "dd8bcb02b46aefc1cd30ce8846ffa5155624238ddf29828d42f4e93aaba62b1d" {
		t.Fatalf("sensor SoR migration checksum drift: %s", item.SHA256)
	}
	for _, table := range []string{
		"sensor", "sensor_segment", "sensor_batch", "sensor_batch_index_completion",
		"sensor_observation", "sensor_dns_question", "sensor_dns_record",
		"sensor_conntrack_event", "sensor_identity_snapshot",
		"sensor_identity_endpoint", "sensor_health", "sensor_connection",
		"sensor_connection_observation_evidence",
		"sensor_connection_conntrack_evidence",
	} {
		if !strings.Contains(item.SQL, "CREATE TABLE pathfinder."+table+" (") {
			t.Errorf("missing sensor SoR table: %s", table)
		}
	}
	for _, required := range []string{
		"PARTITION BY RANGE (observed_at)",
		"UNIQUE (sensor_id, stream, wire_batch_id)",
		"FOREIGN KEY (batch_id, sensor_id, stream)",
		"REFERENCES pathfinder.sensor_batch(batch_id, sensor_id, stream)",
		"REFERENCES pathfinder.sensor_observation(observed_at, observation_id)",
		"REFERENCES pathfinder.sensor_conntrack_event(observed_at, event_id)",
		"raw_record jsonb NOT NULL",
		"observed_at_ns bigint NOT NULL",
		"GRANT SELECT ON pathfinder.sensor TO pathfinder_app",
		"CREATE TABLE pathfinder.sensor_observation_default PARTITION",
		"CREATE TABLE pathfinder.sensor_conntrack_event_default PARTITION",
	} {
		if !strings.Contains(item.SQL, required) {
			t.Errorf("missing sensor SoR contract: %s", required)
		}
	}
}
