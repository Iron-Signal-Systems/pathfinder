#!/bin/sh
# Deterministic Linux-CI simulation of the READ-ONLY FreeBSD host preflight.
# No host mounts/jails/SQL are touched; fake commands live in temporary PATH.
set -eu
umask 077

base=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/mock" "$tmp/host" "$tmp/ingest" "$tmp/sor"
cat > "$tmp/mock/mock-command" <<'EOF'
#!/bin/sh
set -eu
case "${0##*/}" in
 uname)
  printf '%s\n' FreeBSD ;;
 id)
  [ "$1" = "-u" ] || exit 1
  printf '%s\n' 0 ;;
 sysrc)
  [ "$1" = "-n" ] || exit 1
  case "$2" in
   jail_enable) printf '%s\n' YES ;;
   jail_list) printf '%s\n' 'net-core pathfinder-ingest pathfinder-sor' ;;
   *) exit 1 ;;
  esac ;;
 jls)
  [ "$1" = "-j" ] || exit 1
  case "$2" in
   pathfinder-ingest|pathfinder-sor) printf '101\n' ;;
   *) exit 1 ;;
  esac ;;
 mount)
  [ "$1" = "-p" ] || exit 1
  printf 'tmpfs %s tmpfs rw 0 0\n' "$PF_C1_HOST_RAM"
  printf '%s %s nullfs rw 0 0\n' "$PF_C1_HOST_RAM" "$PF_C1_INGEST_RAM"
  if [ "${PF_C1_SIM_SOR_RW:-0}" = 1 ]; then
   printf '%s %s nullfs rw 0 0\n' "$PF_C1_HOST_RAM" "$PF_C1_SOR_RAM"
  else
   printf '%s %s nullfs ro 0 0\n' "$PF_C1_HOST_RAM" "$PF_C1_SOR_RAM"
  fi ;;
 stat)
  [ "$1" = "-f" ] && [ "$2" = "%g" ] || exit 1
  printf '44444\n' ;;
 jexec)
  [ "$1" = "-U" ] || exit 1
  user=$2
  jail=$3
  shift 3
  case "$1" in
   id)
    case "$2" in
     -u)
      case "$user:$jail" in
       ingestuser:pathfinder-ingest) printf '1001\n' ;;
       soruser:pathfinder-sor) printf '1002\n' ;;
       *) exit 1 ;;
      esac ;;
     -G)
      case "$user:$jail" in
       ingestuser:pathfinder-ingest) printf '1001 44444\n' ;;
       soruser:pathfinder-sor)
        if [ "${PF_C1_SIM_MISSING_GID:-0}" = 1 ]; then
         printf '1002\n'
        else
         printf '1002 44444\n'
        fi ;;
       *) exit 1 ;;
      esac ;;
     *) exit 1 ;;
    esac ;;
   test)
    [ "$3" = "$PF_C1_JAIL_RAM_PATH" ] || exit 1
    case "$user:$jail:$2" in
      ingestuser:pathfinder-ingest:-w|soruser:pathfinder-sor:-r) exit 0 ;;
      *) exit 1 ;;
    esac ;;
   *) exit 1 ;;
  esac ;;
 *)
  exit 1 ;;
esac
EOF
chmod 0700 "$tmp/mock/mock-command"
for command in uname id sysrc jls mount stat jexec; do
 ln -s mock-command "$tmp/mock/$command"
done

export PATH="$tmp/mock:$PATH"
export PF_C1_HOST_RAM="$tmp/host"
export PF_C1_INGEST_RAM="$tmp/ingest"
export PF_C1_SOR_RAM="$tmp/sor"
export PF_C1_JAIL_RAM_PATH="/var/run/pathfinder-c1"
export PF_C1_SHARED_GID="44444"
export PF_C1_INGEST_USER="ingestuser"
export PF_C1_SOR_USER="soruser"

script="$base/tools/pathfinder_c1_c220_preflight.sh"
good=$(sh "$script") || {
 echo "C1 MOCK PREFLIGHT FAIL: valid configuration was rejected" >&2
 exit 1
}
case "$good" in
 *PATHFINDER_C1_C220_RAM_PREFLIGHT_PASS*"shared_gid=44444"*) ;;
 *) echo "C1 MOCK PREFLIGHT FAIL: valid configuration missing full PASS proof" >&2; exit 1 ;;
esac

if PF_C1_SHARED_GID=44445 sh "$script" >/dev/null 2>&1; then
 echo "C1 MOCK PREFLIGHT FAIL: mismatched source group accepted" >&2
 exit 1
fi
if PF_C1_SIM_SOR_RW=1 sh "$script" >/dev/null 2>&1; then
 echo "C1 MOCK PREFLIGHT FAIL: SoR writable nullfs accepted" >&2
 exit 1
fi
if PF_C1_SIM_MISSING_GID=1 sh "$script" >/dev/null 2>&1; then
 echo "C1 MOCK PREFLIGHT FAIL: unshared SoR group accepted" >&2
 exit 1
fi
if PF_C1_JAIL_RAM_PATH="/var/run/has space" sh "$script" >/dev/null 2>&1; then
 echo "C1 MOCK PREFLIGHT FAIL: unsafe mount path accepted" >&2
 exit 1
fi
printf 'PATHFINDER_C1_MOCK_PREFLIGHT_PASS positive=1 negative=4 host_modified=false\n'
