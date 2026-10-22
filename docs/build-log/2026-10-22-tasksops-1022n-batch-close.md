# Tasks batch closeout: batch-close writer, store-first delivery screen, multi-target note and hold

Date: 2026-10-22. Lane: claude/tasksops-1022n (base 1125ad00e). Tickets: V1-2396, V1-2397 and
V1-2398 delivered. Requirements `TCD-V0-169` to `TCD-V0-173` in
`docs/specs/corvint-tasks-closeout-debt-v0.md` are accepted under the owner's standing auto-accept
(2026-10-11). `TCD-V0-167..168` were skipped because open PR #1540 uses them.

## Problem

- V1-2396: after batch PR #1525 merged, every delivered ticket was closed by a hand-written
  `ticket complete-manual`. Each one cited the lane PR, the merge sha and the green run URL, with
  the sha256 of the two strings as evidence (V1-0313 shows the shape). `batch-delivered` already
  listed the tickets but wrote nothing, so the orchestrator rebuilt each payload by hand.
- V1-2397: `delivery-screen` judged only PR and commit text. A ticket already COMPLETED, or HELD by
  the owner, could read as `IN_FLIGHT` or `DELIVERED`. The latest operator note was not shown. The
  export had to be a file.
- V1-2398: `note set` and `hold` took one ticket each, and `hold` needed a revision read per ticket
  first. Triage of a sweep took one call, and often two, per ticket.

## Decisions

- `ticket batch-close` reuses the `batch-delivered` parsing and hit derivation, factored into
  `parseBatchInput` and `batchHits` without changing that verb's output. Each close goes through
  `submitMutation`, the same path as `ticket complete-manual`, so receipts, replay and refusals are
  the ordinary ones.
- Planning reads one lock-free snapshot. COMPLETED is skipped. HELD, non-OPEN and live-attempt
  tickets are reported with a code and a recovery and never written. The verb never reaps or
  releases anything.
- Replays are deterministic. The request id is `batch-close-<N>-<local>`. `issuedAt` defaults to
  the export's `mergedAt`. A partial failure can therefore be rerun as is: closed tickets are
  skipped and refused ones retry under the same id.
- The exit status is 0 only when every delivered ticket ends COMPLETED. A held or live ticket keeps
  the call `REFUSED`, so a closer cannot mistake a partial close for a full one. Mentioned-only
  tickets never affect the exit.
- The run URL comes from the export's `url`, else from `--repo`. It is never guessed, because the
  URL string is what the evidence digest binds.
- `delivery-screen` checks the store state before PR evidence: COMPLETED, then HELD. Both stay
  non-`CLEAR`, so the exit rule is unchanged. `operatorNote` reuses the `ticket note show` value.
- Multi-target writes are one ordinary mutation per target in `multiTargetSubmit`. They share an
  `issuedAt` and derive per-target request ids. One shape, `taskman-multi-target/0`, serves both
  `note set` and `hold`. A single target keeps its original output, except that `hold` without
  `--expected-revision` now reads the revision; that call used to be a usage error.
- Several distinct `--target` values on any other mutation verb are now a usage error. Before, the
  last one silently won.

## Evidence

- `TestTCDV0169_BatchCloseClosesDeliveredOpenTickets`, `TestTCDV0170_DeliveryScreenStoreVerdictsAndNotes`,
  `TestTCDV0171_DeliveryScreenReadsPRsFromStdin`, `TestTCDV0172_NoteSetSeveralTickets` and
  `TestTCDV0173_HoldSeveralTicketsReadsRevisions` pass on temp stores. Each one failed with its fix
  disabled: the verb dispatch, the store-first verdict cases, the stdin branch, the positional
  target loop, and the multi-target and revision-read branches.
- The existing `TestTCDV0159`, `TestTCDV0163`, note and hold tests still pass, and so does the
  CAL-V0-131 live-format trap with both new profiles registered as output-only.

## NOT_RUN

- Closing a real merged batch on the live store: the lane charter forbids running `corvint-tasks`
  against it. The first orchestrator use should start with `--dry-run`.
- Tier 2 of `corvint affected` is left to batch CI.

## Rollback

Revert the change. No store format, wire codec, receipt or policy changed. Any close it wrote is
an ordinary `COMPLETE_MANUAL` receipt; reopen a wrong close with `ticket reopen`.
