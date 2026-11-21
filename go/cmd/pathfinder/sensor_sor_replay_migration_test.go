package main

import (
    "strings"
    "testing"
)

func TestSensorBatchReplayMigrationContract(t *testing.T) {
    item, err := loadMigration(
        "migrations/0009-sensor-batch-replay-indexes.sql",
        "sensor-batch-replay-indexes", 9,
    )
    if err != nil { t.Fatal(err) }
    if item.SHA256 != "54991b3749312f4bc10ed2334e5a336d30a5c507cc1b2dde50df539f1018d370" {
        t.Fatalf("migration 0009 SHA drift: %s", item.SHA256)
    }
    for _, required := range []string{
        "sensor_observation_batch_lookup_idx",
        "sensor_conntrack_batch_lookup_idx",
        "(batch_id, observed_at)",
    } {
        if !strings.Contains(item.SQL, required) { t.Errorf("missing replay index %s", required) }
    }
}
