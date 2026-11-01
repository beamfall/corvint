# Tasks MCP read profile V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `TMR-V0`
Intent status: experimental under the standing lane auto-accept (V1-3707, 2026-11-01)
Delivery status: experimental
Authoritative inputs: AGENTS.md invariants 4 and 7; `mcp-server-2026-07-28-v0.md` (`MCPV0-007`,
`MCPV0-820`, `MCPV0-952`, `MCPV0-953`); `agent-harness-integration-v0.md` `AHI-004`; the Corvint
Tasks CLI read verbs `queue status`, `roadmap`, `ticket search`, `ticket show` and `receipt audit`
(`TM-V0-008`).

## Agent digest
- Claim: The `corvint mcp` tasks group: five typed, paged read-only tools that run the installed `corvint-tasks` read verbs and return their envelopes.
- Status: experimental under the standing lane auto-accept (V1-3707, 2026-11-01) / experimental
- Exists: `internal/mcp/tasksread` (`Group`, `New`, the five tools), the `tasks` group token in `internal/mcp/cli`.
- Blocked on: no host plugin selects the group yet; the write profile is out of scope (see Non-goals).
- Read next: Requirements; Non-goals and failure modes.

## Owner intent and scope

An agent reading the Corvint Tasks queue today shells out to `corvint-tasks` and parses its JSON
envelope; `corvint-tasks mcp` was an unknown verb (V1-3707). This profile is the smallest MCP
surface for those reads. Decision 0397 keeps Tasks free of Core imports and lets Core import only
the Tasks wire package, so the profile is a Core-side tool group of `corvint mcp`
(`MMR-V0-012`) that runs the installed `corvint-tasks` binary as a subprocess: the CLI stays the
one authority for every answer, and the server holds no Tasks state or code of its own. Writes
(claim, heartbeat, submit, complete) need destructive annotations, idempotency keys and lease
echo, and belong in a separate opt-in profile under their own spec.

## Requirements

- `TMR-V0-950`: (amends `MMR-V0-012`) `--tool-profile` MUST admit the group token `tasks`, alone
  or beside the other groups and at most one code profile. Selected, it adds five tools per
  declared root, routed by alias in multi-root mode like the other per-root groups
  (`MMR-V0-013`), and an unusable single root lists them and refuses each `invalid-root`
  (`MCPV0-950`). The group links no Tasks package but `internal/tasks/wire`, and `internal/mcp/tasksread`
  links no repository-reading package (`toolgroup.Tasks` adapts its descriptors).
- `TMR-V0-951`: The group MUST list exactly `corvint.tasks.queue_status`,
  `corvint.tasks.receipt_audit`, `corvint.tasks.roadmap`, `corvint.tasks.ticket_search` and
  `corvint.tasks.ticket_show`, each with a closed JSON Schema 2020-12 `inputSchema`
  (`additionalProperties: false`), the read-only annotations (`readOnlyHint` true,
  `destructiveHint` false, `idempotentHint` true, `openWorldHint` false) and the `MCPV0-953`
  `outputSchema`. The paged tools take `offset` (integer, minimum 0) and `limit` (integer, 1 to
  the wire page maximum); `ticket_search` adds bounded `status` and `kind` words, `priority`
  `P0`..`P9`, and bounded control-free `owner`, `milestone`, `label` and `text`; `ticket_show`
  requires `ticket`, a full ticket ID or a local ID with no leading dash.
- `TMR-V0-952`: Each call MUST run `corvint-tasks` from `PATH` with its verb (`queue status`,
  `receipt audit`, `roadmap`, `ticket search`, `ticket show ID`) and each argument as the
  same-named flag, in the bound root, with `CORVINT_TASKS_READ_ONLY=1` so the CLI itself refuses
  any write, a 60-second bound and a 512 KiB output bound. An `OK` `taskman-command-result/0`
  envelope is the `structuredContent`, with `page` (`offset`, `limit`, `total`, `truncated`)
  telling the caller whether to fetch the next offset, and its stdout bytes are the one text
  block inside the `AHI-004` untrusted-data envelope. Any other outcome is an `isError` result
  `corvint-tasks-mcp-tool-error/0` with code `CLI_REFUSED`, the CLI's `codes` and its envelope; a
  missing or failed executable is `TASKS_UNAVAILABLE` and unreadable output
  `TASKS_OUTPUT_UNREADABLE`.
- `TMR-V0-953`: A schema-invalid argument MUST be `-32602` with `MCPV0-952` data (`reason`,
  `field`, and `allowed` or `rule`) before any process starts.
- `TMR-V0-954`: The `corvint-mcp` usage MUST name the `tasks` group, and `docs/CAPABILITIES.md`
  names its five tools.

## Non-goals and failure modes

- No write tools (claim, renew, heartbeat, submit, gate run, complete, ticket mutations), no
  attempt or lease reads, and no cursor paging beyond the CLI's `offset`/`limit`.
- No `corvint-tasks mcp` verb: the Tasks binary stays free of MCP code (decision 0397).
- No host plugin wiring: a launcher adds `tasks` to its `--tool-profile` list.
- No second source of truth: a tool whose verb changes its envelope changes with it, and the
  closed status, kind and priority values are the CLI's, which refuses others as `CLI_REFUSED`.
- Failure mode: no `corvint-tasks` on `PATH` answers each call `TASKS_UNAVAILABLE`; the server and
  its other tools keep serving.
- Failure mode: an envelope carrying the untrusted-data terminator is refused with the `AHI-004`
  collision code, never emitted.

## Acceptance evidence

`internal/mcp/tasksread/tasksread_test.go`'s `TestTMRV0951FiveReadOnlyDescriptors` and
`TestTMRV0952CallsRunTheReadVerb` (argv mapping, root, OK, `CLI_REFUSED` with codes,
`TASKS_UNAVAILABLE`, `TASKS_OUTPUT_UNREADABLE`, five `-32602` cases, nothing run on a refusal);
`internal/mcp/cli/tasks_group_test.go`'s `TestTMRV0950TasksGroupRunsTheInstalledReadVerb` (a
fake `corvint-tasks` on `PATH` run in the bound toplevel of a subdirectory root with
`CORVINT_TASKS_READ_ONLY=1`; an unusable root refuses `invalid-root`); `internal/tasks`'s
`TestImportDirection`; `tools/capabilities-check`.

## Rollback

Remove `internal/mcp/tasksread`, the `tasks` group token and its `openGroups` and
`startUnavailable` cases, and the usage words. No stored state depends on the group.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| `TMR-V0-950` | `toolGroups`, `openGroups`, `startUnavailable` (`internal/mcp/cli`), `toolgroup.Tasks`, `tasksread.New` | `TestTMRV0950TasksGroupRunsTheInstalledReadVerb`; `TestImportDirection` |
| `TMR-V0-951` | `internal/mcp/tasksread` (`tools`, `tool.descriptor`, `pageProperties`, `Group.Descriptors`), `toolgroup.Tasks.Tools` (annotations) | `TestTMRV0951FiveReadOnlyDescriptors` |
| `TMR-V0-952` | `internal/mcp/tasksread` (`Group.Call`, `Failure`, `runInstalled`) | `TestTMRV0952CallsRunTheReadVerb`, `TestTMRV0950TasksGroupRunsTheInstalledReadVerb` |
| `TMR-V0-953` | `internal/mcp/tasksread` (`tool.argv`) | `TestTMRV0952CallsRunTheReadVerb` |
| `TMR-V0-954` | `usage` (`internal/mcp/cli`), `docs/CAPABILITIES.md` | `tools/capabilities-check` |
