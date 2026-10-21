# V1-1991: batched symbol-anchor, CEM projection and discovery path reads

Date: 2026-10-20. Ticket: V1-1991. Requirements: DCP-V1-216, DCP-V1-217
(docs/specs/documentation-corpus-v1.md). This applies V1-1984's batching (DCP-V1-155..159)
to the remaining per-path readers:

- `symbol_anchor.go`
- `cem.go projectClaims`
- `behavior_legacy_discovery.go`
- `behavior_playwright_discovery.go`

## Design

`internal/doccorpus/batch_paths.go` `pathPrefetch` sits in front of the per-path reads as a
cache. It makes these batched calls:

- one verified `ResolvePaths` walk per commit;
- one `BlobHeaders` call;
- one `BlobBatch` call per read.

Any path or blob it does not hold is read by the same per-path call as before. That covers a
batch that failed, a path left out by the 128 MiB batch bound, and a symbol read past the
32 MiB prefetch cap. Refusals, their codes and their order are therefore unchanged. Each
phase keeps its own `newGitBudget()` budget and the DCP-V1-157 capacity typing.

Limitation: `BlobHeaders` fails the whole batch when any object is missing. A corpus with one
missing pinned blob then falls back to per-path reads, which are correct but cost one
operation per path.

## Measurement (Git operations per phase, op counts, not durations)

Method: a temporary `DCP_MEASURE=before|after` hook in `boundCorpusGitOps` gave each budget
1<<20 operations and logged how many were spent. In `before` mode it set `pathPrefetchOff`,
which gives the per-path reads. The hook was reverted before commit. The command was:

`heavy.sh env DCP_MEASURE=<mode> GOTOOLCHAIN=local go test -count=1 -timeout 30m -p 4 -v -run TestDCPV1216 ./internal/doccorpus/`

Host: darwin, 12 cores, load averages 30.46/40.37/53.15 at the start and 20.13/29.36/39.48 at
the end (other lanes were running).

| Fixture | before | after |
|---|---:|---:|
| 5,000 moved symbol anchors | 20,000 | 5 |
| CEM projection, 5,000 anchors | 10,000 | 5 |
| Legacy discovery, 4,096 files (record bound) | 8,197 | 10 |
| Playwright discovery, 2,000 more files | 4,009 | 12 |

Every "after" count is within `phaseGitOps` = 16. The default budget is
`gitrun.NewDefaultBudget` = 1,024 operations, so before this change every one of these
fixtures was refused for capacity under the default budget. Each test also compares the
batched output with the per-path output byte for byte, and they match.

## Residual: DCP-V1-217

Legacy and Playwright enumeration still go through the RevisionFS walk, which costs one
operation per directory listed (DLT-V0-003). That is why the discovery fixtures keep their
files in one directory. This cost scales with the number of directories, not the number of
paths. Making it constant needs a batched tree listing and is left for a follow-up.

## Review round (Codex, one round) and fixes

1. **P2: budget exhaustion in the fallback read as an unavailable blob.** One missing pinned blob
   fails the header batch. The per-blob fallback can then exhaust the default budget, and
   `SymbolResolver.file` turned that error into "blob is unavailable", so symbol states were not
   withheld.
   - Fix: `file` now returns `git-budget-exceeded` and `git-output-exceeded` errors, so the checker
     withholds every state with the diagnostic.
   - This also hardens the per-path route, which had swallowed the same error class at a blob read.
   - Test: `TestDCPV1216SymbolFallbackBudgetWithholdsStates`, with 600 moved anchors plus one
     missing blob. It checks that the batched receipt equals the per-path receipt. It fails
     without the fix.
2. **P2: cached `ResolvePaths` admitted paths the per-path lookup refuses.** The walk's aggregate
   bound is 32 MiB. The per-path `LookupTreeEntry` refuses at 4 MiB of root and parent tree
   records.
   - Fix: `gitauth.ResolvePathLookups` reports each path's per-path tree record bytes, and
     `pathPrefetch` answers only paths whose parents all resolved within `MaxTreeBytes`. Every
     other path keeps its per-path lookup and refusal.
   - Test: `TestDCPV1216ProjectionKeepsPerPathTreeBound`, an anchor under a parent tree of about
     4.25 MiB built with `git mktree`. The batched and per-path routes both refuse with
     `git-output-exceeded` and the same message. It fails without the fix.
   - Residual: the V1-1984 `ResolvePaths` consumers (`query.go`, `batch_input.go`) still use the
     aggregate bound alone. This lane did not change them.
3. **P3 (V1-2129, STR-V0-060): the symlink swap passed without `O_NOFOLLOW`.**
   - Fix: added a `symlink-to-same-inode` swap that renames the original and links to it.
   - Check: with `O_NOFOLLOW` removed the test fails, because the digest succeeds. With the flag
     restored it passes.
