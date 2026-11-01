# 2026-10-31 — MCP server: a missing `--root` serves the launch worktree, visibly (V1-3497, MCPV0-115)

## Problem

Claude desktop can start a spare-claimed session's plugin MCP server with
`--root ${CLAUDE_PROJECT_DIR}` naming the session's spare directory, `~/.claude/spares/spare-*`,
after that directory has been deleted. The process itself runs in the claimed worktree. The
server refused with `corvint-mcp: repository unavailable`, exit 2, and stayed failed for the
session, reconnects included. The stale argument comes from the host; Corvint can only mitigate
it. The hook adapter (`claudeAdapterRoot`) falls back to the working directory only when
`CLAUDE_PROJECT_DIR` is empty; with a nonempty deleted path it keeps that path, and its hooks
return no context. This change does not alter the hooks; that gap is tracked separately.

## Decision

The ticket offered two options: fall back to the worktree enclosing the launch directory, or
refuse with a typed reason while the server stays up. The owner leaned towards the fallback, and
this lane took it, under the standing lane auto-accept (`MCPV0-115`, experimental). The rule is
deliberately narrow:

- **Scope.** It applies only when there is exactly one plain `--root`, `lstat` reports it missing,
  and neither `--follow-ref` nor `--corpus-artifact` is given.
- **What it serves.** The candidate is the nearest ancestor of the launch directory holding a
  `.git` entry of any kind, or whose `.git` lookup fails other than not-exist, observed once at
  startup. The walk never passes such an entry to reach an outer repository. The candidate then
  passes every `MCPV0-001` check as if it had been the `--root`; a symlink marker or unreadable
  entry refuses `corvint-mcp: repository unavailable`, as a direct `--root` of it would.
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
for the whole session. The fallback serves the worktree the process actually runs in. The notice
keeps the substitution visible to the model.

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
  - `TestMCPV0115FallbackIsNarrow` checks five refusals that must stay unchanged, and that a
    nested directory whose `.git` is a symlink to another repository's metadata is refused like a
    direct `--root` of it instead of serving the outer repository. Review finding (Codex, one
    round): the first walk reused `enclosingWorktree`, which skips such markers; with that walk
    restored, this case serves the outer repository and fails.
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
