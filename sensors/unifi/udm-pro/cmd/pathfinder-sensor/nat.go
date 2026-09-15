package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natcorrelation"
)

func nat(args []string) error {
	flags := flag.NewFlagSet("nat", flag.ContinueOnError)
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
		"maximum absolute LAN/WAN observation time difference",
	)
	sinceText := flags.String(
		"since",
		"0",
		"only consider NAT candidates this recent; example 30s, 2m, 1h, or 0",
	)
	source := flags.String(
		"source",
		"",
		"LAN source IP or CIDR",
	)
	destination := flags.String(
		"destination",
		"",
		"destination IP or CIDR",
	)
	protocol := flags.String(
		"protocol",
		"any",
		"protocol filter: any, tcp, or udp",
	)
	limit := flags.Int(
		"limit",
		50,
		"maximum NAT relationships to display; 0 means all",
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
		return fmt.Errorf("nat requires --input-dir")
	}
	if *limit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
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

	result, err := natcorrelation.Correlate(natcorrelation.Options{
		Destination:  *destination,
		IdentityDir:  *identityDir,
		InputDir:     *inputDir,
		LANInterface: *lanInterface,
		Limit:        *limit,
		MaxDelta:     maxDelta,
		Protocol:     strings.ToLower(*protocol),
		Since:        since,
		Source:       *source,
		WANInterface: *wanInterface,
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
		return printNATTable(result)

	default:
		return fmt.Errorf(
			"unsupported --format %q; use table or json",
			*format,
		)
	}
}

func natHistoricalNames(
	identities []natcorrelation.HistoricalIdentitySummary,
) string {
	if len(identities) == 0 {
		return "-"
	}

	unique := make(map[string]struct{})
	for _, identity := range identities {
		name := identity.Hostname
		if name == "" || name == "no_record" {
			name = identity.FQDN
		}
		if name == "" || name == "no_record" {
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

func printNATTable(result natcorrelation.Result) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	fmt.Fprintln(
		writer,
		"DHCP SNAPSHOT NAME\tLAN SOURCE\tLAN MAC\tLPORT\tWAN SOURCE\tWPORT\tDESTINATION\tPROTO\tDPORT\tFIRST LAN (UTC)\tLAST LAN (UTC)\tMATCHED\tMAX OBS Δ µs\tBASIS",
	)

	for _, current := range result.Relationships {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
			natHistoricalNames(current.HistoricalIdentities),
			current.LANSourceIP,
			current.LANSourceMAC,
			current.LANSourcePort,
			current.WANSourceIP,
			current.WANSourcePort,
			current.DestinationIP,
			current.Protocol,
			strconv.Itoa(int(current.DestinationPort)),
			current.FirstLANObservedAt.UTC().Format("2006-01-02 15:04:05.000"),
			current.LastLANObservedAt.UTC().Format("2006-01-02 15:04:05.000"),
			current.MatchedEvents,
			current.MaxAbsoluteDeltaMicros,
			current.CorrelationBasis,
		)
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"files=%d active_files=%d active_trailing_fragments=%d observations=%d observations_before_since=%d identity_files=%d active_identity_files=%d identity_active_trailing_fragments=%d identity_snapshots=%d vanished_segments=%d lan_candidates=%d wan_candidates=%d correlated_events=%d ambiguous_lan=%d unmatched_lan=%d unmatched_wan=%d relationships_found=%d relationships_returned=%d\n",
		result.Stats.ObservationFilesRead,
		result.Stats.ActiveObservationFilesRead,
		result.Stats.ActiveObservationTrailingFragmentsIgnored,
		result.Stats.ObservationsRead,
		result.Stats.ObservationsBeforeSince,
		result.Stats.IdentityFilesRead,
		result.Stats.ActiveIdentityFilesRead,
		result.Stats.ActiveIdentityTrailingFragmentsIgnored,
		result.Stats.IdentitySnapshotsRead,
		result.Stats.SegmentsVanishedDuringRead,
		result.Stats.LANCandidates,
		result.Stats.WANCandidates,
		result.Stats.CorrelatedEvents,
		result.Stats.AmbiguousLANCandidates,
		result.Stats.UnmatchedLANCandidates,
		result.Stats.UnmatchedWANCandidates,
		result.Stats.RelationshipsFound,
		result.Stats.RelationshipsReturned,
	)

	return nil
}
