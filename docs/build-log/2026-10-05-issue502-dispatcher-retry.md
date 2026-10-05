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
  own ID is one of those holders; `held` when any typed hold is current, which outranks
  infrastructure; otherwise `no-progress`.
  The `finished` event carries the class. Only `no-progress` reaches the existing backoff, park and
  499 streak accounting.
- An infrastructure session charges one retry in a per-ticket episode in the existing dispatch
  ledger (`infraRetry`), keyed by acceptance revision and shared across roles. Each due retry is
  reserved under its exact launch identity and saved before the spawn; a failed save launches
  nothing and the next tick reuses the identity and charge. A reopen settles a reservation: its
  recorded worker makes it RUNNING, a missing log directory proves no spawn and returns it to
  WAITING, anything else is UNKNOWN and holds. A republish keeps the reserved role and slot; a
  roster that selects another role holds instead of taking a new identity. Cooldown doubles from `cooldownSeconds` and
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
`TestIssue502_HoldOutranksInfrastructure`, `TestIssue502_InfraReservationAcrossRestart`,
`TestIssue502_InfraReservationSavedBeforeSpawn` (reads the saved ledger at the launch fence),
`TestIssue502_InfraReservationSaveFailure`,
`TestIssue502_InfraRecoveredAndNewAcceptance` and `TestIssue502_LedgerRefusesMalformedInfraRetry`.
CLI package: `TestIssue502_ObserveNamesInfrastructureHolders` drives the real escalate writer and the
native observation, and `TestIssue502_DispatchStatusShowsInfraRetry` checks the status section.
The full `internal/tasks/dispatch` and `internal/tasks/cli` packages pass.

## Review

One independent read-only Codex review of `d524530f..320e9c4d` found no blocker, three major
findings and one minor; all were accepted and fixed in a follow-up commit:

- The strict reader let an episode omit or null a member, which decoded as zero debt or an elapsed
  deadline. It now requires every member but `launch`, refuses nulls, and checks that a waiting,
  reserved or running retry carries its charge and deadline.
- A republish after a proved no-spawn took a new identity when the roster picked another slot or
  role. It now launches the reserved identity in its own slot, waits while that slot is busy, and
  holds `INFRA_RETRY_UNKNOWN` if the roster selects another role.
- Infrastructure was classified before a typed hold, so a session raising both spent a retry and
  could leave an exhaustion hold after the owner answered. The hold now outranks it.
- The pre-spawn witness read the ledger only after the tick. A new test reads it at the launch
  fence and then interrupts there.

Each new test was checked to fail with its fix reverted.

## NOT_RUN

- A live dispatcher over a native store with a real worker raising the request, a process kill
  between reservation and spawn, and an answer-then-refine sequence on a live dispatcher.
- The distinct ESC-V0-010 stage, kinds and ages in the `dispatch status` escalation section, and
  the repository-wide gate, under the owner's focused-test preference.

## Rollback

Before any ledger carries `infraRetry`, revert this change. After that, remove the policy from the
config, stop the program's dispatcher, back up `state.json` and remove only the `infraRetry` member
before running an older binary; the backup keeps the debt for a later upgrade.
