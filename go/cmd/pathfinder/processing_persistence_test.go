package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/collectors/cisakev"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/semantictransaction"
)

const persistenceTestRunID = "01990000-0000-7000-8000-000000000001"

type scriptedSQLResult struct {
	err    error
	output string
}

type scriptedSQLRunner struct {
	calls   int
	results []scriptedSQLResult
	sql     []string
	vars    [][]string
}

func (runner *scriptedSQLRunner) run(
	_ context.Context,
	_ Config,
	variables []string,
	sql string,
) (string, error) {
	runner.sql = append(runner.sql, sql)
	runner.vars = append(runner.vars, append([]string(nil), variables...))

	if runner.calls >= len(runner.results) {
		return "", fmt.Errorf("unexpected SQL call %d", runner.calls+1)
	}

	result := runner.results[runner.calls]
	runner.calls++
	return result.output, result.err
}

func TestRuntimeProcessingStartRunProvesAmbiguousInsert(t *testing.T) {
	t.Parallel()

	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{err: errors.New("connection lost after send")},
			{output: processingRunJSON()},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)

	run, err := persistence.StartRun(
		context.Background(),
		unassignedProcessingRun(),
	)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	if run.ProcessingRunID != persistenceTestRunID {
		t.Fatalf("ProcessingRunID = %q", run.ProcessingRunID)
	}
	if runner.calls != 2 {
		t.Fatalf("SQL calls = %d, want 2", runner.calls)
	}
	if !strings.Contains(runner.sql[0], "ON CONFLICT (processing_run_id) DO NOTHING") {
		t.Fatal("ProcessingRun insert is not retry-safe")
	}
}

func TestRuntimeProcessingStartRunRejectsMismatchedReadBack(t *testing.T) {
	t.Parallel()

	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{},
			{output: strings.Replace(
				processingRunJSON(),
				`"process_version":"1"`,
				`"process_version":"different"`,
				1,
			)},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)

	if _, err := persistence.StartRun(
		context.Background(),
		unassignedProcessingRun(),
	); err == nil {
		t.Fatal("StartRun() error = nil, want mismatched read-back rejection")
	}
}

func TestRuntimeProcessingAppendTerminalProvesAmbiguousInsert(t *testing.T) {
	t.Parallel()

	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{err: errors.New("connection lost after send")},
			{output: `{"processing_run_id":"` + persistenceTestRunID + `","event_type":"FAILED","error_class":"PARSER_FAILURE"}`},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)
	event := processinghistory.FailedEvent(
		persistenceTestRunID,
		"PARSER_FAILURE",
	)

	if err := persistence.AppendTerminal(
		context.Background(),
		event,
	); err != nil {
		t.Fatalf("AppendTerminal() error = %v", err)
	}
}

func TestRuntimeProcessingCommitSuccessProvesCheckpointAfterLostResponse(t *testing.T) {
	t.Parallel()

	batch := validPersistenceBatch()
	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{err: errors.New("connection lost after commit")},
			{output: checkpointJSON(batch)},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)

	if err := persistence.CommitSuccess(
		context.Background(),
		batch,
	); err != nil {
		t.Fatalf("CommitSuccess() error = %v", err)
	}
}

func TestRuntimeProcessingCommitSuccessLeavesUnprovenOutcomeAsError(t *testing.T) {
	t.Parallel()

	batch := validPersistenceBatch()
	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{err: errors.New("connection lost after commit")},
			{output: ""},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)

	if err := persistence.CommitSuccess(
		context.Background(),
		batch,
	); err == nil {
		t.Fatal("CommitSuccess() error = nil, want unresolved outcome")
	}
}

func TestRuntimeProcessingFindCompletedReturnsExactContract(t *testing.T) {
	t.Parallel()

	batch := validPersistenceBatch()
	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{output: checkpointJSON(batch)},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)

	completed, found, err := persistence.FindCompleted(
		context.Background(),
		batch.SourceArtifactID,
		batch.ProcessName,
		batch.ProcessVersion,
	)
	if err != nil {
		t.Fatalf("FindCompleted() error = %v", err)
	}
	if !found {
		t.Fatal("FindCompleted() found = false, want true")
	}

	expected := cisakev.CompletedArtifact{
		CheckpointKind:      semantictransaction.CheckpointKindFullSnapshot,
		CheckpointValue:     batch.CheckpointValue,
		ProcessName:         batch.ProcessName,
		ProcessVersion:      batch.ProcessVersion,
		ProcessingEventType: processinghistory.EventTypeCompleted,
		ProcessingRunID:     batch.ProcessingRunID,
		RecordCount:         0,
		SourceArtifactID:    batch.SourceArtifactID,
	}
	if completed != expected {
		t.Fatalf("FindCompleted() = %#v, want %#v", completed, expected)
	}
}

func TestRuntimeProcessingInterruptUnresolvedBefore(t *testing.T) {
	t.Parallel()

	run1 := "01990000-0000-7000-8000-000000000011"
	run2 := "01990000-0000-7000-8000-000000000012"

	runner := &scriptedSQLRunner{
		results: []scriptedSQLResult{
			{output: `{"processing_run_id":"` + run1 + `"}` + "\n" +
				`{"processing_run_id":"` + run2 + `"}`},
			{},
			{output: `{"processing_run_id":"` + run1 + `","event_type":"INTERRUPTED","error_class":"STALE_PROCESSING_RUN"}`},
			{},
			{output: `{"processing_run_id":"` + run2 + `","event_type":"INTERRUPTED","error_class":"STALE_PROCESSING_RUN"}`},
		},
	}

	persistence := testRuntimeProcessingPersistence(runner)

	interrupted, err := persistence.InterruptUnresolvedBefore(
		context.Background(),
		"cisakev",
		"1",
		time.Unix(100, 0),
	)
	if err != nil {
		t.Fatalf("InterruptUnresolvedBefore() error = %v", err)
	}

	if len(interrupted) != 2 ||
		interrupted[0] != run1 ||
		interrupted[1] != run2 {
		t.Fatalf("interrupted = %#v", interrupted)
	}

	if runner.calls != 5 {
		t.Fatalf("SQL calls = %d, want 5", runner.calls)
	}
}

func checkpointJSON(batch semantictransaction.Batch) string {
	return fmt.Sprintf(
		`{"processing_run_id":"%s","processing_event_type":"COMPLETED","source_artifact_id":"%s","process_name":"%s","process_version":"%s","checkpoint_kind":"FULL_SNAPSHOT","source_checkpoint_value":"%s","record_count":0}`,
		batch.ProcessingRunID,
		batch.SourceArtifactID,
		batch.ProcessName,
		batch.ProcessVersion,
		batch.CheckpointValue,
	)
}

func processingRunJSON() string {
	return `{"processing_run_id":"01990000-0000-7000-8000-000000000001","source_id":"01990000-0000-7000-8000-000000000005","source_collection_id":"01990000-0000-7000-8000-000000000004","retrieval_event_id":"01990000-0000-7000-8000-000000000002","source_artifact_id":"01990000-0000-7000-8000-000000000003","process_name":"cisakev","process_version":"1"}`
}

func testRuntimeProcessingPersistence(
	runner *scriptedSQLRunner,
) runtimeProcessingPersistence {
	return runtimeProcessingPersistence{
		cfg: Config{},
		newUUID: func(time.Time) (string, error) {
			return persistenceTestRunID, nil
		},
		now: func() time.Time {
			return time.Unix(0, 0)
		},
		runSQL: runner.run,
	}
}

func unassignedProcessingRun() processinghistory.Run {
	return processinghistory.Run{
		ProcessName:        "cisakev",
		ProcessVersion:     "1",
		RetrievalEventID:   "01990000-0000-7000-8000-000000000002",
		SourceArtifactID:   "01990000-0000-7000-8000-000000000003",
		SourceCollectionID: "01990000-0000-7000-8000-000000000004",
		SourceID:           "01990000-0000-7000-8000-000000000005",
	}
}

func validPersistenceBatch() semantictransaction.Batch {
	return semantictransaction.Batch{
		CheckpointValue:    "2026.09.13",
		ExtractionVersion:  "cisakev-parser-v1",
		ProcessName:        "cisakev",
		ProcessVersion:     "1",
		ProcessingRunID:    persistenceTestRunID,
		Records:            []semantictransaction.Record{},
		SourceArtifactID:   "01990000-0000-7000-8000-000000000003",
		SourceCollectionID: "01990000-0000-7000-8000-000000000004",
		SourceID:           "01990000-0000-7000-8000-000000000005",
	}
}
