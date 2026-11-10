# CI Control Plane Early Post V0

Owner: Russell Lewis
Date: 2026-11-08
Requirement prefix: `CCEP-V0`
Intent status: proposed (owner instruction 2026-11-08: ci related tickets should be fixed asap, always; V1-5102)
Delivery status: experimental

## Agent digest
- Claim: Post the required CI control-plane status on trusted PR activity without waiting for CI completion.
- Status: proposed (owner instruction 2026-11-08: ci related tickets should be fixed asap, always; V1-5102)/experimental
- Exists: `.github/workflows/ci-control-plane.yml` and `tools/ci-reuse-plan/control_plane_workflow_test.go`.
- Blocked on: hosted qualification; local tests use API fixtures and perform no network writes.
- Read next: Requirements; Acceptance evidence; Rollback.

## Requirements

- `CCEP-V0-001`: (proposed, owner instruction 2026-11-08; V1-5102; amends AFP-V0-016/079/855
  and the trigger restriction of decision 0320; preserves decision 0390's required checks;
  amended by owner decision 2026-11-10 for V1-5263, which replaces the refusal at the files
  API ceiling and AFP-V0-016's failure for a PR above that ceiling with the tree comparison
  below)
  The trusted `.github/workflows/ci-control-plane.yml` SHALL start its metadata-only
  `control-plane` job on `pull_request_target` activity `opened`, `synchronize`, `reopened`
  and `ready_for_review` targeting `main`, without waiting for any CI job or conclusion. The
  job MUST never check out or execute PR code, use actions, or interpolate event values into
  `run:`; event inputs enter only through environment variables. Its only permissions SHALL
  be `contents: read`, `pull-requests: read` and `statuses: write`.

  The job SHALL select the event's exact PR head SHA, read the PR through REST, require that
  it remains open at that head before listing files, and read every files page including
  renamed-from paths. The REST files endpoint is PR-number-addressed, not SHA-addressed:
  the job SHALL fence that listing with head/base checks, require the returned file count to
  equal `changed_files`, require full 100-file nonfinal pages and the exact final-page
  count, reject empty or duplicate filenames, and refuse missing file metadata or any
  failed API call without posting any status. Missing inputs or malformed event identity,
  PR number, repository or head SHA MUST post nothing; only `workflow_run`
  may use the PR lookup fallback. A refused listing MUST NOT be overridden by
  a waiver. Immediately before posting on the captured head SHA, it SHALL re-read the PR and
  require the same open state, head SHA, base SHA and changed-file count. A mismatch posts
  nothing. This is revalidation, not an atomic GitHub snapshot or compare-and-swap guarantee.

  Large pull requests (V1-5263, owner decision 2026-11-10). The files endpoint lists at most
  3000 files, so a listing of a PR whose `changed_files` is 3000 or more cannot be proved
  complete. For such a PR, and only for such a PR, the job SHALL list no file and SHALL
  instead compare Git trees, in this order:

  1. Take the PR's `base.ref`, require a string of at most 255 characters made of
     `/`-separated segments that each start with a letter or digit and continue with
     letters, digits, `.`, `_` or `-`, with no `..`, and read that branch's current tip
     through `git/ref/heads/<ref>`. The answer MUST name exactly that ref and a commit.
     The PR's `base.sha` MUST NOT be the comparison point: it can be a stale ancestor of
     the branch, and a comparison against it can hide a `.github/` difference.
  2. Read the merge base of that tip and the captured head through the compare endpoint,
     requiring the answer's `base_commit` to be that tip.
  3. For the tip, for the merge-base commit and for the captured head, in that order, read
     the commit, require the answer to be for that commit, read its root tree
     non-recursively, and require the answer to be for that tree with `truncated` exactly
     `false` and every entry an object whose `path` is a string. Take the entry whose path
     is exactly `.github`; more than one such entry is malformed.

  Every SHA read from the API MUST match 40 lower-case hexadecimal characters, and the ref
  name MUST pass the rule above, before it is placed in a URL. The result is clean, with
  the same success status and description as a clean listing, only when none of the three
  commits has a `.github` entry or all three have a `.github` entry of type `tree` with the
  same SHA. Any other combination is a `.github/` change and SHALL take the listing path's
  failure, including the approver-waiver lookup, unchanged: an entry present at one commit
  and absent at another, a different SHA at any of the three, or an entry of a type other
  than `tree` at any of them, even when all three carry the same non-tree entry. In that
  last case nothing under `.github/` differs, yet the job logs `not-a-tree` for each commit
  and posts the listing path's failure description, `PR changes .github/; merge needs an
  approver ci-control-plane-waiver status on this SHA`, unless an approver waived it.
  Because the tip is compared too, a PR whose base branch changed `.github` after the merge
  base fails, even when the PR itself left `.github` alone, until the PR merges the base
  branch or an approver waives it. That is intended and fail-closed. A failed read, a
  missing or malformed field, a truncated tree or a failed validation MUST post nothing and
  fail the job, and a waiver MUST NOT override it. The revalidation before posting is
  unchanged. Below 3000 changed files the job's behaviour and API calls SHALL stay exactly
  those of the listing path.

  Known limits of the comparison. The status binds to the tip, merge base and head observed
  when the job ran: like the listing, it is revalidated, not an atomic snapshot. The base
  branch can move afterwards, including a `.github` change, without a new status until the
  PR is updated. When the tip and the head have more than one merge base, GitHub reports
  one. Requiring the tip and the head to agree closes the case where that choice hides a
  `.github/` difference between the two: when the tip and the head carry the same entry, a
  merge commit of the two carries it too, whichever merge base the merge uses, so the merge
  cannot change `.github/` on the base branch as observed. What stays open is that the
  reported merge base can still make the check stricter than necessary (a failure when the
  tip and head agree but that merge base differs), and the listing path below 3000 files
  still depends on GitHub's choice. Equal tree SHAs prove
  identical content under `.github/`, which is at least as strong as a listing with no
  `.github/` path against the same merge base. A base branch whose name is outside the
  rule above is refused, never guessed.

  The existing `workflow_run` completed-CI path SHALL remain a fallback and rerun path,
  including AFP-V0-855's cancelled/superseded admission rule and the V1-5096/AFP-V0-171
  approver-only `ci-control-plane-waiver` lookup on the exact head. Both PR triggers SHALL
  share the `ci-control-plane-pull_request-<head sha>` concurrency group; the merge-group job
  and its status semantics SHALL remain unchanged. A clean complete listing posts success;
  a changed or renamed-from `.github/` path posts failure unless the newest authorized waiver
  on that head is success. No CI conclusion grants this status.

## Non-goals

Greptile re-review, auto-merge arming, ruleset changes, hosted writes and executing PR code.
This companion proposes amendments to AFP-V0-016/079/855 and the trigger restriction of
decision 0320; it preserves decision 0390's required checks and AFP-V0-171's waiver policy.
It does not add obligations to `affected-plan-v0.md`.

## Failure modes

API errors, missing inputs, incomplete/malformed pagination, duplicate filenames and moving
heads abstain. A write failure fails the job. A waiver cannot override unreadable file evidence.
At 3000 or more changed files (V1-5263) the same holds for the tree comparison: an unsafe or
missing base ref name, a ref answer for another ref or a non-commit object, a missing or
malformed merge base, a compare answer for another base, a commit or tree answer for another
object, a truncated or malformed root tree (including an entry that is not an object or whose
`path` is not a string), a duplicate `.github` entry, a SHA that is not 40 lower-case
hexadecimal characters and any failed read, including one that prints a body and then exits
non-zero, abstain and fail the job. Before V1-5263 the job refused every such PR, so a PR of
3000 or more files could never merge even when it changed nothing under `.github/`; PR 169
(4,523 files) was the observed case.
GitHub offers no atomic transaction spanning PR revalidation and the status POST; the POST
always targets the captured head. Event suppression and runner latency can delay the early
path, so the completed-CI fallback remains.

## Acceptance evidence

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| CCEP-V0-001 | `.github/workflows/ci-control-plane.yml` metadata-only job | `TestV15102ControlPlaneStartsOnPRActivity`, `TestV15102ControlPlaneMetadataBoundToHead`, `TestV15263ControlPlaneLargePRComparesGitHubTrees`, `TestV15096ControlPlaneWaiverNeedsAnApprover`, `TestAFPV0079ControlPlaneReportsEveryCompletedRun`, `TestAFPV0079CancelledRunStillPostsTheStatus`, `TestAFPV0855SupersededCancelledRunStartsNoJob`; `actionlint`; `docs/build-log/2026-11-08-ci-control-plane-early-post.md`, `docs/build-log/2026-11-10-control-plane-large-pr.md`; hosted execution and timing NOT_OBSERVED |

The tests execute the actual workflow shell against API fixtures, proving a complete fixture
before injecting failures and asserting the API call trace and absence of status writes.
They cover both triggers, valid multiple pages, short pages, missing
inputs/metadata, duplicate names, stale heads/base/count, renamed paths, API failures and
existing authorized/unauthorized waiver behavior.

For V1-5263, `TestV15263ControlPlaneLargePRComparesGitHubTrees` covers 3000, 3001 and 4,523
files: equal `.github` trees at the tip, merge base and head, and no `.github` at any of
them, post success, and the log names all three entries and commits; a different SHA, an
added, removed or non-tree entry, `absent` against `not-a-tree`, the same non-tree entry at
all three, a tip that differs from a merge base the head equals, and a head that equals the
tip while both differ from the merge base post failure without a waiver, success with an
approver's waiver and failure with another writer's or a withdrawn one, on both triggers for
the no-waiver and approver-waiver cases; a stale `base.sha` whose tree agrees with the head
while the tip's merge base does not posts failure and no read names `base.sha`; each unsafe
ref name, failed read (including one that prints a valid body and then exits non-zero),
mismatched, malformed or truncated answer, a root tree with a null entry, an entry without
`path` or with a non-string `path`, and a failed read of the tip's commit or tree posts
nothing and fails the step even with a waiver, on both triggers; a PR that moved before the
POST posts nothing; and below 3000 files the exact listing call trace and POST arguments
hold with every tree fixture on offer. `TestV15102ControlPlaneMetadataBoundToHead` keeps the
ceiling cases: 3000 and 3001 files without a base ref post nothing.

API reads. On the clean `pull_request_target` path the comparison makes exactly ten GET
requests and one POST: the PR, `git/ref/heads/<ref>`, the compare of the tip and the head,
then the commit and root tree of the tip, of the merge base and of the head (six reads,
made even when the tip is the merge base), and the PR again; the POST is the status. A
`.github/` difference adds the paginated statuses read for the waiver, and the
`workflow_run` trigger adds the paginated open-PR lookup first. The earlier two-commit
version was run once read-only against PR 169 (4,523 files) with the status POST
intercepted and never sent: eight GET requests (the PR, `git/ref/heads/main`, the compare,
two commits, two trees and the PR again), `.github` tree
`f01aa5d9c62bf157d5486dd0b5fa4f61bebc7077` at the merge base and the head, result success.
That run used the owner's token on macOS. The three-commit version has not been run against
a live PR, and execution under the hosted Actions token stays NOT_OBSERVED for both.

## Rollback

Revert the PR trigger and shared metadata changes to restore the completed-CI-only path;
leave required checks, rulesets and waiver approvers intact. Revert this companion and its
registration with the implementation if the proposed amendment is withdrawn.

The V1-5263 amendment rolls back on its own, in two reverts. The workflow change merged
first, alone, as commit `47c053e6b` (PR 171); its test and this text followed in a second PR.
Revert `47c053e6b` to restore the refusal at 3000 or more changed files. That is fail-closed,
and such a PR again cannot merge until it is split below the ceiling. Revert the second PR's
commit with it, since its test asserts the tree comparison and fails without the workflow
change. Reverting the second PR alone leaves the tree comparison active.
