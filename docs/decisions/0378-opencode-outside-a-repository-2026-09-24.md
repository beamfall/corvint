# Decision 0378 — OpenCode outside a Git repository registers nothing

Date: 2026-09-24. Status: accepted. Authority: repository owner, "look at opencode compatibility.
"invalid-arguments" with "post-tool" is failing on my other machine using 0.8" (2026-09-24).
It amends decision 0178's OpenCode clause and `AHI-022`.

## Context

Decision 0178 made a hook outside any Git repository an expected absence for Claude Code, Codex
and Gemini CLI. It left OpenCode unchanged on the premise that the plugin "runs in-process inside a
project and emits no fault notice for this case". That premise is false. OpenCode opens a
directory with no VCS as its global project and reports `worktree` `/`
(`packages/opencode/src/project/project.ts:217` in `sst/opencode`). The plugin passes
`host.worktree || host.directory` as `--root`, so every hook runs
`corvint --root / harness event ...`. Corvint refuses a root without a `.git` entry with
`invalid-arguments` (`notRepositoryRootRefusal`, `cmd/corvint/diagnostic_refusals.go`), in 0.7.0
and 0.8.0 alike. The plugin then prints `[corvint/opencode] {"code":"invalid-arguments",...}`
after every tool call.

## Decision

The OpenCode plugin applies decision 0178's inside-or-outside test to `host.directory`, the
directory OpenCode was opened in. It uses the same rule: only a confirmed absence of `.git` at
every level, after symlinks are resolved, is outside, and an unreadable level counts as inside.
Outside a repository the plugin returns no hooks and no tools. It invokes no Corvint command,
writes no log entry and prints no warning.

Inside a repository nothing changes. `host.worktree` stays the root, and every other failure
keeps its `AHI-022` warning.

## Consequences

- `AHI-022` gains the OpenCode clause.
- `integrations/host-adapters.test.mjs` adds `AHI-022 decision 0378 OpenCode outside a Git
  repository registers and invokes nothing`.
- The `corvint_context` and `corvint_record_outcome` tools are absent from an OpenCode session opened
  outside a repository. They had nothing to serve there, because every call was refused.

## Rollback

Delete the `insideGitRepository` early return in `integrations/opencode/src/index.js`, the helper
in `integrations/opencode/src/runtime.js`, the test and the `AHI-022` sentence. The plugin then
warns `invalid-arguments` on every hook outside a repository again.
