package segmentread

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListIncludesActiveAndExcludesCompressionWork(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{
		"observations-001.jsonl.gz",
		"observations-002.jsonl.active",
		"observations-003.jsonl",
		"observations-004.jsonl.gz.active",
	} {
		if err := os.WriteFile(
			filepath.Join(dir, name),
			[]byte("{}\n"),
			0o640,
		); err != nil {
			t.Fatal(err)
		}
	}

	got, err := List(dir, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("List() count = %d, want 3: %v", len(got), got)
	}

	for _, path := range got {
		if strings.HasSuffix(path, ".gz.active") {
			t.Fatalf("compression work file returned: %s", path)
		}
	}
}

func TestListDeduplicatesRawAndCompressed(t *testing.T) {
	dir := t.TempDir()

	raw := filepath.Join(dir, "observations-001.jsonl")
	compressed := filepath.Join(dir, "observations-001.jsonl.gz")

	if err := os.WriteFile(raw, []byte("{}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(compressed, []byte("not-real-gzip"), 0o640); err != nil {
		t.Fatal(err)
	}

	got, err := List(dir, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("List() count = %d, want 1", len(got))
	}
	if got[0] != compressed {
		t.Fatalf("List() selected %q, want %q", got[0], compressed)
	}
}

func TestReadLinesActiveIgnoresUnfinishedTrailingFragment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observations-001.jsonl.active")

	if err := os.WriteFile(
		path,
		[]byte("{\"id\":1}\n{\"id\":2}\n{\"id\":"),
		0o640,
	); err != nil {
		t.Fatal(err)
	}

	var lines []string
	stats, err := ReadLines(
		path,
		DefaultMaxTokenBytes,
		func(line []byte) error {
			lines = append(lines, string(line))
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ReadLines() error = %v", err)
	}

	if !stats.Active {
		t.Fatal("Active = false, want true")
	}
	if !stats.TrailingFragmentIgnored {
		t.Fatal("TrailingFragmentIgnored = false, want true")
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2: %v", len(lines), lines)
	}
}

func TestReadLinesActiveConsumesFinalCommittedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observations-001.jsonl.active")

	if err := os.WriteFile(
		path,
		[]byte("{\"id\":1}\n{\"id\":2}\n"),
		0o640,
	); err != nil {
		t.Fatal(err)
	}

	var lines []string
	stats, err := ReadLines(
		path,
		DefaultMaxTokenBytes,
		func(line []byte) error {
			lines = append(lines, string(line))
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ReadLines() error = %v", err)
	}

	if stats.TrailingFragmentIgnored {
		t.Fatal("TrailingFragmentIgnored = true, want false")
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2: %v", len(lines), lines)
	}
}

func TestReadLinesCompletedDoesNotHideMalformedFinalLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observations-001.jsonl")

	if err := os.WriteFile(
		path,
		[]byte("{\"id\":1}\nnot-json"),
		0o640,
	); err != nil {
		t.Fatal(err)
	}

	_, err := ReadLines(
		path,
		DefaultMaxTokenBytes,
		func(line []byte) error {
			if !strings.HasPrefix(string(line), "{") {
				return os.ErrInvalid
			}
			return nil
		},
	)
	if err == nil {
		t.Fatal("ReadLines() error = nil, want error")
	}
}

func TestReadLinesResolvesActiveAfterRotation(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "observations-001.jsonl.active")
	final := filepath.Join(dir, "observations-001.jsonl")

	if err := os.WriteFile(active, []byte("{\"id\":1}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(active, final); err != nil {
		t.Fatal(err)
	}

	var lines []string
	stats, err := ReadLines(
		active,
		DefaultMaxTokenBytes,
		func(line []byte) error {
			lines = append(lines, string(line))
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ReadLines() error = %v", err)
	}

	if stats.Active {
		t.Fatal("Active = true, want false after resolution")
	}
	if stats.ResolvedPath != final {
		t.Fatalf(
			"ResolvedPath = %q, want %q",
			stats.ResolvedPath,
			final,
		)
	}
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}
}
