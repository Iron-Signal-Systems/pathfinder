# Phase 1 Fresh-Install Acceptance

Status: **REQUIRED FOR PHASE 1 EXIT**

Pathfinder Phase 1 is not complete until the Phase 1 system can be reconstructed on a clean supported FreeBSD host from repository-owned deployment assets without replaying an exploratory terminal transcript or depending on undocumented administrator knowledge.

## Required Starting State

The clean-host acceptance test begins with only:

```text
supported FreeBSD release
amd64 architecture
ZFS root pool
working host networking and DNS
root account
one non-root administrative user in wheel
```

The target host must not require any pre-existing Pathfinder-specific state.

The acceptance test must not assume that the following are already installed or configured:

```text
git
Go
PostgreSQL
Pathfinder binaries
Pathfinder users or groups
Pathfinder datasets
Pathfinder jail templates
Pathfinder jails
Pathfinder PF rules
Pathfinder secrets
Pathfinder rc.d services
```

If a package or runtime dependency is required, the Pathfinder installer owns installing and validating it.

## Installation Entry Point

The stable installation entry point is the repository-root script:

```sh
sh install.sh --config /path/to/pathfinder.conf
```

The implementation may use supporting files under `deploy/`, but the operator must not need to manually execute the individual construction steps.

A reboot or continuation step may be required when the host boundary requires it, but continuation must remain installer-managed and documented. The operator must not need to reconstruct hidden state from a previous interactive shell session.

The installer must not require Git to be present on the clean host. FreeBSD base-system mechanisms such as `fetch(1)`, `tar(1)`, `sh(1)`, ZFS, boot environments, jail tooling, PF, `sysrc(8)`, and `service(8)` may be relied upon when present in the supported base release.

## Installation-Specific Configuration

Deployment-specific values belong in a non-secret configuration file, not scattered through shell implementation.

Expected installation-specific inputs include at least:

```text
supported FreeBSD release / patch policy
root ZFS pool
external interface
DNS server
private Pathfinder subnet
private bridge address
application jail address
PostgreSQL jail address
jail/template dataset names where configurable
```

The repository reference is `deploy/pathfinder.conf.example`.

Credentials, private keys, database passwords, API tokens, and other secrets must not be stored in that file.

## Installer-Owned Secret Generation

The installer must generate required local secrets on the target host using suitable cryptographic randomness.

At minimum, the Phase 1 install path must preserve the validated separation between:

```text
pathfinder_app
    normal runtime database identity
    no DDL authority

pathfinder_migrator
    separate migration login
    no automatic owner inheritance
    explicit SET ROLE pathfinder_owner for migration DDL

pathfinder_owner
    NOLOGIN database/schema owner
```

The normal `pathfinder` service identity must not be able to read the migration credential.

Secrets must never be written to Git, emitted in normal installer output, or copied into repository reference files.

## Required Installer Behavior

The Phase 1 installer must be:

```text
idempotent where safe
fail-closed
explicit about conflicts
safe around pre-existing host configuration
non-destructive by default
reviewable
repeatable
automatically verifiable
```

It must distinguish at least:

```text
missing state that can be created
already-correct state
compatible state that can be updated safely
conflicting state requiring operator action
failed state
```

It must not silently overwrite incompatible jails, datasets, PF policy, host networking, database state, or secrets.

## Required Construction Scope

By Phase 1 exit, the installer must own the complete construction path required by the Phase 1 architecture, including the applicable portions of:

```text
host preflight
supported FreeBSD validation
ZFS hierarchy
FreeBSD jail template acquisition and checksum verification
base template patching
application/database jail clones
jail rc.conf and service configuration
bridge / VNET / epair configuration
IPv4 forwarding and bridge pfil configuration
PF policy validation and activation
jail boot ordering
PostgreSQL installation and initialization
PostgreSQL bind / HBA boundary
PostgreSQL role creation
runtime and migration credential generation
Pathfinder service identity
Pathfinder runtime directories and datasets
Pathfinder configuration
Pathfinder binary build/install or release artifact installation
rc.d service installation
migration execution
service enable/start
health/readiness validation
network-boundary validation
reboot persistence validation
```

Temporary build dependencies such as the Go toolchain must not remain installed on the final runtime appliance unless a later explicit architecture decision requires them.

## Phase 1.1 Baseline the Installer Must Reproduce

The installer must be capable of reproducing the validated Phase 1.1 runtime foundation, including:

```text
FreeBSD 15.1-RELEASE-p3 reference baseline
native VNET jails: pfdb then pfapp
bridge77 / 10.77.0.0/24 reference topology
host-owned PF / ZFS / jail administration
PostgreSQL 18 runtime in pfdb
PostgreSQL listener restricted to 10.77.0.20:5432
pathfinder_app runtime role without DDL
pathfinder_migrator explicit migration authority
pathfinder_owner NOLOGIN ownership role
Pathfinder non-root service identity
root-owned configuration with service read only
runtime DB secret readable by Pathfinder service
migration DB secret unreadable by Pathfinder service
state/artifact ZFS datasets mounted into pfapp
loopback-only /livez and /readyz endpoints
rc.d startup and graceful SIGTERM shutdown
Pathfinder and PostgreSQL boot persistence
Go toolchain absent from the final runtime jail
```

Installation-specific IP addresses and interface names are reference values, not universal product constants.

## Verification Contract

An install is not successful merely because every construction command returned zero.

The installer must run repository-owned validation after construction and must fail if a required invariant cannot be established.

The final clean-host acceptance test must include a full host reboot and prove at least:

```text
host networking restored
PF restored
jails restored in required order
PostgreSQL automatically started
Pathfinder automatically started
Pathfinder child runs as the pathfinder identity
/livez returns live
/readyz returns ready
Pathfinder listener remains loopback-only
runtime DB identity can authenticate
runtime DB identity cannot perform DDL
Pathfinder service cannot read migration credential
migration preflight succeeds only under migration authority
installed Pathfinder artifact remains unchanged across reboot
Go build toolchain is absent from the final runtime jail
```

The clean-host test result must be retained in the repository or release validation record so Phase 1 completion is independently reviewable.

## Phase 1 Exit Rule

Phase 1 may not be declared complete until all of the following are true:

```text
one narrow intelligence vertical slice passes
Phase 1 semantic/history invariants pass
install.sh reconstructs the Phase 1 system from the defined clean-host starting state
repository-owned verification passes
full reboot persistence passes
no undocumented manual construction step is required
```

A manually working development appliance is not sufficient for Phase 1 completion.

> **If Pathfinder cannot be rebuilt from the repository and a clean supported host, the implementation is not yet a complete product boundary.**
