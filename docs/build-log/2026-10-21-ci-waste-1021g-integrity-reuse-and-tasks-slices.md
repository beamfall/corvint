# 2026-10-21 ci-waste-1021g: integrity tree reuse in the queue, tasks package slices

Tickets: V1-2113 (AFP-V0-163), V1-1752 (AFP-V0-164). Both requirements are accepted under the
owner's standing auto-accept (2026-10-11). The owner approved this lane's `.github/` edits on
2026-10-21; the change needs the owner's `ci-control-plane` post.

## Problem

- V1-2113: merge-queue run 39458994189 reused its pull-request shards (`go-product` done at about
  4.9 min, `go-static` 4.8 min) but reran `artifact-integrity` for 11.35 min on the tree its
  pull-request run 39472511845 had already verified (12.6 min there). With the ALLGREEN queue the
  job was the critical path: the run ended 16:48:20 and the PR merged 16:48:51.
- V1-1752: `internal/tasks/cli` (467.6 s) and `internal/tasks/authority` (444.5 s) were not
  allow-listed, so each ran whole in one shard. Run 39613481398 (revision f15f4111d): slowest
  shard 1561.4 s (shard 6) against an ideal 1132.9 s over 8 shards.

## Decisions

- AFP-V0-163 keeps the `artifact-integrity` job running and skips only `setup-go` and
  `make go-archive-gate` on REUSE. A skipped job would break AFP-V0-080, which needs the queue
  run's three jobs concluded `success` before a `main` push skips them. The `needs`/`if` lines are
  unchanged, so AFP-V0-080/081 tests still bind.
- The proof is the log of a separate `artifact-integrity-record` job, not an artifact. An
  independent review of the first version (head a4d232cef) found that the queue step trusted any
  artifact named `ci-tested-tree-<tree>-artifact-integrity` in the pull-request run. Artifact
  names do not authenticate their producer: any job of that run (a test, a build step, or a fork
  pull request's code) could upload a falsely named record for tree B while the real gate passed
  on tree A, and the "job succeeded" check did not bind the artifact to that job. The record job
  now runs in a pull-request run only after `artifact-integrity` passed, checks out the merge
  commit without persisted credentials, runs only builtins, `git` and `printf`, and prints
  `corvint-ci-integrity-tree/0 tree=<tree> target=<merge> head=<head>` to its own log. The queue
  step lists the successful pull-request run of its second parent, requires exactly one latest
  `artifact-integrity` and exactly one latest `artifact-integrity-record` job of that run, both
  `success` on that head, reads that job's log through the job-log API, and requires exactly one
  record line whose tree and head equal its own. No artifact is read. `artifact-integrity` gains
  `actions: read` for the log read (a declared ARTIFACT-V0-008 exception row).
- Trust argument: only the runner executing a job writes that job's log, and the record job runs
  no repository code, so only its definition in `ci.yml` decides what it prints. A pull-request
  run takes `ci.yml` from the tested merge commit, whose tree must equal the queue commit's, so a
  `ci.yml` able to print a false line is part of the tree being merged, and `.github/` edits need
  the `ci-control-plane` admin review. The tree includes `ci.yml`, the `Makefile`, `go.mod` and
  the gate script, and the gate runs with `GOPROXY=off`, so an equal tree means equal inputs.
  Every failure leaves the decision empty and the gate runs in full. The decision (run id, record
  job id, tree, head) goes to the step summary and the diagnostic `ci-integrity-reuse` artifact,
  which nothing reads back.
- AFP-V0-080 push reuse reads only the jobs listing of the merge-queue run of the pushed commit
  (three job names, `success`, `head_sha`); it reads no artifact, so it does not have this flaw.
  Its trust in the queue run's `artifact-integrity` conclusion now rests on the record-job
  binding above.
- The docs-plan reuse (AFP-V0-024/046, V1-0760; `tools/ci-reuse-plan` `Decide`) has the same flaw
  class: it accepts `ci-tested-tree-<tree>-full-<shard>-of-<shards>` artifacts from an admitted
  run by name alone, and those records are uploaded by `go-product-shard` jobs that also run the
  repository's tests. Not fixed here: a sound fix needs a record job without repository code
  that depends on every shard, and a job-log reader in the Go tool. Reported for filing.
- AFP-V0-164 allow-lists and slices both packages (2 slices each) rather than rebalancing: the
  placement is already longest-first, and a 467 s indivisible unit was the floor. Slices were
  regenerated from run 39613481398 with go1.27.2 (an admitted source); the same regeneration also
  refreshed the cmd/corvint (3) and store (5) slices from that run.
- `package-costs.json` is not refreshed: `tools/ci-shard-costs refresh` needs the raw
  `ci-shard-costs-N` artifacts, which this lane does not download. Residual.

## Order-independence review (V1-1752)

- Neither package has a `TestMain`. Helper processes re-execute the test binary with their own
  anchored `-test.run=^Name$`, so a slice's `-run`/`-skip` flags never reach them.
- cli: package hooks are set through functions that return a restore, released by `defer` or
  `t.Cleanup`; environment changes use `t.Setenv`. authority: tests use private temporary
  repositories and lock files.
- Shuffled full runs passed: `go test -shuffle=on -count=1 ./internal/tasks/authority` (223.9 s)
  and `./internal/tasks/cli` (659.8 s; above the charter's 600 s per-command guideline). Seeds
  were not captured.

## Evidence

- `tools/ci-test-slices replay` over run 39613481398's costs (summed per-shard model, 8 shards):
  before (previous slices) max 1628.8 s; after max 1133.9 s; 365 units (353 whole packages and
  12 slices), each placed exactly once. Under the committed `package-costs.json` that CI uses:
  1556.7 s before, 1414.9 s after. New slices: cli 236.4 s and 231.2 s, authority 222.8 s and
  221.7 s, cmd/corvint at most 336.1 s, store at most 318.1 s.
- Observed maximum shard before: 1561.4 s (run 39613481398). After: the hosted run of this PR,
  `NOT_OBSERVED` at writing.
- Queue wall time before: 11.4 min (run 39458994189). After: predicted about 5 min, bounded by
  `go-static`; hosted `NOT_OBSERVED` until a queue entry runs with this workflow.
- Tests: `TestAFPV0163WorkflowWiresIntegrityTreeReuse` (the step reads only the run listing,
  the jobs listing and one job log; no artifact), `TestAFPV0163RecordJobRunsNoRepositoryCode`
  (checkout without credentials plus one script of allow-listed lines; no token, secret or
  other action), and `TestAFPV0163IntegrityReuseDecidesOnlyForAPassedPullRequestRunOfThisTree`
  (stub curl over a real two-parent Git merge; the served log is the record script's own output
  with GitHub's timestamp and CRLF framing, and a forged tree-named artifact is on offer: REUSE,
  also with the anonymous retry; empty for other head, event, conclusion, workflow, repository,
  string ids, failed, skipped, missing, duplicated, foreign or other-run jobs, unreadable listing
  or log, no record line, two lines, other tree or head, trailing text, an embedded line, and a
  single-parent commit). All three fail on the reviewed `ci.yml`. AFP-V0-080/081 tests still
  pass.
  `TestAFPV0060CommittedSlicesBoundOversizedPackages` now requires cli and authority split; the
  `.github/cishards` package passes (plan complete, every test selected once).
- Workaround: `ci-test-slices replay` and `generate` refuse current CI shard logs at the
  AFP-V0-047 shadow summary line (`{"scope":"FULL","exit":0,...}`, "damaged go test -json record:
  no Action"); the line was filtered out before regeneration. Residual.

## NOT_RUN

- Hosted queue and shard measurements after the change (`NOT_OBSERVED`).
- `package-costs.json` refresh; `make gate`; tier 2 of `corvint affected`.

## Rollback

Revert the commits: the `artifact-integrity` steps, conditions and `permissions` block and the
`artifact-integrity-record` job in `ci.yml`, the `job:artifact-integrity` exception row, the two
allow-list entries, and `test-slices.json`. No stored state depends on them; leftover decision
artifacts expire with the repository's artifact retention.
