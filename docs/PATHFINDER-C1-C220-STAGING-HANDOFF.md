# Pathfinder C1 — C220 staged test handoff

**Status: Ready to begin C220 READ-ONLY discovery only after successful staging-package CI.**
This is not approval for live ingestion or canonical SoR writes.

**GitHub branch:** dev/c1-ingest-sor-20261122
**Scope:** pathfinder-ingest, pathfinder-sor, independent verifier B, frozen UDM-Pro JSONL replay. Receiver deferred.

## Invariants

- Do not change UDM-Pro, FI, its drive replacement, the receiver, or the populated Pathfinder SoR.
- Historical test source requires exact original 9 files, 685,218 records, registered sensor identity, and an independently pinned whole-archive SHA-256 manifest.
- Test SQL may target ONLY a separately verified disposable PostgreSQL 18 database named pathfinder_c1_clone.
- The shared memory source is host tmpfs: ingest RW nullfs; SoR RO nullfs. The two service users are non-root with distinct UIDs and the same approved non-root RAM group.
- Verifier B is a separately executing read-only process with a SELECT-only PostgreSQL role and does not block SQL commit acknowledgments.
- No auto-created clone, role, jail mount, certificate or rc.conf entry is provided or deployed by this branch.

## Gate 1 — Download and verify exact tested FreeBSD binaries

Once the PUSH workflow on this branch is green for the chosen source commit, download the GitHub Actions artifact named pathfinder-c1-freebsd15-amd64. This artifact is uploaded only after Go/unit/race/vet, FreeBSD cross-build and disposable PostgreSQL 18 tests pass. It contains four FreeBSD/amd64 executables, source BUILD-INFO, SHA256SUMS, read-only inspection scripts and the native mount acceptance runbook.

After transfer to a separate C220 staging directory, without starting anything:

    sh /EXACT/STAGING/PATH/tools/pathfinder_c1_verify_stage.sh /EXACT/STAGING/PATH

Require PATHFINDER_C1_STAGE_VERIFY_PASS and source_commit equal to the exact chosen green source commit. Otherwise STOP.

## Gate 2 — First C220 action: read-only discovery

Run from the FreeBSD host, not inside a jail:

    sh /EXACT/STAGING/PATH/tools/pathfinder_c1_c220_discover.sh

Record: FreeBSD version; running pathfinder-ingest/pathfinder-sor names and jail roots; enabled jail startup policy; actual tmpfs/nullfs mounts; relevant ZFS datasets; availability of psql inside SoR.

No configuration changes, mounts, files, SQL connections or sensor/receiver actions occur. **Stop here** if the output contradicts the approved architecture. Do not guess jail root paths or service accounts.

## Gate 3 — Required operator-verified values

- The actual C220 host tmpfs path and capacity; ingest/SOR mountpoints; jail-visible RAM path.
- Separate ingest and SoR service users with approved shared positive RAM GID.
- Original frozen archive paths visible read-only to ingest, SoR and verifier B.
- Original registered sensor key and UUID.
- Pinned SHA-256 archive manifest and exact per-stream records from the read-only frozen-check command.
- PostgreSQL local Unix socket, clone-only write role, separate SELECT-only verifier role, and actual disposable-clone isolation evidence.
- Private SoR jail IP:port, exact mTLS server/client DNS SAN names, separate private keys and pinned CA.

All three native services require the same original source manifest SHA-256. The SoR validates it independently before opening a SQL connection.

## Gate 4 — Native shared RAM proof

After a separately reviewed mount/group setup, use the exact seven PF_C1_ variables described in PATHFINDER-C1-C220-MOUNT-AND-RAM-ACCEPTANCE.md.

    sh /EXACT/STAGING/PATH/tools/pathfinder_c1_c220_preflight.sh

Then follow that runbook: publish a 64-byte temporary probe from the ingest user; read and verify it as the SoR user through its RO mount; remove only that probe as ingest. No PostgreSQL or receiver writes.

## Gate 5 — Read-only full frozen source check

Using the operator-verified identity, original archive and independently built frozen checker:

    /EXACT/CHECKER/pathfinder-c1-frozen-check -root /ORIGINAL/READONLY/ARCHIVE -sensor-id REGISTERED_ID -sensor-uuid REGISTERED_UUID -expect-files 9 -expect-records 685218

Record the FROZEN_SOURCE_MANIFEST_SHA256, each stored-file hash, per-family counts, source preparation rate, CPU and allocations. Do not invent or reuse the manifest SHA-256. Verify all three native service startup flags include -expect-manifest-sha256 set to the observed value.

## Gate 6 — Isolated native SQL acceptance, NOT automatic

Only after the actual C220 mounts, PG clone, credentials, and mTLS credentials are independently verified:

1. Start the SoR development process, explicitly targeting pathfinder_c1_clone through the local PostgreSQL Unix socket.
2. Run verifier B in parallel, with a separate SELECT-only SQL role.
3. Run ingest, with verified GID, frozen archive SHA-256 and private SoR IP:port.
4. Require exact SQL commits, independently verified typed/child rows, original source totals, and no populated SoR modifications.
5. Test interrupted SQL, lost acknowledgments, disappeared tmpfs, duplicate replay, and final exact source integrity before measuring throughput.
6. Measure wall time, CPU, peak RSS, WAL and I/O; compare against isolated clone baseline.

No service is enabled by boot/startup, and no receiver is used in this test.

## STOP conditions

Checksum mismatch; incorrect branch/commit; missing jail; unexpected mount mode; world-accessible RAM object; missing shared group; original source SHA-256 mismatch; an SQL role with access outside the isolated clone; bad mTLS identity; incomplete SQL commit or independent B mismatch.

The next physical action is read-only discovery on C220. Later configuration must be based on its real output and operator approval, never inferred host paths.
