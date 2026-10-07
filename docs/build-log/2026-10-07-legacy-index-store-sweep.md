# 2026-10-07: V1-0928 index write sweeps the superseded per-worktree store

## Intent

Ticket V1-0928. Since `DIRTY-CACHE-013` moved snapshots to the shared store under the Git common
directory, a worktree's own `.corvint/index/` is never read or evicted again, but nothing removed
it. On this machine the primary checkout's held 303 MiB in five snapshots and a Codex worktree
63 MiB. Cleanup must sit on an existing write path (`AGENTS.md` invariant 4) and stay bounded.

## Change

- Proposed `IDX-SNAP-V0-027` (pending owner acceptance) in `docs/specs/index-snapshot-v0.md`:
  after an `index` write publishes to the shared store and evicts there, `WriteSnapshot` calls
  `sweepLegacyStore` on the writing worktree's `.corvint/index/`. It opens `.corvint` and the
  store as `os.Root` descriptors only when each matches a no-follow `lstat` (`os.SameFile`), and
  through them removes regular files named as a snapshot, sectioned companion or pack
  (`<format>-<tree>-<engine>.{gob,sect,aip}`), `build-cost.json`, and writer temporaries older
  than the existing ten-minute cutoff; then, only when nothing was left and the listing was
  complete, the `*\n` ignore file and the empty directory.
- Anything else (links, subdirectories, fresh temporaries, unknown names, a linked or non-directory
  store, a failed removal, an unfinished listing past 256 entries) is left and named.
- The receipt adds `legacy_store`, `legacy_removed` (the `evicted_snapshots` shape) and
  `legacy_left` (`{path, reason}`) only when the directory exists, so the core-freeze receipt key
  set is unchanged otherwise. The fallback store (`store_shared:false`) is never swept.
- `analyzerSchemaID` moves to `corvint-analyzer/107` because `TestAnalyzerSchemaInputs` pins the
  contextindex production source digest; existing analyzer packs are rebuilt on the next write.
  This may collide with a concurrent lane that also bumps the schema at integration.
- Stale line citations in `falsifiable-packet-v0.md` and `revision-cache-dirty-worktree-v0.md`
  were repinned; the cited content only moved, so the hashes are unchanged.

## Measured

`TestIndexWriteSweepsTheSupersededWorktreeStore` fixture: a legacy store with a 1 MiB gob, its
`.sect` and `.aip`, `build-cost.json`, a stale temporary and the ignore file, 1,055,295 bytes in
six files. Before (base 5808c26c): an `index` write to the shared store does not open
`.corvint/index` (no code path names it when `store.shared`; inferred from the code, not
instrumented), so all 1,055,295 bytes remain. After: one write removes all six files and the
directory; `legacy_removed` names each with its byte count, which sum to 1,055,295.

## Review

Codex (gpt-6-astra, read-only) found a blocker: the first version `lstat`ed `.corvint/index` and
then reopened it by path, so a link swapped in between could redirect removals outside the store.
Fixed by the descriptor identity check above. The re-review found the final directory removal
could still delete a regular file swapped in after the re-check; it now uses `rmdir`, which removes
only an empty directory. The races have no deterministic test; the static-link cases are tested.

## Limits

- Only a real write sweeps; an `index --if-stale` run that finds the snapshot fresh writes nothing.
- Only the writing worktree's own legacy store is swept; other worktrees wait for their own writes.
- 256 entries per write; a full batch of unrecognised entries stalls further progress (reported).
- An older binary still using the per-worktree store in that worktree loses its snapshots and
  rebuilds them.
- Not run: `make dogfood-change`/CEM binding, the exhaustive gate, a live sweep of the real
  303 MiB store (forbidden in this lane).
