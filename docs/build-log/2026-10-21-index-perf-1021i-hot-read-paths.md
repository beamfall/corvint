# 2026-10-21 index-perf-1021i: hot read paths (V1-2237, V1-2221, V1-2239)

Lane base `8def072e0`. Apple host shared with other lanes; the load average is given with each
measurement because it inflates every wall time here.

## Problem

- V1-2237: the hook path's gob snapshot decode (about 180 MB) cost 270 to 390 ms.
- V1-2221: `corvint impact --base HEAD~300` took 11 to 13 s.
- V1-2239: a cached `go build ./cmd/corvint` was reported at 10.9 s, and `corvint index` at 4.6 to 4.9 s.

## Profiles and dominant costs

- **Snapshot decode** (scratch harness over `readSnapshotIndex`, event load, pprof). The costs split as:
  - syscall read 22%
  - memmove 20%, from `growslice` under gob's `saferio.ReadData`, which grows each message buffer in 10 MB steps
  - SHA-256 17%, serial after the decode
  - GC and madvise for the rest

  `GOGC` changes had no measurable effect.
- **Range impact**: 0.5 s of CPU in 10.6 s of wall time. A PATH `git` wrapper showed `diff-tree --find-renames --find-copies-harder` taking 11.04 s over 6,921 changed paths. That range adds `café.txt` fixture paths, so the exact parse always refuses with "path unsafe for verification planning". ERI-V0-060's rename-free probe bailed on those same paths, so copy detection ran just to produce a refusal.
- **Index build** (`BuildForSnapshot`, `CORVINT_INDEX_REUSE=0`, 4 iterations, 28.5 s CPU across 10.3 s wall):
  - `readCleanFileUsing` 22%, almost all of it `os.Root` openat walking each path component
  - GC (madvise, memclr, mark) about 30%
  - tokenising 13%
  - `goSymbols` 8.5%
  - document records 7%

  Per iteration, build took 2.08 to 2.12 s and `WriteSnapshot` 0.27 to 0.38 s, at load 11.

## Decisions

- **IDX-SNAP-V0-105.** Decode the unchanged on-disk format through a read-only mapping, and hash the trailer concurrently on a second goroutine. Memory faults while reading the mapping become a miss. Because the format does not change, invariant 7's format gate does not apply; the analyzer inputs changed, so `script/analyzer-schema.sh bump` moved the schema to `corvint-analyzer/138` (IDX-SNAP-V0-060). The rest of the cost, gob buffering plus string copies, is the floor of this format; removing it means changing the format (pack or sectioned), and I rejected that here.
- **ERI-V0-262.** When the rename-free diff proves which path refusal the exact diff must return, return it before copy detection. The proof requires:
  - all failing paths, plus all base-tree copy candidates when there are additions, fail identically
  - the repository is not a partial clone
  - the status output stays within the size bound

  The path refusal is checked before the ERI-V0-060 bound, because the exact parse refuses before it counts.
- **V1-2239.** I recorded the phase breakdown and the floor; there is no code change.
  - The `os.Root` open walk is the containment boundary for worktree pins, so I did not trade it away in this lane.
  - Build timings from a fresh clone at load 19 to 23:
    - first build 8.27 s (user 28.7 s)
    - unchanged cached rebuild 2.37 s
    - an edit to `cmd/corvint` only: 2.86 s (compile 0.98 s, link 1.35 s)
    - an edit to `internal/contextindex`: 2.17 s
  - The 10.9 s figure did not reproduce; it fits a cold dependent rebuild under load.
  - Cold `corvint index` took 3.28 s at load 23, and 1.58 s with a prior snapshot.

## Evidence (before / after, same binary source, outputs compared)

| Path | Load | Before | After |
|---|---|---|---|
| decode only (event) | 16-21 | 188-217 ms | 111-150 ms |
| `LoadEventSnapshot` incl. git | 24 | 232-261 ms | 175-230 ms |
| `impact --base HEAD~300` | 11-14 | 12.3-14.3 s | 0.40-0.49 s |

The `impact` stdout and stderr bytes and the exit code were identical before and after.

## Verification

- New tests:
  - `TestRangeImpactRefusesUnsafePathBeforeCopyDetection`
  - `TestRangeCertainPathRefusalDefersMixedRefusals`
  - `TestMappedSnapshotDecodeMatchesStreamedDecode`
  - `TestMappedSnapshotDecodeFaultIsAMiss`

  The first and last fail with the fix disabled: the range test reports one exact-diff call, and the fault test dies with a fatal fault.
- On macOS a truncated MAP_PRIVATE file keeps its pages readable, so the fault test uses a PROT_NONE mapping. Linux raises SIGBUS on truncation instead.
- `corvint affected` tier 0 passed: `internal/contextindex` in full, 250 s. Tier 1, capped at 14 packages, also passed: diagnostic, specindex, sharedpath, lrfrepo, liveverify/affected, fixture-maintenance-check, corvint-go-test-provider, semescalate, obscorpus, protocol/integrations, capabilities-check, cem-citation-baseline, spec-coverage-audit and tasks/ticket.
- Doc gates passed, as did `make windows-vet`, `script/analyzer-schema.sh check`, `gofmt` and `go vet`.

## NOT_RUN

- `make gate` (charter).
- `./cmd/corvint` package tests (it is unchanged).
- The other 132 affected units: 84 dependency-path importers, 21 binary-exec, 14 unbounded-reader and the rest of the path-literal shell tests.

## Residuals

- gob decode floor: format-level work.
- `os.Root` per-component openat in worktree pins: 22% of index-build CPU.
- Index-build GC share.

## Rollback

Revert the commit:

- IDX-SNAP-V0-105: delete the `mapReadOnly` branch and `snapshot_decode.go`.
- ERI-V0-262: make `rangeCertainRefusal` call `rangeCertainlyExceeds` alone.
