package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/capture"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/conntrackhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/healthhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/identityhistory"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (writer *lockedWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.writer.Write(data)
}

func newHealthRecorder(
	outputDir string,
	interval time.Duration,
	segmentSizeText string,
	maxSizeText string,
	minFreeText string,
	compression string,
	diagnostics io.Writer,
) (*healthhistory.Recorder, error) {
	if outputDir == "" || interval <= 0 {
		return nil, nil
	}

	segmentBytes, err := storage.ParseBytes(segmentSizeText)
	if err != nil {
		return nil, fmt.Errorf("parse --health-segment-size: %w", err)
	}
	maxBytes, err := storage.ParseBytes(maxSizeText)
	if err != nil {
		return nil, fmt.Errorf("parse --health-max-size: %w", err)
	}
	minFreeBytes, err := storage.ParseBytes(minFreeText)
	if err != nil {
		return nil, fmt.Errorf("parse --min-free for health history: %w", err)
	}

	return healthhistory.NewRecorder(healthhistory.RecorderOptions{
		Compression:  compression,
		Directory:    healthhistory.DefaultDirectory(outputDir),
		Interval:     interval,
		MaxBytes:     maxBytes,
		MinFreeBytes: minFreeBytes,
		OnError: func(err error) {
			fmt.Fprintf(diagnostics, "health history warning: %v\n", err)
		},
		SegmentBytes: segmentBytes,
	})
}

func startHealthReporter(
	interval time.Duration,
	metrics *capture.Metrics,
	ring *storage.Ring,
	conntrackRecorder *conntrackhistory.Recorder,
	identityRecorder *identityhistory.Recorder,
	healthRecorder *healthhistory.Recorder,
	writer io.Writer,
) func() {
	if interval <= 0 {
		return func() {}
	}

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := writeHealthReport(
					writer,
					metrics,
					ring,
					conntrackRecorder,
					identityRecorder,
					healthRecorder,
					interval,
					false,
				); err != nil {
					fmt.Fprintf(
						writer,
						"health reporting warning: %v\n",
						err,
					)
				}
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func writeHealthReport(
	writer io.Writer,
	metrics *capture.Metrics,
	ring *storage.Ring,
	conntrackRecorder *conntrackhistory.Recorder,
	identityRecorder *identityhistory.Recorder,
	healthRecorder *healthhistory.Recorder,
	interval time.Duration,
	final bool,
) error {
	storageHealth := healthhistory.Storage{
		Enabled: ring != nil,
		Stats: storage.Stats{
			CompressionMode: "not_configured",
		},
	}
	if ring != nil {
		storageHealth.Stats = ring.Stats()
	}

	conntrackHealth := conntrackhistory.DisabledStats()
	if conntrackRecorder != nil {
		conntrackHealth = conntrackRecorder.Stats()
	}

	identityHealth := identityhistory.DisabledStats()
	if identityRecorder != nil {
		identityHealth = identityRecorder.Stats()
	}

	report := healthhistory.Report{
		Capture:                 metrics.Snapshot(),
		Conntrack:               conntrackHealth,
		ExpectedIntervalSeconds: int64(interval / time.Second),
		Final:                   final,
		Identity:                identityHealth,
		ObservedAt:              time.Now().UTC(),
		RecordType:              healthhistory.RecordType,
		SchemaVersion:           healthhistory.SchemaVersion,
		Storage:                 storageHealth,
	}

	diagnosticErr := json.NewEncoder(writer).Encode(report)
	persistentErr := healthRecorder.Write(report)

	if diagnosticErr != nil && persistentErr != nil {
		return fmt.Errorf(
			"diagnostic health write: %v; persistent health write: %w",
			diagnosticErr,
			persistentErr,
		)
	}
	if diagnosticErr != nil {
		return fmt.Errorf("diagnostic health write: %w", diagnosticErr)
	}
	if persistentErr != nil {
		return fmt.Errorf("persistent health write: %w", persistentErr)
	}

	return nil
}
