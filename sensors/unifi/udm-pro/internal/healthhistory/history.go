// Package healthhistory persists and queries immutable sensor-health snapshots.
package healthhistory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/capture"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/conntrackhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/segmentread"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

const (
	RecordType    = "sensor_health"
	SchemaVersion = "2"

	maxScannerToken = 4 * 1024 * 1024

	QualificationHealthy      = "healthy_for_observation_interval"
	QualificationImpaired     = "capture_impairment_observed"
	QualificationInsufficient = "insufficient_health_coverage"
	QualificationUnavailable  = "health_history_unavailable"

	ConntrackQualificationDisabled     = "conntrack_history_disabled"
	ConntrackQualificationHealthy      = "healthy_for_observation_interval"
	ConntrackQualificationImpaired     = "conntrack_impairment_observed"
	ConntrackQualificationInsufficient = "insufficient_health_coverage"
	ConntrackQualificationUnavailable  = "health_history_unavailable"
)

// Storage captures rolling observation-storage state at the report time.
type Storage struct {
	Enabled bool          `json:"enabled"`
	Stats   storage.Stats `json:"stats"`
}

// Report is one immutable point-in-time sensor-health snapshot.
type Report struct {
	Capture                 capture.Stats          `json:"capture"`
	Conntrack               conntrackhistory.Stats `json:"conntrack"`
	ExpectedIntervalSeconds int64                  `json:"expected_interval_seconds"`
	Final                   bool                   `json:"final"`
	Identity                identityhistory.Stats  `json:"identity"`
	ObservedAt              time.Time              `json:"observed_at"`
	RecordType              string                 `json:"record_type"`
	SchemaVersion           string                 `json:"schema_version"`
	Storage                 Storage                `json:"storage"`
}

// RecorderOptions configures persisted health history.
type RecorderOptions struct {
	Compression  string
	Directory    string
	Interval     time.Duration
	MaxBytes     uint64
	MinFreeBytes uint64
	OnError      func(error)
	SegmentBytes uint64
}

// Recorder writes reports to a separate bounded immutable rolling store.
type Recorder struct {
	interval  time.Duration
	ring      *storage.Ring
	writeErrs atomic.Uint64
	writes    atomic.Uint64
}

// RecorderStats describes persisted health-history writes.
type RecorderStats struct {
	Enabled         bool          `json:"enabled"`
	IntervalSeconds int64         `json:"interval_seconds"`
	ReportsWritten  uint64        `json:"reports_written"`
	Storage         storage.Stats `json:"storage"`
	WriteFailures   uint64        `json:"write_failures"`
}

// LoadStats describes health-history source material.
type LoadStats struct {
	ActiveFilesRead                uint64 `json:"active_files_read"`
	ActiveTrailingFragmentsIgnored uint64 `json:"active_trailing_fragments_ignored"`
	FilesRead                      uint64 `json:"files_read"`
	ReportsRead                    uint64 `json:"reports_read"`
	SegmentsVanishedDuringRead     uint64 `json:"segments_vanished_during_read"`
}

// Qualification reports whether capture-integrity counters support a
// "not observed" statement around one packet observation.
type Qualification struct {
	CoverageEnd         time.Time `json:"coverage_end"`
	CoverageStart       time.Time `json:"coverage_start"`
	DecodeRejected      uint64    `json:"decode_rejected"`
	LANKernelDrops      uint64    `json:"lan_kernel_drops"`
	ObservationDiscards uint64    `json:"observation_discards"`
	Status              string    `json:"status"`
	WANKernelDrops      uint64    `json:"wan_kernel_drops"`
	WriteFailures       uint64    `json:"write_failures"`
}

// ConntrackQualification reports whether the conntrack listener was healthy
// around one packet observation.
type ConntrackQualification struct {
	CoverageEnd        time.Time `json:"coverage_end"`
	CoverageStart      time.Time `json:"coverage_start"`
	DatagramsTruncated uint64    `json:"datagrams_truncated"`
	ParseRejected      uint64    `json:"parse_rejected"`
	ReceiveErrors      uint64    `json:"receive_errors"`
	ReceiveOverruns    uint64    `json:"receive_overruns"`
	Status             string    `json:"status"`
	WriteFailures      uint64    `json:"write_failures"`
}

// Index provides time-bounded qualification over persisted reports.
type Index struct {
	reports []Report
}

// NewRecorder creates a health-history recorder.
func NewRecorder(options RecorderOptions) (*Recorder, error) {
	if options.Directory == "" {
		return nil, fmt.Errorf("health history directory is required")
	}
	if options.Interval < time.Second {
		return nil, fmt.Errorf("health interval must be at least one second")
	}
	if options.SegmentBytes == 0 {
		return nil, fmt.Errorf("health segment size must be greater than zero")
	}
	if options.MaxBytes < options.SegmentBytes {
		return nil, fmt.Errorf("health max size must be at least one segment")
	}

	ring, err := storage.NewRing(storage.RingOptions{
		Compression:  options.Compression,
		Directory:    options.Directory,
		MaxBytes:     options.MaxBytes,
		MinFreeBytes: options.MinFreeBytes,
		OnCompressionError: func(err error) {
			if options.OnError != nil {
				options.OnError(fmt.Errorf("health compression: %w", err))
			}
		},
		Prefix:       "health",
		SegmentBytes: options.SegmentBytes,
	})
	if err != nil {
		return nil, err
	}

	return &Recorder{
		interval: options.Interval,
		ring:     ring,
	}, nil
}

// Close finalizes health-history storage.
func (recorder *Recorder) Close() error {
	if recorder == nil || recorder.ring == nil {
		return nil
	}
	return recorder.ring.Close()
}

// Stats returns persisted health-history write state.
func (recorder *Recorder) Stats() RecorderStats {
	if recorder == nil {
		return RecorderStats{
			Storage: storage.Stats{
				CompressionMode: "not_configured",
			},
		}
	}

	result := RecorderStats{
		Enabled:         true,
		IntervalSeconds: int64(recorder.interval / time.Second),
		ReportsWritten:  recorder.writes.Load(),
		Storage: storage.Stats{
			CompressionMode: "not_configured",
		},
		WriteFailures: recorder.writeErrs.Load(),
	}
	if recorder.ring != nil {
		result.Storage = recorder.ring.Stats()
	}

	return result
}

// Write persists one complete health report.
func (recorder *Recorder) Write(report Report) error {
	if recorder == nil || recorder.ring == nil {
		return nil
	}

	data, err := json.Marshal(report)
	if err != nil {
		recorder.writeErrs.Add(1)
		return fmt.Errorf("encode health report: %w", err)
	}
	data = append(data, '\n')

	if _, err := recorder.ring.Write(data); err != nil {
		recorder.writeErrs.Add(1)
		return fmt.Errorf("write health report: %w", err)
	}

	recorder.writes.Add(1)
	return nil
}

// LoadIndex reads persisted health reports. A missing directory is treated as
// unavailable history rather than an error.
func LoadIndex(directory string) (*Index, LoadStats, error) {
	index := &Index{reports: []Report{}}
	stats := LoadStats{}

	if directory == "" {
		return index, stats, nil
	}

	files, err := segmentread.List(directory, "health-")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return index, stats, nil
		}
		return nil, stats, err
	}

	for _, path := range files {
		readStats, readErr := segmentread.ReadLines(
			path,
			maxScannerToken,
			func(line []byte) error {
				var current Report
				if err := json.Unmarshal(line, &current); err != nil {
					return fmt.Errorf("decode health report: %w", err)
				}
				if current.RecordType != RecordType ||
					current.ObservedAt.IsZero() {
					return nil
				}

				stats.ReportsRead++
				index.reports = append(index.reports, current)
				return nil
			},
		)
		if readErr != nil {
			return nil, stats, fmt.Errorf(
				"read health history %s: %w",
				path,
				readErr,
			)
		}
		if readStats.Vanished {
			stats.SegmentsVanishedDuringRead++
			continue
		}
		stats.FilesRead++
		if readStats.Active {
			stats.ActiveFilesRead++
		}
		if readStats.TrailingFragmentIgnored {
			stats.ActiveTrailingFragmentsIgnored++
		}
	}

	sort.Slice(index.reports, func(i, j int) bool {
		return index.reports[i].ObservedAt.Before(
			index.reports[j].ObservedAt,
		)
	})

	return index, stats, nil
}

// Qualify returns the capture-loss/decode state surrounding an observation.
// It requires a health report after the observation and either a prior report
// from the same capture session or cumulative coverage from capture start.
func (index *Index) Qualify(
	at time.Time,
	wanInterface string,
	lanInterface string,
) Qualification {
	if index == nil || len(index.reports) == 0 {
		return Qualification{Status: QualificationUnavailable}
	}

	afterIndex := -1
	for i := range index.reports {
		if !index.reports[i].ObservedAt.Before(at) {
			afterIndex = i
			break
		}
	}
	if afterIndex < 0 {
		return Qualification{Status: QualificationInsufficient}
	}

	after := index.reports[afterIndex]
	interval := time.Duration(after.ExpectedIntervalSeconds) * time.Second
	if interval <= 0 {
		return Qualification{Status: QualificationInsufficient}
	}

	grace := interval + 5*time.Second
	if after.ObservedAt.Sub(at) > grace {
		return Qualification{Status: QualificationInsufficient}
	}
	if after.Capture.StartedAt.IsZero() ||
		at.Before(after.Capture.StartedAt) {
		return Qualification{Status: QualificationInsufficient}
	}

	var before Report
	hasBefore := false
	if afterIndex > 0 {
		candidate := index.reports[afterIndex-1]
		if candidate.Capture.StartedAt.Equal(after.Capture.StartedAt) &&
			!candidate.ObservedAt.After(at) &&
			at.Sub(candidate.ObservedAt) <= grace {
			before = candidate
			hasBefore = true
		}
	}

	coverageStart := after.Capture.StartedAt
	if hasBefore {
		coverageStart = before.ObservedAt
	}

	wanAfter, wanOK := interfaceStats(after.Capture, wanInterface)
	lanAfter, lanOK := interfaceStats(after.Capture, lanInterface)
	if !wanOK || !lanOK ||
		wanAfter.KernelStatisticsStatus != capture.KernelStatsAvailable ||
		lanAfter.KernelStatisticsStatus != capture.KernelStatsAvailable {
		return Qualification{
			CoverageEnd:   after.ObservedAt,
			CoverageStart: coverageStart,
			Status:        QualificationInsufficient,
		}
	}

	var wanBefore capture.InterfaceStats
	var lanBefore capture.InterfaceStats
	var discardsBefore uint64
	var writesBefore uint64

	if hasBefore {
		var ok bool
		wanBefore, ok = interfaceStats(before.Capture, wanInterface)
		if !ok {
			return Qualification{Status: QualificationInsufficient}
		}
		lanBefore, ok = interfaceStats(before.Capture, lanInterface)
		if !ok {
			return Qualification{Status: QualificationInsufficient}
		}
		discardsBefore = before.Capture.ObservationsDiscarded
		writesBefore = before.Capture.WriteFailures
	}

	wanDrops, ok := delta(wanBefore.KernelDrops, wanAfter.KernelDrops)
	if !ok {
		return Qualification{Status: QualificationInsufficient}
	}
	lanDrops, ok := delta(lanBefore.KernelDrops, lanAfter.KernelDrops)
	if !ok {
		return Qualification{Status: QualificationInsufficient}
	}
	wanRejected, ok := delta(
		wanBefore.DecodeRejected,
		wanAfter.DecodeRejected,
	)
	if !ok {
		return Qualification{Status: QualificationInsufficient}
	}
	lanRejected, ok := delta(
		lanBefore.DecodeRejected,
		lanAfter.DecodeRejected,
	)
	if !ok {
		return Qualification{Status: QualificationInsufficient}
	}
	discards, ok := delta(
		discardsBefore,
		after.Capture.ObservationsDiscarded,
	)
	if !ok {
		return Qualification{Status: QualificationInsufficient}
	}
	writes, ok := delta(
		writesBefore,
		after.Capture.WriteFailures,
	)
	if !ok {
		return Qualification{Status: QualificationInsufficient}
	}

	result := Qualification{
		CoverageEnd:         after.ObservedAt,
		CoverageStart:       coverageStart,
		DecodeRejected:      wanRejected + lanRejected,
		LANKernelDrops:      lanDrops,
		ObservationDiscards: discards,
		Status:              QualificationHealthy,
		WANKernelDrops:      wanDrops,
		WriteFailures:       writes,
	}

	if result.WANKernelDrops != 0 ||
		result.LANKernelDrops != 0 ||
		result.DecodeRejected != 0 ||
		result.ObservationDiscards != 0 ||
		result.WriteFailures != 0 {
		result.Status = QualificationImpaired
	}

	return result
}

// QualifyConntrack returns conntrack-listener health surrounding an
// observation. A healthy result supports saying a matching conntrack event was
// not observed; it never proves that no conntrack state existed.
func (index *Index) QualifyConntrack(at time.Time) ConntrackQualification {
	if index == nil || len(index.reports) == 0 {
		return ConntrackQualification{Status: ConntrackQualificationUnavailable}
	}

	afterIndex := -1
	for i := range index.reports {
		if !index.reports[i].ObservedAt.Before(at) {
			afterIndex = i
			break
		}
	}
	if afterIndex < 0 {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}

	after := index.reports[afterIndex]
	interval := time.Duration(after.ExpectedIntervalSeconds) * time.Second
	if interval <= 0 {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}
	grace := interval + 5*time.Second
	if after.ObservedAt.Sub(at) > grace {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}
	if !after.Conntrack.Enabled {
		return ConntrackQualification{
			CoverageEnd: after.ObservedAt,
			Status:      ConntrackQualificationDisabled,
		}
	}
	if after.Conntrack.StartedAt.IsZero() || at.Before(after.Conntrack.StartedAt) {
		return ConntrackQualification{
			CoverageEnd: after.ObservedAt,
			Status:      ConntrackQualificationInsufficient,
		}
	}

	before := conntrackhistory.Stats{}
	coverageStart := after.Conntrack.StartedAt
	if afterIndex > 0 {
		candidate := index.reports[afterIndex-1]
		if candidate.Conntrack.Enabled &&
			candidate.Conntrack.StartedAt.Equal(after.Conntrack.StartedAt) &&
			!candidate.ObservedAt.After(at) &&
			at.Sub(candidate.ObservedAt) <= grace {
			before = candidate.Conntrack
			coverageStart = candidate.ObservedAt
		}
	}

	overruns, ok := delta(before.ReceiveOverruns, after.Conntrack.ReceiveOverruns)
	if !ok {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}
	truncated, ok := delta(before.DatagramsTruncated, after.Conntrack.DatagramsTruncated)
	if !ok {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}
	parseRejected, ok := delta(before.ParseRejected, after.Conntrack.ParseRejected)
	if !ok {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}
	receiveErrors, ok := delta(before.ReceiveErrors, after.Conntrack.ReceiveErrors)
	if !ok {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}
	writeFailures, ok := delta(before.WriteFailures, after.Conntrack.WriteFailures)
	if !ok {
		return ConntrackQualification{Status: ConntrackQualificationInsufficient}
	}

	result := ConntrackQualification{
		CoverageEnd:        after.ObservedAt,
		CoverageStart:      coverageStart,
		DatagramsTruncated: truncated,
		ParseRejected:      parseRejected,
		ReceiveErrors:      receiveErrors,
		ReceiveOverruns:    overruns,
		Status:             ConntrackQualificationHealthy,
		WriteFailures:      writeFailures,
	}
	if result.ReceiveOverruns != 0 ||
		result.DatagramsTruncated != 0 ||
		result.ParseRejected != 0 ||
		result.ReceiveErrors != 0 ||
		result.WriteFailures != 0 {
		result.Status = ConntrackQualificationImpaired
	}

	return result
}

func delta(before uint64, after uint64) (uint64, bool) {
	if after < before {
		return 0, false
	}
	return after - before, true
}

func interfaceStats(
	stats capture.Stats,
	name string,
) (capture.InterfaceStats, bool) {
	for _, current := range stats.Interfaces {
		if current.Interface == name {
			return current, true
		}
	}
	return capture.InterfaceStats{}, false
}

// DefaultDirectory returns the health subdirectory for an observation store.
func DefaultDirectory(inputDir string) string {
	return filepath.Join(inputDir, "health")
}
