# Tests-reliability batch: sync points, seams and measurements

Date: 2026-10-20/21. Lane: claude/batch-tests-reliability. Base: origin/main 5cf75daf0.
Host: darwin, 12 cores, Go go1.27.3 (and the cached go1.27.2 toolchain where noted).

## Decisions

- **V1-1545 and V1-1434 (`internal/testrunner`).** The execution limit is the package seam
  `executionLimit`, and `groupreap.EscapedSampled` reports the sampled escaped set. The retire
  tests:
  - expire the limit only after the descendant is published and sampled;
  - write the pid file atomically.

  `TimeoutSeconds` stays only as a hang bound.
- **V1-0778 (`internal/authoritystore` darwin tests).** On BSD, a new file inherits its
  directory's group, and `/private/tmp` is wheel. The audit refuses that correctly, so the
  fixture chowns its `t.TempDir` root to the effective gid instead of weakening the audit. A new
  negative test keeps a foreign inherited group refused. The socket tests bind under `/tmp` to
  stay inside the 104-byte `sun_path`.
- **V1-1481 (`internal/doccompiler` process tests).** The PID handoff now waits on the
  command's completion channel, not a deadline. Exec `Wait` returns only once every holder of
  the stdout pipe has closed it, so a surviving descendant can only be reaped or a zombie when
  the test checks it. The `ps` state check accepts `Z` and nothing else.
- **V1-1521 (`tools/corvint-pr-tests`).** On Darwin the frozen Git identity also binds the
  target that `xcrun --find git` resolves behind the `/usr/bin/git` shim. A resolver error, a
  relative path, a missing file, or a target equal to the shim refuses. AFP-V0-053 (1) is
  amended.
- **V1-1445 (`internal/sessiontest`).** `Runner.deadline` binds the discovery and launch
  contexts to the shard deadline on the same clock as `Runner.now`. Tests drive a virtual clock
  and one FIFO gate per fake run. Two mutations still fail STR-V0-015: a per-invocation
  `context.WithTimeout(ctx, budget)`, and a per-invocation virtual deadline. The probe's own
  `discoverDeadline` stays real.

## Measurements

| Ticket | Command (`heavy.sh env GOTOOLCHAIN=local go test ...`) | Load | Result |
|---|---|---|---|
| V1-1001 | `-count=1 -timeout 9m -p 4 -v ./internal/doccorpus/` | 14-24 | 9 m bound hit with 165 of 179 tests done; 7.3 s CPU in 97 s wall in the profile (git subprocesses in `internal/contextindex`, a path outside this lane) |
| V1-1476 | `-count=10 -timeout 9m -p 4 ./internal/playwrightminimize/` | ~31 | pass, 7.0 s, no "descendant snapshot unavailable" |
| V1-1476 | `-count=10 -timeout 9m -p 4 ./internal/testacceptance/` | 23-29 | pass, 137 s |
| V1-1819 | `-count=20 -timeout 9m -p 4 ./internal/liveverify/affected/` | 21 → 9 | pass, 382 s |
| V1-1446, V1-1522 | `-count=1 -timeout 9m -p 4 -v ./tools/corvint-pr-tests/`, cached go1.27.2 on PATH, `GOPROXY=off` | 10-15 | pass, 221.9 s. Slowest tests: TestAFPV0048ShardedRunExecutesSlices 82.5 s, TestSelectedFailureAndFallback 70.5 s, TestShardedPRExecution_AFPV0022 45.3 s |
| V1-1445 | `GOMAXPROCS=1 -count=20 -run 'TestSTRV0014RunnerCancelAndBudgetKillGroup\|TestSTRV0015'` | ~20 | pass, 21.3 s; before the change each run spent at least 14 s of real sleeps |

Under the host go1.27.3, `tools/corvint-pr-tests` fails five tests with "unsupported Go
version", because it pins go1.27.2 (`pinnedGoVersion`). This is a toolchain mismatch, not a
hang.
