# Pathfinder Deployment Assets

These files capture the validated Phase 1.1 FreeBSD platform foundation.

They are intentionally split into configuration, known-good reference files, and verification rather than hiding the platform boundary inside one opaque installer.

## Current Contents

```text
deploy/
    README.md
    pathfinder.conf.example
    files/
        jail.conf
        pathfinder-jails.conf
        pf.conf
    verify/
        pathfinder-verify.sh
```

`pathfinder.conf.example` contains installation-specific values for the validated reference build. It must not contain credentials, private keys, database passwords, API tokens, or other secrets.

`files/` contains the known-good FreeBSD jail/PF configuration used to prove the platform boundary.

`verify/pathfinder-verify.sh` performs read-oriented host/jail/PF checks and HTTPS egress validation. It reports a non-zero exit status when a required check fails.

## Bootstrap Direction

The final goal remains:

```sh
sh pathfinder-bootstrap.sh /path/to/pathfinder.conf
```

The bootstrap is not committed as release-ready merely by translating the interactive build transcript into shell commands.

It must be:

```text
idempotent
fail-closed
safe around pre-existing configuration
explicit about installation-specific values
able to distinguish already-correct state from conflicting state
able to verify each required boundary
clean-host tested
```

The bootstrap should eventually perform the validated sequence:

```text
validate supported FreeBSD host
validate/update base system
validate ZFS root pool
create Pathfinder ZFS hierarchy
retrieve and verify official base.txz
build and patch jail template
freeze template snapshot
create application/database clones
write jail-local rc.conf
install jail configuration
configure bridge/VNET/epairs
configure private DB SysV IPC namespaces
configure IPv4 forwarding
configure bridge PF inspection
validate and load PF policy
configure jail startup ordering
start jails
run verification
```

Do not silently overwrite an incompatible existing jail, dataset, bridge, PF policy, or host configuration.

## Reference vs. General Installer

The files currently record the tested reference values, including:

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

A general bootstrap must render installation-specific configuration from `pathfinder.conf` rather than assuming every deployment uses those values.

See [`../docs/PHASE-1.1-PLATFORM-FOUNDATION.md`](../docs/PHASE-1.1-PLATFORM-FOUNDATION.md) for the validated architecture, security tests, reboot proof, and frozen rollback points.
