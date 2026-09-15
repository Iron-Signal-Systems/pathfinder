package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

func newIdentityRecorder(
	outputDir string,
	interval time.Duration,
	leasePath string,
	hostsPath string,
	segmentSizeText string,
	maxSizeText string,
	minFreeText string,
	compression string,
	diagnostics io.Writer,
) (*identityhistory.Recorder, error) {
	if outputDir == "" || interval <= 0 {
		return nil, nil
	}

	segmentBytes, err := storage.ParseBytes(segmentSizeText)
	if err != nil {
		return nil, fmt.Errorf("parse --identity-segment-size: %w", err)
	}

	maxBytes, err := storage.ParseBytes(maxSizeText)
	if err != nil {
		return nil, fmt.Errorf("parse --identity-max-size: %w", err)
	}

	minFreeBytes, err := storage.ParseBytes(minFreeText)
	if err != nil {
		return nil, fmt.Errorf("parse --min-free for identity history: %w", err)
	}

	return identityhistory.NewRecorder(identityhistory.RecorderOptions{
		Compression:  compression,
		Directory:    filepath.Join(outputDir, "identity"),
		HostsPath:    hostsPath,
		Interval:     interval,
		LeasePath:    leasePath,
		MaxBytes:     maxBytes,
		MinFreeBytes: minFreeBytes,
		OnError: func(err error) {
			fmt.Fprintf(diagnostics, "identity history warning: %v\n", err)
		},
		SegmentBytes: segmentBytes,
	})
}
