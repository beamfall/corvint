# V1-0169: the TCP-V0-029 span-scorer readings are not independently reproducible

Date: 2026-10-21. Lane: claude/batch-dogfood-flow (base 5cf75daf0). Requirement: `TCP-V0-029`
(proposed, experimental; decision 0366).

The V1-0098 entry in `docs/BUILD-LOG.md` ("2026-09-23 V1-0098 TCP-V0-025..029") reports span
recall readings for five subsets from an offline span scorer that was never committed. The
independent review of PR #99 reran that scorer from the session scratchpad and reproduced each
number. The script was never committed, so nothing in the repository can reproduce the
readings.

Decision: the ticket's second acceptance route. Rebuilding the scorer now would be a new
evaluation, not a reproduction of the reviewed one. Without the original inputs it could not show
that it scores the spans the same way. The record is labelled instead:

- NOT_PRODUCED: a committed TCP-V0-029 span scorer under `tools/retrieval-bench`.
- The V1-0098 span readings are not independently reproducible from the repository. They are
  retained as a session observation that one independent reviewer reproduced, and they are not
  evaluation evidence for promoting TCP-V0-029.
- `docs/specs/task-context-packet-v0.md` says the same in its TCP-V0-029 trace row and in its
  pointer to the reading. `docs/BUILD-LOG.md` is closed (decision 0423), so the V1-0098 entry
  itself is unchanged; this entry is the correction.

Rollback: delete this file and restore the two spec sentences. No code or reading changes.
Promoting TCP-V0-029 still needs a committed scorer and a fresh frozen evaluation.
