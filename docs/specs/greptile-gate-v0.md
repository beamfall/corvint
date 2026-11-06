# Greptile Gate V0

Owner: Russell Lewis
Date: 2026-11-06
Requirement prefix: `GRG-V0`
Intent status: accepted (owner instruction 2026-11-06: make Greptile a required check, "Yes, build it")
Delivery status: experimental
Authoritative inputs: owner instructions 2026-11-06 (merge only at Greptile 5/5 on the current head with every Greptile thread resolved); `affected-plan-v0.md` AFP-V0-171 (user-posted success kept).

## Agent digest
- Claim: A trusted workflow posts the required `greptile-gate` status, `success` only when Greptile reviewed the exact PR head at 5/5 with no unresolved Greptile thread.
- Status: accepted / experimental until the hosted workflow has run and the ruleset requires it.
- Exists: `.github/workflows/greptile-gate.yml`; `TestGRGV0001GreptileGatePostsSuccessOnlyFor5Of5OnHead`.
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
  names that exact head as its last reviewed commit, scores it `5/5`, and no review thread opened
  by Greptile is unresolved; `pending` while no summary exists or the summary reviewed another
  commit; `failure` for any other score, an unreadable score, an unresolved Greptile thread, or more
  than 100 review threads (not provable). It runs on `pull_request_target` head events, on
  Greptile's own issue comments, and on `workflow_dispatch` with a PR number (resolving a thread
  raises no event). A closed or unreadable pull request gets no status. Every other refresh posts
  `pending` before it reads the review, so an unreadable status, comment or thread read stops the
  step leaving `pending` and never an earlier `success`. A `greptile-gate` success posted by a
  creator of type `User` on the head is kept (owner waiver, as AFP-V0-171); the waiver lookup reads
  every page of the head's statuses, so newer statuses of other contexts cannot hide it. A run
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

## Non-goals and failure modes

- Not a judgement of code quality beyond Greptile's own review; CI, `ci-control-plane` and
  `doc-gates` stay required.
- Greptile's comment format is not a published contract. A format change yields `pending` or
  `failure`, never a false `success`; the owner waiver unblocks a merge meanwhile.
- Thread resolution raises no workflow event; after resolving threads, re-run the gate with
  `gh workflow run greptile-gate.yml -f pr=N` or wait for Greptile's next comment edit.
- Any user with write access can already post a commit status, so the waiver widens nothing.

## Rollback

Drop `greptile-gate` from the main ruleset's required status checks, then delete the workflow.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GRG-V0-001 | the `gate` job of `.github/workflows/greptile-gate.yml` | `TestGRGV0001GreptileGatePostsSuccessOnlyFor5Of5OnHead` in `tools/ci-reuse-plan/greptile_gate_workflow_test.go` (23 cases including 8 merge-queue cases; a mutation accepting `0/5` fails it, and so do dropping the pending-first post, the merge-group refresh of either commit, or the statuses pagination) and `TestGRGV0001GreptileGateSerializesRunsPerPR` (per-PR concurrency key without cancellation, and 6 resolve-step cases; a mutation to `cancel-in-progress: true` fails it); GitHub's concurrency scheduling itself is not exercisable locally; a read-only dry run against PRs 112, 115 and 118 matched their Greptile summaries; hosted run and required-check enforcement `NOT_OBSERVED` until merged |
