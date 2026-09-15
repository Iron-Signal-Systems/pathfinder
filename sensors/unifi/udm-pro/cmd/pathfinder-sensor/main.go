package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/capture"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/interfaces"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

const (
	name    = "Pathfinder Sensor - UniFi"
	version = "0.0.1-dev"
)

type stringList []string

func (values *stringList) String() string {
	return fmt.Sprintf("%v", []string(*values))
}

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "connections":
			if err := connections(os.Args[2:]); err != nil {
				fatal(err)
			}
			return

		case "conntrack-events":
			if err := conntrackEventsCommand(os.Args[2:]); err != nil {
				fatal(err)
			}
			return

		case "interfaces":
			if err := printInterfaces(); err != nil {
				fatal(err)
			}
			return

		case "nat":
			if err := nat(os.Args[2:]); err != nil {
				fatal(err)
			}
			return

		case "nat-flows":
			if err := natFlows(os.Args[2:]); err != nil {
				fatal(err)
			}
			return

		case "observe":
			if err := observe(os.Args[2:]); err != nil {
				fatal(err)
			}
			return

		case "path":
			if err := pathCommand(os.Args[2:]); err != nil {
				fatal(err)
			}
			return
		}
	}

	printVersion()
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func observe(args []string) error {
	flags := flag.NewFlagSet("observe", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var names stringList
	flags.Var(&names, "interface", "interface to observe; may be repeated")

	count := flags.Int("count", 0, "stop after this many observations; 0 runs until interrupted")
	outputDir := flags.String("output-dir", "", "rolling JSONL output directory; empty writes to stdout")
	segmentSize := flags.String("segment-size", "1GiB", "maximum completed segment size")
	maxSize := flags.String("max-size", "100GiB", "maximum rolling observation storage")
	minFree := flags.String("min-free", "50GiB", "minimum filesystem free space to preserve")
	compression := flags.String("compression", "gzip", "completed-segment compression: gzip or none")
	conntrackHistory := flags.Bool("conntrack-history", true, "persist read-only Linux conntrack NEW/UPDATE/DESTROY history")
	conntrackSegmentSize := flags.String("conntrack-segment-size", "64MiB", "maximum conntrack history segment size")
	conntrackMaxSize := flags.String("conntrack-max-size", "8GiB", "maximum rolling conntrack history storage")
	conntrackSocketBuffer := flags.String("conntrack-socket-buffer", "8MiB", "requested conntrack netlink receive buffer")
	healthIntervalText := flags.String("health-interval", "60s", "sensor-health reporting interval; 0 disables periodic reports")
	healthSegmentSize := flags.String("health-segment-size", "16MiB", "maximum persisted sensor-health segment size")
	healthMaxSize := flags.String("health-max-size", "1GiB", "maximum rolling persisted sensor-health storage")
	identityIntervalText := flags.String("identity-interval", "60s", "endpoint identity snapshot interval; 0 disables history")
	identitySegmentSize := flags.String("identity-segment-size", "16MiB", "maximum endpoint identity segment size")
	identityMaxSize := flags.String("identity-max-size", "1GiB", "maximum rolling endpoint identity storage")
	dhcpLeaseFile := flags.String("dhcp-lease-file", "/run/dnsmasq.lease", "dnsmasq lease source for endpoint identity history")
	dhcpHostsFile := flags.String("dhcp-hosts-file", "/run/dnsmasq.dns.conf.d/hosts.d/leases", "dnsmasq generated hosts source for endpoint identity history")
	observationQueue := flags.Int("observation-queue", 8192, "userspace observation queue depth")
	socketBufferText := flags.String("socket-buffer", "8MiB", "requested AF_PACKET receive buffer per interface")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("observe requires at least one --interface")
	}
	if *count < 0 {
		return fmt.Errorf("--count must be zero or greater")
	}
	if *observationQueue <= 0 {
		return fmt.Errorf("--observation-queue must be greater than zero")
	}

	socketBufferBytes64, err := storage.ParseBytes(*socketBufferText)
	if err != nil {
		return fmt.Errorf("parse --socket-buffer: %w", err)
	}
	maxInt := uint64(^uint(0) >> 1)
	if socketBufferBytes64 == 0 || socketBufferBytes64 > maxInt {
		return fmt.Errorf("--socket-buffer must fit in a positive int")
	}
	socketBufferBytes := int(socketBufferBytes64)

	healthInterval, err := time.ParseDuration(*healthIntervalText)
	if err != nil {
		return fmt.Errorf("parse --health-interval: %w", err)
	}
	if healthInterval < 0 {
		return fmt.Errorf("--health-interval must be zero or greater")
	}

	identityInterval, err := time.ParseDuration(*identityIntervalText)
	if err != nil {
		return fmt.Errorf("parse --identity-interval: %w", err)
	}
	if identityInterval < 0 {
		return fmt.Errorf("--identity-interval must be zero or greater")
	}
	if identityInterval > 0 && identityInterval < time.Second {
		return fmt.Errorf("--identity-interval must be zero or at least one second")
	}

	diagnostics := &lockedWriter{writer: os.Stderr}
	output, ring, err := observationOutput(
		*outputDir,
		*segmentSize,
		*maxSize,
		*minFree,
		*compression,
		diagnostics,
	)
	if err != nil {
		return err
	}

	identityRecorder, err := newIdentityRecorder(
		*outputDir,
		identityInterval,
		*dhcpLeaseFile,
		*dhcpHostsFile,
		*identitySegmentSize,
		*identityMaxSize,
		*minFree,
		*compression,
		diagnostics,
	)
	if err != nil {
		if ring != nil {
			_ = ring.Close()
		}
		return err
	}

	healthRecorder, err := newHealthRecorder(
		*outputDir,
		healthInterval,
		*healthSegmentSize,
		*healthMaxSize,
		*minFree,
		*compression,
		diagnostics,
	)
	if err != nil {
		if identityRecorder != nil {
			_ = identityRecorder.Close()
		}
		if ring != nil {
			_ = ring.Close()
		}
		return err
	}

	conntrackRecorder, err := newConntrackRecorder(
		*conntrackHistory,
		*outputDir,
		*conntrackSegmentSize,
		*conntrackMaxSize,
		*minFree,
		*compression,
		*conntrackSocketBuffer,
		diagnostics,
	)
	if err != nil {
		if healthRecorder != nil {
			_ = healthRecorder.Close()
		}
		if identityRecorder != nil {
			_ = identityRecorder.Close()
		}
		if ring != nil {
			_ = ring.Close()
		}
		return err
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	conntrackDone := make(chan error, 1)
	if conntrackRecorder != nil {
		go func() {
			runErr := conntrackRecorder.Run(ctx)
			conntrackDone <- runErr
			if runErr != nil {
				stop()
			}
		}()
	} else {
		conntrackDone <- nil
	}

	metrics := capture.NewMetrics([]string(names))
	stopHealth := startHealthReporter(
		healthInterval,
		metrics,
		ring,
		conntrackRecorder,
		identityRecorder,
		healthRecorder,
		diagnostics,
	)

	identityStop := make(chan struct{})
	identityDone := make(chan struct{})
	if identityRecorder != nil {
		go func() {
			defer close(identityDone)
			identityRecorder.Run(identityStop)
		}()
	} else {
		close(identityDone)
	}

	captureErr := capture.Observe(ctx, capture.Options{
		Count:             *count,
		Interfaces:        []string(names),
		Metrics:           metrics,
		ObservationQueue:  *observationQueue,
		Output:            output,
		SocketBufferBytes: socketBufferBytes,
	})

	stop()
	conntrackErr := <-conntrackDone

	if identityRecorder != nil {
		close(identityStop)
	}
	<-identityDone

	stopHealth()

	var closeErr error
	if ring != nil {
		closeErr = ring.Close()
	}

	var identityCloseErr error
	if identityRecorder != nil {
		identityCloseErr = identityRecorder.Close()
	}

	healthErr := writeHealthReport(
		diagnostics,
		metrics,
		ring,
		conntrackRecorder,
		identityRecorder,
		healthRecorder,
		healthInterval,
		true,
	)

	var healthCloseErr error
	if healthRecorder != nil {
		healthCloseErr = healthRecorder.Close()
	}

	var conntrackCloseErr error
	if conntrackRecorder != nil {
		conntrackCloseErr = conntrackRecorder.Close()
	}

	return errors.Join(
		captureErr,
		conntrackErr,
		closeErr,
		identityCloseErr,
		healthErr,
		healthCloseErr,
		conntrackCloseErr,
	)
}

func observationOutput(
	outputDir string,
	segmentSizeText string,
	maxSizeText string,
	minFreeText string,
	compression string,
	diagnostics io.Writer,
) (io.Writer, *storage.Ring, error) {
	if outputDir == "" {
		return os.Stdout, nil, nil
	}

	segmentBytes, err := storage.ParseBytes(segmentSizeText)
	if err != nil {
		return nil, nil, fmt.Errorf("parse --segment-size: %w", err)
	}

	maxBytes, err := storage.ParseBytes(maxSizeText)
	if err != nil {
		return nil, nil, fmt.Errorf("parse --max-size: %w", err)
	}

	minFreeBytes, err := storage.ParseBytes(minFreeText)
	if err != nil {
		return nil, nil, fmt.Errorf("parse --min-free: %w", err)
	}

	ring, err := storage.NewRing(storage.RingOptions{
		Compression:  compression,
		Directory:    outputDir,
		MaxBytes:     maxBytes,
		MinFreeBytes: minFreeBytes,
		OnCompressionError: func(err error) {
			fmt.Fprintf(diagnostics, "storage compression warning: %v\n", err)
		},
		SegmentBytes: segmentBytes,
	})
	if err != nil {
		return nil, nil, err
	}

	return ring, ring, nil
}

func printInterfaces() error {
	discovered, err := interfaces.Discover()
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(discovered); err != nil {
		return fmt.Errorf("encode interfaces: %w", err)
	}

	return nil
}

func printVersion() {
	fmt.Printf("%s\n", name)
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("OS:      %s\n", runtime.GOOS)
	fmt.Printf("Arch:    %s\n", runtime.GOARCH)
}
