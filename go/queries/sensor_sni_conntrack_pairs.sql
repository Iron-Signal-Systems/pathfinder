-- Pathfinder C220: SNI flow -> candidate conntrack original/reply tuple.
-- Correlation requires the same sensor, the complete TCP/UDP 5-tuple and a
-- time-bounded conntrack event. IP-only matches are NEVER attributed to SNI.
-- conntrack_match='CANDIDATE' even on exact tuple: no stable per-flow ID
-- connects the sensor observation to conntrack, and ports can be reused.
-- Unmatched TLS flows are preserved with null conntrack columns.
WITH sni_flow AS MATERIALIZED (
    SELECT sensor_id,
           lower(btrim(tls_sni)) AS sni_hostname,
           source_ip,
           destination_ip,
           lower(protocol) AS transport_protocol,
           source_port,
           destination_port,
           min(observed_at) AS first_sni_at,
           max(observed_at) AS last_sni_at,
           count(*) AS sni_observations
    FROM pathfinder.sensor_observation
    WHERE tls_sni IS NOT NULL
      AND btrim(tls_sni) <> ''
      AND lower(btrim(tls_sni)) NOT IN ('no_record', 'not_known')
    GROUP BY sensor_id, lower(btrim(tls_sni)), source_ip,
             destination_ip, lower(protocol), source_port, destination_port
)
SELECT f.sni_hostname,
       f.source_ip::text AS observed_client_ip,
       f.source_port AS observed_client_port,
       f.destination_ip::text AS observed_destination_ip,
       f.destination_port AS observed_destination_port,
       f.transport_protocol,
       f.sni_observations,
       f.first_sni_at,
       f.last_sni_at,
       CASE WHEN c.conntrack_id IS NULL THEN 'UNMATCHED'
            ELSE 'CANDIDATE_EXACT_5TUPLE' END AS conntrack_match,
       c.conntrack_id,
       c.event_type AS matched_event_type,
       c.observed_at AS matched_conntrack_event_at,
       c.original_source_ip::text AS conntrack_original_source_ip,
       c.original_source_port AS conntrack_original_source_port,
       c.original_destination_ip::text AS conntrack_original_destination_ip,
       c.original_destination_port AS conntrack_original_destination_port,
       c.reply_source_ip::text AS conntrack_reply_source_ip,
       c.reply_source_port AS conntrack_reply_source_port,
       c.reply_destination_ip::text AS conntrack_reply_destination_ip,
       c.reply_destination_port AS conntrack_reply_destination_port,
       c.seen_reply AS conntrack_seen_reply,
       c.source_nat AS conntrack_source_nat,
       c.destination_nat AS conntrack_destination_nat
FROM sni_flow f
LEFT JOIN LATERAL (
    SELECT c.conntrack_id,
           c.event_type,
           c.observed_at,
           c.event_id,
           c.original_source_ip,
           c.original_source_port,
           c.original_destination_ip,
           c.original_destination_port,
           c.reply_source_ip,
           c.reply_source_port,
           c.reply_destination_ip,
           c.reply_destination_port,
           c.seen_reply,
           c.source_nat,
           c.destination_nat
    FROM pathfinder.sensor_conntrack_event c
    WHERE c.sensor_id = f.sensor_id
      AND f.source_ip IS NOT NULL
      AND f.destination_ip IS NOT NULL
      AND f.source_port IS NOT NULL
      AND f.destination_port IS NOT NULL
      AND f.transport_protocol IS NOT NULL
      AND c.observed_at >= f.first_sni_at - interval '2 minutes'
      AND c.observed_at <= f.last_sni_at + interval '2 minutes'
      AND (
          (
            lower(c.original_protocol) = f.transport_protocol
            AND c.original_source_ip = f.source_ip
            AND c.original_destination_ip = f.destination_ip
            AND c.original_source_port = f.source_port
            AND c.original_destination_port = f.destination_port
          ) OR (
            lower(c.reply_protocol) = f.transport_protocol
            AND c.reply_source_ip = f.source_ip
            AND c.reply_destination_ip = f.destination_ip
            AND c.reply_source_port = f.source_port
            AND c.reply_destination_port = f.destination_port
          )
      )
    ORDER BY c.seen_reply DESC,
             abs(extract(epoch FROM c.observed_at - f.first_sni_at)),
             c.event_id
    LIMIT 1
) c ON true
ORDER BY f.sni_hostname, f.source_ip, f.destination_ip,
         f.destination_port, f.source_port;
