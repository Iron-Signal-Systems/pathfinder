# Phase 1.1 — FreeBSD Platform Foundation

Status: **VALIDATED / FROZEN**

This document records the tested FreeBSD platform foundation for Pathfinder Phase 1.1.

It does **not** declare all of Phase 1.1 complete. The existing Phase 1.1 exit gate also requires the application runtime identity, configuration and secret boundaries, PostgreSQL dependency/version direction, startup/readiness behavior, migration entry point, validation/test entry point, and build/start behavior to be reviewable without guessing.

The platform portion documented here is complete and is the baseline for that remaining work.

## Validated Host Baseline

```text
Operating system: FreeBSD 15.1-RELEASE-p3
Architecture:      amd64
Root pool:         zroot
Host interface:    em0
Host IPv4:         installation-specific
Private bridge:    bridge77
Private network:   10.77.0.0/24
Bridge gateway:    10.77.0.1
Application jail:  pfapp / 10.77.0.10
Database jail:     pfdb  / 10.77.0.20
```

The reference build used a classic FreeBSD distribution-set host rather than pkgbase.

## ZFS Layout

```text
zroot/jails
zroot/jails/templates
zroot/jails/containers

zroot/pathfinder
zroot/pathfinder/artifacts
zroot/pathfinder/state
zroot/pathfinder/pgdata
```

The Pathfinder durable datasets currently retain `mountpoint=none` until their consuming runtime paths are finalized.

The jail template is:

```text
zroot/jails/templates/15.1-RELEASE
```

The template was built from the official FreeBSD 15.1 `base.txz`, updated to `15.1-RELEASE-p3`, snapshotted as:

```text
zroot/jails/templates/15.1-RELEASE@base-p3
```

and made read-only.

The application and database jails are copy-on-write ZFS clones of that snapshot:

```text
zroot/jails/containers/pathfinder-app
zroot/jails/containers/pathfinder-db
```

Pathfinder does not delegate ZFS administration to either jail.

## Jail Boundary

Pathfinder uses native FreeBSD jail tooling with VNET.

```text
host
  |
bridge77 10.77.0.1/24
  |-- epair10a <-> epair10b -> pfapp 10.77.0.10
  `-- epair20a <-> epair20b -> pfdb  10.77.0.20
```

The private bridge is not attached directly to the host's external/LAN interface.

The database jail receives private SysV IPC namespaces:

```text
sysvmsg = new;
sysvsem = new;
sysvshm = new;
```

The application jail does not receive those capabilities.

Neither jail is granted ZFS mount authority, host PF authority, or SSH administration as part of this baseline.

Host administration remains host-side, including `jexec` access.

## Boot Ordering

The validated boot configuration is:

```text
jail_enable="YES"
jail_list="pfdb pfapp"
jail_reverse_stop="YES"
jail_parallel_start="NO"
```

Therefore normal ordering is:

```text
boot:      pfdb -> pfapp
shutdown:  pfapp -> pfdb
```

This ordering intentionally anticipates the application depending on PostgreSQL.

## Routing and PF

IPv4 forwarding is enabled on the host.

PF provides NAT for the private Pathfinder network through the host external interface.

Bridge filtering is deliberately configured as:

```text
net.link.bridge.pfil_member=1
net.link.bridge.pfil_bridge=0
net.link.bridge.pfil_local_phys=1
```

Meaning:

```text
pfil_member=1
    inspect traffic switched between bridge members

pfil_local_phys=1
    inspect traffic entering the host IP stack from a bridge member

pfil_bridge=0
    avoid an unnecessary second bridge-level PF pass
```

## Validated Network Contract

The Phase 1.1 reference policy currently establishes:

```text
PFAPP 10.77.0.10
    -> configured DNS server TCP/UDP 53     ALLOW
    -> TCP 443                              ALLOW
    -> PFDB 10.77.0.20 TCP 5432             ALLOW
    -> other traffic                         DENY

PFDB 10.77.0.20
    -> configured DNS server TCP/UDP 53     ALLOW
    -> TCP 443                              ALLOW TEMPORARILY
    -> PFAPP unsolicited                    DENY
    -> other traffic                        DENY

HOST
    -> existing host networking/management preserved
```

The DB HTTPS allowance exists for initial package/bootstrap work and is expected to be revisited after PostgreSQL provisioning.

## Security Tests Performed

The reference build explicitly proved:

```text
pfapp -> pfdb ICMP                    DENY
pfdb  -> pfapp ICMP                   DENY
pfapp -> configured LAN DNS ICMP      DENY
pfapp -> pfdb TCP 5432                ALLOW
pfapp -> pfdb TCP 5433                DENY
pfdb  -> pfapp TCP 8443               DENY
pfapp -> external HTTPS               ALLOW
pfdb  -> external HTTPS               ALLOW TEMPORARILY
```

The TCP/5432 test did not rely only on PF rule inspection. A temporary listener in `pfdb` received the expected test payload on 5432, while listeners on denied paths received no data.

PF counters independently recorded the permitted PostgreSQL flow and denied test flows.

## Reboot Verification

A full host reboot was used as the final platform proof.

After reboot, FreeBSD automatically reconstructed and verified:

```text
15.1-RELEASE-p3
pfdb and pfapp jail startup
bridge77 10.77.0.1/24
epair10a and epair20a membership
PF enabled
NAT restored
Pathfinder filtering rules restored
pfil_member=1
pfil_bridge=0
pfil_local_phys=1
IPv4 forwarding=1
pfapp HTTPS success
pfdb HTTPS success
```

The platform foundation is therefore not dependent on an interactive setup session remaining in memory.

## Frozen Rollback Points

The jail roots were snapshotted after the successful reboot proof:

```text
zroot/jails/containers/pathfinder-app@phase-1.1-foundation
zroot/jails/containers/pathfinder-db@phase-1.1-foundation
```

The FreeBSD host also has the boot environment:

```text
pathfinder-phase-1.1-foundation
```

These are operator rollback checkpoints, not application-level migration mechanisms.

## Reproducibility Requirement

The validated platform must be reproducible from a clean supported FreeBSD host without replaying an exploratory terminal transcript.

The deployment assets under `deploy/` capture the known-good configuration direction and verification contract.

The final bootstrap must be idempotent and fail closed. Re-running it must not silently duplicate datasets, overwrite incompatible existing configuration, or report success when a required validation step failed.

Installation-specific values such as external interface, DNS server, private subnet, and addresses belong in deployment configuration rather than being scattered through implementation logic.

A bootstrap is not considered release-ready merely because it exists in the repository. It must itself be exercised against a clean host and pass the same verification contract recorded here.

## Remaining Phase 1.1 Work

The next work remains inside Phase 1.1 and includes:

```text
PostgreSQL version/package direction
zroot/pathfinder/pgdata mount contract
PostgreSQL initialization and bind boundary
application/database service identities
Pathfinder runtime identity
configuration contract
secret-reference boundary
state/artifact runtime paths
startup/readiness behavior
migration entry point
validation/test entry point
build/start behavior
```

Do not begin broad intelligence schema or feed implementation until these runtime/dependency boundaries are reviewable without guessing.

## Governing Principle

> **A platform is not complete because it booted once. It is complete only when its required boundary can be reconstructed, verified, and truthfully reported.**
