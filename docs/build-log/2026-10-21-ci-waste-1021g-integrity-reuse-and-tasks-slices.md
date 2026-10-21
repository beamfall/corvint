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
- The proof is the record name: a pull-request run whose gate passed retains
  `ci-tested-tree-<tree>-artifact-integrity`; the queue step looks it up on the successful
  pull-request run of its second parent and also requires that run's latest `artifact-integrity`
  job concluded `success` on that head. The tree includes `ci.yml`, the `Makefile`, `go.mod` and
  the gate script, and the gate runs with `GOPROXY=off`, so an equal tree means equal inputs.
  Only listings are read; nothing is downloaded. Every failure leaves the decision empty and the
  gate runs in full. The decision is retained as the `ci-integrity-reuse` artifact and in the step
  summary (run id, tree, head, record name).
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
- Tests: `TestAFPV0163WorkflowWiresIntegrityTreeReuse` and
  `TestAFPV0163IntegrityReuseDecidesOnlyForAPassedPullRequestRunOfThisTree` (stub curl over a
  real two-parent Git merge: REUSE with the anonymous retry; empty for other head, event,
  conclusion, workflow, repository, string id, other-tree, missing, duplicated, expired or
  foreign record, unreadable listings, failed, skipped, missing or foreign job, single-parent
  commit). Both fail without the `ci.yml` change. AFP-V0-080/081 tests still pass.
  `TestAFPV0060CommittedSlicesBoundOversizedPackages` now requires cli and authority split; the
  `.github/cishards` package passes (plan complete, every test selected once).
- Workaround: `ci-test-slices replay` and `generate` refuse current CI shard logs at the
  AFP-V0-047 shadow summary line (`{"scope":"FULL","exit":0,...}`, "damaged go test -json record:
  no Action"); the line was filtered out before regeneration. Residual.

## NOT_RUN

- Hosted queue and shard measurements after the change (`NOT_OBSERVED`).
- `package-costs.json` refresh; `make gate`; tier 2 of `corvint affected`.

## Rollback

Revert the commits: the `artifact-integrity` steps and conditions in `ci.yml`, the two
allow-list entries, and `test-slices.json`. No stored state depends on them; leftover record
artifacts expire with the repository's artifact retention.
