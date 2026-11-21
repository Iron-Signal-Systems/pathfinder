#!/bin/sh
# Read-only full-archive SoR preflight inside the C220 dev-pathfinder jail.
# Run as jail root: frozen files include root:wheel 0640.
set -eu
umask 077
root=$${1:-/mnt/udm-pro-20261120-0500EST}
repo=$${PATHFINDER_REPO:-/home/jwood/pathfinder}
python=/usr/local/bin/python3.12
go=/usr/local/bin/go
[ -x "$python" ] || { echo "Python3.12 unavailable" >&2; exit 1; }
[ -x "$go" ] || { echo "Go unavailable" >&2; exit 1; }
[ -d "$root" ] || { echo "Frozen archive not found: $root" >&2; exit 1; }
[ -f "$repo/go/go.mod" ] || { echo "Pathfinder checkout not found: $repo" >&2; exit 1; }
sensor_key=$(SENSOR_ARCHIVE_ROOT="$root" "$python" - <<'PY'
import gzip
import json
import os
from pathlib import Path
root = Path(os.environ["SENSOR_ARCHIVE_ROOT"])
files = sorted(root.glob("observations-*.jsonl.*"))
if not files:
    raise SystemExit("No observations archive for sensor-key consistency check")
path = files[0]
with (gzip.open(path, "rt") if path.name.endswith(".gz") else open(path, "rt")) as fh:
    line = next((line for line in fh if line.strip()), None)
if not line:
    raise SystemExit("Empty first observations file")
key = json.loads(line).get("sensor_id")
if not isinstance(key, str) or not key:
    raise SystemExit("Missing source sensor identity")
print(key)
PY
)
[ -n "$sensor_key" ] || { echo "Source sensor ID is empty" >&2; exit 1; }
echo "SENSOR SOR FROZEN ARCHIVE: read-only projection"
echo "SOURCE_ROOT=$root"
echo "SENSOR_KEY=$sensor_key"
cd "$repo/go"
exec "$go" run ./cmd/sensor-sor-import \
  -mode dry-run \
  -root "$root" \
  -sensor-key "$sensor_key" \
  -expect-files 9 \
  -expect-rows 685218
