-- Pathfinder C220: TLS SNI -> observed client and destination IPs.
-- Direct network evidence only. No IP-only attribution or DNS inference.
-- The SNI predicate is identical to sensor_unique_sni.sql (all 78 names).
WITH tls AS (
    SELECT sensor_id,
           lower(btrim(tls_sni)) AS sni_hostname,
           source_ip,
           destination_ip,
           lower(protocol) AS transport_protocol,
           source_port,
           destination_port,
           direction,
           observed_at
    FROM pathfinder.sensor_observation
    WHERE tls_sni IS NOT NULL
      AND btrim(tls_sni) <> ''
      AND lower(btrim(tls_sni)) NOT IN ('no_record', 'not_known')
)
SELECT sni_hostname,
       source_ip::text AS observed_client_ip,
       destination_ip::text AS observed_destination_ip,
       transport_protocol,
       destination_port,
       count(*) AS sni_observation_count,
       count(DISTINCT source_port) AS distinct_observed_client_ports,
       min(observed_at) AS first_observed_at,
       max(observed_at) AS last_observed_at,
       string_agg(DISTINCT direction, ';' ORDER BY direction)
           FILTER (WHERE direction IS NOT NULL) AS capture_directions
FROM tls
GROUP BY sni_hostname, source_ip, destination_ip, transport_protocol,
         destination_port
ORDER BY sni_hostname, observed_client_ip, observed_destination_ip,
         destination_port;
