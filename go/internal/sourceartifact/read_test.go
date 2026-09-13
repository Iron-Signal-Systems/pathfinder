package sourceartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAvailableRejectsByteLengthMismatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	record := writeTestArtifact(t, root, []byte("authoritative bytes"))
	record.ByteLength++
	if _, err := ReadAvailable(root, record); err == nil {
		t.Fatal("ReadAvailable() error = nil, want byte length mismatch")
	}
}

func TestReadAvailableRejectsIntegrityMismatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := []byte("authoritative bytes")
	record := writeTestArtifact(t, root, content)
	record.SHA256 = strings.Repeat("0", 64)
	record.StorageReference = ExpectedStorageReference(record.SHA256)
	path := filepath.Join(root, filepath.FromSlash(record.StorageReference))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAvailable(root, record); err == nil {
		t.Fatal("ReadAvailable() error = nil, want SHA-256 mismatch")
	}
}

func TestReadAvailableRejectsNonAvailableArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	record := writeTestArtifact(t, root, []byte("authoritative bytes"))
	record.AvailabilityState = AvailabilityQuarantined
	if _, err := ReadAvailable(root, record); err == nil {
		t.Fatal("ReadAvailable() error = nil, want availability rejection")
	}
}

func TestReadAvailableRejectsStorageReferenceMismatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	record := writeTestArtifact(t, root, []byte("authoritative bytes"))
	record.StorageReference = "objects/sha256/00/" + strings.Repeat("0", 64)
	if _, err := ReadAvailable(root, record); err == nil {
		t.Fatal("ReadAvailable() error = nil, want storage reference mismatch")
	}
}

func TestReadAvailableRejectsSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := []byte("authoritative bytes")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	reference := ExpectedStorageReference(digest)
	path := filepath.Join(root, filepath.FromSlash(reference))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	record := Record{AvailabilityState: AvailabilityAvailable, ByteLength: int64(len(content)), SHA256: digest, StorageReference: reference}
	if _, err := ReadAvailable(root, record); err == nil {
		t.Fatal("ReadAvailable() error = nil, want symlink rejection")
	}
}

func TestReadAvailableReturnsExactBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := []byte("{\"vulnerabilities\":[{\"cveID\":\"CVE-2026-1234\"}]}")
	record := writeTestArtifact(t, root, content)
	got, err := ReadAvailable(root, record)
	if err != nil {
		t.Fatalf("ReadAvailable() error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("ReadAvailable() = %q, want exact bytes %q", got, content)
	}
}

func writeTestArtifact(t *testing.T, root string, content []byte) Record {
	t.Helper()
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	reference := ExpectedStorageReference(digest)
	path := filepath.Join(root, filepath.FromSlash(reference))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	return Record{AvailabilityState: AvailabilityAvailable, ByteLength: int64(len(content)), SHA256: digest, StorageReference: reference}
}
