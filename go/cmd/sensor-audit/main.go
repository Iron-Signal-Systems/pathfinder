// sensor-audit checks copied UDM-Pro files read-only. No DB writes or acknowledgements.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func main() {
	root := flag.String("root", "", "frozen sensor capture directory (read-only)")
	expected := flag.String("expected-sensor-id", "", "optional packet sensor_id consistency check (not authentication)")
	since := flag.String("since", "", "inclusive RFC3339 timestamp")
	until := flag.String("until", "", "exclusive RFC3339 timestamp")
	flag.Parse()
	if *root == "" { fmt.Fprintln(os.Stderr, "-root required"); os.Exit(2) }
	start, err := parseTime(*since)
	if err != nil { fatal(err) }
	end, err := parseTime(*until)
	if err != nil { fatal(err) }
	if !start.IsZero() && !end.IsZero() && !start.Before(end) { fatal(fmt.Errorf("since must precede until")) }
	var paths []string
	err = filepath.WalkDir(*root, func(path string, d os.DirEntry, err error) error {
		if err != nil { return err }
		if d.Type().IsRegular() && (strings.HasSuffix(path, ".jsonl.gz") || strings.HasSuffix(path, ".jsonl.active")) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil { fatal(err) }
	sort.Strings(paths)
	if len(paths) == 0 { fatal(fmt.Errorf("no sensor segments found")) }
	var audits []sensoringest.Audit
	for _, path := range paths {
		stream, err := classify(*root, path)
		if err != nil { fatal(err) }
		audit, err := sensoringest.AuditFile(path, stream, *expected, start, end)
		if err != nil { fatal(fmt.Errorf("%s: %w", path, err)) }
		audits = append(audits, audit)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(audits); err != nil { fatal(err) }
}

func classify(root, path string) (sensoringest.Stream, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil { return "", err }
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 1 && strings.HasPrefix(parts[0], "observations-") {
		return sensoringest.Observations, nil
	}
	if len(parts) != 2 { return "", fmt.Errorf("unexpected segment location: %s", rel) }
	switch parts[0] {
	case "conntrack":
		if strings.HasPrefix(parts[1], "conntrack-") { return sensoringest.Conntrack, nil }
	case "identity":
		if strings.HasPrefix(parts[1], "endpoint-identity-") { return sensoringest.Identity, nil }
	case "health":
		if strings.HasPrefix(parts[1], "health-") { return sensoringest.Health, nil }
	}
	return "", fmt.Errorf("unexpected segment location: %s", rel)
}

func parseTime(s string) (time.Time, error) {
	if s == "" { return time.Time{}, nil }
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil { return time.Time{}, fmt.Errorf("invalid timestamp %q: %w", s, err) }
	return at, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}
