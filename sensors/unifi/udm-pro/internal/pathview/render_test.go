package pathview

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natcorrelation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natflow"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

func TestWriteTextRendersAuditablePath(t *testing.T) {
	start := time.Date(
		2026, time.September, 13, 18, 59, 44, 521155821, time.UTC,
	)

	result := natflow.Result{
		Flows: []natflow.Flow{
			{
				AnchorAbsoluteDeltaMicros: 129,
				AnchorLANObservedAt:       start,
				AnchorLANTimestampSource:  observation.TimestampSourceKernelSoftwareNS,
				AnchorWANObservedAt:       start.Add(129 * time.Microsecond),
				AnchorWANTimestampSource:  observation.TimestampSourceKernelSoftwareNS,
				ClosedAt:                  observation.ValueNoRecord,
				CorrelationBasis:          "tcp_syn_exact_signature_time_window_v1",
				DestinationIP:             "34.107.243.93",
				DestinationPort:           443,
				HistoricalIdentities: []natcorrelation.HistoricalIdentitySummary{
					{
						FQDN:               "Johns-Mac-mini.woodsnet.us",
						Hostname:           "Johns-Mac-mini",
						SnapshotObservedAt: start.Add(-3 * time.Second),
						Source:             "dnsmasq_lease_snapshot",
						Status:             "matched",
					},
				},
				LANInterface: "br0",
				LANOutbound: natflow.LegStats{
					Observations:    10,
					ObservedIPBytes: 3240,
				},
				LANReturn: natflow.LegStats{
					Observations:    9,
					ObservedIPBytes: 1362,
				},
				LANSourceIP:   "192.168.1.193",
				LANSourceMAC:  "44:a9:2c:50:80:76",
				LANSourcePort: 61309,
				LANSVI:        "br0",
				LANVLANID:     1,
				LastSeen:      start.Add(58 * time.Second),
				TLSAtInitiation: []natflow.TLSContext{
					{
						Interface:  "br0",
						ObservedAt: start.Add(10 * time.Millisecond),
						SNI:        "www.example.com",
					},
				},
				PathStatus:              "anchored_return_observed",
				Protocol:                "tcp",
				ReturnPathObserved:      true,
				SourceIPTranslated:      true,
				SourcePortPreserved:     true,
				TCPInitiationCorrelated: true,
				WANInterface:            "eth8",
				WANOutbound: natflow.LegStats{
					Observations:    11,
					ObservedIPBytes: 3292,
				},
				WANReturn: natflow.LegStats{
					Observations:    9,
					ObservedIPBytes: 1362,
				},
				WANSourceIP:   "100.65.193.174",
				WANSourcePort: 61309,
			},
		},
	}

	var buffer bytes.Buffer
	if err := WriteText(&buffer, result); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}

	output := buffer.String()
	for _, expected := range []string{
		"Path 1 - Johns-Mac-mini",
		"Observed LAN IP:           192.168.1.193",
		"Observed LAN MAC:          44:a9:2c:50:80:76",
		"source NAT: 100.65.193.174:61309",
		"34.107.243.93:443",
		"Observation timestamp delta: 129 microseconds",
		"LAN timestamp source:      kernel_software_ns",
		"WAN timestamp source:      kernel_software_ns",
		"not asserted as forwarding latency",
		"LAN outbound:",
		"10 observations",
		"3240 observed IP bytes",
		"WAN outbound:",
		"11 observations",
		"3292 observed IP bytes",
		"Return path observed:       yes",
		"Close:                      no close observed",
		"No qualifying DNS answer observed",
		"TLS at initiation",
		"Observed SNI:              www.example.com",
		"Vantage:                   br0 ingress",
		"Network context",
		"SVI:                       br0",
		"VLAN ID:                   1",
		"counts/bytes are per observation point",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf(
				"output missing %q\n--- output ---\n%s",
				expected,
				output,
			)
		}
	}
}

func TestWriteTextRendersDNSAtInitiation(t *testing.T) {
	start := time.Now().UTC().Add(-time.Minute)

	result := natflow.Result{
		Flows: []natflow.Flow{
			{
				AnchorAbsoluteDeltaMicros: 50,
				AnchorLANObservedAt:       start,
				AnchorLANTimestampSource:  observation.TimestampSourceKernelSoftwareNS,
				AnchorWANObservedAt:       start.Add(50 * time.Microsecond),
				AnchorWANTimestampSource:  observation.TimestampSourceKernelSoftwareNS,
				ClosedAt:                  observation.ValueNoRecord,
				CorrelationBasis:          "tcp_syn_exact_signature_time_window_v1",
				DNSAtInitiation: []natflow.DNSContext{
					{
						Address:    "203.0.113.7",
						AnswerName: "cdn.example.test",
						ExpiresAt:  start.Add(time.Minute),
						ObservedAt: start.Add(-time.Second),
						QueryName:  "www.example.test",
						ResolverIP: "192.168.1.1",
						TTL:        60,
					},
				},
				DestinationIP:       "203.0.113.7",
				DestinationPort:     443,
				LANInterface:        "br0",
				LANSourceIP:         "192.168.1.54",
				LANSourceMAC:        "00:11:22:33:44:55",
				LANSourcePort:       50000,
				LastSeen:            start,
				PathStatus:          "anchored_no_return_observed",
				Protocol:            "tcp",
				SourceIPTranslated:  true,
				SourcePortPreserved: true,
				WANInterface:        "eth8",
				WANSourceIP:         "100.65.193.174",
				WANSourcePort:       50000,
			},
		},
	}

	var buffer bytes.Buffer
	if err := WriteText(&buffer, result); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}

	output := buffer.String()
	for _, expected := range []string{
		"www.example.test -> 203.0.113.7",
		"resolver:    192.168.1.1",
		"ttl:         60 seconds",
		"answer name: cdn.example.test",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf(
				"output missing %q\n--- output ---\n%s",
				expected,
				output,
			)
		}
	}
}

func TestWriteTextNoFlows(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteText(&buffer, natflow.Result{}); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	if !strings.Contains(
		buffer.String(),
		"No qualifying NAT-anchored TCP paths found.",
	) {
		t.Fatalf("unexpected output: %q", buffer.String())
	}
}
