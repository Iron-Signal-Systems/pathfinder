package semantictransaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
)

const (
	AssertionTypeKnownExploited = "KNOWN_EXPLOITED"
	AssertedValueTrue           = "TRUE"
	CheckpointKindFullSnapshot  = "FULL_SNAPSHOT"
	ExtractionMethodStructured  = "SOURCE_STRUCTURED"
	IdentifierNamespaceCVE      = "CVE"
	NotApplicable               = "NOT_APPLICABLE"
	NotKnown                    = "NOT_KNOWN"
	NotReported                 = "NOT_REPORTED"
	SourceMarkingNotKnown       = "NOT_KNOWN"
	SubjectTypeVulnerability    = "VULNERABILITY"
)

var cvePattern = regexp.MustCompile(`^CVE-[0-9]{4}-[0-9]{4,19}$`)

// Batch is one successful full-snapshot semantic transaction candidate for an
// already-durable ProcessingRun.
type Batch struct {
	CheckpointValue    string
	ExtractionVersion  string
	ProcessName        string
	ProcessVersion     string
	ProcessingRunID    string
	Records            []Record
	SourceArtifactID   string
	SourceCollectionID string
	SourceID           string
}

// Record is one locatable SourceRecord plus its per-run processing outcome.
// CVEIdentifier is NOT_APPLICABLE for records that did not produce a valid CVE.
type Record struct {
	CVEIdentifier        string `json:"cve_identifier"`
	ExternalRecordID     string `json:"external_record_id"`
	Ordinal              int    `json:"ordinal"`
	ProcessingErrorClass string `json:"processing_error_class"`
	ProcessingState      string `json:"processing_state"`
	SourceLocator        string `json:"source_locator"`
	SourceMarking        string `json:"source_marking"`
	ValidationState      string `json:"validation_state"`
}

type batchPayload struct {
	CheckpointValue    string   `json:"checkpoint_value"`
	ExtractionVersion  string   `json:"extraction_version"`
	ProcessName        string   `json:"process_name"`
	ProcessVersion     string   `json:"process_version"`
	ProcessingRunID    string   `json:"processing_run_id"`
	Records            []Record `json:"records"`
	SourceArtifactID   string   `json:"source_artifact_id"`
	SourceCollectionID string   `json:"source_collection_id"`
	SourceID           string   `json:"source_id"`
}

// BuildSuccessSQL creates the single transaction that owns all semantic rows,
// the COMPLETED ProcessingEvent, and CollectorCheckpoint.
func BuildSuccessSQL(batch Batch) (string, error) {
	if err := ValidateBatch(batch); err != nil {
		return "", err
	}

	payload := batchPayload{
		CheckpointValue:    batch.CheckpointValue,
		ExtractionVersion:  batch.ExtractionVersion,
		ProcessName:        batch.ProcessName,
		ProcessVersion:     batch.ProcessVersion,
		ProcessingRunID:    batch.ProcessingRunID,
		Records:            batch.Records,
		SourceArtifactID:   batch.SourceArtifactID,
		SourceCollectionID: batch.SourceCollectionID,
		SourceID:           batch.SourceID,
	}

	payloadJSON, delimiter, err := marshalDollarQuoted(payload)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
BEGIN;

CREATE TEMP TABLE phase14_batch_context (
    checkpoint_value text NOT NULL,
    extraction_version text NOT NULL,
    process_name text NOT NULL,
    process_version text NOT NULL,
    processing_run_id uuid NOT NULL,
    source_artifact_id uuid NOT NULL,
    source_collection_id uuid NOT NULL,
    source_id uuid NOT NULL
) ON COMMIT DROP;

INSERT INTO phase14_batch_context (
    checkpoint_value,
    extraction_version,
    process_name,
    process_version,
    processing_run_id,
    source_artifact_id,
    source_collection_id,
    source_id
)
SELECT
    checkpoint_value,
    extraction_version,
    process_name,
    process_version,
    processing_run_id::uuid,
    source_artifact_id::uuid,
    source_collection_id::uuid,
    source_id::uuid
FROM jsonb_to_record(%s%s%s::jsonb) AS value(
    checkpoint_value text,
    extraction_version text,
    process_name text,
    process_version text,
    processing_run_id text,
    source_artifact_id text,
    source_collection_id text,
    source_id text
);

DO $guard$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pathfinder.processing_event AS event
        JOIN phase14_batch_context AS context
          ON context.processing_run_id = event.processing_run_id
    ) THEN
        RAISE EXCEPTION 'processing run already has terminal event';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pathfinder.collector_checkpoint AS checkpoint
        JOIN phase14_batch_context AS context
          ON context.source_artifact_id = checkpoint.source_artifact_id
         AND context.process_name = checkpoint.process_name
         AND context.process_version = checkpoint.process_version
    ) THEN
        RAISE EXCEPTION 'artifact/process contract already has completed checkpoint';
    END IF;
END
$guard$;

CREATE TEMP TABLE phase14_semantic_input (
    ordinal integer PRIMARY KEY,
    external_record_id text NOT NULL,
    source_locator text NOT NULL,
    source_marking text NOT NULL,
    validation_state text NOT NULL,
    processing_state text NOT NULL,
    processing_error_class text NOT NULL,
    cve_identifier text NOT NULL
) ON COMMIT DROP;

INSERT INTO phase14_semantic_input (
    ordinal,
    external_record_id,
    source_locator,
    source_marking,
    validation_state,
    processing_state,
    processing_error_class,
    cve_identifier
)
SELECT
    ordinal,
    external_record_id,
    source_locator,
    source_marking,
    validation_state,
    processing_state,
    processing_error_class,
    cve_identifier
FROM jsonb_to_recordset(
    (%s%s%s::jsonb)->'records'
) AS value(
    cve_identifier text,
    external_record_id text,
    ordinal integer,
    processing_error_class text,
    processing_state text,
    source_locator text,
    source_marking text,
    validation_state text
);

INSERT INTO pathfinder.source_record (
    source_artifact_id,
    source_id,
    source_collection_id,
    external_record_id,
    source_locator,
    source_marking
)
SELECT
    context.source_artifact_id,
    context.source_id,
    context.source_collection_id,
    input.external_record_id,
    input.source_locator,
    input.source_marking
FROM phase14_semantic_input AS input
CROSS JOIN phase14_batch_context AS context
ON CONFLICT (source_artifact_id, source_locator) DO NOTHING;

CREATE TEMP TABLE phase14_resolved_source_record
ON COMMIT DROP
AS
SELECT
    input.ordinal,
    record.source_record_id,
    record.source_artifact_id,
    record.source_collection_id,
    record.source_id
FROM phase14_semantic_input AS input
CROSS JOIN phase14_batch_context AS context
JOIN pathfinder.source_record AS record
  ON record.source_artifact_id = context.source_artifact_id
 AND record.source_locator = input.source_locator;

DO $source_record_guard$
BEGIN
    IF (
        SELECT count(*)
        FROM phase14_resolved_source_record
    ) <> (
        SELECT count(*)
        FROM phase14_semantic_input
    ) THEN
        RAISE EXCEPTION 'not every semantic input resolved to one SourceRecord';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM phase14_semantic_input AS input
        CROSS JOIN phase14_batch_context AS context
        JOIN pathfinder.source_record AS record
          ON record.source_artifact_id = context.source_artifact_id
         AND record.source_locator = input.source_locator
        WHERE record.source_id IS DISTINCT FROM context.source_id
           OR record.source_collection_id IS DISTINCT FROM context.source_collection_id
           OR record.external_record_id IS DISTINCT FROM input.external_record_id
           OR record.source_marking IS DISTINCT FROM input.source_marking
    ) THEN
        RAISE EXCEPTION 'immutable SourceRecord identity conflict';
    END IF;
END
$source_record_guard$;

INSERT INTO pathfinder.source_record_processing (
    processing_run_id,
    source_record_id,
    source_artifact_id,
    source_collection_id,
    source_id,
    validation_state,
    processing_state,
    processing_error_class
)
SELECT
    context.processing_run_id,
    record.source_record_id,
    record.source_artifact_id,
    record.source_collection_id,
    record.source_id,
    input.validation_state,
    input.processing_state,
    input.processing_error_class
FROM phase14_semantic_input AS input
JOIN phase14_resolved_source_record AS record
  ON record.ordinal = input.ordinal
CROSS JOIN phase14_batch_context AS context;

CREATE TEMP TABLE phase14_resolved_processing
ON COMMIT DROP
AS
SELECT
    input.ordinal,
    processing.source_record_processing_id
FROM phase14_semantic_input AS input
JOIN phase14_resolved_source_record AS record
  ON record.ordinal = input.ordinal
CROSS JOIN phase14_batch_context AS context
JOIN pathfinder.source_record_processing AS processing
  ON processing.processing_run_id = context.processing_run_id
 AND processing.source_record_id = record.source_record_id;

INSERT INTO pathfinder.vulnerability (
    identifier_namespace,
    identifier_value
)
SELECT DISTINCT
    'CVE',
    cve_identifier
FROM phase14_semantic_input
WHERE cve_identifier <> 'NOT_APPLICABLE'
ON CONFLICT (identifier_namespace, identifier_value) DO NOTHING;

CREATE TEMP TABLE phase14_resolved_vulnerability
ON COMMIT DROP
AS
SELECT
    input.ordinal,
    vulnerability.vulnerability_id
FROM phase14_semantic_input AS input
JOIN pathfinder.vulnerability AS vulnerability
  ON vulnerability.identifier_namespace = 'CVE'
 AND vulnerability.identifier_value = input.cve_identifier
WHERE input.cve_identifier <> 'NOT_APPLICABLE';

INSERT INTO pathfinder.assertion (
    source_record_processing_id,
    vulnerability_id,
    subject_type,
    assertion_type,
    asserted_value,
    extraction_method,
    extraction_version
)
SELECT
    processing.source_record_processing_id,
    vulnerability.vulnerability_id,
    'VULNERABILITY',
    'KNOWN_EXPLOITED',
    'TRUE',
    'SOURCE_STRUCTURED',
    context.extraction_version
FROM phase14_resolved_processing AS processing
JOIN phase14_resolved_vulnerability AS vulnerability
  ON vulnerability.ordinal = processing.ordinal
CROSS JOIN phase14_batch_context AS context;

INSERT INTO pathfinder.processing_event (
    processing_run_id,
    event_type,
    error_class
)
SELECT
    processing_run_id,
    'COMPLETED',
    'NOT_APPLICABLE'
FROM phase14_batch_context;

INSERT INTO pathfinder.collector_checkpoint (
    processing_run_id,
    processing_event_type,
    source_artifact_id,
    process_name,
    process_version,
    checkpoint_kind,
    source_checkpoint_value
)
SELECT
    processing_run_id,
    'COMPLETED',
    source_artifact_id,
    process_name,
    process_version,
    'FULL_SNAPSHOT',
    checkpoint_value
FROM phase14_batch_context;

SELECT json_build_object(
    'processing_run_id', context.processing_run_id::text,
    'source_records', (
        SELECT count(*) FROM phase14_resolved_source_record
    ),
    'record_processing_rows', (
        SELECT count(*) FROM phase14_resolved_processing
    ),
    'vulnerabilities', (
        SELECT count(*) FROM phase14_resolved_vulnerability
    ),
    'assertions', (
        SELECT count(*) FROM phase14_resolved_vulnerability
    ),
    'checkpoint', 'COMPLETED'
)::text
FROM phase14_batch_context AS context;

COMMIT;
`,
		delimiter,
		payloadJSON,
		delimiter,
		delimiter,
		payloadJSON,
		delimiter,
	), nil
}

// ValidateBatch proves application-level invariants before SQL is emitted.
func ValidateBatch(batch Batch) error {
	for name, value := range map[string]string{
		"checkpoint_value":     batch.CheckpointValue,
		"extraction_version":   batch.ExtractionVersion,
		"process_name":         batch.ProcessName,
		"process_version":      batch.ProcessVersion,
		"processing_run_id":    batch.ProcessingRunID,
		"source_artifact_id":   batch.SourceArtifactID,
		"source_collection_id": batch.SourceCollectionID,
		"source_id":            batch.SourceID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}

	locators := make(map[string]struct{}, len(batch.Records))
	ordinals := make(map[int]struct{}, len(batch.Records))

	for _, record := range batch.Records {
		if record.Ordinal < 0 {
			return fmt.Errorf("record ordinal must not be negative")
		}

		if _, exists := ordinals[record.Ordinal]; exists {
			return fmt.Errorf("duplicate record ordinal %d", record.Ordinal)
		}
		ordinals[record.Ordinal] = struct{}{}

		for name, value := range map[string]string{
			"external_record_id":     record.ExternalRecordID,
			"processing_error_class": record.ProcessingErrorClass,
			"processing_state":       record.ProcessingState,
			"source_locator":         record.SourceLocator,
			"source_marking":         record.SourceMarking,
			"validation_state":       record.ValidationState,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("record %d %s must not be empty", record.Ordinal, name)
			}
		}

		if _, exists := locators[record.SourceLocator]; exists {
			return fmt.Errorf("duplicate source_locator %q", record.SourceLocator)
		}
		locators[record.SourceLocator] = struct{}{}

		outcome := processinghistory.RecordOutcome{
			ProcessingErrorClass: record.ProcessingErrorClass,
			ProcessingRunID:      batch.ProcessingRunID,
			ProcessingState:      record.ProcessingState,
			SourceArtifactID:     batch.SourceArtifactID,
			SourceCollectionID:   batch.SourceCollectionID,
			SourceID:             batch.SourceID,
			SourceRecordID:       "validation-placeholder",
			ValidationState:      record.ValidationState,
		}
		if err := processinghistory.ValidateRecordOutcome(outcome); err != nil {
			return fmt.Errorf("record %d: %w", record.Ordinal, err)
		}

		if record.ProcessingState == processinghistory.ProcessingStateProcessed {
			if !cvePattern.MatchString(record.CVEIdentifier) {
				return fmt.Errorf(
					"record %d processed CVE identifier is invalid: %q",
					record.Ordinal,
					record.CVEIdentifier,
				)
			}
		} else if record.CVEIdentifier != NotApplicable {
			return fmt.Errorf(
				"record %d non-processed record requires cve_identifier=%s",
				record.Ordinal,
				NotApplicable,
			)
		}
	}

	return nil
}

func marshalDollarQuoted(value any) (string, string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", "", fmt.Errorf("marshal semantic SQL payload: %w", err)
	}

	for attempt := 0; attempt < 8; attempt++ {
		material := payload
		if attempt != 0 {
			material = append(
				[]byte(fmt.Sprintf("%d:", attempt)),
				payload...,
			)
		}

		digest := sha256.Sum256(material)
		delimiter := "$pf_" + hex.EncodeToString(digest[:]) + "$"
		if !strings.Contains(string(payload), delimiter) {
			return string(payload), delimiter, nil
		}
	}

	return "", "", fmt.Errorf("could not construct safe SQL dollar-quote delimiter")
}
