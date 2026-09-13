package cisakev

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/semantictransaction"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

type fakePipelinePersistence struct {
	appendErr      error
	completed      CompletedArtifact
	completedErr   error
	completedFound bool
	commitErr      error
	committedBatch *semantictransaction.Batch
	startErr       error
	startedRun     *processinghistory.Run
	terminalEvents []processinghistory.TerminalEvent
}

func (fake *fakePipelinePersistence) FindCompleted(
	_ context.Context,
	_ string,
	_ string,
	_ string,
) (CompletedArtifact, bool, error) {
	return fake.completed, fake.completedFound, fake.completedErr
}

func (fake *fakePipelinePersistence) AppendTerminal(
	_ context.Context,
	event processinghistory.TerminalEvent,
) error {
	fake.terminalEvents = append(fake.terminalEvents, event)
	return fake.appendErr
}

func (fake *fakePipelinePersistence) CommitSuccess(
	_ context.Context,
	batch semantictransaction.Batch,
) error {
	copied := batch
	fake.committedBatch = &copied
	return fake.commitErr
}

func (fake *fakePipelinePersistence) StartRun(
	_ context.Context,
	run processinghistory.Run,
) (processinghistory.Run, error) {
	if fake.startErr != nil {
		return processinghistory.Run{}, fake.startErr
	}

	run.ProcessingRunID = "01990000-0000-7000-8000-000000000001"
	copied := run
	fake.startedRun = &copied
	return run, nil
}

func TestProcessPreservedArtifactReturnsExistingCompletedContractBeforeStart(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(
		t,
		[]byte(`{"broken":`),
	)
	persistence := &fakePipelinePersistence{
		completed: CompletedArtifact{
			CheckpointKind:      semantictransaction.CheckpointKindFullSnapshot,
			CheckpointValue:     "already-complete",
			ProcessName:         request.ProcessName,
			ProcessVersion:      request.ProcessVersion,
			ProcessingEventType: processinghistory.EventTypeCompleted,
			ProcessingRunID:     "01990000-0000-7000-8000-000000000099",
			RecordCount:         123,
			SourceArtifactID:    request.Artifact.Record.SourceArtifactID,
		},
		completedFound: true,
	}

	result, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	)
	if err != nil {
		t.Fatalf("ProcessPreservedArtifact() error = %v", err)
	}

	if !result.AlreadyCompleted {
		t.Fatal("AlreadyCompleted = false, want true")
	}
	if result.ProcessingRunID != persistence.completed.ProcessingRunID {
		t.Fatalf("ProcessingRunID = %q", result.ProcessingRunID)
	}
	if result.RecordCount != 123 {
		t.Fatalf("RecordCount = %d, want 123", result.RecordCount)
	}
	if persistence.startedRun != nil {
		t.Fatal("ProcessingRun was created for already-completed contract")
	}
	if persistence.committedBatch != nil {
		t.Fatal("semantic batch committed for already-completed contract")
	}
	if len(persistence.terminalEvents) != 0 {
		t.Fatalf("terminal events = %d, want 0", len(persistence.terminalEvents))
	}
}

func TestProcessPreservedArtifactRejectsMismatchedCompletedContract(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(
		t,
		[]byte(`{"catalogVersion":"x","vulnerabilities":[]}`),
	)
	persistence := &fakePipelinePersistence{
		completed: CompletedArtifact{
			CheckpointKind:      semantictransaction.CheckpointKindFullSnapshot,
			CheckpointValue:     "already-complete",
			ProcessName:         "different-process",
			ProcessVersion:      request.ProcessVersion,
			ProcessingEventType: processinghistory.EventTypeCompleted,
			ProcessingRunID:     "01990000-0000-7000-8000-000000000099",
			RecordCount:         0,
			SourceArtifactID:    request.Artifact.Record.SourceArtifactID,
		},
		completedFound: true,
	}

	if _, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	); err == nil {
		t.Fatal("ProcessPreservedArtifact() error = nil, want mismatch rejection")
	}

	if persistence.startedRun != nil {
		t.Fatal("ProcessingRun was created after completed-contract mismatch")
	}
}

func TestProcessPreservedArtifactAllowsEmptyFullSnapshot(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(
		t,
		[]byte(`{"catalogVersion":"empty-1","vulnerabilities":[]}`),
	)
	persistence := &fakePipelinePersistence{}

	result, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	)
	if err != nil {
		t.Fatalf("ProcessPreservedArtifact() error = %v", err)
	}

	if result.RecordCount != 0 {
		t.Fatalf("RecordCount = %d, want 0", result.RecordCount)
	}
	if persistence.committedBatch == nil {
		t.Fatal("empty full snapshot was not committed")
	}
	if len(persistence.committedBatch.Records) != 0 {
		t.Fatalf("committed records = %d, want 0", len(persistence.committedBatch.Records))
	}
}

func TestProcessPreservedArtifactCommitsSuccessfulSnapshot(t *testing.T) {
	t.Parallel()

	body := []byte(`{
	  "catalogVersion": "2026.09.13",
	  "vulnerabilities": [
	    {"cveID": "CVE-2026-1234"},
	    {"cveID": "not-a-cve"}
	  ]
	}`)

	request := testProcessRequest(t, body)
	persistence := &fakePipelinePersistence{}

	result, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	)
	if err != nil {
		t.Fatalf("ProcessPreservedArtifact() error = %v", err)
	}

	if persistence.startedRun == nil {
		t.Fatal("ProcessingRun was not created")
	}
	if persistence.committedBatch == nil {
		t.Fatal("success batch was not committed")
	}
	if len(persistence.terminalEvents) != 0 {
		t.Fatalf("standalone terminal events = %d, want 0", len(persistence.terminalEvents))
	}

	if result.RecordCount != 2 {
		t.Fatalf("RecordCount = %d, want 2", result.RecordCount)
	}
	if result.CheckpointValue != "2026.09.13" {
		t.Fatalf("CheckpointValue = %q", result.CheckpointValue)
	}

	first := persistence.committedBatch.Records[0]
	if first.CVEIdentifier != "CVE-2026-1234" {
		t.Fatalf("first CVEIdentifier = %q", first.CVEIdentifier)
	}
	if first.ProcessingErrorClass != processinghistory.ErrorClassNotApplicable {
		t.Fatalf("first ProcessingErrorClass = %q", first.ProcessingErrorClass)
	}

	second := persistence.committedBatch.Records[1]
	if second.CVEIdentifier != semantictransaction.NotApplicable {
		t.Fatalf("failed record CVEIdentifier = %q", second.CVEIdentifier)
	}
	if second.ProcessingErrorClass != ErrorClassInvalidCVEIdentifier {
		t.Fatalf("failed record error class = %q", second.ProcessingErrorClass)
	}
}

func TestProcessPreservedArtifactDoesNotParseWhenStartRunFails(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(
		t,
		[]byte(`{"catalogVersion":"2026.09.13","vulnerabilities":[{"cveID":"CVE-2026-1234"}]}`),
	)
	persistence := &fakePipelinePersistence{
		startErr: errors.New("database unavailable"),
	}

	if _, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	); err == nil {
		t.Fatal("ProcessPreservedArtifact() error = nil, want start failure")
	}

	if persistence.committedBatch != nil {
		t.Fatal("success batch committed after ProcessingRun failure")
	}
	if len(persistence.terminalEvents) != 0 {
		t.Fatalf("terminal events = %d, want 0", len(persistence.terminalEvents))
	}
}

func TestProcessPreservedArtifactLeavesAmbiguousCommitUnresolved(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(
		t,
		[]byte(`{"catalogVersion":"2026.09.13","vulnerabilities":[{"cveID":"CVE-2026-1234"}]}`),
	)
	persistence := &fakePipelinePersistence{
		commitErr: errors.New("connection lost after send"),
	}

	_, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	)
	if err == nil {
		t.Fatal("ProcessPreservedArtifact() error = nil, want unresolved commit")
	}

	if len(persistence.terminalEvents) != 0 {
		t.Fatalf(
			"terminal events = %d, want 0 because commit outcome is ambiguous",
			len(persistence.terminalEvents),
		)
	}
}

func TestProcessPreservedArtifactRecordsCaughtParseFailure(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(t, []byte(`{"broken":`))
	persistence := &fakePipelinePersistence{}

	_, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	)
	if err == nil {
		t.Fatal("ProcessPreservedArtifact() error = nil, want parse failure")
	}

	if len(persistence.terminalEvents) != 1 {
		t.Fatalf("terminal events = %d, want 1", len(persistence.terminalEvents))
	}

	event := persistence.terminalEvents[0]
	if event.EventType != processinghistory.EventTypeFailed {
		t.Fatalf("EventType = %q", event.EventType)
	}
	if event.ErrorClass != ErrorClassArtifactParseFailed {
		t.Fatalf("ErrorClass = %q", event.ErrorClass)
	}
	if persistence.committedBatch != nil {
		t.Fatal("success batch committed after parse failure")
	}
}

func TestProcessPreservedArtifactRecordsSourceArtifactReadFailureSeparately(t *testing.T) {
	t.Parallel()

	request := testProcessRequest(
		t,
		[]byte(`{"catalogVersion":"2026.09.13","vulnerabilities":[]}`),
	)

	request.Artifact.Record.ByteLength++

	persistence := &fakePipelinePersistence{}

	_, err := ProcessPreservedArtifact(
		context.Background(),
		request,
		persistence,
	)
	if err == nil {
		t.Fatal("ProcessPreservedArtifact() error = nil, want SourceArtifact read failure")
	}

	if len(persistence.terminalEvents) != 1 {
		t.Fatalf("terminal events = %d, want 1", len(persistence.terminalEvents))
	}

	event := persistence.terminalEvents[0]
	if event.EventType != processinghistory.EventTypeFailed {
		t.Fatalf("EventType = %q", event.EventType)
	}
	if event.ErrorClass != ErrorClassSourceArtifactReadFailed {
		t.Fatalf(
			"ErrorClass = %q, want %q",
			event.ErrorClass,
			ErrorClassSourceArtifactReadFailed,
		)
	}
	if persistence.committedBatch != nil {
		t.Fatal("success batch committed after SourceArtifact read failure")
	}
}

func testProcessRequest(t *testing.T, body []byte) ProcessRequest {
	t.Helper()

	root := t.TempDir()
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	reference := sourceartifact.ExpectedStorageReference(digest)
	path := filepath.Join(root, filepath.FromSlash(reference))

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}

	return ProcessRequest{
		Artifact: ArtifactContext{
			Record: sourceartifact.Record{
				AvailabilityState: sourceartifact.AvailabilityAvailable,
				ByteLength:        int64(len(body)),
				SHA256:            digest,
				SourceArtifactID:  "01990000-0000-7000-8000-000000000002",
				StorageReference:  reference,
			},
			RetrievalEventID:   "01990000-0000-7000-8000-000000000005",
			SourceCollectionID: "01990000-0000-7000-8000-000000000003",
			SourceID:           "01990000-0000-7000-8000-000000000004",
		},
		ArtifactRoot:      root,
		ExtractionVersion: "cisakev-parser-v1",
		ProcessName:       "cisakev",
		ProcessVersion:    "1",
	}
}
