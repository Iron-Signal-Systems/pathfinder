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
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

const (
	phase14LifecycleCheckpointValue   = "phase-1.4-lifecycle-1"
	phase14LifecycleExtractionVersion = "cisakev-parser-v1"
	phase14LifecycleProcessName       = "cisakev"
	phase14LifecycleProcessVersion    = "1"
)

func TestPhase14LiveLifecycle(t *testing.T) {
	if os.Getenv("PATHFINDER_PHASE14_LIFECYCLE") != "YES" {
		t.Skip("live Phase 1.4 lifecycle test requires PATHFINDER_PHASE14_LIFECYCLE=YES")
	}

	configPath := strings.TrimSpace(
		os.Getenv("PATHFINDER_PHASE14_LIFECYCLE_CONFIG"),
	)
	if configPath == "" {
		t.Fatal("PATHFINDER_PHASE14_LIFECYCLE_CONFIG must be set")
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load lifecycle config: %v", err)
	}
	requireDisposableLifecycleConfig(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sourceID := mustLifecycleUUID(t, time.Now())
	collectionID := mustLifecycleUUID(t, time.Now().Add(time.Millisecond))
	retrievalID := mustLifecycleUUID(t, time.Now().Add(2*time.Millisecond))
	artifactID := mustLifecycleUUID(t, time.Now().Add(3*time.Millisecond))

	if err := createLifecycleSourceContext(
		ctx,
		cfg,
		sourceID,
		collectionID,
		retrievalID,
	); err != nil {
		t.Fatalf("create lifecycle source context: %v", err)
	}

	body := lifecycleKEVArtifact()

	commitRequest := sourceartifact.CommitRequest{
		ContentEncoding:    "identity",
		HandlingProfile:    "PHASE_1_4_LIFECYCLE_TEST",
		MediaType:          "application/json",
		RetrievalEventID:   retrievalID,
		SourceArtifactID:   artifactID,
		SourceCollectionID: collectionID,
		SourceID:           sourceID,
		SourceLocation:     "pathfinder-test://cisa-kev/synthetic-lifecycle",
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
		t.Fatalf("preserve lifecycle SourceArtifact: %v", err)
	}

	if err := completeLifecycleRetrieval(
		ctx,
		cfg,
		retrievalID,
		2,
	); err != nil {
		t.Fatalf("complete lifecycle RetrievalEvent: %v", err)
	}

	artifactRecord := sourceartifact.Record{
		AvailabilityState: sourceartifact.AvailabilityAvailable,
		ByteLength:        commitResult.Receipt.ByteLength,
		SHA256:            commitResult.Receipt.SHA256,
		SourceArtifactID:  artifactID,
		StorageReference:  commitResult.Receipt.StorageReference,
	}

	preservedBytes, err := sourceartifact.ReadAvailable(
		cfg.ArtifactsDir,
		artifactRecord,
	)
	if err != nil {
		t.Fatalf("read verified lifecycle SourceArtifact: %v", err)
	}
	if !bytes.Equal(preservedBytes, body) {
		t.Fatal("verified preserved lifecycle bytes differ from submitted bytes")
	}

	persistence := newRuntimeProcessingPersistence(cfg)

	processResult, err := cisakev.ProcessPreservedArtifact(
		ctx,
		cisakev.ProcessRequest{
			Artifact: cisakev.ArtifactContext{
				Record:             artifactRecord,
				RetrievalEventID:   retrievalID,
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
	if err != nil {
		t.Fatalf("process preserved lifecycle artifact: %v", err)
	}

	if processResult.CheckpointValue != phase14LifecycleCheckpointValue {
		t.Fatalf(
			"checkpoint value = %q, want %q",
			processResult.CheckpointValue,
			phase14LifecycleCheckpointValue,
		)
	}
	if processResult.RecordCount != 2 {
		t.Fatalf("record count = %d, want 2", processResult.RecordCount)
	}

	if err := verifyLifecycleDatabase(
		ctx,
		cfg,
		sourceID,
		collectionID,
		retrievalID,
		artifactID,
		processResult.ProcessingRunID,
	); err != nil {
		t.Fatalf("verify lifecycle database: %v", err)
	}

	t.Logf("source_id=%s", sourceID)
	t.Logf("source_collection_id=%s", collectionID)
	t.Logf("retrieval_event_id=%s", retrievalID)
	t.Logf("source_artifact_id=%s", artifactID)
	t.Logf("processing_run_id=%s", processResult.ProcessingRunID)
	t.Logf("checkpoint_value=%s", processResult.CheckpointValue)
	t.Log("Phase 1.4 lifecycle: PASS")
}

func completeLifecycleRetrieval(
	ctx context.Context,
	cfg Config,
	retrievalEventID string,
	recordsReported int,
) error {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{
			"retrieval_event_id=" + retrievalEventID,
			fmt.Sprintf("records_reported=%d", recordsReported),
		},
		`
UPDATE pathfinder.retrieval_event
SET completion_state = 'COMPLETED',
    completed_at = clock_timestamp(),
    operation_result = 'SUCCESS',
    artifacts_received = 1,
    records_reported_state = 'KNOWN',
    records_reported = :'records_reported'::bigint,
    error_class = 'NOT_APPLICABLE'
WHERE retrieval_event_id = :'retrieval_event_id'::uuid
  AND completion_state = 'NOT_COMPLETED'
RETURNING
    completion_state || '|' ||
    operation_result || '|' ||
    artifacts_received::text || '|' ||
    records_reported_state || '|' ||
    records_reported::text || '|' ||
    error_class;
`,
	)
	if err != nil {
		return err
	}

	expected := fmt.Sprintf(
		"COMPLETED|SUCCESS|1|KNOWN|%d|NOT_APPLICABLE",
		recordsReported,
	)
	if strings.TrimSpace(output) != expected {
		return fmt.Errorf(
			"retrieval completion proof = %q, want %q",
			strings.TrimSpace(output),
			expected,
		)
	}

	return nil
}

func createLifecycleSourceContext(
	ctx context.Context,
	cfg Config,
	sourceID string,
	collectionID string,
	retrievalID string,
) error {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{
			"source_id=" + sourceID,
			"source_collection_id=" + collectionID,
			"retrieval_event_id=" + retrievalID,
		},
		`
BEGIN;

INSERT INTO pathfinder.source (
    source_id,
    name,
    source_type,
    provider_identity,
    description
)
VALUES (
    :'source_id'::uuid,
    'CISA Phase 1.4 lifecycle test',
    'GOVERNMENT',
    'CISA',
    'Disposable synthetic lifecycle validation source'
);

INSERT INTO pathfinder.source_collection (
    source_collection_id,
    source_id,
    name,
    collection_type,
    external_collection_identifier,
    transport_type,
    expected_format
)
VALUES (
    :'source_collection_id'::uuid,
    :'source_id'::uuid,
    'Known Exploited Vulnerabilities Catalog lifecycle test',
    'VULNERABILITY_CATALOG',
    'CISA_KEV_SYNTHETIC_LIFECYCLE',
    'HTTPS',
    'JSON'
);

INSERT INTO pathfinder.retrieval_event (
    retrieval_event_id,
    source_id,
    source_collection_id
)
VALUES (
    :'retrieval_event_id'::uuid,
    :'source_id'::uuid,
    :'source_collection_id'::uuid
);

SELECT
    source.source_id::text || '|' ||
    collection.source_collection_id::text || '|' ||
    retrieval.retrieval_event_id::text || '|' ||
    retrieval.completion_state || '|' ||
    retrieval.operation_result
FROM pathfinder.source AS source
JOIN pathfinder.source_collection AS collection
  ON collection.source_id = source.source_id
JOIN pathfinder.retrieval_event AS retrieval
  ON retrieval.source_id = source.source_id
 AND retrieval.source_collection_id = collection.source_collection_id
WHERE source.source_id = :'source_id'::uuid
  AND collection.source_collection_id = :'source_collection_id'::uuid
  AND retrieval.retrieval_event_id = :'retrieval_event_id'::uuid;

COMMIT;
`,
	)
	if err != nil {
		return err
	}

	expected := strings.Join(
		[]string{
			sourceID,
			collectionID,
			retrievalID,
			"NOT_COMPLETED",
			"IN_PROGRESS",
		},
		"|",
	)

	if strings.TrimSpace(output) != expected {
		return fmt.Errorf(
			"source context proof = %q, want %q",
			strings.TrimSpace(output),
			expected,
		)
	}

	return nil
}

func lifecycleKEVArtifact() []byte {
	return []byte(`{
  "title": "CISA Known Exploited Vulnerabilities Catalog - Pathfinder lifecycle test",
  "catalogVersion": "phase-1.4-lifecycle-1",
  "dateReleased": "2026-09-13T00:00:00.000Z",
  "count": 2,
  "vulnerabilities": [
    {
      "cveID": "CVE-2026-1234",
      "vendorProject": "Pathfinder Lifecycle Vendor",
      "product": "Lifecycle Product",
      "vulnerabilityName": "Lifecycle Valid CVE",
      "dateAdded": "2026-09-13",
      "shortDescription": "Synthetic lifecycle validation record.",
      "requiredAction": "Use only for the disposable Phase 1.4 lifecycle test.",
      "dueDate": "2026-10-01",
      "knownRansomwareCampaignUse": "Unknown",
      "notes": "Synthetic lifecycle record.",
      "cwes": ["CWE-79"]
    },
    {
      "cveID": "NOT-A-CVE",
      "vendorProject": "Pathfinder Lifecycle Vendor",
      "product": "Lifecycle Product",
      "vulnerabilityName": "Lifecycle Invalid CVE",
      "dateAdded": "2026-09-13",
      "shortDescription": "Synthetic malformed CVE identity record.",
      "requiredAction": "Use only for the disposable Phase 1.4 lifecycle test.",
      "dueDate": "2026-10-01",
      "knownRansomwareCampaignUse": "Unknown",
      "notes": "Synthetic lifecycle record.",
      "cwes": ["CWE-20"]
    }
  ]
}`)
}

func mustLifecycleUUID(t *testing.T, now time.Time) string {
	t.Helper()

	value, err := sourceartifact.NewUUIDv7(now)
	if err != nil {
		t.Fatalf("generate lifecycle UUIDv7: %v", err)
	}

	return value
}

func requireDisposableLifecycleConfig(t *testing.T, cfg Config) {
	t.Helper()

	if cfg.DatabaseName == "pathfinder" {
		t.Fatal("refusing lifecycle test against production database_name=pathfinder")
	}
	if !strings.Contains(cfg.DatabaseName, "phase14_lifecycle") {
		t.Fatalf(
			"refusing lifecycle test against unexpected database_name=%q",
			cfg.DatabaseName,
		)
	}
	if cfg.ArtifactSocket == "/var/run/pathfinder-artifact/preserve.sock" {
		t.Fatal("refusing lifecycle test against production artifact socket")
	}
	if cfg.ArtifactsDir == "/var/db/pathfinder/artifacts" ||
		strings.HasPrefix(cfg.ArtifactsDir, "/var/db/pathfinder/artifacts/") {
		t.Fatal("refusing lifecycle test against production artifact tree")
	}
	if !strings.Contains(cfg.ArtifactsDir, "phase14-lifecycle") {
		t.Fatalf(
			"refusing lifecycle test against unexpected artifacts_dir=%q",
			cfg.ArtifactsDir,
		)
	}
}

func verifyLifecycleDatabase(
	ctx context.Context,
	cfg Config,
	sourceID string,
	collectionID string,
	retrievalID string,
	artifactID string,
	processingRunID string,
) error {
	variables := []string{
		"source_id=" + sourceID,
		"source_collection_id=" + collectionID,
		"retrieval_event_id=" + retrievalID,
		"source_artifact_id=" + artifactID,
		"processing_run_id=" + processingRunID,
	}

	summary, err := runRuntimePSQL(
		ctx,
		cfg,
		variables,
		`
SELECT
    (SELECT count(*)
       FROM pathfinder.processing_run
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND source_id = :'source_id'::uuid
        AND source_collection_id = :'source_collection_id'::uuid
        AND retrieval_event_id = :'retrieval_event_id'::uuid
        AND source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = 'cisakev'
        AND process_version = '1')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.processing_event
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND event_type = 'COMPLETED'
        AND error_class = 'NOT_APPLICABLE')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.source_record
      WHERE source_artifact_id = :'source_artifact_id'::uuid)::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.source_record_processing
      WHERE processing_run_id = :'processing_run_id'::uuid)::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.vulnerability
      WHERE identifier_namespace = 'CVE'
        AND identifier_value = 'CVE-2026-1234')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.assertion AS assertion
       JOIN pathfinder.source_record_processing AS processing
         ON processing.source_record_processing_id =
            assertion.source_record_processing_id
      WHERE processing.processing_run_id = :'processing_run_id'::uuid
        AND assertion.subject_type = 'VULNERABILITY'
        AND assertion.assertion_type = 'KNOWN_EXPLOITED'
        AND assertion.asserted_value = 'TRUE'
        AND assertion.extraction_method = 'SOURCE_STRUCTURED'
        AND assertion.extraction_version = 'cisakev-parser-v1')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.collector_checkpoint
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND processing_event_type = 'COMPLETED'
        AND source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = 'cisakev'
        AND process_version = '1'
        AND checkpoint_kind = 'FULL_SNAPSHOT'
        AND source_checkpoint_value = 'phase-1.4-lifecycle-1')::text;
`,
	)
	if err != nil {
		return err
	}

	if strings.TrimSpace(summary) != "1|1|2|2|1|1|1" {
		return fmt.Errorf(
			"semantic lineage summary = %q, want %q",
			strings.TrimSpace(summary),
			"1|1|2|2|1|1|1",
		)
	}

	outcomes, err := runRuntimePSQL(
		ctx,
		cfg,
		variables,
		`
SELECT
    record.source_locator || '|' ||
    record.external_record_id || '|' ||
    processing.validation_state || '|' ||
    processing.processing_state || '|' ||
    processing.processing_error_class
FROM pathfinder.source_record AS record
JOIN pathfinder.source_record_processing AS processing
  ON processing.source_record_id = record.source_record_id
WHERE record.source_artifact_id = :'source_artifact_id'::uuid
  AND processing.processing_run_id = :'processing_run_id'::uuid
ORDER BY record.source_locator;
`,
	)
	if err != nil {
		return err
	}

	expectedOutcomes := strings.Join(
		[]string{
			"/vulnerabilities/0|CVE-2026-1234|VALID|PROCESSED|NOT_APPLICABLE",
			"/vulnerabilities/1|NOT-A-CVE|INVALID|FAILED|INVALID_CVE_IDENTIFIER",
		},
		"\n",
	)
	if strings.TrimSpace(outcomes) != expectedOutcomes {
		return fmt.Errorf(
			"record outcomes = %q, want %q",
			strings.TrimSpace(outcomes),
			expectedOutcomes,
		)
	}

	assertionLineage, err := runRuntimePSQL(
		ctx,
		cfg,
		variables,
		`
SELECT
    record.source_locator || '|' ||
    vulnerability.identifier_namespace || '|' ||
    vulnerability.identifier_value || '|' ||
    assertion.subject_type || '|' ||
    assertion.assertion_type || '|' ||
    assertion.asserted_value || '|' ||
    assertion.extraction_method || '|' ||
    assertion.extraction_version
FROM pathfinder.assertion AS assertion
JOIN pathfinder.source_record_processing AS processing
  ON processing.source_record_processing_id =
     assertion.source_record_processing_id
JOIN pathfinder.source_record AS record
  ON record.source_record_id = processing.source_record_id
JOIN pathfinder.vulnerability AS vulnerability
  ON vulnerability.vulnerability_id = assertion.vulnerability_id
WHERE processing.processing_run_id = :'processing_run_id'::uuid;
`,
	)
	if err != nil {
		return err
	}

	expectedAssertion := strings.Join(
		[]string{
			"/vulnerabilities/0",
			"CVE",
			"CVE-2026-1234",
			"VULNERABILITY",
			"KNOWN_EXPLOITED",
			"TRUE",
			"SOURCE_STRUCTURED",
			"cisakev-parser-v1",
		},
		"|",
	)
	if strings.TrimSpace(assertionLineage) != expectedAssertion {
		return fmt.Errorf(
			"assertion lineage = %q, want %q",
			strings.TrimSpace(assertionLineage),
			expectedAssertion,
		)
	}

	acquisitionProof, err := runRuntimePSQL(
		ctx,
		cfg,
		variables,
		`
SELECT count(*)::text
FROM pathfinder.source_artifact AS artifact
JOIN pathfinder.retrieval_event AS retrieval
  ON retrieval.retrieval_event_id = artifact.retrieval_event_id
 AND retrieval.source_collection_id = artifact.source_collection_id
 AND retrieval.source_id = artifact.source_id
WHERE artifact.source_artifact_id = :'source_artifact_id'::uuid
  AND artifact.source_id = :'source_id'::uuid
  AND artifact.source_collection_id = :'source_collection_id'::uuid
  AND artifact.retrieval_event_id = :'retrieval_event_id'::uuid
  AND artifact.preservation_state = 'PRESERVED'
  AND artifact.integrity_state = 'VERIFIED'
  AND artifact.availability_state = 'AVAILABLE'
  AND retrieval.completion_state = 'COMPLETED'
  AND retrieval.operation_result = 'SUCCESS'
  AND retrieval.artifacts_received = 1
  AND retrieval.records_reported_state = 'KNOWN'
  AND retrieval.records_reported = 2
  AND retrieval.error_class = 'NOT_APPLICABLE';
`,
	)
	if err != nil {
		return err
	}

	if strings.TrimSpace(acquisitionProof) != "1" {
		return fmt.Errorf(
			"acquisition lineage proof = %q, want 1",
			strings.TrimSpace(acquisitionProof),
		)
	}

	return nil
}
