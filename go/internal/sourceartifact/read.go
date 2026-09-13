package sourceartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadAvailable reads one committed AVAILABLE SourceArtifact and proves that the
// object still matches the database-bound storage reference, byte length, and
// SHA-256 before returning bytes to a semantic processor.
func ReadAvailable(artifactRoot string, record Record) ([]byte, error) {
	if record.AvailabilityState != AvailabilityAvailable {
		return nil, fmt.Errorf("SourceArtifact availability_state=%s is not readable for processing", record.AvailabilityState)
	}

	expectedReference := ExpectedStorageReference(record.SHA256)
	if expectedReference == "" {
		return nil, fmt.Errorf("SourceArtifact has invalid expected SHA-256")
	}

	if record.StorageReference != expectedReference {
		return nil, fmt.Errorf("SourceArtifact storage_reference=%s does not match expected reference=%s", record.StorageReference, expectedReference)
	}

	if record.ByteLength < 0 {
		return nil, fmt.Errorf("SourceArtifact byte_length must not be negative")
	}

	objectPath := filepath.Join(artifactRoot, filepath.FromSlash(record.StorageReference))

	info, err := os.Lstat(objectPath)
	if err != nil {
		return nil, fmt.Errorf("inspect SourceArtifact object: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("SourceArtifact object is not a regular file")
	}

	file, err := os.Open(objectPath)
	if err != nil {
		return nil, fmt.Errorf("open SourceArtifact object: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, record.ByteLength+1))
	if err != nil {
		return nil, fmt.Errorf("read SourceArtifact object: %w", err)
	}

	if int64(len(content)) != record.ByteLength {
		return nil, fmt.Errorf("SourceArtifact byte_length mismatch: expected=%d actual=%d", record.ByteLength, len(content))
	}

	digest := sha256.Sum256(content)
	actualSHA256 := hex.EncodeToString(digest[:])
	if actualSHA256 != record.SHA256 {
		return nil, fmt.Errorf("SourceArtifact sha256 mismatch: expected=%s actual=%s", record.SHA256, actualSHA256)
	}

	return content, nil
}
