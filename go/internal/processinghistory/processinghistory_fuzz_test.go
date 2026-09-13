package processinghistory

import (
	"strings"
	"testing"
)

func FuzzProcessingHistoryValidators(f *testing.F) {
	f.Add(
		ValidationStateValid,
		ProcessingStateProcessed,
		ErrorClassNotApplicable,
		EventTypeCompleted,
		ErrorClassNotApplicable,
	)
	f.Add(
		ValidationStateInvalid,
		ProcessingStateFailed,
		"INVALID_CVE_IDENTIFIER",
		EventTypeFailed,
		"PARSER_FAILED",
	)
	f.Add(
		ValidationStateNotValidated,
		ProcessingStateUnsupported,
		"UNSUPPORTED_RECORD_SHAPE",
		EventTypeInterrupted,
		"STALE_PROCESSING_RUN",
	)
	f.Add("", "", "", "", "")

	f.Fuzz(func(
		t *testing.T,
		validationState string,
		processingState string,
		processingErrorClass string,
		eventType string,
		eventErrorClass string,
	) {
		validationState = fuzzBoundString(validationState)
		processingState = fuzzBoundString(processingState)
		processingErrorClass = fuzzBoundString(processingErrorClass)
		eventType = fuzzBoundString(eventType)
		eventErrorClass = fuzzBoundString(eventErrorClass)

		outcome := RecordOutcome{
			ProcessingErrorClass: processingErrorClass,
			ProcessingRunID:      "01990000-0000-7000-8000-000000000001",
			ProcessingState:      processingState,
			SourceArtifactID:     "01990000-0000-7000-8000-000000000002",
			SourceCollectionID:   "01990000-0000-7000-8000-000000000003",
			SourceID:             "01990000-0000-7000-8000-000000000004",
			SourceRecordID:       "01990000-0000-7000-8000-000000000005",
			ValidationState:      validationState,
		}

		if err := ValidateRecordOutcome(outcome); err == nil {
			switch outcome.ValidationState {
			case ValidationStateInvalid,
				ValidationStateNotValidated,
				ValidationStateValid:
			default:
				t.Fatalf(
					"validator accepted validation_state=%q",
					outcome.ValidationState,
				)
			}

			switch outcome.ProcessingState {
			case ProcessingStateProcessed:
				if outcome.ValidationState != ValidationStateValid {
					t.Fatal("PROCESSED accepted without VALID")
				}
				if outcome.ProcessingErrorClass != ErrorClassNotApplicable {
					t.Fatal("PROCESSED accepted with error class")
				}

			case ProcessingStateFailed,
				ProcessingStateUnsupported:
				if strings.TrimSpace(outcome.ProcessingErrorClass) == "" ||
					outcome.ProcessingErrorClass == ErrorClassNotApplicable {
					t.Fatal("failure state accepted without explicit error class")
				}

			default:
				t.Fatalf(
					"validator accepted processing_state=%q",
					outcome.ProcessingState,
				)
			}
		}

		event := TerminalEvent{
			ErrorClass:      eventErrorClass,
			EventType:       eventType,
			ProcessingRunID: "01990000-0000-7000-8000-000000000006",
		}

		if err := ValidateTerminalEvent(event); err == nil {
			switch event.EventType {
			case EventTypeCompleted:
				if event.ErrorClass != ErrorClassNotApplicable {
					t.Fatal("COMPLETED accepted with non-NOT_APPLICABLE error")
				}

			case EventTypeFailed,
				EventTypeInterrupted:
				if strings.TrimSpace(event.ErrorClass) == "" ||
					event.ErrorClass == ErrorClassNotApplicable {
					t.Fatal("failure terminal event accepted without explicit error")
				}

			default:
				t.Fatalf(
					"validator accepted event_type=%q",
					event.EventType,
				)
			}
		}
	})
}

func fuzzBoundString(value string) string {
	const max = 256
	if len(value) > max {
		return value[:max]
	}

	return value
}
