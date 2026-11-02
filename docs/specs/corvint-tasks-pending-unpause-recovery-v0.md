# Corvint Tasks pending UNPAUSE recovery V0

Owner: Russell Lewis
Date: 2026-10-09
Intent status: accepted (decision 0478)
Delivery status: experimental
Authoritative inputs: ticket V1-0309 (panel F2, audit addendum STO-01), the Corvint Tasks contract
TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md` at `800682d`: "Settled fixture barriers
(decision 0008)", §5.2 and `TM-V0-007`, `TM-V0-009`), decision 0397 (corvint-tasks built in tree),
`docs/specs/corvint-tasks-intent-worktree-v0.md`, the independent review of 2026-10-09, and the
in-tree sources under `internal/tasks/store` and `internal/tasks/journal`.

## Agent digest
- Claim: An UNPAUSE interrupted after its receipt is linked is settled by retrying a supported command, with the same ticket-divergence tolerance the fresh UNPAUSE had.
- Status: accepted (decision 0478); experimental. PUR-V0-001 through PUR-V0-005 are implemented and tested in tree; live qualification is NOT_RUN.
- Exists: barrier-command redo, receipt-authorized barrier deletion in redo, the pending UNPAUSE audit mode, and the narrowed redo branch guard.
- Blocked on: the matching edit to the external TCP-00 barrier clause in `beamfall/corvint-tasks` (open, not made).
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

TCP-00's settled-barrier clause says "Pending receipts refuse without redo" and leaves a
postcommit UNPAUSE failure pending and unsupported by every callable recovery path, including
ordinary mutation redo, which refused null posts. An operator whose UNPAUSE was interrupted after
the receipt link therefore had no supported way to settle it. `TM-V0-009` already orders redo of a
pending receipt before every mutating command's own transaction, after the barrier check.

This amendment replaces that sentence for the barrier command and for redo of an UNPAUSE receipt.
It adds no command, no error code, no wire field and no journal format. Every other clause of the
settled-barrier contract, including its exclusion of hostile editors, is unchanged.

## Requirements

- `PUR-V0-001`: `pause` and `unpause` run `TM-V0-009` redo of a pending receipt under their own
  writer guards, after the barrier check and before request lookup or modelling. A retried UNPAUSE
  under an ALL barrier therefore settles its own pending receipt, then replays it. An unrelated
  pending receipt is settled first, as every other writer settles it.
- `PUR-V0-002`: Redo of a pending UNPAUSE receipt completes its paired non-null pre/null post
  barrier deletion after the other posts and before the head, the first-write order. Present
  bytes are unlinked only when the session's identity and digest check under the lock finds the
  pre digest, and the parent is synced. An absent barrier is the post state and is synced as
  absent. Any third value, observed before the removal or by the removal's own check, refuses as
  `JOURNAL_FORKED`; the barrier bytes and the head are unchanged. The unlink is by name, so this
  protects against cooperating writers only.
- `PUR-V0-003`: Redo of a pending receipt whose kind is UNPAUSE and which posts no `intent/` path
  uses the fresh UNPAUSE audit with the pending receipt admitted: canonical tickets must have a
  regular physical projection, unrelated ticket bytes may differ (including empty or malformed),
  and every other projection must hold its pre or post value. The observation is rebound before
  effects by a second audit of the same mode that must report the same identity. Divergent
  ticket bytes are preserved. Every other pending receipt keeps the strict `PRE_OR_POST` audit.
- `PUR-V0-004`: Redo applies the `TM-V0-007` intent-branch guard only when the pending receipt
  posts an `intent/` path. A pending receipt that writes only state-directory files is settled
  from any branch, as the barrier command already is.
- `PUR-V0-005`: The amendment adds no top-level recovery verb and no reader error code. Both stay
  owner-pending.
- `PUR-V0-1920`: (experimental under the standing lane auto-accept, 2026-11-02; V1-4261) A Go test
  binary (argv[0] named `PKG.test` or `PKG.test.exe` and at least one `-test.*` argument supplied
  by `go test`) whose source path resolves to a Git checkout
  MUST refuse the writer lock, and so `pause`, `unpause` and every other writer, on that checkout's
  own common directory with `OUT_OF_SCOPE` before any lock file is opened. Before any test runs it
  MUST export that directory as `CORVINT_TASKS_FORBID_COMMON_DIR` unless the variable is already
  set. An ordinary `go build` binary MUST NOT be classified as a test or export the guard merely
  because its executable has a `.test` or `.test.exe` suffix. A helper or a built `corvint-tasks`
  child launched in the wrong working directory inherits
  the refusal. Reads take no lock and are unaffected. Failure mode before the change: a lane test or
  qualification child whose working directory was the live checkout paused the live store and the
  barrier stayed until an operator noticed. Non-goals: a self-expiring barrier (the frozen
  `barrier.json` and its administrative digest gain no field); a -trimpath test build or a child
  given an environment without the variable is not guarded by this clause.
- `PUR-V0-1921`: (experimental under the standing lane auto-accept, 2026-11-02; V1-4261) Any process
  in whose environment `CORVINT_TASKS_FORBID_COMMON_DIR` names a directory MUST refuse the writer
  lock on the store whose Git common directory is that directory (same path, or the same file by
  `os.SameFile`) with `OUT_OF_SCOPE`, leaving the store byte-identical, and the refusal MUST name
  the variable and the fix (run in a fixture repository). Unset or naming another directory, writers
  behave exactly as before. A shell test or qualification harness can export it to fence the live
  store. Rollback: remove `refuseForbiddenStore` from `acquireLock`; no stored format changes.

- `PUR-V0-2210`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4261) Policy optionally declares `barrierWarningSeconds`,
  a canonical decimal Count string 1..604800; absent means 3600 seconds. A live barrier strictly
  older than that limit produces an advisory `STALE_BARRIER` warning in `queue status` (including
  compact summaries) and the Claude/Codex session-start adapter. It names age in seconds, limit,
  since timestamp and the PAUSE request ID from the receipt at `sinceSeq` whose barrier post digest
  matches the observed bytes. An unavailable or mismatched receipt names the request `NOT_OBSERVED`;
  an unreadable hook snapshot or policy reports uncertainty. These bounded reads never lock, repair,
  expire or unpause. The non-null status barrier includes `warningPolicySeconds`. The hook consumes
  only the read-only companion wire (`queue status --fields barrier`, 64 KiB output cap and 500 ms
  deadline); unavailable, invalid, over-budget and older-companion observations stay `NOT_OBSERVED`.
  A checkout with no Tasks projection is outside this optional integration. Tests:
  `TestPURV02210_BarrierAgePolicy`, `TestPURV02210_QueueStatusWarnsStaleBarrier`,
  `TestPURV02210_SessionStartWarnsStaleBarrier`.
- `PUR-V0-2211`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4261) A qualification child inheriting
  the forbidden common directory and started in a linked lane refuses pause and unpause before
  touching the fixture primary store. The guard remains common-directory based, so changing cwd to a
  lane does not bypass it. Tests exercise disposable fixture primaries only
  (`TestPURV02211_LanePauseRefusesPrimary`). Shell qualification harnesses invoking writers must
  export `CORVINT_TASKS_FORBID_COMMON_DIR` to the source checkout's common directory before entering
  their fixtures. Non-goals: authorization against a caller stripping that environment, and
  retroactive certainty about the 2026-10-30 incident; the operator note retains record gaps.

## Non-goals and simpler baseline

The simpler baseline is the prior contract: a pending UNPAUSE stays unsupported. It was rejected
because the operator then has no supported path back to a usable store. A dedicated `recover`
verb and a `WRITER_ACTIVE` reader code are not part of this amendment.

## Failure modes

| Situation | Behavior |
| --- | --- |
| UNPAUSE interrupted before the barrier unlink or before the head | Retrying UNPAUSE, or any admitted writer, settles the identical receipt; a second retry changes nothing (`PUR-V0-001`, `PUR-V0-002`). |
| Same, with canonical tickets edited while paused | Retry settles the receipt and preserves the edits (`PUR-V0-003`). |
| Barrier replaced by a canonical third value before retry | `JOURNAL_FORKED`; barrier and head unchanged (`PUR-V0-002`). |
| Barrier replaced after redo's observation, before the removal | The removal's identity and digest check refuses; `JOURNAL_FORKED`; nothing deleted (`PUR-V0-002`). |
| Non-cooperating editor replaces the name after the last check | Not closed: the unlink is by name. The barrier contract excludes hostile editors (`PUR-V0-002`). |
| Intent tree moves between the two pending UNPAUSE audits | `SNAPSHOT_MOVED`; nothing written (`PUR-V0-003`). |
| A test binary, or its child, writes the checkout it was built from | `OUT_OF_SCOPE` before any lock file is opened; the live store is unchanged (`PUR-V0-1920`). |
| `CORVINT_TASKS_FORBID_COMMON_DIR` names the store's common directory | Every writer refuses `OUT_OF_SCOPE`; the store is byte-identical (`PUR-V0-1921`). |
| Primary off the intent branch | An UNPAUSE receipt settles; a receipt posting intent paths keeps `INTENT_BRANCH_MISMATCH` (`PUR-V0-004`). |

Fail-closed choices: any doubt about the barrier value, the pending observation or the receipt
digest refuses before the head advances.

## Acceptance evidence

Focused `internal/tasks/store` and `internal/tasks/journal` tests, once with `-race`. Process
kill, power loss and hostile-editor runs are `NOT_RUN`.

## Rollback

For the warning extension, remove `barrierWarningSeconds` from policy before reverting the reader and hook changes; old readers refuse that optional key. Stored barriers and receipts do not change.

Revert the V1-0309 commits. Stores written under this amendment use no new format; a store with a
pending UNPAUSE returns to the prior unsupported state.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `PUR-V0-001` | `TestV10309_PendingUnpauseRecovers`, `TestV10309_BarrierSettlesPendingMutation`, `TestTMV0009_AS11_BarrierPublicationReturnedFaults` |
| `PUR-V0-002` | `TestV10309_PendingUnpauseRecovers`, `TestV10309_PendingUnpauseChangedBarrierRefuses`, `TestV10309_PendingUnpauseBarrierChangedBeforeRemoval`, `TestV10309_PendingUnpauseAfterHeadReplays` |
| `PUR-V0-003` | `TestV10309_PendingUnpauseDivergentTicketsRecover` |
| `PUR-V0-004` | `TestV10309_PendingUnpauseRecoversOffIntentBranch` |
| `PUR-V0-005` | Design requirement; no runtime test. |
| `PUR-V0-1920` | `refuseForbiddenStore`, `testSourceCommonDir`, `isTestBinary` in `internal/tasks/authority/live_store_guard.go`, called by `acquireLock`; `TestPURV01920_TestBinaryGuardsItsOwnCheckout`, `TestPURV01920_IsTestBinary` (test flags required; production `.test` and `.test.exe` names alone do not enable the guard) |
| `PUR-V0-1921` | `TestPURV01921_ForbiddenStoreRefusesPause`: `pause` (OWNER and OPERATOR) and `unpause` refuse `OUT_OF_SCOPE` with state and intent untouched and no `barrier.json`; the same pause succeeds once the variable names another directory; it fails before the change (pause returned OK) |

## Unresolved decisions

- The external TCP-00 wording edit in `beamfall/corvint-tasks` (open, not made). Owner acceptance is
  recorded by decision 0478.
- A top-level `recover` verb and a `WRITER_ACTIVE` reader code are not added; decision 0478 accepts
  existing writers as the recovery path. Adding either needs its own owner decision.
