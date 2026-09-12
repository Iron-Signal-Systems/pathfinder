BEGIN;

INSERT INTO pathfinder.source (
    name,
    source_type,
    provider_identity,
    description
)
VALUES (
    'Phase 1.2 Source A',
    'INTERNAL',
    'PATHFINDER_TEST',
    'Phase 1.2 behavioral validation source'
)
RETURNING
    source_id AS source_a_id,
    uuid_extract_version(source_id) AS source_a_uuid_version
\gset

SELECT
    CASE
        WHEN :'source_a_uuid_version' = '7'
            THEN 'PASS: Source uses UUIDv7.'
        ELSE 'FAIL: Source UUID is not version 7.'
    END;

INSERT INTO pathfinder.source_collection (
    source_id,
    name,
    collection_type,
    external_collection_identifier,
    transport_type,
    expected_format
)
VALUES (
    :'source_a_id'::uuid,
    'Phase 1.2 Collection A',
    'TEST_COLLECTION',
    'NOT_KNOWN',
    'HTTPS',
    'JSON'
)
RETURNING
    source_collection_id AS collection_a_id,
    uuid_extract_version(source_collection_id) AS collection_a_uuid_version
\gset

SELECT
    CASE
        WHEN :'collection_a_uuid_version' = '7'
            THEN 'PASS: SourceCollection uses UUIDv7.'
        ELSE 'FAIL: SourceCollection UUID is not version 7.'
    END;

INSERT INTO pathfinder.retrieval_event (
    source_id,
    source_collection_id
)
VALUES (
    :'source_a_id'::uuid,
    :'collection_a_id'::uuid
)
RETURNING
    retrieval_event_id AS retrieval_a_id,
    uuid_extract_version(retrieval_event_id) AS retrieval_a_uuid_version
\gset

SELECT
    CASE
        WHEN :'retrieval_a_uuid_version' = '7'
            THEN 'PASS: RetrievalEvent uses UUIDv7.'
        ELSE 'FAIL: RetrievalEvent UUID is not version 7.'
    END;

SELECT
    CASE
        WHEN completion_state = 'NOT_COMPLETED'
         AND completed_at = '-infinity'::timestamp with time zone
         AND operation_result = 'IN_PROGRESS'
         AND artifacts_received = 0
         AND records_reported_state = 'NOT_KNOWN'
         AND records_reported = 0
         AND error_class = 'NOT_APPLICABLE'
            THEN 'PASS: RetrievalEvent defaults are explicit and consistent.'
        ELSE 'FAIL: RetrievalEvent defaults are inconsistent.'
    END
FROM pathfinder.retrieval_event
WHERE retrieval_event_id = :'retrieval_a_id'::uuid;

UPDATE pathfinder.retrieval_event
SET
    completion_state = 'COMPLETED',
    completed_at = clock_timestamp(),
    operation_result = 'SUCCESS',
    artifacts_received = 1,
    records_reported_state = 'KNOWN',
    records_reported = 1
WHERE retrieval_event_id = :'retrieval_a_id'::uuid;

SELECT
    CASE
        WHEN completion_state = 'COMPLETED'
         AND completed_at <> '-infinity'::timestamp with time zone
         AND completed_at >= started_at
         AND operation_result = 'SUCCESS'
         AND artifacts_received = 1
         AND records_reported_state = 'KNOWN'
         AND records_reported = 1
            THEN 'PASS: RetrievalEvent completed-state transition is valid.'
        ELSE 'FAIL: RetrievalEvent completed-state transition is invalid.'
    END
FROM pathfinder.retrieval_event
WHERE retrieval_event_id = :'retrieval_a_id'::uuid;

UPDATE pathfinder.source
SET description = 'Phase 1.2 behavioral validation source updated'
WHERE source_id = :'source_a_id'::uuid;

SELECT
    CASE
        WHEN description = 'Phase 1.2 behavioral validation source updated'
            THEN 'PASS: Runtime UPDATE permission works on Source.'
        ELSE 'FAIL: Runtime UPDATE permission failed on Source.'
    END
FROM pathfinder.source
WHERE source_id = :'source_a_id'::uuid;

ROLLBACK;
