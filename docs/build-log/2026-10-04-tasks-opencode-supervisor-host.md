# Tasks supervisor: OpenCode host adapter

Native ticket V1-0756, split from owner request
[issue 354](https://github.com/beamfall/corvint/issues/354), asked for an OpenCode host for
`corvint-tasks run`. The contract is S23 of `docs/specs/corvint-tasks-agent-leases-v0.md`
(CAL-V0-076..077, allocated by the coordinator after V1-0755 took CAL-V0-074..075). It is one more entry
on the S22 host-vocabulary seam from V1-0755. Lifecycle, process-group ownership, output caps,
WAIT/resume, review independence, gates and integration are unchanged.

Source of the host contract: OpenCode 2.0.21. It was read from `opencode run --help` in an isolated
`HOME` and from the `run` module (the JSON event writer, `reportRunError`, the permission schema and
the configuration flags). No model run was made. The fake host in
`internal/tasks/store/program_opencode_host_test.go` replays the event shapes read there.

Decisions:

- Process boundary (superseded by the review fixes below). OpenCode extensions are in-process plugins loaded by its session server, not
  hooks (AHI-044). By default `opencode run` attaches to a shared background service. Every stage
  passes `--standalone`. That answers the V1-0755 build-log question of whether a plugin host can
  give the lane leader a foreground process to own and reap: it can. The owned process is
  `opencode run`. The 2.0.21 executable shows it resolving a "standalone server command" from its
  own executable path, speaking `--stdio` to it, keeping a kill signal for it, and failing with
  "Standalone server exited before reporting readiness". So the server, and the plugins it loads,
  is a child inside the stage's process group, which S10 kills and reaps. Not observed: whether
  that child is spawned detached. A detached server or plugin escaping with `setsid` stays a
  recorded residual risk. The supervisor reads only standard output and the exit status. No plugin
  or server endpoint is a supervisor channel.
- Permissions. `--auto` is never passed, so rules that would ask are rejected. An inline
  `OPENCODE_CONFIG_CONTENT` denies `task` and `external_directory` in every stage, and `edit` in
  review and integrate. `OPENCODE_DISABLE_PROJECT_CONFIG=1` keeps configuration and plugins in the
  untrusted worktree from loading. `OPENCODE_DISABLE_AUTOUPDATE=1` keeps the pinned binary from
  replacing itself. Plugins from the operator's own `HOME` configuration still load and are not
  contained, and Bash keeps OpenCode's default rules. Both limits are recorded as non-goals.
- Effort is the model variant. The config model must be one `provider/model` with no `#`, and each
  stage passes `M#effort`. Whether a provider honours the S13 variant names is unverified.
- Multi-repository programs are refused on this host, because every stage denies
  `external_directory`.
- Result vocabulary. Each line must be a qualified JSON event, and the stream must name one constant
  bounded session. There must be no `error` event, and the last `step_finish` must have reason
  `stop`, with the handoff in the last text part before it. Usage sums the disjoint counters of
  every finished step (input + cache read + cache write; output + reasoning). Otherwise it is
  NOT_OBSERVED.
- Resume (superseded by the review fixes below). `--session S` silently creates S when it no longer exists, and the answer then comes from
  a different session. The S22 `Vocabulary.Decode(raw)` seam does not receive the expected session,
  so the check sits in the store right after `supervisor.Run`. Such a stage stops with the
  resumable-handoff question instead of advancing. Its journaled result class stays the decoder's
  `EXIT_ZERO`. This is the one place the seam did not fit; the seam was not changed. A mutation run
  that disabled the check made `TestCALV0077_OpenCodeResumeRefusesFreshSession` fail.
- The sibling's tests used `opencode` as the example unknown host. They now use `gemini-cli`.

Evidence: focused tests named in the CAL-V0-076 and CAL-V0-077 traceability rows, including the
fake-host end-to-end `TestCALV0077_OpenCodeProgramFakeHost`. It covers WAIT, answer, the exact
resumed session, BUILT, the independent review with edits denied, READY_FOR_INTEGRATION, and usage
21/9 re-derived from three runs.

Not run or unmeasured: live OpenCode qualification on a disposable program (`NOT_RUN`); whether
tool-heavy stages exceed the 16 KiB output cap through verbose `tool_use` events; provider variant
support; whether inline permission precedence holds over every project configuration source; and
how OpenCode token totals compare with billed usage.

## Review fixes (Codex CHANGES_REQUIRED on 7117ded2..3516d73d)

An independent Codex review found three defects. Each was checked against the OpenCode 2.0.21
bundle (binary SHA-256 `0b2b68c1efaf20a29aaf636c2ffccc1abb56243a82f48cce45e257d232e03442`) and
confirmed.

- P1, a recreated session passed the resume check. `--session S` on a missing S calls
  `session.create({id: S})`, so the answer comes from the same ID with no history, and the
  ID-equality check accepted it. Fix: a resume passes `--session S --fork`. In the bundle, `--fork`
  on a missing S throws "Session not found" (an `error` line with an empty session, exit 1), and on
  an existing S forks it to a new ID that carries its history. The store now advances a resumed
  stage only when the decoded session is non-empty and differs from S. Otherwise the attempt stays
  `WAITING` with S kept as its resume target. `TestCALV0077_OpenCodeResumeRequiresFork` covers a
  missing session and a same-ID recreation. It reads the durable attempt back with
  `store.AttemptRecord`, independently of the returned error. That the fork carries the history is
  read from the bundle, not observed live.
- P1, the standalone server left the owned process group. The bundle's process spawner defaults to
  `detached: true`. The standalone server spawn, the `bash` tool spawn and a pty daemon do not
  override that default, and the server is killed with `process.kill(-pid)`. So the server and the
  tool processes lead groups and sessions of their own. The first delivery called this
  "not observed"; it was observable.
  - Option (a), a launch path that keeps the server in the group, does not exist: no flag or
    environment entry changes the spawn options.
  - Option (b) was taken. `Vocabulary.Detached` marks the host. Its capsule must carry
    `OPENCODE_PRINT_LOGS=1` as the last assignment of the key, which makes the server inherit the
    stage's standard error. `OPENCODE_LOG_LEVEL=ERROR` keeps that output small.
  - While the host runs, the supervisor scans `ps` every 200 ms, and once more before cleanup. It
    records, by PID and start identity, every process whose parent is in the owned group or in a
    known escape but whose group differs. An escape that does not lead its own group is
    uncertainty.
  - Cleanup drains the owned group, then each escaped group: children first, then its leader with
    `SIGTERM` and, after 5 s, `SIGKILL`.
  - A clean stop also needs end of file on both pipes within 5 s. Because the server holds standard
    error, that end of file proves no server survived, including one orphaned before any scan
    found it. `TestCALV0077_DetachedOrphanFailsClosed` covers that case.
  - Recovery (`RecoverHost`) of a detached host is proved only when a host process other than the
    leader is still alive to observe escapes through. Otherwise it fails closed.
  - Residual: a non-server escape that is orphaned between two scans and does not hold standard
    error is not detected.
  - Supervisor tests use a `perl` `setsid` fake server: timeout, forced kill of a `SIGTERM`-ignoring
    server, host crash by `SIGKILL`, the undiscovered orphan, uncertain observation, and recovery
    with the host alive or gone. A mutation that skipped the escape drain made the timeout,
    forced-kill and crash tests fail.
- P2, partial usage was reported as known. Usage summed every finished step even when a later
  `step_start` never finished, or when the output was cut at the cap on a line boundary. Usage is
  now known only when accounting is complete: no step is left open, the last finish has reason
  `stop`, and the retained output is shorter than the 16 KiB cap.
  - The cap rule sits in the vocabulary, not the workflow, because the native transition re-derives
    usage from the retained bytes. A first attempt keyed on the `OUTPUT_LIMIT` class in the
    workflow was refused `MALFORMED` ("usage must derive from retained host output").
  - Tests: `TestCALV0077_OpenCodeUsageIncompleteAccounting` covers an interrupted step, a cut
    mid-line, a cut at a line boundary, and a stream exactly at the cap.
    `TestCALV0077_OpenCodeOutputLimitUsageUnknown` checks the same end to end.

Owner decisions (2026-10-04):
1. OUTPUT_LIMIT stays at 16 KiB, and overflow fails closed.
2. Plugins from the operator's user configuration are an explicit known limit of this slice,
   recorded in the S23 non-goals and plugin boundary. They are not contained here.

Still not run: live OpenCode qualification, including the fork resume, escape draining against
the real server, stderr volume at error level, and the cost of `ps` scans on a busy host
(`NOT_RUN`).
