BEGIN;

INSERT INTO pathfinder.source (
    name,
    source_type
)
VALUES (
    'Phase 1.2 Count Source',
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
    'Phase 1.2 Count Collection',
    'TEST_COLLECTION',
    'HTTPS',
    'JSON'
)
RETURNING source_collection_id AS collection_id
\gset

INSERT INTO pathfinder.retrieval_event (
    source_id,
    source_collection_id,
    records_reported_state,
    records_reported
)
VALUES (
    :'source_id'::uuid,
    :'collection_id'::uuid,
    'NOT_KNOWN',
    5
);

ROLLBACK;
