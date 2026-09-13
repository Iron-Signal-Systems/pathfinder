ALTER TABLE pathfinder.source_artifact
    ADD CONSTRAINT source_artifact_retrieval_identity_context_key
    UNIQUE (
        source_artifact_id,
        retrieval_event_id,
        source_collection_id,
        source_id
    );

CREATE TABLE pathfinder.processing_run (
    processing_run_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    source_id uuid NOT NULL,

    source_collection_id uuid NOT NULL,

    retrieval_event_id uuid NOT NULL,

    source_artifact_id uuid NOT NULL,

    process_name text NOT NULL
        CHECK (btrim(process_name) <> ''),

    process_version text NOT NULL
        CHECK (btrim(process_version) <> ''),

    started_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT processing_run_artifact_retrieval_context_fk
        FOREIGN KEY (
            source_artifact_id,
            retrieval_event_id,
            source_collection_id,
            source_id
        )
        REFERENCES pathfinder.source_artifact (
            source_artifact_id,
            retrieval_event_id,
            source_collection_id,
            source_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT processing_run_artifact_context_key
        UNIQUE (
            processing_run_id,
            source_artifact_id,
            source_collection_id,
            source_id
        ),

    CONSTRAINT processing_run_contract_key
        UNIQUE (
            processing_run_id,
            source_artifact_id,
            process_name,
            process_version
        )
);

CREATE INDEX processing_run_artifact_version_idx
    ON pathfinder.processing_run (
        source_artifact_id,
        process_name,
        process_version,
        started_at DESC
    );

CREATE INDEX processing_run_collection_started_idx
    ON pathfinder.processing_run (
        source_collection_id,
        started_at DESC
    );

CREATE TABLE pathfinder.processing_event (
    processing_event_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    processing_run_id uuid NOT NULL
        REFERENCES pathfinder.processing_run(processing_run_id)
        ON DELETE RESTRICT,

    event_type text NOT NULL
        CHECK (
            event_type IN (
                'COMPLETED',
                'FAILED',
                'INTERRUPTED'
            )
        ),

    error_class text NOT NULL
        DEFAULT 'NOT_APPLICABLE'
        CHECK (btrim(error_class) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT processing_event_terminal_consistency_check
        CHECK (
            (
                event_type = 'COMPLETED'
                AND error_class = 'NOT_APPLICABLE'
            )
            OR
            (
                event_type IN ('FAILED', 'INTERRUPTED')
                AND error_class <> 'NOT_APPLICABLE'
            )
        ),

    CONSTRAINT processing_event_one_terminal_per_run_key
        UNIQUE (processing_run_id),

    CONSTRAINT processing_event_run_type_key
        UNIQUE (
            processing_run_id,
            event_type
        )
);

CREATE INDEX processing_event_created_idx
    ON pathfinder.processing_event (created_at DESC);

CREATE TABLE pathfinder.source_record_processing (
    source_record_processing_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    processing_run_id uuid NOT NULL,

    source_record_id uuid NOT NULL,

    source_artifact_id uuid NOT NULL,

    source_collection_id uuid NOT NULL,

    source_id uuid NOT NULL,

    validation_state text NOT NULL
        CHECK (
            validation_state IN (
                'NOT_VALIDATED',
                'VALID',
                'INVALID'
            )
        ),

    processing_state text NOT NULL
        CHECK (
            processing_state IN (
                'PROCESSED',
                'FAILED',
                'UNSUPPORTED'
            )
        ),

    processing_error_class text NOT NULL
        DEFAULT 'NOT_APPLICABLE'
        CHECK (btrim(processing_error_class) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT source_record_processing_record_context_fk
        FOREIGN KEY (
            source_record_id,
            source_artifact_id,
            source_collection_id,
            source_id
        )
        REFERENCES pathfinder.source_record (
            source_record_id,
            source_artifact_id,
            source_collection_id,
            source_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT source_record_processing_run_context_fk
        FOREIGN KEY (
            processing_run_id,
            source_artifact_id,
            source_collection_id,
            source_id
        )
        REFERENCES pathfinder.processing_run (
            processing_run_id,
            source_artifact_id,
            source_collection_id,
            source_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT source_record_processing_consistency_check
        CHECK (
            (
                processing_state = 'PROCESSED'
                AND validation_state = 'VALID'
                AND processing_error_class = 'NOT_APPLICABLE'
            )
            OR
            (
                processing_state IN ('FAILED', 'UNSUPPORTED')
                AND processing_error_class <> 'NOT_APPLICABLE'
            )
        ),

    CONSTRAINT source_record_processing_run_record_key
        UNIQUE (
            processing_run_id,
            source_record_id
        )
);

CREATE INDEX source_record_processing_record_idx
    ON pathfinder.source_record_processing (source_record_id);

CREATE INDEX source_record_processing_run_idx
    ON pathfinder.source_record_processing (processing_run_id);

CREATE INDEX source_record_processing_state_idx
    ON pathfinder.source_record_processing (
        processing_state,
        validation_state
    );

GRANT SELECT, INSERT
ON pathfinder.processing_run,
   pathfinder.processing_event,
   pathfinder.source_record_processing
TO pathfinder_app;
