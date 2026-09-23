# Decision 0336 — Behavior-falsification control list cap

Date: 2026-09-22. Status: accepted (batch bug fix, autonomous ticket V1-0043).

## Context

`internal/behaviorfalsify/execute.go`'s `receiptOutputLimit` returns
`max(share, acceptedReceiptBound(control))`, and `acceptedReceiptBound` grows with
`len(control.UnrelatedCriteria)` and `len(control.RequiredSetup)`. `validateControl` in
`internal/behaviorfalsify/plan.go` previously capped those lists only by uniqueness and the ID
pattern, so a control with roughly 2000 criteria could push its `OutputLimit` above the BBF-V0-010
32 MiB document bound; `ensureDocumentBound(report)` then failed the whole report with
`document exceeds bound` instead of the per-receipt floor ever failing closed on its own.

## Decision

`validateControl` refuses a control whose `UnrelatedCriteria` or `RequiredSetup` exceeds
`maxControlListLength = 1000` entries. At that cap, `acceptedReceiptBound` is at most
`8192 * (80 + 2*1000 + 1000) = 25,231,360` bytes (~24.06 MiB), which stays under the 32 MiB
document bound with headroom for the plan's other receipts. BBF-V0-010 in
`docs/specs/browser-behavior-falsification-v0.md` now states the cap next to the existing 32 MiB
figure it protects.

## Rollback

Remove the length check in `validateControl` and the cap constant; revert the BBF-V0-010 sentence.
No wire shape, requirement ID, or existing conformance vector changes.
