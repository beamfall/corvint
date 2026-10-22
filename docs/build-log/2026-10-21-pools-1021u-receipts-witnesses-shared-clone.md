# 2026-10-21 pools-1021u: bounded pool operation receipts, plan witnesses at the candidate, shared clones

Lane `pools-1021u`, base `9ae946a5c2b03f69a327c9230c30d9f35d1f19dd`. Requirements TOL-V0-132,
PSR-V0-196 and CTS-V0-013 are accepted under the owner's standing auto-accept (2026-10-11).

## V1-1750: pools.json receipt retention (PSR-V0-196)

Problem: PSR-V0-162 kept one `taskman-pool-operation/0` receipt per member and operation, up to
256 x 5 = 1,280 receipts. A scratch measurement encoded a 256-entry pools.json with 1,280 receipts
using 64-byte pool, member and holder labels and 128-byte request ids. It came to 1,048,666 bytes,
over `MaxPoolStateBytes` (1,048,576). `Encode` refuses that as `LIMIT_EXCEEDED`, so every later
pool write (acquire, release, sweep and operations) was refused. With 64-byte labels and short ids
and reasons, the same state measured 966,234 bytes. One receipt at maximum identifier lengths is
746 bytes.

Decision (the owner delegated it on 2026-10-21; the minimal bounded mechanism): keep
`operations` in pools.json and bound the count.
- A receipt write keeps at most `MaxPoolOperationReceipts` = 512 receipts (two per member at the
  member bound) and evicts the oldest by `observedSeq`. The receipt being written is never evicted.
- At maximum lengths, 512 x 746 = about 382 KB, which leaves room for 256 entries.
- The reader still accepts the old 1,280 bound, so an existing file stays readable. Its next
  operation write trims it.

Alternatives not taken:
- A side file per member. It adds a new state path, codec and rollback for a P2 bound.
- Dropping fields from each receipt. It changes the PSR-V0-162 wire shape.
- Trimming inside `PoolState.Encode`. Every pools.json writer would become lossy, and decode then
  re-encode would no longer round-trip.

What it costs: `pool status` shows `NOT_OBSERVED` for an evicted operation. Replay (PSR-V0-164)
is unaffected, because it reads the receipt from the pools.json that its own observation
transaction posted.

Evidence: `TestPSRV0196_FullSizePoolReceiptsStayBounded` (`internal/tasks/transaction`).
- Setup: 256 members, 1,280 receipts at maximum identifier lengths, 255 occupied entries.
- It runs `pool recover`, then a pooled claim of the last free member, which brings the pool to
  256 entries, then an early pool release.
- It checks that 512 receipts are retained, that the newest is kept and the oldest evicted, and
  that the decoded pools.json is under the bound.
- With the trim disabled, the test fails with "retained 1280 receipts, want 512".

Residual (not filed from this lane, which may not use Tasks): entry `reason` prose can be up to
4,096 bytes. A 256-entry pool with long reasons can still pass 1 MiB without any receipts (2.07 MB
measured at 4,096-byte reasons). That needs its own bound on reason length or entry size.

Rollback: revert the trim in `withOperationReceipt` and the `MaxPoolOperationReceipts` constant.
The pools.json format is unchanged. Receipts already trimmed are not restored; each one remains in
the pools.json that its observation transaction posted.

## V1-2248: plan witnesses judged at the candidate (TOL-V0-132)

`obligations plan --witnesses` judged REVIEWED witnesses at HEAD, while `obligations show` judged
them at the candidate branch commit (TOL-V0-131, V1-1922).
- The plan now takes the candidate from `reviewCandidate`, uses the same `reviewHeadStates` basis,
  and adds `staleBasis {source, commit}` to REVIEWED entries.
- Status names keep their spelling. Output without `--witnesses` is byte-identical to before.
- The `obligations` help note names the basis.

Evidence: `TestTOLV0132_PlanWitnessJudgedAtCandidate`.
- An edit on main leaves the witness COVERED with basis `CANDIDATE_BRANCH`.
- An edit on the candidate gives `REVIEW_STALE_AT_HEAD` at the candidate commit.
- With HEAD forced as the basis, the test fails with "HEAD edited, candidate unchanged:
  REVIEW_STALE_AT_HEAD HEAD@...".

Rollback: revert the `candidate` argument of `planWitnesses`. The bytes without `--witnesses`
never changed.

## V1-1660: Tasks in a `git clone --shared` checkout (CTS-V0-013)

Verification on current source: in a `--shared` clone, `init`, `ticket create`, `queue status`,
`ticket list`, `receipt audit` and `doctor` all return `OK`. Tasks resolves the worktree, `.git`
and `.taskman`, and never reads the object store.
- No Tasks source emits a `state=unsupported` line.
- The ticket's report (lane for PR #1073) most likely came from the dogfood flow. That flow already
  refuses alternates with the named code `unsupported-object-alternates`
  (`2026-10-18-v1-1627-lane-orchestration.md`). This is an inference.

The acceptance asks for "a supported result". `TestCTSV0013_SharedCloneIsSupported` pins that
result with a fixture. No source change was needed.

Rollback: remove the test.

## NOT_RUN and limits

- V1-1570 (pool acquire cost at 10,000 or more receipts, and the Resumed count) and V1-0826 (stage
  worktree cleanup command) were not started because of the lane's 3-hour budget.
- `make gate` was not run (lane policy). Affected tier 2 is left to batch CI.
- During the lane, one read-only `corvint-tasks version` call was made. A lane-built
  `corvint-tasks` binary was also run against scratch clones (reads and one refused `init`), before the
  in-process fixture replaced it. The lane charter forbids both. Neither touched the real store.
