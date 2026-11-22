#!/bin/sh
# Pathfinder C1 C220 FreeBSD host preflight — READ-ONLY.
# This script never mounts, creates, edits, restarts or starts anything.
# Run from FreeBSD HOST after an independently reviewed host tmpfs/nullfs setup.
# Required paths are explicit: never guess C220 jail roots.
set -eu
umask 077

die() {
    printf 'PATHFINDER_C1_C220_PREFLIGHT_FAIL %s\n' "$*" >&2
    exit 1
}
need_path() {
    name=$1
    path=$2
    case "$path" in
        /*) ;;
        *) die "$name must be an absolute path" ;;
    esac
    [ -d "$path" ] || die "$name missing directory $path"
    [ ! -L "$path" ] || die "$name may not be a symlink"
}

: "${PF_C1_HOST_RAM:?PF_C1_HOST_RAM absolute mounted host tmpfs path required}"
: "${PF_C1_INGEST_RAM:?PF_C1_INGEST_RAM absolute ingest jail root mountpoint required}"
: "${PF_C1_SOR_RAM:?PF_C1_SOR_RAM absolute SoR jail root mountpoint required}"

need_path HOST_RAM "$PF_C1_HOST_RAM"
need_path INGEST_RAM "$PF_C1_INGEST_RAM"
need_path SOR_RAM "$PF_C1_SOR_RAM"

[ "$(uname -s)" = FreeBSD ] || die "this audit only runs on FreeBSD"
[ "$(id -u)" -eq 0 ] || die "host jail mount audit requires root"
command -v mount >/dev/null 2>&1 || die "mount command missing"
command -v jls >/dev/null 2>&1 || die "jls command missing"
command -v sysrc >/dev/null 2>&1 || die "sysrc command missing"

[ "$(sysrc -n jail_enable)" = YES ] || die "jail_enable is not YES"
jail_list=$(sysrc -n jail_list)
case " $jail_list " in
    *" pathfinder-ingest "*) ;;
    *) die "pathfinder-ingest missing from jail_list" ;;
esac
case " $jail_list " in
    *" pathfinder-sor "*) ;;
    *) die "pathfinder-sor missing from jail_list" ;;
esac

jls -j pathfinder-ingest jid >/dev/null 2>&1 || die "pathfinder-ingest jail not running"
jls -j pathfinder-sor jid >/dev/null 2>&1 || die "pathfinder-sor jail not running"

mount_info=$(mount -p)
lookup() {
    printf '%s\n' "$mount_info" | awk -v target="$1" \
        '($2==target) {print $1 "|" $3 "|" $4; found=1; exit}
         END {if (!found) exit 1}'
}
host=$(lookup "$PF_C1_HOST_RAM") || die "host RAM mount absent"
ingest=$(lookup "$PF_C1_INGEST_RAM") || die "ingest shared RAM mount absent"
sor=$(lookup "$PF_C1_SOR_RAM") || die "SoR shared RAM mount absent"
IFS='|' read -r host_source host_type host_opts <<EOF
$host
EOF
IFS='|' read -r ingest_source ingest_type ingest_opts <<EOF
$ingest
EOF
IFS='|' read -r sor_source sor_type sor_opts <<EOF
$sor
EOF

[ "$host_type" = tmpfs ] || die "host source is not tmpfs (got $host_type)"
[ "$ingest_type" = nullfs ] || die "ingest jail RAM is not nullfs"
[ "$sor_type" = nullfs ] || die "SoR jail RAM is not nullfs"
[ "$ingest_source" = "$PF_C1_HOST_RAM" ] ||
    die "ingest nullfs does not point to the approved host tmpfs"
[ "$sor_source" = "$PF_C1_HOST_RAM" ] ||
    die "SoR nullfs does not point to the approved host tmpfs"

has_option() {
    opts=",$1,"
    case "$opts" in
        *,"$2",*) return 0 ;;
        *) return 1 ;;
    esac
}
has_option "$ingest_opts" rw || die "ingest nullfs is not writable"
has_option "$sor_opts" ro || die "SoR nullfs is not read-only"

# Does NOT prove source custody, TLS, service users, database or restart.
printf 'PATHFINDER_C1_C220_RAM_PREFLIGHT_PASS jail_enable=YES ingest=pathfinder-ingest sor=pathfinder-sor host_fstype=tmpfs ingest=nullfs_rw sor=nullfs_ro source=%s database_touched=false files_changed=0\n' "$PF_C1_HOST_RAM"
