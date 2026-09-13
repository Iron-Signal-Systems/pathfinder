package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/collectors/cisakev"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/processinghistory"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/semantictransaction"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

const sourceRecordConflictTestErrorClass = "SOURCE_RECORD_IDENTITY_CONFLICT_TEST"

func TestPhase14SourceRecordIdentityGuard(t *testing.T) {
	if os.Getenv("PATHFINDER_PHASE14_SOURCE_RECORD_GUARD") != "YES" {
		t.Skip("live SourceRecord identity guard requires PATHFINDER_PHASE14_SOURCE_RECORD_GUARD=YES")
	}

	configPath := strings.TrimSpace(os.Getenv("PATHFINDER_PHASE14_SOURCE_RECORD_GUARD_CONFIG"))
	if configPath == "" {
		t.Fatal("PATHFINDER_PHASE14_SOURCE_RECORD_GUARD_CONFIG must be set")
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load SourceRecord guard config: %v", err)
	}
	requireDisposableLifecycleConfig(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	now := time.Now()
	sourceID := mustLifecycleUUID(t, now)
	collectionID := mustLifecycleUUID(t, now.Add(time.Millisecond))
	retrievalID := mustLifecycleUUID(t, now.Add(2*time.Millisecond))
	artifactID := mustLifecycleUUID(t, now.Add(3*time.Millisecond))

	if err := createLifecycleSourceContext(ctx, cfg, sourceID, collectionID, retrievalID); err != nil {
		t.Fatalf("create SourceRecord guard source context: %v", err)
	}

	body := lifecycleKEVArtifact()
	commitResult, err := sourceartifact.Commit(
		ctx,
		cfg.ArtifactSocket,
		sourceartifact.CommitRequest{
			ContentEncoding:    "identity",
			HandlingProfile:    "PHASE_1_4_SOURCE_RECORD_GUARD_TEST",
			MediaType:          "application/json",
			RetrievalEventID:   retrievalID,
			SourceArtifactID:   artifactID,
			SourceCollectionID: collectionID,
			SourceID:           sourceID,
			SourceLocation:     "pathfinder-test://source-record-identity-guard",
		},
		bytes.NewReader(body),
		func(
			ctx context.Context,
			request sourceartifact.CommitRequest,
			receipt artifactpreserve.Receipt,
		) (sourceartifact.DatabaseCommitResult, error) {
			return commitSourceArtifactDatabase(ctx, cfg, request, receipt)
		},
	)
	if err != nil {
		t.Fatalf("preserve SourceRecord guard artifact: %v", err)
	}

	if err := completeLifecycleRetrieval(ctx, cfg, retrievalID, 2); err != nil {
		t.Fatalf("complete SourceRecord guard retrieval: %v", err)
	}

	artifactRecord := sourceartifact.Record{
		AvailabilityState: sourceartifact.AvailabilityAvailable,
		ByteLength:        commitResult.Receipt.ByteLength,
		SHA256:            commitResult.Receipt.SHA256,
		SourceArtifactID:  artifactID,
		StorageReference:  commitResult.Receipt.StorageReference,
	}

	persistence := newRuntimeProcessingPersistence(cfg)

	baseline, err := cisakev.ProcessPreservedArtifact(
		ctx,
		cisakev.ProcessRequest{
			Artifact: cisakev.ArtifactContext{
				Record:             artifactRecord,
				RetrievalEventID:   retrievalID,
				SourceCollectionID: collectionID,
				SourceID:           sourceID,
			},
			ArtifactRoot:      cfg.ArtifactsDir,
			ExtractionVersion: "source-record-guard-v1",
			ProcessName:       "source-record-guard",
			ProcessVersion:    "baseline",
		},
		persistence,
	)
	if err != nil {
		t.Fatalf("create baseline SourceRecords: %v", err)
	}
	if baseline.RecordCount != 2 {
		t.Fatalf("baseline RecordCount=%d, want 2", baseline.RecordCount)
	}

	beforeIdentity, err := readSourceRecordGuardIdentity(ctx, cfg, artifactID, "/vulnerabilities/0")
	if err != nil {
		t.Fatal(err)
	}
	if beforeIdentity != "CVE-2026-1234|NOT_KNOWN" {
		t.Fatalf("baseline identity=%q", beforeIdentity)
	}

	reuseRun, err := persistence.StartRun(ctx, processinghistory.Run{
		ProcessName:        "source-record-guard",
		ProcessVersion:     "reuse",
		RetrievalEventID:   retrievalID,
		SourceArtifactID:   artifactID,
		SourceCollectionID: collectionID,
		SourceID:           sourceID,
	})
	if err != nil {
		t.Fatalf("start exact-reuse run: %v", err)
	}

	if err := persistence.CommitSuccess(ctx, sourceRecordGuardBatch(
		reuseRun,
		artifactID,
		collectionID,
		sourceID,
		"reuse",
	)); err != nil {
		t.Fatalf("exact immutable SourceRecord reuse failed: %v", err)
	}

	if err := proveSourceRecordGuardRun(ctx, cfg, reuseRun.ProcessingRunID, "2|1|1"); err != nil {
		t.Fatalf("prove exact-reuse run: %v", err)
	}

	conflicts := []struct {
		name   string
		mutate func(*semantictransaction.Batch)
	}{
		{
			name: "external-record-id",
			mutate: func(batch *semantictransaction.Batch) {
				batch.Records[0].ExternalRecordID = "CVE-2026-9999"
			},
		},
		{
			name: "source-marking",
			mutate: func(batch *semantictransaction.Batch) {
				batch.Records[0].SourceMarking = "TLP:CLEAR"
			},
		},
	}

	for index, test := range conflicts {
		t.Run(test.name, func(t *testing.T) {
			version := fmt.Sprintf("conflict-%d", index+1)
			run, err := persistence.StartRun(ctx, processinghistory.Run{
				ProcessName:        "source-record-guard",
				ProcessVersion:     version,
				RetrievalEventID:   retrievalID,
				SourceArtifactID:   artifactID,
				SourceCollectionID: collectionID,
				SourceID:           sourceID,
			})
			if err != nil {
				t.Fatalf("start conflict run: %v", err)
			}

			batch := sourceRecordGuardBatch(run, artifactID, collectionID, sourceID, version)
			test.mutate(&batch)

			err = persistence.CommitSuccess(ctx, batch)
			if err == nil {
				t.Fatal("conflicting immutable SourceRecord metadata unexpectedly committed")
			}
			if !strings.Contains(err.Error(), "immutable SourceRecord identity conflict") {
				t.Fatalf("conflict error=%v", err)
			}

			if err := proveSourceRecordGuardRun(ctx, cfg, run.ProcessingRunID, "0|0|0"); err != nil {
				t.Fatalf("prove conflict rollback: %v", err)
			}

			if err := persistence.AppendTerminal(
				ctx,
				processinghistory.FailedEvent(run.ProcessingRunID, sourceRecordConflictTestErrorClass),
			); err != nil {
				t.Fatalf("close rejected conflict run: %v", err)
			}
		})
	}

	afterIdentity, err := readSourceRecordGuardIdentity(ctx, cfg, artifactID, "/vulnerabilities/0")
	if err != nil {
		t.Fatal(err)
	}
	if afterIdentity != beforeIdentity {
		t.Fatalf("SourceRecord identity changed: before=%q after=%q", beforeIdentity, afterIdentity)
	}

	counts, err := sourceRecordGuardCounts(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if counts != "4|4|4|2|1|2|2" {
		t.Fatalf("final guard counts=%q, want 4|4|4|2|1|2|2", counts)
	}

	t.Logf("source_artifact_id=%s", artifactID)
	t.Logf("baseline_processing_run_id=%s", baseline.ProcessingRunID)
	t.Logf("final_counts=%s", counts)
	t.Log("Phase 1.4 immutable SourceRecord identity guard: PASS")
}

func sourceRecordGuardBatch(
	run processinghistory.Run,
	artifactID string,
	collectionID string,
	sourceID string,
	processVersion string,
) semantictransaction.Batch {
	return semantictransaction.Batch{
		CheckpointValue:   phase14LifecycleCheckpointValue,
		ExtractionVersion: "source-record-guard-v1",
		ProcessName:       run.ProcessName,
		ProcessVersion:    processVersion,
		ProcessingRunID:   run.ProcessingRunID,
		Records: []semantictransaction.Record{
			{
				CVEIdentifier:        "CVE-2026-1234",
				ExternalRecordID:     "CVE-2026-1234",
				Ordinal:              0,
				ProcessingErrorClass: processinghistory.ErrorClassNotApplicable,
				ProcessingState:      processinghistory.ProcessingStateProcessed,
				SourceLocator:        "/vulnerabilities/0",
				SourceMarking:        semantictransaction.SourceMarkingNotKnown,
				ValidationState:      processinghistory.ValidationStateValid,
			},
			{
				CVEIdentifier:        semantictransaction.NotApplicable,
				ExternalRecordID:     "NOT-A-CVE",
				Ordinal:              1,
				ProcessingErrorClass: "INVALID_CVE_IDENTIFIER",
				ProcessingState:      processinghistory.ProcessingStateFailed,
				SourceLocator:        "/vulnerabilities/1",
				SourceMarking:        semantictransaction.SourceMarkingNotKnown,
				ValidationState:      processinghistory.ValidationStateInvalid,
			},
		},
		SourceArtifactID:   artifactID,
		SourceCollectionID: collectionID,
		SourceID:           sourceID,
	}
}

func proveSourceRecordGuardRun(
	ctx context.Context,
	cfg Config,
	processingRunID string,
	expected string,
) error {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{"processing_run_id=" + processingRunID},
		`
SELECT
    (SELECT count(*)
       FROM pathfinder.source_record_processing
      WHERE processing_run_id = :'processing_run_id'::uuid)::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.assertion AS assertion
       JOIN pathfinder.source_record_processing AS processing
         ON processing.source_record_processing_id = assertion.source_record_processing_id
      WHERE processing.processing_run_id = :'processing_run_id'::uuid)::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.collector_checkpoint
      WHERE processing_run_id = :'processing_run_id'::uuid)::text;
`,
	)
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != expected {
		return fmt.Errorf("run proof=%q, want %q", strings.TrimSpace(output), expected)
	}
	return nil
}

func readSourceRecordGuardIdentity(
	ctx context.Context,
	cfg Config,
	artifactID string,
	locator string,
) (string, error) {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{
			"source_artifact_id=" + artifactID,
			"source_locator=" + locator,
		},
		`
SELECT external_record_id || '|' || source_marking
FROM pathfinder.source_record
WHERE source_artifact_id = :'source_artifact_id'::uuid
  AND source_locator = :'source_locator';
`,
	)
	if err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" || strings.Contains(output, "\n") {
		return "", fmt.Errorf("SourceRecord identity proof=%q", output)
	}
	return output, nil
}

func sourceRecordGuardCounts(ctx context.Context, cfg Config) (string, error) {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		nil,
		`
SELECT
    (SELECT count(*) FROM pathfinder.processing_run) || '|' ||
    (SELECT count(*) FROM pathfinder.processing_event) || '|' ||
    (SELECT count(*) FROM pathfinder.source_record_processing) || '|' ||
    (SELECT count(*) FROM pathfinder.source_record) || '|' ||
    (SELECT count(*) FROM pathfinder.vulnerability) || '|' ||
    (SELECT count(*) FROM pathfinder.assertion) || '|' ||
    (SELECT count(*) FROM pathfinder.collector_checkpoint);
`,
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
