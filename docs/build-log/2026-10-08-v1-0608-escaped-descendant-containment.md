# 2026-10-08: Escaped-descendant containment for the test-runner executor (V1-0608)

## Intent

Ticket V1-0608 records that a Playwright 1.63 run hitting the six-second executor timeout left its
native Chrome process group alive: Playwright launches the browser detached (`setsid`), so the
executor's single SIGKILL to the runner's process group never reached it. The 2026-10-01 repair
(`2026-10-01-cem-stable-target.md`) made the executor send Playwright a cooperative SIGINT; that
retires the browser only when the runner cooperates. This change makes retirement executor-owned
for every profile. It adds proposed `PGO-V0-007` (`groupreap.RunContained`) and `TRE-V0-034`
(executor use), pending owner acceptance.

## Decisions

- **Structural ownership first.** After the leader exits, it stays an unreaped zombie
  (`waitid WNOWAIT`), so its group ID cannot be reused. The group is SIGSTOPped; an escaped
  descendant (a process reached from a group member by parent links, never older than its parent)
  is stopped or killed individually only while its parent is an owned process observed stopped,
  and each individual signal is pinned to the target's PID and start time (below). A session-leading escapee gets one group
  SIGKILL, because every member of its session descends from it. Then comes the usual group
  SIGKILL and reap.
- **Sampled orphans by identity.** A run-time sampler (200 ms, at most 4096 identities) records
  escaped identities while their ownership is structural. An identity orphaned before the sweep
  (Playwright tears down its worker on SIGINT, so a browser that worker spawned is reparented to
  launchd) is SIGKILLed, deepest first, only while a fresh table shows the same PID and kernel
  start time. A first draft only reported such orphans. The live detached-browser runs below showed
  that left 8 or 9 survivors on every graceful path, so the identity step was added. Its residual
  window between the table read and `kill` is documented in PGO-V0-007.
- **Leader-only non-graceful cancel.** A non-graceful timeout or interruption now kills only the
  leader (`cmd.Process.Kill`), not the group, so escapees stay attached to stopped group members
  for the structural sweep. The graceful SIGINT path and its 5 s fallback are unchanged.
- **Incomplete stays incomplete.** A survivor, an unavailable process table, an unstoppable owned
  tree, a signal failure or an unavailable leader exit observation adds a `process-containment`
  execution problem, so shared normalization marks the observation incomplete.
- **Process tables.** Darwin reads `sysctl kern.proc.all` (`kinfo_proc` offsets checked by
  `TestProcessTableIdentity` against the live process); Linux reads `/proc/<pid>/stat` through the
  existing bounded readers. `Run`, `Wait` and the Owner are unchanged (PGO-V0-005).

## Evidence

Failing before, passing after (Darwin arm64, this host):

- `TestExecuteRetiresEscapedDetachedDescendants` with the base `execute_unix.go`: all four
  subtests fail with `detached browser N survived the executor`
  (`/private/tmp/claude-501/pw-td/failing-before.txt`, sha256 `4d78c4fe…50cc`). With the change, all
  pass. Focused `go test -p 1 ./internal/groupreap/... ./internal/testrunner/...` passes.
- Unit identity and collateral negatives: a host-browser stand-in in its own session, a reused PID
  whose start predates its claimed parent, a reparented same-group process, a running (unstopped)
  parent, a zombie, a reused sampled PID and an already-retired identity are never signalled
  (`TestRetireEscapedIdentityBoundary`, `TestRetireSampledOrphansIdentityBoundary`,
  `TestObserveSurvivorsUsesIdentity`). Real-process tests keep a bystander in its own session
  alive (`TestRunContainedRetiresEscapedSessionWithoutCollateral`,
  `TestRunContainedRetiresSampledOrphanByIdentity`).

Live native qualification through the actual `corvint-test-runner plan`/`run` path:
Playwright `@playwright/test` 1.61.1 (an existing local install, used read-only), Node 22.23.3 and
Playwright's Chromium 1228 (`Google Chrome for Testing`) via `executablePath`. The host's own
Chrome was never used or signalled. Each spec writes a DOM-ready marker after
`expect(#ready).toHaveText('ready')`. The `-detached` variants also spawn a second Chrome with
`detached: true` from the test worker, standing in for a non-cooperating native browser. The
harness (`/private/tmp/claude-501/pw-td/live/scripts/qualify.py`, sha256 `e81a5db2…c76`) samples
the runner's process tree, recording pid and start time. After return it checks survivors by that
identity and the count of pre-existing Chrome-like processes (9, including the owner's Chrome).

| Run | Base binary (722904f9) | This change |
|---|---|---|
| normal | not run | exit 0, observation complete, 0 survivors |
| timeout (6 s) | 0 survivors | 0 survivors |
| SIGINT | 0 survivors | 0 survivors |
| timeout + detached browser | 9 survivors (manual identity-checked cleanup) | 0 survivors, no `process-containment` |
| SIGINT + detached browser | 8 survivors (manual identity-checked cleanup) | 0 survivors, no `process-containment` |

Before review, every run kept all 9 pre-existing Chrome-like processes. After the review repairs
the five runs were repeated with the same results: 0 survivors and no manual cleanup. The
owner's Chrome had by then grown to 16 or 17 Chrome-like processes. In the timeout-detached run,
one of the owner's `data_decoder` utility helpers, started 08:11:19, exited during the run. That
was not a signal from the runner: ownership reaches only the leader's group and its descendants,
and none of them can start before the leader (08:18:02). Orphan retirement signals only those
sampled identities. A 90 s idle control with no Corvint run saw two more of the owner's helpers
exit (`live/control-idle-churn.json`). Results are under
`/private/tmp/claude-501/pw-td/live/{base,new}-*/result.json`. The post-review
new-timeout-detached result has sha256 `920614de…08d8`; base-timeout-detached has
`fb6ce334…9ac0`. Timeout and SIGINT receipts stay incomplete (`timeout`/`interrupted` problems),
as before.

## Independent review

Codex (gpt-6-astra, read-only) reviewed `f9c35b24`. Its three findings were all fixed:

- P1: sampled orphans were retired after `Wait`, so an orphan holding the inherited output pipe
  blocked `Wait` (up to the executor's one-minute `WaitDelay`). They are now retired before
  `Wait`. `TestRunContainedRetiresSampledOrphanByIdentity` now pipes the leader's output and
  failed at the old order (`RunContained waited on an orphan holding the output pipe`), then passed.
- P2: on Linux, a zombie thread-group leader with live threads was classified dead. It is now
  live when `/proc/<pid>/status` counts more than one thread (`TestReadProcTableThreadAwareZombie`
  on a fixture `/proc`; a live Linux run remains NOT_RUN).
- P2: identities beyond the 4096 bound were dropped silently. They now make containment
  incomplete (`TestRecordEscapedBound`).

Codex re-reviewed `a424229b`. It confirmed the three repairs and raised three Linux findings, all
repaired in a follow-up commit:

- P1: a stopped parent does not hold its child's PID when it auto-reaps (SIGCHLD ignored or
  `SA_NOCLDWAIT`); the kernel reaps the exiting child even while the parent is stopped. Every
  individual signal now goes through `signalPinned`. On Linux it opens a pidfd, rechecks the start
  time, then signals through the pidfd. A session-group signal counts only if the pinned leader
  still exists afterwards, and a kernel without pidfds fails closed. Darwin has no pidfd, so it
  rereads `kern.proc.pid` just before `kill`; that leaves a residual window, recorded in
  PGO-V0-007. The real-process test `TestSignalPinnedIdentity` shows that a changed start time is
  never signalled.
- P1: `/proc/<pid>/stat` reports only the leader thread. A stopped or zombie leader is now
  classified from every task under `/proc/<pid>/task`. The process counts as stopped only when
  every live task is in a group stop (`T`). A tracing stop (`t`) counts as running, so a traced
  parent fails closed.
- P2: an exited leader with stopped threads was classified as running forever, so the freeze timed
  out. It now takes its threads' state (`TestReadProcTableThreadAwareState`, a fixture `/proc`).

After these repairs, a rebuilt runner repeated the live timeout-detached, SIGINT-detached and
normal runs. Each ended with 0 survivors and no `process-containment`, escaping 8, 7 and 6
browser processes respectively, and all 23 pre-existing Chrome-like processes were still present
(`live/v3-*/result.json`; timeout-detached sha256 `043951c1…6983`). The same follow-up moved
`TestRecordEscapedBound`'s synthetic PIDs above `pid_max`. It had failed once when the test
process's own PID fell inside them, because `ownedTree` excludes that PID.
`TestRetireEscapedIdentityBoundary` skips that collision explicitly.

## Non-goals

No change to Playwright's cooperative SIGINT, the Owner, `Run`/`Wait`, or other callers. No
retirement of a descendant that escapes and is orphaned within one sample interval (never
observed, so never signalled); no account-wide process management; no hostile process that
rewrites its start time or races PID exhaustion.

## Failure modes

Process-table read failure, an owned tree that does not stop within 2 s, a signal error, or a
retired or sampled identity still alive after the 2 s signal-free observation: each is reported as
`process-containment` and leaves the observation incomplete. If the leader's exit cannot be
observed unreaped, nothing escaped is signalled and containment is incomplete. On Linux a missing
pidfd or a session leader released during its group signal is incomplete too. On Darwin a target
that exits and is reaped between the identity read and `kill`, with its PID reused within that
window, is the documented residual risk.

## Rollback

Revert `internal/groupreap/escape.go`, `proctable_*.go` and their tests. Return
`internal/testrunner/execute_unix.go` to `groupreap.Run` with the group-SIGKILL `Cancel`, and drop
PGO-V0-007/TRE-V0-034. Plans and receipts carry no new fields.

## NOT_RUN / NOT_OBSERVED

- Exact Playwright 1.63 qualification: NOT_RUN. No 1.63 install exists locally, and downloading
  one needs separate approval. The live runs above used 1.61.1 with Chromium 1228.
- The ticket's original three-survivor evidence files under `/private/tmp/cem10-build/
  browser-qualification` are no longer on disk (temporary-directory purge): NOT_OBSERVED here.
  They are retained only by the ticket's recorded digests and are kept separate from this evidence.
- Linux execution of the escaped-descendant path (pidfd signalling, `/proc` task-state parsing on a
  live host): NOT_RUN. Linux code is checked only by `GOOS=linux go vet` and fixture tests.
- SIGTERM to the runner is handled by the same context cancel as SIGINT but was not separately
  live-run.
