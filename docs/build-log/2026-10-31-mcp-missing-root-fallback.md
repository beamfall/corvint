# 2026-10-31 — MCP server: a missing `--root` serves the launch worktree, visibly (V1-3497, MCPV0-115)

## Problem

Claude desktop can start a spare-claimed session's plugin MCP server with
`--root ${CLAUDE_PROJECT_DIR}` naming the session's spare directory, `~/.claude/spares/spare-*`,
after that directory has been deleted. The process itself runs in the claimed worktree. The
server refused with `corvint-mcp: repository unavailable`, exit 2, and stayed failed for the
session, reconnects included. Hooks kept working, because the hook adapter already falls back
from `CLAUDE_PROJECT_DIR` to the working directory (`claudeAdapterRoot`). The stale argument comes
from the host; Corvint can only mitigate it.

## Decision

The ticket offered two options: fall back to the worktree enclosing the launch directory, or
refuse with a typed reason while the server stays up. The owner leaned towards the fallback, and
this lane took it, under the standing lane auto-accept (`MCPV0-115`, experimental). The rule is
deliberately narrow:

- **Scope.** It applies only when there is exactly one plain `--root`, `lstat` reports it missing,
  and neither `--follow-ref` nor `--corpus-artifact` is given.
- **What it serves.** The substitute is the nearest enclosing `.git` directory or gitfile,
  observed once at startup. It then passes every `MCPV0-001` check as if it had been the
  `--root`.
- **Visibility.** It prints one path-free stderr line, and adds an `MMR-V0-009`-schema notice
  block (code `missing-root-launch-worktree`) to every routed tool result. The first block and
  `structuredContent` are those of a direct `--root` run.
- **Outside a worktree.** With no launch worktree, startup still refuses, now with a reason that
  names the cause.
- **Unchanged refusals.** These keep their existing refusals:
  - a root that exists but is not a repository;
  - a path through a non-directory;
  - aliased and `--workspace` roots;
  - relative roots (`MCPV0-113`);
  - `--follow-ref`, which moves a dedicated worktree;
  - `--corpus-artifact`, which names a file in the declared root.
- **Client roots.** They stay ignored (`MCPV0-015` and `MMR-V0-005`).
- **Placement.** The rule lives in the shared `internal/mcp/cli` run. `corvint mcp` (plugin 0.3.8)
  and the standalone `corvint-mcp` (the installed 0.3.6 plugin) therefore both get it with no
  plugin change.

**Why not a typed refusal.** A refusal that keeps the server up still leaves every tool unusable
for the whole session. The fallback serves the worktree the process actually runs in, which is the
same root the hooks already use. The notice keeps the substitution visible to the model.

**Specs.** `MCPV0-015` and `MCPV0-001` now point to `MCPV0-115`. `AHI-198` and `AHI-373` carry
amendment notes. `AHI-198`'s "nowhere else" is unchanged: it says where the server is declared,
not where its root comes from.

## Evidence

- **Focused tests.** `go test -run 'TestMCPV0115|TestMMRV0009|TestMMRV0001|TestMCPV0113'
  ./internal/mcp/cli/` passes.
  - `TestMCPV0115MissingRootServesLaunchWorktree` covers subdirectory and linked-worktree
    launches. It checks the served revision, that the first block and `structuredContent` equal a
    direct `--root` run, the notice code, and that no path appears.
  - `TestMCPV0115MissingRootOutsideWorktreeRefuses` checks the exact refusal, and the
    unobservable-launch-directory case.
  - `TestMCPV0115FallbackIsNarrow` checks five refusals that must stay unchanged.
- **Mutation check.** With `main.go` reverted to base, the first two tests fail with the
  production symptom: `exit=2 stderr="corvint-mcp: repository unavailable\n"`.
- **Built binaries.** Both `corvint-mcp --root ~/.claude/spares/<deleted>` and
  `corvint mcp --root ~/.claude/spares/<deleted>` were run from a worktree subdirectory. Each gave
  `corvint.status` with `isError: false` at the worktree HEAD, 2 content blocks, the notice
  `missing-root-launch-worktree`, the stderr line, and exit 0. Run from a directory outside any
  worktree, each exited 2 with the new refusal.
- **`NOT_OBSERVED`.** A real Claude desktop spare-claimed session against a sanctioned install of
  this build has not been run. That remains the last verification step.

## Rollback

Delete `missingRootFallback` (`internal/mcp/cli/missing_root.go`) and its two uses in `run`. A
missing root then refuses as `corvint-mcp: repository unavailable` again. No state depends on it.
