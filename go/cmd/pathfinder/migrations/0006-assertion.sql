CREATE TABLE pathfinder.assertion (
    assertion_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    source_record_processing_id uuid NOT NULL
        REFERENCES pathfinder.source_record_processing(source_record_processing_id)
        ON DELETE RESTRICT,

    vulnerability_id uuid NOT NULL
        REFERENCES pathfinder.vulnerability(vulnerability_id)
        ON DELETE RESTRICT,

    subject_type text NOT NULL
        DEFAULT 'VULNERABILITY'
        CHECK (
            subject_type = 'VULNERABILITY'
        ),

    assertion_type text NOT NULL
        CHECK (
            assertion_type = 'KNOWN_EXPLOITED'
        ),

    asserted_value text NOT NULL
        CHECK (
            asserted_value = 'TRUE'
        ),

    extraction_method text NOT NULL
        CHECK (
            extraction_method = 'SOURCE_STRUCTURED'
        ),

    extraction_version text NOT NULL
        CHECK (btrim(extraction_version) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT assertion_processing_identity_key
        UNIQUE (
            source_record_processing_id,
            vulnerability_id,
            assertion_type,
            extraction_method,
            extraction_version
        )
);

CREATE INDEX assertion_source_record_processing_idx
    ON pathfinder.assertion (source_record_processing_id);

CREATE INDEX assertion_vulnerability_idx
    ON pathfinder.assertion (vulnerability_id);

GRANT SELECT, INSERT
ON pathfinder.assertion
TO pathfinder_app;
