# Typed escalations: operation-scoped answer grant and writer slot (issue 502)

Human-owned intent: GitHub issue 502 (native ticket V1-0699). The owner's Gate A acceptance is
recorded by decision 0428 and is not restated here; the spec header, `INDEX.json` and the README
row carry its status. This entry records what the slice adds on top: the operation-scoped grant,
`NO_OPEN_QUESTION`, and a second owner decision of 2026-10-04 that chose the writer slot. The owner
gave the slot decision directly, through structured questions in the orchestrating agent session.
Delivery stays `experimental`.

## Decision

This slice builds on the foundation hardening and stays inside the same pure files:

- `EscalationObservation.PolicyOperation` names the one request operation a supplied grant covers.
  A mismatch refuses `POLICY_NOT_ALLOWED`, so a source holder's OPEN grant is never answer
  authority (ESC-V0-004). Before this change the finding was assigned to the writer slice. Pinning
  it in the reducer means the writer cannot forget it; the writer must supply the operation from
  the policy matrix.
- Shorthand ANSWER with no current open question refuses `NO_OPEN_QUESTION`, not
  `QUESTION_NOT_FOUND`. ESC-V0-004 requires the zero case to be distinguishable from an exact
  request ID that does not exist.

## Why the native integration is not in this slice

Native writes need a stage. Issue 501's operator-note slice adds a MUTATE derived-event slot. That
slice is not on public main, and it admits exactly one event per MUTATE; its measured worst case is
6 of 6 artifacts and 1669 of 1670 descriptor bytes. OPEN and ANSWER fit one event, but paired
supersession (ESC-V0-003) does not. ESC-V0-010 names a distinct typed-event stage, and its kill
criterion forbids widening bounds. Building on an unmerged slot, or copying it, would fork the 501
contract. The stage choice was put to the owner.

Owner decision 2026-10-04: the native writer reuses the 501 single derived-event MUTATE slot.
Escalate and answer each declare the derived event per operation in the transaction model, and
native supersession is deferred until a distinct typed-event stage fits StageLease bounds.
ESC-V0-010 is revised in place, with its ID kept, and ESC-V0-003 notes the deferral. The 501 review
found that the slot was not yet bound to an operation and that raw import could inject note state.
The spec therefore requires the escalation writer to declare its event, to bind it in receipt
audit and redo, and to refuse the `escalations` reference in `importChain`. The writer itself
waits until 501 lands and is remainder of V1-0699.

## Evidence

`error-code-ownership-check` matches kebab-case codes only, so it does not see the upper-case
reducer codes. `NO_OPEN_QUESTION` is owned by the ESC-V0-004 text, not by that gate.

`TestIssue502_AnswerGrantAndEmptyShorthand` passes. As a negative control, the test fails with
the operation check removed ("want POLICY_NOT_ALLOWED, got <nil>"). Every integrated witness in
the spec's acceptance table remains NOT_RUN, and V1-0699 stays open.

## Rollback

Revert this change. No byte format, store or installed behavior changes.
