# Corvint Tasks ticket worktrees V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted under the owner's standing auto-accept (2026-10-11) (V1-1588; GitHub #1031)
Delivery status: experimental
Authoritative inputs: GitHub issue beamfall/corvint#1031 and native ticket V1-1588 ("tasks ensures a
per-ticket worktree on a ticket-named branch in a declared repository"), decision 0397 (corvint-tasks
built in tree), the [intent worktree contract](corvint-tasks-intent-worktree-v0.md), and the in-tree
sources under `internal/tasks/intent`, `internal/tasks/store` and `internal/tasks/cli`.

## Agent digest
- Claim: `corvint-tasks worktree ensure <ticket>` creates or reuses one worktree per ticket on a pattern-derived branch inside a declared root and never resets it.
- Status: TWT-V0-001..007 accepted under the owner's standing auto-accept (2026-10-11) (V1-1588; GitHub #1031); experimental; implemented with focused tests.
- Exists: the optional policy `worktrees {repository, base, branchPattern}`, `store.EnsureTicketWorktree`, and the `worktree ensure` verb.
- Blocked on: nothing for V0; recording the worktree head on release or completion is a residual (see Non-goals).
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

An agent or operator working a ticket needs an isolated checkout on a predictable branch, and a
retried start must land in the same place rather than a second copy. Before this contract each
caller derived its own branch and path, so two runs of the same ticket could diverge, and a reset
could discard uncommitted work. The verb writes no Tasks state: it is a Git operation bound to a
declared policy, and its only effect is `git worktree add` when the worktree does not exist yet.

## Requirements

- `TWT-V0-001`: Policy MAY declare `worktrees` as a closed object `{repository, base,
  branchPattern}`. `repository` is `.`, a repository-relative path without `..`, or an absolute
  path; `base` is a ref name that does not start with `-` and holds no `..`; `branchPattern`
  holds the token `{ticket}` exactly once, does not start with `-` and holds no `..`. Any other
  shape refuses `MALFORMED`. Without the key `worktree ensure` refuses
  `PREREQUISITE_UNSATISFIED`.
- `TWT-V0-002`: The ticket branch is `branchPattern` with `{ticket}` replaced by the ticket's local
  id, and nothing else: no title, actor, time or attempt enters it. A derived name Git refuses
  (`git check-ref-format --branch`) refuses `MALFORMED`.
- `TWT-V0-003`: When the branch is already checked out in a worktree, ensure returns that worktree
  unchanged with `created: false`; repeated calls return the same path and branch. Otherwise it
  adds a worktree at `<root>/.worktrees/<local>` or the caller's `--path` (absolute or relative to
  the root), creating the branch at the base commit when it does not exist. A `--path` that
  differs from the branch's existing worktree, or an existing path that is not the branch's
  worktree, refuses `RESOURCE_COLLISION`. A concurrent ensure that wins the add is observed and
  returned: while the path is empty or already holds its `.git` entry, the loser re-observes for
  up to 30 seconds before it refuses.
- `TWT-V0-004`: The base must name a commit (`MISSING_EVIDENCE` otherwise). An existing branch is
  attached only when the base commit is an ancestor of its tip; otherwise ensure refuses
  `PREREQUISITE_UNSATISFIED` and attaches nothing. On reuse the result reports `baseAncestor`
  and warns when it is false; it never rebases or merges.
- `TWT-V0-005`: The repository must resolve to the top level of a Git worktree. The target path is
  resolved through its longest existing ancestor, following symbolic links, and must lie strictly
  inside the resolved root; an absolute path elsewhere, a `..` escape, the root itself or a
  symbolic link out of the root refuses `OUT_OF_SCOPE` before any Git write. A reused worktree
  outside the root also refuses `OUT_OF_SCOPE`.
- `TWT-V0-006`: The result carries the ticket id, `repository {root, sha256}` (the SHA-256 of the
  absolute Git common directory), `branch`, `path`, `base`, `baseCommit`, `baseAncestor`, `head`,
  `created`, `dirty` and `dirtyEntries` (porcelain status entries, untracked included). A dirty
  worktree is reported with a warning and is never reset, cleaned or checked out.
- `TWT-V0-007`: `worktree ensure <ticket> [--path PATH]` resolves the ticket in the queue
  inventory and writes no journal, receipt or intent byte. Git runs with the sanitized Tasks Git
  environment and no credential helper.

## Non-goals and simpler baseline

The simpler baseline is caller-managed worktrees; it was rejected because it cannot make a retried
start idempotent or keep branches derivable from the ticket. Out of scope: recording the worktree
head on release or completion (a residual of V1-1588), removing worktrees, multi-repository
programs (the existing CAL-V0-071 sibling worktrees stay separate), and any network fetch of the
base.

## Failure modes

| Situation | Behavior |
| --- | --- |
| No `worktrees` policy | `PREREQUISITE_UNSATISFIED`; nothing written (`TWT-V0-001`). |
| Repeated ensure, worktree dirty | Same path and branch, `created: false`, `dirty: true`, warning; files untouched (`TWT-V0-003`, `TWT-V0-006`). |
| Existing branch not descended from base | `PREREQUISITE_UNSATISFIED`; no worktree added (`TWT-V0-004`). |
| Unknown base | `MISSING_EVIDENCE` (`TWT-V0-004`). |
| `--path` outside the root, `..`, or a symlink out | `OUT_OF_SCOPE`; no branch or worktree created (`TWT-V0-005`). |
| `--path` differs from the branch's checked-out worktree | `RESOURCE_COLLISION` (`TWT-V0-003`). |
| Two ensures race | One adds; the other observes and returns it (`TWT-V0-003`). |

## Acceptance evidence

Focused `internal/tasks/intent`, `internal/tasks/store` and `internal/tasks/cli` tests on Darwin.
Linux and live agent-host qualification are `NOT_RUN`.

## Rollback

Revert the V1-1588 change and remove `worktrees` from policy. The verb stores no Tasks state, so no
store byte depends on it; created worktrees and branches stay as ordinary Git state and are removed
with `git worktree remove` and `git branch -d`. An older binary refuses a policy carrying
`worktrees` closed.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `TWT-V0-001` | `TestTWTV0001_PolicyWorktrees`; `TestTWTV0007_WorktreeEnsureThroughTheCLI` (undeclared refusal) |
| `TWT-V0-002` | `TestTWTV0002_TWTV0003_TWTV0006_EnsureIsIdempotentAndNeverResets` |
| `TWT-V0-003` | `TestTWTV0002_TWTV0003_TWTV0006_EnsureIsIdempotentAndNeverResets`; `TestTWTV0003_ConcurrentEnsureConvergesOnTheWinner`; `TestTWTV0007_WorktreeEnsureThroughTheCLI` |
| `TWT-V0-004` | `TestTWTV0004_TWTV0005_RefusesOutsideRootAndUnrelatedBranch` |
| `TWT-V0-005` | `TestTWTV0004_TWTV0005_RefusesOutsideRootAndUnrelatedBranch`; `TestTWTV0007_WorktreeEnsureThroughTheCLI` |
| `TWT-V0-006` | `TestTWTV0002_TWTV0003_TWTV0006_EnsureIsIdempotentAndNeverResets`; `TestTWTV0007_WorktreeEnsureThroughTheCLI` |
| `TWT-V0-007` | `TestTWTV0007_WorktreeEnsureThroughTheCLI` (state and intent trees unchanged) |
