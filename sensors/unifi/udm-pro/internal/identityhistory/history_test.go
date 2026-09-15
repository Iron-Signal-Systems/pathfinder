package identityhistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

func TestLoadIndexMatchesRecentSnapshotWithinLease(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Truncate(time.Second)

	snapshot := Snapshot{
		Endpoints: []Endpoint{
			{
				FQDN:           "Johns-Mac-mini.woodsnet.us",
				Hostname:       "Johns-Mac-mini",
				IP:             "192.168.1.193",
				LeaseExpiresAt: base.Add(time.Hour).Format(time.RFC3339),
				MAC:            "44:a9:2c:50:80:76",
			},
		},
		ExpectedIntervalSeconds: 60,
		ObservedAt:              base,
		RecordType:              RecordType,
		SchemaVersion:           SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}
	writeSnapshot(t, filepath.Join(dir, "endpoint-identity-test.jsonl"), snapshot)

	index, stats, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}
	if stats.SnapshotsRead != 1 {
		t.Fatalf("snapshots read = %d, want 1", stats.SnapshotsRead)
	}

	got := index.Lookup(
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		base.Add(30*time.Second),
		LookupOptions{},
	)

	if got.Status != "matched" {
		t.Fatalf("status = %q, want matched", got.Status)
	}
	if got.Hostname != "Johns-Mac-mini" {
		t.Fatalf("hostname = %q", got.Hostname)
	}
	if !got.SnapshotObservedAt.Equal(base) {
		t.Fatalf("snapshot time = %s, want %s", got.SnapshotObservedAt, base)
	}
}

func TestLoadIndexRejectsStaleSnapshot(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)

	snapshot := Snapshot{
		Endpoints: []Endpoint{
			{
				FQDN:           "host.example",
				Hostname:       "host",
				IP:             "192.168.1.10",
				LeaseExpiresAt: base.Add(time.Hour).Format(time.RFC3339),
				MAC:            "00:11:22:33:44:55",
			},
		},
		ExpectedIntervalSeconds: 30,
		ObservedAt:              base,
		RecordType:              RecordType,
		SchemaVersion:           SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}
	writeSnapshot(t, filepath.Join(dir, "endpoint-identity-test.jsonl"), snapshot)

	index, _, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}

	got := index.Lookup(
		"192.168.1.10",
		"00:11:22:33:44:55",
		base.Add(5*time.Minute),
		LookupOptions{},
	)
	if got.Status != "snapshot_too_old" {
		t.Fatalf("status = %q, want snapshot_too_old", got.Status)
	}
}

func TestRecorderWritesImmediateSnapshot(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "dnsmasq.lease")
	hostsPath := filepath.Join(dir, "hosts")
	historyDir := filepath.Join(dir, "identity")

	expiry := time.Now().UTC().Add(time.Hour).Unix()
	leaseLine := []byte(
		fmt.Sprintf(
			"%d 44:a9:2c:50:80:76 192.168.1.193 Johns-Mac-mini 01:44:a9:2c:50:80:76\n",
			expiry,
		),
	)
	if err := os.WriteFile(leasePath, leaseLine, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		hostsPath,
		[]byte("192.168.1.193 Johns-Mac-mini.woodsnet.us\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	recorder, err := NewRecorder(RecorderOptions{
		Compression:  storage.CompressionNone,
		Directory:    historyDir,
		HostsPath:    hostsPath,
		Interval:     time.Hour,
		LeasePath:    leasePath,
		MaxBytes:     1024 * 1024,
		SegmentBytes: 64 * 1024,
	})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	if recorder.Stats().SnapshotsWritten != 1 {
		t.Fatalf(
			"snapshots written = %d, want 1",
			recorder.Stats().SnapshotsWritten,
		)
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	index, stats, err := LoadIndex(historyDir)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}
	if stats.SnapshotsRead != 1 {
		t.Fatalf("snapshots read = %d, want 1", stats.SnapshotsRead)
	}

	got := index.Lookup(
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		time.Now().UTC(),
		LookupOptions{},
	)
	if got.Status != "matched" {
		t.Fatalf("lookup status = %q, want matched", got.Status)
	}
}

func writeSnapshot(t *testing.T, path string, snapshot Snapshot) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestLoadIndexReadsActiveSnapshotSegment(t *testing.T) {
	dir := t.TempDir()
	at := time.Now().UTC().Add(-time.Minute)

	snapshot := Snapshot{
		Endpoints: []Endpoint{
			{
				FQDN:           "host.example.test",
				Hostname:       "host",
				IP:             "192.168.1.50",
				LeaseExpiresAt: at.Add(time.Hour).Format(time.RFC3339),
				MAC:            "00:11:22:33:44:55",
			},
		},
		ExpectedIntervalSeconds: 10,
		ObservedAt:              at,
		RecordType:              RecordType,
		SchemaVersion:           SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}

	writeSnapshot(
		t,
		filepath.Join(dir, "endpoint-identity-live.jsonl.active"),
		snapshot,
	)

	index, stats, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}

	if stats.ActiveFilesRead != 1 {
		t.Fatalf("active files = %d, want 1", stats.ActiveFilesRead)
	}

	got := index.Lookup(
		"192.168.1.50",
		"00:11:22:33:44:55",
		at.Add(time.Second),
		LookupOptions{},
	)
	if got.Status != "matched" {
		t.Fatalf("lookup status = %q, want matched", got.Status)
	}
}
