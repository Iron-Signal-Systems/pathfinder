ALTER TABLE pathfinder.source_artifact
    ADD CONSTRAINT source_artifact_identity_context_key
    UNIQUE (
        source_artifact_id,
        source_collection_id,
        source_id
    );

CREATE TABLE pathfinder.source_record (
    source_record_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    source_artifact_id uuid NOT NULL,

    source_id uuid NOT NULL,

    source_collection_id uuid NOT NULL,

    external_record_id text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(external_record_id) <> ''),

    source_locator text NOT NULL
        CHECK (btrim(source_locator) <> ''),

    source_marking text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(source_marking) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT source_record_artifact_context_fk
        FOREIGN KEY (
            source_artifact_id,
            source_collection_id,
            source_id
        )
        REFERENCES pathfinder.source_artifact (
            source_artifact_id,
            source_collection_id,
            source_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT source_record_artifact_locator_key
        UNIQUE (
            source_artifact_id,
            source_locator
        ),

    CONSTRAINT source_record_identity_context_key
        UNIQUE (
            source_record_id,
            source_artifact_id,
            source_collection_id,
            source_id
        )
);

CREATE INDEX source_record_external_record_id_idx
    ON pathfinder.source_record (external_record_id)
    WHERE external_record_id <> 'NOT_KNOWN';

CREATE INDEX source_record_source_artifact_idx
    ON pathfinder.source_record (source_artifact_id);

CREATE INDEX source_record_source_collection_idx
    ON pathfinder.source_record (source_collection_id);

GRANT SELECT, INSERT
ON pathfinder.source_record
TO pathfinder_app;
