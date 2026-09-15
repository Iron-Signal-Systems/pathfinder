package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewDoesNotMarshalNullCollections(t *testing.T) {
	record := New()

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if strings.Contains(string(data), "null") {
		t.Fatalf("serialized observation contains null: %s", data)
	}
}

func TestNewSetsSchemaVersion(t *testing.T) {
	record := New()

	if record.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", record.SchemaVersion, SchemaVersion)
	}
}

func TestNewSetsTimestampSourceUnknown(t *testing.T) {
	record := New()

	if record.TimestampSource != ValueNotKnown {
		t.Fatalf(
			"TimestampSource = %q, want %q",
			record.TimestampSource,
			ValueNotKnown,
		)
	}
}

func TestNewInitializesTLSWithoutNullSemantics(t *testing.T) {
	current := New()

	if current.SchemaVersion != "5" {
		t.Fatalf("schema version = %q, want 5", current.SchemaVersion)
	}
	if current.Observed.TLS.ClientHelloObserved {
		t.Fatal("ClientHelloObserved = true, want false")
	}
	if current.Observed.TLS.SNI != ValueNoRecord {
		t.Fatalf(
			"TLS SNI = %q, want %q",
			current.Observed.TLS.SNI,
			ValueNoRecord,
		)
	}
}
