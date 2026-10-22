# 2026-10-21 taskscli-1021r: corvint-tasks refusal and recovery messages

Lane taskscli-1021r, base `ef2a01db52d5436c262725d136060c9eb95e8580`. Tickets in priority order:
V1-0830, V1-1785, V1-1438, V1-1874, V1-1895, V1-0828, V1-1020, V1-0324, V1-1964.

## Problem

Each reported problem was reproduced against the base code before building:

- V1-0830: the dispatch verbs' usage line said `[--grant FILE]`, but the flag takes a grant ID.
- V1-1785: the `receipt compact` help described the compaction cost of a complete walk. It did not
  say that the writer-checkpoint and read-checkpoint routes avoid that cost, or that compaction
  forces one complete audit per route.
- V1-1438: after a mutation's output is lost, a later update moved the ticket's `updatedAt`. The
  CAL-V0-250 explanation then could not recover the defaulted issuedAt.
- V1-1874: an `APPROVAL_REQUIRED` ticket named only the missing `RUN` grant at claim. The missing
  `COMPLETE` grant surfaced only at `complete`.
- V1-0828: an expired-lease `FENCED` refusal named only the expiry.

## Decisions

Requirements ATR-V0-270..275 (`docs/specs/corvint-tasks-attempt-runner-v0.md`) and PSR-V0-185
(`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`) are accepted under the owner's standing
auto-accept (2026-10-11). The IDs were checked against origin/main (ATR max 212, PSR max 169) and
open PRs #1468 and #1474 (ATR-V0-213, 240, 241).

- V1-1438: mutation results do not carry `issuedAt`. The result wire contract stays closed, and a
  member in lost output is lost too. A defaulted issuedAt equals the writer `now` that the
  mutation's receipt records as `recordedAt`, so the explanation now rebuilds the request digest
  from the receipt that the request-index entry names. This is a read: it never restores or writes
  a receipt.
- V1-1874: `MissingGrants` reports `RUN` and `COMPLETE` at the acceptance revision. The claim
  warning reads the ticket file only when its digest equals the attempt's `ticketRecordSha256`.
  `INTEGRATE` grants (supervised programs) are out of scope.

Tickets this lane did not build:

- V1-1895 (complete bound to a merge-queue merge): skipped. It needs an owner design decision.
- V1-1020 (macOS TCC prompt): skipped. The prompt was not reproduced (NOT_OBSERVED).
- V1-0324: skipped. The ticket holds no narrowing text, the governing spec is the external
  TCP-00 §3.3/C6, and its acceptance requires the full gate.
- V1-1964: criterion 2 was already delivered by `119ed45a2` (ATR-V0-192,
  `TestATRV0192_CollisionNamesHoldingAttempt`). Criteria 1 and 3 conflict with the accepted
  ATR-V0-190..192 non-goals, and the ticket holds no narrowing text.

## Evidence

The new tests are in `internal/tasks/cli`:

- `TestATRV0270_DispatchUsageNamesGrantID`
- `TestPSRV0185_CompactHelpNamesFastRoute`
- `TestATRV0271_GateRunAfterExpiryNamesRemedy`
- `TestATRV0272_ApprovalGrantsNamedUpFront` (covers ATR-V0-272..274)
- `TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers`

All five failed with the fix sources restored to the base and pass with the fix. Package, trap and
doc-gate results are in the PR body.

## NOT_RUN

- `make gate`: not run (lane charter).
- Affected tier-2 packages beyond the cap: left to batch CI.
- V1-1020 live TCC reproduction: NOT_OBSERVED.

## Rollback

Revert the commit. The help text, the expiry refusal and the claim blocker return to their base
wording. `missingGrants` and the claim grant warning disappear, and the conflict explanation again
tries only `createdAt` and `updatedAt`. No stored record or wire profile changes.
