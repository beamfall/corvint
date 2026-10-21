# 2026-10-21 speed-1021: workflow test wall, parallel gate index, corvint test double scan

Lane `speed-1021` on base `c9ca8beff43d8f548b054d3b3d40e3054cecf816`, under the owner's
2026-10-21 rule to speed up slow Corvint processes. Every number below was taken on the same
12-CPU host, which ran other lanes throughout; the 1-minute load average is recorded with each
measurement. AFP-V0-316 is accepted under the owner's standing auto-accept (2026-10-11).

## V1-2195: internal/cem/workflow tests

Problem. The package ran 82 top-level tests one after another (one, in `self_modified_test.go`,
already ran in parallel). `go test -json` showed the time spread across fixture-heavy tests,
not concentrated in a few. The slowest were `TestDiscriminateRecordsWitnessAndReportDowngrades`
(14.5 s, nested mutation runs), `TestStatusReportsSelfModifiedAuthority` (11.4 s),
`TestMutatorsRefuseStableInput` (10.1 s, ten `makeRepo` builds), `TestStableRefusalPrecedesLock`
(10.1 s) and `TestCEMV1141StablePrepareWritesResumesAndReplaces` (9.2 s). The cause was serial
execution plus repeated git setup: `makeRepo` ran seven git subprocesses on each of its 58 calls.
User plus system CPU was 95 s of the 253 s wall, so the run spent most of its time waiting on
subprocess latency.

Decisions.
- Every top-level test that does not touch shared process state now calls `t.Parallel()` (76
  tests). Six stay serial, because Go runs them before any paused parallel test resumes: the two
  that replace the package's `lockMap` (`TestMutationsLockTheirEffectiveOutput`,
  `TestMarkReplacementAtFirstRead`), the one that clears `OpenHunkJudge`
  (`TestDiscriminateWithoutRunnerIsNotRun`), the two that set `TMPDIR` (`TestCEMV1250*`), and
  `TestSelfModifiedAuthorityListsAnUncitedGoverningChange`. Every git call already had its own
  home, identity and clock, and every repository is under its test's own `t.TempDir()`.
- `makeRepo` builds its base/target repository once per package under a directory that
  `TestMain` removes, then copies it into each caller's `t.TempDir()`. The fixed identity and
  clock make the copied base and target IDs equal to those of a fresh build. `TestMain` captures
  `os.TempDir()` before any test runs, so a `TMPDIR` test cannot relocate the template.
- No assertion, subtest or test was removed or changed.

Evidence (package `Elapsed` from `go test -p 4 -count=1 -json ./internal/cem/workflow/`):

| run | code | load at start, end | package wall |
|---|---|---|---|
| 1 | base | 37, 120 | 253.5 s |
| 2 | parallel only | 78, 114 | 53.8 s |
| 3 | parallel only, `-parallel 1` (serial equivalent) | 117, 63 | 252.4 s |
| 4 | parallel only | 117, 194 | 123.9 s |
| 5 | parallel and shared template | 187, 181 | 65.7 s |
| 6 | base (separate worktree), back to back with 7 | 173, 28 | 240.7 s |
| 7 | parallel and shared template | 28, 47 | 32.7 s |

Each serial/parallel pair at a comparable load is at least 51% faster (rows 3 and 4 at load
63 to 194); the delivered code is 74% to 86% faster (rows 1 and 5, rows 6 and 7). All 82 tests
passed in every run.

## V1-2169: parallel gateselect scan

Problem. `indexRepository` read, parsed and lexed every Go source inside the `filepath.WalkDir`
callback, one at a time.

Decision (AFP-V0-316). The walk now only collects the sources. A pool of at most eight workers
(never more than `GOMAXPROCS`) reads, parses, lexes and tokenizes each one into its own slot.
The results are then recorded in walk order, so the index matches the serial pass exactly. A
source error precedes any walk error found after it, which keeps the first error identical.
`TestAFPV0316ParallelIndexMatchesSerial` compares the pooled index with a `GOMAXPROCS=1` serial
index. `TestAFPV0316ParallelIndexReportsTheFirstErrorInWalkOrder` pins the first error. With the
results recorded in reverse order, a mutation run failed the error test. Both tests pass on the
serial code too: they guard against regressions and do not reproduce a defect.

Evidence. A scratch benchmark (not committed) indexed this repository's checkout, five runs of
ten iterations each, at load 31 to 64. Base: 1.55, 1.56, 2.72, 1.77 and 1.79 s per index, median
1.77 s. Delivered: 1.04, 0.65, 0.67, 0.59 and 0.50 s, median 0.65 s (-63%).

## V1-1880: corvint test scans the repository twice

Measured decision: the two scans are not shared in this change. The affected Go graph
(`internal/liveverify/affected/golang`) already observes directories with an eight-worker pool.
The gate index lexes differently in four ways:
- it uses comment scanning, to see `//go:embed`;
- it lexes the whole file and skips import declarations by state, where the graph lexes from the
  end of the imports;
- it maps tokens against the root module, where the graph uses each file's owning module;
- it skips nested modules.

Sharing one scan would mean proving that the two token sets are equivalent across every
AFP-V0-012 rule, which is larger than a speed fix. V1-2169 instead removes most of the second
scan's wall time. `corvint test --dry-run --base c9ca8beff…` over this lane's two-package
working change (NARROWED, 179 units) ran five interleaved times per binary at load 50 to 100.
Base: 5.40, 5.05, 4.94, 4.33 and 5.21 s, median 5.05 s, user 3.1 to 3.3 s. Delivered: 4.20,
5.23, 3.62, 3.90 and 2.80 s, median 3.90 s (-23%), user 3.1 to 3.2 s. Stdout, stderr and the
`--manifest` document were byte-identical between the two binaries.

## V1-0246 and V1-0081: skipped

Measured at base: a newline-only edit to `internal/behaviorfalsify/receipt_test.go` makes
`corvint test --dry-run` select 144 of 357 root-module packages (40%). Bounding `cmd/corvint`
would need a declared read scope that only the Linux Landlock wrapper can qualify. The tickets
record that the confined hosted run needs admin ci-control-plane approval, and whether 40% is
"well under half" is the owner's call. No change was made.

## NOT_RUN and rollback

`go test -race` over `internal/gateselect` and `internal/cem/workflow` passed (load 58 to 118).
NOT_RUN: `make gate` (lane policy) and the hosted CI run.
Rollback: revert the commit. The workflow tests return to serial and per-call fixtures, and
`indexRepository` returns to calling `scanFile` inside the walk.
