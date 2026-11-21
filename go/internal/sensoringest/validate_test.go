package sensoringest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const stamp = `"observed_at":"2026-11-20T10:45:25Z"`

func TestRecordProfiles(t *testing.T) {
	tests := []struct{ stream Stream; source string }{
		{Observations, "{"+stamp+`,"schema_version":"5","sensor_id":"sensor-a","observed":{},"context":{},"enrichment":{}}`},
		{Conntrack, "{"+stamp+`,"schema_version":"1","record_type":"conntrack_event","event_type":"new","original":{},"reply":{},"original_counters":{},"reply_counters":{}}`},
		{Identity, "{"+stamp+`,"schema_version":"1","record_type":"endpoint_identity_snapshot","endpoints":[]}`},
		{Health, "{"+stamp+`,"schema_version":"2","record_type":"sensor_health","capture":{},"conntrack":{},"identity":{},"storage":{}}`},
	}
	for _, tc := range tests {
		t.Run(string(tc.stream), func(t *testing.T) {
			rec, err := Validate(tc.stream, []byte(tc.source), "sensor-a")
			if err != nil { t.Fatal(err) }
			if rec.Stream != tc.stream || rec.ObservedAt.IsZero() { t.Fatalf("invalid result: %#v", rec) }
		})
	}
}

func TestRejectUnknownAndMalformed(t *testing.T) {
	source := "{"+stamp+`,"schema_version":"5","sensor_id":"sensor-a","observed":{},"context":{},"enrichment":{}}`
	cases := []struct { raw string; want error }{
		{strings.Replace(source, `"5"`, `"6"`, 1), ErrUnsupported},
		{strings.Replace(source, `"sensor-a"`, `"sensor-b"`, 1), ErrSensorMismatch},
		{strings.Replace(source, `"observed":{}`, `"observed":null`, 1), ErrMalformed},
		{`[1,2,3]`, ErrMalformed},
		{``, ErrMalformed},
		{`   `, ErrMalformed},
		{`{not json}`, ErrMalformed},
		{strings.Replace(source, `"2026-11-20T10:45:25Z"`, `"tomorrow"`, 1), ErrMalformed},
	}
	for i, tc := range cases {
		_, err := Validate(Observations, []byte(tc.raw), "sensor-a")
		if !errors.Is(err, tc.want) { t.Errorf("case %d: got %v want %v", i, err, tc.want) }
	}
}

func TestAuditReplayDeterminism(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observations-1.jsonl.active")
	source := "{"+stamp+`,"schema_version":"5","sensor_id":"sensor-a","observed":{},"context":{},"enrichment":{}}`+"\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil { t.Fatal(err) }
	a, err := AuditFile(path, Observations, "sensor-a", time.Time{}, time.Time{})
	if err != nil { t.Fatal(err) }
	b, err := AuditFile(path, Observations, "sensor-a", time.Time{}, time.Time{})
	if err != nil { t.Fatal(err) }
	if a.SHA256 != b.SHA256 || a.Total != 1 || b.WithinWindow != 1 { t.Fatalf("unexpected audit %v %v", a, b) }
	if err := os.WriteFile(path, []byte(strings.TrimSuffix(source, "\n")), 0600); err != nil { t.Fatal(err) }
	if _, err := AuditFile(path, Observations, "sensor-a", time.Time{}, time.Time{}); err == nil {
		t.Fatal("expected incomplete trailing line failure")
	}
}
