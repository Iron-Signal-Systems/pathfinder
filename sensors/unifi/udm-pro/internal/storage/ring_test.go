package storage

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRingCompressesCompletedSegment(t *testing.T) {
	dir := t.TempDir()

	ring, err := NewRing(RingOptions{
		Compression:  CompressionGzip,
		Directory:    dir,
		MaxBytes:     4096,
		MinFreeBytes: 0,
		SegmentBytes: 100,
	})
	if err != nil {
		t.Fatalf("NewRing() error = %v", err)
	}

	first := strings.Repeat("a", 59) + "\n"
	second := strings.Repeat("b", 59) + "\n"

	if _, err := ring.Write([]byte(first)); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	if _, err := ring.Write([]byte(second)); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var compressedPath string

	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), completedSuffix+compressedSuffix) {
				compressedPath = filepath.Join(dir, entry.Name())
				break
			}
		}

		if compressedPath != "" {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if compressedPath == "" {
		t.Fatal("completed segment was not compressed")
	}

	file, err := os.Open(compressedPath)
	if err != nil {
		t.Fatalf("Open() compressed segment error = %v", err)
	}
	defer file.Close()

	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(data) != first {
		t.Fatalf("compressed content = %q, want %q", string(data), first)
	}

	if err := ring.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestRingRecoversOnlyCompleteActiveRecords(t *testing.T) {
	dir := t.TempDir()

	active := filepath.Join(
		dir,
		"observations-20260913T120000.000000000Z.jsonl.active",
	)
	if err := os.WriteFile(
		active,
		[]byte("{\"complete\":true}\n{\"partial\":"),
		0o640,
	); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ring, err := NewRing(RingOptions{
		Directory:    dir,
		MaxBytes:     1024,
		MinFreeBytes: 0,
		SegmentBytes: 512,
	})
	if err != nil {
		t.Fatalf("NewRing() error = %v", err)
	}
	defer ring.Close()

	recovered := strings.TrimSuffix(active, activeSuffix) + recoveredSuffix

	data, err := os.ReadFile(recovered)
	if err != nil {
		t.Fatalf("ReadFile() recovered segment error = %v", err)
	}

	if string(data) != "{\"complete\":true}\n" {
		t.Fatalf("recovered data = %q", string(data))
	}
}

func TestRingRotatesWithoutSplittingRecords(t *testing.T) {
	dir := t.TempDir()

	ring, err := NewRing(RingOptions{
		Directory:    dir,
		MaxBytes:     220,
		MinFreeBytes: 0,
		SegmentBytes: 100,
	})
	if err != nil {
		t.Fatalf("NewRing() error = %v", err)
	}

	records := []string{
		strings.Repeat("a", 59) + "\n",
		strings.Repeat("b", 59) + "\n",
		strings.Repeat("c", 59) + "\n",
	}

	for _, record := range records {
		if _, err := ring.Write([]byte(record)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := ring.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var completed int
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), completedSuffix) {
			completed++

			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			if len(data) == 0 || data[len(data)-1] != '\n' {
				t.Fatalf("segment %s does not end on a record boundary", entry.Name())
			}
		}
	}

	if completed == 0 {
		t.Fatal("no completed segments created")
	}
}

func TestRingStats(t *testing.T) {
	dir := t.TempDir()

	ring, err := NewRing(RingOptions{
		Compression:  CompressionNone,
		Directory:    dir,
		MaxBytes:     1024,
		MinFreeBytes: 0,
		SegmentBytes: 128,
	})
	if err != nil {
		t.Fatalf("NewRing() error = %v", err)
	}

	if _, err := ring.Write([]byte("{\"test\":true}\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	stats := ring.Stats()
	if stats.ActiveBytes == 0 {
		t.Fatal("active bytes = 0, want greater than zero")
	}
	if stats.CompressionMode != CompressionNone {
		t.Fatalf("compression mode = %q, want %q", stats.CompressionMode, CompressionNone)
	}

	if err := ring.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	stats = ring.Stats()
	if stats.ActiveBytes != 0 {
		t.Fatalf("active bytes after close = %d, want 0", stats.ActiveBytes)
	}
	if stats.CompletedSegments != 1 {
		t.Fatalf("completed segments = %d, want 1", stats.CompletedSegments)
	}
}

func TestRingUsesConfiguredPrefix(t *testing.T) {
	dir := t.TempDir()

	ring, err := NewRing(RingOptions{
		Compression:  CompressionNone,
		Directory:    dir,
		MaxBytes:     1024 * 1024,
		Prefix:       "endpoint-identity",
		SegmentBytes: 64 * 1024,
	})
	if err != nil {
		t.Fatalf("NewRing() error = %v", err)
	}

	if _, err := ring.Write([]byte("{\"test\":true}\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := ring.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "endpoint-identity-") &&
			strings.HasSuffix(entry.Name(), ".jsonl") {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected completed segment using endpoint-identity prefix")
	}
}
