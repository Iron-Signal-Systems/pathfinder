package sourceartifact

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
)

const (
	AvailabilityAvailable       = "AVAILABLE"
	AvailabilityQuarantined     = "QUARANTINED"
	AvailabilityUnavailable     = "UNAVAILABLE"
	AvailabilityDestroyedPolicy = "DESTROYED_BY_POLICY"

	DatabaseCommitConfirmed        = "CONFIRMED"
	DatabaseCommitAlreadyConfirmed = "ALREADY_CONFIRMED"

	FindingInvalidStorageReference = "INVALID_STORAGE_REFERENCE"
	FindingMissingObject           = "MISSING_OBJECT"
	FindingObjectIntegrityMismatch = "OBJECT_INTEGRITY_MISMATCH"
	FindingOrphanObject            = "ORPHAN_OBJECT"
	FindingUnexpectedObjectPath    = "UNEXPECTED_OBJECT_PATH"
)

type CommitDatabaseFunc func(
	context.Context,
	CommitRequest,
	artifactpreserve.Receipt,
) (DatabaseCommitResult, error)

type CommitRequest struct {
	ContentEncoding    string
	HandlingProfile    string
	MediaType          string
	RetrievalEventID   string
	SourceArtifactID   string
	SourceCollectionID string
	SourceID           string
	SourceLocation     string
}

type CommitResult struct {
	DatabaseState    string
	Receipt          artifactpreserve.Receipt
	SourceArtifactID string
}

type DatabaseCommitResult struct {
	AlreadyCommitted bool
	SourceArtifactID string
}

type Finding struct {
	ActualByteLength   int64
	ActualSHA256       string
	ExpectedByteLength int64
	ExpectedSHA256     string
	Kind               string
	SourceArtifactID   string
	StorageReference   string
}

type PreserveFunc func(
	context.Context,
	string,
	io.Reader,
) (artifactpreserve.Receipt, error)

type Record struct {
	AvailabilityState string
	ByteLength        int64
	SHA256            string
	SourceArtifactID  string
	StorageReference  string
}

type UnprovenCommitError struct {
	Cause   error
	Receipt artifactpreserve.Receipt
	Request CommitRequest
}

func (err *UnprovenCommitError) Error() string {
	return fmt.Sprintf(
		"artifact preserved but SourceArtifact database commit is not proven; source_artifact_id=%s sha256=%s byte_length=%d storage_reference=%s: %v",
		err.Request.SourceArtifactID,
		err.Receipt.SHA256,
		err.Receipt.ByteLength,
		err.Receipt.StorageReference,
		err.Cause,
	)
}

func (err *UnprovenCommitError) Unwrap() error {
	return err.Cause
}

func Commit(
	ctx context.Context,
	socketPath string,
	request CommitRequest,
	source io.Reader,
	commitDatabase CommitDatabaseFunc,
) (CommitResult, error) {
	return preserveAndCommit(
		ctx,
		socketPath,
		request,
		source,
		artifactpreserve.Preserve,
		commitDatabase,
	)
}

func ExpectedStorageReference(digest string) string {
	if len(digest) != 64 {
		return ""
	}

	return filepath.ToSlash(
		filepath.Join(
			"objects",
			"sha256",
			digest[:2],
			digest,
		),
	)
}

func NewUUIDv7(now time.Time) (string, error) {
	var value [16]byte

	milliseconds := uint64(now.UnixMilli())
	if milliseconds > 0x0000ffffffffffff {
		return "", fmt.Errorf("UUIDv7 timestamp out of range")
	}

	value[0] = byte(milliseconds >> 40)
	value[1] = byte(milliseconds >> 32)
	value[2] = byte(milliseconds >> 24)
	value[3] = byte(milliseconds >> 16)
	value[4] = byte(milliseconds >> 8)
	value[5] = byte(milliseconds)

	if _, err := rand.Read(value[6:]); err != nil {
		return "", fmt.Errorf("generate UUIDv7 randomness: %w", err)
	}

	value[6] = (value[6] & 0x0f) | 0x70
	value[8] = (value[8] & 0x3f) | 0x80

	return formatUUID(value), nil
}

func Reconcile(
	objectsDir string,
	records []Record,
) ([]Finding, error) {
	objects, scanFindings, err := scanObjects(objectsDir)
	if err != nil {
		return nil, err
	}

	findings := append([]Finding(nil), scanFindings...)
	referenced := make(map[string]bool)

	for _, record := range records {
		if record.StorageReference == "NOT_APPLICABLE" {
			continue
		}

		referenced[record.StorageReference] = true

		if record.AvailabilityState != AvailabilityAvailable &&
			record.AvailabilityState != AvailabilityQuarantined {
			continue
		}

		expectedReference := ExpectedStorageReference(record.SHA256)
		if expectedReference == "" || record.StorageReference != expectedReference {
			findings = append(findings, Finding{
				ExpectedByteLength: record.ByteLength,
				ExpectedSHA256:     record.SHA256,
				Kind:               FindingInvalidStorageReference,
				SourceArtifactID:   record.SourceArtifactID,
				StorageReference:   record.StorageReference,
			})
			continue
		}

		object, exists := objects[record.StorageReference]
		if !exists {
			findings = append(findings, Finding{
				ExpectedByteLength: record.ByteLength,
				ExpectedSHA256:     record.SHA256,
				Kind:               FindingMissingObject,
				SourceArtifactID:   record.SourceArtifactID,
				StorageReference:   record.StorageReference,
			})
			continue
		}

		if object.ByteLength != record.ByteLength || object.SHA256 != record.SHA256 {
			findings = append(findings, Finding{
				ActualByteLength:   object.ByteLength,
				ActualSHA256:       object.SHA256,
				ExpectedByteLength: record.ByteLength,
				ExpectedSHA256:     record.SHA256,
				Kind:               FindingObjectIntegrityMismatch,
				SourceArtifactID:   record.SourceArtifactID,
				StorageReference:   record.StorageReference,
			})
		}
	}

	for storageReference, object := range objects {
		if referenced[storageReference] {
			continue
		}

		findings = append(findings, Finding{
			ActualByteLength: object.ByteLength,
			ActualSHA256:     object.SHA256,
			Kind:             FindingOrphanObject,
			StorageReference: storageReference,
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		left := findings[i].Kind + "|" + findings[i].StorageReference + "|" + findings[i].SourceArtifactID
		right := findings[j].Kind + "|" + findings[j].StorageReference + "|" + findings[j].SourceArtifactID
		return left < right
	})

	return findings, nil
}

func ValidateRequest(request CommitRequest) error {
	if err := validateUUID(request.SourceArtifactID, true); err != nil {
		return fmt.Errorf("source_artifact_id: %w", err)
	}

	for name, value := range map[string]string{
		"retrieval_event_id":   request.RetrievalEventID,
		"source_collection_id": request.SourceCollectionID,
		"source_id":            request.SourceID,
	} {
		if err := validateUUID(value, false); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	for name, value := range map[string]string{
		"content_encoding": request.ContentEncoding,
		"handling_profile": request.HandlingProfile,
		"media_type":       request.MediaType,
		"source_location":  request.SourceLocation,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}

		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("%s contains NUL", name)
		}
	}

	return nil
}

type objectState struct {
	ByteLength int64
	SHA256     string
}

func formatUUID(value [16]byte) string {
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded)
}

func preserveAndCommit(
	ctx context.Context,
	socketPath string,
	request CommitRequest,
	source io.Reader,
	preserve PreserveFunc,
	commitDatabase CommitDatabaseFunc,
) (CommitResult, error) {
	if err := ValidateRequest(request); err != nil {
		return CommitResult{}, err
	}

	receipt, err := preserve(ctx, socketPath, source)
	if err != nil {
		return CommitResult{}, fmt.Errorf("preserve SourceArtifact bytes: %w", err)
	}

	databaseResult, err := commitDatabase(ctx, request, receipt)
	if err != nil {
		return CommitResult{}, &UnprovenCommitError{
			Cause:   err,
			Receipt: receipt,
			Request: request,
		}
	}

	databaseState := DatabaseCommitConfirmed
	if databaseResult.AlreadyCommitted {
		databaseState = DatabaseCommitAlreadyConfirmed
	}

	return CommitResult{
		DatabaseState:    databaseState,
		Receipt:          receipt,
		SourceArtifactID: databaseResult.SourceArtifactID,
	}, nil
}

func scanObjects(objectsDir string) (map[string]objectState, []Finding, error) {
	result := make(map[string]objectState)
	var findings []Finding

	err := filepath.WalkDir(objectsDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(objectsDir, path)
		if err != nil {
			return err
		}

		if !entry.Type().IsRegular() {
			findings = append(findings, Finding{
				Kind:             FindingUnexpectedObjectPath,
				StorageReference: filepath.ToSlash(filepath.Join("objects", relative)),
			})
			return nil
		}

		if !strings.ContainsRune(relative, filepath.Separator) && strings.HasPrefix(relative, ".incoming-") {
			return nil
		}

		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) != 3 || parts[0] != "sha256" || len(parts[1]) != 2 || len(parts[2]) != 64 || parts[1] != parts[2][:2] {
			findings = append(findings, Finding{
				Kind:             FindingUnexpectedObjectPath,
				StorageReference: filepath.ToSlash(filepath.Join("objects", relative)),
			})
			return nil
		}

		if _, err := hex.DecodeString(parts[2]); err != nil || strings.ToLower(parts[2]) != parts[2] {
			findings = append(findings, Finding{
				Kind:             FindingUnexpectedObjectPath,
				StorageReference: filepath.ToSlash(filepath.Join("objects", relative)),
			})
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		hasher := sha256.New()
		byteLength, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}

		storageReference := filepath.ToSlash(filepath.Join("objects", relative))
		result[storageReference] = objectState{
			ByteLength: byteLength,
			SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scan artifact objects: %w", err)
	}

	return result, findings, nil
}

func validateUUID(value string, requireV7 bool) error {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return fmt.Errorf("invalid UUID format")
	}

	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 16 {
		return fmt.Errorf("invalid UUID format")
	}

	if decoded[8]&0xc0 != 0x80 {
		return fmt.Errorf("invalid UUID variant")
	}

	if requireV7 && decoded[6]>>4 != 7 {
		return fmt.Errorf("UUID must be version 7")
	}

	return nil
}
