# V1-1494: lease writes outside the preparation gate (issue 967)

Date: 2026-10-16
Ticket: V1-1494 (GitHub issue beamfall/corvint#967)
Requirements: CAL-V0-263 and CAL-V0-265 (`docs/specs/corvint-tasks-agent-leases-v0.md`), both
proposed. CAL-V0-265 amends CAL-V0-117 step 2, so acceptance is an owner decision (or the standing
lane auto-accept, if it applies). CAL-V0-264 was proposed and then declined by the owner (see
"Owner decision" below); its ID is not reused.

## Problem

On a queue of about 42,000 receipts with about 15 writers, lease writes failed `LOCK_TIMEOUT` about
150 times an hour, and every timeout named `taskman.prepare.lock`. `leaseWrite` acquired the gate
and only then prepared. Every lease verb outside the writer route (pool family, reap, widen, submit,
complete) ran the complete audit from receipt 1 while it held the gate, refusals included. Heartbeat
and renew are on the writer route, but they still queued behind those audits for the gate. The gate
wait therefore grew with the journal.

## Design

Accepted semantics are unchanged. The design moves the work and keeps the locked recheck:

- CAL-V0-263: `prepareLease` (inventory, complete audit or verified cache reuse, replay lookup,
  model, handoff history audit, guard sweep) runs before the gate. A preparation that plans no
  transaction is reported under the writer lock after the same head and observation recheck as
  before, without entering the gate. The gate (`underPreparation`) surrounds only the writer route
  and the locked commit of a planned or pending-receipt preparation plus monitor teardown. If the
  locked recheck finds the head moved, the result is `SNAPSHOT_MOVED`, and the existing 16-round loop
  prepares again. Receipt bytes, audit verdicts, receipt order, the issue 494 admission order and
  every lock bound are unchanged.
- The gate is entered for the writer route only when a lock-free precheck (`writerRouteReady`)
  finds a writer-route verb, a head at or above the route floor and a checkpoint file. The
  writer-route verb list (`writerLeaseVerb`) is unchanged from the base.
- CAL-V0-265: the scheduled writer-checkpoint refresh first makes one complete attempt. On
  `SNAPSHOT_MOVED`, it walks receipts 1..H for the captured head H and accepts the result only if a
  fresh listing shows 1..H unchanged. A fault refuses only on a held journal. A prefix result
  retains a writer checkpoint and never a read checkpoint. Without this, the refresh never finishes
  on a store that appends faster than a complete audit, and the route never binds.

## Evidence

The fail-on-base pin is `TestCALV0263_NoCompleteAuditUnderGate`. An audit hook probes the gate
while every complete audit of a lease preparation runs. On base `294542f02` all three subtests
(planned claim, refusal, no-op reap) fail, each with "audits 1, 1 under the gate". On the branch all
three pass.

For the mutation check, `prepareLease` was moved back inside `underPreparation` on the branch. That
failed all three `TestCALV0263_NoCompleteAuditUnderGate` subtests, plus
`TestCALV0026_PreparationGateStore/held-gate-waits-only-at-commit` (deadline exceeded) and
`TestCALV0026_PreparationGateStore/cooperating-preparations-overlap`.

`TestCALV0117_WriterAdvanceAndScheduledRefresh` was reported to the coordinator as failing ("read
checkpoint at 581, want 582"). That failure came from an intermediate version of this change, a
prefix-only refresh that retained no read checkpoint. It was never a base failure. The test passes
on the base and on the branch with the final complete-attempt-first refresh, including
`-count=3` of every `TestCALV0117_` test.

## Load harness

`TestCALV0263_LeaseGateLoad` (`internal/tasks/store`) runs only when
`CORVINT_TASKS_LEASE_GATE_LOAD=<absolute report path>` is set. It builds a synthetic temp store and
re-executes the test binary as 15 worker processes for 120 seconds with 10-second think time:

- worker 0 reaps;
- workers 1-2 claim, then widen;
- the other workers claim, then alternate heartbeat and renew;
- every worker releases at the deadline.

Each call records gate wait and hold, writer-lock hold and `LeaseTiming`. The in-process audit
cache is cleared before every call, so each call behaves like a fresh CLI process.

The same harness file was run on the base worktree and twice on the branch: once with the declined
CAL-V0-264 (superseded, kept for comparison) and once on the final code (CAL-V0-263 and CAL-V0-265
only). All three used 40,000 receipts and 15 writers, on Darwin with 12 CPUs and Go 1.27.2. Other
lanes were running, so host load was 37 to 50 for the first two runs and 22 to 40 for the final run.
Absolute times are inflated and are not comparable across columns at the millisecond level.

| measure | base 294542f02 | with CAL-V0-264 (declined) | final branch |
| --- | --- | --- | --- |
| calls completed in the window | 71 | 216 | 273 |
| LOCK_TIMEOUT | 42 of 71 (≈1,116 per hour) | 0 of 216 | 0 of 273 |
| claim wall p50 / p99 | 30,009 / 30,033 ms (42 of 56 timed out) | 4,275 / 13,625 ms | 2,595 / 4,181 ms |
| gate hold p50 / p99 / max, all gated calls | 0 / 117,542 / 117,542 ms | 252 / 1,322 / 1,543 ms | 252 / 788 / 917 ms |
| heartbeat wall p50 / p99 | none completed (claims timed out) | 496 / 11,596 ms | 352 / 1,962 ms |
| renew wall p50 / p99 | none completed | 491 / 14,708 ms | 407 / 3,547 ms |
| reap | 1 call, held the gate 117,542 ms, succeeded | 14 calls, all succeeded | 4 calls, all `SNAPSHOT_MOVED` after 16 rounds (21 to 33 s each), none entered the gate |
| widen | none (claims timed out) | 26 calls, all succeeded | 5 calls: 4 `SNAPSHOT_MOVED` after 16 rounds, 1 `STORAGE_FAILED` (harness artifact, below); none entered the gate |
| gate utilisation (sum of holds / elapsed) | 0.99 | 0.72 | 0.44 |

Criterion 2 (no complete audit under the gate) and the LOCK_TIMEOUT part of criterion 3 hold in
the final run: no call timed out, and the largest gate hold was 917 ms, a bounded writer-route call.

The heartbeat/renew p99 below 1 second part of criterion 3 is **not met**. In the final run
heartbeat p99 is 1,962 ms and renew p99 is 3,547 ms, almost all of it gate wait (p99 1,598 and
2,863 ms) behind other bounded writer-route holds (p50 about 250 ms, mostly `SnapshotRead` under
the writer lock). The remaining serialisation is the single writer lock, which every write needs.
Lowering it further means shrinking the writer route's locked snapshot read, which is issue 967
proposal 2 work. The p99 was not measured on a quiet host, and the Linux p99 was not measured
either.

The final run shows the cost the owner accepted when declining CAL-V0-264. Widen and reap prepare
the complete route outside the gate (11 to 61 s of `SnapshotRead` per call at this load), and the
heartbeat/renew traffic moves the head before each preparation reaches its locked recheck, so
every round was invalidated and each call ended `SNAPSHOT_MOVED` (retryable) after 16 rounds. On
the base the same verbs waited at the gate instead and, when they got it, held it for a complete
audit (117,542 ms) while every other writer timed out.

The one widen `STORAGE_FAILED` was a harness artifact. The harness passed a single `recordedAt`
sample and no live clock, while the command line gives every writer a live clock
(`cli.writerContext` → `store.WithClock`). That widen started 1.3 s before the deadline and
prepared for 43 s, so other writers' receipts moved the head's `recordedAt` past its sample, and
the model refused the stale sample as a clock stepping backward (CAL-V0-012). After the run the
harness was changed to use `WithClock(ctx, WallClock)` as the CLI does. A rerun with that change
did not finish inside the 10-minute foreground window: host load rose to 62, and two widen workers
were still retrying past the deadline when the test timed out. Its numbers are `NOT_RUN`.

The retained reports are:

- `docs/build-log/evidence/v1-1494/lease-gate-load-base-294542f02.json` (SHA256
  `94afd9508fc3a502b676b7df11ada1f8ec82caa194d66fe0ceab43867841fadf`);
- `docs/build-log/evidence/v1-1494/lease-gate-load-branch.json`, the superseded run with
  CAL-V0-264 (SHA256 `bcdfe1789ebab9b09ec0a96634e6bfa459e89e98566d145b512fbf3cbe56b32c`);
- `docs/build-log/evidence/v1-1494/lease-gate-load-branch-final.json`, the final code (SHA256
  `48983c7fb3a995c12e5a61d0a2f06a25dcb31a115df4210150da5deb42c2b707`).

Each contains per-call samples, host load before and after, and setup time (base about 13 minutes,
the CAL-V0-264 run about 12 minutes, the final run about 5.4 minutes to build the store).

## Independent review (one Codex round)

- Major, resolved by the owner declining CAL-V0-264: CAL-V0-264 routed widen, reap and the
  pool family through the writer route. Those verbs then inherit the accepted CAL-V0-116
  trade-off: a fast writer does not detect a rewrite of the receipt prefix before its checkpoint,
  which their complete audit used to refuse. That changes what those verbs' audit refuses.
- Major, fixed: the prefix walk's recheck covered only receipts. `prefixHeld` now also requires
  every compared request, evidence and pinned afterimage to be the observed file (or still
  absent). `TestCALV0265_PrefixRecheckAfterWalk` pins this and fails when the blob recheck is
  removed.
- Minor, fixed: added negative recheck coverage and the `Checkpoint() == nil` assertion for a
  prefix result.
- Minor, fixed: the spec's failure modes now list the writer-route declines that still prepare
  the complete route.

## Owner decision: CAL-V0-264 declined

The owner (through the coordinator, 2026-10-16) declined CAL-V0-264 and kept CAL-V0-263 and
CAL-V0-265. The reason: a retryable `SNAPSHOT_MOVED` on widen, reap and the pool family is better
than losing CAL-V0-116 rewritten-prefix detection for those verbs. The writer-route change for
those verbs was reverted; `writerLeaseVerb` serves the base set (claim, claim-next, renew,
heartbeat, release and pool confirm-safe). The coordinator files the `SNAPSHOT_MOVED` retry
follow-up.

## Limits

- The heartbeat/renew p99 below 1 second target (criterion 3) is not met on this host.
- Issue 967 proposals 2 (remainder), 3 and 5 are not delivered (listed below, not filed).
- Widen, reap and the pool verbs other than confirm-safe always prepare the complete route, now
  outside the gate. Under sustained appends they can exhaust 16 rounds and fail `SNAPSHOT_MOVED`
  (retryable), where they used to wait at the gate and succeed. In the final harness run every
  widen and reap call that the harness clock did not refuse (8 of 9) ended this way.
- Not run: Linux, a quiet-host measurement, live fleet qualification, the full `internal/tasks/store`
  package under quiet load, a full `-race` store run, and `make gate`.

## Proposals not delivered (not filed here)

- Proposal 2, remainder: a single-owner background refresh, and removing the `WriterFullBound`
  (4,096) cliff.
- Proposal 3: bounded per-write payload and generation history, segment sealing, and offline full
  verification.
- Proposal 5: `--lock-wait` on `reap`, the current gate holder in status/doctor, and `--timing` for
  every lease verb.
- Proposal 4: no separate liveness admission class. CAL-V0-263 removes the complete-audit wait,
  not the bounded writer-lock queue.

## Rollback

Revert the gate scope in `leaseWrite`, the `writerRouteReady` precheck, the refresh fallback
(`AuditWriterRefresh`) and their tests, and the spec section. No store, receipt, checkpoint or wire
format changes, so either build reads stores written by the other.
