// Package segmentread provides read-only snapshots of Pathfinder JSONL
// segments, including a segment that is actively being appended.
package segmentread

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultMaxTokenBytes = 4 * 1024 * 1024

	activeSuffix          = ".jsonl.active"
	compressedSuffix      = ".jsonl.gz"
	rawSuffix             = ".jsonl"
	recoveredCompressed   = ".jsonl.recovered.gz"
	recoveredRaw          = ".jsonl.recovered"
	compressionWorkSuffix = ".jsonl.gz.active"
)

// ReadStats describes how one listed segment was consumed.
type ReadStats struct {
	Active                  bool   `json:"active"`
	ResolvedPath            string `json:"resolved_path"`
	TrailingFragmentIgnored bool   `json:"trailing_fragment_ignored"`
	Vanished                bool   `json:"vanished"`
}

// List returns one authoritative readable representation per logical JSONL
// segment. Compression work files are never returned.
func List(directory string, prefix string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	type selected struct {
		path     string
		priority int
	}

	byLogicalName := make(map[string]selected)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasSuffix(name, compressionWorkSuffix) {
			continue
		}

		logicalName, priority, ok := classify(name)
		if !ok {
			continue
		}

		current, exists := byLogicalName[logicalName]
		if exists && current.priority >= priority {
			continue
		}

		byLogicalName[logicalName] = selected{
			path:     filepath.Join(directory, name),
			priority: priority,
		}
	}

	paths := make([]string, 0, len(byLogicalName))
	for _, current := range byLogicalName {
		paths = append(paths, current.path)
	}
	sort.Strings(paths)

	return paths, nil
}

// ReadLines reads a stable snapshot of one listed JSONL segment.
//
// For an active segment, only newline-terminated records present at the file
// size captured immediately after open are considered committed. A trailing
// non-newline fragment is ignored. Completed segments retain strict behavior:
// malformed completed records are returned to the caller as errors.
//
// If rotation or compression renames a listed path before it can be opened,
// the same logical segment is resolved under its finalized name. A segment
// removed by retention during the query is reported as Vanished rather than
// causing the whole live query to fail.
func ReadLines(
	listedPath string,
	maxTokenBytes int,
	consume func([]byte) error,
) (ReadStats, error) {
	if maxTokenBytes <= 0 {
		maxTokenBytes = DefaultMaxTokenBytes
	}

	resolvedPath, vanished, err := resolve(listedPath)
	if err != nil {
		return ReadStats{}, err
	}
	if vanished {
		return ReadStats{
			ResolvedPath: listedPath,
			Vanished:     true,
		}, nil
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReadStats{
				ResolvedPath: resolvedPath,
				Vanished:     true,
			}, nil
		}
		return ReadStats{}, err
	}
	defer file.Close()

	stats := ReadStats{
		Active:       strings.HasSuffix(resolvedPath, activeSuffix),
		ResolvedPath: resolvedPath,
	}

	var reader io.Reader = file
	var gzipReader *gzip.Reader

	if strings.HasSuffix(resolvedPath, ".gz") {
		gzipReader, err = gzip.NewReader(file)
		if err != nil {
			return stats, fmt.Errorf("open gzip stream: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	} else {
		info, statErr := file.Stat()
		if statErr != nil {
			return stats, statErr
		}

		snapshotSize := info.Size()
		reader = io.LimitReader(file, snapshotSize)

		if stats.Active && snapshotSize > 0 {
			lastByte := []byte{0}
			if _, readErr := file.ReadAt(lastByte, snapshotSize-1); readErr != nil {
				return stats, fmt.Errorf(
					"read active segment final byte: %w",
					readErr,
				)
			}

			if lastByte[0] != '\n' {
				stats.TrailingFragmentIgnored = true
			}
		}
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxTokenBytes)

	var pending []byte
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		current := append([]byte(nil), scanner.Bytes()...)

		if pending != nil {
			if err := consume(pending); err != nil {
				return stats, fmt.Errorf(
					"line %d: %w",
					lineNumber-1,
					err,
				)
			}
		}

		pending = current
	}

	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("scan JSONL: %w", err)
	}

	if pending != nil {
		if !(stats.Active && stats.TrailingFragmentIgnored) {
			if err := consume(pending); err != nil {
				return stats, fmt.Errorf(
					"line %d: %w",
					lineNumber,
					err,
				)
			}
		}
	}

	return stats, nil
}

func classify(name string) (string, int, bool) {
	switch {
	case strings.HasSuffix(name, recoveredCompressed):
		return strings.TrimSuffix(name, recoveredCompressed), 50, true

	case strings.HasSuffix(name, compressedSuffix):
		return strings.TrimSuffix(name, compressedSuffix), 40, true

	case strings.HasSuffix(name, recoveredRaw):
		return strings.TrimSuffix(name, recoveredRaw), 30, true

	case strings.HasSuffix(name, rawSuffix):
		return strings.TrimSuffix(name, rawSuffix), 20, true

	case strings.HasSuffix(name, activeSuffix):
		return strings.TrimSuffix(name, activeSuffix), 10, true
	}

	return "", 0, false
}

func resolve(listedPath string) (string, bool, error) {
	for _, candidate := range resolutionCandidates(listedPath) {
		_, err := os.Stat(candidate)
		switch {
		case err == nil:
			return candidate, false, nil
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return "", false, err
		}
	}

	return listedPath, true, nil
}

func resolutionCandidates(path string) []string {
	candidates := []string{path}

	switch {
	case strings.HasSuffix(path, activeSuffix):
		raw := strings.TrimSuffix(path, ".active")
		root := strings.TrimSuffix(raw, rawSuffix)

		candidates = append(
			candidates,
			raw,
			raw+".gz",
			root+recoveredRaw,
			root+recoveredCompressed,
		)

	case strings.HasSuffix(path, recoveredRaw):
		candidates = append(candidates, path+".gz")

	case strings.HasSuffix(path, rawSuffix):
		candidates = append(candidates, path+".gz")
	}

	return candidates
}
