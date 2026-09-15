package healthhistory

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/capture"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

func TestRecorderWritesLoadableHistory(t *testing.T) {
	dir := t.TempDir()
	healthDir := filepath.Join(dir, "health")

	recorder, err := NewRecorder(RecorderOptions{
		Compression:  storage.CompressionNone,
		Directory:    healthDir,
		Interval:     10 * time.Second,
		MaxBytes:     4 * 1024 * 1024,
		MinFreeBytes: 0,
		SegmentBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}

	started := time.Now().UTC().Add(-time.Minute)
	report := reportForTest(
		started,
		started.Add(10*time.Second),
		10,
		0,
		0,
		0,
		0,
	)
	report.Capture.Interfaces[0].KernelTimestampedFrames = 10
	report.Capture.Interfaces[1].KernelTimestampedFrames = 10

	if err := recorder.Write(report); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	index, stats, err := LoadIndex(healthDir)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}
	if stats.ReportsRead != 1 {
		t.Fatalf("reports read = %d, want 1", stats.ReportsRead)
	}

	got := index.Qualify(
		started.Add(5*time.Second),
		"eth8",
		"br0",
	)
	if got.Status != QualificationHealthy {
		t.Fatalf("qualification = %q, want %q", got.Status, QualificationHealthy)
	}
}

var _ = capture.KernelStatsAvailable
