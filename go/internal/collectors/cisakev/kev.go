package cisakev

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	ErrorClassInvalidCVEIdentifier   = "INVALID_CVE_IDENTIFIER"
	ErrorClassUnsupportedRecordShape = "UNSUPPORTED_RECORD_SHAPE"

	NotKnown    = "NOT_KNOWN"
	NotReported = "NOT_REPORTED"

	ProcessingStateFailed      = "FAILED"
	ProcessingStateProcessed   = "PROCESSED"
	ProcessingStateUnsupported = "UNSUPPORTED"

	ValidationStateInvalid      = "INVALID"
	ValidationStateNotValidated = "NOT_VALIDATED"
	ValidationStateValid        = "VALID"
)

var cvePattern = regexp.MustCompile(`^CVE-[0-9]{4}-[0-9]{4,19}$`)

// Catalog is the source-attributable result of parsing one preserved CISA KEV
// SourceArtifact. It does not own or replace the preserved artifact bytes.
type Catalog struct {
	CatalogVersion string
	DateReleased   string
	DeclaredCount  int
	Records        []Record
}

// Record is one logical vulnerabilities[] entry located inside one preserved
// SourceArtifact. SourceLocator is stable for the artifact regardless of parser
// version.
type Record struct {
	CVEIdentifier        string
	ExternalRecordID     string
	ProcessingErrorClass string
	ProcessingState      string
	SourceFields         SourceFields
	SourceLocator        string
	ValidationState      string
}

// SourceFields retains CISA-specific fields as source-attributable data. Phase
// 1.4 does not promote these values into mutable native Vulnerability
// properties.
type SourceFields struct {
	CWEReferences              []string
	DateAdded                  string
	DueDate                    string
	KnownRansomwareCampaignUse string
	Notes                      string
	Product                    string
	RequiredAction             string
	ShortDescription           string
	VendorProject              string
	VulnerabilityName          string
}

type catalogJSON struct {
	CatalogVersion  string          `json:"catalogVersion"`
	Count           int             `json:"count"`
	DateReleased    string          `json:"dateReleased"`
	Vulnerabilities json.RawMessage `json:"vulnerabilities"`
}

type vulnerabilityJSON struct {
	CVEID                      string   `json:"cveID"`
	CWEs                       []string `json:"cwes"`
	DateAdded                  string   `json:"dateAdded"`
	DueDate                    string   `json:"dueDate"`
	KnownRansomwareCampaignUse string   `json:"knownRansomwareCampaignUse"`
	Notes                      string   `json:"notes"`
	Product                    string   `json:"product"`
	RequiredAction             string   `json:"requiredAction"`
	ShortDescription           string   `json:"shortDescription"`
	VendorProject              string   `json:"vendorProject"`
	VulnerabilityName          string   `json:"vulnerabilityName"`
}

// CheckpointValue returns the source checkpoint value that is safe to persist
// for a successful full-snapshot run.
func (catalog Catalog) CheckpointValue() string {
	if strings.TrimSpace(catalog.CatalogVersion) == "" {
		return NotReported
	}

	return catalog.CatalogVersion
}

// newUnsupportedRecord returns a locatable per-record processing failure for a
// vulnerabilities[] element whose JSON shape cannot be interpreted as a KEV
// record.
func newUnsupportedRecord(index int) Record {
	return Record{
		CVEIdentifier:        NotKnown,
		ExternalRecordID:     NotKnown,
		ProcessingErrorClass: ErrorClassUnsupportedRecordShape,
		ProcessingState:      ProcessingStateUnsupported,
		SourceFields: SourceFields{
			CWEReferences: []string{},
		},
		SourceLocator:   fmt.Sprintf("/vulnerabilities/%d", index),
		ValidationState: ValidationStateNotValidated,
	}
}

// parseRecord translates one locatable vulnerabilities[] element into a
// source-attributable processing result.
func parseRecord(index int, raw json.RawMessage) Record {
	var source vulnerabilityJSON

	if err := json.Unmarshal(raw, &source); err != nil {
		return newUnsupportedRecord(index)
	}

	cweReferences := append([]string{}, source.CWEs...)

	record := Record{
		CVEIdentifier:        NotKnown,
		ExternalRecordID:     NotKnown,
		ProcessingErrorClass: ErrorClassInvalidCVEIdentifier,
		ProcessingState:      ProcessingStateFailed,
		SourceFields: SourceFields{
			CWEReferences:              cweReferences,
			DateAdded:                  source.DateAdded,
			DueDate:                    source.DueDate,
			KnownRansomwareCampaignUse: source.KnownRansomwareCampaignUse,
			Notes:                      source.Notes,
			Product:                    source.Product,
			RequiredAction:             source.RequiredAction,
			ShortDescription:           source.ShortDescription,
			VendorProject:              source.VendorProject,
			VulnerabilityName:          source.VulnerabilityName,
		},
		SourceLocator:   fmt.Sprintf("/vulnerabilities/%d", index),
		ValidationState: ValidationStateInvalid,
	}

	if source.CVEID != "" {
		record.ExternalRecordID = source.CVEID
	}

	if !cvePattern.MatchString(source.CVEID) {
		return record
	}

	record.CVEIdentifier = source.CVEID
	record.ExternalRecordID = source.CVEID
	record.ProcessingErrorClass = ""
	record.ProcessingState = ProcessingStateProcessed
	record.ValidationState = ValidationStateValid

	return record
}

// Parse interprets one preserved CISA KEV HTTP response entity body. The input
// bytes remain authoritative and are never reserialized or rewritten here.
func Parse(body []byte) (Catalog, error) {
	var source catalogJSON

	if err := json.Unmarshal(body, &source); err != nil {
		return Catalog{}, fmt.Errorf("decode CISA KEV catalog: %w", err)
	}

	if len(source.Vulnerabilities) == 0 || string(source.Vulnerabilities) == "null" {
		return Catalog{}, errors.New("CISA KEV catalog does not contain vulnerabilities[]")
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(source.Vulnerabilities, &entries); err != nil {
		return Catalog{}, fmt.Errorf("decode CISA KEV vulnerabilities[]: %w", err)
	}

	records := make([]Record, 0, len(entries))
	for index, raw := range entries {
		records = append(records, parseRecord(index, raw))
	}

	return Catalog{
		CatalogVersion: source.CatalogVersion,
		DateReleased:   source.DateReleased,
		DeclaredCount:  source.Count,
		Records:        records,
	}, nil
}
