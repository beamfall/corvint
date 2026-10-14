# 2026-10-14: V1-1363 appflows interruption test separates a reap-pending descendant from a survivor

## Intent

Ticket V1-1363 (P2, suspected flake). Hosted run 38494366263 (PR #870, `go-product-shard (6)`,
ubuntu-24.04, `go test -race -p 1` under test-confine) failed
`TestAFUV0NoDescendantsOnInterruption` (`AFU-V0-010`, `docs/specs/application-flow-understanding-v0.md`)
with `descendant N survived interruption` for both the child and the grandchild, in 0.64s. A rerun
of the whole package passed. The goal was to find out whether the observer leaks a descendant, or
whether the test counts an exited descendant that has not yet been reaped as a survivor.

BASE: `origin/main` `adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f`.

## Cause

The local runs reproduce the mechanism: a descendant that has exited but has not yet been reaped
still answers signal 0. The hosted failure is classified as that case by inference from its
duration (see below), because its process state was not captured. No real survivor was found.

### Teardown order

The observer kills its descendants, but it reaps only its leader.

1. On cancellation, `runObserver`'s `BeforeStop` hook sends SIGINT to the Node leader only, then
   waits up to 500 ms (`internal/appflows/observer.go:96-110`). The leader exits. Its child is
   orphaned at that point and reparented to PID 1 (or the nearest subreaper). The grandchild's
   parent is the child, so the grandchild is reparented the same way once the child exits.
2. `procgroup.run` sends SIGKILL to the owned group (`internal/procgroup/process.go:421`;
   `internal/procgroup/process_posix.go:93`). The child and the grandchild are members of the
   group.
3. `procgroup.run` then proves the group quiet while the exited leader is held unreaped
   (`internal/procgroup/process.go:447`, `proveProcessGroupQuiescent` in
   `internal/procgroup/process_posix.go:137`). The proof is `groupreap.HeldGroupQuiet`
   (`internal/groupreap/owner_waitid.go:76`). On Linux it is the `/proc` proof of `PGO-V0-006`,
   which counts a single-threaded zombie as quiet (`procDead` in
   `internal/groupreap/owner_proc.go:37`).
4. Only the leader is reaped (`internal/procgroup/process.go:454-461`), and the observation
   returns.

Nothing in the observer waits for PID 1 to reap the two orphans. Until PID 1 does, `kill(pid, 0)`
succeeds on their zombies, and the BASE test reported that as "survived".

### PID 1 and subreapers under test-confine

- test-confine only applies Landlock and `exec`s the test binary
  (`.github/testconfine/landlock_linux.go:82`, `.github/testconfine/cmd/exec_unix.go:11`).
- It creates no PID namespace and sets no child subreaper. In this repository only
  `internal/tasks/service` sets `PR_SET_CHILD_SUBREAPER`.
- The hosted shard runs directly on the runner VM, with no `container:` in the workflow, so the
  orphans go to that VM's PID 1. That PID 1 is systemd on GitHub-hosted images. This is an
  inference: the hosted process tree was not observed.
- Reaping is therefore asynchronous to the test, and it is slower when the runner is loaded.

### How the hosted failure is classified

The hosted failure was a zombie awaiting its reap, not a real survivor. The evidence is the run's
duration:

- A real survivor stays a live group member. The Linux quiet proof then keeps polling until the
  2 s shutdown deadline (`defaultProcessShutdownLimit`). With a mutation that signals only the
  leader, the BASE test failed both descendants in 2.67 s and 2.68 s (Linux container).
- The hosted failure took 0.64 s, which is the passing duration (0.60 to 0.81 s in the container
  runs below). So the quiet proof saw no live member: both descendants had already exited, and
  signal 0 reached only their zombies.
- The hosted `/proc/<pid>/stat` state was not captured. Its classification is an inference from
  timing; the mechanism itself was reproduced locally.

### Local reproduction

These runs used a Linux arm64 container: `golang:1.27.2` plus the Node 24.21.0 binary, kernel
6.8.0-117, under colima. `go.mod` now requires go 1.27.2, so the go1.27.1 named in the charter
cannot build the module. A scratch diagnostic test, never committed, logged `/proc` state at the
moment the interrupted observation returned:

- **PID 1 that does not reap** (`exec go test`, so `go` is PID 1):
  - The BASE test failed 3 of 3 runs. Each failure reported both descendants, the hosted message
    exactly.
  - Both descendants read `state=Z ppid=1 pgrp=<leader> threads=1` when the observation returned,
    and still read Z 3 s later.
- **Reaping PID 1** (`docker-init`/tini) **with 12 busy loops on 6 CPUs**:
  - In 1 of 40 descendant observations, the descendant was `state=Z ppid=1` when the observation
    returned. It reached ESRCH 2.7 ms later.
  - The BASE test happened to pass that batch (20/20). It fails only when its signal-0 check lands
    inside that window, which is how the hosted run failed.

## Change (test only)

`TestAFUV0NoDescendantsOnInterruption` (`internal/appflows/observer_unix_test.go`) now calls
`awaitDescendantRetired` for each descendant. It polls every 20 ms for up to 2 s until the
descendant is retired:

- A descendant is retired when signal 0 returns ESRCH, or when it is an exited zombie. Only ESRCH
  proves absence. A process table that cannot be read leaves the descendant unretired, and the
  next poll sees ESRCH if it has exited meanwhile.
  - On Linux, a zombie is `/proc/<pid>/stat` state Z or X with `Threads: 1`. A zombie leader with
    other live threads is still live, as in `PGO-V0-006`.
  - On Darwin, a zombie is a `ps -o stat=` state that starts with Z. Each `ps` call is bounded at
    1 s, so the wait ends within about 3 s.
- A descendant that is still live at the bound fails the test. The failure names its last state,
  for example `(state S)`, so a future hosted failure classifies itself.
- The test SIGKILLs any such survivor so that a failing run does not leak Node processes.

No production code changed: no leak was found, so clause 3 does not apply. `AFU-V0-010` behaviour
and its traceability row are unchanged, and no requirement was added.

## Acceptance evidence

- **The test still catches a real survivor (mutation).** In a scratch copy, the observer's
  `procgroup.Spec` was set to `JoinAncestorProcessGroup: true`, so procgroup signals only the
  leader and never kills the descendants. The new test failed every run:
  - Linux (tini): 2/2 runs failed, `descendant N survived interruption (state S)` for both
    descendants.
  - Darwin: 2/2 runs failed with the same message. This was rerun with the final test.
  - No Node process was left behind (`pgrep -f setInterval` counted 0 before and after).
  - The mutation was reverted and was not committed.
- **Linux, new test, `-race`:**
  - `-count=20` under tini: 20/20 PASS.
  - `-count=20` under tini with 12 busy loops on 6 CPUs (load average reached 5.84): 20/20 PASS.
  - `-count=20` with a non-reaping PID 1 and the same contention: 20/20 PASS.
  - `-count=5` with a non-reaping PID 1, where the BASE test fails every time: 5/5 PASS.
  - Full package `go test -race -count=1 ./internal/appflows/`: ok. `go vet ./internal/appflows/`:
    ok.
- **Darwin (host load average 60 to 76), new test, `-race`:**
  - `-count=5` with the final test: PASS.
  - Full package `go test -race -count=1 ./internal/appflows/`: ok (1455 s), on the first
    revision of the test, before the review fixes. The Linux full-package run was repeated with
    the final test: ok (264 s).

- **Other packages from `corvint affected --base adf587fdb`:** the plan selected 52 units.
  - The 8 with a specific witness (path-literal or declared read scope) were run with
    `go test -count=1` on Darwin, and all passed: `internal/dashboard/adapters`,
    `internal/diagnostic`, `internal/mcp/bridge`, `internal/obscorpus`, `internal/semescalate`,
    `cmd/corvint-go-test-provider`, `tools/corvint-pr-tests` and
    `tools/fixture-maintenance-check`.
  - `internal/mcp/bridge` first failed once under the host load, in
    `TestMCPCEMReportRefusesAMissingCitedBlobWithAValidMapWithoutFetching`
    (`repository-unavailable`). It passed when rerun alone, so it was environment noise.
  - The 43 units selected only as unbounded readers were NOT_RUN. The change touches only a
    `_test.go` file and this build log, and the hosted shards run those units.
- **Doc gates:** `gofmt -l` printed nothing, and the 12 CI doc gates passed.

## Retained limits

- **20 hosted runs not met.** The acceptance clause asks for 20 consecutive `-race` runs on hosted
  ubuntu, and that cannot be met locally. It is a residual. The PR's own hosted shards observe
  only one run of the package, and no workflow was added.
- **Hosted cause inferred.** The hosted runner's PID 1 and the hosted `/proc` states were not
  observed (NOT_OBSERVED). The classification rests on the run's duration and on the local
  reproduction.
- **Darwin zombie check reads `ps`.** If `ps` is unavailable, a zombie reads as unreadable, so
  the test waits the full 2 s and then fails. The test fails safe; it never passes a survivor.
- **Survivor cleanup is best effort.** The test SIGKILLs a survivor but does not check the
  result or wait for its exit. The test has already failed at that point. Codex finding 3 was
  recorded here, not fixed.

## Review

There was one Codex read-only round (`codex exec -s read-only`), with verdict CONCERNS. Each
finding was checked against the source:

1. A missing `/proc` entry was read as absent after signal 0 had found the PID. That is fail-open
   if `/proc` is unavailable. Confirmed and fixed: only ESRCH proves absence.
2. The Darwin `ps` call had no timeout. Confirmed and fixed: each call is bounded at 1 s.
3. The survivor SIGKILL result is ignored. Recorded under retained limits, not fixed.
4. The build log stated the hosted cause as confirmed. Confirmed and fixed: the cause is now
   worded as an inference.

The Linux batches and the mutation runs above were repeated with the final test.

## Rollback

Revert the commit. The test returns to an immediate signal-0 check, and the flake returns with it.
