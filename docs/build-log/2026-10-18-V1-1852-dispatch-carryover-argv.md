# V1-1852: carry-over host argv placeholders and invalid dispatch config files (issue 1157)

Ticket V1-1852, GitHub issue 1157. Requirements ATR-V0-133..135 in
[corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md), accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). This is not a new owner
decision; it applies that standing acceptance to this lane's requirements.

## Why the restriction existed

V1-1782 put untrusted prior-session output into `{prompt}` and reused the `{operatorNote}` host rule
(ON-V0-011): no interpreter, no code-string option. That check reads only the literal configured
argv. Store-derived values (`{ticket}`, `{ticketLocal}`, `{state}`, `{pool}`, `{member}`,
`{nextStage}`) are tokens, but a ticket local of `sh` or `python3`, or `-{ticketLocal}` rendering
`-ce`, would turn the launched argv into one that runs its arguments as code, and the configuration
check cannot see that. V1-1782 refused those placeholders outright, while its spec sentence said
"Other placeholders in the host argv are allowed", so code and spec disagreed.

## Decisions

- **Allow, then recheck as rendered.** Every argv placeholder is allowed after `argv[0]` with
  carry-over on. The interpreter and code-string checks run on the literal argv at load and again on
  each placeholder-bearing element as rendered at a carry-over launch. A failing launch proceeds
  without the block and alerts, so a ticket named `sh` loses carry-over, not its session.
- **Literal text only at load.** The load check runs on each element with its placeholders removed,
  so a placeholder name (the `c` and `e` in `{ticketLocal}`) is never read as option text:
  `-t{ticketLocal}` loads, `-c{ticketLocal}` and `--eval={role}` are refused, and `-{ticketLocal}`
  loads and is caught at launch when it renders `-ce`. Found by the review round on PR 1160.
- **Still refused, by name:** `{prompt}` spliced into another element, no whole `{prompt}`, any
  placeholder in `argv[0]`, a literal interpreter, a literal code-string option, `{prompt}` in an
  activity path. `{operatorNote}` and `{detachedRun}` stay out of argv; the `{operatorNote}`
  literal-argv rule is unchanged.
- **Status exit code over a new verb.** `dispatch status` already validates the file and reports
  `configFile.state` `INVALID` (CAL-V0-127). Failing it with `ERROR`/`MALFORMED`/exit 1 while keeping
  the item is the smaller change than a `dispatch validate` verb.
- **Restart: document, do not fall back.** The ledger keeps the applied configuration's digest and
  time, not its bytes, so there is no durable last-applied configuration to run on. Startup already
  refuses an invalid file before it touches the ledger; ATR-V0-135 records that a restart then
  leaves the program with no dispatcher, and ATR-V0-134 is the check that predicts it.

## Evidence

- `internal/tasks/dispatch` full package: ok. `TestATRV0133_*`, `TestATRV0110_*` included.
- `internal/tasks/cli` full package: ok. `TestATRV0134_StatusFailsOnInvalidConfigFile` plus three
  updated status tests; `TestCALV0131_*` included.
- `TestImportDirection` (`internal/tasks`): ok.
- Mutation: disabling the launch recheck made `TestATRV0133_LaunchRechecksRenderedArgv` fail with the
  block in the prompt.
- NOT_RUN: Linux, live host qualification.
