package cisakev

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/semantictransaction"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

const (
	ErrorClassArtifactParseFailed      = "CISA_KEV_ARTIFACT_PARSE_FAILED"
	ErrorClassSemanticBatchInvalid     = "CISA_KEV_SEMANTIC_BATCH_INVALID"
	ErrorClassSourceArtifactReadFailed = "CISA_KEV_SOURCE_ARTIFACT_READ_FAILED"
)

type ArtifactContext struct {
	Record             sourceartifact.Record
	RetrievalEventID   string
	SourceCollectionID string
	SourceID           string
}

// CompletedArtifact is the read-back proof for one already-completed
// SourceArtifact + process_name + process_version contract.
type CompletedArtifact struct {
	CheckpointKind      string
	CheckpointValue     string
	ProcessName         string
	ProcessVersion      string
	ProcessingEventType string
	ProcessingRunID     string
	RecordCount         int
	SourceArtifactID    string
}

type Persistence interface {
	FindCompleted(
		context.Context,
		string,
		string,
		string,
	) (CompletedArtifact, bool, error)
	AppendTerminal(
		context.Context,
		processinghistory.TerminalEvent,
	) error
	CommitSuccess(
		context.Context,
		semantictransaction.Batch,
	) error
	StartRun(
		context.Context,
		processinghistory.Run,
	) (processinghistory.Run, error)
}

type ProcessRequest struct {
	Artifact          ArtifactContext
	ArtifactRoot      string
	ExtractionVersion string
	ProcessName       string
	ProcessVersion    string
}

type ProcessResult struct {
	AlreadyCompleted bool
	CheckpointValue  string
	ProcessingRunID  string
	RecordCount      int
}

func ProcessPreservedArtifact(
	ctx context.Context,
	request ProcessRequest,
	persistence Persistence,
) (ProcessResult, error) {
	if persistence == nil {
		return ProcessResult{}, fmt.Errorf("persistence must not be nil")
	}

	if err := validateProcessRequest(request); err != nil {
		return ProcessResult{}, err
	}

	completed, found, err := persistence.FindCompleted(
		ctx,
		request.Artifact.Record.SourceArtifactID,
		request.ProcessName,
		request.ProcessVersion,
	)
	if err != nil {
		return ProcessResult{}, fmt.Errorf(
			"check existing completed processing contract: %w",
			err,
		)
	}
	if found {
		if err := validateCompletedArtifact(request, completed); err != nil {
			return ProcessResult{}, fmt.Errorf(
				"prove existing completed processing contract: %w",
				err,
			)
		}

		return ProcessResult{
			AlreadyCompleted: true,
			CheckpointValue:  completed.CheckpointValue,
			ProcessingRunID:  completed.ProcessingRunID,
			RecordCount:      completed.RecordCount,
		}, nil
	}

	run, err := persistence.StartRun(
		ctx,
		processinghistory.Run{
			ProcessName:        request.ProcessName,
			ProcessVersion:     request.ProcessVersion,
			RetrievalEventID:   request.Artifact.RetrievalEventID,
			SourceArtifactID:   request.Artifact.Record.SourceArtifactID,
			SourceCollectionID: request.Artifact.SourceCollectionID,
			SourceID:           request.Artifact.SourceID,
		},
	)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("create ProcessingRun: %w", err)
	}

	if err := validateStartedRun(request, run); err != nil {
		return ProcessResult{}, fmt.Errorf(
			"prove created ProcessingRun contract: %w",
			err,
		)
	}

	catalog, err := ParseSourceArtifact(
		request.ArtifactRoot,
		request.Artifact.Record,
	)
	if err != nil {
		errorClass := ErrorClassArtifactParseFailed

		var readErr *SourceArtifactReadError
		if errors.As(err, &readErr) {
			errorClass = ErrorClassSourceArtifactReadFailed
		}

		return ProcessResult{}, failCaughtAttempt(
			ctx,
			persistence,
			run.ProcessingRunID,
			errorClass,
			err,
		)
	}

	batch, err := buildSemanticBatch(request, run, catalog)
	if err != nil {
		return ProcessResult{}, failCaughtAttempt(
			ctx,
			persistence,
			run.ProcessingRunID,
			ErrorClassSemanticBatchInvalid,
			err,
		)
	}

	if err := persistence.CommitSuccess(ctx, batch); err != nil {
		return ProcessResult{}, fmt.Errorf(
			"semantic success transaction outcome unresolved: %w",
			err,
		)
	}

	return ProcessResult{
		AlreadyCompleted: false,
		CheckpointValue:  batch.CheckpointValue,
		ProcessingRunID:  run.ProcessingRunID,
		RecordCount:      len(batch.Records),
	}, nil
}

func buildSemanticBatch(
	request ProcessRequest,
	run processinghistory.Run,
	catalog Catalog,
) (semantictransaction.Batch, error) {
	records := make([]semantictransaction.Record, 0, len(catalog.Records))

	for ordinal, record := range catalog.Records {
		errorClass := record.ProcessingErrorClass
		if errorClass == "" {
			errorClass = processinghistory.ErrorClassNotApplicable
		}

		cveIdentifier := semantictransaction.NotApplicable
		if record.ProcessingState == ProcessingStateProcessed {
			cveIdentifier = record.CVEIdentifier
		}

		records = append(records, semantictransaction.Record{
			CVEIdentifier:        cveIdentifier,
			ExternalRecordID:     record.ExternalRecordID,
			Ordinal:              ordinal,
			ProcessingErrorClass: errorClass,
			ProcessingState:      record.ProcessingState,
			SourceLocator:        record.SourceLocator,
			SourceMarking:        semantictransaction.SourceMarkingNotKnown,
			ValidationState:      record.ValidationState,
		})
	}

	batch := semantictransaction.Batch{
		CheckpointValue:    catalog.CheckpointValue(),
		ExtractionVersion:  request.ExtractionVersion,
		ProcessName:        run.ProcessName,
		ProcessVersion:     run.ProcessVersion,
		ProcessingRunID:    run.ProcessingRunID,
		Records:            records,
		SourceArtifactID:   run.SourceArtifactID,
		SourceCollectionID: run.SourceCollectionID,
		SourceID:           run.SourceID,
	}

	if err := semantictransaction.ValidateBatch(batch); err != nil {
		return semantictransaction.Batch{}, err
	}

	return batch, nil
}

func failCaughtAttempt(
	ctx context.Context,
	persistence Persistence,
	processingRunID string,
	errorClass string,
	cause error,
) error {
	event := processinghistory.FailedEvent(processingRunID, errorClass)
	if err := persistence.AppendTerminal(ctx, event); err != nil {
		return fmt.Errorf(
			"%v; append FAILED ProcessingEvent: %w",
			cause,
			err,
		)
	}

	return cause
}

func validateCompletedArtifact(
	request ProcessRequest,
	completed CompletedArtifact,
) error {
	if completed.CheckpointKind != semantictransaction.CheckpointKindFullSnapshot {
		return fmt.Errorf(
			"checkpoint_kind=%q, want %q",
			completed.CheckpointKind,
			semantictransaction.CheckpointKindFullSnapshot,
		)
	}
	if completed.ProcessingEventType != processinghistory.EventTypeCompleted {
		return fmt.Errorf(
			"processing_event_type=%q, want %q",
			completed.ProcessingEventType,
			processinghistory.EventTypeCompleted,
		)
	}
	if completed.SourceArtifactID != request.Artifact.Record.SourceArtifactID {
		return fmt.Errorf("completed source_artifact_id does not match request")
	}
	if completed.ProcessName != request.ProcessName {
		return fmt.Errorf("completed process_name does not match request")
	}
	if completed.ProcessVersion != request.ProcessVersion {
		return fmt.Errorf("completed process_version does not match request")
	}
	if strings.TrimSpace(completed.ProcessingRunID) == "" {
		return fmt.Errorf("completed processing_run_id must not be empty")
	}
	if strings.TrimSpace(completed.CheckpointValue) == "" {
		return fmt.Errorf("completed checkpoint value must not be empty")
	}
	if completed.RecordCount < 0 {
		return fmt.Errorf("completed record count must not be negative")
	}

	return nil
}

func validateProcessRequest(request ProcessRequest) error {
	for name, value := range map[string]string{
		"artifact_root":        request.ArtifactRoot,
		"extraction_version":   request.ExtractionVersion,
		"process_name":         request.ProcessName,
		"process_version":      request.ProcessVersion,
		"retrieval_event_id":   request.Artifact.RetrievalEventID,
		"source_artifact_id":   request.Artifact.Record.SourceArtifactID,
		"source_collection_id": request.Artifact.SourceCollectionID,
		"source_id":            request.Artifact.SourceID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}

	return nil
}

func validateStartedRun(
	request ProcessRequest,
	run processinghistory.Run,
) error {
	if err := processinghistory.ValidateRun(run); err != nil {
		return err
	}

	expected := processinghistory.Run{
		ProcessName:        request.ProcessName,
		ProcessVersion:     request.ProcessVersion,
		ProcessingRunID:    run.ProcessingRunID,
		RetrievalEventID:   request.Artifact.RetrievalEventID,
		SourceArtifactID:   request.Artifact.Record.SourceArtifactID,
		SourceCollectionID: request.Artifact.SourceCollectionID,
		SourceID:           request.Artifact.SourceID,
	}

	if run != expected {
		return fmt.Errorf("created ProcessingRun does not match requested context")
	}

	return nil
}
