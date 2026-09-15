package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natcorrelation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natflow"
)

func natFlows(args []string) error {
	flags := flag.NewFlagSet("nat-flows", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	inputDir := flags.String(
		"input-dir",
		"",
		"directory containing Pathfinder observation segments",
	)
	identityDir := flags.String(
		"identity-dir",
		"",
		"historical endpoint identity directory; default <input-dir>/identity",
	)
	lanInterface := flags.String(
		"lan-interface",
		"br0",
		"LAN-side observation interface",
	)
	wanInterface := flags.String(
		"wan-interface",
		"eth8",
		"WAN-side observation interface",
	)
	maxDeltaText := flags.String(
		"max-delta",
		"250ms",
		"maximum absolute LAN/WAN SYN correlation delta",
	)
	maxFlowAgeText := flags.String(
		"max-flow-age",
		"1h",
		"maximum time to follow an anchored flow",
	)
	sinceText := flags.String(
		"since",
		"0",
		"only consider TCP initiations this recent; example 30s, 2m, 1h, or 0",
	)
	source := flags.String(
		"source",
		"",
		"LAN source IP or CIDR",
	)
	sni := flags.String(
		"sni",
		"",
		"observed TLS SNI exact/glob match; case-insensitive, for example '*.mozilla.*'",
	)
	destination := flags.String(
		"destination",
		"",
		"destination IP or CIDR",
	)
	destinationPort := flags.Int(
		"destination-port",
		0,
		"destination TCP port; 0 means any",
	)
	svi := flags.String(
		"svi",
		"",
		"historical LAN SVI from the anchored initiation; empty means any",
	)
	vlanID := flags.Int(
		"vlan",
		-1,
		"historical LAN VLAN ID from the anchored initiation; omit for any",
	)
	limit := flags.Int(
		"limit",
		50,
		"maximum derived flows to display; 0 means all",
	)
	format := flags.String(
		"format",
		"table",
		"output format: table or json",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputDir == "" {
		return fmt.Errorf("nat-flows requires --input-dir")
	}
	if *limit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}
	if *destinationPort < 0 || *destinationPort > 65535 {
		return fmt.Errorf("--destination-port must be between 0 and 65535")
	}
	if *vlanID != -1 && (*vlanID < 1 || *vlanID > 4094) {
		return fmt.Errorf("--vlan must be between 1 and 4094")
	}

	since, err := parseSince(*sinceText)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}

	maxDelta, err := time.ParseDuration(*maxDeltaText)
	if err != nil {
		return fmt.Errorf("parse --max-delta: %w", err)
	}
	if maxDelta <= 0 {
		return fmt.Errorf("--max-delta must be greater than zero")
	}

	maxFlowAge, err := time.ParseDuration(*maxFlowAgeText)
	if err != nil {
		return fmt.Errorf("parse --max-flow-age: %w", err)
	}
	if maxFlowAge <= 0 {
		return fmt.Errorf("--max-flow-age must be greater than zero")
	}

	result, err := natflow.Build(natflow.Options{
		Destination:     *destination,
		DestinationPort: uint16(*destinationPort),
		IdentityDir:     *identityDir,
		InputDir:        *inputDir,
		LANInterface:    *lanInterface,
		Limit:           *limit,
		MaxDelta:        maxDelta,
		MaxFlowAge:      maxFlowAge,
		Since:           since,
		SNI:             *sni,
		Source:          *source,
		SVI:             *svi,
		VLANID:          *vlanID,
		WANInterface:    *wanInterface,
	})
	if err != nil {
		return err
	}

	switch strings.ToLower(*format) {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case "table":
		return printNATFlowsTable(result)

	default:
		return fmt.Errorf(
			"unsupported --format %q; use table or json",
			*format,
		)
	}
}

func flowDNSNames(contexts []natflow.DNSContext) string {
	if len(contexts) == 0 {
		return "-"
	}

	unique := make(map[string]struct{})
	for _, current := range contexts {
		name := current.QueryName
		if name == "" {
			name = current.AnswerName
		}
		if name == "" {
			continue
		}
		unique[name] = struct{}{}
	}

	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return "-"
	}
	if len(names) > 3 {
		return strings.Join(names[:3], ",") + ",+"
	}
	return strings.Join(names, ",")
}

func flowSNINames(contexts []natflow.TLSContext) string {
	if len(contexts) == 0 {
		return "-"
	}

	unique := make(map[string]struct{})
	for _, current := range contexts {
		name := strings.TrimSpace(current.SNI)
		if name == "" ||
			name == "no_record" ||
			name == "not_known" {
			continue
		}
		unique[name] = struct{}{}
	}

	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return "-"
	}
	if len(names) > 3 {
		return strings.Join(names[:3], ",") + ",+"
	}
	return strings.Join(names, ",")
}

func flowHistoricalName(
	identities []natcorrelation.HistoricalIdentitySummary,
) string {
	if len(identities) == 0 {
		return "-"
	}

	names := make(map[string]struct{})
	for _, identity := range identities {
		name := identity.Hostname
		if name == "" || name == "no_record" {
			name = identity.FQDN
		}
		if name == "" || name == "no_record" {
			continue
		}
		names[name] = struct{}{}
	}

	values := make([]string, 0, len(names))
	for name := range names {
		values = append(values, name)
	}
	sort.Strings(values)

	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func printNATFlowsTable(result natflow.Result) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	fmt.Fprintln(
		writer,
		"DHCP SNAPSHOT NAME\tLAN SOURCE\tLPORT\tWAN SOURCE\tWPORT\tDESTINATION\tDPORT\tSTART (UTC)\tLAST (UTC)\tLAN OUT\tWAN OUT\tWAN RET\tLAN RET\tRETURN\tFIN OUT\tFIN RET\tRST\tDNS AT START\tSNI\tSTATUS",
	)

	for _, current := range result.Flows {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%d\t%s\t%d\t%s\t%d\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			flowHistoricalName(current.HistoricalIdentities),
			current.LANSourceIP,
			current.LANSourcePort,
			current.WANSourceIP,
			current.WANSourcePort,
			current.DestinationIP,
			current.DestinationPort,
			current.AnchorLANObservedAt.UTC().Format("2006-01-02 15:04:05.000"),
			current.LastSeen.UTC().Format("2006-01-02 15:04:05.000"),
			current.LANOutbound.Observations,
			current.WANOutbound.Observations,
			current.WANReturn.Observations,
			current.LANReturn.Observations,
			yesNo(current.ReturnPathObserved),
			yesNo(current.TCPFINOutboundObserved),
			yesNo(current.TCPFINReturnObserved),
			yesNo(current.TCPRSTObserved),
			flowDNSNames(current.DNSAtInitiation),
			flowSNINames(current.TLSAtInitiation),
			current.PathStatus,
		)
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"files=%d active_files=%d active_trailing_fragments=%d observations=%d observations_before_since=%d active_identity_files=%d identity_active_trailing_fragments=%d vanished_segments=%d nat_relationships=%d anchors_accepted=%d anchors_refused_multiple=%d flows_refused_additional_initiation=%d flows_filtered_by_sni=%d assigned_observations=%d ambiguous_flow_observations=%d dns_bindings=%d tls_client_hellos=%d tls_sni_names=%d flows_found=%d flows_returned=%d\n",
		result.Stats.ObservationFilesRead,
		result.Stats.ActiveObservationFilesRead,
		result.Stats.ActiveObservationTrailingFragmentsIgnored,
		result.Stats.ObservationsRead,
		result.Stats.ObservationsBeforeSince,
		result.Stats.ActiveIdentityFilesRead,
		result.Stats.ActiveIdentityTrailingFragmentsIgnored,
		result.Stats.SegmentsVanishedDuringRead,
		result.Stats.NATRelationshipsFound,
		result.Stats.AnchorsAccepted,
		result.Stats.AnchorsRefusedMultipleInitiations,
		result.Stats.FlowsRefusedAdditionalInitiation,
		result.Stats.FlowsFilteredBySNI,
		result.Stats.FlowObservationsAssigned,
		result.Stats.AmbiguousFlowObservations,
		result.Stats.DNSBindingsObserved,
		result.Stats.TLSClientHellosObserved,
		result.Stats.TLSSNINamesObserved,
		result.Stats.FlowsFound,
		result.Stats.FlowsReturned,
	)

	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
