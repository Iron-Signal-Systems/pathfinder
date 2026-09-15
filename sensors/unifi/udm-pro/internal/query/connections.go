package query

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/enrichment"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/segmentread"
)

const (
	derivedRecordType = "derived_connection_summary"
	maxScannerToken   = 4 * 1024 * 1024
)

// ConnectionOptions controls the derived connection query.
type ConnectionOptions struct {
	Destination   string
	Direction     string
	DNSMode       string
	DHCPHostsFile string
	DHCPLeaseFile string
	IdentityDir   string
	IdentityMode  string
	InputDir      string
	Interface     string
	Limit         int
	Protocol      string
	Source        string
	Since         time.Duration
}

// DNSContext describes a DNS answer that was valid when a summarized
// observation occurred. It is derived only from DNS responses the sensor
// actually observed.
type DNSContext struct {
	Address    string    `json:"address"`
	AnswerName string    `json:"answer_name"`
	ExpiresAt  time.Time `json:"expires_at"`
	ObservedAt time.Time `json:"observed_at"`
	QueryName  string    `json:"query_name"`
	ResolverIP string    `json:"resolver_ip"`
	TTL        uint32    `json:"ttl"`
}

// ConnectionSummary is a read-only derived view over original observations.
type ConnectionSummary struct {
	CurrentSourceIdentity                 enrichment.CurrentDHCPIdentity       `json:"current_source_identity"`
	DNS                                   []DNSContext                         `json:"dns"`
	DestinationIP                         string                               `json:"destination_ip"`
	DestinationPort                       uint16                               `json:"destination_port"`
	Direction                             string                               `json:"direction"`
	FirstSeen                             time.Time                            `json:"first_seen"`
	HistoricalIdentityMatchedObservations uint64                               `json:"historical_identity_matched_observations"`
	HistoricalSourceIdentities            []identityhistory.HistoricalIdentity `json:"historical_source_identities"`
	Interface                             string                               `json:"interface"`
	LastSeen                              time.Time                            `json:"last_seen"`
	ObservationCount                      uint64                               `json:"observation_count"`
	Protocol                              string                               `json:"protocol"`
	RecordType                            string                               `json:"record_type"`
	SourceIP                              string                               `json:"source_ip"`
	SourceMAC                             string                               `json:"source_mac"`
	TCPInitiationObserved                 bool                                 `json:"tcp_initiation_observed"`
}

// ScanStats describes the source material read for one query.
type ScanStats struct {
	ActiveIdentityFilesRead                   uint64 `json:"active_identity_files_read"`
	ActiveIdentityTrailingFragmentsIgnored    uint64 `json:"active_identity_trailing_fragments_ignored"`
	ActiveObservationFilesRead                uint64 `json:"active_observation_files_read"`
	ActiveObservationTrailingFragmentsIgnored uint64 `json:"active_observation_trailing_fragments_ignored"`
	FilesRead                                 uint64 `json:"files_read"`
	IdentityFilesRead                         uint64 `json:"identity_files_read"`
	IdentitySnapshotsRead                     uint64 `json:"identity_snapshots_read"`
	ObservationsRead                          uint64 `json:"observations_read"`
	RelationshipsFound                        uint64 `json:"relationships_found"`
	RelationshipsReturned                     uint64 `json:"relationships_returned"`
	SegmentsVanishedDuringRead                uint64 `json:"segments_vanished_during_read"`
}

// Result contains connection summaries and scan information.
type Result struct {
	Connections []ConnectionSummary `json:"connections"`
	Stats       ScanStats           `json:"stats"`
}

type connectionKey struct {
	destinationIP   string
	destinationPort uint16
	direction       string
	interfaceName   string
	protocol        string
	sourceIP        string
	sourceMAC       string
}

type dnsBinding struct {
	DNSContext
}

type dnsIndex struct {
	byClientAddress map[string][]dnsBinding
}

type matchAddress struct {
	exact  netip.Addr
	prefix netip.Prefix
	set    bool
}

// Connections scans preserved observation segments and derives directed
// connection summaries. It never modifies observation files.
func Connections(options ConnectionOptions) (Result, error) {
	if options.InputDir == "" {
		return Result{}, fmt.Errorf("input directory is required")
	}
	if options.Interface == "" {
		options.Interface = "br0"
	}
	if options.Direction == "" {
		options.Direction = "ingress"
	}
	if options.DNSMode == "" {
		options.DNSMode = "auto"
	}
	options.DNSMode = strings.ToLower(options.DNSMode)
	if options.DNSMode != "auto" &&
		options.DNSMode != "client" &&
		options.DNSMode != "off" {
		return Result{}, fmt.Errorf(
			"unsupported DNS mode %q; use auto, client, or off",
			options.DNSMode,
		)
	}

	if options.IdentityMode == "" {
		options.IdentityMode = "auto"
	}
	options.IdentityMode = strings.ToLower(options.IdentityMode)
	if options.IdentityMode != "auto" &&
		options.IdentityMode != "current-dhcp" &&
		options.IdentityMode != "off" {
		return Result{}, fmt.Errorf(
			"unsupported identity mode %q; use auto, current-dhcp, or off",
			options.IdentityMode,
		)
	}
	if options.DHCPLeaseFile == "" {
		options.DHCPLeaseFile = "/run/dnsmasq.lease"
	}
	if options.DHCPHostsFile == "" {
		options.DHCPHostsFile = "/run/dnsmasq.dns.conf.d/hosts.d/leases"
	}
	if options.IdentityDir == "" {
		options.IdentityDir = filepath.Join(options.InputDir, "identity")
	}

	if options.Limit < 0 {
		return Result{}, fmt.Errorf("limit must be zero or greater")
	}

	sourceMatch, err := parseAddressMatch(options.Source)
	if err != nil {
		return Result{}, fmt.Errorf("parse source: %w", err)
	}
	destinationMatch, err := parseAddressMatch(options.Destination)
	if err != nil {
		return Result{}, fmt.Errorf("parse destination: %w", err)
	}

	files, err := observationFiles(options.InputDir)
	if err != nil {
		return Result{}, err
	}

	cutoff := time.Time{}
	if options.Since > 0 {
		cutoff = time.Now().UTC().Add(-options.Since)
	}

	dnsMode := resolveDNSMode(options.DNSMode, options.Interface)
	identityMode := resolveIdentityMode(
		options.IdentityMode,
		options.Interface,
		options.Direction,
	)

	var dhcpState *enrichment.DNSMasqState
	if identityMode == "current-dhcp" {
		dhcpState, err = enrichment.ReadDNSMasqState(
			options.DHCPLeaseFile,
			options.DHCPHostsFile,
		)
		if err != nil {
			if options.IdentityMode == "current-dhcp" {
				return Result{}, err
			}
			dhcpState = nil
		}
	}

	identityIndex, identityStats, err := identityhistory.LoadIndex(options.IdentityDir)
	if err != nil {
		return Result{}, fmt.Errorf("load identity history: %w", err)
	}

	dns := newDNSIndex()
	summaries := make(map[connectionKey]*ConnectionSummary)
	stats := ScanStats{
		ActiveIdentityFilesRead:                identityStats.ActiveFilesRead,
		ActiveIdentityTrailingFragmentsIgnored: identityStats.ActiveTrailingFragmentsIgnored,
		IdentityFilesRead:                      identityStats.FilesRead,
		IdentitySnapshotsRead:                  identityStats.SnapshotsRead,
		SegmentsVanishedDuringRead:             identityStats.SegmentsVanishedDuringRead,
	}

	for _, path := range files {
		readStats, readErr := readObservationFile(
			path,
			func(current observation.Observation) error {
				stats.ObservationsRead++

				if dnsMode == "client" && isDNSResponse(current) {
					dns.add(current)
				}

				if !cutoff.IsZero() && current.ObservedAt.Before(cutoff) {
					return nil
				}
				if !matchesObservation(current, options, sourceMatch, destinationMatch) {
					return nil
				}

				key := connectionKey{
					destinationIP:   current.Observed.Network.DestinationIP,
					destinationPort: current.Observed.Transport.DestinationPort,
					direction:       current.Context.Direction,
					interfaceName:   current.Observed.Interface,
					protocol:        current.Observed.Network.Protocol,
					sourceIP:        current.Observed.Network.SourceIP,
					sourceMAC:       current.Observed.Ethernet.SourceMAC,
				}

				summary, exists := summaries[key]
				if !exists {
					identity := disabledIdentity(
						key.sourceIP,
						key.sourceMAC,
						identityMode,
					)
					if identityMode == "current-dhcp" {
						identity = dhcpIdentity(
							dhcpState,
							key.sourceIP,
							key.sourceMAC,
						)
					}

					summary = &ConnectionSummary{
						CurrentSourceIdentity:      identity,
						DNS:                        []DNSContext{},
						DestinationIP:              key.destinationIP,
						DestinationPort:            key.destinationPort,
						Direction:                  key.direction,
						FirstSeen:                  current.ObservedAt,
						HistoricalSourceIdentities: []identityhistory.HistoricalIdentity{},
						Interface:                  key.interfaceName,
						LastSeen:                   current.ObservedAt,
						Protocol:                   key.protocol,
						RecordType:                 derivedRecordType,
						SourceIP:                   key.sourceIP,
						SourceMAC:                  key.sourceMAC,
					}
					summaries[key] = summary
				}

				if current.ObservedAt.Before(summary.FirstSeen) {
					summary.FirstSeen = current.ObservedAt
				}
				if current.ObservedAt.After(summary.LastSeen) {
					summary.LastSeen = current.ObservedAt
				}

				summary.ObservationCount++

				historicalIdentity := identityIndex.Lookup(
					current.Observed.Network.SourceIP,
					current.Observed.Ethernet.SourceMAC,
					current.ObservedAt,
					identityhistory.LookupOptions{},
				)
				if historicalIdentity.Status == "matched" {
					summary.HistoricalIdentityMatchedObservations++
					appendHistoricalIdentity(summary, historicalIdentity)
				}

				if current.Observed.Network.Protocol == "tcp" &&
					hasTCPFlag(current.Observed.Transport.TCPFlags, "SYN") &&
					!hasTCPFlag(current.Observed.Transport.TCPFlags, "ACK") {
					summary.TCPInitiationObserved = true
				}

				for _, dnsContext := range dns.lookup(
					current.Observed.Interface,
					current.Observed.Network.SourceIP,
					current.Observed.Network.DestinationIP,
					current.ObservedAt,
				) {
					appendDNSContext(summary, dnsContext)
				}

				return nil
			},
		)
		if readErr != nil {
			return Result{}, fmt.Errorf("read %s: %w", path, readErr)
		}
		if readStats.Vanished {
			stats.SegmentsVanishedDuringRead++
			continue
		}

		stats.FilesRead++
		if readStats.Active {
			stats.ActiveObservationFilesRead++
		}
		if readStats.TrailingFragmentIgnored {
			stats.ActiveObservationTrailingFragmentsIgnored++
		}
	}

	connections := make([]ConnectionSummary, 0, len(summaries))
	for _, summary := range summaries {
		sort.Slice(summary.DNS, func(i, j int) bool {
			return summary.DNS[i].ObservedAt.Before(summary.DNS[j].ObservedAt)
		})
		sort.Slice(summary.HistoricalSourceIdentities, func(i, j int) bool {
			return summary.HistoricalSourceIdentities[i].SnapshotObservedAt.Before(
				summary.HistoricalSourceIdentities[j].SnapshotObservedAt,
			)
		})
		connections = append(connections, *summary)
	}

	sort.Slice(connections, func(i, j int) bool {
		if connections[i].LastSeen.Equal(connections[j].LastSeen) {
			if connections[i].SourceIP == connections[j].SourceIP {
				if connections[i].DestinationIP == connections[j].DestinationIP {
					return connections[i].DestinationPort < connections[j].DestinationPort
				}
				return connections[i].DestinationIP < connections[j].DestinationIP
			}
			return connections[i].SourceIP < connections[j].SourceIP
		}
		return connections[i].LastSeen.After(connections[j].LastSeen)
	})

	stats.RelationshipsFound = uint64(len(connections))

	if options.Limit > 0 && len(connections) > options.Limit {
		connections = connections[:options.Limit]
	}

	stats.RelationshipsReturned = uint64(len(connections))

	return Result{
		Connections: connections,
		Stats:       stats,
	}, nil
}

func appendHistoricalIdentity(
	summary *ConnectionSummary,
	candidate identityhistory.HistoricalIdentity,
) {
	for _, current := range summary.HistoricalSourceIdentities {
		if current.SnapshotObservedAt.Equal(candidate.SnapshotObservedAt) &&
			current.Hostname == candidate.Hostname &&
			current.FQDN == candidate.FQDN &&
			current.IP == candidate.IP &&
			current.MAC == candidate.MAC {
			return
		}
	}

	summary.HistoricalSourceIdentities = append(
		summary.HistoricalSourceIdentities,
		candidate,
	)
}

func appendDNSContext(summary *ConnectionSummary, candidate DNSContext) {
	for _, current := range summary.DNS {
		if current.Address == candidate.Address &&
			current.AnswerName == candidate.AnswerName &&
			current.ObservedAt.Equal(candidate.ObservedAt) &&
			current.QueryName == candidate.QueryName &&
			current.ResolverIP == candidate.ResolverIP {
			return
		}
	}

	summary.DNS = append(summary.DNS, candidate)
}

func hasTCPFlag(flags []string, target string) bool {
	for _, flag := range flags {
		if flag == target {
			return true
		}
	}
	return false
}

func isDNSResponse(current observation.Observation) bool {
	return current.Observed.DNS.MessageType == "response" &&
		current.Observed.Network.SourceIP != observation.ValueNoRecord &&
		current.Observed.Network.DestinationIP != observation.ValueNoRecord
}

func matchesObservation(
	current observation.Observation,
	options ConnectionOptions,
	sourceMatch matchAddress,
	destinationMatch matchAddress,
) bool {
	network := current.Observed.Network

	if network.SourceIP == "" ||
		network.SourceIP == observation.ValueNoRecord ||
		network.DestinationIP == "" ||
		network.DestinationIP == observation.ValueNoRecord {
		return false
	}

	if options.Interface != "any" && current.Observed.Interface != options.Interface {
		return false
	}
	if options.Direction != "any" && current.Context.Direction != options.Direction {
		return false
	}
	if options.Protocol != "" &&
		options.Protocol != "any" &&
		network.Protocol != strings.ToLower(options.Protocol) {
		return false
	}

	source, err := netip.ParseAddr(network.SourceIP)
	if err != nil {
		return false
	}
	destination, err := netip.ParseAddr(network.DestinationIP)
	if err != nil {
		return false
	}

	if !sourceMatch.matches(source) || !destinationMatch.matches(destination) {
		return false
	}

	return true
}

func newDNSIndex() *dnsIndex {
	return &dnsIndex{
		byClientAddress: make(map[string][]dnsBinding),
	}
}

func (index *dnsIndex) add(current observation.Observation) {
	clientIP := current.Observed.Network.DestinationIP
	resolverIP := current.Observed.Network.SourceIP
	dns := current.Observed.DNS

	for _, answer := range dns.Answers {
		if answer.Type != "A" && answer.Type != "AAAA" {
			continue
		}
		if answer.Value == "" ||
			answer.Value == observation.ValueNoRecord ||
			answer.Value == observation.ValueNotKnown {
			continue
		}
		if _, err := netip.ParseAddr(answer.Value); err != nil {
			continue
		}

		queryName := dns.QueryName
		if queryName == "" || queryName == observation.ValueNoRecord {
			queryName = answer.Name
		}

		context := DNSContext{
			Address:    answer.Value,
			AnswerName: answer.Name,
			ExpiresAt:  current.ObservedAt.Add(time.Duration(answer.TTL) * time.Second),
			ObservedAt: current.ObservedAt,
			QueryName:  queryName,
			ResolverIP: resolverIP,
			TTL:        answer.TTL,
		}

		key := dnsBindingKey(
			current.Observed.Interface,
			clientIP,
			answer.Value,
		)
		existing := index.byClientAddress[key]
		kept := existing[:0]
		for _, prior := range existing {
			if !prior.ExpiresAt.Before(current.ObservedAt) {
				kept = append(kept, prior)
			}
		}

		index.byClientAddress[key] = append(kept, dnsBinding{DNSContext: context})
	}
}

func (index *dnsIndex) lookup(
	interfaceName string,
	clientIP string,
	address string,
	at time.Time,
) []DNSContext {
	key := dnsBindingKey(interfaceName, clientIP, address)
	bindings := index.byClientAddress[key]
	if len(bindings) == 0 {
		return []DNSContext{}
	}

	result := make([]DNSContext, 0, len(bindings))
	kept := bindings[:0]

	for _, binding := range bindings {
		if binding.ExpiresAt.Before(at) {
			continue
		}
		kept = append(kept, binding)

		if binding.ObservedAt.After(at) || binding.TTL == 0 {
			continue
		}

		result = append(result, binding.DNSContext)
	}

	index.byClientAddress[key] = kept
	return result
}

func dnsBindingKey(interfaceName string, clientIP string, address string) string {
	return interfaceName + "\x00" + clientIP + "\x00" + address
}

func dhcpIdentity(
	state *enrichment.DNSMasqState,
	ip string,
	mac string,
) enrichment.CurrentDHCPIdentity {
	if state == nil {
		return enrichment.CurrentDHCPIdentity{
			FQDN:           observation.ValueNoRecord,
			Hostname:       observation.ValueNoRecord,
			IP:             ip,
			LeaseExpiresAt: observation.ValueNoRecord,
			MAC:            mac,
			Source:         observation.ValueNoRecord,
			Status:         "source_unavailable",
		}
	}

	return state.Identity(ip, mac)
}

func disabledIdentity(
	ip string,
	mac string,
	mode string,
) enrichment.CurrentDHCPIdentity {
	status := "disabled"
	if mode == "off" {
		status = "disabled"
	}

	return enrichment.CurrentDHCPIdentity{
		FQDN:           observation.ValueNoRecord,
		Hostname:       observation.ValueNoRecord,
		IP:             ip,
		LeaseExpiresAt: observation.ValueNoRecord,
		MAC:            mac,
		Source:         observation.ValueNoRecord,
		Status:         status,
	}
}

func resolveIdentityMode(mode string, interfaceName string, direction string) string {
	if mode == "current-dhcp" || mode == "off" {
		return mode
	}

	if strings.HasPrefix(interfaceName, "br") && direction == "ingress" {
		return "current-dhcp"
	}

	return "off"
}

func resolveDNSMode(mode string, interfaceName string) string {
	if mode == "client" || mode == "off" {
		return mode
	}

	if strings.HasPrefix(interfaceName, "br") {
		return "client"
	}

	return "off"
}

func observationFiles(directory string) ([]string, error) {
	return segmentread.List(directory, "")
}

func parseAddressMatch(value string) (matchAddress, error) {
	if value == "" || value == "any" {
		return matchAddress{}, nil
	}

	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return matchAddress{}, err
		}
		return matchAddress{prefix: prefix, set: true}, nil
	}

	address, err := netip.ParseAddr(value)
	if err != nil {
		return matchAddress{}, err
	}

	return matchAddress{exact: address, set: true}, nil
}

func (match matchAddress) matches(address netip.Addr) bool {
	if !match.set {
		return true
	}
	if match.prefix.IsValid() {
		return match.prefix.Contains(address)
	}
	return match.exact == address
}

func readObservationFile(
	path string,
	consume func(observation.Observation) error,
) (segmentread.ReadStats, error) {
	return segmentread.ReadLines(
		path,
		maxScannerToken,
		func(line []byte) error {
			var current observation.Observation
			if err := json.Unmarshal(line, &current); err != nil {
				return fmt.Errorf("decode observation: %w", err)
			}

			if current.ObservedAt.IsZero() {
				return nil
			}

			return consume(current)
		},
	)
}
