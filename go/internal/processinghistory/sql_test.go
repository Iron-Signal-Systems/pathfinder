package processinghistory

import (
	"strings"
	"testing"
)

func TestBuildStartRunSQLRequiresAssignedIdentity(t *testing.T) {
	t.Parallel()

	run := validAssignedRun()
	run.ProcessingRunID = ""

	if _, err := BuildStartRunSQL(run); err == nil {
		t.Fatal("BuildStartRunSQL() error = nil, want missing identity rejection")
	}
}

func TestBuildStartRunSQLUsesRetrySafeIdentityAndJSONPayload(t *testing.T) {
	t.Parallel()

	run := validAssignedRun()
	run.ProcessName = `cisakev'); DROP TABLE pathfinder.processing_run; --`

	sql, err := BuildStartRunSQL(run)
	if err != nil {
		t.Fatalf("BuildStartRunSQL() error = %v", err)
	}

	for _, required := range []string{
		"jsonb_to_record(",
		"INSERT INTO pathfinder.processing_run",
		"ON CONFLICT (processing_run_id) DO NOTHING",
		`"processing_run_id":"01990000-0000-7000-8000-000000000001"`,
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("SQL missing %q", required)
		}
	}

	if !strings.Contains(sql, `"process_name":"cisakev'); DROP TABLE pathfinder.processing_run; --"`) {
		t.Fatal("source text was not preserved inside JSON payload")
	}
}

func TestBuildTerminalEventSQLAllowsFailureTerminals(t *testing.T) {
	t.Parallel()

	for _, event := range []TerminalEvent{
		FailedEvent("run-1", "PARSER_FAILURE"),
		InterruptedEvent("run-2", "STALE_PROCESSING_RUN"),
	} {
		sql, err := BuildTerminalEventSQL(event)
		if err != nil {
			t.Fatalf("BuildTerminalEventSQL() error = %v", err)
		}

		if !strings.Contains(sql, "INSERT INTO pathfinder.processing_event") {
			t.Fatal("terminal SQL missing processing_event insert")
		}
	}
}

func TestBuildTerminalEventSQLRejectsCompleted(t *testing.T) {
	t.Parallel()

	if _, err := BuildTerminalEventSQL(CompletedEvent("run-1")); err == nil {
		t.Fatal("BuildTerminalEventSQL() error = nil, want COMPLETED rejection")
	}
}

func validAssignedRun() Run {
	return Run{
		ProcessName:        "cisakev",
		ProcessVersion:     "1",
		ProcessingRunID:    "01990000-0000-7000-8000-000000000001",
		RetrievalEventID:   "01990000-0000-7000-8000-000000000002",
		SourceArtifactID:   "01990000-0000-7000-8000-000000000003",
		SourceCollectionID: "01990000-0000-7000-8000-000000000004",
		SourceID:           "01990000-0000-7000-8000-000000000005",
	}
}
