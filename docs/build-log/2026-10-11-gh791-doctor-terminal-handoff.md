# GitHub 791: doctor skips finished tickets for NO_PROGRESS_HANDOFF

Date: 2026-10-11

GitHub issue 791 (ticket V1-1176) reports that `corvint-tasks doctor` lists COMPLETED tickets as
NO_PROGRESS_HANDOFF. The remedy for that finding is to refine, split or hold the ticket, which
does not apply to a ticket that is already finished. In one run, 3 of 15 alerts were completed
tickets.

Change: the no-progress detector now skips any ticket whose current audited record is
COMPLETED, ARCHIVED or HELD. It reads the same inventory that SETUP_ONLY_PROOF already uses, so
the doctor stays a pure read and makes no extra journal read. This is proposed TQD-V0-013 in
[the doctor contract](../specs/corvint-tasks-doctor-v0.md), pending owner acceptance (V1-1176).
No decision is recorded here.

Design choices:

- **Current status, not journal transitions.** Ticket records live in the intent tree, and the
  receipt kind `MUTATION` does not say which operation ran. Resetting the streak at the exact
  completion receipt would therefore mean reading ticket record history. The current-status test
  covers the reported case. It leaves one limit, recorded as a failure mode: if a ticket is
  reopened after completion, its earlier hand-offs still count.
- **HELD is excluded, not downgraded.** The issue offered an informational finding for held
  tickets as an option. A hold already does what the remedy asks for, so the finding is dropped.

Evidence: `TestTQDV0013_NoProgressSkipsCompletedHeldArchived` gives four tickets three no-progress
hand-offs each. All four are reported. The test then completes one ticket, holds one and
archives one, and only the OPEN ticket is still reported. Without the fix, the test fails on the
completed ticket.

Out of scope, proposed as follow-ups: the roughly 55 s scan of 4096 receipts on a journal of
about 30k receipts, and documenting whether `doctor --line` reuses a cached scan.
