# index-1021h: `query` names its snapshot miss; citations refuse an unmerged path

Date: 2026-10-21. Lane: claude/index-1021h (base 8def072e0). Tickets: V1-1640, V1-0328, V1-2152,
V1-1003. Requirements `IDX-SNAP-V0-104` and `DCG-V0-043` are accepted under the owner's standing
auto-accept (2026-10-11). No index, snapshot or analyzer input changes, so the analyzer schema and
the invariant 7 format gate are not engaged.

## V1-1640: the unmet criterion

PR #1427 delivered the notice for `context`, path/range/working-tree `impact` and `feature`
(`IDX-SNAP-V0-102`), the `DOGFOOD.md` step-1 `corvint index --if-stale` (criterion 2) and the
generic-import prefilter with its measurement (criterion 3, `IDX-SNAP-V0-103`). Criterion 1 also
names "other snapshot readers". `query`, the other pre-change read, still built silently.
Measured at 8def072e0 in a plain clone (9,481 sources): `query` took 1.40 s wall without a
snapshot and 0.57 s with one. `context` took 2.19 s and 0.40 s.

Decision (`IDX-SNAP-V0-104`): the CLI `query` adapter arms the same notice through a context value
(`withSnapshotMissNotice`). The repository (`BuildEval`) and authority-start (`BuildQuery`) miss
builds are timed by `timedMissBuild`. The harness event path and `batch` share that build code but
never arm it, so they stay silent. Stdout was byte-identical against the base binary apart from the
scratch-file dirty-path list. On this host the repository-query build ran under the one-second
floor, so the live run printed nothing. The test drives both paths at threshold 0.

## V1-0328: already delivered

Re-measured on 8def072e0, base binary, plain clone: a cold `corvint index` took 2.51 s wall, 5.78 s
user and 1.52 GB RSS, and wrote 180,421,991 bytes. A re-index after a one-file commit took 1.32 s
wall, 2.40 s user and 1.24 GB RSS, and wrote 180,422,001 bytes. The re-index reuses verified facts
(`IDX-SNAP-V0-037`, #1381), so it is not a full rebuild. The per-tree write is a whole snapshot of
about 180 MB, not 63 MB. Shrinking it needs a delta or shared-segment encoding, which is an
invariant 7 format change. That residual is unchanged from `2026-10-21-index-1021-context-snapshot-miss.md`.

## V1-2152: what remained

`--repin` (`DCG-V0-040`) already relocates moved citations in process, and the ambiguous and
shifted cases are tested (cases 24-29). The full gate ran in 0.46 s. Gap: `--hash` on a conflicted
path exited 128 with only git's `cat-file` message. `DCG-V0-043` adds a loader built from
`git ls-files -u`. Any read of a path with unmerged entries and no stage 0 now refuses, naming the
stages. Case 30 failed on the base script and passes now.

## V1-1003: skipped

There are three MCP wire changes. The CLI `context` has no `--budget-bytes` (only
`--summary-bytes`), so "the CLI's semantics" needs a design choice. MCP `tools/list` has no
cross-tool shared definition, and an external `$ref` would break offline validators, so the dedupe
needs an owner decision. Counting the `maxEvidence` truncation touches about 12 sites in the
analyzer-pinned `impact.go`.

## Evidence, NOT_RUN, rollback

Tests: `TestSnapshotMissNoticeNamesTheBuild` (fails on the base for `query`), `line-citations-test`
case 30 (fails on the base). Tier 2 of `corvint affected` is NOT_RUN locally and left for batch CI.
Rollback: revert this commit. `IDX-SNAP-V0-104` and `DCG-V0-043` drop with it.
