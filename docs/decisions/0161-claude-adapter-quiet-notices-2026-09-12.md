# Decision 0161 — Claude adapter quiet notices

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12,
delegated coordinator call. Base: `56913d3480011b86936074ff9f117e9e601d23c5`.

The owner reported endless Claude Code notices from Corvint, for example
`PostToolUse:Write says: Corvint FALLBACK harness-receipt:sha256:...; gaps:
frontier-authority-unavailable, tool-change-path-not-observed` and `UserPromptSubmit says: Corvint
FALLBACK degraded: prompt-over-query-bound; coding continues`. `cmd/corvint/host_adapter.go`
wrote a `systemMessage` on every in-root `post-tool` and `file-change` receipt
(`invokeLegacyClaudeEvent`), on every degradation (`degradedAdapterOutput`), and on every
`session-start` (`renderClaudeContext`). In this session's own Claude Code transcript the
adapter degradations alone reached the user 421 times as `prompt-over-query-bound`, 140 times as
`corvint-invocation-timeout` and 54 times as `corvint-event-rejected`.

Those notices came from the installed plugin cache, `~/.claude/plugins/cache/corvint/corvint/0.1.4`,
whose `hooks.json` still runs `python3`. The repository ships 0.1.5, which runs `corvint`.
`tool-change-path-not-observed` and the `gaps:` suffix appear nowhere in the Go tree except
archived `conformance/perf-v0` outputs. This decision fixes the `corvint` adapter. The owner sees
it only after the installed plugin and `corvint` binary are refreshed, which this change does not
do.

## Owner call

Routine receipts and expected, non-fault degradations do not produce a user-visible
`systemMessage`. Genuine adapter faults the user must act on keep one. Quieting a notice must not
lose its information, because those degradations are defects to fix. Recorded as `AHI-021`, with
`AHI-019` amended, in `docs/specs/agent-harness-integration-v0.md`.

## Split

The split follows the reason codes in the Failure codes table and `AHI-009` of that spec.

| Class | Reasons | Claude output |
|---|---|---|
| expected | `prompt-over-query-bound`, `missing-prompt`, `file-change-path-not-project-relative`, `adapter-host-kill-deadline` (a bound, not a diagnosed fault: `AHI-012`) | `hookSpecificOutput.additionalContext` on `session-start`, `user-prompt`, `post-tool`; otherwise the unchanged `systemMessage` |
| fault | `corvint-event-rejected` and `corvint-event-rejected:<code>`, `malformed-corvint-output`, `invalid-input`, `project-root-unavailable`, `corvint-output-too-large`, the envelope collision code, `unsupported-hook-event`, `hook-input-too-large`, `malformed-hook-json`, `missing-session-identity`, `invalid-session-identity`, `invalid-start-source`, `invalid-stop-hook-active` | `systemMessage` (unchanged) |
| receipt | in-root `post-tool` receipt ID | `PostToolUse` additionalContext |
| receipt | `file-change` receipt ID | no host output; ledgered by `harness event` |
| notice | `session-start` fixed-degradation notice | removed |

An expected reason keeps its `systemMessage` on `stop`, `session-end` and `file-change`. Those
Claude hook events have no model-visible channel, and dropping the reason there would leave it
recorded nowhere. The Stop `continuationLimitMsg`, the `native-hook` and qualified-lifecycle
paths, and the Codex, Gemini CLI and OpenCode adapters are unchanged.

## Where each quieted item still lands

- In-root `post-tool` receipt: `PostToolUse` additionalContext (`claudeReceiptOutput`,
  `cmd/corvint/host_adapter.go:640@d197ebb5`). `post-tool` is not a `SOL-V0-001` ledger event
  (`internal/gokernel/harness.go:468@3f2d8101`), so the Claude Code transcript is its only record.
- `file-change` receipt: `harness event` appends a row with its receipt ID and degradations to
  `.corvint/self-observations.jsonl` (`internal/gokernel/harness.go:468-472@54ec26c7`, row fields at
  `internal/gokernel/harness.go:478@ea1571be`).
- Expected degradations on `session-start`, `user-prompt`, `post-tool`: additionalContext
  (`claudeDegradedOutput`, `cmd/corvint/host_adapter.go:711@695e4ab7`). A Claude `user-prompt` runs
  `dogfood event`, which owns no observations (`LCP-V0-003`), and `prompt-over-query-bound` is
  refused before any Corvint call, so again the transcript is the only record.
- `session-start` notice: its `frontier-authority-unavailable` and `host-version-unknown`
  degradations remain in the framed receipt (`cmd/corvint/local_completion_event.go:304@b0ea6e77`).

Claude Code writes hook output into the local session transcript
`~/.claude/projects/<project-slug>/<session>.jsonl` as `hook_additional_context`,
`hook_success` (with `stdout`) and `hook_system_message` attachments. This was observed in this
repository's four most recent transcripts, where SessionStart and UserPromptSubmit additionalContext
appears. A `PostToolUse` additionalContext attachment has not yet been observed there; that relies
on the owner-stated host support. The invariant-4 ledger exception is not widened.

## Inspecting recent degradations

- Transcript-only reasons: `rg -o 'Corvint FALLBACK degraded: [a-z0-9:-]*' ~/.claude/projects/<project-slug>/*.jsonl | sort | uniq -c`.
- Ledgered `file-change` receipts and their degradation counts: `corvint --root <root> observations`.

Corvint has no verb of its own for the transcript-only reasons. That is filed in
`docs/agent-memory/ideas.md`.

## Amends

- Decision 0097 item 2 kept the `prompt-over-query-bound` refusal's visible degradation under
  `AHI-009`. The refusal stays. For Claude `user-prompt` the degradation is now model-visible
  additionalContext, not a `systemMessage`.
- `AHI-009`, `AHI-012` and `AHI-017` visibility is met by that additionalContext for the expected
  reasons.

- 2026-09-13 amendment: `corvint-event-rejected:dogfood-event-deadline` joins the expected set. It is
  the Go successor of the 140 `corvint-invocation-timeout` notices above: the dogfood event's own
  deadline expires 100 ms or more before the `adapter-host-kill-deadline` watchdog, so on a loaded
  host the bound reached the user as a fault. Every other `corvint-event-rejected:<code>` stays a
  fault. Rollback removes that one entry from `claudeExpectedDegradation`.
- 2026-09-13 cross-reference: decision 0178 makes a hook outside any Git repository an expected
  absence ahead of this split. The adapter returns `{}` and records nothing, and the fault list
  above is unchanged.

## Rollback

Restore `degradedAdapterOutput` for every Claude reason, the `systemMessage` receipt in
`invokeLegacyClaudeEvent`, and the `session-start` notice in `renderClaudeContext`, then restore
`AHI-019`'s prior wording and delete `AHI-021`. No stored state, wire field or index format
changes.
