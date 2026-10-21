# Tasks refusals name the blocking attempt (V1-2263, V1-1964)

Lane: tasks-store-1021g. Requirements: ATR-V0-190..192 in
`docs/specs/corvint-tasks-attempt-runner-v0.md`, accepted under the owner's standing auto-accept
(2026-10-11).

## Problem

`ticket complete-manual` was refused `ATTEMPT_LIVE` by an attempt whose lease expired hours
earlier; the refusal and `ticket show` named nothing about it. `RESOURCE_COLLISION` named only an
attempt ID. Operators read `reservations.json` by hand.

## Decisions

- An expired-lease attempt keeps blocking `complete-manual` until `reap`: reservation-set
  membership is the liveness contract and no command reaps implicitly. The refusal names the exact
  reap command instead (ATR-V0-190).
- The writer route decodes no attempt records (it passes an empty attempt list for speed), so the
  mutation-side detail keeps the identity and reports the rest `NOT_OBSERVED`; the CLI adds the full
  summary as a warning from a read after the refusal. This keeps the hot write path unchanged.
- `HolderStatus` moved from `cli` to `transaction` so refusals, `ticket show` and the observation
  report share one classifier.
- V1-1964 criteria 1 and 3 (reap a `STALE_HOLDER` attempt with a live lease; reap a supervised
  attempt with no lease) conflict with the accepted contract that no read, heartbeat, renew or reap
  frees such an attempt. Left for an owner decision; criterion 2 is delivered.

## Evidence

`TestATRV0190_CompleteManualRefusalNamesExpiredAttempt` and
`TestATRV0192_CollisionNamesHoldingAttempt` (`internal/tasks/cli`) pass with the change and fail
with it reverted.

## NOT_RUN

Tier 2 affected units (batch CI); Linux; `make gate` (not required for scoped lane work).

## Rollback

Revert the commit. No stored format changes; `ticket show` drops `currentAttempt`.
