# Tasks: sample recordedAt after the head is held (issue 437)

Human-owned intent: [GitHub #437](https://github.com/beamfall/corvint/issues/437) and native ticket
V1-0532. CAL-V0-012 governs; this amends its text and adds no wire field, code or receipt shape.

## Finding

Every store writer received `recordedAt` from its caller, sampled before the writer waited for the
store lock (mutate, policy, barrier, release, reconcile, import) or before a lease preparation read
the head. A second writer that committed during that wait moved the head receipt to a later second,
and the first writer was then refused by the CAL-V0-012 backward-clock check as
`STORAGE_FAILED`. No clock had stepped backward. The refusal wrote nothing, which is why an
unchanged retry succeeded (observed on 2026-09-30 under build 202 and on 2026-10-01 with about ten
concurrent agents).

## Decision

`store.WithClock` attaches a live clock to a writer's context. `recordedAt` is sampled again once
the writer holds the head it plans against: after `AcquireLock` in the lock-first writers, and
after the head read in `prepareLease`, where a head that moves later already fails the commit as
`SNAPSHOT_MOVED` and the next round samples again. The later of the live sample and the caller's
timestamp is used. The command line attaches the wall clock to every writer, and the pool, program
and workflow writers that already sampled the wall clock attach it too.

Rejected: clamping `recordedAt` up to the head receipt's time. It would record a time the clock
never read and would hide a clock that really stepped backward. Rejected: a new retryable outcome
code. With the sample taken at the head, the refusal surfaces only for a real backward clock, where
`STORAGE_FAILED` before any write remains the CAL-V0-012 contract; a new code would be a wire change.

A writer without a live clock (the deterministic store tests, and any embedding caller that passes
only a timestamp) behaves exactly as before.

## Evidence

- `TestCALV0012_WriterBehindNewerHeadSamplesAgain`: a mutation and a claim whose timestamps predate
  a newer head complete under a live clock, the lease expiry is counted from the sampled time, and
  a live clock behind the head still refuses a mutation and a renew without changing the store. The
  test failed (`STORAGE_FAILED` on the first mutation) with the resample disabled.
  A renew whose first sample is overtaken by a write between its head read and its commit completes
  on the retried round with the second sample (added after independent review, which found no
  defect but noted the first cases did not move the head mid-write). The lock-first writers have
  no such mid-write case: their sample follows the lock, which the test does not observe directly.
- `TestCALV0012_BackwardClockRefusesEveryWriter` is unchanged and still passes.

The natural multi-agent race was not reproduced; the test reproduces its mechanism (a head newer
than the caller's sample) deterministically. Timestamps have one-second resolution and are compared
on one host; clock disagreement between hosts sharing one store is outside this change.

Rollback: revert this change's commits. No store state, receipt or policy is migrated.
