# Typed escalation foundation hardening: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699). The contract
`docs/specs/corvint-tasks-escalations-v0.md` stays proposed intent with experimental delivery, and
no owner acceptance of its Gate A decisions is recorded.

## Decision

The spec's rollout order puts the native writer and material next. That slice integrates only after
the owner accepts the Gate A decisions, and the first delivery's independent review left one
finding that had to be fixed "before integration". This slice therefore stays inside the same four
pure files and resolves the review findings that do not need the writer or the CLI.

- **Stale OPEN capacity.** An OPEN question from an earlier acceptance revision can be neither
  answered nor superseded, yet counted toward the 16-open bound, so sixteen stale questions locked
  every later OPEN on the ticket. The bound now counts only OPEN questions of the current
  acceptance revision. The reference codec no longer checks the open count, because it cannot
  know the current acceptance revision; `EscalationCapacity` takes it and enforces the bound for
  the writer, and reader validation refuses a reference over it as `OPEN_CAPACITY`.
  Stale questions stay visible and still count toward the 64 lifetime entries. An explicit
  retirement route was the alternative; it was not chosen because it adds a mutation with its own
  authority. This is an agent decision, listed with the Gate A capacity decision for owner
  acceptance.
- **Coded refusals.** Refusals are `*EscalationRefusal` with a `Code`. Ambiguous shorthand carries
  the sorted current open request IDs, so the CLI slice can name them. The message text keeps the
  `escalation: CODE` form.
- **Decode cost.** Each reducer call decoded every event blob several times and re-validated the
  whole post-state. A per-call `escalationView` memo now decodes each digest once per exported
  call. It never outlives the call, so a blob changed between calls is re-verified.

The operation-scoped answer grant (ESC-V0-004) stays assigned to the writer slice.

## Evidence

- `go test -count=1 -run Issue502 ./internal/tasks/ticket/ ./internal/tasks/transaction/` passes.
  Every reducer refusal test now asserts its exact code; ticket codec refusals are plain errors.
  New coverage: `SUPERSESSION_PAIR_MISMATCH`, `SUPERSESSION_SOURCE`,
  `TestIssue502_StaleOpenReleasesCapacity` and `TestIssue502_ReadersRefuseOpenOverflow`.
- Negative control: with the open count reverted to every OPEN entry,
  `TestIssue502_StaleOpenReleasesCapacity` fails with `CAPACITY_EXCEEDED` at the first question of
  the new acceptance revision. With the reader bound raised to 17,
  `TestIssue502_ReadersRefuseOpenOverflow` fails because a crafted 17-open reference is served.
- `BenchmarkIssue502_ApplyNearCapacity`, 200 iterations at 63 answered questions, on a 12-core
  host with load average about 7 to 8: 57 ms, 43 MB and 526k allocations per call before the memo;
  14 ms, 12 MB and 143k allocations after it. The transaction package's escalation tests fell
  from about 5.6 s to 1.8 s. A quiet-host figure is NOT_RUN.

Independent review approved with fixes. It found that readers no longer enforced the 16-open
bound once the codec dropped it, that `EscalationCapacity` trusted its acceptance argument, and
that the exact-code claim covered codec tests it did not reach; all three are fixed above. Two
nits stay: the event-ceiling test also trips the 64-entry bound, so its exact code cannot tell
which bound fired, and request-ID order relies on validation (now commented).

Native writes, holds, claim delivery, the CLI, dispatcher retry and native completion remain
NOT_RUN. V1-0699 stays open.

Rollback: revert this change. No byte format that any writer produces changes; the reference
codec only accepts more OPEN entries than before, and nothing writes references yet.
