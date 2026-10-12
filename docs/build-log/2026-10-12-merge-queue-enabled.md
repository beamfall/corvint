# 2026-10-12 Merge queue enabled on `main` (CORE-GITHUB-MERGE-QUEUE-COMPATIBILITY-1)

The owner enabled GitHub's merge queue on `main` on 2026-10-12 by adding a `merge_queue` rule to
ruleset 23699808 (`updated_at` 2026-10-12T14:12:55-04:00). The CI side was already on `main`:
`ci.yml` runs on `merge_group` `checks_requested` and is FULL there, and `ci-control-plane.yml`
posts `ci-control-plane` `success` on the head of a completed `merge_group` run whose branch
begins `gh-readonly-queue/main/pr-` (AFP-V0-026). This entry records the configuration choices.
The pull request that carries this entry is the first one to merge through the queue.

## Configuration

The ruleset keeps its `pull_request` rule (merge commits only) and its required checks
`go-product`, `ci-control-plane` and `doc-gates`, with no bypass actor. The new rule:

| Parameter | Value | Reason |
| --- | --- | --- |
| `merge_method` | `MERGE` | AFP-V0-016: the queue commit is the merge commit that becomes `main` |
| `max_entries_to_merge` | 1 | AFP-V0-016: maximum group size 1 |
| `min_entries_to_merge` | 1, wait 0 minutes | no batching delay |
| `max_entries_to_build` | 2 | caps speculative full CI runs at two at a time (runner cost, V1-0809) |
| `grouping_strategy` | `ALLGREEN` | every entry's own run must pass |
| `check_response_timeout_minutes` | 120 | must exceed one full `CI` run plus the `ci-control-plane` run that follows it: recent full runs took 10 to 38 minutes, and the shard job is capped at 75 |

GraphQL `mergeQueue(branch: "main")` read back `mergeMethod MERGE`, `maximumEntriesToMerge 1`,
`maximumEntriesToBuild 2` and `checkResponseTimeout 7200` seconds.

## Cost and limits

- Every merge now runs a full `CI` run on the queue commit, and the `main` push runs another,
  until V1-0760 lets the push reuse the queue run of the identical commit.
- Hosted behavior stays `NOT_OBSERVED` until the first queued merge: the queue branch form,
  `workflow_run` delivery for `merge_group` runs (without it `ci-control-plane` is never posted
  on the queue commit and the entry is ejected after 120 minutes), and `main` advancing to the
  tested queue commit. The ticket records that merge's evidence.
- Failed or stale group handling is not exercised by this change.

## Rollback

Restore the previous ruleset (the same body without the `merge_queue` rule) with
`gh api -X PUT repos/beamfall/corvint/rulesets/23699808 --input <rules without merge_queue>`.
AFP-V0-026's order applies only once PR narrowing exists: restore FULL PR runs first, then
disable the queue. Narrowing is not enabled, so disabling the queue alone is a complete rollback
today; the `merge_group` trigger stays inert without it.
