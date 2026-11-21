package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyArchivePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "frozen")
	for path, want := range map[string]string{"observations-1.jsonl.active": "observations", "conntrack/conntrack-1.jsonl.gz": "conntrack", "identity/endpoint-identity-1.jsonl.gz": "identity", "health/health-1.jsonl.active": "health"} {
		got, err := classify(root, filepath.Join(root, path))
		if err != nil || got != want {
			t.Fatalf("%s got=%q err=%v", path, got, err)
		}
	}
	if _, err := classify(root, filepath.Join(root, "unexpected/data.jsonl.gz")); err == nil {
		t.Fatal("unexpected source accepted")
	}
}
func TestPreserveFrozenSourceAndVerifyExisting(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "test.jsonl.active")
	original := []byte("one\ntwo\n")
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	digest, n, err := shaFile(source)
	if err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(dir, "private")
	got, err := preserveSegment(source, private, digest, n)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(got); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("custody perms: %v %v", info, err)
	}
	repeated, err := preserveSegment(source, private, digest, n)
	if err != nil || repeated != got {
		t.Fatalf("replay path changed %s %v", repeated, err)
	}
	if err := os.WriteFile(got, []byte("different bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := preserveSegment(source, private, digest, n); err == nil {
		t.Fatal("corrupted custody accepted")
	}
}
func TestPreserveRejectsChangedSourceAfterAudit(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "test.jsonl.active")
	if err := os.WriteFile(source, []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, n, err := shaFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := preserveSegment(source, filepath.Join(dir, "private"), digest, n); err == nil || !strings.Contains(err.Error(), "changed after audit") {
		t.Fatalf("expected drift failure got %v", err)
	}
}


func TestAuditIgnoresOnlyRootTransferArchive(t *testing.T) {
	root := t.TempDir()
	observation := "{\"observed_at\":\"2026-11-20T10:00:00Z\"," +
		"\"schema_version\":\"5\",\"sensor_id\":\"Gateway\"," +
		"\"observed\":{},\"context\":{},\"enrichment\":{}}\n"
	if err := os.WriteFile(filepath.Join(root, "observations-1.jsonl.active"),
		[]byte(observation), 0600); err != nil {
		t.Fatal(err)
	}
	// Transfer packaging is not an observation stream, even when it is
	// colocated with the read-only nine-file source snapshot.
	if err := os.WriteFile(filepath.Join(root, ".pathfinder-transfer.tar"),
		[]byte("transfer packaging, not JSONL"), 0600); err != nil {
		t.Fatal(err)
	}
	files, counts, err := audit(root, "Gateway")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || counts["observations"] != 1 ||
		counts["conntrack"] != 0 || counts["identity"] != 0 ||
		counts["health"] != 0 {
		t.Fatalf("unexpected audit files=%v counts=%v", files, counts)
	}

	// Do not silently discard an unknown root-level object.
	other := filepath.Join(root, ".different-transfer.tar")
	if err := os.WriteFile(other, []byte("unknown"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := audit(root, "Gateway"); err == nil ||
		!strings.Contains(err.Error(), "unexpected input file") {
		t.Fatalf("unknown root-level object silently accepted: %v", err)
	}
	if err := os.Remove(other); err != nil {
		t.Fatal(err)
	}

	// The exemption is for the exact root-level filename, not a wildcard.
	if err := os.Mkdir(filepath.Join(root, "unexpected"), 0700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "unexpected", ".pathfinder-transfer.tar")
	if err := os.WriteFile(nested, []byte("unknown"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := audit(root, "Gateway"); err == nil ||
		!strings.Contains(err.Error(), "unexpected input file") {
		t.Fatalf("nested transfer file silently accepted: %v", err)
	}
}
