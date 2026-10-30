# Corvint for Codex

This package is the source of the native Codex CLI/Desktop integration. The qualified install does
not use this directory: the installable plugin is published from a committed revision into a user-owned
marketplace, `~/.local/share/corvint/codex-marketplace/corvint-full-candidate`, whose `PUBLISH.json`
records the source revision, the stamped plugin version and the three transforms applied to this
tree (version stamp; `hooks/hooks.json` replaced by the `prepare-qualified-hooks` rendering for the
installed root release; `scripts/` omitted because the adapter lives in the root release). Add that
marketplace with `codex plugin marketplace add ~/.local/share/corvint/codex-marketplace/corvint-full-candidate`,
install with `codex plugin add corvint@corvint-full-candidate`, then review and trust its lifecycle hooks
with `/hooks`. After a republish, rerun `codex plugin add corvint@corvint-full-candidate`; `codex plugin
marketplace upgrade` refuses a local marketplace (`is not configured as a Git marketplace`). Disable or
uninstall with `codex plugin remove corvint@corvint-full-candidate`. Removing the plugin
leaves Corvint repository data unchanged.

To try the preview without that publish step, put `corvint` on `PATH` (see
[Try it](../../README.md#try-it-in-sixty-seconds)) and, from the root of a Corvint checkout, run
`codex plugin marketplace add ./integrations/codex` and `codex plugin add corvint@corvint-source`. This
installs the source tree as-is: its `hooks/hooks.json` calls `corvint adapter codex` from `PATH`
rather than the qualified root-release rendering. Remove it with
`codex plugin remove corvint@corvint-source` and `codex plugin marketplace remove corvint-source`.

Before refreshing a local plugin with `codex plugin add`, wait for active tasks using its hooks
to finish: reinstall can prune the old version cache while those tasks retain absolute hook paths.
Record the installed version and preserve its exact artifact, including file modes, before refresh.
Verify the new cache against its source and start a new task to pick it up. If an upgrade removes
a still-referenced cache, restore that exact old artifact atomically at its original version path
without selecting it as the installed version or changing the new cache or hook trust. Keep it until
the affected tasks end; source/cache equality alone does not verify active hook paths.

The package provides the Corvint skill, bounded `SessionStart`, `UserPromptSubmit`, `Stop`, and
`SessionEnd` hooks, and the read-only stdio `corvint-mcp` server (`.mcp.json`, AHI-207). The server
binds the Git worktree of the directory Codex launches it in (the session working directory) once,
at startup; after switching to another worktree, start a new Codex session there to rebind. It
speaks MCP `2025-11-25`, because Codex 0.162 still gates the `2026-07-28` protocol behind an
unreleased feature. It needs `corvint-mcp` on `PATH` (`go build -o DIR/corvint-mcp ./cmd/corvint-mcp`).
Outside a Git worktree it exits 2 before starting and Codex continues without Corvint tools.
Current status is `FALLBACK`: supported context works when the local `corvint` and `corvint-mcp`
executables are available. A separately enrolled local completion policy can request one bounded
Stop remediation; formal closing Frontier authority remains unavailable.

The hook routes all four native events to `corvint`, including `UserPromptSubmit` and
`SessionStart(source=compact)`. Prompt input is trimmed to the core's canonical task and carries
only that task plus an optional session-ID hash. A missing `corvint` binary makes the hook command
exit 127, which Codex shows as a failed hook (`hook exited with code 127`) without blocking the event;
no event routes to a legacy runtime. Every runtime
response passes the separate `corvint-dogfood-event/0` full-result digest, adapter, event, repository,
support-level, prompt-privacy and local-policy validation boundary; only its actionable
`corvint-hook-context/0` projection (AHI-045) is then placed in the fixed untrusted-data envelope, and
a `UserPromptSubmit` with nothing actionable injects nothing (AHI-046). The legacy
`corvint-harness-event/0` CLI profile stays unchanged.

The installed `corvint` process owns input normalization, the bounded native event, validation,
and host rendering under the existing event deadline. No adapter subprocess or detached worker is
created. Oversized input or output, interruption, runtime failure, and deadline expiry produce fixed
visible fallback reasons.

Codex 0.162 hook events are `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`,
`PermissionRequest`, `PreCompact`, `PostCompact`, `SubagentStart`, `SubagentStop`, `Stop` and
`SessionEnd` (AHI-208). The plugin registers only the first-listed four it maps; the others stay
unregistered. If Codex delivers one anyway, the adapter abstains silently with the non-blocking
`{}` and records `unsupported-hook-event` in the local ledger `.corvint/self-observations.jsonl`.

Codex also exposes [`PostToolUse`](https://developers.openai.com/codex/hooks#posttooluse), but its
native event supplies raw tool arguments and tool output, not Corvint's normalized changed paths,
evidence handles, or verification observations. This plugin therefore does not register that hook:
forwarding those raw bodies would violate `AHI-014`, while an empty receipt after every tool call
would add overhead without evidence value.

The bundled Corvint skill can invoke the Go authority-start query directly for roadmap, orientation,
work-queue, and workflow-gate tasks. The plugin does not reinterpret that context receipt as a
harness-event receipt. Automatic prompt context remains `FALLBACK` and is injected only after the
native core returns a valid bounded receipt. The new profile separates root governance, declared
enrollment scope and explicit task anchors. Vague follow-ups retain governance with
`explicit-task-anchor-required`, withholding incidental lexical matches. General natural-language
discovery remains available through direct `query`.

After Codex compacts a session, `SessionStart(source=compact)` asks Corvint to rehydrate a bounded
impact packet from exact dirty paths tracked at the pinned revision. Untracked paths remain an
explicit count/digest gap rather than invented evidence. This restores repository evidence only:
the adapter never reads Codex's transcript or persists prompt/model text, so decisions and other
uncommitted task state remain an explicit gap.

Adapter version: `0.2.3`; the last tested source adapter version is `0.2.2`. The shipped compatibility metadata records Codex `0.149.0`
static manifest validation, the state at package build. The published matrix
(`../compatibility.json`) records the later installed-lifecycle PASS for Codex CLI `0.153.2` with
adapter `0.2.2` on darwin-arm64 against Corvint 0.8.1. Every other host version, adapter version and
OS remains untested `FALLBACK` by contract.

The bundled skill drives `dogfood begin/status/verify/finish/review` for authorized substantive Corvint
changes. It still judges citation meaning and check adequacy. The core executes selected checks,
binds report inspection and validates the exact final artifact set before local satisfaction.
Only explicit enrollment can enable Stop remediation. Recursive Stop, cancellation, unsupported
hosts and timeout release without a satisfaction claim. This is local workflow enforcement under
decision 0009 option 2; it is not an execution attestation or host-conformance promotion.

Run `corvint index --if-stale` explicitly after replacing the binary or changing the committed
tree to refresh the derived snapshot. A fresh worktree without that snapshot can exceed the native
event deadline and fail open; the hook never silently warms persistent state. Original explicit
query/impact receipts may retain their caller-entered task in the private provenance archive; this
adds no capture of native prompt bodies or transcripts. Secret-shaped copies are refused visibly.

## Read-only review profile

`profiles/corvint-review.config.toml` (AHI-206) is a Codex profile for review runs. Copy it to
`$CODEX_HOME/corvint-review.config.toml`. It sets a read-only sandbox, `approval_policy = "never"`
and `CORVINT_TASKS_READ_ONLY=1` in the shell environment, so neither an escalated command nor a
`corvint-tasks` mutation can write the Tasks store (ATR-V0-212). It does not stop a write that
bypasses the `corvint-tasks` CLI.

On Codex 0.162 a trusted project's `.codex/config.toml` layers above the selected profile, so a
project can switch the sandbox to `danger-full-access` and the variable to `0`. Only `-c` overrides
outrank it, so run every review with the same three keys on the command line:

```sh
codex exec -p corvint-review -c 'sandbox_mode="read-only"' -c 'approval_policy="never"' \
  -c 'shell_environment_policy.set.CORVINT_TASKS_READ_ONLY="1"' PROMPT
```

Live receipt (Codex 0.162.0, a trusted project that sets the opposite values):
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-codex-integration-matrix.md`.

## Approvals across Codex agents

With automatic approval review (`approvals_reviewer = "auto_review"`, or `codex exec
--approve-for-me`), Codex 0.162 reviews every escalated command in a separate guardian thread. For a
native subagent that review receives the root thread's user messages, and only those can authorize
an action; assistant relays, files and the child's own justification are untrusted (AHI-209). So
the supported way for a subagent to carry out a human-approved action is for the human to approve the
exact action (command, target, revision) in the root thread, and for the subagent to request
escalation for that exact command. A changed target is denied. A low-risk action can still be
allowed with `user_authorization: low`, so approval review is not a substitute for a Corvint gate.
Each decision is recorded as a `guardian_review` session under `$CODEX_HOME/sessions/`, whose
`parent_thread_id` names the acting thread, with JSON `outcome`, `risk_level`, `user_authorization`
and `rationale`. `codex exec --json` does not emit these decisions. To grant an action that the
root thread has not approved yet, the human approves it as a new user turn in the root thread, for
example `codex exec resume ROOT_THREAD PROMPT` (live-tested for a root-thread action; `codex queue
--thread` is untested); never relay it as an assistant message or a file.
