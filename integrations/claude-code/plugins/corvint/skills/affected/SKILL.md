---
name: affected
description: List the tests Corvint selects for the current uncommitted or committed change, with the reason each was selected and what remains unknown. Use before running tests for a change.
argument-hint: "[BASE_COMMIT]"
allowed-tools: Bash(corvint affected:*), Bash(git rev-parse:*)
---

# Corvint affected tests

Compile Corvint's affected-test plan. With a commit in `$ARGUMENTS`, resolve it to a full commit ID
with `git rev-parse` and run `corvint affected --base FULL_COMMIT_ID`. Without one, run
`corvint affected` for the dirty worktree. The command runs no tests and writes no repository state.

Summarize the plan for a person:

1. The selected test units, grouped by package or file, each with the witness Corvint recorded.
2. Exclusions and their reasons.
3. Every unknown, including `NO_AFFECTED_SELECTION_PROOF`. Corvint never claims that an omitted
   test is safe to skip, so say that the selection is a starting point, not proof of coverage.
4. For Go, the exact `go test` command for the selected packages from `provider.go.packages`.

Do not run the tests unless the user asks.
