package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/collectors/cisakev"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

func TestPhase14LiveCISAKEVAcquisition(t *testing.T) {
	if os.Getenv("PATHFINDER_PHASE14_CISA_ACQUISITION") != "YES" {
		t.Skip("live CISA KEV acquisition requires PATHFINDER_PHASE14_CISA_ACQUISITION=YES")
	}

	configPath := strings.TrimSpace(
		os.Getenv("PATHFINDER_PHASE14_CISA_ACQUISITION_CONFIG"),
	)
	if configPath == "" {
		t.Fatal("PATHFINDER_PHASE14_CISA_ACQUISITION_CONFIG must be set")
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load CISA acquisition config: %v", err)
	}
	requireDisposableLifecycleConfig(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	now := time.Now()
	sourceID := mustLifecycleUUID(t, now)
	collectionID := mustLifecycleUUID(t, now.Add(time.Millisecond))
	retrievalID := mustLifecycleUUID(t, now.Add(2*time.Millisecond))
	artifactID := mustLifecycleUUID(t, now.Add(3*time.Millisecond))

	if err := createCISAKEVAcquisitionContext(
		ctx,
		cfg,
		sourceID,
		collectionID,
		retrievalID,
	); err != nil {
		t.Fatalf("create CISA KEV acquisition context: %v", err)
	}

	response, err := cisakev.FetchCatalog(
		ctx,
		cisakev.NewHTTPClient(),
	)
	if err != nil {
		if completionErr := completeCISAKEVRetrieval(
			ctx,
			cfg,
			retrievalID,
			"FAILED",
			0,
			"CISA_KEV_TRANSPORT_FAILED",
		); completionErr != nil {
			t.Fatalf(
				"CISA transport failed (%v); retrieval completion also failed (%v)",
				err,
				completionErr,
			)
		}

		t.Fatalf("retrieve canonical CISA KEV catalog: %v", err)
	}
	defer response.Body.Close()

	if response.FinalURL == "" {
		t.Fatal("CISA response FinalURL is empty")
	}

	hasher := sha256.New()
	counter := &countingReader{
		reader: io.TeeReader(response.Body, hasher),
	}

	mediaType := strings.TrimSpace(response.ContentType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	commitResult, err := sourceartifact.Commit(
		ctx,
		cfg.ArtifactSocket,
		sourceartifact.CommitRequest{
			ContentEncoding:    response.PreservationContentEncoding(),
			HandlingProfile:    "CISA_KEV_CANONICAL_HTTP_V1",
			MediaType:          mediaType,
			RetrievalEventID:   retrievalID,
			SourceArtifactID:   artifactID,
			SourceCollectionID: collectionID,
			SourceID:           sourceID,
			SourceLocation:     response.FinalURL,
		},
		counter,
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
		if completionErr := completeCISAKEVRetrieval(
			ctx,
			cfg,
			retrievalID,
			"FAILED",
			0,
			"CISA_KEV_PRESERVATION_FAILED",
		); completionErr != nil {
			t.Fatalf(
				"CISA preservation failed (%v); retrieval completion also failed (%v)",
				err,
				completionErr,
			)
		}

		t.Fatalf("preserve canonical CISA KEV response: %v", err)
	}

	networkSHA := hex.EncodeToString(hasher.Sum(nil))
	if networkSHA != commitResult.Receipt.SHA256 {
		t.Fatalf(
			"network SHA-256=%s, preserved SHA-256=%s",
			networkSHA,
			commitResult.Receipt.SHA256,
		)
	}
	if counter.count != commitResult.Receipt.ByteLength {
		t.Fatalf(
			"network bytes=%d, preserved bytes=%d",
			counter.count,
			commitResult.Receipt.ByteLength,
		)
	}
	if response.ContentLength >= 0 &&
		response.ContentLength != counter.count {
		t.Fatalf(
			"HTTP Content-Length=%d, received bytes=%d",
			response.ContentLength,
			counter.count,
		)
	}

	switch {
	case response.StatusCode != http.StatusOK:
		errorClass := fmt.Sprintf(
			"CISA_KEV_HTTP_STATUS_%d",
			response.StatusCode,
		)
		if err := completeCISAKEVRetrieval(
			ctx,
			cfg,
			retrievalID,
			"FAILED",
			1,
			errorClass,
		); err != nil {
			t.Fatalf("complete failed CISA RetrievalEvent: %v", err)
		}

		t.Fatalf(
			"canonical CISA KEV returned %s; response body was preserved as SourceArtifact %s",
			response.Status,
			artifactID,
		)

	case !response.UsesIdentityEncoding():
		if err := completeCISAKEVRetrieval(
			ctx,
			cfg,
			retrievalID,
			"FAILED",
			1,
			"CISA_KEV_UNSUPPORTED_CONTENT_ENCODING",
		); err != nil {
			t.Fatalf("complete encoding-failed CISA RetrievalEvent: %v", err)
		}

		t.Fatalf(
			"canonical CISA KEV returned unsupported Content-Encoding %q; response body was preserved",
			response.ContentEncoding,
		)
	}

	if err := completeCISAKEVRetrieval(
		ctx,
		cfg,
		retrievalID,
		"SUCCESS",
		1,
		"NOT_APPLICABLE",
	); err != nil {
		t.Fatalf("complete successful CISA RetrievalEvent: %v", err)
	}

	artifactRecord := sourceartifact.Record{
		AvailabilityState: sourceartifact.AvailabilityAvailable,
		ByteLength:        commitResult.Receipt.ByteLength,
		SHA256:            commitResult.Receipt.SHA256,
		SourceArtifactID:  artifactID,
		StorageReference:  commitResult.Receipt.StorageReference,
	}

	catalog, err := cisakev.ParseSourceArtifact(
		cfg.ArtifactsDir,
		artifactRecord,
	)
	if err != nil {
		t.Fatalf("parse verified canonical CISA KEV SourceArtifact: %v", err)
	}

	if catalog.DeclaredCount != len(catalog.Records) {
		t.Fatalf(
			"CISA declared count=%d, parsed records=%d",
			catalog.DeclaredCount,
			len(catalog.Records),
		)
	}
	if len(catalog.Records) == 0 {
		t.Fatal("canonical CISA KEV catalog unexpectedly contains zero records")
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
			ExtractionVersion: cisakev.ExtractionVersion,
			ProcessName:       cisakev.ProcessName,
			ProcessVersion:    cisakev.ProcessVersion,
		},
		persistence,
	)
	if err != nil {
		t.Fatalf("process canonical CISA KEV SourceArtifact: %v", err)
	}

	if processResult.AlreadyCompleted {
		t.Fatal("first canonical CISA KEV processing unexpectedly reported AlreadyCompleted")
	}
	if processResult.CheckpointValue != catalog.CheckpointValue() {
		t.Fatalf(
			"checkpoint=%q, parsed catalog checkpoint=%q",
			processResult.CheckpointValue,
			catalog.CheckpointValue(),
		)
	}
	if processResult.RecordCount != len(catalog.Records) {
		t.Fatalf(
			"process record_count=%d, parsed records=%d",
			processResult.RecordCount,
			len(catalog.Records),
		)
	}

	if err := verifyCISAKEVAcquisitionDatabase(
		ctx,
		cfg,
		sourceID,
		collectionID,
		retrievalID,
		artifactID,
		processResult.ProcessingRunID,
		processResult.RecordCount,
	); err != nil {
		t.Fatalf("verify canonical CISA KEV database lineage: %v", err)
	}

	t.Logf("http_status=%s", response.Status)
	t.Logf("final_url=%s", response.FinalURL)
	t.Logf("content_type=%s", response.ContentType)
	t.Logf("content_encoding=%s", response.PreservationContentEncoding())
	t.Logf("source_artifact_id=%s", artifactID)
	t.Logf("source_artifact_sha256=%s", artifactRecord.SHA256)
	t.Logf("source_artifact_bytes=%d", artifactRecord.ByteLength)
	t.Logf("catalog_version=%s", catalog.CheckpointValue())
	t.Logf("records=%d", processResult.RecordCount)
	t.Logf("processing_run_id=%s", processResult.ProcessingRunID)
	t.Log("Phase 1.4 canonical CISA KEV acquisition: PASS")
}

type countingReader struct {
	count  int64
	reader io.Reader
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.count += int64(count)

	return count, err
}

func completeCISAKEVRetrieval(
	ctx context.Context,
	cfg Config,
	retrievalEventID string,
	operationResult string,
	artifactsReceived int,
	errorClass string,
) error {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{
			"retrieval_event_id=" + retrievalEventID,
			"operation_result=" + operationResult,
			fmt.Sprintf("artifacts_received=%d", artifactsReceived),
			"error_class=" + errorClass,
		},
		`
UPDATE pathfinder.retrieval_event
SET completion_state = 'COMPLETED',
    completed_at = clock_timestamp(),
    operation_result = :'operation_result',
    artifacts_received = :'artifacts_received'::bigint,
    records_reported_state = 'NOT_KNOWN',
    records_reported = 0,
    error_class = :'error_class'
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

	expected := strings.Join(
		[]string{
			"COMPLETED",
			operationResult,
			strconv.Itoa(artifactsReceived),
			"NOT_KNOWN",
			"0",
			errorClass,
		},
		"|",
	)

	if strings.TrimSpace(output) != expected {
		return fmt.Errorf(
			"retrieval completion proof=%q, want %q",
			strings.TrimSpace(output),
			expected,
		)
	}

	return nil
}

func createCISAKEVAcquisitionContext(
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
    'Cybersecurity and Infrastructure Security Agency',
    'GOVERNMENT',
    'CISA',
    'United States Cybersecurity and Infrastructure Security Agency'
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
    'Known Exploited Vulnerabilities Catalog',
    'VULNERABILITY_CATALOG',
    'CISA_KEV',
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
    source.name || '|' ||
    source.provider_identity || '|' ||
    collection.name || '|' ||
    collection.collection_type || '|' ||
    collection.transport_type || '|' ||
    collection.expected_format || '|' ||
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
			"Cybersecurity and Infrastructure Security Agency",
			"CISA",
			"Known Exploited Vulnerabilities Catalog",
			"VULNERABILITY_CATALOG",
			"HTTPS",
			"JSON",
			"NOT_COMPLETED",
			"IN_PROGRESS",
		},
		"|",
	)
	if strings.TrimSpace(output) != expected {
		return fmt.Errorf(
			"CISA source context proof=%q, want %q",
			strings.TrimSpace(output),
			expected,
		)
	}

	return nil
}

func verifyCISAKEVAcquisitionDatabase(
	ctx context.Context,
	cfg Config,
	sourceID string,
	collectionID string,
	retrievalID string,
	artifactID string,
	runID string,
	recordCount int,
) error {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{
			"source_id=" + sourceID,
			"source_collection_id=" + collectionID,
			"retrieval_event_id=" + retrievalID,
			"source_artifact_id=" + artifactID,
			"processing_run_id=" + runID,
		},
		`
SELECT
    (SELECT count(*)
       FROM pathfinder.retrieval_event
      WHERE retrieval_event_id = :'retrieval_event_id'::uuid
        AND source_id = :'source_id'::uuid
        AND source_collection_id = :'source_collection_id'::uuid
        AND completion_state = 'COMPLETED'
        AND operation_result = 'SUCCESS'
        AND artifacts_received = 1
        AND records_reported_state = 'NOT_KNOWN'
        AND records_reported = 0
        AND error_class = 'NOT_APPLICABLE')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.source_artifact
      WHERE source_artifact_id = :'source_artifact_id'::uuid
        AND retrieval_event_id = :'retrieval_event_id'::uuid
        AND source_id = :'source_id'::uuid
        AND source_collection_id = :'source_collection_id'::uuid
        AND preservation_state = 'PRESERVED'
        AND integrity_state = 'VERIFIED'
        AND availability_state = 'AVAILABLE')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.processing_run
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = 'cisa-kev'
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
       FROM pathfinder.source_record_processing
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND processing_state = 'PROCESSED')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.vulnerability)::text || '|' ||

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
        AND assertion.extraction_version = 'cisa-kev-json-v1')::text || '|' ||

    (SELECT count(*)
       FROM pathfinder.collector_checkpoint
      WHERE processing_run_id = :'processing_run_id'::uuid
        AND source_artifact_id = :'source_artifact_id'::uuid
        AND process_name = 'cisa-kev'
        AND process_version = '1'
        AND checkpoint_kind = 'FULL_SNAPSHOT'
        AND processing_event_type = 'COMPLETED')::text;
`,
	)
	if err != nil {
		return err
	}

	fields := strings.Split(strings.TrimSpace(output), "|")
	if len(fields) != 10 {
		return fmt.Errorf(
			"CISA lineage proof field count=%d, want 10; payload=%q",
			len(fields),
			output,
		)
	}

	values := make([]int, len(fields))
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf(
				"decode CISA lineage field %d=%q: %w",
				index,
				field,
				err,
			)
		}
		values[index] = value
	}

	if values[0] != 1 ||
		values[1] != 1 ||
		values[2] != 1 ||
		values[3] != 1 ||
		values[4] != recordCount ||
		values[5] != recordCount ||
		values[9] != 1 {
		return fmt.Errorf(
			"CISA lineage proof=%q, expected retrieval/artifact/run/event/checkpoint=1 and record/SRP=%d",
			strings.TrimSpace(output),
			recordCount,
		)
	}

	processedCount := values[6]
	vulnerabilityCount := values[7]
	assertionCount := values[8]

	if vulnerabilityCount > processedCount {
		return fmt.Errorf(
			"native Vulnerability count=%d exceeds processed record count=%d",
			vulnerabilityCount,
			processedCount,
		)
	}
	if assertionCount != processedCount {
		return fmt.Errorf(
			"Assertion count=%d, processed record count=%d",
			assertionCount,
			processedCount,
		)
	}

	return nil
}
