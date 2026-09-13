package semantictransaction

import (
	"strings"
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
)

func TestBuildSuccessSQLKeepsTerminalAndCheckpointInsideTransaction(t *testing.T) {
	t.Parallel()

	sql, err := BuildSuccessSQL(validBatch())
	if err != nil {
		t.Fatalf("BuildSuccessSQL() error = %v", err)
	}

	requiredOrder := []string{
		"BEGIN;",
		"INSERT INTO pathfinder.source_record",
		"INSERT INTO pathfinder.source_record_processing",
		"INSERT INTO pathfinder.vulnerability",
		"INSERT INTO pathfinder.assertion",
		"INSERT INTO pathfinder.processing_event",
		"INSERT INTO pathfinder.collector_checkpoint",
		"COMMIT;",
	}

	position := -1
	for _, token := range requiredOrder {
		next := strings.Index(sql[position+1:], token)
		if next < 0 {
			t.Fatalf("SQL missing %q", token)
		}
		position += next + 1
	}

	if !strings.Contains(sql, "'COMPLETED'") {
		t.Fatal("SQL missing COMPLETED processing event")
	}
	if !strings.Contains(sql, "'FULL_SNAPSHOT'") {
		t.Fatal("SQL missing FULL_SNAPSHOT checkpoint")
	}
}

func TestBuildSuccessSQLPreservesBatchMetadataInsideJSONPayload(t *testing.T) {
	t.Parallel()

	batch := validBatch()
	batch.ProcessName = `cisakev'); DROP TABLE pathfinder.collector_checkpoint; --`

	sql, err := BuildSuccessSQL(batch)
	if err != nil {
		t.Fatalf("BuildSuccessSQL() error = %v", err)
	}

	if !strings.Contains(
		sql,
		`"process_name":"cisakev'); DROP TABLE pathfinder.collector_checkpoint; --"`,
	) {
		t.Fatal("batch metadata was not preserved inside JSON payload")
	}
}

func TestBuildSuccessSQLPreservesSourceTextInsideJSONPayload(t *testing.T) {
	t.Parallel()

	batch := validBatch()
	batch.Records[0].ExternalRecordID =
		`CVE-2026-1234'); DROP TABLE pathfinder.assertion; --`

	sql, err := BuildSuccessSQL(batch)
	if err != nil {
		t.Fatalf("BuildSuccessSQL() error = %v", err)
	}

	if !strings.Contains(
		sql,
		`"external_record_id":"CVE-2026-1234'); DROP TABLE pathfinder.assertion; --"`,
	) {
		t.Fatal("source text was not preserved inside JSON payload")
	}
}

func TestValidateBatchAllowsEmptyFullSnapshot(t *testing.T) {
	t.Parallel()

	batch := validBatch()
	batch.Records = []Record{}

	if err := ValidateBatch(batch); err != nil {
		t.Fatalf("ValidateBatch(empty full snapshot) error = %v", err)
	}

	sql, err := BuildSuccessSQL(batch)
	if err != nil {
		t.Fatalf("BuildSuccessSQL(empty full snapshot) error = %v", err)
	}

	if !strings.Contains(sql, "INSERT INTO pathfinder.collector_checkpoint") {
		t.Fatal("empty full snapshot SQL missing checkpoint")
	}
	if !strings.Contains(sql, "'COMPLETED'") {
		t.Fatal("empty full snapshot SQL missing COMPLETED event")
	}
}

func TestValidateBatchRejectsDuplicateLocator(t *testing.T) {
	t.Parallel()

	batch := validBatch()
	duplicate := batch.Records[0]
	duplicate.Ordinal = 1
	batch.Records = append(batch.Records, duplicate)

	if err := ValidateBatch(batch); err == nil {
		t.Fatal("ValidateBatch() error = nil, want duplicate locator rejection")
	}
}

func TestValidateBatchRejectsInvalidProcessedCVE(t *testing.T) {
	t.Parallel()

	batch := validBatch()
	batch.Records[0].CVEIdentifier = "not-a-cve"

	if err := ValidateBatch(batch); err == nil {
		t.Fatal("ValidateBatch() error = nil, want invalid CVE rejection")
	}
}

func TestValidateBatchRejectsVulnerabilityForFailedRecord(t *testing.T) {
	t.Parallel()

	batch := validBatch()
	batch.Records[0] = Record{
		CVEIdentifier:        "CVE-2026-1234",
		ExternalRecordID:     "not-a-cve",
		Ordinal:              0,
		ProcessingErrorClass: "INVALID_CVE_IDENTIFIER",
		ProcessingState:      processinghistory.ProcessingStateFailed,
		SourceLocator:        "/vulnerabilities/0",
		SourceMarking:        SourceMarkingNotKnown,
		ValidationState:      processinghistory.ValidationStateInvalid,
	}

	if err := ValidateBatch(batch); err == nil {
		t.Fatal("ValidateBatch() error = nil, want failed-record CVE rejection")
	}
}

func validBatch() Batch {
	return Batch{
		CheckpointValue:   "2026.09.13",
		ExtractionVersion: "cisakev-parser-v1",
		ProcessName:       "cisakev",
		ProcessVersion:    "1",
		ProcessingRunID:   "01990000-0000-7000-8000-000000000001",
		Records: []Record{
			{
				CVEIdentifier:        "CVE-2026-1234",
				ExternalRecordID:     "CVE-2026-1234",
				Ordinal:              0,
				ProcessingErrorClass: processinghistory.ErrorClassNotApplicable,
				ProcessingState:      processinghistory.ProcessingStateProcessed,
				SourceLocator:        "/vulnerabilities/0",
				SourceMarking:        SourceMarkingNotKnown,
				ValidationState:      processinghistory.ValidationStateValid,
			},
			{
				CVEIdentifier:        NotApplicable,
				ExternalRecordID:     "not-a-cve",
				Ordinal:              1,
				ProcessingErrorClass: "INVALID_CVE_IDENTIFIER",
				ProcessingState:      processinghistory.ProcessingStateFailed,
				SourceLocator:        "/vulnerabilities/1",
				SourceMarking:        SourceMarkingNotKnown,
				ValidationState:      processinghistory.ValidationStateInvalid,
			},
		},
		SourceArtifactID:   "01990000-0000-7000-8000-000000000002",
		SourceCollectionID: "01990000-0000-7000-8000-000000000003",
		SourceID:           "01990000-0000-7000-8000-000000000004",
	}
}

func TestBuildSuccessSQLGuardsImmutableSourceRecordIdentity(t *testing.T) {
	t.Parallel()

	sql, err := BuildSuccessSQL(validBatch())
	if err != nil {
		t.Fatalf("BuildSuccessSQL() error = %v", err)
	}

	required := []string{
		"record.source_id IS DISTINCT FROM context.source_id",
		"record.source_collection_id IS DISTINCT FROM context.source_collection_id",
		"record.external_record_id IS DISTINCT FROM input.external_record_id",
		"record.source_marking IS DISTINCT FROM input.source_marking",
		"RAISE EXCEPTION 'immutable SourceRecord identity conflict'",
	}

	for _, token := range required {
		if !strings.Contains(sql, token) {
			t.Fatalf("SQL missing immutable SourceRecord guard %q", token)
		}
	}
}

func TestBuildSuccessSQLChecksSourceRecordIdentityBeforeDependentRows(t *testing.T) {
	t.Parallel()

	sql, err := BuildSuccessSQL(validBatch())
	if err != nil {
		t.Fatalf("BuildSuccessSQL() error = %v", err)
	}

	guard := strings.Index(sql, "immutable SourceRecord identity conflict")
	processing := strings.Index(sql, "INSERT INTO pathfinder.source_record_processing")
	assertion := strings.Index(sql, "INSERT INTO pathfinder.assertion")
	checkpoint := strings.Index(sql, "INSERT INTO pathfinder.collector_checkpoint")

	if guard < 0 || processing < 0 || assertion < 0 || checkpoint < 0 {
		t.Fatal("SQL missing guard or dependent semantic statement")
	}
	if !(guard < processing && guard < assertion && guard < checkpoint) {
		t.Fatal("immutable SourceRecord guard must execute before dependent semantic rows")
	}
}
