# Pathfinder C1 — FI-derived, TCP-free shared tmpfs handoff (2026-11-22)

**Development architecture, not permission to deploy, enable a service, create a SQL clone, or write to the populated Pathfinder SoR.**

## Proven blueprint versus Pathfinder decision

FI's current dev/c1-two-process-20261120 branch uses one bounded host tmpfs for prepared buffers: fi-ingest RW nullfs and fi-sor RO nullfs, group-only files, immutable publication, and SQL-confirmed reclaim. FI's current *control* connection is still TLS 1.3 over TCP for references and commit ACKs. The C220 FI RAM and read-only-consumer proof passed, but that does **not** prove TCP-free control or Pathfinder.

Pathfinder adopts FI's volatile data-plane, independent source verification, SQL ownership and bounded handoff. Unlike FI's current implementation, **Pathfinder C1 uses filesystem IPC in BOTH directions**. The FreeBSD native ingest and SoR services must not open a TCP/HTTPS listener or use an ingest→SoR TCP client.

## Two distinct host tmpfs instances, four nullfs views

| Host tmpfs | Writer and jail view | Reader and jail view | Content |
| --- | --- | --- | --- |
| work (target 4 GiB, bounded) | pathfinder-ingest RW nullfs | pathfinder-sor RO nullfs | Immutable .ready prepared bundles |
| receipts (small bounded separate tmpfs) | pathfinder-sor RW nullfs | pathfinder-ingest RO nullfs | Immutable .ready.ack receipts |

Every jail has its own distinct unprivileged UID. Both users must be members of a dedicated **Pathfinder-only** non-root GID, which must be independently inventoried and approved. Work tmpfs root is owned by the ingest UID; receipt tmpfs root by the SoR UID. Both group-owned, mode 0750. Immutable work and receipt objects are 0640 in that group. Host mount options include noexec,nosuid. Each jail's RO mount is the final enforcement boundary; do not grant group-wide writes by mounting an entire common directory RW.

Do not reuse FI's /var/run/fi-c1-handoff, its service identities, trust or database, or the UDM-Pro receiver. Original source is kept separately read-only. Work/receipts are never ZFS snapshots, backups, journals or authority for recovery.

## State machine and ownership

1. Ingest independently verifies the **whole** frozen archive, known sensor ID/UUID and manifest. Four continuous family buffers (observations/DNS, conntrack, identity and health) flush only by independent capacity/idle/real-end. A submitted bundle consumes 1 of 16 credits; no source-file-boundary flush.
2. Ingest publishes a complete, checksum-bound JSON bundle as a temporary private file, then atomically links it to a random .ready name on its RW work tmpfs. The SoR never writes to or removes work.
3. SoR discovers .ready on its RO work view, independently reads and hashes the exact bytes, reopens ORIGINAL frozen source and compares all projections. Only SoR holds the PostgreSQL local Unix-socket credentials. The target must be independently verified as **pathfinder_c1_clone**; the populated database must be untouched.
4. SoR performs idempotent SQL operations against the isolated clone. After **every** source batch in the exact immutable bundle has committed, SoR publishes an immutable matching receipt on its RW receipt tmpfs. A socket reply, successful discovery, or appearance of a work file is never a SQL COMMIT ACK.
5. Ingest reads the receipt through its **RO** receipt view, validates original work name, digest, family, complete batch UUID list and exact record count, then reclaims only its original work file and returns the credit. It cannot write or delete receipts.
6. SoR may delete its own orphan receipt only after the corresponding work object has disappeared. Independent verifier B runs in a separate OS process and independently verifies committed SQL versus original custody, never blocking normal commit ACKs.

Loss or corruption of either tmpfs, lack of a verified receipt, SQL uncertainty, incomplete source, or a mismatched GID **must fail closed**. Recovery replays immutable frozen original source and inspects PostgreSQL durable batch state, never volatile file listings. Receipts are transient acknowledgments, not canonical acceptance, sensor acknowledgments, or a replacement for the independent verifier.

## Current code and remaining acceptance

- go/internal/sensorc1/ram_receipts.go implements the two-directory receipt transport, bounded work discovery, independent SQL processing and temporary-file atomic receipt publication.
- go/cmd/pathfinder-c1-ingest-dev and pathfinder-c1-sor-dev use tmpfs filesystem IPC, not TCP. The previous mTLS code remains in isolated development tests but is NOT called by these native entrypoints.
- tools/pathfinder_c1_c220_preflight.sh audits both tmpfs sources, all four nullfs mounts, path ownership, service UIDs and shared GID without making changes.
- Go and isolated PG18 CI checks are distinct from native FreeBSD validation. The package retrieved for source commit 93db76cd0e55 is **superseded for deployment staging** because its native services still use TCP.
- C220 2026-11-22 read-only discovery confirms running pathfinder-ingest (172.20.10.11), pathfinder-sor (172.20.10.12), dev-pathfinder, original frozen 9-file archive at /work/pathfinder/udm-pro-20261120-0500EST, PostgreSQL 18.6 and the original archive RO mount in SoR. **No Pathfinder C1 RAM, account, role or clone setup has been performed.**

**Next physical gate:** choose and approve two unused host mountpoints, four jail paths, two distinct unused service UIDs and a shared non-root GID. Confirm the actual /etc/jail.conf.d mount-lifecycle mechanism before writing persistent configuration. Create/test only deliberate nonpersistent mounts after reviewing all existing jail mappings, especially FI. Run dual-direction probes, source hash and clone isolation proofs **before** starting either native C1 worker. Do not enable boot services or use the populated SoR.

Source blueprint (read from GitHub):
- FI: docs/architecture/FI-C1-SHARED-TMPFS-TWO-JAIL-DEVELOPMENT-20261121.md
- FI: docs/architecture/FI-C1-TMPFS-MOUNT-PLAN-20261121.md
- FI: go/internal/c1handoff/shared_ram.go
- FI: go/internal/c1handoff/session.go (documents FI's current TCP control, not reused)
