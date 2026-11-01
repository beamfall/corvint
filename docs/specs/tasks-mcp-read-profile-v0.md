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
- Claim: `corvint-tasks mcp` is a local read-only stdio MCP server whose five typed, paged tools answer with the Corvint Tasks CLI read verbs' own envelopes.
- Status: experimental under the standing lane auto-accept (V1-3707, 2026-11-01) / experimental
- Exists: `internal/mcp/tasksread` (`New`, `Handler`, the five tools), `internal/tasks/cli/mcp.go` (`mcpCommand`), `internal/tasks/cli/mcp_test.go`.
- Blocked on: no host plugin declares the server yet; the write profile is out of scope (see Non-goals).
- Read next: Requirements; Non-goals and failure modes.

## Owner intent and scope

An agent reading the Corvint Tasks queue today shells out to `corvint-tasks` and parses its JSON
envelope; `corvint-tasks mcp` was an unknown verb (V1-3707). This profile is the smallest MCP
surface for those reads: a read-only stdio server inside the `corvint-tasks` binary whose tools run
the existing read verbs in-process, so the CLI stays the one authority for every answer and the
server holds no state of its own. Writes (claim, heartbeat, submit, complete) need destructive
annotations, idempotency keys and lease echo, and belong in a separate opt-in profile under their
own spec.

## Requirements

- `TMR-V0-950`: `corvint-tasks mcp [--protocol-version 2026-07-28|2025-11-25]` MUST serve one stdio
  MCP server through `internal/mcp/server`, `2026-07-28` by default, named `corvint-tasks-mcp`
  with the binary's version, under `NameLegacyFlag` (`MCPV0-820`, `MCPV0-954`), until stdin closes
  or SIGINT/SIGTERM arrives. Any other argument exits 2 with `corvint-tasks mcp: invalid
  arguments` before serving. It answers in the process's own directory, as every other verb does.
- `TMR-V0-951`: `tools/list` MUST list exactly `corvint.tasks.queue_status`,
  `corvint.tasks.receipt_audit`, `corvint.tasks.roadmap`, `corvint.tasks.ticket_search` and
  `corvint.tasks.ticket_show`, sorted, each with a closed JSON Schema 2020-12 `inputSchema`
  (`additionalProperties: false`), an `outputSchema` requiring `profile`, and the read-only
  annotations (`readOnlyHint` true, `destructiveHint` false, `idempotentHint` true,
  `openWorldHint` false). The paged tools take `offset` (integer, minimum 0) and `limit` (integer,
  1 to the CLI's page maximum); `ticket_search` adds `status` and `kind` enums from the ticket
  package, `priority` `P0`..`P4`, and bounded `owner`, `milestone`, `label` and `text`; `ticket_show`
  requires `ticket`, a full ticket ID or a local ID.
- `TMR-V0-952`: Each call MUST run its verb (`queue status`, `receipt audit`, `roadmap`, `ticket
  search`, `ticket show ID`) with each argument as the same-named flag, through the CLI's own
  `Run`, and return its `taskman-command-result/0` envelope as `structuredContent`, with `page`
  (`offset`, `limit`, `total`, `truncated`) telling the caller whether to fetch the next offset,
  and its stdout bytes as the one text block inside the `AHI-004` untrusted-data envelope. A verb
  that does not answer `OK` is an `isError` result `corvint-tasks-mcp-tool-error/0` with code
  `CLI_REFUSED`, the CLI's `codes`, and its envelope. Every tool reads only: a call takes no store
  lock and writes nothing.
- `TMR-V0-953`: A schema-invalid argument, an unknown tool, an unknown param or a non-object
  `arguments` MUST be `-32602` with `MCPV0-952` data (`reason`, `field`, and `allowed` or `rule`),
  before any verb runs; a non-empty `tools/list` cursor is `invalid-cursor`.
- `TMR-V0-954`: The CLI help MUST list `corvint-tasks mcp` with usage, flags and an example, and
  its command tree node MUST be in `internal/capcatalog/trees/corvint-tasks.json`; `docs/CAPABILITIES.md`
  names the verb and its five tools.

## Non-goals and failure modes

- No write tools (claim, renew, heartbeat, submit, gate run, complete, ticket mutations), no
  attempt or lease reads, and no cursor paging beyond the CLI's `offset`/`limit`.
- No host plugin wiring and no `corvint mcp` group: a host launches `corvint-tasks mcp` itself.
- No second source of truth: a tool whose verb changes its envelope changes with it.
- Failure mode: a store the CLI cannot read answers each call `CLI_REFUSED` with the CLI's codes;
  the server keeps serving.
- Failure mode: an envelope carrying the untrusted-data terminator is refused with the
  `AHI-004` collision code, never emitted.

## Acceptance evidence

`internal/tasks/cli/mcp_test.go`'s `TestTMRV0950ReadOnlyTasksMCPProfile` (five descriptors with
closed schemas and read-only annotations; search and roadmap pages; show and queue status OK;
the enveloped text is the CLI's bytes; an unknown ticket is `CLI_REFUSED` with the CLI's codes; a
bad enum, an unknown tool and an out-of-range limit are `-32602` with data; the queue is unchanged)
and `TestTMRV0950InvalidArguments`; the `corvint-tasks` tree snapshot and `tools/capabilities-check`.

## Rollback

Remove `internal/mcp/tasksread`, `internal/tasks/cli/mcp.go`, the `mcp` case in `Run`, its `ReadVerbs`, `readOnlyVerbs` and
`commandUsage` entries and help line, and regenerate the command tree. No stored state depends on
the server.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| `TMR-V0-950` | `mcpCommand`, the `mcp` case of `Run` | `TestTMRV0950InvalidArguments`; `TestTMRV0950ReadOnlyTasksMCPProfile` (2025-11-25 handshake) |
| `TMR-V0-951` | `internal/mcp/tasksread` (`tasksMCPTools`, `tasksMCPTool.descriptor`, `pageProperties`) | `TestTMRV0950ReadOnlyTasksMCPProfile` (descriptor checks) |
| `TMR-V0-952` | `internal/mcp/tasksread` (`Handler.call`, `toolFailureResult`), the `Run` callback in `mcpCommand` | `TestTMRV0950ReadOnlyTasksMCPProfile` (pages, enveloped CLI bytes, `CLI_REFUSED`, unchanged queue) |
| `TMR-V0-953` | `internal/mcp/tasksread` (`tasksMCPTool.argv`, `Handler.Handle`) | `TestTMRV0950ReadOnlyTasksMCPProfile` (enum, unknown tool, range) |
| `TMR-V0-954` | `commandUsage["mcp"]`, `helpResult`, `internal/capcatalog/trees/corvint-tasks.json`, `docs/CAPABILITIES.md` | `TestCAPV0046HelpJSONIsTheTree`, `tools/capabilities-check` |
