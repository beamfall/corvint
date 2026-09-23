# Corvint for Claude Code (developer preview)

This local marketplace contains the `corvint@corvint` plugin. It is intentionally `FALLBACK`: the formal harness hooks
translate only fields exposed by Claude Code into `corvint-harness-event/0`. Corvint ships an
experimental local stdio MCP server (`corvint-mcp`), but this plugin does not declare it;
official-schema execution and official MCP conformance remain `NOT_RUN`. No accepted Frontier
authority can block or continue a Stop event.

Prerequisite: `corvint` is on `PATH` (see [Try it](../../README.md#try-it-in-a-minute)), and the repository is
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

Use `claude --plugin-dir ./integrations/claude-code/plugins/corvint` for an installation-free local
check, `/hooks` to inspect the registered hooks, and `/corvint:context` for an explicit query. Corvint
has no exact-handle expansion command yet (`exact-expansion-command-unavailable`). Plugin changes
load after `/reload-plugins` or the next session.

`SessionStart`, `UserPromptSubmit`, `Stop`, and `SessionEnd` use the separate native
`corvint-dogfood-event/0` profile. The session key hashes the raw session ID with the
Claude-specific domain `corvint-local-completion-session/claude-code/0`; it does not inherit a Codex
session. A handoff must retain the original worktree and explicit enrollment key in every command.
`PostToolUse` retains the formal `corvint-harness-event/0` fallback profile. The plugin registers no
`FileChanged` hook: Claude Code watches only the file names a `FileChanged` matcher lists, so a
matcherless registration never fires, and Edit/Write changes already reach `PostToolUse`.

Automatic hooks never create a persistent index or launch a detached refresh. An existing current
snapshot is read when available; the existing bounded in-memory cold fallback may succeed within
the hook deadline. Run `corvint index --if-stale` explicitly under an owned supervisor after binary
or committed-tree changes. Missing, stale, or over-budget context remains visible uncertainty;
manual warmup does not qualify the unresolved automatic-refresh performance contract.

Prompt context is produced once by the native event and preserves separate governance, declared
scope and current-task evidence. The complete core envelope is untrusted repository data; trusted
status and warmup guidance is rendered outside it using validated root/key values and JSON argv.
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

The exact tested range and unavailable capabilities are in
`plugins/corvint/compatibility.json`. Static validation is not black-box conformance.
