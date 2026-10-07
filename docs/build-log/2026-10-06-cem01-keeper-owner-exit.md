# 2026-10-06: cem01-go Darwin keeper retires when its owner exits

## Intent

Ticket V1-0904. On 2026-10-06 the owner found a `cem01-go.test` keeper process (pid 5049) that had
outlived its test run by days, and asked for it to be cleaned up and fixed. The Darwin keeper in
`interop/cem01-go/stable_process_darwin.go` is the test binary re-executed as a process-group
leader. It starts one Git child, reports its status over the fd 3 control socket, and then waits
for control EOF before it kills its group.

## Cause

The keeper treated control EOF as its only sign that the owner had gone. Two cases defeat that:

- **A leaked control end.** If any longer-lived process holds a copy of the owner's end of the
  control socket, fd 3 never reaches EOF and the keeper parks forever.
  `TestStableLifecycleKeeperProtocolError` created its socketpair without close-on-exec and outside
  `syscall.ForkLock`, so both ends could leak into children started concurrently. The production
  owner already sets close-on-exec under ForkLock.
- **A Git child that never exits.** While Git ran, the keeper blocked in `Wait` and did not look at
  its owner at all. An owner killed mid-operation, for example by a test timeout, left the keeper
  and Git running until Git exited.

The pid 5049 process was killed before it was inspected, so which of these two cases left it behind
is NOT_OBSERVED. The fix covers both.

## Change

- The keeper records its parent at start as its owner and registers a kqueue
  `EVFILT_PROC`/`NOTE_EXIT` watch on it.
  - If the owner has already gone at start (registration fails, or the parent pid has changed), the
    keeper retires immediately.
  - The keeper also retires immediately if the kqueue cannot be created or registered (fail closed).
- **While Git runs:** the keeper waits on kqueue for either the Git child's exit or the owner's
  exit. If the owner exits first, the keeper kills its own group, Git included.
- **While holding:** the keeper waits on kqueue for fd 3 to become readable or for the owner to
  exit. Control EOF still retires the group, exactly as before.
- Both new exits use the existing retirement path: `kill(0, SIGKILL)` then exit 2. No polling or
  timer is added, so a parked keeper uses no CPU.
- While the owner is alive, behaviour is unchanged: the keeper stays owned and unreaped until the
  owner's signal decision, as the proposed rules in
  `protocol/cem-1.0/stable/repository-envelope-packet/PROPOSED-RULES.md` require. The new exits
  apply only when no owner is left to make that decision.
- Tests:
  - The protocol-error test now creates its socketpair the way the production owner does.
  - Two new tests re-execute the test binary as a short-lived owner. That owner leaks its control
    end into a long-lived `/bin/sleep` holder, then exits:
    - `TestStableLifecycleKeeperOwnerExitWhileHolding`: a failed launch, then the hold loop.
    - `TestStableLifecycleKeeperOwnerExitWhileRunning`: a fake Git that runs `sleep 300`.
  - Each test requires the keeper (and the Git child) to be gone within 10 seconds while the holder
    still runs.

## Evidence

- Against the unfixed keeper (base `0c27e35f`), both new tests fail with "outlived its owner" after
  10 s. With the fix, the lifecycle suite passes; both new tests take under 0.05 s.
- The full `interop/cem01-go` module tests and vet are recorded below. `go vet` was also run with
  `GOOS=linux` and `GOOS=windows`, which is compile evidence only.
- No other platform is affected. The keeper exists only on Darwin; `stable_process_other.go`
  stubs it out elsewhere. The production binary (`main.go`) and the test binary share the same
  keeper entry point, so the fix covers both.

## Rollback

Revert this change. The keeper then waits only for control EOF again, and the two new tests fail.
