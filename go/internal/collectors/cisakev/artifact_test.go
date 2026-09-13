package cisakev

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

func TestParseSourceArtifactRejectsChangedObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := []byte(`{"vulnerabilities":[{"cveID":"CVE-2026-1234"}]}`)
	changed := []byte(`{"vulnerabilities":[{"cveID":"CVE-2026-9999"}]}`)
	sum := sha256.Sum256(original)
	digest := hex.EncodeToString(sum[:])
	reference := sourceartifact.ExpectedStorageReference(digest)
	path := filepath.Join(root, filepath.FromSlash(reference))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	record := sourceartifact.Record{AvailabilityState: sourceartifact.AvailabilityAvailable, ByteLength: int64(len(original)), SHA256: digest, StorageReference: reference}
	_, err := ParseSourceArtifact(root, record)
	if err == nil {
		t.Fatal("ParseSourceArtifact() error = nil, want integrity rejection")
	}

	var readErr *SourceArtifactReadError
	if !errors.As(err, &readErr) {
		t.Fatalf("error type = %T, want *SourceArtifactReadError", err)
	}

	var parseErr *ArtifactParseError
	if errors.As(err, &parseErr) {
		t.Fatal("integrity/read failure was incorrectly classified as parse failure")
	}
}

func TestParseSourceArtifactRequiresVerifiedPreservedObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := []byte(`{"catalogVersion":"2026.09.13","vulnerabilities":[{"cveID":"CVE-2026-1234"}]}`)
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	reference := sourceartifact.ExpectedStorageReference(digest)
	path := filepath.Join(root, filepath.FromSlash(reference))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	record := sourceartifact.Record{AvailabilityState: sourceartifact.AvailabilityAvailable, ByteLength: int64(len(content)), SHA256: digest, StorageReference: reference}
	catalog, err := ParseSourceArtifact(root, record)
	if err != nil {
		t.Fatalf("ParseSourceArtifact() error = %v", err)
	}
	if len(catalog.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(catalog.Records))
	}
	if catalog.Records[0].CVEIdentifier != "CVE-2026-1234" {
		t.Fatalf("CVEIdentifier = %q", catalog.Records[0].CVEIdentifier)
	}
}

func TestParseSourceArtifactClassifiesMalformedVerifiedBytesAsParseFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := []byte(`{"catalogVersion":"broken","vulnerabilities":[`)
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	reference := sourceartifact.ExpectedStorageReference(digest)
	path := filepath.Join(root, filepath.FromSlash(reference))

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	record := sourceartifact.Record{
		AvailabilityState: sourceartifact.AvailabilityAvailable,
		ByteLength:        int64(len(content)),
		SHA256:            digest,
		StorageReference:  reference,
	}

	_, err := ParseSourceArtifact(root, record)
	if err == nil {
		t.Fatal("ParseSourceArtifact() error = nil, want parse failure")
	}

	var parseErr *ArtifactParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want *ArtifactParseError", err)
	}

	var readErr *SourceArtifactReadError
	if errors.As(err, &readErr) {
		t.Fatal("parse failure was incorrectly classified as read failure")
	}
}
