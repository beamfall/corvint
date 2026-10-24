# 2026-10-24 Fork-safe executable writes and pipe-drain ordering (V1-2550, V1-2476, V1-2469)

Lane `claude/p1-flakes-1024`; base `0949876eba7b706a77988d94944a46c0979690d2`. Requirement: WQO-V0-093
(proposed). No on-disk or wire encoding changed.

## Findings

- **V1-2550 premise corrected.** In CI run 40048074466 attempt 1 (shard 3, `-race`),
  `TestGoImportsDifferentialAgainstScanner` PASSED (`files=5179 unparseable=0 missed=0 spurious=331`);
  its `DIFFERENTIAL` lines are `t.Logf` output that the log viewer tagged as errors. The test cannot
  fail on a spurious import. The shard's actual failure was
  `TestBuildWithGitExecutionRequiresImmutableObjects/WQO-V0-042`:
  `immutable acquisition fell back to worktree: index=false error=Git error: cannot start Git`. The
  wrapper script had just been written with `os.WriteFile(..., 0o700)` by a parallel test.
- **Mechanism (observed, linux/WSL2).** A process that writes a script and then executes it can fail
  the start with `ETXTBSY` when another goroutine forks between the writer's open and close: the
  child inherits the write descriptor until its own exec closes it (Go issue 22315). The
  `contextindex` package runs dozens of parallel tests that fork Git continuously. Holding
  `syscall.ForkLock.RLock` across open/write/close excludes every concurrent fork from the window.
  `internal/execfile` stress (4 forkers, 4 writer/exec goroutines, `-race`, box WSL2 Ubuntu
  go1.27.3): unguarded `os.WriteFile` 497-630 `ETXTBSY` starts per 5,300-5,850 runs (about 10 %);
  guarded `execfile.WriteFile` 0 per 5,258-8,259 runs, three consecutive runs.
- **V1-2476 (inferred cause, not reproduced).** `workFinalCapture` reported a bare `ADAPTER_FAILED`
  because `observeWork` drops the runner's cause. Materialization wrote `100755` entries with no fork
  exclusion while four parallel captures fork Git, so the adapter's first start could fail `ETXTBSY`
  past the 375 ms retry budget of V1-2433 under a starved CI host. `observeWorkCause` now returns the
  failing step's error and the test prints it; materialization writes executables under the fork
  lock (WQO-V0-093). The runner's 250 ms post-reap drain bound and "process residue" remain
  candidate causes that the new diagnostic will name if the failure recurs.
- **V1-2469 (observed race, code).** `waitForSignalExit` read the server's stdout concurrently with
  `Cmd.Wait`; `Wait` closes the `StdoutPipe` reader once it observes the exit, so a read still in
  flight fails `read |0: file already closed`. The helper now drains to EOF before `Wait`, as
  `waitForExit` already did.

## Stress evidence (box: Windows 11 WSL2 Ubuntu, linux/amd64, go1.27.3, via heavy.sh)

| Run | Base `0949876` | With change |
|---|---|---|
| `internal/execfile -run TestWriteFile -race -count=3` | n/a | PASS; control 10 % ETXTBSY, guarded 0 |
| `internal/contextindex -run TestBuildWithGitExecution -race -count=20` | PASS 45 s | PASS 45 s |
| `internal/contextindex` whole package `-race -count=3` | PASS 323 s | PASS 280 s |
| `conformance/mcp-2026-07-28 -run TestTerminationSignalsCancelInFlightDescendantGroup -count=50` | PASS 10 s | PASS 9 s |
| `cmd/corvint -run TestWorkFinalCheckSourceBoundary -race -count=20` under `taskset -c 0-3` | PASS 334 s | PASS 317 s |

The three CI failures did not reproduce at base under this load (runs were concurrent, four box
slots busy), so the per-test fixes rest on the reproduced mechanism and the code race, not on a
before/after flip of the failing test itself. `-count=50` of the termination test and `-count=20`
of the other two pass with the change.

## Scope and rollback

- Test fixtures in `internal/contextindex`, `conformance/mcp-2026-07-28/lifecycle_unix_test.go`,
  `cmd/corvint/work_final_check_test.go` and `cmd/corvint/work_materialization_test.go` write
  executables through `execfile.WriteFile`. Other packages' write-then-exec fixtures are unchanged
  (ticket filed).
- Rollback: revert the commit; the V1-2433 retry again bounds, but does not remove, the race, and the
  termination helper again races the pipe close.
