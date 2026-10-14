# 2026-10-14 STR-V0 promotion evidence so far: shadow parity, forward rows, timing baseline (V1-1257)

Human-owned intent: `docs/specs/session-test-runner-v0.md`, "Acceptance evidence: promotion and
kill criteria". Promotion from experimental needs three things:

- the STR-V0-013 parity report;
- 20 merged PRs through the queue on the new CI call site, with complete forward rows and no trip;
- median and p90 PR wall time and runner-minutes per merged PR, before and after.

This entry records what is observable now. The CI call site is still the step 3 shadow: a
`corvint test --dry-run` beside a driver that runs every shard in full. This entry is evidence
only. It changes no spec status, workflow, code or `Delivery status`, and promotes nothing.

Snapshot: GitHub artifacts and runs listed up to 2026-10-14T15:45:10Z. BASE is
`92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431`. The scripts are retained in
`evidence/v1-1257/scripts/`. All reads were read-only (`gh run download`, `gh api`).

## Per-acceptance-clause status

| Clause | Status |
| --- | --- |
| Parity (STR-V0-013) over the retained shadow rows | Met over 14 shadow runs (112 PR shards), with no unexplained difference. Residual: the selector-to-shadow comparison is still `NOT_OBSERVED` in CI, because the driver pins are empty and no selector runs. Slice identity was checked only for the one split package. |
| 20-PR bar (merged through the queue, complete forward rows, no trip) | 3/20 (#861, #873, #874). 12 rows retained: all `CLEAN`, all complete, 0 `MISS`, 0 `INCOMPLETE`, no trip. |
| Timing before/after | Before: met (n=20). After: partial (n=30). It measures the shadow, not narrowing. |
| Promotion | NOT met. It waits on the 20-PR bar, and on the shell declaration gap below. |
| Balanced placement, local 20% p90 bar | `NOT_RUN` (contended host). CI balanced placement stays unpromoted, because no accepted AFP-V0-028 amendment exists. |

## 1. Parity over the retained shadow rows

### Inputs

Every `pull_request` CI run that retained `corvint-test-shadow-<k>` (k = 0..7) by the snapshot was
used. The inventory is `evidence/v1-1257/artifact-inventory.tsv`. There are 14 runs: 38494366263
(#870), 38559867961 (#861), 38571221901 (#873), 38577371128 (#874), 38577580746 (#875), 38577925692
(#876), 38578015095 (#877), 38578668987 (#878), 38579877153 (#879), 38580551537 (#880),
38581168572 (#881), 38583469095 (#882), 38583990529 (#883) and 38585457566 (#884).

Three of these runs, #881, #883 and #884, were still running at the snapshot. Their driver outcomes
were downloaded after they completed, between 15:51:40Z and 15:59:39Z.

Each shard is compared on three sources:

- the shadow `selection.json` (`selected`, `scope`, `incomplete`, `target`, `head`) and its
  `dry-run.json` (per-invocation argv and units);
- the driver's `ci-outcomes-pull_request-<k>/outcomes.json` (`corvint-ci-outcomes/0`: every package the
  shard executed, with its result);
- for the split package, the driver's `ci-shard-costs-<k>` `go test -json` stream.

Every run used shadow tool source `5163d1ed4b61084fc32cf79355848899c3e8a5bb`.

### Tool

`evidence/v1-1257/scripts/parity.sh` with `parity.jq` (one row per run and shard), and `slices.sh`
(slice identity). Commands:

```sh
gh run download RUN --repo beamfall/corvint --pattern 'corvint-test-shadow-*' --pattern 'ci-outcomes-*' --dir work/runs/RUN
gh run download RUN --repo beamfall/corvint --pattern 'ci-shard-costs-1' --pattern 'ci-shard-costs-2' --dir work/costs/RUN
sh parity.sh work/runs > parity.tsv
sh slices.sh work/runs work/costs > slices.tsv
```

### Classes

The class column applies these rules in order:

- `TARGET_MISMATCH`: the target or head differs.
- `UNEXPLAINED`: a selected package was not executed, or a shadow Go argv prefix differs from the
  driver's.
- `MISS`: a failed or missing package was not selected.
- `subset`: none of the above.

The driver's argv prefix is not in the outcomes. It is taken from the FULL branch of the `ci.yml`
Go tests step:
`go test -exec /home/runner/work/_temp/test-confine -json -p 1 -count=1 -race -timeout 50m`.

### Result (`evidence/v1-1257/parity.tsv`, `slices.tsv`)

- All 112 shards are `subset`: 0 `TARGET_MISMATCH`, 0 `UNEXPLAINED` and 0 `MISS`.
  - `selected_not_executed` is 0 on every shard.
  - The shadow Go argv prefix equals the driver's on every invocation.
  - Every shadow exit is 3.
  - One driver failure was observed: `internal/companionrelease` on #878, shard 5. The shadow
    selected it.
- The three `FULL` runs select exactly what the driver ran: 336 package-shard entries, which is
  335 packages plus `internal/tasks/store` split over shards 1 and 2. Their reasons are
  `UNINDEXED_SOURCE_PATH`:
  - `.github/forward-rows/bar.sh` (#870);
  - `interop/cem01-go/stable_citation_vectors_test.go` (#879);
  - `interop/cem01-go/main.go` (#881).

  This is
  `str-v0-003-unknown-widening` from the 2026-10-11 parity entry.
- `executed_not_selected` covers 113 to 201 entries per `NARROWED` run (1,701 in total). The shadow
  did not select these packages. The driver runs them only because its pins are empty and it
  executes in full. This lane does not independently check whether the diff affects them; the
  substitute-corpus parity in the 2026-10-11 entry covers selection accuracy. This is the `ci-admission-no-qualification` category, so it is not a
  selection defect.
- Slices match on all 28 split-package shards (14 runs × shards 1 and 2):
  - The `-skip` slice 1/2 ran 248 to 251 top-level tests, none of them in the pattern.
  - The `-run` slice 0/2 ran 139 of its 141 names. The two names it did not run,
    `TestCALV0026_LeaseCommitSweepsOutsideWriterLock` and
    `TestCALV0026_InPlaceIntentEditAfterCommitSweep`, are in `lease_guard_darwin_test.go` and are not
    compiled on Linux.
- Per-shard manifest digests agree within each run.

### Residual: shell declaration gap

Every one of the 112 shard plans is `INCOMPLETE` with this limit:

```
shell: no qualified adapter and .github/cishards/test-commands.json declares no shell full command
```

This is `str-v0-004-undeclared-language`. The affected plan selects `shell:script/*_test.sh` units
on every PR, including #874, which touches no shell file. `test-commands.json` declares
`typescript`, and records `none` for dotnet, kotlin, python, ruby, rust and swift. It has no shell
entry.

The driver runs no shell suite either, so there is no parity difference. The forward rows stay
`CLEAN` only because `row.jq` treats a non-Go adapter gap as `limit_only`. The gap reproduces
locally: `corvint test --base 92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431 --dry-run` on the #874 head
gives rc=3, `NARROWED`, 219 units, with the same shell limit.

Until a shell full command or `none` is declared, a promoted call site would end every observed PR
`INCOMPLETE`. That is a declaration gap, not a selector defect, so this lane fixes nothing.

### Residual: selector comparison not observed

The comparison with the AFP-V0-013 selector stays `NOT_OBSERVED` in CI. Every row records
`notProduced: selectionDigest ... the driver pins are empty, so no selector runs`. The
selector-to-`corvint test` comparison therefore rests on the substitute corpus in the 2026-10-11
parity entry.

## 2. Forward rows under epoch 1 (`evidence/v1-1257/forward-rows.tsv`)

Every `ci-forward-*-e1-*` artifact present at the snapshot is included. There are 12 rows, built
by `scripts/rows.sh` from each `row.json` (`corvint-pr-forward-row/0`):

| PR | Kind | sha | Outcomes | Scope | Selected | Failed | Merged |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 861 | pr | e0ee3f6067b6 | pull_request | NARROWED | 173 | 0 | queue, 12:25:31Z |
| 861 | queue | adf587fdb65f | reuse | NARROWED | 173 | 0 | yes |
| 873 | pr | 5691de65a8ee | pull_request | NARROWED | 190 | 0 | queue, 13:57:28Z |
| 873 | queue | 92c298a6e8fa | reuse | NARROWED | 190 | 0 | yes |
| 874 | pr | 93ff92fd372c | pull_request | NARROWED | 182 | 0 | queue, 15:42:50Z |
| 874 | queue | 5c4844bc84fa | reuse | NARROWED | 182 | 0 | yes |
| 875 | pr | 401e4ce5e018 | pull_request | NARROWED | 170 | 0 | open |
| 876 | pr | f4f39fa740e1 | pull_request | NARROWED | 164 | 0 | open |
| 877 | pr | d4324b84ee62 | pull_request | NARROWED | 168 | 0 | open |
| 878 | pr | 5c7b49aeceac | pull_request | NARROWED | 210 | 1 (selected) | open |
| 879 | pr | b65b3293ebb8 | pull_request | FULL | 335 | 0 | open |
| 882 | pr | 1d2a2919a82d | pull_request | NARROWED | 184 | 0 | open |

- **Classes and completeness.** Every row is class `CLEAN` and epoch 1, with no incomplete
  entries and none truncated. The row `sha` is the tested merge ref, and `head` is the PR head.
- **MISS and FLAKY.** No row has a `MISS`, so there is nothing to attribute to runner or selector.
  No row has a `FLAKY`.
- **Trip.** The AFP-V0-030 kill switch itself is not implemented (affected-plan-v0, traceability
  row AFP-V0-030). Its thresholds over the latest 20 rows, 2 `MISS` or 3 `INCOMPLETE`, are both at
  0, so there is no trip.
- **AFP-V0-029 shadow bar (`bar.jq`).** 12/20 complete rows, and 1/5 rows with failing packages.
  NOT MET.
- **STR-V0 promotion bar.** 3/20 merged PRs through the queue with complete PR and queue rows
  (#861, #873, #874). NOT MET.
- **Shadow runs without a row at the snapshot.**
  - #870's run predates the `forward-rows` job.
  - #880's row had not arrived. Rows land about 17 to 25 minutes after a run completes.
  - These runs are still counted in the parity above.

## 3. Timing baseline (`evidence/v1-1257/timing-*.tsv`)

### Windows

Merged PRs are taken in `mergedAt` order from
`gh pr list --state merged --base main --json number,mergedAt,mergeCommit,headRefOid,headRefName,createdAt`.

- **Slice 3** is #815 (merge `1f75ce55277d`, 2026-10-12T17:16:45Z). It contains `8b4ef6fa3`.
- **Before** is the 20 PRs merged immediately before it: #818 to #839 (818, 819, 820, 821, 823, 822,
  824, 825, 826, 827, 828, 829, 830, 831, 833, 834, 836, 837, 838, 839), merged
  2026-10-12T00:46:55Z to 16:36:12Z.
- **After** is every PR merged after #815 up to the snapshot. This is partial, n=30: #841 to #874,
  ending with #874 at 15:42:50Z.

### Run attribution (`scripts/attribute.jq`)

All workflow runs were listed with `actions/runs?created=2026-10-11T00:00:00Z..2026-10-14T15:45:10Z`
(677 runs). A run belongs to PR N, with head branch B, merge commit M, created C and merged T, when
any of these holds:

- its `head_branch` is B, its event is `pull_request` or `workflow_run`, and it was created in
  [C, T];
- its `head_branch` starts with `gh-readonly-queue/main/pr-N-` and it was created at or before T;
- it is a `main` `push` or `workflow_run` with `head_sha` M.

This attributes 458 runs, none of them twice. The retained per-run table is `timing-runs.tsv`, and
the raw job rows (2,920) are in `jobs.tsv`. Every attributed run's jobs come from
`actions/runs/<id>/jobs?filter=all` (all attempts), the same source as the v1-0809 method.

### Formulas (`scripts/timing.py`, `scripts/stats.py`)

- **Runner seconds of a run.** The sum over its jobs of `completed_at - started_at`. Jobs that
  never started count 0. Every attempt counts.
- **Run wall time.** The latest job `completed_at` minus the run's `created_at`. This includes
  queueing for runners and any rerun attempt.
- **PR wall time (`pr_ci_wall_s`).** The run wall time of the final `ci.yml` `pull_request` run on
  the PR head sha (the latest created).
- **Queue wall time (`queue_ci_wall_s`).** The run wall time of the `ci.yml` `merge_group` run whose
  head sha is the merge commit.
- **Runner-minutes per merged PR (`all_workflows_runner_s`).** The runner seconds of every
  attributed run, divided by 60.
  - `ci_yml_runner_s` restricts this to `ci.yml`.
  - `pr_ci_runner_s` and `queue_ci_runner_s` cover the final PR run and the queue run only.
- **Median.** The middle value. For even n, the mean of the two middle values.
- **p90.** Nearest rank: the value at ascending rank ceil(0.9 n).

### Result

| Metric | Before (n=20) median / p90 | After, partial (n=30) median / p90 |
| --- | --- | --- |
| PR wall time, final `ci.yml` PR run | 22.9 / 24.8 min | 24.0 / 37.9 min |
| Queue wall time, `ci.yml` merge_group | none (no queue) | 20.8 / 27.7 min (n=28) |
| Runner-minutes per merged PR, all workflows | 251.7 / 362.1 | 363.5 / 587.8 |
| Runner-minutes, `ci.yml` only | 250.7 / 360.8 | 362.4 / 585.9 |
| Runner-minutes, final PR run | 153.5 / 158.2 | 154.9 / 163.2 |
| Runner-minutes, queue run | 0 / 0 | 138.1 / 144.8 |
| PR open to merge (context, not CI) | 23.8 / 53.7 min | 53.3 / 122.7 min |

### Confounders

The after figures do not measure `corvint test`, because nothing narrows: the driver still runs
every shard in full. The after window mixes three changes:

- The merge queue starts at #840. #841 and #842 merged without a queue run; 28 of 30 had one.
  Before the queue, no PR had a queue run.
- AFP-V0-024 identical-tree reuse (#867) applies from #869 onward. Queue-run runner time drops from
  about 140 to about 16 to 19 min.
- The shadow dry-run step is live from the #868 merge (`5163d1ed4b61`).

A few long runs drive the PR-run p90 rise. #867 and #870 each had a second attempt, and both
attempts count. The others, #841, #848 and #874, are not attributed: this method does not separate
runner queueing from execution. #874's queue run took 46 minutes of wall time for 17
runner-minutes. The per-run table is `timing-runs.tsv`.

### NOT_OBSERVED

- billed minutes and runner queue time apart from wall time;
- runs created before 2026-10-11T00:00Z;
- PR-run wall time for pushes before the final head sha (they count in runner-minutes but not in
  wall time).

Each PR row also records `jobs_not_completed`, which is 0 for all 50 PRs.

## 4. Balanced placement, local bar

`NOT_RUN` (contended host). On this 12-CPU host, `uptime` read load averages of 80.93, 90.49 and
81.61 at the start of the lane, and 55.71, 49.96 and 55.59 at 12:04 local time. The 1-minute load
stayed above the CPU count, so an executed wall-time comparison would not be valid.

The comparison that would run uses the 50 pairs in `tools/str-parity/testdata/pairs-645304a2.txt`
(`<merge> <base> <head>`), on a detached worktree at `<head>` for each pair, with 8 shards.
Universe and balanced placement are interleaved per pair, the whole run is bounded to 90 minutes of
wall time, and each shard's wall time is recorded:

```sh
corvint --root WT test --base BASE --placement P --shard 0/8 --dry-run --manifest m-P.json
for k in 0 1 2 3 4 5 6 7; do corvint --root WT test --base BASE --placement P --shard k/8 --manifest m-P.json --receipt r-P-k.json; done
corvint test verify-shards --manifest m-P.json r-P-*.json
```

Here P is `universe` or `balanced`. The bar is that p90, over pairs, of the slowest shard's wall
time is at least 20% lower for balanced, with every unit placed once.

In CI, balanced placement stays computed-only and unpromoted. No accepted AFP-V0-028 amendment
defines an executed comparison.

## Limits and rollback

- **Moving tally.** The forward-row tally and the after window are a snapshot at
  2026-10-14T15:45:10Z, and both grow. A later lane should recount from the artifacts:
  - Shadow artifacts are kept for 14 days.
  - `bar.sh` lists 20 pages of artifacts, so older rows can fall out of its window.
- **Independence.** The parity is between the shadow and the driver on the same runs. It is not an
  independent replay.
- **Rollback.** This entry and `evidence/v1-1257/` are documentation only. Revert the commit.

## Review

One Codex read-only round, on the first version of this commit.

It recomputed every median and p90 from the TSVs and found that all of them match. It also found
that all 88 parity classifications, all 22 slice rows and all 12 forward rows match the downloaded
JSON, and so do the 3/20 and 12/20 counts.

Its three findings are fixed here:

- The artifact inventory (`artifact-inventory.tsv`) and the raw job rows (`jobs.tsv`) are now
  retained, so the completeness and timing inputs can be checked after the downloads expire.
- "Not affected by the diff" is narrowed to "not selected".
- This review section is filled in.

The three runs that completed after the snapshot, #881, #883 and #884, were added after the review.
They use the same scripts, and all of their shards are `subset`.
