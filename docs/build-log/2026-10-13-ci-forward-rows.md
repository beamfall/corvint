# 2026-10-13 CI forward rows: activate the corvint test shadow and record AFP-V0-029 rows (V1-0811, V1-1315)

Branch `claude/ci-forward-rows` from `origin/main` 5163d1ed4b61084fc32cf79355848899c3e8a5bb (the
merge of #868, which contains #867, the identical-tree reuse of AFP-V0-046). `AFP-V0-047` is new
in `docs/specs/affected-plan-v0.md`, written under the owner's standing auto-accept of
2026-10-11, so its mode is `standing` until the owner reads it back. It is Corvint 1.0 scope
under `PRS-V1-018` (#857). This entry covers V1-1315 AC1 (the shadow runs) and V1-0811 (rows,
classifier tests, no narrowing, bar summary). The orchestrator holds both leases.

## Why

The owner asked on 2026-10-13 to "only run the tests that need running". PR narrowing
(AFP-V0-028, V1-0812) waits on the AFP-V0-029 shadow bar: 20 complete forward rows, at least
5 with failing packages, and no miss other than `FLAKY`. The bar had zero rows because S was
never computed. The `corvint test` shadow step ran only when `CORVINT_PR_TOOL_SOURCE` was set,
and that pin also activates the deciding driver `tools/corvint-pr-tests`. The driver refuses
sharded execution while AFP-V0-041 slices are active, and `.github/cishards/test-slices.json`
names `internal/tasks/store` slices, so setting the pin would fail every PR shard.

## Decision

1. **Split the pin; do not repair the driver.** `CORVINT_TEST_SHADOW_SOURCE` is pinned to
   BASE, 5163d1ed4b61084fc32cf79355848899c3e8a5bb. Two new steps check it out to `trusted-shadow`
   without credentials, verify `HEAD`, and build only `./cmd/corvint` to
   `$RUNNER_TEMP/corvint-shadow-bin/corvint`. The shadow step and its retention key on this pin.
   The driver pins stay empty, so the Go tests step still runs every shard in full. The epoch
   stays `1`. Teaching the driver to place slices would change the deciding path and
   `tools/corvint-pr-tests`, which this lane may not edit, for no gain while nothing narrows.
2. **S is a compact document beside the dry-run.** `selection.jq` turns `dry-run.json` and
   `manifest.json` into `selection.json` (`corvint-pr-selection/0`). It names the AFP-V0-013
   selection digest as `NOT_PRODUCED`, because no selector runs while the driver pins are
   empty, instead of inventing it. The plan, partition and source digests come from the
   dry-run as printed.
3. **F and the rerun live in the shard.** `outcomes.sh first` reduces the captured
   `go test -json` stream to package-level outcomes (`Test` absent), and marks expected packages
   and slices without a terminal event `missing`. A failed shard reruns each failed package or
   slice once, at most 8, under the same confinement wrapper. `outcomes.sh rerun` records the
   rerun outcomes. The shard's exit status is unchanged. The rerun has to run on a runner that
   has the tested tree, the toolchain and the confinement wrapper. The read-only row job has
   none of these and must never execute repository code.
4. **The row builder is jq, run from the default branch.** The `forward-rows` job checks out
   only `.github/forward-rows` at its own `github.sha`, which is the default-branch commit a
   `workflow_run` job runs from. It runs `collect.sh`, `classify.sh` and `row.jq` with `gh`
   and `jq`. It has `actions: read` and `contents: read` and downloads only small artifacts.
   My reading of "never executing repository code" is this: the job runs reviewed scripts of
   the default branch, as the existing control-plane jobs run their inline shell, and never
   code of the commit under test. A pull request that edits these scripts changes them only
   after review and merge.
5. **Every input defect is INCOMPLETE.** The classifier collects reasons before classifying:
   - missing, unreadable, misshapen or duplicate shards
   - a shard count other than `FORWARD_SHARDS`
   - another run, head or target, or another tree for REUSE
   - a dry-run exit other than 0 or 3
   - shards that disagree on the manifest digest
   - a selection gap other than a non-Go language without an adapter
   - a failed shard with no failed package
   - a collector error

   Any reason makes the row INCOMPLETE, and that outranks every other class.
6. **Names carry the bar.** A row's name is
   `ci-forward-<CLASS>-pr<N>-<sha>-e<EPOCH>-f<0|1>`. `bar.sh` counts the latest 20 rows of the
   epoch from artifact names alone, as the AFP-V0-030 PR check will, with one row per pull
   request and commit. The epoch and failing flag extend the charter's
   `ci-forward-<CLASS>-pr<N>-<sha>` name so that this count needs no download.

## Rejected alternatives

- **Activate `CORVINT_PR_TOOL_SOURCE`.** This fails every PR shard: the driver's `refuseSlices`
  exits 2.
- **Rerun in the row job.** This would execute tested code in a privileged default-branch
  context.
- **Build a Go classifier in the row job.** This works, as `docs-plan` builds
  `tools/ci-reuse-plan`, but it adds a toolchain setup and a build to a 10-minute read-only job.
  jq is preinstalled, and the Go test drives the same scripts.
- **Use `go test -json` from `ci-shard-costs-*`.** That artifact exists only for a passed shard
  and keeps 3 days. Outcomes must also exist for a failed shard, and that is the case that
  matters.

## Evidence

- **Real fixtures.**
  - Outcomes: merge-queue run 38485555207 (queue commit 5163d1ed4, PR #868, head 460fc31ae6,
    tree 728e6e3f99) passed every shard. Its `ci-shard-costs-*` streams were downloaded
    read-only to the lane scratch directory, and `outcomes.sh first` reduced them to
    `tools/ci-forward-row/testdata/outcomes/shard-{0..7}.json`.
  - Selections: the shadow binary at the pin dry-ran PR #868 (base 142f0e3e7b) on each shard.
    The result was 193 manifest units, scope `NARROWED`, exit 3 and one shell-adapter limit.
    These dry-runs became `testdata/selection/shard-{0..7}.json`, with run 38482995697 (the
    latest successful PR run of the head) as `runId`.
  - FLAKY: pull-request run 38476173623 failed shard 7 on attempt 1 on the
    `internal/contextindex` flake V1-1362 and passed on attempt 2. The job log's package
    events are in `testdata/streams/`.
  - REUSE: push run 38487301206's `ci-reuse` decision.
- **Local classification of the real fixtures.** CLEAN, 1115 bytes, 192 selected packages,
  shell limit retained.
- **Read-only replay of `collect.sh` against hosted runs** with the owner's `gh` token:
  - 38485555207 (merge_group) gives
    `ci-forward-INCOMPLETE-pr868-5163d1ed4b61084fc32cf79355848899c3e8a5bb-e1-f0`. Its reasons:
    run 38485555207 retained no `ci-outcomes` artifact, and run 38482995697 retained no
    `corvint-test-shadow` artifact. Both runs predate this change, so INCOMPLETE is correct.
  - 38487301206 (push): not a forward row.
  - 38482995697 (pull_request, DOCS): no row.
  - `bar.sh` over the live artifact list: 0 rows, NOT MET, kill switch clear.
- Tests are listed in the AFP-V0-029 and AFP-V0-047 traceability rows.

## Limits

These hosted items are `NOT_OBSERVED`:
- the first hosted shadow build and selection
- the first `ci-outcomes-*` upload
- `workflow_run` delivery of the `forward-rows` job
- the first row, which comes on the first queue run on `main` after this change

Other retained limits:
- The rerun bound leaves a ninth failed invocation without a rerun, so its misses are MISS.
- Shadow artifacts keep 14 days, and a later queue run is INCOMPLETE.
- The bar reads at most 20 pages of artifacts and assumes the API lists newest first.
- A shard whose capture file cannot be created writes no outcomes, so its row is INCOMPLETE.
- Fork pull requests produce no row.
- PR rows and queue rows both count toward the bar.
- Every selection carries the shell-adapter gap as a limit until `.github/cishards` declares a
  shell full command.
- The `selection.json` builder runs from the tested checkout, as the PR's own workflow does.
  Rows from a PR that edits `.github/` already need AFP-V0-016 consent.
- The selection digest is `NOT_PRODUCED`.
- Each PR shard pays one extra `./cmd/corvint` build.
- `make gate` is `NOT_RUN`, by the owner's focused-test preference.

## Rollback

Clear `CORVINT_TEST_SHADOW_SOURCE`, which stops the shadow and the selections, and remove the
`forward-rows` job. Nothing reads the rows to narrow before V1-0812, so neither change alters a
required check.
