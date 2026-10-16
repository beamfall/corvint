# V1-1494: lease writes outside the preparation gate (issue 967)

Date: 2026-10-16
Ticket: V1-1494 (GitHub issue beamfall/corvint#967)
Requirements: CAL-V0-263, CAL-V0-264, CAL-V0-265 (`docs/specs/corvint-tasks-agent-leases-v0.md`),
all three proposed. CAL-V0-264 replaces the CAL-V0-205 non-goal and CAL-V0-265 amends CAL-V0-117
step 2, so acceptance is an owner decision (or the standing lane auto-accept, if it applies).

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
- CAL-V0-264: `writerLeaseVerb` admits every lease verb, so with a bound checkpoint no lease verb
  pays the complete audit at all. The route's declines and parity rules are unchanged. The gate is
  entered for the route only when a lock-free precheck (`writerRouteReady`) finds a checkpoint.
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

The same harness file was run on the base worktree and on the branch working tree. Both used
40,000 receipts and 15 writers, on Darwin with 12 CPUs and Go 1.27.2. Host load was 37 to 50
during both runs (other lanes were running), which inflates every absolute time below.

| measure | base 294542f02 | branch |
| --- | --- | --- |
| calls completed in the window | 71 | 216 |
| LOCK_TIMEOUT | 42 of 71 (≈1,116 per hour) | 0 of 216 |
| claim wall p50 / p99 | 30,009 / 30,033 ms (42 of 56 timed out) | 4,275 / 13,625 ms |
| reap gate hold (max) | 117,542 ms (one complete audit) | 719 ms |
| gate hold p50 / p99 / max, all verbs | 0 / 117,542 / 117,542 ms | 252 / 1,322 / 1,543 ms |
| heartbeat wall p50 / p99 | none completed (claims timed out) | 496 / 11,596 ms |
| renew wall p50 / p99 | none completed | 491 / 14,708 ms |
| gate utilisation (sum of holds / elapsed) | 0.99 | 0.72 |

Criterion 2 (no complete audit under the gate) and the LOCK_TIMEOUT part of criterion 3 hold.

The heartbeat/renew p99 below 1 second part of criterion 3 is **not met** on this host. On the
branch, every gate hold is a bounded writer-route call. The p50 hold is 252 ms, mostly
`SnapshotRead` at 280 ms average under this load. But 15 writers at 10-second think time keep the
gate 72% busy, so calls queue in bursts:

- the startup convoy of 15 claims;
- a burst at about 70 s where holds rose to about 1.3 s.

The queueing wait (p99 about 14 s) dominates the p99. After the first 30 s, heartbeat/renew wall p50
is 375 ms and p99 is 12,140 ms. The remaining serialisation is the single writer lock, which every
write needs. Lowering it further means shrinking the writer route's locked snapshot read, which is
issue 967 proposal 2 work. The p99 was not measured on a quiet host, and the Linux p99 was not
measured either.

The retained reports are `docs/build-log/evidence/v1-1494/lease-gate-load-base-294542f02.json`
(SHA256 `94afd9508fc3a502b676b7df11ada1f8ec82caa194d66fe0ceab43867841fadf`) and
`docs/build-log/evidence/v1-1494/lease-gate-load-branch.json` (SHA256
`bcdfe1789ebab9b09ec0a96634e6bfa459e89e98566d145b512fbf3cbe56b32c`). Each contains per-call samples,
host load before and after, and setup time (base about 13 minutes, branch about 12 minutes to build
the store).

## Proposals not delivered (not filed here)

- Proposal 2, remainder: a single-owner background refresh, and removing the `WriterFullBound`
  (4,096) cliff.
- Proposal 3: bounded per-write payload and generation history, segment sealing, and offline full
  verification.
- Proposal 5: `--lock-wait` on `reap`, the current gate holder in status/doctor, and `--timing` for
  every lease verb.
- Proposal 4: no separate liveness admission class. CAL-V0-263/264 remove the complete-audit wait,
  not the bounded writer-lock queue.

## Rollback

Revert the gate scope in `leaseWrite`, the verb list in `writerLeaseVerb`, the refresh fallback
(`AuditWriterRefresh`) and their tests, and the spec section. No store, receipt, checkpoint or wire
format changes, so either build reads stores written by the other.
