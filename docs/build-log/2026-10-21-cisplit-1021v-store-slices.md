# 2026-10-21 cisplit-1021v: store slices and cost refresh for the batch critical path

Tickets: V1-0961, V1-1499 (store package), V1-1752 and V1-1574 (tasks cli). The AFP-V0-164
amendment is accepted under the owner's standing auto-accept (2026-10-11). The orchestrator
granted this lane's `.github/cishards/**` edits for this goal; the change needs the owner's
`ci-control-plane` post.

## Problem

Batch rounds took about 40-45 min. In run 39709153564 (revision f29ddc0db, the tree the merge
queue later reused) shard 4 ran 2483.9 s of package time (44.9 min wall) against an ideal
1293.2 s over 8 shards. The cause was the `internal/tasks/store` catch-all slice: about 1530 s
for 202 tests. The previous slice file was generated at 754708fd3, a branch revision that
predates the JSGV journal-segment tests merged in 72b6aadc4, so every one of those tests landed
in the catch-all (AFP-V0-060 (3) behaves as specified; the file had simply gone stale). The
committed `package-costs.json` still put the store at 1546.6 s against an observed 2739.2 s, so
the planner predicted a 1045 s maximum shard and placed the store badly.

Slowest store tests in that run: TestCALV0089_ProgramWallExpiryEndsContinuation 182.6 s,
TestJSGV0012_CompactionOnASealedStore 121.3 s, TestJSGV0013_UnsealStopsSettle 114.7 s,
TestJSGV0001_LivenessBindDeclinesMovedInputs 87.8 s, TestJSGV0008_SealStopsSettle 80.1 s,
TestJSGV0011_ResumeDeclinesRewrittenSegment 74.1 s, TestCALV0116_WriterFullBoundDeclines 73.4 s.

## Decisions

- Regenerate, do not hand-edit: `tools/ci-shard-costs refresh --shards 8` and
  `tools/ci-test-slices generate --target 350s` from the eight shard logs of run 39709153564, built
  with go1.27.2 (the generator refuses other toolchains). Both files record one revision and run.
- The store now has eight slices (max 357.8 s), `internal/tasks/cli` two (276.4 s, 268.5 s) and
  `cmd/corvint` three. `internal/tasks/authority` costs 90.4 s in this run, under the target, so
  the generator keeps it whole; it stays allow-listed. AFP-V0-164 is amended to say so, and
  `TestAFPV0060CommittedSlicesBoundOversizedPackages` now checks that every allowed package above
  the bound is split instead of naming authority.
- Shard count stays 8; `SHARDS` and `REUSE_SHARDS` in `ci.yml` are unchanged. The partition digest
  is computed from the files and is not pinned.
- No store test is changed: the slowest tests are journal/segment code reserved for another lane
  and the store package is being edited by peer lanes.
- Two `internal/sessiontest` tests encoded the old split: the five-slice comment and threshold
  (now eight), and `TestSTRV0008BalancedUsesEveryShard`, which assumed shard 0 holds four whole
  packages at 4 shards; the store now runs whole there and fills shard 0 alone, so the test takes
  the first shard with four whole packages. It failed before that edit.

## Evidence

- `tools/ci-test-slices replay` over run 39709153564: observed max 2483.9 s; previous files 2173.8 s;
  refreshed files 1293.7 s (every shard 1291.6-1293.7 s; ideal 1293.2 s). Estimated shard wall
  about 25 min instead of about 45 min (package time plus about 3 min of setup).
- Focused tests pass: `./.github/cishards`, `./internal/sessiontest`, `./tools/corvint-pr-tests`,
  `./tools/ci-test-slices`, `./tools/ci-shard-costs`.

## NOT_RUN

- Hosted observation of the new max shard (next batch run).
- Load reproduction of V1-1574 and the local whole-package store runtime of V1-0961/V1-1499:
  the CI path is sliced, but `go test ./internal/tasks/store` locally is unchanged.
- Tier-2 affected units, left to batch CI.

## Residual

The catch-all can silently regrow whenever tests land after generation. A drift check (catch-all
predicted cost against the bound from the latest run) or regeneration from a main revision should
be filed as a ticket.

## Rollback

Restore the previous `.github/cishards/test-slices.json` and `package-costs.json` and revert the
test edits in `.github/cishards/slices_test.go` and `internal/sessiontest/`.
