# Dispatcher infrastructure retry and session classification: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699), ESC-V0-007 and ESC-V0-008, issue
499's dispatch model-escalation tiers, and the Gate A decisions the owner accepted as written on
2026-10-04 (decision 0428). The owner asked for this sixth slice, dispatcher retry with 499 tier
composition, after claim delivery. Contract: `docs/specs/corvint-tasks-escalations-v0.md`.

## Decision

- The native observation names, per ticket, the holders of current OPEN or ANSWERED infrastructure
  requests at its acceptance revision, read from each request's origin event. It also substitutes
  `workRevision` for the ticket revision while the latest write is a typed control write, so an
  escalate or answer is never checked progress. Unreadable material marks the ticket UNKNOWN.
- When a worker ends without checked progress, the dispatcher first classifies it. A session is
  `infrastructure` only when the policy is configured, the material is readable and the worker's
  own ID is one of those holders; `held` when any typed hold is current; otherwise `no-progress`.
  The `finished` event carries the class. Only `no-progress` reaches the existing backoff, park and
  499 streak accounting.
- An infrastructure session charges one retry in a per-ticket episode in the existing dispatch
  ledger (`infraRetry`), keyed by acceptance revision and shared across roles. Each due retry is
  reserved under its exact launch identity and saved before the spawn; a failed save launches
  nothing and the next tick reuses the identity and charge. A reopen settles a reservation: its
  recorded worker makes it RUNNING, a missing log directory proves no spawn and returns it to
  WAITING, anything else is UNKNOWN and holds. Cooldown doubles from `cooldownSeconds` and
  saturates at `maxCooldownSeconds`.
- Exhaustion, `maxRetries` 0, a native `RETRY_EXHAUSTED` plan and an ambiguous reservation are named
  dispatcher holds with `needs-owner` events. They are status labels, not wire codes, so TCP-00 is
  unchanged. `dispatch unpark` releases a hold and keeps the debt.
- For 499 tiers an infrastructure session lowers the streak to the largest selected tier threshold
  across roles: unknown progress drops only the proved failure suffix and keeps the tier.
- Without the policy, accounting is the legacy one and recorded episodes stay inert, so toggling
  the policy cannot refill debt.

These were agent design choices under the owner's request within ESC-V0-007 and ESC-V0-008 as
written; none changes an accepted decision. The spec lists them under the slice's findings.

## Evidence

Dispatch package: `TestIssue502_InfraRetryPolicyBoundsAndCooldown`,
`TestIssue502_InfraSessionChargesWithoutParking`, `TestIssue502_InfraRetryDisabledAndNativeExhausted`,
`TestIssue502_InfraClassificationNeedsTheHoldersOwnRequest`, `TestIssue502_HeldSessionIsNotFailureParked`,
`TestIssue502_InfraSessionKeepsTierAndResetsSuffix` (the mixed 499 witness),
`TestIssue502_InfraReservationAcrossRestart`, `TestIssue502_InfraReservationSaveFailure`,
`TestIssue502_InfraRecoveredAndNewAcceptance` and `TestIssue502_LedgerRefusesMalformedInfraRetry`.
CLI package: `TestIssue502_ObserveNamesInfrastructureHolders` drives the real escalate writer and the
native observation, and `TestIssue502_DispatchStatusShowsInfraRetry` checks the status section.
The full `internal/tasks/dispatch` and `internal/tasks/cli` packages pass.

## NOT_RUN

- A live dispatcher over a native store with a real worker raising the request, a kill between
  reservation and spawn, and an answer-then-refine sequence on a live dispatcher.
- The distinct ESC-V0-010 stage, kinds and ages in the `dispatch status` escalation section, and
  the repository-wide gate, under the owner's focused-test preference.

## Rollback

Before any ledger carries `infraRetry`, revert this change. After that, remove the policy from the
config, stop the program's dispatcher, back up `state.json` and remove only the `infraRetry` member
before running an older binary; the backup keeps the debt for a later upgrade.
