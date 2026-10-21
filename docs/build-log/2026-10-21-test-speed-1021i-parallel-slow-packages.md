# Test speed 1021i: authority scan revalidation, contextindex and doccorpus parallelism

Date: 2026-10-21. Lane: test-speed-1021i. Base: origin/main 8def072e0.

Test-only change: no product code, wire contract or requirement changes, so no new requirement IDs.
Times are `go test -count=1` through the host semaphore (`heavy.sh`); load is the 1-minute load
average at start and end. Each is a single paired measurement, not a benchmark.

## Decisions

- **V1-2268 (internal/tasks/authority).** The recovery test reader `scan` called the full
  `fixtureSession.check` (descent from `/`, retained-parent Lstats, mount and lock identity) in each
  per-file read, twice per file. Scan now reads files with `ftScanRead`, which keeps the
  regular-file, bound, retained-parent identity and digest confirmations. It brackets the reads with one full
  check at the start (the HEAD read) and one at the end. The product mechanism is unchanged.
  New `TestFixtureCaptureRefusesParentSwappedToSymlink` swaps each retained parent (primary, state,
  intent) for a symlink and requires both capture and scan to refuse. With the closing check removed
  and HEAD read through `ftScanRead`, all three subtests fail. PR #1447 (V1-0348) touches other hunks
  of the same files, so the two changes do not conflict.
- **V1-2238 (internal/contextindex).** 698 top-level tests ran sequentially (sum 318.9 s). An AST
  and pattern audit marked 151 tests of at least 0.3 s as parallel-safe. These tests assign no package
  variable or bound (`lowerBound`, `.limit =`) and use no Setenv, Chdir, allocation, MemStats,
  goroutine-count or wall-clock assertions, directly or through test helpers. They now call `t.Parallel`. Go
  resumes parallel tests only after every sequential top-level test returns, so the hook-mutating
  tests never overlap them.
- **V1-2166 / V1-1001 (internal/doccorpus).** The package had no parallel tests. The same audit
  marked 164 of 188 tests parallel-safe, and they now call `t.Parallel`. The 24 that set
  `newGitBudget`, `pathPrefetchOff`, frontier or key bounds, env, or timing stay sequential.

## Evidence

| Package / set | Before | After |
| --- | --- | --- |
| authority, `ReturnedFaults` test | 50.8 s, load 23-26; check 60.7 % of CPU | 23.1 s, load ~11; check 26.3 % |
| authority, package | 225.0 s, load 11-16 | 151.8 s, load 16-11 (122.8 s at load 9-10); check 22.6 % of package CPU |
| contextindex, package | 311.8 s, load 10.7-7.3 | 157.5 s, load 22.8-44.8 (after merging origin/main); 300.9 s in an earlier run at load 17.5-37.3 |
| doccorpus, 164 parallel-safe tests | 408.9 s, load 29.3-10.3 | 114.1 s, load 21.2-31.0 |

In the earlier contextindex run the sequential tests took 278.6 s, against
164.2 s before, while the 151 parallel tests finished in about 22 s of wall time (154.6 s serial
before). 

The doccorpus profile names the cost driver. The DCP-V1-216 bulk equivalence tests run the per-path,
unbounded Git reads for 5,000 anchors or 4,096 files to compare bytes with the batched reads:
`TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount` 217.6 s,
`...CEMProjection...` 111.0 s and `...LegacyDiscovery...` 103.3 s, at load ~20. They mutate
the package hooks and stay sequential, so they now dominate the package (a full run hit an 8 m
bound inside the sequential set). The whole package was not measured after the change in one run.

## NOT_RUN

- Full doccorpus package in one run; quiet-host timing (V1-1001 AC1).
- Tier 2 of `corvint affected` (UNBOUNDED_READER and PATH_LITERAL_READER units outside the
  changed packages, including cmd/corvint): left to batch CI.
- V1-2220 (verify envelope, gitauth, verify+workflow+gitnotes): not started.

## Residuals

- The DCP-V1-216 equivalence tests need a per-call (context-carried) budget and prefetch hook
  before they can run in parallel.
- `TestContextRecencyReportsCodeOwnersBlameDisagreement` (TCP-V0-037) fails when `GIT_AUTHOR_*` is
  exported in the test environment; contextindex has no TestMain that scrubs it.

## Rollback

Revert the three test commits. No product, data or wire state changes.
