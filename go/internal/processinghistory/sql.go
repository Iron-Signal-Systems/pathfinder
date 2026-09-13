package processinghistory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type startRunPayload struct {
	ProcessName        string `json:"process_name"`
	ProcessVersion     string `json:"process_version"`
	ProcessingRunID    string `json:"processing_run_id"`
	RetrievalEventID   string `json:"retrieval_event_id"`
	SourceArtifactID   string `json:"source_artifact_id"`
	SourceCollectionID string `json:"source_collection_id"`
	SourceID           string `json:"source_id"`
}

type terminalEventPayload struct {
	ErrorClass      string `json:"error_class"`
	EventType       string `json:"event_type"`
	ProcessingRunID string `json:"processing_run_id"`
}

// BuildStartRunSQL returns one retry-safe append-only ProcessingRun insert.
// The caller owns the UUIDv7 identity so an ambiguous insert can be proven by
// exact read-back instead of losing the identity of the durable STARTED row.
func BuildStartRunSQL(run Run) (string, error) {
	if err := ValidateRun(run); err != nil {
		return "", err
	}

	payload := startRunPayload{
		ProcessName:        run.ProcessName,
		ProcessVersion:     run.ProcessVersion,
		ProcessingRunID:    run.ProcessingRunID,
		RetrievalEventID:   run.RetrievalEventID,
		SourceArtifactID:   run.SourceArtifactID,
		SourceCollectionID: run.SourceCollectionID,
		SourceID:           run.SourceID,
	}

	payloadJSON, delimiter, err := marshalDollarQuoted(payload)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
WITH input AS (
    SELECT *
    FROM jsonb_to_record(%s%s%s::jsonb) AS value(
        process_name text,
        process_version text,
        processing_run_id text,
        retrieval_event_id text,
        source_artifact_id text,
        source_collection_id text,
        source_id text
    )
)
INSERT INTO pathfinder.processing_run (
    processing_run_id,
    source_id,
    source_collection_id,
    retrieval_event_id,
    source_artifact_id,
    process_name,
    process_version
)
SELECT
    processing_run_id::uuid,
    source_id::uuid,
    source_collection_id::uuid,
    retrieval_event_id::uuid,
    source_artifact_id::uuid,
    process_name,
    process_version
FROM input
ON CONFLICT (processing_run_id) DO NOTHING
RETURNING json_build_object(
    'processing_run_id', processing_run_id::text,
    'source_id', source_id::text,
    'source_collection_id', source_collection_id::text,
    'retrieval_event_id', retrieval_event_id::text,
    'source_artifact_id', source_artifact_id::text,
    'process_name', process_name,
    'process_version', process_version
)::text;
`,
		delimiter,
		payloadJSON,
		delimiter,
	), nil
}

// BuildTerminalEventSQL returns the append-only SQL for FAILED or INTERRUPTED.
// COMPLETED is intentionally rejected because a successful terminal event must
// commit atomically with semantic rows and CollectorCheckpoint.
func BuildTerminalEventSQL(event TerminalEvent) (string, error) {
	if event.EventType == EventTypeCompleted {
		return "", fmt.Errorf(
			"COMPLETED must be committed by the semantic success transaction",
		)
	}

	if err := ValidateTerminalEvent(event); err != nil {
		return "", err
	}

	payload := terminalEventPayload{
		ErrorClass:      event.ErrorClass,
		EventType:       event.EventType,
		ProcessingRunID: event.ProcessingRunID,
	}

	payloadJSON, delimiter, err := marshalDollarQuoted(payload)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
WITH input AS (
    SELECT *
    FROM jsonb_to_record(%s%s%s::jsonb) AS value(
        error_class text,
        event_type text,
        processing_run_id text
    )
)
INSERT INTO pathfinder.processing_event (
    processing_run_id,
    event_type,
    error_class
)
SELECT
    processing_run_id::uuid,
    event_type,
    error_class
FROM input
RETURNING json_build_object(
    'processing_run_id', processing_run_id::text,
    'event_type', event_type,
    'error_class', error_class
)::text;
`,
		delimiter,
		payloadJSON,
		delimiter,
	), nil
}

func marshalDollarQuoted(value any) (string, string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", "", fmt.Errorf("marshal SQL JSON payload: %w", err)
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
