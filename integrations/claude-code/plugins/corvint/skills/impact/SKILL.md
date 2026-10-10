---
name: impact
description: Show what a committed change or a set of tracked paths affects, with pinned evidence and explicit uncertainty. Use before or after changing code to find likely side effects and the code that depends on it.
argument-hint: "[BASE_COMMIT | PATH...]"
allowed-tools: Bash(corvint impact:*), Bash(git rev-parse:*), Bash(git status:*)
---

# Corvint impact

Run Corvint's read-only impact analysis for `$ARGUMENTS` in the current Git repository.

- No arguments: if the worktree has commits ahead of its upstream or of `origin/main`, use their
  merge base as the base. Otherwise use the tracked paths that `git status --porcelain` reports as
  modified.
- A commit: resolve it to a full commit ID with `git rev-parse` and run
  `corvint impact --base FULL_COMMIT_ID --limit 20`. This needs a clean worktree whose `HEAD`
  ends the range.
- Paths: run `corvint impact PATH... --limit 10`. Add `--working-tree-untracked` only for paths the
  user identified as untracked.

Report the result for a person, not as raw JSON:

1. One sentence on scope: base or paths, revision, and whether the result was budgeted.
2. The affected items, most important first, each with its path and the evidence reason Corvint gave.
3. Every uncertainty, omission, unsupported language, `impact-range-drift` or `BUDGETED` marker,
   stated plainly. These keep the impact question open; do not present the result as complete.

If Corvint refuses the request or the target is unsupported, show its reason and say which
repository tool would answer instead. Never approximate a Corvint result.
