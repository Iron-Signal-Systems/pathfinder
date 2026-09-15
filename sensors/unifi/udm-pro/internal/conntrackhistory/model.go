// Package conntrackhistory records and queries Linux conntrack events without
// modifying conntrack state or firewall policy.
package conntrackhistory

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

const (
	RecordType    = "conntrack_event"
	SchemaVersion = "1"

	EventDestroy = "destroy"
	EventNew     = "new"
	EventUpdate  = "update"

	TimestampSourceUserspaceReceive = "userspace_receive"
)

// Counters are conntrack packet/byte counters for one tuple direction.
type Counters struct {
	Bytes   uint64 `json:"bytes"`
	Packets uint64 `json:"packets"`
	Present bool   `json:"present"`
}

// Event is one immutable conntrack netlink event.
type Event struct {
	Assured          bool      `json:"assured"`
	Confirmed        bool      `json:"confirmed"`
	DestinationNAT   bool      `json:"destination_nat"`
	Dying            bool      `json:"dying"`
	EventType        string    `json:"event_type"`
	HardwareOffload  bool      `json:"hardware_offload"`
	ID               uint32    `json:"id"`
	Mark             uint32    `json:"mark"`
	ObservedAt       time.Time `json:"observed_at"`
	Offload          bool      `json:"offload"`
	Original         Tuple     `json:"original"`
	OriginalCounters Counters  `json:"original_counters"`
	RecordType       string    `json:"record_type"`
	Reply            Tuple     `json:"reply"`
	ReplyCounters    Counters  `json:"reply_counters"`
	SchemaVersion    string    `json:"schema_version"`
	SeenReply        bool      `json:"seen_reply"`
	SourceNAT        bool      `json:"source_nat"`
	Status           uint32    `json:"status"`
	TCPState         string    `json:"tcp_state"`
	TimeoutSeconds   uint32    `json:"timeout_seconds"`
	TimestampSource  string    `json:"timestamp_source"`
	Unreplied        bool      `json:"unreplied"`
}

// Tuple preserves one original or reply conntrack tuple.
type Tuple struct {
	DestinationIP   string `json:"destination_ip"`
	DestinationPort uint16 `json:"destination_port"`
	Family          string `json:"family"`
	ICMPCode        uint8  `json:"icmp_code"`
	ICMPID          uint16 `json:"icmp_id"`
	ICMPType        uint8  `json:"icmp_type"`
	Protocol        string `json:"protocol"`
	ProtocolNumber  uint8  `json:"protocol_number"`
	SourceIP        string `json:"source_ip"`
	SourcePort      uint16 `json:"source_port"`
}

// RecorderOptions configures conntrack history.
type RecorderOptions struct {
	Compression       string
	Directory         string
	MaxBytes          uint64
	MinFreeBytes      uint64
	OnError           func(error)
	SegmentBytes      uint64
	SocketBufferBytes int
}

// Stats is a point-in-time conntrack listener/storage snapshot.
type Stats struct {
	DatagramsTruncated           uint64        `json:"datagrams_truncated"`
	DestroyEvents                uint64        `json:"destroy_events"`
	Enabled                      bool          `json:"enabled"`
	EventsReceived               uint64        `json:"events_received"`
	EventsWritten                uint64        `json:"events_written"`
	MessagesIgnored              uint64        `json:"messages_ignored"`
	NewEvents                    uint64        `json:"new_events"`
	ParseRejected                uint64        `json:"parse_rejected"`
	ReceiveErrors                uint64        `json:"receive_errors"`
	ReceiveOverruns              uint64        `json:"receive_overruns"`
	SocketReceiveBufferBytes     int           `json:"socket_receive_buffer_bytes"`
	SocketReceiveBufferMode      string        `json:"socket_receive_buffer_mode"`
	SocketReceiveBufferRequested int           `json:"socket_receive_buffer_requested"`
	StartedAt                    time.Time     `json:"started_at"`
	Storage                      storage.Stats `json:"storage"`
	UpdateEvents                 uint64        `json:"update_events"`
	WriteFailures                uint64        `json:"write_failures"`
}

// metrics is kept separate from the recorder's storage lock.
type metrics struct {
	datagramsTruncated           uint64
	destroyEvents                uint64
	eventsReceived               uint64
	eventsWritten                uint64
	messagesIgnored              uint64
	mu                           sync.Mutex
	newEvents                    uint64
	parseRejected                uint64
	receiveErrors                uint64
	receiveOverruns              uint64
	socketReceiveBufferBytes     int
	socketReceiveBufferMode      string
	socketReceiveBufferRequested int
	startedAt                    time.Time
	updateEvents                 uint64
	writeFailures                uint64
}

func newMetrics() *metrics {
	return &metrics{
		socketReceiveBufferMode: "not_configured",
		startedAt:               time.Now().UTC(),
	}
}

func (m *metrics) snapshot(ring *storage.Ring) Stats {
	if m == nil {
		return DisabledStats()
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := Stats{
		DatagramsTruncated:           m.datagramsTruncated,
		DestroyEvents:                m.destroyEvents,
		Enabled:                      true,
		EventsReceived:               m.eventsReceived,
		EventsWritten:                m.eventsWritten,
		MessagesIgnored:              m.messagesIgnored,
		NewEvents:                    m.newEvents,
		ParseRejected:                m.parseRejected,
		ReceiveErrors:                m.receiveErrors,
		ReceiveOverruns:              m.receiveOverruns,
		SocketReceiveBufferBytes:     m.socketReceiveBufferBytes,
		SocketReceiveBufferMode:      m.socketReceiveBufferMode,
		SocketReceiveBufferRequested: m.socketReceiveBufferRequested,
		StartedAt:                    m.startedAt,
		Storage: storage.Stats{
			CompressionMode: "not_configured",
		},
		UpdateEvents:  m.updateEvents,
		WriteFailures: m.writeFailures,
	}
	if ring != nil {
		stats.Storage = ring.Stats()
	}
	return stats
}

func (m *metrics) addDatagramTruncated() {
	m.mu.Lock()
	m.datagramsTruncated++
	m.mu.Unlock()
}

func (m *metrics) addEvent(eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventsReceived++
	switch eventType {
	case EventNew:
		m.newEvents++
	case EventUpdate:
		m.updateEvents++
	case EventDestroy:
		m.destroyEvents++
	}
}

func (m *metrics) addIgnored() {
	m.mu.Lock()
	m.messagesIgnored++
	m.mu.Unlock()
}

func (m *metrics) addParseRejected() {
	m.mu.Lock()
	m.parseRejected++
	m.mu.Unlock()
}

func (m *metrics) addReceiveError() {
	m.mu.Lock()
	m.receiveErrors++
	m.mu.Unlock()
}

func (m *metrics) addReceiveOverrun() {
	m.mu.Lock()
	m.receiveOverruns++
	m.mu.Unlock()
}

func (m *metrics) addWriteFailure() {
	m.mu.Lock()
	m.writeFailures++
	m.mu.Unlock()
}

func (m *metrics) addWritten() {
	m.mu.Lock()
	m.eventsWritten++
	m.mu.Unlock()
}

func (m *metrics) setSocketBuffer(requested int, actual int, mode string) {
	m.mu.Lock()
	m.socketReceiveBufferRequested = requested
	m.socketReceiveBufferBytes = actual
	m.socketReceiveBufferMode = mode
	m.mu.Unlock()
}

// DefaultDirectory returns the conntrack history directory.
func DefaultDirectory(inputDir string) string {
	return filepath.Join(inputDir, "conntrack")
}

// DisabledStats returns an explicit disabled state for health reports.
func DisabledStats() Stats {
	return Stats{
		Enabled: false,
		Storage: storage.Stats{
			CompressionMode: "not_configured",
		},
		SocketReceiveBufferMode: "not_configured",
	}
}

func (tuple Tuple) addressMatches(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return false
	}
	return tuple.SourceIP == addr.String() || tuple.DestinationIP == addr.String()
}

func (tuple Tuple) String() string {
	source := tuple.SourceIP
	destination := tuple.DestinationIP
	if tuple.SourcePort != 0 {
		source = fmt.Sprintf("%s:%d", source, tuple.SourcePort)
	}
	if tuple.DestinationPort != 0 {
		destination = fmt.Sprintf("%s:%d", destination, tuple.DestinationPort)
	}
	return fmt.Sprintf("%s -> %s", source, destination)
}
