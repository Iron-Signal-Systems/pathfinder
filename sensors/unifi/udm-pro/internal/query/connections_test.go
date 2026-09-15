package query

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

func TestConnectionsCorrelatesObservedDNSWithinTTL(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Second)

	dns := observation.New()
	dns.ObservedAt = base
	dns.Observed.Interface = "br0"
	dns.Context.Direction = "egress"
	dns.Observed.Network.SourceIP = "192.168.1.1"
	dns.Observed.Network.DestinationIP = "192.168.1.54"
	dns.Observed.Network.Protocol = "udp"
	dns.Observed.Transport.SourcePort = 53
	dns.Observed.DNS.MessageType = "response"
	dns.Observed.DNS.QueryName = "www.example.com"
	dns.Observed.DNS.QueryType = "A"
	dns.Observed.DNS.Answers = []observation.DNSRecord{{
		Class: "IN", Name: "www.example.com", TTL: 300,
		Type: "A", Value: "203.0.113.7",
	}}

	connection := observation.New()
	connection.ObservedAt = base.Add(200 * time.Millisecond)
	connection.Observed.Interface = "br0"
	connection.Context.Direction = "ingress"
	connection.Observed.Network.SourceIP = "192.168.1.54"
	connection.Observed.Network.DestinationIP = "203.0.113.7"
	connection.Observed.Network.Protocol = "tcp"
	connection.Observed.Transport.DestinationPort = 443
	connection.Observed.Transport.SourcePort = 52184
	connection.Observed.Transport.TCPFlags = []string{"SYN"}

	writeObservations(t, filepath.Join(dir, "observations-1.jsonl"), []observation.Observation{dns, connection})

	result, err := Connections(ConnectionOptions{InputDir: dir, Interface: "br0", Direction: "ingress"})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}

	got := result.Connections[0]
	if !got.TCPInitiationObserved {
		t.Fatal("TCP initiation was not detected")
	}
	if len(got.DNS) != 1 {
		t.Fatalf("DNS contexts = %d, want 1", len(got.DNS))
	}
	if got.DNS[0].QueryName != "www.example.com" {
		t.Fatalf("query name = %q", got.DNS[0].QueryName)
	}
	if got.DNS[0].TTL != 300 {
		t.Fatalf("TTL = %d, want 300", got.DNS[0].TTL)
	}
}

func TestConnectionsDoesNotUseExpiredDNS(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Second)

	dns := observation.New()
	dns.ObservedAt = base
	dns.Observed.Interface = "br0"
	dns.Observed.Network.SourceIP = "192.168.1.1"
	dns.Observed.Network.DestinationIP = "192.168.1.54"
	dns.Observed.DNS.MessageType = "response"
	dns.Observed.DNS.QueryName = "expired.example"
	dns.Observed.DNS.Answers = []observation.DNSRecord{{
		Class: "IN", Name: "expired.example", TTL: 1,
		Type: "A", Value: "203.0.113.8",
	}}

	connection := observation.New()
	connection.ObservedAt = base.Add(2 * time.Second)
	connection.Observed.Interface = "br0"
	connection.Context.Direction = "ingress"
	connection.Observed.Network.SourceIP = "192.168.1.54"
	connection.Observed.Network.DestinationIP = "203.0.113.8"
	connection.Observed.Network.Protocol = "tcp"
	connection.Observed.Transport.DestinationPort = 443

	writeObservations(t, filepath.Join(dir, "observations-1.jsonl"), []observation.Observation{dns, connection})

	result, err := Connections(ConnectionOptions{InputDir: dir})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
	if len(result.Connections[0].DNS) != 0 {
		t.Fatalf("DNS contexts = %d, want 0", len(result.Connections[0].DNS))
	}
}

func TestConnectionsReadsGzipAndFiltersCIDR(t *testing.T) {
	dir := t.TempDir()

	current := observation.New()
	current.ObservedAt = time.Now().UTC().Add(-time.Second)
	current.Observed.Interface = "br0"
	current.Context.Direction = "ingress"
	current.Observed.Network.SourceIP = "192.168.1.25"
	current.Observed.Network.DestinationIP = "198.51.100.20"
	current.Observed.Network.Protocol = "udp"
	current.Observed.Transport.DestinationPort = 443

	writeGzipObservations(t, filepath.Join(dir, "observations-1.jsonl.gz"), []observation.Observation{current})

	result, err := Connections(ConnectionOptions{InputDir: dir, Source: "192.168.1.0/24"})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
}

func writeGzipObservations(t *testing.T, path string, observations []observation.Observation) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	gzipWriter := gzip.NewWriter(file)
	encoder := json.NewEncoder(gzipWriter)
	for _, current := range observations {
		if err := encoder.Encode(current); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}

	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip Close() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("file Close() error = %v", err)
	}
}

func writeObservations(t *testing.T, path string, observations []observation.Observation) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, current := range observations {
		if err := encoder.Encode(current); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}
}

func TestConnectionsDoesNotCrossObservationInterfacesForDNS(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Second)

	dns := observation.New()
	dns.ObservedAt = base
	dns.Observed.Interface = "eth8"
	dns.Observed.Network.SourceIP = "1.1.1.1"
	dns.Observed.Network.DestinationIP = "100.65.193.174"
	dns.Observed.DNS.MessageType = "response"
	dns.Observed.DNS.QueryName = "wrong.example"
	dns.Observed.DNS.Answers = []observation.DNSRecord{{
		Class: "IN", Name: "wrong.example", TTL: 300,
		Type: "A", Value: "203.0.113.7",
	}}

	connection := observation.New()
	connection.ObservedAt = base.Add(time.Second)
	connection.Observed.Interface = "br0"
	connection.Context.Direction = "ingress"
	connection.Observed.Network.SourceIP = "192.168.1.54"
	connection.Observed.Network.DestinationIP = "203.0.113.7"
	connection.Observed.Network.Protocol = "tcp"
	connection.Observed.Transport.DestinationPort = 443

	writeObservations(t, filepath.Join(dir, "observations-1.jsonl"), []observation.Observation{
		dns, connection,
	})

	result, err := Connections(ConnectionOptions{
		InputDir: dir, Interface: "br0", Direction: "ingress",
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
	if len(result.Connections[0].DNS) != 0 {
		t.Fatalf("DNS contexts = %d, want 0", len(result.Connections[0].DNS))
	}
}

func TestConnectionsAutoDisablesDNSOnWANStyleInterface(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Second)

	dns := observation.New()
	dns.ObservedAt = base
	dns.Observed.Interface = "eth8"
	dns.Observed.Network.SourceIP = "1.1.1.1"
	dns.Observed.Network.DestinationIP = "100.65.193.174"
	dns.Observed.DNS.MessageType = "response"
	dns.Observed.DNS.QueryName = "ambiguous.example"
	dns.Observed.DNS.Answers = []observation.DNSRecord{{
		Class: "IN", Name: "ambiguous.example", TTL: 300,
		Type: "A", Value: "203.0.113.9",
	}}

	connection := observation.New()
	connection.ObservedAt = base.Add(time.Second)
	connection.Observed.Interface = "eth8"
	connection.Context.Direction = "egress"
	connection.Observed.Network.SourceIP = "100.65.193.174"
	connection.Observed.Network.DestinationIP = "203.0.113.9"
	connection.Observed.Network.Protocol = "tcp"
	connection.Observed.Transport.DestinationPort = 443

	writeObservations(t, filepath.Join(dir, "observations-1.jsonl"), []observation.Observation{
		dns, connection,
	})

	result, err := Connections(ConnectionOptions{
		InputDir: dir, Interface: "eth8", Direction: "egress", DNSMode: "auto",
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
	if len(result.Connections[0].DNS) != 0 {
		t.Fatalf("DNS contexts = %d, want 0", len(result.Connections[0].DNS))
	}
}

func TestConnectionsCanExplicitlyEnableDNSOnWANStyleInterface(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Second)

	dns := observation.New()
	dns.ObservedAt = base
	dns.Observed.Interface = "eth8"
	dns.Observed.Network.SourceIP = "1.1.1.1"
	dns.Observed.Network.DestinationIP = "100.65.193.174"
	dns.Observed.DNS.MessageType = "response"
	dns.Observed.DNS.QueryName = "explicit.example"
	dns.Observed.DNS.Answers = []observation.DNSRecord{{
		Class: "IN", Name: "explicit.example", TTL: 300,
		Type: "A", Value: "203.0.113.10",
	}}

	connection := observation.New()
	connection.ObservedAt = base.Add(time.Second)
	connection.Observed.Interface = "eth8"
	connection.Context.Direction = "egress"
	connection.Observed.Network.SourceIP = "100.65.193.174"
	connection.Observed.Network.DestinationIP = "203.0.113.10"
	connection.Observed.Network.Protocol = "tcp"
	connection.Observed.Transport.DestinationPort = 443

	writeObservations(t, filepath.Join(dir, "observations-1.jsonl"), []observation.Observation{
		dns, connection,
	})

	result, err := Connections(ConnectionOptions{
		InputDir: dir, Interface: "eth8", Direction: "egress", DNSMode: "client",
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
	if len(result.Connections[0].DNS) != 1 {
		t.Fatalf("DNS contexts = %d, want 1", len(result.Connections[0].DNS))
	}
}

func TestConnectionsStatsDistinguishFoundFromReturned(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Second)

	for i, destination := range []string{"203.0.113.1", "203.0.113.2"} {
		current := observation.New()
		current.ObservedAt = base.Add(time.Duration(i) * time.Millisecond)
		current.Observed.Interface = "br0"
		current.Context.Direction = "ingress"
		current.Observed.Network.SourceIP = "192.168.1.54"
		current.Observed.Network.DestinationIP = destination
		current.Observed.Network.Protocol = "tcp"
		current.Observed.Transport.DestinationPort = 443

		writeObservations(
			t,
			filepath.Join(dir, "observations-"+destination+".jsonl"),
			[]observation.Observation{current},
		)
	}

	result, err := Connections(ConnectionOptions{
		InputDir: dir, Limit: 1,
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if result.Stats.RelationshipsFound != 2 {
		t.Fatalf("relationships found = %d, want 2", result.Stats.RelationshipsFound)
	}
	if result.Stats.RelationshipsReturned != 1 {
		t.Fatalf("relationships returned = %d, want 1", result.Stats.RelationshipsReturned)
	}
}

func TestConnectionsSeparatesSameIPByObservedSourceMAC(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Second)

	first := observation.New()
	first.ObservedAt = base
	first.Observed.Interface = "br0"
	first.Context.Direction = "ingress"
	first.Observed.Network.SourceIP = "192.168.1.50"
	first.Observed.Network.DestinationIP = "203.0.113.20"
	first.Observed.Network.Protocol = "tcp"
	first.Observed.Transport.DestinationPort = 443
	first.Observed.Ethernet.SourceMAC = "00:11:22:33:44:55"

	second := first
	second.ObservedAt = base.Add(time.Millisecond)
	second.Observed.Ethernet.SourceMAC = "00:11:22:33:44:66"

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{first, second},
	)

	result, err := Connections(ConnectionOptions{
		InputDir:     dir,
		IdentityMode: "off",
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}

	if len(result.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(result.Connections))
	}
}

func TestConnectionsAddsCurrentDHCPIdentityOnExactIPMACMatch(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-time.Second)

	leasePath := filepath.Join(dir, "dnsmasq.lease")
	hostsPath := filepath.Join(dir, "hosts")

	if err := os.WriteFile(
		leasePath,
		[]byte(
			"1789387227 44:a9:2c:50:80:76 192.168.1.193 Johns-Mac-mini 01:44:a9:2c:50:80:76\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		hostsPath,
		[]byte("192.168.1.193 Johns-Mac-mini.woodsnet.us\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	current := observation.New()
	current.ObservedAt = base
	current.Observed.Interface = "br0"
	current.Context.Direction = "ingress"
	current.Observed.Network.SourceIP = "192.168.1.193"
	current.Observed.Network.DestinationIP = "140.82.114.3"
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Transport.DestinationPort = 443
	current.Observed.Ethernet.SourceMAC = "44:a9:2c:50:80:76"

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{current},
	)

	result, err := Connections(ConnectionOptions{
		DHCPHostsFile: hostsPath,
		DHCPLeaseFile: leasePath,
		IdentityMode:  "current-dhcp",
		InputDir:      dir,
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}

	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}

	got := result.Connections[0]
	if got.SourceMAC != "44:a9:2c:50:80:76" {
		t.Fatalf("source MAC = %q", got.SourceMAC)
	}
	if got.CurrentSourceIdentity.Status != "matched" {
		t.Fatalf(
			"identity status = %q, want matched",
			got.CurrentSourceIdentity.Status,
		)
	}
	if got.CurrentSourceIdentity.Hostname != "Johns-Mac-mini" {
		t.Fatalf(
			"hostname = %q",
			got.CurrentSourceIdentity.Hostname,
		)
	}
	if got.CurrentSourceIdentity.FQDN != "Johns-Mac-mini.woodsnet.us" {
		t.Fatalf(
			"fqdn = %q",
			got.CurrentSourceIdentity.FQDN,
		)
	}
}

func TestConnectionsRejectsCurrentDHCPIdentityOnMACMismatch(t *testing.T) {
	dir := t.TempDir()

	leasePath := filepath.Join(dir, "dnsmasq.lease")
	if err := os.WriteFile(
		leasePath,
		[]byte(
			"1789387227 44:a9:2c:50:80:76 192.168.1.193 Johns-Mac-mini 01:44:a9:2c:50:80:76\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	current := observation.New()
	current.ObservedAt = time.Now().UTC()
	current.Observed.Interface = "br0"
	current.Context.Direction = "ingress"
	current.Observed.Network.SourceIP = "192.168.1.193"
	current.Observed.Network.DestinationIP = "203.0.113.5"
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Transport.DestinationPort = 443
	current.Observed.Ethernet.SourceMAC = "00:11:22:33:44:55"

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{current},
	)

	result, err := Connections(ConnectionOptions{
		DHCPHostsFile: "",
		DHCPLeaseFile: leasePath,
		IdentityMode:  "current-dhcp",
		InputDir:      dir,
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}

	if result.Connections[0].CurrentSourceIdentity.Status != "ip_present_mac_mismatch" {
		t.Fatalf(
			"status = %q",
			result.Connections[0].CurrentSourceIdentity.Status,
		)
	}
}

func TestConnectionsUsesHistoricalIdentitySnapshot(t *testing.T) {
	dir := t.TempDir()
	identityDir := filepath.Join(dir, "identity")
	if err := os.MkdirAll(identityDir, 0o750); err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Truncate(time.Second)

	snapshotFile, err := os.Create(
		filepath.Join(identityDir, "endpoint-identity-test.jsonl"),
	)
	if err != nil {
		t.Fatal(err)
	}
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
		ExpectedIntervalSeconds: 60,
		ObservedAt:              base,
		RecordType:              identityhistory.RecordType,
		SchemaVersion:           identityhistory.SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}
	if err := json.NewEncoder(snapshotFile).Encode(snapshot); err != nil {
		snapshotFile.Close()
		t.Fatal(err)
	}
	if err := snapshotFile.Close(); err != nil {
		t.Fatal(err)
	}

	current := observation.New()
	current.ObservedAt = base.Add(20 * time.Second)
	current.Observed.Interface = "br0"
	current.Context.Direction = "ingress"
	current.Observed.Network.SourceIP = "192.168.1.193"
	current.Observed.Network.DestinationIP = "140.82.114.3"
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Transport.DestinationPort = 443
	current.Observed.Ethernet.SourceMAC = "44:a9:2c:50:80:76"

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{current},
	)

	result, err := Connections(ConnectionOptions{
		IdentityMode: "off",
		InputDir:     dir,
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}

	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
	got := result.Connections[0]
	if got.HistoricalIdentityMatchedObservations != 1 {
		t.Fatalf(
			"historical matched observations = %d, want 1",
			got.HistoricalIdentityMatchedObservations,
		)
	}
	if len(got.HistoricalSourceIdentities) != 1 {
		t.Fatalf(
			"historical identities = %d, want 1",
			len(got.HistoricalSourceIdentities),
		)
	}
	if got.HistoricalSourceIdentities[0].Hostname != "Johns-Mac-mini" {
		t.Fatalf(
			"historical hostname = %q",
			got.HistoricalSourceIdentities[0].Hostname,
		)
	}
	if result.Stats.IdentitySnapshotsRead != 1 {
		t.Fatalf(
			"identity snapshots read = %d, want 1",
			result.Stats.IdentitySnapshotsRead,
		)
	}
}

func TestConnectionsDoesNotUseHistoricalIdentityBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	identityDir := filepath.Join(dir, "identity")
	if err := os.MkdirAll(identityDir, 0o750); err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Truncate(time.Second)

	snapshotFile, err := os.Create(
		filepath.Join(identityDir, "endpoint-identity-test.jsonl"),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := identityhistory.Snapshot{
		Endpoints: []identityhistory.Endpoint{
			{
				FQDN:           "future.example",
				Hostname:       "future",
				IP:             "192.168.1.50",
				LeaseExpiresAt: base.Add(time.Hour).Format(time.RFC3339),
				MAC:            "00:11:22:33:44:55",
			},
		},
		ExpectedIntervalSeconds: 60,
		ObservedAt:              base.Add(time.Minute),
		RecordType:              identityhistory.RecordType,
		SchemaVersion:           identityhistory.SchemaVersion,
		Source:                  "dnsmasq_lease",
		SourcePath:              "/run/dnsmasq.lease",
	}
	if err := json.NewEncoder(snapshotFile).Encode(snapshot); err != nil {
		snapshotFile.Close()
		t.Fatal(err)
	}
	if err := snapshotFile.Close(); err != nil {
		t.Fatal(err)
	}

	current := observation.New()
	current.ObservedAt = base
	current.Observed.Interface = "br0"
	current.Context.Direction = "ingress"
	current.Observed.Network.SourceIP = "192.168.1.50"
	current.Observed.Network.DestinationIP = "203.0.113.5"
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Transport.DestinationPort = 443
	current.Observed.Ethernet.SourceMAC = "00:11:22:33:44:55"

	writeObservations(
		t,
		filepath.Join(dir, "observations.jsonl"),
		[]observation.Observation{current},
	)

	result, err := Connections(ConnectionOptions{
		IdentityMode: "off",
		InputDir:     dir,
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}

	if result.Connections[0].HistoricalIdentityMatchedObservations != 0 {
		t.Fatal("future identity snapshot was incorrectly applied backward")
	}
}

func TestConnectionsReadsActiveObservationSegment(t *testing.T) {
	dir := t.TempDir()
	at := time.Now().UTC().Add(-time.Minute)

	current := observation.New()
	current.ObservedAt = at
	current.Observed.Interface = "br0"
	current.Context.Direction = "ingress"
	current.Observed.Ethernet.SourceMAC = "44:a9:2c:50:80:76"
	current.Observed.Network.SourceIP = "192.168.1.193"
	current.Observed.Network.DestinationIP = "203.0.113.7"
	current.Observed.Network.Protocol = "tcp"
	current.Observed.Transport.SourcePort = 50000
	current.Observed.Transport.DestinationPort = 443
	current.Observed.Transport.TCPFlags = []string{"SYN"}

	writeObservations(
		t,
		filepath.Join(dir, "observations-live.jsonl.active"),
		[]observation.Observation{current},
	)

	result, err := Connections(ConnectionOptions{
		InputDir:     dir,
		IdentityMode: "off",
	})
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}

	if len(result.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(result.Connections))
	}
	if result.Stats.ActiveObservationFilesRead != 1 {
		t.Fatalf(
			"active observation files = %d, want 1",
			result.Stats.ActiveObservationFilesRead,
		)
	}
}
