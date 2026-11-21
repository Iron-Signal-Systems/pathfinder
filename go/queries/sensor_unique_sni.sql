-- Pathfinder sensor SoR: unique observed TLS SNI names from PostgreSQL.
-- Source: relational columns in pathfinder.sensor_observation, NOT input JSONL.
-- This intentionally covers every indexed observation, with no time filter.
-- Lowercasing collapses DNS case differences; source records remain unchanged.
SELECT DISTINCT lower(btrim(tls_sni)) AS sni_hostname
FROM pathfinder.sensor_observation
WHERE tls_sni IS NOT NULL
  AND btrim(tls_sni) <> ''
  AND lower(btrim(tls_sni)) NOT IN ('no_record', 'not_known')
ORDER BY sni_hostname;
