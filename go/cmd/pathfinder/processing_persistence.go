package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/collectors/cisakev"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/semantictransaction"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

type runtimeProcessingPersistence struct {
	cfg     Config
	newUUID func(time.Time) (string, error)
	now     func() time.Time
	runSQL  func(context.Context, Config, []string, string) (string, error)
}

const ErrorClassStaleProcessingRun = "STALE_PROCESSING_RUN"

type completedCheckpointProof struct {
	CheckpointKind        string `json:"checkpoint_kind"`
	ProcessName           string `json:"process_name"`
	ProcessVersion        string `json:"process_version"`
	ProcessingEventType   string `json:"processing_event_type"`
	ProcessingRunID       string `json:"processing_run_id"`
	RecordCount           int    `json:"record_count"`
	SourceArtifactID      string `json:"source_artifact_id"`
	SourceCheckpointValue string `json:"source_checkpoint_value"`
}

type terminalEventProof struct {
	ErrorClass      string `json:"error_class"`
	EventType       string `json:"event_type"`
	ProcessingRunID string `json:"processing_run_id"`
}

func newRuntimeProcessingPersistence(
	cfg Config,
) runtimeProcessingPersistence {
	return runtimeProcessingPersistence{
		cfg:     cfg,
		newUUID: sourceartifact.NewUUIDv7,
		now:     time.Now,
		runSQL:  runRuntimePSQL,
	}
}

func (p runtimeProcessingPersistence) FindCompleted(
	ctx context.Context,
	sourceArtifactID string,
	processName string,
	processVersion string,
) (cisakev.CompletedArtifact, bool, error) {
	for name, value := range map[string]string{
		"source_artifact_id": sourceArtifactID,
		"process_name":       processName,
		"process_version":    processVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return cisakev.CompletedArtifact{}, false, fmt.Errorf(
				"%s must not be empty",
				name,
			)
		}
	}

	output, err := p.runSQL(
		ctx,
		p.cfg,
		[]string{
			"source_artifact_id=" + sourceArtifactID,
			"process_name=" + processName,
			"process_version=" + processVersion,
		},
		`
SELECT json_build_object(
    'processing_run_id', checkpoint.processing_run_id::text,
    'processing_event_type', checkpoint.processing_event_type,
    'source_artifact_id', checkpoint.source_artifact_id::text,
    'process_name', checkpoint.process_name,
    'process_version', checkpoint.process_version,
    'checkpoint_kind', checkpoint.checkpoint_kind,
    'source_checkpoint_value', checkpoint.source_checkpoint_value,
    'record_count', (
        SELECT count(*)
        FROM pathfinder.source_record_processing AS processing
        WHERE processing.processing_run_id = checkpoint.processing_run_id
    )
)::text
FROM pathfinder.collector_checkpoint AS checkpoint
WHERE checkpoint.source_artifact_id = :'source_artifact_id'::uuid
  AND checkpoint.process_name = :'process_name'
  AND checkpoint.process_version = :'process_version';
`,
	)
	if err != nil {
		return cisakev.CompletedArtifact{}, false, err
	}

	proof, found, err := decodeCompletedCheckpointProof(output)
	if err != nil {
		return cisakev.CompletedArtifact{}, false, err
	}
	if !found {
		return cisakev.CompletedArtifact{}, false, nil
	}

	return cisakev.CompletedArtifact{
		CheckpointKind:      proof.CheckpointKind,
		CheckpointValue:     proof.SourceCheckpointValue,
		ProcessName:         proof.ProcessName,
		ProcessVersion:      proof.ProcessVersion,
		ProcessingEventType: proof.ProcessingEventType,
		ProcessingRunID:     proof.ProcessingRunID,
		RecordCount:         proof.RecordCount,
		SourceArtifactID:    proof.SourceArtifactID,
	}, true, nil
}

func (p runtimeProcessingPersistence) AppendTerminal(
	ctx context.Context,
	event processinghistory.TerminalEvent,
) error {
	sql, err := processinghistory.BuildTerminalEventSQL(event)
	if err != nil {
		return err
	}

	_, executeErr := p.runSQL(ctx, p.cfg, nil, sql)

	proof, found, readErr := p.readTerminalEvent(
		ctx,
		event.ProcessingRunID,
	)
	if readErr == nil && found && terminalEventMatches(proof, event) {
		return nil
	}

	if executeErr != nil {
		if readErr != nil {
			return fmt.Errorf(
				"append terminal event failed (%v); read-back failed (%v)",
				executeErr,
				readErr,
			)
		}

		if found {
			return fmt.Errorf(
				"append terminal event failed (%v); existing terminal event does not match requested event",
				executeErr,
			)
		}

		return fmt.Errorf(
			"append terminal event failed and no matching row was proven: %w",
			executeErr,
		)
	}

	if readErr != nil {
		return fmt.Errorf(
			"terminal event insert outcome not established; read-back failed: %w",
			readErr,
		)
	}

	if found {
		return fmt.Errorf(
			"processing run already has a different terminal event",
		)
	}

	return fmt.Errorf(
		"terminal event insert returned without a provable terminal row",
	)
}

func (p runtimeProcessingPersistence) CommitSuccess(
	ctx context.Context,
	batch semantictransaction.Batch,
) error {
	sql, err := semantictransaction.BuildSuccessSQL(batch)
	if err != nil {
		return err
	}

	_, executeErr := p.runSQL(ctx, p.cfg, nil, sql)

	proof, found, readErr := p.readCompletedCheckpoint(
		ctx,
		batch.ProcessingRunID,
	)
	if readErr == nil && found && checkpointMatches(proof, batch) {
		return nil
	}

	if executeErr != nil {
		if readErr != nil {
			return fmt.Errorf(
				"semantic transaction failed (%v); checkpoint read-back failed (%v)",
				executeErr,
				readErr,
			)
		}

		if found {
			return fmt.Errorf(
				"semantic transaction failed (%v); existing checkpoint does not match requested contract",
				executeErr,
			)
		}

		return fmt.Errorf(
			"semantic transaction outcome unresolved and no matching checkpoint was proven: %w",
			executeErr,
		)
	}

	if readErr != nil {
		return fmt.Errorf(
			"semantic transaction returned success but checkpoint read-back failed: %w",
			readErr,
		)
	}

	if found {
		return fmt.Errorf(
			"semantic transaction returned success but checkpoint does not match requested contract",
		)
	}

	return fmt.Errorf(
		"semantic transaction returned success but no completed checkpoint was proven",
	)
}

func (p runtimeProcessingPersistence) InterruptUnresolvedBefore(
	ctx context.Context,
	processName string,
	processVersion string,
	startedBefore time.Time,
) ([]string, error) {
	if strings.TrimSpace(processName) == "" {
		return nil, fmt.Errorf("process_name must not be empty")
	}
	if strings.TrimSpace(processVersion) == "" {
		return nil, fmt.Errorf("process_version must not be empty")
	}
	if startedBefore.IsZero() {
		return nil, fmt.Errorf("started_before must not be zero")
	}

	output, err := p.runSQL(
		ctx,
		p.cfg,
		[]string{
			"process_name=" + processName,
			"process_version=" + processVersion,
			"started_before=" + startedBefore.UTC().Format(time.RFC3339Nano),
		},
		`
SELECT json_build_object(
    'processing_run_id', run.processing_run_id::text
)::text
FROM pathfinder.processing_run AS run
WHERE run.process_name = :'process_name'
  AND run.process_version = :'process_version'
  AND run.started_at < :'started_before'::timestamptz
  AND NOT EXISTS (
      SELECT 1
      FROM pathfinder.processing_event AS event
      WHERE event.processing_run_id = run.processing_run_id
  )
ORDER BY run.started_at, run.processing_run_id;
`,
	)
	if err != nil {
		return nil, fmt.Errorf("query unresolved ProcessingRuns: %w", err)
	}

	runIDs, err := decodeProcessingRunIDs(output)
	if err != nil {
		return nil, err
	}

	interrupted := make([]string, 0, len(runIDs))
	for _, processingRunID := range runIDs {
		event := processinghistory.InterruptedEvent(
			processingRunID,
			ErrorClassStaleProcessingRun,
		)
		if err := p.AppendTerminal(ctx, event); err != nil {
			return interrupted, fmt.Errorf(
				"interrupt unresolved ProcessingRun %s: %w",
				processingRunID,
				err,
			)
		}

		interrupted = append(interrupted, processingRunID)
	}

	return interrupted, nil
}

func (p runtimeProcessingPersistence) StartRun(
	ctx context.Context,
	run processinghistory.Run,
) (processinghistory.Run, error) {
	if strings.TrimSpace(run.ProcessingRunID) != "" {
		return processinghistory.Run{}, fmt.Errorf(
			"StartRun requires an unassigned processing_run_id",
		)
	}

	processingRunID, err := p.newUUID(p.now())
	if err != nil {
		return processinghistory.Run{}, fmt.Errorf(
			"generate ProcessingRun UUIDv7: %w",
			err,
		)
	}
	run.ProcessingRunID = processingRunID

	sql, err := processinghistory.BuildStartRunSQL(run)
	if err != nil {
		return processinghistory.Run{}, err
	}

	_, executeErr := p.runSQL(ctx, p.cfg, nil, sql)

	proof, found, readErr := p.readProcessingRun(
		ctx,
		run.ProcessingRunID,
	)
	if readErr == nil && found && proof == run {
		return run, nil
	}

	if executeErr != nil {
		if readErr != nil {
			return processinghistory.Run{}, fmt.Errorf(
				"create ProcessingRun failed (%v); read-back failed (%v)",
				executeErr,
				readErr,
			)
		}

		if found {
			return processinghistory.Run{}, fmt.Errorf(
				"create ProcessingRun failed (%v); existing processing_run_id does not match requested context",
				executeErr,
			)
		}

		return processinghistory.Run{}, fmt.Errorf(
			"create ProcessingRun failed and no matching row was proven: %w",
			executeErr,
		)
	}

	if readErr != nil {
		return processinghistory.Run{}, fmt.Errorf(
			"ProcessingRun insert outcome not established; read-back failed: %w",
			readErr,
		)
	}

	if found {
		return processinghistory.Run{}, fmt.Errorf(
			"processing_run_id already exists with different context",
		)
	}

	return processinghistory.Run{}, fmt.Errorf(
		"ProcessingRun insert returned without a provable row",
	)
}

func (p runtimeProcessingPersistence) readCompletedCheckpoint(
	ctx context.Context,
	processingRunID string,
) (completedCheckpointProof, bool, error) {
	output, err := p.runSQL(
		ctx,
		p.cfg,
		[]string{"processing_run_id=" + processingRunID},
		`
SELECT json_build_object(
    'processing_run_id', checkpoint.processing_run_id::text,
    'processing_event_type', checkpoint.processing_event_type,
    'source_artifact_id', checkpoint.source_artifact_id::text,
    'process_name', checkpoint.process_name,
    'process_version', checkpoint.process_version,
    'checkpoint_kind', checkpoint.checkpoint_kind,
    'source_checkpoint_value', checkpoint.source_checkpoint_value,
    'record_count', (
        SELECT count(*)
        FROM pathfinder.source_record_processing AS processing
        WHERE processing.processing_run_id = checkpoint.processing_run_id
    )
)::text
FROM pathfinder.collector_checkpoint AS checkpoint
WHERE checkpoint.processing_run_id = :'processing_run_id'::uuid;
`,
	)
	if err != nil {
		return completedCheckpointProof{}, false, err
	}

	return decodeCompletedCheckpointProof(output)
}

func (p runtimeProcessingPersistence) readProcessingRun(
	ctx context.Context,
	processingRunID string,
) (processinghistory.Run, bool, error) {
	output, err := p.runSQL(
		ctx,
		p.cfg,
		[]string{"processing_run_id=" + processingRunID},
		`
SELECT json_build_object(
    'processing_run_id', processing_run_id::text,
    'source_id', source_id::text,
    'source_collection_id', source_collection_id::text,
    'retrieval_event_id', retrieval_event_id::text,
    'source_artifact_id', source_artifact_id::text,
    'process_name', process_name,
    'process_version', process_version
)::text
FROM pathfinder.processing_run
WHERE processing_run_id = :'processing_run_id'::uuid;
`,
	)
	if err != nil {
		return processinghistory.Run{}, false, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return processinghistory.Run{}, false, nil
	}

	var run struct {
		ProcessName        string `json:"process_name"`
		ProcessVersion     string `json:"process_version"`
		ProcessingRunID    string `json:"processing_run_id"`
		RetrievalEventID   string `json:"retrieval_event_id"`
		SourceArtifactID   string `json:"source_artifact_id"`
		SourceCollectionID string `json:"source_collection_id"`
		SourceID           string `json:"source_id"`
	}
	if err := json.Unmarshal([]byte(output), &run); err != nil {
		return processinghistory.Run{}, false, fmt.Errorf(
			"decode ProcessingRun read-back: %w",
			err,
		)
	}

	return processinghistory.Run{
		ProcessName:        run.ProcessName,
		ProcessVersion:     run.ProcessVersion,
		ProcessingRunID:    run.ProcessingRunID,
		RetrievalEventID:   run.RetrievalEventID,
		SourceArtifactID:   run.SourceArtifactID,
		SourceCollectionID: run.SourceCollectionID,
		SourceID:           run.SourceID,
	}, true, nil
}

func (p runtimeProcessingPersistence) readTerminalEvent(
	ctx context.Context,
	processingRunID string,
) (terminalEventProof, bool, error) {
	output, err := p.runSQL(
		ctx,
		p.cfg,
		[]string{"processing_run_id=" + processingRunID},
		`
SELECT json_build_object(
    'processing_run_id', processing_run_id::text,
    'event_type', event_type,
    'error_class', error_class
)::text
FROM pathfinder.processing_event
WHERE processing_run_id = :'processing_run_id'::uuid;
`,
	)
	if err != nil {
		return terminalEventProof{}, false, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return terminalEventProof{}, false, nil
	}

	var proof terminalEventProof
	if err := json.Unmarshal([]byte(output), &proof); err != nil {
		return terminalEventProof{}, false, fmt.Errorf(
			"decode ProcessingEvent read-back: %w",
			err,
		)
	}

	return proof, true, nil
}

func decodeCompletedCheckpointProof(
	output string,
) (completedCheckpointProof, bool, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return completedCheckpointProof{}, false, nil
	}

	if strings.Contains(output, "\n") {
		return completedCheckpointProof{}, false, fmt.Errorf(
			"multiple CollectorCheckpoint rows returned for unique contract",
		)
	}

	var proof completedCheckpointProof
	if err := json.Unmarshal([]byte(output), &proof); err != nil {
		return completedCheckpointProof{}, false, fmt.Errorf(
			"decode CollectorCheckpoint read-back: %w",
			err,
		)
	}
	if proof.RecordCount < 0 {
		return completedCheckpointProof{}, false, fmt.Errorf(
			"CollectorCheckpoint record_count must not be negative",
		)
	}

	return proof, true, nil
}

func decodeProcessingRunIDs(output string) ([]string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(output, "\n")
	runIDs := make([]string, 0, len(lines))

	for _, line := range lines {
		var value struct {
			ProcessingRunID string `json:"processing_run_id"`
		}
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			return nil, fmt.Errorf(
				"decode unresolved ProcessingRun: %w",
				err,
			)
		}
		if strings.TrimSpace(value.ProcessingRunID) == "" {
			return nil, fmt.Errorf(
				"unresolved ProcessingRun returned empty processing_run_id",
			)
		}

		runIDs = append(runIDs, value.ProcessingRunID)
	}

	return runIDs, nil
}

func checkpointMatches(
	proof completedCheckpointProof,
	batch semantictransaction.Batch,
) bool {
	return proof.CheckpointKind == semantictransaction.CheckpointKindFullSnapshot &&
		proof.ProcessName == batch.ProcessName &&
		proof.ProcessVersion == batch.ProcessVersion &&
		proof.ProcessingEventType == processinghistory.EventTypeCompleted &&
		proof.ProcessingRunID == batch.ProcessingRunID &&
		proof.SourceArtifactID == batch.SourceArtifactID &&
		proof.SourceCheckpointValue == batch.CheckpointValue
}

func terminalEventMatches(
	proof terminalEventProof,
	event processinghistory.TerminalEvent,
) bool {
	return proof.ErrorClass == event.ErrorClass &&
		proof.EventType == event.EventType &&
		proof.ProcessingRunID == event.ProcessingRunID
}
