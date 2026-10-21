# 2026-10-20: tasks ops performance lane (V1-1656, V1-1193, V1-0436)

## V1-1656: pool identity probe outside the writer lock (PSR-V0-168)

Decision: a confirm-safe for a guarded member with no recorded identity no longer runs the member's
identity command inside the writer route. Under the lock the route's observer (`identityProbe.locked`)
returns the memoised observation when its policy and pools bytes are byte-identical to the ones it was
probed against; otherwise it records them, runs nothing, and declines the route
(`errIdentityDeferred`). `leaseWrite` then settles the probe outside the lock and outside the
preparation gate and retries the writer route once; the retry revalidates the bytes under the lock and
uses the memo. A second miss (pools or policy moved during the probe) falls back to the complete route,
which already prepares outside the lock and rechecks the head under it. The identity guard
(PSR-V0-083/084) and every refusal are unchanged; only where the command runs moved.

Evidence: `TestPSRV0168_IdentityProbeOutsideWriterLock` blocks the identity command mid-run and shows a
concurrent writer committing, the confirmation committing on the writer route after release, and the
command running exactly once. Mutation check: forcing the in-lock probe makes the concurrent writer
fail with LOCK_TIMEOUT after 30 s, so the test catches the regression.

Rollback: revert `pool_identity.go`, `pool_commands.go`, `lease.go` and `lease_write.go`; the probe
runs under the lock again. No format change.

## V1-1193: doctor elided-record walks share receipt reads (TQD-V0-045)

Finding: every attempt whose latest liveness receipt was elided by `CompactLiveness` (CAL-V0-300/302)
walked back to its retained record independently, re-reading and re-decoding up to
`snapshot.MaxElidedRun` receipts per attempt. Decision: one doctor run keeps the decoded posts (or the
read/decode error) per sequence and shares them across walks. Verdicts are byte-identical; nothing is
cached between runs; the scan and walk bounds are unchanged.

Measurement (`CORVINT_DOCTOR_WALK_COST=960 CORVINT_DOCTOR_WALK_ATTEMPTS=24 go test -run
TestTQDV0045_WalkCostMeasurement ./internal/tasks/cli/`, 1,009 receipts, best of five interleaved
runs, host load 29-64 on 12 CPUs): unshared 4.06 s CPU / 3.37 s wall, shared 1.30 s CPU / 1.03 s wall.
Fixture read counts: unshared 496 receipt reads (max 7 per receipt), shared 161 (max 2).

NOT_RUN: the ticket's >=30k-receipt before/after. A CLI-built store costs about 0.2 s per write on this
host (960 heartbeats took 3 min 29 s), so 30k receipts is about 100 minutes, and the only live store
has 9,467 receipts and may not be opened by this lane. Not established: whether GH #791's 55 s came
from walks (elision postdates the report) or from store loading and the journal audit, which belong to
the format and audit code owned by the claude/feat-tasks-perf lane.

Rollback: set `doctorWalkShared` false or revert `doctor.go`; results are identical.

## V1-0436: incremental import (CAL-V0-018)

Already delivered on main by #926: `transaction.ImportSequence`
(`internal/tasks/transaction/import_sequence.go:128`) and `BenchmarkCALV0018_ImportBatchModel`
(`internal/tasks/transaction/import_sequence_bench_test.go:64`); see
`2026-10-15-v1-0436-incremental-import.md`. Criterion 2 remains NOT_RUN: the pinned 2,894-item
Beamfall export was not found on this host, and host load stayed above the CPU count (14-132 on 12
CPUs) throughout the lane. No change in this lane.
