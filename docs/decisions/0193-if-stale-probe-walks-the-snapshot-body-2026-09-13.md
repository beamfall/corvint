# Decision 0193 — `index --if-stale` walks the snapshot body; blob shard opens check the entry

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The 2026-09-13 persistence bug hunt confirmed two defects with failing tests.

1. `ProbeSnapshot` decoded only the gob header. A snapshot whose header survived but whose body was
   truncated (a crash after renaming an unsynced temporary file, or outside damage) probed fresh,
   while every loader missed on it under `IDX-SNAP-V0-003`. `index --if-stale` therefore never
   repaired the file, and each read rebuilt the index silently. A one-byte truncation of a written
   snapshot reproduced it in the unit test, and a 4 KiB truncation reproduced it end to end with
   a built `corvint` on a clone of this repository.

   The call: the probe decodes the index message into the compact event shape, the same walk the
   compact `session-start` load already makes. Gob reads the whole length-framed message, so a
   short body fails. The index is not materialized. `IDX-SNAP-V0-011` now says so. On the 64 MiB
   snapshot of this repository at load average about 20, seven paired `--if-stale` runs took
   49-122 ms before and 77-173 ms after, so the probe costs about 25-30 ms more. Syncing the three
   writers' temporary files before rename is deferred to the backlog. The writer cost was not
   measured, and a short body now repairs itself.

2. `openBlobShard` opened the leaf through `os.Root` with `O_NOFOLLOW`, which Go 1.27.0 does not
   honor for a symlink whose target stays inside the root. A name swapped to an in-root symlink
   after `shardRegularPath`'s `Lstat` was therefore read. The call: after the open, the file must
   be `os.SameFile` with a no-follow `Lstat` of its name, or the accelerated attempt refuses and
   falls back (`IDX-SNAP-V0-016`). The `cleanFileOpener` comment no longer claims re-rooting makes
   symlinks unusable. Its callers stay safe because the bytes must hash to the committed blob.

The analyzer schema moves to `corvint-analyzer/45`. No requirement IDs added.
