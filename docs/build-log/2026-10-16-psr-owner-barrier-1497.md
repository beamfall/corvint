# 2026-10-16 TestPSRSweepOwnerBarrier host-load failure (V1-1497)

Base `294542f02c8cef48b50c2babcd17504339019eea`. Darwin, shared with other agent lanes, no burners:
the host load was measured as it was. Only `internal/tasks/store/pool_sweep_test.go` changed; no
product deadline, budget or allowance was changed.

## Root cause

- The test gave the sweep a fixed 2s wall-clock bound to reach the verify marker. The sweep ran with
  the product's fixed observation bounds and the fixture's 3s attempt bound, which also had to cover
  the whole time verify was held open by the test. Under load an earlier observation timed out, or
  the sweep had not yet reached verify after 2s.
- The barrier assertion was vacuous. The manual pool-safe and pool-cleanup commands were recorded at
  the fixture time `t0`, which is earlier than the sweep owner's wall-clock receipt. The clock check
  refused them with `STORAGE_FAILED` before any pool logic ran, and the test accepted any outcome
  other than `COMPLETED`. With only the timing fix applied, mutating `planPoolSweepSafe` to complete
  for a non-owner still passed 3/3, and both manual outcomes were `STORAGE_FAILED`.
- No production code holds a 2s deadline that the test exposed. The product's fixed Git-read and
  process-probe bounds (V1-1324) apply, and other tests own them.

## Fix

- The sweep uses the V1-1324 load-tolerant test bounds (`psrLoadTolerant`, `psrRunLoaded`).
- The test waits for either the verify marker or the sweep's own bounded return. A return before the
  marker fails with the sweep report and pool states. The join waits end at 90% of `t.Deadline()`.
- The manual commands run at the wall clock (`now(t)`). Each must be refused `BLOCKED` with exactly
  `RESOURCE_COLLISION`, which is the PSR-V0-003 owner refusal.

## Evidence

- BASE, `-count=20`, load 57-53 (start) to 52-62 (end): 11/20 FAIL, all `verify barrier not reached`.
- Fixed, `-count=20`, load 44-58 to 52-56: 20/20 PASS (2.4s-15.2s). Final version, load 102-80 to
  78-90: 20/20 PASS.
- Mutations, each restored after the run:
  - `planPoolSweepSafe` completes for a non-owner: 2/2 FAIL `manual POOL_CONFIRM_SAFE stole owner`.
  - `planPoolCleanup` skips its `Sweep != nil` refusal: 2/2 FAIL `manual POOL_CLEANUP not refused
    by the sweep owner` (`QUIESCENCE_UNPROVED`).

## The `program_*` 30m package timeout is a different class

The `program_*` tests alone (56 top-level tests, `-timeout 9m`, load 66-86) ran out of time after 29
passes. Single tests took 40-93s. `TestCALV0089_IntegrateCheckpointRestartKeepsGrant` also failed
an assertion at `program_continuation_test.go:612` after 65s. This is aggregate package runtime
under load plus at least one state assertion, not a fixed short wall-clock bound in a test. It is
left for a separate ticket.
