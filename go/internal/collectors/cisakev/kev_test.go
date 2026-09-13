package cisakev

import (
	"strings"
	"testing"
)

func TestCatalogCheckpointValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		catalog Catalog
		want    string
	}{
		{catalog: Catalog{CatalogVersion: "2026.09.13"}, want: "2026.09.13"},
		{catalog: Catalog{CatalogVersion: ""}, want: NotReported},
		{catalog: Catalog{CatalogVersion: "   "}, want: NotReported},
	}

	for _, test := range tests {
		if got := test.catalog.CheckpointValue(); got != test.want {
			t.Fatalf("CheckpointValue() = %q, want %q", got, test.want)
		}
	}
}

func TestParseDoesNotRewriteInputBytesOrSourceFields(t *testing.T) {
	t.Parallel()

	input := []byte(`{"vulnerabilities":[{"cveID":"CVE-2026-1234","notes":"  preserve source spacing  ","cwes":["CWE-79",""]}]}`)
	before := string(input)

	catalog, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if string(input) != before {
		t.Fatal("Parse() modified source artifact bytes")
	}

	if catalog.Records[0].SourceFields.Notes != "  preserve source spacing  " {
		t.Fatalf("Notes = %q, source value changed", catalog.Records[0].SourceFields.Notes)
	}

	if len(catalog.Records[0].SourceFields.CWEReferences) != 2 {
		t.Fatalf("CWEReferences = %#v, source list changed", catalog.Records[0].SourceFields.CWEReferences)
	}

	if !strings.Contains(before, `"cwes":["CWE-79",""]`) {
		t.Fatal("test input changed unexpectedly")
	}
}

func TestParseMalformedArtifactFailsAtArtifactLevel(t *testing.T) {
	t.Parallel()

	if _, err := Parse([]byte(`{"vulnerabilities":[`)); err == nil {
		t.Fatal("Parse() error = nil, want artifact-level JSON error")
	}
}

func TestParsePreservesStableRecordLocatorsAndOutcomes(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"title": "CISA Catalog of Known Exploited Vulnerabilities",
		"catalogVersion": "2026.09.13",
		"dateReleased": "2026-09-13T00:00:00.000Z",
		"count": 3,
		"vulnerabilities": [
			{
				"cveID": "CVE-2026-1234",
				"vendorProject": "Example Vendor",
				"product": "Example Product",
				"vulnerabilityName": "Example vulnerability",
				"dateAdded": "2026-09-13",
				"shortDescription": "Source description",
				"requiredAction": "Apply mitigations",
				"dueDate": "2026-10-01",
				"knownRansomwareCampaignUse": "Unknown",
				"notes": "Source notes",
				"cwes": ["CWE-79"]
			},
			{
				"cveID": "not-a-cve",
				"vendorProject": "Other Vendor"
			},
			"unsupported-record-shape"
		]
	}`)

	catalog, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if catalog.CatalogVersion != "2026.09.13" {
		t.Fatalf("CatalogVersion = %q, want %q", catalog.CatalogVersion, "2026.09.13")
	}

	if catalog.DateReleased != "2026-09-13T00:00:00.000Z" {
		t.Fatalf("DateReleased = %q", catalog.DateReleased)
	}

	if catalog.DeclaredCount != 3 {
		t.Fatalf("DeclaredCount = %d, want 3", catalog.DeclaredCount)
	}

	if len(catalog.Records) != 3 {
		t.Fatalf("len(Records) = %d, want 3", len(catalog.Records))
	}

	valid := catalog.Records[0]
	if valid.SourceLocator != "/vulnerabilities/0" {
		t.Fatalf("valid SourceLocator = %q", valid.SourceLocator)
	}
	if valid.ExternalRecordID != "CVE-2026-1234" {
		t.Fatalf("valid ExternalRecordID = %q", valid.ExternalRecordID)
	}
	if valid.CVEIdentifier != "CVE-2026-1234" {
		t.Fatalf("valid CVEIdentifier = %q", valid.CVEIdentifier)
	}
	if valid.ValidationState != ValidationStateValid {
		t.Fatalf("valid ValidationState = %q", valid.ValidationState)
	}
	if valid.ProcessingState != ProcessingStateProcessed {
		t.Fatalf("valid ProcessingState = %q", valid.ProcessingState)
	}
	if valid.ProcessingErrorClass != "" {
		t.Fatalf("valid ProcessingErrorClass = %q, want empty", valid.ProcessingErrorClass)
	}
	if valid.SourceFields.VendorProject != "Example Vendor" {
		t.Fatalf("VendorProject = %q", valid.SourceFields.VendorProject)
	}
	if len(valid.SourceFields.CWEReferences) != 1 || valid.SourceFields.CWEReferences[0] != "CWE-79" {
		t.Fatalf("CWEReferences = %#v", valid.SourceFields.CWEReferences)
	}

	invalid := catalog.Records[1]
	if invalid.SourceLocator != "/vulnerabilities/1" {
		t.Fatalf("invalid SourceLocator = %q", invalid.SourceLocator)
	}
	if invalid.ExternalRecordID != "not-a-cve" {
		t.Fatalf("invalid ExternalRecordID = %q", invalid.ExternalRecordID)
	}
	if invalid.CVEIdentifier != NotKnown {
		t.Fatalf("invalid CVEIdentifier = %q", invalid.CVEIdentifier)
	}
	if invalid.ValidationState != ValidationStateInvalid {
		t.Fatalf("invalid ValidationState = %q", invalid.ValidationState)
	}
	if invalid.ProcessingState != ProcessingStateFailed {
		t.Fatalf("invalid ProcessingState = %q", invalid.ProcessingState)
	}
	if invalid.ProcessingErrorClass != ErrorClassInvalidCVEIdentifier {
		t.Fatalf("invalid ProcessingErrorClass = %q", invalid.ProcessingErrorClass)
	}

	unsupported := catalog.Records[2]
	if unsupported.SourceLocator != "/vulnerabilities/2" {
		t.Fatalf("unsupported SourceLocator = %q", unsupported.SourceLocator)
	}
	if unsupported.ValidationState != ValidationStateNotValidated {
		t.Fatalf("unsupported ValidationState = %q", unsupported.ValidationState)
	}
	if unsupported.ProcessingState != ProcessingStateUnsupported {
		t.Fatalf("unsupported ProcessingState = %q", unsupported.ProcessingState)
	}
	if unsupported.ProcessingErrorClass != ErrorClassUnsupportedRecordShape {
		t.Fatalf("unsupported ProcessingErrorClass = %q", unsupported.ProcessingErrorClass)
	}
}

func TestParseRequiresVulnerabilitiesArray(t *testing.T) {
	t.Parallel()

	tests := []string{
		`{"catalogVersion":"2026.09.13"}`,
		`{"catalogVersion":"2026.09.13","vulnerabilities":null}`,
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse([]byte(input)); err == nil {
				t.Fatal("Parse() error = nil, want missing vulnerabilities[] error")
			}
		})
	}
}

func TestParseReturnsNonNilEmptyRecordsForEmptyCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := Parse([]byte(`{"catalogVersion":"","vulnerabilities":[]}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if catalog.CatalogVersion != "" {
		t.Fatalf("CatalogVersion = %q, want raw empty source value", catalog.CatalogVersion)
	}

	if catalog.CheckpointValue() != NotReported {
		t.Fatalf("CheckpointValue() = %q, want %q", catalog.CheckpointValue(), NotReported)
	}

	if catalog.Records == nil {
		t.Fatal("Records = nil, want non-nil empty slice")
	}

	if len(catalog.Records) != 0 {
		t.Fatalf("len(Records) = %d, want 0", len(catalog.Records))
	}
}

func TestParseValidatesCVEIdentityWithoutNormalization(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"vulnerabilities": [
			{"cveID":"CVE-2026-1234"},
			{"cveID":"CVE-2026-1234567890123456789"},
			{"cveID":"cve-2026-1234"},
			{"cveID":" CVE-2026-1234 "},
			{"cveID":"CVE-26-1234"},
			{"cveID":""}
		]
	}`)

	catalog, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	wantValid := []bool{true, true, false, false, false, false}
	for index, record := range catalog.Records {
		gotValid := record.ValidationState == ValidationStateValid
		if gotValid != wantValid[index] {
			t.Fatalf("record %d validity = %v, want %v", index, gotValid, wantValid[index])
		}
	}
}
