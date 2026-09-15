// Package pathview renders derived NAT flows for operators.
package pathview

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natcorrelation"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/natflow"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

// WriteText renders an operator-facing report without changing the underlying
// derived-flow semantics.
func WriteText(writer io.Writer, result natflow.Result) error {
	if len(result.Flows) == 0 {
		_, err := fmt.Fprintln(
			writer,
			"No qualifying NAT-anchored TCP paths found.",
		)
		return err
	}

	for index, current := range result.Flows {
		if index > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}

		if err := writeFlow(writer, index+1, current); err != nil {
			return err
		}
	}

	return nil
}

func writeFlow(writer io.Writer, number int, current natflow.Flow) error {
	name := historicalName(current.HistoricalIdentities)
	if name == "-" {
		name = current.LANSourceIP
	}

	title := fmt.Sprintf("Path %d - %s", number, name)
	if _, err := fmt.Fprintln(writer, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("=", len(title))); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "Endpoint"); err != nil {
		return err
	}
	if err := writeHistoricalIdentity(
		writer,
		current.HistoricalIdentities,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  Observed LAN IP:           %s\n",
		current.LANSourceIP,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  Observed LAN MAC:          %s\n",
		current.LANSourceMAC,
	); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "\nObserved path"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  %s ingress  %s:%d -> %s:%d\n",
		current.LANInterface,
		current.LANSourceIP,
		current.LANSourcePort,
		current.DestinationIP,
		current.DestinationPort,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(
		writer,
		"       | TCP initiation observed",
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"       | source NAT: %s:%d\n",
		current.WANSourceIP,
		current.WANSourcePort,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"       v\n  %s egress   %s:%d -> %s:%d\n",
		current.WANInterface,
		current.WANSourceIP,
		current.WANSourcePort,
		current.DestinationIP,
		current.DestinationPort,
	); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "\nInitiation"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  LAN SYN observed:          %s\n",
		formatTimestamp(current.AnchorLANObservedAt, true),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  LAN timestamp source:      %s\n",
		displayTimestampSource(current.AnchorLANTimestampSource),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  WAN SYN observed:          %s\n",
		formatTimestamp(current.AnchorWANObservedAt, true),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  WAN timestamp source:      %s\n",
		displayTimestampSource(current.AnchorWANTimestampSource),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  Observation timestamp delta: %d microseconds\n",
		current.AnchorAbsoluteDeltaMicros,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  Correlation basis:         %s\n",
		current.CorrelationBasis,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  Source IP translated:      %s\n",
		yesNo(current.SourceIPTranslated),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  Source port preserved:     %s\n",
		yesNo(current.SourcePortPreserved),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(
		writer,
		"  Timing interpretation:     observation-point delta; not asserted as forwarding latency",
	); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(
		writer,
		"\nObserved traffic by vantage",
	); err != nil {
		return err
	}
	for _, entry := range []struct {
		label string
		stats natflow.LegStats
	}{
		{"LAN outbound", current.LANOutbound},
		{"WAN outbound", current.WANOutbound},
		{"WAN return", current.WANReturn},
		{"LAN return", current.LANReturn},
	} {
		if err := writeLeg(writer, entry.label, entry.stats); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(
		writer,
		"  Note: counts/bytes are per observation point and are not summed across interfaces.",
	); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "\nConnection state"); err != nil {
		return err
	}
	for _, entry := range []struct {
		label string
		value string
	}{
		{"Return path observed", yesNo(current.ReturnPathObserved)},
		{"FIN outbound observed", yesNo(current.TCPFINOutboundObserved)},
		{"FIN return observed", yesNo(current.TCPFINReturnObserved)},
		{"RST observed", yesNo(current.TCPRSTObserved)},
		{"Close", closeText(current.ClosedAt)},
		{"First observed", formatTimestamp(current.AnchorLANObservedAt, false)},
		{"Last observed", formatTimestamp(current.LastSeen, false)},
		{"Observed duration", durationText(current.AnchorLANObservedAt, current.LastSeen)},
		{"Path status", current.PathStatus},
	} {
		if _, err := fmt.Fprintf(
			writer,
			"  %-27s %s\n",
			entry.label+":",
			entry.value,
		); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(writer, "\nDNS at initiation"); err != nil {
		return err
	}
	if err := writeDNS(writer, current.DNSAtInitiation); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "\nTLS at initiation"); err != nil {
		return err
	}
	if err := writeTLS(writer, current.TLSAtInitiation); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "\nNetwork context"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  SVI:                       %s\n",
		displayContextValue(current.LANSVI),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"  VLAN ID:                   %s\n",
		displayVLANID(current.LANVLANID),
	); err != nil {
		return err
	}

	return nil
}

func writeHistoricalIdentity(
	writer io.Writer,
	identities []natcorrelation.HistoricalIdentitySummary,
) error {
	if len(identities) == 0 {
		_, err := fmt.Fprintln(
			writer,
			"  Historical DHCP identity: no qualifying snapshot",
		)
		return err
	}

	copyIdentities := append(
		[]natcorrelation.HistoricalIdentitySummary(nil),
		identities...,
	)
	sort.Slice(copyIdentities, func(i, j int) bool {
		return copyIdentities[i].SnapshotObservedAt.Before(
			copyIdentities[j].SnapshotObservedAt,
		)
	})

	seen := make(map[string]struct{})
	for _, current := range copyIdentities {
		key := current.Hostname + "\x00" +
			current.FQDN + "\x00" +
			current.SnapshotObservedAt.Format(time.RFC3339Nano)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		name := current.Hostname
		if name == "" || name == observation.ValueNoRecord {
			name = current.FQDN
		}
		if name == "" || name == observation.ValueNoRecord {
			name = observation.ValueNotKnown
		}

		if _, err := fmt.Fprintf(
			writer,
			"  Historical DHCP identity: %s\n",
			name,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  Identity snapshot:         %s\n",
			formatTimestamp(current.SnapshotObservedAt, false),
		); err != nil {
			return err
		}
		if current.FQDN != "" &&
			current.FQDN != observation.ValueNoRecord {
			if _, err := fmt.Fprintf(
				writer,
				"  Historical FQDN:         %s\n",
				current.FQDN,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeTLS(
	writer io.Writer,
	contexts []natflow.TLSContext,
) error {
	if len(contexts) == 0 {
		_, err := fmt.Fprintln(
			writer,
			"  No plaintext SNI observed in a complete TLS ClientHello for this path.",
		)
		return err
	}

	for _, current := range contexts {
		if _, err := fmt.Fprintf(
			writer,
			"  Observed SNI:              %s\n",
			current.SNI,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  Observed at:               %s\n",
			formatTimestamp(current.ObservedAt, true),
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  Vantage:                   %s ingress\n",
			current.Interface,
		); err != nil {
			return err
		}
	}

	return nil
}

func writeLeg(
	writer io.Writer,
	label string,
	stats natflow.LegStats,
) error {
	if _, err := fmt.Fprintf(
		writer,
		"  %-14s %6d observations / %10d observed IP bytes",
		label+":",
		stats.Observations,
		stats.ObservedIPBytes,
	); err != nil {
		return err
	}

	if stats.OffloadSuspectedObservations > 0 {
		if _, err := fmt.Fprintf(
			writer,
			" / %d offload-suspected",
			stats.OffloadSuspectedObservations,
		); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(writer)
	return err
}

func writeDNS(writer io.Writer, contexts []natflow.DNSContext) error {
	if len(contexts) == 0 {
		_, err := fmt.Fprintln(
			writer,
			"  No qualifying DNS answer observed for this endpoint/address at initiation.",
		)
		return err
	}

	copyContexts := append([]natflow.DNSContext(nil), contexts...)
	sort.Slice(copyContexts, func(i, j int) bool {
		return copyContexts[i].ObservedAt.Before(copyContexts[j].ObservedAt)
	})

	for _, current := range copyContexts {
		name := current.QueryName
		if name == "" {
			name = current.AnswerName
		}
		if name == "" {
			name = observation.ValueNotKnown
		}

		if _, err := fmt.Fprintf(
			writer,
			"  %s -> %s\n",
			name,
			current.Address,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"    resolver:    %s\n",
			current.ResolverIP,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"    observed:    %s\n",
			formatTimestamp(current.ObservedAt, false),
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"    ttl:         %d seconds\n",
			current.TTL,
		); err != nil {
			return err
		}
		if current.AnswerName != "" &&
			current.AnswerName != name {
			if _, err := fmt.Fprintf(
				writer,
				"    answer name: %s\n",
				current.AnswerName,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func closeText(value string) string {
	if value == "" ||
		value == observation.ValueNoRecord ||
		value == observation.ValueNotKnown {
		return "no close observed"
	}
	return value
}

func displayContextValue(value string) string {
	if value == "" {
		return observation.ValueNotKnown
	}
	return value
}

func displayVLANID(value int) string {
	if value <= 0 {
		return observation.ValueNotKnown
	}
	return fmt.Sprintf("%d", value)
}

func displayTimestampSource(value string) string {
	if value == "" {
		return observation.ValueNotKnown
	}
	return value
}

func durationText(start time.Time, end time.Time) string {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return observation.ValueNotKnown
	}

	duration := end.Sub(start)
	if duration < time.Millisecond {
		return duration.String()
	}
	return duration.Round(time.Millisecond).String()
}

func formatTimestamp(value time.Time, microseconds bool) string {
	if microseconds {
		return value.UTC().Format("2006-01-02 15:04:05.000000 UTC")
	}
	return value.UTC().Format("2006-01-02 15:04:05.000 UTC")
}

func historicalName(
	identities []natcorrelation.HistoricalIdentitySummary,
) string {
	names := make(map[string]struct{})
	for _, current := range identities {
		name := current.Hostname
		if name == "" || name == observation.ValueNoRecord {
			name = current.FQDN
		}
		if name == "" || name == observation.ValueNoRecord {
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

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
