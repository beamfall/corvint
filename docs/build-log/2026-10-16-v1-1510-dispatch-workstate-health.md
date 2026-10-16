# 2026-10-16 V1-1510: dispatcher work-state reader health (issue 984)

This entry covers native ticket V1-1510, from owner request
[issue 984](https://github.com/beamfall/corvint/issues/984). A failing work-state command was
visible only as one `alert` event per tick. `dispatch status` had no field for it, nothing
escalated, and a run that ended during the failure was charged a no-progress cooldown that nothing
attributed to the failure.

Base: `origin/main` `ff96980fd`. Spec: `docs/specs/corvint-tasks-agent-leases-v0.md`, section
"V1-1510 work-state reader health (issue 984)".

## Contracts

The first requirements are CAL-V0-270..273. They are accepted under the owner's standing auto-accept
of lane requirements (2026-10-11). IDs 263..265 are taken by open PR #985, so this lane uses
270..273. No other open PR or `origin/main` uses them. The added scope uses 274..276 and 280, because open PR #990 (V1-1511) takes 277 and 278.

- CAL-V0-270 adds the health record `workState {ok, since, lastError, failedTicks, escalated}`.
  - It lives in the dispatcher ledger, because `dispatch status` reads only the ledger.
  - Status reports it as a `workState` object: `health` is `OK`, `FAILING` or `NOT_OBSERVED`,
    alongside the thresholds and `launchesHeld`.
  - The record and the worker mark below are new ledger members, so the ledger moves to
    `taskman-dispatch-state/5` (CAL-V0-131). A `/4` ledger is adopted, with or without workers, as
    TOL-V0-021 adopted `/3`, and `/4` joins `adoptOnlyProfiles`.
- CAL-V0-271 sets the escalation point with `workState.escalateAfterTicks` (1..1440, default 3).
  - Each failure episode emits one `needs-owner` event, and the first successful read emits one
    `alert` with `health: RECOVERED`.
  - The `escalated` mark is in the ledger, so a restart during an episode does not escalate again.
  - Both reuse existing event kinds. The CAL-V0-058 vocabulary is closed, so no new kind was added.
- CAL-V0-272 adds a `workStateUnknown` mark on the worker, for a run that saw an UNKNOWN work state
  at launch or during the run. A charged run without progress that carries the mark, or that ends
  with the state UNKNOWN, has `workState: UNKNOWN` on its `finished`, `cooldown` and `parked`
  events.
- CAL-V0-273 adds the opt-in `workState.holdLaunchesAfterTicks`, which is off by default. While the
  hold applies, the dispatcher launches nothing, and it emits one `throttled` event when the hold
  begins.

### Added scope from an issue 984 comment

A later comment on issue 984 asked for three more changes in the same reader:
- A timed-out read logged `signal: killed`.
- One failed read flipped every ticket to `UNKNOWN` and emitted one `state` event per ticket, and
  the recovery emitted as many again.
- Completed and archived tickets churned `state` events the same way.

- CAL-V0-274 is accepted under the standing auto-accept, because it only adds to the CAL-V0-053
  alerts. It has two parts:
  - A read past its limit is reported as `timed out after Ns`. A dispatcher cancellation is not
    renamed.
  - The first alert of an outage carries `health: UNAVAILABLE` and `since`, which makes it the
    outage's one provider-level event. The existing CAL-V0-271 `RECOVERED` event closes the outage.
    The per-read CAL-V0-053 alerts stay.
- CAL-V0-275 is **proposed**. It adds `workState.timeoutSeconds` (1..600, default 60), and
  qualifies CAL-V0-053's fixed "60 s timeout".
- CAL-V0-276 is **proposed**. It adds `workState.staleAfterSeconds` (1..86400, default one tick).
  - Within that window of the outage's `since`, only the state-event baseline keeps each ticket's
    last known work state, so one failed read emits zero per-ticket `state` events.
  - The observation and every decision still see `UNKNOWN`. CAL-V0-053 says "never a guessed
    state", so the stale value is kept out of every decision.
  - Status reports `stale`, `timeoutSeconds` and `staleAfterSeconds`.
  - It qualifies CAL-V0-058, which says state changes compare against the previous observation.
- CAL-V0-280 is **proposed**. A `COMPLETED` or `ARCHIVED` ticket emits no `state` event for a
  work-state-only change, and it also qualifies CAL-V0-058.
  - The comment asked that terminal tickets get no work state at all. The read is kept, because
    role `match.statuses` is not restricted, so a role may match a terminal status. Only the event
    is suppressed.

## Decisions

- **The cooldown exemption is not implemented; it needs an owner decision.** The issue asks to skip
  cooldown and park accounting for such runs, or to mark them UNKNOWN.
  - Accepted CAL-V0-057 says "No progress MUST start a cooldown, and after `parkAfter` consecutive
    runs MUST park the key". `TestCALV0056_IdentityOutageAndUnknownState` pins that an UNKNOWN
    state is charged as no-progress.
  - The exemption amends an accepted requirement, which the lane charter excludes from the standing
    auto-accept. A first draft of the exemption was built, then replaced by the marking alternative,
    which the issue also offers.
  - Owner decision needed: amend CAL-V0-057, so that a no-progress run carrying the CAL-V0-272 mark
    charges no cooldown, park or ladder streak, and update the CAL-V0-056 test. The mark already
    identifies those runs, so the code change is a small guarded `continue` in `finish`.
- **`doctor` is not implemented; it needs an owner decision.** Accepted TQD-V0-001
  (`docs/specs/corvint-tasks-doctor-v0.md`) fixes `doctor`'s arguments, and `doctor` has no
  dispatcher configuration or program. Proposed amendment: optional `--dispatch-config FILE
  --program ID` arguments, with a `WORK_STATE_FAILING` finding read from the ledger health record.
- **Other external commands are already covered.**
  - The CAL-V0-068 pressure sampler reports failed inputs in status `pressure.problems`.
  - Host launches emit `launch-failed`.
  - An unreadable process table never ends a worker (CAL-V0-056).
  - None of these needed a new record.
- **What counts as a failure.** A tick counts as failed when any `status-line` read or the command
  alerts. Each tick counts at most once, even when a tick reads more than once.
- **Escalation is unaffected by the UNKNOWN mark.** The mark covers only the program work state,
  not `EscalationUnknown`, so the existing ESC rule ("accounted as ordinary no-progress") is
  unchanged.

## Evidence

The host had 12 cores, and the load averages shown below were taken during the runs.

- **Reproduction, BASE versus fixed.** A scratch test ran in a BASE worktree and was then copied
  unchanged to the fixed tree. It used a failing `/bin/cat` work-state command and one worker, with
  six failed ticks and the worker ending while the reader was failing. Each tree ran it three times,
  at load 69/64/57. The test was deleted afterwards.

  | Measure | BASE | Fixed |
  | --- | --- | --- |
  | Ledger `workState` | absent | present |
  | Alerts | 7 | 7 |
  | `needs-owner` | 0 | 1 |
  | Cooldown detail | `{}` | `{workState: UNKNOWN}` |
  | `noProgress` | 1 | 1 |

- **Mutations.** Each mutation was run at load 59, and each made its test fail:
  - M1: status without the `workState` object fails both CLI tests.
  - M2: dropping the `noteWorkState` call fails the CAL-V0-270 and 271 subtests.
  - M3: dropping the `!Escalated` guard (per-tick escalation) fails the 270 and 271 subtests.
  - M4: dropping the recovery event fails the 270 subtest.
  - M5: dropping the `markWorkStateUnknown` call fails both UNKNOWN subtests of CAL-V0-272.
  - M6: dropping the launch hold fails the CAL-V0-273 `held` subtest.
  - M7: dropping `workState` from the strict member allowlist fails
    `TestCALV0270_ConfigAndLedgerBounds`.
- **Strict reader.** A ledger that carries progress, infra-retry, budget or stall state is read
  through a member allowlist. The first focused package run (load 56) failed nine dispatch tests and
  one CLI test with "malformed progress JSON", because the allowlist lacked the two version 5
  members. Both members are now on it, and the bounds test pins the allowlist.
- **Mutations for the added scope.** Each was run at load 66 to 70, and each made its test fail:
  - M8: keeping `signal: killed` fails `TestCALV0274_…` ("timeout not named").
  - M9: never marking the outage's first alert fails `TestCALV0274_…` ("unavailable events []").
  - M10: no stale baseline fails `TestCALV0276_…` ("one failed read emitted 2 state events").
  - M11: no terminal suppression fails `TestCALV0280_…`.
  - M13 and M14 are under Independent review below.
  - M12: ignoring `timeoutSeconds` fails `TestCALV0274_…` (the run took 60 s instead of 1 s).
- **Changed expectations.**
  - `TestCALV0053_ReaderFailureAndRealUnknownRemainObservable`:
    - A first failed read now emits one event, the alert, and no `state` event.
    - The read is still `UNKNOWN`, the ledger records it as failing, and the baseline keeps the
      last known state.
    - Recovery emits the state transition and the CAL-V0-271 recovery alert.
  - `TestCALV0053_ReaderLocalTimeout` now asserts `timed out after 60s` in the health record.
  - CAL-V0-053's text fixes neither event counts nor error text, so it is qualified only through
    the proposed CAL-V0-275 and 276.
- **Unrelated store failures.** The dependent-package run of `internal/tasks/store` failed 13 PSR
  and CAL-V0-089 tests at load about 70, with "context deadline exceeded" and "barrier not reached".
  `TestPSRHistoricalReplayDeadline` and `TestPSRSweepOwnerBarrier` fail the same way on BASE
  `ff96980fd` at load 62. The package does not use `internal/tasks/dispatch` outside its test
  dependencies. These are recorded as load flakes, not as regressions.
- **Process note.** Two long test runs went to background shells after they passed the tool time
  limit. The lane charter forbids background jobs. Both finished, and later runs were split to stay
  in the foreground.
- **New tests.**
  - In `internal/tasks/dispatch`: `TestCALV0270_FailingWorkStateCommandHealthEscalatesOnceAndRecovers`,
    `TestCALV0270_ConfigAndLedgerBounds`, `TestCALV0272_UnknownWorkStateRunsAreRecorded`,
    `TestCALV0273_LaunchHoldIsOptInAndLifts`, `TestCALV0274_TimedOutReadIsNamedAndUnavailableOnce`
    (the timeout fixture), `TestCALV0276_OneFailedReadEmitsNoPerTicketStateEvents` (the
    zero-events fixture) and `TestCALV0280_TerminalTicketWorkStateEmitsNoStateEvent`.
  - In `internal/tasks/cli`: `TestCALV0270_DispatchStatusShowsFailingWorkStateAfterOneTick`, which
    is the issue's reproduction through `dispatch --once` and `dispatch status`, and
    `TestCALV0270_DispatchStatusWorkState`.
- **Updated tests.**
  - The ledger schema pin for `/5`, whose hash is `c5b600f2…`.
  - The "next version" fixtures, moved to `/6`.
  - `adoptOnlyProfiles`, which now includes `/4`.

## Independent review

One Codex round (`codex exec -s read-only`) reviewed `4f1cd96e2..HEAD`. It found no path that feeds
the stale baseline into a decision. It reported four findings, and all four were addressed before
binding.

1. **P2: plan churn.** A failed read also empties the CAL-V0-105 held set, so the replanned window
   changed tickets' plan and reason and still emitted `state` events. The outage fixture had no
   roles, so it missed this.
   - Fixed: inside the window, the baseline keeps an `UNKNOWN` ticket's plan and reason as well.
   - Test: `TestCALV0276_FailedReadKeepsHeldPlansInBaseline`.
   - M13, keeping only the work state, fails it with `state` events on `h1`.
2. **P2: `/4` with live workers.** A `/4` ledger with live workers is adopted, while the spec said
   "drained".
   - This follows the `/3` precedent of TOL-V0-021: version 5 only adds optional members, so a
     running worker loses nothing.
   - The CAL-V0-270 text now says so, and the bounds test pins it.
3. **P2: hold enabled mid-outage.** A reload that enabled or lowered the hold mid-outage held
   launches without a `throttled` event.
   - Fixed: the event fires on the transition into the hold. A process flag is seeded at `Open`, so
     a restart while held does not repeat it.
   - Test: subtest `enabled mid-outage`. M14, announcing only when `failedTicks == threshold`,
     fails it with 0 events.
4. **P3: explicit 0.** An explicit 0 selects the default for the new numbers. Like the other
   optional dispatcher numbers, the field does not keep presence. The spec now says that an
   explicit 0 is read as absent, and the code is unchanged.

## Owner decisions

- Amend CAL-V0-057 to exempt marked runs from cooldown and park.
- Amend TQD-V0-001 for `doctor` reporting.
- Accept or decline the proposed CAL-V0-275, 276 and 280.
- Optionally let decisions use the stale state, which would amend CAL-V0-053's "never a guessed
  state". This is not implemented.

## Residual

- The process-table and pressure readers have no health record of their own (see Decisions).
- With the `status-line` kind, a partial failure inside the CAL-V0-276 window treats a ticket's
  literal `UNKNOWN` value as stale for the event baseline only.

- `laterMembers` for drained `/1` and `/2` ledgers does not exclude the version 4 `seen.obligations`
  member, so a drained `/1` or `/2` ledger carrying it would be adopted rather than refused. This
  predates this change; it is noted, not changed here.
