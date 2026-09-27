## 2026-09-26 V1-0390 WQO-V0-005 VPO-V0-010: the Git pipe-drain bound detects a held pipe, not a slow reader

Found on a loaded host (swap full, load above 400) while binding change evidence. `dogfood change`
and `record` failed with `Git error: exec: WaitDelay expired before I/O complete`, and the trace
record adapter rendered the same failure as `returned non-zero exit status -1`. Git had exited 0.
`os/exec` starts the `WaitDelay` timer when the process exits; if the goroutine copying stdout has
not been scheduled to finish within that delay, `Wait` returns `exec.ErrWaitDelay` even though every
byte is already in the pipe. `contextindex` and `gokernel` set `WaitDelay` to one second, so a
successful call failed whenever the reader was starved for a second. Of 112 contextindex tests run
on that host, 37 failed this way and passed on a single rerun.

Decision: the bound after Git exits or cancellation starts exists to detect a descendant that keeps
the pipes open. It is not a latency budget. `contextindex` and `gokernel` now use one minute, the
value `work-queue-observation-v0.md` already states for its "pipe-drain detector" and that
`worksource` uses. Cancellation still kills the process group, which closes the pipes, so a
cancelled call does not wait out the bound. `contextindex` keeps the bound in a variable so the
held-pipe test can shorten it. `recordIndexError` no longer renders the `-1` placeholder of a
signal or a drain expiry as a Python exit status; it returns the underlying cause.

Out of scope: the one-second `WaitDelay` in `doccompiler`, `taskman`, `liveverify` mutation and the
work executable binding. They are not Git acquisition, and the last has tests that depend on the
value. They can show the same starvation symptom and are left for a follow-up.

Evidence: `TestGitCapturesOutputThatDrainsAfterGitExits` passes, and fails with the production error
when the bound is set back to one second. `TestBuildWithGitExecutionOwnsPipesAndCancellation`, with
the bound shortened to one second, still fails a held pipe within its four-second lifecycle bound and
reaps both descendants. `TestRecordIndexErrorKeepsASignalCause` passes, and without the guard it
fails with `returned non-zero exit status -1.`. The gokernel descendant and process-group tests pass.
Rollback: revert the change; loaded hosts then fail successful Git calls again.
