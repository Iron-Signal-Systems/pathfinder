package natcorrelation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

func TestCorrelateExactSourcePortPreservedTCPSYN(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lan := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)
	wan := tcpObservation(
		base.Add(2*time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{
		InputDir: dir,
		MaxDelta: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}

	if len(result.Relationships) != 1 {
		t.Fatalf("relationships = %d, want 1", len(result.Relationships))
	}

	got := result.Relationships[0]
	if got.CorrelationBasis != "tcp_syn_exact_signature_time_window_v1" {
		t.Fatalf("basis = %q", got.CorrelationBasis)
	}
	if got.LANSVI != "br0" {
		t.Fatalf("LAN SVI = %q, want br0", got.LANSVI)
	}
	if got.LANVLANID != 1 {
		t.Fatalf("LAN VLAN ID = %d, want 1", got.LANVLANID)
	}

	if got.LANSourceIP != "192.168.1.193" {
		t.Fatalf("LAN source = %q", got.LANSourceIP)
	}
	if got.WANSourceIP != "100.65.193.174" {
		t.Fatalf("WAN source = %q", got.WANSourceIP)
	}
	if !got.SourceIPTranslated {
		t.Fatal("source IP translated = false, want true")
	}
	if !got.SourcePortPreserved {
		t.Fatal("source port preserved = false, want true")
	}
	if !got.TCPInitiationCorrelated {
		t.Fatal("TCP initiation correlated = false, want true")
	}
	if got.MatchedEvents != 1 {
		t.Fatalf("matched events = %d, want 1", got.MatchedEvents)
	}
	if got.MaxAbsoluteDeltaMicros != 2000 {
		t.Fatalf(
			"max absolute delta = %d, want 2000",
			got.MaxAbsoluteDeltaMicros,
		)
	}
}

func TestCorrelateDoesNotUseEstablishedTCPPacket(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lan := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		51730,
		"172.64.148.235",
		443,
		52,
		[]string{"ACK"},
	)
	wan := tcpObservation(
		base.Add(time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		51730,
		"172.64.148.235",
		443,
		52,
		[]string{"ACK"},
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{InputDir: dir})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(result.Relationships) != 0 {
		t.Fatalf("relationships = %d, want 0", len(result.Relationships))
	}
}

func TestCorrelateDoesNotClaimChangedSourcePort(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lan := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)
	wan := tcpObservation(
		base.Add(time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		60000,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{
		InputDir: dir,
		MaxDelta: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}

	if len(result.Relationships) != 0 {
		t.Fatalf("relationships = %d, want 0", len(result.Relationships))
	}
}

func TestCorrelateRejectsAmbiguousCrossClientMatch(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lan1 := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.10",
		"00:11:22:33:44:10",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)
	lan2 := tcpObservation(
		base.Add(time.Millisecond),
		"br0",
		"ingress",
		"192.168.1.11",
		"00:11:22:33:44:11",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)
	wan := tcpObservation(
		base.Add(2*time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan1, lan2, wan},
	)

	result, err := Correlate(Options{
		InputDir: dir,
		MaxDelta: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}

	if len(result.Relationships) != 0 {
		t.Fatalf("relationships = %d, want 0", len(result.Relationships))
	}
	if result.Stats.AmbiguousLANCandidates == 0 {
		t.Fatal("expected ambiguous LAN candidate count > 0")
	}
}

func TestCorrelateUDPExactPacketSignature(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lan := udpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.53",
		"5a:4f:ca:ef:8f:02",
		53000,
		"9.9.9.9",
		53,
		72,
	)
	wan := udpObservation(
		base.Add(1500*time.Microsecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		53000,
		"9.9.9.9",
		53,
		72,
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{
		InputDir: dir,
		Protocol: "udp",
		MaxDelta: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}

	if len(result.Relationships) != 1 {
		t.Fatalf("relationships = %d, want 1", len(result.Relationships))
	}
	if result.Relationships[0].CorrelationBasis !=
		"udp_exact_packet_signature_time_window_v1" {
		t.Fatalf(
			"basis = %q",
			result.Relationships[0].CorrelationBasis,
		)
	}
}

func TestCorrelateDoesNotClaimOutsideTimeWindow(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Minute)

	lan := udpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.53",
		"5a:4f:ca:ef:8f:02",
		53000,
		"9.9.9.9",
		53,
		72,
	)
	wan := udpObservation(
		base.Add(time.Second),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		53000,
		"9.9.9.9",
		53,
		72,
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{
		InputDir: dir,
		MaxDelta: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(result.Relationships) != 0 {
		t.Fatalf("relationships = %d, want 0", len(result.Relationships))
	}
}

func TestCorrelateAddsHistoricalIdentity(t *testing.T) {
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
		filepath.Join(identityDir, "endpoint-identity-test.jsonl"),
		[]any{snapshot},
	)

	lan := tcpObservation(
		base,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)
	wan := tcpObservation(
		base.Add(time.Millisecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		51730,
		"172.64.148.235",
		443,
		60,
		[]string{"SYN"},
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{
		InputDir:    dir,
		IdentityDir: identityDir,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}

	if len(result.Relationships) != 1 {
		t.Fatalf("relationships = %d, want 1", len(result.Relationships))
	}
	if len(result.Relationships[0].HistoricalIdentities) != 1 {
		t.Fatalf(
			"historical identities = %d, want 1",
			len(result.Relationships[0].HistoricalIdentities),
		)
	}
	if result.Relationships[0].HistoricalIdentities[0].Hostname !=
		"Johns-Mac-mini" {
		t.Fatalf(
			"hostname = %q",
			result.Relationships[0].HistoricalIdentities[0].Hostname,
		)
	}
}

func tcpObservation(
	at time.Time,
	interfaceName string,
	direction string,
	sourceIP string,
	sourceMAC string,
	sourcePort uint16,
	destinationIP string,
	destinationPort uint16,
	ipLength uint32,
	flags []string,
) observation.Observation {
	current := observation.New()
	current.ObservedAt = at
	current.TimestampSource = observation.TimestampSourceKernelSoftwareNS
	current.Observed.Interface = interfaceName
	current.Context.Direction = direction
	if interfaceName == "br0" {
		current.Context.SVI = "br0"
		current.Context.VLANID = 1
	}
	current.Observed.Ethernet.SourceMAC = sourceMAC
	current.Observed.Network.SourceIP = sourceIP
	current.Observed.Network.DestinationIP = destinationIP
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Network.Length = ipLength
	current.Observed.Transport.SourcePort = sourcePort
	current.Observed.Transport.DestinationPort = destinationPort
	current.Observed.Transport.TCPFlags = flags
	return current
}

func udpObservation(
	at time.Time,
	interfaceName string,
	direction string,
	sourceIP string,
	sourceMAC string,
	sourcePort uint16,
	destinationIP string,
	destinationPort uint16,
	ipLength uint32,
) observation.Observation {
	current := observation.New()
	current.ObservedAt = at
	current.TimestampSource = observation.TimestampSourceKernelSoftwareNS
	current.Observed.Interface = interfaceName
	current.Context.Direction = direction
	if interfaceName == "br0" {
		current.Context.SVI = "br0"
		current.Context.VLANID = 1
	}
	current.Observed.Ethernet.SourceMAC = sourceMAC
	current.Observed.Network.SourceIP = sourceIP
	current.Observed.Network.DestinationIP = destinationIP
	current.Observed.Network.Protocol = "udp"
	current.Observed.Network.Length = ipLength
	current.Observed.Transport.SourcePort = sourcePort
	current.Observed.Transport.DestinationPort = destinationPort
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

func TestCorrelateSinceExcludesOldInitiation(t *testing.T) {
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
			61000,
			"203.0.113.10",
			443,
			60,
			[]string{"SYN"},
		),
		tcpObservation(
			oldAt.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			61000,
			"203.0.113.10",
			443,
			60,
			[]string{"SYN"},
		),
		tcpObservation(
			recentAt,
			"br0",
			"ingress",
			"192.168.1.193",
			"44:a9:2c:50:80:76",
			61001,
			"203.0.113.11",
			443,
			60,
			[]string{"SYN"},
		),
		tcpObservation(
			recentAt.Add(100*time.Microsecond),
			"eth8",
			"egress",
			"100.65.193.174",
			"68:d7:9a:35:bb:79",
			61001,
			"203.0.113.11",
			443,
			60,
			[]string{"SYN"},
		),
	}

	writeObservations(
		t,
		filepath.Join(dir, "observations-since.jsonl"),
		values,
	)

	result, err := Correlate(Options{
		InputDir: dir,
		MaxDelta: time.Millisecond,
		Protocol: "tcp",
		Since:    time.Minute,
		Source:   "192.168.1.193",
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}

	if len(result.Relationships) != 1 {
		t.Fatalf("relationships = %d, want 1", len(result.Relationships))
	}
	if result.Relationships[0].LANSourcePort != 61001 {
		t.Fatalf(
			"source port = %d, want 61001",
			result.Relationships[0].LANSourcePort,
		)
	}
	if result.Stats.ObservationsBeforeSince != 2 {
		t.Fatalf(
			"observations before since = %d, want 2",
			result.Stats.ObservationsBeforeSince,
		)
	}
}

func TestCorrelatePushesNetworkFiltersIntoCandidateCollection(t *testing.T) {
	dir := t.TempDir()
	at := time.Now().UTC().Add(-time.Second)

	lan := tcpObservation(
		at,
		"br0",
		"ingress",
		"192.168.1.193",
		"44:a9:2c:50:80:76",
		62000,
		"203.0.113.20",
		443,
		60,
		[]string{"SYN"},
	)
	wan := tcpObservation(
		at.Add(100*time.Microsecond),
		"eth8",
		"egress",
		"100.65.193.174",
		"68:d7:9a:35:bb:79",
		62000,
		"203.0.113.20",
		443,
		60,
		[]string{"SYN"},
	)

	writeObservations(
		t,
		filepath.Join(dir, "observations-filter.jsonl"),
		[]observation.Observation{lan, wan},
	)

	result, err := Correlate(Options{
		DestinationPort: 443,
		InputDir:        dir,
		MaxDelta:        time.Millisecond,
		Protocol:        "tcp",
		SVI:             "br0",
		VLANID:          1,
		VLANFilterSet:   true,
	})
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(result.Relationships) != 1 {
		t.Fatalf("relationships = %d, want 1", len(result.Relationships))
	}

	result, err = Correlate(Options{
		DestinationPort: 80,
		InputDir:        dir,
		MaxDelta:        time.Millisecond,
		Protocol:        "tcp",
		SVI:             "br0",
		VLANID:          1,
		VLANFilterSet:   true,
	})
	if err != nil {
		t.Fatalf("Correlate() mismatch error = %v", err)
	}
	if len(result.Relationships) != 0 {
		t.Fatalf(
			"mismatched relationships = %d, want 0",
			len(result.Relationships),
		)
	}
}
