## 2026-09-27 V1-0331 CTS-V0-004: corvint-tasks resolves a symlinked ancestor of the repository

The pre-1.0 panel addendum reported that `corvint-tasks` refused every path with a symbolic link
component. On macOS `/tmp` and `/var` are symbolic links into `/private`, so `init` failed with
`UNSUPPORTED_FILESYSTEM` for any repository under them, including the README quickstart run in a
temporary directory. The test fixture hid this by creating its repositories under the resolved path.

Decision: store resolution now resolves symbolic links in the directories above the primary worktree
and records the resolved primary-worktree path. The primary worktree and its `.git` path stay
literal, so a symbolic link there is still refused as `UNSUPPORTED_FILESYSTEM`. An ancestor that
cannot be resolved is refused with the same code. One repository therefore has one recorded primary
worktree, however the working directory spelled it. `CTS-V0-004` states the rule in
`docs/specs/corvint-tasks-store-init-v0.md`, which owns store initialization until the task-store
contract is recovered (V1-0310).

Set aside: refusing only repository-internal links by checking the path from the worktree down,
which would record whichever spelling the caller used, so `/tmp/x` and `/private/tmp/x` would name
two primary worktrees for one journal; and resolving the whole path, which would accept a symbolic
link at the worktree itself.

Evidence: `TestCTSV0004_InitThroughSymlinkedAncestor` initializes a repository through a symbolic
link to its parent directory, checks that `head.json` records the canonical path, and runs
`queue status` through both spellings. The existing `internal/tasks/intent` and `internal/tasks/cli`
tests still pass.

Owner acceptance: the owner accepted CTS-V0-001 and CTS-V0-004 on 2026-09-27. CTS-V0-002 and
CTS-V0-003 stay proposals.

Rollback: remove `canonicalAncestors` in `internal/tasks/intent/worktree.go`. A repository under a
symlinked ancestor is refused again; a journal it already wrote keeps the canonical path and stays
readable through that path.
