// Package natcorrelation derives conservative cross-interface NAT mappings
// from original packet metadata observations.
package natcorrelation

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/segmentread"
)

const (
	RecordType    = "derived_nat_correlation"
	SchemaVersion = "1"

	defaultMaxDelta = 250 * time.Millisecond
	maxScannerToken = 4 * 1024 * 1024
)

// Options controls NAT correlation.
type Options struct {
	Destination     string
	DestinationPort uint16
	IdentityDir     string
	InputDir        string
	LANInterface    string
	Limit           int
	MaxDelta        time.Duration
	Protocol        string
	Since           time.Duration
	Source          string
	SVI             string
	VLANID          int
	VLANFilterSet   bool
	WANInterface    string
}

// HistoricalIdentitySummary is the historical endpoint identity supporting a
// NAT relationship, when Pathfinder had a qualifying DHCP snapshot.
type HistoricalIdentitySummary struct {
	FQDN               string    `json:"fqdn"`
	Hostname           string    `json:"hostname"`
	SnapshotObservedAt time.Time `json:"snapshot_observed_at"`
	Source             string    `json:"source"`
	Status             string    `json:"status"`
}

// Relationship summarizes conservative cross-interface correlations for one
// exact source-port-preserved NAT mapping.
type Relationship struct {
	CorrelationBasis        string                      `json:"correlation_basis"`
	DestinationIP           string                      `json:"destination_ip"`
	DestinationPort         uint16                      `json:"destination_port"`
	FirstLANObservedAt      time.Time                   `json:"first_lan_observed_at"`
	FirstWANObservedAt      time.Time                   `json:"first_wan_observed_at"`
	HistoricalIdentities    []HistoricalIdentitySummary `json:"historical_identities"`
	LANInterface            string                      `json:"lan_interface"`
	LANSourceIP             string                      `json:"lan_source_ip"`
	LANSourceMAC            string                      `json:"lan_source_mac"`
	LANSourcePort           uint16                      `json:"lan_source_port"`
	LANSVI                  string                      `json:"lan_svi"`
	LANTimestampSource      string                      `json:"lan_timestamp_source"`
	LANVLANID               int                         `json:"lan_vlan_id"`
	LastLANObservedAt       time.Time                   `json:"last_lan_observed_at"`
	LastWANObservedAt       time.Time                   `json:"last_wan_observed_at"`
	MatchedEvents           uint64                      `json:"matched_events"`
	MaxAbsoluteDeltaMicros  int64                       `json:"max_absolute_delta_microseconds"`
	MinAbsoluteDeltaMicros  int64                       `json:"min_absolute_delta_microseconds"`
	Protocol                string                      `json:"protocol"`
	RecordType              string                      `json:"record_type"`
	SchemaVersion           string                      `json:"schema_version"`
	SourceIPTranslated      bool                        `json:"source_ip_translated"`
	SourcePortPreserved     bool                        `json:"source_port_preserved"`
	TCPInitiationCorrelated bool                        `json:"tcp_initiation_correlated"`
	WANInterface            string                      `json:"wan_interface"`
	WANSourceIP             string                      `json:"wan_source_ip"`
	WANSourceMAC            string                      `json:"wan_source_mac"`
	WANSourcePort           uint16                      `json:"wan_source_port"`
	WANTimestampSource      string                      `json:"wan_timestamp_source"`
}

// Stats describes source material and conservative correlation outcomes.
type Stats struct {
	ActiveIdentityFilesRead                   uint64 `json:"active_identity_files_read"`
	ActiveIdentityTrailingFragmentsIgnored    uint64 `json:"active_identity_trailing_fragments_ignored"`
	ActiveObservationFilesRead                uint64 `json:"active_observation_files_read"`
	ActiveObservationTrailingFragmentsIgnored uint64 `json:"active_observation_trailing_fragments_ignored"`
	AmbiguousLANCandidates                    uint64 `json:"ambiguous_lan_candidates"`
	CorrelatedEvents                          uint64 `json:"correlated_events"`
	IdentityFilesRead                         uint64 `json:"identity_files_read"`
	IdentitySnapshotsRead                     uint64 `json:"identity_snapshots_read"`
	LANCandidates                             uint64 `json:"lan_candidates"`
	ObservationFilesRead                      uint64 `json:"observation_files_read"`
	ObservationsBeforeSince                   uint64 `json:"observations_before_since"`
	ObservationsRead                          uint64 `json:"observations_read"`
	RelationshipsFound                        uint64 `json:"relationships_found"`
	RelationshipsReturned                     uint64 `json:"relationships_returned"`
	SegmentsVanishedDuringRead                uint64 `json:"segments_vanished_during_read"`
	UnmatchedLANCandidates                    uint64 `json:"unmatched_lan_candidates"`
	UnmatchedWANCandidates                    uint64 `json:"unmatched_wan_candidates"`
	WANCandidates                             uint64 `json:"wan_candidates"`
}

// Result is the complete derived NAT-correlation query result.
type Result struct {
	Relationships []Relationship `json:"relationships"`
	Stats         Stats          `json:"stats"`
}

type candidate struct {
	destinationIP   string
	destinationPort uint16
	ipLength        uint32
	observedAt      time.Time
	protocol        string
	sourceIP        string
	sourceMAC       string
	sourcePort      uint16
	svi             string
	tcpFlags        string
	timestampSource string
	vlanID          int
	used            bool
}

type relationshipKey struct {
	destinationIP   string
	destinationPort uint16
	lanSourceIP     string
	lanSourceMAC    string
	lanSourcePort   uint16
	protocol        string
	wanSourceIP     string
	wanSourceMAC    string
	wanSourcePort   uint16
}

type matchAddress struct {
	exact  netip.Addr
	prefix netip.Prefix
	set    bool
}

// Correlate derives source-port-preserved NAT mappings from original
// observations.
//
// TCP is intentionally limited to a SYN without ACK observed on both LAN
// ingress and WAN egress. UDP uses the exact packet signature. In either case,
// a match must be unique from both the LAN and WAN side within MaxDelta.
func Correlate(options Options) (Result, error) {
	if options.InputDir == "" {
		return Result{}, fmt.Errorf("input directory is required")
	}
	if options.LANInterface == "" {
		options.LANInterface = "br0"
	}
	if options.WANInterface == "" {
		options.WANInterface = "eth8"
	}
	if options.MaxDelta <= 0 {
		options.MaxDelta = defaultMaxDelta
	}
	if options.Limit < 0 {
		return Result{}, fmt.Errorf("limit must be zero or greater")
	}
	if options.Protocol == "" {
		options.Protocol = "any"
	}
	options.Protocol = strings.ToLower(options.Protocol)
	switch options.Protocol {
	case "any", "tcp", "udp":
	default:
		return Result{}, fmt.Errorf(
			"unsupported protocol %q; use any, tcp, or udp",
			options.Protocol,
		)
	}
	if options.IdentityDir == "" {
		options.IdentityDir = filepath.Join(options.InputDir, "identity")
	}
	if options.Since < 0 {
		return Result{}, fmt.Errorf("since must be zero or greater")
	}
	if options.VLANFilterSet &&
		(options.VLANID < 1 || options.VLANID > 4094) {
		return Result{}, fmt.Errorf("VLAN ID must be between 1 and 4094")
	}

	cutoff := time.Time{}
	if options.Since > 0 {
		cutoff = time.Now().UTC().Add(-options.Since)
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

	identityIndex, identityStats, err := identityhistory.LoadIndex(options.IdentityDir)
	if err != nil {
		return Result{}, fmt.Errorf("load identity history: %w", err)
	}

	lanCandidates := make([]candidate, 0)
	wanCandidates := make([]candidate, 0)

	stats := Stats{
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

				if !cutoff.IsZero() && current.ObservedAt.Before(cutoff) {
					stats.ObservationsBeforeSince++
					return nil
				}

				if current.Observed.Interface == options.LANInterface &&
					current.Context.Direction == "ingress" {
					if options.SVI != "" &&
						current.Context.SVI != options.SVI {
						return nil
					}
					if options.VLANFilterSet &&
						current.Context.VLANID != options.VLANID {
						return nil
					}
					if options.DestinationPort != 0 &&
						current.Observed.Transport.DestinationPort !=
							options.DestinationPort {
						return nil
					}

					if next, ok := candidateFromObservation(
						current,
						options.Protocol,
						sourceMatch,
						destinationMatch,
					); ok {
						lanCandidates = append(lanCandidates, next)
					}
				}

				if current.Observed.Interface == options.WANInterface &&
					current.Context.Direction == "egress" {
					if options.DestinationPort != 0 &&
						current.Observed.Transport.DestinationPort !=
							options.DestinationPort {
						return nil
					}

					// The WAN source may already be translated, so the LAN source
					// filter is intentionally not applied here.
					if next, ok := candidateFromObservation(
						current,
						options.Protocol,
						matchAddress{},
						destinationMatch,
					); ok {
						wanCandidates = append(wanCandidates, next)
					}
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

		stats.ObservationFilesRead++
		if readStats.Active {
			stats.ActiveObservationFilesRead++
		}
		if readStats.TrailingFragmentIgnored {
			stats.ActiveObservationTrailingFragmentsIgnored++
		}
	}

	sort.Slice(lanCandidates, func(i, j int) bool {
		return lanCandidates[i].observedAt.Before(lanCandidates[j].observedAt)
	})
	sort.Slice(wanCandidates, func(i, j int) bool {
		return wanCandidates[i].observedAt.Before(wanCandidates[j].observedAt)
	})

	stats.LANCandidates = uint64(len(lanCandidates))
	stats.WANCandidates = uint64(len(wanCandidates))

	lanBySignature := make(map[string][]int)
	wanBySignature := make(map[string][]int)

	for index := range lanCandidates {
		signature := packetSignature(lanCandidates[index])
		lanBySignature[signature] = append(lanBySignature[signature], index)
	}
	for index := range wanCandidates {
		signature := packetSignature(wanCandidates[index])
		wanBySignature[signature] = append(wanBySignature[signature], index)
	}

	relationships := make(map[relationshipKey]*Relationship)

	for lanIndex := range lanCandidates {
		lan := &lanCandidates[lanIndex]
		if lan.used {
			continue
		}

		signature := packetSignature(*lan)
		wanIndexes := unusedWithinWindow(
			lan.observedAt,
			wanCandidates,
			wanBySignature[signature],
			options.MaxDelta,
		)

		if len(wanIndexes) != 1 {
			if len(wanIndexes) > 1 {
				stats.AmbiguousLANCandidates++
			}
			continue
		}

		wanIndex := wanIndexes[0]
		wan := &wanCandidates[wanIndex]

		// Require the WAN candidate to point back to exactly one unused LAN
		// candidate as well. This prevents a time-close collision involving
		// multiple internal clients from being silently guessed.
		lanIndexes := unusedWithinWindow(
			wan.observedAt,
			lanCandidates,
			lanBySignature[signature],
			options.MaxDelta,
		)
		if len(lanIndexes) != 1 || lanIndexes[0] != lanIndex {
			stats.AmbiguousLANCandidates++
			continue
		}

		// If source IP is unchanged, the observation may represent ordinary
		// routing. Do not label it NAT.
		if lan.sourceIP == wan.sourceIP {
			continue
		}

		lan.used = true
		wan.used = true
		stats.CorrelatedEvents++

		key := relationshipKey{
			destinationIP:   lan.destinationIP,
			destinationPort: lan.destinationPort,
			lanSourceIP:     lan.sourceIP,
			lanSourceMAC:    lan.sourceMAC,
			lanSourcePort:   lan.sourcePort,
			protocol:        lan.protocol,
			wanSourceIP:     wan.sourceIP,
			wanSourceMAC:    wan.sourceMAC,
			wanSourcePort:   wan.sourcePort,
		}

		absoluteDeltaMicros := absDuration(
			wan.observedAt.Sub(lan.observedAt),
		).Microseconds()

		relationship, exists := relationships[key]
		if !exists {
			relationship = &Relationship{
				CorrelationBasis:        basisForProtocol(lan.protocol),
				DestinationIP:           key.destinationIP,
				DestinationPort:         key.destinationPort,
				FirstLANObservedAt:      lan.observedAt,
				FirstWANObservedAt:      wan.observedAt,
				HistoricalIdentities:    []HistoricalIdentitySummary{},
				LANInterface:            options.LANInterface,
				LANSourceIP:             key.lanSourceIP,
				LANSourceMAC:            key.lanSourceMAC,
				LANSourcePort:           key.lanSourcePort,
				LANSVI:                  lan.svi,
				LANTimestampSource:      normalizeTimestampSource(lan.timestampSource),
				LANVLANID:               lan.vlanID,
				LastLANObservedAt:       lan.observedAt,
				LastWANObservedAt:       wan.observedAt,
				MaxAbsoluteDeltaMicros:  absoluteDeltaMicros,
				MinAbsoluteDeltaMicros:  absoluteDeltaMicros,
				Protocol:                key.protocol,
				RecordType:              RecordType,
				SchemaVersion:           SchemaVersion,
				SourceIPTranslated:      true,
				SourcePortPreserved:     key.lanSourcePort == key.wanSourcePort,
				TCPInitiationCorrelated: key.protocol == "tcp",
				WANInterface:            options.WANInterface,
				WANSourceIP:             key.wanSourceIP,
				WANSourceMAC:            key.wanSourceMAC,
				WANSourcePort:           key.wanSourcePort,
				WANTimestampSource:      normalizeTimestampSource(wan.timestampSource),
			}
			relationships[key] = relationship
		}

		relationship.MatchedEvents++

		if lan.observedAt.Before(relationship.FirstLANObservedAt) {
			relationship.FirstLANObservedAt = lan.observedAt
		}
		if wan.observedAt.Before(relationship.FirstWANObservedAt) {
			relationship.FirstWANObservedAt = wan.observedAt
		}
		if lan.observedAt.After(relationship.LastLANObservedAt) {
			relationship.LastLANObservedAt = lan.observedAt
		}
		if wan.observedAt.After(relationship.LastWANObservedAt) {
			relationship.LastWANObservedAt = wan.observedAt
		}
		if absoluteDeltaMicros < relationship.MinAbsoluteDeltaMicros {
			relationship.MinAbsoluteDeltaMicros = absoluteDeltaMicros
		}
		if absoluteDeltaMicros > relationship.MaxAbsoluteDeltaMicros {
			relationship.MaxAbsoluteDeltaMicros = absoluteDeltaMicros
		}

		historical := identityIndex.Lookup(
			lan.sourceIP,
			lan.sourceMAC,
			lan.observedAt,
			identityhistory.LookupOptions{},
		)
		if historical.Status == "matched" {
			appendHistoricalIdentity(
				relationship,
				HistoricalIdentitySummary{
					FQDN:               historical.FQDN,
					Hostname:           historical.Hostname,
					SnapshotObservedAt: historical.SnapshotObservedAt,
					Source:             historical.Source,
					Status:             historical.Status,
				},
			)
		}
	}

	resultRelationships := make([]Relationship, 0, len(relationships))
	for _, relationship := range relationships {
		sort.Slice(
			relationship.HistoricalIdentities,
			func(i, j int) bool {
				return relationship.HistoricalIdentities[i].SnapshotObservedAt.Before(
					relationship.HistoricalIdentities[j].SnapshotObservedAt,
				)
			},
		)
		resultRelationships = append(resultRelationships, *relationship)
	}

	sort.Slice(resultRelationships, func(i, j int) bool {
		if resultRelationships[i].LastLANObservedAt.Equal(
			resultRelationships[j].LastLANObservedAt,
		) {
			if resultRelationships[i].LANSourceIP ==
				resultRelationships[j].LANSourceIP {
				if resultRelationships[i].DestinationIP ==
					resultRelationships[j].DestinationIP {
					return resultRelationships[i].LANSourcePort <
						resultRelationships[j].LANSourcePort
				}
				return resultRelationships[i].DestinationIP <
					resultRelationships[j].DestinationIP
			}
			return resultRelationships[i].LANSourceIP <
				resultRelationships[j].LANSourceIP
		}

		return resultRelationships[i].LastLANObservedAt.After(
			resultRelationships[j].LastLANObservedAt,
		)
	})

	stats.RelationshipsFound = uint64(len(resultRelationships))
	stats.UnmatchedLANCandidates = countUnused(lanCandidates)
	stats.UnmatchedWANCandidates = countUnused(wanCandidates)

	if options.Limit > 0 && len(resultRelationships) > options.Limit {
		resultRelationships = resultRelationships[:options.Limit]
	}
	stats.RelationshipsReturned = uint64(len(resultRelationships))

	return Result{
		Relationships: resultRelationships,
		Stats:         stats,
	}, nil
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func appendHistoricalIdentity(
	relationship *Relationship,
	candidate HistoricalIdentitySummary,
) {
	for _, current := range relationship.HistoricalIdentities {
		if current.SnapshotObservedAt.Equal(candidate.SnapshotObservedAt) &&
			current.Hostname == candidate.Hostname &&
			current.FQDN == candidate.FQDN {
			return
		}
	}

	relationship.HistoricalIdentities = append(
		relationship.HistoricalIdentities,
		candidate,
	)
}

func basisForProtocol(protocol string) string {
	switch protocol {
	case "tcp":
		return "tcp_syn_exact_signature_time_window_v1"
	case "udp":
		return "udp_exact_packet_signature_time_window_v1"
	default:
		return observation.ValueNotKnown
	}
}

func candidateFromObservation(
	current observation.Observation,
	protocolFilter string,
	sourceMatch matchAddress,
	destinationMatch matchAddress,
) (candidate, bool) {
	network := current.Observed.Network
	transport := current.Observed.Transport

	switch network.Protocol {
	case "tcp":
		// TCP NAT is only claimed when initiation is observed on both sides.
		if !hasTCPFlag(transport.TCPFlags, "SYN") ||
			hasTCPFlag(transport.TCPFlags, "ACK") {
			return candidate{}, false
		}
	case "udp":
	default:
		return candidate{}, false
	}

	if protocolFilter != "any" && network.Protocol != protocolFilter {
		return candidate{}, false
	}

	source, err := netip.ParseAddr(network.SourceIP)
	if err != nil || !sourceMatch.matches(source) {
		return candidate{}, false
	}

	destination, err := netip.ParseAddr(network.DestinationIP)
	if err != nil || !destinationMatch.matches(destination) {
		return candidate{}, false
	}

	if transport.SourcePort == 0 || transport.DestinationPort == 0 {
		return candidate{}, false
	}

	return candidate{
		destinationIP:   network.DestinationIP,
		destinationPort: transport.DestinationPort,
		ipLength:        network.Length,
		observedAt:      current.ObservedAt,
		protocol:        network.Protocol,
		sourceIP:        network.SourceIP,
		sourceMAC:       current.Observed.Ethernet.SourceMAC,
		sourcePort:      transport.SourcePort,
		svi:             valueOrNotKnown(current.Context.SVI),
		tcpFlags:        canonicalTCPFlags(transport.TCPFlags),
		timestampSource: normalizeTimestampSource(current.TimestampSource),
		vlanID:          current.Context.VLANID,
	}, true
}

func canonicalTCPFlags(flags []string) string {
	if len(flags) == 0 {
		return ""
	}

	copyFlags := append([]string(nil), flags...)
	sort.Strings(copyFlags)
	return strings.Join(copyFlags, ",")
}

func countUnused(candidates []candidate) uint64 {
	var count uint64
	for _, current := range candidates {
		if !current.used {
			count++
		}
	}
	return count
}

func hasTCPFlag(flags []string, target string) bool {
	for _, flag := range flags {
		if flag == target {
			return true
		}
	}
	return false
}

func valueOrNotKnown(value string) string {
	if value == "" {
		return observation.ValueNotKnown
	}
	return value
}

func normalizeTimestampSource(value string) string {
	if value == "" {
		return observation.ValueNotKnown
	}
	return value
}

func observationFiles(directory string) ([]string, error) {
	return segmentread.List(directory, "")
}

func packetSignature(current candidate) string {
	parts := []string{
		current.protocol,
		strconv.Itoa(int(current.sourcePort)),
		current.destinationIP,
		strconv.Itoa(int(current.destinationPort)),
		strconv.FormatUint(uint64(current.ipLength), 10),
	}

	if current.protocol == "tcp" {
		parts = append(parts, current.tcpFlags)
	}

	return strings.Join(parts, "\x00")
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
		return matchAddress{
			prefix: prefix,
			set:    true,
		}, nil
	}

	address, err := netip.ParseAddr(value)
	if err != nil {
		return matchAddress{}, err
	}

	return matchAddress{
		exact: address,
		set:   true,
	}, nil
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

func unusedWithinWindow(
	at time.Time,
	candidates []candidate,
	indexes []int,
	maxDelta time.Duration,
) []int {
	matches := make([]int, 0, 2)

	for _, index := range indexes {
		if index < 0 || index >= len(candidates) {
			continue
		}

		current := candidates[index]
		if current.used {
			continue
		}
		if absDuration(current.observedAt.Sub(at)) > maxDelta {
			continue
		}

		matches = append(matches, index)
	}

	return matches
}
