package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/artifactpreserve"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

type sourceArtifactCommitOptions struct {
	ConfigPath         string
	ContentEncoding    string
	HandlingProfile    string
	MediaType          string
	RetrievalEventID   string
	SourceArtifactID   string
	SourceCollectionID string
	SourceID           string
	SourceLocation     string
}

type sourceArtifactDatabaseRow struct {
	AvailabilityState  string `json:"availability_state"`
	ByteLength         int64  `json:"byte_length"`
	ContentEncoding    string `json:"content_encoding"`
	HandlingProfile    string `json:"handling_profile"`
	IntegrityState     string `json:"integrity_state"`
	MediaType          string `json:"media_type"`
	PreservationState  string `json:"preservation_state"`
	RetrievalEventID   string `json:"retrieval_event_id"`
	SHA256             string `json:"sha256"`
	SourceArtifactID   string `json:"source_artifact_id"`
	SourceCollectionID string `json:"source_collection_id"`
	SourceID           string `json:"source_id"`
	SourceLocation     string `json:"source_location"`
	StorageReference   string `json:"storage_reference"`
}

func commitSourceArtifactDatabase(
	ctx context.Context,
	cfg Config,
	request sourceartifact.CommitRequest,
	receipt artifactpreserve.Receipt,
) (sourceartifact.DatabaseCommitResult, error) {
	variables := []string{
		"source_artifact_id=" + request.SourceArtifactID,
		"source_id=" + request.SourceID,
		"source_collection_id=" + request.SourceCollectionID,
		"retrieval_event_id=" + request.RetrievalEventID,
		"source_location=" + request.SourceLocation,
		"media_type=" + request.MediaType,
		"content_encoding=" + request.ContentEncoding,
		"byte_length=" + strconv.FormatInt(receipt.ByteLength, 10),
		"sha256=" + receipt.SHA256,
		"storage_reference=" + receipt.StorageReference,
		"handling_profile=" + request.HandlingProfile,
	}

	insertOutput, insertErr := runRuntimePSQL(
		ctx,
		cfg,
		variables,
		`
INSERT INTO pathfinder.source_artifact (
    source_artifact_id,
    source_id,
    source_collection_id,
    retrieval_event_id,
    source_location,
    media_type,
    content_encoding,
    byte_length,
    sha256,
    preservation_state,
    integrity_state,
    availability_state,
    storage_reference,
    handling_profile
)
VALUES (
    :'source_artifact_id'::uuid,
    :'source_id'::uuid,
    :'source_collection_id'::uuid,
    :'retrieval_event_id'::uuid,
    :'source_location',
    :'media_type',
    :'content_encoding',
    :'byte_length'::bigint,
    :'sha256',
    'PRESERVED',
    'VERIFIED',
    'AVAILABLE',
    :'storage_reference',
    :'handling_profile'
)
ON CONFLICT (source_artifact_id) DO NOTHING
RETURNING source_artifact_id::text;
`,
	)

	if insertErr == nil && strings.TrimSpace(insertOutput) == request.SourceArtifactID {
		return sourceartifact.DatabaseCommitResult{
			SourceArtifactID: request.SourceArtifactID,
		}, nil
	}

	row, found, readErr := readSourceArtifactDatabase(
		ctx,
		cfg,
		request.SourceArtifactID,
	)
	if readErr == nil && found && sourceArtifactRowMatches(row, request, receipt) {
		return sourceartifact.DatabaseCommitResult{
			AlreadyCommitted: true,
			SourceArtifactID: request.SourceArtifactID,
		}, nil
	}

	if insertErr != nil {
		if readErr != nil {
			return sourceartifact.DatabaseCommitResult{}, fmt.Errorf(
				"insert failed (%v); read-back failed (%v)",
				insertErr,
				readErr,
			)
		}

		if found {
			return sourceartifact.DatabaseCommitResult{}, fmt.Errorf(
				"insert failed (%v); existing source_artifact_id does not match preservation receipt and provenance context",
				insertErr,
			)
		}

		return sourceartifact.DatabaseCommitResult{}, fmt.Errorf(
			"insert failed and no matching SourceArtifact row was proven: %w",
			insertErr,
		)
	}

	if readErr != nil {
		return sourceartifact.DatabaseCommitResult{}, fmt.Errorf(
			"SourceArtifact insert outcome not established; read-back failed: %w",
			readErr,
		)
	}

	if found {
		return sourceartifact.DatabaseCommitResult{}, fmt.Errorf(
			"source_artifact_id already exists with different content or provenance context",
		)
	}

	return sourceartifact.DatabaseCommitResult{}, fmt.Errorf(
		"SourceArtifact insert returned no row and read-back found no record",
	)
}

func loadSourceArtifactRecords(
	ctx context.Context,
	cfg Config,
) ([]sourceartifact.Record, error) {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		nil,
		`
SELECT json_build_object(
    'source_artifact_id', source_artifact_id::text,
    'sha256', sha256,
    'byte_length', byte_length,
    'storage_reference', storage_reference,
    'availability_state', availability_state
)::text
FROM pathfinder.source_artifact
ORDER BY source_artifact_id;
`,
	)
	if err != nil {
		return nil, fmt.Errorf("query SourceArtifact records: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var records []sourceartifact.Record
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var row struct {
			AvailabilityState string `json:"availability_state"`
			ByteLength        int64  `json:"byte_length"`
			SHA256            string `json:"sha256"`
			SourceArtifactID  string `json:"source_artifact_id"`
			StorageReference  string `json:"storage_reference"`
		}

		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("decode SourceArtifact record: %w", err)
		}

		records = append(records, sourceartifact.Record{
			AvailabilityState: row.AvailabilityState,
			ByteLength:        row.ByteLength,
			SHA256:            row.SHA256,
			SourceArtifactID:  row.SourceArtifactID,
			StorageReference:  row.StorageReference,
		})
	}

	return records, nil
}

func readSourceArtifactDatabase(
	ctx context.Context,
	cfg Config,
	sourceArtifactID string,
) (sourceArtifactDatabaseRow, bool, error) {
	output, err := runRuntimePSQL(
		ctx,
		cfg,
		[]string{"source_artifact_id=" + sourceArtifactID},
		`
SELECT json_build_object(
    'source_artifact_id', source_artifact_id::text,
    'source_id', source_id::text,
    'source_collection_id', source_collection_id::text,
    'retrieval_event_id', retrieval_event_id::text,
    'source_location', source_location,
    'media_type', media_type,
    'content_encoding', content_encoding,
    'byte_length', byte_length,
    'sha256', sha256,
    'preservation_state', preservation_state,
    'integrity_state', integrity_state,
    'availability_state', availability_state,
    'storage_reference', storage_reference,
    'handling_profile', handling_profile
)::text
FROM pathfinder.source_artifact
WHERE source_artifact_id = :'source_artifact_id'::uuid;
`,
	)
	if err != nil {
		return sourceArtifactDatabaseRow{}, false, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return sourceArtifactDatabaseRow{}, false, nil
	}

	var row sourceArtifactDatabaseRow
	if err := json.Unmarshal([]byte(output), &row); err != nil {
		return sourceArtifactDatabaseRow{}, false, fmt.Errorf(
			"decode SourceArtifact read-back: %w",
			err,
		)
	}

	return row, true, nil
}

func runRuntimePSQL(
	ctx context.Context,
	cfg Config,
	variables []string,
	sql string,
) (string, error) {
	password, err := os.ReadFile(cfg.DatabasePasswordFile)
	if err != nil {
		return "", fmt.Errorf("read runtime database password: %w", err)
	}

	passwordText := strings.TrimSpace(string(password))
	if passwordText == "" {
		return "", fmt.Errorf("runtime database password is empty")
	}

	passfile, err := os.CreateTemp("", "pathfinder-runtime-pgpass-*")
	if err != nil {
		return "", fmt.Errorf("create runtime PostgreSQL password file: %w", err)
	}
	passfilePath := passfile.Name()
	defer func() {
		_ = os.Remove(passfilePath)
	}()

	if err := passfile.Chmod(0600); err != nil {
		_ = passfile.Close()
		return "", fmt.Errorf("secure runtime PostgreSQL password file: %w", err)
	}

	if _, err := fmt.Fprintf(
		passfile,
		"%s:%d:%s:%s:%s\n",
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
		cfg.DatabaseUser,
		passwordText,
	); err != nil {
		_ = passfile.Close()
		return "", fmt.Errorf("write runtime PostgreSQL password file: %w", err)
	}

	if err := passfile.Close(); err != nil {
		return "", fmt.Errorf("close runtime PostgreSQL password file: %w", err)
	}

	arguments := []string{
		"-X",
		"-A",
		"-t",
		"-q",
		"-v",
		"ON_ERROR_STOP=1",
		"-h",
		cfg.DatabaseHost,
		"-p",
		strconv.Itoa(cfg.DatabasePort),
		"-U",
		cfg.DatabaseUser,
		"-d",
		cfg.DatabaseName,
	}

	for _, variable := range variables {
		arguments = append(arguments, "-v", variable)
	}

	command := exec.CommandContext(ctx, psqlPath, arguments...)
	command.Env = append(
		os.Environ(),
		"PGAPPNAME=pathfinder-source-artifact",
		"PGCONNECT_TIMEOUT=3",
		"PGPASSFILE="+passfilePath,
	)
	command.Stdin = strings.NewReader(sql)

	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"%s: %w",
			strings.TrimSpace(string(output)),
			err,
		)
	}

	return strings.TrimSpace(string(output)), nil
}

func runSourceArtifact(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: pathfinder source-artifact <commit|reconcile>")
	}

	switch args[0] {
	case "commit":
		return runSourceArtifactCommit(args[1:])
	case "reconcile":
		return runSourceArtifactReconcile(args[1:])
	default:
		return fmt.Errorf("unknown source-artifact command %q", args[0])
	}
}

func runSourceArtifactCommit(args []string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("source-artifact commit refuses root; run as Pathfinder runtime identity")
	}

	options, err := sourceArtifactCommitArgs(args)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(options.ConfigPath)
	if err != nil {
		return err
	}

	artifactID := options.SourceArtifactID
	if artifactID == "" {
		artifactID, err = sourceartifact.NewUUIDv7(time.Now())
		if err != nil {
			return err
		}
	}

	request := sourceartifact.CommitRequest{
		ContentEncoding:    options.ContentEncoding,
		HandlingProfile:    options.HandlingProfile,
		MediaType:          options.MediaType,
		RetrievalEventID:   options.RetrievalEventID,
		SourceArtifactID:   artifactID,
		SourceCollectionID: options.SourceCollectionID,
		SourceID:           options.SourceID,
		SourceLocation:     options.SourceLocation,
	}

	result, err := sourceartifact.Commit(
		context.Background(),
		cfg.ArtifactSocket,
		request,
		os.Stdin,
		func(
			ctx context.Context,
			commitRequest sourceartifact.CommitRequest,
			receipt artifactpreserve.Receipt,
		) (sourceartifact.DatabaseCommitResult, error) {
			return commitSourceArtifactDatabase(ctx, cfg, commitRequest, receipt)
		},
	)
	if err != nil {
		return err
	}

	fmt.Println("Pathfinder SourceArtifact commit: PASS")
	fmt.Printf("  source_artifact_id=%s\n", result.SourceArtifactID)
	fmt.Printf("  sha256=%s\n", result.Receipt.SHA256)
	fmt.Printf("  byte_length=%d\n", result.Receipt.ByteLength)
	fmt.Printf("  storage_reference=%s\n", result.Receipt.StorageReference)
	fmt.Printf("  reused=%t\n", result.Receipt.Reused)
	fmt.Printf("  database_commit=%s\n", result.DatabaseState)

	return nil
}

func runSourceArtifactReconcile(args []string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("source-artifact reconcile refuses root; run as Pathfinder runtime identity")
	}

	configPath, err := configPath("source-artifact reconcile", args)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	records, err := loadSourceArtifactRecords(context.Background(), cfg)
	if err != nil {
		return err
	}

	findings, err := sourceartifact.Reconcile(
		filepath.Join(cfg.ArtifactsDir, "objects"),
		records,
	)
	if err != nil {
		return err
	}

	if len(findings) == 0 {
		fmt.Println("Pathfinder SourceArtifact reconciliation: PASS")
		fmt.Printf("  database_records=%d\n", len(records))
		fmt.Println("  findings=0")
		return nil
	}

	fmt.Println("Pathfinder SourceArtifact reconciliation: FINDINGS")
	for _, finding := range findings {
		fmt.Printf(
			"  kind=%s source_artifact_id=%s storage_reference=%s expected_sha256=%s actual_sha256=%s expected_byte_length=%d actual_byte_length=%d\n",
			finding.Kind,
			valueOrNotApplicable(finding.SourceArtifactID),
			valueOrNotApplicable(finding.StorageReference),
			valueOrNotApplicable(finding.ExpectedSHA256),
			valueOrNotApplicable(finding.ActualSHA256),
			finding.ExpectedByteLength,
			finding.ActualByteLength,
		)
	}

	return fmt.Errorf("SourceArtifact reconciliation found %d inconsistency(s)", len(findings))
}

func sourceArtifactCommitArgs(args []string) (sourceArtifactCommitOptions, error) {
	flags := flag.NewFlagSet("source-artifact commit", flag.ContinueOnError)

	config := flags.String(
		"config",
		"/usr/local/etc/pathfinder/pathfinder.conf",
		"Pathfinder configuration file",
	)
	artifactID := flags.String("source-artifact-id", "", "UUIDv7 SourceArtifact identity for retry-safe commit")
	sourceID := flags.String("source-id", "", "Source UUID")
	collectionID := flags.String("source-collection-id", "", "SourceCollection UUID")
	retrievalID := flags.String("retrieval-event-id", "", "RetrievalEvent UUID")
	sourceLocation := flags.String("source-location", "NOT_KNOWN", "source location metadata")
	mediaType := flags.String("media-type", "NOT_KNOWN", "media type metadata")
	contentEncoding := flags.String("content-encoding", "NOT_KNOWN", "content encoding metadata")
	handlingProfile := flags.String("handling-profile", "NOT_KNOWN", "handling profile metadata")

	if err := flags.Parse(args); err != nil {
		return sourceArtifactCommitOptions{}, err
	}

	if flags.NArg() != 0 {
		return sourceArtifactCommitOptions{}, fmt.Errorf("unexpected arguments")
	}

	if *sourceID == "" || *collectionID == "" || *retrievalID == "" {
		return sourceArtifactCommitOptions{}, fmt.Errorf(
			"source-id, source-collection-id, and retrieval-event-id are required",
		)
	}

	return sourceArtifactCommitOptions{
		ConfigPath:         *config,
		ContentEncoding:    *contentEncoding,
		HandlingProfile:    *handlingProfile,
		MediaType:          *mediaType,
		RetrievalEventID:   *retrievalID,
		SourceArtifactID:   *artifactID,
		SourceCollectionID: *collectionID,
		SourceID:           *sourceID,
		SourceLocation:     *sourceLocation,
	}, nil
}

func sourceArtifactRowMatches(
	row sourceArtifactDatabaseRow,
	request sourceartifact.CommitRequest,
	receipt artifactpreserve.Receipt,
) bool {
	return row.SourceArtifactID == request.SourceArtifactID &&
		row.SourceID == request.SourceID &&
		row.SourceCollectionID == request.SourceCollectionID &&
		row.RetrievalEventID == request.RetrievalEventID &&
		row.SourceLocation == request.SourceLocation &&
		row.MediaType == request.MediaType &&
		row.ContentEncoding == request.ContentEncoding &&
		row.ByteLength == receipt.ByteLength &&
		row.SHA256 == receipt.SHA256 &&
		row.PreservationState == "PRESERVED" &&
		row.IntegrityState == "VERIFIED" &&
		row.AvailabilityState == "AVAILABLE" &&
		row.StorageReference == receipt.StorageReference &&
		row.HandlingProfile == request.HandlingProfile
}

func valueOrNotApplicable(value string) string {
	if value == "" {
		return "NOT_APPLICABLE"
	}
	return value
}
