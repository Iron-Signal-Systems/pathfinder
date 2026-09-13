package cisakev

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{
		"catalogVersion":"2026.09.11",
		"dateReleased":"2026-09-11T00:00:00.000Z",
		"count":2,
		"vulnerabilities":[
			{"cveID":"CVE-2026-1234","cwes":["CWE-79"]},
			{"cveID":"NOT-A-CVE"}
		]
	}`))
	f.Add([]byte(`{"catalogVersion":"","vulnerabilities":[]}`))
	f.Add([]byte(`{"vulnerabilities":[null,{},[],1,"x"]}`))
	f.Add([]byte(`{"vulnerabilities":null}`))
	f.Add([]byte(`{`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, body []byte) {
		const maxFuzzBody = 1 << 20
		if len(body) > maxFuzzBody {
			body = body[:maxFuzzBody]
		}

		original := append([]byte(nil), body...)

		catalog, err := Parse(body)

		if !bytes.Equal(body, original) {
			t.Fatal("Parse modified authoritative input bytes")
		}
		if err != nil {
			return
		}

		var envelope struct {
			Vulnerabilities []json.RawMessage `json:"vulnerabilities"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Fatalf("Parse succeeded but envelope decode failed: %v", err)
		}

		if len(catalog.Records) != len(envelope.Vulnerabilities) {
			t.Fatalf(
				"records=%d, vulnerabilities=%d",
				len(catalog.Records),
				len(envelope.Vulnerabilities),
			)
		}

		for index, record := range catalog.Records {
			expectedLocator := fmt.Sprintf(
				"/vulnerabilities/%d",
				index,
			)
			if record.SourceLocator != expectedLocator {
				t.Fatalf(
					"record %d locator=%q, want %q",
					index,
					record.SourceLocator,
					expectedLocator,
				)
			}

			switch record.ProcessingState {
			case ProcessingStateProcessed:
				if record.ValidationState != ValidationStateValid {
					t.Fatalf(
						"processed record validation_state=%q",
						record.ValidationState,
					)
				}
				if record.ProcessingErrorClass != "" {
					t.Fatalf(
						"processed record error_class=%q",
						record.ProcessingErrorClass,
					)
				}
				if !cvePattern.MatchString(record.CVEIdentifier) {
					t.Fatalf(
						"processed CVE identifier invalid: %q",
						record.CVEIdentifier,
					)
				}
				if record.ExternalRecordID != record.CVEIdentifier {
					t.Fatalf(
						"processed external_record_id=%q, cve=%q",
						record.ExternalRecordID,
						record.CVEIdentifier,
					)
				}

			case ProcessingStateFailed:
				if record.ValidationState != ValidationStateInvalid {
					t.Fatalf(
						"failed record validation_state=%q",
						record.ValidationState,
					)
				}
				if record.ProcessingErrorClass != ErrorClassInvalidCVEIdentifier {
					t.Fatalf(
						"failed record error_class=%q",
						record.ProcessingErrorClass,
					)
				}
				if record.CVEIdentifier != NotKnown {
					t.Fatalf(
						"failed record cve_identifier=%q",
						record.CVEIdentifier,
					)
				}

			case ProcessingStateUnsupported:
				if record.ValidationState != ValidationStateNotValidated {
					t.Fatalf(
						"unsupported record validation_state=%q",
						record.ValidationState,
					)
				}
				if record.ProcessingErrorClass != ErrorClassUnsupportedRecordShape {
					t.Fatalf(
						"unsupported record error_class=%q",
						record.ProcessingErrorClass,
					)
				}
				if record.CVEIdentifier != NotKnown ||
					record.ExternalRecordID != NotKnown {
					t.Fatalf(
						"unsupported record identities=%q/%q",
						record.CVEIdentifier,
						record.ExternalRecordID,
					)
				}

			default:
				t.Fatalf(
					"unsupported parser processing_state=%q",
					record.ProcessingState,
				)
			}
		}
	})
}
