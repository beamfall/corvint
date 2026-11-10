# Corvint for Codex

This package is the source of the Codex CLI/Desktop plugin. It provides one skill that runs the
installed `corvint` command for change-scoped answers (`corvint impact`, `corvint feature`). It
registers no lifecycle hooks and no MCP server: both, with the qualified-hook publish step, were
removed by decision 0594. Its support level is `FALLBACK`.

To install from source, put `corvint` on `PATH` (see
[Try it](../../README.md#try-it-in-sixty-seconds)) and, from the root of a Corvint checkout, run
`codex plugin marketplace add ./integrations/codex` and `codex plugin add corvint@corvint-source`.
Remove it with `codex plugin remove corvint@corvint-source` and
`codex plugin marketplace remove corvint-source`. Removing the plugin leaves Corvint repository data
unchanged.

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
