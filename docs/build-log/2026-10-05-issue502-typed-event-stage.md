# Typed-event stage and escalation binding fold: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699), ESC-V0-010, and the Gate A decisions
the owner accepted as written on 2026-10-04 (decision 0428, item 7: supersession within the
StageLease bounds). The owner asked for this seventh slice, the typed-event stage, after dispatcher
retry. Contract: `docs/specs/corvint-tasks-escalations-v0.md`.

## Decision

Agent decision, 2026-10-05, under the owner's in-task delegation; the owner may redirect it.

- A committed escalation stages under a new closed stage operation, `ESCALATION`, instead of
  widening the LEASE shape. It admits the request entry, exactly one ticket, one or two events at
  their content addresses (two only for a supersession) and at most one evidence blob, which must
  be that ticket's record. Nothing else is admitted. The measured maximum is 7 artifacts and 1879
  descriptor bytes, inside the StageLease limits of 11 and 2658, so decision 0428 item 7 holds.
- The writer, its LEASE request digest, request-index entries and native supersession are
  unchanged. `freeze` selects the operation only for a committed ESCALATE or ANSWER; refusals are
  unrecorded and never stage.
- ESC-V0-010 is revised in place, keeping its ID: it now names the distinct stage, records the
  2026-10-04 MUTATE-slot decision and its supersession by decision 0428, and states that a question
  reference may change only in one completed TRANSITION and is never dropped.
- The closed material branch is a pure receipt fold, `transaction.EscalationReceiptAudit`. For each
  ticket post whose references differ from the folded predecessor it requires a completed
  TRANSITION posting one ticket, rebuilds the typed LEASE request digest from the event's original
  request and the receipt's actor and binds it to the posted request entry, requires the verb's
  grant in the retained policy, requires an OPEN's source to be exactly a folded completed claim
  admission that is still the current, unsupervised, reserved and unexpired attempt, and replays the pure reducer and the
  ticket finalizer to the posted bytes. It runs in one pass with the external-review fold for
  receipt audit and both redo paths (`FoldReceiptBindings`).

The native store never persists the stage operation: it lives in the frozen plan and is validated
when the plan freezes, so no stored byte changes.

## Findings fixed in this slice

- The lease-path redo (`redoLeaseProof`) never ran the binding fold that the mutation redo runs,
  so an interrupted escalation with a self-consistent forged event would have been republished.
  It now runs the fold before republishing posts.
- ADMIT receipts name no ticket, so the fold reads the admission's queue from the admitted attempt.
- Supervision posted after the claim is now read from the latest folded attempt, as the writer
  reads it.

## Evidence

Snapshot package: `TestESCV0010_EscalationStageNarrow` (zero or three events, attempt and
reservation posts, an event off its address, a non-ticket blob, a missing ticket, an oversized
event) and `TestTMV0002_AS10_StageCodecActualMaxima` (measured maximum). Store package:
`TestIssue502_SupersedePostsBothEvents`, `TestIssue502_InterruptedSupersessionRedoes` (a fault at
every published artifact of a supersession; the retry commits once and redoes exactly when the
receipt was published), `TestIssue502_ForgedEventRefusesAsJournalDamage` (forged time, question and
actor pass the generic journal audit; the fold refuses them settled, and the pending redo refuses
`JOURNAL_FORKED` with head and ticket unchanged), `TestIssue502_EscalationBindingFoldRefusals` (including an extra post, a removed source attempt,
no matching reservation, an expired lease, and an unreferenced event accepted) and
`TestIssue502_SupervisedSinceClaimFoldRefuses`. The existing writer, CLI and dispatcher escalation
tests pass unchanged.

Verification (`-count=1 -timeout 30m`): the `snapshot`, `transaction`, `wire`, `ticket`, `cli`,
`store` and `dispatch` packages under `internal/tasks/`, plus `internal/taskman` and
`internal/specindex`, pass, and `go run ./conformance/use-cases-v0` exits 0. One earlier run on a
loaded host failed `TestCALV0078_SupervisedRunAfterCommitIsNotRetryable/refresh-STOPPED` before its
fault point: the implementer attempt ended `BLOCKED_RECOVERY` with survivors. It passed with
`-count=3` in isolation and in the final run, so it is filed as a suspected flake, not attributed
to this change.

## Review

One independent read-only Codex review of `b1e37605..3f062ecb` found no blocker, four major
findings and one minor; all were accepted and fixed in a follow-up commit:

- Supervision was a never-cleared flag per attempt ID, so a later unsupervised generation of the
  same ID could not escalate in audit. The fold now keeps the latest attempt per ID and reads
  supervision from it, as the writer does.
- The fold accepted a fenced or expired source: it checked only that the admission existed. It now
  requires the latest folded attempt to be the source's live generation, holder, ticket record and
  acceptance, unexpired at the receipt's time and matched by the latest folded reservations.
- A bound transition could carry an extra post, such as a policy, which redo would publish. It now
  posts only its request entry, its ticket and the replayed events.
- An escalation event posted while the ticket's references stayed unchanged skipped the fold. A
  first fix refused any such event; the re-review below withdrew it.
- The rollout text claimed an older binary refuses an in-flight `ESCALATION` plan. The operation is
  never persisted, so an older binary recovers through its own generic redo and only loses the
  fold's checks; the spec says so now.

Each new refusal names a detail only the fix produces, so the new cases cannot pass without it.
A supervised generation followed by an unsupervised one of the same attempt ID has no live witness.

A read-only Codex re-review of the fix commit found two issues with the event refusal. It was
not enforced on redo of a ticketless receipt (P1). It also falsely refused legitimate gate output
that captured event-shaped bytes, because evidence is content addressed (P2). Bind replays every
referenced event from the reducer, so an unreferenced event blob installs nothing. The refusal was
removed, and the test case now asserts that such a blob is accepted. This resolves both findings.

## NOT_RUN

- A live-store integrated witness and a ticket over the inline POST bound taking the blob slot.
- Kinds and ages in the `dispatch status` escalation section (ESC-V0-009), and the
  repository-wide gate, under the owner's focused-test preference.

## Rollback

No stored byte format changes and the stage operation is never persisted, so an older binary
recovers any interrupted write through its own generic redo. Revert the change; the fold is a pure
audit, and removing it returns audit and redo to the generic journal checks.
