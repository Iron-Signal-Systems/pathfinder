package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/conntrackhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

func conntrackEventsCommand(args []string) error {
	flags := flag.NewFlagSet("conntrack-events", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	inputDir := flags.String(
		"input-dir",
		"",
		"Pathfinder observation directory or conntrack subdirectory",
	)
	eventType := flags.String(
		"event",
		"any",
		"event type: any, new, update, or destroy",
	)
	source := flags.String(
		"source",
		"",
		"source IP or CIDR in either original or reply tuple",
	)
	destination := flags.String(
		"destination",
		"",
		"destination IP or CIDR in either original or reply tuple",
	)
	protocol := flags.String(
		"protocol",
		"",
		"original tuple protocol, for example tcp or udp",
	)
	sinceText := flags.String(
		"since",
		"0",
		"only consider events this recent; example 30s, 2m, 1h, or 0",
	)
	limit := flags.Int(
		"limit",
		50,
		"maximum events to display; 0 means all",
	)
	format := flags.String(
		"format",
		"text",
		"output format: text or json",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputDir == "" {
		return fmt.Errorf("conntrack-events requires --input-dir")
	}
	if *limit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}

	since, err := parseSince(*sinceText)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}

	result, err := conntrackhistory.Query(conntrackhistory.QueryOptions{
		Destination: *destination,
		EventType:   *eventType,
		InputDir:    *inputDir,
		Limit:       *limit,
		Protocol:    *protocol,
		Since:       since,
		Source:      *source,
	})
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return err
		}

	case "text":
		if err := conntrackhistory.WriteText(
			os.Stdout,
			result,
		); err != nil {
			return err
		}

	default:
		return fmt.Errorf(
			"unsupported --format %q; use text or json",
			*format,
		)
	}

	fmt.Fprintf(
		os.Stderr,
		"files=%d active_files=%d active_trailing_fragments=%d vanished_segments=%d events=%d events_before_since=%d events_returned=%d\n",
		result.Stats.FilesRead,
		result.Stats.ActiveFilesRead,
		result.Stats.ActiveTrailingFragmentsIgnored,
		result.Stats.SegmentsVanishedDuringRead,
		result.Stats.EventsRead,
		result.Stats.EventsBeforeSince,
		result.Stats.EventsReturned,
	)

	return nil
}

func newConntrackRecorder(
	enabled bool,
	outputDir string,
	segmentSizeText string,
	maxSizeText string,
	minFreeText string,
	compression string,
	socketBufferText string,
	diagnostics io.Writer,
) (*conntrackhistory.Recorder, error) {
	if !enabled || outputDir == "" {
		return nil, nil
	}

	segmentBytes, err := storage.ParseBytes(segmentSizeText)
	if err != nil {
		return nil, fmt.Errorf(
			"parse --conntrack-segment-size: %w",
			err,
		)
	}
	maxBytes, err := storage.ParseBytes(maxSizeText)
	if err != nil {
		return nil, fmt.Errorf(
			"parse --conntrack-max-size: %w",
			err,
		)
	}
	minFreeBytes, err := storage.ParseBytes(minFreeText)
	if err != nil {
		return nil, fmt.Errorf(
			"parse --min-free for conntrack history: %w",
			err,
		)
	}
	socketBufferBytes64, err := storage.ParseBytes(socketBufferText)
	if err != nil {
		return nil, fmt.Errorf(
			"parse --conntrack-socket-buffer: %w",
			err,
		)
	}
	maxInt := uint64(^uint(0) >> 1)
	if socketBufferBytes64 == 0 ||
		socketBufferBytes64 > maxInt {
		return nil, fmt.Errorf(
			"--conntrack-socket-buffer must fit in a positive int",
		)
	}

	return conntrackhistory.NewRecorder(
		conntrackhistory.RecorderOptions{
			Compression: compression,
			Directory: conntrackhistory.DefaultDirectory(
				outputDir,
			),
			MaxBytes:     maxBytes,
			MinFreeBytes: minFreeBytes,
			OnError: func(err error) {
				fmt.Fprintf(
					diagnostics,
					"conntrack history warning: %v\n",
					err,
				)
			},
			SegmentBytes:      segmentBytes,
			SocketBufferBytes: int(socketBufferBytes64),
		},
	)
}
