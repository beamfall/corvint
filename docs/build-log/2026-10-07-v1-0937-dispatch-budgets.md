# 2026-10-07: dispatcher budgets, token accounting and declared effort (V1-0937)

## Intent

Issue 652 (ticket V1-0937) asks for three dispatcher features:

- per-role and per-ticket daily budgets, enforced at launch;
- a token total for each worker, in the ledger and in its `finished` event;
- the declared model and effort, shown in `dispatch status`.

The change adds CAL-V0-155..161 to `docs/specs/corvint-tasks-agent-leases-v0.md`. They are
proposed, pending owner acceptance.

## Decisions

- **Window.** Budgets apply over a rolling 24-hour window, not a calendar day. A hold carries
  `resetsAt`, the earliest time the window alone releases it. The CAL-V0-139 idle gate treats
  that time as a deadline, so an idle dispatcher wakes on time to release a hold.
- **Where the check runs.** The budget check sits in the pure roster, after the role, tier and
  global caps and before the pressure budget. It is a static fence like the tier cap.
  - An assignment the roster admits is charged against its scopes for the rest of that roster, so
    two launches in one tick cannot both use the last session.
  - A held assignment is not parked, backed off or charged.
  - When only a ticket scope is exhausted, other tickets of the same role still launch.
- **Recording holds and events.** Holds are recorded after the tick's launches (a deferred
  `recordBudget`), so the `budget` event counts them. One event is emitted per new
  (scope, name, limit), and a hold that persists across ticks is not reported again.
- **History.** The launch history is always recorded, even with no budget configured. A budget
  added by a CAL-V0-127 reload therefore counts launches that happened before it, and removing a
  budget releases its hold on the same tick.
  - A launch that may have started but whose identity was not proved is recorded with an UNKNOWN
    total.
  - The history holds at most 4,096 records. When it is truncated, every budgeted scope is held
    until the dropped records leave the window. The alternative, admitting against a history
    known to be incomplete, would break the budget silently.
- **Unknown is never zero.**
  - A session with an UNKNOWN total counts as a session and adds no tokens.
  - `tokensPerDay` requires a declared `usageFormat`: on a role budget, for that role; on
    `ticketBudget`, for every ticket role.
  - Status and the `finished` event write `UNKNOWN`, never `0`.
- **Usage parsing.** Usage is parsed incrementally from `stdout.log`.
  - Only the vocabulary a role declares is read, through the supervisor's existing strict readers
    (`ObservedUsage`, `ObservedClaudeUsage` and the new `OpenCodeLineUsage`, which shares
    `openCodeStepCounters` with the supervised host).
  - Only complete lines are read, at most 1 MiB per line and 8 MiB per read, on each supervising
    tick before the CAL-V0-143 cap and once more at finish.
  - Every cut of stdout leaves the total PARTIAL, because bytes appended between the size check
    and the truncation cannot be proven absent. The first line after a cut is skipped, so a torn
    line is never counted and the counters stay a lower bound.
  - A claude-code session with two `result` objects keeps the larger counters and is PARTIAL.
  - An opencode session is KNOWN only when its last step finished with `stop` and no step is open.
- **Effort.** Effort follows the `{model}` rule. A declared effort needs a host that renders
  `{effort}`, and a host that renders it serves only roles that declare one, so the effort shown
  is the one delivered. Nothing is inferred from host defaults.
- **Configuration and ledger versions.**
  - The configuration profile does not change. The new members are optional under
    `taskman-dispatch/0`, and an older build refuses them as unknown.
  - The ledger moves to `taskman-dispatch-state/2` with drained adoption of `/1`. The adopted
    history starts at `historyFrom`, the adoption time, and does not hold. Only the previous
    version is adopted, as before, so `/0` now refuses.
  - The pinned ledger schema hash for `/2` is
    `8d851eaa19d55f5dc4727701b1f1ce829fde819762e22224a013a647c12a263d`.

## Evidence

- The focused tests are named in the spec's traceability rows. The dispatch, cli and supervisor
  packages pass in full (`go test -count=1`): dispatch in 112 s, cli in 301 s, supervisor in 29 s.
- The tests use an injected `d.Now`. With a one-session role budget, the second ticket is held in
  the first tick, with `resetsAt` 24 hours after the launch and exactly one `budget` event. At 24 h
  plus 1 s it launches, and the old record is pruned.
- A codex-format worker's `finished` event reports KNOWN with 10/3/13 tokens and effort `high`.
  The rendered `{effort}` argument reached the host. After two such sessions, a 20-token budget
  holds with `observedTokens` 26.

## Limits

- A token budget is a launch gate on tokens already observed. Sessions admitted in the same tick,
  and running sessions, can overshoot it. `sessionsPerDay` bounds the overshoot.
- PARTIAL totals are lower bounds, so a token budget can admit more than complete counts would.
- Adoption sets `historyFrom` from the wall clock, not the dispatcher's injected clock. This only
  matters in tests.
- If a test lowered `maxUsageRead` below `maxUsageLine`, the scan would not advance past a long
  line without a newline until the final read, which would mark the total lost. Production uses
  8 MiB against 1 MiB.
- Live dispatcher qualification with real Codex, Claude Code and OpenCode hosts is NOT_RUN.

## Independent review

Codex (`gpt-6-astra`, read-only) reviewed the diff. Round 1 found four defects, each verified
against the code and fixed with a focused test:

- A codex `turn.completed` with null counters read as KNOWN 0, because `ObservedUsage` decodes
  null into `uint64` as zero. The dispatch reader now treats a null counter as malformed. The
  supervised Codex host's own use of `ObservedUsage` is unchanged and is outside this change.
- Input plus output, and a scope's sum across sessions, could wrap. A session whose total would
  overflow is now malformed (PARTIAL), the ledger refuses an overflowing account, and the gate sums
  tokens in 128 bits, so releasing the oldest sessions never wraps. Status saturates the display.
- A codex turn started and never completed left the total KNOWN. `turn.started` now opens a turn
  and an open turn leaves the total PARTIAL.
- An explicit `0` or `null` limit read as absent and silently disabled that limit. A present
  member must now be positive; the budget object stays closed.
