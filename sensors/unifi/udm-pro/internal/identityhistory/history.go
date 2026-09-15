package identityhistory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/enrichment"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/segmentread"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

const (
	RecordType    = "endpoint_identity_snapshot"
	SchemaVersion = "1"

	defaultHistoricalMaxAge = 2 * time.Minute
	maxScannerToken         = 4 * 1024 * 1024
)

// Endpoint is one endpoint identity present in a point-in-time DHCP snapshot.
type Endpoint struct {
	FQDN           string `json:"fqdn"`
	Hostname       string `json:"hostname"`
	IP             string `json:"ip"`
	LeaseExpiresAt string `json:"lease_expires_at"`
	MAC            string `json:"mac"`
}

// Snapshot is one complete point-in-time view of the dnsmasq lease state.
type Snapshot struct {
	Endpoints               []Endpoint `json:"endpoints"`
	ExpectedIntervalSeconds int64      `json:"expected_interval_seconds"`
	ObservedAt              time.Time  `json:"observed_at"`
	RecordType              string     `json:"record_type"`
	SchemaVersion           string     `json:"schema_version"`
	Source                  string     `json:"source"`
	SourcePath              string     `json:"source_path"`
}

// HistoricalIdentity is a conservative time-bounded match derived from a
// previously recorded endpoint identity snapshot.
type HistoricalIdentity struct {
	FQDN               string    `json:"fqdn"`
	Hostname           string    `json:"hostname"`
	IP                 string    `json:"ip"`
	LeaseExpiresAt     string    `json:"lease_expires_at"`
	MAC                string    `json:"mac"`
	SnapshotObservedAt time.Time `json:"snapshot_observed_at"`
	Source             string    `json:"source"`
	Status             string    `json:"status"`
}

// RecorderOptions configures periodic endpoint identity snapshots.
type RecorderOptions struct {
	Compression  string
	Directory    string
	HostsPath    string
	Interval     time.Duration
	LeasePath    string
	MaxBytes     uint64
	MinFreeBytes uint64
	OnError      func(error)
	SegmentBytes uint64
}

// Stats describes endpoint identity history health.
type Stats struct {
	Enabled          bool          `json:"enabled"`
	IntervalSeconds  int64         `json:"interval_seconds"`
	LastSnapshotAt   string        `json:"last_snapshot_at"`
	ReadFailures     uint64        `json:"read_failures"`
	SnapshotsWritten uint64        `json:"snapshots_written"`
	Storage          storage.Stats `json:"storage"`
	WriteFailures    uint64        `json:"write_failures"`
}

// Recorder periodically captures current dnsmasq state to a separate
// immutable rolling JSONL store.
type Recorder struct {
	captureMu  sync.Mutex
	hostsPath  string
	interval   time.Duration
	lastMu     sync.Mutex
	lastAt     string
	leasePath  string
	onError    func(error)
	readErrors atomic.Uint64
	ring       *storage.Ring
	writes     atomic.Uint64
	writeErrs  atomic.Uint64
}

// Index provides historical identity lookup by observed IP/MAC and time.
type Index struct {
	byEndpoint map[string][]historicalEntry
}

// LoadStats describes historical identity source material.
type LoadStats struct {
	ActiveFilesRead                uint64 `json:"active_files_read"`
	ActiveTrailingFragmentsIgnored uint64 `json:"active_trailing_fragments_ignored"`
	FilesRead                      uint64 `json:"files_read"`
	SegmentsVanishedDuringRead     uint64 `json:"segments_vanished_during_read"`
	SnapshotsRead                  uint64 `json:"snapshots_read"`
}

// LookupOptions controls conservative historical identity matching.
type LookupOptions struct {
	DefaultMaxAge time.Duration
}

type historicalEntry struct {
	endpoint         Endpoint
	expectedInterval time.Duration
	snapshotAt       time.Time
}

// NewRecorder creates the endpoint identity history recorder and its ring.
func NewRecorder(options RecorderOptions) (*Recorder, error) {
	if options.Directory == "" {
		return nil, fmt.Errorf("identity history directory is required")
	}
	if options.LeasePath == "" {
		return nil, fmt.Errorf("dnsmasq lease path is required")
	}
	if options.Interval < time.Second {
		return nil, fmt.Errorf("identity snapshot interval must be at least one second")
	}
	if options.SegmentBytes == 0 {
		return nil, fmt.Errorf("identity segment size must be greater than zero")
	}
	if options.MaxBytes < options.SegmentBytes {
		return nil, fmt.Errorf("identity max size must be at least one segment")
	}

	ring, err := storage.NewRing(storage.RingOptions{
		Compression:  options.Compression,
		Directory:    options.Directory,
		MaxBytes:     options.MaxBytes,
		MinFreeBytes: options.MinFreeBytes,
		OnCompressionError: func(err error) {
			if options.OnError != nil {
				options.OnError(fmt.Errorf("identity compression: %w", err))
			}
		},
		Prefix:       "endpoint-identity",
		SegmentBytes: options.SegmentBytes,
	})
	if err != nil {
		return nil, err
	}

	recorder := &Recorder{
		hostsPath: options.HostsPath,
		interval:  options.Interval,
		lastAt:    observation.ValueNoRecord,
		leasePath: options.LeasePath,
		onError:   options.OnError,
		ring:      ring,
	}

	// Capture the initial state synchronously so packet capture cannot begin
	// before the first endpoint identity snapshot attempt.
	recorder.capture()

	return recorder, nil
}

// Close finalizes endpoint identity history storage.
func (recorder *Recorder) Close() error {
	if recorder == nil || recorder.ring == nil {
		return nil
	}
	return recorder.ring.Close()
}

// Run writes periodic snapshots until done is closed. NewRecorder performs
// the initial snapshot synchronously before packet capture begins. Individual
// read/write failures are counted and reported but do not stop packet capture.
func (recorder *Recorder) Run(done <-chan struct{}) {
	if recorder == nil {
		return
	}

	ticker := time.NewTicker(recorder.interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			recorder.capture()
		}
	}
}

// Stats returns a concurrency-safe endpoint identity health snapshot.
func (recorder *Recorder) Stats() Stats {
	if recorder == nil {
		return DisabledStats()
	}

	recorder.lastMu.Lock()
	lastAt := recorder.lastAt
	recorder.lastMu.Unlock()

	result := Stats{
		Enabled:          true,
		IntervalSeconds:  int64(recorder.interval / time.Second),
		LastSnapshotAt:   lastAt,
		ReadFailures:     recorder.readErrors.Load(),
		SnapshotsWritten: recorder.writes.Load(),
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

// DisabledStats returns a stable no-null health representation.
func DisabledStats() Stats {
	return Stats{
		Enabled:        false,
		LastSnapshotAt: observation.ValueNoRecord,
		Storage: storage.Stats{
			CompressionMode: "not_configured",
		},
	}
}

// LoadIndex reads endpoint identity history from a directory. A missing
// directory is treated as no historical identity data.
func LoadIndex(directory string) (*Index, LoadStats, error) {
	index := &Index{
		byEndpoint: make(map[string][]historicalEntry),
	}
	stats := LoadStats{}

	if directory == "" {
		return index, stats, nil
	}

	files, err := historyFiles(directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return index, stats, nil
		}
		return nil, stats, err
	}

	for _, path := range files {
		readStats, readErr := readSnapshotFile(
			path,
			func(snapshot Snapshot) error {
				stats.SnapshotsRead++

				interval := time.Duration(snapshot.ExpectedIntervalSeconds) * time.Second
				for _, endpoint := range snapshot.Endpoints {
					if endpoint.IP == "" ||
						endpoint.IP == observation.ValueNoRecord ||
						endpoint.MAC == "" ||
						endpoint.MAC == observation.ValueNoRecord {
						continue
					}

					key := endpointKey(endpoint.IP, endpoint.MAC)
					index.byEndpoint[key] = append(
						index.byEndpoint[key],
						historicalEntry{
							endpoint:         endpoint,
							expectedInterval: interval,
							snapshotAt:       snapshot.ObservedAt,
						},
					)
				}

				return nil
			},
		)
		if readErr != nil {
			return nil, stats, fmt.Errorf(
				"read identity history %s: %w",
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

	for key := range index.byEndpoint {
		sort.Slice(index.byEndpoint[key], func(i, j int) bool {
			return index.byEndpoint[key][i].snapshotAt.Before(
				index.byEndpoint[key][j].snapshotAt,
			)
		})
	}

	return index, stats, nil
}

// Lookup returns the most recent qualifying snapshot at or before at.
func (index *Index) Lookup(
	ip string,
	mac string,
	at time.Time,
	options LookupOptions,
) HistoricalIdentity {
	result := HistoricalIdentity{
		FQDN:           observation.ValueNoRecord,
		Hostname:       observation.ValueNoRecord,
		IP:             ip,
		LeaseExpiresAt: observation.ValueNoRecord,
		MAC:            normalizeMAC(mac),
		Source:         "dnsmasq_lease_snapshot",
		Status:         "no_matching_snapshot",
	}

	if index == nil {
		result.Source = observation.ValueNoRecord
		result.Status = "source_unavailable"
		return result
	}

	entries := index.byEndpoint[endpointKey(ip, mac)]
	if len(entries) == 0 {
		return result
	}

	position := sort.Search(len(entries), func(i int) bool {
		return entries[i].snapshotAt.After(at)
	})
	if position == 0 {
		return result
	}

	entry := entries[position-1]

	maxAge := options.DefaultMaxAge
	if maxAge <= 0 {
		if entry.expectedInterval > 0 {
			maxAge = 2*entry.expectedInterval + 5*time.Second
		} else {
			maxAge = defaultHistoricalMaxAge
		}
	}

	if age := at.Sub(entry.snapshotAt); age < 0 || age > maxAge {
		result.Status = "snapshot_too_old"
		return result
	}

	if entry.endpoint.LeaseExpiresAt != observation.ValueNoRecord &&
		entry.endpoint.LeaseExpiresAt != observation.ValueNotKnown {
		expiresAt, err := time.Parse(time.RFC3339, entry.endpoint.LeaseExpiresAt)
		if err != nil {
			result.Status = "invalid_lease_expiry"
			return result
		}
		if at.After(expiresAt) {
			result.Status = "lease_expired"
			return result
		}
	}

	result.FQDN = valueOrNoRecord(entry.endpoint.FQDN)
	result.Hostname = valueOrNoRecord(entry.endpoint.Hostname)
	result.LeaseExpiresAt = entry.endpoint.LeaseExpiresAt
	result.MAC = normalizeMAC(entry.endpoint.MAC)
	result.SnapshotObservedAt = entry.snapshotAt
	result.Status = "matched"

	return result
}

func (recorder *Recorder) capture() {
	recorder.captureMu.Lock()
	defer recorder.captureMu.Unlock()

	state, err := enrichment.ReadDNSMasqState(
		recorder.leasePath,
		recorder.hostsPath,
	)
	if err != nil {
		recorder.readErrors.Add(1)
		recorder.report(fmt.Errorf("identity snapshot read: %w", err))
		return
	}

	identities := state.LeaseIdentities()
	endpoints := make([]Endpoint, 0, len(identities))
	for _, current := range identities {
		endpoints = append(endpoints, Endpoint{
			FQDN:           valueOrNoRecord(current.FQDN),
			Hostname:       valueOrNoRecord(current.Hostname),
			IP:             current.IP,
			LeaseExpiresAt: current.LeaseExpiresAt,
			MAC:            normalizeMAC(current.MAC),
		})
	}

	observedAt := state.ReadAt()
	snapshot := Snapshot{
		Endpoints:               endpoints,
		ExpectedIntervalSeconds: int64(recorder.interval / time.Second),
		ObservedAt:              observedAt,
		RecordType:              RecordType,
		SchemaVersion:           SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              recorder.leasePath,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		recorder.writeErrs.Add(1)
		recorder.report(fmt.Errorf("identity snapshot encode: %w", err))
		return
	}
	data = append(data, '\n')

	if _, err := recorder.ring.Write(data); err != nil {
		recorder.writeErrs.Add(1)
		recorder.report(fmt.Errorf("identity snapshot write: %w", err))
		return
	}

	recorder.writes.Add(1)
	recorder.lastMu.Lock()
	recorder.lastAt = observedAt.UTC().Format(time.RFC3339Nano)
	recorder.lastMu.Unlock()
}

func endpointKey(ip string, mac string) string {
	return strings.TrimSpace(ip) + "\x00" + normalizeMAC(mac)
}

func historyFiles(directory string) ([]string, error) {
	return segmentread.List(directory, "endpoint-identity-")
}

func normalizeMAC(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return observation.ValueNoRecord
	}
	return value
}

func readSnapshotFile(
	path string,
	consume func(Snapshot) error,
) (segmentread.ReadStats, error) {
	return segmentread.ReadLines(
		path,
		maxScannerToken,
		func(line []byte) error {
			var snapshot Snapshot
			if err := json.Unmarshal(line, &snapshot); err != nil {
				return fmt.Errorf("decode snapshot: %w", err)
			}

			if snapshot.RecordType != RecordType ||
				snapshot.SchemaVersion != SchemaVersion ||
				snapshot.ObservedAt.IsZero() {
				return nil
			}

			if snapshot.Endpoints == nil {
				snapshot.Endpoints = []Endpoint{}
			}

			return consume(snapshot)
		},
	)
}

func (recorder *Recorder) report(err error) {
	if recorder.onError != nil {
		recorder.onError(err)
	}
}

func valueOrNoRecord(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "*" {
		return observation.ValueNoRecord
	}
	return value
}
