BEGIN;

INSERT INTO pathfinder.source (
    name,
    source_type
)
VALUES (
    'Phase 1.2 Source A',
    'INTERNAL'
)
RETURNING source_id AS source_a_id
\gset

INSERT INTO pathfinder.source (
    name,
    source_type
)
VALUES (
    'Phase 1.2 Source B',
    'INTERNAL'
)
RETURNING source_id AS source_b_id
\gset

INSERT INTO pathfinder.source_collection (
    source_id,
    name,
    collection_type,
    transport_type,
    expected_format
)
VALUES (
    :'source_a_id'::uuid,
    'Phase 1.2 Collection A',
    'TEST_COLLECTION',
    'HTTPS',
    'JSON'
)
RETURNING source_collection_id AS collection_a_id
\gset

INSERT INTO pathfinder.retrieval_event (
    source_id,
    source_collection_id
)
VALUES (
    :'source_b_id'::uuid,
    :'collection_a_id'::uuid
);

ROLLBACK;
