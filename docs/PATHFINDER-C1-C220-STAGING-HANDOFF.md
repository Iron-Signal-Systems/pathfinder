# Pathfinder C1 — C220 staging handoff, TCP-free native development

**Status: isolated development, C220 read-only discovery completed; native mount, clone and 685,218-record replay not accepted.**
**Branch: dev/c1-ingest-sor-20261122.**
Receiver, UDM-Pro, FI, populated Pathfinder SoR and boot services remain untouched.

## Transport correction

Read [TCP-free tmpfs design](PATHFINDER-C1-TCP-FREE-TMPFS-DESIGN-20261122.md). Native C1 uses **two separate host tmpfs instances and four jail nullfs views**, not a TCP or mTLS control listener. Ingest: work RW, receipts RO. SoR: work RO, receipts RW. The original FI reference design has tmpfs payloads but mTLS/TCP control, which is deliberately *not* reused by Pathfinder. The existing 93db76cd0e55 stage is for source discovery only; **do not deploy its TCP native binaries.**

## Gate 1 — exact GitHub CI artifact, staged via dev-pathfinder

Use the existing authenticated checkout /home/jwood/pathfinder in dev-pathfinder. Verify that the active branch is dev/c1-ingest-sor-20261122 and the tracked checkout is clean. Fetch and fast-forward, then retrieve a **new exact-commit CI artifact**:

    cd /home/jwood/pathfinder
    git fetch origin dev/c1-ingest-sor-20261122
    git pull --ff-only origin dev/c1-ingest-sor-20261122
    gh auth status
    sh tools/pathfinder_c1_fetch_stage_from_gh.sh "$HOME/pathfinder-c1-stage-$(git rev-parse --short=12 HEAD)"

The helper refuses missing CI, wrong commit, overwriting an existing stage, and bad file hashes. Require both stage-verifier PASS lines and final DEV_FETCH_PASS. Reconcile the actual dev jail root on the host using jls and reverify via staged pathfinder_c1_verify_stage.sh before any further test.

## Gate 2 — host discovery (already performed, repeat if topology changes)

    sh /ACTUAL/HOST-VISIBLE/STAGE/tools/pathfinder_c1_c220_discover.sh

Observed 2026-11-22: FreeBSD 15.1-p6, running Pathfinder ingest and SoR jail roots under /fast/jails/roots/, original 9-file frozen UDM-Pro archive at /work/pathfinder/udm-pro-20261120-0500EST, RO SoR source mount, PostgreSQL 18.6. No Pathfinder C1 RAM mounts or service users existed. These are discovery facts, not configuration approval.

## Gate 3 — deliberate, separately authorized two-way RAM mount preparation

Follow [mount and RAM acceptance](PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md) and choose two isolated, bounded tmpfs mountpoints, four nullfs paths, distinct unprivileged service identities and a Pathfinder-only shared non-root GID. Invent no UID/GID or jail.conf linkages. Do not change existing FI mountpoints or Pathfinder populated PostgreSQL storage. The approved mount setup must include BOTH directions and all original-source read-only views.

Run the **read-only** host preflight with the 11 explicitly verified PF_C1_ inputs and require:

    PATHFINDER_C1_DUPLEX_RAM_PREFLIGHT_PASS

It verifies mount sources, read/write modes, ownership and group membership, but does not perform actual probes or establish recovery.

## Gate 4 — native two-direction permission and data proofs

Stage the matching RAM probe from the new exact CI artifact inside both jails without altering existing services. Run ingest publish of a disposable 64-byte work object, read as SoR, and clean up from ingest. Then independently demonstrate that SoR can publish a disposable receipt on its dedicated writable tmpfs and ingest can read it through its RO mount, while neither reader can write to its read-only mount. The full real receipt proof also requires a confirmed isolated SQL commit. No DB writes at the probe gate.

STOP if any mount allows the wrong writer, has world access, or points to persistent storage.

## Gate 5 — read-only frozen original and SQL clone preflight

Use actual registered original sensor ID/UUID and read-only original archive with the exact staged frozen checker:

    /ACTUAL/STAGE/bin/pathfinder-c1-frozen-check -root /ORIGINAL/READONLY/ARCHIVE -sensor-id /ACTUAL/SENSOR/ID -sensor-uuid /ACTUAL/UUID -expect-files 9 -expect-records 685218

Record per-file and whole-archive SHA-256, exact four stream counts, memory and timings. The original source must be independently readable within ingest, SoR and verifier B, and its manifest pinned in all three native process invocations.

Only after a separate isolation review may a **disposable** PostgreSQL 18 database named pathfinder_c1_clone be provisioned and a clone-only write role and separate SELECT-only verifier role verified. The SoR SQL connection must use the local Unix socket; no PostgreSQL access in ingest.

## Gate 6 — native two-jail C1 acceptance, not automatic

After work/receipts mounts, source checks and clone roles are verified, explicitly start SoR filesystem consumer against the clone, independent verifier B alongside it, then frozen replay ingest using the matching tmpfs paths. Observe that no native C1 service listens on TCP. Require exact original counts (685,218), child/typed readback, matching commit receipts, duplicate replay safety, lost receipt and tmpfs-loss recovery, independent B, and resource telemetry. No canonical promotion or live sensor acknowledgment.

**STOP** on source mismatch, unsafe mount, role crossing database boundary, incomplete SQL, forged/replayed receipt, failed recovery, or unknown path. Nothing in this branch automatically deploys to C220 or enables rc startup.
