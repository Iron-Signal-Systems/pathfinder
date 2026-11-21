-- One-time operator registration of the frozen UDM-Pro sensor identity.
-- Execute as pathfinder_migrator (must be allowed SET ROLE pathfinder_owner).
-- psql -X -v ON_ERROR_STOP=1 -v sensor_key=YOUR_SENSOR_ID
--      -v sensor_name='UDM-Pro' -f go/queries/sensor_register.sql
-- The sensor_key must match the embedded observation sensor_id exactly.
\set ON_ERROR_STOP on
BEGIN;
SET ROLE pathfinder_owner;
SELECT set_config('pathfinder.sensor_registration_key', :'sensor_key', true);
INSERT INTO pathfinder.sensor (sensor_key,sensor_type,display_name)
VALUES (:'sensor_key','unifi_udm_pro',:'sensor_name')
ON CONFLICT (sensor_key) DO NOTHING;
DO $registration$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pathfinder.sensor
     WHERE sensor_key = current_setting('pathfinder.sensor_registration_key') AND sensor_type = 'unifi_udm_pro'
  ) THEN RAISE EXCEPTION 'sensor registration conflict'; END IF;
END;
$registration$;
SELECT sensor_id, sensor_key, sensor_type FROM pathfinder.sensor
WHERE sensor_key=:'sensor_key';
COMMIT;
