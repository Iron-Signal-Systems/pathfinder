package cisakev

import (
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
)

func TestNewProcessingOutcomeMapsSuccessfulRecord(t *testing.T) {
	t.Parallel()

	record := Record{
		ProcessingErrorClass: "",
		ProcessingState:      ProcessingStateProcessed,
		ValidationState:      ValidationStateValid,
	}

	outcome, err := NewProcessingOutcome(
		validProcessingContext(),
		"record-1",
		record,
	)
	if err != nil {
		t.Fatalf("NewProcessingOutcome() error = %v", err)
	}

	if outcome.ProcessingErrorClass != processinghistory.ErrorClassNotApplicable {
		t.Fatalf(
			"ProcessingErrorClass = %q, want %q",
			outcome.ProcessingErrorClass,
			processinghistory.ErrorClassNotApplicable,
		)
	}
	if outcome.ProcessingState != processinghistory.ProcessingStateProcessed {
		t.Fatalf("ProcessingState = %q", outcome.ProcessingState)
	}
	if outcome.ValidationState != processinghistory.ValidationStateValid {
		t.Fatalf("ValidationState = %q", outcome.ValidationState)
	}
}

func TestNewProcessingOutcomePreservesExplicitFailureClass(t *testing.T) {
	t.Parallel()

	record := Record{
		ProcessingErrorClass: ErrorClassInvalidCVEIdentifier,
		ProcessingState:      ProcessingStateFailed,
		ValidationState:      ValidationStateInvalid,
	}

	outcome, err := NewProcessingOutcome(
		validProcessingContext(),
		"record-1",
		record,
	)
	if err != nil {
		t.Fatalf("NewProcessingOutcome() error = %v", err)
	}

	if outcome.ProcessingErrorClass != ErrorClassInvalidCVEIdentifier {
		t.Fatalf(
			"ProcessingErrorClass = %q, want %q",
			outcome.ProcessingErrorClass,
			ErrorClassInvalidCVEIdentifier,
		)
	}
}

func TestNewProcessingOutcomeRejectsMissingContext(t *testing.T) {
	t.Parallel()

	context := validProcessingContext()
	context.SourceArtifactID = ""

	record := Record{
		ProcessingState: ProcessingStateProcessed,
		ValidationState: ValidationStateValid,
	}

	if _, err := NewProcessingOutcome(context, "record-1", record); err == nil {
		t.Fatal("NewProcessingOutcome() error = nil, want missing context failure")
	}
}

func TestNewProcessingOutcomePreservesUnsupportedRecord(t *testing.T) {
	t.Parallel()

	record := Record{
		ProcessingErrorClass: ErrorClassUnsupportedRecordShape,
		ProcessingState:      ProcessingStateUnsupported,
		ValidationState:      ValidationStateNotValidated,
	}

	outcome, err := NewProcessingOutcome(
		validProcessingContext(),
		"record-1",
		record,
	)
	if err != nil {
		t.Fatalf("NewProcessingOutcome() error = %v", err)
	}

	if outcome.ProcessingState != processinghistory.ProcessingStateUnsupported {
		t.Fatalf("ProcessingState = %q", outcome.ProcessingState)
	}
	if outcome.ValidationState != processinghistory.ValidationStateNotValidated {
		t.Fatalf("ValidationState = %q", outcome.ValidationState)
	}
}

func validProcessingContext() ProcessingContext {
	return ProcessingContext{
		ProcessingRunID:    "run-1",
		SourceArtifactID:   "artifact-1",
		SourceCollectionID: "collection-1",
		SourceID:           "source-1",
	}
}
