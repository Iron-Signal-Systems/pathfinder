BEGIN;

INSERT INTO pathfinder.source (
    name,
    source_type
)
VALUES (
    'Phase 1.2 Completion Source',
    'INTERNAL'
)
RETURNING source_id AS source_id
\gset

INSERT INTO pathfinder.source_collection (
    source_id,
    name,
    collection_type,
    transport_type,
    expected_format
)
VALUES (
    :'source_id'::uuid,
    'Phase 1.2 Completion Collection',
    'TEST_COLLECTION',
    'HTTPS',
    'JSON'
)
RETURNING source_collection_id AS collection_id
\gset

INSERT INTO pathfinder.retrieval_event (
    source_id,
    source_collection_id,
    completion_state,
    completed_at,
    operation_result
)
VALUES (
    :'source_id'::uuid,
    :'collection_id'::uuid,
    'COMPLETED',
    '-infinity'::timestamp with time zone,
    'SUCCESS'
);

ROLLBACK;
