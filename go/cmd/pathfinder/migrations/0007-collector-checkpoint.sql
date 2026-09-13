CREATE TABLE pathfinder.collector_checkpoint (
    collector_checkpoint_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    processing_run_id uuid NOT NULL,

    processing_event_type text NOT NULL
        DEFAULT 'COMPLETED'
        CHECK (
            processing_event_type = 'COMPLETED'
        ),

    source_artifact_id uuid NOT NULL,

    process_name text NOT NULL
        CHECK (btrim(process_name) <> ''),

    process_version text NOT NULL
        CHECK (btrim(process_version) <> ''),

    checkpoint_kind text NOT NULL
        CHECK (
            checkpoint_kind = 'FULL_SNAPSHOT'
        ),

    source_checkpoint_value text NOT NULL
        DEFAULT 'NOT_REPORTED'
        CHECK (btrim(source_checkpoint_value) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT collector_checkpoint_completed_run_fk
        FOREIGN KEY (
            processing_run_id,
            processing_event_type
        )
        REFERENCES pathfinder.processing_event (
            processing_run_id,
            event_type
        )
        ON DELETE RESTRICT,

    CONSTRAINT collector_checkpoint_processing_contract_fk
        FOREIGN KEY (
            processing_run_id,
            source_artifact_id,
            process_name,
            process_version
        )
        REFERENCES pathfinder.processing_run (
            processing_run_id,
            source_artifact_id,
            process_name,
            process_version
        )
        ON DELETE RESTRICT,

    CONSTRAINT collector_checkpoint_run_key
        UNIQUE (processing_run_id),

    CONSTRAINT collector_checkpoint_artifact_contract_key
        UNIQUE (
            source_artifact_id,
            process_name,
            process_version
        )
);

CREATE INDEX collector_checkpoint_created_idx
    ON pathfinder.collector_checkpoint (created_at DESC);

GRANT SELECT, INSERT
ON pathfinder.collector_checkpoint
TO pathfinder_app;
