# 2026-10-07: queue throughput visibility (V1-0966)

## Intent

Issue 661 (ticket V1-0966) records that a supervisor of a long-running dispatcher could not tell
whether the queue was still moving. Answering it took paging every ticket and reading receipts:

- `ticket list` had no status filter;
- list items carried no transition times;
- `queue status` had no last completion or completion rate;
- a dispatcher that kept relaunching sessions on a ticket that never changed status (a program that
  never submits candidates) gave no signal.

The change adds proposed CAL-V0-181 to CAL-V0-185 to
`docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner acceptance.

## Decisions

- **No record field.** Every new value is derived from records the store already holds, or from the
  dispatcher's own ledger. The ticket, attempt and receipt shapes do not change. A parallel batch
  touches the same packages, and a record field would need a format version. The cost is precision:
  `statusChangedAt` is exact only where the current record proves its last status transition, and is
  `UNKNOWN` everywhere else (fail closed).
- **`ticket list --status`.** The filter takes a strict comma-separated set of distinct §3.2
  statuses, given at most once. It is applied before paging, so `page.total` and offsets follow the
  filtered sequence. Bad forms refuse with the existing `MALFORMED` code (where `--status`) before
  the store is read, so no new result code is added.
- **`lastAttemptEndedAt`.** This is the `recordedAt` of the receipt that wrote the latest terminal
  attempt phase. The receipt is read by name and checked against the audited attempt afterimage
  digest and sequence. That is at most one bounded receipt read per listed ticket on the page, never
  a scan, and `ticket list` only.
- **`queue status` `lastCompletion` and `completions`.** Both come from the loaded inventory.
  - `lastCompletion` is the latest current-record completion. Its `receipt` is the audited
    latest-afterimage sequence when the completion was the record's last write, and `UNKNOWN`
    otherwise.
  - That audit covers one intent path. If it fails (on the 361-post fixture receipt it exceeds the
    inline post cap), only `receipt` degrades to `UNKNOWN`. The state reads keep their own failure
    behaviour.
  - The windows are `[observedAt-1h, observedAt]` and `[observedAt-24h, observedAt]`, at second
    resolution. They count current records only.
- **Dispatcher stall counts.** Counts are kept per ticket key in the new ledger member `stall`.
  - A count is seeded at launch, counted when a session finishes, restarted on any native status
    change and dropped at a terminal status.
  - Revision or work-state changes still count. That is the requested signal for programs that make
    edits but never move the ticket.
  - The optional `stalledAfterSessions` N (1..1000) emits one typed `stalled` event at N. The
    signal is advisory: no hold, park, cooldown or skip.
  - `dispatch status` shows counts of at least one, capped at 64 rows.
- **Ledger version.** The new member moves the dispatcher ledger to `taskman-dispatch-state/3`
  under CAL-V0-131. Drained version 1 and version 2 ledgers are adopted under their own closed
  member sets. A version 1 ledger still gets the adoption-time budget history.

## Limits and owner questions

1. An exact `statusChangedAt` for every record needs a write-time record field (a ticket format
   change). It was deferred.
2. Completion windows exclude completions that a later REOPEN removed from the record.
3. The stall count is keyed on native status only. A session whose post-session observation is
   pending (work state unknown) is not counted.
4. An earlier build refuses a version 3 dispatcher ledger. Rollback needs a drain, then removal of
   the `stall` member or the ledger.
5. `queue status` `lastCompletion.receipt` degrades to `UNKNOWN` when the audit cannot complete,
   rather than failing the command.

## Evidence

- Focused tests: `TestCALV0181_TicketListStatusFilter`,
  `TestCALV0182_ListTransitionTimesFromTheRecord`, `TestCALV0183_ListLastAttemptEndedAt`,
  `TestCALV0184_QueueStatusLastCompletionAndWindows`, `TestCALV0167_SummaryShapes` and
  `TestCALV0185_DispatchStatusShowsStallCounts` in `internal/tasks/cli`.
- Dispatcher tests in `internal/tasks/dispatch`: the four `TestCALV0185_*` tests, plus the
  version 3 schema pin in `TestCALV0131_LedgerSchemaChangeMovesTheStateVersion` and the drained and
  format tests.
- Full packages passed: `internal/tasks/cli` (248 s), `internal/tasks/dispatch` (128 s),
  `internal/tasks/service`, `internal/tasks/transaction`, `internal/tasks` and `internal/console`.
  `gofmt` and `go vet ./internal/tasks/...` are clean.
- Doc gates passed: `spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check` and
  `use-case-receipts-test`.

### Timing

The timing store is a journaled synthetic store with 880 tickets (12 COMPLETED) and 15,133
receipts. The figures are whole-process wall medians of 15 runs of the built binary, in two
interleaved rounds, base 0b45b052 against this change. Host load average was 12 to 15, from other
lanes. The spec amendment carries the table.

- The new reads take about the same time as the existing reads: 160 to 230 ms, dominated by the
  existing store open and audit.
- No read scans receipts.
- The incremental cost of the new fields is within run-to-run noise (about 40 ms under this load).
- `ticket list` grows from 51,596 to 60,970 bytes per 100-item page. `queue status` grows from
  1,290 to 1,480 bytes, and `--summary` from 744 to 934 bytes.

### NOT_RUN

- `make gate` (owner preference for scoped work).
- The other 38 units of the `corvint affected` first pass, which are unrelated analyzers,
  conformance suites and `cmd/corvint`. The plan scope was `UNKNOWN`, with `LANGUAGE_FRONTIER`
  unknowns.
- Live dispatcher qualification of the stall signal.
- A timing run on an idle host.
- Linux.

## Independent review

Codex (gpt-6-astra, read-only) round 1 on 75ebd31f reported no P0 or P1 and three P2:

1. Same-second archive or edit after completion named the wrong completion receipt. Fixed for the
   archive (the receipt now needs a COMPLETED record) with a regression in
   `TestCALV0184_QueueStatusLastCompletionAndWindows` that fails without the fix. A same-second
   non-acceptance edit stays a recorded limit, because no cheap discriminator exists: both writes
   are `MUTATION` receipts.
2. A tick during a running session absorbed the session's status change. Fixed: `pruneStall` skips
   keys with a running worker. Regression `TestCALV0185_TickDuringSessionKeepsStatusChange` fails
   without the fix.
3. `lastAttemptEndedAt` reads a receipt that a checkpointed audit may not re-walk. Recorded as a
   spec failure mode. Binding it would need a forward chain walk, which is the receipt scan the
   issue rules out.

Round 2 on 261c98c6 reported no P0 or P1 and one P2: the running-session skip discarded a status
change observed mid-session, so OPEN to HELD to OPEN before the finish counted against the old
streak and could emit a false `stalled`. Fixed: a tick that observes a different status for a
running ticket records `changed` (written only as `true`) in the ledger stall entry, and the finish
restarts the count when it is set. The marker is durable across a dispatcher restart. The
unreleased `/3` schema pin was recomputed (the version stays `/3`, which is new in this branch).
`TestCALV0185_TickDuringSessionKeepsStatusChange` now also runs the OPEN to HELD to OPEN sequence,
which fails without the fix, and the ledger format test covers the `changed` member forms.

## Dogfood use

At lane start, `corvint affected --base 0b45b052` and `corvint --root . context --task ...
--limit 25` were run.

- The context named this spec and the dispatcher ledger files.
- The affected plan selected `internal/tasks/cli` and `internal/tasks/dispatch` first.
- No CEM was bound or sealed in this lane (orchestrator instruction): NOT_PRODUCED.
