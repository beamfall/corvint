# Decision 0178 — Not a repository is not a host fault

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12), delegated coordinator call.

## Context

The Claude Code, Codex and Gemini CLI hooks are installed per user, so they also fire in sessions
whose working directory is not inside any Git repository. Corvint has nothing to serve there, since
evidence is pinned to Git content (invariant 1). Before this decision each adapter still ran the
Corvint call and reported the failure as a fault:

- Claude Code (`cmd/corvint/host_adapter.go`, `runClaudeAdapter`) showed a `systemMessage`
  (`AHI-021`) on `session-start` and on every Edit/Write `post-tool`.
- Codex (`runCodexAdapter`) did the same.
- The Gemini CLI hook (`integrations/gemini-cli/hooks/corvint-hook.mjs`) spawned `corvint` and
  surfaced its rejection as a fault notice. The installed binary returned an `invalid-arguments`
  `systemMessage`.

Each of those notices repeats on every such hook. There is also no repository to hold a
`SOL-V0-010` ledger row.

## Governing contract

- `AHI-021` and `AHI-022` in `docs/specs/agent-harness-integration-v0.md`.
- Decision 0161 (the notice split).
- Decision 0169 (the adapter degradation ledger).

## The call

A hook whose working directory is not inside a Git repository is an expected absence, not a fault.

**Inside or outside.** A directory is inside a repository when it, or any ancestor, holds a `.git`
entry, after its symlinks are resolved. An ancestor level that cannot be read counts as inside, so
only a confirmed absence at every level is outside. A directory beneath a repository root is
therefore still inside it. That includes one Corvint rejects, such as a subdirectory passed as the
root, and its failure keeps its fault notice.

**What each adapter does outside a repository.** It invokes no Corvint command and appends no ledger
row.

- Claude Code (root from `CLAUDE_PROJECT_DIR` or the working directory) returns `{}`.
- Codex (root from an absolute payload `cwd`) returns `{}`. A missing `cwd` stays the
  `missing-cwd` fault.
- The Gemini CLI hook returns `{"continue":true,"suppressOutput":true}` after its cwd validation.

Every other failure keeps its fault notice under `AHI-021` and `AHI-022`. OpenCode is unchanged: it
runs in-process inside a project and emits no fault notice for this case.

## Consequences

- `AHI-021` and `AHI-022` gain the not-a-repository rule.
- Decision 0161's fault list is unchanged; this is a precondition checked before any reason arises.
- `TestClaudeAdapterFaultKeepsNotice` now pins `missing-session-identity` inside a repository.
- `TestClaudeAdapterNotARepositoryIsSilent` covers Claude `session-start`, Claude Edit and Write
  `post-tool`, and Codex `SessionStart`.
- The `host-adapters.test.mjs` test `AHI-022 decision 0178 Gemini outside a Git repository emits and
  invokes nothing` covers Gemini.
- The Claude Code plugin moves to 0.1.6 and the Gemini CLI extension to 0.1.3 (`AHI-020`).

## Rollback

1. Delete `insideGitRepository` and its two call sites in `cmd/corvint/host_adapter.go`.
2. Delete the `insideGitRepository` early return in `corvint-hook.mjs`.
3. Delete the two tests, and restore the prior fault in `TestClaudeAdapterFaultKeepsNotice`.
4. Remove the `AHI-021` and `AHI-022` sentences.
5. Bump each package version.

No stored state, wire field or index format changes.
