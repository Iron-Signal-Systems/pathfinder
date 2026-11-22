-- Pathfinder LOW-LEVEL / DEVELOPMENT / ADMINISTRATOR diagnostic report.
-- One block per unique SNI, with independent distinct observed destination
-- and source IP lists, displayed side by side for terminal review.
-- IMPORTANT: TO and FROM entries on the same visual row are NOT flow pairs.
-- This is direct sensor observation evidence, not DNS resolution, conntrack,
-- NAT translation, reconstructed sessions, or a consumer reporting API.
-- Read-only; requires SELECT on pathfinder.sensor_observation.
WITH filtered AS (
    SELECT
        lower(btrim(tls_sni)) AS sni,
        host(destination_ip) AS to_ip,
        host(source_ip) AS from_ip
    FROM pathfinder.sensor_observation
    WHERE tls_sni IS NOT NULL
      AND btrim(tls_sni) <> ''
      AND lower(btrim(tls_sni)) NOT IN ('no_record', 'not_known')
      AND source_ip IS NOT NULL
      AND destination_ip IS NOT NULL
),
to_list AS (
    SELECT
        sni,
        to_ip,
        row_number() OVER (PARTITION BY sni ORDER BY to_ip) AS rn
    FROM (SELECT DISTINCT sni, to_ip FROM filtered) AS t
),
from_list AS (
    SELECT
        sni,
        from_ip,
        row_number() OVER (PARTITION BY sni ORDER BY from_ip) AS rn
    FROM (SELECT DISTINCT sni, from_ip FROM filtered) AS f
),
combined AS (
    SELECT
        coalesce(t.sni, f.sni) AS sni,
        coalesce(t.rn, f.rn) AS rn,
        t.to_ip,
        f.from_ip
    FROM to_list AS t
    FULL JOIN from_list AS f
      ON t.sni = f.sni
     AND t.rn = f.rn
)
SELECT
    E'SNI: ' || sni ||
    E'\n\n  ' ||
    rpad('TO:', 43) || 'FROM:' ||
    E'\n' ||
    string_agg(
        '  ' || rpad(coalesce(to_ip, ''), 43) || coalesce(from_ip, ''),
        E'\n' ORDER BY rn
    ) || E'\n'
FROM combined
GROUP BY sni
ORDER BY sni;
