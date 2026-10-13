# 2026-10-13 Identical-tree CI reuse across pull-request, merge-queue and main-push runs (V1-0760, V1-1360)

Branch `claude/ci-identical-tree-reuse` from `origin/main` 1fb2fbc64 (the merge of #866).
`AFP-V0-024` is amended for V1-0760, `AFP-V0-026` is amended for V1-1360, and `AFP-V0-046` is new
in `docs/specs/affected-plan-v0.md`. All three were written under the owner's standing auto-accept
of 2026-10-11, so their mode is `standing` until the owner reads them back. They are Corvint 1.0
scope under `PRS-V1-018`. Tickets V1-0760 and V1-1360 stay open here; the orchestrator holds
their leases and completes them from the merge.

## Why

The owner asked for this on 2026-10-13. The runs list for that day shows each merge running the
full 8-shard race matrix three times:

| Run | Typical time |
| --- | --- |
| `pull_request` | about 25 minutes |
| `merge_group` | about 20 minutes |
| `main` push | about 9 minutes when AFP-V0-024 reuse hits, about 20 otherwise |

The `beamfall` organization is on the GitHub free plan, which allows 20 concurrent hosted jobs.
One CI run holds 13 jobs, so queue entries are bound by runners. Raising `max_entries_to_build`
cannot add parallelism until the cost of each run drops. When the tree is identical by
construction, this change removes two of the three full race invocations.

## Decision

1. **The main push reuses the queue run (V1-0760).** With merge method `MERGE`, the queue
   commit is the commit that becomes `main`. A push can now admit either of two runs:
   - a successful `pull_request` run of the merged head (the existing case);
   - a successful `merge_group` run whose `head_sha` is the pushed commit itself.

   The AFP-V0-024 record step in `Go tests` now also writes
   `ci-tested-tree-TREE-full-S-of-N` on `merge_group`. It still writes only from the full branch,
   so a narrowed, DOCS or REUSE shard never reaches it.
2. **The queue reuses the identical-tree PR run (V1-1360).** The `docs-plan` `reuse` step now
   also runs on `merge_group`. It requires a two-parent queue commit whose first parent is an
   ancestor of the checked-out `refs/remotes/origin/main`. It then admits only a successful
   `pull_request` run of `^2` whose records name the queue commit's tree.
   - The first entry on an unmoved `main` has the parents of the PR merge ref, so the trees are
     equal.
   - A later entry built on a preceding queue commit fails the first-parent test or has a
     different tree, so it runs FULL.
3. **Admission contract.** `tools/ci-reuse-plan` takes two new required flags:
   - `--event push|merge_group`;
   - `--target COMMIT`, the tested commit.

   `--runs` may repeat, and a push passes both the `pull_request` and `merge_group` listings.
   - On `push`, `admits` accepts a `pull_request` run with `head_sha == --head` or a
     `merge_group` run with `head_sha == --target`.
   - On `merge_group`, it accepts only the `pull_request` run.
   - Any other event admits nothing.

   An artifact counts only when its `workflow_run` id and head both match the admitted run. The
   decision document `corvint-ci-reuse/0` keeps its closed member set. `head` always means the
   tested commit's second parent (the merged PR head), whichever run is admitted, and `runId`
   names the reused run. So the jq admission check in `ci.yml` is unchanged and still exact.
4. **Tool provenance.** On `merge_group` the tool is built from `git archive` of the queue
   commit's first parent, and only once that parent is shown to be on `main`. A PR that edits
   `tools/ci-reuse-plan` therefore cannot decide its own queue run. On `push` the tool is still
   built from the pushed `main` commit, as before.
5. **Unchanged:**
   - The gocache restore stays keyed on the mode.
   - The `main`-only cache save already requires the race step's success, which a REUSE shard
     skips, so it never saves on REUSE.
   - The AFP-V0-025 share report stays `pull_request`-only.
   - `ci-shard-cost-drift` abstains without every shard's outcomes.
   - AFP-V0-027 (`ci:batched`), the shard matrix and the ruleset are untouched.

## Replay evidence (live API, read-only)

A local replay of the step (`gh api` listings, the new tool, the same arguments) covered the 25
most recent first-parent merges on `main`:

- 13 had a PR run whose 8 records all name the merged tree.
- For `b8c22fb35`:
  - the `merge_group` replay returned REUSE of PR run 38464173485;
  - the `push` replay returned REUSE of the same run.
- For `1fb2fbc64`:
  - both replays returned FULL;
  - its queue run 38469561645 was admitted, but it predates the `merge_group` record step and
    so has no records.

These replays exercise the decision logic only. They are not a hosted run.

## Verification

- `GOTOOLCHAIN=local go test -count=1 -race ./tools/ci-reuse-plan/...`: ok. It covers the
  admitted queue run and each refusal: failed, cancelled, in progress, other workflow path,
  other repository, wrong head, partial records, different shard count, other tree, records of
  another head, unavailable artifacts, a merged-head PR run offered as a queue run, and a PR run
  with the wrong head offered to a `merge_group` decision.
- `TestAFPV0046WorkflowWiresQueueReuse` binds the step and record conditions, the event and
  target arguments, the queue listing and the first-parent check in `ci.yml`.
- `go vet ./tools/ci-reuse-plan/...`, `go build ./...` and `actionlint .github/workflows/ci.yml`
  all passed.
- The 12 CI doc gates and `gofmt` passed.

## Limits

- **Hosted behavior is `NOT_OBSERVED`.** Neither a queue run reusing a PR run nor a main push
  reusing a queue run has been seen on hosted runners. This PR's own queue run builds the
  previous tool from `main`, which lacks `--event`. Its decision step therefore fails under
  `continue-on-error`, and that run is FULL and writes records. That makes this PR's own `main`
  push the first hosted candidate for V1-0760 reuse.
- **A REUSE queue run has no race outcomes of its own.** AFP-V0-029 shadow rows (V1-0811) must
  take them from the reused pull-request run. That is recorded in AFP-V0-046 but not implemented
  here. AFP-V0-028 and AFP-V0-029 wording is unchanged.
- **`ci-control-plane` still posts on the completed `merge_group` run.** A run still in progress
  is never admitted.
- **The second listing is best effort.** An unreadable `merge_group` listing on a push leaves
  only the PR candidate.

## Rollback

- Restore `if: github.event_name == 'push'` on the `reuse` and `Retain reuse decision` steps.
- Restore the `pull_request`-only record step.
- Drop the `merge_group` listing.
- Revert `tools/ci-reuse-plan` to its single-event form.

Removing the `reuse` step altogether restores FULL for every event.
