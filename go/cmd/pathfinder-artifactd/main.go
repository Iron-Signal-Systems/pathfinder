package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
)

const (
	defaultMaxBytes = int64(268435456)
	version         = "0.0.0-dev"
)

type options struct {
	MaxBytes   int64
	ObjectsDir string
	SocketPath string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("ERROR: %v", err)
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet(
		"pathfinder-artifactd",
		flag.ContinueOnError,
	)

	maxBytes := flags.Int64(
		"max-bytes",
		defaultMaxBytes,
		"maximum accepted artifact byte count",
	)

	objectsDir := flags.String(
		"objects",
		"/var/db/pathfinder/artifacts/objects",
		"committed artifact object directory",
	)

	socketPath := flags.String(
		"socket",
		"/var/run/pathfinder-artifact/preserve.sock",
		"Unix socket path",
	)

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	if flags.NArg() != 0 {
		return options{}, fmt.Errorf(
			"unexpected arguments",
		)
	}

	return options{
		MaxBytes:   *maxBytes,
		ObjectsDir: *objectsDir,
		SocketPath: *socketPath,
	}, nil
}

func run(args []string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf(
			"pathfinder-artifactd refuses to run as root",
		)
	}

	options, err := parseOptions(args)
	if err != nil {
		return err
	}

	syscall.Umask(0027)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	log.Printf(
		"Pathfinder artifact preservation %s starting; socket=%s max_bytes=%d",
		version,
		options.SocketPath,
		options.MaxBytes,
	)

	server := artifactpreserve.Server{
		MaxBytes:   options.MaxBytes,
		ObjectsDir: options.ObjectsDir,
		SocketPath: options.SocketPath,
	}

	if err := server.Serve(ctx); err != nil {
		return err
	}

	log.Printf(
		"Pathfinder artifact preservation shutdown complete",
	)

	return nil
}
