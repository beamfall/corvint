# PMR-V2-006 collector cleanup boundary repair (#395, V1-0542)

The Codex re-review of `7d2918ae..9e3c82d8` confirmed that the first-round fixes hold. It found four
new defects in the owned cleanup boundary that the first round added. Each one was checked against
the source before it was changed.

Findings and fixes:

- P1-1, signals by bare PID. Confirmed. The walk sent SIGSTOP and SIGKILL with `kill(pid)`. It also
  treated a changed start time as "stopped" and then listed that PID's children. If a parent ignores
  SIGCHLD, its exited child is reaped at once and the PID can be recycled.
  - The walk now opens a pidfd (`pidfd_open`) for each listed birth. It confirms the birth after the
    open, and it signals only through that pidfd (`pidfd_send_signal`).
  - Every later state read is confirmed with a signal-0 probe on the pidfd. That probe fails only
    after a reap, so the PID still named the process during the read.
  - A gone or mismatched birth is never signalled or walked. It is reported unresolved.
  - Tests:
    - `TestProcessCollectorV2CleanupSkipsRecycledPID`: a controlled stand-in that substitutes the
      listed start time. The real process stays unstopped.
    - `TestProcessCollectorV2CleanupReportsAutoReapedChild`: a perl parent with
      `$SIG{CHLD}="IGNORE"`, whose child is killed between listing and freezing.
- P1-2, a zombie counted as stopped. Confirmed. A child that forks and exits before its stop leaves
  its grandchild reparented outside the tree, and cleanup still reported success.
  - Design fork, settled without owner input as the smallest fail-closed option: no subreaper
    containment.
    - Why: `PR_SET_CHILD_SUBREAPER` would change process-global state of the launcher. It would also
      change the PPID graph that the verifier derives, and it would leave unreaped orphans in the Go
      process.
  - Chosen behaviour: a member that exited, was reaped or changed birth before its freeze was
    confirmed makes the cleanup unresolved.
    - The BLOCKED detail names each unresolved member, and the failure is already latched, so no
      proof follows.
    - Zombie children of frozen members are listed for this reason.
  - Test: `TestProcessCollectorV2CleanupReportsForkAndExit`. It uses a FIFO to release the child
    between listing and freezing.
- P2-3, unbounded `Cmd.Wait`. Confirmed: when an escaped process held the leader's stdout, `Wait`
  blocked for that process's whole life (31.7s in the mutation run).
  - `start` now sets a zero `WaitDelay` to 3s.
  - `Retire` fails the collection on any `Wait` error other than an exit status.
  - Test: `TestProcessCollectorV2EscapedOutputDoesNotStallWait`.
- P2-4, unlatched capture errors. Confirmed.
  - `Capture` now latches capture failures, and so do child discovery and capture in `AwaitChild`.
  - A post-sweep capture remains a refusal and is not latched.
  - Test: `TestHostCollectorV2FailedCaptureEmitsNoProof`. A cancelled during-run capture, then a
    successful sweep, still leaves `Proof` refused.
- Hypothesis on thread-group stop. Implemented rather than only recorded: a member counts as frozen
  only when every `/proc/PID/task/*/stat` shows `T`.
  - The walk repeats until a pass finds every member frozen and no new child. That pass also catches
    children added through `CLONE_PARENT` or subreaper reparenting during the walk.
  - Fixture: `TestProcessCollectorV2CleanupFreezesAllThreads`. It re-executes the multithreaded Go
    test binary and retires it cleanly.

Mutation evidence, Linux arm64 container:

- Each of the following reverts was applied in turn, with everything else unchanged:
  - drop the birth check from the pidfd read;
  - drop the `Capture` latch;
  - leave `WaitDelay` unset.
- Each made its new test fail: `CleanupSkipsRecycledPID`, `FailedCaptureEmitsNoProof` and
  `EscapedOutputDoesNotStallWait`.

Evidence:

- darwin: `go test` passes for `internal/postmergehost`, `internal/postmergeproof/...` and the
  launcher.
- Linux arm64 container (go1.27.1, kernel 6.8): all new and related tests pass under `-race -v`, and
  the three packages pass in a full run.
- `go vet` is clean for darwin, linux/arm64 and linux/amd64.
- NOT_RUN: Linux amd64 runtime.

Limits retained:

- The pidfd calls need Linux 5.3 or later. When they are unavailable, cleanup is unresolved.
- The cleanup boundary does not cover:
  - a descendant reparented before the walk could list its parent;
  - tree members that resume or stop one another during the walk, which is outside the
    trusted-local model.

Rollback: revert this change. Nothing consumes the collector yet.
