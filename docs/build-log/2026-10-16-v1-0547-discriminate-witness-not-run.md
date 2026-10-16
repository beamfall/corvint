# 2026-10-16 V1-0547: discrimination witness not-run, nested sandbox and host load

Lane V1-0547 (spike). Base `b1ee552b3bebeaacf8f54e62f7f67572f231848e` (origin/main). The fix
changes one test file, `internal/cem/workflow/discriminate_test.go`. It adds no requirement and
amends none. The runner, its sandbox profile and every witness rule (`TCQ-V0-055..059`) are
unchanged.

## Observed

The ticket reports `TestDiscriminateRecordsWitnessAndReportDowngrades` returning
`discriminates 0 / notRun 2` under the nested Codex filesystem sandbox, with a 206 ms strong run.
The host is a 12-core darwin machine. Load averages are the `uptime` values read at the start and
end of each run. All runs below use the same source at BASE.

1. Default agent shell (no enclosing sandbox; a nested `sandbox-exec` applies its profile) at
   load 57-67. The test FAILS with `discriminates 0 / notRun 2`. The strong run takes 56.5 s and
   the test 92.9 s. A scratch probe (not committed) read the witness detail
   `mutation runner failed for pkg/calc/calc_test.go: mutate: run ended without a verdict`
   after 60.4 s.
2. Qualified nested run: the compiled test binary under `codex sandbox -P :workspace` at load
   70-77. The test FAILS with `discriminates 0 / notRun 2` and a 19.8 s strong run. The probe
   detail is `mutation runner failed for pkg/calc/calc_test.go: mutate: sandbox unavailable: the
   host refused to apply the sandbox-exec profile`. Inside that sandbox a nested `sandbox-exec`
   prints `sandbox-exec: sandbox_apply: Operation not permitted` and exits 71.

## Diagnosis

There are two causes, and both produce the same envelope.

1. Nested containment, which is the ticket's case. The enclosing Codex seatbelt forbids a nested
   profile, so every mutant run dies before `go test` starts. `HostSandbox` only looks up
   `/usr/bin/sandbox-exec`. It reports availability and does not probe nesting. V1-0651
   (`TCQ-V0-059`, 2026-10-08) already names this outcome with `mutate.ErrSandboxUnavailable`, so
   the witness stays `not-run`. This is a supported execution limit, not a defect: no
   enforcement can be relaxed without running mutants unsandboxed.
2. Host load, a test defect found while reproducing. The runner gives each `go test` the limit
   `wallTime / (maxMutants + 1)`, and the command is killed 5 s after that. The test passed
   `WallTime: "5m"`, so the limit was 300/7 = 42.9 s. Every run builds from a fresh `GOCACHE`. A
   cold sandboxed `go test` of the fixture package with the runner's exact environment and
   profile measured 74.5 s at load 58-62, and 6.5 s warm. The killed run has no verdict, so
   `runBroken` reports `run ended without a verdict`. With a 30-minute wall time, the same probe
   at load 71-77 returned `discriminates 1` (Mutants 4, Killed 4) and `notRun 1` in 2 min 27 s.

## Decision

- The test's wall time is now `discriminateWallTime = "20m"`, a hang bound rather than a
  performance budget (decision 0082). That gives each run a limit of about 171 s, above the
  measured 74.5 s cold build. The asserted `Bounds.WallTimeSeconds` is 1200.
- When the strong run's first witness detail ends with `mutate.ErrSandboxUnavailable`'s fixed
  text, the test skips with that detail. The detail is the runner's own fixed sentinel
  (`TCQ-V0-059`), and the fixture package is the test's own, so repository output cannot reach
  the skip. On any other envelope mismatch, the test now prints both witnesses instead of the
  bare counts.
- Rejected: probing nesting in `HostSandbox`. That would turn a nested host from `not-run
  (sandbox unavailable)` into `cited tests cannot be sandboxed`, which changes
  `prove --mutate`'s exit contract. V1-0651 rejected the same change for that reason.
- Rejected: a shared or warm `GOCACHE` across mutant runs. That would weaken run isolation and
  is a product change outside this spike.

## Evidence

| Run | Source | Host | Load | Result |
| --- | --- | --- | --- | --- |
| BASE | BASE | default shell | 57-67 | FAIL `discriminates 0 / notRun 2`, `run ended without a verdict` |
| BASE | BASE | nested `codex sandbox -P :workspace` | 70-77 | FAIL `notRun 2`, `sandbox unavailable` |
| fixed | this change | default shell | 69-60 | PASS 470.8 s (strong 2 min 19 s, weak 2 min 13 s) |
| fixed | this change | nested `codex sandbox -P :workspace` | 63-69 | SKIP `host refused the nested sandbox: mutation runner failed for pkg/calc/calc_test.go: mutate: sandbox unavailable: ...` |
| mutation 1: skip removed | this change minus the skip | nested | 63-69 | FAIL; the first witness printed carries the sandbox-unavailable detail, the second `hunk limit 1 reached` |
| mutation 2: wall time 5m/300 | this change with the old bound | default shell | 66-60 | FAIL `run ended without a verdict`, strong 58.1 s |

## Non-goals

- Changes to the mutation runner's per-run limit, its fresh `GOCACHE`, `HostSandbox` or
  `prove --mutate`.
- Linux `bwrap` nesting, which was not observed on a Linux host.

## Residuals (for the orchestrator to file)

1. The runner reports a per-run timeout kill as the generic `run ended without a verdict`
   instead of naming the timeout.
2. The per-run limit is charged with a cold build from a fresh `GOCACHE`. Under host load, a
   mutant run can time out before the first test executes.
3. `HostSandbox` cannot tell an available launcher from one an enclosing sandbox refuses.
