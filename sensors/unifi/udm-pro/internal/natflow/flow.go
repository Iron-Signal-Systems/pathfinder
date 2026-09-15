// Package natflow promotes a conservatively proven TCP NAT initiation into a
// bounded derived path view over the original observations.
package natflow

import (
	"encoding/json"
	"fmt"
	"net"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natcorrelation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/segmentread"
)

const (
	RecordType    = "derived_nat_flow"
	SchemaVersion = "2"

	defaultMaxFlowAge = time.Hour
	maxScannerToken   = 4 * 1024 * 1024
)

// Options controls derived NAT-flow construction.
type Options struct {
	Destination     string
	DestinationPort uint16
	IdentityDir     string
	InputDir        string
	LANInterface    string
	Limit           int
	MaxDelta        time.Duration
	MaxFlowAge      time.Duration
	Since           time.Duration
	SNI             string
	Source          string
	SVI             string
	VLANID          int
	WANInterface    string
}

// DNSContext describes an observed DNS answer that was valid at the proven
// TCP initiation time for the same LAN endpoint.
type DNSContext struct {
	Address    string    `json:"address"`
	AnswerName string    `json:"answer_name"`
	ExpiresAt  time.Time `json:"expires_at"`
	ObservedAt time.Time `json:"observed_at"`
	QueryName  string    `json:"query_name"`
	ResolverIP string    `json:"resolver_ip"`
	TTL        uint32    `json:"ttl"`
}

type TLSContext struct {
	Interface  string    `json:"interface"`
	ObservedAt time.Time `json:"observed_at"`
	SNI        string    `json:"sni"`
}

// LegStats reports observations at one authoritative vantage point. Counts and
// byte totals are intentionally not deduplicated across interfaces.
type LegStats struct {
	FirstSeen                    string `json:"first_seen"`
	LastSeen                     string `json:"last_seen"`
	Observations                 uint64 `json:"observations"`
	ObservedIPBytes              uint64 `json:"observed_ip_bytes"`
	OffloadSuspectedObservations uint64 `json:"offload_suspected_observations"`
}

// Flow is a bounded derived path anchored by exactly one conservatively
// correlated TCP SYN mapping.
type Flow struct {
	AnchorAbsoluteDeltaMicros int64                                      `json:"anchor_absolute_delta_microseconds"`
	AnchorLANObservedAt       time.Time                                  `json:"anchor_lan_observed_at"`
	AnchorLANTimestampSource  string                                     `json:"anchor_lan_timestamp_source"`
	AnchorWANObservedAt       time.Time                                  `json:"anchor_wan_observed_at"`
	AnchorWANTimestampSource  string                                     `json:"anchor_wan_timestamp_source"`
	ClosedAt                  string                                     `json:"closed_at"`
	CorrelationBasis          string                                     `json:"correlation_basis"`
	DNSAtInitiation           []DNSContext                               `json:"dns_at_initiation"`
	DestinationIP             string                                     `json:"destination_ip"`
	DestinationPort           uint16                                     `json:"destination_port"`
	HistoricalIdentities      []natcorrelation.HistoricalIdentitySummary `json:"historical_identities"`
	LANInterface              string                                     `json:"lan_interface"`
	LANOutbound               LegStats                                   `json:"lan_outbound"`
	LANReturn                 LegStats                                   `json:"lan_return"`
	LANSourceIP               string                                     `json:"lan_source_ip"`
	LANSourceMAC              string                                     `json:"lan_source_mac"`
	LANSourcePort             uint16                                     `json:"lan_source_port"`
	LANSVI                    string                                     `json:"lan_svi"`
	LANVLANID                 int                                        `json:"lan_vlan_id"`
	LastSeen                  time.Time                                  `json:"last_seen"`
	MaxFlowAgeSeconds         int64                                      `json:"max_flow_age_seconds"`
	PathStatus                string                                     `json:"path_status"`
	Protocol                  string                                     `json:"protocol"`
	RecordType                string                                     `json:"record_type"`
	ReturnPathObserved        bool                                       `json:"return_path_observed"`
	SchemaVersion             string                                     `json:"schema_version"`
	SourceIPTranslated        bool                                       `json:"source_ip_translated"`
	SourcePortPreserved       bool                                       `json:"source_port_preserved"`
	TCPFINOutboundObserved    bool                                       `json:"tcp_fin_outbound_observed"`
	TCPFINReturnObserved      bool                                       `json:"tcp_fin_return_observed"`
	TCPInitiationCorrelated   bool                                       `json:"tcp_initiation_correlated"`
	TCPRSTObserved            bool                                       `json:"tcp_rst_observed"`
	TLSAtInitiation           []TLSContext                               `json:"tls_at_initiation"`
	WANInterface              string                                     `json:"wan_interface"`
	WANOutbound               LegStats                                   `json:"wan_outbound"`
	WANReturn                 LegStats                                   `json:"wan_return"`
	WANSourceIP               string                                     `json:"wan_source_ip"`
	WANSourceMAC              string                                     `json:"wan_source_mac"`
	WANSourcePort             uint16                                     `json:"wan_source_port"`
}

// Stats describes how the path view was built.
type Stats struct {
	ActiveIdentityFilesRead                   uint64 `json:"active_identity_files_read"`
	ActiveIdentityTrailingFragmentsIgnored    uint64 `json:"active_identity_trailing_fragments_ignored"`
	ActiveObservationFilesRead                uint64 `json:"active_observation_files_read"`
	ActiveObservationTrailingFragmentsIgnored uint64 `json:"active_observation_trailing_fragments_ignored"`
	AmbiguousFlowObservations                 uint64 `json:"ambiguous_flow_observations"`
	AnchorsAccepted                           uint64 `json:"anchors_accepted"`
	AnchorsRefusedMultipleInitiations         uint64 `json:"anchors_refused_multiple_initiations"`
	DNSBindingsObserved                       uint64 `json:"dns_bindings_observed"`
	FlowObservationsAssigned                  uint64 `json:"flow_observations_assigned"`
	FlowsFilteredBySNI                        uint64 `json:"flows_filtered_by_sni"`
	FlowsFound                                uint64 `json:"flows_found"`
	FlowsRefusedAdditionalInitiation          uint64 `json:"flows_refused_additional_initiation"`
	FlowsReturned                             uint64 `json:"flows_returned"`
	NATRelationshipsFound                     uint64 `json:"nat_relationships_found"`
	ObservationFilesRead                      uint64 `json:"observation_files_read"`
	ObservationsBeforeSince                   uint64 `json:"observations_before_since"`
	ObservationsRead                          uint64 `json:"observations_read"`
	SegmentsVanishedDuringRead                uint64 `json:"segments_vanished_during_read"`
	TLSClientHellosObserved                   uint64 `json:"tls_client_hellos_observed"`
	TLSSNINamesObserved                       uint64 `json:"tls_sni_names_observed"`
}

// Result contains derived NAT flows and construction statistics.
type Result struct {
	Flows []Flow `json:"flows"`
	Stats Stats  `json:"stats"`
}

type dnsBinding struct {
	DNSContext
	clientIP  string
	clientMAC string
}

type flowState struct {
	Flow
	additionalInitiation bool
	hardEnd              time.Time
	closedAt             time.Time
}

type leg string

const (
	legLANOutbound leg = "lan_outbound"
	legLANReturn   leg = "lan_return"
	legWANOutbound leg = "wan_outbound"
	legWANReturn   leg = "wan_return"
)

// Build promotes proven NAT initiations into bounded derived TCP paths.
func Build(options Options) (Result, error) {
	if options.InputDir == "" {
		return Result{}, fmt.Errorf("input directory is required")
	}
	if options.LANInterface == "" {
		options.LANInterface = "br0"
	}
	if options.WANInterface == "" {
		options.WANInterface = "eth8"
	}
	if options.MaxFlowAge <= 0 {
		options.MaxFlowAge = defaultMaxFlowAge
	}
	if options.Limit < 0 {
		return Result{}, fmt.Errorf("limit must be zero or greater")
	}
	if options.IdentityDir == "" {
		options.IdentityDir = filepath.Join(options.InputDir, "identity")
	}
	if options.Since < 0 {
		return Result{}, fmt.Errorf("since must be zero or greater")
	}
	if err := validateSNIPattern(options.SNI); err != nil {
		return Result{}, err
	}
	if options.VLANID != -1 &&
		(options.VLANID < 1 || options.VLANID > 4094) {
		return Result{}, fmt.Errorf(
			"VLAN ID must be -1 or between 1 and 4094",
		)
	}

	cutoff := time.Time{}
	if options.Since > 0 {
		cutoff = time.Now().UTC().Add(-options.Since)
	}

	natResult, err := natcorrelation.Correlate(natcorrelation.Options{
		Destination:     options.Destination,
		DestinationPort: options.DestinationPort,
		IdentityDir:     options.IdentityDir,
		InputDir:        options.InputDir,
		LANInterface:    options.LANInterface,
		Limit:           0,
		MaxDelta:        options.MaxDelta,
		Protocol:        "tcp",
		Since:           options.Since,
		Source:          options.Source,
		SVI:             options.SVI,
		VLANID:          options.VLANID,
		VLANFilterSet:   options.VLANID != -1,
		WANInterface:    options.WANInterface,
	})
	if err != nil {
		return Result{}, err
	}

	stats := Stats{
		ActiveIdentityFilesRead:                natResult.Stats.ActiveIdentityFilesRead,
		ActiveIdentityTrailingFragmentsIgnored: natResult.Stats.ActiveIdentityTrailingFragmentsIgnored,
		NATRelationshipsFound:                  natResult.Stats.RelationshipsFound,
		SegmentsVanishedDuringRead:             natResult.Stats.SegmentsVanishedDuringRead,
	}

	states := make([]flowState, 0, len(natResult.Relationships))
	for _, relationship := range natResult.Relationships {
		if relationship.Protocol != "tcp" ||
			!relationship.TCPInitiationCorrelated {
			continue
		}
		if options.DestinationPort != 0 &&
			relationship.DestinationPort != options.DestinationPort {
			continue
		}
		if options.SVI != "" &&
			relationship.LANSVI != options.SVI {
			continue
		}
		if options.VLANID >= 0 &&
			relationship.LANVLANID != options.VLANID {
			continue
		}

		// v1 refuses to promote a tuple when more than one initiation was
		// correlated. That can be retransmission or later source-port reuse,
		// and neither should be silently merged into one flow.
		if relationship.MatchedEvents != 1 {
			stats.AnchorsRefusedMultipleInitiations++
			continue
		}

		start := relationship.FirstLANObservedAt
		if relationship.FirstWANObservedAt.Before(start) {
			start = relationship.FirstWANObservedAt
		}

		state := flowState{
			Flow: Flow{
				AnchorAbsoluteDeltaMicros: absoluteDuration(
					relationship.FirstWANObservedAt.Sub(
						relationship.FirstLANObservedAt,
					),
				).Microseconds(),
				AnchorLANObservedAt:      relationship.FirstLANObservedAt,
				AnchorLANTimestampSource: relationship.LANTimestampSource,
				AnchorWANObservedAt:      relationship.FirstWANObservedAt,
				AnchorWANTimestampSource: relationship.WANTimestampSource,
				ClosedAt:                 observation.ValueNoRecord,
				CorrelationBasis:         relationship.CorrelationBasis,
				DNSAtInitiation:          []DNSContext{},
				DestinationIP:            relationship.DestinationIP,
				DestinationPort:          relationship.DestinationPort,
				HistoricalIdentities:     relationship.HistoricalIdentities,
				LANInterface:             relationship.LANInterface,
				LANOutbound:              emptyLegStats(),
				LANReturn:                emptyLegStats(),
				LANSourceIP:              relationship.LANSourceIP,
				LANSourceMAC:             normalizeMAC(relationship.LANSourceMAC),
				LANSourcePort:            relationship.LANSourcePort,
				LANSVI:                   relationship.LANSVI,
				LANVLANID:                relationship.LANVLANID,
				LastSeen:                 start,
				MaxFlowAgeSeconds:        int64(options.MaxFlowAge / time.Second),
				PathStatus:               "anchored_no_return_observed",
				Protocol:                 relationship.Protocol,
				RecordType:               RecordType,
				SchemaVersion:            SchemaVersion,
				SourceIPTranslated:       relationship.SourceIPTranslated,
				SourcePortPreserved:      relationship.SourcePortPreserved,
				TCPInitiationCorrelated:  true,
				TLSAtInitiation:          []TLSContext{},
				WANInterface:             relationship.WANInterface,
				WANOutbound:              emptyLegStats(),
				WANReturn:                emptyLegStats(),
				WANSourceIP:              relationship.WANSourceIP,
				WANSourceMAC:             normalizeMAC(relationship.WANSourceMAC),
				WANSourcePort:            relationship.WANSourcePort,
			},
			hardEnd: start.Add(options.MaxFlowAge),
		}

		states = append(states, state)
		stats.AnchorsAccepted++
	}

	files, err := observationFiles(options.InputDir)
	if err != nil {
		return Result{}, err
	}

	dnsBindings := make([]dnsBinding, 0)

	for _, path := range files {
		readStats, readErr := readObservationFile(
			path,
			func(current observation.Observation) error {
				stats.ObservationsRead++

				if binding, ok := dnsBindingFromObservation(
					current,
					options.LANInterface,
				); ok {
					dnsBindings = append(dnsBindings, binding...)
					stats.DNSBindingsObserved += uint64(len(binding))
				}

				if !cutoff.IsZero() && current.ObservedAt.Before(cutoff) {
					stats.ObservationsBeforeSince++
					return nil
				}

				matches := make([]struct {
					index int
					leg   leg
				}, 0, 2)

				for index := range states {
					state := &states[index]

					if current.ObservedAt.Before(state.anchorStart()) ||
						current.ObservedAt.After(state.hardEnd) {
						continue
					}
					if !state.closedAt.IsZero() &&
						current.ObservedAt.After(state.closedAt) {
						continue
					}

					currentLeg, ok := observationLeg(current, &state.Flow)
					if !ok {
						continue
					}

					matches = append(matches, struct {
						index int
						leg   leg
					}{
						index: index,
						leg:   currentLeg,
					})
				}

				if len(matches) == 0 {
					return nil
				}
				if len(matches) > 1 {
					stats.AmbiguousFlowObservations++
					return nil
				}

				match := matches[0]
				state := &states[match.index]
				state.addObservation(
					current,
					match.leg,
					&stats,
				)
				stats.FlowObservationsAssigned++

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

	sort.Slice(dnsBindings, func(i, j int) bool {
		return dnsBindings[i].ObservedAt.Before(dnsBindings[j].ObservedAt)
	})

	flows := make([]Flow, 0, len(states))
	for index := range states {
		state := &states[index]

		if state.additionalInitiation {
			stats.FlowsRefusedAdditionalInitiation++
			continue
		}

		state.DNSAtInitiation = dnsAtInitiation(
			dnsBindings,
			state.LANSourceIP,
			state.LANSourceMAC,
			state.DestinationIP,
			state.AnchorLANObservedAt,
		)

		state.ReturnPathObserved =
			state.WANReturn.Observations > 0 ||
				state.LANReturn.Observations > 0

		state.PathStatus = pathStatus(state)
		if !state.closedAt.IsZero() {
			state.ClosedAt = state.closedAt.UTC().Format(time.RFC3339Nano)
		}

		matchesSNI, matchErr := flowMatchesSNI(
			options.SNI,
			state.TLSAtInitiation,
		)
		if matchErr != nil {
			return Result{}, matchErr
		}
		if !matchesSNI {
			stats.FlowsFilteredBySNI++
			continue
		}

		flows = append(flows, state.Flow)
	}

	sort.Slice(flows, func(i, j int) bool {
		if flows[i].LastSeen.Equal(flows[j].LastSeen) {
			if flows[i].LANSourceIP == flows[j].LANSourceIP {
				if flows[i].DestinationIP == flows[j].DestinationIP {
					return flows[i].LANSourcePort < flows[j].LANSourcePort
				}
				return flows[i].DestinationIP < flows[j].DestinationIP
			}
			return flows[i].LANSourceIP < flows[j].LANSourceIP
		}
		return flows[i].LastSeen.After(flows[j].LastSeen)
	})

	stats.FlowsFound = uint64(len(flows))
	if options.Limit > 0 && len(flows) > options.Limit {
		flows = flows[:options.Limit]
	}
	stats.FlowsReturned = uint64(len(flows))

	return Result{
		Flows: flows,
		Stats: stats,
	}, nil
}

func (state *flowState) addObservation(
	current observation.Observation,
	currentLeg leg,
	stats *Stats,
) {
	switch currentLeg {
	case legLANOutbound:
		addLegObservation(&state.LANOutbound, current)
	case legLANReturn:
		addLegObservation(&state.LANReturn, current)
	case legWANOutbound:
		addLegObservation(&state.WANOutbound, current)
	case legWANReturn:
		addLegObservation(&state.WANReturn, current)
	}

	if currentLeg == legLANOutbound &&
		current.Observed.TLS.ClientHelloObserved {
		stats.TLSClientHellosObserved++

		if current.Observed.TLS.SNI != "" &&
			current.Observed.TLS.SNI != observation.ValueNoRecord &&
			current.Observed.TLS.SNI != observation.ValueNotKnown {
			stats.TLSSNINamesObserved++
			state.addTLSContext(TLSContext{
				Interface:  current.Observed.Interface,
				ObservedAt: current.ObservedAt,
				SNI:        current.Observed.TLS.SNI,
			})
		}
	}

	if current.ObservedAt.After(state.LastSeen) {
		state.LastSeen = current.ObservedAt
	}

	flags := current.Observed.Transport.TCPFlags

	if currentLeg == legLANOutbound &&
		hasTCPFlag(flags, "SYN") &&
		!hasTCPFlag(flags, "ACK") &&
		!current.ObservedAt.Equal(state.AnchorLANObservedAt) {
		state.additionalInitiation = true
	}

	if hasTCPFlag(flags, "RST") {
		state.TCPRSTObserved = true
		if state.closedAt.IsZero() ||
			current.ObservedAt.Before(state.closedAt) {
			state.closedAt = current.ObservedAt
		}
	}

	if hasTCPFlag(flags, "FIN") {
		switch currentLeg {
		case legLANOutbound, legWANOutbound:
			state.TCPFINOutboundObserved = true
		case legLANReturn, legWANReturn:
			state.TCPFINReturnObserved = true
		}
	}
}

func flowMatchesSNI(
	pattern string,
	contexts []TLSContext,
) (bool, error) {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return true, nil
	}

	for _, current := range contexts {
		name := strings.ToLower(strings.TrimSpace(current.SNI))
		if name == "" ||
			name == observation.ValueNoRecord ||
			name == observation.ValueNotKnown {
			continue
		}

		matched, err := pathpkg.Match(pattern, name)
		if err != nil {
			return false, fmt.Errorf(
				"invalid SNI pattern %q: %w",
				pattern,
				err,
			)
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

func validateSNIPattern(pattern string) error {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return nil
	}

	if _, err := pathpkg.Match(pattern, "validation.example"); err != nil {
		return fmt.Errorf(
			"invalid SNI pattern %q: %w",
			pattern,
			err,
		)
	}

	return nil
}

func (state *flowState) addTLSContext(current TLSContext) {
	for _, existing := range state.TLSAtInitiation {
		if existing.SNI == current.SNI &&
			existing.Interface == current.Interface &&
			existing.ObservedAt.Equal(current.ObservedAt) {
			return
		}
	}

	state.TLSAtInitiation = append(
		state.TLSAtInitiation,
		current,
	)

	sort.Slice(state.TLSAtInitiation, func(i, j int) bool {
		return state.TLSAtInitiation[i].ObservedAt.Before(
			state.TLSAtInitiation[j].ObservedAt,
		)
	})
}

func (state *flowState) anchorStart() time.Time {
	start := state.AnchorLANObservedAt
	if state.AnchorWANObservedAt.Before(start) {
		start = state.AnchorWANObservedAt
	}
	return start
}

func absoluteDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func addLegObservation(stats *LegStats, current observation.Observation) {
	if stats.Observations == 0 {
		stats.FirstSeen = current.ObservedAt.UTC().Format(time.RFC3339Nano)
	}
	stats.LastSeen = current.ObservedAt.UTC().Format(time.RFC3339Nano)
	stats.Observations++
	stats.ObservedIPBytes += uint64(current.Observed.Network.Length)

	if current.Context.KernelOffloadSuspected {
		stats.OffloadSuspectedObservations++
	}
}

func dnsAtInitiation(
	bindings []dnsBinding,
	clientIP string,
	clientMAC string,
	address string,
	at time.Time,
) []DNSContext {
	result := make([]DNSContext, 0)

	for _, binding := range bindings {
		if binding.clientIP != clientIP ||
			normalizeMAC(binding.clientMAC) != normalizeMAC(clientMAC) ||
			binding.Address != address {
			continue
		}
		if binding.ObservedAt.After(at) {
			continue
		}
		if binding.TTL == 0 || binding.ExpiresAt.Before(at) {
			continue
		}

		duplicate := false
		for _, existing := range result {
			if existing.QueryName == binding.QueryName &&
				existing.AnswerName == binding.AnswerName &&
				existing.ObservedAt.Equal(binding.ObservedAt) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		result = append(result, binding.DNSContext)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ObservedAt.Before(result[j].ObservedAt)
	})

	return result
}

func dnsBindingFromObservation(
	current observation.Observation,
	lanInterface string,
) ([]dnsBinding, bool) {
	if current.Observed.Interface != lanInterface ||
		current.Context.Direction != "egress" ||
		current.Observed.DNS.MessageType != "response" {
		return nil, false
	}

	clientIP := current.Observed.Network.DestinationIP
	clientMAC := normalizeMAC(current.Observed.Ethernet.DestinationMAC)
	resolverIP := current.Observed.Network.SourceIP

	if net.ParseIP(clientIP) == nil ||
		clientMAC == "" ||
		clientMAC == observation.ValueNoRecord ||
		clientMAC == observation.ValueNotKnown {
		return nil, false
	}

	result := make([]dnsBinding, 0)
	for _, answer := range current.Observed.DNS.Answers {
		if answer.Type != "A" && answer.Type != "AAAA" {
			continue
		}
		if net.ParseIP(answer.Value) == nil {
			continue
		}

		queryName := current.Observed.DNS.QueryName
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

		result = append(result, dnsBinding{
			DNSContext: context,
			clientIP:   clientIP,
			clientMAC:  clientMAC,
		})
	}

	return result, len(result) > 0
}

func emptyLegStats() LegStats {
	return LegStats{
		FirstSeen: observation.ValueNoRecord,
		LastSeen:  observation.ValueNoRecord,
	}
}

func hasTCPFlag(flags []string, target string) bool {
	for _, flag := range flags {
		if flag == target {
			return true
		}
	}
	return false
}

func macMatches(observed string, expected string) bool {
	return normalizeMAC(observed) == normalizeMAC(expected)
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

func observationFiles(directory string) ([]string, error) {
	return segmentread.List(directory, "")
}

func observationLeg(
	current observation.Observation,
	flow *Flow,
) (leg, bool) {
	network := current.Observed.Network
	transport := current.Observed.Transport

	if network.Protocol != "tcp" {
		return "", false
	}

	switch {
	case current.Observed.Interface == flow.LANInterface &&
		current.Context.Direction == "ingress" &&
		network.SourceIP == flow.LANSourceIP &&
		network.DestinationIP == flow.DestinationIP &&
		transport.SourcePort == flow.LANSourcePort &&
		transport.DestinationPort == flow.DestinationPort &&
		macMatches(current.Observed.Ethernet.SourceMAC, flow.LANSourceMAC):
		return legLANOutbound, true

	case current.Observed.Interface == flow.WANInterface &&
		current.Context.Direction == "egress" &&
		network.SourceIP == flow.WANSourceIP &&
		network.DestinationIP == flow.DestinationIP &&
		transport.SourcePort == flow.WANSourcePort &&
		transport.DestinationPort == flow.DestinationPort &&
		macMatches(current.Observed.Ethernet.SourceMAC, flow.WANSourceMAC):
		return legWANOutbound, true

	case current.Observed.Interface == flow.WANInterface &&
		current.Context.Direction == "ingress" &&
		network.SourceIP == flow.DestinationIP &&
		network.DestinationIP == flow.WANSourceIP &&
		transport.SourcePort == flow.DestinationPort &&
		transport.DestinationPort == flow.WANSourcePort &&
		macMatches(current.Observed.Ethernet.DestinationMAC, flow.WANSourceMAC):
		return legWANReturn, true

	case current.Observed.Interface == flow.LANInterface &&
		current.Context.Direction == "egress" &&
		network.SourceIP == flow.DestinationIP &&
		network.DestinationIP == flow.LANSourceIP &&
		transport.SourcePort == flow.DestinationPort &&
		transport.DestinationPort == flow.LANSourcePort &&
		macMatches(current.Observed.Ethernet.DestinationMAC, flow.LANSourceMAC):
		return legLANReturn, true
	}

	return "", false
}

func pathStatus(state *flowState) string {
	switch {
	case state.TCPRSTObserved:
		return "anchored_rst_observed"
	case state.TCPFINOutboundObserved && state.TCPFINReturnObserved:
		return "anchored_fin_both_directions_observed"
	case state.ReturnPathObserved:
		return "anchored_return_observed"
	default:
		return "anchored_no_return_observed"
	}
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
