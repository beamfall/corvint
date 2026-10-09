# 2026-10-08: Context in a linked worktree pins its own HEAD (V1-0998, refuted)

## Intent

Ticket V1-0998 (suspected, P2) reported that `corvint --root WT context` in a linked worktree
whose HEAD was `0b5096ca` answered for revision `21f255f3`. It suspected that the index or
revision was resolved from the shared `.git` rather than the worktree's HEAD. Acceptance asked
for `context` to pin the worktree's HEAD, or to refuse or mark uncertainty when no index exists
for it, plus a test with a linked worktree whose HEAD differs from the primary checkout.

## Finding: refuted

`21f255f3c1bdcee6e905a8776d288739cc5013ad` is the tree of commit `0b5096ca`
(`git rev-parse 0b5096ca^{tree}` in a fresh clone of `beamfall/corvint`). The packet's
`revision` is the tree OID, not a commit ID, as `docs/AGENT-ROUTES.md` already says ("compare
the packet's `revision` with the captured commit's Git **tree** identity"). The observed packet
answered for the worktree's own HEAD. The mismatch came from comparing a tree OID with a commit
OID.

The code path agrees. `loadSnapshot` and the build both read identity through
`git rev-parse ... HEAD HEAD^{tree}` with the worktree as the working directory
(`internal/contextindex/git.go` `readIdentity`). The shared store under the Git common directory
is keyed by tree OID (DIRTY-CACHE-013). A linked worktree therefore hits only a snapshot of its
own tree. With no such snapshot it misses and builds over its own `HEAD^{tree}`, so the
"refuse or mark uncertainty" branch of the acceptance does not arise.

## Evidence

- Manual reproduction (binary built from `f33ea8ef`): two worktrees with a shared store holding
  only the primary's snapshot. Primary context `revision` = primary tree (`hit=true`). Linked
  context `revision` = linked tree (`hit=false`, built), and it names the linked-only file.
  After `index` in the linked worktree, the linked tree hits and answers the same way.
- New regression `TestContextInALinkedWorktreeAnswersForItsOwnHead`
  (`cmd/corvint/context_linked_worktree_test.go`) covers both the miss and the hit for a linked
  branch whose HEAD and tree differ from the primary. It checks that `revision` equals the linked
  `HEAD^{tree}` and that the linked-only file is named, and it asserts the loader outcome
  (`compileTaskContext`'s `hit`) for each phase. It also checks that the primary still answers its
  own tree, and that no `context` run changes the shared store (every file's path and SHA-256)
  or either worktree's porcelain status (TCP-V0-001). Two mutations failed the test: querying
  the primary root in place of the linked one (primary tree as `revision`), and inverting the
  expected hit/miss.
- Independent review (Codex, read-only): refutation confirmed, no path found that substitutes the
  primary's tree or content. Two P3 test findings (unasserted hit/miss, name-only store check)
  were fixed as described above.

## Decisions

- No behavior or wire change, and no new requirement: the existing contracts (TCP-V0-001,
  DIRTY-CACHE-001, DIRTY-CACHE-013) already state the observed behavior.
- Follow-up idea (not filed here; lane rule): the packet carries only the tree OID, so a reader
  comparing it with `git rev-parse HEAD` misreads it. Adding a `commit` member would be a wire
  change against the TCP-V0-024 golden and needs its own spec decision.
