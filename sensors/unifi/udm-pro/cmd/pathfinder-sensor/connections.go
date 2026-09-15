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

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/query"
)

func connections(args []string) error {
	flags := flag.NewFlagSet("connections", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	inputDir := flags.String("input-dir", "", "directory containing Pathfinder observation segments")
	interfaceName := flags.String("interface", "br0", "observation interface; use any for all")
	direction := flags.String("direction", "ingress", "observation direction; use any for all")
	dnsMode := flags.String("dns-mode", "auto", "DNS attribution: auto, client, or off")
	identityMode := flags.String("identity-mode", "auto", "current source identity enrichment: auto, current-dhcp, or off")
	identityDir := flags.String("identity-dir", "", "historical endpoint identity directory; default <input-dir>/identity")
	dhcpLeaseFile := flags.String("dhcp-lease-file", "/run/dnsmasq.lease", "current dnsmasq lease file")
	dhcpHostsFile := flags.String("dhcp-hosts-file", "/run/dnsmasq.dns.conf.d/hosts.d/leases", "current dnsmasq generated hosts file")
	source := flags.String("source", "", "source IP or CIDR")
	destination := flags.String("destination", "", "destination IP or CIDR")
	protocol := flags.String("protocol", "any", "protocol filter, such as tcp or udp")
	sinceText := flags.String("since", "0", "only summarize observations this recent; example 1h, 24h, or 0")
	limit := flags.Int("limit", 50, "maximum relationships to display; 0 means all")
	format := flags.String("format", "table", "output format: table or json")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputDir == "" {
		return fmt.Errorf("connections requires --input-dir")
	}
	if *limit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}

	since, err := parseSince(*sinceText)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}

	result, err := query.Connections(query.ConnectionOptions{
		Destination:   *destination,
		Direction:     strings.ToLower(*direction),
		DNSMode:       strings.ToLower(*dnsMode),
		DHCPHostsFile: *dhcpHostsFile,
		DHCPLeaseFile: *dhcpLeaseFile,
		IdentityDir:   *identityDir,
		IdentityMode:  strings.ToLower(*identityMode),
		InputDir:      *inputDir,
		Interface:     *interfaceName,
		Limit:         *limit,
		Protocol:      strings.ToLower(*protocol),
		Source:        *source,
		Since:         since,
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
		return printConnectionTable(result)
	default:
		return fmt.Errorf("unsupported --format %q; use table or json", *format)
	}
}

func connectionDNSNames(contexts []query.DNSContext) string {
	if len(contexts) == 0 {
		return "-"
	}

	unique := make(map[string]struct{})
	for _, current := range contexts {
		name := current.QueryName
		if name == "" {
			name = current.AnswerName
		}
		if name != "" {
			unique[name] = struct{}{}
		}
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

func historicalSourceNames(current query.ConnectionSummary) string {
	if len(current.HistoricalSourceIdentities) == 0 {
		return "-"
	}

	unique := make(map[string]struct{})
	for _, identity := range current.HistoricalSourceIdentities {
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

func currentSourceName(current query.ConnectionSummary) string {
	identity := current.CurrentSourceIdentity
	if identity.Status != "matched" {
		return "-"
	}
	if identity.Hostname != "" && identity.Hostname != "no_record" {
		return identity.Hostname
	}
	if identity.FQDN != "" && identity.FQDN != "no_record" {
		return identity.FQDN
	}
	return "-"
}

func parseSince(value string) (time.Duration, error) {
	if value == "" || value == "0" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration < 0 {
		return 0, fmt.Errorf("duration must be zero or greater")
	}
	return duration, nil
}

func printConnectionTable(result query.Result) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	fmt.Fprintln(
		writer,
		"DHCP SNAPSHOT NAME\tCURRENT DHCP NAME\tSOURCE\tSOURCE MAC\tDESTINATION\tPROTO\tDPORT\tFIRST SEEN (UTC)\tLAST SEEN (UTC)\tOBS\tHIST ID OBS\tSYN\tDNS AT TIME",
	)

	for _, current := range result.Connections {
		port := "-"
		if current.DestinationPort != 0 {
			port = strconv.Itoa(int(current.DestinationPort))
		}

		syn := "-"
		if current.Protocol == "tcp" {
			if current.TCPInitiationObserved {
				syn = "yes"
			} else {
				syn = "no"
			}
		}

		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\n",
			historicalSourceNames(current),
			currentSourceName(current),
			current.SourceIP,
			current.SourceMAC,
			current.DestinationIP,
			current.Protocol,
			port,
			current.FirstSeen.UTC().Format("2006-01-02 15:04:05"),
			current.LastSeen.UTC().Format("2006-01-02 15:04:05"),
			current.ObservationCount,
			current.HistoricalIdentityMatchedObservations,
			syn,
			connectionDNSNames(current.DNS),
		)
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"files=%d active_files=%d active_trailing_fragments=%d observations=%d identity_files=%d active_identity_files=%d identity_active_trailing_fragments=%d identity_snapshots=%d vanished_segments=%d relationships_found=%d relationships_returned=%d\n",
		result.Stats.FilesRead,
		result.Stats.ActiveObservationFilesRead,
		result.Stats.ActiveObservationTrailingFragmentsIgnored,
		result.Stats.ObservationsRead,
		result.Stats.IdentityFilesRead,
		result.Stats.ActiveIdentityFilesRead,
		result.Stats.ActiveIdentityTrailingFragmentsIgnored,
		result.Stats.IdentitySnapshotsRead,
		result.Stats.SegmentsVanishedDuringRead,
		result.Stats.RelationshipsFound,
		result.Stats.RelationshipsReturned,
	)

	return nil
}
