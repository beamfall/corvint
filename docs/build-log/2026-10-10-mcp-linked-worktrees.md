# corvint-mcp linked worktrees (V1-1139, GitHub #781)

Date: 2026-10-10. Status: slice delivered, remainder proposed; no new requirement.

The multi-root profile (`MMR-V0`, decision 0441) already lets an operator declare each linked worktree
as an alias and returns per-alias commit and `worktreeState` in `corvint.status`. This change adds
`TestMMRV0007LinkedWorktreesBindTheirOwnCommit` (real `git worktree add`, two aliases, distinct
commits) and a spec section recording the gap. A per-call client-supplied root, snapshot or revision,
cwd discovery and a caller-worktree mismatch flag conflict with `MCPV0-015` (operator-only authority)
and need an owner decision, so they are not implemented. Related: V1-0254, V1-0191.
