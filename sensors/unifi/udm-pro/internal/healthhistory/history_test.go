package healthhistory

import (
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/capture"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/conntrackhistory"
)

func TestQualifyHealthyInterval(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(15 * time.Second)

	index := &Index{
		reports: []Report{
			reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0),
			reportForTest(started, started.Add(20*time.Second), 10, 0, 0, 0, 0),
		},
	}

	got := index.Qualify(at, "eth8", "br0")
	if got.Status != QualificationHealthy {
		t.Fatalf("status = %q, want %q", got.Status, QualificationHealthy)
	}
	if got.WANKernelDrops != 0 ||
		got.LANKernelDrops != 0 ||
		got.DecodeRejected != 0 ||
		got.ObservationDiscards != 0 ||
		got.WriteFailures != 0 {
		t.Fatalf("unexpected impairment counters: %+v", got)
	}
}

func TestQualifyDetectsIntervalImpairment(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(15 * time.Second)

	before := reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0)
	after := reportForTest(started, started.Add(20*time.Second), 10, 2, 3, 1, 1)
	after.Capture.Interfaces[0].DecodeRejected = 1

	index := &Index{reports: []Report{before, after}}

	got := index.Qualify(at, "eth8", "br0")
	if got.Status != QualificationImpaired {
		t.Fatalf("status = %q, want %q", got.Status, QualificationImpaired)
	}
	if got.WANKernelDrops != 2 {
		t.Fatalf("WAN drops = %d, want 2", got.WANKernelDrops)
	}
	if got.LANKernelDrops != 3 {
		t.Fatalf("LAN drops = %d, want 3", got.LANKernelDrops)
	}
	if got.DecodeRejected != 1 {
		t.Fatalf("decode rejected = %d, want 1", got.DecodeRejected)
	}
	if got.ObservationDiscards != 1 {
		t.Fatalf("discards = %d, want 1", got.ObservationDiscards)
	}
	if got.WriteFailures != 1 {
		t.Fatalf("write failures = %d, want 1", got.WriteFailures)
	}
}

func TestQualifyCanUseCumulativeFirstReport(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(5 * time.Second)

	index := &Index{
		reports: []Report{
			reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0),
		},
	}

	got := index.Qualify(at, "eth8", "br0")
	if got.Status != QualificationHealthy {
		t.Fatalf("status = %q, want %q", got.Status, QualificationHealthy)
	}
	if !got.CoverageStart.Equal(started) {
		t.Fatalf("coverage start = %v, want %v", got.CoverageStart, started)
	}
}

func TestQualifyRefusesMissingAfterCoverage(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(30 * time.Second)

	index := &Index{
		reports: []Report{
			reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0),
		},
	}

	got := index.Qualify(at, "eth8", "br0")
	if got.Status != QualificationInsufficient {
		t.Fatalf("status = %q, want %q", got.Status, QualificationInsufficient)
	}
}

func TestQualifyConntrackHealthyInterval(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(15 * time.Second)

	before := reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0)
	after := reportForTest(started, started.Add(20*time.Second), 10, 0, 0, 0, 0)
	before.Conntrack = conntrackStatsForTest(started, 0, 0, 0, 0, 0)
	after.Conntrack = conntrackStatsForTest(started, 0, 0, 0, 0, 0)

	index := &Index{reports: []Report{before, after}}
	got := index.QualifyConntrack(at)
	if got.Status != ConntrackQualificationHealthy {
		t.Fatalf("status = %q, want %q", got.Status, ConntrackQualificationHealthy)
	}
}

func TestQualifyConntrackDetectsImpairment(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(15 * time.Second)

	before := reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0)
	after := reportForTest(started, started.Add(20*time.Second), 10, 0, 0, 0, 0)
	before.Conntrack = conntrackStatsForTest(started, 0, 0, 0, 0, 0)
	after.Conntrack = conntrackStatsForTest(started, 1, 2, 3, 4, 5)

	index := &Index{reports: []Report{before, after}}
	got := index.QualifyConntrack(at)
	if got.Status != ConntrackQualificationImpaired {
		t.Fatalf("status = %q, want %q", got.Status, ConntrackQualificationImpaired)
	}
	if got.ReceiveOverruns != 1 || got.DatagramsTruncated != 2 ||
		got.ParseRejected != 3 || got.ReceiveErrors != 4 ||
		got.WriteFailures != 5 {
		t.Fatalf("unexpected conntrack impairment counters: %+v", got)
	}
}

func TestQualifyConntrackDisabled(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	at := started.Add(5 * time.Second)
	report := reportForTest(started, started.Add(10*time.Second), 10, 0, 0, 0, 0)
	index := &Index{reports: []Report{report}}

	got := index.QualifyConntrack(at)
	if got.Status != ConntrackQualificationDisabled {
		t.Fatalf("status = %q, want %q", got.Status, ConntrackQualificationDisabled)
	}
}

func reportForTest(
	started time.Time,
	at time.Time,
	intervalSeconds int64,
	wanDrops uint64,
	lanDrops uint64,
	discards uint64,
	writeFailures uint64,
) Report {
	return Report{
		Capture: capture.Stats{
			Interfaces: []capture.InterfaceStats{
				{
					Interface:              "eth8",
					KernelDrops:            wanDrops,
					KernelStatisticsStatus: capture.KernelStatsAvailable,
				},
				{
					Interface:              "br0",
					KernelDrops:            lanDrops,
					KernelStatisticsStatus: capture.KernelStatsAvailable,
				},
			},
			ObservationsDiscarded: discards,
			StartedAt:             started,
			WriteFailures:         writeFailures,
		},
		ExpectedIntervalSeconds: intervalSeconds,
		ObservedAt:              at,
		RecordType:              RecordType,
		SchemaVersion:           SchemaVersion,
	}
}

func conntrackStatsForTest(
	started time.Time,
	overruns uint64,
	truncated uint64,
	parseRejected uint64,
	receiveErrors uint64,
	writeFailures uint64,
) conntrackhistory.Stats {
	return conntrackhistory.Stats{
		DatagramsTruncated: truncated,
		Enabled:            true,
		ParseRejected:      parseRejected,
		ReceiveErrors:      receiveErrors,
		ReceiveOverruns:    overruns,
		StartedAt:          started,
		WriteFailures:      writeFailures,
	}
}
