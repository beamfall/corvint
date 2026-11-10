# V1-5263 — Let a 3000-file PR pass the control plane by comparing `.github` trees

Owner decision 2026-11-10, ticket V1-5263 (confirmed bug). Base and initial HEAD:
`9684e632e49bb54584a00ab44f3a7a27454d67c5` (`origin/main`), branch
`claude/control-plane-large-pr`. Contract: `CCEP-V0-001` in
`docs/specs/ci-control-plane-early-post-v0.md`, amended under the same ID; intent stays
proposed and delivery experimental.

## Problem

The `control-plane` job of `.github/workflows/ci-control-plane.yml` refused every PR whose
`changed_files` was 3000 or more: it printed `files API limit reached; no status posted` and
exited 1. The refusal was deliberate, because the REST files endpoint lists at most 3000 files
and a longer listing cannot be proved complete. But `ci-control-plane` is a required status on
`main` with no bypass actor, and a refusal cannot be waived, so such a PR could never merge
even when it changed nothing under `.github/`. PR 169 (4,523 files) was the live case.

## Decision

Keep the job fail-closed and keep the listing path untouched below 3000 files. At 3000 or more
the job lists no file. It reads the base branch's current tip, the merge base of that tip and
the captured head, and the root tree of the tip, of the merge-base commit and of the head, and
compares the one entry named `.github` at all three. Git tree SHAs are content addresses, so
equal SHAs at all three, or no entry at any, prove that no path under `.github/` differs
between them. That answers the listing's question, with no pagination to trust. Any
difference (added, removed, another SHA at any of the three, or an entry that is not a tree)
is treated exactly like a listed `.github/` path: failure, then the unchanged approver-waiver
lookup. When all three carry the same non-tree entry nothing differs, but the job still logs
`not-a-tree` for each commit and posts the usual failure description, `PR changes .github/;
merge needs an approver ci-control-plane-waiver status on this SHA`, unless an approver
waived it; a `.github` that is not a directory is not a state this repository expects. Any
failed read, mismatched, malformed or truncated answer posts nothing and fails the job, and no
waiver can override it. The reads need only `contents: read`, which the job already held;
permissions, triggers, the waiver lookup, the revalidation before the POST and the POST are
unchanged.

The tip is compared as well as the merge base so that the answer does not depend on which
merge base GitHub reports when there are several. The intended, fail-closed consequence: a
large PR whose base branch changed `.github` after the merge base fails, even when the PR
itself left `.github` alone, until the PR merges the base branch or an approver waives it.

Reads were added beyond the minimum to bind each answer to its question. The job reads
each commit object and takes its tree SHA, then reads that tree, instead of handing a commit
SHA to the trees endpoint and relying on it to peel the commit. It also requires the ref
answer to be one object naming exactly `refs/heads/<ref>` and a commit, so a list of refs or
an answer for a ref that merely shares the prefix is refused, and requires the compare
answer's `base_commit` to be the tip it asked about. Every SHA is checked against 40
lower-case hexadecimal characters and the ref name against a strict allow-list before either
reaches a URL. A root tree is refused unless every entry is an object with a string `path`, so
a null entry or one without a path cannot hide a `.github` entry by being skipped.

On the clean `pull_request_target` path the comparison makes exactly ten GET requests and one
POST: the PR, `git/ref/heads/<ref>`, the compare, the commit and root tree of the tip, of the
merge base and of the head (made even when the tip is the merge base), and the PR again. A
difference adds the paginated statuses read; the `workflow_run` trigger adds the paginated
open-PR lookup first.

## Rejected alternatives

- Split the PR below 3000 files. It works around the check without fixing it, cannot be done
  for a change that is one unit (a generated tree, a vendored import, a history import), and
  leaves the next large PR blocked the same way.
- Owner merges by hand. The main ruleset has no bypass actor by design; adding one, or dropping
  the required status for one merge, removes the guard for every PR for as long as it is off
  and leaves no status on the head that says `.github/` was checked.
- Compare against the PR's `base.sha`. Rejected because it is a recorded value, not the point
  the merge uses: it can lag behind the base branch, so the head can agree with it on
  `.github` while the head differs from the current tip's merge base, and a comparison against
  it would then post success for a PR that changes `.github/` relative to the branch it merges
  into. The job never reads it for the comparison. A test holds a fixture where the stale base
  agrees with the head and the tip's merge base does not, and requires failure with no read
  naming `base.sha`.
- Recursive tree read, or reading the `.github` subtree. Unneeded: the root entry's SHA already
  covers everything below it, and a recursive read brings back truncation on a large tree.
- Raising or paginating past the files ceiling. GitHub does not offer it.

## Verification and retained limits

- `TestV15263ControlPlaneLargePRComparesGitHubTrees` runs the workflow's real shell against a
  `gh` spy that serves each read by exact URL and records the call trace. A second round added
  the tip comparison, the entry-shape guard, reads that print a valid body and then exit
  non-zero, null and path-less entries, `absent` against `not-a-tree`, the three-way cases
  (tip differs while the head equals the merge base, head equals the tip while both differ
  from the merge base, all three equal, a failed read of the tip's commit or tree), and every
  difference and refusal on the `workflow_run` trigger as well. The fixture runner
  was hoisted out of `TestV15102ControlPlaneMetadataBoundToHead` unchanged so both tests share
  it; that test's two ceiling cases now assert that 3000 and 3001 files without a base ref post
  nothing. Focused run on linux/amd64, go1.27.3, through `script/heavy.sh`: pass.
- Scratch mutation check, not kept: eight single weakenings of the new branch (compare against
  `base.sha`, drop the truncation test, accept an equal non-tree entry, drop the ref allow-list,
  drop the compare-base binding, move the threshold to 3001, accept a duplicate entry, drop the
  tree-answer binding) each failed the focused tests. In the second round, dropping the
  tip-against-merge-base comparison failed the `tip differs from the merge base` and
  `absent at the tip only` cases, and dropping the entry-shape guard failed the null-entry,
  path-less and non-string-path refusals.
- Read-only dry run of the first, two-commit version against PR 169 with the status POST
  intercepted and never sent: eight GET requests (the PR, `git/ref/heads/main`, the compare,
  two commits, two trees and the PR again); `.github` was tree
  `f01aa5d9c62bf157d5486dd0b5fa4f61bebc7077` at the merge base and the head; result success.
  It ran with the owner's token under macOS bash 3.2. The three-commit version was not run
  against a live PR (NOT_RUN), so whether PR 169's base branch has moved `.github` since its
  merge base is NOT_OBSERVED. Execution under the hosted Actions token, and the read
  permissions it needs there, stay NOT_OBSERVED.
- `actionlint .github/workflows/ci-control-plane.yml` passes. No repository-wide gate was run.
- Retained limit, state at check time: the status binds to the tip, merge base and head seen
  when the job ran. The base branch can move afterwards, including a `.github` change, without
  a new status until the PR is updated. The listing path has the same property; both are
  revalidated, neither is an atomic snapshot.
- Retained limit, several merge bases: for a criss-cross history GitHub reports one merge
  base. Comparing the tip as well closes the case where that choice hides a difference: with
  the same entry at the tip and the head, a merge commit of the two carries that entry
  whichever merge base the merge uses. Still open: the reported merge base can make the check
  stricter than needed (failure when the tip and head agree but that merge base differs), and
  the listing path below 3000 files still depends on GitHub's choice.
- Retained limit, ref names: a base branch whose name falls outside the allow-list (legal in
  Git, for example a segment starting with `_`) is refused. The PR trigger targets `main`.
- `AFP-V0-016` in `affected-plan-v0.md` still says a PR above the listing limit fails. That
  spec is at its obligation limit and was not edited; `CCEP-V0-001` records the amendment.
- This change touches `.github/` and other paths, which `script/github-split-check.sh`
  refuses in one range for any commit layout. It landed as two PRs: the workflow change
  alone as commit `47c053e6b` (PR 171), then the spec, this entry and the test.

## Rollback

Revert the workflow commit `47c053e6b`. The job again refuses 3000 or more changed files,
which is fail-closed. Revert the second PR's commit with it: its test asserts the tree
comparison. Reverting the second PR alone leaves the tree comparison active.
