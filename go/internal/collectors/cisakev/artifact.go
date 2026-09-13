package cisakev

import (
	"fmt"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sourceartifact"
)

// ArtifactParseError means the preserved bytes were read and integrity-verified,
// but the CISA KEV payload could not be parsed under the supported contract.
type ArtifactParseError struct {
	Err error
}

func (err *ArtifactParseError) Error() string {
	return fmt.Sprintf("parse preserved CISA KEV SourceArtifact: %v", err.Err)
}

func (err *ArtifactParseError) Unwrap() error {
	return err.Err
}

// SourceArtifactReadError means the authoritative preserved object could not be
// read or did not satisfy its recorded availability/length/hash/reference
// contract. This is operationally distinct from malformed source JSON.
type SourceArtifactReadError struct {
	Err error
}

func (err *SourceArtifactReadError) Error() string {
	return fmt.Sprintf("read preserved CISA KEV SourceArtifact: %v", err.Err)
}

func (err *SourceArtifactReadError) Unwrap() error {
	return err.Err
}

// ParseSourceArtifact reads and verifies one already-preserved SourceArtifact
// before applying KEV semantics. Preservation remains authoritative and
// interpretation never reads from the acquisition stream directly.
func ParseSourceArtifact(artifactRoot string, record sourceartifact.Record) (Catalog, error) {
	content, err := sourceartifact.ReadAvailable(artifactRoot, record)
	if err != nil {
		return Catalog{}, &SourceArtifactReadError{Err: err}
	}

	catalog, err := Parse(content)
	if err != nil {
		return Catalog{}, &ArtifactParseError{Err: err}
	}

	return catalog, nil
}
