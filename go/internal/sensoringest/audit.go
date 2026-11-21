package sensoringest

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Audit struct {
	Stream Stream `json:"stream"`
	File string `json:"file"`
	SHA256 string `json:"file_sha256"`
	Total uint64 `json:"total"`
	WithinWindow uint64 `json:"within_window"`
	FirstAt time.Time `json:"first_at"`
	LastAt time.Time `json:"last_at"`
}

// AuditFile makes a bounded read-only pass over a frozen source segment.
// The SHA-256 covers exact stored bytes, including gzip if present.
// A rejected segment never produces an acknowledgement or checkpoint.
func AuditFile(path string, stream Stream, expectedSensorID string, since, until time.Time) (Audit, error) {
	result := Audit{Stream: stream, File: filepath.Base(path)}
	f, err := os.Open(path)
	if err != nil { return result, err }
	defer f.Close()
	info, err := f.Stat()
	if err != nil { return result, err }
	if !info.Mode().IsRegular() { return result, fmt.Errorf("not regular file") }
	if strings.HasSuffix(path, ".jsonl.active") && info.Size() > 0 {
		if _, err := f.Seek(-1, io.SeekEnd); err != nil { return result, err }
		b := make([]byte, 1)
		if _, err := io.ReadFull(f, b); err != nil { return result, err }
		if b[0] != '
' { return result, fmt.Errorf("active snapshot has incomplete trailing line") }
		if _, err := f.Seek(0, io.SeekStart); err != nil { return result, err }
	}
	h := sha256.New()
	var input io.Reader = io.TeeReader(f, h)
	if strings.HasSuffix(path, ".jsonl.gz") {
		gz, err := gzip.NewReader(input)
		if err != nil { return result, fmt.Errorf("gzip header: %w", err) }
		defer gz.Close()
		input = gz
	} else if !strings.HasSuffix(path, ".jsonl.active") {
		return result, fmt.Errorf("unsupported segment filename")
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), MaxRecordBytes+2)
	for scanner.Scan() {
		record, err := Validate(stream, scanner.Bytes(), expectedSensorID)
		if err != nil { return result, fmt.Errorf("record %d: %w", result.Total+1, err) }
		result.Total++
		if result.FirstAt.IsZero() || record.ObservedAt.Before(result.FirstAt) { result.FirstAt = record.ObservedAt }
		if result.LastAt.IsZero() || record.ObservedAt.After(result.LastAt) { result.LastAt = record.ObservedAt }
		if (since.IsZero() || !record.ObservedAt.Before(since)) &&
			(until.IsZero() || record.ObservedAt.Before(until)) { result.WithinWindow++ }
	}
	if err := scanner.Err(); err != nil { return result, fmt.Errorf("read segment: %w", err) }
	if result.Total == 0 { return result, fmt.Errorf("empty segment") }
	result.SHA256 = hex.EncodeToString(h.Sum(nil))
	return result, nil
}
