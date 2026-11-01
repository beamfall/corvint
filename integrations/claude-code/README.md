# Corvint for Claude Code

This local marketplace contains the `corvint@corvint` plugin, which works in the Claude Code terminal
and in the Code tab of the Claude desktop app. Its support level is intentionally `FALLBACK`: the formal harness hooks
translate only fields exposed by Claude Code into `corvint-harness-event/0`. The plugin declares
Corvint's experimental local stdio MCP server (`corvint mcp`, `AHI-198`, `AHI-373`) so the read-only
`corvint.*` tools, including `corvint.help`, are reachable in-session; official-schema execution and
official MCP conformance remain `NOT_RUN`. No accepted Frontier authority can block or continue a
Stop event.

Prerequisite: `corvint` is on `PATH` (see [Try it](../../README.md#try-it-in-sixty-seconds)), and the repository is
trusted in Claude Code.
The plugin does not install dependencies, start a daemon, read transcripts, fetch from the network,
or inherit secrets into the Corvint child process.

Native lifecycle, run from the root of a Corvint checkout:

```text
claude plugin marketplace add ./integrations/claude-code
claude plugin install corvint@corvint --scope user
claude plugin list
claude plugin details corvint@corvint
claude plugin disable corvint@corvint --scope user
claude plugin enable corvint@corvint --scope user
claude plugin uninstall corvint@corvint --scope user
claude plugin marketplace remove corvint
```

Commands (skills, `AHI-051`). Each runs existing verbs and reports their uncertainty. Only
`/corvint:context`, inside an enrolled change workflow, and `/corvint:index` write state:

| Command | Runs | Started by |
|---|---|---|
| `/corvint:context` | `query`, `impact`, `feature` and the enrolled workflow | user or model |
| `/corvint:impact [BASE \| PATH...]` | `corvint impact` | user or model |
| `/corvint:affected [BASE]` | `corvint affected` | user or model |
| `/corvint:review [BASE]` | `corvint review` | user or model |
| `/corvint:status` | `--version`, `observations`, keyed `dogfood status` | user only |
| `/corvint:index` | `corvint index --if-stale` (writes only the derived snapshot) | user only |
| `/corvint:done [BASE]` | `corvint adapter done-list` (with a base, `--mutate` runs the new tests against mutants) | user only |

Hooks add no routine notices to the conversation (decision 0161); their context goes to the model
only. `/corvint:status` is the on-demand view of versions, index freshness, workflow state and
recent adapter degradations.

The done-list (`AHI-190` to `AHI-197`) answers "what does this change still owe": the tests
`corvint affected` selects, the ranges `corvint witness` leaves unwitnessed, and a mutation verdict
per new test (`KILLS_MUTANTS`, `KILLS_NO_MUTANT` for a test that asserts nothing, or `NOT_RUN` with a
reason). Set `CORVINT_STOP_DONE_LIST=1` in the settings `env` to also print it, to the user only, at
the end of each main-thread turn on a dirty tree. The Stop hook runs only `affected` within its
10-second bound; witness and mutation need a committed range and minutes of test time, so the hook
names the exact `corvint adapter done-list --base <HEAD> --mutate` argv instead of running them.

MCP (`AHI-198`, `AHI-373`): `.mcp.json` starts `corvint mcp --root ${CLAUDE_PROJECT_DIR}
--protocol-version 2025-11-25` (the initialize-handshake protocol Claude Code speaks), served by the
same `corvint` binary the hooks use, so no second executable is needed. A `corvint` older than the
`mcp` verb makes `/mcp` and `claude mcp list` show the server as failed; the hooks and skills are
unaffected. Disable the server alone from `/mcp`. When `${CLAUDE_PROJECT_DIR}` names a directory
that no longer exists (a Claude desktop spare-claimed session can pass its deleted
`~/.claude/spares/spare-*` directory), the server serves the Git worktree enclosing the directory
it was started in instead, prints `corvint-mcp: --root does not exist; serving the Git worktree
enclosing the launch directory instead` to its log, and adds a `missing-root-launch-worktree`
notice block to every tool result (`MCPV0-115`). When `${CLAUDE_PROJECT_DIR}` names an existing
directory outside Git (a spare directory not yet deleted) or arrives unexpanded, the server still
connects and lists its tools, prints `corvint-mcp: repository unavailable: <reason>` to its log,
and every repository tool call answers `invalid-root` (`MCPV0-950`, `AHI-860`). Restart the session
from the intended repository if that worktree is the wrong one or the tools answer `invalid-root`.

If `/mcp` shows the server failed with `CONNECTION_CLOSED`, Claude Code keeps the server's reason
in its MCP log (`~/Library/Caches/claude-cli-nodejs/<project>/mcp-logs-plugin-corvint-corvint/` on
macOS), not in the session. A `corvint` built before `MCPV0-950` exits on an unusable root, which
the host reports only as a closed connection: rebuild or update `corvint` on `PATH`, run
`claude plugin marketplace update corvint` and `claude plugin update corvint@corvint`, then start a
new session.

Use `claude --plugin-dir ./integrations/claude-code/plugins/corvint` for an installation-free local
check and `/hooks` to inspect the registered hooks. Corvint has no exact-handle expansion command yet
(`exact-expansion-command-unavailable`). Plugin changes load after `/reload-plugins` or the next
session.

`SessionStart`, `UserPromptSubmit`, `Stop`, and `SessionEnd` use the separate native
`corvint-dogfood-event/0` profile. The session key hashes the raw session ID with the
Claude-specific domain `corvint-local-completion-session/claude-code/0`; it does not inherit a Codex
session. A handoff must retain the original worktree and explicit enrollment key in every command.
`PostToolUse` retains the formal `corvint-harness-event/0` fallback profile. The plugin registers no
`FileChanged` hook: Claude Code watches only the file names a `FileChanged` matcher lists, so a
matcherless registration never fires, and Edit/Write changes already reach `PostToolUse`.

An existing current snapshot is read when available; the bounded in-memory cold fallback may
succeed within the hook deadline. The experimental V1-1128 implementation (proposed, pending owner
acceptance; `IDX-SNAP-V0-031` through `IDX-SNAP-V0-036`, `AHI-053`) starts a separate bounded
`corvint index --if-stale` writer when an event reports a stale snapshot. One background writer
per shared store runs at lowered priority and exits within its 60-second wall bound. The current
hook still fails open and retains the manual refresh argv; a started notice is not completion.
Set `CORVINT_INDEX_AUTO_REFRESH=0` to retain manual-only refresh. Forced SIGKILL cleanup must target
the writer's process group: killing only its PID cannot run descendant cleanup. This proposal
supersedes the accepted no-detached-refresh clause only once the owner accepts it; it does not
qualify the unresolved automatic-refresh performance contract.

Prompt context is produced once by the native event and preserves separate governance, declared
scope and current-task evidence. The adapter validates the full receipt, then injects only its
actionable `corvint-hook-context/0` projection (AHI-045) as untrusted repository data: evidence rows
with blob pins, governance, non-current scope, named omissions, new degradations, policy and the
compaction block, without digests, coverage counters or repository and adapter metadata. A
`UserPromptSubmit` or subagent `SessionStart` with nothing actionable injects nothing (AHI-046).
Trusted status and warmup guidance (the workflow argv, rendered outside the envelope from validated
root/key values) is emitted only at a main-thread `SessionStart`, including `source=compact`
(AHI-047), and only in the Corvint repository or a worktree already enrolled in dogfood (AHI-230). An older plugin needs no change: the projection is rendered by the installed binary.
The default prompt hook does not make a second experimental `context` call. The opt-in self-use
handoff experiment captures its packet once and passes that same packet to the legacy projection.

Explicitly enrolled incomplete work can block the first `Stop` with one remediation continuation.
An already-active recursive Stop releases with the unresolved state; it cannot loop. Inactive and
cancelled enrollment never means satisfied. `SessionEnd` is observational because the host exposes
no explicit Corvint outcome and discards its output. This local completion policy carries no Frontier
harness authority and is not execution attestation. See the context skill for the complete workflow.

The host payload is limited to 8 MiB and the normalized request to 131,072 bytes. The installed
`corvint` process owns normalization, native event evaluation, and host rendering under the hook
deadline without an adapter subprocess. Complete host output, including framing, trusted guidance,
escaping and its trailing newline, is capped at 8,000 bytes.
Malformed responses, unsupported sources and budgets produce visible host-valid degradation.
Prompt text above 2,000 Unicode characters or 16,384 UTF-8 bytes is served by a query over the
explicit anchors it names, with a trusted disclosure (`AHI-016`); only an over-bound prompt with no
usable anchor is refused before Corvint is invoked.
A file-change path outside the repository or untracked at HEAD cannot become revision-pinned evidence.
After Claude Code compacts a session, `SessionStart(source=compact)` rehydrates a bounded impact
packet for the dirty paths tracked at the pinned revision; untracked paths remain a count/digest
gap. The adapter never reads the transcript.
`PreCompact` and `PostCompact` (decision 0340) add a pin around that cycle: before compaction the
adapter prints one `corvint-compaction-pin/0` line (revision, dirty-path counts, at most 24 tracked
dirty paths) that the host joins into the compactor's instructions; after compaction it re-reads
the pin from the summary, verifies the pinned tree and paths against the object store with one
read-only `git cat-file`, and prints a `corvint-compaction-report/0` line naming every
non-rehydratable path. Claude Code shows that report to the user only; the compact `SessionStart`
packet stays the model-facing rehydration, and it now opens with a disclosure saying so. The hook
names and payloads were read from the installed Claude Code 2.1.267; a host without these events
ignores the registration silently, and no live compaction cycle has been run against them.

The shipped declaration `plugins/corvint/compatibility.json` records the static validation at
package build; static validation is not black-box conformance. The published matrix
(`../compatibility.json`) records the later installed-lifecycle PASS for Claude Code `2.1.293` with
adapter `0.3.1` on darwin-arm64 against Corvint 1.0.0-rc.2; the installed-lifecycle run against
1.0.0 is pending its qualification. The compaction hooks remain statically
verified against `2.1.267` only; a live compaction cycle is `NOT_RUN`.
