# V1-1608, V1-1673, V1-1647: adopter-declared progress signal and ticket budget overrides

Tickets V1-1608 (GitHub issue 1050), V1-1673 (issue 1068) and V1-1647 (issue 1066). Requirements
ATR-V0-080..089 in [corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md)
and TQD-V0-040..042 in [corvint-tasks-doctor-v0](../specs/corvint-tasks-doctor-v0.md), accepted
under the owner's standing auto-accept of lane requirements (2026-10-11). This is not a new owner
decision; it applies that standing acceptance to this lane's requirements. Nothing is added to the
agent leases spec (CAL-V0); the requirements live in the Tasks domain specs that govern the
dispatcher, the loop evidence and the doctor.

## Decisions

- **One signal for both issues.** Issue 1050 asks for an adopter-declared progress signal honoured by
  loop detection, the dispatcher's no-progress backoff and the doctor; issue 1068 asks how a
  non-submitting workflow records progress that `noProgressRun` counts. One policy key,
  `progressSignal`, answers both, and every no-progress judgment reads the same fingerprint.
- **Option 2 of issue 1068.** A changed `--evidence` reference on the clean HANDOFF release is
  progress. The reference is already recorded on the attempt and bounded, the agent writes it when
  it hands off, and no new verb, receipt kind or journal format is needed; nothing runs a shell. A
  dedicated `attempt progress` verb (option 1) would add a native write path for the same fact, and
  a policy-chosen signal set (option 3) would let a policy remove the artefact signals. The enum has
  one member today and room for more.
- **Loop evidence carries the signal.** A claim records `progress{signal, fingerprint}` per prior
  generation only when the signal is declared, so an undeclared store's attempt records are
  byte-identical. A record without the member, or recorded under another signal, is UNKNOWN and never
  counts: declaring the signal cannot extend a streak recorded before it.
- **Dispatcher.** The CAL-V0-057 fingerprint adds each observed attempt's hand-off reference only
  under the signal. The ledger profile is strict, so the signal is named in the LOOP_DETECTED,
  cooldown and parked event details and messages, not in the ledger's loop hold.
- **SETUP_ONLY_PROOF unchanged.** Its judgment is witnessed obligations; a progress fingerprint is
  not a witness (TQD-V0-042).
- **Budget overrides.** `ticketBudget.overrides[]` takes exactly one selector and at least one limit;
  precedence is ticket, then label (first in configuration order), then kind, then the default, and
  unset limits inherit the default's. The two refusal reasons are stable tokens in the CAL-V0-127
  refusal message. `dispatch status` reads the store's tickets only when overrides exist, so the
  undeclared path does no new read.

## Evidence

Focused tests on Darwin, one package per command: `internal/tasks/intent`, `snapshot`,
`transaction`, `dispatch` and `cli` (the ATRV008 and TQDV004 sets with the CAL-V0-102, CAL-V0-057,
CAL-V0-155..159 and TQD-V0-005..008 tests), plus TestCALV0131, TestImportDirection and
TestIndexCoversSpecsAndHeaders.

## Residuals

- Live dispatcher qualification of both features and Linux runs are NOT_RUN.
- The dispatcher ledger's loop hold does not record the signal; only events name it.
- A hold for a ticket the observation no longer supplies resolves its budget by key alone.
- An override cannot remove a default limit (non-goal).
