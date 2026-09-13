package cisakev

import (
	"fmt"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
)

// ProcessingContext carries the exact ProcessingRun and artifact/source context
// needed to construct one SourceRecordProcessing row candidate.
type ProcessingContext struct {
	ProcessingRunID    string
	SourceArtifactID   string
	SourceCollectionID string
	SourceID           string
}

// NewProcessingOutcome translates one KEV parser result into the generic
// SourceRecordProcessing contract without changing parser/source semantics.
func NewProcessingOutcome(
	context ProcessingContext,
	sourceRecordID string,
	record Record,
) (processinghistory.RecordOutcome, error) {
	errorClass := record.ProcessingErrorClass
	if errorClass == "" {
		errorClass = processinghistory.ErrorClassNotApplicable
	}

	outcome := processinghistory.RecordOutcome{
		ProcessingErrorClass: errorClass,
		ProcessingRunID:      context.ProcessingRunID,
		ProcessingState:      record.ProcessingState,
		SourceArtifactID:     context.SourceArtifactID,
		SourceCollectionID:   context.SourceCollectionID,
		SourceID:             context.SourceID,
		SourceRecordID:       sourceRecordID,
		ValidationState:      record.ValidationState,
	}

	if err := processinghistory.ValidateRecordOutcome(outcome); err != nil {
		return processinghistory.RecordOutcome{}, fmt.Errorf(
			"build CISA KEV SourceRecordProcessing outcome: %w",
			err,
		)
	}

	return outcome, nil
}
