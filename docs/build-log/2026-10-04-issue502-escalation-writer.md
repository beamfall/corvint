# Typed escalation native writer: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699) and the Gate A decisions the owner
accepted as written on 2026-10-04 (decision 0428). The owner asked for this second slice, writer and
material, after the codec slice. Contract: `docs/specs/corvint-tasks-escalations-v0.md`.

## Decision

The writer is a LEASE transaction, not an envelope operation, because the envelope decoder refuses
any operation outside its closed set. `store.Escalate` and `store.AnswerEscalation` take the
canonical `taskman-escalation-request/0` bytes. They post the ticket and one event, or two for a
supersession, through the existing writer, receipt and request-index pipeline. The request's own
digest is the lease request's evidence field, so a retry replays through the request index.

- The planner audits an OPEN's origin from the journal. The named receipt must be a committed ADMIT
  (only CLAIM and CLAIM_NEXT write one) with the named digest, and its POST attempt must have the
  named digest. The source is read from that attempt, never from the request. An origin that
  cannot be audited abstains as `MISSING_ADMISSION_CONTEXT`.
- Grants are per operation: `ESCALATE` and `ANSWER` join the operation vocabulary, and only the
  OWNER row of the default matrix holds them. Policy may narrow them. After issue 559's explicit
  grant list landed, OPERATOR may hold either only through an explicit `policy.roles.OPERATOR` row;
  no other non-owner role may name them. Under the default matrix only OWNER-bound claimers can
  escalate, and they can answer their own questions.
- The request digest covers the invoking actor. The store therefore binds the actor to the request
  before replay, so a different actor gets `ACTOR_BINDING` rather than `REQUEST_ID_CONFLICT`.
- A committed escalation is a ticket-revision lease outcome, so the replay shape check accepts
  `ResultingRevision` for ESCALATE and ANSWER, as it does for COMPLETE.
- `mutation.SetEscalations` is the writer's only way to set the tool-owned key. It finalizes like
  any mutation (record revision, previous digest, size revalidation) and leaves the acceptance
  revision alone, so live attempts are not fenced (ESC-V0-003).

## Findings

- A program attaches supervision after the claim, so the claim's POST attempt is never
  supervised, and a check on it alone let a supervised attempt escalate. The writer now refuses
  `UNSUPPORTED` when either the claimed or the current attempt is supervised. The new test fails
  without the fix.
- ESC-V0-010 is only partly delivered. No stage permission was added, and the bounds hold: a
  supersession measured 6 artifacts and 1421 descriptor bytes against StageLease's 11 and 2658.
  However, the events use the lease stage's existing evidence allowance (the gate-output key, at
  most two), and stage material validation is generic. The distinct closed typed-event stage and
  material branch remain open, and the spec says so.
- Events are retained evidence published before the commit point. Crash point C2 redo republishes
  the ticket and head; a deleted event file is refused `JOURNAL_FORKED`, not recreated.
- The independent review found no blockers. It found that the role separation was overstated, that
  the rollback missed the policy enum widening, and that three evidence claims had no test. The
  wording is corrected, and the deleted-event and shorthand-answer replay cases now have tests.

## Evidence

Native store tests against real claim receipts:
`TestIssue502_OpenAnswerCommitsTicketAndEvents`, `TestIssue502_SupersedePostsBothEvents`,
`TestIssue502_ActorBindingBeforeReplay`, `TestIssue502_OpenAuditsTheClaim`,
`TestIssue502_AnswerRaceHasOneWinner`, `TestIssue502_RedoRepublishesTheTicket`,
`TestIssue502_SupervisedAttemptIsUnsupported`, `TestIssue502_DeletedEventIsJournalDamage` and
`TestIssue502_ShorthandAnswerReplaysAfterLaterOpen` and `TestIssue502_OperatorAnswersThroughExplicitGrant` (reverting the grant makes its OPERATOR policy refused). Not run: a `MaxTicketFileBytes` overflow
witness, `STALE_ADMISSION`, policy narrowing of the two grants, an OPERATOR-bound claimer escalating, a stage with a blob-posted ticket,
interrupted paired publication at each artifact, the CLI, holds, claim delivery and dispatcher
retry. `corvint affected` selected the repository-wide gate; it is NOT_RUN under the owner's
focused-test preference, and the Tasks packages ran in full.

## Rollback

Restore a policy that names neither `ESCALATE` nor `ANSWER`, then revert the writer files and the
grants; a reverted binary refuses a policy naming them. Keep the record-key slice, so readers
still accept a record that already carries a reference.
