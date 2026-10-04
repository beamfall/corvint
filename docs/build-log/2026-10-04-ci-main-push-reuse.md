# Main-push reuse of a passed pull-request result on an identical tree (V1-0717)

Human-owned intent: ticket V1-0717. Requirement: AFP-V0-024 (proposed).

## Finding

Read 2026-10-04: of the 65 most recent `main` push runs of CI, 27 succeeded, 32 were cancelled by
`cancel-in-progress`, 4 failed and 2 were still running (30 of 65 were cancelled when the ticket
was filed). So 31 of 65 push runs (48%) completed, counted over runs, not distinct commits. A cancelled push leaves that `main` commit with no completed result, although the
pull request it merged had usually just passed the same suite.

Of the 40 most recent first-parent merges on `main`, all 40 have a successful pull-request run
for the merged head, and for 19 the newest such run was created after the merge's first parent
was committed. Those 19 are an estimate of the pushes whose tested merge tree equals the pushed
tree; the exact tree was not retained by those runs, so the true count is `NOT_OBSERVED`.

## Change

- A full pull-request shard that passed retains an artifact named
  `ci-tested-tree-TREE-full-SHARD-of-SHARDS`. DOCS and selection-narrowed branches write none.
- On a push, `docs-plan` builds `tools/ci-reuse-plan` from the pushed commit, lists successful
  pull-request runs of `ci.yml` for the second parent and their artifacts, and sets mode `REUSE`
  only when one run holds a record of the pushed tree for all four shards. The shards then skip
  the root race invocation; every other check still runs. The decision is retained as `ci-reuse`.
- The record travels in the artifact name because listing public runs and artifacts needs no
  `actions` permission, while downloading artifact contents does. ARTIFACT-V0-008 and
  `script/check-ci-least-privilege.sh` are unchanged.

## Evidence and limits

- `go test ./tools/ci-reuse-plan`: the reuse case and 16 refusal cases pass.
- An unreadable run listing still retains a FULL decision with its reason.
- The push step was replayed locally against the live API for `main` at
  `be98482b` with an invalid token: the authenticated call returned 401, the anonymous retry
  succeeded, one successful run was found with no records, and the decision was FULL.
- Whether the hosted `GITHUB_TOKEN` with only `contents: read` can list runs and artifacts is
  `NOT_OBSERVED`; the anonymous retry shares the runner address's rate limit. Either failure
  runs FULL.
- Hosted reuse, the time saved and the fraction of `main` commits with a completed or reused
  result after the change are `NOT_OBSERVED` until merges land with matching records. The first
  eligible merge is the first one whose pull-request run already contained this workflow.
- Tree identity covers tracked files only. The commit id and history seen by the tests, the
  event environment, apt packages and the runner image are not compared between the reused run
  and the push. Independent review found no test whose verdict depends on them; that is
  unverified.
- Not decided here: whether `main` pushes should queue instead of cancel. That is an owner
  decision; this change leaves `cancel-in-progress` as it is.

## Rollback

Remove the `reuse` step from `docs-plan`. Every push then runs FULL; the record artifacts become
unused.
