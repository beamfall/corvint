# V1-1081: complete accepts upstream-integrated commits

Date: 2026-10-09

## Problem

`corvint-tasks complete --commit <oid>` checked only `refs/heads/<intentBranch>`. In the primary
checkout, `main` is checked out with about 2,000 unrelated staged changes and lags `origin/main`.
A commit already merged on `origin/main` was therefore refused `STALE_TREE` ("is not reachable from
main"). Fast-forwarding a checked-out branch under unrelated staged work is unsafe, so native
completion could not happen at all. This blocked V1-1053, V1-1060 and V1-1080. The refusal's
reason also existed only in the free-text detail: the same `STALE_TREE` code covered both an
unintegrated commit and a tree mismatch.

## Decision

The owner delegated the fork. This change amends `CAL-V0-017` in
`docs/specs/corvint-tasks-agent-leases-v0.md`. It does not add a new requirement ID: the next free
`CAL-V0` ID, 210, is already used on an unmerged batch branch.

- When the local intent branch lacks the commit, `completeFacts` (`internal/tasks/store/lease_gate.go`)
  reads the branch's configured upstream with `git for-each-ref --format=%(refname)%00%(upstream)`.
  It accepts `merge-base --is-ancestor` against that upstream only when the upstream is under
  `refs/remotes/` and `show-ref --verify` finds it. Git runs under the same sanitized environment
  with no credential helper. Nothing is fetched, and no ref is created, moved or deleted.
- Fail closed: an unconfigured, unresolvable or absent upstream, or one that is a local branch
  (`branch.<b>.remote = .`), leaves the commit unreachable. The multi-repository
  `repositoryIntegrated` loop is unchanged.
- Criterion 2: the refusal now carries the new closed code `COMMIT_NOT_INTEGRATED` beside
  `STALE_TREE`. `STALE_TREE` stays first in the plan's codes, so existing callers that test for
  `STALE_TREE` and `Codes[0]` readers still work. The command-result envelope sorts its codes.
  A tree mismatch still refuses `STALE_TREE` alone. No item reason field existed for refused lease
  verbs, and adding a second code fits the existing `codes` array without changing any profile.
  The code is registered in `wire.Codes` (now 78), classified not retryable in `wire.RetryOf`, and
  listed in the spec's TCP-00 amendment A27 and in its retryable table.
- The detail names `refs/heads/<intentBranch>` and either the upstream ref checked or that none is
  configured. It also names the recovery: run `git fetch` so the remote-tracking ref contains the
  commit, or set the branch's upstream. A multi-repository refusal names the extra repository that
  is not integrated, which the old text misreported as unreachable from main.

## Non-goals

- No fetch, no ref moves, and no automatic upstream configuration.
- No change to the tip comparisons in `submit` and `gate run`, or to the multi-repository
  integration branches.
- V1-0557, the broader public-integration authority item, stays open. This change trusts the
  operator's own remote-tracking refs and nothing more.

## Failure modes

- A stale remote-tracking ref: the commit stays refused until the operator fetches. Detail and docs
  name that recovery.
- A remote-tracking ref that was moved by hand: it is accepted the same way as a hand-moved local
  branch. The trust boundary is the local repository's refs, as before.
- The upstream is configured but the remote-tracking ref is absent: refused, never an error.
- An older strict decoder of command results refuses one that carries `COMMIT_NOT_INTEGRATED` as an
  unknown code, as with earlier code additions. A refused completion writes nothing, so no stored
  state carries the code.

## Evidence

- `TestCALV0017_CompleteAcceptsAnUpstreamIntegratedCommit` (`internal/tasks/store`) covers five
  cases. With no upstream, an absent remote-tracking ref, an upstream behind the commit, or a local
  upstream branch, the commit is refused `[STALE_TREE, COMMIT_NOT_INTEGRATED]` and the store is
  unchanged. A tree mismatch is refused `STALE_TREE` alone. With the commit only on
  `refs/remotes/origin/main` and local `main` behind, `complete` succeeds and leaves both refs
  where they were.
- Existing CAL-V0-017 tests and the wire code-count and classification tests pass.

## Rollback

Revert the commit. No store format changes, and refusals write nothing, so a revert leaves no
state behind.

## Acceptance

Owner acceptance of the CAL-V0-017 amendment and of A27 is pending. The spec marks both as
proposed.
