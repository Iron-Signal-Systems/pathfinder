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
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

const (
	phase14ResilienceCrossProcessName    = "cisakev-cross-artifact-test"
	phase14ResilienceCrossProcessVersion = "1"
	phase14ResilienceStaleProcessVersion = "stale-1"
)

type phase14PreservedArtifact struct {
	ArtifactID  string
	Record      sourceartifact.Record
	RetrievalID string
}

func TestPhase14LiveResilience(t *testing.T) {
	if os.Getenv("PATHFINDER_PHASE14_RESILIENCE") != "YES" {
		t.Skip("live Phase 1.4 resilience test requires PATHFINDER_PHASE14_RESILIENCE=YES")
	}

	configPath := strings.TrimSpace(
		os.Getenv("PATHFINDER_PHASE14_RESILIENCE_CONFIG"),
	)
	if configPath == "" {
		t.Fatal("PATHFINDER_PHASE14_RESILIENCE_CONFIG must be set")
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load resilience config: %v", err)
	}
	requireDisposableLifecycleConfig(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	now := time.Now()
	sourceID := mustLifecycleUUID(t, now)
	collectionID := mustLifecycleUUID(t, now.Add(time.Millisecond))
	successRetrievalID := mustLifecycleUUID(t, now.Add(2*time.Millisecond))

	if err := createLifecycleSourceContext(
		ctx,
		cfg,
		sourceID,
		collectionID,
		successRetrievalID,
	); err != nil {
		t.Fatalf("create resilience source context: %v", err)
	}

	persistence := newRuntimeProcessingPersistence(cfg)

	t.Run("completed_retry_is_idempotent", func(t *testing.T) {
		success := preservePhase14ResilienceArtifact(
			t,
			ctx,
			cfg,
			sourceID,
			collectionID,
			successRetrievalID,
			lifecycleKEVArtifact(),
			2,
			"pathfinder-test://cisa-kev/resilience-success",
		)

		request := cisakev.ProcessRequest{
			Artifact: cisakev.ArtifactContext{
				Record:             success.Record,
				RetrievalEventID:   success.RetrievalID,
				SourceCollectionID: collectionID,
				SourceID:           sourceID,
			},
			ArtifactRoot:      cfg.ArtifactsDir,
			ExtractionVersion: phase14LifecycleExtractionVersion,
			ProcessName:       phase14LifecycleProcessName,
			ProcessVersion:    phase14LifecycleProcessVersion,
		}

		first, err := cisakev.ProcessPreservedArtifact(
			ctx,
			request,
			persistence,
		)
		if err != nil {
			t.Fatalf("first processing attempt: %v", err)
		}
		if first.AlreadyCompleted {
			t.Fatal("first processing attempt unexpectedly reported AlreadyCompleted")
		}

		before := resilienceContractCounts(
			t,
			ctx,
			cfg,
			success.ArtifactID,
			phase14LifecycleProcessName,
			phase14LifecycleProcessVersion,
		)

		second, err := cisakev.ProcessPreservedArtifact(
			ctx,
			request,
			persistence,
		)
		if err != nil {
			t.Fatalf("completed retry: %v", err)
		}
		if !second.AlreadyCompleted {
			t.Fatal("completed retry did not report AlreadyCompleted")
		}
		if second.ProcessingRunID != first.ProcessingRunID {
			t.Fatalf(
				"completed retry processing_run_id=%q, want original %q",
				second.ProcessingRunID,
				first.ProcessingRunID,
			)
		}
		if second.CheckpointValue != first.CheckpointValue {
			t.Fatalf(
				"completed retry checkpoint=%q, want %q",
				second.CheckpointValue,
				first.CheckpointValue,
			)
		}
		if second.RecordCount != first.RecordCount {
			t.Fatalf(
				"completed retry record_count=%d, want %d",
				second.RecordCount,
				first.RecordCount,
			)
		}

		after := resilienceContractCounts(
			t,
			ctx,
			cfg,
			success.ArtifactID,
			phase14LifecycleProcessName,
			phase14LifecycleProcessVersion,
		)

		if before != after {
			t.Fatalf(
				"completed retry changed contract counts: before=%q after=%q",
				before,
				after,
			)
		}
		if after != "1|1|2|2|1" {
			t.Fatalf(
				"completed contract counts=%q, want 1|1|2|2|1",
				after,
			)
		}

		t.Logf("completed_processing_run_id=%s", first.ProcessingRunID)
		t.Log("completed retry idempotency: PASS")
	})

	var malformed phase14PreservedArtifact
	var malformedRunID string

	t.Run("malformed_artifact_records_failed_without_semantics", func(t *testing.T) {
		retrievalID := createPhase14ResilienceRetrieval(
			t,
			ctx,
			cfg,
			sourceID,
			collectionID,
			now.Add(10*time.Millisecond),
		)

		malformed = preservePhase14ResilienceArtifact(
			t,
			ctx,
			cfg,
			sourceID,
			collectionID,
			retrievalID,
			[]byte(`{"catalogVersion":"broken","vulnerabilities":[`),
			0,
			"pathfinder-test://cisa-kev/resilience-malformed",
		)

		_, err := cisakev.ProcessPreservedArtifact(
			ctx,
			cisakev.ProcessRequest{
				Artifact: cisakev.ArtifactContext{
					Record:             malformed.Record,
					RetrievalEventID:   malformed.RetrievalID,
					SourceCollectionID: collectionID,
					SourceID:           sourceID,
				},
				ArtifactRoot:      cfg.ArtifactsDir,
				ExtractionVersion: phase14LifecycleExtractionVersion,
				ProcessName:       phase14LifecycleProcessName,
				ProcessVersion:    phase14LifecycleProcessVersion,
			},
			persistence,
		)
		if err == nil {
			t.Fatal("malformed artifact processing returned nil error")
		}

		malformedRunID = resilienceProcessingRunID(
			t,
			ctx,
			cfg,
			malformed.ArtifactID,
			phase14LifecycleProcessName,
			phase14LifecycleProcessVersion,
		)

		proof := resilienceMalformedProof(
			t,
			ctx,
			cfg,
			malformed.ArtifactID,
			malformedRunID,
		)
		if proof != "1|1|0|0|0" {
			t.Fatalf(
				"malformed artifact proof=%q, want 1|1|0|0|0",
				proof,
			)
		}

		t.Logf("malformed_processing_run_id=%s", malformedRunID)
		t.Log("malformed artifact FAILED/zero semantics: PASS")
	})

	var successArtifactID string
	var successRetrievalIDForStale string

	t.Run("stale_unresolved_run_is_interrupted_then_retry_uses_new_uuid", func(t *testing.T) {
		successArtifactID = resilienceCompletedArtifactID(
			t,
			ctx,
			cfg,
			phase14LifecycleProcessName,
			phase14LifecycleProcessVersion,
		)
		successRetrievalIDForStale = resilienceArtifactRetrievalID(
			t,
			ctx,
			cfg,
			successArtifactID,
		)

		successRecord := resilienceArtifactRecord(
			t,
			ctx,
			cfg,
			successArtifactID,
		)

		staleRun, err := persistence.StartRun(
			ctx,
			processinghistory.Run{
				ProcessName:        phase14LifecycleProcessName,
				ProcessVersion:     phase14ResilienceStaleProcessVersion,
				RetrievalEventID:   successRetrievalIDForStale,
				SourceArtifactID:   successArtifactID,
				SourceCollectionID: collectionID,
				SourceID:           sourceID,
			},
		)
		if err != nil {
			t.Fatalf("create stale ProcessingRun: %v", err)
		}

		interrupted, err := persistence.InterruptUnresolvedBefore(
			ctx,
			phase14LifecycleProcessName,
			phase14ResilienceStaleProcessVersion,
			time.Now().Add(time.Minute),
		)
		if err != nil {
			t.Fatalf("interrupt stale ProcessingRun: %v", err)
		}
		if len(interrupted) != 1 || interrupted[0] != staleRun.ProcessingRunID {
			t.Fatalf(
				"interrupted=%#v, want [%s]",
				interrupted,
				staleRun.ProcessingRunID,
			)
		}

		eventProof := resilienceTerminalProof(
			t,
			ctx,
			cfg,
			staleRun.ProcessingRunID,
		)
		if eventProof != "INTERRUPTED|STALE_PROCESSING_RUN" {
			t.Fatalf(
				"stale event proof=%q, want INTERRUPTED|STALE_PROCESSING_RUN",
				eventProof,
			)
		}

		retry, err := cisakev.ProcessPreservedArtifact(
			ctx,
			cisakev.ProcessRequest{
				Artifact: cisakev.ArtifactContext{
					Record:             successRecord,
					RetrievalEventID:   successRetrievalIDForStale,
					SourceCollectionID: collectionID,
					SourceID:           sourceID,
				},
				ArtifactRoot:      cfg.ArtifactsDir,
				ExtractionVersion: phase14LifecycleExtractionVersion,
				ProcessName:       phase14LifecycleProcessName,
				ProcessVersion:    phase14ResilienceStaleProcessVersion,
			},
			persistence,
		)
		if err != nil {
			t.Fatalf("retry after interruption: %v", err)
		}
		if retry.AlreadyCompleted {
			t.Fatal("fresh retry unexpectedly reported AlreadyCompleted")
		}
		if retry.ProcessingRunID == staleRun.ProcessingRunID {
			t.Fatal("retry reused interrupted ProcessingRun identity")
		}

		retryProof := resilienceTerminalProof(
			t,
			ctx,
			cfg,
			retry.ProcessingRunID,
		)
		if retryProof != "COMPLETED|NOT_APPLICABLE" {
			t.Fatalf(
				"retry event proof=%q, want COMPLETED|NOT_APPLICABLE",
				retryProof,
			)
		}

		t.Logf("stale_processing_run_id=%s", staleRun.ProcessingRunID)
		t.Logf("retry_processing_run_id=%s", retry.ProcessingRunID)
		t.Log("stale interruption + new UUID retry: PASS")
	})

	t.Run("cross_artifact_source_record_processing_is_rejected", func(t *testing.T) {
		retrievalID := createPhase14ResilienceRetrieval(
			t,
			ctx,
			cfg,
			sourceID,
			collectionID,
			now.Add(20*time.Millisecond),
		)

		cross := preservePhase14ResilienceArtifact(
			t,
			ctx,
			cfg,
			sourceID,
			collectionID,
			retrievalID,
			[]byte(`{"catalogVersion":"cross","vulnerabilities":[]}`),
			0,
			"pathfinder-test://cisa-kev/resilience-cross-artifact",
		)

		crossRun, err := persistence.StartRun(
			ctx,
			processinghistory.Run{
				ProcessName:        phase14ResilienceCrossProcessName,
				ProcessVersion:     phase14ResilienceCrossProcessVersion,
				RetrievalEventID:   cross.RetrievalID,
				SourceArtifactID:   cross.ArtifactID,
				SourceCollectionID: collectionID,
				SourceID:           sourceID,
			},
		)
		if err != nil {
			t.Fatalf("create cross-artifact ProcessingRun: %v", err)
		}

		_, insertErr := runRuntimePSQL(
			ctx,
			cfg,
			[]string{
				"processing_run_id=" + crossRun.ProcessingRunID,
				"source_artifact_id=" + cross.ArtifactID,
				"source_collection_id=" + collectionID,
				"source_id=" + sourceID,
				"record_artifact_id=" + successArtifactID,
			},
			`
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
    :'processing_run_id'::uuid,
    record.source_record_id,
    :'source_artifact_id'::uuid,
    :'source_collection_id'::uuid,
    :'source_id'::uuid,
    'VALID',
    'PROCESSED',
    'NOT_APPLICABLE'
FROM pathfinder.source_record AS record
WHERE record.source_artifact_id = :'record_artifact_id'::uuid
  AND record.source_locator = '/vulnerabilities/0';
`,
		)
		if insertErr == nil {
			t.Fatal("cross-artifact SourceRecordProcessing insert unexpectedly succeeded")
		}

		count := resilienceScalar(
			t,
			ctx,
			cfg,
			[]string{"processing_run_id=" + crossRun.ProcessingRunID},
			`
SELECT count(*)::text
FROM pathfinder.source_record_processing
WHERE processing_run_id = :'processing_run_id'::uuid;
`,
		)
		if count != "0" {
			t.Fatalf(
				"cross-artifact run SourceRecordProcessing count=%q, want 0",
				count,
			)
		}

		if err := persistence.AppendTerminal(
			ctx,
			processinghistory.InterruptedEvent(
				crossRun.ProcessingRunID,
				ErrorClassStaleProcessingRun,
			),
		); err != nil {
			t.Fatalf("close cross-artifact test run: %v", err)
		}

		t.Log("cross-artifact SourceRecordProcessing rejection: PASS")
	})

	t.Run("final_disposable_database_summary", func(t *testing.T) {
		summary := resilienceScalar(
			t,
			ctx,
			cfg,
			nil,
			`
SELECT
    (SELECT count(*) FROM pathfinder.processing_run)::text || '|' ||
    (SELECT count(*) FROM pathfinder.processing_event)::text || '|' ||
    (SELECT count(*) FROM pathfinder.source_record_processing)::text || '|' ||
    (SELECT count(*) FROM pathfinder.source_record)::text || '|' ||
    (SELECT count(*) FROM pathfinder.vulnerability)::text || '|' ||
    (SELECT count(*) FROM pathfinder.assertion)::text || '|' ||
    (SELECT count(*) FROM pathfinder.collector_checkpoint)::text;
`,
		)

		if summary != "5|5|4|2|1|2|2" {
			t.Fatalf(
				"final disposable summary=%q, want 5|5|4|2|1|2|2",
				summary,
			)
		}

		t.Logf("disposable_summary=%s", summary)
		t.Log("Phase 1.4 live resilience: PASS")
	})
}

func createPhase14ResilienceRetrieval(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	sourceID string,
	collectionID string,
	now time.Time,
) string {
	t.Helper()

	retrievalID := mustLifecycleUUID(t, now)

	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{
			"retrieval_event_id=" + retrievalID,
			"source_collection_id=" + collectionID,
			"source_id=" + sourceID,
		},
		`
INSERT INTO pathfinder.retrieval_event (
    retrieval_event_id,
    source_id,
    source_collection_id
)
VALUES (
    :'retrieval_event_id'::uuid,
    :'source_id'::uuid,
    :'source_collection_id'::uuid
)
RETURNING
    retrieval_event_id::text || '|' ||
    completion_state || '|' ||
    operation_result;
`,
	)
	if err != nil {
		t.Fatalf("create resilience RetrievalEvent: %v", err)
	}

	expected := retrievalID + "|NOT_COMPLETED|IN_PROGRESS"
	if strings.TrimSpace(output) != expected {
		t.Fatalf(
			"retrieval creation proof=%q, want %q",
			strings.TrimSpace(output),
			expected,
		)
	}

	return retrievalID
}

func preservePhase14ResilienceArtifact(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	sourceID string,
	collectionID string,
	retrievalID string,
	body []byte,
	recordsReported int,
	sourceLocation string,
) phase14PreservedArtifact {
	t.Helper()

	artifactID := mustLifecycleUUID(t, time.Now())

	commitRequest := sourceartifact.CommitRequest{
		ContentEncoding:    "identity",
		HandlingProfile:    "PHASE_1_4_RESILIENCE_TEST",
		MediaType:          "application/json",
		RetrievalEventID:   retrievalID,
		SourceArtifactID:   artifactID,
		SourceCollectionID: collectionID,
		SourceID:           sourceID,
		SourceLocation:     sourceLocation,
	}

	commitResult, err := sourceartifact.Commit(
		ctx,
		cfg.ArtifactSocket,
		commitRequest,
		bytes.NewReader(body),
		func(
			ctx context.Context,
			request sourceartifact.CommitRequest,
			receipt artifactpreserve.Receipt,
		) (sourceartifact.DatabaseCommitResult, error) {
			return commitSourceArtifactDatabase(
				ctx,
				cfg,
				request,
				receipt,
			)
		},
	)
	if err != nil {
		t.Fatalf("preserve resilience SourceArtifact: %v", err)
	}

	if err := completeLifecycleRetrieval(
		ctx,
		cfg,
		retrievalID,
		recordsReported,
	); err != nil {
		t.Fatalf("complete resilience RetrievalEvent: %v", err)
	}

	record := sourceartifact.Record{
		AvailabilityState: sourceartifact.AvailabilityAvailable,
		ByteLength:        commitResult.Receipt.ByteLength,
		SHA256:            commitResult.Receipt.SHA256,
		SourceArtifactID:  artifactID,
		StorageReference:  commitResult.Receipt.StorageReference,
	}

	preservedBytes, err := sourceartifact.ReadAvailable(
		cfg.ArtifactsDir,
		record,
	)
	if err != nil {
		t.Fatalf("read verified resilience SourceArtifact: %v", err)
	}
	if !bytes.Equal(preservedBytes, body) {
		t.Fatal("verified preserved resilience bytes differ from submitted bytes")
	}

	return phase14PreservedArtifact{
		ArtifactID:  artifactID,
		Record:      record,
		RetrievalID: retrievalID,
	}
}

func resilienceArtifactRecord(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	artifactID string,
) sourceartifact.Record {
	t.Helper()

	output := resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{"source_artifact_id=" + artifactID},
		`
SELECT
    availability_state || '|' ||
    byte_length::text || '|' ||
    sha256 || '|' ||
    source_artifact_id::text || '|' ||
    storage_reference
FROM pathfinder.source_artifact
WHERE source_artifact_id = :'source_artifact_id'::uuid;
`,
	)

	parts := strings.Split(output, "|")
	if len(parts) != 5 {
		t.Fatalf("SourceArtifact record fields=%d, want 5; payload=%q", len(parts), output)
	}

	var byteLength int64
	if _, err := fmt.Sscanf(parts[1], "%d", &byteLength); err != nil {
		t.Fatalf("decode SourceArtifact byte_length: %v", err)
	}

	return sourceartifact.Record{
		AvailabilityState: parts[0],
		ByteLength:        byteLength,
		SHA256:            parts[2],
		SourceArtifactID:  parts[3],
		StorageReference:  parts[4],
	}
}

func resilienceArtifactRetrievalID(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	artifactID string,
) string {
	t.Helper()

	return resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{"source_artifact_id=" + artifactID},
		`
SELECT retrieval_event_id::text
FROM pathfinder.source_artifact
WHERE source_artifact_id = :'source_artifact_id'::uuid;
`,
	)
}

func resilienceCompletedArtifactID(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	processName string,
	processVersion string,
) string {
	t.Helper()

	return resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{
			"process_name=" + processName,
			"process_version=" + processVersion,
		},
		`
SELECT source_artifact_id::text
FROM pathfinder.collector_checkpoint
WHERE process_name = :'process_name'
  AND process_version = :'process_version';
`,
	)
}

func resilienceContractCounts(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	artifactID string,
	processName string,
	processVersion string,
) string {
	t.Helper()

	return resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{
			"source_artifact_id=" + artifactID,
			"process_name=" + processName,
			"process_version=" + processVersion,
		},
		`
SELECT
    (SELECT count(*)
       FROM pathfinder.processing_run
      WHERE source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = :'process_name'
        AND process_version = :'process_version')::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.processing_event AS event
       JOIN pathfinder.processing_run AS run
         ON run.processing_run_id = event.processing_run_id
      WHERE run.source_artifact_id = :'source_artifact_id'::uuid
        AND run.process_name = :'process_name'
        AND run.process_version = :'process_version')::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.source_record
      WHERE source_artifact_id = :'source_artifact_id'::uuid)::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.source_record_processing AS processing
       JOIN pathfinder.processing_run AS run
         ON run.processing_run_id = processing.processing_run_id
      WHERE run.source_artifact_id = :'source_artifact_id'::uuid
        AND run.process_name = :'process_name'
        AND run.process_version = :'process_version')::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.collector_checkpoint
      WHERE source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = :'process_name'
        AND process_version = :'process_version')::text;
`,
	)
}

func resilienceMalformedProof(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	artifactID string,
	runID string,
) string {
	t.Helper()

	return resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{
			"source_artifact_id=" + artifactID,
			"processing_run_id=" + runID,
		},
		`
SELECT
    (SELECT count(*)
       FROM pathfinder.processing_run
      WHERE processing_run_id = :'processing_run_id'::uuid)::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.processing_event
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND event_type = 'FAILED'
        AND error_class = 'CISA_KEV_ARTIFACT_PARSE_FAILED')::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.source_record_processing
      WHERE processing_run_id = :'processing_run_id'::uuid)::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.source_record
      WHERE source_artifact_id = :'source_artifact_id'::uuid)::text || '|' ||
    (SELECT count(*)
       FROM pathfinder.collector_checkpoint
      WHERE source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = 'cisakev'
        AND process_version = '1')::text;
`,
	)
}

func resilienceProcessingRunID(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	artifactID string,
	processName string,
	processVersion string,
) string {
	t.Helper()

	return resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{
			"source_artifact_id=" + artifactID,
			"process_name=" + processName,
			"process_version=" + processVersion,
		},
		`
SELECT processing_run_id::text
FROM pathfinder.processing_run
WHERE source_artifact_id = :'source_artifact_id'::uuid
  AND process_name = :'process_name'
  AND process_version = :'process_version';
`,
	)
}

func resilienceTerminalProof(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	runID string,
) string {
	t.Helper()

	return resilienceScalar(
		t,
		ctx,
		cfg,
		[]string{"processing_run_id=" + runID},
		`
SELECT event_type || '|' || error_class
FROM pathfinder.processing_event
WHERE processing_run_id = :'processing_run_id'::uuid;
`,
	)
}

func resilienceScalar(
	t *testing.T,
	ctx context.Context,
	cfg Config,
	variables []string,
	sql string,
) string {
	t.Helper()

	output, err := runRuntimePSQL(
		ctx,
		cfg,
		variables,
		sql,
	)
	if err != nil {
		t.Fatalf("resilience SQL query failed: %v", err)
	}

	value := strings.TrimSpace(output)
	if value == "" {
		t.Fatal("resilience SQL query returned no rows")
	}
	if strings.Contains(value, "\n") {
		t.Fatalf("resilience SQL query returned multiple rows: %q", value)
	}

	return value
}
