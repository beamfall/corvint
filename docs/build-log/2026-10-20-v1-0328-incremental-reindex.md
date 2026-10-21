# 2026-10-20 V1-0328: re-index reuses verified facts from the prior snapshot (IDX-SNAP-V0-037)

Ticket V1-0328: "re-index is a full rebuild that writes about 63 MB of snapshot per tree
(unverified)". Acceptance: measured snapshot bytes and time per re-index retained; an improvement
lands only with a benchmark. `2026-10-16-v1-0328-reindex-snapshot-measurement.md` confirmed the
full rebuild (a one-file change cost as much CPU as a cold build). This change makes the default
re-index incremental in compute, not in bytes.

## Decision

`BuildForSnapshot` (blob shards off) now loads the newest prior snapshot of the same object format
and engine from the shared store, verifies it, and reuses the per-source facts of every source whose
blob id is unchanged and whose prior bytes rehash to that id. Facts that depend only on a source's
path and bytes are reused: symbols, markers, imports and the approximate-import flag, unparsed rows,
extraction notes and the per-source term, word and path-term postings. Everything that reads the
whole source set is recomputed: document records, symbol windows, the identifier graph. The
vocabulary is a two-pointer merge of the prior postings, renumbered to the new sorted source order
with removed sources dropped, and freshly tokenised postings for the changed sources.

Fail-closed boundaries are in `IDX-SNAP-V0-037` (a) to (c): the prior is read only through
`decodeSnapshot`; per-source regrouping refuses any inconsistency and then reuses nothing; a
source is reused only after its bytes rehash to the new tree's blob id (invariant 1). The served
index is identical under `IDX-SNAP-V0-022`'s canonical encoding to a cold build.

One ordering change was needed for that identity: the symbol sort was path, line, name, so ties
kept their input order, and reuse changes input order. The sort now also compares kind and end
line. The cold path is unaffected wherever no two symbols shared path, line and name; where they
did, the order is now total.

Not done: snapshot bytes per tree are unchanged (187,024,595 bytes on the subject). Sharing
unchanged content across trees on disk needs a format change, which invariant 7 puts behind the
benchmark/format gate (`deployment-neutral-index-platform-v0.md`). V1-0328 stays partial on bytes.

Rollback: `CORVINT_INDEX_REUSE=0` restores the cold build per process; reverting
`internal/contextindex/snapshot_reuse.go` and its two call sites removes the path. Snapshot files
and their names are unchanged in both directions.

## Measurement

- Subject: a local clone of this repository at `a148e3a92` (11,733 tracked files, 11,187 sources).
- Binaries: `corvint-base` built from `a148e3a92` and `corvint-new` built from this change, both
  `GOTOOLCHAIN=local go build -buildvcs=false ./cmd/corvint` (`go1.27.2`).
- Host: Apple M2 Max, 12 cores, darwin/arm64, shared with other lanes. Load1 30 to 45 throughout;
  CPU seconds are the main evidence and wall time is indicative.
- Command, per run: `/usr/bin/time -l BIN --root SUBJECT index` through the shared heavy-run
  semaphore. Before the timed runs, the store was cleared and the prior commit indexed. The target
  tree's own snapshot was removed before each run, except in the no-change rows. Three runs each;
  the medians are below.

| Scenario | Real s | CPU s | Max RSS MB | Bytes written |
|---|---|---|---|---|
| base, prior `HEAD~1` | 3.17 | 9.16 | 1463 | 187,024,595 |
| new, `CORVINT_INDEX_REUSE=0`, prior `HEAD~1` | 3.26 | 9.25 | 1368 | 187,024,595 |
| new, reuse, prior `HEAD~1` (24 sources not reused) | 1.67 | 3.46 | 1245 | 187,024,595 |
| new, reuse, prior `HEAD~30` (1,435 files changed) | 2.09 | 4.34 | 1299 | 187,024,595 |
| new, `CORVINT_INDEX_REUSE=0`, no change | 3.33 | 9.26 | 1368 | 187,024,595 |
| new, reuse, no change | 1.67 | 3.38 | 1324 | 187,024,595 |

The remaining reuse cost is mostly the prior decode, the rehash of every reused blob, the
whole-index products and the gob encode/write. It is not per-source parsing.

## Parity

- `CORVINT_LIFECYCLE_CORPUS` set to the subject clone, moving `HEAD~1` to `HEAD`: canonical index
  236,134,574 bytes, sha256 `c26e2fae...dfdc`, identical cold and reused; 11,163 of 11,187 sources
  reused.
- A scratch run moving `HEAD~30` to `HEAD` (not maintained): identical sha256; 9,765 of 11,187
  sources reused.
- Mutation check: a deliberately broken symbol regrouping made both maintained parity tests fail.

Tests: `TestSnapshotReuseMatchesColdBuild`, `TestSnapshotReuseRefusesUnverifiedPrior`,
`TestMergeReusedPostingsEqualsFullInversion`, and `TestColdAndIncrementalSnapshotsAreByteIdentical`
(the cold side now uses `CORVINT_INDEX_REUSE=0`, and the test asserts the reuse counts).

The symbol-order change alters the facts in an opt-in analyzer pack, so `analyzerSchemaID` moves to
`corvint-analyzer/132`. The `IDX-SNAP-V0-017` audit pin is updated to match. `origin/main` already
uses 131, so a merge must recompute the pin.
