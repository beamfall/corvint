# Escalation kinds and ages in dispatch status: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699) and ESC-V0-009. The owner asked for
this eighth slice, the dispatch status section, after the typed-event stage. Contract:
`docs/specs/corvint-tasks-escalations-v0.md`.

## Decision

Agent decision, 2026-10-05, under the owner's in-task delegation; the owner may redirect it.

- `dispatch status` keeps reading only the dispatcher's own ledger, as it did for the
  `ESCALATION_PENDING` hold. The native observation already validates each ticket's escalation
  material to bind infrastructure holders, so it now also records the kind and audited OPEN
  `recordedAt` of every current OPEN request, of any kind. The dispatcher saves them in two
  optional ledger members, `seen.requests` (ticket to sorted requests) and `seen.requestsUnknown`
  (sorted tickets whose material could not be validated). Status therefore opens no native store,
  hydrates no evidence and writes nothing.
- Age is computed at read time against the reader's clock by the helper `ticket escalation list`
  already used, so both surfaces agree: a nonnegative `ageSeconds`, and `clockUncertain` with age
  0 when the local clock is behind the OPEN time.
- The new `escalationRequests` section names each request's kind, OPEN time, age and `holds`,
  which is true exactly when the request is in the recorded `ESCALATION_PENDING` hold. It sits
  apart from `parked`, `infrastructureRetry` and the 499 `escalation` ladder, and is omitted when
  empty, so an existing status shape is unchanged. An ANSWERED infrastructure request still binds
  its holder for retry but is not an open request.
- The ledger validator admits only the shape diff writes (ticket keys, 1 to 16 strictly sorted
  requests of a known kind with a canonical time, sorted unknown tickets that name no request), and
  the strict reader closes the request object's members.

## Evidence

Focused tests: `TestIssue502_DispatchStatusShowsEscalationRequests` (including the clock-behind
witness), `TestIssue502_DispatchStatusRequestsFromLedgerWithoutWrite`,
`TestIssue502_DispatchLedgerRequestsValidated` and the extended
`TestIssue502_ObserveNamesInfrastructureHolders`. Disabling the validator makes all eight refused
ledger shapes load, so the validation test detects it.

## Limits

The section shows the dispatcher's last observation, not the live store, so a request opened or
answered since that tick is not yet reflected; `ticket escalation list` is the live read. A live
dispatcher against a writer-produced store, the list's 1 MiB page cut and a forked history chain
stay NOT_RUN. An older dispatcher binary refuses a ledger carrying the new members; the spec's
rollback names the one-member removal.
