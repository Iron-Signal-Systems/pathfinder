package sensoringest

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	MaxBatchBytes   = 8 << 20
	MaxBatchRecords = 25000
	custodyFormat   = "pathfinder-sensor-custody-v1"
)

var (
	ErrBatchConflict  = errors.New("sensor batch identity conflicts with existing custody")
	ErrCustodyCorrupt = errors.New("existing sensor custody record is corrupt")
	safeIdentity      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

// Batch is the original newline-terminated JSONL payload and its claimed
// envelope. The caller must authenticate and authorize SensorID before calling
// Preserve. This package does not implement that authentication.
type Batch struct {
	SensorID      string
	Stream        Stream
	SchemaVersion string
	BatchID       string
	Payload       []byte
}

type CustodyReceipt struct {
	SensorID         string `json:"sensor_id"`
	Stream           Stream `json:"stream"`
	BatchID          string `json:"batch_id"`
	SHA256           string `json:"sha256"`
	RecordCount      int    `json:"record_count"`
	AlreadyPreserved bool   `json:"already_preserved"`
}

type custodyEnvelope struct {
	Format        string    `json:"format"`
	SensorID      string    `json:"sensor_id"`
	Stream        Stream    `json:"stream"`
	SchemaVersion string    `json:"schema_version"`
	BatchID       string    `json:"batch_id"`
	SHA256        string    `json:"sha256"`
	RecordCount   int       `json:"record_count"`
	PreservedAt   time.Time `json:"preserved_at"`
	Payload       []byte    `json:"payload"`
}

type Custody struct{ directory string }

// OpenCustody requires a private, nonsymlink directory. Storage must support
// durable fsync and atomic hard links (same filesystem).
func OpenCustody(directory string) (*Custody, error) {
	if !filepath.IsAbs(directory) {
		return nil, fmt.Errorf("custody path must be absolute")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, fmt.Errorf("create custody directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("custody directory must be a private, nonsymlink directory")
	}
	return &Custody{directory: directory}, nil
}

// Preserve validates all records before creating an immutable custody file.
// An existing identical batch returns AlreadyPreserved; a different payload
// under the same identity fails closed. A receipt means file custody, not
// database indexing, Sighting derivation, or a delivery acknowledgement.
func (store *Custody) Preserve(batch Batch) (CustodyReceipt, error) {
	var receipt CustodyReceipt
	if store == nil {
		return receipt, fmt.Errorf("custody store not configured")
	}
	if !safeIdentity.MatchString(batch.SensorID) || !safeIdentity.MatchString(batch.BatchID) {
		return receipt, fmt.Errorf("invalid sensor or batch identity")
	}
	if len(batch.Payload) == 0 || len(batch.Payload) > MaxBatchBytes || batch.Payload[len(batch.Payload)-1] != '\n' {
		return receipt, fmt.Errorf("invalid batch size or missing terminating newline")
	}
	count, err := validateBatch(batch)
	if err != nil {
		return receipt, err
	}
	digest := sha256.Sum256(batch.Payload)
	payloadSHA := hex.EncodeToString(digest[:])
	receipt = CustodyReceipt{SensorID: batch.SensorID, Stream: batch.Stream,
		BatchID: batch.BatchID, SHA256: payloadSHA, RecordCount: count}

	key := sha256.Sum256([]byte(custodyFormat + "\x00" + batch.SensorID + "\x00" + string(batch.Stream) + "\x00" + batch.BatchID))
	destination := filepath.Join(store.directory, hex.EncodeToString(key[:])+".json")
	envelope := custodyEnvelope{Format: custodyFormat, SensorID: batch.SensorID,
		Stream: batch.Stream, SchemaVersion: batch.SchemaVersion, BatchID: batch.BatchID,
		SHA256: payloadSHA, RecordCount: count, PreservedAt: time.Now().UTC(), Payload: batch.Payload}

	data, err := json.Marshal(envelope)
	if err != nil {
		return receipt, err
	}
	f, err := os.CreateTemp(store.directory, ".pending-")
	if err != nil {
		return receipt, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return receipt, fmt.Errorf("custody write: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return receipt, fmt.Errorf("custody fsync: %w", err)
	}
	if err := f.Close(); err != nil {
		return receipt, err
	}

	if err := os.Link(f.Name(), destination); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return receipt, fmt.Errorf("custody link: %w", err)
		}
		if err := verifyExisting(destination, envelope); err != nil {
			return receipt, err
		}
		if err := syncDirectory(store.directory); err != nil {
			return receipt, err
		}
		receipt.AlreadyPreserved = true
		return receipt, nil
	}
	// The committed name is durable only after the directory is synced.
	if err := syncDirectory(store.directory); err != nil {
		return receipt, fmt.Errorf("custody directory fsync: %w", err)
	}
	if err := os.Remove(f.Name()); err != nil {
		return receipt, fmt.Errorf("custody temporary cleanup: %w", err)
	}
	if err := syncDirectory(store.directory); err != nil {
		return receipt, fmt.Errorf("custody cleanup fsync: %w", err)
	}
	return receipt, nil
}

func validateBatch(batch Batch) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(batch.Payload))
	scanner.Buffer(make([]byte, 64*1024), MaxRecordBytes+2)
	count := 0
	for scanner.Scan() {
		count++
		if count > MaxBatchRecords {
			return 0, fmt.Errorf("batch exceeds record limit")
		}
		record, err := Validate(batch.Stream, scanner.Bytes(), batch.SensorID)
		if err != nil {
			return 0, fmt.Errorf("batch record %d: %w", count, err)
		}
		if record.SchemaVersion != batch.SchemaVersion {
			return 0, fmt.Errorf("batch record %d: schema version mismatches envelope", count)
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan batch: %w", err)
	}
	if count == 0 {
		return 0, fmt.Errorf("empty batch")
	}
	return count, nil
}

func verifyExisting(path string, expected custodyEnvelope) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read existing custody: %w", err)
	}
	defer f.Close()
	existingData, err := io.ReadAll(io.LimitReader(f, MaxBatchBytes*2+4096))
	if err != nil {
		return fmt.Errorf("read existing custody: %w", err)
	}
	if len(existingData) >= MaxBatchBytes*2+4096 {
		return ErrCustodyCorrupt
	}
	var previous custodyEnvelope
	if err := json.Unmarshal(existingData, &previous); err != nil {
		return ErrCustodyCorrupt
	}
	actual := sha256.Sum256(previous.Payload)
	if previous.Format != custodyFormat || !strings.EqualFold(hex.EncodeToString(actual[:]), previous.SHA256) || previous.RecordCount <= 0 {
		return ErrCustodyCorrupt
	}
	if previous.SensorID != expected.SensorID || previous.Stream != expected.Stream || previous.BatchID != expected.BatchID || previous.SchemaVersion != expected.SchemaVersion {
		return ErrCustodyCorrupt
	}
	if previous.SHA256 != expected.SHA256 || previous.RecordCount != expected.RecordCount || !bytes.Equal(previous.Payload, expected.Payload) {
		return ErrBatchConflict
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
