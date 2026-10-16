# 2026-10-16 V1-1492: pool sweep observation bounds scale with the sweep budget

Base `c4a7522224bb83ae47d2e544eb03fa73b2af39fd`. Darwin/arm64, 12 CPUs, go1.27.2, shared with other
agent lanes. Decision `0536-pool-sweep-observation-bounds-scale-with-budget-2026-10-16.md`
(option (a), owner delegated to the orchestrator 2026-10-16); requirement `PSR-V0-023`.

## Change

- `store.PoolSweep` puts its `--timeout-seconds` budget on the sweep context through a private key.
- `sweepScaledBound` multiplies each fixed bound by budget/10 s, with the fixed value as the floor
  and 5 times it as the cap. It applies to the sweep Git probe (2 s), the runner-identity and
  group-listing probes (1 s, pipe delay 250 ms) and the sweep command pipe delay (1 s).
- The cleanup proof keeps its fixed 5 s allowance and recorded `cleanupAllowanceMillis` 5000. Its
  group probes take the scaled bounds through `sweepBudgetOnly`, which drops any V1-1324 test lift.
- Lease-path pool preparation and cleanup carry no budget and keep the fixed bounds.
- Outcome classes are unchanged.

## Evidence

- Unit (`TestPSRV0023SweepBoundsScaleWithBudget`, `TestPSRV0023ScaledProbeBoundEnforced`): the
  scaling table, floor and cap. A real `/bin/sleep 1.5` probe passes at budget 50 s and fails with
  no budget. At budget 1800 s a `/bin/sleep 30` probe still fails, at the 5 s cap.
- Mutations, one run each:
  1. Scaling disabled (`sweepScaledBound` returns the fixed bound): both tests fail. The table
     reports the fixed values, and the cleanup proof's bound is 1 s.
  2. Cap removed: the table fails (6m0s and 3m0s at 1800 s), and the real probe runs the full
     30 s and fails with "capped bound not enforced" (32.6 s).
- CLI target: a `-race` test binary, `-run 'TestPSRForegroundSignals|TestPSRDispatchNativeAdapter'`.
  The host's own lane load was raised by 10 CPU busy loops held only for the loop.
  - Fixed: 20/20 consecutive runs passed. Load averages (1 min) at run start/end were 44.3-104.4,
    mean 72.4. `compiled-worker-overlap` skips without `PSR_TEST_BINARY`, as in the ticket's
    reproduction.
  - BASE in this lane: the failure did not reproduce. With 36 busy loops (load 47-150), 8/8 runs
    passed; with natural load 47-125, 6/6 passed. The failing counts from #965 (signals 24/34,
    dispatch 4/5 at load 56-88, I/O-heavy lane load and no burners) remain the before-evidence.
    CPU-only burners do not recreate them.
- No assertion in either CLI test changed.
- Store PSR tests (`-run '^TestPSR'`, `-race` binaries, run one after the other on the shared host,
  no burners):
  - Fixed, at load 62-72: 10 top-level failures, 376 s.
  - BASE, at load 53-61: 16 top-level failures, 759 s.
  - Every fixed-binary failure also failed on BASE. These tests are already load-sensitive without
    the V1-1324 lift (V1-0849), and this change does not make them worse.
  - In isolation at load ~75, `TestPSRSweepStderrIsNotStdout`, `TestPSRSweepSuccessReplaySuccessor`
    and `TestPSRDispatchAllocationFence` pass on both binaries.

## Independent review

One Codex round (read-only, diff range) found one issue, which is fixed. `sweepGit` truncated the
scaled Git bound to the whole seconds a pool command carries, so at a 14 s budget the bound was 2 s
instead of 2.8 s. `sweepGitBoundSeconds` now rounds up, keeping the 2 s floor and the 10 s cap, and
PSR-V0-023 says so. The table test checks budgets 0, 14, 15, 30 and 1800 s. A mutation back to
truncation fails it (14 s: got 2s want 3s). After the fix, the CLI target passed one `-race` run at
load 57-58.

## Not run

`make gate`; a quiet-host run of the whole `internal/tasks/store` package (one loaded
`-run 'PSR|Pool|Sweep|CALV0078|CALV0033'` pass failed only in the load-sensitive PSR and
program-workflow CALV0078 tests named above and in the 2026-10-15 V1-1465 entry); Linux.
