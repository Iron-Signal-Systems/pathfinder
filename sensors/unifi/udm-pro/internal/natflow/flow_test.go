package natflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

func TestBuildTracksAllFourTCPPathLegs(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	values := []observation.Observation{
		tcpObservation(
			base,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(20*time.Millisecond),
			"eth8",
			"ingress",
			"34.107.243.93",
			"00:11:22:33:44:55",
			"68:d7:9a:35:bb:79",
			443,
			"100.65.193.174",
			61310,
			60,
			[]string{"SYN", "ACK"},
			false,
		),
		tcpObservation(
			base.Add(20100*time.Microsecond),
			"br0",
			"egress",
			"34.107.243.93",
			"68:d7:9a:35:bb:7a",
			"44:a9:2c:50:80:76",
			443,
			"192.168.1.193",
			61310,
			60,
			[]string{"SYN", "ACK"},
			false,
		),
		tcpObservation(
			base.Add(30*time.Millisecond),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			1200,
			[]string{"ACK", "PSH"},
			true,
		),
		tcpObservation(
			base.Add(30100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61310,
			"34.107.243.93",
			443,
			1200,
			[]string{"ACK", "PSH"},
			true,
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		values,
	)

	result, err := Build(Options{
		VLANID:     -1,
		InputDir:   dir,
		MaxDelta:   10 * time.Millisecond,
		MaxFlowAge: time.Minute,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}

	got := result.Flows[0]
	if got.LANSVI != "br0" {
		t.Fatalf("LAN SVI = %q, want br0", got.LANSVI)
	}
	if got.LANVLANID != 1 {
		t.Fatalf("LAN VLAN ID = %d, want 1", got.LANVLANID)
	}

	if got.AnchorAbsoluteDeltaMicros != 100 {
		t.Fatalf(
			"anchor absolute delta = %d, want 100",
			got.AnchorAbsoluteDeltaMicros,
		)
	}
	if got.AnchorLANTimestampSource !=
		observation.TimestampSourceKernelSoftwareNS {
		t.Fatalf(
			"LAN timestamp source = %q, want %q",
			got.AnchorLANTimestampSource,
			observation.TimestampSourceKernelSoftwareNS,
		)
	}
	if got.AnchorWANTimestampSource !=
		observation.TimestampSourceKernelSoftwareNS {
		t.Fatalf(
			"WAN timestamp source = %q, want %q",
			got.AnchorWANTimestampSource,
			observation.TimestampSourceKernelSoftwareNS,
		)
	}
	if got.LANOutbound.Observations != 2 {
		t.Fatalf(
			"LAN outbound observations = %d, want 2",
			got.LANOutbound.Observations,
		)
	}
	if got.WANOutbound.Observations != 2 {
		t.Fatalf(
			"WAN outbound observations = %d, want 2",
			got.WANOutbound.Observations,
		)
	}
	if got.WANReturn.Observations != 1 {
		t.Fatalf(
			"WAN return observations = %d, want 1",
			got.WANReturn.Observations,
		)
	}
	if got.LANReturn.Observations != 1 {
		t.Fatalf(
			"LAN return observations = %d, want 1",
			got.LANReturn.Observations,
		)
	}
	if !got.ReturnPathObserved {
		t.Fatal("return path observed = false, want true")
	}
	if got.PathStatus != "anchored_return_observed" {
		t.Fatalf("path status = %q", got.PathStatus)
	}
	if got.LANOutbound.OffloadSuspectedObservations != 1 {
		t.Fatalf(
			"LAN offload suspected = %d, want 1",
			got.LANOutbound.OffloadSuspectedObservations,
		)
	}
}

func TestBuildRefusesMultipleCorrelatedInitiationsForOneTuple(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	values := []observation.Observation{
		tcpObservation(
			base,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(time.Millisecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(2*time.Second),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(2001*time.Millisecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		values,
	)

	result, err := Build(Options{
		VLANID:     -1,
		InputDir:   dir,
		MaxDelta:   100 * time.Millisecond,
		MaxFlowAge: time.Minute,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 0 {
		t.Fatalf("flows = %d, want 0", len(result.Flows))
	}
	if result.Stats.AnchorsRefusedMultipleInitiations != 1 {
		t.Fatalf(
			"refused = %d, want 1",
			result.Stats.AnchorsRefusedMultipleInitiations,
		)
	}
}

func TestBuildAttachesHistoricalIdentityAndDNSAtInitiation(t *testing.T) {
	dir := t.TempDir()
	identityDir := filepath.Join(dir, "identity")
	if err := os.MkdirAll(identityDir, 0o750); err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Add(-time.Minute)

	snapshot := identityhistory.Snapshot{
		Endpoints: []identityhistory.Endpoint{
			{
				FQDN:           "Johns-Mac-mini.woodsnet.us",
				Hostname:       "Johns-Mac-mini",
				IP:             "192.168.1.193",
				LeaseExpiresAt: base.Add(time.Hour).Format(time.RFC3339),
				MAC:            "44:a9:2c:50:80:76",
			},
		},
		ExpectedIntervalSeconds: 10,
		ObservedAt:              base.Add(-5 * time.Second),
		RecordType:              identityhistory.RecordType,
		SchemaVersion:           identityhistory.SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}
	writeJSONL(
		t,
		filepath.Join(identityDir, "endpoint-identity-test.jsonl"),
		[]any{snapshot},
	)

	dns := observation.New()
	dns.ObservedAt = base.Add(-time.Second)
	dns.Observed.Interface = "br0"
	dns.Context.Direction = "egress"
	dns.Observed.Ethernet.DestinationMAC = "44:a9:2c:50:80:76"
	dns.Observed.Network.SourceIP = "192.168.1.1"
	dns.Observed.Network.DestinationIP = "192.168.1.193"
	dns.Observed.Network.Protocol = "udp"
	dns.Observed.DNS.MessageType = "response"
	dns.Observed.DNS.QueryName = "example.test"
	dns.Observed.DNS.Answers = []observation.DNSRecord{
		{
			Class: "IN",
			Name:  "example.test",
			TTL:   60,
			Type:  "A",
			Value: "34.107.243.93",
		},
	}

	lan := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		"68:d7:9a:35:bb:7a",
		61310,
		"34.107.243.93",
		443,
		60,
		[]string{"SYN"},
		false,
	)
	wan := tcpObservation(
		base.Add(time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		"00:11:22:33:44:55",
		61310,
		"34.107.243.93",
		443,
		60,
		[]string{"SYN"},
		false,
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{dns, lan, wan},
	)

	result, err := Build(Options{
		VLANID:      -1,
		InputDir:    dir,
		IdentityDir: identityDir,
		MaxDelta:    100 * time.Millisecond,
		MaxFlowAge:  time.Minute,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	got := result.Flows[0]

	if len(got.HistoricalIdentities) != 1 {
		t.Fatalf(
			"historical identities = %d, want 1",
			len(got.HistoricalIdentities),
		)
	}
	if got.HistoricalIdentities[0].Hostname != "Johns-Mac-mini" {
		t.Fatalf(
			"hostname = %q",
			got.HistoricalIdentities[0].Hostname,
		)
	}
	if len(got.DNSAtInitiation) != 1 {
		t.Fatalf(
			"DNS at initiation = %d, want 1",
			len(got.DNSAtInitiation),
		)
	}
	if got.DNSAtInitiation[0].QueryName != "example.test" {
		t.Fatalf(
			"DNS query name = %q",
			got.DNSAtInitiation[0].QueryName,
		)
	}
}

func TestBuildStopsAssignmentAfterRST(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	values := []observation.Observation{
		tcpObservation(
			base,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(time.Millisecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61310,
			"34.107.243.93",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(2*time.Second),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			52,
			[]string{"RST", "ACK"},
			false,
		),
		tcpObservation(
			base.Add(3*time.Second),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61310,
			"34.107.243.93",
			443,
			100,
			[]string{"ACK"},
			false,
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		values,
	)

	result, err := Build(Options{
		VLANID:     -1,
		InputDir:   dir,
		MaxDelta:   100 * time.Millisecond,
		MaxFlowAge: time.Minute,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	got := result.Flows[0]
	if !got.TCPRSTObserved {
		t.Fatal("RST observed = false, want true")
	}
	if got.LANOutbound.Observations != 2 {
		t.Fatalf(
			"LAN outbound observations = %d, want 2",
			got.LANOutbound.Observations,
		)
	}
	if got.PathStatus != "anchored_rst_observed" {
		t.Fatalf("path status = %q", got.PathStatus)
	}
}

func tcpObservation(
	at time.Time,
	interfaceName string,
	direction string,
	sourceIP string,
	sourceMAC string,
	destinationMAC string,
	sourcePort uint16,
	destinationIP string,
	destinationPort uint16,
	ipLength uint32,
	flags []string,
	offload bool,
) observation.Observation {
	current := observation.New()
	current.ObservedAt = at
	current.TimestampSource = observation.TimestampSourceKernelSoftwareNS
	current.Observed.Interface = interfaceName
	current.Context.Direction = direction
	current.Context.KernelOffloadSuspected = offload
	if interfaceName == "br0" {
		current.Context.SVI = "br0"
		current.Context.VLANID = 1
	}
	current.Observed.Ethernet.SourceMAC = sourceMAC
	current.Observed.Ethernet.DestinationMAC = destinationMAC
	current.Observed.Network.SourceIP = sourceIP
	current.Observed.Network.DestinationIP = destinationIP
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Network.Length = ipLength
	current.Observed.Transport.SourcePort = sourcePort
	current.Observed.Transport.DestinationPort = destinationPort
	current.Observed.Transport.TCPFlags = flags
	return current
}

func writeJSONL(t *testing.T, path string, values []any) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, value := range values {
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
	}
}

func writeObservations(
	t *testing.T,
	path string,
	values []observation.Observation,
) {
	t.Helper()

	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	writeJSONL(t, path, items)
}

func TestBuildReadsActiveObservationAndIdentitySegments(t *testing.T) {
	dir := t.TempDir()
	identityDir := filepath.Join(dir, "identity")
	if err := os.MkdirAll(identityDir, 0o750); err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Add(-time.Minute)

	snapshot := identityhistory.Snapshot{
		Endpoints: []identityhistory.Endpoint{
			{
				FQDN:           "Johns-Mac-mini.woodsnet.us",
				Hostname:       "Johns-Mac-mini",
				IP:             "192.168.1.193",
				LeaseExpiresAt: base.Add(time.Hour).Format(time.RFC3339),
				MAC:            "44:a9:2c:50:80:76",
			},
		},
		ExpectedIntervalSeconds: 10,
		ObservedAt:              base.Add(-time.Second),
		RecordType:              identityhistory.RecordType,
		SchemaVersion:           identityhistory.SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}
	writeJSONL(
		t,
		filepath.Join(identityDir, "endpoint-identity-test.jsonl.active"),
		[]any{snapshot},
	)

	lan := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		"68:d7:9a:35:bb:7a",
		61310,
		"34.107.243.93",
		443,
		60,
		[]string{"SYN"},
		false,
	)
	wan := tcpObservation(
		base.Add(time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		"00:11:22:33:44:55",
		61310,
		"34.107.243.93",
		443,
		60,
		[]string{"SYN"},
		false,
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations-test.jsonl.active"),
		[]observation.Observation{lan, wan},
	)

	result, err := Build(Options{
		VLANID:      -1,
		InputDir:    dir,
		IdentityDir: identityDir,
		MaxDelta:    100 * time.Millisecond,
		MaxFlowAge:  time.Minute,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	if result.Stats.ActiveObservationFilesRead != 1 {
		t.Fatalf(
			"active observation files = %d, want 1",
			result.Stats.ActiveObservationFilesRead,
		)
	}
	if result.Stats.ActiveIdentityFilesRead != 1 {
		t.Fatalf(
			"active identity files = %d, want 1",
			result.Stats.ActiveIdentityFilesRead,
		)
	}
	if result.Flows[0].LANSVI != "br0" || result.Flows[0].LANVLANID != 1 {
		t.Fatalf(
			"network context = %s vlan %d, want br0 vlan 1",
			result.Flows[0].LANSVI,
			result.Flows[0].LANVLANID,
		)
	}
}

func TestBuildFiltersBySVIVLANDestinationPort(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	values := []observation.Observation{
		tcpObservation(
			base,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61000,
			"203.0.113.10",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61000,
			"203.0.113.10",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(time.Second),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61001,
			"203.0.113.11",
			80,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(time.Second+100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61001,
			"203.0.113.11",
			80,
			60,
			[]string{"SYN"},
			false,
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		values,
	)

	result, err := Build(Options{
		DestinationPort: 443,
		InputDir:        dir,
		MaxDelta:        10 * time.Millisecond,
		MaxFlowAge:      time.Minute,
		SVI:             "br0",
		VLANID:          1,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	got := result.Flows[0]
	if got.DestinationIP != "203.0.113.10" {
		t.Fatalf("destination = %q, want 203.0.113.10", got.DestinationIP)
	}
	if got.DestinationPort != 443 {
		t.Fatalf("destination port = %d, want 443", got.DestinationPort)
	}
	if got.LANSVI != "br0" || got.LANVLANID != 1 {
		t.Fatalf(
			"network context = %s/%d, want br0/1",
			got.LANSVI,
			got.LANVLANID,
		)
	}
}

func TestBuildFilterMismatchReturnsNoFlows(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	values := []observation.Observation{
		tcpObservation(
			base,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			61000,
			"203.0.113.10",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			base.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			61000,
			"203.0.113.10",
			443,
			60,
			[]string{"SYN"},
			false,
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		values,
	)

	result, err := Build(Options{
		DestinationPort: 443,
		InputDir:        dir,
		MaxDelta:        10 * time.Millisecond,
		MaxFlowAge:      time.Minute,
		SVI:             "br99",
		VLANID:          99,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 0 {
		t.Fatalf("flows = %d, want 0", len(result.Flows))
	}
}

func TestBuildSinceReturnsOnlyRecentAnchors(t *testing.T) {
	dir := t.TempDir()
	oldAt := time.Now().UTC().Add(-10 * time.Minute)
	recentAt := time.Now().UTC().Add(-10 * time.Second)

	values := []observation.Observation{
		tcpObservation(
			oldAt,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			63000,
			"203.0.113.30",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			oldAt.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			63000,
			"203.0.113.30",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			recentAt,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			63001,
			"203.0.113.31",
			443,
			60,
			[]string{"SYN"},
			false,
		),
		tcpObservation(
			recentAt.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			63001,
			"203.0.113.31",
			443,
			60,
			[]string{"SYN"},
			false,
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations-since.jsonl"),
		values,
	)

	result, err := Build(Options{
		InputDir:   dir,
		MaxDelta:   time.Millisecond,
		MaxFlowAge: time.Minute,
		Since:      time.Minute,
		Source:     "192.168.1.193",
		VLANID:     -1,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	if result.Flows[0].LANSourcePort != 63001 {
		t.Fatalf(
			"source port = %d, want 63001",
			result.Flows[0].LANSourcePort,
		)
	}
	if result.Stats.ObservationsBeforeSince != 2 {
		t.Fatalf(
			"second-pass observations before since = %d, want 2",
			result.Stats.ObservationsBeforeSince,
		)
	}
}

func TestBuildPreservesObservedLANClientHelloSNI(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lanSYN := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		"68:d7:9a:35:bb:7a",
		64000,
		"203.0.113.40",
		443,
		60,
		[]string{"SYN"},
		false,
	)
	wanSYN := tcpObservation(
		base.Add(100*time.Microsecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		"00:11:22:33:44:55",
		64000,
		"203.0.113.40",
		443,
		60,
		[]string{"SYN"},
		false,
	)
	clientHello := tcpObservation(
		base.Add(10*time.Millisecond),
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		"68:d7:9a:35:bb:7a",
		64000,
		"203.0.113.40",
		443,
		300,
		[]string{"PSH", "ACK"},
		false,
	)
	clientHello.Observed.TLS.ClientHelloObserved = true
	clientHello.Observed.TLS.SNI = "www.example.com"

	writeObservations(
		t,
		filepath.Join(dir, "observations-sni.jsonl"),
		[]observation.Observation{
			lanSYN,
			wanSYN,
			clientHello,
		},
	)

	result, err := Build(Options{
		InputDir:   dir,
		MaxDelta:   time.Millisecond,
		MaxFlowAge: time.Minute,
		VLANID:     -1,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	if len(result.Flows[0].TLSAtInitiation) != 1 {
		t.Fatalf(
			"TLS contexts = %d, want 1",
			len(result.Flows[0].TLSAtInitiation),
		)
	}
	if result.Flows[0].TLSAtInitiation[0].SNI != "www.example.com" {
		t.Fatalf(
			"SNI = %q, want www.example.com",
			result.Flows[0].TLSAtInitiation[0].SNI,
		)
	}
	if result.Stats.TLSClientHellosObserved != 1 {
		t.Fatalf(
			"TLS ClientHellos = %d, want 1",
			result.Stats.TLSClientHellosObserved,
		)
	}
	if result.Stats.TLSSNINamesObserved != 1 {
		t.Fatalf(
			"TLS SNI names = %d, want 1",
			result.Stats.TLSSNINamesObserved,
		)
	}
}

func TestBuildDoesNotUseWANOnlySNIForInternalAttribution(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lanSYN := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		"68:d7:9a:35:bb:7a",
		64001,
		"203.0.113.41",
		443,
		60,
		[]string{"SYN"},
		false,
	)
	wanSYN := tcpObservation(
		base.Add(100*time.Microsecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		"00:11:22:33:44:55",
		64001,
		"203.0.113.41",
		443,
		60,
		[]string{"SYN"},
		false,
	)
	wanHello := tcpObservation(
		base.Add(10*time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		"00:11:22:33:44:55",
		64001,
		"203.0.113.41",
		443,
		300,
		[]string{"PSH", "ACK"},
		false,
	)
	wanHello.Observed.TLS.ClientHelloObserved = true
	wanHello.Observed.TLS.SNI = "wan-only.example.com"

	writeObservations(
		t,
		filepath.Join(dir, "observations-wan-sni.jsonl"),
		[]observation.Observation{
			lanSYN,
			wanSYN,
			wanHello,
		},
	)

	result, err := Build(Options{
		InputDir:   dir,
		MaxDelta:   time.Millisecond,
		MaxFlowAge: time.Minute,
		VLANID:     -1,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	if len(result.Flows[0].TLSAtInitiation) != 0 {
		t.Fatalf(
			"TLS contexts = %d, want 0",
			len(result.Flows[0].TLSAtInitiation),
		)
	}
}

func TestFlowMatchesSNIExactCaseInsensitive(t *testing.T) {
	contexts := []TLSContext{
		{
			Interface:  "br0",
			ObservedAt: time.Now().UTC(),
			SNI:        "Alive.GitHub.Com",
		},
	}

	matched, err := flowMatchesSNI("alive.github.com", contexts)
	if err != nil {
		t.Fatalf("flowMatchesSNI() error = %v", err)
	}
	if !matched {
		t.Fatal("flowMatchesSNI() = false, want true")
	}
}

func TestFlowMatchesSNIGlobCaseInsensitive(t *testing.T) {
	contexts := []TLSContext{
		{
			Interface:  "br0",
			ObservedAt: time.Now().UTC(),
			SNI:        "ohttp-merino.mozilla.fastly-edge.com",
		},
	}

	matched, err := flowMatchesSNI("*.MOZILLA.*", contexts)
	if err != nil {
		t.Fatalf("flowMatchesSNI() error = %v", err)
	}
	if !matched {
		t.Fatal("flowMatchesSNI() = false, want true")
	}
}

func TestFlowMatchesSNIRequiresObservedTLSName(t *testing.T) {
	matched, err := flowMatchesSNI(
		"*.example.com",
		[]TLSContext{},
	)
	if err != nil {
		t.Fatalf("flowMatchesSNI() error = %v", err)
	}
	if matched {
		t.Fatal("flowMatchesSNI() = true, want false")
	}
}

func TestValidateSNIPatternRejectsMalformedGlob(t *testing.T) {
	if err := validateSNIPattern("[broken"); err == nil {
		t.Fatal("validateSNIPattern() error = nil, want error")
	}
}

func TestBuildFiltersByObservedSNI(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	makeFlow := func(
		sourcePort uint16,
		destinationIP string,
		sni string,
		offset time.Duration,
	) []observation.Observation {
		lanSYN := tcpObservation(
			base.Add(offset),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			sourcePort,
			destinationIP,
			443,
			60,
			[]string{"SYN"},
			false,
		)
		wanSYN := tcpObservation(
			base.Add(offset+100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			"00:11:22:33:44:55",
			sourcePort,
			destinationIP,
			443,
			60,
			[]string{"SYN"},
			false,
		)
		hello := tcpObservation(
			base.Add(offset+10*time.Millisecond),
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			"68:d7:9a:35:bb:7a",
			sourcePort,
			destinationIP,
			443,
			300,
			[]string{"PSH", "ACK"},
			false,
		)
		hello.Observed.TLS.ClientHelloObserved = true
		hello.Observed.TLS.SNI = sni

		return []observation.Observation{lanSYN, wanSYN, hello}
	}

	values := append(
		makeFlow(
			65000,
			"203.0.113.50",
			"alive.github.com",
			0,
		),
		makeFlow(
			65001,
			"203.0.113.51",
			"ohttp-merino.mozilla.fastly-edge.com",
			time.Second,
		)...,
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations-sni-filter.jsonl"),
		values,
	)

	result, err := Build(Options{
		InputDir:   dir,
		MaxDelta:   time.Millisecond,
		MaxFlowAge: time.Minute,
		SNI:        "*.mozilla.*",
		VLANID:     -1,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.Flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(result.Flows))
	}
	if len(result.Flows[0].TLSAtInitiation) != 1 {
		t.Fatalf(
			"TLS contexts = %d, want 1",
			len(result.Flows[0].TLSAtInitiation),
		)
	}
	if result.Flows[0].TLSAtInitiation[0].SNI !=
		"ohttp-merino.mozilla.fastly-edge.com" {
		t.Fatalf(
			"SNI = %q",
			result.Flows[0].TLSAtInitiation[0].SNI,
		)
	}
	if result.Stats.FlowsFilteredBySNI != 1 {
		t.Fatalf(
			"flows filtered by SNI = %d, want 1",
			result.Stats.FlowsFilteredBySNI,
		)
	}
	if result.Stats.FlowsFound != 1 {
		t.Fatalf(
			"flows found = %d, want 1",
			result.Stats.FlowsFound,
		)
	}
}
