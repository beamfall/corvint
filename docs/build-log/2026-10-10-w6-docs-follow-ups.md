# V1-1155, V1-1156, V1-1158 — docs-lane follow-ups; V1-1157 held for an owner decision

Date: 2026-10-11 (filed under the wave's 2026-10-10 prefix). Branch `claude/w6-docs` from
`131b5499753bee0271eade50b6a78609ebf1cf70`. All new requirements are proposed, pending owner
acceptance; no decision file was created and no accepted intent changed.

## Delivered

- **V1-1155.** `corvint changes` read its refs file on a goroutine that a cancelled context
  abandoned. The read now opens the descriptor in the goroutine and a `context.AfterFunc` closes it on
  cancellation, so a pollable stalled reader returns and the goroutine exits; a completed read closes
  its descriptor. A read the kernel cannot interrupt (for example a hard NFS mount) still holds its
  goroutine until the syscall returns; that limit is stated in `UCL1142-V0-004`. Evidence:
  `TestCompileRefsReadHonoursContext` under `-race`.
- **V1-1158 (GitHub #784).** GitHub issue state enters only through `--issue-state-file`, an
  operator-supplied JSON array (for example from `gh issue list --json number,state`); Corvint makes
  no network call (AGENTS.md invariant 7) and labels the state "not verified by Corvint". An OPEN
  issue with closing evidence gets a review hint. Per PR, `new_flags` is a local diff of literal
  flag declarations in non-test `cmd/corvint/*.go` between the two revisions; truncation or overflow
  is `NOT_OBSERVED`. Command diff and reopen/revert semantics stay `NOT_OBSERVED`. New proposed
  requirements `UCL1142-V0-006` and `-007`; `UCL1142-V0-005` amended. The flag diff was a
  non-goal of this still-proposed spec; the slice narrows that non-goal under V1-1158.
- **V1-1156.** A one-shot `unit`/`e2e --retain` run with an unprofiled receipt now records
  `testRepositoryAtStart` from the existing bounded Git observer, so `LPCVD-V0-004` can claim
  revision staleness for basic JS receipts (`LPCVD-V0-005`; `LPCVD-V0-002` wording amended). Go
  events and JS watch-mode receipts still retain no revision and stay `UNKNOWN`.

## Held: V1-1157 (GitHub #785)

A frontier `reason` field changes the `corvint-harness-event/0` wire shared by four host adapters
(including closed output) and conformance fixtures, and an acknowledge/suppress flow would let
adopter-owned state alter notice presentation under the AHI notice rules and decision 0161. Both
change existing authority and wire rules, so the lane stopped for an owner decision and changed
nothing for this ticket.

## Rollback

Revert the three ticket commits. Retained basic receipts carrying `testRepositoryAtStart` still
decode; discovery treats documents without it as `UNKNOWN` as before. No store or migration exists.
