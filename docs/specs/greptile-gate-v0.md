# Greptile Gate V0

Owner: Russell Lewis
Date: 2026-11-06
Requirement prefix: `GRG-V0`
Intent status: accepted (owner instruction 2026-11-06: make Greptile a required check, "Yes, build it")
Delivery status: experimental
Authoritative inputs: owner instruction 2026-11-08 ("figure out why this is so difficult, and fix it": carry 5/5 across unchanged PR diffs); owner instructions 2026-11-06 (merge only at Greptile 5/5 on the current head with every Greptile thread resolved); `affected-plan-v0.md` AFP-V0-171 (user-posted success kept).

## Agent digest
- Claim: The required gate accepts Greptile 5/5 on the head or an unchanged PR diff, with no unresolved Greptile threads.
- Status: accepted (owner instruction 2026-11-06: make Greptile a required check, "Yes, build it") / experimental
- Exists: `.github/workflows/greptile-gate.yml`; `TestGRGV0001GreptileGatePostsSuccessOnlyFor5Of5OnHead`; `TestGRGV0002CarryOnlyUnchangedPRDiff`.
- Blocked on: merge, a first hosted run, and adding `greptile-gate` to the main ruleset's required checks.
- Read next: Requirements; Non-goals and failure modes.

## Problem and current state

Greptile reviews every pull request but posts no check run or commit status, only a summary issue
comment (edited in place on re-review) and inline review threads. An armed pull request therefore
merges on green CI whatever its Greptile score: #100 and #117 merged at 4/5, the latter on a stale
review of an older head. The owner's merge rule (5/5 on the current head, every Greptile thread
resolved) was enforced only by the orchestrator remembering to check it.

## Requirements

- `GRG-V0-001`: `.github/workflows/greptile-gate.yml` SHALL post the `greptile-gate` commit status on
  an open pull request's head from GitHub metadata alone, never checking out pull-request code:
  `success` only when the newest `greptile-apps[bot]` summary comment carrying `Confidence Score`
  names that exact head as its last reviewed commit (or satisfies GRG-V0-002), scores it `5/5`, and no review thread opened
  by Greptile is unresolved; `pending` while no summary exists or the summary reviewed another
  commit (except the GRG-V0-002 carry below); `failure` for any other score, an unreadable score, an unresolved Greptile thread, or more
  than 100 review threads (not provable). It runs on `pull_request_target` head events, on
  Greptile's own issue comments, and on `workflow_dispatch` with a PR number (resolving a thread
  raises no event). A closed or unreadable pull request gets no status. Every other refresh posts
  `pending` before it reads the review, so an unreadable status, comment or thread read stops the
  step leaving `pending` and never an earlier `success`. An owner waiver (V1-5096) is the newest
  `greptile-gate-waiver` status on the head created by a login in the step's `WAIVER_APPROVERS`
  (the same list as AFP-V0-171, matched case-insensitively); when it is `success` the gate posts
  `success` itself, and any other writer's status, including a hand-posted `greptile-gate`
  success, is ignored. The waiver lookup reads every page of the head's statuses, so newer
  statuses of other contexts cannot hide it, and an unreadable lookup stops with `pending`. A run
  outside the queue also posts on the pull request's active merge-group commit (read from its
  `mergeQueueEntry`; an unreadable entry stops with `pending`), so a later Greptile edit refreshes a
  queued pass. Because reopening a
  thread raises no event, a `merge_group` run re-evaluates the queued pull request named by the
  queue ref (`gh-readonly-queue/<base>/pr-<N>-<sha>`) with fresh reads of its head and posts the
  result on the head and the merge-group commit (or `success` on the merge-group commit for an
  owner waiver on that head); an unparsable queue ref or commit stops with no post, so the queue
  entry fails. Runs for one pull request are serialized: a `resolve` job names the pull request for
  every event, and the gate job's concurrency group is keyed by that number with
  `cancel-in-progress: false`, so an older run (for example a `merge_group` run that read `5/5`)
  finishes before a newer one reads and cannot post a stale pass over a newer result.

- `GRG-V0-002`: When the newest Greptile summary scores its last reviewed commit R exactly
  `5/5` and R differs from the PR head H, the gate SHALL carry that review only when GitHub
  metadata proves the PR-authored diff unchanged. Read the PR base branch ref and compare
  `<base_ref>...R` and `<base_ref>...H` using the GitHub compare API (diffs from their merge
  bases), never checking out or executing PR code. Canonicalize each file to `(status, filename,
  previous_filename, patch)`, sort the file set, and keep every patch byte, `@@` hunk coordinates included, so a
  shifted or moved hunk refuses carry. Exclude only
  `.corvint/change.cem.json` and `.corvint/changes/*.cem.json` (direct children); a rename from
  a non-evidence path is not excluded. Carry only when the canonical sets are byte-identical
  and `compare/R...H` reports `ahead` or `identical`. A patch omits the file mode, so carry also
  SHALL require the GraphQL `Commit.file(path:).mode` of every canonical filename and
  previous_filename to match at R and H (an absent path reads as null); a failed mode read or an
  answer without one mode per path refuses carry. A failed base/compare read, missing files
  array, either diff compare reporting 300 or more files (before exclusions), pagination or
  truncation, or any non-excluded file without a string patch SHALL refuse carry. A score other
  than exactly `5/5` or non-ancestor R also refuses carry. Refusals retain `pending` and the
  existing `ask @greptileai to re-review` description. A carried pass still requires zero
  unresolved Greptile threads and no more than 100 threads, and posts
  `Greptile 5/5 on <R9>; head <H9> adds only base merges and change evidence`. Apply the same
  rule to the PR head on `merge_group`, posting to both existing targets; preserve pending-first
  withdrawal, owner waiver, serialization and all other GRG-V0-001 behavior. Owner authority:
  2026-11-08, "figure out why this is so difficult, and fix it".

## Non-goals and failure modes

- Not a judgement of code quality beyond Greptile's own review; CI, `ci-control-plane` and
  `doc-gates` stay required.
- Greptile's comment format is not a published contract. A format change yields `pending` or
  `failure`, never a false `success`; the owner waiver unblocks a merge meanwhile.
- Thread resolution raises no workflow event; after resolving threads, re-run the gate with
  `gh workflow run greptile-gate.yml -f pr=N` or wait for Greptile's next comment edit.
- Only `WAIVER_APPROVERS` (read from the default-branch workflow) can waive; anything holding an
  approver's token still can. Ignoring hand-posted `greptile-gate` statuses at merge time also
  needs the ruleset to bind the required context to GitHub Actions (owner step).

## Rollback

For GRG-V0-002 alone, remove the carry branch to restore exact-head-only reviews.

Drop `greptile-gate` from the main ruleset's required status checks, then delete the workflow.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GRG-V0-001 | the `gate` job of `.github/workflows/greptile-gate.yml` | `TestGRGV0001GreptileGatePostsSuccessOnlyFor5Of5OnHead` in `tools/ci-reuse-plan/greptile_gate_workflow_test.go` (26 cases including 8 merge-queue cases and 4 V1-5096 waiver-authority cases; a mutation accepting `0/5` fails it, and so do dropping the pending-first post, the merge-group refresh of either commit, or the statuses pagination) and `TestGRGV0001GreptileGateSerializesRunsPerPR` (per-PR concurrency key without cancellation, and 6 resolve-step cases; a mutation to `cancel-in-progress: true` fails it); GitHub's concurrency scheduling itself is not exercisable locally; a read-only dry run against PRs 112, 115 and 118 matched their Greptile summaries; hosted run and required-check enforcement `NOT_OBSERVED` until merged |
| GRG-V0-002 | metadata-only normalized comparisons in the `gate` job | `TestGRGV0002CarryOnlyUnchangedPRDiff` exercises carry and refusal cases on both PR and merge-group events, including changed lines, missing patches, failed reads, non-5/5 scores, evidence-only changes, ancestry, pagination, the 300-file limit, unresolved threads, and a file mode that differs at R and H or cannot be read in full; identical-diff cases fail against the unmodified workflow; hosted carry `NOT_OBSERVED` |
