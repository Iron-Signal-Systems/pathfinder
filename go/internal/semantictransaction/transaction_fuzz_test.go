package semantictransaction

import (
	"strings"
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
)

func FuzzBuildSuccessSQL(f *testing.F) {
	f.Add(
		"CVE-2026-1234",
		"CVE-2026-1234",
		"/vulnerabilities/0",
		SourceMarkingNotKnown,
		processinghistory.ValidationStateValid,
		processinghistory.ProcessingStateProcessed,
		processinghistory.ErrorClassNotApplicable,
		int16(0),
	)
	f.Add(
		NotApplicable,
		"NOT-A-CVE",
		"/vulnerabilities/1",
		SourceMarkingNotKnown,
		processinghistory.ValidationStateInvalid,
		processinghistory.ProcessingStateFailed,
		"INVALID_CVE_IDENTIFIER",
		int16(1),
	)
	f.Add(
		"$pf_deadbeef$",
		`x'"\\`,
		"/vulnerabilities/2\n",
		"NOT_KNOWN",
		"INVALID",
		"FAILED",
		"FUZZ_ERROR",
		int16(-1),
	)

	f.Fuzz(func(
		t *testing.T,
		cveIdentifier string,
		externalRecordID string,
		sourceLocator string,
		sourceMarking string,
		validationState string,
		processingState string,
		processingErrorClass string,
		ordinal int16,
	) {
		cveIdentifier = semanticFuzzBound(cveIdentifier)
		externalRecordID = semanticFuzzBound(externalRecordID)
		sourceLocator = semanticFuzzBound(sourceLocator)
		sourceMarking = semanticFuzzBound(sourceMarking)
		validationState = semanticFuzzBound(validationState)
		processingState = semanticFuzzBound(processingState)
		processingErrorClass = semanticFuzzBound(processingErrorClass)

		batch := Batch{
			CheckpointValue:   "2026.09.11",
			ExtractionVersion: "cisa-kev-json-v1",
			ProcessName:       "cisa-kev",
			ProcessVersion:    "1",
			ProcessingRunID:   "01990000-0000-7000-8000-000000000001",
			Records: []Record{
				{
					CVEIdentifier:        cveIdentifier,
					ExternalRecordID:     externalRecordID,
					Ordinal:              int(ordinal),
					ProcessingErrorClass: processingErrorClass,
					ProcessingState:      processingState,
					SourceLocator:        sourceLocator,
					SourceMarking:        sourceMarking,
					ValidationState:      validationState,
				},
			},
			SourceArtifactID:   "01990000-0000-7000-8000-000000000002",
			SourceCollectionID: "01990000-0000-7000-8000-000000000003",
			SourceID:           "01990000-0000-7000-8000-000000000004",
		}

		validationErr := ValidateBatch(batch)
		sql, buildErr := BuildSuccessSQL(batch)

		if (validationErr == nil) != (buildErr == nil) {
			t.Fatalf(
				"ValidateBatch error=%v, BuildSuccessSQL error=%v",
				validationErr,
				buildErr,
			)
		}
		if buildErr != nil {
			if sql != "" {
				t.Fatal("invalid batch returned non-empty SQL")
			}
			return
		}

		for _, required := range []string{
			"BEGIN;",
			"CREATE TEMP TABLE phase14_batch_context",
			"CREATE TEMP TABLE phase14_semantic_input",
			"INSERT INTO pathfinder.source_record",
			"INSERT INTO pathfinder.source_record_processing",
			"INSERT INTO pathfinder.vulnerability",
			"INSERT INTO pathfinder.assertion",
			"INSERT INTO pathfinder.processing_event",
			"INSERT INTO pathfinder.collector_checkpoint",
			"COMMIT;",
		} {
			if !strings.Contains(sql, required) {
				t.Fatalf("generated SQL missing %q", required)
			}
		}
	})
}

func semanticFuzzBound(value string) string {
	const max = 512
	if len(value) > max {
		return value[:max]
	}

	return value
}
