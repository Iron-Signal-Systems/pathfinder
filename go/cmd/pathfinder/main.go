package main

import (
	"fmt"
	"log"
	"os"
)

const version = "0.0.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "migrate":
		err = runMigrate(os.Args[2:])

	case "serve":
		err = runServe(os.Args[2:])

	case "validate":
		err = runValidate(os.Args[2:])

	case "version":
		fmt.Printf("Pathfinder %s\n", version)
		return

	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		log.Printf("ERROR: %v", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(
		os.Stderr,
		"usage: pathfinder <migrate|serve|validate|version>",
	)
}
