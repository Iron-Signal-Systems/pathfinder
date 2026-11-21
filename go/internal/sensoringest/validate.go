// Package sensoringest validates source records before preservation or interpretation.
// It does not authenticate sensors, issue acknowledgements, or create Sightings.
package sensoringest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Stream string

const (
	Observations Stream = "observations"
	Conntrack Stream = "conntrack"
	Identity Stream = "identity"
	Health Stream = "health"
	MaxRecordBytes = 4 << 20
)

type Record struct {
	Stream Stream
	SchemaVersion string
	ObservedAt time.Time
	SensorID string // Only embedded in packet observations; not authenticated here.
}

var (
	ErrMalformed = errors.New("malformed sensor record")
	ErrUnsupported = errors.New("unsupported sensor record schema")
	ErrSensorMismatch = errors.New("sensor identifier mismatch")
)

// Validate checks the current sensor wire profiles. expectedSensorID is only
// a consistency check; transport authentication belongs to the receiver.
func Validate(stream Stream, raw []byte, expectedSensorID string) (Record, error) {
	if len(raw) == 0 || len(raw) > MaxRecordBytes {
		return Record{}, fmt.Errorf("%w: record size %d", ErrMalformed, len(raw))
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Record{}, fmt.Errorf("%w: JSON object required", ErrMalformed)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return Record{}, fmt.Errorf("%w: invalid JSON object: %v", ErrMalformed, err)
	}
	schema, err := textField(fields, "schema_version")
	if err != nil { return Record{}, err }

	var version, recordType string
	switch stream {
	case Observations:
		version = "5"
	case Conntrack:
		version, recordType = "1", "conntrack_event"
	case Identity:
		version, recordType = "1", "endpoint_identity_snapshot"
	case Health:
		version, recordType = "2", "sensor_health"
	default:
		return Record{}, fmt.Errorf("%w: unknown stream %q", ErrUnsupported, stream)
	}
	if schema != version {
		return Record{}, fmt.Errorf("%w: stream=%s version=%q", ErrUnsupported, stream, schema)
	}
	if recordType != "" {
		got, err := textField(fields, "record_type")
		if err != nil { return Record{}, err }
		if got != recordType {
			return Record{}, fmt.Errorf("%w: stream=%s record_type=%q", ErrMalformed, stream, got)
		}
	}
	observedText, err := textField(fields, "observed_at")
	if err != nil { return Record{}, err }
	observed, err := time.Parse(time.RFC3339Nano, observedText)
	if err != nil || observed.IsZero() {
		return Record{}, fmt.Errorf("%w: invalid observed_at", ErrMalformed)
	}
	result := Record{Stream: stream, SchemaVersion: schema, ObservedAt: observed}
	switch stream {
	case Observations:
		id, err := textField(fields, "sensor_id")
		if err != nil { return Record{}, err }
		if id == "not_known" || id == "no_record" {
			return Record{}, fmt.Errorf("%w: unconfigured sensor_id", ErrMalformed)
		}
		if expectedSensorID != "" && id != expectedSensorID {
			return Record{}, ErrSensorMismatch
		}
		result.SensorID = id
		for _, key := range []string{"observed", "context", "enrichment"} {
			if err := objectField(fields, key); err != nil { return Record{}, err }
		}
	case Conntrack:
		for _, key := range []string{"original", "reply", "original_counters", "reply_counters"} {
			if err := objectField(fields, key); err != nil { return Record{}, err }
		}
		event, err := textField(fields, "event_type")
		if err != nil { return Record{}, err }
		if event != "new" && event != "update" && event != "destroy" {
			return Record{}, fmt.Errorf("%w: unknown conntrack event_type", ErrMalformed)
		}
	case Identity:
		if err := arrayField(fields, "endpoints"); err != nil { return Record{}, err }
	case Health:
		for _, key := range []string{"capture", "conntrack", "identity", "storage"} {
			if err := objectField(fields, key); err != nil { return Record{}, err }
		}
	}
	return result, nil
}

func textField(fields map[string]json.RawMessage, name string) (string, error) {
	raw, ok := fields[name]
	if !ok { return "", fmt.Errorf("%w: missing %s", ErrMalformed, name) }
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", fmt.Errorf("%w: invalid %s", ErrMalformed, name)
	}
	return value, nil
}

func objectField(fields map[string]json.RawMessage, name string) error {
	raw, ok := fields[name]
	if !ok || len(raw) == 0 || raw[0] != '{' {
		return fmt.Errorf("%w: %s must be object", ErrMalformed, name)
	}
	return nil
}

func arrayField(fields map[string]json.RawMessage, name string) error {
	raw, ok := fields[name]
	if !ok || len(raw) == 0 || raw[0] != '[' {
		return fmt.Errorf("%w: %s must be array", ErrMalformed, name)
	}
	return nil
}
