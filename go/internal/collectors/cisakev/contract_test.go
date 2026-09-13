package cisakev

import "testing"

func TestProductionProcessingContractIdentity(t *testing.T) {
	t.Parallel()

	if ProcessName != "cisa-kev" {
		t.Fatalf("ProcessName = %q", ProcessName)
	}
	if ProcessVersion != "1" {
		t.Fatalf("ProcessVersion = %q", ProcessVersion)
	}
	if ExtractionVersion != "cisa-kev-json-v1" {
		t.Fatalf("ExtractionVersion = %q", ExtractionVersion)
	}
}
