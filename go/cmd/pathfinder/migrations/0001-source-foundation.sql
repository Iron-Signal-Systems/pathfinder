CREATE TABLE pathfinder.schema_migration (
    version bigint PRIMARY KEY
        CHECK (version > 0),

    name text NOT NULL
        CHECK (btrim(name) <> ''),

    sha256 character(64) NOT NULL
        CHECK (sha256 ~ '^[0-9a-f]{64}$'),

    applied_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    applied_by text NOT NULL
        CHECK (btrim(applied_by) <> '')
);

CREATE TABLE pathfinder.source (
    source_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    name text NOT NULL
        CHECK (btrim(name) <> ''),

    source_type text NOT NULL
        CHECK (
            source_type IN (
                'GOVERNMENT',
                'INDUSTRY',
                'ISAC',
                'VENDOR',
                'COMMUNITY',
                'INTERNAL',
                'CUSTOMER',
                'PARTNER',
                'RESEARCH',
                'OTHER'
            )
        ),

    provider_identity text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(provider_identity) <> ''),

    description text NOT NULL
        DEFAULT 'NOT_RECORDED'
        CHECK (btrim(description) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp()
);

CREATE TABLE pathfinder.source_collection (
    source_collection_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    source_id uuid NOT NULL
        REFERENCES pathfinder.source(source_id)
        ON DELETE RESTRICT,

    name text NOT NULL
        CHECK (btrim(name) <> ''),

    collection_type text NOT NULL
        CHECK (btrim(collection_type) <> ''),

    external_collection_identifier text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(external_collection_identifier) <> ''),

    transport_type text NOT NULL
        CHECK (btrim(transport_type) <> ''),

    expected_format text NOT NULL
        CHECK (btrim(expected_format) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    UNIQUE (
        source_collection_id,
        source_id
    )
);

CREATE TABLE pathfinder.retrieval_event (
    retrieval_event_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    source_id uuid NOT NULL,

    source_collection_id uuid NOT NULL,

    started_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    completion_state text NOT NULL
        DEFAULT 'NOT_COMPLETED'
        CHECK (
            completion_state IN (
                'NOT_COMPLETED',
                'COMPLETED'
            )
        ),

    completed_at timestamp with time zone NOT NULL
        DEFAULT '-infinity'::timestamp with time zone,

    operation_result text NOT NULL
        DEFAULT 'IN_PROGRESS'
        CHECK (
            operation_result IN (
                'IN_PROGRESS',
                'SUCCESS',
                'PARTIAL',
                'FAILED',
                'INTERRUPTED'
            )
        ),

    artifacts_received bigint NOT NULL
        DEFAULT 0
        CHECK (artifacts_received >= 0),

    records_reported_state text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (
            records_reported_state IN (
                'KNOWN',
                'NOT_KNOWN'
            )
        ),

    records_reported bigint NOT NULL
        DEFAULT 0
        CHECK (records_reported >= 0),

    error_class text NOT NULL
        DEFAULT 'NOT_APPLICABLE'
        CHECK (btrim(error_class) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT retrieval_event_collection_fk
        FOREIGN KEY (
            source_collection_id,
            source_id
        )
        REFERENCES pathfinder.source_collection (
            source_collection_id,
            source_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT retrieval_event_completion_check
        CHECK (
            (
                completion_state = 'NOT_COMPLETED'
                AND completed_at = '-infinity'::timestamp with time zone
                AND operation_result = 'IN_PROGRESS'
            )
            OR
            (
                completion_state = 'COMPLETED'
                AND completed_at <> '-infinity'::timestamp with time zone
                AND completed_at >= started_at
                AND operation_result <> 'IN_PROGRESS'
            )
        ),

    CONSTRAINT retrieval_event_reported_count_consistency_check
        CHECK (
            (
                records_reported_state = 'NOT_KNOWN'
                AND records_reported = 0
            )
            OR
            records_reported_state = 'KNOWN'
        )
);

GRANT SELECT, INSERT, UPDATE
ON pathfinder.source,
   pathfinder.source_collection,
   pathfinder.retrieval_event
TO pathfinder_app;
