ALTER TABLE pathfinder.retrieval_event
    ADD CONSTRAINT retrieval_event_identity_context_key
    UNIQUE (
        retrieval_event_id,
        source_collection_id,
        source_id
    );

CREATE TABLE pathfinder.source_artifact (
    source_artifact_id uuid PRIMARY KEY
        DEFAULT uuidv7(),

    source_id uuid NOT NULL,

    source_collection_id uuid NOT NULL,

    retrieval_event_id uuid NOT NULL,

    received_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    source_location text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(source_location) <> ''),

    media_type text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(media_type) <> ''),

    content_encoding text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(content_encoding) <> ''),

    byte_length bigint NOT NULL
        DEFAULT 0
        CHECK (byte_length >= 0),

    sha256 text NOT NULL
        DEFAULT 'NOT_ESTABLISHED'
        CHECK (
            sha256 = 'NOT_ESTABLISHED'
            OR sha256 ~ '^[0-9a-f]{64}$'
        ),

    preservation_state text NOT NULL
        CHECK (
            preservation_state IN (
                'RECEIVING',
                'PRESERVED',
                'PARTIAL',
                'FAILED'
            )
        ),

    integrity_state text NOT NULL
        DEFAULT 'NOT_VERIFIED'
        CHECK (
            integrity_state IN (
                'NOT_VERIFIED',
                'VERIFIED',
                'MISMATCH'
            )
        ),

    availability_state text NOT NULL
        DEFAULT 'UNAVAILABLE'
        CHECK (
            availability_state IN (
                'AVAILABLE',
                'QUARANTINED',
                'UNAVAILABLE',
                'DESTROYED_BY_POLICY'
            )
        ),

    storage_reference text NOT NULL
        DEFAULT 'NOT_APPLICABLE'
        CHECK (btrim(storage_reference) <> ''),

    handling_profile text NOT NULL
        DEFAULT 'NOT_KNOWN'
        CHECK (btrim(handling_profile) <> ''),

    created_at timestamp with time zone NOT NULL
        DEFAULT clock_timestamp(),

    CONSTRAINT source_artifact_retrieval_context_fk
        FOREIGN KEY (
            retrieval_event_id,
            source_collection_id,
            source_id
        )
        REFERENCES pathfinder.retrieval_event (
            retrieval_event_id,
            source_collection_id,
            source_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT source_artifact_preserved_content_check
        CHECK (
            preservation_state <> 'PRESERVED'
            OR (
                sha256 ~ '^[0-9a-f]{64}$'
                AND storage_reference <> 'NOT_APPLICABLE'
            )
        ),

    CONSTRAINT source_artifact_verified_content_check
        CHECK (
            integrity_state <> 'VERIFIED'
            OR (
                preservation_state = 'PRESERVED'
                AND sha256 ~ '^[0-9a-f]{64}$'
                AND storage_reference <> 'NOT_APPLICABLE'
            )
        ),

    CONSTRAINT source_artifact_available_content_check
        CHECK (
            availability_state NOT IN ('AVAILABLE', 'QUARANTINED')
            OR (
                preservation_state IN ('PRESERVED', 'PARTIAL')
                AND sha256 ~ '^[0-9a-f]{64}$'
                AND storage_reference <> 'NOT_APPLICABLE'
            )
        )
);

CREATE INDEX source_artifact_sha256_idx
    ON pathfinder.source_artifact (sha256)
    WHERE sha256 <> 'NOT_ESTABLISHED';

CREATE INDEX source_artifact_retrieval_event_idx
    ON pathfinder.source_artifact (retrieval_event_id);

GRANT SELECT, INSERT
ON pathfinder.source_artifact
TO pathfinder_app;
