# 2026-10-07: bind several attempts to one pool allocation (V1-0946)

## Intent

GitHub #653 (native V1-0946) asks one holder to run several lanes on one pooled environment. Before
this change every claim with `--pool` took its own member, so batching lanes needed one member each.
Proposed PSR-V0-016..021 (`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`), pending owner
acceptance, add `claim <ticket> --pool P --stage S --share-allocation <allocationId>`.

## Decision

- The binding is recorded twice in the share's one ADMIT receipt: the new attempt's record gets
  `sharedAllocation {sourceAttemptId, sourceGeneration, boundSeq}`, and the pools.json entry gets an
  ordered `shared` list. Both keys are optional and written only when present, so unshared records
  keep their bytes and digests. The lease request preimage gains `shareAllocation` the same way.
- The bound is 4 attempts per member, the original included (`snapshot.MaxSharedAttempts`). This
  keeps one member's blast radius and its pools.json entry small.
- Lifetime is the union of the bound leases. An ending shared attempt leaves the list. An ending
  entry attempt hands the allocation to the first shared attempt rather than refusing the release or
  quarantining. Only the last attempt reaches the ordinary end path, so the member quarantines once.
  The hand-over is a pools.json post in the ending attempt's own receipt.
- Expired bound leases are returned as `Expired`, so the existing store loop reaps each one in its own
  receipt before the share is retried (CAL-V0-011). Reaping one attempt leaves the others alone.
- Paths that act on a whole allocation are refused rather than redesigned. Supervisor ATTACH refuses
  UNSUPPORTED for any shared allocation, because a supervised stage stop quarantines the whole member.
  A lane-untouched release refuses while the entry binds shares; after a hand-over its `changedSeq`
  no longer matches the original admission. A share also refuses if any bound attempt is supervised.
- A share takes no member, so it skips CAL-V0-101 priority yield.
- The views stay pure reads. `pool status` gains `boundAttempts`. `attempt show` gains a derived
  `poolBinding` observation from the same audited read, now including pools.json. The canonical
  capture without observations is unchanged.
- An older binary's closed decoders refuse an unknown `shared` or `sharedAllocation` key `MALFORMED`.
  That is a safe refusal, not a misread of occupancy. It is recorded as a rollback limit.

## Evidence

Focused tests: `TestPSRV0016_*`, `TestPSRV0017_*`, `TestPSRV0018_*`, `TestPSRV0019_*` and
`TestPSRV0021_*` in `internal/tasks/{cli,transaction,snapshot,store}`. The store crash test faults
each of the six publications of a share claim: `RECEIPT`, `POST` of the attempt, pools.json, the
request and reservations.json, then `HEAD`. It shows a fresh retry before the receipt and a redo
after it, followed by a byte-identical replay.

The older-binary refusal is inferred from the closed decoders and an unknown-key test. No older
binary was executed.

## Limits

- Dispatcher batching over shared allocations is not implemented.
- Supervised attempts cannot share.
- `cli/pools.go` occupancy rows in other views still name only the entry's attempt.
