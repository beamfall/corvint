# Decision 0210 — Snapshot writers sync before rename; concurrent first reads share one mapping

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

This settles the five unconfirmed contextindex persistence hypotheses of the 2026-09-13 bug hunt.
Decision `0193-if-stale-probe-walks-the-snapshot-body-2026-09-13.md` made a short gob body a miss
and deferred the writer fsync. The evidence below came from scratch tests that were deleted after
the run.

1. **Zeroed range: confirmed defect in the gob loader.** A fixture was written with its header
   intact and then damaged in two ways. With the whole body zeroed, every loader misses: the gob
   decode fails, the sectioned read fails the `identity` section digest, and the pack read fails the
   `identity` block digest table. With only one aligned 4 KiB block zeroed inside a 40 KiB source
   body, the sectioned and pack readers still refuse, because every section or block is hashed.
   The gob full and event decodes, `LoadSnapshot` and `ProbeSnapshot` all accepted that file. They
   returned an index whose `Sources["cache/big.go"].Data` held 4096 zero bytes under the unchanged
   blob hash. That is a wrong index served as a hit, which breaks invariant 2. A crash after
   renaming an unsynced temporary can leave such a range.

   The call is to sync each writer's temporary file before the rename (gob, sectioned, pack).
   Blob shards already sync. Go's `File.Sync` is `F_FULLFSYNC` on darwin. A load-time integrity
   check was the alternative. It would need a new gob format with a digest trailer, and every read
   would pay for it, compact `session-start` included. Measured on this repository's 63,847,442-byte
   gob snapshot, five rounds each, with the load average at 122 (1 min), 112 (5 min) and 68
   (15 min):
   - `Sync` alone took 13.0-20.0 ms, median 13.5 ms.
   - Encode, sync and close took a median of 96 ms, against 138 ms for encode and close without
     sync. At this load the difference is inside the noise.
   - The load-time alternative: SHA-256 over the file took a median of 44 ms (32-77 ms), and the
     full gob decode it would add to took a median of 67 ms.

   The write path runs once per tree. The read path runs on every verb, so the cost goes on the
   write. The sync does not protect against outside damage to a published gob file, which remains
   undetected. The sectioned and pack formats detect it.

2. **SIGBUS on in-place truncation: accepted residual.** The readers map the file `MAP_PRIVATE`
   (`snapshot_sectioned_mmap_unix.go`). Another process that truncates a mapped `.sect` or `.aip`
   in place faults the next touch of a page past the new end. Corvint publishes only by rename, so
   the inode a reader mapped is never truncated by Corvint. There is no cheap guard.
   `debug.SetPanicOnFault` is per goroutine, and the ranking reads the aliased bytes on many
   goroutines. Checking the size before each touch still races. Both formats are opt-in and
   experimental.

3. **The `.aip` removal in `evictSnapshotsAt`: refuted as dead, kept.** `git grep packPath` shows
   every current caller passes `analyzerEngine()` (`WriteSnapshot`, `readSnapshotIndex`,
   `probeAnalyzerPack`), so no current writer produces a gob-stem `.aip`. Binaries before
   `0074-wave-1-analyzer-engine-key-2026-09-05.md` (commit `b66e7df1`) keyed packs by the executable
   digest. The line removes those legacy packs when their gob companion is evicted, even when
   the pack opt-in is off and `evictAnalyzerPacks` never runs. It stays, with a comment. There is
   no removal, so no test was added.

4. **Two first reads map one pack twice: confirmed defect, fixed.** `readPackSnapshot` and
   `readSectionedSnapshot` checked the cache, then decoded, then retained. A racer that lost the
   retention returned an index that aliased its own mapping, and nothing ever unmapped it. The new
   tests start 16 racers on an empty cache. They failed on the old code: pack in 3 of 3 runs,
   sectioned in 7 of 10. The call: retention returns the winning file. A loser drops the history
   registration and the mapping of the index it never handed out, then decodes from the retained
   mapping. The extra decode happens only on the race. With the fix both tests passed
   `-count=10`.

5. **u32 pack source lengths: refuted.** Admission excludes any tree entry larger than
   `maxSourceBytes` = 1,000,000 (`internal/contextindex/git.go:31@6d4d1a9b`, `internal/contextindex/index.go:501@9640ef12`).
   A pinned body is the batch blob whose size must equal the entry size (`internal/contextindex/git.go:560@d68f134a`), or a
   worktree copy that hashes to the same blob. No body can reach 2^32 bytes, and body offsets are
   u64.

Amends `IDX-SNAP-V0-001` (sync before rename), `IDX-SNAP-V0-014` and `IDX-SNAP-V0-015` (retention
under concurrent first reads), and the failure modes. No requirement IDs added. Analyzer schema
`corvint-analyzer/50` (renumbered at merge).
