package cisakev

const (
	// ExtractionVersion is the stable identifier for the CISA KEV JSON
	// extraction contract used to create source-attributable assertions.
	ExtractionVersion = "cisa-kev-json-v1"

	// ProcessName identifies the semantic processing family. It is deliberately
	// distinct from transport/acquisition identity.
	ProcessName = "cisa-kev"

	// ProcessVersion identifies the current semantic processing contract.
	// Changing parser or semantic behavior that affects persisted meaning must
	// use a new value rather than silently reusing this version.
	ProcessVersion = "1"
)
