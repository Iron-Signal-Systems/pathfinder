# Pathfinder Deployment Assets

These files capture the validated Phase 1.1 FreeBSD runtime/platform foundation and support the repository-owned Phase 1 fresh-install path.

The stable operator entry point is the repository-root script:

```sh
sh install.sh --config /path/to/pathfinder.conf
```

Phase 1 is still under development. `install.sh` intentionally refuses a partial installation until every required Phase 1 construction stage is automated and clean-host validated. It already provides clean-host preflight and installed-system verification routing.

## Current Contents

```text
install.sh

deploy/
    README.md
    pathfinder.conf.example
    files/
        host-rc.conf.pathfinder
        host-sysctl.conf.pathfinder
        jail.conf
        pathfinder-jails.conf
        pathfinder.rc.d
        pathfinder-runtime.conf.example
        pf.conf
        pfapp-rc.conf.pathfinder
        pfdb-rc.conf.pathfinder
        pg_hba.conf.pathfinder
        postgresql.conf.pathfinder
    verify/
        pathfinder-verify.sh
```

`pathfinder.conf.example` contains installation-specific non-secret values for the validated reference build.

`files/` contains known-good reference configuration for the FreeBSD host, jail, PF, PostgreSQL, and Pathfinder service/runtime boundaries. Files with reference IP addresses or interface names must be rendered from deployment configuration by the general installer rather than copied blindly.

`verify/pathfinder-verify.sh` validates the Phase 1.1 host, jail, PF, PostgreSQL, Pathfinder service, health/readiness, migration-secret isolation, migration preflight, and runtime-toolchain boundary. It exits non-zero if a required invariant fails.

The validated Go runtime source is stored under:

```text
go/cmd/pathfinder/
```

## Clean-Host Starting Contract

The Phase 1 install acceptance test starts with only:

```text
supported FreeBSD amd64 release
ZFS root pool
working host networking and DNS
root account
one non-root user in wheel
```

The installer must not require pre-existing Git, Go, PostgreSQL, Pathfinder identities, jail templates, Pathfinder jails, Pathfinder datasets, PF policy, Pathfinder secrets, or Pathfinder services.

FreeBSD base-system tools may be used to acquire and construct the system. Package/runtime dependencies required by Pathfinder are installer-owned.

## Installer Contract

The completed Phase 1 installer must be:

```text
idempotent where safe
fail-closed
non-destructive by default
explicit about conflicts
safe around pre-existing configuration
able to distinguish already-correct state from conflicting state
able to verify every required boundary
clean-host tested
```

It must not silently overwrite an incompatible existing jail, dataset, bridge, PF policy, host network boundary, PostgreSQL installation, or secret.

The installer must own the applicable construction sequence:

```text
validate supported FreeBSD host
validate ZFS root pool and administrative starting state
create Pathfinder ZFS hierarchy
retrieve and verify official FreeBSD base.txz
build and patch the jail template
freeze the template snapshot
create application/database clones
write jail-local rc.conf
install jail configuration
configure bridge/VNET/epairs
configure private DB SysV IPC namespaces
configure IPv4 forwarding
configure bridge PF inspection
render, validate, and load PF policy
configure jail startup ordering
install and initialize PostgreSQL
configure PostgreSQL bind/HBA boundary
create database roles and authority separation
generate runtime and migration secrets locally
create Pathfinder service identity
mount runtime state/artifact/pgdata datasets
install Pathfinder configuration and rc.d service
build/install Pathfinder or install a validated release artifact
run explicit migrations
enable/start services
run repository-owned verification
prove graceful shutdown and reboot persistence
remove temporary build dependencies from the runtime appliance
```

## Current Phase 1.1 Reference Boundary

The final validated steady-state network direction is:

```text
PFAPP
    DNS to configured resolver              ALLOW
    PostgreSQL to PFDB:5432                 ALLOW
    public HTTPS                            ALLOW
    broad HTTPS to private ranges           DENY
    other traffic                           DENY

PFDB
    DNS to configured resolver              ALLOW
    public HTTPS                            DENY
    unsolicited traffic to PFAPP            DENY
    other traffic                           DENY
```

The earlier temporary PFDB HTTPS bootstrap allowance is no longer part of the reference runtime policy.

## Reference vs. General Installer

The current reference values include:

```text
em0
192.168.1.53
10.77.0.0/24
10.77.0.1
10.77.0.10
10.77.0.20
bridge77
epair10
epair20
```

Those are reference values, not universal product constants. A general installation must render installation-specific configuration from the supplied deployment config.

Secrets never belong in `pathfinder.conf`.

## Verification

Preflight only:

```sh
sh install.sh --preflight --config /path/to/pathfinder.conf
```

Verify an installed Phase 1.1 system:

```sh
sh install.sh --verify --config /path/to/pathfinder.conf
```

The Phase 1.1 closing architecture and test record is:

[`../docs/PHASE-1.1-PLATFORM-FOUNDATION.md`](../docs/PHASE-1.1-PLATFORM-FOUNDATION.md)

The Phase 1 clean-host exit contract is:

[`../docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md`](../docs/PHASE-1-FRESH-INSTALL-ACCEPTANCE.md)
