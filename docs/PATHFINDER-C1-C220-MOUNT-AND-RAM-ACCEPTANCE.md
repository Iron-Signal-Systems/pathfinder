# Pathfinder C1 — C220 native duplex RAM mount acceptance

**Development-only, not authorization to modify live C220 services, populated SQL, receiver or FI.** New native IPC uses **two independent host tmpfs stores**, four nullfs views, and NO ingest↔SoR TCP. See [TCP-free design](PATHFINDER-C1-TCP-FREE-TMPFS-DESIGN-20261122.md).

## Required host mount model

| Host backing source | pathfinder-ingest jail view | pathfinder-sor jail view |
| --- | --- | --- |
| dedicated bounded work tmpfs, noexec,nosuid | **RW nullfs** | **RO nullfs** |
| dedicated bounded commit-receipt tmpfs, noexec,nosuid | **RO nullfs** | **RW nullfs** |

Two different **non-root UIDs** own the corresponding writable tmpfs roots; both are members of the same dedicated non-root Pathfinder GID. Each tmpfs root is 0750 and objects are 0640. A wrong-direction writable nullfs is a STOP condition. Never recycle FI's existing /var/run/fi-c1-handoff or identity. Source UDM-Pro JSONL custody is independent, read-only and not copied into either RAM store. Work and receipts deliberately vanish on reboot.

## Read-only host preflight input contract

After explicitly reviewing host jail roots, jail.conf.d mount configuration, unused UIDs/GIDs, capacity, and creating the **two mounts and four mappings under separate operator authorization**, set the following variables to *real* paths and account names. Do not use assumed numeric IDs or generic placeholders:

- PF_C1_HOST_WORK: mounted work tmpfs on FreeBSD host
- PF_C1_HOST_RECEIPTS: separately mounted receipt tmpfs on host
- PF_C1_INGEST_WORK: host-side path of ingest RW nullfs mount
- PF_C1_SOR_WORK: host-side path of SoR RO nullfs mount
- PF_C1_SOR_RECEIPTS: host-side path of SoR RW nullfs mount
- PF_C1_INGEST_RECEIPTS: host-side path of ingest RO nullfs mount
- PF_C1_JAIL_WORK_PATH: work mount path inside both jails
- PF_C1_JAIL_RECEIPT_PATH: receipt mount path inside both jails
- PF_C1_SHARED_GID: verified dedicated nonzero GID shared by both jails
- PF_C1_INGEST_USER: verified non-root ingest service identity
- PF_C1_SOR_USER: verified distinct non-root SoR service identity

Run **from C220 host root**:

    sh /ACTUAL/VERIFIED/STAGE/tools/pathfinder_c1_c220_preflight.sh

Expected: PATHFINDER_C1_DUPLEX_RAM_PREFLIGHT_PASS. Preflight only reads mounts, users and access flags. It refuses missing mount, wrong underlying host tmpfs, incorrect RO/RW, missing noexec/nosuid, missing group, wrong ownership or cross-project FI paths. Do not mount or change accounts in response to a failure without inspecting the exact reported condition.

## Actual cross-jail handoff proof

With matching CI-built development probe binaries staged into both runtime jails, run a one-file test as the **ingest service identity**:

    pathfinder-c1-ram-probe-dev -operation publish -ram-dir "$PF_C1_JAIL_WORK_PATH" -shared-gid "$PF_C1_SHARED_GID"

Pass the exact emitted name and SHA-256 to the **separate SoR service identity**, using its RO work view:

    pathfinder-c1-ram-probe-dev -operation read -ram-dir "$PF_C1_JAIL_WORK_PATH" -shared-gid "$PF_C1_SHARED_GID" -name NAME -sha256 SHA256

Require PATHFINDER_C1_RAM_PROBE_READ_PASS and demonstrate the SoR cannot create/delete files on the read-only work mount. Clean up the exact disposable probe **only from ingest** with -operation remove, preserving all non-probe files.

A complementary SoR→ingest receipt proof is required on the second pair of nullfs views: SoR is the sole receipt publisher, ingest is the read-only receiver. Actual native service SQL commit receipts must match the exact work file hash, family, batch UUID list and row count. Never treat the appearance of an ACK file as receipt authenticity without OS writer isolation and full field checks.

## Further gates

Before native C1 replay, establish independent read-only original source access to ingest, SoR and verifier B; actual frozen archive manifest and 685,218 records; approved isolated PostgreSQL 18 pathfinder_c1_clone, clone-only SQL role and SELECT-only verifier role. Run SQL only from SoR on its local Unix socket. Verify no C1 TCP listener. Reconcile reboot/startup and loss of either tmpfs; replay source from immutable original and reconcile SQL completion markers, not from RAM. The original populated SoR and FI remain unchanged. Independent verifier B runs asynchronously.

**Status: C220 discovery completed. Duplex mounts, account/group setup, dual-direction native proof, clone isolation and full replay are NOT yet accepted.**
