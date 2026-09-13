package processinghistory

import (
	"fmt"
	"strings"
)

const (
	ErrorClassNotApplicable = "NOT_APPLICABLE"

	EventTypeCompleted   = "COMPLETED"
	EventTypeFailed      = "FAILED"
	EventTypeInterrupted = "INTERRUPTED"

	ProcessingStateFailed      = "FAILED"
	ProcessingStateProcessed   = "PROCESSED"
	ProcessingStateUnsupported = "UNSUPPORTED"

	ValidationStateInvalid      = "INVALID"
	ValidationStateNotValidated = "NOT_VALIDATED"
	ValidationStateValid        = "VALID"
)

// RecordOutcome is one SourceRecordProcessing row candidate. The context fields
// intentionally repeat the exact artifact/source context so cross-artifact
// linkage can be rejected by both application validation and database foreign
// keys.
type RecordOutcome struct {
	ProcessingErrorClass string
	ProcessingRunID      string
	ProcessingState      string
	SourceArtifactID     string
	SourceCollectionID   string
	SourceID             string
	SourceRecordID       string
	ValidationState      string
}

// Run is one immutable semantic processing attempt against one preserved
// SourceArtifact. The durable STARTED state is the existence of the
// ProcessingRun row itself.
type Run struct {
	ProcessName        string
	ProcessVersion     string
	ProcessingRunID    string
	RetrievalEventID   string
	SourceArtifactID   string
	SourceCollectionID string
	SourceID           string
}

// TerminalEvent is the single terminal event that may be appended to one
// ProcessingRun.
type TerminalEvent struct {
	ErrorClass      string
	EventType       string
	ProcessingRunID string
}

func CompletedEvent(processingRunID string) TerminalEvent {
	return TerminalEvent{
		ErrorClass:      ErrorClassNotApplicable,
		EventType:       EventTypeCompleted,
		ProcessingRunID: processingRunID,
	}
}

func FailedEvent(processingRunID string, errorClass string) TerminalEvent {
	return TerminalEvent{
		ErrorClass:      errorClass,
		EventType:       EventTypeFailed,
		ProcessingRunID: processingRunID,
	}
}

func InterruptedEvent(processingRunID string, errorClass string) TerminalEvent {
	return TerminalEvent{
		ErrorClass:      errorClass,
		EventType:       EventTypeInterrupted,
		ProcessingRunID: processingRunID,
	}
}

func ValidateRecordOutcome(outcome RecordOutcome) error {
	for name, value := range map[string]string{
		"processing_run_id":      outcome.ProcessingRunID,
		"source_artifact_id":     outcome.SourceArtifactID,
		"source_collection_id":   outcome.SourceCollectionID,
		"source_id":              outcome.SourceID,
		"source_record_id":       outcome.SourceRecordID,
		"validation_state":       outcome.ValidationState,
		"processing_state":       outcome.ProcessingState,
		"processing_error_class": outcome.ProcessingErrorClass,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}

	switch outcome.ValidationState {
	case ValidationStateInvalid,
		ValidationStateNotValidated,
		ValidationStateValid:
	default:
		return fmt.Errorf("unsupported validation_state %q", outcome.ValidationState)
	}

	switch outcome.ProcessingState {
	case ProcessingStateProcessed:
		if outcome.ValidationState != ValidationStateValid {
			return fmt.Errorf(
				"processing_state=%s requires validation_state=%s",
				ProcessingStateProcessed,
				ValidationStateValid,
			)
		}
		if outcome.ProcessingErrorClass != ErrorClassNotApplicable {
			return fmt.Errorf(
				"processing_state=%s requires processing_error_class=%s",
				ProcessingStateProcessed,
				ErrorClassNotApplicable,
			)
		}

	case ProcessingStateFailed,
		ProcessingStateUnsupported:
		if outcome.ProcessingErrorClass == ErrorClassNotApplicable {
			return fmt.Errorf(
				"processing_state=%s requires explicit processing_error_class",
				outcome.ProcessingState,
			)
		}

	default:
		return fmt.Errorf("unsupported processing_state %q", outcome.ProcessingState)
	}

	return nil
}

func ValidateRun(run Run) error {
	for name, value := range map[string]string{
		"process_name":         run.ProcessName,
		"process_version":      run.ProcessVersion,
		"processing_run_id":    run.ProcessingRunID,
		"retrieval_event_id":   run.RetrievalEventID,
		"source_artifact_id":   run.SourceArtifactID,
		"source_collection_id": run.SourceCollectionID,
		"source_id":            run.SourceID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}

	return nil
}

func ValidateTerminalEvent(event TerminalEvent) error {
	if strings.TrimSpace(event.ProcessingRunID) == "" {
		return fmt.Errorf("processing_run_id must not be empty")
	}
	if strings.TrimSpace(event.ErrorClass) == "" {
		return fmt.Errorf("error_class must not be empty")
	}

	switch event.EventType {
	case EventTypeCompleted:
		if event.ErrorClass != ErrorClassNotApplicable {
			return fmt.Errorf(
				"event_type=%s requires error_class=%s",
				EventTypeCompleted,
				ErrorClassNotApplicable,
			)
		}

	case EventTypeFailed,
		EventTypeInterrupted:
		if event.ErrorClass == ErrorClassNotApplicable {
			return fmt.Errorf(
				"event_type=%s requires explicit error_class",
				event.EventType,
			)
		}

	default:
		return fmt.Errorf("unsupported event_type %q", event.EventType)
	}

	return nil
}
