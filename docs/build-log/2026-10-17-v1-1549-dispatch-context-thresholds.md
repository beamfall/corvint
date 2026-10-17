# 2026-10-17 V1-1549: dispatch context thresholds (issue 1007)

Decision. A dispatch role may declare `context: {warnTokens, handoffTokens, hardTokens}`
(`CAL-V0-340`..`CAL-V0-346` in `docs/specs/corvint-tasks-agent-leases-v0.md`). The requirements
were accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

Design:

- The estimate comes only from recognised host events, read in the existing incremental usage read
  (`CAL-V0-157`). For Claude Code, it is the latest main-session `assistant` event's
  `message.usage` (input plus both cache counters plus output). Subagent events carry a non-null
  `parent_tool_use_id` and are ignored. For opencode, it is the latest `step_finish` input plus
  output. Codex reports only cumulative turn totals, so a Codex role cannot declare `context`.
  A recognised line that cannot be read, or a line too long to read, makes the estimate UNKNOWN.
  Nothing is inferred from text, and the `result` total is not a context figure.
- The state lives in a per-worker sidecar, `workers/<id>/context.json` (profile
  `taskman-dispatch-context/0`), not in the ledger. The ledger stays at
  `taskman-dispatch-state/5`, which leaves the ledger and `keeper.go` free for the concurrent
  lease-expiry work (PR #1014). The sidecar is the worker's notice through
  `CORVINT_DISPATCH_CONTEXT_FILE`. It also records each crossing once, so a restart reloads the
  crossings and never re-reports them. The only shared lines are one `EventKinds` entry
  (`context`) and the kill-reason switch in `supervise`.
- Thresholds come from the applied configuration, not the worker's launch configuration. This is
  the one recorded exception to `CAL-V0-127`, so lowering a threshold applies to running workers
  at the next tick.
- The hard stop uses the existing stop-and-heal path. The kill reason is `CONTEXT`, and the
  hand-off event carries `reason`, `contextTokens` and `hardTokens`. `hardTokens` requires
  `heal.handoff`. An UNKNOWN estimate never stops a worker.
- A write failure keeps the estimate, which still decides the hard stop. It records no crossing
  and raises one alert per run of failed writes. The temporary file name is random, so the alert
  is not keyed per error text. That change was made during testing and recorded in `CAL-V0-343`.

Evidence:

- Focused tests trace every requirement (`TestCALV0340`..`TestCALV0346`):
  - 7 tests in `internal/tasks/dispatch`
  - 1 test in `internal/tasks/cli`
- The whole `internal/tasks/dispatch` package was run once. Its only failure was a `t.TempDir`
  cleanup race in `TestIssue502_InfraSessionKeepsTierAndResetsSuffix`, which passed on three
  isolated reruns. That test declares no usage format, so this change is inert for it.

NOT_RUN: there was no live qualification against a real Claude Code or opencode session. The
figures come from the hosts' documented stream-JSON fields, exercised through fixtures.

Residuals:

- Codex has no per-call estimate.
- No `corvint-tasks` verb surfaces the notice. The worker reads the record file.
- The detached-run hand-off path does not carry the `CONTEXT` detail.

Rollback: remove `context` from the role configurations, which disables the feature. To remove it
completely, revert the change. Leftover `context.json` sidecars are inert.
