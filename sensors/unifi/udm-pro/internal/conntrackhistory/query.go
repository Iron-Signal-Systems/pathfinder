package conntrackhistory

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/segmentread"
)

const maxScannerToken = 4 * 1024 * 1024

// QueryOptions controls read-only conntrack history queries.
type QueryOptions struct {
	Destination string
	EventType   string
	InputDir    string
	Limit       int
	Protocol    string
	Since       time.Duration
	Source      string
}

// QueryStats reports conntrack history source material.
type QueryStats struct {
	ActiveFilesRead                uint64 `json:"active_files_read"`
	ActiveTrailingFragmentsIgnored uint64 `json:"active_trailing_fragments_ignored"`
	EventsBeforeSince              uint64 `json:"events_before_since"`
	EventsRead                     uint64 `json:"events_read"`
	EventsReturned                 uint64 `json:"events_returned"`
	FilesRead                      uint64 `json:"files_read"`
	SegmentsVanishedDuringRead     uint64 `json:"segments_vanished_during_read"`
}

// QueryResult is a filtered conntrack event set.
type QueryResult struct {
	Events []Event    `json:"events"`
	Stats  QueryStats `json:"stats"`
}

// Query reads current and completed conntrack segments without modifying them.
func Query(options QueryOptions) (QueryResult, error) {
	if options.InputDir == "" {
		return QueryResult{}, fmt.Errorf("input directory is required")
	}
	if options.Limit < 0 {
		return QueryResult{}, fmt.Errorf("limit must be zero or greater")
	}
	if options.Since < 0 {
		return QueryResult{}, fmt.Errorf("since must be zero or greater")
	}

	eventType := strings.ToLower(strings.TrimSpace(options.EventType))
	if eventType == "" {
		eventType = "any"
	}
	switch eventType {
	case "any", EventNew, EventUpdate, EventDestroy:
	default:
		return QueryResult{}, fmt.Errorf(
			"unsupported event type %q",
			eventType,
		)
	}

	protocol := strings.ToLower(strings.TrimSpace(options.Protocol))
	sourceMatch, err := parseAddressMatch(options.Source)
	if err != nil {
		return QueryResult{}, fmt.Errorf("parse source: %w", err)
	}
	destinationMatch, err := parseAddressMatch(options.Destination)
	if err != nil {
		return QueryResult{}, fmt.Errorf("parse destination: %w", err)
	}

	directory := options.InputDir
	if filepath.Base(directory) != "conntrack" {
		directory = DefaultDirectory(directory)
	}

	files, err := segmentread.List(directory, "conntrack-")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return QueryResult{
				Events: []Event{},
			}, nil
		}
		return QueryResult{}, err
	}

	cutoff := time.Time{}
	if options.Since > 0 {
		cutoff = time.Now().UTC().Add(-options.Since)
	}

	result := QueryResult{
		Events: make([]Event, 0),
	}

	for _, path := range files {
		readStats, readErr := segmentread.ReadLines(
			path,
			maxScannerToken,
			func(line []byte) error {
				var current Event
				if err := json.Unmarshal(line, &current); err != nil {
					return fmt.Errorf(
						"decode conntrack event: %w",
						err,
					)
				}
				if current.RecordType != RecordType ||
					current.ObservedAt.IsZero() {
					return nil
				}

				result.Stats.EventsRead++

				if !cutoff.IsZero() &&
					current.ObservedAt.Before(cutoff) {
					result.Stats.EventsBeforeSince++
					return nil
				}
				if eventType != "any" &&
					current.EventType != eventType {
					return nil
				}
				if protocol != "" &&
					current.Original.Protocol != protocol {
					return nil
				}
				if !sourceMatch.matchesTupleSource(current) {
					return nil
				}
				if !destinationMatch.matchesTupleDestination(current) {
					return nil
				}

				result.Events = append(result.Events, current)
				return nil
			},
		)
		if readErr != nil {
			return QueryResult{}, fmt.Errorf(
				"read conntrack history %s: %w",
				path,
				readErr,
			)
		}
		if readStats.Vanished {
			result.Stats.SegmentsVanishedDuringRead++
			continue
		}
		result.Stats.FilesRead++
		if readStats.Active {
			result.Stats.ActiveFilesRead++
		}
		if readStats.TrailingFragmentIgnored {
			result.Stats.ActiveTrailingFragmentsIgnored++
		}
	}

	sort.Slice(result.Events, func(i, j int) bool {
		return result.Events[i].ObservedAt.After(
			result.Events[j].ObservedAt,
		)
	})

	if options.Limit > 0 && len(result.Events) > options.Limit {
		result.Events = result.Events[:options.Limit]
	}
	result.Stats.EventsReturned = uint64(len(result.Events))

	return result, nil
}

type addressMatch struct {
	exact  netip.Addr
	prefix netip.Prefix
	set    bool
}

func parseAddressMatch(value string) (addressMatch, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "any") {
		return addressMatch{}, nil
	}
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return addressMatch{}, err
		}
		return addressMatch{
			prefix: prefix,
			set:    true,
		}, nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return addressMatch{}, err
	}
	return addressMatch{
		exact: address,
		set:   true,
	}, nil
}

func (match addressMatch) matches(value string) bool {
	if !match.set {
		return true
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return false
	}
	if match.prefix.IsValid() {
		return match.prefix.Contains(address)
	}
	return match.exact == address
}

func (match addressMatch) matchesTupleDestination(event Event) bool {
	if !match.set {
		return true
	}
	return match.matches(event.Original.DestinationIP) ||
		match.matches(event.Reply.DestinationIP)
}

func (match addressMatch) matchesTupleSource(event Event) bool {
	if !match.set {
		return true
	}
	return match.matches(event.Original.SourceIP) ||
		match.matches(event.Reply.SourceIP)
}
