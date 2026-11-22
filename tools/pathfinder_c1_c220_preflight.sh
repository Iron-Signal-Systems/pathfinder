#!/bin/sh
# Read-only C220 host audit for TCP-FREE Pathfinder C1 duplex tmpfs.
# No mounts, users, binaries, services, credentials or SQL are changed.
set -eu
umask 077
fail(){ printf 'PATHFINDER_C1_DUPLEX_PREFLIGHT_FAIL %s\n' "$*" >&2; exit 1; }
: "${PF_C1_HOST_WORK:?host work tmpfs path required}"
: "${PF_C1_HOST_RECEIPTS:?host SQL receipt tmpfs path required}"
: "${PF_C1_INGEST_WORK:?host-side ingest RW work nullfs target required}"
: "${PF_C1_SOR_WORK:?host-side SoR RO work nullfs target required}"
: "${PF_C1_SOR_RECEIPTS:?host-side SoR RW receipt nullfs target required}"
: "${PF_C1_INGEST_RECEIPTS:?host-side ingest RO receipt nullfs target required}"
: "${PF_C1_JAIL_WORK_PATH:?inside-jail work mount path required}"
: "${PF_C1_JAIL_RECEIPT_PATH:?inside-jail receipt mount path required}"
: "${PF_C1_SHARED_GID:?dedicated shared group numeric GID required}"
: "${PF_C1_INGEST_USER:?dedicated ingest service username required}"
: "${PF_C1_SOR_USER:?dedicated SoR service username required}"

[ "$(uname -s)" = FreeBSD ] || fail "FreeBSD host required"
[ "$(id -u)" -eq 0 ] || fail "root host audit required"
case "$PF_C1_SHARED_GID" in *[!0-9]*|'') fail "numeric shared GID required";; 0) fail "wheel/root group forbidden";; esac
for p in "$PF_C1_HOST_WORK" "$PF_C1_HOST_RECEIPTS" "$PF_C1_INGEST_WORK" "$PF_C1_SOR_WORK" \
 "$PF_C1_SOR_RECEIPTS" "$PF_C1_INGEST_RECEIPTS"; do
 case "$p" in /*) ;; *) fail "all mount paths must be absolute";; esac
 case "$p" in /|*' '*|*'..'*|*fi-c1-handoff*) fail "unsafe/foreign mount path $p";; esac
 [ -d "$p" ] || fail "mount path missing $p"
 [ ! -L "$p" ] || fail "mount path symlink $p"
done
for p in "$PF_C1_JAIL_WORK_PATH" "$PF_C1_JAIL_RECEIPT_PATH"; do
 case "$p" in /*) ;; *) fail "jail path must be absolute";; esac
 case "$p" in /|*' '*|*'..'*|*fi-c1-handoff*) fail "unsafe jail path $p";; esac
done
[ "$PF_C1_HOST_WORK" != "$PF_C1_HOST_RECEIPTS" ] || fail "work and receipt host tmpfs must be distinct"
[ "$PF_C1_JAIL_WORK_PATH" != "$PF_C1_JAIL_RECEIPT_PATH" ] || fail "separate work and receipt jail paths required"
[ "$(sysrc -n jail_enable)" = YES ] || fail "jail boot policy not enabled"
for jail in pathfinder-ingest pathfinder-sor; do
 jls -j "$jail" jid >/dev/null 2>&1 || fail "jail $jail not running"
 jexec "$jail" test -d "$PF_C1_JAIL_WORK_PATH" || fail "$jail missing work mountpoint"
 jexec "$jail" test -d "$PF_C1_JAIL_RECEIPT_PATH" || fail "$jail missing receipts mountpoint"
done
jail_list=$(sysrc -n jail_list 2>/dev/null || true)
for jail in pathfinder-ingest pathfinder-sor; do
 if [ -n "$jail_list" ]; then
  case " $jail_list " in *" $jail "*) ;; *) fail "$jail not in jail_list";; esac
 fi
done
mount_info=$(mount -p)
lookup(){
 printf '%s\n' "$mount_info" | awk -v target="$1" '
   $2==target {print $1 "|" $3 "|" $4; found++; if(found>1)exit 2}
   END {if(found!=1)exit 1}'
}
check_mount(){
 label=$1; target=$2; source=$3; fstype=$4; option=$5
 found=$(lookup "$target") || fail "$label mount not found exactly once"
 IFS='|' read -r got_source got_type got_opts <<EOF
$found
EOF
 [ "$got_source" = "$source" ] || fail "$label wrong source $got_source"
 [ "$got_type" = "$fstype" ] || fail "$label wrong type $got_type"
 case ",$got_opts," in *,"$option",*) ;; *) fail "$label missing $option";; esac
 case "$label" in
  HOST_*)
   for opt in noexec nosuid; do
    case ",$got_opts," in *,"$opt",*) ;; *) fail "$label missing $opt";; esac
   done
   ;;
 esac
}
check_mount HOST_WORK "$PF_C1_HOST_WORK" tmpfs tmpfs rw
check_mount HOST_RECEIPTS "$PF_C1_HOST_RECEIPTS" tmpfs tmpfs rw
check_mount INGEST_WORK "$PF_C1_INGEST_WORK" "$PF_C1_HOST_WORK" nullfs rw
check_mount SOR_WORK "$PF_C1_SOR_WORK" "$PF_C1_HOST_WORK" nullfs ro
check_mount SOR_RECEIPTS "$PF_C1_SOR_RECEIPTS" "$PF_C1_HOST_RECEIPTS" nullfs rw
check_mount INGEST_RECEIPTS "$PF_C1_INGEST_RECEIPTS" "$PF_C1_HOST_RECEIPTS" nullfs ro

ingest_uid=$(jexec -U "$PF_C1_INGEST_USER" pathfinder-ingest id -u) || fail "ingest service identity absent"
sor_uid=$(jexec -U "$PF_C1_SOR_USER" pathfinder-sor id -u) || fail "SoR service identity absent"
[ "$ingest_uid" -gt 0 ] && [ "$sor_uid" -gt 0 ] && [ "$ingest_uid" != "$sor_uid" ] || fail "separate non-root UIDs required"
for assignment in "pathfinder-ingest:$PF_C1_INGEST_USER" "pathfinder-sor:$PF_C1_SOR_USER"; do
 jail=${assignment%%:*}; user=${assignment#*:}
 ids=$(jexec -U "$user" "$jail" id -G) || fail "cannot read $jail groups"
 case " $ids " in *" $PF_C1_SHARED_GID "*) ;; *) fail "$jail lacks shared GID";; esac
done
for spec in "$PF_C1_HOST_WORK:$ingest_uid" "$PF_C1_HOST_RECEIPTS:$sor_uid"; do
 path=${spec%:*}; expected=${spec##*:}
 [ "$(stat -f '%u' "$path")" = "$expected" ] || fail "wrong tmpfs writer UID $path"
 [ "$(stat -f '%g' "$path")" = "$PF_C1_SHARED_GID" ] || fail "wrong tmpfs shared GID $path"
 [ "$(stat -f '%Lp' "$path")" = "750" ] || fail "tmpfs root must be mode 0750 $path"
done
jexec -U "$PF_C1_INGEST_USER" pathfinder-ingest test -w "$PF_C1_JAIL_WORK_PATH" || fail "ingest cannot write work"
jexec -U "$PF_C1_INGEST_USER" pathfinder-ingest test -r "$PF_C1_JAIL_RECEIPT_PATH" || fail "ingest cannot read receipts"
jexec -U "$PF_C1_SOR_USER" pathfinder-sor test -r "$PF_C1_JAIL_WORK_PATH" || fail "SoR cannot read work"
jexec -U "$PF_C1_SOR_USER" pathfinder-sor test -w "$PF_C1_JAIL_RECEIPT_PATH" || fail "SoR cannot write receipts"
printf 'PATHFINDER_C1_DUPLEX_RAM_PREFLIGHT_PASS work=tmpfs ingest_work=rw sor_work=ro receipts=tmpfs sor_receipts=rw ingest_receipts=ro group=%s tcp_listeners=0 SQL_writes=0\n' "$PF_C1_SHARED_GID"
