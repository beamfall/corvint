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

- Process boundary. OpenCode extensions are in-process plugins loaded by its session server, not
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
- Resume. `--session S` silently creates S when it no longer exists, and the answer then comes from
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
