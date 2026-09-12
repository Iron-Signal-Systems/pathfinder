# Phase 1.1 — Runtime and Repository Foundation

Status: **VALIDATED / COMPLETE**

This document records the completed Pathfinder Phase 1.1 FreeBSD runtime, repository, database, service, migration-authority, and reboot-persistence foundation.

Phase 1.1 is closed. Phase 1.2 may build the first minimal relational schema on this boundary.

## Validated Reference Baseline

```text
Operating system: FreeBSD 15.1-RELEASE-p3
Architecture:      amd64
Root pool:         zroot
Host interface:    installation-specific (reference: em0)
Private bridge:    bridge77
Private network:   10.77.0.0/24
Bridge gateway:    10.77.0.1
Application jail:  pfapp / 10.77.0.10
Database jail:     pfdb  / 10.77.0.20
PostgreSQL:        18.6
Pathfinder:        0.0.0-dev reference runtime
```

The reference build uses the classic FreeBSD distribution-set model rather than pkgbase.

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

Validated runtime mountpoints are:

```text
zroot/pathfinder/artifacts
    /usr/local/jails/containers/pathfinder-app/var/db/pathfinder/artifacts

zroot/pathfinder/state
    /usr/local/jails/containers/pathfinder-app/var/db/pathfinder/state

zroot/pathfinder/pgdata
    /usr/local/jails/containers/pathfinder-db/var/db/postgres/data18
```

The PostgreSQL dataset uses the validated properties:

```text
compression=lz4
recordsize=32K
atime=off
sync=standard
primarycache=all
logbias=latency
```

State and artifact datasets use the normal Pathfinder ZFS hierarchy and remain separately manageable from the jail root.

## Jail Template and Clones

The template is:

```text
zroot/jails/templates/15.1-RELEASE
```

It was built from the official FreeBSD 15.1 `base.txz`, updated to `15.1-RELEASE-p3`, frozen at:

```text
zroot/jails/templates/15.1-RELEASE@base-p3
```

and made read-only.

The application and database jails are copy-on-write clones:

```text
zroot/jails/containers/pathfinder-app
zroot/jails/containers/pathfinder-db
```

Neither jail receives ZFS administration or host PF authority.

The database jail receives private SysV IPC namespaces:

```text
sysvmsg = new;
sysvsem = new;
sysvshm = new;
```

The application jail does not receive those capabilities.

## Boot Ordering

The validated jail startup contract is:

```text
jail_enable="YES"
jail_list="pfdb pfapp"
jail_reverse_stop="YES"
jail_parallel_start="NO"
```

Therefore:

```text
boot:      pfdb -> pfapp
shutdown:  pfapp -> pfdb
```

This makes the database dependency explicit rather than relying on race timing.

## Routing and PF

IPv4 forwarding is enabled on the host.

Bridge packet filtering is:

```text
net.link.bridge.pfil_member=1
net.link.bridge.pfil_bridge=0
net.link.bridge.pfil_local_phys=1
```

PF owns NAT and the jail-originated network boundary.

The final Phase 1.1 steady-state policy is:

```text
PFAPP 10.77.0.10
    -> configured DNS TCP/UDP 53           ALLOW
    -> PFDB 10.77.0.20 TCP 5432           ALLOW
    -> private ranges through broad HTTPS  DENY
    -> public TCP 443                       ALLOW
    -> all other traffic                    DENY

PFDB 10.77.0.20
    -> configured DNS TCP/UDP 53           ALLOW
    -> public HTTPS                         DENY
    -> PFAPP unsolicited                    DENY
    -> all other traffic                    DENY

HOST
    -> existing host operation/management preserved
```

The temporary database-jail HTTPS provisioning allowance used earlier in construction is **not** part of the final runtime boundary.

Validated negative tests included application-to-database TCP/443 denial and database-to-application TCP/443 denial using real temporary listeners so execution failures were not mistaken for network denials.

## PostgreSQL Runtime

The database jail runs PostgreSQL 18.6.

Validated configuration includes:

```text
PGDATA=/var/db/postgres/data18
listen_addresses=10.77.0.20
port=5432
password_encryption=scram-sha-256
data_checksums=on
postgresql_enable=YES
```

Host authentication permits the application and migration identities only from the application jail address and rejects the remaining host-address space after the explicit entries.

The runtime listener is restricted to:

```text
10.77.0.20:5432
```

## Database Authority Separation

Phase 1.1 freezes three distinct database authorities:

```text
pathfinder_owner
    NOLOGIN
    owns the pathfinder database/schema
    no superuser
    no createdb
    no createrole
    no replication
    no bypassrls

pathfinder_app
    LOGIN
    normal Pathfinder runtime identity
    CONNECT / runtime access only
    no DDL authority

pathfinder_migrator
    LOGIN
    separate migration identity
    NOINHERIT
    member of pathfinder_owner with explicit SET authority
    cannot perform owner DDL until SET ROLE pathfinder_owner
```

The following were explicitly tested:

```text
pathfinder_app DDL                         DENY
pathfinder_migrator DDL before SET ROLE   DENY
pathfinder_migrator SET ROLE owner        ALLOW
owner-role DDL inside rollback probe      ALLOW
probe rollback leaves no table            PASS
```

Normal `serve` startup does not silently migrate schema.

## Application Runtime Identity and Filesystem

The application jail contains the dedicated service identity:

```text
pathfinder:pathfinder
uid=1001
gid=1001
```

Validated runtime paths are:

```text
/usr/local/sbin/pathfinder
/usr/local/etc/pathfinder/pathfinder.conf
/usr/local/etc/pathfinder/secrets/
/var/db/pathfinder/state
/var/db/pathfinder/artifacts
/var/log/pathfinder
```

Ownership and access boundaries are:

```text
/usr/local/etc/pathfinder
    root:pathfinder 0750

/usr/local/etc/pathfinder/secrets
    root:pathfinder 0750

pathfinder.conf
    root:pathfinder 0640

runtime PostgreSQL password
    root:pathfinder 0640
    readable by pathfinder service

migration PostgreSQL password
    root:wheel 0600
    NOT readable by pathfinder service

state/artifact/log paths
    pathfinder:pathfinder 0750
    writable by pathfinder service
```

Secrets are referenced from configuration; they are not stored in the non-secret configuration file.

## Pathfinder Runtime Contract

The installed Phase 1.1 binary exposes:

```text
pathfinder version
pathfinder validate -config <file>
pathfinder serve -config <file>
pathfinder migrate -config <file>
```

The configuration parser is strict:

```text
unknown key      -> fail
missing key      -> fail
empty value      -> fail
duplicate key    -> fail
invalid port     -> fail
non-loopback readiness address -> fail
```

`validate` is usable under the normal `pathfinder` service identity.

`migrate` requires root invocation and uses the separate migration credential. The service identity cannot invoke migration authority.

Phase 1.1 migration behavior is an authority/preflight contract with zero schema migrations pending; real versioned schema migration begins in Phase 1.2.

## Health and Readiness

The runtime exposes loopback-only health endpoints:

```text
127.0.0.1:8080/livez
127.0.0.1:8080/readyz
```

Validated behavior:

```text
/livez   -> live
/readyz  -> ready when the PostgreSQL endpoint is reachable
```

The readiness listener is not exposed on the jail network address.

## rc.d Service Contract

Pathfinder is supervised by FreeBSD `daemon(8)` through:

```text
/usr/local/etc/rc.d/pathfinder
```

The service runs the child as the non-root `pathfinder` identity.

Validated runtime files are:

```text
/var/run/pathfinder/pathfinder-supervisor.pid
/var/run/pathfinder/pathfinder.pid
/var/log/pathfinder/pathfinder.log
```

The service contract validates configuration, credential readability, write boundaries, and executable presence before startup.

Validated lifecycle behavior includes:

```text
start                   PASS
non-root child          PASS
/livez                  PASS
/readyz                 PASS
SIGTERM delivery        PASS
graceful shutdown       PASS
child exit              PASS
listener cleanup        PASS
PID cleanup             PASS
```

## Build Boundary

The reference runtime was built with Go 1.25.14 during Phase 1.1 construction.

The validated installed binary SHA-256 was:

```text
dd00b06d935dc8b8c0a53399473f1776f7cbd93039ac91849ddafd7942e85be8
```

After installation and lifecycle validation, the temporary Go toolchain was removed from the application jail.

The installed static Pathfinder binary continued to pass runtime validation and migration preflight afterward.

Therefore the final runtime appliance does not depend on a Go compiler being installed.

## Full Reboot Persistence Proof

A full host reboot was used as the Phase 1.1 closing test.

Before reboot:

```text
pathfinder_enable=YES
postgresql_enable=YES
Pathfinder live=PASS
Pathfinder ready=PASS
```

The reboot delivered SIGTERM to the running Pathfinder service and the application logged a graceful shutdown before host termination.

After reboot, the system proved:

```text
FreeBSD 15.1-RELEASE-p3
pfdb automatically restored
pfapp automatically restored
PostgreSQL automatically started
PostgreSQL listener restored on 10.77.0.20:5432
Pathfinder automatically started
Pathfinder child owned by pathfinder
/livez = live
/readyz = ready
127.0.0.1:8080 listener restored
migration secret still denied to pathfinder service
Go toolchain still absent
installed Pathfinder SHA-256 unchanged
PF enabled
IPv4 forwarding=1
pfil_member=1
pfil_bridge=0
pfil_local_phys=1
```

This is the final Phase 1.1 completion proof.

## Rollback / Reference Checkpoints

Relevant reference checkpoints include:

```text
pathfinder-phase-1.1-foundation
pathfinder-phase-1.1-pre-final-reboot
pathfinder-phase-1.1-runtime-ready
```

Durable datasets also have Phase 1.1 runtime-ready snapshots for state, artifacts, and PostgreSQL data, plus a database-jail runtime-ready snapshot.

The application jail is intentionally not snapshotted at the final runtime-ready point because it contains live runtime and migration credentials.

Rollback checkpoints are operational safeguards, not application-level migration mechanisms.

## Reproducibility and Phase 1 Installation Requirement

The Phase 1.1 appliance was constructed and validated interactively, but Phase 1 itself may not close on that basis.

Phase 1 requires a repository-owned fresh-install path defined by:

[`PHASE-1-FRESH-INSTALL-ACCEPTANCE.md`](PHASE-1-FRESH-INSTALL-ACCEPTANCE.md)

The required clean-host starting state is intentionally small:

```text
supported FreeBSD
ZFS root
working network/DNS
root account
one non-root wheel user
```

The stable operator entry point is:

```sh
sh install.sh --config /path/to/pathfinder.conf
```

Until every required construction stage is automated and clean-host tested, `install.sh` must fail closed rather than perform a partial installation and report success.

## Phase 1.1 Exit Result

```text
supported runtime environment          PASS
jail/network/PF boundary               PASS
PostgreSQL runtime                     PASS
runtime database authority             PASS
separate migration authority           PASS
configuration/secret boundary          PASS
state/artifact filesystem boundary     PASS
strict validation entry point          PASS
migration entry point                  PASS
rc.d service lifecycle                 PASS
health/readiness                       PASS
graceful shutdown                      PASS
boot persistence                       PASS
runtime toolchain cleanup              PASS
full host reboot proof                 PASS
```

Phase 1.1 result:

```text
PASS — COMPLETE
```

## Governing Principle

> **A platform is not complete because it booted once. It is complete only when its required boundary can be reconstructed, verified, and truthfully reported.**
