package enrichment

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

// CurrentDHCPIdentity describes current dnsmasq state. It is enrichment and
// must not be interpreted as proving that a hostname was valid historically.
type CurrentDHCPIdentity struct {
	FQDN           string `json:"fqdn"`
	Hostname       string `json:"hostname"`
	IP             string `json:"ip"`
	LeaseExpiresAt string `json:"lease_expires_at"`
	MAC            string `json:"mac"`
	Source         string `json:"source"`
	Status         string `json:"status"`
}

// LeaseIdentity is one current dnsmasq lease identity suitable for a
// point-in-time snapshot.
type LeaseIdentity struct {
	FQDN           string `json:"fqdn"`
	Hostname       string `json:"hostname"`
	IP             string `json:"ip"`
	LeaseExpiresAt string `json:"lease_expires_at"`
	MAC            string `json:"mac"`
}

// DNSMasqState is a point-in-time view of current dnsmasq lease/name state.
type DNSMasqState struct {
	hostsByIP map[string]string
	leases    map[string]dnsmasqLease
	readAt    time.Time
}

type dnsmasqLease struct {
	clientID  string
	expiresAt time.Time
	hostname  string
	ip        string
	mac       string
}

// ReadDNSMasqState loads current lease and generated-host data.
func ReadDNSMasqState(leasePath string, hostsPath string) (*DNSMasqState, error) {
	state := &DNSMasqState{
		hostsByIP: make(map[string]string),
		leases:    make(map[string]dnsmasqLease),
		readAt:    time.Now().UTC(),
	}

	if err := state.readLeases(leasePath); err != nil {
		return nil, fmt.Errorf("read dnsmasq leases: %w", err)
	}

	if hostsPath != "" {
		if err := state.readHosts(hostsPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read dnsmasq generated hosts: %w", err)
		}
	}

	return state, nil
}

// Identity resolves current DHCP identity only when IP and observed MAC match.
func (state *DNSMasqState) Identity(ip string, observedMAC string) CurrentDHCPIdentity {
	result := CurrentDHCPIdentity{
		FQDN:           observation.ValueNoRecord,
		Hostname:       observation.ValueNoRecord,
		IP:             ip,
		LeaseExpiresAt: observation.ValueNoRecord,
		MAC:            normalizeMAC(observedMAC),
		Source:         "dnsmasq_current_lease",
		Status:         "no_matching_lease",
	}

	if state == nil {
		result.Source = observation.ValueNoRecord
		result.Status = "source_unavailable"
		return result
	}

	lease, exists := state.leases[ip]
	if !exists {
		return result
	}

	if normalizeMAC(lease.mac) != normalizeMAC(observedMAC) {
		result.Status = "ip_present_mac_mismatch"
		return result
	}

	result.Status = "matched"
	result.MAC = lease.mac
	if !lease.expiresAt.IsZero() {
		result.LeaseExpiresAt = lease.expiresAt.UTC().Format(time.RFC3339)
	}

	if lease.hostname != "" && lease.hostname != "*" {
		result.Hostname = lease.hostname
	}
	if fqdn, exists := state.hostsByIP[ip]; exists && fqdn != "" {
		result.FQDN = fqdn
	}

	return result
}

// LeaseIdentities returns all current dnsmasq leases in deterministic order.
func (state *DNSMasqState) LeaseIdentities() []LeaseIdentity {
	if state == nil {
		return []LeaseIdentity{}
	}

	identities := make([]LeaseIdentity, 0, len(state.leases))
	for _, lease := range state.leases {
		hostname := observation.ValueNoRecord
		if lease.hostname != "" && lease.hostname != "*" {
			hostname = lease.hostname
		}

		fqdn := observation.ValueNoRecord
		if value, exists := state.hostsByIP[lease.ip]; exists && value != "" {
			fqdn = value
		}

		leaseExpiresAt := observation.ValueNoRecord
		if !lease.expiresAt.IsZero() {
			leaseExpiresAt = lease.expiresAt.UTC().Format(time.RFC3339)
		}

		identities = append(identities, LeaseIdentity{
			FQDN:           fqdn,
			Hostname:       hostname,
			IP:             lease.ip,
			LeaseExpiresAt: leaseExpiresAt,
			MAC:            lease.mac,
		})
	}

	sort.Slice(identities, func(i, j int) bool {
		if identities[i].IP == identities[j].IP {
			return identities[i].MAC < identities[j].MAC
		}
		return identities[i].IP < identities[j].IP
	})

	return identities
}

// ReadAt is when the current dnsmasq files were loaded.
func (state *DNSMasqState) ReadAt() time.Time {
	if state == nil {
		return time.Time{}
	}
	return state.readAt
}

func dnsmasqLeaseKey(ip string) string {
	return strings.TrimSpace(ip)
}

func normalizeMAC(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" ||
		value == observation.ValueNoRecord ||
		value == observation.ValueNotKnown {
		return value
	}

	parsed, err := net.ParseMAC(value)
	if err != nil {
		return value
	}

	return strings.ToLower(parsed.String())
}

func (state *DNSMasqState) readHosts(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		ip := strings.TrimSpace(fields[0])
		fqdn := strings.TrimSpace(fields[1])
		if net.ParseIP(ip) == nil || fqdn == "" {
			continue
		}

		state.hostsByIP[ip] = fqdn
	}

	return scanner.Err()
}

func (state *DNSMasqState) readLeases(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}

		expiryUnix, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return fmt.Errorf("line %d: invalid expiry %q", lineNumber, fields[0])
		}

		mac := normalizeMAC(fields[1])
		ip := strings.TrimSpace(fields[2])
		hostname := strings.TrimSpace(fields[3])
		clientID := strings.TrimSpace(fields[4])

		if net.ParseIP(ip) == nil {
			continue
		}
		if _, err := net.ParseMAC(mac); err != nil {
			continue
		}

		expiresAt := time.Time{}
		if expiryUnix > 0 {
			expiresAt = time.Unix(expiryUnix, 0).UTC()
		}

		state.leases[dnsmasqLeaseKey(ip)] = dnsmasqLease{
			clientID:  clientID,
			expiresAt: expiresAt,
			hostname:  hostname,
			ip:        ip,
			mac:       mac,
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
