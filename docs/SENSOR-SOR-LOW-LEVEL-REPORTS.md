# Pathfinder low-level sensor SQL reports

**Classification: developer / diagnostic / administrator direct-evidence review.**
These are not polished end-user reports, dashboards, threat conclusions, or
production-ready connection correlations.

## Unique SNI hostname diagnostic

Source query: `go/queries/sensor_unique_sni.sql` (existing committed query).
Runner: `tools/pathfinder_c220_unique_sni_report.sh`.

Lists each distinct normalized, nonempty TLS SNI hostname observed in the
PostgreSQL sensor SoR, one per line, followed by `UNIQUE_SNI_COUNT`.
It reads only committed observations, without parsing the frozen archive or
attempting DNS, flow, or identity correlation.

After pulling `dev/sensor-ingest-20261121`, run **on C220 as root**:

```sh
sh /fast/jails/roots/dev-pathfinder/home/jwood/pathfinder/tools/pathfinder_c220_unique_sni_report.sh
```

The script runs the existing SNI-only importer query mode inside
`pathfinder-sor` using local PostgreSQL at `127.0.0.1:5432`.
The historical frozen import previously reconciled **685,218 rows** and
returned **78 unique SNI names**; subsequent reporting reads the current
committed database and may return a different count if new observations
are ingested. This runner performs no ingestion, DDL, or other writes.

## SNI → TO / FROM IP diagnostic

File: `go/queries/sensor_sni_ip_diagnostic.sql`

One block per distinct normalized TLS SNI hostname observed in PostgreSQL.
It lists each unique destination (TO) address and each unique source (FROM)
address, in separate columns for readable SSH terminal output.

Example (illustrative addresses):

```text
SNI: courier.push.apple.com

  TO:                                        FROM:
  17.57.144.10                               192.168.1.114
  17.57.144.11                               192.168.1.163
  17.57.147.7                                192.168.1.193
```

**The two IP lists are independent sets for the SNI; entries aligned on the
same display row are not asserted to have communicated with each other.**
The underlying `sensor_observation` records contain the actual source/
destination pair for every observed TLS ClientHello.

### Run on C220

On the **C220 FreeBSD host as root**, update the existing development checkout
once, then execute this report:

```sh
jexec -U jwood dev-pathfinder /bin/sh -c '
  cd /home/jwood/pathfinder
  git pull --ff-only origin dev/sensor-ingest-20261121
'
sh /fast/jails/roots/dev-pathfinder/home/jwood/pathfinder/tools/pathfinder_c220_sni_ip_report.sh
```

The script supplies the committed SQL over standard input to the local
`pathfinder_app` connection inside `pathfinder-sor` at
`127.0.0.1:5432`. It does not rebuild, import, modify database schema,
create tables, write report files, or change any jail/network settings.
You can run it repeatedly whenever fresh observations are present.

**Scope and limits:** Uses only nonempty `tls_sni`, `source_ip`, and
`destination_ip` in the committed `pathfinder.sensor_observation` rows.
SNI names are lowercased and trimmed; destination and source IPs are
independently de-duplicated. NAT translations and source-device identity are
not inferred. A host can have multiple TO and FROM addresses.

The separate `sensor_sni_ip_pairs.sql` and
`sensor_sni_conntrack_pairs.sql` experiments are more detailed engineering
investigations; this terminal report is the default quick-look low-level
SNI/IP diagnostic. Rich analytical reporting remains a separate future
product feature, with different design and acceptance criteria.
