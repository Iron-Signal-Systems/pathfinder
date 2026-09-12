package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
)

func preserveSocket(args []string) (string, error) {
	flags := flag.NewFlagSet(
		"preserve",
		flag.ContinueOnError,
	)

	socket := flags.String(
		"socket",
		artifactpreserve.DefaultSocketPath,
		"artifact preservation Unix socket",
	)

	if err := flags.Parse(args); err != nil {
		return "", err
	}

	if flags.NArg() != 0 {
		return "", fmt.Errorf(
			"unexpected arguments",
		)
	}

	return *socket, nil
}

func runPreserve(args []string) error {
	socket, err := preserveSocket(args)
	if err != nil {
		return err
	}

	receipt, err := artifactpreserve.Preserve(
		context.Background(),
		socket,
		os.Stdin,
	)
	if err != nil {
		return err
	}

	fmt.Println(
		"Pathfinder artifact preservation: PASS",
	)
	fmt.Printf(
		"  sha256=%s\n",
		receipt.SHA256,
	)
	fmt.Printf(
		"  byte_length=%d\n",
		receipt.ByteLength,
	)
	fmt.Printf(
		"  storage_reference=%s\n",
		receipt.StorageReference,
	)
	fmt.Printf(
		"  reused=%t\n",
		receipt.Reused,
	)

	return nil
}
