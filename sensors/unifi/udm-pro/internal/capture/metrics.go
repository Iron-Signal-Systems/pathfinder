package capture

import (
	"sort"
	"sync"
	"time"
)

const (
	KernelStatsAvailable   = "available"
	KernelStatsNotPolled   = "not_polled"
	KernelStatsUnavailable = "unavailable"
)

// InterfaceStats summarizes capture activity for one observed interface.
type InterfaceStats struct {
	DecodeRejected                   uint64 `json:"decode_rejected"`
	Interface                        string `json:"interface"`
	KernelDrops                      uint64 `json:"kernel_drops"`
	KernelPackets                    uint64 `json:"kernel_packets"`
	KernelStatisticsStatus           string `json:"kernel_statistics_status"`
	KernelTimestampedFrames          uint64 `json:"kernel_timestamped_frames"`
	ObservationsDecoded              uint64 `json:"observations_decoded"`
	ObservationsQueued               uint64 `json:"observations_queued"`
	SocketReceiveBufferBytes         int    `json:"socket_receive_buffer_bytes"`
	SocketReceiveBufferMode          string `json:"socket_receive_buffer_mode"`
	SocketReceiveBufferRequested     int    `json:"socket_receive_buffer_requested"`
	TimestampControlFailures         uint64 `json:"timestamp_control_failures"`
	TimestampMode                    string `json:"timestamp_mode"`
	UserspaceFrames                  uint64 `json:"userspace_frames"`
	UserspaceTimestampFallbackFrames uint64 `json:"userspace_timestamp_fallback_frames"`
}

// Stats is a point-in-time snapshot of capture and writer counters.
type Stats struct {
	Interfaces                []InterfaceStats `json:"interfaces"`
	ObservationQueueCapacity  int              `json:"observation_queue_capacity"`
	ObservationQueueHighWater int              `json:"observation_queue_high_water"`
	ObservationsDiscarded     uint64           `json:"observations_discarded"`
	ObservationsWritten       uint64           `json:"observations_written"`
	StartedAt                 time.Time        `json:"started_at"`
	WriteFailures             uint64           `json:"write_failures"`
}

// Metrics safely accumulates capture counters from multiple interfaces.
type Metrics struct {
	interfaces                map[string]*InterfaceStats
	mu                        sync.Mutex
	observationQueueCapacity  int
	observationQueueHighWater int
	observationsDiscarded     uint64
	observationsWritten       uint64
	startedAt                 time.Time
	writeFailures             uint64
}

// NewMetrics creates an empty metric set for the requested interfaces.
func NewMetrics(interfaceNames []string) *Metrics {
	metrics := &Metrics{
		interfaces: make(map[string]*InterfaceStats),
		startedAt:  time.Now().UTC(),
	}

	for _, name := range interfaceNames {
		if _, exists := metrics.interfaces[name]; exists {
			continue
		}
		metrics.interfaces[name] = &InterfaceStats{
			Interface:               name,
			KernelStatisticsStatus:  KernelStatsNotPolled,
			SocketReceiveBufferMode: "not_configured",
			TimestampMode:           "not_configured",
		}
	}

	return metrics
}

// Snapshot returns a stable copy of all counters.
func (metrics *Metrics) Snapshot() Stats {
	if metrics == nil {
		return Stats{Interfaces: []InterfaceStats{}}
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	snapshot := Stats{
		Interfaces:                make([]InterfaceStats, 0, len(metrics.interfaces)),
		ObservationQueueCapacity:  metrics.observationQueueCapacity,
		ObservationQueueHighWater: metrics.observationQueueHighWater,
		ObservationsDiscarded:     metrics.observationsDiscarded,
		ObservationsWritten:       metrics.observationsWritten,
		StartedAt:                 metrics.startedAt,
		WriteFailures:             metrics.writeFailures,
	}

	for _, current := range metrics.interfaces {
		snapshot.Interfaces = append(snapshot.Interfaces, *current)
	}

	sort.Slice(snapshot.Interfaces, func(i, j int) bool {
		return snapshot.Interfaces[i].Interface < snapshot.Interfaces[j].Interface
	})

	return snapshot
}

func (metrics *Metrics) addDecoded(interfaceName string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.ObservationsDecoded++
}

func (metrics *Metrics) addDiscarded() {
	metrics.mu.Lock()
	metrics.observationsDiscarded++
	metrics.mu.Unlock()
}

func (metrics *Metrics) addKernelStatistics(interfaceName string, packets uint64, drops uint64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.KernelPackets += packets
	current.KernelDrops += drops
	current.KernelStatisticsStatus = KernelStatsAvailable
}

func (metrics *Metrics) addQueued(interfaceName string, queueLength int) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.ObservationsQueued++

	if queueLength > metrics.observationQueueHighWater {
		metrics.observationQueueHighWater = queueLength
	}
}

func (metrics *Metrics) addRejected(interfaceName string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.DecodeRejected++
}

func (metrics *Metrics) addTimestampControlFailure(interfaceName string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.TimestampControlFailures++
}

func (metrics *Metrics) addTimestampObservation(
	interfaceName string,
	source string,
) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)

	switch source {
	case "kernel_software_ns":
		current.KernelTimestampedFrames++
	default:
		current.UserspaceTimestampFallbackFrames++
	}
}

func (metrics *Metrics) addUserspaceFrame(interfaceName string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.UserspaceFrames++
}

func (metrics *Metrics) addWriteFailure() {
	metrics.mu.Lock()
	metrics.writeFailures++
	metrics.mu.Unlock()
}

func (metrics *Metrics) addWritten() {
	metrics.mu.Lock()
	metrics.observationsWritten++
	metrics.mu.Unlock()
}

func (metrics *Metrics) configureObservationQueue(capacity int) {
	metrics.mu.Lock()
	metrics.observationQueueCapacity = capacity
	metrics.mu.Unlock()
}

func (metrics *Metrics) ensureInterface(interfaceName string) *InterfaceStats {
	current, exists := metrics.interfaces[interfaceName]
	if exists {
		return current
	}

	current = &InterfaceStats{
		Interface:               interfaceName,
		KernelStatisticsStatus:  KernelStatsNotPolled,
		SocketReceiveBufferMode: "not_configured",
		TimestampMode:           "not_configured",
	}
	metrics.interfaces[interfaceName] = current
	return current
}

func (metrics *Metrics) markKernelStatisticsUnavailable(interfaceName string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	if current.KernelStatisticsStatus != KernelStatsAvailable {
		current.KernelStatisticsStatus = KernelStatsUnavailable
	}
}

func (metrics *Metrics) setTimestampMode(
	interfaceName string,
	mode string,
) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.TimestampMode = mode
}

func (metrics *Metrics) setSocketReceiveBuffer(
	interfaceName string,
	requested int,
	actual int,
	mode string,
) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	current := metrics.ensureInterface(interfaceName)
	current.SocketReceiveBufferBytes = actual
	current.SocketReceiveBufferMode = mode
	current.SocketReceiveBufferRequested = requested
}
