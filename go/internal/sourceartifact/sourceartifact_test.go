package sourceartifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
)

func TestCommitPreservesBeforeDatabaseCommit(t *testing.T) {
	request := testRequest(t)
	var order []string

	preserve := func(
		context.Context,
		string,
		io.Reader,
	) (artifactpreserve.Receipt, error) {
		order = append(order, "preserve")
		return testReceipt(), nil
	}

	commitDatabase := func(
		context.Context,
		CommitRequest,
		artifactpreserve.Receipt,
	) (DatabaseCommitResult, error) {
		order = append(order, "database")
		return DatabaseCommitResult{
			SourceArtifactID: request.SourceArtifactID,
		}, nil
	}

	result, err := preserveAndCommit(
		context.Background(),
		"/tmp/test.sock",
		request,
		bytes.NewBufferString("artifact"),
		func(ctx context.Context, socket string, reader io.Reader) (artifactpreserve.Receipt, error) {
			return preserve(ctx, socket, reader)
		},
		commitDatabase,
	)
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if strings.Join(order, ",") != "preserve,database" {
		t.Fatalf("unexpected operation order: %v", order)
	}

	if result.DatabaseState != DatabaseCommitConfirmed {
		t.Fatalf("unexpected database state: %s", result.DatabaseState)
	}
}

func TestCommitStopsWhenPreservationFails(t *testing.T) {
	request := testRequest(t)
	databaseCalled := false

	_, err := preserveAndCommit(
		context.Background(),
		"/tmp/test.sock",
		request,
		bytes.NewBufferString("artifact"),
		func(context.Context, string, io.Reader) (artifactpreserve.Receipt, error) {
			return artifactpreserve.Receipt{}, errors.New("preserve failed")
		},
		func(context.Context, CommitRequest, artifactpreserve.Receipt) (DatabaseCommitResult, error) {
			databaseCalled = true
			return DatabaseCommitResult{}, nil
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if databaseCalled {
		t.Fatal("database commit ran after preservation failure")
	}
}

func TestCommitReportsPreservedButUnprovenDatabaseCommit(t *testing.T) {
	request := testRequest(t)
	receipt := testReceipt()

	_, err := preserveAndCommit(
		context.Background(),
		"/tmp/test.sock",
		request,
		bytes.NewBufferString("artifact"),
		func(context.Context, string, io.Reader) (artifactpreserve.Receipt, error) {
			return receipt, nil
		},
		func(context.Context, CommitRequest, artifactpreserve.Receipt) (DatabaseCommitResult, error) {
			return DatabaseCommitResult{}, errors.New("database unavailable")
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	var unproven *UnprovenCommitError
	if !errors.As(err, &unproven) {
		t.Fatalf("expected UnprovenCommitError, got %T: %v", err, err)
	}

	if unproven.Receipt.SHA256 != receipt.SHA256 {
		t.Fatalf("receipt was not retained: %+v", unproven.Receipt)
	}
}

func TestCommitRecognizesAlreadyConfirmedRetry(t *testing.T) {
	request := testRequest(t)

	result, err := preserveAndCommit(
		context.Background(),
		"/tmp/test.sock",
		request,
		bytes.NewBufferString("artifact"),
		func(context.Context, string, io.Reader) (artifactpreserve.Receipt, error) {
			return testReceipt(), nil
		},
		func(context.Context, CommitRequest, artifactpreserve.Receipt) (DatabaseCommitResult, error) {
			return DatabaseCommitResult{
				AlreadyCommitted: true,
				SourceArtifactID: request.SourceArtifactID,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if result.DatabaseState != DatabaseCommitAlreadyConfirmed {
		t.Fatalf("unexpected database state: %s", result.DatabaseState)
	}
}

func TestNewUUIDv7(t *testing.T) {
	value, err := NewUUIDv7(time.UnixMilli(1_788_000_000_123))
	if err != nil {
		t.Fatalf("NewUUIDv7 returned error: %v", err)
	}

	if err := validateUUID(value, true); err != nil {
		t.Fatalf("generated UUID is not UUIDv7: %s: %v", value, err)
	}
}

func TestReconcileDetectsOrphanWithoutCollapsingSharedObject(t *testing.T) {
	root := t.TempDir()
	objectsDir := filepath.Join(root, "objects")
	if err := os.MkdirAll(objectsDir, 0750); err != nil {
		t.Fatal(err)
	}

	shared := []byte("shared bytes")
	sharedDigest := writeObject(t, objectsDir, shared)
	orphanDigest := writeObject(t, objectsDir, []byte("orphan bytes"))

	sharedReference := ExpectedStorageReference(sharedDigest)
	records := []Record{
		{
			AvailabilityState: AvailabilityAvailable,
			ByteLength:        int64(len(shared)),
			SHA256:            sharedDigest,
			SourceArtifactID:  "01900000-0000-7000-8000-000000000001",
			StorageReference:  sharedReference,
		},
		{
			AvailabilityState: AvailabilityAvailable,
			ByteLength:        int64(len(shared)),
			SHA256:            sharedDigest,
			SourceArtifactID:  "01900000-0000-7000-8000-000000000002",
			StorageReference:  sharedReference,
		},
	}

	findings, err := Reconcile(objectsDir, records)
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %+v", findings)
	}

	if findings[0].Kind != FindingOrphanObject || findings[0].ActualSHA256 != orphanDigest {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestReconcileDetectsMissingAndIntegrityMismatch(t *testing.T) {
	root := t.TempDir()
	objectsDir := filepath.Join(root, "objects")
	if err := os.MkdirAll(objectsDir, 0750); err != nil {
		t.Fatal(err)
	}

	actual := []byte("actual")
	digest := writeObject(t, objectsDir, actual)

	records := []Record{
		{
			AvailabilityState: AvailabilityAvailable,
			ByteLength:        99,
			SHA256:            digest,
			SourceArtifactID:  "01900000-0000-7000-8000-000000000003",
			StorageReference:  ExpectedStorageReference(digest),
		},
		{
			AvailabilityState: AvailabilityAvailable,
			ByteLength:        1,
			SHA256:            strings.Repeat("a", 64),
			SourceArtifactID:  "01900000-0000-7000-8000-000000000004",
			StorageReference:  ExpectedStorageReference(strings.Repeat("a", 64)),
		},
	}

	findings, err := Reconcile(objectsDir, records)
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	kinds := make(map[string]bool)
	for _, finding := range findings {
		kinds[finding.Kind] = true
	}

	if !kinds[FindingObjectIntegrityMismatch] || !kinds[FindingMissingObject] {
		t.Fatalf("expected integrity mismatch and missing object, got %+v", findings)
	}
}

func testReceipt() artifactpreserve.Receipt {
	return artifactpreserve.Receipt{
		ByteLength:       8,
		Reused:           false,
		SHA256:           strings.Repeat("b", 64),
		StorageReference: "objects/sha256/bb/" + strings.Repeat("b", 64),
	}
}

func testRequest(t *testing.T) CommitRequest {
	t.Helper()

	id, err := NewUUIDv7(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	return CommitRequest{
		ContentEncoding:    "IDENTITY",
		HandlingProfile:    "DEFAULT",
		MediaType:          "application/octet-stream",
		RetrievalEventID:   "01900000-0000-7000-8000-000000000010",
		SourceArtifactID:   id,
		SourceCollectionID: "01900000-0000-7000-8000-000000000011",
		SourceID:           "01900000-0000-7000-8000-000000000012",
		SourceLocation:     "test://artifact",
	}
}

func writeObject(t *testing.T, objectsDir string, content []byte) string {
	t.Helper()

	hash := sha256.Sum256(content)
	digest := hex.EncodeToString(hash[:])
	path := filepath.Join(objectsDir, "sha256", digest[:2], digest)

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0440); err != nil {
		t.Fatal(err)
	}

	return digest
}
