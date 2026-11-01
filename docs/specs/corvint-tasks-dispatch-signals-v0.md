# Corvint Tasks dispatch signals V0

Owner: Russell Lewis
Date: 2026-10-25
Intent status: proposed
Delivery status: experimental

Authoritative inputs: GitHub beamfall/corvint#1820 (native V1-2724), #1821 (V1-2725) and #1815
(V1-2710). These requirements amend the [agent lease contract](corvint-tasks-agent-leases-v0.md) (dispatcher CAL-V0 requirements),
whose Requirements section is at the 256-obligation OCM limit, so they live here. They add
observation fields, one alert, one host mode and one read-only verb; they change no admission
order, lease, ledger format or receipt; DSG-V0-010 to 015 add an opt-in in-place handover with its
own sidecar records beside the unchanged ledger.

## Agent digest
- Claim: Dispatch flags unmatched work states and idle capacity, delivers OpenCode notices by plugin, and checks and applies an in-place upgrade.
- Status: proposed; experimental; DSG-V0-001 to 015 are implemented with focused tests (DSG-V0-012 to 015 on Linux and macOS only); live OpenCode qualification of DSG-V0-008 and the post-handoff tool-call count are NOT_OBSERVED.
- Exists: NO_ROLE_MATCH plan reason, workState holdStates, unmatchedStates and the idle streak in admission.json, the IDLE_WITH_CAPACITY alert, doctor and status fields, contextNotices "plugin" with the OpenCode plugin module, `dispatch upgrade --check` and `--apply`, per-worker build records and the `upgrade.json` handover record.
- Blocked on: a live OpenCode dispatcher worker run for DSG-V0-008 acceptance.
- Read next: Requirements; Acceptance evidence.

## User and current state

Human-owned input: GitHub #1820, #1821 and #1815. Before this change a ticket whose work state no
enabled role admitted (for example a suffixed `X~2`) was deferred `WORK_STATE_HELD`, the same reason
a deliberate hold gets, so an operator could not tell a misconfigured state from a parked lane, and a
dispatcher with free capacity that launched nothing for hours raised no alert. An OpenCode worker had
only the polled context file (`contextNotices "file"`) or no notice at all, and so ran past
`handoffTokens` with no in-session notice. Upgrading the dispatcher binary had no up-front check of
whether the new build would adopt the live workers or refuse the ledger.

## Requirements

- `DSG-V0-001`: (proposed; V1-2724) A ticket deferred for its work state MUST read
  `WORK_STATE_HELD` only when an enabled role names that state in `excludeStates` or the state is
  listed in `workState.holdStates`; any other known state that no enabled role admits MUST read
  `NO_ROLE_MATCH`. Both leave the window exactly as before. `holdStates` MUST hold at most 64
  distinct non-empty states of at most 128 bytes and MUST NOT name the none or unknown state.
- `DSG-V0-002`: (proposed; V1-2724) The admission record MUST list `unmatchedStates`, one entry per
  state of a ticket still deferred `NO_ROLE_MATCH` with its ticket count and one example ticket, ordered by state.
- `DSG-V0-003`: (proposed; V1-2724) Each tick that launches nothing while the global cap has free
  capacity and at least one open planned ticket is deferred MUST extend an idle streak recorded as
  `idleTicks`, `idleSince`, `idleDeferred` (count per deferral reason) and `idleAlertSeconds` (the
  DSG-V0-004 threshold in force, 0 when off). A launch (a worker the previous tick's ledger did not
  hold), a full global cap or no deferred planned work MUST reset the streak, and so does a tick whose
  launches the work-state reader holds (CAL-V0-273), which already raises its own events.
- `DSG-V0-004`: (proposed; V1-2724) When an idle streak lasts `idleAlertMinutes` (default 60, 0 off,
  range 0 to 1440, never null), the dispatcher MUST append one `alert` event with code
  `IDLE_WITH_CAPACITY`, the running count and the deferral reasons, and then at most once per further
  threshold of the same streak. The idle gate's skip window MUST end at the next threshold.
- `DSG-V0-005`: (proposed; V1-2724) `doctor` MUST report a finding for unmatched states and for an
  idle streak at or past the record's `idleAlertSeconds` (none when 0), naming the configuration change that resolves each.
- `DSG-V0-006`: (proposed; V1-2724) `dispatch status` MUST show the unmatched states and the idle
  streak from the admission record.
- `DSG-V0-007`: (proposed; V1-2710) `dispatch upgrade --program P --config C --check NEW_BUILD` MUST
  run NEW_BUILD's read-only `dispatch status` (60 s timeout with a 5 s pipe wait delay, 4 MiB output cap) and report
  `ROLLING`/`STATE_COMPATIBLE` when it reads the configuration and ledger, `REFUSED`/`CONFIG_INVALID`
  when it refuses the configuration, `DRAIN_REQUIRED`/`STATE_FORMAT` when it refuses the ledger
  version while workers are live, `REFUSED`/`STATE_FORMAT` when it refuses the ledger with no live
  worker, and `UNKNOWN`/`PROBE_FAILED` otherwise, with NEW_BUILD's SHA-256 and the live worker count.
  It MUST NOT write state, signal a process or start a dispatcher. NEW_BUILD MUST be an executable
  regular file.
- `DSG-V0-008`: (proposed; V1-2725) Amending ATR-V0-125, a host MAY also declare `contextNotices "plugin"`; every role on it
  MUST use `usageFormat "opencode"`, and its delivery mode MUST read `HOOK`. The OpenCode plugin MUST,
  only when `CORVINT_DISPATCH_CONTEXT_FILE` is set, run `corvint-tasks dispatch context-hook` (5 s
  timeout, 64 KiB output cap) when that record's modification time or size changes, keep at most four
  notices of at most 2000 bytes, and add each once to the system context of every later model
  request. A hook failure MUST leave the request unchanged.
- `DSG-V0-009`: (proposed; V1-2725) The `finished` event of a worker whose context record holds a
  HANDOFF crossing MUST record `tokensAfterHandoff` (last estimate minus the crossing's tokens) and
  `secondsAfterHandoff`.
- `DSG-V0-010`: (proposed; V1-2710) Each worker launch MUST record the launching build (path and
  SHA-256 of the dispatcher executable) in `workers/<id>/build.json` (profile
  `taskman-dispatch-worker-build/0`; a record that cannot be written reads `UNKNOWN` and never blocks
  the launch), and the worker MUST receive `CORVINT_TASKS_BIN` naming that
  executable unless the host environment sets it, so the session's own `corvint-tasks` calls run the
  build that launched it. `dispatch status` MUST show each worker's `build` (`UNKNOWN` when
  unrecorded) and an `upgrade` object with the handover protocol, the record's phase and the pending
  request, if any.
- `DSG-V0-011`: (proposed; V1-2710) `dispatch upgrade --apply NEW_BUILD` (exclusive with `--check`)
  MUST run the DSG-V0-007 check and, only on `ROLLING`, write one exclusive request
  (`upgrade-request.json`, profile `taskman-dispatch-upgrade-request/0`, NEW_BUILD path and SHA-256)
  and report `REQUESTED`/`HANDOVER_REQUESTED`. It MUST otherwise write nothing and report
  `REFUSED` with `NOT_RUNNING` (no live dispatcher), `NO_HANDOVER` (the running dispatcher or
  NEW_BUILD does not advertise `taskman-dispatch-handover/0`), `UPGRADE_IN_PROGRESS`,
  `MIXED_BUILD_LIMIT` (more than 3 distinct builds would be live: the running one, the builds of live
  workers, and NEW_BUILD) or `REQUEST_PENDING`, or `UNKNOWN` with `PROBE_FAILED` or
  `REQUEST_UNWRITTEN`. It MUST NOT signal a process or start a dispatcher.
- `DSG-V0-012`: (proposed; V1-2710) A long-running dispatcher MUST consume a pending request between
  ticks: it MUST remove the request first (a request it cannot remove disables further upgrades for
  the run, `UPGRADE_REQUEST_STUCK`), refuse with an alert and keep running on its own build when the
  request is unreadable, NEW_BUILD's digest changed, the configuration file no longer decodes or moves
  `stateDir` or `workRoot` (`UPGRADE_CONFIG_CHANGED`), the reader or lock identity is unknown, or the
  ledger or `upgrade.json` cannot be saved, then record phase `EXEC`, emit `upgrading`, and replace
  its own image with NEW_BUILD (same pid, same argv) through exec. A failed exec MUST record `FAILED`,
  raise `UPGRADE_FAILED` and continue on the old build with every worker untouched. There is no global
  drain: no worker is signalled, and no lease, attempt or lock is released.
- `DSG-V0-013`: (proposed; V1-2710) The handed-over dispatcher lock MUST pass as an inherited file
  descriptor named by `CORVINT_DISPATCH_HANDOVER`; the new image MUST verify it is the program's lock
  file, re-take the lock on it and close it on any later exec, or take the lock afresh with
  `UPGRADE_LOCK_UNVERIFIED`. Workers stay children of the same pid, so the new image MUST wait for
  each adopted worker whose leader identity still matches and observe its real exit code, and exit
  codes the old image reaped but had not yet recorded MUST be carried through `upgrade.json`.
- `DSG-V0-014`: (proposed; V1-2710) The new image MUST emit `upgraded` with `from`, `fromSha256`,
  `to`, `toSha256`, `oldBuildWorkers` and, when any remain, `windowEndsBy` (the latest
  start-plus-`wallSeconds` of an old-build worker, which bounds the mixed-build window), keep
  admitting new work on the new build, let old-build sessions finish on their own build, and emit
  `upgrade-settled` once no recorded worker runs on another build.
- `DSG-V0-015`: (proposed; V1-2710) A start that finds phase `EXEC` with no inherited lock (the old
  process died between recording `EXEC` and the new image starting) MUST raise `UPGRADE_INTERRUPTED`,
  record `INTERRUPTED`, discard any stale request (a handed-over image keeps one, which was made for it) (`UPGRADE_REQUEST_DISCARDED`), and adopt the
  recorded workers through the existing CAL-V0-056 path with their leases and attempts unchanged.
  An unreadable record MUST raise `UPGRADE_STATE_UNREADABLE` and start normally.
- `DSG-V0-684`: (standing lane auto-accept, 2026-11-01; V1-1556) Amending CAL-V0-311, a
  configuration MUST be refused when a `keepLease` role's `leaseMinutes` × 60 does not exceed two
  keeper writes at the program's `tickSeconds`: the keeper acts at most once per tick, so its
  writes are `ceil(intervalSeconds / tickSeconds) × tickSeconds` seconds apart. The refusal names
  the role, `tickSeconds` and that spacing. Before this, a tick longer than the lease was accepted
  silently and the kept lease expired between ticks. Failure mode: a configuration that loaded
  before (for example `tickSeconds` 600 with a 5-minute kept lease) now refuses; raise
  `leaseMinutes` or shorten `tickSeconds`. Rollback: drop the check in `Config.validate`.
- `DSG-V0-940`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1389, moved from the
  leases contract at its OCM limit) An
  operator recovery MUST become an observable `reader-recovered` event, kind `OPERATOR`, in the
  program's event log. `recover-reader` MUST NOT write the event log or the ledger, because only
  the dispatcher advances the event sequence and the ledger may be one the recovery does not own.
  Before it removes the marker, it MUST hard-link the recovery record under the same name into
  `reader-recoveries/unannounced/`, fsynced with that directory. An existing link is kept, so a
  replay that finishes an interrupted removal links nothing twice. The next `dispatch` start for
  the program MUST announce each linked record, under the program lock, after the quarantine
  check passes and before any launch. It emits one `reader-recovered` event per record, in name
  order and at most 64 per start, with detail `kind`, `run`, `requestId` and `recordedAt` and a
  message naming the reason and evidence. It MUST remove a link only after that event's line is
  appended. A linked record that does not decode strictly stays linked and is reported by an
  `alert` event naming it. Delivery is at least once: a crash after the append and before the
  link removal repeats the event at the next start. A record an earlier build wrote has no link
  and is never announced.

## Non-goals

No change to admission order, caps, leases or the ledger format; no automatic drain, no automatic
upgrade without an operator `--apply`, no handover on platforms where the dispatcher itself does not run (Windows),
and no migration of a live session to the new build (it finishes on the build that launched it); no
OpenCode tool-call count after handoff; no notice delivery for other
hosts beyond the existing hook and file modes.

Non-goals of DSG-V0-940: an event at the moment of the attestation (no dispatcher runs then, and the command's
own result already reports the record); announcing a HOST_REBOOT recovery that a direct
work-state read applies outside a dispatcher start; a native Tasks journal entry (CAL-V0-226
rationale); any change to what lifts the quarantine.

## Failure modes

A misspelled `holdStates` entry reads as `NO_ROLE_MATCH` and is surfaced by DSG-V0-005 rather than
hidden. A dispatcher restarted mid-streak restarts the idle streak from the next tick, so an alert
can come up to one threshold late. An upgrade probe that hangs is killed at 60 s and reads
`UNKNOWN`; a NEW_BUILD that prints a forged result can only mislead the advisory verdict, since the
check changes nothing. A plugin whose hook binary is missing delivers no notice and leaves the
request unchanged; the `context-hook` marks each level exclusively, so a notice is delivered to one
consumer only. A crash after `EXEC` is recorded and before the new image starts kills the
dispatcher but no worker; the next start reports `UPGRADE_INTERRUPTED` and adopts them. A signal after
the old image resets its handlers and before exec ends the process the same way. A worker reaped in the
exec instant, before its exit is carried, can read `NOT_OBSERVED` for its exit code, never a wrong
code. A request for a build replaced after the check is refused by digest.

For DSG-V0-940, a program that is never started again keeps its links, one directory entry per
recovery beside records Corvint never removes. An older binary ignores `unannounced/`, so it
emits no event and changes nothing else. A link that cannot be removed repeats its event at each
start until it is removed. More than 64 links are announced across successive starts.

## Acceptance evidence

- DSG-V0-001 and 002: `TestDSGV0001_UnmatchedStateIsNoRoleMatch`, `TestDSGV0001_HoldStatesValidation`
  (internal/tasks/dispatch/signals_test.go).
- DSG-V0-003 and 004: `TestDSGV0003_IdleWithCapacity`, `TestDSGV0004_IdleGateWakesForTheAlert`.
- DSG-V0-005 evidence: `TestDSGV0005_DoctorNoRoleMatchAndIdle` (internal/tasks/cli/doctor_admission_test.go).
- DSG-V0-006 evidence: `TestDSGV0006_DispatchStatusSignals` (internal/tasks/cli/dispatch_signals_internal_test.go).
- DSG-V0-007 evidence: `TestDSGV0007_UpgradeCheck` (internal/tasks/cli/dispatch_upgrade_internal_test.go).
- DSG-V0-008 and 009: `TestDSGV0008_OpencodePluginDelivery` and
  integrations/opencode/dispatch-notice.test.mjs; end to end (GitHub #1821, a session that crosses
  `handoffTokens` receives the notice in-session through the shipped plugin module and the compiled
  hook): `TestDSGV0008_OpenCodeSessionReceivesHandoffInSession`
  (internal/tasks/cli/dispatch_opencode_notice_internal_test.go); live OpenCode qualification NOT_RUN.
- DSG-V0-684: `TestDSGV0684_KeepLeaseMustOutlastTwoTicks` (internal/tasks/dispatch/keeper_test.go); a live
  long-running kept attempt (V1-1556 AC1) NOT_RUN.
- DSG-V0-010 to 015: `TestDSGV0011_UpgradeApplyRequestsOrRefuses`
  (internal/tasks/cli/dispatch_upgrade_internal_test.go); `TestDSGV0012_HandoverFailureAndInterruption`
  (internal/tasks/dispatch/upgrade_test.go: build record, failed exec, changed digest, interrupted
  EXEC with stale request); `TestDSGV0012_RollingUpgradeWithRealExec`
  (internal/tasks/cli/dispatch_rolling_upgrade_test.go: three real builds, two upgrades in a row with
  the same pid, an old-build session finishing with its real exit code after the new dispatcher owns
  the queue, `upgrade-settled`, and a crash mid-upgrade whose restart adopts the live worker with its
  lease unchanged).
- For DSG-V0-940, `TestDSGV0940_OperatorRecoveryAnnouncedAtNextStart` and
`TestDSGV0940_UnreadableLinkedRecordAlertsAndStays` (`internal/tasks/dispatch`), and the
extended `TestCALV0226_DispatchRecoverReaderOperator` and `TestCALV0228_DispatchHelpNamesRecovery`
(`internal/tasks/cli`).; live dispatcher qualification and a real reboot NOT_RUN.

## Rollback

Revert the change. `holdStates` and `idleAlertMinutes` are optional configuration keys and
`contextNotices "plugin"` is an opt-in host value; a configuration that uses them is refused by the
prior build, so remove them first. The admission record's new fields are rewritten each tick.
To leave a new build, `dispatch upgrade --apply PREVIOUS_BUILD` hands back in place, or stop the
dispatcher and start the previous build, which adopts the live workers. Builds without DSG-V0-010 to
015 ignore `workers/<id>/build.json`, `upgrade.json` and `upgrade-request.json`; the ledger format is
unchanged, so delete those files only after the dispatcher stops.

For DSG-V0-940, revert the link in `RecoverReader`, the announcement in `Open` and this amendment.
`reader-recoveries/unannounced/` can then be deleted; the records it links stay in place.
