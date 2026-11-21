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
