#!/bin/sh
# Linux CI simulation of read-only C220 duplex tmpfs preflight.
# All host commands are deterministic mocks; no mounts or jails modified.
set -eu
umask 077
base=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/mock" "$tmp/hostwork" "$tmp/hostack" \
 "$tmp/ingwork" "$tmp/sorwork" "$tmp/sorack" "$tmp/ingack"
cat > "$tmp/mock/mock-command" <<'MOCK'
#!/bin/sh
set -eu
case "${0##*/}" in
 uname) printf '%s\n' FreeBSD ;;
 id) [ "$1" = "-u" ] && printf '0\n' ;;
 sysrc)
  [ "$1" = "-n" ] || exit 1
  case "$2" in
   jail_enable) printf 'YES\n' ;;
   jail_list)
    if [ "${PF_SIM_EMPTY_JAIL_LIST:-0}" = 1 ]; then printf '\n'
    elif [ "${PF_SIM_EXCLUDE_SOR:-0}" = 1 ]; then printf 'pathfinder-ingest\n'
    else printf 'pathfinder-ingest pathfinder-sor\n'; fi ;;
   *) exit 1 ;;
  esac ;;
 jls)
  [ "$1" = "-j" ] || exit 1
  case "$2" in pathfinder-ingest|pathfinder-sor) printf '12\n';; *) exit 1;; esac ;;
 mount)
  [ "$1" = "-p" ] || exit 1
  printf 'tmpfs %s tmpfs rw,noexec,nosuid 0 0\n' "$PF_C1_HOST_WORK"
  printf 'tmpfs %s tmpfs rw,noexec,nosuid 0 0\n' "$PF_C1_HOST_RECEIPTS"
  printf '%s %s nullfs rw 0 0\n' "$PF_C1_HOST_WORK" "$PF_C1_INGEST_WORK"
  printf '%s %s nullfs %s 0 0\n' "$PF_C1_HOST_WORK" "$PF_C1_SOR_WORK" "${PF_SIM_SOR_WORK_MODE:-ro}"
  printf '%s %s nullfs rw 0 0\n' "$PF_C1_HOST_RECEIPTS" "$PF_C1_SOR_RECEIPTS"
  printf '%s %s nullfs %s 0 0\n' "$PF_C1_HOST_RECEIPTS" "$PF_C1_INGEST_RECEIPTS" "${PF_SIM_ING_ACK_MODE:-ro}"
  ;;
 stat)
  [ "$1" = "-f" ] || exit 1
  fmt=$2; path=$3
  case "$fmt" in
   %u)
    if [ "$path" = "$PF_C1_HOST_WORK" ]; then printf '1001\n'
    elif [ "$path" = "$PF_C1_HOST_RECEIPTS" ]; then printf '1002\n'
    else exit 1; fi ;;
   %g) printf '44444\n' ;;
   %Lp) printf '750\n' ;;
   *) exit 1 ;;
  esac ;;
 jexec)
  case "$1" in
   -U)
    user=$2; jail=$3; shift 3
    [ "$1" = id ] || {
      [ "$1" = test ] || exit 1
      case "$user:$jail:$2:$3" in
       ingestuser:pathfinder-ingest:-w:"$PF_C1_JAIL_WORK_PATH"|ingestuser:pathfinder-ingest:-r:"$PF_C1_JAIL_RECEIPT_PATH"|soruser:pathfinder-sor:-r:"$PF_C1_JAIL_WORK_PATH"|soruser:pathfinder-sor:-w:"$PF_C1_JAIL_RECEIPT_PATH") exit 0;;
       *) exit 1;;
      esac
    }
    case "$user:$jail:$2" in
     ingestuser:pathfinder-ingest:-u) printf '1001\n';;
     soruser:pathfinder-sor:-u) printf '1002\n';;
     ingestuser:pathfinder-ingest:-G) printf '1001 44444\n';;
     soruser:pathfinder-sor:-G)
      if [ "${PF_SIM_NO_GID:-0}" = 1 ]; then printf '1002\n'
      else printf '1002 44444\n'; fi ;;
     *) exit 1 ;;
    esac ;;
   pathfinder-ingest|pathfinder-sor)
    [ "$2" = test ] && [ "$3" = -d ] || exit 1
    [ "$4" = "$PF_C1_JAIL_WORK_PATH" ] || [ "$4" = "$PF_C1_JAIL_RECEIPT_PATH" ] ;;
   *) exit 1 ;;
  esac ;;
 *) exit 1 ;;
esac
MOCK
chmod 0700 "$tmp/mock/mock-command"
for cmd in uname id sysrc jls mount stat jexec; do ln -s mock-command "$tmp/mock/$cmd"; done
export PATH="$tmp/mock:$PATH"
export PF_C1_HOST_WORK="$tmp/hostwork" PF_C1_HOST_RECEIPTS="$tmp/hostack"
export PF_C1_INGEST_WORK="$tmp/ingwork" PF_C1_SOR_WORK="$tmp/sorwork"
export PF_C1_SOR_RECEIPTS="$tmp/sorack" PF_C1_INGEST_RECEIPTS="$tmp/ingack"
export PF_C1_JAIL_WORK_PATH="/var/run/pathfinder-c1/work"
export PF_C1_JAIL_RECEIPT_PATH="/var/run/pathfinder-c1/receipts"
export PF_C1_SHARED_GID=44444 PF_C1_INGEST_USER=ingestuser PF_C1_SOR_USER=soruser

script="$base/tools/pathfinder_c1_c220_preflight.sh"
good=$(sh "$script") || { echo "C1 MOCK PREFLIGHT FAIL valid duplex rejected" >&2; exit 1; }
case "$good" in
 *PATHFINDER_C1_DUPLEX_RAM_PREFLIGHT_PASS*"tcp_listeners=0"*) ;;
 *) echo "C1 MOCK PREFLIGHT FAIL incomplete PASS" >&2; exit 1 ;;
esac
PF_SIM_EMPTY_JAIL_LIST=1 sh "$script" >/dev/null || exit 1
bad(){
 if "$@" sh "$script" >/dev/null 2>&1; then
  echo "C1 MOCK PREFLIGHT FAIL unsafe configuration accepted: $*" >&2
  exit 1
 fi
}
bad env PF_SIM_EXCLUDE_SOR=1
bad env PF_SIM_SOR_WORK_MODE=rw
bad env PF_SIM_ING_ACK_MODE=rw
bad env PF_SIM_NO_GID=1
bad env PF_C1_SHARED_GID=44445
bad env PF_C1_JAIL_WORK_PATH=/var/run/fi-c1-handoff
printf '%s\n' 'PATHFINDER_C1_DUPLEX_MOCK_PREFLIGHT_PASS positive=2 negative=6 host_modified=false'
