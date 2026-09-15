package enrichment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

func TestDNSMasqIdentityRequiresIPAndMACMatch(t *testing.T) {
	dir := t.TempDir()

	leasePath := filepath.Join(dir, "dnsmasq.lease")
	hostsPath := filepath.Join(dir, "leases")

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

	state, err := ReadDNSMasqState(leasePath, hostsPath)
	if err != nil {
		t.Fatalf("ReadDNSMasqState() error = %v", err)
	}

	matched := state.Identity("192.168.1.193", "44:A9:2C:50:80:76")
	if matched.Status != "matched" {
		t.Fatalf("status = %q, want matched", matched.Status)
	}
	if matched.Hostname != "Johns-Mac-mini" {
		t.Fatalf("hostname = %q", matched.Hostname)
	}
	if matched.FQDN != "Johns-Mac-mini.woodsnet.us" {
		t.Fatalf("fqdn = %q", matched.FQDN)
	}

	mismatch := state.Identity("192.168.1.193", "00:11:22:33:44:55")
	if mismatch.Status != "ip_present_mac_mismatch" {
		t.Fatalf(
			"mismatch status = %q, want ip_present_mac_mismatch",
			mismatch.Status,
		)
	}
}

func TestDNSMasqIdentityPreservesNoRecordForUnnamedLease(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "dnsmasq.lease")

	if err := os.WriteFile(
		leasePath,
		[]byte(
			"1789380059 4a:59:db:45:77:24 192.168.1.197 * 01:4a:59:db:45:77:24\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	state, err := ReadDNSMasqState(leasePath, "")
	if err != nil {
		t.Fatalf("ReadDNSMasqState() error = %v", err)
	}

	identity := state.Identity("192.168.1.197", "4a:59:db:45:77:24")
	if identity.Status != "matched" {
		t.Fatalf("status = %q, want matched", identity.Status)
	}
	if identity.Hostname != observation.ValueNoRecord {
		t.Fatalf("hostname = %q, want no_record", identity.Hostname)
	}
	if identity.FQDN != observation.ValueNoRecord {
		t.Fatalf("fqdn = %q, want no_record", identity.FQDN)
	}
}

func TestDNSMasqLeaseIdentitiesAreDeterministicAndNoNullNames(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "dnsmasq.lease")
	hostsPath := filepath.Join(dir, "leases")

	if err := os.WriteFile(
		leasePath,
		[]byte(
			"1789387227 44:a9:2c:50:80:76 192.168.1.193 Johns-Mac-mini 01:44:a9:2c:50:80:76\n"+
				"1789380059 4a:59:db:45:77:24 192.168.1.197 * 01:4a:59:db:45:77:24\n",
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

	state, err := ReadDNSMasqState(leasePath, hostsPath)
	if err != nil {
		t.Fatalf("ReadDNSMasqState() error = %v", err)
	}

	identities := state.LeaseIdentities()
	if len(identities) != 2 {
		t.Fatalf("identities = %d, want 2", len(identities))
	}
	if identities[0].IP != "192.168.1.193" {
		t.Fatalf("first IP = %q", identities[0].IP)
	}
	if identities[1].Hostname != observation.ValueNoRecord {
		t.Fatalf("unnamed hostname = %q, want no_record", identities[1].Hostname)
	}
}

func TestDNSMasqZeroExpiryUsesNoRecord(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "dnsmasq.lease")

	if err := os.WriteFile(
		leasePath,
		[]byte(
			"0 00:11:22:33:44:55 192.168.1.50 static-host 01:00:11:22:33:44:55\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	state, err := ReadDNSMasqState(leasePath, "")
	if err != nil {
		t.Fatalf("ReadDNSMasqState() error = %v", err)
	}

	identity := state.Identity("192.168.1.50", "00:11:22:33:44:55")
	if identity.LeaseExpiresAt != observation.ValueNoRecord {
		t.Fatalf(
			"lease expiry = %q, want no_record",
			identity.LeaseExpiresAt,
		)
	}
}
