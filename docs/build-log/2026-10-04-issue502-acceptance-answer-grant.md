# Typed escalations: owner acceptance and operation-scoped answer grant (issue 502)

Human-owned intent: GitHub issue 502 (native ticket V1-0699). Owner decision 2026-10-04 accepts
the ESC-V0 intent as drafted. The acceptance covers the Gate A decisions with their defaults: one
current event per question, sole-open-at-commit shorthand next to exact CAS, infrastructure
exhaustion as a visible automation hold, the typed-control `workRevision` exclusion and the
reference capacities, including the current-acceptance 16-open bound. It also covers the
stale-OPEN fix that `2026-10-04-issue502-foundation-hardening.md` delivered. The spec header,
`INDEX.json` and the README row now record `accepted`. Delivery stays `experimental`.

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
contract. The stage choice is therefore an open owner decision (spec section Integration slot).

## Evidence

`TestIssue502_AnswerGrantAndEmptyShorthand` passes. As a negative control, the test fails with
the operation check removed ("want POLICY_NOT_ALLOWED, got <nil>"). Every integrated witness in
the spec's acceptance table remains NOT_RUN, and V1-0699 stays open.

## Rollback

Revert this change. No byte format, store or installed behavior changes.
