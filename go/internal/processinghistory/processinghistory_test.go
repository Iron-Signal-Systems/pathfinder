package processinghistory

import "testing"

func TestCompletedEventMatchesTerminalContract(t *testing.T) {
	t.Parallel()

	event := CompletedEvent("run-1")
	if err := ValidateTerminalEvent(event); err != nil {
		t.Fatalf("ValidateTerminalEvent() error = %v", err)
	}
}

func TestFailedAndInterruptedRequireExplicitErrorClass(t *testing.T) {
	t.Parallel()

	tests := []TerminalEvent{
		FailedEvent("run-1", ErrorClassNotApplicable),
		InterruptedEvent("run-1", ErrorClassNotApplicable),
	}

	for _, event := range tests {
		if err := ValidateTerminalEvent(event); err == nil {
			t.Fatalf("ValidateTerminalEvent(%#v) error = nil, want failure", event)
		}
	}
}

func TestRecordOutcomeFailedAndUnsupportedRequireErrorClass(t *testing.T) {
	t.Parallel()

	for _, processingState := range []string{
		ProcessingStateFailed,
		ProcessingStateUnsupported,
	} {
		outcome := validOutcome()
		outcome.ProcessingState = processingState
		outcome.ProcessingErrorClass = ErrorClassNotApplicable

		if err := ValidateRecordOutcome(outcome); err == nil {
			t.Fatalf(
				"ValidateRecordOutcome() error = nil for processing_state=%s",
				processingState,
			)
		}
	}
}

func TestRecordOutcomeProcessedRequiresValidAndNoError(t *testing.T) {
	t.Parallel()

	valid := validOutcome()
	if err := ValidateRecordOutcome(valid); err != nil {
		t.Fatalf("ValidateRecordOutcome(valid) error = %v", err)
	}

	invalidValidation := valid
	invalidValidation.ValidationState = ValidationStateInvalid
	if err := ValidateRecordOutcome(invalidValidation); err == nil {
		t.Fatal("ValidateRecordOutcome() error = nil, want validation-state failure")
	}

	explicitError := valid
	explicitError.ProcessingErrorClass = "UNEXPECTED"
	if err := ValidateRecordOutcome(explicitError); err == nil {
		t.Fatal("ValidateRecordOutcome() error = nil, want error-class failure")
	}
}

func TestRunRequiresExactProcessingContext(t *testing.T) {
	t.Parallel()

	run := Run{
		ProcessName:        "cisakev",
		ProcessVersion:     "1",
		ProcessingRunID:    "run-1",
		RetrievalEventID:   "retrieval-1",
		SourceArtifactID:   "artifact-1",
		SourceCollectionID: "collection-1",
		SourceID:           "source-1",
	}

	if err := ValidateRun(run); err != nil {
		t.Fatalf("ValidateRun(valid) error = %v", err)
	}

	run.SourceArtifactID = ""
	if err := ValidateRun(run); err == nil {
		t.Fatal("ValidateRun() error = nil, want missing artifact context failure")
	}
}

func TestTerminalEventRejectsUnknownType(t *testing.T) {
	t.Parallel()

	event := TerminalEvent{
		ErrorClass:      ErrorClassNotApplicable,
		EventType:       "STARTED",
		ProcessingRunID: "run-1",
	}

	if err := ValidateTerminalEvent(event); err == nil {
		t.Fatal("ValidateTerminalEvent() error = nil, want unsupported event failure")
	}
}

func validOutcome() RecordOutcome {
	return RecordOutcome{
		ProcessingErrorClass: ErrorClassNotApplicable,
		ProcessingRunID:      "run-1",
		ProcessingState:      ProcessingStateProcessed,
		SourceArtifactID:     "artifact-1",
		SourceCollectionID:   "collection-1",
		SourceID:             "source-1",
		SourceRecordID:       "record-1",
		ValidationState:      ValidationStateValid,
	}
}
