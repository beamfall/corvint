# Claude Code integration matrix (host lane, 2026-10-23)

Date: 2026-10-23. Host: Claude Code 2.1.293 (`claude --version`). Base: origin/main
`368b80d438669f618ac4cf1884ac19dd65fbc4a5`. Branch: `claude/host-claude-1023`. Tickets: V1-0403
(Stop done-list), V1-2466 / GitHub #1588 (plugin MCP declaration), V1-2440 (heavy-target
priority), V1-1563 (live dispatch context estimate, Claude half). Requirements: `AHI-190` to
`AHI-199` in [agent-harness-integration-v0](../specs/agent-harness-integration-v0.md).

Cell values: QUALIFIED means a live receipt from the installed host exists. DELIVERED means code and
tests exist but there is no live receipt on 2.1.293. The other values are GAP, NOT_APPLICABLE (with
a reason) and NEEDS-OWNER. Extension points come from the installed host: plugin hooks per event,
plugin skills, plugin `.mcp.json`, plugin agents, the user `statusLine` setting, plugin
`userConfig`, plugin `lspServers` and skill `allowed-tools` permissions.

## Matrix

| Corvint capability | Claude Code extension point | Before | After | Evidence |
|---|---|---|---|---|
| Session start context, dogfood argv and Tasks projection | `SessionStart` hook | DELIVERED | DELIVERED | `hooks.json`, `AHI-045`..`047` tests |
| Prompt context | `UserPromptSubmit` hook | DELIVERED | DELIVERED | V1-2440 timing below (CLI, not a live session) |
| Edit observation | `PostToolUse` Edit/Write/NotebookEdit | DELIVERED | DELIVERED | `hooks.json` |
| Pre-tool guard | `PreToolUse` | NOT_APPLICABLE | NOT_APPLICABLE | no accepted Frontier authority can block (README) |
| Completion check | `Stop` hook | DELIVERED | DELIVERED | `AHI-021` |
| Tests, witness and mutation still owed (V1-0403) | second `Stop` hook, `/corvint:done` skill | GAP | DELIVERED | `AHI-190`..`197`; direct hook timing below |
| Session end | `SessionEnd` hook | DELIVERED | DELIVERED | `hooks.json` |
| Compaction pin | `PreCompact` / `PostCompact` | QUALIFIED (2.1.267) | QUALIFIED (2.1.267) | `compatibility.json` `compactionHooks`; 2.1.293 NOT_RUN |
| Subagent context | `SubagentStart` hook | GAP | GAP | V1-0401 (existing) |
| affected, context, impact, review, status, index | plugin skills | DELIVERED | DELIVERED | `AHI-051` |
| Read-only MCP tools `corvint.help`, `status`, `query`, `impact` (V1-2466) | plugin `.mcp.json` | GAP | QUALIFIED (connect) | `AHI-198`; receipts below |
| Docs corpus and test-validity MCP tools | plugin `.mcp.json` | GAP | GAP | V1-2513: servers refuse the 2025-11-25 initialize |
| Other read verbs (flows, docs, features, overview, test-plan, witness, prove) | Bash and the `corvint.help` catalog | GAP (discovery) | DELIVERED | `corvint.help` returns the capability catalog |
| Skill permissions | skill `allowed-tools` | DELIVERED | DELIVERED | each skill pre-approves only its own argv |
| Done-list opt-in at install time | plugin `userConfig` | GAP | GAP | V1-2514 |
| Change state at a glance | user `statusLine` | GAP | GAP | V1-2517; the plugin cannot set it, so it stays NEEDS-OWNER to enable |
| Plugin agents | plugin `agents/` | NOT_APPLICABLE | NOT_APPLICABLE | the skills already wrap each verb; an agent adds no capability |
| Go language server | plugin `lspServers` | NOT_APPLICABLE | NOT_APPLICABLE | `corvint-lsp` is experimental and Go-only; covered by the LSP quality tickets |
| Hook latency under lane test load (V1-2440) | Makefile and `script/heavy.sh` | GAP | DELIVERED | `AHI-199`; measurements below |
| Live dispatch context estimate (V1-1563) | live session | GAP | GAP | NOT_RUN: no authenticated model turn |
| Live model-turn qualification on 2.1.293 | live session | NEEDS-OWNER | NEEDS-OWNER | V1-2515: `claude -p` reported expired OAuth |

Counts over 21 rows:

| Value | Before | After |
|---|---|---|
| QUALIFIED | 1 | 2 |
| DELIVERED | 7 | 10 |
| GAP | 9 | 5 |
| NOT_APPLICABLE | 3 | 3 |
| NEEDS-OWNER | 1 | 1 |

Five GAPs remain. Three have tickets filed in this lane (V1-2513, V1-2514, V1-2517). One has an
existing ticket (V1-0401). V1-1563 stays open.

## Live receipts (Claude Code 2.1.293)

All runs used an isolated `CLAUDE_CONFIG_DIR` and `--plugin-dir` against the branch plugin. The
user's settings and installed plugins were not touched. Binaries were built from the branch:
`corvint-mcp` sha256 `19f7e1e8…305e`.

- **MCP connect:** `claude --plugin-dir <plugin> mcp list` in a scratch repository printed
  `plugin:corvint:corvint: corvint-mcp --root <repo> --protocol-version 2025-11-25 - ✔ Connected`.
  Receipt sha256 prefix `b3dcde64669220730d82`.
- **Linked worktree:** the same check run from a linked worktree of the scratch repository printed
  `✔ Connected`.
- **corvint-mcp missing from PATH:** the check printed `✘ Failed to connect` with `ENOENT`. Hooks
  are unaffected.
- **Plugin validation:** `claude plugin validate` printed `✔ Validation passed`.
- **corvint.help over stdio:** a 2025-11-25 `initialize`, then `tools/list` (4 tools: corvint.help,
  corvint.impact, corvint.query, corvint.status), then `tools/call corvint.help`. The call returned
  `isError: false` with the `corvint-capability-catalog/0` payload. Transcript sha256 prefix
  `eff8c52117db6db4f959`.
- **Default protocol refuses initialize:** without `--protocol-version`, corvint-mcp answers
  `initialize` with -32601. That is why the plugin pins the legacy version (V1-2507, existing).
- **NOT_RUN:**
  - Any model turn: `claude -p` reported expired OAuth.
  - A done-list systemMessage at a real Stop.
  - A model-invoked `corvint.*` call.
  - V1-1563.

## Stop done-list timing (direct, lane worktree)

`CORVINT_STOP_DONE_LIST=1 corvint adapter claude-code stop-done-list` on the dirty lane worktree,
3 runs: 2.21 s, 1.39 s and 1.37 s wall time, against the 9.6 s deadline (`AHI-194`). The first run
was cold. The message listed 103 selected units, with witness and mutation `NOT_RUN` and the exact
argv to run after committing.

## V1-2440 measurements (`AHI-199`)

Each row is 50 consecutive `corvint adapter claude-code user-prompt` calls. They ran while 3 or 6
package test runs were going: `cmd/corvint`, `internal/tasks/store` and `cmd/corvint-tasks`, plus
`internal/tasks/service`, `internal/witness` and `cmd/corvint-mcp` for 6. Every row had 0 calls
over 2 s and 0 `dogfood-event-deadline` fallbacks.

| Load | Mode | Load average | min / p50 / p95 / max ms |
|---|---|---|---|
| 3 runs | `HEAVY_NICE` 10 | about 10-12 | 378 / 452 / 669 / 1123 |
| 3 runs | nice off | about 19 | 358 / 448 / 652 / 939 |
| 6 runs | `HEAVY_NICE` 10 | about 15 | 386 / 489 / 653 / 999 |
| 6 runs | nice off | about 14 | 355 / 435 / 725 / 1248 |

The ticket's acceptance bound holds both with and without priority. This host did not reproduce
the load-40 regime the ticket describes, so these runs do not show that the priority change makes
a difference. V1-2440 stays open for that reproduction.

## Independent review

One Codex read-only review round (`codex exec -s read-only`) over the staged diff reported three
medium findings. All three are fixed in this change:

- A mutation attack whose mutants were all uncompilable or skipped was labelled `KILLS_NO_MUTANT`.
  It is now `NOT_RUN` with `no-mutant-executed` (`AHI-197`,
  `TestAHI197DoneListVerdictNeedsExecutedMutants`).
- The receipt's verbs each resolve HEAD, so a commit made mid-run could mix two ranges. The
  receipt is now bound to the HEAD read before any verb ran and refused as `head-moved` otherwise
  (`AHI-196`, `TestAHI196DoneListRefusesWhenHeadMoves`).
- The AHI-199 shell test inherited `HEAVY_NICE` and assumed a caller at priority 0. It now clears
  the variable and expects the caller's priority when that is higher than 10.

A second Codex review of PR #1659 at `d9cbe2029` reported three findings, fixed in a follow-up
commit without a dogfood rebind, which is left to the owner (`AHI-196`):

- P1: the no-base status scan ran a raw `git status`, so a configured clean or process filter could
  run during the read-only Stop hook. It now uses `gitstatus.Status`, as `affected` does
  (`TestAHI196DoneListRunsNoRepositoryFilter`; reverting to the raw status makes that test fail on
  the filter's marker file).
- P2: two failed HEAD probes compared equal as "". A probe must now name a full commit identifier
  both times; otherwise the receipt is refused with `head-unavailable`
  (`TestAHI196DoneListRefusesWithoutHead`).
- P2: the receipt recorded `base` but not the commit it describes. It now carries the validated
  commit as `head` (`TestAHI196DoneListReceiptNamesHead`).
