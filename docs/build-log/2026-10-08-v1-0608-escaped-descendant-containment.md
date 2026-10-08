# 2026-10-08: Escaped-descendant containment for the test-runner executor (V1-0608)

## Intent

Ticket V1-0608 records that a Playwright 1.63 run hitting the six-second executor timeout left its
native Chrome process group alive: Playwright launches the browser detached (`setsid`), so the
executor's single SIGKILL to the runner's process group never reached it. The 2026-10-01 repair
(`2026-10-01-cem-stable-target.md`) made the executor send Playwright a cooperative SIGINT; that
retires the browser only when the runner cooperates. This change makes retirement executor-owned
for every profile. It adds proposed `PGO-V0-007` (`groupreap.RunContained`) and `TRE-V0-024`
(executor use), pending owner acceptance.

## Decisions

- **Structural ownership first.** After the leader exits, it stays an unreaped zombie
  (`waitid WNOWAIT`), so its group ID cannot be reused. The group is SIGSTOPped; an escaped
  descendant (a process reached from a group member by parent links, never older than its parent)
  is stopped or killed individually only while its parent is an owned process observed stopped, so
  its PID cannot be reaped and reused before the signal. A session-leading escapee gets one group
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

Every run lost none of the 9 pre-existing Chrome-like processes. Results are under
`/private/tmp/claude-501/pw-td/live/{base,new}-*/result.json` (new-timeout-detached sha256
`02f8a136…eb7e`, base-timeout-detached `fb6ce334…9ac0`). Timeout and SIGINT receipts stay
incomplete (`timeout`/`interrupted` problems), as before.

## Non-goals

No change to Playwright's cooperative SIGINT, the Owner, `Run`/`Wait`, or other callers. No
retirement of a descendant that escapes and is orphaned within one sample interval (never
observed, so never signalled); no account-wide process management; no hostile process that
rewrites its start time or races PID exhaustion.

## Failure modes

Process-table read failure, an owned tree that does not stop within 2 s, a signal error, or a
retired or sampled identity still alive after the 2 s signal-free observation: each is reported as
`process-containment` and leaves the observation incomplete. If the leader's exit cannot be
observed unreaped, nothing escaped is signalled and containment is incomplete. A parent that
reaps its child between the sampled-identity table read and `kill`, with the PID reused at the
same start time, is the documented residual risk.

## Rollback

Revert `internal/groupreap/escape.go`, `proctable_*.go` and their tests. Return
`internal/testrunner/execute_unix.go` to `groupreap.Run` with the group-SIGKILL `Cancel`, and drop
PGO-V0-007/TRE-V0-024. Plans and receipts carry no new fields.

## NOT_RUN / NOT_OBSERVED

- Exact Playwright 1.63 qualification: NOT_RUN. No 1.63 install exists locally, and downloading
  one needs separate approval. The live runs above used 1.61.1 with Chromium 1228.
- The ticket's original three-survivor evidence files under `/private/tmp/cem10-build/
  browser-qualification` are no longer on disk (temporary-directory purge): NOT_OBSERVED here.
  They are retained only by the ticket's recorded digests and are kept separate from this evidence.
- Linux execution of the escaped-descendant path (and `/proc` table parsing on a live host): NOT_RUN.
- SIGTERM to the runner is handled by the same context cancel as SIGINT but was not separately
  live-run.
