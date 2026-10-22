# testspeed-1021v: parallel DCP-V1-216 equivalence tests, and measured test-time tickets

Date: 2026-10-21. Tickets: V1-2286, V1-1001 (delivered); V1-2211, V1-1148 (measured, not cut);
V1-2291 (skipped). Requirement: DCP-V1-216 traceability row only. No requirement was added or
changed, because production behaviour is unchanged. The traceability edit falls under the owner's
standing auto-accept (2026-10-11).

## Problem

The `internal/doccorpus` package took 650 s on this host and about 35 min under heavy load. The
DCP-V1-216 bulk equivalence tests dominate it. These tests changed two package globals,
`newGitBudget` and `pathPrefetchOff`, and `writeBulkFixture` called `t.Setenv`. As a result
none of the heavy tests could use `t.Parallel`.

## Cost drivers (V1-1001)

Baseline is `go test -json` of the whole package at f11ac58d3, run through `heavy.sh`.

Elapsed time per test:

| Test | Elapsed |
|---|---|
| `TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount` | 213.8 s |
| `TestDCPV1216CEMProjectionGitOpsIndependentOfAnchorCount` | 116.6 s |
| `TestDCPV1216LegacyDiscoveryGitOpsIndependentOfPathCount` | 88.0 s |
| `TestBehaviorLegacyParity` | 82.8 s |
| `TestBehaviorAcceptanceAmendment` | 77.5 s |
| `TestDCPV1216PlaywrightDiscoveryGitOpsIndependentOfPathCount` | 43.0 s |

The top-level test times sum to 1,862 s.

The CEM projection and legacy tests were timed by phase:

- Building the fixture takes about 7 s.
- The batched run under test takes 1.0 to 1.2 s.
- The per-path oracle run takes 93 to 99 s. That is about 20 ms of `git` subprocess per path.

So the cost driver is the per-path oracle, not the batched code under test. The fixture sizes
were kept: 5,000 / 5,000 / 4,096 / 2,000. They are the coverage that the DCP-V1-216 row states,
and 4,096 is the legacy record bound.

## Decision (V1-2286)

- Budget and prefetch hooks now travel on the call's context through an unexported key
  (`corpusGitHooks` in `capacity.go`). Production contexts never carry them, so production code
  keeps `gitrun.NewDefaultBudget()` and batched reads.
- `newGitBudget(ctx)` and `pathPrefetchOff(ctx)` replace the globals.
- `symbolAnchorCache.open` takes the context.
- `writeBulkFixture` passes the fixed commit dates per command instead of through `t.Setenv`.
- The 11 DCP-V1-155..159 and DCP-V1-216 budget and equivalence tests now run under `t.Parallel`.
- New test `TestDCPV1216GitHooksTravelWithTheContext` shows that a hook-carrying child context
  changes only its own calls. It failed with the per-path override forced off, with the message
  "per-path projection of 40 anchors within 16 operations = <nil>".

## Evidence

- Whole-package wall time on the same host, through `heavy.sh`, `-p 4 -count=1`:
  - Before: 650.5 s (real 674.8 s), load 8.3 rising to 33.7.
  - After: 414.5 s (real 431.2 s), load 29.6 falling to 23.8, 1-min peak 33.
  - All tests passed in both runs. The package finishes well inside the 30 min timeout.
  - Under contention each parallel test runs longer (Symbol 398 s, CEM 268 s, Legacy 257 s), but
    they overlap.
- `-race -run 'TestDCPV121|TestDCPV1243|TestDCPV115'`: ok in 352.0 s (real 397.6 s), load 17 to 34.

## V1-2211 (tools/gate-ledger): measured, not cut

- Whole package: 23.1 s at load about 22.
- The slowest tests start real `go test` or `go list` subprocesses on fixture modules:

  | Test | Elapsed |
  |---|---|
  | `TestGoTestPartialPassRerun` (two subtests, each with two real `go test` runs) | 8.6 s |
  | `TestGoTestKeysResolvedPackagesPerPackage` | 4.0 s |
  | `TestGoTestKeysDeclaredPackagesByTheirScope` | 3.1 s |

  Together they are 68% of the package time.
- All 17 fixture tests use `t.Setenv` and `t.Chdir`. The tool reads `CORVINT_GATE_LEDGER*`,
  `GOFLAGS` and `GO_TEST_TIMEOUT` from the process environment, and it runs `git rev-parse` in the
  working directory.
- Running the tests in parallel needs root and environment injection through `open` and `run`.
  That is a production refactor and was not cheap within this lane, so this ticket stays open.

## V1-1148 (tasks-scale gen): measured, not cut

`tasks-scale gen` was built from this lane and run at load 20 to 25:

| Tickets | Wall time | CPU (user+sys) |
|---|---|---|
| 200 | 72.8 s | 51.8 s |
| 400 | 155.4 s | 122.7 s |
| 800 | 428.3 s | 352.4 s |

Wall time rises 2.1x from 200 to 400 tickets and 2.8x from 400 to 800. Per-ticket cost grows with
the store's size, so the growth is super-linear, and 10,000 tickets would take hours, not minutes.

`gen` calls `cli.Run` in-process for every create, prioritize, claim and complete. Inference, not
profiled: each call re-reads state that grows with the ticket count. Fixing that belongs to the
Tasks store and lease code, which peer lanes hold (storeperf/pools), so the ticket stays open.
The next step is a CPU profile of `gen --tickets 800`.

## V1-2291 (worksource git child groups): skipped

Git children deliberately run in their own process group. `groupreap.Contain` and `Drain` must
be able to sweep that group on cancel (VPO-V0-010, V1-0373), and Darwin has no parent-death
signal. A producer SIGKILL therefore cannot reach them. The fix has to be one of two designs:

- a `groupreap.Retirer` token carried in the run environment and retired by `run.Close` before
  `RemoveAll`;
- a change to the VPO-V0-010 contract.

Either one spans `cmd/corvint` work materialization and the producer-build tests. That is a
design decision, not a test-speed fix.

## NOT_RUN

- Tier 2 of `corvint affected` (batch CI), as AFP-V0-162 requires.
- `make gate`, which is not required for this scope.

## Rollback

Revert the doccorpus commit. The globals and sequential tests come back, and production
behaviour does not change either way.
